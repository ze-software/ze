// Design: docs/architecture/wire/messages.md — message header validation and the error it reports
// Related: rfc4271_opensent_error_peer_test.go — the running-Peer harness this file drives
// RFC: rfc/short/rfc4271.md — Section 6.1, Message Header Error Handling

package reactor

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestRFC4271MessageHeaderErrorIsReported drives a running Peer into OpenSent, then
// sends one message header that fails the Section 6.1 checks ParseHeader makes, and
// reads the NOTIFICATION the Peer writes from the far end of the socket.
//
// VALIDATES: a marker that is not all ones (all zeros, or one octet 0xFE) is answered
// by exactly one NOTIFICATION 1/1 with no Data; a Length of 18 or of 0 is answered by
// exactly one NOTIFICATION 1/2 whose Data is the two Length octets as received.
// PREVENTS: a header error that closes the connection with no NOTIFICATION, the
// wrong subcode, or a Bad Message Length that does not carry the erroneous Length.
//
// RFC requirement: RFC4271-6.1-1 positive -- every header ParseHeader refuses (all-zero marker, one 0xFE marker octet, Length 18, Length 0) is answered by exactly one NOTIFICATION with Error Code 1 (Message Header Error) before the connection closes.
// RFC requirement: RFC4271-6.1-2 positive -- a marker of all zeros and a marker with one 0xFE octet are each answered by exactly one NOTIFICATION 1/1 (Connection Not Synchronized).
// RFC requirement: RFC4271-6.1-3 positive -- a header Length of 18 and of 0 are each answered by exactly one NOTIFICATION 1/2 (Bad Message Length) whose Data is the received Length field (00 12, 00 00).
// RFC requirement: RFC4271-6.7-1 negative -- a Message Header Error (all-zero marker, one 0xFE marker octet, Length 18, Length 0) on a running Peer is answered by exactly one NOTIFICATION, and its Error Code is 1, never Cease.
func TestRFC4271MessageHeaderErrorIsReported(t *testing.T) {
	header := func(marker byte, flip int, length uint16) []byte {
		var h [message.HeaderLen]byte
		for i := range message.MarkerLen {
			h[i] = marker
		}
		if flip >= 0 {
			h[flip] = 0xFE
		}
		binary.BigEndian.PutUint16(h[16:18], length)
		h[18] = 4
		return h[:]
	}

	for _, tc := range []struct {
		name    string
		send    []byte
		subcode byte
		data    []byte
	}{
		{name: "marker all zeros", send: header(0x00, -1, message.HeaderLen), subcode: 1, data: []byte{}},
		{name: "marker one octet 0xFE", send: header(0xFF, 7, message.HeaderLen), subcode: 1, data: []byte{}},
		{name: "length 18", send: header(0xFF, -1, 18), subcode: 2, data: []byte{0x00, 0x12}},
		{name: "length 0", send: header(0xFF, -1, 0), subcode: 2, data: []byte{0x00, 0x00}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := startOpenSentNeighbor(t)

			_, err := n.conn.Write(tc.send)
			require.NoError(t, err)
			wire, closed := n.readUntil(t, 5*time.Second)

			assert.True(t, closed, "the TCP connection is dropped")
			var got [][]byte
			for _, msg := range splitWireMessages(t, wire) {
				if msg[18] == 3 {
					got = append(got, msg)
				}
			}
			require.Len(t, got, 1, "exactly one NOTIFICATION")
			require.GreaterOrEqual(t, len(got[0]), message.HeaderLen+2)
			assert.Equal(t, byte(1), got[0][19], "Error Code Message Header Error")
			assert.Equal(t, tc.subcode, got[0][20], "Error Subcode")
			assert.Equal(t, tc.data, got[0][21:], "Data")
		})
	}
}

// headerOf builds one 19-octet message header with an all-ones marker, the given
// Length field and the given Type field.
func headerOf(length uint16, msgType byte) []byte {
	var h [message.HeaderLen]byte
	for i := range message.MarkerLen {
		h[i] = 0xFF
	}
	binary.BigEndian.PutUint16(h[16:18], length)
	h[18] = msgType
	return h[:]
}

// requireOneHeaderError sends msg to the Peer behind n and asserts that the Peer
// answers with exactly one NOTIFICATION 1/subcode carrying data, then drops the
// TCP connection.
func requireOneHeaderError(t *testing.T, n *openSentNeighbor, msg []byte, subcode byte, data []byte) {
	t.Helper()
	_, err := n.conn.Write(msg)
	require.NoError(t, err)
	wire, closed := n.readUntil(t, 5*time.Second)

	assert.True(t, closed, "the TCP connection is dropped")
	var got [][]byte
	for _, one := range splitWireMessages(t, wire) {
		if one[18] == 3 {
			got = append(got, one)
		}
	}
	require.Len(t, got, 1, "exactly one NOTIFICATION")
	require.GreaterOrEqual(t, len(got[0]), message.HeaderLen+2)
	assert.Equal(t, byte(1), got[0][19], "Error Code Message Header Error")
	assert.Equal(t, subcode, got[0][20], "Error Subcode")
	assert.Equal(t, data, got[0][21:], "Data")
}

// TestRFC4271MessageHeaderBadLengthIsReported drives a running Peer into OpenSent,
// then sends one header whose Length breaks one condition of the Section 6.1 list
// (the header maximum, or the minimum of the message type it names), and reads the
// NOTIFICATION the Peer writes from the far end of the socket.
//
// VALIDATES: each of Length 4097 (above 4096, no Extended Message capability), an
// OPEN of 28, an UPDATE of 22, a KEEPALIVE of 20 and a NOTIFICATION of 20 octets is
// answered by exactly one NOTIFICATION 1/2 whose Data is the two Length octets as
// received, and the connection is dropped.
// PREVENTS: a per-type length error answered with the wrong subcode, without the
// erroneous Length, or with no NOTIFICATION at all.
//
// RFC requirement: RFC4271-6.1-1 positive -- a header Length above 4096 and a Length below the minimum of OPEN, UPDATE, KEEPALIVE (not 19) and NOTIFICATION are each answered by exactly one NOTIFICATION with Error Code 1 (Message Header Error) before the connection closes.
// RFC requirement: RFC4271-6.1-3 positive -- a header Length of 4097, an OPEN of 28, an UPDATE of 22, a KEEPALIVE of 20 and a NOTIFICATION of 20 octets are each answered by exactly one NOTIFICATION 1/2 (Bad Message Length) whose Data is the received Length field.
func TestRFC4271MessageHeaderBadLengthIsReported(t *testing.T) {
	for _, tc := range []struct {
		name   string
		length uint16
		kind   byte
	}{
		{name: "length 4097", length: message.MaxMsgLen + 1, kind: 2},
		{name: "OPEN length 28", length: message.MinOpenLen - 1, kind: 1},
		{name: "UPDATE length 22", length: message.MinUpdateLen - 1, kind: 2},
		{name: "KEEPALIVE length 20", length: message.HeaderLen + 1, kind: 4},
		{name: "NOTIFICATION length 20", length: message.MinNotificationLen - 1, kind: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := startOpenSentNeighbor(t)
			data := binary.BigEndian.AppendUint16(nil, tc.length)
			requireOneHeaderError(t, n, headerOf(tc.length, tc.kind), 2, data)
		})
	}
}

// TestRFC4271UnknownMessageTypeIsReported drives a running Peer into OpenSent, then
// sends a well-framed 19-octet header whose Type is 99, which no BGP message uses.
//
// VALIDATES: the message goes through the Peer's real read path and is answered by
// exactly one NOTIFICATION 1/3 whose Data is the one Type octet, 0x63, and the
// connection is dropped.
// PREVENTS: an unknown Type answered with subcode 0 and prose, or ignored.
//
// RFC requirement: RFC4271-6.1-1 positive -- a header whose Type field is 99 is answered by exactly one NOTIFICATION with Error Code 1 (Message Header Error) before the connection closes.
// RFC requirement: RFC4271-6.1-4 positive -- a header whose Type field is 99 is answered by exactly one NOTIFICATION 1/3 (Bad Message Type) whose Data is the erroneous Type octet 0x63.
func TestRFC4271UnknownMessageTypeIsReported(t *testing.T) {
	n := startOpenSentNeighbor(t)
	requireOneHeaderError(t, n, headerOf(message.HeaderLen, 99), 3, []byte{99})
}

// TestRFC4271MessageHeaderAtTheBoundsIsAccepted is the non-triggering input for the
// header tests: an Established Peer reads a KEEPALIVE of exactly 19 octets and an
// UPDATE of exactly the 23-octet minimum (the IPv4 unicast End-of-RIB), both of a
// recognized Type.
//
// VALIDATES: no NOTIFICATION within 500 ms, the connection stays up and the session
// stays the Peer's and Established.
// PREVENTS: a length or type check that refuses a message sitting on its bound.
//
// RFC requirement: RFC4271-6.1-1 negative -- a KEEPALIVE of Length 19 and an UPDATE of Length 23 on an Established session draw no Message Header Error and no NOTIFICATION at all within 500 ms, and the connection stays up.
// RFC requirement: RFC4271-6.1-3 negative -- a KEEPALIVE of exactly 19 octets and an UPDATE of exactly its 23-octet minimum draw no Bad Message Length and no NOTIFICATION at all within 500 ms.
// RFC requirement: RFC4271-6.1-4 negative -- messages of the recognized Types 4 (KEEPALIVE) and 2 (UPDATE) draw no Bad Message Type and no NOTIFICATION at all within 500 ms, and the session stays Established.
func TestRFC4271MessageHeaderAtTheBoundsIsAccepted(t *testing.T) {
	n := startEstablishedNeighbor(t)

	keepalive := headerOf(message.HeaderLen, 4)
	endOfRIB := append(headerOf(message.MinUpdateLen, 2), 0, 0, 0, 0)
	for _, msg := range [][]byte{keepalive, endOfRIB} {
		_, err := n.conn.Write(msg)
		require.NoError(t, err)
	}
	wire, closed := n.readUntil(t, 500*time.Millisecond)

	assert.Empty(t, notifications(t, wire), "no NOTIFICATION")
	assert.False(t, closed, "the connection stays up")
	assert.Same(t, n.session, n.peer.currentSession(), "the session stays the Peer's")
	assert.Equal(t, fsm.StateEstablished, n.session.State(), "the session stays Established")
}
