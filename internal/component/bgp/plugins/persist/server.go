// Design: docs/architecture/core-design.md — route persistence plugin
// Related: register.go — plugin registration

package persist

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/textparse"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// persistMetrics holds Prometheus metrics for the persist plugin.
type persistMetrics struct {
	routesStored metrics.Gauge   // current stored routes for replay
	peersTracked metrics.Gauge   // tracked peers
	routeReplays metrics.Counter // replay events triggered
}

// persistMetricsPtr stores persist metrics, set by SetMetricsRegistry.
var persistMetricsPtr atomic.Pointer[persistMetrics]

// SetMetricsRegistry creates persist metrics from the given registry.
// Called via ConfigureMetrics callback before RunEngine.
func SetMetricsRegistry(reg metrics.Registry) {
	m := &persistMetrics{
		routesStored: reg.Gauge("ze_persist_routes_stored", "Routes stored for replay."),
		peersTracked: reg.Gauge("ze_persist_peers_tracked", "Tracked peers."),
		routeReplays: reg.Counter("ze_persist_route_replays_total", "Replay events triggered."),
	}
	persistMetricsPtr.Store(m)
}

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
	Family family.Family
	Prefix string // Full NLRI string including type keyword (e.g., "prefix 192.168.1.0/24")
}

// PersistPeer tracks per-peer state for the persist plugin.
type PersistPeer struct {
	Address  string
	ASN      uint32
	Up       bool
	Families map[family.Family]bool // Negotiated families from OPEN

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
	ribOut map[string]map[family.Family]map[string]*StoredRoute // peer → family → prefix → StoredRoute
	mu     sync.RWMutex

	// updateRouteHook is called instead of updateRoute for test inspection.
	// Nil in production.
	updateRouteHook func(peer, cmd string)
}

// RunPersistServer runs the persist plugin using the SDK RPC protocol.
func RunPersistServer(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-persist", conn)
	defer func() { _ = p.Close() }()

	ps := &PersistServer{
		plugin: p,
		peers:  make(map[string]*PersistPeer),
		ribOut: make(map[string]map[family.Family]map[string]*StoredRoute),
	}

	// Structured event handler for DirectBridge delivery.
	// State events use metadata fields directly. UPDATE/OPEN events dispatch
	// to appropriate handlers based on EventType.
	p.OnStructuredEvent(func(events []any) error {
		for _, event := range events {
			se, ok := event.(*rpc.StructuredEvent)
			if !ok || se.PeerAddress == "" {
				continue
			}
			switch se.EventType { //nolint:exhaustive // only state+update+open handled on structured path
			case rpc.EventKindState:
				ps.handleStructuredState(se)
			case rpc.EventKindUpdate:
				ps.handleSentStructured(se)
				ps.updateStoredRoutesMetric()
			case rpc.EventKindOpen:
				ps.handleOpenStructured(se)
			}
		}
		return nil
	})

	// Fallback: text event handler for UPDATE/OPEN and non-DirectBridge delivery.
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

	ctx, cancel := sdk.SignalContext()
	defer cancel()
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
		ps.updateStoredRoutesMetric()
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
		ps.ribOut[peerAddr] = make(map[family.Family]map[string]*StoredRoute)
	}

	for fam, familyOps := range ops {
		for _, op := range familyOps {
			for _, nlri := range op.NLRIs {
				prefix, ok := nlri.(string)
				if !ok {
					continue
				}

				switch op.Action {
				case "add":
					if ps.ribOut[peerAddr][fam] == nil {
						ps.ribOut[peerAddr][fam] = make(map[string]*StoredRoute)
					}
					// Release old entry if replacing.
					if old, exists := ps.ribOut[peerAddr][fam][prefix]; exists && old.MsgID != msgID {
						ps.updateRoute(peerAddr, fmt.Sprintf("cache %d release", old.MsgID))
					}
					ps.ribOut[peerAddr][fam][prefix] = &StoredRoute{
						MsgID:  msgID,
						Family: fam,
						Prefix: prefix,
					}
					ps.updateRoute(peerAddr, fmt.Sprintf("cache %d retain", msgID))

				case "del":
					familyRoutes := ps.ribOut[peerAddr][fam]
					if familyRoutes == nil {
						continue
					}
					if old, exists := familyRoutes[prefix]; exists {
						ps.updateRoute(peerAddr, fmt.Sprintf("cache %d release", old.MsgID))
						delete(familyRoutes, prefix)
					}
					// Clean up empty maps
					if len(familyRoutes) == 0 {
						delete(ps.ribOut[peerAddr], fam)
					}
					if len(ps.ribOut[peerAddr]) == 0 {
						delete(ps.ribOut, peerAddr)
					}
				}
			}
		}
	}
}

// updateStoredRoutesMetric refreshes the routes_stored gauge from current ribOut state.
func (ps *PersistServer) updateStoredRoutesMetric() {
	m := persistMetricsPtr.Load()
	if m == nil {
		return
	}
	ps.mu.RLock()
	total := 0
	for _, families := range ps.ribOut {
		for _, routes := range families {
			total += len(routes)
		}
	}
	ps.mu.RUnlock()
	m.routesStored.Set(float64(total))
}

// handleSentStructured processes a sent UPDATE from StructuredEvent wire types.
// Builds NLRI family operations from WireUpdate and delegates to the existing store logic.
func (ps *PersistServer) handleSentStructured(se *rpc.StructuredEvent) {
	msg, ok := se.RawMessage.(*bgptypes.RawMessage)
	if !ok || msg == nil || msg.WireUpdate == nil {
		return
	}

	wu := msg.WireUpdate
	ctx := bgpctx.Registry.Get(wu.SourceCtxID())
	peerAddr := se.PeerAddress
	msgID := se.MessageID

	// Build fam operations from wire sections.
	ops := make(map[family.Family][]persistFamilyOp)

	// IPv4 unicast announces.
	nlriData, err := wu.NLRI()
	if err == nil && len(nlriData) > 0 {
		addPath := ctx != nil && ctx.AddPath(family.IPv4Unicast)
		nlris := persistWireNLRIs(nlriData, addPath, false)
		if len(nlris) > 0 {
			ops[family.IPv4Unicast] = append(ops[family.IPv4Unicast], persistFamilyOp{Action: "add", NLRIs: nlris})
		}
	}

	// IPv4 unicast withdrawals.
	wdData, err := wu.Withdrawn()
	if err == nil && len(wdData) > 0 {
		addPath := ctx != nil && ctx.AddPath(family.IPv4Unicast)
		nlris := persistWireNLRIs(wdData, addPath, false)
		if len(nlris) > 0 {
			ops[family.IPv4Unicast] = append(ops[family.IPv4Unicast], persistFamilyOp{Action: "del", NLRIs: nlris})
		}
	}

	// MP_REACH_NLRI announces.
	mpReach, err := wu.MPReach()
	if err == nil && mpReach != nil {
		fam := mpReach.Family()
		nlriBytes := mpReach.NLRIBytes()
		if len(nlriBytes) > 0 {
			addPath := ctx != nil && ctx.AddPath(fam)
			nlris := persistWireNLRIs(nlriBytes, addPath, fam.AFI == family.AFIIPv6)
			if len(nlris) > 0 {
				ops[fam] = append(ops[fam], persistFamilyOp{Action: "add", NLRIs: nlris})
			}
		}
	}

	// MP_UNREACH_NLRI withdrawals.
	mpUnreach, err := wu.MPUnreach()
	if err == nil && mpUnreach != nil {
		fam := mpUnreach.Family()
		wdBytes := mpUnreach.WithdrawnBytes()
		if len(wdBytes) > 0 {
			addPath := ctx != nil && ctx.AddPath(fam)
			nlris := persistWireNLRIs(wdBytes, addPath, fam.AFI == family.AFIIPv6)
			if len(nlris) > 0 {
				ops[fam] = append(ops[fam], persistFamilyOp{Action: "del", NLRIs: nlris})
			}
		}
	}

	// Use the existing store logic.
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.ribOut[peerAddr] == nil {
		ps.ribOut[peerAddr] = make(map[family.Family]map[string]*StoredRoute)
	}

	for fam, familyOps := range ops {
		for _, op := range familyOps {
			for _, n := range op.NLRIs {
				prefix, ok := n.(string)
				if !ok {
					continue
				}
				if op.Action == "add" {
					if ps.ribOut[peerAddr][fam] == nil {
						ps.ribOut[peerAddr][fam] = make(map[string]*StoredRoute)
					}
					if old, exists := ps.ribOut[peerAddr][fam][prefix]; exists && old.MsgID != msgID {
						ps.updateRoute(peerAddr, fmt.Sprintf("cache %d release", old.MsgID))
					}
					ps.ribOut[peerAddr][fam][prefix] = &StoredRoute{MsgID: msgID, Family: fam, Prefix: prefix}
					ps.updateRoute(peerAddr, fmt.Sprintf("cache %d retain", msgID))
				} else if op.Action == "del" {
					familyRoutes := ps.ribOut[peerAddr][fam]
					if familyRoutes == nil {
						continue
					}
					if old, exists := familyRoutes[prefix]; exists {
						ps.updateRoute(peerAddr, fmt.Sprintf("cache %d release", old.MsgID))
						delete(familyRoutes, prefix)
					}
					if len(familyRoutes) == 0 {
						delete(ps.ribOut[peerAddr], fam)
					}
					if len(ps.ribOut[peerAddr]) == 0 {
						delete(ps.ribOut, peerAddr)
					}
				}
			}
		}
	}
}

// handleOpenStructured processes an OPEN event from StructuredEvent wire types.
// Extracts ASN and negotiated families from raw OPEN wire bytes.
func (ps *PersistServer) handleOpenStructured(se *rpc.StructuredEvent) {
	msg, ok := se.RawMessage.(*bgptypes.RawMessage)
	if !ok || msg == nil || msg.RawBytes == nil {
		return
	}

	open, err := message.UnpackOpen(msg.RawBytes)
	if err != nil {
		return
	}

	asn := uint32(open.MyAS)
	if open.ASN4 > 0 {
		asn = open.ASN4
	}

	families := make(map[family.Family]bool)
	hasMultiprotocol := false

	// Parse capabilities from optional parameters to find multiprotocol families.
	// RFC 3392/5492: Type 2 = Capabilities Optional Parameter.
	capData := open.OptionalParams
	offset := 0
	for offset < len(capData) {
		if offset+2 > len(capData) {
			break
		}
		paramType := capData[offset]
		paramLen := int(capData[offset+1])
		offset += 2
		if offset+paramLen > len(capData) {
			break
		}
		if paramType == 2 {
			// Walk capability TLVs within this parameter.
			capOffset := 0
			paramData := capData[offset : offset+paramLen]
			for capOffset < len(paramData) {
				if capOffset+2 > len(paramData) {
					break
				}
				capCode := paramData[capOffset]
				capLen := int(paramData[capOffset+1])
				capOffset += 2
				if capOffset+capLen > len(paramData) {
					break
				}
				if capCode == 1 && capLen == 4 { // Multiprotocol (code 1)
					afi := family.AFI(uint16(paramData[capOffset])<<8 | uint16(paramData[capOffset+1]))
					safi := family.SAFI(paramData[capOffset+3])
					families[family.Family{AFI: afi, SAFI: safi}] = true
					hasMultiprotocol = true
				} else if capCode == 65 && capLen == 4 { // ASN4 capability
					asn = uint32(paramData[capOffset])<<24 | uint32(paramData[capOffset+1])<<16 |
						uint32(paramData[capOffset+2])<<8 | uint32(paramData[capOffset+3])
				}
				capOffset += capLen
			}
		}
		offset += paramLen
	}

	// RFC 4760: implicit ipv4/unicast if no multiprotocol capability.
	if !hasMultiprotocol {
		families[family.IPv4Unicast] = true
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	peer := ps.peers[se.PeerAddress]
	if peer == nil {
		peer = &PersistPeer{Address: se.PeerAddress}
		ps.peers[se.PeerAddress] = peer
	}
	peer.ASN = asn
	peer.Families = families
}

// persistWireNLRIs walks wire NLRI bytes and returns prefix strings as []any.
// Uses stack-allocated [16]byte buffer to avoid per-prefix heap allocation.
func persistWireNLRIs(data []byte, addPath, isIPv6 bool) []any {
	addrLen := 4
	if isIPv6 {
		addrLen = 16
	}
	var result []any
	var buf [16]byte // stack-allocated — large enough for IPv6
	offset := 0
	for offset < len(data) {
		if addPath {
			if offset+4 >= len(data) {
				break
			}
			offset += 4
		}
		if offset >= len(data) {
			break
		}
		prefixLen := int(data[offset])
		byteCount := (prefixLen + 7) / 8
		offset++
		if offset+byteCount > len(data) {
			break
		}
		clear(buf[:])
		copy(buf[:], data[offset:offset+byteCount])
		offset += byteCount
		addr, ok := netip.AddrFromSlice(buf[:addrLen])
		if !ok {
			continue
		}
		result = append(result, netip.PrefixFrom(addr, prefixLen).String())
	}
	return result
}

// handleStructuredState processes a structured state event from DirectBridge.
func (ps *PersistServer) handleStructuredState(se *rpc.StructuredEvent) {
	peerAddr := se.PeerAddress

	ps.mu.Lock()

	if se.State == rpc.SessionStateUp {
		peer := ps.peers[peerAddr]
		if peer == nil {
			peer = &PersistPeer{Address: peerAddr}
			ps.peers[peerAddr] = peer
		}
		peer.Up = true
		peer.replayGen++
		gen := peer.replayGen
		ps.mu.Unlock()
		if m := persistMetricsPtr.Load(); m != nil {
			m.routeReplays.Inc()
			m.peersTracked.Set(float64(len(ps.peers)))
		}
		go ps.replayForPeer(peerAddr, gen)
		return
	}

	if se.State == rpc.SessionStateDown {
		if peer := ps.peers[peerAddr]; peer != nil {
			peer.Up = false
		}
	}
	ps.mu.Unlock()
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
		if m := persistMetricsPtr.Load(); m != nil {
			m.routeReplays.Inc()
			m.peersTracked.Set(float64(len(ps.peers)))
		}
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

	peerFamilies := ps.ribOut[peerAddr]
	if len(peerFamilies) == 0 {
		// No routes to replay — still send EOR.
		families := ps.peerFamilies(peerAddr)
		ps.mu.RUnlock()
		ps.sendEOR(peerAddr, families)
		return
	}

	// Collect routes to replay — flatten all families.
	type replayEntry struct {
		msgID uint64
	}
	var entries []replayEntry
	for _, familyRoutes := range peerFamilies {
		for _, route := range familyRoutes {
			entries = append(entries, replayEntry{msgID: route.MsgID})
		}
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

		ps.updateRoute(peerAddr, fmt.Sprintf("cache %d forward %s", entry.msgID, peerAddr))
	}

	ps.sendEOR(peerAddr, families)
}

// peerFamilies returns the negotiated families for a peer. Caller must hold ps.mu (read).
func (ps *PersistServer) peerFamilies(peerAddr string) map[family.Family]bool {
	peer := ps.peers[peerAddr]
	if peer == nil || len(peer.Families) == 0 {
		return nil
	}
	fam := make(map[family.Family]bool, len(peer.Families))
	maps.Copy(fam, peer.Families)
	return fam
}

// sendEOR sends End-of-RIB markers for each negotiated family.
func (ps *PersistServer) sendEOR(peerAddr string, families map[family.Family]bool) {
	for fam := range families {
		ps.updateRoute(peerAddr, fmt.Sprintf("update text nlri %s eor", fam))
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
	families map[family.Family]bool
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
	// remote as
	if tok, ok := s.Next(); !ok || tok != "remote" {
		return "", 0, "", "", fmt.Errorf("missing remote keyword")
	}
	if tok, ok := s.Next(); !ok || tok != "as" {
		return "", 0, "", "", fmt.Errorf("missing as keyword")
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
// Unknown family names are dropped at the parse boundary.
func parsePersistNLRIOps(text string) map[family.Family][]persistFamilyOp {
	result := make(map[family.Family][]persistFamilyOp)
	s := textparse.NewScanner(strings.TrimRight(text, "\n"))

	// Skip header: peer <addr> remote as <n> <dir> update <id>
	for i := 0; i < 8 && !s.Done(); i++ {
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
			famStr, ok := s.Next()
			if !ok {
				continue
			}
			fam, ok := family.LookupFamily(famStr)
			if !ok {
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
				result[fam] = append(result[fam], persistFamilyOp{Action: action, NLRIs: nlris})
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
		case "remote":
			// remote as <n>
			if as, ok := s.Next(); ok && as == "as" {
				if v, ok := s.Next(); ok {
					if n, err := strconv.ParseUint(v, 10, 32); err == nil {
						event.asn = uint32(n) //nolint:gosec // bounded by ParseUint bitSize=32
					}
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
		families: make(map[family.Family]bool),
	}

	// remote as <n>
	s.Next() // "remote"
	s.Next() // "as"
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
				if value, ok := s.Next(); ok {
					if fam, ok := family.LookupFamily(value); ok {
						event.families[fam] = true
						hasMultiprotocol = true
					}
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
		event.families[family.IPv4Unicast] = true
	}

	return event
}
