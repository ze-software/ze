package bgp_rs

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// --- Integration test: prove redistribution works through real SDK RPCs ---

// newIntegrationRouteServer creates a RouteServer with live net.Pipe SDK
// connections and an engine-side RPC conn for verifying RPCs.
func newIntegrationRouteServer(t *testing.T) (*RouteServer, *rpc.Conn) {
	t.Helper()
	enginePluginEnd, engineServerEnd := net.Pipe()
	callbackPluginEnd, callbackServerEnd := net.Pipe()
	t.Cleanup(func() {
		if err := enginePluginEnd.Close(); err != nil {
			t.Logf("cleanup: %v", err)
		}
		if err := engineServerEnd.Close(); err != nil {
			t.Logf("cleanup: %v", err)
		}
		if err := callbackPluginEnd.Close(); err != nil {
			t.Logf("cleanup: %v", err)
		}
		if err := callbackServerEnd.Close(); err != nil {
			t.Logf("cleanup: %v", err)
		}
	})

	p := sdk.NewWithConn("rr-integration", enginePluginEnd, callbackPluginEnd)
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Logf("cleanup: %v", err)
		}
	})

	engineConn := rpc.NewConn(engineServerEnd, engineServerEnd)

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

	return rs, engineConn
}

// TestRedistribution_ForwardReachesEngine verifies that UPDATE events dispatched
// to the route server produce correctly ordered cache-forward SDK RPCs that
// arrive at the engine side of the connection.
//
// VALIDATES: Full path: event → parse → handleUpdate → channel → worker →
// forwardUpdate → SDK RPC → engine receives "bgp cache N forward peer1,peer2".
// PREVENTS: Silent RPC failures masking a broken redistribution path.
func TestRedistribution_ForwardReachesEngine(t *testing.T) {
	rs, engineConn := newIntegrationRouteServer(t)

	// Register peers: 10.0.0.1 (source), 10.0.0.2 and 10.0.0.3 (targets).
	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.3"] = &PeerState{
		Address: "10.0.0.3", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.mu.Unlock()

	// Dispatch 3 UPDATE events from peer 10.0.0.1.
	for i := 1; i <= 3; i++ {
		input := fmt.Sprintf(
			"peer 10.0.0.1 asn 65001 received update %d origin igp next-hop 192.168.1.1 nlri ipv4/unicast add prefix 10.0.%d.0/24",
			i, i,
		)
		rs.dispatchText(input)
	}

	// Read RPCs from the engine side. With batch accumulation, the 3 UPDATEs
	// may arrive as a single batch RPC (e.g., "bgp cache 1,2,3 forward ...").
	// Read until all 3 message IDs are accounted for.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var allIDs []uint64
	for len(allIDs) < 3 {
		req, err := engineConn.ReadRequest(ctx)
		if err != nil {
			t.Fatalf("read RPC (have %d IDs): %v", len(allIDs), err)
		}

		if req.Method != "ze-plugin-engine:update-route" {
			t.Errorf("method = %q, want ze-plugin-engine:update-route", req.Method)
		}

		var input rpc.UpdateRouteInput
		if err := json.Unmarshal(req.Params, &input); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}

		// Parse "bgp cache <ids> forward <selector>" — ids may be comma-separated.
		parts := strings.Fields(input.Command)
		if len(parts) < 4 || parts[0] != "bgp" || parts[1] != "cache" || parts[3] != "forward" {
			t.Fatalf("unexpected command format: %q", input.Command)
		}

		idStr := parts[2]
		selectorStr := parts[4]

		for s := range strings.SplitSeq(idStr, ",") {
			id, parseErr := strconv.ParseUint(s, 10, 64)
			if parseErr != nil {
				t.Fatalf("invalid ID %q in command: %v", s, parseErr)
			}
			allIDs = append(allIDs, id)
		}

		// Verify selector contains both targets (order may vary).
		peers := strings.Split(selectorStr, ",")
		sort.Strings(peers)
		if len(peers) != 2 || peers[0] != "10.0.0.2" || peers[1] != "10.0.0.3" {
			t.Errorf("selector = %q, want 10.0.0.2,10.0.0.3", selectorStr)
		}

		// Send success response so the worker unblocks.
		if err := engineConn.SendResult(ctx, req.ID, rpc.UpdateRouteOutput{
			PeersAffected: 2,
			RoutesSent:    2,
		}); err != nil {
			t.Fatalf("send response: %v", err)
		}
	}

	// Verify all 3 message IDs arrived in order.
	if len(allIDs) != 3 {
		t.Fatalf("expected 3 IDs, got %d: %v", len(allIDs), allIDs)
	}
	for i, id := range allIDs {
		if id != uint64(i+1) {
			t.Errorf("ID[%d] = %d, want %d", i, id, i+1)
		}
	}
}

// TestRedistribution_ReleaseReachesEngine verifies that an UPDATE with no NLRI
// produces a cache-release SDK RPC that reaches the engine.
//
// VALIDATES: Release path: empty UPDATE → channel → worker → releaseCache → SDK RPC.
// PREVENTS: Cache entries stuck forever when no targets exist.
func TestRedistribution_ReleaseReachesEngine(t *testing.T) {
	rs, engineConn := newIntegrationRouteServer(t)

	// Dispatch UPDATE with no NLRI → should trigger release.
	input := "peer 10.0.0.1 asn 65001 received update 42"
	rs.dispatchText(input)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := engineConn.ReadRequest(ctx)
	if err != nil {
		t.Fatalf("read RPC: %v", err)
	}

	var rpcInput rpc.UpdateRouteInput
	if err := json.Unmarshal(req.Params, &rpcInput); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}

	if rpcInput.Command != "bgp cache 42 release" {
		t.Errorf("command = %q, want %q", rpcInput.Command, "bgp cache 42 release")
	}

	if err := engineConn.SendResult(ctx, req.ID, rpc.UpdateRouteOutput{}); err != nil {
		t.Fatalf("send response: %v", err)
	}
}

// TestRedistribution_FamilyFiltering verifies that the forward command only
// includes peers that support the UPDATE's families.
//
// VALIDATES: ipv6/unicast UPDATE only forwarded to peers supporting ipv6/unicast.
// PREVENTS: Routes sent to peers that cannot process them (engine does no family filtering).
func TestRedistribution_FamilyFiltering(t *testing.T) {
	rs, engineConn := newIntegrationRouteServer(t)

	// 10.0.0.2 supports ipv4+ipv6, 10.0.0.3 supports only ipv4.
	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true},
	}
	rs.peers["10.0.0.3"] = &PeerState{
		Address: "10.0.0.3", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.mu.Unlock()

	// ipv6/unicast UPDATE from 10.0.0.1 → only 10.0.0.2 should be a target.
	input := "peer 10.0.0.1 asn 65001 received update 7 origin igp next-hop 2001:db8::1 nlri ipv6/unicast add prefix 2001:db8::/32"
	rs.dispatchText(input)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := engineConn.ReadRequest(ctx)
	if err != nil {
		t.Fatalf("read RPC: %v", err)
	}

	var rpcInput rpc.UpdateRouteInput
	if err := json.Unmarshal(req.Params, &rpcInput); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}

	// Should forward to 10.0.0.2 only (not 10.0.0.3 which lacks ipv6/unicast).
	expected := "bgp cache 7 forward 10.0.0.2"
	if rpcInput.Command != expected {
		t.Errorf("command = %q, want %q", rpcInput.Command, expected)
	}

	if err := engineConn.SendResult(ctx, req.ID, rpc.UpdateRouteOutput{
		PeersAffected: 1, RoutesSent: 1,
	}); err != nil {
		t.Fatalf("send response: %v", err)
	}
}

// --- Forward worker ordering tests ---

// TestForwardWorker_OrderPreserved verifies that the queue-based worker
// delivers cache forward commands in strictly increasing message-ID order.
//
// VALIDATES: 100 rapid UPDATE events produce work items in FIFO order (AC-1).
// PREVENTS: FIFO ordering violation that caused 98% route loss under load —
// the engine's cache requires acks in message-ID order, and out-of-order acks
// implicitly evict earlier entries.
func TestForwardWorker_OrderPreserved(t *testing.T) {
	rs := newTestRouteServer(t)

	// Set up two peers so forwarding has targets
	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.mu.Unlock()

	// Replace the default worker pool with one that records processing order.
	// Stop the existing pool first (created by newTestRouteServer).
	rs.workers.Stop()

	var mu sync.Mutex
	var processed []uint64
	rs.workers = newWorkerPool(func(_ workerKey, item workItem) {
		mu.Lock()
		processed = append(processed, item.msgID)
		mu.Unlock()
	}, poolConfig{chanSize: 128, idleTimeout: 5 * time.Second})

	// Dispatch 100 UPDATE events — handleUpdate stores fwdCtx and dispatches
	// to the per-source-peer worker. Channel preserves FIFO order per key.
	const numUpdates = 100
	for i := 1; i <= numUpdates; i++ {
		input := fmt.Sprintf(
			"peer 10.0.0.1 asn 65001 received update %d origin igp next-hop 192.168.1.1 nlri ipv4/unicast add prefix 10.0.%d.0/24",
			i, i,
		)
		rs.dispatchText(input)
	}

	// Stop workers — drains all pending items before returning.
	rs.workers.Stop()

	if len(processed) != numUpdates {
		t.Fatalf("expected %d processed items, got %d", numUpdates, len(processed))
	}

	for i := 1; i < len(processed); i++ {
		if processed[i] <= processed[i-1] {
			t.Errorf("FIFO violation: processed[%d]=%d <= processed[%d]=%d", i, processed[i], i-1, processed[i-1])
		}
	}
}

// TestForwardWorker_ReleaseInOrder verifies that release commands interleaved
// with forward commands maintain strict message-ID ordering.
//
// VALIDATES: Mix of forward and release work items preserves FIFO order (AC-2).
// PREVENTS: Release commands bypassing the channel and arriving out of order.
func TestForwardWorker_ReleaseInOrder(t *testing.T) {
	rs := newTestRouteServer(t)

	// Set up one peer that supports ipv4/unicast only.
	// UPDATEs with no NLRI will produce releases.
	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.mu.Unlock()

	// Replace the default worker pool with one that records processing order
	// and reads the forwarding context to determine forward vs release.
	rs.workers.Stop()

	type recordedItem struct {
		msgID   uint64
		release bool
	}
	var mu sync.Mutex
	var items []recordedItem
	rs.workers = newWorkerPool(func(_ workerKey, item workItem) {
		val, ok := rs.fwdCtx.LoadAndDelete(item.msgID)
		if !ok {
			return
		}
		ctx, ok := val.(*forwardCtx)
		if !ok {
			return
		}
		families := parseTextUpdateFamilies(ctx.textPayload)
		isRelease := len(families) == 0
		mu.Lock()
		items = append(items, recordedItem{msgID: item.msgID, release: isRelease})
		mu.Unlock()
	}, poolConfig{chanSize: 128, idleTimeout: 5 * time.Second})

	// Dispatch alternating forward/release events:
	// Even msgIDs → ipv4/unicast (has targets → forward)
	// Odd msgIDs → no NLRI (→ release)
	const numUpdates = 20
	for i := 1; i <= numUpdates; i++ {
		var input string
		if i%2 == 0 {
			input = fmt.Sprintf(
				"peer 10.0.0.1 asn 65001 received update %d origin igp next-hop 192.168.1.1 nlri ipv4/unicast add prefix 10.0.%d.0/24",
				i, i,
			)
		} else {
			// Empty NLRI → triggers release path
			input = fmt.Sprintf("peer 10.0.0.1 asn 65001 received update %d", i)
		}
		rs.dispatchText(input)
	}

	// Stop workers — drains all pending items.
	rs.workers.Stop()

	if len(items) != numUpdates {
		t.Fatalf("expected %d items, got %d", numUpdates, len(items))
	}

	var ids []uint64
	for _, item := range items {
		ids = append(ids, item.msgID)
		expectRelease := (item.msgID%2 != 0)
		if item.release != expectRelease {
			t.Errorf("item msgID=%d: release=%v, want %v", item.msgID, item.release, expectRelease)
		}
	}

	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			t.Errorf("FIFO violation: ids[%d]=%d <= ids[%d]=%d", i, ids[i], i-1, ids[i-1])
		}
	}
}

// TestForwardWorker_DrainOnClose verifies that stopping the worker pool
// causes it to process all remaining queued items before returning.
//
// VALIDATES: Worker pool drains queued items on shutdown (AC-3).
// PREVENTS: Lost forward/release commands during plugin shutdown.
func TestForwardWorker_DrainOnClose(t *testing.T) {
	// Create a worker pool with a counting handler to verify drain behavior.
	var count sync.WaitGroup
	var processed atomic.Int32
	pool := newWorkerPool(func(_ workerKey, item workItem) {
		processed.Add(1)
		count.Done()
	}, poolConfig{chanSize: 128, idleTimeout: 5 * time.Second})

	// Dispatch items to the pool.
	const numItems = 10
	count.Add(numItems)
	for i := 1; i <= numItems; i++ {
		pool.Dispatch(workerKey{sourcePeer: "10.0.0.1"}, workItem{msgID: uint64(i)})
	}

	// Stop must drain all items before returning.
	pool.Stop()

	if int(processed.Load()) != numItems {
		t.Fatalf("expected %d processed items after drain, got %d", numItems, processed.Load())
	}
}

// TestForwardOrdering_SequentialPreservesOrder demonstrates that sequential
// forwarding (no goroutines) preserves message ordering.
//
// VALIDATES: Sequential dispatch produces in-order cache commands.
// PREVENTS: False positive from ordering tests.
func TestForwardOrdering_SequentialPreservesOrder(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.mu.Unlock()

	const numUpdates = 100
	var commands []string

	// Simulate sequential forwarding
	for i := 1; i <= numUpdates; i++ {
		msgID := uint64(i)
		families := map[string]bool{"ipv4/unicast": true}

		rs.mu.RLock()
		targets := rs.selectForwardTargets("10.0.0.1", families)
		rs.mu.RUnlock()

		if len(targets) > 0 {
			sel := strings.Join(targets, ",")
			cmd := fmt.Sprintf("bgp cache %d forward %s", msgID, sel)
			commands = append(commands, cmd)
		}
	}

	if len(commands) != numUpdates {
		t.Fatalf("expected %d commands, got %d", numUpdates, len(commands))
	}

	// Verify ordering: each command should have strictly increasing msg ID
	for i := 1; i < len(commands); i++ {
		var prevID, currID uint64
		if _, err := fmt.Sscanf(commands[i-1], "bgp cache %d", &prevID); err != nil {
			t.Fatalf("parse command[%d]: %v", i-1, err)
		}
		if _, err := fmt.Sscanf(commands[i], "bgp cache %d", &currID); err != nil {
			t.Fatalf("parse command[%d]: %v", i, err)
		}
		if currID <= prevID {
			t.Errorf("out of order: command %d (id=%d) followed by command %d (id=%d)",
				i-1, prevID, i, currID)
		}
	}
}

// --- Target selection tests (forwardUpdate decision logic) ---

// TestSelectTargets_SingleFamily_AllSupport verifies basic single-family forwarding.
//
// VALIDATES: UPDATE with one family is forwarded to all peers that support it.
// PREVENTS: Forward logic silently dropping routes in the simple case.
func TestSelectTargets_SingleFamily_AllSupport(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true},
	}
	rs.peers["10.0.0.3"] = &PeerState{
		Address: "10.0.0.3", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.mu.Unlock()

	// UPDATE from 10.0.0.1 with ipv4/unicast
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.1", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()

	sort.Strings(targets)
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d: %v", len(targets), targets)
	}
	if targets[0] != "10.0.0.2" || targets[1] != "10.0.0.3" {
		t.Errorf("expected [10.0.0.2, 10.0.0.3], got %v", targets)
	}
}

// TestSelectTargets_SingleFamily_PartialSupport verifies family filtering.
//
// VALIDATES: Peers that don't support the UPDATE's family are excluded.
// PREVENTS: Sending routes to peers that can't process them.
func TestSelectTargets_SingleFamily_PartialSupport(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/unicast": true}, // No ipv6
	}
	rs.peers["10.0.0.3"] = &PeerState{
		Address: "10.0.0.3", Up: true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true},
	}
	rs.mu.Unlock()

	// ipv6/unicast UPDATE from 10.0.0.1 → only 10.0.0.3 supports it
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.1", map[string]bool{"ipv6/unicast": true})
	rs.mu.RUnlock()

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d: %v", len(targets), targets)
	}
	if targets[0] != "10.0.0.3" {
		t.Errorf("expected [10.0.0.3], got %v", targets)
	}
}

// TestSelectTargets_MultiFamilyUpdate_PartialOverlap verifies multi-family UPDATE forwarding.
//
// VALIDATES: When an UPDATE carries families A+B, a peer supporting only A still
// receives the UPDATE (known limitation — see spec Known Limitation section).
// PREVENTS: All-or-nothing family check that drops the entire UPDATE for peers
// with partial family overlap. This was the original bug: the code required ALL
// families to match, so peers missing even one family got NOTHING.
func TestSelectTargets_MultiFamilyUpdate_PartialOverlap(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/unicast": true}, // Only ipv4
	}
	rs.mu.Unlock()

	// UPDATE from "10.0.0.0" carries both ipv4/unicast and ipv6/unicast
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.0", map[string]bool{
		"ipv4/unicast": true,
		"ipv6/unicast": true,
	})
	rs.mu.RUnlock()

	sort.Strings(targets)
	// BOTH peers should be targets: 10.0.0.1 supports both, 10.0.0.2 supports ipv4.
	// Known limitation: peer receives full UPDATE including unnegotiated families.
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets (partial overlap should include peer), got %d: %v", len(targets), targets)
	}
}

// TestSelectTargets_ExcludesSourcePeer verifies source peer is never a target.
//
// VALIDATES: Source peer exclusion prevents routing loops.
// PREVENTS: Route reflected back to the sender.
func TestSelectTargets_ExcludesSourcePeer(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.mu.Unlock()

	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.1", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()

	if len(targets) != 1 {
		t.Fatalf("expected 1 target (excluding source), got %d: %v", len(targets), targets)
	}
	if targets[0] != "10.0.0.2" {
		t.Errorf("expected target 10.0.0.2, got %v", targets)
	}
}

// TestSelectTargets_ExcludesDownPeer verifies down peers are never targets.
//
// VALIDATES: Only established peers receive forwarded routes.
// PREVENTS: Sending routes to peers that can't process them.
func TestSelectTargets_ExcludesDownPeer(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: false,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.mu.Unlock()

	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.0", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()

	if len(targets) != 1 {
		t.Fatalf("expected 1 target (excluding down peer), got %d: %v", len(targets), targets)
	}
	if targets[0] != "10.0.0.2" {
		t.Errorf("expected target 10.0.0.2, got %v", targets)
	}
}

// TestSelectTargets_NilFamilies_AcceptsAll verifies nil Families means "accept all".
//
// VALIDATES: Peers without OPEN data (Families=nil) receive all updates.
// PREVENTS: Dropping routes to peers whose capabilities are unknown.
func TestSelectTargets_NilFamilies_AcceptsAll(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address:  "10.0.0.1",
		Up:       true,
		Families: nil, // No OPEN processed yet
	}
	rs.mu.Unlock()

	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.0", map[string]bool{"ipv6/vpn": true})
	rs.mu.RUnlock()

	if len(targets) != 1 {
		t.Fatalf("expected 1 target (nil Families → accept all), got %d: %v", len(targets), targets)
	}
}

// TestSelectTargets_MPWithoutIPv4_DeclinesIPv4Unicast verifies that a peer
// advertising MP families without ipv4/unicast explicitly declines it.
//
// VALIDATES: When MP caps are present but omit ipv4/unicast, the peer rejects ipv4/unicast routes.
// PREVENTS: Forwarding ipv4/unicast to peers that explicitly opted out via MP capability negotiation.
// NOTE: RFC 4760 Section 1 — ipv4/unicast is only the implicit default when NO MP caps are sent.
func TestSelectTargets_MPWithoutIPv4_DeclinesIPv4Unicast(t *testing.T) {
	rs := newTestRouteServer(t)

	// Peer advertised MP for l2vpn/evpn only — explicitly declined ipv4/unicast
	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address:  "10.0.0.1",
		Up:       true,
		Families: map[string]bool{"l2vpn/evpn": true},
	}
	rs.mu.Unlock()

	// ipv4/unicast should be rejected — peer explicitly omitted it from MP caps
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.0", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()

	if len(targets) != 0 {
		t.Errorf("expected 0 targets (ipv4/unicast declined via MP), got %d: %v", len(targets), targets)
	}

	// l2vpn/evpn should be accepted — it's in the MP caps
	rs.mu.RLock()
	targets = rs.selectForwardTargets("10.0.0.0", map[string]bool{"l2vpn/evpn": true})
	rs.mu.RUnlock()

	if len(targets) != 1 {
		t.Errorf("expected 1 target (l2vpn/evpn in MP caps), got %d: %v", len(targets), targets)
	}
}

// TestSelectTargets_NoTargets_AllExcluded verifies empty result when all peers are excluded.
//
// VALIDATES: No targets when only peer is the source.
// PREVENTS: Crash or panic on empty target list.
func TestSelectTargets_NoTargets_AllExcluded(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.mu.Unlock()

	// Source is the only peer
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.1", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()

	if len(targets) != 0 {
		t.Errorf("expected 0 targets (only peer is source), got %d: %v", len(targets), targets)
	}
}

// --- Event ordering / race condition tests ---

// TestOpenCreatesEmptyFamilies verifies OPEN with no multiprotocol creates empty Families.
//
// VALIDATES: handleOpen with non-multiprotocol capabilities creates non-nil empty Families map.
// PREVENTS: Assumption that Families is nil when peer has capabilities.
// NOTE: This is a real scenario — a peer sending OPEN with only asn4 + route-refresh
// but no multiprotocol capability would have empty Families after handleOpen.
func TestOpenCreatesEmptyFamilies(t *testing.T) {
	rs := newTestRouteServer(t)

	// OPEN with capabilities but NO multiprotocol entries
	input := "peer 10.0.0.1 asn 65001 received open 0 router-id 10.0.0.1 hold-time 180 cap 2 route-refresh cap 65 asn4 65001"
	rs.dispatchText(input)

	rs.mu.RLock()
	peer := rs.peers["10.0.0.1"]
	rs.mu.RUnlock()

	if peer == nil {
		t.Fatal("peer not created")
	}
	if peer.Families == nil {
		t.Fatal("expected non-nil Families after OPEN")
	}
	// RFC 4271: ipv4/unicast is always implicitly negotiated, even with no
	// multiprotocol capabilities in the OPEN message.
	if len(peer.Families) != 1 || !peer.Families["ipv4/unicast"] {
		t.Errorf("expected Families={ipv4/unicast: true} (RFC 4271 default), got %v", peer.Families)
	}

	// Peer is Up=false (no state event yet), so excluded regardless
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.0", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()
	if len(targets) != 0 {
		t.Errorf("expected 0 targets (peer is down), got %d: %v", len(targets), targets)
	}

	// Set it up and try again with Up=true — now ipv4/unicast should match
	rs.mu.Lock()
	peer.Up = true
	rs.mu.Unlock()

	rs.mu.RLock()
	targets = rs.selectForwardTargets("10.0.0.0", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()

	if len(targets) != 1 {
		t.Errorf("expected 1 target (ipv4/unicast always supported), got %d: %v", len(targets), targets)
	}
}

// TestStateUpBeforeOpen_FamiliesNil verifies state-up before OPEN means nil Families.
//
// VALIDATES: If state "up" arrives before OPEN, peer has nil Families (accepts all).
// PREVENTS: Race where routes are dropped because OPEN hasn't been processed yet.
func TestStateUpBeforeOpen_FamiliesNil(t *testing.T) {
	rs := newTestRouteServer(t)
	rs.dispatchCommandHook = func(cmd string) (string, string, error) {
		return statusDone, `{"last-index":0,"replayed":0}`, nil
	}

	// State up arrives first (before OPEN)
	stateInput := "peer 10.0.0.1 asn 65001 state up"
	rs.dispatchText(stateInput)
	time.Sleep(50 * time.Millisecond) // Let replay goroutine complete.

	rs.mu.RLock()
	peer := rs.peers["10.0.0.1"]
	rs.mu.RUnlock()

	if peer == nil {
		t.Fatal("peer not created")
	}
	if !peer.Up {
		t.Error("expected peer to be up")
	}
	if peer.Families != nil {
		t.Errorf("expected nil Families before OPEN, got %v", peer.Families)
	}

	// With nil Families, peer should accept ALL families
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.0", map[string]bool{"l2vpn/evpn": true})
	rs.mu.RUnlock()

	if len(targets) != 1 {
		t.Fatalf("expected 1 target (nil Families → accept all), got %d", len(targets))
	}
}

// TestOpenThenStateUp_FamiliesPopulated verifies normal OPEN→state-up sequence.
//
// VALIDATES: After OPEN + state-up, peer has correct Families and accepts matching routes.
// PREVENTS: Missing family extraction from OPEN capabilities.
func TestOpenThenStateUp_FamiliesPopulated(t *testing.T) {
	rs := newTestRouteServer(t)
	rs.dispatchCommandHook = func(cmd string) (string, string, error) {
		return statusDone, `{"last-index":0,"replayed":0}`, nil
	}

	// Step 1: OPEN with multiprotocol for ipv4/unicast and ipv6/unicast
	openInput := "peer 10.0.0.1 asn 65001 received open 0 router-id 10.0.0.1 hold-time 180 cap 1 multiprotocol ipv4/unicast cap 1 multiprotocol ipv6/unicast cap 2 route-refresh"
	rs.dispatchText(openInput)

	// Step 2: State up
	stateInput := "peer 10.0.0.1 asn 65001 state up"
	rs.dispatchText(stateInput)
	time.Sleep(50 * time.Millisecond) // Let replay goroutine complete.

	rs.mu.RLock()
	peer := rs.peers["10.0.0.1"]
	rs.mu.RUnlock()

	if !peer.Up {
		t.Error("expected peer up")
	}
	if !peer.SupportsFamily("ipv4/unicast") {
		t.Error("expected ipv4/unicast support")
	}
	if !peer.SupportsFamily("ipv6/unicast") {
		t.Error("expected ipv6/unicast support")
	}
	if peer.SupportsFamily("l2vpn/evpn") {
		t.Error("should NOT support l2vpn/evpn")
	}

	// ipv4/unicast UPDATE should target this peer
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.0", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()
	if len(targets) != 1 {
		t.Fatalf("expected 1 target for ipv4/unicast, got %d", len(targets))
	}

	// l2vpn/evpn UPDATE should NOT target this peer
	rs.mu.RLock()
	targets = rs.selectForwardTargets("10.0.0.0", map[string]bool{"l2vpn/evpn": true})
	rs.mu.RUnlock()
	if len(targets) != 0 {
		t.Errorf("expected 0 targets for l2vpn/evpn, got %d: %v", len(targets), targets)
	}
}

// --- Full propagation scenario tests ---

// TestPropagation_ThreePeers_SingleFamily simulates basic 3-peer route reflection.
//
// VALIDATES: Route from peer A tracked in withdrawal map, peers B and C would be forward targets.
// PREVENTS: Basic forwarding failure in simple topology.
func TestPropagation_ThreePeers_SingleFamily(t *testing.T) {
	rs := newTestRouteServer(t)

	// Set up 3 peers with OPEN + state-up
	peers := []struct {
		addr     string
		families []string
	}{
		{"10.0.0.1", []string{"ipv4/unicast"}},
		{"10.0.0.2", []string{"ipv4/unicast"}},
		{"10.0.0.3", []string{"ipv4/unicast"}},
	}

	for _, p := range peers {
		fam := make(map[string]bool)
		for _, f := range p.families {
			fam[f] = true
		}
		rs.mu.Lock()
		rs.peers[p.addr] = &PeerState{
			Address:      p.addr,
			Up:           true,
			Families:     fam,
			Capabilities: map[string]bool{"route-refresh": true, "multiprotocol": true},
		}
		rs.mu.Unlock()
	}

	// Peer 1 sends an UPDATE with 10.0.0.0/24
	updateInput := "peer 10.0.0.1 asn 65001 received update 100 origin igp next-hop 192.168.1.1 nlri ipv4/unicast add prefix 10.0.0.0/24"
	rs.dispatchText(updateInput)
	flushWorkers(t, rs)

	// Verify withdrawal map has the route.
	rs.withdrawalMu.Lock()
	wdLen := len(rs.withdrawals["10.0.0.1"])
	rs.withdrawalMu.Unlock()
	if wdLen != 1 {
		t.Fatalf("expected 1 withdrawal entry, got %d", wdLen)
	}

	// Verify forward targets: should be 10.0.0.2 and 10.0.0.3 (not source 10.0.0.1)
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.1", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()

	sort.Strings(targets)
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d: %v", len(targets), targets)
	}
	if targets[0] != "10.0.0.2" || targets[1] != "10.0.0.3" {
		t.Errorf("expected [10.0.0.2, 10.0.0.3], got %v", targets)
	}
}

// TestPropagation_FourPeers_SevenFamilies simulates the chaos test topology.
//
// VALIDATES: Route forwarding works correctly with mixed family support across 4 peers.
// PREVENTS: Route propagation failure in the exact topology that the chaos test uses.
func TestPropagation_FourPeers_SevenFamilies(t *testing.T) {
	rs := newTestRouteServer(t)

	// Simulate chaos test: 4 peers, 7 families, not all peers support all families
	peerFamilies := map[string][]string{
		"10.0.0.1": {"ipv4/unicast", "ipv4/flow", "ipv6/unicast", "ipv6/flow", "ipv4/vpn", "ipv6/vpn", "l2vpn/evpn"},
		"10.0.0.2": {"ipv4/unicast", "ipv6/unicast", "ipv4/vpn"},
		"10.0.0.3": {"ipv4/unicast", "ipv6/unicast", "ipv4/flow", "l2vpn/evpn"},
		"10.0.0.4": {"ipv4/unicast", "ipv6/unicast", "ipv6/vpn", "ipv6/flow"},
	}

	for addr, families := range peerFamilies {
		fam := make(map[string]bool)
		for _, f := range families {
			fam[f] = true
		}
		rs.mu.Lock()
		rs.peers[addr] = &PeerState{
			Address:      addr,
			Up:           true,
			Families:     fam,
			Capabilities: map[string]bool{"route-refresh": true, "multiprotocol": true},
		}
		rs.mu.Unlock()
	}

	tests := []struct {
		name      string
		source    string
		families  map[string]bool
		wantCount int
		wantAddrs []string // sorted
	}{
		{
			name:      "ipv4/unicast from peer1 → all others",
			source:    "10.0.0.1",
			families:  map[string]bool{"ipv4/unicast": true},
			wantCount: 3,
			wantAddrs: []string{"10.0.0.2", "10.0.0.3", "10.0.0.4"},
		},
		{
			name:      "ipv4/flow from peer1 → only peer3",
			source:    "10.0.0.1",
			families:  map[string]bool{"ipv4/flow": true},
			wantCount: 1,
			wantAddrs: []string{"10.0.0.3"},
		},
		{
			name:      "l2vpn/evpn from peer1 → only peer3",
			source:    "10.0.0.1",
			families:  map[string]bool{"l2vpn/evpn": true},
			wantCount: 1,
			wantAddrs: []string{"10.0.0.3"},
		},
		{
			name:      "ipv6/vpn from peer1 → only peer4",
			source:    "10.0.0.1",
			families:  map[string]bool{"ipv6/vpn": true},
			wantCount: 1,
			wantAddrs: []string{"10.0.0.4"},
		},
		{
			name:      "ipv6/unicast from peer2 → peers 1,3,4",
			source:    "10.0.0.2",
			families:  map[string]bool{"ipv6/unicast": true},
			wantCount: 3,
			wantAddrs: []string{"10.0.0.1", "10.0.0.3", "10.0.0.4"},
		},
		{
			name:      "ipv4/vpn from peer2 → only peer1",
			source:    "10.0.0.2",
			families:  map[string]bool{"ipv4/vpn": true},
			wantCount: 1,
			wantAddrs: []string{"10.0.0.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs.mu.RLock()
			targets := rs.selectForwardTargets(tt.source, tt.families)
			rs.mu.RUnlock()

			sort.Strings(targets)
			if len(targets) != tt.wantCount {
				t.Errorf("expected %d targets, got %d: %v", tt.wantCount, len(targets), targets)
			}
			for i, want := range tt.wantAddrs {
				if i >= len(targets) {
					t.Errorf("missing target %s", want)
					continue
				}
				if targets[i] != want {
					t.Errorf("target[%d] = %s, want %s", i, targets[i], want)
				}
			}
		})
	}
}

// TestPropagation_UpdateBeforeAnyPeerKnown verifies UPDATE when no peers are registered.
//
// VALIDATES: UPDATE arriving before any peers are known produces no targets (release).
// PREVENTS: Panic on empty peers map, or route leak.
func TestPropagation_UpdateBeforeAnyPeerKnown(t *testing.T) {
	rs := newTestRouteServer(t)

	// No peers registered at all
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.1", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()

	if len(targets) != 0 {
		t.Errorf("expected 0 targets (no peers known), got %d: %v", len(targets), targets)
	}
}

// TestPropagation_UpdateWhenOnlySourceKnown verifies UPDATE when only source peer exists.
//
// VALIDATES: UPDATE with only the source peer registered produces no targets.
// PREVENTS: Forward to source (routing loop).
func TestPropagation_UpdateWhenOnlySourceKnown(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true},
	}
	rs.mu.Unlock()

	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.1", map[string]bool{"ipv4/unicast": true})
	rs.mu.RUnlock()

	if len(targets) != 0 {
		t.Errorf("expected 0 targets (only source peer), got %d: %v", len(targets), targets)
	}
}

// TestPropagation_VPNRoute verifies VPN NLRI with complex prefix format is handled.
//
// VALIDATES: VPN routes with object NLRIs (containing prefix field) tracked in withdrawal map.
// PREVENTS: Lost VPN routes due to NLRI format mismatch.
func TestPropagation_VPNRoute(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{
		Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/vpn": true},
	}
	rs.peers["10.0.0.2"] = &PeerState{
		Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/vpn": true},
	}
	rs.mu.Unlock()

	// VPN UPDATE with NLRI
	input := "peer 10.0.0.1 asn 65001 received update 200 origin igp next-hop 192.168.1.1 nlri ipv4/vpn add prefix 10.0.0.0/24"
	rs.dispatchText(input)
	flushWorkers(t, rs)

	rs.withdrawalMu.Lock()
	peerWd := rs.withdrawals["10.0.0.1"]
	rs.withdrawalMu.Unlock()
	if len(peerWd) != 1 {
		t.Fatalf("expected 1 VPN withdrawal entry, got %d", len(peerWd))
	}
	entry, ok := peerWd["ipv4/vpn|10.0.0.0/24"]
	if !ok {
		t.Fatal("missing withdrawal entry for VPN route")
	}
	if entry.Family != "ipv4/vpn" {
		t.Errorf("expected family ipv4/vpn, got %s", entry.Family)
	}

	// Verify forward target
	rs.mu.RLock()
	targets := rs.selectForwardTargets("10.0.0.1", map[string]bool{"ipv4/vpn": true})
	rs.mu.RUnlock()

	if len(targets) != 1 || targets[0] != "10.0.0.2" {
		t.Errorf("expected target [10.0.0.2], got %v", targets)
	}
}

// TestPropagation_WithdrawClearsWithdrawalMap verifies withdrawal removes route from withdrawal map.
//
// VALIDATES: After withdrawal, route is removed from withdrawal tracking.
// PREVENTS: Stale routes persisting after withdrawal.
func TestPropagation_WithdrawClearsWithdrawalMap(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true}}
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true,
		Families: map[string]bool{"ipv4/unicast": true}}
	rs.mu.Unlock()

	// Add route.
	addInput := "peer 10.0.0.1 asn 65001 received update 100 origin igp next-hop 192.168.1.1 nlri ipv4/unicast add prefix 10.0.0.0/24"
	rs.dispatchText(addInput)
	flushWorkers(t, rs)

	rs.withdrawalMu.Lock()
	addLen := len(rs.withdrawals["10.0.0.1"])
	rs.withdrawalMu.Unlock()
	if addLen != 1 {
		t.Fatal("route not added to withdrawal map")
	}

	// Withdraw route.
	delInput := "peer 10.0.0.1 asn 65001 received update 101 nlri ipv4/unicast del prefix 10.0.0.0/24"
	rs.dispatchText(delInput)
	flushWorkers(t, rs)

	rs.withdrawalMu.Lock()
	delLen := len(rs.withdrawals["10.0.0.1"])
	rs.withdrawalMu.Unlock()
	if delLen != 0 {
		t.Error("route not withdrawn from withdrawal map")
	}
}

// TestPropagation_PeerDownClearsAllRoutes verifies session down clears all routes.
//
// VALIDATES: When a peer goes down, all its routes are removed from withdrawal map.
// PREVENTS: Ghost routes from disconnected peers.
func TestPropagation_PeerDownClearsAllRoutes(t *testing.T) {
	rs := newTestRouteServer(t)

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true,
		Families: map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true}}
	rs.mu.Unlock()

	// Populate withdrawal map for multiple families.
	rs.withdrawalMu.Lock()
	rs.withdrawals["10.0.0.1"] = map[string]withdrawalInfo{
		"ipv4/unicast|10.0.0.0/24":   {Family: "ipv4/unicast", Prefix: "10.0.0.0/24"},
		"ipv4/unicast|10.0.1.0/24":   {Family: "ipv4/unicast", Prefix: "10.0.1.0/24"},
		"ipv6/unicast|2001:db8::/32": {Family: "ipv6/unicast", Prefix: "2001:db8::/32"},
	}
	rs.withdrawalMu.Unlock()

	// Peer goes down.
	downInput := "peer 10.0.0.1 asn 65001 state down"
	rs.dispatchText(downInput)

	rs.withdrawalMu.Lock()
	wdLen := len(rs.withdrawals["10.0.0.1"])
	rs.withdrawalMu.Unlock()
	if wdLen != 0 {
		t.Error("withdrawal map not cleared on peer down")
	}
}
