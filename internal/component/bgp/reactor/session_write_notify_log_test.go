package reactor

import (
	"bytes"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// syncBuffer is a mutex-guarded log sink. The session starts background goroutines
// (keepalive/hold timers, the cancel goroutine) that log through the same
// sessionLogger(), so a bare bytes.Buffer here is a data race under -race.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// notifyLogSession builds an accepted session whose sessionLogger() writes into sink,
// and returns the conn to hand sendNotification.
func notifyLogSession(t *testing.T, sink *syncBuffer) (*Session, net.Conn) {
	t.Helper()

	lg := slog.New(slog.NewTextHandler(sink, &slog.HandlerOptions{Level: slog.LevelDebug}))
	t.Cleanup(swapSessionLogger(func() *slog.Logger { return lg }))

	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Connection = ConnectionPassive

	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	t.Cleanup(func() { _ = server.Close() })

	_ = acceptWithReader(t, session, server, client)

	// Drain whatever sendNotification puts on the wire; net.Pipe is unbuffered, so an
	// unread write blocks forever.
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := client.Read(buf); err != nil {
				return
			}
		}
	}()

	return session, server
}

// TestSendNotificationLogsCodeName pins the operator-facing half of the NOTIFICATION
// WARN line.
//
// VALIDATES: the `notification sent` WARN names the BGP error code, not its wire number.
// PREVENTS: regressing to `code=3`, which forces an operator to hand-decode RFC 4271
// Section 4.5 from a log line whose whole purpose is to say WHY the session ended.
//
// message.NotifyErrorCode already has a String() (notification.go:144), so the numeric
// form was throwing away a name that existed -- the exact hardcode-over-derive the
// project refuses (ai/rules/evidence.md), and leg 2 of what an error owes its
// reader (ai/rules/cli.md).
func TestSendNotificationLogsCodeName(t *testing.T) {
	sink := &syncBuffer{}
	session, conn := notifyLogSession(t, sink)

	require.NoError(t, session.sendNotification(
		conn, message.NotifyUpdateMessage, message.NotifyUpdateMalformedAttr, nil,
	))

	out := sink.String()
	require.Contains(t, out, `msg="notification sent"`)
	require.Contains(t, out, `code="UPDATE Message Error"`)
	// The bare wire number is what this test exists to keep out of the line.
	require.NotContains(t, out, "code=3")
}

// TestSendNotificationLogsCeaseSubcodeName pins the subcode half.
//
// VALIDATES: a Cease NOTIFICATION logs the RFC 4486 subcode name, not its number.
// PREVENTS: regressing to `subcode=2`, and pins the derivation to the SAME exported
// helper the rest of the reactor already reads Cease subcodes through
// (message.CeaseSubcodeString, used at session_handlers.go:294 and
// session_connection.go:441) rather than a second private spelling of the table.
func TestSendNotificationLogsCeaseSubcodeName(t *testing.T) {
	sink := &syncBuffer{}
	session, conn := notifyLogSession(t, sink)

	require.NoError(t, session.sendNotification(
		conn, message.NotifyCease, message.NotifyCeaseAdminShutdown, nil,
	))

	out := sink.String()
	require.Contains(t, out, "code=Cease")
	require.Contains(t, out, `subcode="Administrative Shutdown"`)
	require.NotContains(t, out, "subcode=2")
}

// TestLogNotifyErrLogsCodeName covers the sibling Debug on the send-failure path.
//
// VALIDATES: `notification send failed` names the code the same way the WARN does.
// PREVENTS: the two log lines drifting apart, so that the line an operator reads when the
// NOTIFICATION could NOT be sent is the less readable of the two.
//
// The send is forced to fail by closing the connection first, which is the situation
// logNotifyErr exists for (an error/shutdown path where the conn may already be dead).
func TestLogNotifyErrLogsCodeName(t *testing.T) {
	sink := &syncBuffer{}
	session, conn := notifyLogSession(t, sink)

	require.NoError(t, conn.Close())
	session.logNotifyErr(conn, message.NotifyCease, message.NotifyCeaseAdminShutdown, nil)

	out := sink.String()
	require.Contains(t, out, `msg="notification send failed"`)
	require.Contains(t, out, "code=Cease")
	require.Contains(t, out, `subcode="Administrative Shutdown"`)
	require.NotContains(t, out, "code=6")
}
