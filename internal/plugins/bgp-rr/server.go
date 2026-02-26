// Design: docs/architecture/core-design.md — route reflector plugin
// Related: worker.go — per-source-peer worker pool with backpressure
// Related: peer.go — PeerState tracking (families, up/down)

package bgp_rr

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// statusDone is the command response status for successful operations.
const statusDone = "done"

// updateRouteTimeout is the context deadline for updateRoute RPC calls.
// Set to 60s (was 10s) as defense-in-depth against transient congestion
// when many concurrent workers send update-route RPCs.
const updateRouteTimeout = 60 * time.Second

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

	// NLRI action tokens from text UPDATE parsing.
	actionAdd = "add"
	actionDel = "del"
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

// forwardCtx holds the source peer and raw text event for a cached UPDATE.
// sourcePeer is extracted by quickParseTextEvent in dispatchText (cheap) to avoid
// re-parsing the event in the worker, and to make the source-exclusion
// data flow explicit. textPayload is the raw text event line(s) for deferred
// family+NLRI parsing by the worker via parseTextUpdateFamilies/parseTextNLRIOps.
// Stored in RouteServer.fwdCtx (sync.Map) keyed by msgID (uint64).
type forwardCtx struct {
	sourcePeer  string
	textPayload string
}

// withdrawalInfo stores the minimum information needed to send withdrawal
// commands when a source peer goes down. Only family+prefix are needed
// for "update text nlri <family> del <prefix>" commands.
type withdrawalInfo struct {
	Family string
	Prefix string
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
}

// forwardBatch accumulates forward items for batch RPC.
// Per-worker state: no concurrent access for a given workerKey.
type forwardBatch struct {
	ids      []uint64
	selector string // comma-joined target peers
}

// forwardCmd is a single fire-and-forget forward RPC to be sent by the background sender.
type forwardCmd struct {
	peer string
	cmd  string
}

// RunRouteServer runs the Route Server plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func RunRouteServer(engineConn, callbackConn net.Conn) int {
	p := sdk.NewWithConn("bgp-rr", engineConn, callbackConn)
	defer func() { _ = p.Close() }()

	rs := &RouteServer{
		plugin:      p,
		peers:       make(map[string]*PeerState),
		withdrawals: make(map[string]map[string]withdrawalInfo),
	}

	// ZE_RR_CHAN_SIZE overrides the per-source-peer worker channel capacity.
	// Default: 4096. Invalid/zero/negative values use default (guard in newWorkerPool).
	rrChanSize := 4096
	if v := os.Getenv("ZE_RR_CHAN_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			rrChanSize = n
		} else if err != nil {
			logger().Warn("ignoring invalid ZE_RR_CHAN_SIZE", "value", v, "error", err)
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

	// Register event handler: dispatches BGP events (update, state, open, refresh).
	// Only a lightweight parse runs here (type + msgID + peerAddr); expensive
	// parsing and withdrawal map updates are deferred to per-source-peer workers.
	p.OnEvent(func(eventStr string) error {
		rs.dispatchText(eventStr)
		return nil
	})

	// Register command handler: responds to "rr status" and "rr peers"
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
	err := p.Run(ctx, sdk.Registration{
		// CacheConsumer: true is required for bgp-cache-forward to work.
		// Without it, Activate(id, 0) evicts cache entries immediately after
		// event delivery, causing the forward worker to find the entry
		// already gone (ErrUpdateExpired).
		CacheConsumer: true,
		// CacheConsumerUnordered: true switches from cumulative (TCP-like) ack
		// to per-entry ack. bgp-rr uses per-peer workers that process entries
		// out of global FIFO order — without this, acking a high ID from the
		// heavy-peer worker would cumulatively evict intermediate entries that
		// small-peer workers haven't processed yet, causing ErrUpdateExpired.
		CacheConsumerUnordered: true,
		Commands: []sdk.CommandDecl{
			{Name: "rr status", Description: "Show RS status"},
			{Name: "rr peers", Description: "Show peer states"},
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
		logger().Error("update-route failed", "peer", peerSelector, "error", err)
	}
}

// wireFlowControl connects the worker pool's backpressure signals to
// pause/resume RPCs. Called once after worker pool creation.
//
// High-water (>90%): dispatch() checks BackpressureDetected() after each
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

// processForward handles a forwarding work item in a worker goroutine.
// Loads pre-parsed payload from fwdCtx, performs full parse, forwards
// the UPDATE to compatible peers, then updates the withdrawal map.
// Forward-first ordering minimizes UPDATE delivery latency — the withdrawal
// map is only needed for withdrawal tracking on peer-down, not for forwarding.
func (rs *RouteServer) processForward(key workerKey, msgID uint64) {
	val, ok := rs.fwdCtx.LoadAndDelete(msgID)
	if !ok {
		return
	}
	ctx, ok := val.(*forwardCtx)
	if !ok {
		rs.releaseCache(msgID)
		return
	}

	// Guard: release cache entry on any early return or panic.
	// forwardUpdate handles the entry when reached (forward or release),
	// so the flag prevents double-release on the normal path.
	forwarded := false
	defer func() {
		if !forwarded {
			rs.releaseCache(msgID)
		}
	}()

	// If the source peer is down, skip withdrawal map update and forward — handleStateDown
	// will withdraw all routes. This prevents PeerDown from blocking while
	// workers process queued UPDATEs for a peer that is already gone.
	rs.mu.RLock()
	peer := rs.peers[ctx.sourcePeer]
	peerDown := peer == nil || !peer.Up
	rs.mu.RUnlock()
	if peerDown {
		return
	}

	// Extract family names from text event for forward target selection.
	families := parseTextUpdateFamilies(ctx.textPayload)
	if len(families) == 0 {
		return
	}

	// Accumulate forward in per-worker batch (flushed on batch-full or channel drain).
	forwarded = true
	rs.batchForwardUpdate(key, ctx.sourcePeer, msgID, families)

	// Update withdrawal map: track announced routes for withdrawal on peer-down.
	// Only family+prefix needed — bgp-adj-rib-in owns full route state.
	rs.withdrawalMu.Lock()
	for family, ops := range parseTextNLRIOps(ctx.textPayload) {
		for _, op := range ops {
			switch op.Action {
			case actionAdd:
				if rs.withdrawals[ctx.sourcePeer] == nil {
					rs.withdrawals[ctx.sourcePeer] = make(map[string]withdrawalInfo)
				}
				for _, n := range op.NLRIs {
					if prefix, ok := n.(string); ok && prefix != "" {
						routeKey := family + "|" + prefix
						rs.withdrawals[ctx.sourcePeer][routeKey] = withdrawalInfo{
							Family: family,
							Prefix: prefix,
						}
					}
				}
			case actionDel:
				if rs.withdrawals[ctx.sourcePeer] != nil {
					for _, n := range op.NLRIs {
						if prefix, ok := n.(string); ok && prefix != "" {
							routeKey := family + "|" + prefix
							delete(rs.withdrawals[ctx.sourcePeer], routeKey)
						}
					}
				}
			}
		}
	}
	rs.withdrawalMu.Unlock()
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
				logger().Warn("pausing peer", "source-peer", peerAddr)
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

// selectForwardTargets returns peers that should receive an UPDATE with the given families.
// A peer is included if it is up, is not the source, and supports at least one family
// in the UPDATE (or has nil Families, meaning unknown/all-accepted).
func (rs *RouteServer) selectForwardTargets(sourcePeer string, families map[string]bool) []string {
	var targets []string
	for addr, peer := range rs.peers {
		if addr == sourcePeer || !peer.Up || peer.Replaying {
			continue
		}
		if peer.Families != nil {
			hasAny := false
			for family := range families {
				if peer.SupportsFamily(family) {
					hasAny = true
					break
				}
			}
			if !hasAny {
				continue
			}
		}
		targets = append(targets, addr)
	}
	sort.Strings(targets)
	return targets
}

// batchForwardUpdate accumulates a forward item into the per-worker batch.
// Selects targets, then appends to the current batch. Flushes the old batch
// if the target selector changes (different peer set). Flushes when the batch
// reaches maxBatchSize items. Partial batches are flushed by the onDrained
// callback when the worker channel empties.
func (rs *RouteServer) batchForwardUpdate(key workerKey, sourcePeer string, msgID uint64, families map[string]bool) {
	rs.mu.RLock()
	targets := rs.selectForwardTargets(sourcePeer, families)
	rs.mu.RUnlock()

	if len(targets) == 0 {
		rs.releaseCache(msgID)
		return
	}

	sel := strings.Join(targets, ",")

	val, _ := rs.batches.LoadOrStore(key, &forwardBatch{})
	batch, ok := val.(*forwardBatch)
	if !ok {
		rs.releaseCache(msgID)
		return
	}

	// Selector changed — flush old batch, start fresh.
	if batch.selector != "" && batch.selector != sel {
		rs.flushBatch(batch)
		batch.ids = batch.ids[:0]
		batch.selector = ""
	}

	batch.ids = append(batch.ids, msgID)
	batch.selector = sel

	// Flush on batch full.
	if len(batch.ids) >= maxBatchSize {
		rs.flushBatch(batch)
		batch.ids = batch.ids[:0]
		batch.selector = ""
	}
}

// maxBatchSize is the maximum number of IDs accumulated before a batch flush.
const maxBatchSize = 50

// flushBatch sends a single batched cache-forward RPC for all accumulated IDs.
// Uses asyncForward (fire-and-forget) so the worker goroutine doesn't block
// waiting for the engine's RPC response.
func (rs *RouteServer) flushBatch(batch *forwardBatch) {
	if len(batch.ids) == 0 {
		return
	}

	// Single ID — use existing format (no comma).
	if len(batch.ids) == 1 {
		rs.asyncForward("*", fmt.Sprintf("bgp cache %d forward %s", batch.ids[0], batch.selector))
		return
	}

	// Multiple IDs — comma-separated batch format.
	idStrs := make([]string, len(batch.ids))
	for i, id := range batch.ids {
		idStrs[i] = strconv.FormatUint(id, 10)
	}
	rs.asyncForward("*", fmt.Sprintf("bgp cache %s forward %s", strings.Join(idStrs, ","), batch.selector))
}

// flushWorkerBatch flushes the batch for a given worker key.
// Called by the onDrained callback when the worker's channel empties.
func (rs *RouteServer) flushWorkerBatch(key workerKey) {
	val, loaded := rs.batches.Load(key)
	if !loaded {
		return
	}
	batch, ok := val.(*forwardBatch)
	if !ok {
		return
	}
	rs.flushBatch(batch)
	batch.ids = batch.ids[:0]
	batch.selector = ""
}

// handleState processes peer state changes.
// ze-bgp JSON: {"type":"bgp","bgp":{"message":{"type":"state"},"peer":{...},"state":"up"}}.
func (rs *RouteServer) handleState(event *Event) {
	peerAddr := event.PeerAddr
	state := event.State

	if peerAddr == "" {
		return
	}

	rs.mu.Lock()
	if rs.peers[peerAddr] == nil {
		rs.peers[peerAddr] = &PeerState{Address: peerAddr}
	}
	rs.peers[peerAddr].Up = (state == "up")
	rs.mu.Unlock()

	switch state {
	case "down":
		rs.handleStateDown(peerAddr)
	case "up":
		rs.handleStateUp(peerAddr)
	}
}

// handleStateDown processes peer session teardown.
// Sends withdrawals asynchronously — per-lifecycle goroutine (not hot path).
func (rs *RouteServer) handleStateDown(peerAddr string) {
	// Drain workers first: in-flight forwards may update the withdrawal map.
	// PeerDown waits for all workers to finish, so after this call no more
	// updates for this peer can occur.
	rs.workers.PeerDown(peerAddr)

	// Extract and clear withdrawal entries for this peer.
	rs.withdrawalMu.Lock()
	entries := rs.withdrawals[peerAddr]
	delete(rs.withdrawals, peerAddr)
	rs.withdrawalMu.Unlock()

	go func() {
		for _, info := range entries {
			rs.updateRoute("!"+peerAddr, fmt.Sprintf("update text nlri %s del %s", info.Family, info.Prefix))
		}
	}()
}

// handleStateUp processes peer session establishment.
//
// Replays existing routes to the newly-connected peer via DispatchCommand
// to bgp-adj-rib-in, replacing the previous ROUTE-REFRESH approach which
// caused a thundering herd (N peers × M families). The replay runs in a
// per-peer lifecycle goroutine (not blocking the event loop). The peer is
// marked as "replaying" and excluded from selectForwardTargets until the
// full replay completes, preventing ghost routes from interleaved forwarding.
// A delta replay then covers routes that arrived during the full replay.
func (rs *RouteServer) handleStateUp(peerAddr string) {
	// Mark peer as replaying — excluded from selectForwardTargets until complete.
	// Increment generation so stale goroutines from a previous session (rapid
	// reconnect) don't prematurely clear Replaying for the new session.
	rs.mu.Lock()
	var gen uint64
	if rs.peers[peerAddr] != nil {
		rs.peers[peerAddr].Replaying = true
		rs.peers[peerAddr].ReplayGen++
		gen = rs.peers[peerAddr].ReplayGen
	}
	rs.mu.Unlock()

	// Spawn per-peer lifecycle goroutine for replay (not blocking event loop).
	go rs.replayForPeer(peerAddr, gen)
}

// replayForPeer runs the full+delta replay sequence for a newly-connected peer.
// Runs in a per-peer lifecycle goroutine — not blocking the event loop.
// The gen parameter is the replay generation at the time handleStateUp was called.
// If the peer's ReplayGen has changed (rapid reconnect), this goroutine is stale
// and must not clear Replaying — the newer goroutine owns that transition.
func (rs *RouteServer) replayForPeer(peerAddr string, gen uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Full replay from index 0.
	// Retry on transient failure: adj-rib-in may not be Running yet during the
	// startup race (bgp-rr's subscriptions activate before adj-rib-in completes
	// stage 5). adj-rib-in is guaranteed present via dependency auto-loading,
	// so "unknown command" errors are no longer expected.
	cmd := fmt.Sprintf("adj-rib-in replay %s 0", peerAddr)
	var status, data string
	var err error
	for attempt := range 5 {
		status, data, err = rs.dispatchCommand(ctx, cmd)
		if err == nil && status == statusDone {
			break
		}
		if ctx.Err() != nil {
			break
		}
		if attempt < 4 {
			logger().Debug("replay retry", "peer", peerAddr, "attempt", attempt+1, "error", err)
			time.Sleep(100 * time.Millisecond)
		}
	}
	if err != nil || status != statusDone {
		logger().Error("replay failed", "peer", peerAddr, "status", status, "error", err)
		// Still add to forward targets so peer gets new routes going forward.
		// Only if this goroutine's generation is still current.
		rs.mu.Lock()
		if p := rs.peers[peerAddr]; p != nil && p.ReplayGen == gen {
			p.Replaying = false
		}
		rs.mu.Unlock()
		return
	}

	// Parse last-index from replay response.
	lastIndex := parseLastIndex(data)

	// Add peer to forward targets (new UPDATEs now flow to this peer).
	// Only if this goroutine's generation is still current.
	rs.mu.Lock()
	stale := rs.peers[peerAddr] == nil || rs.peers[peerAddr].ReplayGen != gen
	if !stale {
		rs.peers[peerAddr].Replaying = false
	}
	rs.mu.Unlock()

	if stale {
		return
	}

	// Delta replay to cover routes that arrived during full replay.
	if lastIndex > 0 {
		_, _, err := rs.dispatchCommand(ctx, fmt.Sprintf("adj-rib-in replay %s %d", peerAddr, lastIndex))
		if err != nil {
			logger().Warn("delta replay failed", "peer", peerAddr, "error", err)
		}
	}
}

// parseLastIndex extracts the last-index value from a replay response JSON.
// Expected format: {"last-index":N,"replayed":M}.
func parseLastIndex(data string) uint64 {
	var resp struct {
		LastIndex uint64 `json:"last-index"`
	}
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		return 0
	}
	return resp.LastIndex
}

// handleOpen processes OPEN events to capture peer capabilities.
// Text format capabilities: "cap <code> <name> [<value>]" tokens parsed by parseTextOpen.
func (rs *RouteServer) handleOpen(event *Event) {
	peerAddr := event.PeerAddr
	if peerAddr == "" {
		return
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.peers[peerAddr] == nil {
		rs.peers[peerAddr] = &PeerState{Address: peerAddr}
	}
	peer := rs.peers[peerAddr]

	peer.ASN = event.PeerASN

	if event.Open != nil {
		peer.Capabilities = make(map[string]bool)
		peer.Families = make(map[string]bool)

		// RFC 4760 Section 1: ipv4/unicast is the implicit default only when
		// the peer sends no Multiprotocol capability. If the peer advertises
		// MP but omits ipv4/unicast, it explicitly declines it.
		hasMP := capabilityPresent(event.Open.Capabilities, "multiprotocol")

		for _, cap := range event.Open.Capabilities {
			peer.Capabilities[cap.Name] = true

			if cap.Name == "multiprotocol" && cap.Value != "" {
				peer.Families[cap.Value] = true
			}
		}

		if !hasMP {
			peer.Families["ipv4/unicast"] = true
		}
	}
}

// handleRefresh processes route refresh requests.
// Text format: "peer <addr> <dir> refresh <id> family <afi/safi>" parsed by parseTextRefresh.
//
// Collects eligible peers under the lock, then sends refresh commands after
// releasing — updateRoute does an SDK RPC with a 10 s timeout, so holding
// the lock during network I/O would block all state updates.
func (rs *RouteServer) handleRefresh(event *Event) {
	peerAddr := event.PeerAddr
	family := event.AFI + "/" + event.SAFI

	if peerAddr == "" {
		return
	}

	rs.mu.RLock()
	var targets []string
	for addr, peer := range rs.peers {
		if addr == peerAddr {
			continue
		}
		if !peer.Up {
			continue
		}
		if !peer.HasCapability("route-refresh") {
			continue
		}
		if peer.Families != nil && !peer.SupportsFamily(family) {
			continue
		}
		targets = append(targets, addr)
	}
	rs.mu.RUnlock()

	// Send refreshes asynchronously — per-lifecycle goroutine (not hot path).
	go func() {
		for _, addr := range targets {
			rs.updateRoute(addr, "refresh "+family)
		}
	}()
}

// handleCommand processes command requests via SDK execute-command callback.
// Returns (status, data, error) for the SDK to send back to the engine.
func (rs *RouteServer) handleCommand(command string) (string, string, error) {
	switch command {
	case "rr status":
		return statusDone, `{"running":true}`, nil
	case "rr peers":
		return statusDone, rs.peersJSON(), nil
	default: // fail on unknown command
		return "error", "", fmt.Errorf("unknown command: %s", command)
	}
}

// peersJSON returns peer state as JSON.
func (rs *RouteServer) peersJSON() string {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	peers := make([]map[string]any, 0, len(rs.peers))
	for _, p := range rs.peers {
		peers = append(peers, map[string]any{
			"address": p.Address,
			"asn":     p.ASN,
			"up":      p.Up,
		})
	}

	data, _ := json.Marshal(map[string]any{"peers": peers})
	return string(data)
}

// --- Event types ---

// Event represents a parsed BGP event (from text format).
// Fields are extracted by parseTextState, parseTextOpen, parseTextRefresh.
type Event struct {
	Type     string    // Event type: "update", "state", "open", "refresh"
	MsgID    uint64    // Message ID (for cache-forward)
	PeerAddr string    // Peer address
	PeerASN  uint32    // Peer ASN
	State    string    // State for state events ("up", "down", "connected")
	Open     *OpenInfo // OPEN: decoded open data
	AFI      string    // Refresh: AFI
	SAFI     string    // Refresh: SAFI
}

// FamilyOperation represents a single add or del operation for a family.
// Text format: "announce ... <family> next-hop <nh> nlri <prefix>..."
// or "withdraw <family> nlri <prefix>...".
type FamilyOperation struct {
	Action string // "add" or "del"
	NLRIs  []any  // Prefix strings from text parsing
}

// OpenInfo contains OPEN message details.
type OpenInfo struct {
	ASN          uint32
	RouterID     string
	HoldTime     uint16
	Capabilities []CapabilityInfo
}

// capabilityPresent returns true if any capability in the list has the given name.
func capabilityPresent(caps []CapabilityInfo, name string) bool {
	for _, c := range caps {
		if c.Name == name {
			return true
		}
	}
	return false
}

// CapabilityInfo represents a single capability from the OPEN message.
type CapabilityInfo struct {
	Code  int
	Name  string
	Value string
}

// --- Text event parsing ---
//
// Text format replaces JSON for the bgp-rr hot path.
// Format: "peer <addr> <dir> <type> <id> ..."
// State:  "peer <addr> asn <n> state <state>"
// Parsed with strings.Fields instead of json.Unmarshal (6+ calls per UPDATE → 0).

// quickParseTextEvent extracts event type, message ID, peer address, and raw text
// from a text-format event line. Returns the full text as payload for deferred parsing.
//
// UPDATE:  "peer <addr> <dir> update <id> ..."  → fields[3]="update", fields[4]=id
// OPEN:    "peer <addr> <dir> open <id> ..."    → fields[3]="open", fields[4]=id
// STATE:   "peer <addr> asn <n> state <state>"  → fields[4]="state" (different layout)
// REFRESH: "peer <addr> <dir> refresh <id> ..." → fields[3]="refresh".
func quickParseTextEvent(text string) (string, uint64, string, string, error) {
	text = strings.TrimRight(text, "\n")
	fields := strings.Fields(text)
	if len(fields) < 5 || fields[0] != "peer" {
		return "", 0, "", "", fmt.Errorf("invalid text event: too short or missing peer prefix")
	}

	peerAddr := fields[1]

	// State events have a different layout: "peer <addr> asn <n> state <state>"
	if fields[4] == eventState {
		return eventState, 0, peerAddr, text, nil
	}

	// Message events: "peer <addr> <dir> <type> <id> ..."
	eventType := fields[3]
	id, err := strconv.ParseUint(fields[4], 10, 64)
	if err != nil {
		return eventType, 0, peerAddr, text, nil //nolint:nilerr // Non-numeric ID is valid for some event types
	}

	return eventType, id, peerAddr, text, nil
}

// parseTextUpdateFamilies extracts family names from a text UPDATE event.
// Families are tokens containing "/" (e.g., "ipv4/unicast", "ipv6/unicast").
// Returns a map of family → true for selectForwardTargets compatibility.
func parseTextUpdateFamilies(text string) map[string]bool {
	fields := strings.Fields(text)
	families := make(map[string]bool)
	for _, f := range fields {
		if isFamilyToken(f) {
			families[f] = true
		}
	}
	return families
}

// parseTextNLRIOps extracts family operations (add/del + NLRIs) from text UPDATE lines.
// Used by processForward to populate the withdrawal map.
//
// Announce: "... announce <attrs> <family> next-hop <nh> nlri <prefix>..."
// Withdraw: "... withdraw <family> nlri <prefix>..."
//
// Multi-line text (announce + withdraw in same UPDATE) is handled by processing
// each line independently and merging results.
func parseTextNLRIOps(text string) map[string][]FamilyOperation {
	result := make(map[string][]FamilyOperation)
	for line := range strings.SplitSeq(strings.TrimRight(text, "\n"), "\n") {
		fields := strings.Fields(line)
		parseTextNLRIOpsLine(fields, result)
	}
	return result
}

// parseTextNLRIOpsLine parses a single text UPDATE line into family operations.
// Scans for "announce" or "withdraw" tokens, then extracts family/nlri sequences.
func parseTextNLRIOpsLine(fields []string, result map[string][]FamilyOperation) {
	// Find "announce" or "withdraw" token
	action := ""
	startIdx := 0
	for i, f := range fields {
		if f == "announce" {
			action = actionAdd
			startIdx = i + 1
			break
		}
		if f == "withdraw" {
			action = actionDel
			startIdx = i + 1
			break
		}
	}
	if action == "" {
		return
	}

	// Scan remaining fields for family→nlri sequences.
	// Pattern: <family> [next-hop <nh>] nlri <prefix>... [<next-family> ...]
	var currentFamily string
	var nlris []any
	inNLRI := false

	for i := startIdx; i < len(fields); i++ {
		f := fields[i]

		// Family token: "afi/safi" with non-numeric suffix (not NLRI prefix like "10.0.0.0/24")
		if isFamilyToken(f) && !inNLRI {
			// Flush previous family
			if currentFamily != "" && len(nlris) > 0 {
				result[currentFamily] = append(result[currentFamily],
					FamilyOperation{Action: action, NLRIs: nlris})
			}
			currentFamily = f
			nlris = nil
			inNLRI = false
			continue
		}

		// Skip "next-hop" and its value
		if f == "next-hop" {
			i++ // skip the next-hop address
			continue
		}

		// "nlri" token starts NLRI collection
		if f == "nlri" {
			inNLRI = true
			continue
		}

		// New family while collecting NLRIs: flush current and restart
		if inNLRI && isFamilyToken(f) {
			if currentFamily != "" && len(nlris) > 0 {
				result[currentFamily] = append(result[currentFamily],
					FamilyOperation{Action: action, NLRIs: nlris})
			}
			currentFamily = f
			nlris = nil
			inNLRI = false
			continue
		}

		// Collect NLRIs or skip attribute tokens
		if inNLRI {
			nlris = append(nlris, f)
		}
	}

	// Flush last family
	if currentFamily != "" && len(nlris) > 0 {
		result[currentFamily] = append(result[currentFamily],
			FamilyOperation{Action: action, NLRIs: nlris})
	}
}

// isFamilyToken distinguishes BGP address family tokens (e.g., "ipv4/unicast")
// from NLRI prefix tokens (e.g., "10.0.0.0/24", "2001:db8::/32").
// Families have a non-numeric suffix after "/"; prefixes have a numeric suffix.
func isFamilyToken(s string) bool {
	idx := strings.LastIndex(s, "/")
	if idx < 0 || idx == len(s)-1 {
		return false
	}
	suffix := s[idx+1:]
	if suffix == "" {
		return false
	}
	// If the suffix is all digits, it's a prefix length (NLRI), not a family.
	for _, c := range suffix {
		if c < '0' || c > '9' {
			return true // non-numeric → family (e.g., "unicast", "vpn", "evpn")
		}
	}
	return false
}

// parseTextOpen extracts OPEN event data from text format.
// Format: "peer <addr> <dir> open <id> asn <n> router-id <ip> hold-time <t> cap <code> <name> [<value>]...".
func parseTextOpen(text string) *Event {
	fields := strings.Fields(strings.TrimRight(text, "\n"))
	if len(fields) < 6 {
		return nil
	}

	event := &Event{
		Type:     eventOpen,
		PeerAddr: fields[1],
		Open:     &OpenInfo{},
	}

	// Parse known key-value pairs
	for i := 5; i < len(fields)-1; i++ {
		switch fields[i] {
		case tokenASN:
			n, err := strconv.ParseUint(fields[i+1], 10, 32)
			if err == nil {
				event.PeerASN = uint32(n)  //nolint:gosec // bounded by ParseUint bitSize=32
				event.Open.ASN = uint32(n) //nolint:gosec // bounded by ParseUint bitSize=32
			}
			i++
		case tokenRouterID:
			event.Open.RouterID = fields[i+1]
			i++
		case tokenHoldTime:
			n, err := strconv.ParseUint(fields[i+1], 10, 16)
			if err == nil {
				event.Open.HoldTime = uint16(n) //nolint:gosec // bounded by ParseUint bitSize=16
			}
			i++
		case tokenCap:
			// cap <code> <name> [<value>]
			if i+2 >= len(fields) {
				break
			}
			code, _ := strconv.Atoi(fields[i+1])
			name := fields[i+2]
			value := ""
			// Check if next token is a value (not another keyword)
			if i+3 < len(fields) && fields[i+3] != tokenCap && fields[i+3] != tokenASN &&
				fields[i+3] != tokenRouterID && fields[i+3] != tokenHoldTime {
				value = fields[i+3]
				i++
			}
			event.Open.Capabilities = append(event.Open.Capabilities,
				CapabilityInfo{Code: code, Name: name, Value: value})
			i += 2
		}
	}

	return event
}

// parseTextState extracts state event data from text format.
// Format: "peer <addr> asn <n> state <state>".
func parseTextState(text string) *Event {
	fields := strings.Fields(strings.TrimRight(text, "\n"))
	if len(fields) < 5 {
		return nil
	}

	event := &Event{
		Type:     eventState,
		PeerAddr: fields[1],
	}

	for i := 2; i < len(fields)-1; i++ {
		switch fields[i] {
		case "asn":
			n, err := strconv.ParseUint(fields[i+1], 10, 32)
			if err == nil {
				event.PeerASN = uint32(n) //nolint:gosec // bounded by ParseUint bitSize=32
			}
			i++
		case "state":
			event.State = fields[i+1]
			i++
		}
	}

	return event
}

// parseTextRefresh extracts refresh event data from text format.
// Format: "peer <addr> <dir> refresh <id> family <family>"
// Also handles "borr" and "eorr" subtypes per RFC 7313.
func parseTextRefresh(text string) *Event {
	fields := strings.Fields(strings.TrimRight(text, "\n"))
	if len(fields) < 5 {
		return nil
	}

	// Subtype is fields[3]: "refresh", "borr", or "eorr"
	event := &Event{
		Type:     fields[3],
		PeerAddr: fields[1],
	}

	for i := 5; i < len(fields)-1; i++ {
		if fields[i] == "family" {
			family := fields[i+1]
			if parts := strings.SplitN(family, "/", 2); len(parts) == 2 {
				event.AFI = parts[0]
				event.SAFI = parts[1]
			}
			break
		}
	}

	return event
}
