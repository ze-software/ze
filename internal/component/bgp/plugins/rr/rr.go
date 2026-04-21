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
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
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

// RouteReflector implements a BGP Route Reflector plugin (RFC 4456).
// Subscribes to UPDATE events and forwards them to all peers via cache-forward.
// The reactor handles RFC 4456 forwarding rules, ORIGINATOR_ID injection,
// CLUSTER_LIST prepend, and next-hop rewriting.
type RouteReflector struct {
	plugin   *sdk.Plugin
	peers    map[string]*peerState
	mu       sync.RWMutex
	stopping atomic.Bool

	// withdrawalMu protects the withdrawals map.
	withdrawalMu sync.Mutex
	// withdrawals tracks announced routes per source peer for withdrawal on peer-down.
	// sourcePeer -> routeKey (family|prefix) -> withdrawalInfo.
	withdrawals map[string]map[string]withdrawalInfo
}

// RunRouteReflector runs the Route Reflector plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func RunRouteReflector(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-rr", conn)
	defer func() { _ = p.Close() }()

	rr := &RouteReflector{
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
			switch se.EventType { //nolint:exhaustive // only update/state/open are subscribed
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
	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, string, error) {
		return rr.handleCommand(command)
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
	err := p.Run(ctx, sdk.Registration{
		CacheConsumer:          true,
		CacheConsumerUnordered: true,
		Commands: []sdk.CommandDecl{
			{Name: "rr status", Description: "Show RR status"},
			{Name: "rr peers", Description: "Show peer states"},
		},
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
func (rr *RouteReflector) forwardUpdate(msgID uint64) {
	if rr.stopping.Load() {
		return
	}
	rr.updateRoute("*", fmt.Sprintf("cache %d forward *", msgID))
}

// updateRoute sends a route update command to matching peers via the engine.
func (rr *RouteReflector) updateRoute(peerSelector, command string) {
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

// isConnectionError reports whether err indicates the target peer's connection is closed.
func isConnectionError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "use of closed network connection")
}

// handleStructuredState processes state events from DirectBridge.
func (rr *RouteReflector) handleStructuredState(se *rpc.StructuredEvent) {
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
	switch se.State { //nolint:exhaustive // only up/down are actionable for RR
	case rpc.SessionStateUp:
		peer.ReplayGen++
		gen := peer.ReplayGen
		rr.mu.Unlock()
		go rr.replayForPeer(se.PeerAddress, gen)
	case rpc.SessionStateDown:
		rr.mu.Unlock()
		rr.handleStateDown(se.PeerAddress)
	default: // other states (e.g. connected) -- no action
		rr.mu.Unlock()
	}
}

// handleStateDown sends withdrawals for all routes from the downed source peer.
// Routes are batched by family into comma-separated prefix lists to minimize RPCs.
// Withdrawals are sent asynchronously (per-lifecycle goroutine, not hot path).
//
// Concurrency: OnStructuredEvent is called serially by the engine's delivery
// goroutine, so handleStateDown and UPDATE processing for the same peer cannot
// race within the structured path. The withdrawalMu guards against the
// (theoretical) case of text and structured paths processing the same peer
// concurrently.
func (rr *RouteReflector) handleStateDown(peerAddr string) {
	rr.withdrawalMu.Lock()
	entries := rr.withdrawals[peerAddr]
	delete(rr.withdrawals, peerAddr)
	rr.withdrawalMu.Unlock()

	if len(entries) == 0 {
		return
	}

	// Group prefixes by family for batched withdrawal.
	byFamily := make(map[string][]string)
	for _, info := range entries {
		byFamily[info.Family] = append(byFamily[info.Family], info.Prefix)
	}

	// Send batched withdrawals to all peers except the one that went down.
	// Cap each RPC at withdrawalBatchSize prefixes to bound command length.
	go func() {
		for fam, prefixes := range byFamily {
			for i := 0; i < len(prefixes); i += withdrawalBatchSize {
				end := min(i+withdrawalBatchSize, len(prefixes))
				rr.updateRoute("!"+peerAddr, fmt.Sprintf("update text nlri %s del %s", fam, strings.Join(prefixes[i:end], ",")))
			}
		}
	}()
}

// handleStructuredOpen processes OPEN events from DirectBridge.
// Decodes raw OPEN wire bytes to extract peer capabilities and families.
func (rr *RouteReflector) handleStructuredOpen(se *rpc.StructuredEvent, msg *bgptypes.RawMessage) {
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
func (rr *RouteReflector) dispatchText(text string) {
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
				go rr.replayForPeer(peerAddr, gen)
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
func (rr *RouteReflector) handleCommand(command string) (string, string, error) {
	switch command {
	case "rr status":
		return statusDone, `{"running":true}`, nil
	case "rr peers":
		return statusDone, rr.peersJSON(), nil
	default: // fail on unknown command
		return statusError, "", fmt.Errorf("unknown command: %s", command)
	}
}

// peersJSON returns peer state as JSON.
func (rr *RouteReflector) peersJSON() string {
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

	data, _ := json.Marshal(map[string]any{"peers": peers})
	return string(data)
}

// replayForPeer replays existing routes to a newly-connected peer via adj-rib-in,
// then sends End-of-RIB markers per negotiated family. Runs in a per-peer
// lifecycle goroutine (not blocking the event loop).
//
// The gen parameter guards against rapid reconnects: if the peer's ReplayGen
// has changed by the time replay finishes, this goroutine is stale.
func (rr *RouteReflector) replayForPeer(peerAddr string, gen uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Full replay from adj-rib-in index 0.
	cmd := fmt.Sprintf("adj-rib-in replay %s 0", peerAddr)
	status, data, err := rr.dispatchCommand(ctx, cmd)
	if err != nil || status != statusDone {
		// Replay failure is non-fatal: the peer will still receive new routes
		// going forward. Log and return without sending EOR.
		logger().Warn("replay failed", "peer", peerAddr, "status", status, "error", err)
		return
	}

	// Parse last-index for convergent delta replay.
	lastIndex, replayed := parseReplayResponse(data)

	// Convergent delta replay: catch routes adj-rib-in received after the
	// full replay snapshot (race between event delivery and replay).
	for i := range replayConvergenceMax {
		if lastIndex == 0 {
			break
		}
		if i > 0 {
			time.Sleep(replayConvergenceDelay)
		}
		deltaCmd := fmt.Sprintf("adj-rib-in replay %s %d", peerAddr, lastIndex)
		_, deltaData, deltaErr := rr.dispatchCommand(ctx, deltaCmd)
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

	// Send EOR only after a non-empty replay. On initial session establishment
	// with empty RIB, the reactor already sends EOR for negotiated families.
	if replayed > 0 {
		rr.sendEOR(peerAddr, gen)
	}
}

// dispatchCommand sends a command to the engine via the SDK.
func (rr *RouteReflector) dispatchCommand(ctx context.Context, command string) (string, string, error) {
	return rr.plugin.DispatchCommand(ctx, command)
}

// sendEOR sends End-of-RIB markers for each of the peer's negotiated families.
func (rr *RouteReflector) sendEOR(peerAddr string, gen uint64) {
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

	sort.Strings(families)
	for _, fam := range families {
		rr.updateRoute(peerAddr, fmt.Sprintf("update text nlri %s eor", fam))
	}
	logger().Info("sent EOR", "peer", peerAddr, "families", families)
}

// parseReplayResponse extracts last-index and replayed count from a replay response.
func parseReplayResponse(data string) (lastIndex uint64, replayed int) {
	var resp struct {
		LastIndex uint64 `json:"last-index"`
		Replayed  int    `json:"replayed"`
	}
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		return 0, 0
	}
	return resp.LastIndex, resp.Replayed
}
