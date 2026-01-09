// Package persist implements a route persistence plugin for ZeBGP.
// It tracks routes sent to peers and replays them on reconnect.
package persist

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"sync"
)

// Persister implements a BGP route persistence plugin.
// It tracks routes sent to peers and replays them on session re-establishment.
type Persister struct {
	input  *bufio.Scanner
	output io.Writer
	// ribOut stores routes per peer: peerAddr -> routes
	ribOut map[string]map[string]*Route
	// peerUp tracks which peers are currently up
	peerUp map[string]bool
	mu     sync.RWMutex
	serial int
}

// Route represents a stored route.
type Route struct {
	MsgID   uint64
	Family  string
	Prefix  string
	NextHop string
}

// routeKey creates a unique key for a route.
func routeKey(family, prefix string) string {
	return family + ":" + prefix
}

// MaxLineSize is the maximum size of a single JSON event line (1MB).
const MaxLineSize = 1024 * 1024

// NewPersister creates a new Persister.
func NewPersister(input io.Reader, output io.Writer) *Persister {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, MaxLineSize), MaxLineSize)
	return &Persister{
		input:  scanner,
		output: output,
		ribOut: make(map[string]map[string]*Route),
		peerUp: make(map[string]bool),
	}
}

// Run starts the persister event loop.
func (p *Persister) Run() int {
	// 5-stage plugin registration protocol
	p.doStartupProtocol()

	for p.input.Scan() {
		line := p.input.Bytes()
		if len(line) == 0 {
			continue
		}

		event, err := parseEvent(line)
		if err != nil {
			continue
		}

		p.dispatch(event)
	}

	if err := p.input.Err(); err != nil {
		return 1
	}

	return 0
}

// doStartupProtocol performs the 5-stage plugin registration protocol.
func (p *Persister) doStartupProtocol() {
	// Stage 1: Declaration
	p.send("declare cmd persist status")
	p.send("declare cmd persist routes")
	p.send("declare done")

	// Stage 2: Wait for config (discard)
	p.waitForLine("config done")

	// Stage 3: No capabilities
	p.send("capability done")

	// Stage 4: Wait for registry (discard)
	p.waitForLine("registry done")

	// Stage 5: Ready
	p.send("ready")
}

// waitForLine reads lines until one matches the expected line.
func (p *Persister) waitForLine(expected string) {
	for p.input.Scan() {
		line := p.input.Text()
		if line == expected {
			return
		}
	}
}

// sendCommand sends a numbered command to ZeBGP.
func (p *Persister) sendCommand(cmd string) {
	p.serial++
	_, _ = fmt.Fprintf(p.output, "#%d %s\n", p.serial, cmd)
}

// send sends raw output to ZeBGP.
func (p *Persister) send(format string, args ...any) {
	_, _ = fmt.Fprintf(p.output, format+"\n", args...)
}

// dispatch routes an event to the appropriate handler.
func (p *Persister) dispatch(event *Event) {
	switch event.Type {
	case "sent":
		p.handleSent(event)
	case "state":
		p.handleState(event)
	case "request":
		p.handleRequest(event)
	}
}

// handleSent processes sent UPDATE events.
// Stores routes in ribOut for replay on reconnect.
func (p *Persister) handleSent(event *Event) {
	peerAddr := event.Peer.Address
	msgID := event.MsgID

	if peerAddr == "" {
		return
	}

	// announce/withdraw are at top level of event, not in message wrapper
	if event.Announce == nil && event.Withdraw == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Initialize peer's ribOut if needed
	if p.ribOut[peerAddr] == nil {
		p.ribOut[peerAddr] = make(map[string]*Route)
	}

	// Process announcements - store routes
	// Format: {"ipv4/unicast": {"1.1.1.1": ["1.1.0.0/16", "2.2.0.0/16"]}}
	// family -> nexthop -> [prefixes]
	for family, nexthops := range event.Announce {
		for nexthop, prefixes := range nexthops {
			prefixList, ok := prefixes.([]any)
			if !ok {
				continue
			}
			for _, pv := range prefixList {
				prefix, ok := pv.(string)
				if !ok {
					continue
				}
				key := routeKey(family, prefix)
				p.ribOut[peerAddr][key] = &Route{
					MsgID:   msgID,
					Family:  family,
					Prefix:  prefix,
					NextHop: nexthop,
				}
			}
		}
	}

	// Process withdrawals - remove routes
	for family, prefixes := range event.Withdraw {
		for _, prefix := range prefixes {
			key := routeKey(family, prefix)
			delete(p.ribOut[peerAddr], key)
		}
	}
}

// handleState processes peer state changes.
func (p *Persister) handleState(event *Event) {
	peerAddr := event.Peer.Address
	state := event.State // State is at top level, not inside Peer

	p.mu.Lock()
	wasUp := p.peerUp[peerAddr]
	p.peerUp[peerAddr] = (state == "up")
	p.mu.Unlock()

	// On peer up, replay stored routes
	if state == "up" && !wasUp {
		p.handleStateUp(peerAddr)
	}
	// Note: We don't clear ribOut on down - that's the point of persist!
}

// handleStateUp replays stored routes to the peer.
// Uses API sync protocol to ensure routes arrive before EOR.
func (p *Persister) handleStateUp(peerAddr string) {
	p.mu.RLock()
	routes := p.ribOut[peerAddr]
	routesCopy := make([]*Route, 0, len(routes))
	for _, r := range routes {
		routesCopy = append(routesCopy, r)
	}
	p.mu.RUnlock()

	// Sort by MsgID to replay in original announcement order
	// (Go maps iterate in random order)
	sort.Slice(routesCopy, func(i, j int) bool {
		return routesCopy[i].MsgID < routesCopy[j].MsgID
	})

	// Replay all stored routes
	for _, route := range routesCopy {
		// Re-announce the route with its original next-hop
		p.send("peer %s announce route %s next-hop %s", peerAddr, route.Prefix, route.NextHop)
	}

	// Signal done with peer-specific ready - ZeBGP can now send EOR for this peer
	p.sendCommand("peer " + peerAddr + " session api ready")
}

// handleRequest processes command requests from ZeBGP.
func (p *Persister) handleRequest(event *Event) {
	serial := event.Serial
	command := event.Command

	switch command {
	case "persist status":
		p.respondDone(serial, p.statusJSON())
	case "persist routes":
		p.respondDone(serial, p.routesJSON())
	default:
		p.respondError(serial, "unknown command: "+command)
	}
}

// respondDone sends a successful response.
func (p *Persister) respondDone(serial, data string) {
	_, _ = fmt.Fprintf(p.output, "@%s done %s\n", serial, data)
}

// respondError sends an error response.
func (p *Persister) respondError(serial, message string) {
	_, _ = fmt.Fprintf(p.output, "@%s error %q\n", serial, message)
}

// statusJSON returns status as JSON.
func (p *Persister) statusJSON() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	routeCount := 0
	for _, routes := range p.ribOut {
		routeCount += len(routes)
	}

	return fmt.Sprintf(`{"running":true,"peers":%d,"routes":%d}`, len(p.peerUp), routeCount)
}

// routesJSON returns stored routes as JSON.
func (p *Persister) routesJSON() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Simple JSON construction
	result := `{"peers":{`
	first := true
	for peer, routes := range p.ribOut {
		if !first {
			result += ","
		}
		first = false
		result += fmt.Sprintf(`"%s":[`, peer)
		rfirst := true
		for _, r := range routes {
			if !rfirst {
				result += ","
			}
			rfirst = false
			result += fmt.Sprintf(`{"family":"%s","prefix":"%s","next-hop":"%s"}`, r.Family, r.Prefix, r.NextHop)
		}
		result += "]"
	}
	result += "}}"
	return result
}
