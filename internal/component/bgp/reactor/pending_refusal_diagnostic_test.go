// Design: docs/architecture/behavior/peer-lifecycle.md -- a failed pending-connection refusal remains visible after closure.
// Related: reactor_connection.go -- rejectPendingConnection owns the refusal and connection close.

package reactor

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/peer/yang"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
)

// pendingRefusalConn is used synchronously. Its zero value accepts writes and
// remains open until Close; the two error fields fail distinct writer stages.
type pendingRefusalConn struct {
	written     bytes.Buffer
	deadline    time.Time
	deadlineErr error
	writeErr    error
	writes      int
	closed      bool
}

func (c *pendingRefusalConn) Read([]byte) (int, error) { return 0, io.EOF }
func (c *pendingRefusalConn) Write(data []byte) (int, error) {
	c.writes++
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	return c.written.Write(data)
}
func (c *pendingRefusalConn) Close() error { c.closed = true; return nil }
func (c *pendingRefusalConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 179}
}
func (c *pendingRefusalConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(192, 0, 2, 99), Port: 2179}
}
func (c *pendingRefusalConn) SetDeadline(time.Time) error     { return nil }
func (c *pendingRefusalConn) SetReadDeadline(time.Time) error { return nil }
func (c *pendingRefusalConn) SetWriteDeadline(deadline time.Time) error {
	c.deadline = deadline
	return c.deadlineErr
}

// pendingRefusalLog observes the actual reactor logger at the deployed WARN
// level and the existing report bus. Tests using these globals MUST NOT run in
// parallel. Cleanup restores both before another test can run.
func pendingRefusalLog(t *testing.T) *syncBuffer {
	t.Helper()
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	sink := &syncBuffer{}
	previous := reactorLogger
	reactorLogger = func() *slog.Logger {
		return slog.New(slog.NewJSONHandler(sink, &slog.HandlerOptions{Level: slog.LevelWarn}))
	}
	t.Cleanup(func() { reactorLogger = previous })
	return sink
}

// TestPendingRefusalFailureDiagnostic forces each failure through the actual
// refusal entry point. Closure remains mandatory, and its cause must reach one
// coded warning and one doctor-visible event rather than disappear at Debug.
func TestPendingRefusalFailureDiagnostic(t *testing.T) {
	for _, deadlineFailure := range []bool{true, false} {
		name := "notification-write"
		if deadlineFailure {
			name = "write-deadline"
		}
		t.Run(name, func(t *testing.T) {
			sink := pendingRefusalLog(t)
			cause := errors.New("refusal socket operation failed")
			conn := &pendingRefusalConn{}
			wantWrites := 1
			if deadlineFailure {
				conn.deadlineErr = cause
				wantWrites = 0
			} else {
				conn.writeErr = cause
			}

			rejectPendingConnection(conn, clock.RealClock{}, message.NotifyCease, message.NotifyCeaseConnectionCollision)

			require.True(t, conn.closed, "a failed rejection must still close the pending connection")
			require.Equal(t, wantWrites, conn.writes, "a refused deadline must prevent an unbounded write")
			require.False(t, conn.deadline.IsZero(), "the normal control-write bound must be armed first")
			coded := closeRecordsWith(closeLogRecords(t, sink), "code", codePendingRefusalUndelivered)
			require.Len(t, coded, 1, "one refusal failure gives one visible coded warning")
			require.Equal(t, "WARN", coded[0]["level"])
			require.Equal(t, conn.RemoteAddr().String(), coded[0]["peer"])
			require.Equal(t, cause.Error(), coded[0]["error"])
			require.Equal(t, pendingRefusalFailureFix, coded[0]["fix"])

			events := report.Errors(0)
			require.Len(t, events, 1, "the same failure must reach show errors and show doctor once")
			require.Equal(t, reportSourceBGP, events[0].Source)
			require.Equal(t, codePendingRefusalUndelivered, events[0].Code)
			require.Equal(t, conn.RemoteAddr().String(), events[0].Subject)
			require.Equal(t, pendingRefusalFailureFix, events[0].Fix)
			require.Contains(t, events[0].Message, cause.Error())
		})
	}
}

// TestPendingRefusalSuccess preserves the exact refusal bytes and closure while
// proving a delivered refusal creates no delivery-failure warning or event.
func TestPendingRefusalSuccess(t *testing.T) {
	sink := pendingRefusalLog(t)
	conn := &pendingRefusalConn{}
	rejectPendingConnection(conn, clock.RealClock{}, message.NotifyCease, message.NotifyCeaseConnectionCollision)
	want := append(bytes.Repeat([]byte{0xff}, 16), 0, 21, 3, 6, 7)
	require.Equal(t, want, conn.written.Bytes())
	require.Equal(t, 1, conn.writes)
	require.True(t, conn.closed)
	require.False(t, conn.deadline.IsZero())
	require.Empty(t, closeLogRecords(t, sink))
	require.Empty(t, report.Errors(0))
}

// TestPendingRefusalDiagnosticRegistration ensures recent-events can render
// the raised code under its own metadata rather than an undescribed fallback.
func TestPendingRefusalDiagnosticRegistration(t *testing.T) {
	metadata := diagnostic.Lookup(codePendingRefusalUndelivered)
	require.NotNil(t, metadata)
	require.NotEmpty(t, metadata.Title)
	require.NotEmpty(t, metadata.Description)
	require.Contains(t, diagnostic.EventCodes(), codePendingRefusalUndelivered)
	require.LessOrEqual(t, len(pendingRefusalFailureFix), report.FixMax)
}

// TestPendingRefusalFixResolvesPeerList checks both refusal fixes against the
// peer plugin's registered YANG command paths. A grouping prefix cannot stand
// in for the list command, and the fix must need no omitted peer selector.
func TestPendingRefusalFixResolvesPeerList(t *testing.T) {
	schema, err := yang.DefaultLoader()
	require.NoError(t, err)
	paths := yang.WireMethodToPaths(schema)
	require.Contains(t, paths, "ze-bgp:peer-list")
	args := yang.PathToArgDefs(schema)

	var boundary *opererr.Error
	require.ErrorAs(t, announceQueueBoundaryFailure(errors.New("transport closed")), &boundary)
	for name, fix := range map[string]string{
		"pending-refusal": pendingRefusalFailureFix,
		"api-boundary":    boundary.Fix(),
	} {
		t.Run(name, func(t *testing.T) {
			path := strings.TrimPrefix(fix, "ze ")
			require.Contains(t, paths["ze-bgp:peer-list"], path,
				"the fix must resolve to the peer list command, not a grouping prefix")
			require.Empty(t, args[path], "the fix must not omit a required selector")
		})
	}
}
