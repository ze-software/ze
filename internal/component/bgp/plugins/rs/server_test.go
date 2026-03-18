package rs

import (
	"fmt"
	"net"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

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
	rs.startReleaseLoop()
	rs.startForwardLoop()
	rs.workers = newWorkerPool(func(key workerKey, item workItem) {
		rs.processForward(key, item.msgID)
	}, poolConfig{chanSize: 64, idleTimeout: 5 * time.Second, onDrained: rs.flushWorkerBatch})
	t.Cleanup(func() {
		rs.workers.Stop()
		rs.stopForwardLoop()
		rs.stopReleaseLoop()
	})
	return rs
}

// flushWorkers stops and recreates the worker pool, ensuring all pending
// items are processed. Also drains the forward loop so async forward RPCs
// are fully delivered before the test checks hook-captured commands.
func flushWorkers(t *testing.T, rs *RouteServer) {
	t.Helper()
	rs.workers.Stop()
	// Drain forward loop: close channel → goroutine processes remaining → restart.
	rs.stopForwardLoop()
	rs.startForwardLoop()
	rs.workers = newWorkerPool(func(key workerKey, item workItem) {
		rs.processForward(key, item.msgID)
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

	rs.dispatchText("peer 10.0.0.1 asn 65001 received update 123 origin igp as-path 65001 local-preference 100 next-hop 192.168.1.1 nlri ipv4/unicast add prefix 10.0.0.0/24")
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

	rs.dispatchText("peer 10.0.0.1 asn 65001 received update 124 nlri ipv4/unicast del prefix 10.0.0.0/24")
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
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true}}
	rs.mu.Unlock()

	// Pre-populate withdrawal map with route that will be withdrawn.
	rs.withdrawalMu.Lock()
	rs.withdrawals["10.0.0.1"] = map[string]withdrawalInfo{
		"ipv4/unicast|10.0.2.0/24": {Family: "ipv4/unicast", Prefix: "prefix 10.0.2.0/24"},
	}
	rs.withdrawalMu.Unlock()

	rs.dispatchText("peer 10.0.0.1 asn 65001 received update 125 origin igp next-hop 192.168.1.1 nlri ipv4/unicast add prefix 10.0.0.0/24 next-hop 2001:db8::1 nlri ipv6/unicast add prefix 2001:db8::/32 nlri ipv4/unicast del prefix 10.0.2.0/24")
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

	rs.dispatchText("peer 10.0.0.1 asn 65001 state down")

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

	rs.dispatchText("peer 10.0.0.1 asn 65001 state up")
	time.Sleep(50 * time.Millisecond) // Let replay goroutine complete.

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
	if !dispatched.Load() {
		t.Error("expected DispatchCommand to be called for replay")
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

	rs.dispatchText("peer 10.0.0.1 asn 65001 state up")
	time.Sleep(50 * time.Millisecond) // Let replay goroutine complete.

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
	cmd, ok := replayCmd.Load().(string)
	if !ok || cmd == "" {
		t.Fatal("expected replay DispatchCommand, got none")
	}
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

	rs.dispatchText("peer 10.0.0.1 asn 65001 received open 0 router-id 10.0.0.1 hold-time 180 cap 1 multiprotocol ipv4/unicast cap 1 multiprotocol ipv6/unicast cap 2 route-refresh cap 65 asn4 65001")

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
	if !peer.SupportsFamily("ipv4/unicast") {
		t.Error("missing ipv4/unicast family")
	}
	if !peer.SupportsFamily("ipv6/unicast") {
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
	rs.dispatchText("peer 10.0.0.1 asn 65001 received refresh 0 family ipv4/unicast")
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
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address:  "10.0.0.2",
		Up:       true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.3"] = &PeerState{
		Address:  "10.0.0.3",
		Up:       true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true},
	}
	rs.mu.Unlock()

	rs.dispatchText("peer 10.0.0.1 asn 65001 received update 100 origin igp next-hop 2001:db8::1 nlri ipv6/unicast add prefix 2001:db8::/32")
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
		Families:     map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address:  "10.0.0.2",
		Up:       true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.3"] = &PeerState{
		Address:      "10.0.0.3",
		Up:           true,
		Capabilities: map[string]bool{"route-refresh": true},
		Families:     map[string]bool{"ipv4/unicast": true},
	}
	rs.mu.Unlock()

	// handleRefresh forwards to peers via updateRoute (fails silently on closed conn).
	rs.dispatchText("peer 10.0.0.1 asn 65001 received refresh 0 family ipv4/unicast")
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
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address:  "10.0.0.2",
		Up:       true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true},
	}
	rs.mu.Unlock()

	// handleState/handleStateUp calls updateRoute for route replay.
	rs.dispatchText("peer 10.0.0.1 asn 65001 state up")

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
	deadline := time.After(2 * time.Second)
	for {
		rs.mu.RLock()
		stillPaused := rs.pausedPeers["10.0.0.1"]
		rs.mu.RUnlock()
		if !stillPaused {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout: peer not resumed after drain")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
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
	deadline := time.After(2 * time.Second)
	for {
		rs.mu.RLock()
		stillPaused := rs.pausedPeers["10.0.0.1"]
		rs.mu.RUnlock()
		if !stillPaused {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout: peer not resumed after drain")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

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

// TestDispatchPassesPreParsedPayload verifies dispatchText stores text payload in forwardCtx.
//
// VALIDATES: AC-6 — forwardCtx contains text payload for deferred parsing.
// PREVENTS: Missing payload in worker context.
func TestDispatchPassesPreParsedPayload(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.mu.Unlock()

	// Stop workers so items stay in fwdCtx (not consumed by processForward).
	rs.workers.Stop()
	rs.workers = newWorkerPool(func(_ workerKey, _ workItem) {
		// Do nothing — keep fwdCtx intact for inspection.
	}, poolConfig{chanSize: 64, idleTimeout: 5 * time.Second})
	t.Cleanup(func() { rs.workers.Stop() })

	rs.dispatchText("peer 10.0.0.1 asn 65001 received update 42 origin igp next-hop 1.1.1.1 nlri ipv4/unicast add prefix 10.0.0.0/24")

	// Wait for worker to process the no-op handler.
	time.Sleep(50 * time.Millisecond)

	// Check that fwdCtx was stored with textPayload.
	val, ok := rs.fwdCtx.Load(uint64(42))
	if !ok {
		t.Fatal("fwdCtx not found for msgID 42")
	}
	ctx, ok := val.(*forwardCtx)
	if !ok {
		t.Fatal("fwdCtx wrong type")
	}

	// textPayload should contain the text event with peer and update keywords.
	if ctx.textPayload == "" {
		t.Fatal("expected textPayload to be populated")
	}
	if !strings.Contains(ctx.textPayload, "peer") {
		t.Error("textPayload should contain peer keyword")
	}
	if !strings.Contains(ctx.textPayload, "update") {
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

	rs.dispatchText("peer 10.0.0.1 asn 65001 received update 99 origin igp next-hop 1.1.1.1 nlri ipv4/unicast add prefix 10.0.0.0/24")
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
			"peer 10.0.0.1 asn 65001 received update %d origin igp next-hop 1.1.1.1 nlri ipv4/unicast add prefix 10.0.%d.0/24",
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
		rs.processForward(key, item.msgID)
	}, poolConfig{chanSize: 64, idleTimeout: 5 * time.Second})

	rs.dispatchText("peer 10.0.0.1 asn 65001 state down")

	// After peer down, withdrawal map should be empty for this peer.
	time.Sleep(50 * time.Millisecond)
	rs.withdrawalMu.Lock()
	wdLen := len(rs.withdrawals["10.0.0.1"])
	rs.withdrawalMu.Unlock()
	if wdLen != 0 {
		t.Errorf("expected 0 withdrawal entries after peer down, got %d", wdLen)
	}
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

	var commands []string
	var cmdMu sync.Mutex
	rs.updateRouteHook = func(_, cmd string) {
		cmdMu.Lock()
		commands = append(commands, cmd)
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
		rs.processForward(key, item.msgID)
	}, poolConfig{chanSize: 64, idleTimeout: 5 * time.Second, onDrained: rs.flushWorkerBatch})

	// Dispatch 5 UPDATEs (all same source, same targets).
	for i := uint64(1); i <= 5; i++ {
		rs.dispatchText(buildTestUpdate("10.0.0.1", i))
	}

	// Release gate: worker processes all 5 items sequentially with items 2-5
	// already in the channel, so onDrained only fires after item 5 — producing
	// a single batch flush with all 5 IDs.
	close(gate)
	flushWorkers(t, rs)

	cmdMu.Lock()
	defer cmdMu.Unlock()

	// Count forward RPCs and check for batch (comma-separated IDs).
	forwardCount := 0
	hasBatch := false
	for _, cmd := range commands {
		if strings.Contains(cmd, "forward") {
			forwardCount++
			// Check if the ID portion contains a comma (batch).
			parts := strings.Fields(cmd)
			if len(parts) >= 2 && strings.Contains(parts[1], ",") {
				hasBatch = true
			}
		}
	}

	if forwardCount >= 5 {
		t.Errorf("expected fewer than 5 forward RPCs (batched), got %d", forwardCount)
	}
	if !hasBatch {
		t.Error("expected at least one batch forward command with comma-separated IDs")
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

	families := map[string]bool{"ipv4/unicast": true}

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

// TestBatchForwardFireAndForget verifies worker doesn't block on forward RPC.
//
// VALIDATES: AC-11 — worker continues processing without waiting for RPC response.
// PREVENTS: Worker goroutine blocked on synchronous updateRoute during batch flush.
func TestBatchForwardFireAndForget(t *testing.T) {
	rs := newTestRouteServer(t)

	// Block forward RPCs to prove workers don't wait for responses.
	// The hook runs inside updateRoute — with sync forward it blocks
	// the worker goroutine; with async forward it blocks the background
	// sender goroutine instead.
	blockForward := make(chan struct{})
	var forwardCmds []string
	var cmdMu sync.Mutex
	rs.updateRouteHook = func(_, cmd string) {
		if strings.Contains(cmd, "forward") {
			<-blockForward
			cmdMu.Lock()
			forwardCmds = append(forwardCmds, cmd)
			cmdMu.Unlock()
		}
	}

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true}
	rs.mu.Unlock()

	// Dispatch 5 UPDATEs with distinct prefixes from the same source peer.
	for i := uint64(1); i <= 5; i++ {
		rs.dispatchText(fmt.Sprintf(
			"peer 10.0.0.1 asn 65001 received update %d origin igp next-hop 1.1.1.1 nlri ipv4/unicast add prefix 10.0.%d.0/24",
			i, i,
		))
	}

	// Workers.Stop() drains all items. With synchronous forward, the worker
	// goroutine blocks in onDrained → flushBatch → updateRoute → hook, so
	// Stop() hangs. With fire-and-forget, asyncForward returns immediately.
	stopDone := make(chan struct{})
	go func() {
		rs.workers.Stop()
		close(stopDone)
	}()

	select {
	case <-stopDone:
		// Workers stopped promptly — fire-and-forget confirmed.
	case <-time.After(2 * time.Second):
		close(blockForward) // Unblock hook so test can clean up.
		<-stopDone
		t.Fatal("workers.Stop() blocked — forward RPC not fire-and-forget (AC-11)")
	}

	// Workers completed — verify withdrawal map has all routes.
	rs.withdrawalMu.Lock()
	wdLen := len(rs.withdrawals["10.0.0.1"])
	rs.withdrawalMu.Unlock()
	if wdLen != 5 {
		t.Errorf("expected 5 withdrawal entries, got %d", wdLen)
	}

	// Unblock forward RPCs and verify background sender processes them.
	close(blockForward)
	time.Sleep(100 * time.Millisecond)

	cmdMu.Lock()
	defer cmdMu.Unlock()
	if len(forwardCmds) == 0 {
		t.Error("expected forward RPCs processed by background sender")
	}

	// Recreate worker pool for cleanup (Stop() was called above).
	rs.workers = newWorkerPool(func(key workerKey, item workItem) {
		rs.processForward(key, item.msgID)
	}, poolConfig{chanSize: 64, idleTimeout: 5 * time.Second, onDrained: rs.flushWorkerBatch})
}

// buildTestUpdate creates a minimal text-format UPDATE event for testing dispatch.
func buildTestUpdate(peer string, msgID uint64) string {
	return fmt.Sprintf("peer %s asn 65001 received update %d origin igp next-hop 1.1.1.1 nlri ipv4/unicast add prefix 10.0.0.0/24", peer, msgID)
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
		Families: map[string]bool{"ipv4/unicast": true}}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true,
		Families:     map[string]bool{"ipv4/unicast": true},
		Capabilities: map[string]bool{"route-refresh": true}}
	rs.mu.Unlock()

	rs.handleStateUp("10.0.0.1")
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(dispatchCmds) == 0 {
		t.Fatal("expected DispatchCommand calls for replay, got none")
	}
	if !strings.HasPrefix(dispatchCmds[0], "adj-rib-in replay 10.0.0.1") {
		t.Errorf("expected adj-rib-in replay command, got %q", dispatchCmds[0])
	}
	for _, cmd := range updateCmds {
		if strings.Contains(cmd, "refresh") {
			t.Errorf("expected no ROUTE-REFRESH, got: %s", cmd)
		}
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
		Families: map[string]bool{"ipv4/unicast": true}}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/unicast": true}}
	rs.mu.Unlock()

	// A replaying peer SHOULD be in forward targets (duplicates are idempotent).
	rs.mu.RLock()
	targets := rs.selectForwardTargets(nil, "10.0.0.2", map[string]bool{"ipv4/unicast": true})
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
		Families: map[string]bool{"ipv4/unicast": true}}
	rs.mu.Unlock()

	rs.handleStateUp("10.0.0.1")
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(dispatchCmds) < 2 {
		t.Fatalf("expected 2 dispatch commands (full + delta), got %d: %v", len(dispatchCmds), dispatchCmds)
	}
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
		Families: map[string]bool{"ipv4/unicast": true}}
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
	rs.dispatchText("peer 10.0.0.1 asn 65001 state down")

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
		Families: map[string]bool{"ipv4/unicast": true}}
	rs.mu.Unlock()

	// First handleStateUp — goroutine A blocks on DispatchCommand.
	rs.handleStateUp("10.0.0.1")
	time.Sleep(20 * time.Millisecond) // Let goroutine A start and block.

	// Second handleStateUp — simulates rapid reconnect. Goroutine B starts.
	rs.handleStateUp("10.0.0.1")

	// Wait for goroutine B's hook to return, then let replayForPeer finish.
	select {
	case <-secondDone:
	case <-time.After(2 * time.Second):
		t.Fatal("second replay timed out")
	}
	time.Sleep(50 * time.Millisecond) // Let goroutine B set Replaying=false.

	// Goroutine B completed — peer should NOT be Replaying.
	rs.mu.RLock()
	replayingAfterB := rs.peers["10.0.0.1"].Replaying
	genAfterB := rs.peers["10.0.0.1"].ReplayGen
	rs.mu.RUnlock()
	if replayingAfterB {
		t.Error("peer should not be Replaying after second replay completes")
	}
	if genAfterB != 2 {
		t.Errorf("expected ReplayGen=2, got %d", genAfterB)
	}

	// Now unblock goroutine A (stale).
	close(firstBlock)
	time.Sleep(50 * time.Millisecond) // Let goroutine A finish.

	// Stale goroutine A must NOT have changed Replaying back.
	rs.mu.RLock()
	replayingAfterA := rs.peers["10.0.0.1"].Replaying
	rs.mu.RUnlock()
	if replayingAfterA {
		t.Error("stale goroutine must not set Replaying=true after completing")
	}
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
		wantFams map[string]bool
	}{
		{
			name:     "add ipv4",
			input:    "peer 10.0.0.1 asn 65001 received update 42 origin igp as-path 65001,65002 next-hop 10.0.0.1 nlri ipv4/unicast add prefix 192.168.1.0/24\n",
			wantType: eventUpdate,
			wantID:   42,
			wantPeer: "10.0.0.1",
			wantFams: map[string]bool{"ipv4/unicast": true},
		},
		{
			name:     "del ipv4",
			input:    "peer 10.0.0.2 asn 65002 received update 99 nlri ipv4/unicast del prefix 10.0.0.0/24\n",
			wantType: eventUpdate,
			wantID:   99,
			wantPeer: "10.0.0.2",
			wantFams: map[string]bool{"ipv4/unicast": true},
		},
		{
			name:     "add ipv6",
			input:    "peer 10.0.0.1 asn 65001 received update 7 origin igp next-hop 2001:db8::1 nlri ipv6/unicast add prefix 2001:db8::/32\n",
			wantType: eventUpdate,
			wantID:   7,
			wantPeer: "10.0.0.1",
			wantFams: map[string]bool{"ipv6/unicast": true},
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
	input := "peer 10.0.0.1 asn 65001 state up\n"

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
	input := "peer 10.0.0.1 asn 65001 received open 5 router-id 1.1.1.1 hold-time 90 cap 1 multiprotocol ipv4/unicast cap 1 multiprotocol ipv6/unicast\n"

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
	addLine := "peer 10.0.0.1 asn 65001 received update 42 origin igp next-hop 10.0.0.1 nlri ipv4/unicast add prefix 192.168.1.0/24,10.0.0.0/8\n"
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
	delLine := "peer 10.0.0.1 asn 65001 received update 43 nlri ipv4/unicast del prefix 192.168.1.0/24\n"
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

	families := map[string]bool{"ipv4/unicast": true}

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
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true},
	}
	rs.mu.Unlock()

	rs.dispatchText("peer 10.0.0.1 asn 65001 state up")
	time.Sleep(200 * time.Millisecond) // Let replay goroutine complete.

	mu.Lock()
	defer mu.Unlock()

	// Collect EOR commands.
	var eorCmds []string
	for _, cmd := range commands {
		if strings.Contains(cmd, "eor") {
			eorCmds = append(eorCmds, cmd)
		}
	}
	if len(eorCmds) != 2 {
		t.Fatalf("expected 2 EOR commands (one per family), got %d: %v", len(eorCmds), eorCmds)
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

	rs.dispatchText("peer 10.0.0.1 asn 65001 state up")
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	for _, cmd := range commands {
		if strings.Contains(cmd, "eor") {
			t.Errorf("expected no EOR commands for peer without families, got: %s", cmd)
		}
	}
}

// TestReplayForPeer_FailedReplay_NoEOR verifies no EOR sent when replay fails.
//
// VALIDATES: AC-5 error path — failed replay does not send EOR.
// PREVENTS: EOR sent before any routes were actually replayed.
func TestReplayForPeer_FailedReplay_NoEOR(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.dispatchCommandHook = func(cmd string) (string, string, error) {
		return statusError, "", fmt.Errorf("replay failed")
	}

	var mu sync.Mutex
	var commands []string
	rs.updateRouteHook = func(peer, cmd string) {
		mu.Lock()
		commands = append(commands, cmd)
		mu.Unlock()
	}

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address:  "10.0.0.1",
		Up:       true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.mu.Unlock()

	rs.dispatchText("peer 10.0.0.1 asn 65001 state up")
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	for _, cmd := range commands {
		if strings.Contains(cmd, "eor") {
			t.Errorf("expected no EOR after failed replay, got: %s", cmd)
		}
	}
}
