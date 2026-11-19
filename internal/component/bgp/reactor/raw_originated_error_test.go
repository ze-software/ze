// Design: docs/architecture/api/update-syntax.md -- raw origination refusal
// Related: session_aigp.go -- partial split errors retain flushed output and causes
// Related: session_raw_ownership_test.go -- synchronous transport failure and close observer
// Related: rfc4271_raw_originated_next_hop_test.go -- valid negotiated raw fixtures

package reactor

import (
	"bufio"
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// TestRawOriginatedSplitErrorFlushesOwedWithdrawal proves the first split chunk
// reaches the actual connection before a later oversized field returns an error.
// Both raw forms first advertise the exact route that the failing call withdraws.
func TestRawOriginatedSplitErrorFlushesOwedWithdrawal(t *testing.T) {
	for _, fullPacket := range []bool{false, true} {
		t.Run(rawOriginatedErrorCarrierName(fullPacket), func(t *testing.T) {
			peer, conn := rawOriginatedErrorPeer(t, false)
			rawOriginatedErrorSeedLegacy(t, peer, conn, fullPacket)
			err := rawOriginatedErrorCall(peer, fullPacket, rawOriginatedOversizedMixedBody(t))
			require.ErrorIs(t, err, message.ErrAttributesTooLarge)
			rawOriginatedErrorRequireLegacyWithdrawal(t, conn)
			require.False(t, conn.closed, "a split refusal must not close a usable transport")
			require.Empty(t, peer.session.adjOut.routes, "the flushed withdrawal removes its actual local owner")

			conn.reset()
			after := []byte{24, 203, 0, 114}
			body := rawOriginatedNextHopBody(t, nil, []byte{192, 0, 2, 9}, after)
			require.NoError(t, rawOriginatedErrorCall(peer, fullPacket, body))
			bodies := aigpSocketBodies(t, &conn.recordingConn)
			require.Len(t, bodies, 1)
			sections, err := wire.ParseUpdateSections(bodies[0])
			require.NoError(t, err)
			require.Equal(t, after, sections.NLRI(bodies[0]), "the same transport remains useful after the error")
		})
	}
}

// TestRawOriginatedSplitAndFlushErrorsKeepBothCauses injects a real transport
// failure after a previously advertised route becomes the first split withdrawal.
// Its returned error must retain the split failure and the failed flush together.
func TestRawOriginatedSplitAndFlushErrorsKeepBothCauses(t *testing.T) {
	for _, fullPacket := range []bool{false, true} {
		t.Run(rawOriginatedErrorCarrierName(fullPacket), func(t *testing.T) {
			peer, conn := rawOriginatedErrorPeer(t, false)
			rawOriginatedErrorSeedLegacy(t, peer, conn, fullPacket)
			flushFailure := errors.New("test raw partial flush failed")
			conn.writeError = flushFailure
			err := rawOriginatedErrorCall(peer, fullPacket, rawOriginatedOversizedMixedBody(t))
			require.ErrorIs(t, err, message.ErrAttributesTooLarge)
			require.ErrorIs(t, err, flushFailure)
			require.Empty(t, conn.written(), "the failed transport accepted no output")
			require.True(t, conn.closed, "the actual failed transport is retired")
			cause := peer.session.closeReason.Load()
			require.NotNil(t, cause)
			require.ErrorIs(t, *cause, flushFailure, "retirement records the actual transport failure")
		})
	}
}

// TestRawOriginatedGenericSplitErrorFlushesOwedWithdrawal reaches the raw=false
// generic writer with borrowed writeBuf input. A malformed later MP announcement
// must not strand the earlier owed legacy withdrawal in the buffered writer.
func TestRawOriginatedGenericSplitErrorFlushesOwedWithdrawal(t *testing.T) {
	peer, conn := rawOriginatedErrorPeer(t, true)
	rawOriginatedErrorSeedLegacy(t, peer, conn, false)
	attrs := rawOriginatedMixedNextHopAttrs([]byte{192, 0, 2, 9},
		netip.MustParseAddr("2001:db8::2").AsSlice(), nil, []byte{129})
	body := makeUpdateBody([]byte{24, 198, 51, 100}, attrs, nil)
	sections, err := wire.ParseUpdateSections(body)
	require.NoError(t, err)
	update := message.Update{
		WithdrawnRoutes: sections.Withdrawn(body),
		PathAttributes:  sections.Attrs(body),
		NLRI:            sections.NLRI(body),
	}
	err = peer.session.writeMessage(conn, &update)
	require.ErrorIs(t, err, message.ErrNLRIMalformed)
	rawOriginatedErrorRequireLegacyWithdrawal(t, conn)
}

// TestRawOriginatedDuplicateMPRefusalTerminates supplies duplicate MP_REACH
// attributes through both real raw APIs. The malformed child remains mixed after
// splitting; it must be refused directly, without re-entering another split.
func TestRawOriginatedDuplicateMPRefusalTerminates(t *testing.T) {
	for _, fullPacket := range []bool{false, true} {
		t.Run(rawOriginatedErrorCarrierName(fullPacket), func(t *testing.T) {
			peer, conn := rawOriginatedErrorPeer(t, true)
			rawOriginatedErrorSeedLegacy(t, peer, conn, fullPacket)
			attrs := rawOriginatedMixedNextHopAttrs([]byte{192, 0, 2, 9},
				netip.MustParseAddr("2001:db8::2").AsSlice(), nil,
				[]byte{48, 0x20, 1, 0x0d, 0xb8, 0, 0x20})
			attrs = append(attrs, mixedReach(1, netip.MustParseAddr("2001:db8::9").AsSlice(),
				[]byte{48, 0x20, 1, 0x0d, 0xb8, 0, 0x30})...)
			body := makeUpdateBody([]byte{24, 198, 51, 100}, attrs, nil)
			require.NoError(t, rawOriginatedErrorCall(peer, fullPacket, body))
			rawOriginatedErrorRequireLegacyWithdrawal(t, conn)
			require.False(t, conn.closed, "nothing opaque was emitted from the refused duplicate announcement")
		})
	}
}

// TestRawOriginatedOpaquePartialErrorFlushesBeforeRetirement produces opaque
// output through actual malformed attribute bytes, without altering ownership
// state. The first MP withdrawal leaves before a later malformed NLRI errors;
// the exact session then retires with its existing raw-ownership reason.
func TestRawOriginatedOpaquePartialErrorFlushesBeforeRetirement(t *testing.T) {
	for _, fullPacket := range []bool{false, true} {
		t.Run(rawOriginatedErrorCarrierName(fullPacket), func(t *testing.T) {
			peer, conn := rawOriginatedErrorPeer(t, true)
			owed := []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 0x10}
			attrs := rawOriginatedMixedNextHopAttrs([]byte{192, 0, 2, 9},
				netip.MustParseAddr("2001:db8::9").AsSlice(), nil, owed)
			require.NoError(t, rawOriginatedErrorCall(peer, fullPacket, makeUpdateBody(nil, attrs, nil)))
			before := rawOriginatedMixedNextHopWire(t, &conn.recordingConn)
			require.Equal(t, owed, before.mpAnnounced, "the MP withdrawal is owed on this actual connection")
			require.Len(t, peer.session.adjOut.routes, 1)
			conn.reset()

			attrs = rawOriginatedMixedNextHopAttrs([]byte{192, 0, 2, 9},
				netip.MustParseAddr("2001:db8::2").AsSlice(), owed, []byte{129})
			// A complete section with a dangling attribute header is opaque to
			// ownership.filter. The splitter retains it in its base attributes.
			attrs = append(attrs, 0x40)
			err := rawOriginatedErrorCall(peer, fullPacket, makeUpdateBody(nil, attrs, nil))
			require.ErrorIs(t, err, message.ErrNLRIMalformed)
			bodies := aigpSocketBodies(t, &conn.recordingConn)
			require.Len(t, bodies, 1, "only the earlier MP withdrawal is emitted")
			sections, err := wire.ParseUpdateSections(bodies[0])
			require.NoError(t, err)
			outputAttrs := sections.Attrs(bodies[0])
			_, _, withdrawn, found := attribute.AttrFind(outputAttrs, attribute.AttrMPUnreachNLRI)
			require.True(t, found)
			require.Equal(t, append([]byte{0, 2, 1}, owed...), withdrawn)
			require.Equal(t, byte(0x40), outputAttrs[len(outputAttrs)-1], "the output actually retains the malformed tail")
			require.Empty(t, sections.NLRI(bodies[0]))
			require.True(t, conn.closed)
			require.Equal(t, conn.written(), conn.closeBytes, "opaque bytes flush before the captured transport closes")
			cause := peer.session.closeReason.Load()
			require.NotNil(t, cause)
			require.ErrorIs(t, *cause, errRawOwnership)
		})
	}
}

// rawOriginatedErrorPeer binds the existing synchronous close/write-failure
// observer before any API operation. All route ownership is created by sends.
func rawOriginatedErrorPeer(t *testing.T, ipv6Recipient bool) (*Peer, *rawOwnershipConn) {
	t.Helper()
	var peer *Peer
	if ipv6Recipient {
		peer, _ = rawOriginatedMixedNextHopPeer(t, "2001:db8::2", "2001:db8::1")
	} else {
		peer, _ = rawOriginatedMixedNextHopPeer(t, "192.0.2.2", "192.0.2.1")
	}
	conn := &rawOwnershipConn{}
	peer.session.conn = conn
	peer.session.bufWriter = bufio.NewWriterSize(conn, message.MaxMsgLen)
	t.Cleanup(peer.clearEncodingContexts)
	require.Len(t, peer.session.writeBuf.Buffer(), message.MaxMsgLen)
	return peer, conn
}

func rawOriginatedErrorCarrierName(fullPacket bool) string {
	if fullPacket {
		return "full-packet"
	}
	return "body"
}

// rawOriginatedErrorCall returns the real API error rather than asserting that
// a diagnostic request succeeded. RFC 4271 Section 4.1 defines the full header.
func rawOriginatedErrorCall(peer *Peer, fullPacket bool, body []byte) error {
	if fullPacket {
		return peer.SendRawMessage(0, rawOwnershipPacket(body, msgtype.TypeUPDATE))
	}
	return peer.SendRawMessage(uint8(msgtype.TypeUPDATE), body)
}

func rawOriginatedErrorSeedLegacy(t *testing.T, peer *Peer, conn *rawOwnershipConn, fullPacket bool) {
	t.Helper()
	body := rawOriginatedNextHopBody(t, nil, []byte{192, 0, 2, 9}, []byte{24, 198, 51, 100})
	require.NoError(t, rawOriginatedErrorCall(peer, fullPacket, body))
	bodies := aigpSocketBodies(t, &conn.recordingConn)
	require.Len(t, bodies, 1)
	sections, err := wire.ParseUpdateSections(bodies[0])
	require.NoError(t, err)
	require.Equal(t, []byte{24, 198, 51, 100}, sections.NLRI(bodies[0]))
	require.Len(t, peer.session.adjOut.routes, 1, "actual advertised ownership exists before the failing request")
	conn.reset()
}

// rawOriginatedOversizedMixedBody requires the real 4K peer to emit a legacy
// withdrawal before its MP branch discovers that the base attributes cannot fit.
// RFC 4271 Section 4.3: the optional attribute uses Flags/Type/Length(2)/Value.
func rawOriginatedOversizedMixedBody(t *testing.T) []byte {
	t.Helper()
	attrs := rawOriginatedMixedNextHopAttrs([]byte{192, 0, 2, 2},
		netip.MustParseAddr("2001:db8::9").AsSlice(), nil,
		[]byte{48, 0x20, 1, 0x0d, 0xb8, 0, 0x20})
	attrs = append(attrs, 0xd0, 200, 0x10, 0)
	attrs = append(attrs, make([]byte, 4096)...)
	body := makeUpdateBody([]byte{24, 198, 51, 100}, attrs, []byte{24, 203, 0, 113})
	require.Greater(t, len(body), message.MaxMsgLen)
	require.Less(t, len(body), message.ExtMsgLen-message.HeaderLen)
	return body
}

func rawOriginatedErrorRequireLegacyWithdrawal(t *testing.T, conn *rawOwnershipConn) {
	t.Helper()
	require.Equal(t, [][]byte{{0, 4, 24, 198, 51, 100, 0, 0}},
		aigpSocketBodies(t, &conn.recordingConn), "the exact original withdrawal must reach the wire before the API error")
}
