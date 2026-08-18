// RFC: rfc/short/rfc8654.md
// Overview: peer_send.go — the send choke point these tests drive

package reactor

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

// RFC 8654 Section 4 obliges the producer of a BGP payload to take the maximum
// message size into account. Ze has two producers that do it, and both are
// reached from a peer rather than from message.UpdateBuilder:
//
//   - Peer.sendUpdateWithSplit (peer_send.go:147) — "the single choke point every
//     send passes through". It hands the UPDATE to message.Splitter.Split with the
//     peer's negotiated maximum.
//   - Peer.sendPluginRoutesVia -> sendPluginRouteGroup (peer_initial_sync.go:600,
//     :688) — it measures the concatenated group and falls back to packNLRIs
//     (peer_initial_sync.go:552) to batch the NLRIs when the group does not fit.
//
// The tests below drive those two, so they fail when either producer stops
// applying the bound. message/update_build_test.go also tags this requirement,
// on UpdateBuilder.buildUnicastWithMaxSize, which no non-test caller reaches.

// maxSizeFrame is one BGP message read back off the recording socket.
type maxSizeFrame struct {
	msgType byte
	length  int    // RFC 4271 Section 4.1 Length field, header included
	body    []byte // the octets after the 19-byte header
}

// framesOnTheWire splits a recorded byte stream into BGP messages by walking the
// RFC 4271 Section 4.1 headers. It fails the test on a stream that does not frame
// cleanly, which is itself an outcome worth catching: a producer that overruns its
// buffer leaves a message whose Length field runs past the bytes that were written.
func framesOnTheWire(t *testing.T, b []byte) []maxSizeFrame {
	t.Helper()
	var out []maxSizeFrame
	for off := 0; off < len(b); {
		require.GreaterOrEqual(t, len(b)-off, message.HeaderLen,
			"%d trailing octets are shorter than a BGP header", len(b)-off)
		length := int(binary.BigEndian.Uint16(b[off+16 : off+18]))
		require.GreaterOrEqual(t, length, message.HeaderLen,
			"message at offset %d declares length %d, under the 19-octet header", off, length)
		require.LessOrEqual(t, off+length, len(b),
			"message at offset %d declares length %d, past the %d octets on the wire", off, length, len(b))
		out = append(out, maxSizeFrame{
			msgType: b[off+18],
			length:  length,
			body:    b[off+message.HeaderLen : off+length],
		})
		off += length
	}
	return out
}

// newMaxSizePeer returns an Established peer whose Session writes into the
// returned recordingConn, so an assertion can read the octets that reached the
// socket rather than the Update the builder handed over.
//
// extended selects the peer half of RFC 8654's negotiation. When it is set, the
// session write buffer is resized exactly as session_negotiate.go:39 resizes it
// when the capability is negotiated for real; without that the buffer stays at
// 4096 and Update.WriteTo (update.go:132, copy-bounded) would silently truncate.
func newMaxSizePeer(t *testing.T, extended bool) (*Peer, *recordingConn) {
	t.Helper()
	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.2"),
		LocalAS:    65000,
		PeerAS:     65001,
		RouterID:   0x01020301,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[family.Family]bool{family.IPv4Unicast: true},
		ExtendedMessage: extended,
	})

	session := NewSession(settings)
	require.NoError(t, session.fsm.Event(fsm.EventManualStart))
	require.NoError(t, session.fsm.Event(fsm.EventTCPConnectionConfirmed))
	require.NoError(t, session.fsm.Event(fsm.EventBGPOpen))
	require.NoError(t, session.fsm.Event(fsm.EventKeepaliveMsg))
	require.Equal(t, fsm.StateEstablished, session.fsm.State())

	conn := &recordingConn{}
	session.mu.Lock()
	session.conn = conn
	session.bufWriter = bufio.NewWriterSize(conn, wire.ExtendedMaxSize)
	if extended {
		session.writeBuf.Resize(true)
	}
	session.mu.Unlock()

	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()
	return peer, conn
}

// maxSizeAttrs is the attribute set every unicast fixture below carries. The
// NEXT_HOP is 10.0.0.1 and the peer is 10.0.0.2, so RFC 4271 Section 5.1.3's
// refusal (session_write.go:394) does not withhold the route.
var maxSizeAttrs = []byte{
	0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
	0x40, 0x02, 0x00, // AS_PATH, empty
	0x40, 0x03, 0x04, 10, 0, 0, 1, // NEXT_HOP 10.0.0.1
}

// maxSizeNLRI builds n distinct IPv4 /24 NLRIs, 4 octets each.
func maxSizeNLRI(n int) []byte {
	out := make([]byte, 0, n*4)
	for i := range n {
		out = append(out, 0x18, 10, byte(i/256), byte(i%256))
	}
	return out
}

// TestSendUpdateWithSplitBoundsAnOversizePayload drives the send choke point with
// a payload larger than the peer's maximum and reads back what reached the socket.
//
// VALIDATES: Peer.sendUpdateWithSplit applies the maximum message size to a
// payload that exceeds it -- more than one UPDATE reaches the wire, none of them
// over the maximum, and every NLRI octet survives.
// PREVENTS: an oversize UPDATE reaching a peer that must reject it, and the
// silent truncation Update.WriteTo performs when the payload outgrows the session
// write buffer (update.go:164, a copy into a 4096-octet buffer).
//
// RFC requirement: RFC8654-4-2 negative -- a payload that exceeds the maximum
// message size is refused as a single message: Peer.sendUpdateWithSplit
// (peer_send.go:152) hands it to message.Splitter.Split, which bounds every
// emitted UPDATE by the peer's negotiated maximum.
func TestSendUpdateWithSplitBoundsAnOversizePayload(t *testing.T) {
	peer, conn := newMaxSizePeer(t, false)

	nlri := maxSizeNLRI(1200)
	update := &message.Update{PathAttributes: maxSizeAttrs, NLRI: nlri}

	maxSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, false))
	require.Equal(t, message.MaxMsgLen, maxSize,
		"a peer that did not negotiate Extended Message is bounded at 4096")
	require.Greater(t, message.HeaderLen+4+len(maxSizeAttrs)+len(nlri), maxSize,
		"the fixture must exceed the maximum, or this test asserts nothing")

	require.NoError(t, peer.sendUpdateWithSplit(update, maxSize, false))

	frames := framesOnTheWire(t, conn.written())
	require.Greater(t, len(frames), 1,
		"an oversize payload must reach the wire as more than one UPDATE")

	var got []byte
	for i, f := range frames {
		require.Equal(t, byte(msgtype.TypeUPDATE), f.msgType, "message %d is not an UPDATE", i)
		require.LessOrEqual(t, f.length, maxSize,
			"message %d is %d octets, over the %d maximum", i, f.length, maxSize)
		sec, err := wire.ParseUpdateSections(f.body)
		require.NoError(t, err, "message %d does not parse as an UPDATE body", i)
		require.Equal(t, maxSizeAttrs, sec.Attrs(f.body),
			"RFC 4271 Section 4.3: message %d must carry the full attribute set", i)
		got = append(got, sec.NLRI(f.body)...)
	}
	require.Equal(t, nlri, got, "every NLRI octet must survive the split, in order")
}

// TestSendUpdateWithSplitLeavesAFittingPayloadWhole is the other polarity: the
// bound is applied to what exceeds it and to nothing else.
//
// VALIDATES: a payload within the maximum reaches the wire as exactly one UPDATE,
// of exactly its own size.
// PREVENTS: a producer that splits unconditionally, which would multiply the
// message count on every send and pass an oversize-only assertion.
//
// RFC requirement: RFC8654-4-2 positive -- a payload that already takes the
// maximum message size into account is emitted unchanged: Splitter.Split's
// fits-already branch (update_split.go:153-156) emits the UPDATE as it stands.
func TestSendUpdateWithSplitLeavesAFittingPayloadWhole(t *testing.T) {
	peer, conn := newMaxSizePeer(t, false)

	nlri := maxSizeNLRI(100)
	update := &message.Update{PathAttributes: maxSizeAttrs, NLRI: nlri}

	maxSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, false))
	want := message.HeaderLen + 4 + len(maxSizeAttrs) + len(nlri)
	require.LessOrEqual(t, want, maxSize, "the fixture must fit, or this test asserts nothing")

	require.NoError(t, peer.sendUpdateWithSplit(update, maxSize, false))

	frames := framesOnTheWire(t, conn.written())
	require.Len(t, frames, 1, "a payload within the maximum must not be split")
	require.Equal(t, want, frames[0].length, "the emitted UPDATE must be exactly the payload's size")

	sec, err := wire.ParseUpdateSections(frames[0].body)
	require.NoError(t, err)
	require.Equal(t, nlri, sec.NLRI(frames[0].body))
}

// TestSendUpdateWithSplitTracksTheNegotiatedMaximum sends the payload that the
// oversize test above splits, to a peer that DID negotiate Extended Message.
//
// This is the case a hardcoded 4096 cannot pass: the same octets are within the
// maximum here and over it there, and only the negotiated capability tells the two
// apart. Message size is a property of the session, not a constant.
//
// VALIDATES: the bound Peer.sendUpdateWithSplit applies comes from
// message.MaxMessageLength over the negotiated capability, so an Extended Message
// peer receives one 65535-bounded UPDATE where a plain peer receives several.
// PREVENTS: a producer wired to RFC 4271's 4096 that would fragment needlessly on
// an RFC 8654 session, and would let 4-2 pass while ignoring the negotiation.
//
// RFC requirement: RFC8654-4-2 positive -- the maximum message size the payload is
// limited to is the NEGOTIATED one (message.MaxMessageLength, header.go:206),
// 65535 octets once the Extended Message capability is negotiated.
func TestSendUpdateWithSplitTracksTheNegotiatedMaximum(t *testing.T) {
	peer, conn := newMaxSizePeer(t, true)

	nc := peer.negotiated.Load()
	require.NotNil(t, nc)
	maxSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, nc.ExtendedMessage))
	require.Equal(t, message.ExtMsgLen, maxSize,
		"a peer that negotiated Extended Message is bounded at 65535")

	nlri := maxSizeNLRI(1200)
	update := &message.Update{PathAttributes: maxSizeAttrs, NLRI: nlri}
	want := message.HeaderLen + 4 + len(maxSizeAttrs) + len(nlri)
	require.Greater(t, want, message.MaxMsgLen, "the fixture must exceed RFC 4271's 4096")
	require.LessOrEqual(t, want, maxSize, "the fixture must fit RFC 8654's 65535")

	require.NoError(t, peer.sendUpdateWithSplit(update, maxSize, false))

	frames := framesOnTheWire(t, conn.written())
	require.Len(t, frames, 1,
		"a payload within the NEGOTIATED maximum must not be split; splitting here means the bound is a constant 4096")
	require.Equal(t, want, frames[0].length)
	require.Greater(t, frames[0].length, message.MaxMsgLen,
		"RFC 8654 Section 4: a negotiated session carries an UPDATE over 4096 octets")

	sec, err := wire.ParseUpdateSections(frames[0].body)
	require.NoError(t, err)
	require.Equal(t, nlri, sec.NLRI(frames[0].body))
}

// mvpnGroupNLRIs builds n distinct MVPN NLRIs of 16 octets each: route type 1
// (Intra-AS I-PMSI A-D), a 14-octet value whose first four octets carry the index
// so an assertion can find each one in the emitted attribute bytes.
func mvpnGroupNLRIs(n int) [][]byte {
	out := make([][]byte, 0, n)
	for i := range n {
		nlri := make([]byte, 16)
		nlri[0] = 1
		nlri[1] = 14
		binary.BigEndian.PutUint32(nlri[2:6], uint32(i)+1) //nolint:gosec // test index, bounded by n
		nlri[6] = 0xA5
		out = append(out, nlri)
	}
	return out
}

// newPluginGroupPeer returns a peer carrying nlris as grouped ipv4/mvpn plugin
// routes. Group=true is what routes them through sendPluginRouteGroup, the packer
// under test; ipv4/mvpn is registered by RegisterTestFamilies (testmain_test.go).
func newPluginGroupPeer(t *testing.T, extended bool, nlris [][]byte) *Peer {
	t.Helper()
	fam, ok := family.LookupFamily("ipv4/mvpn")
	require.True(t, ok, "ipv4/mvpn must be registered")

	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.2"),
		LocalAS:    65000,
		PeerAS:     65001,
		RouterID:   0x01020301,
	}
	for _, n := range nlris {
		settings.PluginRoutes = append(settings.PluginRoutes, PluginRoute{
			Family:  "ipv4/mvpn",
			NLRI:    n,
			NextHop: netip.MustParseAddr("10.0.0.1"),
			Group:   true,
		})
	}

	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[family.Family]bool{fam: true},
		ExtendedMessage: extended,
	})
	return peer
}

// collectPluginRoutes runs the plugin-route rail and returns a deep copy of every
// UPDATE it emitted. The copy is required: BuildPlugin's PathAttributes alias the
// pooled builder scratch, which sendPluginRouteGroup returns to the pool as soon
// as the send callback comes back (peer_initial_sync.go:698, :713).
func collectPluginRoutes(p *Peer) []*message.Update {
	var sent []*message.Update
	p.sendPluginRoutesVia(func(u *message.Update) error {
		sent = append(sent, &message.Update{
			PathAttributes: append([]byte(nil), u.PathAttributes...),
		})
		return nil
	})
	return sent
}

// TestSendPluginRoutesBoundsAnOversizeGroup drives the second producer: the
// grouped plugin-route rail, whose NLRIs share one MP_REACH and are batched by
// packNLRIs when the whole group does not fit.
//
// VALIDATES: sendPluginRoutesVia emits an oversize same-attribute group as several
// UPDATEs, each within the negotiated maximum, with every NLRI carried exactly once.
// PREVENTS: a grouped family (MVPN) emitting one UPDATE larger than the peer
// accepts, which is what the rail did before packNLRIs, and losing or duplicating
// an NLRI while batching.
//
// RFC requirement: RFC8654-4-2 negative -- a grouped payload that exceeds the
// maximum message size is refused as a single message: sendPluginRouteGroup
// (peer_initial_sync.go:706) measures the whole group and falls back to packNLRIs
// (peer_initial_sync.go:552), which starts a new batch whenever the measured
// UPDATE would pass maxSize.
func TestSendPluginRoutesBoundsAnOversizeGroup(t *testing.T) {
	nlris := mvpnGroupNLRIs(300)
	peer := newPluginGroupPeer(t, false, nlris)

	maxMsgSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, false))
	require.Greater(t, message.HeaderLen+4+len(nlris)*16, maxMsgSize,
		"the fixture must exceed the maximum, or this test asserts nothing")

	sent := collectPluginRoutes(peer)
	require.Greater(t, len(sent), 1,
		"an oversize group must reach the wire as more than one UPDATE")

	var all []byte
	for i, u := range sent {
		require.LessOrEqual(t, pluginUpdateSize(u), maxMsgSize,
			"UPDATE %d is %d octets, over the %d maximum", i, pluginUpdateSize(u), maxMsgSize)
		all = append(all, u.PathAttributes...)
	}
	for i, n := range nlris {
		require.Equal(t, 1, bytes.Count(all, n),
			"NLRI %d must appear exactly once across the emitted UPDATEs", i)
	}
}

// TestSendPluginRoutesLeavesAFittingGroupWhole is the other polarity on the same
// rail: a group within the maximum is built once and sent once.
//
// VALIDATES: the fits-already branch of sendPluginRouteGroup emits one UPDATE.
// PREVENTS: a packer that batches unconditionally, turning every grouped family
// into one UPDATE per route and passing an oversize-only assertion.
//
// RFC requirement: RFC8654-4-2 positive -- a grouped payload that already takes
// the maximum message size into account is emitted unchanged, as one UPDATE
// (peer_initial_sync.go:706-717).
func TestSendPluginRoutesLeavesAFittingGroupWhole(t *testing.T) {
	nlris := mvpnGroupNLRIs(10)
	peer := newPluginGroupPeer(t, false, nlris)

	maxMsgSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, false))

	sent := collectPluginRoutes(peer)
	require.Len(t, sent, 1, "a group within the maximum must reach the wire as one UPDATE")
	require.LessOrEqual(t, pluginUpdateSize(sent[0]), maxMsgSize)

	for i, n := range nlris {
		require.Equal(t, 1, bytes.Count(sent[0].PathAttributes, n),
			"NLRI %d must be carried by the single UPDATE", i)
	}
}

// TestSendPluginRoutesTracksTheNegotiatedMaximum sends the group that the oversize
// test above batches, to a peer that DID negotiate Extended Message.
//
// VALIDATES: the bound this rail applies is message.MaxMessageLength over the
// negotiated capability (peer_initial_sync.go:608), so the same group is one
// UPDATE here and several there.
// PREVENTS: the rail being wired to a constant 4096, which would batch an
// RFC 8654 session's grouped routes it had no need to batch.
//
// RFC requirement: RFC8654-4-2 positive -- the maximum a grouped payload is
// limited to is the NEGOTIATED one, 65535 octets once Extended Message is
// negotiated.
func TestSendPluginRoutesTracksTheNegotiatedMaximum(t *testing.T) {
	nlris := mvpnGroupNLRIs(300)
	peer := newPluginGroupPeer(t, true, nlris)

	maxMsgSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, true))
	require.Equal(t, message.ExtMsgLen, maxMsgSize)

	sent := collectPluginRoutes(peer)
	require.Len(t, sent, 1,
		"a group within the NEGOTIATED maximum must not be batched; batching here means the bound is a constant 4096")
	require.Greater(t, pluginUpdateSize(sent[0]), message.MaxMsgLen,
		"RFC 8654 Section 4: a negotiated session carries an UPDATE over 4096 octets")
	require.LessOrEqual(t, pluginUpdateSize(sent[0]), maxMsgSize)

	for i, n := range nlris {
		require.Equal(t, 1, bytes.Count(sent[0].PathAttributes, n),
			"NLRI %d must be carried by the single UPDATE", i)
	}
}
