// Design: docs/architecture/core-design.md -- route reflector plugin
// Detail: withdrawal.go -- NLRI tracking and peer-down withdrawal
//
// RFC 4456: BGP Route Reflection.
// Subscribes to UPDATE events and forwards them to all peers via cache-forward.
// The reactor handles the RFC 4456 forwarding rules:
//   - Source exclusion (don't send back to source)
//   - Client/non-client filtering (client->all, non-client->clients only)
//   - ORIGINATOR_ID injection (source peer's BGP Identifier)
//   - CLUSTER_LIST prepend (reflector's cluster-id)
//   - Next-hop rewriting (per destination peer settings)

package rr

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/internal/core/textbuf"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// setLogger configures the package-level logger for the RR plugin.
func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

const (
	eventUpdate = "update"
	eventState  = "state"
	eventOpen   = "open"

	updateRouteTimeout     = 60 * time.Second
	replayConvergenceMax   = 10
	replayConvergenceDelay = 20 * time.Millisecond
	statusDone             = "done"
	statusError            = "error"

	// NLRI action tokens for withdrawal map tracking.
	actionAdd = "add"
	actionDel = "del"

	// withdrawalBatchSize caps the number of prefixes per withdrawal RPC.
	withdrawalBatchSize = 1000
)

// peerState tracks a connected peer's state and capabilities.
type peerState struct {
	Address      string
	ASN          uint32
	Up           bool
	ReplayGen    uint64 // Incremented on each state-up, guards stale goroutines
	Families     map[family.Family]bool
	Capabilities map[string]bool
}

// routeReflector implements a BGP Route Reflector plugin (RFC 4456).
// Subscribes to UPDATE events and forwards them to all peers via cache-forward.
// The reactor handles RFC 4456 forwarding rules, ORIGINATOR_ID injection,
// CLUSTER_LIST prepend, and next-hop rewriting.
type routeReflector struct {
	plugin   *sdk.Plugin
	peers    map[string]*peerState
	mu       sync.RWMutex
	buf      textbuf.Buffer
	stopping atomic.Bool

	// withdrawalMu protects the withdrawals map.
	withdrawalMu sync.Mutex
	// withdrawals tracks announced routes per source peer for withdrawal on peer-down.
	// sourcePeer -> routeKey (family|prefix) -> withdrawalInfo.
	withdrawals map[string]map[string]withdrawalInfo
}

// runRouteReflector runs the Route Reflector plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func runRouteReflector(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-rr", conn)
	defer func() { _ = p.Close() }()

	rr := &routeReflector{
		plugin:      p,
		peers:       make(map[string]*peerState),
		withdrawals: make(map[string]map[string]withdrawalInfo),
	}

	// Register structured event handler for DirectBridge delivery (hot path).
	p.OnStructuredEvent(func(events []any) error {
		for _, event := range events {
			se, ok := event.(*rpc.StructuredEvent)
			if !ok {
				continue
			}
			switch se.EventType {
			case rpc.EventKindUpdate:
				if msg, ok := se.RawMessage.(*bgptypes.RawMessage); ok {
					// Update withdrawal map BEFORE forwarding: the forward path can
					// trigger cache eviction which frees the pool buffer backing
					// msg.WireUpdate. Reading WireUpdate after forward is use-after-free.
					rr.withdrawalMu.Lock()
					rr.updateWithdrawalMapWire(se.PeerAddress, msg)
					rr.withdrawalMu.Unlock()
					rr.forwardUpdate(msg.MessageID)
				}
			case rpc.EventKindState:
				rr.handleStructuredState(se)
			case rpc.EventKindOpen:
				if msg, ok := se.RawMessage.(*bgptypes.RawMessage); ok {
					rr.handleStructuredOpen(se, msg)
				}
			case rpc.EventKindUnspecified, rpc.EventKindNotification, rpc.EventKindKeepalive,
				rpc.EventKindRefresh, rpc.EventKindEOR, rpc.EventKindBoRR, rpc.EventKindEoRR,
				rpc.EventKindSent, rpc.EventKindNegotiated, rpc.EventKindCount:
				// These events do not trigger reflection.
			default:
				// The plugin event set is open; unknown events do not trigger reflection.
			}
		}
		return nil
	})

	// Register text event handler for fork-mode delivery (fallback).
	p.OnEvent(func(eventStr string) error {
		rr.dispatchText(eventStr)
		return nil
	})

	// Register command handler.
	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, any, error) {
		return rr.handleCommand(command)
	})

	// Startup ownership comes from Registration.Claims. Mid-life auto-load
	// and restart deliver this callback to notify an already-running store.
	p.OnAllPluginsReady(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		status, _, err := rr.dispatchCommand(ctx, "request bgp adj-rib-in claim-replay")
		if err == nil && status != statusDone {
			err = fmt.Errorf("adj-rib-in replay claim returned status %q", status)
		}
		if err != nil {
			logger().Warn("could not claim adj-rib-in replay ownership; peer-up routes may be announced twice", "error", err)
		}
		return err
	})

	// Subscribe to received-direction only for UPDATE and OPEN events.
	// Same rationale as bgp-rs: subscribing to "both" for UPDATEs creates
	// a circular deadlock (ForwardUpdate -> onMessageSent -> deliver -> block).
	p.SetStartupSubscriptions([]string{
		eventUpdate + " direction received",
		eventState,
		eventOpen + " direction received",
	}, nil, "")

	p.SetEncoding("text")

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	defer rr.stopping.Store(true)
	if bus := validationBus.Load(); bus != nil {
		stop := rr.startValidation(ctx, *bus)
		defer stop()
	}
	err := p.Run(ctx, sdk.Registration{
		CacheConsumer:          true,
		CacheConsumerUnordered: true,
		Commands:               commandDecls(),
	})

	if err != nil {
		logger().Error("rr plugin failed", "error", err)
		return 1
	}

	return 0
}

// forwardUpdate forwards a cached UPDATE to all peers via cache-forward.
// The reactor handles source exclusion, client/non-client filtering (RFC 4456),
// ORIGINATOR_ID, CLUSTER_LIST, and next-hop rewriting.
func (rr *routeReflector) forwardUpdate(msgID uint64) {
	if rr.stopping.Load() {
		return
	}
	rr.updateRoute("*", rr.buf.Reset().Str("cached ").Uint(msgID).String())
}

// updateRoute sends a route update command to matching peers via the engine.
func (rr *routeReflector) updateRoute(peerSelector, command string) {
	ctx, cancel := context.WithTimeout(context.Background(), updateRouteTimeout)
	defer cancel()

	_, _, err := rr.plugin.UpdateRoute(ctx, peerSelector, command)
	if err != nil { //nolint:gocritic // ifElseChain: switch blocked by block-silent-ignore hook
		if rr.stopping.Load() {
			logger().Debug("update-route failed (shutting down)",
				"peer", peerSelector, "command", command, "error", err)
		} else if isConnectionError(err) {
			logger().Warn("update-route failed (peer disconnected)",
				"peer", peerSelector, "command", command, "error", err)
		} else {
			logger().Error("update-route failed",
				"peer", peerSelector, "command", command, "error", err)
		}
	}
}

// updateRouteSel sends a route update using a typed selector via DirectBridge.
func (rr *routeReflector) updateRouteSel(sel *selector.Selector, command string) {
	ctx, cancel := context.WithTimeout(context.Background(), updateRouteTimeout)
	defer cancel()

	_, _, err := rr.plugin.UpdateRouteSel(ctx, sel, command)
	if err != nil { //nolint:gocritic // ifElseChain: switch blocked by block-silent-ignore hook
		if rr.stopping.Load() {
			logger().Debug("update-route failed (shutting down)",
				"peer", sel, "command", command, "error", err)
		} else if isConnectionError(err) {
			logger().Warn("update-route failed (peer disconnected)",
				"peer", sel, "command", command, "error", err)
		} else {
			logger().Error("update-route failed",
				"peer", sel, "command", command, "error", err)
		}
	}
}

// isConnectionError reports whether err indicates the target peer's connection is closed.
func isConnectionError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "use of closed network connection")
}

// handleStructuredState processes state events from DirectBridge.
func (rr *routeReflector) handleStructuredState(se *rpc.StructuredEvent) {
	if se.PeerAddress == "" {
		return
	}

	rr.mu.Lock()
	if rr.peers[se.PeerAddress] == nil {
		rr.peers[se.PeerAddress] = &peerState{Address: se.PeerAddress}
	}
	peer := rr.peers[se.PeerAddress]
	peer.Up = (se.State == rpc.SessionStateUp)
	peer.ASN = se.PeerAS
	switch se.State {
	case rpc.SessionStateUp:
		peer.ReplayGen++
		gen := peer.ReplayGen
		rr.mu.Unlock()
		go rr.replayForPeer(se.PeerAddress, gen, se.InitialReplay)
	case rpc.SessionStateDown:
		rr.mu.Unlock()
		rr.handleStateDown(se.PeerAddress)
	case rpc.SessionStateUnspecified, rpc.SessionStateCount:
		rr.mu.Unlock()
	default:
		// The plugin state set is open; unknown states need no further action.
		rr.mu.Unlock()
	}
}

// handleStateDown sends withdrawals for all routes from the downed source peer.
// Routes are batched by family and wire form to minimize RPCs.
// Withdrawals are sent asynchronously (per-lifecycle goroutine, not hot path).
//
// Concurrency: OnStructuredEvent is called serially by the engine's delivery
// goroutine, so handleStateDown and UPDATE processing for the same peer cannot
// race within the structured path. The withdrawalMu guards against the
// (theoretical) case of text and structured paths processing the same peer
// concurrently.
func (rr *routeReflector) handleStateDown(peerAddr string) {
	rr.withdrawalMu.Lock()
	entries := rr.withdrawals[peerAddr]
	delete(rr.withdrawals, peerAddr)
	rr.withdrawalMu.Unlock()

	if len(entries) == 0 {
		return
	}

	// VPN and ADD-PATH labeled routes retain native bytes and negotiation.
	type withdrawalGroup struct {
		family   string
		wireForm bool
		addPath  bool
	}
	byGroup := make(map[withdrawalGroup][]string)
	for _, info := range entries {
		group := withdrawalGroup{family: info.Family, wireForm: info.WireForm, addPath: info.AddPath}
		byGroup[group] = append(byGroup[group], info.Prefix)
	}

	// Send batched withdrawals to all peers except the one that went down.
	// Cap each RPC at withdrawalBatchSize prefixes to bound command length.
	addr, err := netip.ParseAddr(peerAddr)
	if err != nil {
		logger().Error("invalid peer address in withdrawal", "peer", peerAddr, "error", err)
		return
	}
	excludeSel := selector.ExcludeAddr(addr)
	go func() {
		var command textbuf.Buffer
		for group, prefixes := range byGroup {
			slices.Sort(prefixes)
			for i := 0; i < len(prefixes); i += withdrawalBatchSize {
				end := min(i+withdrawalBatchSize, len(prefixes))
				if !group.wireForm {
					rr.updateRouteSel(excludeSel, nlriDelCmd(group.family, textbuf.Join(prefixes[i:end], ",")))
					continue
				}
				// RFC 8277 Section 2.4: "When using an MP_UNREACH_NLRI
				// attribute to withdraw a route whose NLRI was previously
				// specified in an MP_REACH_NLRI attribute, the lengths and
				// values of the respective prefixes must match, and the
				// respective AFI/SAFIs must match." Native bytes keep RD
				// and prefix; RFC 7911 framing keeps the identifier,
				// including zero.
				command.Reset().Str("update hex nlri ").Str(group.family)
				if group.addPath {
					command.Str(" addpath")
				}
				for _, prefix := range prefixes[i:end] {
					command.Str(" del ").Str(prefix)
				}
				rr.updateRouteSel(excludeSel, command.String())
			}
		}
	}()
}

// handleStructuredOpen processes OPEN events from DirectBridge.
// Decodes raw OPEN wire bytes to extract peer capabilities and families.
func (rr *routeReflector) handleStructuredOpen(se *rpc.StructuredEvent, msg *bgptypes.RawMessage) {
	if se.PeerAddress == "" || msg.RawBytes == nil {
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
	capabilities := make(map[string]bool)
	hasMP := false

	offset := 0
	for offset < len(open.OptionalParams) {
		if offset+2 > len(open.OptionalParams) {
			break
		}
		paramType := open.OptionalParams[offset]
		paramLen := int(open.OptionalParams[offset+1])
		offset += 2
		if offset+paramLen > len(open.OptionalParams) {
			break
		}
		if paramType == 2 { // Capability (RFC 3392)
			caps, parseErr := capability.Parse(open.OptionalParams[offset : offset+paramLen])
			if parseErr == nil {
				for _, c := range caps {
					if mp, ok := c.(*capability.Multiprotocol); ok {
						hasMP = true
						capabilities["multiprotocol"] = true
						families[family.Family{AFI: mp.AFI, SAFI: mp.SAFI}] = true
					}
					if asn4, ok := c.(*capability.ASN4); ok {
						asn = asn4.ASN
						capabilities["asn4"] = true
					}
				}
			}
		}
		offset += paramLen
	}

	// RFC 4760 Section 1: ipv4/unicast is the implicit default only when
	// the peer sends no Multiprotocol capability.
	if !hasMP {
		families[family.IPv4Unicast] = true
	}

	rr.mu.Lock()
	defer rr.mu.Unlock()

	if rr.peers[se.PeerAddress] == nil {
		rr.peers[se.PeerAddress] = &peerState{Address: se.PeerAddress}
	}
	peer := rr.peers[se.PeerAddress]
	peer.ASN = asn
	peer.Families = families
	peer.Capabilities = capabilities
}

// dispatchText routes text-format events to handlers (fork-mode fallback).
func (rr *routeReflector) dispatchText(text string) {
	if rr.stopping.Load() {
		return
	}

	// Quick parse: "peer <addr> remote as <n> <dir> <type> <id> ..."
	// State:       "peer <addr> remote as <n> state <state>"
	fields := strings.Fields(strings.TrimRight(text, "\n"))
	if len(fields) < 6 {
		return
	}

	// fields[5] is either "state" or the direction token.
	if fields[5] == eventState {
		if len(fields) >= 7 {
			peerAddr := fields[1]
			state := fields[6]
			var initialReplay uint64
			for i := 7; i < len(fields); i++ {
				if fields[i] != "initial-replay" {
					continue
				}
				i++
				if i == len(fields) {
					return
				}
				token, err := strconv.ParseUint(fields[i], 10, 64)
				if err != nil {
					return
				}
				initialReplay = token
			}

			rr.mu.Lock()
			if rr.peers[peerAddr] == nil {
				rr.peers[peerAddr] = &peerState{Address: peerAddr}
			}
			peer := rr.peers[peerAddr]
			peer.Up = (state == "up")
			switch state {
			case "up":
				peer.ReplayGen++
				gen := peer.ReplayGen
				rr.mu.Unlock()
				go rr.replayForPeer(peerAddr, gen, initialReplay)
			case "down":
				rr.mu.Unlock()
				rr.handleStateDown(peerAddr)
			default: // other states (e.g. "connected")
				rr.mu.Unlock()
			}
		}
		return
	}

	// Message events: fields[5]=direction [6]=type [7]=id
	if len(fields) < 8 {
		return
	}

	if fields[6] == eventUpdate {
		msgID, err := strconv.ParseUint(fields[7], 10, 64)
		if err != nil {
			return
		}
		// Update withdrawal map from text before forwarding.
		peerAddr := fields[1]
		ops := parseTextNLRIOps(text)
		if len(ops) > 0 {
			rr.withdrawalMu.Lock()
			rr.updateWithdrawalMapText(peerAddr, ops)
			rr.withdrawalMu.Unlock()
		}
		rr.forwardUpdate(msgID)
	}
}

// handleCommand processes command requests via SDK execute-command callback.
func (rr *routeReflector) handleCommand(command string) (string, any, error) {
	switch command {
	case "show rr status":
		return statusDone, map[string]any{"running": true}, nil
	case "show rr peers":
		return statusDone, rr.peerStatus(), nil
	default: // fail on unknown command
		return statusError, "", fmt.Errorf("unknown command: %s", command)
	}
}

// peerStatus returns peer state.
func (rr *routeReflector) peerStatus() any {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	peers := make([]map[string]any, 0, len(rr.peers))
	for _, p := range rr.peers {
		peers = append(peers, map[string]any{
			"address": p.Address,
			"remote":  map[string]any{"as": p.ASN},
			"up":      p.Up,
		})
	}

	return map[string]any{"peers": peers}
}

// replayForPeer replays existing routes to a newly-connected peer via adj-rib-in,
// then sends End-of-RIB markers per negotiated family. Runs in a per-peer
// lifecycle goroutine (not blocking the event loop).
//
// The gen parameter guards against rapid reconnects: if the peer's ReplayGen
// has changed by the time replay finishes, this goroutine is stale.
// initialReplay MUST be the receipt captured by this replay's peer-up event.
func (rr *routeReflector) replayForPeer(peerAddr string, gen, initialReplay uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// The mandatory selecting RIB owns FlowSpec replay independently of the
	// optional store. Complete it before EOR and the readiness report on both
	// successful and failed optional-store replay.
	var replayed int
	defer func() {
		replayed += rr.replayFlowSpecs(peerAddr, gen)
		if replayed > 0 {
			rr.sendEOR(peerAddr, gen)
		}
		rr.signalSessionReady(peerAddr, gen, initialReplay)
	}()

	// Full replay from adj-rib-in index 0.
	status, data, err := rr.dispatchCommand(ctx, "request bgp adj-rib-in replay", peerAddr, "0")
	if err != nil || status != statusDone {
		// Optional-store failure does not prevent mandatory FlowSpec replay.
		logger().Warn("replay failed", "peer", peerAddr, "status", status, "error", err)
		return
	}

	// Parse last-index for convergent delta replay.
	lastIndex, replayCount := parseReplayResponse(data)
	replayed = replayCount

	// Convergent delta replay: catch routes adj-rib-in received after the
	// full replay snapshot (race between event delivery and replay).
	for i := range replayConvergenceMax {
		if lastIndex == 0 {
			break
		}
		if i > 0 {
			time.Sleep(replayConvergenceDelay)
		}
		_, deltaData, deltaErr := rr.dispatchCommand(ctx, "request bgp adj-rib-in replay", peerAddr, textbuf.StringUint(lastIndex))
		if deltaErr != nil {
			logger().Warn("delta replay failed", "peer", peerAddr, "attempt", i, "error", deltaErr)
			break
		}
		newLast, deltaReplayed := parseReplayResponse(deltaData)
		if deltaReplayed == 0 {
			break
		}
		replayed += deltaReplayed
		logger().Debug("delta replay caught new routes", "peer", peerAddr, "attempt", i, "replayed", deltaReplayed)
		lastIndex = newLast
	}

}

// dispatchCommand sends a command to the engine via the SDK.
func (rr *routeReflector) dispatchCommand(ctx context.Context, command string, args ...string) (string, json.RawMessage, error) {
	return rr.plugin.DispatchCommandArgs(ctx, command, args, "")
}

// sendEOR sends End-of-RIB markers for each of the peer's negotiated families.
func (rr *routeReflector) sendEOR(peerAddr string, gen uint64) {
	rr.mu.RLock()
	p := rr.peers[peerAddr]
	if p == nil || p.ReplayGen != gen || len(p.Families) == 0 {
		rr.mu.RUnlock()
		return
	}
	families := make([]string, 0, len(p.Families))
	for f := range p.Families {
		families = append(families, f.String())
	}
	rr.mu.RUnlock()

	slices.Sort(families)
	for _, fam := range families {
		rr.updateRoute(peerAddr, rrEORCmd(fam))
	}
	logger().Info("sent EOR", "peer", peerAddr, "families", families)
}

// signalSessionReady reports completion of this plugin's initial replay so the
// engine can release live forwards held behind it. End-of-RIB is independent.
// The local generation check rejects known stale work; the captured receipt also
// fences a reconnect after that check at the command consumer.
// initialReplay MUST come from the replay's peer-up event, never a current-session lookup.
func (rr *routeReflector) signalSessionReady(peerAddr string, gen, initialReplay uint64) {
	if initialReplay == 0 {
		logger().Warn("cannot report replay readiness without the peer-up session receipt", "peer", peerAddr)
		return
	}
	rr.mu.RLock()
	p := rr.peers[peerAddr]
	stale := p == nil || p.ReplayGen != gen
	rr.mu.RUnlock()
	if stale {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), updateRouteTimeout)
	defer cancel()
	var tb textbuf.Buffer
	command := tb.Str("request peer ").Str(peerAddr).Str(" plugin session ready session ").Uint(initialReplay).String()
	if _, _, err := rr.plugin.DispatchCommand(ctx, command); err != nil {
		logger().Warn("plugin session ready failed; this peer's live forwards remain fenced",
			"peer", peerAddr, "error", err)
	}
}

// parseReplayResponse extracts last-index and replayed count from a replay response.
func parseReplayResponse(data json.RawMessage) (lastIndex uint64, replayed int) {
	var resp struct {
		LastIndex uint64 `json:"last-index"`
		Replayed  int    `json:"replayed"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, 0
	}
	return resp.LastIndex, resp.Replayed
}

func nlriDelCmd(fam, prefixes string) string {
	var b textbuf.Buffer
	return b.Reset().Str("update text nlri ").Str(fam).Str(" del ").Str(prefixes).String()
}

func rrEORCmd(fam string) string {
	var b textbuf.Buffer
	return b.Reset().Str("update text nlri ").Str(fam).Str(" eor").String()
}

// commandDecls names the commands this plugin serves and states what each
// answer holds (pkg/plugin/rpc/types.go, CommandDecl).
//
// It has two readers and MUST stay one function. init() puts it on the
// registry.Registration, which anything linking the composition root reads
// without starting an engine, and the runner sends it in the Stage 1
// registration message, which a running daemon reads.
func commandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{Name: "show rr status", ShortHelp: "Show RR status"},
		{Name: "show rr peers", ShortHelp: "Show peer states"},
	}
}
