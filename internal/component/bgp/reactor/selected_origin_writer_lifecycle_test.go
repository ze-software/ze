// Design: docs/architecture/update-building.md -- scoped local selected writer boundaries.
// Related: selected_origin_writer_test.go -- private issuer and real final writer fixture.
package reactor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	policy "github.com/ze-software/ze/internal/component/policy"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSelectedOriginWriterKeepsLocalExportKind gives a policy-less internal
// recipient useful local output; the learned control is refused and a later
// explicit rejection retires the local winner without manufacturing fallback.
func TestSelectedOriginWriterKeepsLocalExportKind(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	f.peer.settings.PeerAS = f.peer.settings.LocalAS
	f.peer.settings.ExportPolicy = filterapi.Policy{}
	f.peer.refreshForwardFacts()
	local, _ := f.issue(t, selectedOriginBuildInput(), 1)
	answer, err := f.send(t, t.Context(), local)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	learned := f.route(t, 2, "192.0.2.79", 0, false)
	learned.KeyNLRI = []byte{24, 198, 18, 8}
	learned.Route.NLRIHex = hex.EncodeToString(learned.KeyNLRI)
	answer, err = f.send(t, t.Context(), learned)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedSuppressed, answer.Results[0].Outcome)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 1, "the unrelated authorized local output survives learned refusal")
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[0], false))
	f.peer.settings.ExportPolicy, err = filterapi.NewPolicy(nil, policy.FinalReject)
	require.NoError(t, err)
	f.peer.refreshForwardFacts()
	local.Revision = 3
	answer, err = f.send(t, t.Context(), local)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedSuppressed, answer.Results[0].Outcome)
	bodies = updateBodies(t, f.conn.written())
	require.Len(t, bodies, 2)
	require.Empty(t, updatePrefixes(t, bodies[1], false))
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[1], true))
	require.Empty(t, f.peer.adjOut.routes)
	require.Empty(t, f.peer.adjOut.selectedPaths)
}

// TestSelectedOriginWriterReplacesAcrossOriginKinds runs both replacements
// through one selected stream, observing its final wire and last withdrawal.
func TestSelectedOriginWriterReplacesAcrossOriginKinds(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	local, _ := f.issue(t, selectedOriginBuildInput(), 1)
	_, err := f.send(t, t.Context(), local)
	require.NoError(t, err)
	learned := f.route(t, 2, "192.0.2.79", 0, false)
	answer, err := f.send(t, t.Context(), learned)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	local.Revision = 3
	answer, err = f.send(t, t.Context(), local)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	absent := rpc.SelectedRoute{Kind: rpc.SelectedRouteAbsent, Family: local.Family, KeyNLRI: local.KeyNLRI, Revision: 4}
	answer, err = f.send(t, t.Context(), absent)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedAbsent, answer.Results[0].Outcome)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 4)
	for i, asn := range []uint32{65007, 65003, 65007} {
		path, present := bodyPathAttr(t, bodies[i], 2)
		require.True(t, present)
		require.Equal(t, []uint32{65000, asn}, aspathASNs(t, path, 4))
	}
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[3], true))
	require.Empty(t, f.peer.adjOut.routes)
	require.Empty(t, f.peer.adjOut.selectedPaths)
}

// TestSelectedOriginWriterKeepsOriginalASNWidth offers two-octet AS_PATH
// input to a four-octet destination. The engine must use the original context,
// never infer width from the selected echo or the destination's negotiation.
func TestSelectedOriginWriterKeepsOriginalASNWidth(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	input := selectedOriginBuildInput()
	input.attributes = []byte{0x40, 1, 1, 0, 0x40, 2, 4, 2, 1, 0xfd, 0xef}
	original, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(false, nil))
	require.NoError(t, err)
	input.attributeContext = original
	route, _ := f.issue(t, input, 1)
	route.Route.AttrHex = "4001010040020602010000fdff"
	answer, err := f.send(t, t.Context(), route)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 1)
	path, present := bodyPathAttr(t, bodies[0], 2)
	require.True(t, present)
	require.Equal(t, []uint32{65000, 65007}, aspathASNs(t, path, 4))
}

// TestSelectedOriginWriterKeepsOriginASIntent proves the issuer's virtual-AS
// build choice survives a selected echo that carries another path entirely.
func TestSelectedOriginWriterKeepsOriginASIntent(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	input := selectedOriginBuildInput()
	input.attributes = []byte{0x40, 1, 1, 0}
	input.originAS = 64512
	route, _ := f.issue(t, input, 1)
	route.Route.AttrHex = "4001010040020602010000fdff"
	_, err := f.send(t, t.Context(), route)
	require.NoError(t, err)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 1)
	path, present := bodyPathAttr(t, bodies[0], 2)
	require.True(t, present)
	require.Equal(t, []uint32{65000, 64512}, aspathASNs(t, path, 4))
}

// selectedOriginEndpointConn supplies one controlled connection endpoint to the
// production connectedTransport publisher while the recording writer observes
// actual UPDATE bytes. It does not claim a TCP establishment handshake.
type selectedOriginEndpointConn struct {
	net.Conn
	local netip.AddrPort
}

func (connection selectedOriginEndpointConn) LocalAddr() net.Addr {
	return net.TCPAddrFromAddrPort(connection.local)
}

// TestSelectedOriginWriterResolvesOriginalSessionSelf admits self before an
// endpoint is published, then proves the later original Session endpoint, not
// the configured hint or caller echo, supplies the actual NEXT_HOP.
func TestSelectedOriginWriterResolvesOriginalSessionSelf(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	control := selectedOriginBuildInput()
	control.native = []byte{24, 198, 18, 9}
	survivor, _ := f.issue(t, control, 1)
	_, err := f.send(t, t.Context(), survivor)
	require.NoError(t, err)
	require.Len(t, updateBodies(t, f.conn.written()), 1)
	input := selectedOriginBuildInput()
	input.nextHop = bgptypes.NewNextHopSelf()
	route, _ := f.issue(t, input, 2)
	_, err = f.send(t, t.Context(), route)
	require.ErrorIs(t, err, ErrNextHopSelfNoLocal)
	require.Len(t, updateBodies(t, f.conn.written()), 1, "missing endpoint must spare the useful existing prefix")
	endpoint := selectedOriginEndpointConn{Conn: f.conn, local: netip.MustParseAddrPort("192.0.2.253:179")}
	f.peer.session.conn = endpoint
	f.peer.session.bufWriter = bufio.NewWriterSize(endpoint, 4096)
	f.peer.session.transport.Store(connectedTransport(f.peer.session.conn))
	route.Revision = 3
	_, err = f.send(t, t.Context(), route)
	require.NoError(t, err)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 2)
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.9.0/24")}, updatePrefixes(t, bodies[0], false))
	hop, present := bodyPathAttr(t, bodies[1], 3)
	require.True(t, present)
	require.Equal(t, []byte{192, 0, 2, 253}, hop)
}

// TestSelectedOriginWriterRetiresPathIdentifierZero keeps explicit identifier
// zero through a useful first announcement and retires exactly zero when the
// selected origin changes to another admitted local path identifier.
func TestSelectedOriginWriterRetiresPathIdentifierZero(t *testing.T) {
	f := newSelectedOriginFixture(t, true)
	input := selectedOriginBuildInput()
	input.hasPathID = true
	first, _ := f.issue(t, input, 1)
	_, err := f.send(t, t.Context(), first)
	require.NoError(t, err)
	input.pathID = 7
	second, _ := f.issue(t, input, 2)
	_, err = f.send(t, t.Context(), second)
	require.NoError(t, err)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 3)
	for i, want := range []uint32{0, 7} {
		offset := 4 + int(binary.BigEndian.Uint16(bodies[i][2:4]))
		require.Equal(t, want, binary.BigEndian.Uint32(bodies[i][offset:]))
		require.Equal(t, first.KeyNLRI, bodies[i][offset+4:])
	}
	withdrawn := bodies[2][2 : 2+int(binary.BigEndian.Uint16(bodies[2][:2]))]
	require.Equal(t, uint32(0), binary.BigEndian.Uint32(withdrawn))
	require.Equal(t, first.KeyNLRI, withdrawn[4:])
	require.Len(t, f.peer.adjOut.routes, 1)
	require.Len(t, f.peer.adjOut.selectedPaths, 1)
}

// TestSelectedOriginWriterRetiresOpaqueKeysIndependently exercises EVPN native
// identities through actual MP output. Removing the first RD leaves the second
// owned native path and its current output slot unchanged.
func TestSelectedOriginWriterRetiresOpaqueKeysIndependently(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	fam := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	f.peer.negotiated.Store(&NegotiatedCapabilities{families: map[family.Family]bool{family.IPv4Unicast: true, fam: true}})
	f.peer.session.negotiated = forwardFixtureNegotiation(f.peer)
	f.peer.refreshForwardFacts()
	input := selectedOriginBuildInput()
	input.family = fam
	input.native = make([]byte, 27)
	input.native[0], input.native[1], input.native[26] = 1, 25, 1
	left, _ := f.issue(t, input, 1)
	input.native = bytes.Clone(input.native)
	input.native[9] = 1
	right, _ := f.issue(t, input, 2)
	for _, route := range []rpc.SelectedRoute{left, right} {
		answer, err := f.send(t, t.Context(), route)
		require.NoError(t, err)
		require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	}
	absent := rpc.SelectedRoute{Kind: rpc.SelectedRouteAbsent, Family: fam.String(), KeyNLRI: left.KeyNLRI, Revision: 3}
	answer, err := f.send(t, t.Context(), absent)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedAbsent, answer.Results[0].Outcome)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 3)
	for i, native := range [][]byte{left.KeyNLRI, right.KeyNLRI} {
		reach, present := bodyPathAttr(t, bodies[i], 14)
		require.True(t, present)
		require.Equal(t, uint16(fam.AFI), binary.BigEndian.Uint16(reach[:2]))
		require.Equal(t, byte(fam.SAFI), reach[2])
		require.Equal(t, native, reach[5+int(reach[3]):])
	}
	withdrawn, present := bodyPathAttr(t, bodies[2], 15)
	require.True(t, present)
	require.Equal(t, left.KeyNLRI, withdrawn[3:])
	require.Len(t, f.peer.adjOut.routes, 1)
	require.Len(t, f.peer.adjOut.selectedPaths, 1)
}

// TestSelectedOriginWriterRefusesWrongAuthority establishes useful origin
// output before each refusal, then sends a distinct learned control afterward.
// A missing receipt/domain or wrong identity cannot become local admission.
func TestSelectedOriginWriterRefusesWrongAuthority(t *testing.T) {
	for _, name := range []string{"unknown-receipt", "wrong-native", "wrong-incarnation", "revoked-receipt", "missing-domain", "old-session-token"} {
		t.Run(name, func(t *testing.T) {
			f := newSelectedOriginFixture(t, false)
			input := selectedOriginBuildInput()
			input.native = []byte{24, 198, 18, 9}
			useful, _ := f.issue(t, input, 1)
			_, err := f.send(t, t.Context(), useful)
			require.NoError(t, err)
			target, value := f.issue(t, selectedOriginBuildInput(), 2)
			owner := f.owner
			token := f.peer.session.initialReplay.Load()
			switch name {
			case "unknown-receipt":
				target.OriginReceipt += 100
			case "wrong-native":
				target.KeyNLRI = []byte{24, 198, 18, 8}
			case "wrong-incarnation":
				lifetime, cancel := context.WithCancel(t.Context())
				t.Cleanup(cancel)
				owner, err = plugin.NewSelectedOwner(plugin.ProcessSender(selectedWriterProcess), lifetime.Done(), nil)
				require.NoError(t, err)
				owner, err = owner.WithConnection(f.owner.CurrentSelection, lifetime)
				require.NoError(t, err)
			case "revoked-receipt":
				require.NoError(t, f.store.withdraw(t.Context(), f.producer, f.source, value.originIdentity, f.peer))
			case "missing-domain":
				f.api.r.originReceipts = nil
			case "old-session-token":
				token++
			default:
				t.Fatal("unhandled authority fixture")
			}
			f.mu.Lock()
			f.cut = target.Revision
			f.current[string(target.KeyNLRI)] = target.Revision
			f.mu.Unlock()
			batch, err := plugin.NewSelectedBatch(rpc.SelectRoutesInput{Destination: f.peer.addrString, DestinationSession: token, Routes: []rpc.SelectedRoute{target}})
			require.NoError(t, err)
			_, err = f.api.SelectRoutes(t.Context(), batch, owner)
			require.Error(t, err)
			require.Len(t, updateBodies(t, f.conn.written()), 1, "refused input cannot replace the useful unrelated prefix")
			healthy := f.route(t, 3, "192.0.2.79", 0, false)
			healthy.KeyNLRI = []byte{24, 198, 18, 10}
			healthy.Route.NLRIHex = hex.EncodeToString(healthy.KeyNLRI)
			answer, err := f.send(t, t.Context(), healthy)
			require.NoError(t, err)
			require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
			bodies := updateBodies(t, f.conn.written())
			require.Len(t, bodies, 2)
			require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.9.0/24")}, updatePrefixes(t, bodies[0], false))
			require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.10.0/24")}, updatePrefixes(t, bodies[1], false))
		})
	}
}

// TestSelectedOriginWriterRevocationJoinsBlockedFlush holds a real socket
// flush, proves another destination completes useful origin output, then
// revokes only the original contribution and joins its interrupted writer.
func TestSelectedOriginWriterRevocationJoinsBlockedFlush(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	route, value := f.issue(t, selectedOriginBuildInput(), 1)
	other, otherConn := originRevocationScope(t, f.selectedWriterFixture, "192.0.2.81", 72)
	other.session.egressRouteFilter = other.egressGate(other.session)
	input := selectedOriginBuildInput()
	input.native = []byte{24, 198, 18, 9}
	otherValue, err := newOriginBuildValue(input)
	require.NoError(t, err)
	otherID, err := f.store.issue(t.Context(), f.producer, f.source, otherValue, other, f.owner)
	require.NoError(t, err)
	otherRoute := route
	otherRoute.KeyNLRI, otherRoute.Revision, otherRoute.OriginReceipt = otherValue.native, 2, otherID
	stored := *route.Route
	stored.NLRIHex = hex.EncodeToString(otherValue.native)
	otherRoute.Route = &stored
	writer, remote := net.Pipe()
	t.Cleanup(func() { require.NoError(t, writer.Close()); require.NoError(t, remote.Close()) })
	connection := &observedWriteConn{Conn: writer, writing: make(chan struct{})}
	f.peer.session.conn = connection
	f.peer.session.bufWriter = bufio.NewWriterSize(connection, 4096)
	work, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	joined := make(chan struct{})
	go func() { defer close(joined); _, err := f.send(t, work, route); finished <- err }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-joined:
		case <-time.After(5 * time.Second):
			t.Error("origin selected writer did not join cleanup")
		}
	})
	select {
	case <-connection.writing:
	case err := <-finished:
		t.Fatalf("origin failed before its flush: %v", err)
	case <-work.Done():
		t.Fatal("origin writer did not reach flush")
	}
	f.mu.Lock()
	f.cut = otherRoute.Revision
	f.current[string(otherRoute.KeyNLRI)] = otherRoute.Revision
	f.mu.Unlock()
	batch, err := plugin.NewSelectedBatch(rpc.SelectRoutesInput{Destination: other.addrString, DestinationSession: 72, Routes: []rpc.SelectedRoute{otherRoute}})
	require.NoError(t, err)
	healthy, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	answer, err := f.api.SelectRoutes(healthy, batch, f.owner)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	bodies := updateBodies(t, otherConn.written())
	require.Len(t, bodies, 1)
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.9.0/24")}, updatePrefixes(t, bodies[0], false))
	select {
	case err := <-finished:
		t.Fatalf("original writer stopped before revocation: %v", err)
	default:
	}
	removal, done := context.WithTimeout(t.Context(), 5*time.Second)
	defer done()
	require.NoError(t, f.store.withdraw(removal, f.producer, f.source, value.originIdentity, f.peer))
	select {
	case err := <-finished:
		require.Error(t, err)
	case <-removal.Done():
		t.Fatal("revoked origin writer did not join")
	}
	require.Empty(t, f.peer.adjOut.routes, "the interrupted frame retires its destination session ownership")
	require.Len(t, other.adjOut.routes, 1)
	f.store.mu.Lock()
	_, old := f.store.receipts[route.OriginReceipt]
	survivor := f.store.receipts[otherID]
	active := 0
	if survivor != nil {
		active = len(survivor.writes)
	}
	f.store.mu.Unlock()
	require.NotNil(t, survivor)
	require.False(t, old)
	require.Zero(t, active)
}

// TestSelectedOriginWriterSentReceiptsStayLocal observes announcement and
// retirement callbacks. Neither may acquire received-source provenance.
func TestSelectedOriginWriterSentReceiptsStayLocal(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	route, _ := f.issue(t, selectedOriginBuildInput(), 1)
	var origins []wireu.SentOrigin
	f.peer.session.onMessageReceived = func(_ netip.Addr, _ msgtype.MessageType, _ []byte, update *wireu.WireUpdate,
		_ bgpctx.ContextID, direction rpc.MessageDirection, _ BufHandle, _ map[string]any, original string, id uint64) bool {
		require.Equal(t, rpc.DirectionSent, direction)
		require.Empty(t, original)
		require.Zero(t, id)
		origin := update.SentOrigin()
		require.NotNil(t, origin)
		origins = append(origins, *origin)
		return false
	}
	_, err := f.send(t, t.Context(), route)
	require.NoError(t, err)
	absent := rpc.SelectedRoute{Kind: rpc.SelectedRouteAbsent, Family: route.Family, KeyNLRI: route.KeyNLRI, Revision: 2}
	_, err = f.send(t, t.Context(), absent)
	require.NoError(t, err)
	require.Len(t, updateBodies(t, f.conn.written()), 2)
	require.Len(t, origins, 2)
	for _, origin := range origins {
		require.True(t, origin.Local)
		require.Zero(t, origin.SourceOwner)
		require.Zero(t, origin.SourceReceipt)
		require.Zero(t, origin.SourcePolicyEpoch)
	}
}

// TestSelectedOriginWriterRechecksPublicationAtFinalWrite pauses the independent
// current query under writeMu, invalidates the exact destination publication,
// and proves its delayed local write cannot replace a useful advertised route.
func TestSelectedOriginWriterRechecksPublicationAtFinalWrite(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	route, _ := f.issue(t, selectedOriginBuildInput(), 1)
	_, err := f.send(t, t.Context(), route)
	require.NoError(t, err)
	before := bytes.Clone(f.conn.written())
	reader := f.owner
	lifetime, cancel := context.WithCancel(t.Context())
	defer cancel()
	entered, resume := make(chan struct{}), make(chan struct{})
	calls := 0
	owner, err := plugin.NewSelectedOwner(plugin.ProcessSender(selectedWriterProcess), lifetime.Done(), nil)
	require.NoError(t, err)
	owner, err = owner.WithConnection(func(ctx context.Context, input rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
		calls++
		if calls == 2 {
			close(entered)
			select {
			case <-resume:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return reader.CurrentSelection(ctx, input)
	}, lifetime)
	require.NoError(t, err)
	// Issue for this exact fresh provider, rather than rebinding an older receipt.
	f.owner = owner
	route, _ = f.issue(t, selectedOriginBuildInput(), 2)
	work, stop := context.WithTimeout(t.Context(), 10*time.Second)
	defer stop()
	result := make(chan error, 1)
	joined := make(chan struct{})
	go func() { defer close(joined); _, err := f.send(t, work, route); result <- err }()
	t.Cleanup(func() {
		stop()
		select {
		case <-joined:
		case <-time.After(5 * time.Second):
			t.Error("publication-fenced origin writer did not join")
		}
	})
	select {
	case <-entered:
	case err := <-result:
		t.Fatalf("write failed before final current check: %v", err)
	case <-work.Done():
		t.Fatal("write did not reach final current check")
	}
	facts := *f.peer.forwardFacts()
	snapshot := *facts.dissemination
	snapshot.session = &Session{}
	facts.dissemination = &snapshot
	f.peer.fwdFacts.Store(&facts)
	close(resume)
	select {
	case err := <-result:
		require.Error(t, err)
	case <-work.Done():
		t.Fatal("invalidated origin writer did not finish")
	}
	require.Equal(t, before, f.conn.written(), "delayed origin bytes must not replace the advertised predecessor")
	require.Len(t, f.peer.adjOut.routes, 1)
}

// TestSelectedOriginWriterRejectsUnissuedWireChange uses EVPN's label-excluding
// semantic key. Changed wire bytes need a fresh origin admission even when the
// semantic key is equal; a genuine replacement then reaches the wire.
func TestSelectedOriginWriterRejectsUnissuedWireChange(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	fam := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	f.peer.negotiated.Store(&NegotiatedCapabilities{families: map[family.Family]bool{family.IPv4Unicast: true, fam: true}})
	f.peer.session.negotiated = forwardFixtureNegotiation(f.peer)
	f.peer.refreshForwardFacts()
	input := selectedOriginBuildInput()
	input.family = fam
	input.native = make([]byte, 27)
	input.native[0], input.native[1], input.native[26] = 1, 25, 1
	original, _ := f.issue(t, input, 1)
	_, err := f.send(t, t.Context(), original)
	require.NoError(t, err)
	before := bytes.Clone(f.conn.written())
	altered := original
	altered.Revision = 2
	altered.KeyNLRI = bytes.Clone(original.KeyNLRI)
	altered.KeyNLRI[26] = 0x11
	_, first, err := selectedNativeKey(original)
	require.NoError(t, err)
	_, second, err := selectedNativeKey(altered)
	require.NoError(t, err)
	require.Equal(t, first, second, "this test must reach the exact-byte guard after semantic-key admission")
	_, err = f.send(t, t.Context(), altered)
	require.Error(t, err)
	require.Equal(t, before, f.conn.written())
	input.native = altered.KeyNLRI
	replacement, _ := f.issue(t, input, 3)
	answer, err := f.send(t, t.Context(), replacement)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 2)
	reach, present := bodyPathAttr(t, bodies[1], 14)
	require.True(t, present)
	require.Equal(t, replacement.KeyNLRI, reach[5+int(reach[3]):])
	require.Len(t, f.peer.adjOut.routes, 1)
}

// TestSelectedOriginWriterRunsExportGateOnce observes the existing local
// export gate while still calling its real body and requiring actual output.
func TestSelectedOriginWriterRunsExportGateOnce(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	gate := f.peer.session.egressRouteFilter
	calls := 0
	f.peer.session.egressRouteFilter = func(body []byte) (bool, []byte, [][]byte) { calls++; return gate(body) }
	route, _ := f.issue(t, selectedOriginBuildInput(), 1)
	answer, err := f.send(t, t.Context(), route)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	require.Len(t, updateBodies(t, f.conn.written()), 1)
	require.Equal(t, 1, calls, "local selected output must not skip or run its export policy twice")
}
