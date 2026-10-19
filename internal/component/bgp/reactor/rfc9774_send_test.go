// VALIDATES: RFC 9774 Section 3 on the send rail. An UPDATE whose AS_PATH or AS4_PATH
// holds an AS_SET or AS_CONFED_SET, handed to the one session writer, leaves as the
// withdrawal of its routes: nothing for a prefix the peer never held, a withdrawal for
// one it did. Raw injection (`send bgp ... raw`) is exempt (owner, Q-4 (b)).
// PREVENTS: advertising a set from config raw attributes, the API or a plugin, and
// leaving a stale clean route at the peer when its producer replaced it with a set.

package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// rfc9774SentBodies splits what the session wrote into UPDATE bodies.
func rfc9774SentBodies(t *testing.T, written []byte) [][]byte {
	t.Helper()
	var bodies [][]byte
	for len(written) != 0 {
		require.GreaterOrEqual(t, len(written), message.HeaderLen)
		length := int(written[16])<<8 | int(written[17])
		require.GreaterOrEqual(t, len(written), length)
		require.Equal(t, byte(msgtype.TypeUPDATE), written[18])
		bodies = append(bodies, written[message.HeaderLen:length])
		written = written[length:]
	}
	return bodies
}

// TestRFC9774SendASSetWrittenAsWithdrawal drives AC-9, AC-10 and AC-12 through
// sendRawUpdateBody, the relay and API writer that funnels into writeUpdateBody, and
// assert ze_bgp_as_set_withdrawn_total counts the withheld routes. Each
// set case is paired with the clean twin that IS written, so the guard is bound to the
// segment type. Method: a session on a recording connection, two-octet AS numbers.
func TestRFC9774SendASSetWrittenAsWithdrawal(t *testing.T) {
	prefix := []byte{24, 203, 0, 113}
	clean := rfc9774Segment(rfc9774SegSequence, 2, 65000, 65010)
	set := append(rfc9774Segment(rfc9774SegSequence, 2, 65000), rfc9774Segment(rfc9774SegSet, 2, 65010, 65011)...)
	confedSet := append(rfc9774Segment(rfc9774SegConfedSet, 2, 65100), rfc9774Segment(rfc9774SegSequence, 2, 65000)...)
	as4Set := collapseAttr(0xc0, 17, append(rfc9774Segment(rfc9774SegSequence, 4, 65000), rfc9774Segment(rfc9774SegSet, 4, 4200000001)...))

	variants := map[string][]byte{
		"AS_SET in AS_PATH":        rfc9774Attrs(set),
		"AS_CONFED_SET in AS_PATH": rfc9774Attrs(confedSet),
		"AS_SET in AS4_PATH":       rfc9774Attrs(clean, as4Set...),
		// A configured raw AS_PATH is written beside the typed one; the second
		// occurrence still puts the set in the message.
		"AS_SET in a second AS_PATH": rfc9774Attrs(clean, collapseAttr(0x40, 2, set)...),
	}
	for name, attrs := range variants {
		t.Run(name+"/never-held", func(t *testing.T) {
			peer, conn := newAnnouncePeer(t, "192.0.2.2")
			registry := newSpyRegistry()
			peer.session.prefixMetrics = initReactorMetrics(registry, "test", "192.0.2.2", "65001")
			twoPrefixes := append(append([]byte{}, prefix...), 24, 198, 51, 100)
			require.NoError(t, peer.session.sendRawUpdateBody(makeUpdateBody(nil, attrs, twoPrefixes)))
			for _, body := range rfc9774SentBodies(t, conn.written()) {
				sections, err := wire.ParseUpdateSections(body)
				require.NoError(t, err)
				assert.Empty(t, sections.NLRI(body), "the set routes must not be announced: %x", body)
			}
			counter := registry.counters["ze_bgp_as_set_withdrawn_total"]
			require.NotNil(t, counter, "ze_bgp_as_set_withdrawn_total is registered")
			assert.Equal(t, float64(2), counter.Value(), "the counter grows by the route count (AC-9)")
		})
		t.Run(name+"/held-then-replaced", func(t *testing.T) {
			peer, conn := newAnnouncePeer(t, "192.0.2.2")
			require.NoError(t, peer.session.sendRawUpdateBody(makeUpdateBody(nil, rfc9774Attrs(clean), prefix)))
			require.NoError(t, peer.session.sendRawUpdateBody(makeUpdateBody(nil, attrs, prefix)))
			bodies := rfc9774SentBodies(t, conn.written())
			require.Len(t, bodies, 2, "the clean announcement, then one withdrawal")
			sections, err := wire.ParseUpdateSections(bodies[0])
			require.NoError(t, err)
			assert.Equal(t, prefix, sections.NLRI(bodies[0]), "the clean twin is advertised")
			assert.Equal(t, makeUpdateBody(prefix, nil, nil), bodies[1], "the set variant leaves as a withdrawal")
		})
	}
}

// TestRFC9774SendMPASSetWrittenAsWithdrawal covers the MP_REACH_NLRI form: an IPv6
// unicast announcement with an AS_SET, after a clean one, leaves as an MP_UNREACH_NLRI
// for the same NLRI.
func TestRFC9774SendMPASSetWrittenAsWithdrawal(t *testing.T) {
	v6 := []byte{32, 0x20, 0x01, 0x0d, 0xb8}
	mpAttrs := func(path []byte) []byte {
		reach := []byte{0x00, 0x02, 0x01, 16}
		reach = append(reach, netip.MustParseAddr("2001:db8::1").AsSlice()...)
		reach = append(reach, 0)
		reach = append(reach, v6...)
		attrs := collapseAttr(0x40, 1, []byte{0})
		attrs = append(attrs, collapseAttr(0x40, 2, path)...)
		return append(attrs, collapseAttr(0x80, 14, reach)...)
	}
	peer, conn := newAnnouncePeer(t, "192.0.2.2")
	clean := rfc9774Segment(rfc9774SegSequence, 2, 65000)
	set := append(rfc9774Segment(rfc9774SegSequence, 2, 65000), rfc9774Segment(rfc9774SegSet, 2, 65010)...)
	require.NoError(t, peer.session.sendRawUpdateBody(makeUpdateBody(nil, mpAttrs(clean), nil)))
	require.NoError(t, peer.session.sendRawUpdateBody(makeUpdateBody(nil, mpAttrs(set), nil)))

	bodies := rfc9774SentBodies(t, conn.written())
	require.Len(t, bodies, 2)
	sections, err := wire.ParseUpdateSections(bodies[1])
	require.NoError(t, err)
	attrs := sections.Attrs(bodies[1])
	require.GreaterOrEqual(t, len(attrs), 3)
	assert.Equal(t, byte(15), attrs[1], "the withdrawal carries MP_UNREACH_NLRI only: %x", bodies[1])
	assert.True(t, bytes.HasSuffix(attrs, append([]byte{0x00, 0x02, 0x01}, v6...)), "same NLRI withdrawn: %x", attrs)
}

// TestRFC9774SendAddPathFraming proves A-4: with ADD-PATH send negotiated, the
// withdrawal that replaces a set variant names the path the peer holds. Method:
// announce each family clean with path id 7, then the set variant of the same path; the
// second write must withdraw exactly that path, in the IPv4 Withdrawn Routes field and
// in MP_UNREACH_NLRI. Unicast NLRIs are withdrawn byte for byte, so the case that binds
// the send-side ADD-PATH predicate is labeled unicast: its label is rewritten to the
// RFC 8277 Section 2.4 compatibility value, found only past the path identifier.
func TestRFC9774SendAddPathFraming(t *testing.T) {
	useAddPath := func(t *testing.T, s *Session) {
		t.Helper()
		caps := []capability.Capability{
			&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
			&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
			&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIMPLSLabel},
		}
		local := append(caps, &capability.AddPath{Families: []capability.AddPathFamily{
			{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast, Mode: capability.AddPathSend},
			{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast, Mode: capability.AddPathSend},
			{AFI: capability.AFIIPv4, SAFI: capability.SAFIMPLSLabel, Mode: capability.AddPathSend},
		}})
		remote := append(caps, &capability.AddPath{Families: []capability.AddPathFamily{
			{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast, Mode: capability.AddPathReceive},
			{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast, Mode: capability.AddPathReceive},
			{AFI: capability.AFIIPv4, SAFI: capability.SAFIMPLSLabel, Mode: capability.AddPathReceive},
		}})
		s.negotiated = capability.Negotiate(local, remote, capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001})
		ctxID, err := bgpctx.Registry.Register(bgpctx.FromNegotiatedSend(s.negotiated))
		require.NoError(t, err)
		s.sendCtxID = ctxID
	}
	clean := rfc9774Segment(rfc9774SegSequence, 2, 65000)
	set := append(rfc9774Segment(rfc9774SegSequence, 2, 65000), rfc9774Segment(rfc9774SegSet, 2, 65010)...)

	t.Run("IPv4", func(t *testing.T) {
		peer, conn := newAnnouncePeer(t, "192.0.2.2")
		useAddPath(t, peer.session)
		route := []byte{0, 0, 0, 7, 24, 203, 0, 113}
		require.NoError(t, peer.session.sendRawUpdateBody(makeUpdateBody(nil, rfc9774Attrs(clean), route)))
		require.NoError(t, peer.session.sendRawUpdateBody(makeUpdateBody(nil, rfc9774Attrs(set), route)))
		bodies := rfc9774SentBodies(t, conn.written())
		require.Len(t, bodies, 2, "the clean announcement, then one withdrawal")
		assert.Equal(t, makeUpdateBody(route, nil, nil), bodies[1], "path id 7 withdrawn: %x", bodies[1])
	})
	// The MP cases: unicast NLRIs are withdrawn as written, and a labeled NLRI is
	// rewritten to the RFC 8277 Section 2.4 compatibility label, which the
	// synthesizer can only find past the path identifier the send predicate names.
	mpCases := []struct {
		name      string
		afiSAFI   []byte
		nextHop   []byte
		route     []byte
		withdrawn []byte
	}{
		{
			name:      "IPv6 unicast",
			afiSAFI:   []byte{0x00, 0x02, 0x01},
			nextHop:   netip.MustParseAddr("2001:db8::1").AsSlice(),
			route:     []byte{0, 0, 0, 7, 32, 0x20, 0x01, 0x0d, 0xb8},
			withdrawn: []byte{0, 0, 0, 7, 32, 0x20, 0x01, 0x0d, 0xb8},
		},
		{
			name:      "IPv4 labeled unicast",
			afiSAFI:   []byte{0x00, 0x01, 0x04},
			nextHop:   []byte{192, 0, 2, 1},
			route:     []byte{0, 0, 0, 7, 48, 0x00, 0x01, 0x01, 203, 0, 113},
			withdrawn: []byte{0, 0, 0, 7, 48, 0x80, 0x00, 0x00, 203, 0, 113},
		},
	}
	for _, tc := range mpCases {
		t.Run(tc.name, func(t *testing.T) {
			mpAttrs := func(path []byte) []byte {
				reach := append(append([]byte{}, tc.afiSAFI...), byte(len(tc.nextHop)))
				reach = append(reach, tc.nextHop...)
				reach = append(reach, 0)
				reach = append(reach, tc.route...)
				attrs := collapseAttr(0x40, 1, []byte{0})
				attrs = append(attrs, collapseAttr(0x40, 2, path)...)
				return append(attrs, collapseAttr(0x80, 14, reach)...)
			}
			peer, conn := newAnnouncePeer(t, "192.0.2.2")
			useAddPath(t, peer.session)
			require.NoError(t, peer.session.sendRawUpdateBody(makeUpdateBody(nil, mpAttrs(clean), nil)))
			require.NoError(t, peer.session.sendRawUpdateBody(makeUpdateBody(nil, mpAttrs(set), nil)))
			bodies := rfc9774SentBodies(t, conn.written())
			require.Len(t, bodies, 2, "the clean announcement, then one withdrawal")
			sections, err := wire.ParseUpdateSections(bodies[1])
			require.NoError(t, err)
			attrs := sections.Attrs(bodies[1])
			require.GreaterOrEqual(t, len(attrs), 3)
			assert.Equal(t, byte(15), attrs[1], "the withdrawal carries MP_UNREACH_NLRI only: %x", bodies[1])
			assert.True(t, bytes.HasSuffix(attrs, append(append([]byte{}, tc.afiSAFI...), tc.withdrawn...)),
				"path id 7 withdrawn: %x", attrs)
		})
	}
}

// TestRFC9774SendRawUntouched proves AC-15: a full UPDATE packet sent through
// SendRawMessage (the `send bgp ... raw` path, which writes with raw=true) reaches the
// wire with its AS_SET intact, so the guard does not touch the operator's test bytes.
func TestRFC9774SendRawUntouched(t *testing.T) {
	peer, conn := newAnnouncePeer(t, "192.0.2.2")
	set := append(rfc9774Segment(rfc9774SegSequence, 2, 65000), rfc9774Segment(rfc9774SegSet, 2, 65010)...)
	body := makeUpdateBody(nil, rfc9774Attrs(set), []byte{24, 203, 0, 113})
	packet := buildUpdateMsg(body)
	require.NoError(t, peer.session.SendRawMessage(0, packet))

	bodies := rfc9774SentBodies(t, conn.written())
	require.Len(t, bodies, 1)
	sections, err := wire.ParseUpdateSections(bodies[0])
	require.NoError(t, err)
	assert.Equal(t, []byte{24, 203, 0, 113}, sections.NLRI(bodies[0]), "the raw UPDATE is announced as given")
	assert.True(t, bytes.Contains(sections.Attrs(bodies[0]), set), "the AS_SET reaches the wire")
}

// BenchmarkWriteUpdateBodyCleanPath measures the guard's cost on a body with no set
// (R-5): one attribute header walk, and it must allocate nothing.
func BenchmarkWriteUpdateBodyCleanPath(b *testing.B) {
	peer := &Session{}
	body := makeUpdateBody(nil, rfc9774Attrs(rfc9774Segment(rfc9774SegSequence, 2, 65000, 65010)), []byte{24, 203, 0, 113})
	b.ReportAllocs()
	for b.Loop() {
		if peer.transmittedPathHoldsSet(body) {
			b.Fatal("a clean body was judged to hold a set")
		}
	}
}
