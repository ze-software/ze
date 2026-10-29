// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owner
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection
// Related: source_policy.go -- the handler these tests drive

package rib

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// sourcePolicyRequest builds one validated lifecycle request for peer, the
// way the SDK worker admits it.
func sourcePolicyRequest(t *testing.T, op rpc.SourcePolicyOperation, txn string, peer netip.Addr, epoch, next uint64) rpc.SourcePolicyInput {
	t.Helper()
	input := rpc.SourcePolicyInput{TransactionID: txn, Operation: op,
		Source: rpc.SourcePolicyIdentity{Peer: peer.String(), PeerOwner: 1, SessionGeneration: 3,
			PolicyEpoch: epoch, ReceiveFrontier: 9, TemplateGeneration: 1},
		NextPolicyEpoch: next, DeadlineMS: time.Now().Add(time.Minute).UnixMilli()}
	require.NoError(t, input.Validate(time.Now()))
	return input
}

// runSourcePolicy calls the registered handler with a request.
func runSourcePolicy(t *testing.T, r *RIBManager, op rpc.SourcePolicyOperation, txn string, peer netip.Addr, epoch, next uint64) error {
	t.Helper()
	return r.handleSourcePolicy(context.Background(), sourcePolicyRequest(t, op, txn, peer, epoch, next))
}

// sourcePolicyFixture is a source and an alternative that both announce
// 10.1.0.0/24, a prefix only the source announces, and one forwarded copy of
// the source's shared path at a destination.
type sourcePolicyFixture struct {
	r           *RIBManager
	bus         *testEventBus
	source      netip.Addr
	alternative netip.Addr
	destination netip.Addr
	commands    []string
}

func newSourcePolicyFixture(t *testing.T) *sourcePolicyFixture {
	t.Helper()
	bus := newTestEventBus()
	f := &sourcePolicyFixture{r: newTestRIBManagerWithBus(bus), bus: bus,
		source:      netip.MustParseAddr("10.0.0.1"),
		alternative: netip.MustParseAddr("10.0.0.2"),
		destination: netip.MustParseAddr("10.0.0.3")}
	r := f.r
	r.updateHook = func(command string, _ map[string]any) { f.commands = append(f.commands, command) }
	receiveIPv4Route(t, r, f.source.String(), 100, "10.1.0.0/24", "180a0100")
	receiveIPv4Route(t, r, f.source.String(), 101, "10.2.0.0/24", "180a0200")
	receiveIPv4Route(t, r, f.alternative.String(), 102, "10.1.0.0/24", "180a0100")
	for _, peer := range []netip.Addr{f.source, f.alternative, f.destination} {
		r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: peer.String(), State: rpc.SessionStateUp, Reason: rpc.ReasonAttached})
	}
	for _, raw := range [][]byte{{24, 10, 1, 0}, {24, 10, 2, 0}} {
		r.bgpPeers[f.source].ModifyFamilyEntry(family.IPv4Unicast, raw, func(entry *storage.RouteEntry) { entry.SourcePolicyEpoch = 5 })
	}
	handle, err := pool.RibOut.Intern(nativeSentAttrs())
	require.NoError(t, err)
	r.ribOut[f.destination] = map[family.Family]map[ribOutKey]ribOutEntry{family.IPv4Unicast: {
		ribOutKey{Prefix: netip.MustParsePrefix("10.1.0.0/24")}: {MsgID: 900, AttrHandle: handle,
			NativeNLRI: string([]byte{24, 10, 1, 0}), NextHop: "10.0.0.1", SourcePeer: f.source.String(),
			SourceMessageID: 100, SourcePolicyEpoch: 5},
	}}
	return f
}

// sourceCandidates counts the source's candidates for the IPv4 prefix nlri.
func (f *sourcePolicyFixture) sourceCandidates(t *testing.T, nlri []byte) int {
	t.Helper()
	count := 0
	for _, c := range gatherCandidatesHeld(t, f.r, family.IPv4Unicast, nlri, false) {
		if c.PeerIP == f.source {
			count++
		}
	}
	return count
}

// TestRFC8212SourcePolicyRIBLifecycle drives prepare, apply, abort and
// finalize through bgp-rib's registered handler.
//
// VALIDATES: apply excludes the source's paths in place, so the alternative is
// the only candidate, the source-only prefix publishes a withdrawal, and the
// forwarded copy is withdrawn at the destination before the receipt; abort
// restores the same paths under the fresh epoch, announces the copy again
// without Route Refresh and returns it to the sent inventory; finalize after
// abort is refused; a duplicate apply writes nothing.
// PREVENTS: a denied path still elected or still advertised, and a rollback
// that loses the source's routes or needs a refresh.
func TestRFC8212SourcePolicyRIBLifecycle(t *testing.T) {
	f := newSourcePolicyFixture(t)
	r := f.r
	shared, only := []byte{24, 10, 1, 0}, []byte{24, 10, 2, 0}
	require.Equal(t, 1, f.sourceCandidates(t, shared))

	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx1", f.source, 5, 0))
	events := f.bus.eventCount()
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx1", f.source, 5, 6))
	require.Equal(t, 0, f.sourceCandidates(t, shared), "a denied path must not be a candidate")
	require.Equal(t, 0, f.sourceCandidates(t, only))
	require.Greater(t, f.bus.eventCount(), events, "apply must publish the source-only prefix's withdrawal")
	require.Len(t, f.commands, 1, "apply must withdraw the forwarded copy")
	require.Contains(t, f.commands[0], "10.1.0.0/24")
	require.Empty(t, r.ribOut[f.destination], "the withdrawn copy leaves the sent inventory")
	require.Equal(t, 1, r.bgpPeers[f.alternative].Len(), "the alternative is untouched")

	events, commands := f.bus.eventCount(), len(f.commands)
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx1", f.source, 5, 6), "duplicate apply")
	require.Equal(t, events, f.bus.eventCount(), "a duplicate apply publishes nothing")
	require.Len(t, f.commands, commands, "a duplicate apply writes nothing")

	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx1", f.source, 5, 7))
	require.Equal(t, 1, f.sourceCandidates(t, shared), "abort restores the path")
	require.Equal(t, 1, f.sourceCandidates(t, only))
	r.bgpPeers[f.source].IterateFamily(family.IPv4Unicast, func(_ []byte, entry storage.RouteEntry) bool {
		require.Equal(t, uint64(7), entry.SourcePolicyEpoch, "a restored path carries the fresh epoch")
		return true
	})
	require.Len(t, f.commands, commands+1, "abort announces the forwarded copy again")
	require.NotEqual(t, f.commands[0], f.commands[commands])
	require.Contains(t, f.commands[commands], "nlri ipv4/unicast add 180a0100")

	sent, held := r.ribOut[f.destination][family.IPv4Unicast][ribOutKey{Prefix: netip.MustParsePrefix("10.1.0.0/24")}]
	require.True(t, held, "abort returns the announced copy to the sent inventory")
	require.Equal(t, uint64(7), sent.SourcePolicyEpoch, "the restored copy carries the restored path's epoch")
	txn := r.sourcePolicy[f.source]
	require.Nil(t, txn.received, "abort releases the pins")
	require.Nil(t, txn.sent, "the inventory took the copy's reference")
	require.Error(t, runSourcePolicy(t, r, rpc.SourcePolicyFinalize, "tx1", f.source, 5, 0), "finalize after abort is refused")
	require.Error(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx1", f.source, 5, 8), "a second abort with another target is refused")
}

// TestRFC8212SourcePolicyRIBFinalizeDropsDenied proves an accepted denial
// keeps no denied state: finalize after apply removes the source's paths.
func TestRFC8212SourcePolicyRIBFinalizeDropsDenied(t *testing.T) {
	f := newSourcePolicyFixture(t)
	r := f.r
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx1", f.source, 5, 0))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx1", f.source, 5, 6))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyFinalize, "tx1", f.source, 5, 0))
	require.Equal(t, 0, r.bgpPeers[f.source].Len(), "denied paths are not retained for a later recovery")
	require.Equal(t, 1, r.bgpPeers[f.alternative].Len())
	require.Len(t, f.commands, 1, "finalize writes nothing more")
}

// TestRFC8212SourcePolicyRIBRefusesBeforeMutation proves a stale, foreign or
// unprepared token changes nothing.
func TestRFC8212SourcePolicyRIBRefusesBeforeMutation(t *testing.T) {
	f := newSourcePolicyFixture(t)
	r := f.r
	shared := []byte{24, 10, 1, 0}
	require.Error(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "old", f.source, 4, 0), "a token older than an admitted path")
	require.Nil(t, r.sourcePolicy[f.source])
	require.Error(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx1", f.source, 5, 6), "apply without prepare")
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx1", f.source, 5, 0))
	require.Error(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "other", f.source, 5, 6), "a foreign transaction")
	require.Error(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "other", f.source, 5, 0), "a second open transaction")
	foreign := sourcePolicyRequest(t, rpc.SourcePolicyApply, "tx1", f.source, 5, 6)
	foreign.Source.SessionGeneration = 4
	require.Error(t, r.handleSourcePolicy(context.Background(), foreign), "a replacement-session token")
	require.Equal(t, 1, f.sourceCandidates(t, shared), "no refusal changed eligibility")
	require.Empty(t, f.commands)

	r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: f.source.String(), State: rpc.SessionStateDown})
	require.Error(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx1", f.source, 5, 6), "the session that prepared ended")
}

// TestRFC8212SourcePolicyRIBWithdrawnNotRestored proves a path the source
// withdrew between apply and abort is not resurrected, nor its copy.
func TestRFC8212SourcePolicyRIBWithdrawnNotRestored(t *testing.T) {
	f := newSourcePolicyFixture(t)
	r := f.r
	shared := []byte{24, 10, 1, 0}
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx1", f.source, 5, 0))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx1", f.source, 5, 6))
	require.True(t, r.bgpPeers[f.source].Remove(family.IPv4Unicast, shared))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx1", f.source, 5, 7))
	require.Equal(t, 0, f.sourceCandidates(t, shared), "a withdrawn path is not restored")
	require.Equal(t, 1, f.sourceCandidates(t, []byte{24, 10, 2, 0}))
	require.Len(t, f.commands, 1, "no announcement for a withdrawn path's copy")
}

// TestRFC8212SourcePolicyRIBAddPath proves ADD-PATH identifiers zero and
// nonzero are both denied and both restored.
func TestRFC8212SourcePolicyRIBAddPath(t *testing.T) {
	r := newTestRIBManager(t)
	source := netip.MustParseAddr("10.0.0.1")
	fam := family.IPv4Unicast
	received := storage.NewPeerRIB(source.String())
	received.SetAddPath(fam, true)
	r.bgpPeers[source] = received
	r.peerUp[source] = true
	r.peerMeta[source] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	for i, raw := range [][]byte{{0, 0, 0, 0, 24, 10, 1, 0}, {0, 0, 0, 7, 24, 10, 1, 0}} {
		received.Insert(fam, nativeSentAttrs(), raw)
		received.ModifyFamilyEntry(fam, raw, func(entry *storage.RouteEntry) {
			entry.MsgID, entry.SourcePolicyEpoch = uint64(200+i), 5
		})
	}
	count := func() int { return len(gatherCandidatesHeld(t, r, fam, []byte{24, 10, 1, 0}, false)) }
	require.Equal(t, 2, count())
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx1", source, 5, 0))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx1", source, 5, 6))
	require.Equal(t, 0, count(), "identifiers zero and seven are both denied")
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx1", source, 5, 7))
	require.Equal(t, 2, count(), "identifiers zero and seven are both restored")
}

// TestRFC8212SourcePolicyRIBVPN proves a VPN (RFC 4364) source path is denied
// in favor of another PE and restored, through the structured receive rail.
func TestRFC8212SourcePolicyRIBVPN(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)
	peA := netip.MustParseAddr("192.0.2.1")
	peB := netip.MustParseAddr("192.0.2.2")
	r.peerMeta[peA] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	r.peerMeta[peB] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	r.peerUp[peA], r.peerUp[peB] = true, true
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)
	nlriA := vpnv4NLRI(100, vpnRouteKeyRD, 0x0a)
	feedReceived(r, peA, ctxID, vpnv4AnnounceBody([4]byte{10, 0, 0, 1}, 100, nlriA))
	feedReceived(r, peB, ctxID, vpnv4AnnounceBody([4]byte{10, 0, 0, 2}, 200, vpnv4NLRI(101, vpnRouteKeyRD, 0x0a)))
	countA := func() int {
		n := 0
		for _, c := range gatherCandidatesHeld(t, r, vpnv4Family, nlriA, false) {
			if c.PeerIP == peA {
				n++
			}
		}
		return n
	}
	require.Equal(t, 1, countA())
	before := len(vpnBestChanges(bus, vpnv4Family))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx1", peA, 5, 0))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx1", peA, 5, 6))
	require.Equal(t, 0, countA())
	changes := vpnBestChanges(bus, vpnv4Family)
	require.Len(t, changes, before+1, "PE A held the best on its lower MED; PE B replaces it")
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx1", peA, 5, 7))
	require.Equal(t, 1, countA())
	require.Len(t, vpnBestChanges(bus, vpnv4Family), before+2, "abort returns the best to PE A")
}
