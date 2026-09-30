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
