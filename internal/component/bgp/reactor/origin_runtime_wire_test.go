// Design: docs/architecture/api/process-protocol.md -- exact original lifecycle.
// Related: selected_origin_writer_test.go -- useful actual selected wire output.
package reactor

import (
	"bufio"
	"context"
	"encoding/hex"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func originRuntimeRoute(record *originReceiptRecord) rpc.SelectedRoute {
	return rpc.SelectedRoute{Kind: rpc.SelectedRouteOrigin, Family: record.build.family.String(), KeyNLRI: record.build.native,
		Revision: 2, OriginReceipt: record.id, StaleLevel: rpc.SelectedFresh,
		Route: &rpc.StoredRoute{Family: record.build.family.String(), NLRIHex: hex.EncodeToString(record.build.native), NLRIFraming: rpc.NLRIFramingPrefixOnly}}
}

func originRuntimeCurrent(t *testing.T, store *originReceiptStore, key originContributionKey) *originReceiptRecord {
	t.Helper()
	store.mu.Lock()
	record := store.current[key]
	store.mu.Unlock()
	require.NotNil(t, record)
	return record
}

func TestOriginRuntimeEndedProducerKeepsOnlyOriginalWire(t *testing.T) {
	processFactory := sourceLeaseProcessFactory(t)
	f := newSelectedOriginFixture(t, false)
	proc := processFactory(selectedWriterProcess)
	producerOwner, err := proc.SelectedOwner()
	require.NoError(t, err)
	f.producer, err = newProcessOriginProducer(producerOwner)
	require.NoError(t, err)
	original, value := f.issue(t, selectedOriginBuildInput(), 1)
	result, err := f.send(t, t.Context(), original)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, result.Results[0].Outcome)
	proc.Stop()
	require.Error(t, producerOwner.Current())
	kept, err := f.store.endOwner(t.Context(), producerOwner)
	require.NoError(t, err)
	require.Len(t, kept, 1)
	require.True(t, kept[0].retained)
	require.Error(t, kept[0].ctx.Err())
	_, err = f.store.issue(t.Context(), f.producer, f.source, value, f.peer, f.owner)
	require.Error(t, err, "an ended producer cannot submit arbitrary new data")
	_, err = f.store.admit(original.OriginReceipt, f.owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.Error(t, err, "the old receipt is canceled even though original data is retained")
	fresh, err := f.store.reissue(t.Context(), kept[0], f.owner)
	require.NoError(t, err)
	require.Greater(t, fresh, original.OriginReceipt)
	record := originRuntimeCurrent(t, f.store, kept[0].key)
	require.Same(t, value, record.build)
	replay := originRuntimeRoute(record)
	result, err = f.send(t, t.Context(), replay)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, result.Results[0].Outcome)
	bodies := updateBodies(t, f.conn.written())
	require.GreaterOrEqual(t, len(bodies), 2)
	last := bodies[len(bodies)-1]
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, last, false))
	path, present := bodyPathAttr(t, last, 2)
	require.True(t, present)
	require.Equal(t, []uint32{65000, 65007}, aspathASNs(t, path, 4))
	hop, present := bodyPathAttr(t, last, 3)
	require.True(t, present)
	require.Equal(t, []byte{192, 0, 2, 254}, hop)
}

func TestOriginRuntimeDownPeerAdmissionWaitsForRealSession(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	facts := f.peer.fwdFacts.Load()
	f.peer.fwdFacts.Store(nil)
	original, value := f.issue(t, selectedOriginBuildInput(), 1)
	require.Nil(t, f.peer.forwardFacts(), "admission must not make a down peer appear Established")
	require.Empty(t, f.conn.written())
	_, err := f.store.admit(original.OriginReceipt, f.owner, f.peer, nil, value.key)
	require.Error(t, err, "admitted scope is not a captured TCP Session")
	f.peer.fwdFacts.Store(facts)
	result, err := f.send(t, t.Context(), original)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, result.Results[0].Outcome)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 1)
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[0], false))
}

func TestOriginRuntimeProviderEndJoinsActualCallback(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	reader := f.owner.CurrentSelection
	lifetime, stop := context.WithCancel(t.Context())
	t.Cleanup(stop)
	owner, err := plugin.NewSelectedOwner(plugin.ProcessSender(selectedWriterProcess), lifetime.Done(), nil)
	require.NoError(t, err)
	owner, err = owner.WithConnection(reader, lifetime)
	require.NoError(t, err)
	f.owner = owner
	f.producer, err = newProcessOriginProducer(owner)
	require.NoError(t, err)
	route, _ := f.issue(t, selectedOriginBuildInput(), 1)
	f.store.mu.Lock()
	record := f.store.receipts[route.OriginReceipt]
	f.store.mu.Unlock()
	call, release, err := f.store.beginOriginCall(t.Context(), record, owner, false)
	require.NoError(t, err)
	defer release()
	ended := make(chan error, 1)
	stop()
	go func() { _, err := f.store.endOwner(t.Context(), owner); ended <- err }()
	select {
	case <-call.ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("provider cleanup did not cancel its actual admitted callback")
	}
	select {
	case err := <-ended:
		t.Fatalf("cleanup acknowledged an unjoined callback: %v", err)
	default:
	}
	// A useful unrelated original can still use the final writer while this
	// particular callback is canceled but has not acknowledged its completion.
	otherLife, otherCancel := context.WithCancel(t.Context())
	t.Cleanup(otherCancel)
	otherOwner, err := plugin.NewSelectedOwner(plugin.ProcessSender(selectedWriterProcess), otherLife.Done(), nil)
	require.NoError(t, err)
	otherOwner, err = otherOwner.WithConnection(reader, otherLife)
	require.NoError(t, err)
	otherProducer, err := newProcessOriginProducer(otherOwner)
	require.NoError(t, err)
	input := selectedOriginBuildInput()
	input.native = []byte{24, 198, 18, 9}
	value, err := newOriginBuildValue(input)
	require.NoError(t, err)
	id, err := f.store.issue(t.Context(), otherProducer, f.source, value, f.peer, otherOwner)
	require.NoError(t, err)
	// The selected writer uses this current provider incarnation on the same
	// configured destination; cleanup of the original token cannot consume it.
	f.owner = otherOwner
	healthy := originRuntimeCurrent(t, f.store, originKey(otherProducer, f.source, value.originIdentity, f.peer.sourceOwner))
	require.Equal(t, id, healthy.id)
	result, err := f.send(t, t.Context(), originRuntimeRoute(healthy))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, result.Results[0].Outcome)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 1)
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.9.0/24")}, updatePrefixes(t, bodies[0], false))
	release()
	select {
	case err := <-ended:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("provider callback cleanup failed to join")
	}
	require.NotNil(t, originRuntimeCurrent(t, f.store, healthy.key))
	originRuntimeExpectCounts(t, f.store, -1, -1, 0, -1)
}

func TestOriginRuntimeKeptSuccessorReplacesAndWithdrawsExactData(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	producerOwner, cancel := originOwnerFixture(t, selectedWriterProcess)
	var err error
	f.producer, err = newProcessOriginProducer(producerOwner)
	require.NoError(t, err)
	old, value := f.issue(t, selectedOriginBuildInput(), 1)
	cancel()
	kept, err := f.store.endOwner(t.Context(), producerOwner)
	require.NoError(t, err)
	require.Len(t, kept, 1)
	id, err := f.store.reissue(t.Context(), kept[0], f.owner)
	require.NoError(t, err)
	current := originRuntimeCurrent(t, f.store, kept[0].key)
	successor, err := newProcessOriginProducer(f.owner)
	require.NoError(t, err)
	found, err := f.store.originWithdrawal(t.Context(), successor, f.source, value.originIdentity, f.peer, f.owner)
	require.NoError(t, err)
	require.Same(t, current, found)
	newInput := selectedOriginBuildInput()
	newInput.nextHop = selectedOriginBuildInput().nextHop
	freshValue, err := newOriginBuildValue(newInput)
	require.NoError(t, err)
	fresh, err := f.store.issue(t.Context(), successor, f.source, freshValue, f.peer, f.owner)
	require.NoError(t, err)
	require.Greater(t, fresh, id)
	require.NoError(t, f.store.joinOriginPredecessors(t.Context(), originKey(successor, f.source, value.originIdentity, f.peer.sourceOwner), fresh))
	_, err = f.store.admit(old.OriginReceipt, f.owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.Error(t, err)
	_, err = f.store.admit(id, f.owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.Error(t, err)
	record := originRuntimeCurrent(t, f.store, originKey(successor, f.source, value.originIdentity, f.peer.sourceOwner))
	result, err := f.send(t, t.Context(), originRuntimeRoute(record))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, result.Results[0].Outcome)
	originRuntimeExpectCounts(t, f.store, 1, -1, -1, -1)
	// Stale exact-generation timer retirement cannot reach this successor.
	expired := plugin.WithOriginRetirement(t.Context(), originRuntimeAcceptedRetirement(t, producerOwner, f.peer, value, old.OriginReceipt))
	stale, err := f.store.originWithdrawal(expired, kept[0].key.producer, f.source, value.originIdentity, f.peer, f.owner)
	require.ErrorIs(t, err, errOriginOriginalGone)
	require.Nil(t, stale)
	require.NoError(t, f.store.withdraw(t.Context(), successor, f.source, value.originIdentity, f.peer))
	originRuntimeExpectCounts(t, f.store, 0, 0, -1, -1)
}

func TestOriginRuntimeRegistryRetirementCannotOriginate(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	owner, cancel := originOwnerFixture(t, selectedWriterProcess)
	var err error
	f.producer, err = newProcessOriginProducer(owner)
	require.NoError(t, err)
	route, value := f.issue(t, selectedOriginBuildInput(), 1)
	cancel()
	kept, err := f.store.endOwner(t.Context(), owner)
	require.NoError(t, err)
	_, err = f.store.reissue(t.Context(), kept[0], f.owner)
	require.NoError(t, err)
	work := plugin.WithOriginRetirement(t.Context(), originRuntimeAcceptedRetirement(t, owner, f.peer, value, route.OriginReceipt))
	batch := bgpOriginRuntimeBatch(owner, value)
	_, err = originBatchProducer(work, batch, owner.Sender())
	require.Error(t, err, "retirement never authorizes Announce")
	producer, err := originBatchProducerOperation(work, batch, owner.Sender(), rpc.OriginIntakeWithdraw)
	require.NoError(t, err)
	record, err := f.store.originWithdrawal(work, producer, f.source, value.originIdentity, f.peer, f.owner)
	require.NoError(t, err)
	require.NotNil(t, record)
	require.Greater(t, record.id, route.OriginReceipt)
	forged, err := newOriginIdentity(value.family, []byte{24, 198, 18, 99}, 0, false)
	require.NoError(t, err)
	unknown, err := f.store.originWithdrawal(work, producer, f.source, forged, f.peer, f.owner)
	require.ErrorIs(t, err, errOriginOriginalGone)
	require.Nil(t, unknown)
	_, err = f.store.originWithdrawal(t.Context(), producer, f.source, value.originIdentity, f.peer, f.owner)
	require.Error(t, err)
	require.NoError(t, f.store.retireOriginReceipt(t.Context(), record.id))
	originRuntimeExpectCounts(t, f.store, 0, 0, -1, -1)
}

func bgpOriginRuntimeBatch(owner plugin.SelectedOwner, value *originBuildValue) bgptypes.NLRIBatch {
	return bgptypes.NLRIBatch{OriginOwner: owner, Family: value.family, NextHop: value.nextHop}
}

func TestOriginRuntimePermanentPeerRemovalJoinsFinalFlush(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	route, _ := f.issue(t, selectedOriginBuildInput(), 1)
	writer, remote := net.Pipe()
	t.Cleanup(func() { require.NoError(t, writer.Close()); require.NoError(t, remote.Close()) })
	connection := &observedWriteConn{Conn: writer, writing: make(chan struct{})}
	f.peer.session.conn = connection
	f.peer.session.bufWriter = bufio.NewWriterSize(connection, 4096)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	ended := make(chan error, 1)
	go func() { _, err := f.send(t, ctx, route); ended <- err }()
	select {
	case <-connection.writing:
	case err := <-ended:
		t.Fatalf("writer failed before blocked flush: %v", err)
	case <-ctx.Done():
		t.Fatal("writer did not reach real flush")
	}
	f.peer.stopping.Store(true)
	f.api.r.completeOriginPeerRemoval(f.peer)
	select {
	case err := <-ended:
		require.Error(t, err)
	case <-ctx.Done():
		t.Fatal("permanent Peer removal acknowledged an unjoined flush")
	}
	originRuntimeExpectCounts(t, f.store, 0, 0, 0, -1)
	// Equal-address replacement is a new original Peer, never an old receipt.
	_, err := f.store.admit(route.OriginReceipt, f.owner, f.peer, f.peer.selectedOwnerSnapshot(), f.storeKeyForTest(t, route))
	require.Error(t, err)
}

func (f *selectedOriginFixture) storeKeyForTest(t *testing.T, route rpc.SelectedRoute) fwdPathKey {
	t.Helper()
	identity, err := newOriginIdentity(family.IPv4Unicast, route.KeyNLRI, 0, false)
	require.NoError(t, err)
	return identity.key
}

func TestOriginRuntimeCloseJoinsCallbackAndKeepsNoHistory(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	route, _ := f.issue(t, selectedOriginBuildInput(), 1)
	f.store.mu.Lock()
	record := f.store.receipts[route.OriginReceipt]
	f.store.mu.Unlock()
	call, release, err := f.store.beginOriginCall(t.Context(), record, f.owner, false)
	require.NoError(t, err)
	defer release()
	ended := make(chan struct{})
	go func() { f.api.r.completeOriginClose(); close(ended) }()
	select {
	case <-call.ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not cancel admitted provider callback")
	}
	select {
	case <-ended:
		t.Fatal("shutdown acknowledged a callback that has not joined")
	default:
	}
	release()
	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not join callback acknowledgment")
	}
	originRuntimeExpectCounts(t, f.store, 0, 0, 0, 0)
}

func TestOriginRuntimeProviderReplacementReusesOriginalExactBytes(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	reader := f.owner.CurrentSelection
	lifetime, stop := context.WithCancel(t.Context())
	t.Cleanup(stop)
	owner, err := plugin.NewSelectedOwner(plugin.ProcessSender(selectedWriterProcess), lifetime.Done(), nil)
	require.NoError(t, err)
	owner, err = owner.WithConnection(reader, lifetime)
	require.NoError(t, err)
	f.owner = owner
	// An explicit operator remains alive independently of the deciding Process.
	f.producer = operatorOriginProducer()
	route, value := f.issue(t, selectedOriginBuildInput(), 1)
	stop()
	parked, err := f.store.endOwner(t.Context(), owner)
	require.NoError(t, err)
	require.Len(t, parked, 1)
	require.False(t, parked[0].retained, "provider replacement must not invent an ended producer")
	nextLife, nextCancel := context.WithCancel(t.Context())
	t.Cleanup(nextCancel)
	next, err := plugin.NewSelectedOwner(plugin.ProcessSender(selectedWriterProcess), nextLife.Done(), nil)
	require.NoError(t, err)
	next, err = next.WithConnection(reader, nextLife)
	require.NoError(t, err)
	receipt, err := f.store.reissue(t.Context(), parked[0], next)
	require.NoError(t, err)
	require.Greater(t, receipt, route.OriginReceipt)
	record := originRuntimeCurrent(t, f.store, parked[0].key)
	require.Same(t, value, record.build)
	f.owner = next
	answer, err := f.send(t, t.Context(), originRuntimeRoute(record))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 1)
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[0], false))
	_, err = f.store.admit(receipt, owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.Error(t, err)
}

func TestOriginRuntimePeerRemovalKeepsOtherDestinationUseful(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	original, _ := f.issue(t, selectedOriginBuildInput(), 1)
	other, connection := originRevocationScope(t, f.selectedWriterFixture, "192.0.2.81", 72)
	other.session.egressRouteFilter = other.egressGate(other.session)
	input := selectedOriginBuildInput()
	input.native = []byte{24, 198, 18, 9}
	value, err := newOriginBuildValue(input)
	require.NoError(t, err)
	receipt, err := f.store.issue(t.Context(), f.producer, f.source, value, other, f.owner)
	require.NoError(t, err)
	f.peer.stopping.Store(true)
	f.api.r.completeOriginPeerRemoval(f.peer)
	f.store.mu.Lock()
	_, removed := f.store.receipts[original.OriginReceipt]
	record := f.store.receipts[receipt]
	f.store.mu.Unlock()
	require.False(t, removed)
	require.NotNil(t, record)
	route := originRuntimeRoute(record)
	f.mu.Lock()
	f.cut = 2
	f.current[string(route.KeyNLRI)] = 2
	f.mu.Unlock()
	batch, err := plugin.NewSelectedBatch(rpc.SelectRoutesInput{Destination: other.addrString, DestinationSession: 72, Routes: []rpc.SelectedRoute{route}})
	require.NoError(t, err)
	result, err := f.api.SelectRoutes(t.Context(), batch, f.owner)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, result.Results[0].Outcome)
	bodies := updateBodies(t, connection.written())
	require.Len(t, bodies, 1)
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.9.0/24")}, updatePrefixes(t, bodies[0], false))
	originRuntimeExpectCounts(t, f.store, 1, 1, -1, -1)
}

func originRuntimeExpectCounts(t *testing.T, store *originReceiptStore, current, receipts, calls, intakes int) {
	t.Helper()
	store.mu.Lock()
	actual := [4]int{len(store.current), len(store.receipts), len(store.calls), len(store.intakes)}
	store.mu.Unlock()
	for i, want := range [4]int{current, receipts, calls, intakes} {
		if want >= 0 {
			require.Equal(t, want, actual[i], "current/receipts/calls/intakes index %d", i)
		}
	}
}

func TestOriginRuntimeExpiryWithUnavailableProviderCannotResurrect(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	healthyProvider := f.owner
	providerLifetime, stopProvider := context.WithCancel(t.Context())
	t.Cleanup(stopProvider)
	provider, err := plugin.NewSelectedOwner(plugin.ProcessSender(selectedWriterProcess), providerLifetime.Done(), nil)
	require.NoError(t, err)
	provider, err = provider.WithConnection(healthyProvider.CurrentSelection, providerLifetime)
	require.NoError(t, err)
	provider, err = provider.WithOriginIntake(func(context.Context, rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
		return &rpc.UpdateRouteOutput{Withdrawn: 1}, nil
	})
	require.NoError(t, err)
	f.owner = provider
	producerOwner, stop := originOwnerFixture(t, selectedWriterProcess)
	f.producer, err = newProcessOriginProducer(producerOwner)
	require.NoError(t, err)
	original, value := f.issue(t, selectedOriginBuildInput(), 1)
	answer, err := f.send(t, t.Context(), original)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	stop()
	kept, err := f.store.endOwner(t.Context(), producerOwner)
	require.NoError(t, err)
	require.Len(t, kept, 1)
	stopProvider()
	require.Error(t, provider.Current(), "the captured provider, not only the producer, is unavailable")
	batch := bgpOriginRuntimeBatch(producerOwner, value)
	batch.NLRIs = []nlri.NLRI{nlri.NewINET(value.family, netip.MustParsePrefix("198.18.7.0/24"), 0, false)}
	expired := plugin.WithOriginRetirement(t.Context(), originRuntimeAcceptedRetirement(t, producerOwner, f.peer, value, original.OriginReceipt))
	_, err = f.api.WithdrawNLRIBatchResult(expired, selector.ParseDefault(f.peer.addrString), batch, producerOwner.Sender())
	require.Error(t, err, "unavailable provider cleanup must remain explicit")
	originRuntimeExpectCounts(t, f.store, 0, 0, 0, 0)
	f.owner = healthyProvider
	require.NoError(t, f.api.ReplayOriginOwner(t.Context(), f.owner))
	require.Len(t, updateBodies(t, f.conn.written()), 1, "expired data must not be minted again on readiness")
	// A later genuine source at the same native key is useful and survives the stale old timer.
	successor, err := newProcessOriginProducer(f.owner)
	require.NoError(t, err)
	successorInput := selectedOriginBuildInput()
	successorInput.nextHop = bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.253"))
	successorValue, err := newOriginBuildValue(successorInput)
	require.NoError(t, err)
	receipt, err := f.store.issue(t.Context(), successor, f.source, successorValue, f.peer, f.owner)
	require.NoError(t, err)
	record := originRuntimeCurrent(t, f.store, originKey(successor, f.source, value.originIdentity, f.peer.sourceOwner))
	answer, err = f.send(t, t.Context(), originRuntimeRoute(record))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	// This exact original is already absent. No remote mutation or configured
	// owner lookup is needed, and the newer source must stay entirely useful.
	withdrawn, err := f.api.WithdrawNLRIBatchResult(expired, selector.ParseDefault(f.peer.addrString), batch, producerOwner.Sender())
	require.NoError(t, err)
	require.Equal(t, uint32(1), withdrawn.Withdrawn)
	require.Equal(t, receipt, originRuntimeCurrent(t, f.store, record.key).id)
	require.Len(t, updateBodies(t, f.conn.written()), 2)
}

func TestOriginRuntimeRevocationJoinsPostReplyCompensation(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			f := newOriginCompensationFixture(t, typed)
			f.afterRetain = func(in rpc.OriginIntakeInput) *rpc.UpdateRouteOutput {
				reply := originCompensationReply(in)
				reply.Admissions[0].PeerOwner++
				return reply
			}
			offered := make(chan error, 1)
			go func() { _, err := f.announce(t.Context(), originGapBatch()); offered <- err }()
			var cleanup rpc.OriginIntakeInput
			select {
			case cleanup = <-f.withdrawal:
			case <-f.lifetime.Done():
				t.Fatal("did not reach compensation")
			}
			joined := make(chan error, 1)
			waiting := &originRevocationWaitContext{Context: t.Context(), entered: make(chan struct{})}
			go func() { joined <- f.writer.api.r.originReceipts.revokePeer(waiting, f.writer.peer) }()
			select {
			case <-waiting.entered:
			case err := <-joined:
				t.Fatalf("scope ACK escaped composite compensation: %v", err)
			case <-f.lifetime.Done():
				t.Fatal("revocation did not reach its join")
			}
			select {
			case err := <-joined:
				t.Fatalf("scope ACK escaped held compensation: %v", err)
			default:
			}
			require.Contains(t, f.snapshot(), cleanup.OriginReceipt)
			// Existing direct API output to an unrelated destination remains useful while this join is held.
			healthy, wire := newGroupUpdatesPeer(t, "192.0.2.81", "absent")
			healthy.settings.ExportPolicy = f.writer.peer.settings.ExportPolicy
			healthy.reactor = f.writer.api.r
			f.writer.api.r.mu.Lock()
			f.writer.api.r.peers[healthy.Settings().PeerKey()] = healthy
			f.writer.api.r.mu.Unlock()
			healthy.refreshForwardFacts()
			result, err := f.writer.api.AnnounceNLRIBatchResult(t.Context(), selector.ParseDefault(healthy.addrString), originGapBatch(), plugin.OperatorSender())
			require.NoError(t, err)
			require.Equal(t, uint32(1), result.Announced)
			require.NotEmpty(t, wire.written())
			f.unblock()
			select {
			case err := <-offered:
				require.Error(t, err)
			case <-f.lifetime.Done():
				t.Fatal("offer did not join compensation")
			}
			select {
			case err := <-joined:
				require.NoError(t, err)
			case <-f.lifetime.Done():
				t.Fatal("revocation did not join composite operation")
			}
			require.NotContains(t, f.snapshot(), cleanup.OriginReceipt)
			originRuntimeExpectCounts(t, f.writer.api.r.originReceipts, 0, 0, 0, 0)
		})
	}
}

func TestOriginRuntimeExpiredReplaySnapshotKeepsUsefulSuccessor(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	route, value := f.issue(t, selectedOriginBuildInput(), 1)
	original := originRuntimeCurrent(t, f.store, originKey(f.producer, f.source, value.originIdentity, f.peer.sourceOwner))
	f.store.mu.Lock()
	f.store.park(original)
	f.store.mu.Unlock()
	require.NoError(t, f.store.retireOriginReceipt(t.Context(), route.OriginReceipt))
	originRuntimeExpectCounts(t, f.store, 0, 0, 0, 0)
	input := selectedOriginBuildInput()
	input.nextHop = bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.253"))
	successor, _ := f.issue(t, input, 2)
	answer, err := f.send(t, t.Context(), successor)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	require.NoError(t, f.api.replayOrigin(t.Context(), original, f.owner), "an expired ready snapshot owes no new offer")
	current := originRuntimeCurrent(t, f.store, original.key)
	require.Equal(t, successor.OriginReceipt, current.id)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 1)
	hop, present := bodyPathAttr(t, bodies[0], 3)
	require.True(t, present)
	require.Equal(t, []byte{192, 0, 2, 253}, hop)
}

// The isolated issuer questions already own the genuine issued original. Their
// registry value is captured from that exact receipt, never a successor's ID.
func originRuntimeAcceptedRetirement(t *testing.T, owner plugin.SelectedOwner, peer *Peer, value *originBuildValue, generation uint64) *plugin.OriginRetirement {
	t.Helper()
	sender := owner.Sender()
	if !sender.IsSet() {
		sender = plugin.OperatorSender()
	}
	result := &rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{{
		Family: value.family.String(), KeyNLRI: value.native,
		Destination: peer.addrString, PeerOwner: peer.sourceOwner,
		Generation: generation, Outcome: rpc.RouteAdmissionRetained,
		Disposition: rpc.RouteAdmissionSelected,
	}}}
	retirement, err := plugin.CaptureOriginRetirement(sender, owner, result)
	require.NoError(t, err)
	return retirement
}

// A withdrawal with no matching original is a completed no-op at the real
// adapter boundary. It preserves useful current output and does not swallow
// an actual source-grant failure.
// MUTATION: return the lookup's no-match sentinel as an admission failure.
func TestOriginRuntimeUnmatchedWithdrawalKeepsUsefulOriginal(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	original, value := f.issue(t, selectedOriginBuildInput(), 1)
	answer, err := f.send(t, t.Context(), original)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	retirement := originRuntimeAcceptedRetirement(t, f.owner, f.peer, value, original.OriginReceipt)

	next := selectedOriginBuildInput()
	next.nextHop = bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.253"))
	current, currentValue := f.issue(t, next, 2)
	answer, err = f.send(t, t.Context(), current)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	before := f.conn.written()
	record := originRuntimeCurrent(t, f.store, originKey(f.producer, f.source, currentValue.originIdentity, f.peer.sourceOwner))
	missing, err := newOriginIdentity(value.family, []byte{24, 198, 18, 99}, 0, false)
	require.NoError(t, err)
	for _, input := range []struct {
		name     string
		ctx      context.Context
		identity *originIdentity
	}{
		{name: "missing native key", ctx: t.Context(), identity: missing},
		{name: "older accepted lineage", ctx: plugin.WithOriginRetirement(t.Context(), retirement), identity: value.originIdentity},
	} {
		t.Run(input.name, func(t *testing.T) {
			result, err := f.api.withdrawOriginFromPeer(input.ctx, f.producer, f.source, input.identity, f.peer, f.owner)
			require.NoError(t, err)
			require.Equal(t, uint32(1), result.Withdrawn)
			require.Zero(t, result.Announced)
			require.Equal(t, before, f.conn.written(), "an unmatched withdrawal cannot change useful selected output")
			require.Same(t, record, originRuntimeCurrent(t, f.store, record.key))
		})
	}

	f.peer.mu.Lock()
	f.peer.settings.ProcessBindings = nil
	f.peer.mu.Unlock()
	result, err := f.api.withdrawOriginFromPeer(t.Context(), f.producer, f.source, missing, f.peer, f.owner)
	require.ErrorContains(t, err, "no current original Peer grant")
	require.Nil(t, result, "a real admission failure is not a completed no-op")
	require.Equal(t, before, f.conn.written())
	require.Same(t, record, originRuntimeCurrent(t, f.store, record.key))
}
