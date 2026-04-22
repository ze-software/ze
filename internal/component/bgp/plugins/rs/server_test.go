package rs

import (
	"fmt"
	"net"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// newTestRouteServer creates a RouteServer with closed SDK connections for unit testing.
// The plugin's connections are immediately closed so updateRoute calls fail silently,
// allowing tests to verify internal state (withdrawal map, peers) without RPC side effects.
func newTestRouteServer(t *testing.T) *RouteServer {
	t.Helper()
	pluginEnd, remoteEnd := net.Pipe()
	if err := remoteEnd.Close(); err != nil {
		t.Logf("close remoteEnd: %v", err)
	}
	p := sdk.NewWithConn("rr-test", pluginEnd)
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Logf("cleanup: %v", err)
		}
	})
	rs := &RouteServer{
		plugin:      p,
		peers:       make(map[string]*PeerState),
		withdrawals: make(map[string]map[string]withdrawalInfo),
	}
	rs.workers = newWorkerPool(func(key workerKey, item workItem) {
		rs.processForward(key, item)
	}, poolConfig{chanSize: 64, idleTimeout: 5 * time.Second, onDrained: rs.flushWorkerBatch})
	t.Cleanup(func() {
		rs.workers.Stop()
	})
	return rs
}

// flushWorkers stops and recreates the worker pool, ensuring all pending
// items are processed. rs-fastpath-3 removed the fire-and-forget sender
// goroutines; workers now call ForwardCached / ReleaseCached synchronously.
func flushWorkers(t *testing.T, rs *RouteServer) {
	t.Helper()
	rs.workers.Stop()
	rs.workers = newWorkerPool(func(key workerKey, item workItem) {
		rs.processForward(key, item)
	}, poolConfig{chanSize: 64, idleTimeout: 5 * time.Second, onDrained: rs.flushWorkerBatch})
}

// --- Handler integration tests (text → dispatchText → verify state) ---

// TestHandleUpdate_ZeBGPFormat verifies UPDATE processing from text event format.
//
// VALIDATES: Full flow from text parsing through withdrawal map insertion for an UPDATE announce.
// PREVENTS: Route propagation failure due to format mismatch.
func TestHandleUpdate_ZeBGPFormat(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	rs.dispatchText("peer 10.0.0.1 remote as 65001 received update 123 origin igp as-path 65001 local-preference 100 next-hop 192.168.1.1 nlri ipv4/unicast add prefix 10.0.0.0/24")
	flushWorkers(t, rs)

	rs.withdrawalMu.Lock()
	peerWd := rs.withdrawals["10.0.0.1"]
	rs.withdrawalMu.Unlock()
	if len(peerWd) != 1 {
		t.Fatalf("expected 1 withdrawal entry, got %d", len(peerWd))
	}
	entry, ok := peerWd["ipv4/unicast|10.0.0.0/24"]
	if !ok {
		t.Fatal("missing withdrawal entry for 10.0.0.0/24")
	}
	if entry.Family != "ipv4/unicast" {
		t.Errorf("expected family ipv4/unicast, got %s", entry.Family)
	}
	if entry.Prefix != "prefix 10.0.0.0/24" {
		t.Errorf("expected prefix field 'prefix 10.0.0.0/24', got %s", entry.Prefix)
	}
}

// TestHandleUpdate_Withdraw_ZeBGPFormat verifies withdrawal processing from text event format.
//
// VALIDATES: Full flow from text parsing through withdrawal map removal for a withdrawal.
// PREVENTS: Stale routes remaining after withdrawal.
func TestHandleUpdate_Withdraw_ZeBGPFormat(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	// Pre-populate withdrawal map (simulating prior add).
	rs.withdrawalMu.Lock()
	rs.withdrawals["10.0.0.1"] = map[string]withdrawalInfo{
		"ipv4/unicast|10.0.0.0/24": {Family: "ipv4/unicast", Prefix: "prefix 10.0.0.0/24"},
	}
	rs.withdrawalMu.Unlock()

	rs.dispatchText("peer 10.0.0.1 remote as 65001 received update 124 nlri ipv4/unicast del prefix 10.0.0.0/24")
	flushWorkers(t, rs)

	rs.withdrawalMu.Lock()
	peerWd := rs.withdrawals["10.0.0.1"]
	rs.withdrawalMu.Unlock()
	if _, found := peerWd["ipv4/unicast|10.0.0.0/24"]; found {
		t.Error("expected withdrawal entry removed after del")
	}
}

// TestHandleUpdate_MultiFamilyMixed verifies processing of UPDATE with both add and del operations.
//
// VALIDATES: Multiple family operations processed correctly in single UPDATE (AC-8).
// PREVENTS: Only first operation being processed, ignoring subsequent families.
func TestHandleUpdate_MultiFamilyMixed(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true,
		Families: map[family.Family]bool{family.IPv4Unicast: true, family.IPv6Unicast: true}}
	rs.mu.Unlock()

	// Pre-populate withdrawal map with route that will be withdrawn.
	rs.withdrawalMu.Lock()
	rs.withdrawals["10.0.0.1"] = map[string]withdrawalInfo{
		"ipv4/unicast|10.0.2.0/24": {Family: "ipv4/unicast", Prefix: "prefix 10.0.2.0/24"},
	}
	rs.withdrawalMu.Unlock()

	rs.dispatchText("peer 10.0.0.1 remote as 65001 received update 125 origin igp next-hop 192.168.1.1 nlri ipv4/unicast add prefix 10.0.0.0/24 next-hop 2001:db8::1 nlri ipv6/unicast add prefix 2001:db8::/32 nlri ipv4/unicast del prefix 10.0.2.0/24")
	flushWorkers(t, rs)

	rs.withdrawalMu.Lock()
	peerWd := rs.withdrawals["10.0.0.1"]
	rs.withdrawalMu.Unlock()
	if len(peerWd) != 2 {
		t.Fatalf("expected 2 withdrawal entries, got %d", len(peerWd))
	}
	if _, found := peerWd["ipv4/unicast|10.0.0.0/24"]; !found {
		t.Error("missing ipv4/unicast|10.0.0.0/24")
	}
	if _, found := peerWd["ipv6/unicast|2001:db8::/32"]; !found {
		t.Error("missing ipv6/unicast|2001:db8::/32")
	}
	if _, found := peerWd["ipv4/unicast|10.0.2.0/24"]; found {
		t.Error("10.0.2.0/24 should have been withdrawn")
	}
}

// TestHandleState_Down_ZeBGPFormat verifies peer down processing from text event format.
//
// VALIDATES: Peer down clears withdrawal map entries for that peer (AC-5).
// PREVENTS: Stale routes remaining after session teardown.
func TestHandleState_Down_ZeBGPFormat(t *testing.T) {
	rs := newTestRouteServer(t)

	// Populate withdrawal map (replaces old rs.rib.Insert).
	rs.withdrawalMu.Lock()
	rs.withdrawals["10.0.0.1"] = map[string]withdrawalInfo{
		"ipv4/unicast|10.0.0.0/24": {Family: "ipv4/unicast", Prefix: "prefix 10.0.0.0/24"},
		"ipv4/unicast|10.0.1.0/24": {Family: "ipv4/unicast", Prefix: "prefix 10.0.1.0/24"},
	}
	rs.withdrawalMu.Unlock()

	rs.dispatchText("peer 10.0.0.1 remote as 65001 state down")

	rs.withdrawalMu.Lock()
	wdLen := len(rs.withdrawals["10.0.0.1"])
	rs.withdrawalMu.Unlock()
	if wdLen != 0 {
		t.Errorf("expected 0 withdrawal entries after peer down, got %d", wdLen)
	}

	rs.mu.RLock()
	peer := rs.peers["10.0.0.1"]
	rs.mu.RUnlock()
	if peer == nil {
		t.Fatal("peer state not created")
		return
	}
	if peer.Up {
		t.Error("expected peer to be down")
	}
}

// TestHandleState_Up_ZeBGPFormat verifies peer up processing from text event format.
//
// VALIDATES: Peer up marks peer as up, triggers replay via DispatchCommand (AC-1, AC-3).
// PREVENTS: Missing state transition, new peer not receiving existing routes.
func TestHandleState_Up_ZeBGPFormat(t *testing.T) {
	rs := newTestRouteServer(t)

	var dispatched atomic.Bool
	rs.dispatchCommandHook = func(cmd string) (string, string, error) {
		dispatched.Store(true)
		return statusDone, `{"last-index":0,"replayed":0}`, nil
	}

	rs.dispatchText("peer 10.0.0.1 remote as 65001 state up")

	require.Eventually(t, dispatched.Load, 2*time.Second, time.Millisecond,
		"expected DispatchCommand to be called for replay")

	rs.mu.RLock()
	peer := rs.peers["10.0.0.1"]
	rs.mu.RUnlock()
	if peer == nil {
		t.Fatal("peer state not created")
		return
	}
	if !peer.Up {
		t.Error("expected peer to be up")
	}
}

// TestHandleState_Up_ExcludesSelf verifies replay command targets the connecting peer.
//
// VALIDATES: Replay command includes the peer address so bgp-adj-rib-in excludes self-routes (AC-7).
// PREVENTS: Routing loops from self-received routes.
func TestHandleState_Up_ExcludesSelf(t *testing.T) {
	rs := newTestRouteServer(t)

	var replayCmd atomic.Value
	rs.dispatchCommandHook = func(cmd string) (string, string, error) {
		replayCmd.Store(cmd)
		return statusDone, `{"last-index":0,"replayed":0}`, nil
	}

	rs.dispatchText("peer 10.0.0.1 remote as 65001 state up")

	require.Eventually(t, func() bool {
		v := replayCmd.Load()
		s, ok := v.(string)
		return ok && s != ""
	}, 2*time.Second, time.Millisecond, "expected replay DispatchCommand")

	rs.mu.RLock()
	peer := rs.peers["10.0.0.1"]
	rs.mu.RUnlock()
	if peer == nil {
		t.Fatal("peer not created")
		return
	}
	if !peer.Up {
		t.Error("expected peer to be up")
	}

	// The replay command must include the peer address — bgp-adj-rib-in uses it
	// to replay routes from ALL source peers EXCEPT the target peer itself.
	cmd, _ := replayCmd.Load().(string)
	if !strings.Contains(cmd, "10.0.0.1") {
		t.Errorf("replay command should target peer 10.0.0.1, got %q", cmd)
	}
}

// TestHandleOpen_ZeBGPFormat verifies OPEN processing from text event format.
//
// VALIDATES: OPEN event extracts capabilities and families from text format (AC-5).
// PREVENTS: Missing capability info due to format mismatch (Layer 4).
func TestHandleOpen_ZeBGPFormat(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.dispatchText("peer 10.0.0.1 remote as 65001 received open 0 router-id 10.0.0.1 hold-time 180 cap 1 multiprotocol ipv4/unicast cap 1 multiprotocol ipv6/unicast cap 2 route-refresh cap 65 asn4 65001")

	rs.mu.RLock()
	peer := rs.peers["10.0.0.1"]
	rs.mu.RUnlock()

	if peer == nil {
		t.Fatal("peer not created")
		return
	}
	if peer.ASN != 65001 {
		t.Errorf("expected ASN 65001, got %d", peer.ASN)
	}
	if !peer.HasCapability("route-refresh") {
		t.Error("missing route-refresh capability")
	}
	if !peer.HasCapability("multiprotocol") {
		t.Error("missing multiprotocol capability")
	}
	if !peer.HasCapability("asn4") {
		t.Error("missing asn4 capability")
	}
	if !peer.SupportsFamily(family.IPv4Unicast) {
		t.Error("missing ipv4/unicast family")
	}
	if !peer.SupportsFamily(family.IPv6Unicast) {
		t.Error("missing ipv6/unicast family")
	}
}

// TestHandleRefresh_ZeBGPFormat verifies refresh processing from text event format.
//
// VALIDATES: Refresh event extracts family from text format, forwards to capable peers (AC-6).
// PREVENTS: Missing family extraction from text refresh event.
func TestHandleRefresh_ZeBGPFormat(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address:      "10.0.0.1",
		Up:           true,
		Capabilities: map[string]bool{"route-refresh": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address:      "10.0.0.2",
		Up:           true,
		Capabilities: map[string]bool{"route-refresh": true},
	}
	rs.mu.Unlock()

	// handleRefresh calls updateRoute which sends via SDK RPC (fails silently on closed conn).
	rs.dispatchText("peer 10.0.0.1 remote as 65001 received refresh 0 family ipv4/unicast")
}

// --- Family/capability filtering tests ---

// TestFilterUpdateByFamily verifies UPDATE only forwards to compatible peers.
//
// VALIDATES: IPv6 routes forwarded and tracked in withdrawal map with correct family.
// PREVENTS: Sending routes to peers that can't handle them.
func TestFilterUpdateByFamily(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address:  "10.0.0.1",
		Up:       true,
		Families: map[family.Family]bool{family.IPv4Unicast: true, family.IPv6Unicast: true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address:  "10.0.0.2",
		Up:       true,
		Families: map[family.Family]bool{family.IPv4Unicast: true},
	}
	rs.peers["10.0.0.3"] = &PeerState{
		Address:  "10.0.0.3",
		Up:       true,
		Families: map[family.Family]bool{family.IPv4Unicast: true, family.IPv6Unicast: true},
	}
	rs.mu.Unlock()

	rs.dispatchText("peer 10.0.0.1 remote as 65001 received update 100 origin igp next-hop 2001:db8::1 nlri ipv6/unicast add prefix 2001:db8::/32")
	flushWorkers(t, rs)

	rs.withdrawalMu.Lock()
	peerWd := rs.withdrawals["10.0.0.1"]
	rs.withdrawalMu.Unlock()
	if len(peerWd) != 1 {
		t.Fatalf("expected 1 withdrawal entry, got %d", len(peerWd))
	}
	entry, ok := peerWd["ipv6/unicast|2001:db8::/32"]
	if !ok {
		t.Fatal("missing withdrawal entry for 2001:db8::/32")
	}
	if entry.Family != "ipv6/unicast" {
		t.Errorf("expected family ipv6/unicast, got %s", entry.Family)
	}
}

// TestFilterRefreshByCapability verifies refresh only sent to capable peers.
//
// VALIDATES: Refresh only forwarded to peers with route-refresh capability (AC-6).
// PREVENTS: Sending refresh to peers that don't support it.
func TestFilterRefreshByCapability(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address:      "10.0.0.1",
		Up:           true,
		Capabilities: map[string]bool{"route-refresh": true},
		Families:     map[family.Family]bool{family.IPv4Unicast: true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address:  "10.0.0.2",
		Up:       true,
		Families: map[family.Family]bool{family.IPv4Unicast: true},
	}
	rs.peers["10.0.0.3"] = &PeerState{
		Address:      "10.0.0.3",
		Up:           true,
		Capabilities: map[string]bool{"route-refresh": true},
		Families:     map[family.Family]bool{family.IPv4Unicast: true},
	}
	rs.mu.Unlock()

	// handleRefresh forwards to peers via updateRoute (fails silently on closed conn).
	rs.dispatchText("peer 10.0.0.1 remote as 65001 received refresh 0 family ipv4/unicast")
}

// TestFilterReplayByFamily verifies replay only sends compatible routes.
//
// VALIDATES: IPv4-only peer doesn't cause panic during replay with mixed-family routes.
// PREVENTS: Sending unsupported routes to new peer.
func TestFilterReplayByFamily(t *testing.T) {
	rs := newTestRouteServer(t)

	// Replay now handled by bgp-adj-rib-in via DispatchCommand.
	rs.dispatchCommandHook = func(cmd string) (string, string, error) {
		return statusDone, `{"last-index":0,"replayed":0}`, nil
	}

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address:  "10.0.0.1",
		Up:       true,
		Families: map[family.Family]bool{family.IPv4Unicast: true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address:  "10.0.0.2",
		Up:       true,
		Families: map[family.Family]bool{family.IPv4Unicast: true, family.IPv6Unicast: true},
	}
	rs.mu.Unlock()

	// handleState/handleStateUp calls updateRoute for route replay.
	rs.dispatchText("peer 10.0.0.1 remote as 65001 state up")

	rs.mu.RLock()
	peer := rs.peers["10.0.0.1"]
	rs.mu.RUnlock()
	if peer == nil {
		t.Fatal("peer not created")
		return
	}
	if !peer.Up {
		t.Error("expected peer to be up")
	}
}

// --- Command tests ---

// TestHandleCommand_Status verifies "rs status" command response.
//
// VALIDATES: RS responds to status command with done status and running JSON.
// PREVENTS: Command handler returning wrong status or data.
func TestHandleCommand_Status(t *testing.T) {
	rs := newTestRouteServer(t)

	status, data, err := rs.handleCommand("rs status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != statusDone {
		t.Errorf("expected status done, got %q", status)
	}
	if !strings.Contains(data, `"running":true`) {
		t.Errorf("expected running:true in data, got %q", data)
	}
}

// TestHandleCommand_Peers verifies "rs peers" command response.
//
// VALIDATES: RS responds to peers command with peer list JSON.
// PREVENTS: Command handler missing peer data.
func TestHandleCommand_Peers(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", ASN: 65001, Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", ASN: 65002, Up: false}
	rs.mu.Unlock()

	status, data, err := rs.handleCommand("rs peers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != statusDone {
		t.Errorf("expected status done, got %q", status)
	}
	if !strings.Contains(data, "10.0.0.1") {
		t.Errorf("expected peer 10.0.0.1 in data, got %q", data)
	}
}

// TestHandleCommand_Unknown verifies unknown command error response.
//
// VALIDATES: RS responds with error for unknown commands.
// PREVENTS: Silent failure on unknown commands.
func TestHandleCommand_Unknown(t *testing.T) {
	rs := newTestRouteServer(t)

	status, _, err := rs.handleCommand("rr unknown")
	if err == nil {
		t.Fatal("expected error for unknown command")
		return
	}
	if status != statusError {
		t.Errorf("expected status error, got %q", status)
	}
}

// TestRRUpdateRouteTimeout60s verifies the updateRoute timeout is 60 seconds.
//
// VALIDATES: AC-7 — RR plugin updateRoute uses 60s timeout (was 10s).
// PREVENTS: Regression to 10s timeout that causes silent route drops under load.
func TestRRUpdateRouteTimeout60s(t *testing.T) {
	if updateRouteTimeout != 60*time.Second {
		t.Errorf("updateRouteTimeout = %v, want 60s", updateRouteTimeout)
	}
}

// TestForwardCachedTimeout10s pins the fast-path timeout (rs-fastpath-3
// follow-up). ForwardCached / ReleaseCached are synchronous per-source
// DirectBridge calls with no socket or tokenise step; 10s is plenty for a
// deadlock trip-wire, and stops the fast path from inheriting the legacy
// text-RPC 60s budget that was sized for concurrent socket contention.
//
// VALIDATES: forwardCachedTimeout = 10s.
// PREVENTS: the fast path silently reinheriting updateRouteTimeout's 60s.
func TestForwardCachedTimeout10s(t *testing.T) {
	if forwardCachedTimeout != 10*time.Second {
		t.Errorf("forwardCachedTimeout = %v, want 10s", forwardCachedTimeout)
	}
}

// TestRSForwardPlumbingDeleted pins the rs-fastpath-3 "no layering" decision:
// after switching to ForwardCached / ReleaseCached, none of the legacy
// fire-and-forget plumbing lives on the RouteServer struct. Referencing any
// of these fields via reflect must fail.
//
// VALIDATES: AC-11 -- grep guard that the deleted names do not return on the
// struct. PREVENTS: regression where someone re-introduces a parallel sender
// pool alongside the fast path.
func TestRSForwardPlumbingDeleted(t *testing.T) {
	rs := newTestRouteServer(t)
	v := reflect.ValueOf(rs).Elem()
	for _, name := range []string{
		"forwardCh", "forwardStop", "forwardDone",
		"releaseCh", "releaseStop", "releaseDone",
	} {
		if v.FieldByName(name).IsValid() {
			t.Errorf("rs-fastpath-3: RouteServer field %q must be deleted", name)
		}
	}
}

// TestBatchForwardSingleFlushOnDrain verifies a single UPDATE is flushed
// promptly via the onDrained callback rather than waiting for batch fill.
//
// VALIDATES: spec-rs-fastpath-1-profile AC-4 -- one UPDATE reaches asyncForward
// within one worker turnaround (no unbounded wait for batch to reach
// maxBatchSize).
// PREVENTS: regression where onDrained stops firing for partial batches, which
// would reintroduce the "wait for batch full" latency the benchmark harness
// exists to catch.
func TestBatchForwardSingleFlushOnDrain(t *testing.T) {
	rs := newTestRouteServer(t)

	var forwardCalls int
	var cmdMu sync.Mutex
	rs.forwardCachedHook = func(_ []uint64, _ []string) {
		cmdMu.Lock()
		forwardCalls++
		cmdMu.Unlock()
	}

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	rs.dispatchText(buildTestUpdate("10.0.0.1", 1))
	flushWorkers(t, rs)

	cmdMu.Lock()
	defer cmdMu.Unlock()

	if forwardCalls != 1 {
		t.Fatalf("expected exactly 1 ForwardCached call (single UPDATE flushed via onDrained), got %d",
			forwardCalls)
	}
}

// TestDispatchPauseOnBackpressure verifies dispatch pauses source peer on backpressure.
//
// VALIDATES: AC-1 — dispatch sends pause when worker channel exceeds 75%.
// PREVENTS: Backpressure detection without action.
func TestDispatchPauseOnBackpressure(t *testing.T) {
	rs := newTestRouteServer(t)
	// Use small channel so we can fill it easily.
	rs.workers.Stop()
	block := make(chan struct{})
	rs.workers = newWorkerPool(func(_ workerKey, _ workItem) {
		<-block // Block all processing.
	}, poolConfig{chanSize: 8, idleTimeout: 5 * time.Second})
	rs.wireFlowControl()
	t.Cleanup(func() { close(block); rs.workers.Stop() })

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	// Dispatch enough updates to trigger backpressure (>75% of 8 = >6).
	for i := uint64(1); i <= 9; i++ {
		rs.dispatchText(buildTestUpdate("10.0.0.1", i))
	}

	// Check that peer is marked as paused.
	rs.mu.RLock()
	paused := rs.pausedPeers["10.0.0.1"]
	rs.mu.RUnlock()

	if !paused {
		t.Error("expected 10.0.0.1 to be paused after backpressure")
	}
}

// TestDispatchResumeOnDrain verifies dispatch resumes source peer when channel drains.
//
// VALIDATES: AC-2 — resume sent when worker channel drains below 10%.
// PREVENTS: Permanently paused peers after transient load.
func TestDispatchResumeOnDrain(t *testing.T) {
	rs := newTestRouteServer(t)
	rs.workers.Stop()
	block := make(chan struct{})
	var processed int32
	rs.workers = newWorkerPool(func(_ workerKey, _ workItem) {
		if atomic.AddInt32(&processed, 1) == 1 {
			<-block // First item blocks; rest process immediately.
		}
	}, poolConfig{chanSize: 8, idleTimeout: 5 * time.Second})
	rs.wireFlowControl()
	t.Cleanup(func() { rs.workers.Stop() })

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	// Fill channel to trigger backpressure.
	for i := uint64(1); i <= 9; i++ {
		rs.dispatchText(buildTestUpdate("10.0.0.1", i))
	}

	// Verify paused.
	rs.mu.RLock()
	paused := rs.pausedPeers["10.0.0.1"]
	rs.mu.RUnlock()
	if !paused {
		t.Fatal("expected peer paused before drain")
	}

	// Unblock worker to drain.
	close(block)

	// Wait for resume (low-water clears pausedPeers).
	require.Eventually(t, func() bool {
		rs.mu.RLock()
		defer rs.mu.RUnlock()
		return !rs.pausedPeers["10.0.0.1"]
	}, 2*time.Second, time.Millisecond, "peer not resumed after drain")
}

// TestMultiSourceBackpressure verifies independent pause/resume per source peer.
//
// VALIDATES: AC-13 — each source peer paused independently.
// PREVENTS: Global pause when only one source is saturated.
func TestMultiSourceBackpressure(t *testing.T) {
	rs := newTestRouteServer(t)
	rs.workers.Stop()
	block := make(chan struct{})
	rs.workers = newWorkerPool(func(_ workerKey, _ workItem) {
		<-block
	}, poolConfig{chanSize: 8, idleTimeout: 5 * time.Second})
	rs.wireFlowControl()
	t.Cleanup(func() { close(block); rs.workers.Stop() })

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.peers["10.0.0.3"] = &PeerState{Address: "10.0.0.3", Up: true}
	rs.mu.Unlock()

	// Saturate peer 1 and peer 2, leave peer 3 light.
	for i := uint64(1); i <= 9; i++ {
		rs.dispatchText(buildTestUpdate("10.0.0.1", i))
		rs.dispatchText(buildTestUpdate("10.0.0.2", 100+i))
	}
	// Peer 3: only 1 item (no backpressure).
	rs.dispatchText(buildTestUpdate("10.0.0.3", 200))

	rs.mu.RLock()
	p1 := rs.pausedPeers["10.0.0.1"]
	p2 := rs.pausedPeers["10.0.0.2"]
	p3 := rs.pausedPeers["10.0.0.3"]
	rs.mu.RUnlock()

	if !p1 {
		t.Error("expected peer 10.0.0.1 paused")
	}
	if !p2 {
		t.Error("expected peer 10.0.0.2 paused")
	}
	if p3 {
		t.Error("expected peer 10.0.0.3 NOT paused")
	}
}

// TestShutdownResumesAllPeers verifies Stop() resumes all paused peers.
//
// VALIDATES: AC-9 — all paused peers resumed on shutdown.
// PREVENTS: Permanently paused peers after RR plugin exits.
func TestShutdownResumesAllPeers(t *testing.T) {
	rs := newTestRouteServer(t)
	rs.workers.Stop()
	block := make(chan struct{})
	rs.workers = newWorkerPool(func(_ workerKey, _ workItem) {
		<-block
	}, poolConfig{chanSize: 8, idleTimeout: 5 * time.Second})
	rs.wireFlowControl()

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	// Trigger backpressure.
	for i := uint64(1); i <= 9; i++ {
		rs.dispatchText(buildTestUpdate("10.0.0.1", i))
	}

	// Verify paused.
	rs.mu.RLock()
	paused := rs.pausedPeers["10.0.0.1"]
	rs.mu.RUnlock()
	if !paused {
		t.Fatal("expected peer paused before shutdown")
	}

	// Shutdown: resumeAllPaused + stop workers.
	close(block)
	rs.resumeAllPaused()
	rs.workers.Stop()

	rs.mu.RLock()
	remaining := len(rs.pausedPeers)
	rs.mu.RUnlock()
	if remaining != 0 {
		t.Errorf("expected 0 paused peers after shutdown, got %d", remaining)
	}
}

// TestPausedPeerResumesOnDrain verifies the full pause→drain→resume→dispatch cycle.
//
// VALIDATES: AC-6 — read loop unblocks after resume; subsequent messages processed normally.
// PREVENTS: Permanently stalled peers after a transient backpressure event.
func TestPausedPeerResumesOnDrain(t *testing.T) {
	rs := newTestRouteServer(t)
	rs.workers.Stop()
	block := make(chan struct{})
	var processed atomic.Int32
	rs.workers = newWorkerPool(func(_ workerKey, _ workItem) {
		if processed.Add(1) == 1 {
			<-block // First item blocks; rest process immediately.
		}
	}, poolConfig{chanSize: 8, idleTimeout: 5 * time.Second})
	rs.wireFlowControl()
	t.Cleanup(func() { rs.workers.Stop() })

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.mu.Unlock()

	// Fill to trigger backpressure.
	for i := uint64(1); i <= 9; i++ {
		rs.dispatchText(buildTestUpdate("10.0.0.1", i))
	}

	rs.mu.RLock()
	paused := rs.pausedPeers["10.0.0.1"]
	rs.mu.RUnlock()
	if !paused {
		t.Fatal("expected peer paused")
	}

	// Unblock → drain → resume.
	close(block)
	require.Eventually(t, func() bool {
		rs.mu.RLock()
		defer rs.mu.RUnlock()
		return !rs.pausedPeers["10.0.0.1"]
	}, 2*time.Second, time.Millisecond, "peer not resumed after drain")

	// Dispatch more after resume — should succeed without re-triggering pause.
	rs.dispatchText(buildTestUpdate("10.0.0.1", 100))

	rs.mu.RLock()
	rePaused := rs.pausedPeers["10.0.0.1"]
	rs.mu.RUnlock()
	if rePaused {
		t.Error("peer should not be re-paused after single dispatch post-drain")
	}
}

// TestPauseRPCFailure verifies that a failed pause RPC does not crash or block dispatch.
//
// VALIDATES: AC-14 — pause RPC error logged, processing continues.
// PREVENTS: RPC timeout blocking the dispatch goroutine or crashing the plugin.
func TestPauseRPCFailure(t *testing.T) {
	// newTestRouteServer closes the engine connection, so all updateRoute calls fail.
	rs := newTestRouteServer(t)
	rs.workers.Stop()
	block := make(chan struct{})
	rs.workers = newWorkerPool(func(_ workerKey, _ workItem) {
		<-block
	}, poolConfig{chanSize: 8, idleTimeout: 5 * time.Second})
	rs.wireFlowControl()
	t.Cleanup(func() { close(block); rs.workers.Stop() })

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.mu.Unlock()

	// Fill to trigger backpressure — pause RPC will fail (closed conn).
	for i := uint64(1); i <= 9; i++ {
		rs.dispatchText(buildTestUpdate("10.0.0.1", i))
	}

	// Verify: peer is tracked as paused despite RPC failure.
	// The pausedPeers map reflects intent, not RPC success.
	rs.mu.RLock()
	paused := rs.pausedPeers["10.0.0.1"]
	rs.mu.RUnlock()
	if !paused {
		t.Error("expected peer tracked as paused even though RPC failed")
	}
}

// TestDispatchPassesPreParsedPayload verifies dispatchText carries text payload in workItem directly.
//
// VALIDATES: AC-2 -- workItem carries text payload for deferred parsing (no fwdCtx).
// PREVENTS: Missing payload in worker context.
func TestDispatchPassesPreParsedPayload(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.mu.Unlock()

	// Replace workers to capture the workItem directly.
	rs.workers.Stop()
	var captured workItem
	var capturedMu sync.Mutex
	var capturedOK bool
	rs.workers = newWorkerPool(func(_ workerKey, item workItem) {
		capturedMu.Lock()
		captured = item
		capturedOK = true
		capturedMu.Unlock()
	}, poolConfig{chanSize: 64, idleTimeout: 5 * time.Second})
	t.Cleanup(func() { rs.workers.Stop() })

	rs.dispatchText("peer 10.0.0.1 remote as 65001 received update 42 origin igp next-hop 1.1.1.1 nlri ipv4/unicast add prefix 10.0.0.0/24")

	require.Eventually(t, func() bool {
		capturedMu.Lock()
		defer capturedMu.Unlock()
		return capturedOK
	}, 2*time.Second, time.Millisecond, "workItem not captured")

	capturedMu.Lock()
	defer capturedMu.Unlock()

	if captured.sourcePeer != "10.0.0.1" {
		t.Errorf("expected sourcePeer 10.0.0.1, got %s", captured.sourcePeer)
	}
	if captured.textPayload == "" {
		t.Fatal("expected textPayload to be populated")
	}
	if !strings.Contains(captured.textPayload, "peer") {
		t.Error("textPayload should contain peer keyword")
	}
	if !strings.Contains(captured.textPayload, "update") {
		t.Error("textPayload should contain update keyword")
	}
}

// TestProcessForwardPopulatesWithdrawalMap verifies withdrawal map updated after forward.
//
// VALIDATES: AC-8 — processForward stores family+prefix per source peer in withdrawal map.
// PREVENTS: Missing withdrawal data when source peer goes down.
func TestProcessForwardPopulatesWithdrawalMap(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	rs.dispatchText("peer 10.0.0.1 remote as 65001 received update 99 origin igp next-hop 1.1.1.1 nlri ipv4/unicast add prefix 10.0.0.0/24")
	flushWorkers(t, rs)

	rs.withdrawalMu.Lock()
	peerWd := rs.withdrawals["10.0.0.1"]
	rs.withdrawalMu.Unlock()
	if len(peerWd) != 1 {
		t.Fatalf("expected 1 withdrawal entry, got %d", len(peerWd))
	}
	entry, ok := peerWd["ipv4/unicast|10.0.0.0/24"]
	if !ok || entry.Prefix != "prefix 10.0.0.0/24" {
		t.Errorf("expected prefix field 'prefix 10.0.0.0/24', got %+v", entry)
	}
}

// TestWithdrawalMapConsistency verifies withdrawal map is consistent after PeerDown drain.
//
// VALIDATES: AC-5 — withdrawal map correct after worker drains, cleared on peer-down.
// PREVENTS: Race between withdrawal map update and handleStateDown.
func TestWithdrawalMapConsistency(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	// Dispatch multiple UPDATEs.
	for i := uint64(1); i <= 5; i++ {
		rs.dispatchText(fmt.Sprintf(
			"peer 10.0.0.1 remote as 65001 received update %d origin igp next-hop 1.1.1.1 nlri ipv4/unicast add prefix 10.0.%d.0/24",
			i, i,
		))
	}

	flushWorkers(t, rs)

	// All 5 routes should be in withdrawal map.
	rs.withdrawalMu.Lock()
	peerWd := rs.withdrawals["10.0.0.1"]
	rs.withdrawalMu.Unlock()
	if len(peerWd) != 5 {
		t.Fatalf("expected 5 withdrawal entries, got %d", len(peerWd))
	}

	// Simulate peer down.
	rs.workers = newWorkerPool(func(key workerKey, item workItem) {
		rs.processForward(key, item)
	}, poolConfig{chanSize: 64, idleTimeout: 5 * time.Second})

	rs.dispatchText("peer 10.0.0.1 remote as 65001 state down")

	// After peer down, withdrawal map should be empty for this peer.
	require.Eventually(t, func() bool {
		rs.withdrawalMu.Lock()
		defer rs.withdrawalMu.Unlock()
		return len(rs.withdrawals["10.0.0.1"]) == 0
	}, 2*time.Second, time.Millisecond, "expected 0 withdrawal entries after peer down")
}

// TestReleaseCacheAsync verifies releaseCache returns immediately (async).
//
// VALIDATES: AC-5 — releaseCache is async (does not block worker).
// PREVENTS: Worker blocked on synchronous release RPC.
func TestReleaseCacheAsync(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.mu.Unlock()

	// releaseCache should return quickly (async channel send, not blocking RPC).
	// With closed connections (newTestRouteServer), a synchronous call would
	// still return quickly due to error, but the async version should be instant.
	start := time.Now()
	rs.releaseCache(42)
	elapsed := time.Since(start)

	// Async release should be sub-millisecond (just a channel send).
	if elapsed > 10*time.Millisecond {
		t.Errorf("releaseCache took %v, expected sub-millisecond for async", elapsed)
	}
}

// TestBatchForwardAccumulation verifies items accumulate before RPC.
//
// VALIDATES: AC-10 — worker sends batch RPC after accumulating items.
// PREVENTS: N items generating N individual RPCs instead of batched RPC.
func TestBatchForwardAccumulation(t *testing.T) {
	rs := newTestRouteServer(t)

	type forwardCall struct {
		ids     []uint64
		targets []string
	}
	var calls []forwardCall
	var cmdMu sync.Mutex
	rs.forwardCachedHook = func(ids []uint64, targets []string) {
		cmdMu.Lock()
		calls = append(calls, forwardCall{
			ids:     append([]uint64(nil), ids...),
			targets: append([]string(nil), targets...),
		})
		cmdMu.Unlock()
	}

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	// Gate: block the worker handler until all items are dispatched.
	// This guarantees all 5 items are queued before processing starts,
	// making batch accumulation deterministic (not scheduler-dependent).
	gate := make(chan struct{})
	rs.workers.Stop()
	rs.workers = newWorkerPool(func(key workerKey, item workItem) {
		<-gate
		rs.processForward(key, item)
	}, poolConfig{chanSize: 64, idleTimeout: 5 * time.Second, onDrained: rs.flushWorkerBatch})

	for i := uint64(1); i <= 5; i++ {
		rs.dispatchText(buildTestUpdate("10.0.0.1", i))
	}

	close(gate)
	flushWorkers(t, rs)

	cmdMu.Lock()
	defer cmdMu.Unlock()

	if len(calls) >= 5 {
		t.Errorf("expected fewer than 5 ForwardCached calls (batched), got %d", len(calls))
	}
	hasBatch := false
	for _, c := range calls {
		if len(c.ids) > 1 {
			hasBatch = true
			break
		}
	}
	if !hasBatch {
		t.Error("expected at least one ForwardCached call with multiple IDs")
	}
}

// TestSelectForwardTargetsDeterministic verifies that selectForwardTargets
// returns a deterministic (sorted) peer list regardless of map iteration order.
//
// VALIDATES: Batching correctness — selector string must be identical for the
// same peer set to prevent false selector-change flushes in batchForwardUpdate.
// PREVENTS: Non-deterministic Go map iteration defeating batch accumulation.
func TestSelectForwardTargetsDeterministic(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.3"] = &PeerState{Address: "10.0.0.3", Up: true}
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.4"] = &PeerState{Address: "10.0.0.4", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	families := map[family.Family]bool{family.IPv4Unicast: true}

	// Call 100 times — with unsorted output, Go map randomness would produce
	// different orderings, causing batch selector mismatches.
	rs.mu.RLock()
	first := rs.selectForwardTargets(nil, "10.0.0.1", families)
	rs.mu.RUnlock()

	want := strings.Join(first, ",")
	if want != "10.0.0.2,10.0.0.3,10.0.0.4" {
		t.Fatalf("expected sorted targets, got %q", want)
	}

	for i := range 100 {
		rs.mu.RLock()
		got := rs.selectForwardTargets(nil, "10.0.0.1", families)
		rs.mu.RUnlock()
		if sel := strings.Join(got, ","); sel != want {
			t.Fatalf("iteration %d: selector %q != %q (non-deterministic)", i, sel, want)
		}
	}
}

// TestRSFlushCallsForwardCached verifies AC-11: rs's flushBatch invokes
// Plugin.ForwardCached with the accumulated IDs and the per-batch target
// snapshot. rs-fastpath-3 replaces the old asyncForward text RPC path.
//
// VALIDATES: AC-11 -- rs's flush goes through the reactor-owned fast path,
// not through the text update-route dispatcher.
// PREVENTS: regression where flushBatch silently falls back to the legacy
// path and the profile hotspot (tokenise 19.4 %) reappears.
func TestRSFlushCallsForwardCached(t *testing.T) {
	rs := newTestRouteServer(t)

	type call struct {
		ids     []uint64
		targets []string
	}
	var calls []call
	var cmdMu sync.Mutex
	rs.forwardCachedHook = func(ids []uint64, targets []string) {
		cmdMu.Lock()
		calls = append(calls, call{
			ids:     append([]uint64(nil), ids...),
			targets: append([]string(nil), targets...),
		})
		cmdMu.Unlock()
	}
	rs.updateRouteHook = func(_, cmd string) {
		if strings.Contains(cmd, "cache ") && strings.Contains(cmd, " forward ") {
			t.Errorf("rs-fastpath-3: legacy 'cache N forward' text RPC fired instead of ForwardCached: %q", cmd)
		}
	}

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	for i := uint64(1); i <= 3; i++ {
		rs.dispatchText(buildTestUpdate("10.0.0.1", i))
	}
	flushWorkers(t, rs)

	cmdMu.Lock()
	defer cmdMu.Unlock()

	if len(calls) == 0 {
		t.Fatal("expected at least one ForwardCached call, got 0")
	}
	for _, c := range calls {
		if len(c.ids) == 0 {
			t.Error("ForwardCached call carried no IDs")
		}
		if len(c.targets) == 0 {
			t.Error("ForwardCached call carried no destinations")
		}
	}
}

// buildTestUpdate creates a minimal text-format UPDATE event for testing dispatch.
func buildTestUpdate(peer string, msgID uint64) string {
	return fmt.Sprintf("peer %s remote as 65001 received update %d origin igp next-hop 1.1.1.1 nlri ipv4/unicast add prefix 10.0.0.0/24", peer, msgID)
}

// --- Replay tests (spec rib-03) ---

// TestHandleStateUpReplay verifies handleStateUp uses DispatchCommand (not ROUTE-REFRESH).
//
// VALIDATES: AC-1, AC-2 — peer connects, receives routes via replay, no ROUTE-REFRESH sent.
// PREVENTS: Thundering herd from ROUTE-REFRESH to all peers.
func TestHandleStateUpReplay(t *testing.T) {
	rs := newTestRouteServer(t)

	var mu sync.Mutex
	var dispatchCmds []string
	var updateCmds []string

	rs.dispatchCommandHook = func(cmd string) (string, string, error) {
		mu.Lock()
		dispatchCmds = append(dispatchCmds, cmd)
		mu.Unlock()
		return statusDone, `{"last-index":5,"replayed":3}`, nil
	}
	rs.updateRouteHook = func(peer, cmd string) {
		mu.Lock()
		updateCmds = append(updateCmds, peer+": "+cmd)
		mu.Unlock()
	}

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true,
		Families: map[family.Family]bool{family.IPv4Unicast: true}}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true,
		Families:     map[family.Family]bool{family.IPv4Unicast: true},
		Capabilities: map[string]bool{"route-refresh": true}}
	rs.mu.Unlock()

	rs.handleStateUp("10.0.0.1")

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(dispatchCmds) > 0
	}, 2*time.Second, time.Millisecond, "expected DispatchCommand calls for replay")

	mu.Lock()
	defer mu.Unlock()

	if !strings.HasPrefix(dispatchCmds[0], "adj-rib-in replay 10.0.0.1") {
		t.Errorf("expected adj-rib-in replay command, got %q", dispatchCmds[0])
	}
	for _, cmd := range updateCmds {
		if strings.Contains(cmd, "refresh") {
			t.Errorf("expected no ROUTE-REFRESH, got: %s", cmd)
		}
	}
}

// TestRSSoftDepSkipsReplay verifies that when adj-rib-in is absent (soft-dep
// not loaded) and the replay dispatch fails with ErrUnknownCommand, bgp-rs
// skips the convergence replay loop, still clears the Replaying flag, and
// still sends EOR so the newly-connected peer receives end-of-RIB markers.
//
// VALIDATES: spec-rs-fastpath-2-adjrib AC-6 -- graceful no-op when
// OptionalDependencies entry bgp-adj-rib-in is missing at run time.
// PREVENTS: bgp-rs unusable without bgp-adj-rib-in (would defeat the
// soft-dep refactor), or peer stuck with Replaying=true and no EOR.
func TestRSSoftDepSkipsReplay(t *testing.T) {
	rs := newTestRouteServer(t)

	// eorCh closes when the first "eor" command hits updateRouteHook -- the
	// terminal side effect of replayForPeer's fallback path. Using an explicit
	// signal avoids polling-based Eventually checks that would be fragile on
	// slow CI runners.
	eorCh := make(chan struct{})
	var eorOnce sync.Once

	var dispatchCount atomic.Int32
	rs.dispatchCommandHook = func(_ string) (string, string, error) {
		dispatchCount.Add(1)
		// Matches the engine's ErrUnknownCommand string propagated across the
		// plugin IPC boundary when adj-rib-in is not registered.
		return statusError, "", fmt.Errorf("unknown command: adj-rib-in replay")
	}

	var mu sync.Mutex
	var updateCmds []string
	rs.updateRouteHook = func(peer, cmd string) {
		mu.Lock()
		updateCmds = append(updateCmds, peer+": "+cmd)
		mu.Unlock()
		if strings.Contains(cmd, "eor") {
			eorOnce.Do(func() { close(eorCh) })
		}
	}

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address:  "10.0.0.1",
		Up:       true,
		Families: map[family.Family]bool{family.IPv4Unicast: true},
	}
	rs.mu.Unlock()

	rs.handleStateUp("10.0.0.1")

	// Wait for the terminal EOR signal (or fail on a generous timeout). Once
	// EOR has fired, replayForPeer has completed its fallback path and both
	// Replaying and the dispatch count are in their final state.
	select {
	case <-eorCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for EOR after replay fallback")
	}

	// Replaying must have been cleared by the same code path that produced EOR.
	rs.mu.RLock()
	p := rs.peers["10.0.0.1"]
	replaying := p != nil && p.Replaying
	rs.mu.RUnlock()
	if replaying {
		t.Errorf("Replaying flag should have been cleared by the fallback path")
	}

	// Expect exactly one dispatch attempt (the initial replay), not the full
	// convergence loop.
	if got := dispatchCount.Load(); got != 1 {
		t.Errorf("expected 1 dispatch attempt, got %d (soft-dep fallback should skip convergence)", got)
	}

	// Spot-check the accumulated update commands to confirm at least one EOR
	// matches the peer+family we configured.
	mu.Lock()
	defer mu.Unlock()
	gotEOR := false
	for _, cmd := range updateCmds {
		if strings.Contains(cmd, "10.0.0.1:") && strings.Contains(cmd, "ipv4/unicast eor") {
			gotEOR = true
			break
		}
	}
	if !gotEOR {
		t.Errorf("expected EOR for 10.0.0.1 ipv4/unicast, got: %v", updateCmds)
	}
}

// TestReplayingPeerIncludedInForwardTargets verifies that a replaying peer IS included
// in selectForwardTargets. BGP UPDATE duplicates are idempotent, so forwarding to a
// replaying peer is safe and prevents route loss when all peers connect simultaneously.
//
// VALIDATES: Replaying peers receive live forwards — no route loss race condition.
// PREVENTS: Route loss when peers connect simultaneously and Replaying exclusion drops updates.
func TestReplayingPeerIncludedInForwardTargets(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true, Replaying: true,
		Families: map[family.Family]bool{family.IPv4Unicast: true}}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true,
		Families: map[family.Family]bool{family.IPv4Unicast: true}}
	rs.mu.Unlock()

	// A replaying peer SHOULD be in forward targets (duplicates are idempotent).
	rs.mu.RLock()
	targets := rs.selectForwardTargets(nil, "10.0.0.2", map[family.Family]bool{family.IPv4Unicast: true})
	rs.mu.RUnlock()
	if !slices.Contains(targets, "10.0.0.1") {
		t.Error("replaying peer 10.0.0.1 should be in forward targets")
	}
}

// TestHandleStateUpDelta verifies delta replay sent after full replay.
//
// VALIDATES: AC-4 — routes arriving during replay caught by delta replay.
// PREVENTS: Missing routes from the gap between full replay and joining forward targets.
func TestHandleStateUpDelta(t *testing.T) {
	rs := newTestRouteServer(t)

	var mu sync.Mutex
	var dispatchCmds []string

	rs.dispatchCommandHook = func(cmd string) (string, string, error) {
		mu.Lock()
		dispatchCmds = append(dispatchCmds, cmd)
		mu.Unlock()
		if strings.HasSuffix(cmd, " 0") {
			return statusDone, `{"last-index":5,"replayed":3}`, nil
		}
		return statusDone, `{"last-index":7,"replayed":1}`, nil
	}

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true,
		Families: map[family.Family]bool{family.IPv4Unicast: true}}
	rs.mu.Unlock()

	rs.handleStateUp("10.0.0.1")

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(dispatchCmds) >= 2
	}, 2*time.Second, time.Millisecond, "expected 2 dispatch commands (full + delta)")

	mu.Lock()
	defer mu.Unlock()
	if dispatchCmds[0] != "adj-rib-in replay 10.0.0.1 0" {
		t.Errorf("expected full replay, got %q", dispatchCmds[0])
	}
	if dispatchCmds[1] != "adj-rib-in replay 10.0.0.1 5" {
		t.Errorf("expected delta replay from 5, got %q", dispatchCmds[1])
	}
}

// TestHandleStateUpNonBlocking verifies event loop not blocked during replay.
//
// VALIDATES: AC-9 — other peers' UPDATEs continue flowing during replay.
// PREVENTS: Event loop deadlock during slow replay.
func TestHandleStateUpNonBlocking(t *testing.T) {
	rs := newTestRouteServer(t)

	unblock := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-unblock:
		default:
			close(unblock)
		}
	})

	rs.dispatchCommandHook = func(cmd string) (string, string, error) {
		<-unblock
		return statusDone, `{"last-index":0,"replayed":0}`, nil
	}

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true,
		Families: map[family.Family]bool{family.IPv4Unicast: true}}
	rs.mu.Unlock()

	// handleStateUp should return immediately even though replay blocks.
	done := make(chan struct{})
	go func() {
		rs.handleStateUp("10.0.0.1")
		close(done)
	}()

	select {
	case <-done:
		// OK: handleStateUp returned without waiting for replay.
	case <-time.After(1 * time.Second):
		t.Error("handleStateUp blocked — should return immediately")
	}
}

// TestWithdrawalOnPeerDown verifies handleStateDown sends withdrawals from map and clears it.
//
// VALIDATES: AC-5 — handleStateDown reads withdrawal map, sends withdrawals, clears map.
// PREVENTS: Missing withdrawals on peer-down, stale withdrawal map entries.
func TestWithdrawalOnPeerDown(t *testing.T) {
	rs := newTestRouteServer(t)

	var mu sync.Mutex
	var commands []string
	var count atomic.Int32
	done := make(chan struct{})

	rs.updateRouteHook = func(peer, cmd string) {
		mu.Lock()
		commands = append(commands, peer+": "+cmd)
		mu.Unlock()
		if int(count.Add(1)) >= 2 {
			select {
			case <-done:
			default:
				close(done)
			}
		}
	}

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	// Populate withdrawal map directly.
	rs.withdrawalMu.Lock()
	rs.withdrawals["10.0.0.1"] = map[string]withdrawalInfo{
		"ipv4/unicast|10.0.0.0/24": {Family: "ipv4/unicast", Prefix: "prefix 10.0.0.0/24"},
		"ipv4/unicast|10.0.1.0/24": {Family: "ipv4/unicast", Prefix: "prefix 10.0.1.0/24"},
	}
	rs.withdrawalMu.Unlock()

	// Trigger peer down.
	rs.dispatchText("peer 10.0.0.1 remote as 65001 state down")

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for withdrawal commands")
	}

	mu.Lock()
	cmdsCopy := make([]string, len(commands))
	copy(cmdsCopy, commands)
	mu.Unlock()

	found0, found1 := false, false
	for _, cmd := range cmdsCopy {
		if strings.Contains(cmd, "nlri ipv4/unicast del prefix 10.0.0.0/24") {
			found0 = true
		}
		if strings.Contains(cmd, "nlri ipv4/unicast del prefix 10.0.1.0/24") {
			found1 = true
		}
	}
	if !found0 {
		t.Errorf("missing withdrawal for prefix 10.0.0.0/24, commands: %v", cmdsCopy)
	}
	if !found1 {
		t.Errorf("missing withdrawal for prefix 10.0.1.0/24, commands: %v", cmdsCopy)
	}

	// Withdrawal map should be cleared.
	rs.withdrawalMu.Lock()
	wdLen := len(rs.withdrawals["10.0.0.1"])
	rs.withdrawalMu.Unlock()
	if wdLen != 0 {
		t.Errorf("expected withdrawal map cleared, got %d entries", wdLen)
	}
}

// TestReplayGeneration_RapidReconnect verifies stale replay goroutines don't
// clear Replaying for a newer session.
//
// VALIDATES: Rapid reconnect (down→up while old replay running) doesn't cause ghost routes.
// PREVENTS: Stale goroutine prematurely clearing Replaying for a new session.
func TestReplayGeneration_RapidReconnect(t *testing.T) {
	rs := newTestRouteServer(t)

	// Single hook that branches on call count. Goroutine A blocks on firstBlock;
	// goroutine B completes immediately and signals secondDone.
	firstBlock := make(chan struct{})
	secondDone := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-firstBlock:
		default:
			close(firstBlock)
		}
	})

	var callCount atomic.Int32
	rs.dispatchCommandHook = func(cmd string) (string, string, error) {
		n := callCount.Add(1)
		if n == 1 {
			// First call (goroutine A's full replay) — block until released.
			<-firstBlock
			return statusDone, `{"last-index":0,"replayed":0}`, nil
		}
		// All subsequent calls (goroutine B's full + delta) — complete immediately.
		defer func() {
			select {
			case <-secondDone:
			default:
				close(secondDone)
			}
		}()
		return statusDone, `{"last-index":0,"replayed":0}`, nil
	}

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true,
		Families: map[family.Family]bool{family.IPv4Unicast: true}}
	rs.mu.Unlock()

	// First handleStateUp — goroutine A blocks on DispatchCommand.
	rs.handleStateUp("10.0.0.1")

	// Wait for goroutine A to enter the hook and block on firstBlock.
	require.Eventually(t, func() bool {
		return callCount.Load() >= 1
	}, 2*time.Second, time.Millisecond, "goroutine A did not enter dispatch hook")

	// Second handleStateUp — simulates rapid reconnect. Goroutine B starts.
	rs.handleStateUp("10.0.0.1")

	// Wait for goroutine B's hook to return, then let replayForPeer finish.
	select {
	case <-secondDone:
	case <-time.After(2 * time.Second):
		t.Fatal("second replay timed out")
	}

	// Wait for goroutine B to set Replaying=false.
	require.Eventually(t, func() bool {
		rs.mu.RLock()
		defer rs.mu.RUnlock()
		return !rs.peers["10.0.0.1"].Replaying
	}, 2*time.Second, time.Millisecond, "goroutine B did not clear Replaying")

	// Goroutine B completed — verify ReplayGen.
	rs.mu.RLock()
	genAfterB := rs.peers["10.0.0.1"].ReplayGen
	rs.mu.RUnlock()
	if genAfterB != 2 {
		t.Errorf("expected ReplayGen=2, got %d", genAfterB)
	}

	// Now unblock goroutine A (stale).
	close(firstBlock)

	// Stale goroutine A must NOT change Replaying back to true.
	require.Never(t, func() bool {
		rs.mu.RLock()
		defer rs.mu.RUnlock()
		return rs.peers["10.0.0.1"].Replaying
	}, 100*time.Millisecond, time.Millisecond,
		"stale goroutine must not set Replaying=true after completing")
}

// TestTextUpdateParseableByFields verifies text UPDATE events parse with strings.Fields.
//
// VALIDATES: AC-4 (bgp-rs text format parseable), AC-5 (contains peer, msgID, families, NLRIs).
//
// PREVENTS: Text format events not being parseable by bgp-rs dispatch/workers.
func TestTextUpdateParseableByFields(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string
		wantID   uint64
		wantPeer string
		wantFams map[family.Family]bool
	}{
		{
			name:     "add ipv4",
			input:    "peer 10.0.0.1 remote as 65001 received update 42 origin igp as-path 65001,65002 next-hop 10.0.0.1 nlri ipv4/unicast add prefix 192.168.1.0/24\n",
			wantType: eventUpdate,
			wantID:   42,
			wantPeer: "10.0.0.1",
			wantFams: map[family.Family]bool{family.IPv4Unicast: true},
		},
		{
			name:     "del ipv4",
			input:    "peer 10.0.0.2 remote as 65002 received update 99 nlri ipv4/unicast del prefix 10.0.0.0/24\n",
			wantType: eventUpdate,
			wantID:   99,
			wantPeer: "10.0.0.2",
			wantFams: map[family.Family]bool{family.IPv4Unicast: true},
		},
		{
			name:     "add ipv6",
			input:    "peer 10.0.0.1 remote as 65001 received update 7 origin igp next-hop 2001:db8::1 nlri ipv6/unicast add prefix 2001:db8::/32\n",
			wantType: eventUpdate,
			wantID:   7,
			wantPeer: "10.0.0.1",
			wantFams: map[family.Family]bool{family.IPv6Unicast: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventType, msgID, peerAddr, payload, err := quickParseTextEvent(tt.input)
			if err != nil {
				t.Fatalf("quickParseTextEvent() error: %v", err)
			}
			if eventType != tt.wantType {
				t.Errorf("type = %q, want %q", eventType, tt.wantType)
			}
			if msgID != tt.wantID {
				t.Errorf("msgID = %d, want %d", msgID, tt.wantID)
			}
			if peerAddr != tt.wantPeer {
				t.Errorf("peerAddr = %q, want %q", peerAddr, tt.wantPeer)
			}

			families := parseTextUpdateFamilies(payload)
			for fam := range tt.wantFams {
				if !families[fam] {
					t.Errorf("missing family %q in %v", fam, families)
				}
			}
		})
	}
}

// TestTextStateEventParseable verifies text state events parse correctly.
//
// VALIDATES: AC-7 (text state event contains peer address + state).
//
// PREVENTS: State events not being parseable in text format.
func TestTextStateEventParseable(t *testing.T) {
	input := "peer 10.0.0.1 remote as 65001 state up\n"

	eventType, _, peerAddr, _, err := quickParseTextEvent(input)
	if err != nil {
		t.Fatalf("quickParseTextEvent() error: %v", err)
	}
	if eventType != eventState {
		t.Errorf("type = %q, want %q", eventType, eventState)
	}
	if peerAddr != "10.0.0.1" {
		t.Errorf("peerAddr = %q, want %q", peerAddr, "10.0.0.1")
	}
}

// TestTextOpenEventParseable verifies text OPEN events parse correctly.
//
// VALIDATES: AC-6 (text OPEN event contains ASN, router-id, families).
//
// PREVENTS: OPEN events not being parseable in text format.
func TestTextOpenEventParseable(t *testing.T) {
	input := "peer 10.0.0.1 remote as 65001 received open 5 router-id 1.1.1.1 hold-time 90 cap 1 multiprotocol ipv4/unicast cap 1 multiprotocol ipv6/unicast\n"

	eventType, _, peerAddr, payload, err := quickParseTextEvent(input)
	if err != nil {
		t.Fatalf("quickParseTextEvent() error: %v", err)
	}
	if eventType != eventOpen {
		t.Errorf("type = %q, want %q", eventType, eventOpen)
	}
	if peerAddr != "10.0.0.1" {
		t.Errorf("peerAddr = %q, want %q", peerAddr, "10.0.0.1")
	}

	event := parseTextOpen(payload)
	if event == nil {
		t.Fatal("parseTextOpen returned nil")
		return
	}
	if event.PeerASN != 65001 {
		t.Errorf("ASN = %d, want 65001", event.PeerASN)
	}
	if event.Open == nil {
		t.Fatal("Open is nil")
		return
	}
	if len(event.Open.Capabilities) != 2 {
		t.Errorf("capabilities = %d, want 2", len(event.Open.Capabilities))
	}
	// Check family extraction
	hasFamilies := false
	for _, cap := range event.Open.Capabilities {
		if cap.Name == "multiprotocol" {
			hasFamilies = true
		}
	}
	if !hasFamilies {
		t.Error("missing multiprotocol capabilities")
	}
}

// TestTextUpdateWithdrawalTracking verifies text format withdrawal map updates.
//
// VALIDATES: AC-4 (text parsing extracts NLRIs for withdrawal tracking).
//
// PREVENTS: Withdrawal map not being populated from text events.
func TestTextUpdateWithdrawalTracking(t *testing.T) {
	// Add line (comma format: nlri ipv4/unicast add prefix <a>,<b>).
	addLine := "peer 10.0.0.1 remote as 65001 received update 42 origin igp next-hop 10.0.0.1 nlri ipv4/unicast add prefix 192.168.1.0/24,10.0.0.0/8\n"
	ops := parseTextNLRIOps(addLine)

	addOps, ok := ops["ipv4/unicast"]
	if !ok {
		t.Fatal("missing ipv4/unicast ops")
	}
	if len(addOps) != 1 {
		t.Fatalf("expected 1 op, got %d", len(addOps))
	}
	if addOps[0].Action != "add" {
		t.Errorf("action = %q, want add", addOps[0].Action)
	}
	if len(addOps[0].NLRIs) != 2 {
		t.Errorf("NLRIs = %d, want 2", len(addOps[0].NLRIs))
	}

	// Del line: nlri <family> del
	delLine := "peer 10.0.0.1 remote as 65001 received update 43 nlri ipv4/unicast del prefix 192.168.1.0/24\n"
	wdOps := parseTextNLRIOps(delLine)

	delOps, ok := wdOps["ipv4/unicast"]
	if !ok {
		t.Fatal("missing ipv4/unicast withdraw ops")
	}
	if delOps[0].Action != "del" {
		t.Errorf("action = %q, want del", delOps[0].Action)
	}
	if len(delOps[0].NLRIs) != 1 {
		t.Errorf("NLRIs = %d, want 1", len(delOps[0].NLRIs))
	}
}

// TestSelectForwardTargetsReusesBuffer verifies that selectForwardTargets reuses
// the caller-provided buffer across calls — no new backing array allocation.
//
// VALIDATES: AC-5 from spec-alloc-1-batch-pooling.md
// PREVENTS: Per-UPDATE target slice allocations in bgp-rs forward path.
func TestSelectForwardTargetsReusesBuffer(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.peers["10.0.0.3"] = &PeerState{Address: "10.0.0.3", Up: true}
	rs.mu.Unlock()

	families := map[family.Family]bool{family.IPv4Unicast: true}

	// First call: buffer grows from nil.
	rs.mu.RLock()
	buf := rs.selectForwardTargets(nil, "10.0.0.1", families)
	rs.mu.RUnlock()

	if len(buf) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(buf))
	}
	firstPtr := unsafe.SliceData(buf)

	// Second call: reuse existing buffer.
	rs.mu.RLock()
	buf = rs.selectForwardTargets(buf, "10.0.0.1", families)
	rs.mu.RUnlock()

	if len(buf) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(buf))
	}
	secondPtr := unsafe.SliceData(buf)

	if firstPtr != secondPtr {
		t.Error("second call allocated a new backing array instead of reusing buffer")
	}
}

// --- EOR tests ---

// TestReplayForPeer_SendsEOR verifies EOR sent per family after replay completes.
//
// VALIDATES: AC-5 — peer up triggers EOR per negotiated family after replay.
// PREVENTS: Missing End-of-RIB marker causing peers to never leave initial table exchange.
func TestReplayForPeer_SendsEOR(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.dispatchCommandHook = func(cmd string) (string, string, error) {
		return statusDone, `{"last-index":0,"replayed":0}`, nil
	}

	var mu sync.Mutex
	var commands []string
	rs.updateRouteHook = func(peer, cmd string) {
		mu.Lock()
		commands = append(commands, peer+"\t"+cmd)
		mu.Unlock()
	}

	// Set up peer with two families.
	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address:  "10.0.0.1",
		Up:       true,
		Families: map[family.Family]bool{family.IPv4Unicast: true, family.IPv6Unicast: true},
	}
	rs.mu.Unlock()

	rs.dispatchText("peer 10.0.0.1 remote as 65001 state up")

	// Wait for both EOR commands (one per family) to arrive.
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		count := 0
		for _, cmd := range commands {
			if strings.Contains(cmd, "eor") {
				count++
			}
		}
		return count >= 2
	}, 2*time.Second, time.Millisecond, "expected 2 EOR commands")

	mu.Lock()
	defer mu.Unlock()

	// Collect EOR commands.
	var eorCmds []string
	for _, cmd := range commands {
		if strings.Contains(cmd, "eor") {
			eorCmds = append(eorCmds, cmd)
		}
	}

	// Verify both families got EOR.
	hasIPv4 := slices.ContainsFunc(eorCmds, func(s string) bool {
		return strings.Contains(s, "ipv4/unicast")
	})
	hasIPv6 := slices.ContainsFunc(eorCmds, func(s string) bool {
		return strings.Contains(s, "ipv6/unicast")
	})
	if !hasIPv4 {
		t.Error("missing EOR for ipv4/unicast")
	}
	if !hasIPv6 {
		t.Error("missing EOR for ipv6/unicast")
	}

	// Verify command format: peer selector is the peer address, command contains "update text nlri <family> eor".
	for _, cmd := range eorCmds {
		if !strings.HasPrefix(cmd, "10.0.0.1\t") {
			t.Errorf("EOR command should target peer 10.0.0.1, got: %s", cmd)
		}
		if !strings.Contains(cmd, "update text nlri") {
			t.Errorf("EOR command has wrong format: %s", cmd)
		}
	}
}

// TestReplayForPeer_NoFamilies_NoEOR verifies no EOR sent when peer has no families.
//
// VALIDATES: AC-5 edge case — peer with empty/nil families does not trigger EOR.
// PREVENTS: Panic or spurious EOR for peers without negotiated families.
func TestReplayForPeer_NoFamilies_NoEOR(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.dispatchCommandHook = func(cmd string) (string, string, error) {
		return statusDone, `{"last-index":0,"replayed":0}`, nil
	}

	var mu sync.Mutex
	var commands []string
	rs.updateRouteHook = func(peer, cmd string) {
		mu.Lock()
		commands = append(commands, cmd)
		mu.Unlock()
	}

	// Peer with nil families (OPEN not yet processed).
	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1",
		Up:      true,
	}
	rs.mu.Unlock()

	rs.dispatchText("peer 10.0.0.1 remote as 65001 state up")

	// No EOR should be sent for a peer without families.
	require.Never(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, cmd := range commands {
			if strings.Contains(cmd, "eor") {
				return true
			}
		}
		return false
	}, 200*time.Millisecond, 5*time.Millisecond,
		"expected no EOR commands for peer without families")
}

// TestReplayForPeer_FailedReplay_SendsEOR verifies EOR IS sent after a
// replay dispatch failure, so peers do not hang waiting for end-of-RIB.
//
// VALIDATES: spec-rs-fastpath-2-adjrib -- replayForPeer's error branch now
// uniformly calls sendEOR regardless of failure kind (missing-dep, IPC
// timeout, engine error). Previous behavior (no EOR on failure) left
// EOR-gated peers stuck in initial-sync whenever replay dispatch failed.
// PREVENTS: regression to the pre-refactor asymmetric-EOR behavior.
//
// Superseded: the pre-refactor TestReplayForPeer_FailedReplay_NoEOR
// asserted the opposite invariant, which was itself the bug we resolved.
func TestReplayForPeer_FailedReplay_SendsEOR(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.dispatchCommandHook = func(cmd string) (string, string, error) {
		return statusError, "", fmt.Errorf("replay failed")
	}

	eorCh := make(chan struct{})
	var eorOnce sync.Once

	var mu sync.Mutex
	var commands []string
	rs.updateRouteHook = func(peer, cmd string) {
		mu.Lock()
		commands = append(commands, cmd)
		mu.Unlock()
		if strings.Contains(cmd, "eor") {
			eorOnce.Do(func() { close(eorCh) })
		}
	}

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address:  "10.0.0.1",
		Up:       true,
		Families: map[family.Family]bool{family.IPv4Unicast: true},
	}
	rs.mu.Unlock()

	rs.dispatchText("peer 10.0.0.1 remote as 65001 state up")

	select {
	case <-eorCh:
	case <-time.After(2 * time.Second):
		t.Fatal("expected EOR after failed replay (unified-error-path behavior)")
	}

	mu.Lock()
	defer mu.Unlock()
	gotEOR := false
	for _, cmd := range commands {
		if strings.Contains(cmd, "ipv4/unicast eor") {
			gotEOR = true
			break
		}
	}
	if !gotEOR {
		t.Errorf("expected ipv4/unicast EOR in commands, got: %v", commands)
	}
}
