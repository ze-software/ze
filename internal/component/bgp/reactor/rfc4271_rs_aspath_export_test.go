// Design: docs/architecture/wire/attributes.md -- AS_PATH toward an internal peer.
// Related: forward_rs.go -- native forwarding through the destination's export chain.
// Related: rfc7311_aigp_readvertise_test.go -- recording Session and worker fixture.
// RFC 4271 Section 5.1.2 -- see rfc/short/rfc4271.md.
// The Session writers use recordingConn, an in-memory transport. These tests
// exercise no OS TCP connection or foreign peer.
package reactor

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

// rsASPathSockets sends one received route to three actual Session writers in
// one native fan-out. Internal and migration-internal peers preserve the exact
// received AS_PATH; the external peer proves the requested edit remains useful.
func rsASPathSockets(t *testing.T, received []uint32, delta string, override bool, external []uint32) {
	t.Helper()
	f := newAIGPReplayFixture(t, nil)
	f.source.settings.RSClient = true
	f.source.refreshForwardFacts()
	if delta != "" {
		f.r.filterTransportSeam = aspathEditTransport(t, delta)
		f.r.orderedEgressSteps = buildOrderedEgressSteps()
	}
	type recipient struct {
		name string
		conn *recordingConn
		want []byte
	}
	recipients := make([]recipient, 0, 3)
	for index, tc := range []struct {
		name      string
		address   string
		peerAS    uint32
		migration uint32
		want      []uint32
	}{
		{"internal", "198.18.232.2", 65000, 0, received},
		{"migration-internal", "198.18.232.3", 64999, 64999, received},
		{"external", "198.18.232.4", 65053, 0, external},
	} {
		peer, conn := f.destination, f.conn
		if index != 0 {
			peer, conn = newAnnouncePeer(t, tc.address)
			f.r.peers[peer.Settings().PeerKey()] = peer
			f.r.fwdPool.registerOutgoingPool(fwdKey{peerAddr: peer.Settings().PeerKey()}, 4096)
		}
		peer.settings.LocalAS = 65000
		peer.settings.GlobalLocalAS = 65000
		peer.settings.LocalAddress = netip.MustParseAddr("10.0.0.254")
		peer.settings.PeerAS = tc.peerAS
		peer.settings.MigrationAS = tc.migration
		peer.settings.ASOverride = override
		peer.settings.NextHopMode = NextHopUnchanged
		peer.settings.ProcessBindings = []ProcessBinding{rsRelaySendBinding()}
		peer.settings.ImportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
		peer.settings.ExportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
		if delta != "" {
			peer.settings.ExportPolicy = answerPolicyAt(t, libraryIntentExport, routepolicy.FinalAccept, "aspath-edit")
		}
		negotiateForwardTestFamilies(peer, family.IPv4Unicast)
		ctx := bgpctx.NewEncodingContext(&capability.PeerIdentity{
			LocalASN: peer.settings.LocalAS,
			PeerASN:  peer.settings.PeerAS,
			Internal: peer.settings.IsIBGP(),
		}, &capability.EncodingCaps{ASN4: true}, bgpctx.DirectionSend)
		ctxID, err := bgpctx.Registry.Register(ctx)
		require.NoError(t, err)
		peer.sendCtx.Store(ctx)
		peer.sendCtxID = ctxID
		peer.session.sendCtxID = ctxID
		peer.refreshForwardFacts()
		recipients = append(recipients, recipient{tc.name, conn, asSequence4(tc.want...)})
	}
	attrs := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
	attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrASPath), asSequence4(received...))...)
	attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 254})...)
	body := buildUpdatePayload(attrs, a2InlinePrefix)
	original := slices.Clone(body)
	id := f.receive(t, body)
	update, found := f.r.recentUpdates.Get(id)
	require.True(t, found, "the received route remains available to the native relay")
	f.r.rsForwardingEnabled = true
	skipped, dispatched := reactorForwardRS(f.r, update, id, f.source.Settings().Address, f.source)
	f.r.rsForwardingEnabled = false
	require.Empty(t, skipped, "the native relay owns every permitted destination")
	require.Equal(t, len(recipients), dispatched)
	f.source.session.flushFwdDirty()
	forwardSocketBarrier(t, f.r)
	for _, recipient := range recipients {
		t.Run(recipient.name, func(t *testing.T) {
			bodies := aigpSocketBodies(t, recipient.conn)
			require.Len(t, bodies, 1, "%s receives one useful announcement", recipient.name)
			sections, err := wire.ParseUpdateSections(bodies[0])
			require.NoError(t, err)
			require.Empty(t, sections.Withdrawn(bodies[0]), "%s is not withdrawn", recipient.name)
			require.Equal(t, a2InlinePrefix, sections.NLRI(bodies[0]), "%s receives the exact route", recipient.name)
			_, _, path, found := attribute.AttrFind(sections.Attrs(bodies[0]), attribute.AttrASPath)
			require.True(t, found, "%s receives AS_PATH", recipient.name)
			require.Equal(t, recipient.want, path, "%s AS_PATH on its Session socket", recipient.name)
		})
	}
	require.Equal(t, original, body, "native forwarding preserves the shared received bytes")
}

// TestRouteServerIBGPASPathExportEdits drives typed plugin edits through the
// native rail. RFC 4271 Section 5.1.2: "When a given BGP speaker advertises the
// route to an internal peer, the advertising speaker SHALL NOT modify the
// AS_PATH attribute associated with the route."
// MUTATION: pass false as destInternal to the native export chain; both internal
// writers then carry the forbidden prepend or private-AS removal.
// RFC requirement: RFC4271-5.1.2-2 positive -- the native route-server rail preserves the received AS_PATH [65001 64512 64496] byte for byte on both internal and migration-internal Session writers despite export prepend or private-AS removal policy.
// RFC requirement: RFC4271-5.1.2-2 negative -- the external Session writer in that same native fan-out receives the useful route with the requested export edit: prepend gives [65000 65000 65000 65001 64512 64496], and private-AS removal gives [65000 64496].
func TestRouteServerIBGPASPathExportEdits(t *testing.T) {
	for _, tc := range []struct {
		delta    string
		external []uint32
	}{
		{"as-path-prepend 2", []uint32{65000, 65000, 65000, 65001, 64512, 64496}},
		{"remove-private strip", []uint32{65000, 64496}},
	} {
		t.Run(tc.delta, func(t *testing.T) {
			rsASPathSockets(t, []uint32{65001, 64512, 64496}, tc.delta, false, tc.external)
		})
	}
}

// TestRouteServerIBGPASPathProtocolEdits checks the native rail's own prepend
// and as-override intent. The external twin proves the ordinary prepend and
// the requested replacement of its AS, while both internal paths remain exact.
// MUTATION: remove facts.isEBGP from the native ASPathIntent guard; the internal
// writers then carry an extra local-AS prepend.
// RFC requirement: RFC4271-5.1.2-2 positive -- the native route-server rail preserves the exact received AS_PATH on internal and migration-internal Session writers with protocol prepend and configured as-override, for [65001 64496] and [65001 65053] respectively.
// RFC requirement: RFC4271-5.1.2-2 negative -- the external Session writer in the same native fan-out receives the useful route with protocol prepend [65000 65001 64496], or with prepend and configured as-override [65000 65001 65000], rather than the internal preservation rule.
func TestRouteServerIBGPASPathProtocolEdits(t *testing.T) {
	for _, tc := range []struct {
		name     string
		override bool
		received []uint32
		external []uint32
	}{
		{"protocol-prepend", false, []uint32{65001, 64496}, []uint32{65000, 65001, 64496}},
		{"as-override", true, []uint32{65001, 65053}, []uint32{65000, 65001, 65000}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rsASPathSockets(t, tc.received, "", tc.override, tc.external)
		})
	}
}
