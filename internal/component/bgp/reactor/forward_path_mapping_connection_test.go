// Design: docs/architecture/bgp/structural-forwarding.md -- generation-owned source mappings.
// Related: recent_cache.go -- actual final-consumer eviction; source_receipt.go -- retained receipt.
package reactor

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

// mappingConnectionUpdate makes this exact original source generation. The
// original receipt is real and survives DOWN only while its selected lease does.
func mappingConnectionUpdate(t *testing.T, live *liveSourceReceipt, body []byte) *ReceivedUpdate {
	t.Helper()
	facts := live.record.facts
	wire := wireu.NewWireUpdate(bytes.Clone(body), facts.source.ctxID)
	wire.SetSourceID(facts.source.srcID)
	wire.SetSourceReceipt(live.ID())
	wire.SetSourceOwner(facts.peerOwner)
	wire.SetMessageID(nextMsgID())
	return &ReceivedUpdate{WireUpdate: wire, SourcePeerIP: facts.source.addr, SourcePeerStr: facts.source.strAdr, SourceReceipt: live.ID(), SourceOwner: facts.peerOwner}
}

func mappingConnectionEmit(t *testing.T, original *ReceivedUpdate, destination *Peer, ctxID bgpctx.ContextID) uint32 {
	t.Helper()
	origin, err := newFwdPathOrigin(original)
	require.NoError(t, err)
	result, ok := buildFwdBody(original.WireUpdate, message.MaxMsgLen, ctxID, destination, destination.Settings().Address, &fwdParseCache{}, origin)
	require.True(t, ok)
	defer returnReadBuffer(result.transcodeBuf)
	var encoded *message.Update
	if len(result.rawBodies) > 0 {
		require.Len(t, result.rawBodies, 1)
		encoded, err = message.UnpackUpdate(result.rawBodies[0])
		require.NoError(t, err)
	} else {
		require.Len(t, result.updates, 1)
		encoded = result.updates[0]
	}
	require.Empty(t, encoded.WithdrawnRoutes)
	received, err := original.WireUpdate.NLRI()
	require.NoError(t, err)
	require.Len(t, received, 8)
	require.Len(t, encoded.NLRI, 8)
	require.Equal(t, received[4:], encoded.NLRI[4:], "exact useful native prefix accompanies its generated identifier")
	return binary.BigEndian.Uint32(encoded.NLRI[:4])
}

func mappingConnectionReceipt(t *testing.T, source *relaySource, connectionID uint64) *liveSourceReceipt {
	t.Helper()
	facts, err := newSourceReceiptFacts(source, connectionID)
	require.NoError(t, err)
	live, err := sourceReceipts.issue(facts)
	require.NoError(t, err)
	t.Cleanup(live.close)
	return live
}

// Drive the real cache final-consumer eviction after an old receipt survives
// DOWN through an actual retained lease. Fresh same-peer reconnect output and
// its useful second prefix must retain exact IDs despite late old replay and
// cache eviction. A distinct same-address owner supplies an independent control.
// Both raw rewrite and ADD-PATH/ASN conversion are exercised on destination bytes.
func TestForwardPathMappingRetainedCacheEvictionKeepsFreshConnection(t *testing.T) {
	saved := fwdPathIDs
	fwdPathIDs = newFwdPathIDTable()
	t.Cleanup(func() { fwdPathIDs = saved })
	for _, convert := range []bool{false, true} {
		t.Run(map[bool]string{false: "raw", true: "converted"}[convert], func(t *testing.T) {
			source, originalFacts := sourceReceiptFixture(t)
			_, sourceCtxID := registerForwardBodyTestContext(t, true, true)
			originalFacts.source.ctxID = sourceCtxID
			original := mappingConnectionReceipt(t, &originalFacts.source, originalFacts.connectionID)
			owner, done := sourceReceiptOwner(t)
			t.Cleanup(func() { close(done) })
			lease, err := sourceReceipts.acquire(original.ID(), owner)
			require.NoError(t, err)
			require.NoError(t, lease.retain())
			t.Cleanup(func() { require.NoError(t, lease.revoke(t.Context())) })
			destinationCtx, destinationCtxID := registerForwardBodyTestContext(t, !convert, true)
			destination := forwardBodyTestPeer(destinationCtx, destinationCtxID)
			domain := fwdPathSource{sourceID: source.SourceID(), peerOwner: source.sourceOwner}
			t.Cleanup(func() { fwdPathIDs.releaseSource(domain) })
			announced := pathIDTestBody(t, 65001, 7)
			oldAnnouncement := mappingConnectionUpdate(t, original, announced)
			first := mappingConnectionEmit(t, oldAnnouncement, destination, destinationCtxID)
			oldWithdrawal := mappingConnectionUpdate(t, original, fwdShapeBody(fwdPathIDNLRI(7), nil, nil))
			cache := newRecentUpdateCache(8)
			t.Cleanup(cache.Stop)
			cache.RegisterConsumer("mapping-original-withdrawal")
			cache.Add(oldWithdrawal)
			cache.Activate(oldWithdrawal.WireUpdate.MessageID(), 1)

			source.state.Store(int32(PeerStateStopped))
			original.close()
			source.advanceForwardGeneration()
			replacementSession := source.newSession()
			t.Cleanup(replacementSession.timers.StopAll)
			source.mu.Lock()
			source.session = replacementSession
			source.mu.Unlock()
			source.state.Store(int32(PeerStateEstablished))
			replacementSource := originalFacts.source
			replacementSource.generation = source.forwardGeneration.Load()
			replacement := mappingConnectionReceipt(t, &replacementSource, replacementSession.connectionID)
			require.Greater(t, replacementSession.connectionID, originalFacts.connectionID)
			held, err := lease.begin(source.sourcePolicyEpoch.Load())
			require.NoError(t, err, "retained original lease is still legitimate")
			held.complete()
			require.Equal(t, originalFacts.connectionID, sourceReceipts.originalConnectionID(original.ID(), source.sourceOwner))
			current := mappingConnectionUpdate(t, replacement, announced)
			require.Equal(t, first, mappingConnectionEmit(t, current, destination, destinationCtxID), "same logical peer/path keeps its identifier across reconnect")
			usefulBody := bytes.Clone(announced)
			usefulBody[len(usefulBody)-1] = 1
			useful := mappingConnectionUpdate(t, replacement, usefulBody)
			usefulID := mappingConnectionEmit(t, useful, destination, destinationCtxID)
			require.NotEqual(t, first, usefulID, "useful second prefix owns a separate framed mapping")

			sibling := NewPeer(source.settingsSnapshot())
			require.Equal(t, source.SourceID(), sibling.SourceID(), "actual source registry aliases same address")
			require.NotEqual(t, source.sourceOwner, sibling.sourceOwner)
			sibling.advanceForwardGeneration()
			siblingSession := sibling.newSession()
			t.Cleanup(siblingSession.timers.StopAll)
			siblingSource := replacementSource
			siblingSource.info.peer = sibling
			siblingSource.generation = sibling.forwardGeneration.Load()
			independent := mappingConnectionReceipt(t, &siblingSource, siblingSession.connectionID)
			siblingDomain := fwdPathSource{sourceID: sibling.SourceID(), peerOwner: sibling.sourceOwner}
			t.Cleanup(func() { fwdPathIDs.releaseSource(siblingDomain) })
			control := mappingConnectionUpdate(t, independent, announced)
			controlID := mappingConnectionEmit(t, control, destination, destinationCtxID)
			require.NotEqual(t, first, controlID, "distinct logical owners cannot collapse even with identical SourceID/prefix/received ID")

			// A retained replay gets a new cache ID but is still original ingress1.
			late := mappingConnectionUpdate(t, original, announced)
			late.validationReplay = true
			late.validationMsgID = oldAnnouncement.sourceMessageID()
			require.Equal(t, first, mappingConnectionEmit(t, late, destination, destinationCtxID))
			var queried []ribevents.ValidationRoute
			ribevents.RegisterValidationLookup(func(ribevents.ValidationRoute, uint64) bool { return true }, func(route ribevents.ValidationRoute) bool {
				queried = append(queried, route)
				return route.ConnectionID == replacementSession.connectionID
			})
			t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
			require.NoError(t, cache.Ack(oldWithdrawal.WireUpdate.MessageID(), "mapping-original-withdrawal"))
			require.False(t, cache.Contains(oldWithdrawal.WireUpdate.MessageID()), "actual cache eviction ran, not a direct map-only release")
			require.Equal(t, first, mappingConnectionEmit(t, current, destination, destinationCtxID), "late original eviction cannot delete the reused fresh mapping")
			require.Equal(t, usefulID, mappingConnectionEmit(t, useful, destination, destinationCtxID))
			require.Equal(t, controlID, mappingConnectionEmit(t, control, destination, destinationCtxID))
			// Original41 is either rejected before querying or queried as41, never
			// relabeled42. Physical query correctness is also covered by B4's root.
			for _, query := range queried {
				require.Equal(t, originalFacts.connectionID, query.ConnectionID)
			}

			fwdPathIDs.releaseSource(domain)
			require.Equal(t, controlID, mappingConnectionEmit(t, control, destination, destinationCtxID), "removing one owner must not release its same-SourceID sibling")
		})
	}
}

// A real presence provider can reenter generation while cache eviction queries
// it (the remote provider can wait for work that generates IDs). Every reuse,
// including the SAME original generation, must invalidate that in-flight release.
// The delete/remint case proves pointer identity does not follow a recreated row.
func TestForwardPathMappingPresenceQueryCannotReleaseConcurrentReuse(t *testing.T) {
	saved := fwdPathIDs
	fwdPathIDs = newFwdPathIDTable()
	t.Cleanup(func() { fwdPathIDs = saved })
	for _, remint := range []bool{false, true} {
		t.Run(map[bool]string{false: "same-generation-reuse", true: "delete-remint"}[remint], func(t *testing.T) {
			peer, facts := sourceReceiptFixture(t)
			_, ctxID := registerForwardBodyTestContext(t, true, true)
			facts.source.ctxID = ctxID
			original := mappingConnectionReceipt(t, &facts.source, facts.connectionID)
			ctx := bgpctx.Registry.Get(ctxID)
			destination := forwardBodyTestPeer(ctx, ctxID)
			domain := fwdPathSource{sourceID: peer.SourceID(), peerOwner: peer.sourceOwner}
			t.Cleanup(func() { fwdPathIDs.releaseSource(domain) })
			update := mappingConnectionUpdate(t, original, pathIDTestBody(t, 65001, 17))
			before := mappingConnectionEmit(t, update, destination, ctxID)
			withdrawal := mappingConnectionUpdate(t, original, fwdShapeBody(fwdPathIDNLRI(17), nil, nil))
			cache := newRecentUpdateCache(8)
			t.Cleanup(cache.Stop)
			cache.RegisterConsumer("mapping-reentrant-presence")
			cache.Add(withdrawal)
			cache.Activate(withdrawal.WireUpdate.MessageID(), 1)
			calls := 0
			var replacementID uint32
			ribevents.RegisterValidationLookup(func(ribevents.ValidationRoute, uint64) bool { return true }, func(query ribevents.ValidationRoute) bool {
				calls++
				require.Equal(t, facts.connectionID, query.ConnectionID)
				if remint {
					fwdPathIDs.releaseSource(domain)
				}
				replacementID = mappingConnectionEmit(t, update, destination, ctxID)
				return false
			})
			t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
			require.NoError(t, cache.Ack(withdrawal.WireUpdate.MessageID(), "mapping-reentrant-presence"))
			require.Equal(t, 1, calls)
			require.False(t, cache.Contains(withdrawal.WireUpdate.MessageID()))
			if remint {
				require.NotEqual(t, before, replacementID)
			} else {
				require.Equal(t, before, replacementID)
			}
			require.Equal(t, replacementID, mappingConnectionEmit(t, update, destination, ctxID), "query completion cannot delete a mapping reused during that query")
		})
	}
}

// A newer physical withdrawal can retire a mapping left by the old connection,
// but only after the mapping's ORIGINAL receive ownership is absent. This is
// the opposite boundary to stale old41 trying to release fresh42.
func TestForwardPathMappingNewWithdrawalQueriesMappedOriginal(t *testing.T) {
	saved := fwdPathIDs
	fwdPathIDs = newFwdPathIDTable()
	t.Cleanup(func() { fwdPathIDs = saved })
	peer, facts := sourceReceiptFixture(t)
	_, ctxID := registerForwardBodyTestContext(t, true, true)
	facts.source.ctxID = ctxID
	original := mappingConnectionReceipt(t, &facts.source, facts.connectionID)
	ctx := bgpctx.Registry.Get(ctxID)
	destination := forwardBodyTestPeer(ctx, ctxID)
	domain := fwdPathSource{sourceID: peer.SourceID(), peerOwner: peer.sourceOwner}
	t.Cleanup(func() { fwdPathIDs.releaseSource(domain) })
	update := mappingConnectionUpdate(t, original, pathIDTestBody(t, 65001, 19))
	want := mappingConnectionEmit(t, update, destination, ctxID)
	replacementSession := peer.newSession()
	t.Cleanup(replacementSession.timers.StopAll)
	newer := mappingConnectionReceipt(t, &facts.source, replacementSession.connectionID)
	present := true
	var queries []ribevents.ValidationRoute
	ribevents.RegisterValidationLookup(func(ribevents.ValidationRoute, uint64) bool { return true }, func(query ribevents.ValidationRoute) bool {
		queries = append(queries, query)
		return present
	})
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	cache := newRecentUpdateCache(8)
	t.Cleanup(cache.Stop)
	cache.RegisterConsumer("mapping-newer-withdrawal")
	evict := func() {
		t.Helper()
		withdrawal := mappingConnectionUpdate(t, newer, fwdShapeBody(fwdPathIDNLRI(19), nil, nil))
		cache.Add(withdrawal)
		cache.Activate(withdrawal.WireUpdate.MessageID(), 1)
		require.NoError(t, cache.Ack(withdrawal.WireUpdate.MessageID(), "mapping-newer-withdrawal"))
	}
	evict()
	require.Equal(t, want, mappingConnectionEmit(t, update, destination, ctxID))
	present = false
	evict()
	require.Len(t, queries, 2)
	for _, query := range queries {
		require.Equal(t, facts.connectionID, query.ConnectionID, "query the retained mapping's original ownership")
	}
	require.NotEqual(t, want, mappingConnectionEmit(t, update, destination, ctxID), "newer withdrawal can release an obsolete mapping once its original is absent")
}

// Within one physical connection, delayed withdrawal2 cannot free generation3.
// A replay with cache ID5 retains original generation1, so genuine withdrawal4
// still releases generation3 after the old withdrawal was refused.
func TestForwardPathMappingReplayKeepsOriginalIngressOrder(t *testing.T) {
	saved := fwdPathIDs
	fwdPathIDs = newFwdPathIDTable()
	t.Cleanup(func() { fwdPathIDs = saved })
	peer, facts := sourceReceiptFixture(t)
	_, ctxID := registerForwardBodyTestContext(t, true, true)
	facts.source.ctxID = ctxID
	original := mappingConnectionReceipt(t, &facts.source, facts.connectionID)
	destination := forwardBodyTestPeer(bgpctx.Registry.Get(ctxID), ctxID)
	domain := fwdPathSource{sourceID: peer.SourceID(), peerOwner: peer.sourceOwner}
	t.Cleanup(func() { fwdPathIDs.releaseSource(domain) })
	body := pathIDTestBody(t, 65001, 23)
	first := mappingConnectionUpdate(t, original, body)
	want := mappingConnectionEmit(t, first, destination, ctxID)
	cache := newRecentUpdateCache(8)
	t.Cleanup(cache.Stop)
	cache.RegisterConsumer("mapping-ingress-order")
	add := func() *ReceivedUpdate {
		update := mappingConnectionUpdate(t, original, fwdShapeBody(fwdPathIDNLRI(23), nil, nil))
		cache.Add(update)
		cache.Activate(update.WireUpdate.MessageID(), 1)
		return update
	}
	olderWithdrawal := add()
	replacement := mappingConnectionUpdate(t, original, body)
	require.Equal(t, want, mappingConnectionEmit(t, replacement, destination, ctxID))
	currentWithdrawal := add()
	replay := mappingConnectionUpdate(t, original, body)
	replay.validationReplay = true
	replay.validationMsgID = first.sourceMessageID()
	require.Greater(t, replay.WireUpdate.MessageID(), currentWithdrawal.WireUpdate.MessageID())
	require.Equal(t, want, mappingConnectionEmit(t, replay, destination, ctxID))
	ribevents.RegisterValidationLookup(nil, nil)
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	require.NoError(t, cache.Ack(olderWithdrawal.WireUpdate.MessageID(), "mapping-ingress-order"))
	require.Equal(t, want, mappingConnectionEmit(t, replacement, destination, ctxID), "old ingress withdrawal cannot delete a replacement on the same connection")
	require.NoError(t, cache.Ack(currentWithdrawal.WireUpdate.MessageID(), "mapping-ingress-order"))
	require.NotEqual(t, want, mappingConnectionEmit(t, replacement, destination, ctxID), "a new cache ID cannot make an old replay newer than a genuine withdrawal")
}
