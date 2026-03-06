// Design: docs/architecture/core-design.md — route persistence plugin
// Related: register.go — plugin registration

package bgp_persist

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/textparse"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// updateRouteTimeout is the context deadline for updateRoute RPC calls.
const updateRouteTimeout = 30 * time.Second

// Event type and state constants.
const (
	persistEventUpdate = "update"
	persistEventState  = "state"
	persistEventOpen   = "open"

	persistStateUp   = "up"
	persistStateDown = "down"
)

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func persistLogger() *slog.Logger { return loggerPtr.Load() }

// SetPersistLogger configures the package-level logger for the persist plugin.
func SetPersistLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// StoredRoute represents a route stored in the ribOut for replay.
type StoredRoute struct {
	MsgID  uint64
	Family string
	Prefix string // Full NLRI string including type keyword (e.g., "prefix 192.168.1.0/24")
}

// PersistPeer tracks per-peer state for the persist plugin.
type PersistPeer struct {
	Address  string
	ASN      uint32
	Up       bool
	Families map[string]bool // Negotiated families from OPEN

	// replayGen guards against stale replay goroutines on rapid reconnect.
	// Incremented on each peer-up; replay goroutine checks before sending.
	replayGen uint64
}

// PersistServer implements the BGP route persistence plugin.
// It tracks outbound routes (sent UPDATEs) and replays them on peer reconnect
// using cache-forward commands.
type PersistServer struct {
	plugin *sdk.Plugin
	peers  map[string]*PersistPeer
	ribOut map[string]map[string]*StoredRoute // peer → routeKey → StoredRoute
	mu     sync.RWMutex

	// updateRouteHook is called instead of updateRoute for test inspection.
	// Nil in production.
	updateRouteHook func(peer, cmd string)
}

// RunPersistServer runs the persist plugin using the SDK RPC protocol.
func RunPersistServer(engineConn, callbackConn net.Conn) int {
	p := sdk.NewWithConn("bgp-persist", engineConn, callbackConn)
	defer func() { _ = p.Close() }()

	ps := &PersistServer{
		plugin: p,
		peers:  make(map[string]*PersistPeer),
		ribOut: make(map[string]map[string]*StoredRoute),
	}

	p.OnEvent(func(eventStr string) error {
		ps.dispatchText(eventStr)
		return nil
	})

	p.SetStartupSubscriptions([]string{
		"update direction sent",
		"state",
		"open direction received",
	}, nil, "")

	p.SetEncoding("text")

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		CacheConsumer: true,
	})
	if err != nil {
		persistLogger().Error("persist plugin failed", "error", err)
		return 1
	}

	return 0
}

// dispatchText parses and dispatches a text-format event line.
func (ps *PersistServer) dispatchText(text string) {
	eventType, msgID, peerAddr, payload, err := quickParsePersistEvent(text)
	if err != nil {
		persistLogger().Debug("persist: ignoring unparseable event", "error", err)
		return
	}

	switch eventType {
	case persistEventUpdate:
		ps.handleSentUpdate(peerAddr, msgID, payload)
	case persistEventState:
		ps.handleState(peerAddr, payload)
	case persistEventOpen:
		ps.handleOpen(peerAddr, payload)
	}
}

// handleSentUpdate processes a sent UPDATE event.
// Stores routes in ribOut and calls cache retain, or removes on withdrawal and calls release.
func (ps *PersistServer) handleSentUpdate(peerAddr string, msgID uint64, text string) {
	ops := parsePersistNLRIOps(text)

	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.ribOut[peerAddr] == nil {
		ps.ribOut[peerAddr] = make(map[string]*StoredRoute)
	}

	for family, familyOps := range ops {
		for _, op := range familyOps {
			for _, nlri := range op.NLRIs {
				prefix, ok := nlri.(string)
				if !ok {
					continue
				}
				routeKey := family + "|" + prefix

				switch op.Action {
				case "add":
					// Release old entry if replacing.
					if old, exists := ps.ribOut[peerAddr][routeKey]; exists && old.MsgID != msgID {
						ps.updateRoute(peerAddr, fmt.Sprintf("bgp cache %d release", old.MsgID))
					}
					ps.ribOut[peerAddr][routeKey] = &StoredRoute{
						MsgID:  msgID,
						Family: family,
						Prefix: prefix,
					}
					ps.updateRoute(peerAddr, fmt.Sprintf("bgp cache %d retain", msgID))

				case "del":
					if old, exists := ps.ribOut[peerAddr][routeKey]; exists {
						ps.updateRoute(peerAddr, fmt.Sprintf("bgp cache %d release", old.MsgID))
						delete(ps.ribOut[peerAddr], routeKey)
					}
				}
			}
		}
	}
}

// handleState processes a state event (up/down).
// On peer-up: triggers replay of stored routes.
// On peer-down: keeps ribOut intact for replay on reconnect.
func (ps *PersistServer) handleState(peerAddr, text string) {
	event := parsePersistState(text)
	if event == nil {
		return
	}

	ps.mu.Lock()

	if event.state == persistStateUp {
		peer := ps.peers[peerAddr]
		if peer == nil {
			peer = &PersistPeer{Address: peerAddr}
			ps.peers[peerAddr] = peer
		}
		peer.Up = true
		peer.replayGen++
		gen := peer.replayGen
		ps.mu.Unlock()
		go ps.replayForPeer(peerAddr, gen)
		return
	}

	if event.state == persistStateDown {
		if peer := ps.peers[peerAddr]; peer != nil {
			peer.Up = false
		}
	}
	ps.mu.Unlock()
}

// handleOpen processes an OPEN event, extracting negotiated families.
func (ps *PersistServer) handleOpen(peerAddr, text string) {
	event := parsePersistOpen(text)
	if event == nil {
		return
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	peer := ps.peers[peerAddr]
	if peer == nil {
		peer = &PersistPeer{Address: peerAddr}
		ps.peers[peerAddr] = peer
	}
	peer.ASN = event.asn
	peer.Families = event.families
}

// replayForPeer replays all stored routes to a peer via cache-forward commands,
// then sends EOR for each negotiated family.
func (ps *PersistServer) replayForPeer(peerAddr string, gen uint64) {
	ps.mu.RLock()
	peer := ps.peers[peerAddr]
	if peer == nil || peer.replayGen != gen {
		ps.mu.RUnlock()
		return
	}

	routes := ps.ribOut[peerAddr]
	if len(routes) == 0 {
		// No routes to replay — still send EOR.
		families := ps.peerFamilies(peerAddr)
		ps.mu.RUnlock()
		ps.sendEOR(peerAddr, families)
		return
	}

	// Collect routes to replay.
	type replayEntry struct {
		msgID uint64
	}
	entries := make([]replayEntry, 0, len(routes))
	for _, route := range routes {
		entries = append(entries, replayEntry{msgID: route.MsgID})
	}
	families := ps.peerFamilies(peerAddr)
	ps.mu.RUnlock()

	// Forward each cached route.
	for _, entry := range entries {
		// Check generation — abort if peer went down/up again.
		ps.mu.RLock()
		p := ps.peers[peerAddr]
		if p == nil || p.replayGen != gen {
			ps.mu.RUnlock()
			return
		}
		ps.mu.RUnlock()

		ps.updateRoute(peerAddr, fmt.Sprintf("bgp cache %d forward %s", entry.msgID, peerAddr))
	}

	ps.sendEOR(peerAddr, families)
}

// peerFamilies returns the negotiated families for a peer. Caller must hold ps.mu (read).
func (ps *PersistServer) peerFamilies(peerAddr string) map[string]bool {
	peer := ps.peers[peerAddr]
	if peer == nil || len(peer.Families) == 0 {
		return nil
	}
	fam := make(map[string]bool, len(peer.Families))
	maps.Copy(fam, peer.Families)
	return fam
}

// sendEOR sends End-of-RIB markers for each negotiated family.
func (ps *PersistServer) sendEOR(peerAddr string, families map[string]bool) {
	for family := range families {
		ps.updateRoute(peerAddr, fmt.Sprintf("update text nlri %s eor", family))
	}
}

// updateRoute sends a command to the engine via SDK or test hook.
func (ps *PersistServer) updateRoute(peer, cmd string) {
	if ps.updateRouteHook != nil {
		ps.updateRouteHook(peer, cmd)
		return
	}

	if ps.plugin == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), updateRouteTimeout)
	defer cancel()
	if _, _, err := ps.plugin.UpdateRoute(ctx, peer, cmd); err != nil {
		persistLogger().Error("updateRoute failed", "peer", peer, "cmd", cmd, "error", err)
	}
}

// persistEvent holds minimal parsed event data.
type persistEvent struct {
	state    string
	asn      uint32
	families map[string]bool
}

// quickParsePersistEvent extracts event type, message ID, peer address, and full text
// from a text-format event line.
func quickParsePersistEvent(text string) (string, uint64, string, string, error) {
	text = strings.TrimRight(text, "\n")
	s := textparse.NewScanner(text)

	// peer
	if tok, ok := s.Next(); !ok || tok != "peer" {
		return "", 0, "", "", fmt.Errorf("missing peer prefix")
	}
	// <addr>
	peerAddr, ok := s.Next()
	if !ok {
		return "", 0, "", "", fmt.Errorf("missing peer address")
	}
	// asn
	if tok, ok := s.Next(); !ok || tok != "asn" {
		return "", 0, "", "", fmt.Errorf("missing asn keyword")
	}
	// <n>
	if _, ok := s.Next(); !ok {
		return "", 0, "", "", fmt.Errorf("missing asn value")
	}

	// Next token: "state" or <direction>
	dispatchTok, ok := s.Next()
	if !ok {
		return "", 0, "", "", fmt.Errorf("missing dispatch token")
	}
	if dispatchTok == persistEventState {
		return persistEventState, 0, peerAddr, text, nil
	}

	// <direction> was consumed, next is <type> <id>
	eventType, ok := s.Next()
	if !ok {
		return "", 0, "", "", fmt.Errorf("missing event type")
	}
	idStr, ok := s.Next()
	if !ok {
		return eventType, 0, peerAddr, text, nil
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return eventType, 0, peerAddr, text, nil //nolint:nilerr // non-numeric ID valid for some events
	}

	return eventType, id, peerAddr, text, nil
}

// persistFamilyOp represents a single add/del operation for a family.
type persistFamilyOp struct {
	Action string
	NLRIs  []any
}

// parsePersistNLRIOps extracts family operations from a text UPDATE.
func parsePersistNLRIOps(text string) map[string][]persistFamilyOp {
	result := make(map[string][]persistFamilyOp)
	s := textparse.NewScanner(strings.TrimRight(text, "\n"))

	// Skip header: peer <addr> asn <n> <dir> update <id>
	for i := 0; i < 7 && !s.Done(); i++ {
		s.Next()
	}

	for !s.Done() {
		raw, ok := s.Next()
		if !ok {
			break
		}
		tok := textparse.ResolveAlias(raw)

		switch tok {
		case textparse.KWNextHop:
			s.Next() // consume the address

		case textparse.KWNLRI:
			family, ok := s.Next()
			if !ok || !strings.Contains(family, "/") {
				continue
			}

			// Optional path-id modifier
			next, ok := s.Peek()
			if !ok {
				continue
			}
			if textparse.ResolveAlias(next) == textparse.KWPathInformation {
				s.Next() // consume "info"/"path-information"
				s.Next() // consume the ID value
				if _, ok = s.Peek(); !ok {
					continue
				}
			}

			action, ok := s.Next()
			if !ok || (action != textparse.KWAdd && action != textparse.KWDel) {
				continue
			}

			var nlriTokens []string
			for !s.Done() {
				next, ok := s.Peek()
				if !ok || textparse.IsTopLevelKeyword(next) {
					break
				}
				tok, _ := s.Next()
				nlriTokens = append(nlriTokens, tok)
			}

			nlris := buildPersistNLRIEntries(nlriTokens)
			if len(nlris) > 0 {
				result[family] = append(result[family], persistFamilyOp{Action: action, NLRIs: nlris})
			}

		// Skip attribute keywords.
		case textparse.KWOrigin, textparse.KWMED, textparse.KWLocalPreference,
			textparse.KWAggregator, textparse.KWOriginatorID:
			s.Next()
		case textparse.KWASPath, textparse.KWCommunity, textparse.KWLargeCommunity,
			textparse.KWExtendedCommunity, textparse.KWClusterList:
			s.Next()
		case textparse.KWAtomicAggregate:
			// flag, no value
		}
	}

	return result
}

// buildPersistNLRIEntries splits collected tokens into individual NLRI strings.
func buildPersistNLRIEntries(tokens []string) []any {
	if len(tokens) == 0 {
		return nil
	}

	// Check for comma in any token.
	for i, tok := range tokens {
		if !strings.Contains(tok, ",") {
			continue
		}
		typePrefix := strings.Join(tokens[:i], " ")
		var nlris []any
		for part := range strings.SplitSeq(tok, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				if typePrefix != "" {
					nlris = append(nlris, typePrefix+" "+part)
				} else {
					nlris = append(nlris, part)
				}
			}
		}
		return nlris
	}

	// No commas — check for keyword boundary.
	if textparse.NLRITypeKeywords[tokens[0]] {
		var nlris []any
		var current []string
		for _, tok := range tokens {
			if tok == tokens[0] && len(current) > 0 {
				nlris = append(nlris, strings.Join(current, " "))
				current = nil
			}
			current = append(current, tok)
		}
		if len(current) > 0 {
			nlris = append(nlris, strings.Join(current, " "))
		}
		return nlris
	}

	return []any{strings.Join(tokens, " ")}
}

// parsePersistState extracts state from a text state event.
func parsePersistState(text string) *persistEvent {
	s := textparse.NewScanner(strings.TrimRight(text, "\n"))

	// peer <addr>
	s.Next()
	if _, ok := s.Next(); !ok {
		return nil
	}

	event := &persistEvent{}
	for !s.Done() {
		tok, ok := s.Next()
		if !ok {
			break
		}
		switch tok {
		case "asn":
			if v, ok := s.Next(); ok {
				if n, err := strconv.ParseUint(v, 10, 32); err == nil {
					event.asn = uint32(n) //nolint:gosec // bounded by ParseUint bitSize=32
				}
			}
		case persistEventState:
			if v, ok := s.Next(); ok {
				event.state = v
			}
		}
	}

	return event
}

// parsePersistOpen extracts families and ASN from a text OPEN event.
func parsePersistOpen(text string) *persistEvent {
	s := textparse.NewScanner(strings.TrimRight(text, "\n"))

	// peer <addr>
	s.Next()
	if _, ok := s.Next(); !ok {
		return nil
	}

	event := &persistEvent{
		families: make(map[string]bool),
	}

	// asn <n>
	s.Next() // "asn"
	if asnStr, ok := s.Next(); ok {
		if n, err := strconv.ParseUint(asnStr, 10, 32); err == nil {
			event.asn = uint32(n) //nolint:gosec // bounded by ParseUint bitSize=32
		}
	}

	// <dir> open <id>
	s.Next() // direction
	s.Next() // "open"
	s.Next() // message ID

	hasMultiprotocol := false
	for !s.Done() {
		tok, ok := s.Next()
		if !ok {
			break
		}
		if tok == "cap" {
			// cap <code> <name> [<value>]
			if _, ok := s.Next(); !ok {
				continue
			}
			name, ok := s.Next()
			if !ok {
				continue
			}

			if name == "multiprotocol" {
				if value, ok := s.Next(); ok && strings.Contains(value, "/") {
					event.families[value] = true
					hasMultiprotocol = true
				}
			} else {
				// Peek to consume optional value (not cap/router-id/hold-time).
				if next, ok := s.Peek(); ok && next != "cap" && next != "router-id" && next != "hold-time" {
					s.Next()
				}
			}
		}
	}

	// RFC 4760: implicit ipv4/unicast if no multiprotocol capability.
	if !hasMultiprotocol {
		event.families["ipv4/unicast"] = true
	}

	return event
}
