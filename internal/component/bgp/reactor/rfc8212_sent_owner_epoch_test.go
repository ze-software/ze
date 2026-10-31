// Design: docs/architecture/bgp/structural-forwarding.md -- source policy epoch at the final writer.
// RFC naming: untagged -- tests machinery RFC 8212 default reject relies on; rows RFC8212-3-1/3-2 are {gap}, which refuse a proof tag until the spec closes them (AC-11).
// Related: rfc8212_default_reject_test.go -- real ingress admission, dispatch and marker fixtures.
package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// TestRFC8212SentOwnerEpochFence drives a plugin's re-send of a received route
// (bgp-rib's replay, resend or abort restore) through the real update-route
// batch entry, AnnounceNLRIBatch with a sent-owner receipt, and reads the wire.
// The request's SourcePolicyEpoch is the route's original import admission: it
// reannounces only while it is nonzero and equal to the live epoch of a source
// Peer that still exists. A later marker on the same destination settles each
// negative assertion, and each refusal has a positive control beside it.
func TestRFC8212SentOwnerEpochFence(t *testing.T) {
	t.Run("live-epoch-admitted", func(t *testing.T) {
		f, first, batch := rfc8212SentOwnerFixture(t)
		batch.SourcePolicyEpoch = first.SourcePolicyEpoch
		rfc8212SentOwnerAnnounce(t, f, batch)
		rfc8212EpochMarker(t, f, "direct", 90)
		f.assertHistory(t, []byte{1, 1}, 0)
	})
	t.Run("zero-epoch-refused", func(t *testing.T) {
		f, _, batch := rfc8212SentOwnerFixture(t)
		batch.SourcePolicyEpoch = 0
		rfc8212SentOwnerAnnounce(t, f, batch)
		rfc8212EpochMarker(t, f, "direct", 90)
		f.assertHistory(t, []byte{1}, 0)
	})
	t.Run("stale-epoch-refused", func(t *testing.T) {
		f, first, _ := rfc8212SentOwnerFixture(t)
		staleEpoch := first.SourcePolicyEpoch
		f.source.advanceSourcePolicyEpoch()
		// The source re-sends under the new policy, so the destination's
		// entry is current and only the request's epoch is stale.
		body := ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 2)
		second := rfc8212EpochAdmit(t, f, body)
		rfc8212EpochDispatch(t, f, "direct", second)
		f.await(t, 2, syncOrderPrefixWire)
		batch := ownershipReplayBatch(t, f, body)
		batch.SourcePolicyEpoch = staleEpoch
		rfc8212SentOwnerAnnounce(t, f, batch)
		rfc8212EpochMarker(t, f, "direct", 90)
		f.assertHistory(t, []byte{1, 2}, 0)
		// Positive control: the same receipt under the live epoch is sent.
		batch.SourcePolicyEpoch = second.SourcePolicyEpoch
		rfc8212SentOwnerAnnounce(t, f, batch)
		rfc8212EpochMarker(t, f, "direct", 91)
		f.assertHistory(t, []byte{1, 2, 2}, 0)
	})
	t.Run("replaced-source-refused", func(t *testing.T) {
		f, first, batch := rfc8212SentOwnerFixture(t)
		rfc8212SentOwnerReplaceSource(f)
		batch.SourcePolicyEpoch = first.SourcePolicyEpoch
		rfc8212SentOwnerAnnounce(t, f, batch)
		rfc8212EpochMarker(t, f, "direct", 90)
		f.assertHistory(t, []byte{1}, 0)
	})
	t.Run("withdrawal-admitted", func(t *testing.T) {
		f, _, batch := rfc8212SentOwnerFixture(t)
		f.source.advanceSourcePolicyEpoch()
		batch.SourcePolicyEpoch = 0
		api := &reactorAPIAdapter{r: f.r}
		sel := selector.ParseDefault(f.destination.Settings().Address.String())
		if err := api.WithdrawNLRIBatch(t.Context(), sel, batch, plugin.OperatorSender()); err != nil {
			t.Fatal(err)
		}
		rfc8212EpochMarker(t, f, "direct", 90)
		f.assertHistory(t, []byte{1}, 1)
	})
	t.Run("local-origin-admitted", func(t *testing.T) {
		f := rfc8212SentOwnerRail(t)
		body := ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 5)
		parsed, err := message.UnpackUpdate(body) // RFC 4271 Section 4.3.
		if err != nil {
			t.Fatal(err)
		}
		// A local origin: the same attributes, no sent-owner receipt.
		local := adjOutBatch(syncOrderPrefix, "10.0.0.1")
		local.Wire = attribute.NewAttributesWire(parsed.PathAttributes, f.ctxID)
		rfc8212SentOwnerAnnounce(t, f, local)
		f.await(t, 5, syncOrderPrefixWire)
		replay := ownershipReplayBatch(t, f, body)
		replay.SourcePolicyEpoch = 0
		rfc8212SentOwnerAnnounce(t, f, replay)
		rfc8212EpochMarker(t, f, "direct", 90)
		f.assertHistory(t, []byte{5, 5}, 0)
	})
}

// rfc8212SentOwnerRail is the ownership rail with real ingress admission.
func rfc8212SentOwnerRail(t *testing.T) *ownershipRailFixture {
	t.Helper()
	f := ownershipRailNew(t, nil)
	f.r.config = &Config{LocalAS: 65000}
	f.r.clock = f.source.clock
	f.r.messageReceiver = &testDeliveryReceiver{consumerCount: 1}
	return f
}

// rfc8212SentOwnerFixture admits one received route through real ingress,
// forwards it to the destination, and returns the sent-owner batch bgp-rib
// would build from the destination's actual sent receipt.
func rfc8212SentOwnerFixture(t *testing.T) (*ownershipRailFixture, *ReceivedUpdate, bgptypes.NLRIBatch) {
	t.Helper()
	f := rfc8212SentOwnerRail(t)
	body := ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 1)
	first := rfc8212EpochAdmit(t, f, body)
	if first.SourcePolicyEpoch == 0 {
		t.Fatal("ingress admission recorded no source policy epoch")
	}
	rfc8212EpochDispatch(t, f, "direct", first)
	f.await(t, 1, syncOrderPrefixWire)
	return f, first, ownershipReplayBatch(t, f, body)
}

// rfc8212SentOwnerAnnounce is the update-route rail's announce call.
func rfc8212SentOwnerAnnounce(t *testing.T, f *ownershipRailFixture, batch bgptypes.NLRIBatch) {
	t.Helper()
	api := &reactorAPIAdapter{r: f.r}
	sel := selector.ParseDefault(f.destination.Settings().Address.String())
	if err := api.AnnounceNLRIBatch(t.Context(), sel, batch, plugin.OperatorSender()); err != nil {
		t.Fatal(err)
	}
}

// rfc8212SentOwnerReplaceSource leaves the state a configuration replacement
// leaves: doRemovePeer's stopWithCease marks the old Peer stopping, and a new
// Peer, with its own epoch, takes the address.
func rfc8212SentOwnerReplaceSource(f *ownershipRailFixture) {
	prior := f.source
	prior.stopping.Store(true)
	replacement := NewPeer(prior.Settings())
	replacement.state.Store(int32(PeerStateEstablished))
	replacement.advanceForwardGeneration()
	replacement.recvCtxID = f.ctxID
	replacement.sendCtxID = f.ctxID
	replacement.sendCtx.Store(prior.sendCtx.Load())
	replacement.negotiated.Store(prior.negotiated.Load())
	replacement.refreshForwardFacts()
	f.r.peers[prior.Settings().PeerKey()] = replacement
	f.source = replacement
}
