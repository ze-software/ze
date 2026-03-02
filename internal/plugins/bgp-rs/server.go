// Design: docs/architecture/core-design.md — route server plugin
// Related: worker.go — per-source-peer worker pool with backpressure
// Related: peer.go — PeerState tracking (families, up/down)

package bgp_rs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/textparse"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
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

// nlriKey extracts the compact routing key from an NLRI string.
// Strips the "prefix " type keyword since it is redundant within a family.
// Other NLRI types (VPN, BGP-LS, EVPN) use the full string as key.
func nlriKey(nlri string) string {
	if after, ok := strings.CutPrefix(nlri, "prefix "); ok {
		return after
	}
	return nlri
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

// forwardBatch accumulates forward items for batch RPC.
// Per-worker state: no concurrent access for a given workerKey.
type forwardBatch struct {
	ids       []uint64
	selector  string   // comma-joined target peers
	targetBuf []string // reusable buffer for selectForwardTargets
}

// forwardCmd is a single fire-and-forget forward RPC to be sent by the background sender.
type forwardCmd struct {
	peer string
	cmd  string
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

	// Extract families for forward target selection.
	// Structured path (DirectBridge): read directly from wire, no text parsing.
	// Text path (fork-mode): parse from text payload.
	var families map[string]bool
	if ctx.msg != nil {
		families = extractWireFamilies(ctx.msg)
	} else {
		families = parseTextUpdateFamilies(ctx.textPayload)
	}
	if len(families) == 0 {
		return
	}

	// Accumulate forward in per-worker batch (flushed on batch-full or channel drain).
	forwarded = true
	rs.batchForwardUpdate(key, ctx.sourcePeer, msgID, families)

	// Update withdrawal map: track announced routes for withdrawal on peer-down.
	// Only family+prefix needed — bgp-adj-rib-in owns full route state.
	rs.withdrawalMu.Lock()
	if ctx.msg != nil {
		rs.updateWithdrawalMapWire(ctx.sourcePeer, ctx.msg)
	} else {
		rs.updateWithdrawalMapText(ctx.sourcePeer, parseTextNLRIOps(ctx.textPayload))
	}
	rs.withdrawalMu.Unlock()
}

// extractWireFamilies extracts address families from a raw UPDATE message.
// Uses MPReachWire.Family() and MPUnreachWire.Family() (3-byte reads each),
// and checks for IPv4 body NLRIs. No NLRI parsing needed.
func extractWireFamilies(msg *bgptypes.RawMessage) map[string]bool {
	families := make(map[string]bool, 2)
	wu := msg.WireUpdate
	if wu == nil {
		return families
	}

	if mp, err := wu.MPReach(); err == nil && mp != nil {
		families[mp.Family().String()] = true
	}
	if mp, err := wu.MPUnreach(); err == nil && mp != nil {
		families[mp.Family().String()] = true
	}
	// Check IPv4 body NLRIs (only present for IPv4 unicast).
	if body, err := wu.NLRI(); err == nil && len(body) > 0 {
		families["ipv4/unicast"] = true
	}
	if wd, err := wu.Withdrawn(); err == nil && len(wd) > 0 {
		families["ipv4/unicast"] = true
	}

	return families
}

// updateWithdrawalMapWire updates the withdrawal map from raw wire UPDATE data.
// Uses NLRIIterator for zero-allocation NLRI walking on IPv4/IPv6 unicast.
// Falls back to NLRIs() (allocating) for non-unicast families to produce correct text keys.
// Caller must hold rs.withdrawalMu.
func (rs *RouteServer) updateWithdrawalMapWire(sourcePeer string, msg *bgptypes.RawMessage) {
	if msg.WireUpdate == nil {
		return
	}
	wu := msg.WireUpdate

	// Get encoding context for add-path detection.
	var encCtx *bgpctx.EncodingContext
	if msg.AttrsWire != nil {
		encCtx = bgpctx.Registry.Get(msg.AttrsWire.SourceContext())
	}

	// MP_REACH_NLRI — announced routes (add).
	if mp, err := wu.MPReach(); err == nil && mp != nil {
		family := mp.Family()
		addPath := encCtx != nil && encCtx.AddPath(family)
		if isUnicast(family) {
			if iter := mp.NLRIIterator(addPath); iter != nil {
				rs.walkUnicastNLRIs(sourcePeer, family.String(), iter, actionAdd)
			}
		} else {
			nlris, nlriErr := mp.NLRIs(addPath)
			rs.walkNLRIsAllocating(sourcePeer, family, nlris, nlriErr)
		}
	}

	// MP_UNREACH_NLRI — withdrawn routes (del).
	if mp, err := wu.MPUnreach(); err == nil && mp != nil {
		family := mp.Family()
		addPath := encCtx != nil && encCtx.AddPath(family)
		if isUnicast(family) {
			if iter := mp.NLRIIterator(addPath); iter != nil {
				rs.walkUnicastNLRIs(sourcePeer, family.String(), iter, actionDel)
			}
		} else {
			nlris, nlriErr := mp.NLRIs(addPath)
			rs.walkUnreachNLRIsAllocating(sourcePeer, family, nlris, nlriErr)
		}
	}

	// IPv4 body NLRIs — announced routes (add).
	addPathV4 := encCtx != nil && encCtx.AddPath(nlri.IPv4Unicast)
	if iter, err := wu.NLRIIterator(addPathV4); err == nil && iter != nil {
		rs.walkUnicastNLRIs(sourcePeer, "ipv4/unicast", iter, actionAdd)
	}

	// IPv4 body Withdrawn — withdrawn routes (del).
	if iter, err := wu.WithdrawnIterator(addPathV4); err == nil && iter != nil {
		rs.walkUnicastNLRIs(sourcePeer, "ipv4/unicast", iter, actionDel)
	}
}

// isUnicast returns true for IPv4/IPv6 unicast families where NLRIIterator
// prefix bytes can be converted to netip.Prefix directly (zero-alloc path).
func isUnicast(f nlri.Family) bool {
	return f == nlri.IPv4Unicast || f == nlri.IPv6Unicast
}

// walkUnicastNLRIs walks NLRIs via iterator and updates the withdrawal map.
// Converts raw prefix bytes to netip.Prefix for route key — zero allocation per NLRI.
// Only valid for IPv4/IPv6 unicast families.
func (rs *RouteServer) walkUnicastNLRIs(sourcePeer, family string, iter *nlri.NLRIIterator, action string) {
	isV6 := strings.HasPrefix(family, "ipv6/")
	for {
		prefix, _, ok := iter.Next()
		if !ok {
			break
		}
		key := prefixBytesToKey(prefix, isV6)
		if key == "" {
			continue
		}
		routeKey := family + "|" + key
		switch action {
		case actionAdd:
			if rs.withdrawals[sourcePeer] == nil {
				rs.withdrawals[sourcePeer] = make(map[string]withdrawalInfo)
			}
			rs.withdrawals[sourcePeer][routeKey] = withdrawalInfo{Family: family, Prefix: "prefix " + key}
		case actionDel:
			if rs.withdrawals[sourcePeer] != nil {
				delete(rs.withdrawals[sourcePeer], routeKey)
			}
		}
	}
}

// prefixBytesToKey converts raw NLRI prefix bytes from NLRIIterator to a route key string.
// Input: [bitLen, addr_bytes...] from NLRIIterator.Next().
// Returns netip.Prefix.String() (e.g., "10.0.0.0/24", "2001:db8::/32").
func prefixBytesToKey(prefix []byte, isV6 bool) string {
	if len(prefix) == 0 {
		return ""
	}
	bitLen := int(prefix[0])
	addrBytes := prefix[1:]
	if isV6 {
		var addr [16]byte
		copy(addr[:], addrBytes)
		p := netip.PrefixFrom(netip.AddrFrom16(addr), bitLen)
		return p.Masked().String()
	}
	var addr [4]byte
	copy(addr[:], addrBytes)
	p := netip.PrefixFrom(netip.AddrFrom4(addr), bitLen)
	return p.Masked().String()
}

// walkNLRIsAllocating updates the withdrawal map using parsed NLRI objects (add action).
// Used for non-unicast families where raw prefix bytes need family-specific decoding.
// Allocates via NLRIs() — acceptable for rare non-unicast route server traffic.
func (rs *RouteServer) walkNLRIsAllocating(sourcePeer string, family nlri.Family, nlris []nlri.NLRI, err error) {
	if err != nil || len(nlris) == 0 {
		return
	}
	familyStr := family.String()
	if rs.withdrawals[sourcePeer] == nil {
		rs.withdrawals[sourcePeer] = make(map[string]withdrawalInfo)
	}
	for _, n := range nlris {
		s := n.String()
		routeKey := familyStr + "|" + nlriKey(s)
		rs.withdrawals[sourcePeer][routeKey] = withdrawalInfo{Family: familyStr, Prefix: s}
	}
}

// walkUnreachNLRIsAllocating updates the withdrawal map using parsed NLRI objects (del action).
// Used for non-unicast MP_UNREACH_NLRI families.
func (rs *RouteServer) walkUnreachNLRIsAllocating(sourcePeer string, family nlri.Family, nlris []nlri.NLRI, err error) {
	if err != nil || len(nlris) == 0 {
		return
	}
	familyStr := family.String()
	if rs.withdrawals[sourcePeer] != nil {
		for _, n := range nlris {
			delete(rs.withdrawals[sourcePeer], familyStr+"|"+nlriKey(n.String()))
		}
	}
}

// updateWithdrawalMapText updates the withdrawal map from text-parsed NLRI operations.
// Caller must hold rs.withdrawalMu.
func (rs *RouteServer) updateWithdrawalMapText(sourcePeer string, ops map[string][]FamilyOperation) {
	for family, familyOps := range ops {
		for _, op := range familyOps {
			switch op.Action {
			case actionAdd:
				if rs.withdrawals[sourcePeer] == nil {
					rs.withdrawals[sourcePeer] = make(map[string]withdrawalInfo)
				}
				for _, n := range op.NLRIs {
					if s, ok := n.(string); ok && s != "" {
						routeKey := family + "|" + nlriKey(s)
						rs.withdrawals[sourcePeer][routeKey] = withdrawalInfo{Family: family, Prefix: s}
					}
				}
			case actionDel:
				if rs.withdrawals[sourcePeer] != nil {
					for _, n := range op.NLRIs {
						if s, ok := n.(string); ok && s != "" {
							delete(rs.withdrawals[sourcePeer], family+"|"+nlriKey(s))
						}
					}
				}
			}
		}
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

// selectForwardTargets returns peers that should receive an UPDATE with the given families.
// A peer is included if it is up, is not the source, and supports at least one family
// in the UPDATE (or has nil Families, meaning unknown/all-accepted).
func (rs *RouteServer) selectForwardTargets(buf []string, sourcePeer string, families map[string]bool) []string {
	buf = buf[:0]
	for addr, peer := range rs.peers {
		if addr == sourcePeer || !peer.Up {
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
		buf = append(buf, addr)
	}
	sort.Strings(buf)
	return buf
}

// batchForwardUpdate accumulates a forward item into the per-worker batch.
// Selects targets, then appends to the current batch. Flushes the old batch
// if the target selector changes (different peer set). Flushes when the batch
// reaches maxBatchSize items. Partial batches are flushed by the onDrained
// callback when the worker channel empties.
func (rs *RouteServer) batchForwardUpdate(key workerKey, sourcePeer string, msgID uint64, families map[string]bool) {
	val, _ := rs.batches.LoadOrStore(key, &forwardBatch{})
	batch, ok := val.(*forwardBatch)
	if !ok {
		rs.releaseCache(msgID)
		return
	}

	rs.mu.RLock()
	batch.targetBuf = rs.selectForwardTargets(batch.targetBuf, sourcePeer, families)
	rs.mu.RUnlock()
	targets := batch.targetBuf

	if len(targets) == 0 {
		rs.releaseCache(msgID)
		return
	}

	sel := strings.Join(targets, ",")

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
// per-peer lifecycle goroutine (not blocking the event loop).
// A convergent delta replay loop then covers routes that adj-rib-in may not
// have stored yet at full-replay time (race between event delivery and replay).
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
	cmd := fmt.Sprintf("adj-rib-in replay %s 0", peerAddr)
	status, data, err := rs.dispatchCommand(ctx, cmd)
	if err != nil || status != statusDone {
		logger().Error("replay failed", "peer", peerAddr, "command", cmd, "status", status, "error", err)
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
	lastIndex, _ := parseReplayResponse(data)

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

	// Convergent delta replay: catch routes that adj-rib-in received after
	// the full replay snapshot. For internal plugins, events arrive via
	// DirectBridge on the engine's delivery goroutine while replay commands
	// arrive on Socket B — these are concurrent, so adj-rib-in may not have
	// stored recently-delivered routes when the full replay ran. Repeat until
	// no new routes appear (replayed==0), with a brief pause between attempts
	// to let adj-rib-in's event handler process pending deliveries.
	for i := range replayConvergenceMax {
		if lastIndex == 0 {
			break
		}
		if i > 0 {
			time.Sleep(replayConvergenceDelay)
		}
		deltaCmd := fmt.Sprintf("adj-rib-in replay %s %d", peerAddr, lastIndex)
		_, deltaData, deltaErr := rs.dispatchCommand(ctx, deltaCmd)
		if deltaErr != nil {
			logger().Warn("delta replay failed", "peer", peerAddr, "attempt", i, "error", deltaErr)
			break
		}
		newLast, replayed := parseReplayResponse(deltaData)
		if replayed == 0 {
			break
		}
		logger().Debug("delta replay caught new routes", "peer", peerAddr, "attempt", i, "replayed", replayed)
		lastIndex = newLast
	}
}

// parseReplayResponse extracts last-index and replayed count from a replay response.
// Expected format: {"last-index":N,"replayed":M}.
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
	case "rs status":
		return statusDone, `{"running":true}`, nil
	case "rs peers":
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
// Text format: "nlri <family> add <prefix>..." or "nlri <family> del <prefix>...".
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
// Text format replaces JSON for the bgp-rs hot path.
// Uniform header: "peer <addr> asn <n> <dir> <type> <id> ..."
// State:          "peer <addr> asn <n> state <state>"
// Parsed with TextScanner (zero-copy token extraction from original string).

// topLevelKeywords and nlriTypeKeywords are now shared from textparse package.
// Use textparse.IsTopLevelKeyword() and textparse.NLRITypeKeywords.

// buildNLRIEntries splits collected tokens into individual NLRI strings.
// Accepts two formats:
//   - Comma: "prefix 10.0.0.0/24,10.0.1.0/24" — type keyword + comma-separated values.
//   - Keyword boundary: "prefix 10.0.0.0/24 prefix 10.0.1.0/24" — repeated type keyword.
func buildNLRIEntries(tokens []string) []any {
	if len(tokens) == 0 {
		return nil
	}

	// Check for comma in any token.
	for i, tok := range tokens {
		if !strings.Contains(tok, ",") {
			continue
		}
		// Prefix = all tokens before the comma token (e.g., "prefix").
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

	// No commas — check for keyword boundary (repeated type keywords).
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

	// Single complex NLRI: join all tokens.
	return []any{strings.Join(tokens, " ")}
}

// quickParseTextEvent extracts event type, message ID, peer address, and raw text
// from a text-format event line. Returns the full text as payload for deferred parsing.
//
// Uniform header: "peer <addr> asn <n> ..."
// State:   "peer <addr> asn <n> state <state>"       → dispatch="state"
// Message: "peer <addr> asn <n> <dir> <type> <id>"   → dispatch=<type>.
func quickParseTextEvent(text string) (string, uint64, string, string, error) {
	text = strings.TrimRight(text, "\n")
	s := textparse.NewScanner(text)

	// peer
	if tok, ok := s.Next(); !ok || tok != "peer" {
		return "", 0, "", "", fmt.Errorf("invalid text event: missing peer prefix")
	}
	// <addr>
	peerAddr, ok := s.Next()
	if !ok {
		return "", 0, "", "", fmt.Errorf("invalid text event: missing peer address")
	}
	// asn
	if tok, ok := s.Next(); !ok || tok != tokenASN {
		return "", 0, "", "", fmt.Errorf("invalid text event: missing asn keyword")
	}
	// <n> (ASN value — consumed but not returned; available from payload)
	if _, ok := s.Next(); !ok {
		return "", 0, "", "", fmt.Errorf("invalid text event: missing asn value")
	}

	// Next token: either "state" or <direction>
	dispatchTok, ok := s.Next()
	if !ok {
		return "", 0, "", "", fmt.Errorf("invalid text event: missing dispatch token")
	}
	if dispatchTok == eventState {
		return eventState, 0, peerAddr, text, nil
	}

	// Message events: <direction> was consumed as dispatchTok, next is <type> <id>
	eventType, ok := s.Next()
	if !ok {
		return "", 0, "", "", fmt.Errorf("invalid text event: missing event type")
	}
	idStr, ok := s.Next()
	if !ok {
		return eventType, 0, peerAddr, text, nil
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return eventType, 0, peerAddr, text, nil //nolint:nilerr // Non-numeric ID is valid for some event types
	}

	return eventType, id, peerAddr, text, nil
}

// parseTextUpdateFamilies extracts family names from a text UPDATE event.
// Scans for "nlri" keyword followed by an afi/safi token (the family).
// Returns a map of family → true for selectForwardTargets compatibility.
func parseTextUpdateFamilies(text string) map[string]bool {
	s := textparse.NewScanner(text)
	families := make(map[string]bool)
	for !s.Done() {
		tok, ok := s.Next()
		if !ok {
			break
		}
		if tok == textparse.KWNLRI {
			if fam, ok := s.Next(); ok {
				if strings.Contains(fam, "/") {
					families[fam] = true
				}
			}
		}
	}
	return families
}

// parseTextNLRIOps extracts family operations (add/del + NLRIs) from a text UPDATE.
// Used by processForward to populate the withdrawal map.
//
// Format: "peer <addr> asn <n> <dir> update <id> <attrs> [next <nh>] nlri <fam> add|del <nlris> ..."
//
// Key-dispatch loop processes keywords sequentially, resolving aliases via textparse.ResolveAlias:
// - Attribute keywords (origin, path, pref, etc.): skip value(s)
// - "nlri": consume family, extract action (add/del) and collect NLRI tokens until next keyword.
func parseTextNLRIOps(text string) map[string][]FamilyOperation {
	result := make(map[string][]FamilyOperation)
	s := textparse.NewScanner(strings.TrimRight(text, "\n"))

	// Skip header: peer <addr> asn <n> <dir> update <id>
	for i := 0; i < 7 && !s.Done(); i++ {
		s.Next()
	}

	// Key-dispatch loop — resolve aliases so both short (API) and long (config) forms work.
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
			// Family: nlri <family> add|del
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

			// Action: add or del
			action, ok := s.Next()
			if !ok || (action != actionAdd && action != actionDel) {
				continue
			}

			// Collect NLRI tokens until next top-level keyword or end.
			var nlriTokens []string
			for !s.Done() {
				next, ok := s.Peek()
				if !ok || textparse.IsTopLevelKeyword(next) {
					break
				}
				tok, _ := s.Next()
				nlriTokens = append(nlriTokens, tok)
			}

			// Build NLRI entries (handles comma and keyword-boundary formats).
			nlris := buildNLRIEntries(nlriTokens)

			if len(nlris) > 0 {
				result[family] = append(result[family],
					FamilyOperation{Action: action, NLRIs: nlris})
			}

		// Attribute keywords: consume their values.
		// Scalar attributes (one value token).
		case textparse.KWOrigin, textparse.KWMED, textparse.KWLocalPreference,
			textparse.KWAggregator, textparse.KWOriginatorID:
			s.Next()
		// Comma-list attributes (one comma-separated value token).
		case textparse.KWASPath, textparse.KWCommunity, textparse.KWLargeCommunity,
			textparse.KWExtendedCommunity, textparse.KWClusterList:
			s.Next()
		case textparse.KWAtomicAggregate:
			// flag, no value
		}
	}

	return result
}

// parseTextOpen extracts OPEN event data from text format.
// Format: "peer <addr> asn <n> <dir> open <id> router-id <ip> hold-time <t> cap <code> <name> [<value>]...".
// ASN is extracted from the uniform header.
func parseTextOpen(text string) *Event {
	s := textparse.NewScanner(strings.TrimRight(text, "\n"))

	// peer <addr>
	s.Next()
	addr, ok := s.Next()
	if !ok {
		return nil
	}

	event := &Event{
		Type:     eventOpen,
		PeerAddr: addr,
		Open:     &OpenInfo{},
	}

	// asn <n>
	s.Next() // "asn"
	if asnStr, ok := s.Next(); ok {
		if n, err := strconv.ParseUint(asnStr, 10, 32); err == nil {
			event.PeerASN = uint32(n)  //nolint:gosec // bounded by ParseUint bitSize=32
			event.Open.ASN = uint32(n) //nolint:gosec // bounded by ParseUint bitSize=32
		}
	}

	// <dir> open <id>
	s.Next() // direction
	s.Next() // "open"
	s.Next() // message ID

	// Keyword loop for remaining body
	for !s.Done() {
		tok, ok := s.Next()
		if !ok {
			break
		}
		switch tok {
		case tokenRouterID:
			if v, ok := s.Next(); ok {
				event.Open.RouterID = v
			}
		case tokenHoldTime:
			if v, ok := s.Next(); ok {
				if n, err := strconv.ParseUint(v, 10, 16); err == nil {
					event.Open.HoldTime = uint16(n) //nolint:gosec // bounded by ParseUint bitSize=16
				}
			}
		case tokenCap:
			// cap <code> <name> [<value>]
			codeStr, ok := s.Next()
			if !ok {
				continue
			}
			code, _ := strconv.Atoi(codeStr)
			name, ok := s.Next()
			if !ok {
				continue
			}
			value := ""
			if next, ok := s.Peek(); ok && next != tokenCap && next != tokenRouterID && next != tokenHoldTime {
				value, _ = s.Next()
			}
			event.Open.Capabilities = append(event.Open.Capabilities,
				CapabilityInfo{Code: code, Name: name, Value: value})
		}
	}

	return event
}

// parseTextState extracts state event data from text format.
// Format: "peer <addr> asn <n> state <state>".
func parseTextState(text string) *Event {
	s := textparse.NewScanner(strings.TrimRight(text, "\n"))

	// peer <addr>
	s.Next()
	addr, ok := s.Next()
	if !ok {
		return nil
	}

	event := &Event{
		Type:     eventState,
		PeerAddr: addr,
	}

	// Keyword loop
	for !s.Done() {
		tok, ok := s.Next()
		if !ok {
			break
		}
		switch tok {
		case "asn":
			if v, ok := s.Next(); ok {
				if n, err := strconv.ParseUint(v, 10, 32); err == nil {
					event.PeerASN = uint32(n) //nolint:gosec // bounded by ParseUint bitSize=32
				}
			}
		case "state":
			if v, ok := s.Next(); ok {
				event.State = v
			}
		}
	}

	return event
}

// parseTextRefresh extracts refresh event data from text format.
// Format: "peer <addr> asn <n> <dir> refresh|borr|eorr <id> family <family>".
func parseTextRefresh(text string) *Event {
	s := textparse.NewScanner(strings.TrimRight(text, "\n"))

	// peer <addr>
	s.Next()
	addr, ok := s.Next()
	if !ok {
		return nil
	}

	// asn <n>
	s.Next() // "asn"
	s.Next() // ASN value

	// <dir> <type> <id>
	s.Next() // direction
	refreshType, ok := s.Next()
	if !ok {
		return nil
	}
	s.Next() // message ID

	event := &Event{
		Type:     refreshType,
		PeerAddr: addr,
	}

	// Keyword loop for family
	for !s.Done() {
		tok, ok := s.Next()
		if !ok {
			break
		}
		if tok == tokenFamily {
			if family, ok := s.Next(); ok {
				if parts := strings.SplitN(family, "/", 2); len(parts) == 2 {
					event.AFI = parts[0]
					event.SAFI = parts[1]
				}
			}
			break
		}
	}

	return event
}
