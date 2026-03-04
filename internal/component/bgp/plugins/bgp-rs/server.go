// Design: docs/architecture/core-design.md — route server plugin
// Detail: worker.go — per-source-peer worker pool with backpressure
// Detail: peer.go — PeerState tracking (families, up/down)
// Detail: server_text.go — text event parsing (Event, FamilyOperation, OpenInfo types)
// Detail: server_withdrawal.go — withdrawal map management and NLRI walking
// Detail: server_forward.go — forward target selection and batch accumulation
// Detail: server_handlers.go — peer event handlers (state, open, refresh, command)

package bgp_rs

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/textparse"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// statusDone is the command response status for successful operations.
const statusDone = "done"

// updateRouteTimeout is the context deadline for updateRoute RPC calls.
// Set to 60s (was 10s) as defense-in-depth against transient congestion
// when many concurrent workers send update-route RPCs.
const updateRouteTimeout = 60 * time.Second

// replayConvergenceMax is the maximum number of delta replay iterations.
// Each iteration catches routes that adj-rib-in processed since the previous
// iteration. Convergence is typically reached in 1-2 iterations.
const replayConvergenceMax = 10

// replayConvergenceDelay is the pause between delta replay iterations.
// Gives adj-rib-in's event handler time to process pending deliveries
// from the engine's DirectBridge (concurrent with replay commands).
const replayConvergenceDelay = 20 * time.Millisecond

// Event type constants matching ze-bgp message.type values.
//
// Note: The engine also sends "borr" (Beginning of Route Refresh, RFC 7313 subtype 1)
// and "eorr" (End of Route Refresh, RFC 7313 subtype 2) as message.type values.
// These are intentionally not handled — a forward-all route server does not need
// to track refresh cycle boundaries. Only standard refresh is forwarded.
const (
	eventUpdate  = "update"
	eventState   = "state"
	eventOpen    = "open"
	eventRefresh = "refresh"

	// Text format field tokens used in text event parsing.
	tokenASN      = "asn"
	tokenCap      = "cap"
	tokenRouterID = "router-id"
	tokenHoldTime = "hold-time"
	tokenFamily   = "family"

	// NLRI action tokens from text UPDATE parsing — shared with textparse.
	actionAdd = textparse.KWAdd
	actionDel = textparse.KWDel
)

// loggerPtr is the package-level logger, disabled by default.
// Stored as atomic.Pointer to avoid data races when tests start
// multiple in-process plugin instances concurrently.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// SetLogger configures the package-level logger for the RR plugin.
func SetLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// forwardCtx holds the source peer and event data for a cached UPDATE.
// sourcePeer is extracted at dispatch time (cheap) to avoid re-parsing in the worker
// and to make source-exclusion data flow explicit.
// For DirectBridge delivery: msg is set (raw wire data, no text parsing needed).
// For fork-mode delivery: textPayload is set (deferred text parsing by worker).
// Stored in RouteServer.fwdCtx (sync.Map) keyed by msgID (uint64).
type forwardCtx struct {
	sourcePeer  string
	textPayload string
	msg         *bgptypes.RawMessage
}

// withdrawalInfo stores the minimum information needed to send withdrawal
// commands when a source peer goes down. Only family+prefix are needed
// for "update text nlri <family> del <prefix>" commands.
type withdrawalInfo struct {
	Family string
	Prefix string // Full NLRI string including type keyword (e.g., "prefix 10.0.0.0/24").
}

// RouteServer implements a BGP Route Server API plugin.
// It forwards all UPDATEs to all peers except the source (forward-all model).
// UPDATEs are dispatched to per-source-peer workers for parallel processing
// while preserving FIFO ordering within each source peer.
type RouteServer struct {
	plugin  *sdk.Plugin
	peers   map[string]*PeerState
	mu      sync.RWMutex
	workers *workerPool

	// pausedPeers tracks source peers for which we have sent a pause RPC.
	// Protected by mu. Nil until wireFlowControl is called.
	pausedPeers map[string]bool

	// fwdCtx stores forwarding context (forwardCtx) keyed by msgID (uint64).
	// Written by dispatch (OnEvent goroutine), read by worker handler.
	fwdCtx sync.Map

	// releaseCh is a buffered channel for async cache release.
	// Workers send msgIDs here instead of blocking on synchronous RPCs.
	// A background releaseLoop goroutine drains the channel.
	releaseCh   chan uint64
	releaseDone chan struct{} // closed when releaseLoop exits

	// batches holds per-worker batch state for accumulating forward RPCs.
	// Each worker goroutine has its own batch (keyed by workerKey).
	// No concurrent access per key — each worker is single-goroutine.
	batches sync.Map // workerKey → *forwardBatch

	// forwardCh is a buffered channel for fire-and-forget cache-forward RPCs.
	// flushBatch sends commands here instead of calling updateRoute directly.
	// A background forwardLoop goroutine drains the channel and calls updateRoute.
	forwardCh   chan forwardCmd
	forwardDone chan struct{} // closed when forwardLoop exits

	// withdrawalMu protects the withdrawals map.
	withdrawalMu sync.Mutex
	// withdrawals tracks announced routes per source peer for withdrawal on peer-down.
	// Populated by processForward from NLRI parsing. Cleared by handleStateDown.
	// sourcePeer → routeKey (family|prefix) → withdrawalInfo.
	withdrawals map[string]map[string]withdrawalInfo

	// updateRouteHook is called before each updateRoute RPC for test inspection.
	// Nil in production (zero overhead).
	updateRouteHook func(peer, cmd string)

	// dispatchCommandHook is called instead of SDK DispatchCommand for test inspection.
	// Nil in production (zero overhead).
	dispatchCommandHook func(command string) (string, string, error)

	// stopping is set when the plugin is shutting down.
	// Used to downgrade RPC errors from ERROR to DEBUG during teardown.
	stopping atomic.Bool
}

// RunRouteServer runs the Route Server plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func RunRouteServer(engineConn, callbackConn net.Conn) int {
	p := sdk.NewWithConn("bgp-rs", engineConn, callbackConn)
	defer func() { _ = p.Close() }()

	rs := &RouteServer{
		plugin:      p,
		peers:       make(map[string]*PeerState),
		withdrawals: make(map[string]map[string]withdrawalInfo),
	}

	// ZE_RS_CHAN_SIZE overrides the per-source-peer worker channel capacity.
	// Default: 4096. Invalid/zero/negative values use default (guard in newWorkerPool).
	rrChanSize := 4096
	if v := os.Getenv("ZE_RS_CHAN_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			rrChanSize = n
		} else if err != nil {
			logger().Warn("ignoring invalid ZE_RS_CHAN_SIZE", "value", v, "error", err)
		}
	}

	// Start async cache release goroutine before worker pool (workers call releaseCache).
	rs.startReleaseLoop()
	defer rs.stopReleaseLoop()

	// Start fire-and-forget forward sender before worker pool (workers call asyncForward).
	rs.startForwardLoop()
	defer rs.stopForwardLoop()

	// Create worker pool for parallel UPDATE forwarding.
	// Each source peer gets its own worker goroutine (lazy creation, idle cooldown).
	// FIFO ordering is preserved per source peer.
	rs.workers = newWorkerPool(func(key workerKey, item workItem) {
		rs.processForward(key, item.msgID)
	}, poolConfig{
		chanSize:    rrChanSize,
		idleTimeout: 5 * time.Second,
		onItemDrop: func(item workItem) {
			rs.fwdCtx.Delete(item.msgID)
			rs.releaseCache(item.msgID)
		},
		onDrained: rs.flushWorkerBatch,
	})
	defer rs.workers.Stop()

	rs.wireFlowControl()
	defer rs.resumeAllPaused()

	// Register structured event handler for DirectBridge delivery.
	// When active, UPDATE events arrive as *rpc.StructuredUpdate — no text parsing needed.
	p.OnStructuredEvent(func(events []any) error {
		for _, event := range events {
			if su, ok := event.(*rpc.StructuredUpdate); ok {
				if msg, ok := su.Event.(*bgptypes.RawMessage); ok {
					rs.dispatchStructured(su.PeerAddress, msg)
				}
			}
		}
		return nil
	})

	// Register text event handler for fork-mode delivery (fallback).
	// Only a lightweight parse runs here (type + msgID + peerAddr); expensive
	// parsing and withdrawal map updates are deferred to per-source-peer workers.
	p.OnEvent(func(eventStr string) error {
		rs.dispatchText(eventStr)
		return nil
	})

	// Register command handler: responds to "rs status" and "rs peers"
	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, string, error) {
		return rs.handleCommand(command)
	})

	// Register event subscriptions atomically with startup completion.
	// Included in the "ready" RPC so the engine registers them before SignalAPIReady,
	// ensuring the rr sees every event from the very first route.
	//
	// Subscribe to received-direction only for UPDATE events. Subscribing to
	// "both" (the default) creates a circular deadlock: ForwardUpdate sends
	// UPDATEs to peers → onMessageSent fires → tries to deliver sent-event
	// to RR on Socket B → blocks on callMu → Socket A handler blocks →
	// forward worker blocks → workCh fills → OnEvent blocks → deadlock.
	//
	// The "refresh" subscription also delivers "borr" and "eorr" events (RFC 7313)
	// because the engine maps all TypeROUTEREFRESH wire messages to the "refresh"
	// subscription type. These subtypes are silently ignored by dispatchText().
	//
	// OPEN subscription uses "direction received" to capture the remote peer's
	// capabilities only. Without direction filtering, the engine also delivers
	// the locally-sent OPEN which may contain different families — handleOpen
	// overwrites Families each time, so the last OPEN wins. If the sent OPEN
	// arrives last, the peer's families are wrong and selectForwardTargets
	// excludes it from forwarding.
	p.SetStartupSubscriptions([]string{
		eventUpdate + " direction received",
		eventState,
		eventOpen + " direction received",
		eventRefresh,
	}, nil, "")

	// Request text encoding — strings.Fields parsing instead of json.Unmarshal.
	p.SetEncoding("text")

	ctx := context.Background()
	defer rs.stopping.Store(true)
	err := p.Run(ctx, sdk.Registration{
		// CacheConsumer: true is required for bgp-cache-forward to work.
		// Without it, Activate(id, 0) evicts cache entries immediately after
		// event delivery, causing the forward worker to find the entry
		// already gone (ErrUpdateExpired).
		CacheConsumer: true,
		// CacheConsumerUnordered: true switches from cumulative (TCP-like) ack
		// to per-entry ack. bgp-rs uses per-peer workers that process entries
		// out of global FIFO order — without this, acking a high ID from the
		// heavy-peer worker would cumulatively evict intermediate entries that
		// small-peer workers haven't processed yet, causing ErrUpdateExpired.
		CacheConsumerUnordered: true,
		Commands: []sdk.CommandDecl{
			{Name: "rs status", Description: "Show RS status"},
			{Name: "rs peers", Description: "Show peer states"},
		},
	})

	if err != nil {
		logger().Error("rr plugin failed", "error", err)
		return 1
	}

	return 0
}

// releaseCache enqueues an async cache release for a non-forwarded UPDATE.
// Must be called when CacheConsumer: true and we decide not to forward,
// otherwise the entry blocks eviction of subsequent cache entries.
// Non-blocking: sends msgID to the releaseLoop goroutine via buffered channel.
// Falls back to synchronous RPC if the channel is full.
func (rs *RouteServer) releaseCache(msgID uint64) {
	if msgID == 0 {
		return
	}
	select {
	case rs.releaseCh <- msgID:
	default: // Channel full — release synchronously to avoid dropping.
		logger().Warn("release channel full, falling back to sync", "msgID", msgID)
		rs.updateRoute("*", fmt.Sprintf("bgp cache %d release", msgID))
	}
}

// startReleaseLoop starts the background goroutine for async cache release.
func (rs *RouteServer) startReleaseLoop() {
	rs.releaseCh = make(chan uint64, 256)
	rs.releaseDone = make(chan struct{})
	go func() {
		defer close(rs.releaseDone)
		for msgID := range rs.releaseCh {
			rs.updateRoute("*", fmt.Sprintf("bgp cache %d release", msgID))
		}
	}()
}

// stopReleaseLoop closes the release channel and waits for the goroutine to exit.
// Must be called after workers.Stop() to avoid sending on a closed channel.
func (rs *RouteServer) stopReleaseLoop() {
	close(rs.releaseCh)
	<-rs.releaseDone
}

// startForwardLoop starts the background goroutine for fire-and-forget cache-forward RPCs.
// Capacity 16 batches — each batch is up to 50 IDs, so ~800 updates buffered.
// If the channel fills, workers block (natural backpressure from engine).
func (rs *RouteServer) startForwardLoop() {
	rs.forwardCh = make(chan forwardCmd, 16)
	rs.forwardDone = make(chan struct{})
	go func() {
		defer close(rs.forwardDone)
		for cmd := range rs.forwardCh {
			rs.updateRoute(cmd.peer, cmd.cmd)
		}
	}()
}

// stopForwardLoop closes the forward channel and waits for the goroutine to exit.
// Must be called after workers.Stop() to avoid sending on a closed channel.
func (rs *RouteServer) stopForwardLoop() {
	close(rs.forwardCh)
	<-rs.forwardDone
}

// asyncForward enqueues a forward RPC for the background sender goroutine.
// Blocks if the channel is full (natural backpressure when engine is slow).
func (rs *RouteServer) asyncForward(peer, cmd string) {
	rs.forwardCh <- forwardCmd{peer: peer, cmd: cmd}
}

// dispatchCommand calls DispatchCommand via the SDK or the test hook.
func (rs *RouteServer) dispatchCommand(ctx context.Context, command string) (string, string, error) {
	if rs.dispatchCommandHook != nil {
		return rs.dispatchCommandHook(command)
	}
	return rs.plugin.DispatchCommand(ctx, command)
}

// updateRoute sends a route update command to matching peers via the engine.
func (rs *RouteServer) updateRoute(peerSelector, command string) {
	if rs.updateRouteHook != nil {
		rs.updateRouteHook(peerSelector, command)
	}
	ctx, cancel := context.WithTimeout(context.Background(), updateRouteTimeout)
	defer cancel()
	_, _, err := rs.plugin.UpdateRoute(ctx, peerSelector, command)
	if err != nil {
		if rs.stopping.Load() {
			logger().Debug("update-route failed (shutting down)", "peer", peerSelector, "command", command, "error", err)
		} else {
			logger().Error("update-route failed", "peer", peerSelector, "command", command, "error", err)
		}
	}
}

// wireFlowControl connects the worker pool's backpressure signals to
// pause/resume RPCs. Called once after worker pool creation.
//
// High-water (channel full): dispatch() checks BackpressureDetected() after each
// Dispatch and sends "bgp peer pause <addr>" to the engine.
// Low-water (<10%): the onLowWater callback fires in the worker goroutine
// and sends "bgp peer resume <addr>" to the engine.
func (rs *RouteServer) wireFlowControl() {
	rs.pausedPeers = make(map[string]bool)

	rs.workers.onLowWater = func(key workerKey) {
		rs.mu.Lock()
		wasPaused := rs.pausedPeers[key.sourcePeer]
		if wasPaused {
			delete(rs.pausedPeers, key.sourcePeer)
		}
		rs.mu.Unlock()

		if wasPaused {
			logger().Info("resuming peer", "source-peer", key.sourcePeer)
			rs.updateRoute("*", "bgp peer "+key.sourcePeer+" resume")
		}
	}
}

// resumeAllPaused sends resume RPCs for all currently paused peers.
// Called during shutdown to ensure no peers remain paused after the RR exits.
func (rs *RouteServer) resumeAllPaused() {
	rs.mu.Lock()
	paused := make([]string, 0, len(rs.pausedPeers))
	for addr := range rs.pausedPeers {
		paused = append(paused, addr)
	}
	rs.pausedPeers = make(map[string]bool)
	rs.mu.Unlock()

	for _, addr := range paused {
		logger().Info("shutdown: resuming peer", "source-peer", addr)
		rs.updateRoute("*", "bgp peer "+addr+" resume")
	}
}

// dispatchText routes a text-format event to the appropriate handler.
//
// UPDATE events are dispatched to the per-source-peer worker pool with
// only a lightweight parse (type, msgID, peerAddr via strings.Fields).
// The full text is stored in forwardCtx for deferred family+NLRI parsing
// by the worker goroutine, keeping OnEvent latency low.
//
// Non-UPDATE events (state, open, refresh) are handled inline since
// they are infrequent and need immediate state changes.
//
// Events with unrecognized types are silently ignored. This includes "borr" and "eorr"
// (RFC 7313 enhanced route refresh markers) which the engine delivers under the "refresh"
// subscription but encodes with distinct message.type values.
func (rs *RouteServer) dispatchText(text string) {
	// Guard against delivery after shutdown: the engine's deliveryLoop goroutine
	// can call bridge callbacks concurrently with bgp-rs cleanup (workers.Stop()).
	if rs.stopping.Load() {
		return
	}

	eventType, msgID, peerAddr, payload, err := quickParseTextEvent(text)
	if err != nil {
		logger().Warn("parse error", "error", err, "line", text[:min(100, len(text))])
		return
	}

	if eventType == eventUpdate {
		if peerAddr == "" {
			return
		}
		rs.fwdCtx.Store(msgID, &forwardCtx{sourcePeer: peerAddr, textPayload: payload})
		key := workerKey{sourcePeer: peerAddr}
		if !rs.workers.Dispatch(key, workItem{msgID: msgID}) {
			logger().Error("dispatch dropped (pool stopped)", "msgID", msgID, "source-peer", peerAddr)
			rs.fwdCtx.Delete(msgID)
			rs.releaseCache(msgID)
			return
		}

		// Flow control: pause source peer if worker channel crossed high-water mark.
		// BackpressureDetected is clear-on-read, so each transition fires once.
		// Guard on pausedPeers != nil: flow control is only active after wireFlowControl().
		if rs.workers.BackpressureDetected(key) {
			rs.mu.Lock()
			if rs.pausedPeers != nil && !rs.pausedPeers[peerAddr] {
				rs.pausedPeers[peerAddr] = true
				rs.mu.Unlock()
				logger().Debug("pausing peer", "source-peer", peerAddr)
				rs.updateRoute("*", "bgp peer "+peerAddr+" pause")
			} else {
				rs.mu.Unlock()
			}
		}
		return
	}

	// Non-UPDATE: full text parse + handle inline.
	switch eventType {
	case eventState:
		if event := parseTextState(payload); event != nil {
			rs.handleState(event)
		}
	case eventRefresh:
		if event := parseTextRefresh(payload); event != nil {
			rs.handleRefresh(event)
		}
	case eventOpen:
		if event := parseTextOpen(payload); event != nil {
			rs.handleOpen(event)
		}
	}
}

// dispatchStructured routes a structured UPDATE event from DirectBridge delivery.
// Only UPDATE events arrive via this path — non-UPDATE events (state, open, refresh)
// are still delivered as text via OnEvent/dispatchText.
func (rs *RouteServer) dispatchStructured(peerAddr string, msg *bgptypes.RawMessage) {
	// Guard against delivery after shutdown: the engine's deliveryLoop goroutine
	// can call bridge callbacks concurrently with bgp-rs cleanup (workers.Stop()).
	if rs.stopping.Load() {
		return
	}

	msgID := msg.MessageID

	if peerAddr == "" {
		return
	}
	rs.fwdCtx.Store(msgID, &forwardCtx{sourcePeer: peerAddr, msg: msg})
	key := workerKey{sourcePeer: peerAddr}
	if !rs.workers.Dispatch(key, workItem{msgID: msgID}) {
		logger().Error("dispatch dropped (pool stopped)", "msgID", msgID, "source-peer", peerAddr)
		rs.fwdCtx.Delete(msgID)
		rs.releaseCache(msgID)
		return
	}

	// Flow control: pause source peer if worker channel crossed high-water mark.
	if rs.workers.BackpressureDetected(key) {
		rs.mu.Lock()
		if rs.pausedPeers != nil && !rs.pausedPeers[peerAddr] {
			rs.pausedPeers[peerAddr] = true
			rs.mu.Unlock()
			logger().Debug("pausing peer", "source-peer", peerAddr)
			rs.updateRoute("*", "bgp peer "+peerAddr+" pause")
		} else {
			rs.mu.Unlock()
		}
	}
}

// maxBatchSize is the maximum number of IDs accumulated before a batch flush.
const maxBatchSize = 50

// --- Event types ---

// --- Text event parsing ---
//
// Text format replaces JSON for the bgp-rs hot path.
// Uniform header: "peer <addr> asn <n> <dir> <type> <id> ..."
// State:          "peer <addr> asn <n> state <state>"
// Parsed with TextScanner (zero-copy token extraction from original string).

// topLevelKeywords and nlriTypeKeywords are now shared from textparse package.
// Use textparse.IsTopLevelKeyword() and textparse.NLRITypeKeywords.
