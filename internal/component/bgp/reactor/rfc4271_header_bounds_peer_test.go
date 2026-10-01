// Design: docs/architecture/wire/messages.md — ParseHeader and the per-type length checks
// Related: rfc4271_header_error_peer_test.go — the refused lengths on the same read path
// RFC: rfc/short/rfc4271.md — Section 6.1, Bad Message Length

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

// maximumLengthWithdrawal is an UPDATE of exactly 4096 octets, the largest
// Length Section 6.1 allows without the Extended Message capability: 814 /32
// withdrawals and one /16 fill the 4073-octet Withdrawn Routes field, and the
// Total Path Attribute Length is zero.
func maximumLengthWithdrawal() []byte {
	const withdrawnOctets = message.MaxMsgLen - message.MinUpdateLen
	msg := headerOf(message.MaxMsgLen, 2)
	var lengths [2]byte
	binary.BigEndian.PutUint16(lengths[:], withdrawnOctets)
	msg = append(msg, lengths[:]...)
	for i := range 814 {
		msg = append(msg, 32, 10, byte(i>>8), byte(i), 1)
	}
	// The /16 withdrawal, then a zero Total Path Attribute Length.
	return append(msg, 16, 172, 16, 0, 0)
}

// TestRFC4271MessageHeaderAtTheUpperBoundsIsAccepted sends messages whose Length
// sits exactly on a bound Section 6.1 names, where an off-by-one in the length
// check would refuse a legal message: an UPDATE of exactly 4096 octets on an
// Established session, an OPEN of exactly its 29-octet minimum in OpenSent, and a
// NOTIFICATION of exactly its 21-octet minimum in OpenSent.
//
// VALIDATES: no Bad Message Length and no NOTIFICATION at all from the Peer. The
// UPDATE leaves the session Established with the connection up; the OPEN moves the
// session to OpenConfirm with the connection up; the NOTIFICATION is taken as the
// peer's NOTIFICATION (Event 25), so the Peer closes without answering it.
// PREVENTS: a length check that is one octet too strict at 4096, at the OPEN
// minimum or at the NOTIFICATION minimum.
//
// RFC requirement: RFC4271-6.1-3 negative -- an UPDATE of exactly 4096 octets (Established), an OPEN of exactly 29 octets and a NOTIFICATION of exactly 21 octets (OpenSent) draw no Bad Message Length and no NOTIFICATION at all: the UPDATE keeps the session Established and the connection up, the OPEN reaches OpenConfirm with the connection up, and the NOTIFICATION closes the connection unanswered.
func TestRFC4271MessageHeaderAtTheUpperBoundsIsAccepted(t *testing.T) {
	t.Run("UPDATE of 4096", func(t *testing.T) {
		n := startEstablishedNeighbor(t)
		msg := maximumLengthWithdrawal()
		require.Len(t, msg, message.MaxMsgLen)

		_, err := n.conn.Write(msg)
		require.NoError(t, err)
		wire, closed := n.readUntil(t, 500*time.Millisecond)

		assert.Empty(t, notifications(t, wire), "no NOTIFICATION")
		assert.False(t, closed, "the connection stays up")
		assert.Equal(t, fsm.StateEstablished, n.session.State(), "the session stays Established")
	})

	t.Run("OPEN of 29", func(t *testing.T) {
		n := startOpenSentNeighbor(t)
		open := message.PackTo(&message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x02020202}, nil)
		require.Len(t, open, message.HeaderLen+10)

		_, err := n.conn.Write(open)
		require.NoError(t, err)
		require.Eventually(t, func() bool { return n.session.State() == fsm.StateOpenConfirm }, 5*time.Second, time.Millisecond,
			"the 29-octet OPEN is processed")
		wire, closed := n.readUntil(t, 500*time.Millisecond)

		assert.Empty(t, notifications(t, wire), "no NOTIFICATION")
		assert.False(t, closed, "the connection stays up")
	})

	t.Run("NOTIFICATION of 21", func(t *testing.T) {
		n := startOpenSentNeighbor(t)
		cease := message.PackTo(&message.Notification{ErrorCode: message.NotifyCease, ErrorSubcode: message.NotifyCeaseAdminShutdown}, nil)
		require.Len(t, cease, message.HeaderLen+2)

		_, err := n.conn.Write(cease)
		require.NoError(t, err)
		wire, closed := n.readUntil(t, 5*time.Second)

		assert.Empty(t, notifications(t, wire), "the Peer answers a NOTIFICATION with none")
		assert.True(t, closed, "the NOTIFICATION ends the connection")
	})
}
