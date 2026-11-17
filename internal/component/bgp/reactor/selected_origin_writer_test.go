// Design: docs/architecture/update-building.md -- issued local selected output.
// Related: selected_writer_test.go -- actual writer and published current revisions.
package reactor

import (
	"context"
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// selectedOriginFixture supplies the real private issuer and an Established
// recording Session. The trusted owner constructor models captured runtime
// authority; it is not proof of C2 runtime construction or an external provider.
type selectedOriginFixture struct {
	*selectedWriterFixture
	store    *originReceiptStore
	producer originProducer
	source   *originSource
}

func newSelectedOriginFixture(t *testing.T, addPath bool) *selectedOriginFixture {
	t.Helper()
	writer := newSelectedWriterFixture(t, addPath)
	lifetime, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	owner, err := plugin.NewSelectedOwner(plugin.ProcessSender(selectedWriterProcess), lifetime.Done(), nil)
	require.NoError(t, err)
	owner, err = owner.WithConnection(writer.owner.CurrentSelection, lifetime)
	require.NoError(t, err)
	writer.owner = owner
	store, err := newOriginReceiptStore(lifetime)
	require.NoError(t, err)
	writer.api.r.originReceipts = store
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), rpc.RequestCancelTimeout)
		defer stop()
		require.NoError(t, store.close(cleanup))
	})
	producer, err := newProcessOriginProducer(owner)
	require.NoError(t, err)
	source, err := newOriginSource(originSourceInput{kind: originSourceLocalBGP})
	require.NoError(t, err)
	writer.peer.session.egressRouteFilter = writer.peer.egressGate(writer.peer.session)
	writer.peer.session.egressFinalAccepts = func(kind filterapi.ExportKind) bool {
		facts := writer.peer.forwardFacts()
		return facts.exportPolicy.AcceptsExportAtEnd(facts.isEBGP, kind)
	}
	return &selectedOriginFixture{selectedWriterFixture: writer, store: store, producer: producer, source: source}
}

func selectedOriginBuildInput() originBuildInput {
	return originBuildInput{family: family.IPv4Unicast, native: []byte{24, 198, 18, 7},
		attributes: []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xef}, attributesExplicit: true,
		attributeContext: bgpctx.APIContextID, nextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.254"))}
}

func (f *selectedOriginFixture) issue(t *testing.T, input originBuildInput, revision uint64) (rpc.SelectedRoute, *originBuildValue) {
	t.Helper()
	value, err := newOriginBuildValue(input)
	require.NoError(t, err)
	receipt, err := f.store.issue(t.Context(), f.producer, f.source, value, f.peer, f.owner)
	require.NoError(t, err)
	route := rpc.SelectedRoute{Kind: rpc.SelectedRouteOrigin, Family: value.family.String(), KeyNLRI: value.native,
		Revision: revision, OriginReceipt: receipt, StaleLevel: rpc.SelectedFresh,
		Route: &rpc.StoredRoute{Family: value.family.String(), NLRIHex: hex.EncodeToString(value.native), NLRIFraming: rpc.NLRIFramingPrefixOnly}}
	return route, value
}

// TestSelectedOriginWriterUsesIssuedBuild exercises SelectRoutes through the
// final socket. An untrusted StoredRoute echo cannot replace the owned bytes,
// next-hop intent, or original ASN width retained at origin admission.
func TestSelectedOriginWriterUsesIssuedBuild(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	input := selectedOriginBuildInput()
	route, value := f.issue(t, input, 1)
	input.attributes[len(input.attributes)-1] = 0xff
	input.native[3] = 99
	route.Route.AttrHex = "4001010240020602010000fdff"
	route.Route.NextHopHex = "c00002fd"
	route.Route.NLRIHex = "18c61209"
	route.Route.PathID = 99
	answer, err := f.send(t, t.Context(), route)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 1)
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[0], false))
	path, present := bodyPathAttr(t, bodies[0], 2)
	require.True(t, present)
	require.Equal(t, []uint32{65000, 65007}, aspathASNs(t, path, 4))
	hop, present := bodyPathAttr(t, bodies[0], 3)
	require.True(t, present)
	require.Equal(t, []byte{192, 0, 2, 254}, hop)
	origin, present := bodyPathAttr(t, bodies[0], 1)
	require.True(t, present)
	require.Equal(t, []byte{0}, origin)
	require.Equal(t, []byte{24, 198, 18, 7}, value.native)
	require.Len(t, f.peer.adjOut.routes, 1)
	require.Len(t, f.peer.adjOut.selectedPaths, 1)
	f.store.mu.Lock()
	record := f.store.receipts[route.OriginReceipt]
	active := 0
	if record != nil {
		active = len(record.writes)
	}
	f.store.mu.Unlock()
	require.NotNil(t, record)
	require.Zero(t, active, "the final receipt must release its actual origin write lease")
}
