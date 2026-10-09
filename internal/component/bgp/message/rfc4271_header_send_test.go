package message

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// rfc4271SentMessages is one message of each type Ze sends, with the Length
// each must declare, computed from its fields here rather than from Len.
func rfc4271SentMessages() []struct {
	msg    Message
	length int
} {
	opt := []byte{0x02, 0x06, 0x01, 0x04, 0x00, 0x01, 0x00, 0x01} // Capabilities: MP IPv4 unicast
	extOpt := []byte{0x02, 0x00, 0x06, 0x01, 0x04, 0x00, 0x01, 0x00, 0x01}
	longOpt := append([]byte{0x02, 0x01, 0x2C}, bytes.Repeat([]byte{0x01, 0x04, 0x00, 0x01, 0x00, 0x01}, 50)...)
	update := &Update{
		WithdrawnRoutes: []byte{0x18, 0x0a, 0x00, 0x01},
		PathAttributes:  []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00},
		NLRI:            []byte{0x18, 0x0a, 0x00, 0x02, 0x10, 0x0b, 0x01},
	}
	return []struct {
		msg    Message
		length int
	}{
		{&Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x01020304, OptionalParams: opt}, HeaderLen + 10 + len(opt)},
		// RFC 9072 Section 2 envelope: 9 fixed octets, Non-Ext OP Len, Non-Ext OP
		// Type and the two-octet Extended Opt. Parm. Length, then the parameters.
		{&Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x01020304, OptionalParams: extOpt, ExtendedParams: true}, HeaderLen + 13 + len(extOpt)},
		{&Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x01020304, OptionalParams: longOpt}, HeaderLen + 13 + len(longOpt)},
		{update, HeaderLen + 2 + 4 + 2 + 7 + 7},
		{&Notification{ErrorCode: NotifyCease, ErrorSubcode: NotifyCeaseAdminShutdown, Data: []byte{2, 'h', 'i'}}, HeaderLen + 2 + 3},
		{NewKeepalive(), HeaderLen},
		{&RouteRefresh{AFI: 1, SAFI: 1}, HeaderLen + 4},
	}
}

// TestRFC4271EveryMessageSentWithAllOnesMarker proves the sender side of the
// marker rule for every message type Ze builds, OPEN and UPDATE included.
//
// VALIDATES: OPEN, UPDATE, NOTIFICATION, KEEPALIVE and ROUTE-REFRESH each go out
// with 16 marker octets of 0xFF, into a fresh buffer and into a reused one.
// PREVENTS: an encoder that relies on a zeroed or pre-marked buffer and leaves
// the previous occupant's octets in the marker.
//
// Method: each message is written once into a new buffer and once at offset 7
// of a buffer pre-filled with 0x00, the octet a missing marker write leaves.
//
// RFC requirement: RFC4271-4.1-1 positive -- every message type Ze sends, written into a new
// buffer, carries 16 marker octets of 0xFF.
// RFC requirement: RFC4271-4.1-1 negative -- written at offset 7 into a buffer of 0x00 octets,
// every message type still carries 16 marker octets of 0xFF, never the buffer's 0x00.
func TestRFC4271EveryMessageSentWithAllOnesMarker(t *testing.T) {
	marker := bytes.Repeat([]byte{0xFF}, MarkerLen)
	for _, sent := range rfc4271SentMessages() {
		fresh := PackTo(sent.msg, nil)
		require.Equal(t, marker, fresh[:MarkerLen], "%s marker, fresh buffer", sent.msg.Type())

		const off = 7
		reused := make([]byte, off+sent.length+16)
		n := sent.msg.WriteTo(reused, off, nil)
		require.Equal(t, sent.length, n, "%s octets written", sent.msg.Type())
		require.Equal(t, marker, reused[off:off+MarkerLen], "%s marker, reused buffer", sent.msg.Type())
	}
}

// TestRFC4271UpdatesSentWithinLengthBounds proves the sender side of the
// 19..4096 Length bound: an UPDATE too large for one message leaves the
// splitter as several, each within the bound.
//
// VALIDATES: 1500 IPv4 /24 routes (6000 octets of NLRI) under one attribute set
// are emitted as UPDATEs whose Length field and written size lie in 19..4096,
// and together carry every route once, in order.
// PREVENTS: an UPDATE above 4096 octets reaching a peer that did not negotiate
// Extended Message.
//
// Method: the input UPDATE is first shown to need more than 4096 octets, so a
// splitter that passed it through unchanged would fail. Each emitted chunk is
// written and its header read back.
//
// RFC requirement: RFC4271-4.1-3 positive -- every UPDATE the splitter emits for a 4096-octet
// ceiling declares and occupies a Length between 19 and 4096.
// RFC requirement: RFC4271-4.1-3 negative -- an input UPDATE needing more than 4096 octets is
// never emitted as one message: no emitted Length exceeds 4096, and the routes all arrive.
func TestRFC4271UpdatesSentWithinLengthBounds(t *testing.T) {
	var nlri []byte
	for i := range 1500 {
		nlri = append(nlri, 24, 10, byte(i>>8), byte(i))
	}
	u := &Update{
		PathAttributes: []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00, 0x40, 0x03, 0x04, 192, 0, 2, 1},
		NLRI:           nlri,
	}
	require.Greater(t, u.Len(nil), MaxMsgLen, "the input must not fit one message")

	var carried []byte
	err := NewSplitter().Split(u, MaxMsgLen, func(family.Family) bool { return false }, func(chunk *Update) error {
		buf := make([]byte, chunk.Len(nil))
		n := chunk.WriteTo(buf, 0, nil)
		declared := int(binary.BigEndian.Uint16(buf[MarkerLen:]))
		require.Equal(t, n, declared)
		require.GreaterOrEqual(t, declared, HeaderLen)
		require.LessOrEqual(t, declared, MaxMsgLen)
		carried = append(carried, chunk.NLRI...)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, nlri, carried)
}

// TestRFC4271EveryMessageSentWithSmallestLength proves the sender side of the
// smallest-Length rule for every message type Ze builds.
//
// VALIDATES: the Length field of OPEN, UPDATE, NOTIFICATION, KEEPALIVE and
// ROUTE-REFRESH equals the octets their fields need, which is also the number
// of octets written.
// PREVENTS: a Length taken from the buffer, a pool slot, or a padded size.
//
// Method: each message is written at offset 5 into a buffer 64 octets larger
// than it needs and pre-filled with 0xAA. The declared Length must be the
// computed smallest value, and the octets after the message must still be 0xAA.
//
// RFC requirement: RFC4271-4.1-2 positive -- each message type declares exactly the Length
// its fields need (19 for KEEPALIVE, 23 for ROUTE-REFRESH, header plus body otherwise).
// RFC requirement: RFC4271-4.1-2 negative -- written into a larger 0xAA-filled buffer, no
// message declares or writes more than that smallest Length: the octets after it stay 0xAA.
func TestRFC4271EveryMessageSentWithSmallestLength(t *testing.T) {
	for _, sent := range rfc4271SentMessages() {
		const off, spare = 5, 64
		buf := bytes.Repeat([]byte{0xAA}, off+sent.length+spare)
		n := sent.msg.WriteTo(buf, off, nil)
		require.Equal(t, sent.length, n, "%s octets written", sent.msg.Type())
		declared := int(binary.BigEndian.Uint16(buf[off+MarkerLen:]))
		require.Equal(t, sent.length, declared, "%s Length field", sent.msg.Type())
		require.Equal(t, bytes.Repeat([]byte{0xAA}, spare), buf[off+sent.length:], "%s wrote past its Length", sent.msg.Type())
	}
}
