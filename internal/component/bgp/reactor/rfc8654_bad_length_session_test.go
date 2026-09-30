package reactor

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// TestRFC8654OverLengthUpdateFollowsRFC4271ErrorHandling proves that the
// receive path treats an UPDATE longer than the negotiated limit the way RFC
// 4271 Section 6.1 prescribes: a NOTIFICATION, then the connection closed.
//
// VALIDATES: on an established session without Extended Message, an UPDATE
// header declaring 4097 octets draws Message Header Error / Bad Message Length
// whose Data is 0x1001, ReadAndProcess reports the error, and the connection
// is closed after the NOTIFICATION.
// PREVENTS: an over-length message dropped silently, answered with the wrong
// code or Data, or answered while the session stays up.
//
// RFC requirement: RFC8654-5-4 negative -- an improper (over-length) message is handled per
// RFC 4271: the NOTIFICATION is Message Header Error, Bad Message Length, Data the erroneous
// Length field 4097, and the connection closes after it.
func TestRFC8654OverLengthUpdateFollowsRFC4271ErrorHandling(t *testing.T) {
	session, client, cleanup := setupEstablishedSessionRFC2918RouteRefresh(t)
	defer cleanup()
	require.False(t, session.extendedMessage, "the fixture must not have negotiated Extended Message")

	header := append(bytes.Repeat([]byte{0xFF}, message.MarkerLen), 0x10, 0x01, byte(msgtype.TypeUPDATE))
	answer := make(chan []byte, 1)
	closed := make(chan bool, 1)
	go func() {
		_, _ = client.Write(header)
		_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 4096)
		n, _ := client.Read(buf)
		answer <- append([]byte(nil), buf[:max(n, 0)]...)
		_, err := client.Read(buf)
		closed <- errors.Is(err, io.EOF)
	}()

	err := session.ReadAndProcess()
	require.Error(t, err, "an over-length UPDATE must end the read with an error")

	select {
	case raw := <-answer:
		assertNotification(t, raw, message.NotifyMessageHeader, message.NotifyHeaderBadLength, []byte{0x10, 0x01})
	case <-time.After(5 * time.Second):
		t.Fatal("no answer from the session")
	}
	select {
	case isClosed := <-closed:
		require.True(t, isClosed, "the connection must be closed (EOF, not a read timeout) after the NOTIFICATION")
	case <-time.After(5 * time.Second):
		t.Fatal("the connection stayed open")
	}
}
