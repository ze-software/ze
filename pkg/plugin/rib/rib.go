// Package rib implements a RIB (Routing Information Base) plugin for ZeBGP.
// It tracks routes received from peers (Adj-RIB-In) and sent to peers (Adj-RIB-Out).
//
// RFC 7911: ADD-PATH path-id is included in route keys when present.
// Multiple paths to the same prefix with different path-ids are stored separately.
package rib

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"sync"
	"time"
)

func init() {
	// Configure slog to write to stderr with text format
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
}

// RIBManager implements a BGP RIB plugin.
// It tracks routes received from and sent to peers.
type RIBManager struct {
	input  *bufio.Scanner
	output io.Writer

	// ribIn stores routes received FROM peers (Adj-RIB-In)
	ribIn map[string]map[string]*Route // peerAddr -> routeKey -> route

	// ribOut stores routes sent TO peers (Adj-RIB-Out)
	ribOut map[string]map[string]*Route // peerAddr -> routeKey -> route

	// peerUp tracks which peers are currently up
	peerUp map[string]bool

	mu       sync.RWMutex // protects ribIn, ribOut, peerUp
	outputMu sync.Mutex   // protects output writes and serial
	serial   int
}

// Route represents a stored route.
// RFC 7911: PathID is included when ADD-PATH is negotiated.
type Route struct {
	MsgID     uint64    `json:"msg-id,omitempty"`
	Family    string    `json:"family"`
	Prefix    string    `json:"prefix"`
	PathID    uint32    `json:"path-id,omitempty"` // RFC 7911: ADD-PATH path identifier
	NextHop   string    `json:"next-hop"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// routeKey creates a unique key for a route.
// RFC 7911: When ADD-PATH is negotiated, path-id is part of the key.
func routeKey(family, prefix string, pathID uint32) string {
	if pathID == 0 {
		return family + ":" + prefix
	}
	return fmt.Sprintf("%s:%s:%d", family, prefix, pathID)
}

// MaxLineSize is the maximum size of a single JSON event line (1MB).
const MaxLineSize = 1024 * 1024

// NewRIBManager creates a new RIBManager.
func NewRIBManager(input io.Reader, output io.Writer) *RIBManager {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, MaxLineSize), MaxLineSize)
	return &RIBManager{
		input:  scanner,
		output: output,
		ribIn:  make(map[string]map[string]*Route),
		ribOut: make(map[string]map[string]*Route),
		peerUp: make(map[string]bool),
	}
}

// Run starts the RIB manager event loop.
func (r *RIBManager) Run() int {
	// 5-stage plugin registration protocol
	r.doStartupProtocol()

	for r.input.Scan() {
		line := r.input.Bytes()
		if len(line) == 0 {
			continue
		}

		event, err := parseEvent(line)
		if err != nil {
			slog.Warn("parse error", "error", err, "line", string(line[:min(100, len(line))]))
			continue
		}

		r.dispatch(event)
	}

	if err := r.input.Err(); err != nil {
		return 1
	}

	return 0
}

// doStartupProtocol performs the 5-stage plugin registration protocol.
func (r *RIBManager) doStartupProtocol() {
	// Stage 1: Declaration
	r.send("declare cmd rib status")
	r.send("declare cmd rib routes")
	r.send("declare cmd rib routes in")
	r.send("declare cmd rib routes out")
	r.send("declare done")

	// Stage 2: Wait for config (discard)
	r.waitForLine("config done")

	// Stage 3: No capabilities
	r.send("capability done")

	// Stage 4: Wait for registry (discard)
	r.waitForLine("registry done")

	// Stage 5: Ready
	r.send("ready")
}

// waitForLine reads lines until one matches the expected line.
func (r *RIBManager) waitForLine(expected string) {
	for r.input.Scan() {
		line := r.input.Text()
		if line == expected {
			return
		}
	}
}

// sendCommand sends a numbered command to ZeBGP.
func (r *RIBManager) sendCommand(cmd string) {
	r.outputMu.Lock()
	r.serial++
	_, _ = fmt.Fprintf(r.output, "#%d %s\n", r.serial, cmd)
	r.outputMu.Unlock()
}

// send sends raw output to ZeBGP.
func (r *RIBManager) send(format string, args ...any) {
	r.outputMu.Lock()
	_, _ = fmt.Fprintf(r.output, format+"\n", args...)
	r.outputMu.Unlock()
}

// dispatch routes an event to the appropriate handler.
func (r *RIBManager) dispatch(event *Event) {
	eventType := event.GetEventType()

	switch eventType {
	case "sent":
		r.handleSent(event)
	case "update":
		// Received UPDATE from peer
		r.handleReceived(event)
	case "state":
		r.handleState(event)
	case "request":
		r.handleRequest(event)
	}
}

// handleSent processes sent UPDATE events.
// Stores routes in ribOut for replay on reconnect.
func (r *RIBManager) handleSent(event *Event) {
	peerAddr := event.GetPeerAddress()
	msgID := event.GetMsgID()

	if peerAddr == "" {
		slog.Warn("sent event: empty peer address")
		return
	}

	if event.Announce == nil && event.Withdraw == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Initialize peer's ribOut if needed
	if r.ribOut[peerAddr] == nil {
		r.ribOut[peerAddr] = make(map[string]*Route)
	}

	// Process announcements - store routes
	// Format: {"ipv4/unicast": {"1.1.1.1": [{"prefix":"10.0.0.0/24","path-id":1}, ...]}}
	// Also supports legacy string format: {"ipv4/unicast": {"1.1.1.1": ["10.0.0.0/24", ...]}}
	for family, nexthops := range event.Announce {
		for nexthop, prefixes := range nexthops {
			prefixList, ok := prefixes.([]any)
			if !ok {
				slog.Warn("sent: unexpected announce format",
					"peer", peerAddr, "family", family, "nexthop", nexthop,
					"expected", "[]any", "got", fmt.Sprintf("%T", prefixes))
				continue
			}
			for _, pv := range prefixList {
				prefix, pathID := parseNLRIValue(pv)
				if prefix == "" {
					slog.Warn("sent: invalid nlri value",
						"peer", peerAddr, "family", family, "got", fmt.Sprintf("%T", pv))
					continue
				}
				key := routeKey(family, prefix, pathID)
				r.ribOut[peerAddr][key] = &Route{
					MsgID:   msgID,
					Family:  family,
					Prefix:  prefix,
					PathID:  pathID,
					NextHop: nexthop,
				}
			}
		}
	}

	// Process withdrawals - remove routes
	// Format: {"ipv4/unicast": [{"prefix":"10.0.0.0/24","path-id":1}, ...]}
	// Also supports legacy string format: {"ipv4/unicast": ["10.0.0.0/24", ...]}
	for family, nlris := range event.Withdraw {
		for _, nlriVal := range nlris {
			prefix, pathID := parseNLRIValue(nlriVal)
			if prefix == "" {
				continue
			}
			key := routeKey(family, prefix, pathID)
			delete(r.ribOut[peerAddr], key)
		}
	}
}

// parseNLRIValue extracts prefix and path-id from an NLRI value.
// Handles both new format {"prefix":"...", "path-id":N} and legacy string format.
func parseNLRIValue(v any) (prefix string, pathID uint32) {
	switch val := v.(type) {
	case string:
		// Legacy string format: just the prefix
		return val, 0
	case map[string]any:
		// New structured format: {"prefix":"...", "path-id":N}
		if p, ok := val["prefix"].(string); ok {
			prefix = p
		}
		if pid, ok := val["path-id"].(float64); ok {
			pathID = uint32(pid)
		}
		return prefix, pathID
	default:
		return "", 0
	}
}

// handleReceived processes received UPDATE events from peers.
// Stores routes in ribIn (Adj-RIB-In).
func (r *RIBManager) handleReceived(event *Event) {
	peerAddr := event.GetPeerAddress()
	msgID := event.GetMsgID()
	now := time.Now()

	if peerAddr == "" {
		slog.Warn("received event: empty peer address")
		return
	}

	if event.Announce == nil && event.Withdraw == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Initialize peer's ribIn if needed
	if r.ribIn[peerAddr] == nil {
		r.ribIn[peerAddr] = make(map[string]*Route)
	}

	// Process announcements - store routes
	// Format: {"ipv4/unicast": {"1.1.1.1": [{"prefix":"10.0.0.0/24","path-id":1}, ...]}}
	// Also supports legacy string format
	for family, nexthops := range event.Announce {
		for nexthop, prefixes := range nexthops {
			prefixList, ok := prefixes.([]any)
			if !ok {
				slog.Warn("received: unexpected announce format",
					"peer", peerAddr, "family", family, "nexthop", nexthop,
					"expected", "[]any", "got", fmt.Sprintf("%T", prefixes))
				continue
			}
			for _, pv := range prefixList {
				prefix, pathID := parseNLRIValue(pv)
				if prefix == "" {
					slog.Warn("received: invalid nlri value",
						"peer", peerAddr, "family", family, "got", fmt.Sprintf("%T", pv))
					continue
				}
				key := routeKey(family, prefix, pathID)
				r.ribIn[peerAddr][key] = &Route{
					MsgID:     msgID,
					Family:    family,
					Prefix:    prefix,
					PathID:    pathID,
					NextHop:   nexthop,
					Timestamp: now,
				}
			}
		}
	}

	// Process withdrawals - remove routes
	// Format: {"ipv4/unicast": [{"prefix":"10.0.0.0/24","path-id":1}, ...]}
	// Also supports legacy string format
	for family, nlris := range event.Withdraw {
		for _, nlriVal := range nlris {
			prefix, pathID := parseNLRIValue(nlriVal)
			if prefix == "" {
				continue
			}
			key := routeKey(family, prefix, pathID)
			delete(r.ribIn[peerAddr], key)
		}
	}
}

// handleState processes peer state changes.
// Handles state transitions atomically to avoid races between up/down events.
func (r *RIBManager) handleState(event *Event) {
	peerAddr := event.GetPeerAddress()
	state := event.GetPeerState()

	r.mu.Lock()
	wasUp := r.peerUp[peerAddr]
	isUp := state == "up"
	r.peerUp[peerAddr] = isUp

	var routesToReplay []*Route

	if isUp && !wasUp {
		// Peer came up - copy routes for replay while holding lock
		routes := r.ribOut[peerAddr]
		routesToReplay = make([]*Route, 0, len(routes))
		for _, rt := range routes {
			routesToReplay = append(routesToReplay, rt)
		}
	} else if !isUp && wasUp {
		// Peer went down - clear Adj-RIB-In while holding lock
		delete(r.ribIn, peerAddr)
	}
	r.mu.Unlock()

	// I/O operations after releasing lock
	if routesToReplay != nil {
		r.replayRoutes(peerAddr, routesToReplay)
	}
}

// replayRoutes sends stored routes to a peer that just came up.
// Called without lock held - safe for I/O.
func (r *RIBManager) replayRoutes(peerAddr string, routes []*Route) {
	// Sort by MsgID to replay in original announcement order
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].MsgID < routes[j].MsgID
	})

	// Replay all stored routes
	// RFC 7911: Include path-id when present
	for _, route := range routes {
		if route.PathID != 0 {
			r.send("peer %s announce route %s path-id %d next-hop %s", peerAddr, route.Prefix, route.PathID, route.NextHop)
		} else {
			r.send("peer %s announce route %s next-hop %s", peerAddr, route.Prefix, route.NextHop)
		}
	}

	// Signal done with peer-specific ready - ZeBGP can now send EOR for this peer
	r.sendCommand("peer " + peerAddr + " session api ready")
}

// handleRequest processes command requests from ZeBGP.
func (r *RIBManager) handleRequest(event *Event) {
	serial := event.Serial
	command := event.Command

	switch command {
	case "rib status":
		r.respondDone(serial, r.statusJSON())
	case "rib routes":
		r.respondDone(serial, r.routesJSON())
	case "rib routes in":
		r.respondDone(serial, r.routesInJSON())
	case "rib routes out":
		r.respondDone(serial, r.routesOutJSON())
	default:
		r.respondError(serial, "unknown command: "+command)
	}
}

// respondDone sends a successful response.
func (r *RIBManager) respondDone(serial, data string) {
	r.outputMu.Lock()
	_, _ = fmt.Fprintf(r.output, "@%s done %s\n", serial, data)
	r.outputMu.Unlock()
}

// respondError sends an error response.
func (r *RIBManager) respondError(serial, message string) {
	r.outputMu.Lock()
	_, _ = fmt.Fprintf(r.output, "@%s error %q\n", serial, message)
	r.outputMu.Unlock()
}

// statusJSON returns status as JSON.
func (r *RIBManager) statusJSON() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routesIn := 0
	for _, routes := range r.ribIn {
		routesIn += len(routes)
	}

	routesOut := 0
	for _, routes := range r.ribOut {
		routesOut += len(routes)
	}

	return fmt.Sprintf(`{"running":true,"peers":%d,"routes_in":%d,"routes_out":%d}`,
		len(r.peerUp), routesIn, routesOut)
}

// routesJSON returns all stored routes as JSON.
func (r *RIBManager) routesJSON() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := map[string]any{
		"adj_rib_in":  r.buildRoutesMap(r.ribIn),
		"adj_rib_out": r.buildRoutesMap(r.ribOut),
	}

	data, _ := json.Marshal(result)
	return string(data)
}

// routesInJSON returns Adj-RIB-In routes as JSON.
func (r *RIBManager) routesInJSON() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := map[string]any{
		"adj_rib_in": r.buildRoutesMap(r.ribIn),
	}

	data, _ := json.Marshal(result)
	return string(data)
}

// routesOutJSON returns Adj-RIB-Out routes as JSON.
func (r *RIBManager) routesOutJSON() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := map[string]any{
		"adj_rib_out": r.buildRoutesMap(r.ribOut),
	}

	data, _ := json.Marshal(result)
	return string(data)
}

// buildRoutesMap converts a RIB to a map for JSON serialization.
// RFC 7911: Includes path-id when non-zero.
func (r *RIBManager) buildRoutesMap(rib map[string]map[string]*Route) map[string][]map[string]any {
	result := make(map[string][]map[string]any)

	for peer, routes := range rib {
		routeList := make([]map[string]any, 0, len(routes))
		for _, rt := range routes {
			routeMap := map[string]any{
				"family":   rt.Family,
				"prefix":   rt.Prefix,
				"next-hop": rt.NextHop,
			}
			if rt.PathID != 0 {
				routeMap["path-id"] = rt.PathID
			}
			routeList = append(routeList, routeMap)
		}
		result[peer] = routeList
	}

	return result
}
