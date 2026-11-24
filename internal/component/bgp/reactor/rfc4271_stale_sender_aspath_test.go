// Design: docs/architecture/update-building.md -- explicit AS_PATH on announce-origin stale replay.
// Related: reactor_api_batch.go -- sendStaleReadvertiseUnit constructs destination facts.
// Related: stale_readvertise_chain.go -- runStaleChain admits AS_PATH edits.
// RFC 4271 Section 5.1.2 -- see rfc/short/rfc4271.md.
// RFC naming: untagged -- supplementary announce-origin explicit-policy extension coverage, not default learned-route propagation evidence.
// Session output is recorded by recordingConn, an in-memory transport.
package reactor

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

// TestStaleSenderIBGPASPathExportEdits reaches the caller that must derive its
// internal verdict from the actual peer. The retained explicit AS_PATH block
// and NLRI are borrowed input; a real initial ANNOUNCE-origin write supplies the
// replay's owner receipt. Both the opted-member and policy-change runs exercise
// the existing role guard while allowing explicit AS_PATH edits externally.
// These extension controls do not prove default learned-route propagation.
// MUTATION: set the sendStaleReadvertiseUnit destination's IsIBGP field to false;
// internal and migration-internal writes then violate the extension's role guard.
func TestStaleSenderIBGPASPathExportEdits(t *testing.T) {
	for _, rail := range []struct {
		name     string
		judgeAll bool
	}{
		{"opted-filter", false},
		{"whole-chain", true},
	} {
		for _, edit := range []struct {
			delta    string
			external []uint32
		}{
			{"as-path-prepend 2", []uint32{65000, 65000, 65000, 65001, 64512, 64496}},
			{"remove-private strip", []uint32{64496}},
		} {
			for _, role := range []struct {
				name      string
				peerAS    uint32
				migration uint32
				internal  bool
			}{
				{"internal", 65000, 0, true},
				{"migration-internal", 64999, 64999, true},
				{"external", 64999, 0, false},
			} {
				t.Run(rail.name+"/"+edit.delta+"/"+role.name, func(t *testing.T) {
					staleSenderASPathWrite(t, rail.judgeAll, edit.delta, role.peerAS,
						role.migration, role.internal, edit.external)
				})
			}
		}
	}
}

func staleSenderASPathWrite(t *testing.T, judgeAll bool, delta string, peerAS, migration uint32, internal bool, external []uint32) {
	t.Helper()
	f := newAIGPReplayFixture(t, nil)
	peer := f.destination
	peer.settings.PeerAS = peerAS
	peer.settings.MigrationAS = migration
	peer.settings.ExportPolicy = setExportNextHopUnchanged(peer.settings.ExportPolicy)
	require.Equal(t, internal, peer.IsIBGP(), "configured peer classification")
	negotiateForwardTestFamilies(peer, family.IPv4Unicast)
	ctx := bgpctx.NewEncodingContext(&capability.PeerIdentity{
		LocalASN: peer.settings.LocalAS,
		PeerASN:  peer.settings.PeerAS,
		Internal: peer.IsIBGP(),
	}, &capability.EncodingCaps{ASN4: true}, bgpctx.DirectionSend)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID
	peer.session.sendCtxID = ctxID
	peer.session.adjOut = &peer.adjOut
	peer.reactor = f.r
	peer.refreshForwardFacts()

	received := asSequence4(65001, 64512, 64496)
	attrs := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
	attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrASPath), received)...)
	attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 254})...)
	body := buildUpdatePayload(attrs, a2InlinePrefix)
	original := slices.Clone(body)
	sections, err := wire.ParseUpdateSections(body)
	require.NoError(t, err)
	prefix, err := nlri.NewWireNLRI(family.IPv4Unicast, sections.NLRI(body), false)
	require.NoError(t, err)
	batch := bgptypes.NLRIBatch{
		Family: family.IPv4Unicast, NLRIs: []nlri.NLRI{prefix},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.254")),
		Wire:    attribute.NewAttributesWire(sections.Attrs(body), f.ctxID), Stale: 1, Replay: true,
	}
	adapter := &reactorAPIAdapter{r: f.r}
	facts := announceFactsFor(peer, batch.Family, netip.MustParseAddr("192.0.2.254"), peer.IsIBGP(), peer.negotiated.Load())
	seed, err := adapter.buildBatchAnnounceUpdate(make([]byte, message.MaxMsgLen), make([]byte, message.MaxMsgLen), batch, facts)
	require.NoError(t, err)
	require.NoError(t, peer.session.SendUpdate(seed))
	batch.SentOwnerMessage = peer.session.sentReceipt.MessageID()
	require.NotZero(t, batch.SentOwnerMessage, "a real initial write supplies the replay receipt")
	require.Len(t, aigpSocketBodies(t, f.conn), 1, "the initial advertisement reached the Session writer")
	f.conn.reset()

	transport := aspathEditTransport(t, delta)
	f.r.filterTransportSeam = &staleOptedTransport{answerTransport: transport}
	peer.mu.Lock()
	peer.settings.ExportPolicy = answerPolicyAt(t, libraryIntentExport, routepolicy.FinalAccept, "aspath-edit")
	if judgeAll {
		// Use the same policy-change watermark the caller reads, rather than
		// building a staleChain or PeerFilterInfo in the test.
		peer.noteExportPolicyChangeLocked()
		f.r.filterTransportSeam = transport
	}
	peer.mu.Unlock()
	peer.refreshForwardFacts()
	require.Equal(t, judgeAll, peer.exportPolicyNewerThan(batch.SentOwnerMessage), "the actual policy watermark selects the intended stale pass")
	sent, err := adapter.sendStaleReadvertiseUnit(t.Context(), announceTarget{peer: peer, session: peer.session},
		batch, facts, message.MaxMsgLen, make([]byte, message.MaxMsgLen), make([]byte, message.MaxMsgLen))
	require.NoError(t, err)
	require.True(t, sent, "the stale sender accepts the useful advertisement")
	forwardSocketBarrier(t, f.r)
	require.Equal(t, 1, transport.calls(), "the selected stale pass runs the export member exactly once")
	bodies := aigpSocketBodies(t, f.conn)
	require.Len(t, bodies, 1, "the re-advertisement reaches the Session writer")
	got, err := wire.ParseUpdateSections(bodies[0])
	require.NoError(t, err)
	require.Empty(t, got.Withdrawn(bodies[0]), "the useful route is not withdrawn")
	require.Equal(t, a2InlinePrefix, got.NLRI(bodies[0]), "the exact retained route is re-advertised")
	_, _, path, found := attribute.AttrFind(got.Attrs(bodies[0]), attribute.AttrASPath)
	require.True(t, found, "the re-advertisement carries AS_PATH")
	want := received
	if !internal {
		want = asSequence4(external...)
	}
	require.Equal(t, want, path, "the actual stale sender preserves internal AS_PATH and edits the external control")
	require.Equal(t, original, body, "stale re-advertisement leaves the shared retained input unchanged")
}
