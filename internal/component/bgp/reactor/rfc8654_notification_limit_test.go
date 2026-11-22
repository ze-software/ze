// Design: docs/architecture/edge-cases/extended-message.md -- the size of a NOTIFICATION Ze sends.
// RFC: rfc/short/rfc8654.md -- Section 5, NOTIFICATION size to a peer without capability 6.
// Related: session_write.go -- writeMessageHeld, notificationWithinPeerLimit
// Related: rfc_receive_session_dispatch_test.go -- setupCapturingSession

package reactor

import (
	"encoding/binary"
	"io"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// oversizedMPUnreach returns an IPv4 unicast MP_UNREACH_NLRI attribute, whole as
// received (flags, type code, two-octet length, value), carrying count /24
// withdrawals and a last prefix that overruns the attribute: Length 24 with one
// octet left. RFC 7606 Section 5.3 makes it incorrect, Section 3 (j) requires a
// session reset, and RFC 4760 Section 7 names that reset's NOTIFICATION, whose
// Data is the whole attribute (message.MPAttributeErrorNotification).
func oversizedMPUnreach(count int) []byte {
	value := []byte{0, 1, 1} // AFI 1 (IPv4), SAFI 1 (unicast)
	value = append(value, ipv4Slash24s(count)...)
	value = append(value, 24, 10)                             // Length 24, one octet of prefix: overruns
	attr := []byte{0x90, 15, 0, 0}                            // Optional, Extended Length; type 15 MP_UNREACH_NLRI
	binary.BigEndian.PutUint16(attr[2:4], uint16(len(value))) //nolint:gosec // count keeps it under 65,535
	return append(attr, value...)
}

// TestRFC8654NotificationToPeerWithoutExtendedMessage drives the send path every
// NOTIFICATION takes, from a received UPDATE to the octets on the socket.
//
// Goal: a NOTIFICATION Ze sends to a peer that did not advertise capability 6
// is at most 4,096 octets, and one to a peer that did may be larger.
// Method: real OPENs over a pipe bring a session to Established with Ze
// advertising capability 6 and the peer either omitting it or advertising it.
// Ze's advertisement lets the peer send a 4,832-octet UPDATE (RFC 8654 Section
// 6) holding a 4,809-octet MP_UNREACH_NLRI whose last prefix overruns it. The
// reader resets the session with UPDATE Message Error / Optional Attribute Error,
// whose Data copies the attribute, and the test reads everything Ze wrote.
//
// RFC 8654 Section 5: "However, if a NOTIFICATION is to be sent to a BGP
// speaker that has not advertised the BGP Extended Message Capability, the
// size of the message MUST NOT exceed 4,096 octets."
//
// MUTATION: return notif unchanged from notificationWithinPeerLimit: the
// 4,830-octet NOTIFICATION overruns the 4,096-octet writeBuf (a slice-bounds
// panic in writeMessageHeld) and the positive goes red.
// MUTATION: ignore ExtendedMessageSend in notificationWithinPeerLimit: the
// peer that advertised capability 6 receives a cut Data and the negative goes red.
//
// RFC requirement: RFC8654-5-3 positive -- with Ze advertising capability 6 and the peer omitting it, a received 4,832-octet UPDATE whose MP_UNREACH_NLRI overruns draws one NOTIFICATION 3/9 whose Length field and size on the wire are exactly 4,096 octets and whose Data is the first 4,075 octets of the 4,809-octet attribute.
// RFC requirement: RFC8654-5-3 negative -- when the peer advertised capability 6, the same UPDATE draws the NOTIFICATION 3/9 whole: 4,830 octets on the wire and in its Length field, its Data the entire attribute.
func TestRFC8654NotificationToPeerWithoutExtendedMessage(t *testing.T) {
	attr := oversizedMPUnreach(1200)
	require.Len(t, attr, 4809)
	frame := buildUpdateMsg(receivedUpdateBody(attr, nil))
	require.Len(t, frame, 4832)

	for _, peerExtended := range []bool{false, true} {
		t.Run("peer-extended="+strconv.FormatBool(peerExtended), func(t *testing.T) {
			session, client, capture, cleanup := setupCapturingSession(t, 65002, true, peerExtended)
			defer cleanup()

			answer := make(chan []byte, 1)
			go func() {
				// The session closes the pipe after its NOTIFICATION, which ends the read.
				data, _ := io.ReadAll(client)
				answer <- data
			}()
			written := make(chan error, 1)
			go func() {
				_, err := client.Write(frame)
				written <- err
			}()

			// RFC 8654 Section 6: Ze advertised capability 6, so the reader takes the UPDATE.
			require.Error(t, session.ReadAndProcess(), "the overrunning MP_UNREACH_NLRI resets the session")
			require.NoError(t, <-written, "Ze read the whole extended UPDATE")
			require.Empty(t, capture.all(), "the reset UPDATE reached no consumer")
			require.NotEqual(t, fsm.StateEstablished, session.State())

			got := <-answer
			want := attr
			if !peerExtended {
				want = attr[:message.MaxMsgLen-message.HeaderLen-2]
			}
			wantLen := message.HeaderLen + 2 + len(want)
			require.Len(t, got, wantLen, "Ze wrote exactly one NOTIFICATION")
			require.Equal(t, uint16(wantLen), binary.BigEndian.Uint16(got[16:18]), "NOTIFICATION Length field") //nolint:gosec // at most 65,535
			if !peerExtended {
				require.Equal(t, message.MaxMsgLen, len(got))
			}
			// RFC 4760 Section 7: UPDATE Message Error / Optional Attribute Error.
			assertNotification(t, got, message.NotifyUpdateMessage, message.NotifyUpdateOptionalAttr, want)
		})
	}
}
