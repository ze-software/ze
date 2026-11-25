// Design: docs/architecture/plugin/rib-storage-design.md -- original retained RS routes.
// Related: retained_settlement.go -- actual asynchronous cleanup barrier.
package rs

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func retainedRSTestOwner(t *testing.T) (*routeServer, workerKey, rpc.StoredRoute, rpc.StoredRoute) {
	t.Helper()
	key, err := newWorkerKey("192.0.2.1", 11, 101)
	if err != nil {
		t.Fatal(err)
	}
	rs := &routeServer{peers: make(map[workerKey]*PeerState), withdrawals: make(map[string]map[withdrawalKey]withdrawalEntry)}
	rs.peers[key] = &PeerState{Address: key.address.String(), Up: false, RSClient: true, StateSeen: true, sourceLease: true, sourceReceipt: 201}
	a := rpc.StoredRoute{SourcePeer: key.address.String(), SourceOwner: key.owner, SourceConnectionID: key.connectionID, SourceReceipt: 201, SourcePolicyEpoch: 7, MsgID: 301, Family: family.IPv4Unicast.String(), NLRIFraming: rpc.NLRIFramingPrefixOnly, NLRIHex: "180a0001"}
	b := a
	b.NLRIHex = "180a0002"
	b.MsgID = 302
	ka, err := storedKey(a, false)
	if err != nil {
		t.Fatal(err)
	}
	kb, err := storedKey(b, false)
	if err != nil {
		t.Fatal(err)
	}
	rs.withdrawals[key.address.String()] = map[withdrawalKey]withdrawalEntry{
		ka: {owner: key.owner, connectionID: key.connectionID, sourceReceipt: 201, sourcePolicyEpoch: 7, msgID: 301, retained: true},
		kb: {owner: key.owner, connectionID: key.connectionID, sourceReceipt: 201, sourcePolicyEpoch: 7, msgID: 302, retained: true, noLLGR: true},
	}
	return rs, key, a, b
}

func retainedRSCleanup(t *testing.T, rs *routeServer, key workerKey, noLLGR bool) {
	t.Helper()
	payload, err := retention.EncodeCleanup(key.owner, key.connectionID, key.address.String(), []family.Family{family.IPv4Unicast}, noLLGR)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(rpc.EventEnvelope{Namespace: retention.Namespace, Event: retention.CleanupEventType, Payload: json.RawMessage(payload)})
	if err != nil {
		t.Fatal(err)
	}
	matched, err := rs.sealCleanup(string(encoded))
	if err != nil || !matched {
		t.Fatalf("cleanup not sealed: %v %v", matched, err)
	}
}

func TestRSRetainedCurrentMembershipSeparatesSameReceiptNO_LLGR(t *testing.T) {
	rs, key, a, b := retainedRSTestOwner(t)
	retainedRSCleanup(t, rs, key, true)
	var got []rpc.StoredRoute
	rs.relayStoredRouteHook = func(_ string, routes []rpc.StoredRoute) error { got = append(got, routes...); return nil }
	target := rpc.ForwardTarget{Destination: "192.0.2.9", DestinationOwner: 19, DestinationSession: 209}
	if err := rs.relayToTarget(context.Background(), target, []rpc.StoredRoute{a, b}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].NLRIHex != a.NLRIHex {
		t.Fatalf("removed NO_LLGR B survived its shared source receipt: %#v", got)
	}
	// The original receipt cannot be borrowed by an address sibling/successor.
	sibling := a
	sibling.SourceOwner = 12
	successor := a
	successor.SourceConnectionID = 102
	stale := a
	stale.MsgID--
	if err := rs.relayToTarget(context.Background(), target, []rpc.StoredRoute{sibling, successor, stale}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("foreign original source or stale generation was relayed: %#v", got)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rs.settleRetainedEvents(ctx); err != nil {
		t.Fatal(err)
	}
	rs.withdrawalMu.Lock()
	kept := rs.currentStoredRouteLocked(a)
	removed := rs.currentStoredRouteLocked(b)
	rs.withdrawalMu.Unlock()
	if !kept || removed {
		t.Fatalf("post-barrier retained membership A=%v removed B=%v", kept, removed)
	}
	if !rs.peers[key].sourceLease {
		t.Fatal("NO_LLGR B cleanup released retained A's shared lease")
	}
}

func TestRSCleanupWaitsForActualCachedBatchFinalFlush(t *testing.T) {
	rs, key, a, b := retainedRSTestOwner(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	rs.forwardCachedHook = func(ids []uint64, _ []string) {
		if len(ids) != 2 || ids[0] != 301 || ids[1] != 302 {
			t.Errorf("actual A+B cached batch changed: %v", ids)
		}
		close(entered)
		<-release
	}
	rs.workers = newWorkerPool(rs.processForward, poolConfig{chanSize: 2, idleTimeout: time.Hour, onDrained: rs.flushWorkerBatch, onClose: rs.dropParkedValidations})
	defer func() { once.Do(func() { close(release) }); rs.workers.Stop() }()
	batch := rs.workerBatch(key)
	batch.ids = []uint64{301, 302}
	batch.targets = []rpc.ForwardTarget{{Destination: "192.0.2.9", DestinationOwner: 19, DestinationSession: 209}}
	marker := make(chan struct{})
	if !rs.workers.Dispatch(key, workItem{barrier: marker}) {
		t.Fatal("actual worker not admitted")
	}
	<-entered
	retainedRSCleanup(t, rs, key, true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	settled := make(chan error, 1)
	go func() { settled <- rs.settleRetainedEvents(ctx) }()
	for {
		select {
		case err := <-settled:
			t.Fatalf("cleanup acknowledged held actual cached final flush: %v", err)
		default:
		}
		rs.workers.mu.Lock()
		original := rs.workers.workers[key]
		queued := original != nil && len(original.ch) != 0
		rs.workers.mu.Unlock()
		if queued {
			break
		}
		if err := ctx.Err(); err != nil {
			t.Fatalf("settlement did not enqueue its actual source barrier: %v", err)
		}
		runtime.Gosched()
	}
	once.Do(func() { close(release) })
	if err := <-settled; err != nil {
		t.Fatal(err)
	}
	rs.withdrawalMu.Lock()
	kept := rs.currentStoredRouteLocked(a)
	removed := rs.currentStoredRouteLocked(b)
	rs.withdrawalMu.Unlock()
	if !kept || removed {
		t.Fatalf("cached final flush corrupted per-route cleanup A=%v B=%v", kept, removed)
	}
}

func TestRSCleanupFailedOutputJoinKeepsOriginalLeaseAndMembership(t *testing.T) {
	rs, key, a, b := retainedRSTestOwner(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	rs.relayStoredRouteHook = func(_ string, _ []rpc.StoredRoute) error { close(entered); <-release; return nil }
	target := rpc.ForwardTarget{Destination: "192.0.2.9", DestinationOwner: 19, DestinationSession: 209}
	relayDone := make(chan error, 1)
	go func() { relayDone <- rs.relayToTarget(context.Background(), target, []rpc.StoredRoute{a, b}) }()
	<-entered
	retainedRSCleanup(t, rs, key, true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// An explicit expired work deadline is a failed applied settlement.
	expired, stop := context.WithDeadline(ctx, time.Now().Add(-time.Second))
	defer stop()
	err := rs.settleRetainedEvents(expired)
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("held output did not fail settlement: %v", err)
	}
	if len(rs.peers[key].cleanup) != 1 || !rs.peers[key].sourceLease {
		t.Fatal("failed settlement erased current owed scope or original lease")
	}
	if len(rs.withdrawals[key.address.String()]) != 2 {
		t.Fatal("failed settlement purged inventory before held output joined")
	}
	once.Do(func() { close(release) })
	if err := <-relayDone; err != nil {
		t.Fatal(err)
	}
	live, finish := context.WithTimeout(context.Background(), 5*time.Second)
	defer finish()
	if err := rs.settleRetainedEvents(live); err != nil {
		t.Fatal(err)
	}
	if len(rs.withdrawals[key.address.String()]) != 1 {
		t.Fatal("retry did not remove exact NO_LLGR B after actual output joined")
	}
}

func TestRSFinalCurrentRouteReleasesOnlyItsOriginalLease(t *testing.T) {
	rs, key, _, _ := retainedRSTestOwner(t)
	sibling, _ := newWorkerKey(key.address.String(), 12, 102)
	rs.peers[sibling] = &PeerState{Address: key.address.String(), Up: true, RSClient: true, sourceLease: true, sourceReceipt: 202}
	siblingRoute := withdrawalKey{owner: sibling.owner, fam: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.9.0/24")}
	rs.withdrawals[key.address.String()][siblingRoute] = withdrawalEntry{owner: sibling.owner, connectionID: sibling.connectionID, sourceReceipt: 202, sourcePolicyEpoch: 7, msgID: 309}
	calls := []rpc.SourceLeaseInput{}
	rs.sourceLeaseHook = func(_ context.Context, input rpc.SourceLeaseInput) error { calls = append(calls, input); return nil }
	retainedRSCleanup(t, rs, key, false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rs.settleRetainedEvents(ctx); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].Action != rpc.SourceLeaseRelease || calls[0].SourceReceipt != 201 || calls[0].PeerOwner != 11 {
		t.Fatalf("lease release changed original authority: %#v", calls)
	}
	if rs.peers[key] != nil {
		t.Fatal("completed empty physical actor retained history")
	}
	if rs.peers[sibling] == nil || !rs.peers[sibling].sourceLease {
		t.Fatal("same-address useful sibling lost its own live lease")
	}
	if len(rs.withdrawals[key.address.String()]) != 1 {
		t.Fatal("completed cleanup removed a useful sibling current route or retained removed rows")
	}
	if _, present := rs.withdrawals[key.address.String()][siblingRoute]; !present {
		t.Fatal("completed cleanup erased the useful same-address sibling membership")
	}

}

func TestRSSourceLeaseAdmissionPrecedesActualRetainedDown(t *testing.T) {
	rs, key, _, _ := retainedRSTestOwner(t)
	rs.withdrawals = make(map[string]map[withdrawalKey]withdrawalEntry)
	rs.peers[key].Up = true
	rs.peers[key].sourceLease = false
	rs.peers[key].sourceReceipt = 0
	var calls []rpc.SourceLeaseAction
	rs.sourceLeaseHook = func(_ context.Context, input rpc.SourceLeaseInput) error {
		if input.PeerOwner != key.owner || input.SourceReceipt != 201 {
			t.Errorf("lease changed original facts: %#v", input)
		}
		calls = append(calls, input.Action)
		return nil
	}
	rs.releaseCachedHook = func([]uint64) {}
	rs.workers = newWorkerPool(rs.processForward, poolConfig{chanSize: 2, idleTimeout: time.Hour, onDrained: rs.flushWorkerBatch, onClose: rs.dropParkedValidations})
	defer rs.workers.Stop()
	defer rs.stopDepartures()
	if err := rs.retained.Begin(key.owner, key.connectionID); err != nil {
		t.Fatal(err)
	}
	rs.dispatchText("peer 192.0.2.1 remote as 65001 received update 303 peer-owner 11 connection-id 101 source-receipt 201 source-policy-epoch 7 nlri ipv4/unicast info 0 add prefix 10.0.3.0/24")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rs.workers.settleOwner(ctx, key.owner); err != nil {
		t.Fatal(err)
	}
	payload, err := retention.Encode(key.owner, key.connectionID, key.address.String(), []family.Family{family.IPv4Unicast})
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.retained.Record(rpc.EventEnvelope{Namespace: retention.Namespace, Event: retention.EventType, Payload: json.RawMessage(payload)}); err != nil {
		t.Fatal(err)
	}
	rs.handleStateDown(key, 401)
	if err := rs.awaitDeparture(ctx, key.owner); err != nil {
		t.Fatal(err)
	}
	if len(calls) < 2 || calls[0] != rpc.SourceLeaseAcquire || calls[1] != rpc.SourceLeaseRetain {
		t.Fatalf("original lease did not precede actual ordered DOWN: %v", calls)
	}
	if len(rs.withdrawals[key.address.String()]) != 1 {
		t.Fatal("actual retained DOWN discarded its admitted path")
	}
	for native, entry := range rs.withdrawals[key.address.String()] {
		if native.pathID != 0 || !native.addPath || !entry.retained || entry.sourceReceipt != 201 || entry.connectionID != 101 {
			t.Fatalf("actual retained row lost original path zero or session: %#v %#v", native, entry)
		}
	}
	retainedRSCleanup(t, rs, key, false)
	if err := rs.settleRetainedEvents(ctx); err != nil {
		t.Fatal(err)
	}
	if calls[len(calls)-1] != rpc.SourceLeaseRelease || rs.peers[key] != nil {
		t.Fatalf("last actual current row did not release/prune original actor: %v", calls)
	}
}

func TestRSTextCurrentNativePathZeroDoesNotRemoveSibling(t *testing.T) {
	rs, key, _, _ := retainedRSTestOwner(t)
	rs.peers[key].Up = true
	rs.withdrawals = make(map[string]map[withdrawalKey]withdrawalEntry)
	text := "peer 192.0.2.1 remote as 65001 received update 301 peer-owner 11 connection-id 101 source-receipt 201 source-policy-epoch 7 nlri ipv4/unicast info 0 add raw 180a0001 nlri ipv4/unicast info 7 add raw 180a0001"
	announcement := withdrawalEntry{msgID: 301, sourcePolicyEpoch: 7, sourceReceipt: 201, owner: 11, connectionID: 101}
	rs.updateWithdrawalMapText(key.address.String(), parseTextNLRIOps(text), announcement)
	if len(rs.withdrawals[key.address.String()]) != 2 {
		t.Fatal("text original path zero was merged with its sibling identifier")
	}
	withdrawn := "peer 192.0.2.1 remote as 65001 received update 302 peer-owner 11 connection-id 101 source-receipt 201 source-policy-epoch 7 nlri ipv4/unicast info 0 del raw 180a0001"
	announcement.msgID = 302
	rs.updateWithdrawalMapText(key.address.String(), parseTextNLRIOps(withdrawn), announcement)
	if len(rs.withdrawals[key.address.String()]) != 1 {
		t.Fatal("path-zero withdrawal removed another original current path")
	}
	for native := range rs.withdrawals[key.address.String()] {
		if native.pathID != 7 {
			t.Fatalf("wrong source path survived original zero withdrawal: %#v", native)
		}
	}
}
