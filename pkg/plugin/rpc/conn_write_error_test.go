// Design: docs/architecture/api/ipc_protocol.md -- plugin RPC connection
// Related: conn.go (writeFrame, writeBatchWithDeadline, writeFailure, disarmWatchdogLocked)

package rpc

import (
	"context"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expiredDeadlineWriter is a deadline-capable writer whose every write fails as
// a write past its deadline does: with os.ErrDeadlineExceeded, which is what
// net.Conn and net.Pipe return when SetWriteDeadline expires. It lets a test
// reach that answer while the call's own context is still live.
type expiredDeadlineWriter struct{}

func (expiredDeadlineWriter) Write([]byte) (int, error) { return 0, os.ErrDeadlineExceeded }

func (expiredDeadlineWriter) Close() error { return nil }

func (expiredDeadlineWriter) SetWriteDeadline(time.Time) error { return nil }

// brokenPipeWriter is a deadline-capable writer that accepts SetWriteDeadline
// and fails every write as a TCP socket whose peer closed does: EPIPE, wrapped
// the way the net package wraps it. A real net.Pipe cannot stand in: once its
// remote end is closed, SetWriteDeadline itself fails with io.ErrClosedPipe, so
// writeFrame returns "set write deadline: %w" before it writes.
type brokenPipeWriter struct{}

func (brokenPipeWriter) Write([]byte) (int, error) {
	return 0, &net.OpError{Op: "write", Net: "tcp", Err: os.NewSyscallError("write", syscall.EPIPE)}
}

func (brokenPipeWriter) Close() error { return nil }

func (brokenPipeWriter) SetWriteDeadline(time.Time) error { return nil }

// TestConnWriteToAClosedPeerKeepsItsCause proves that a write to a socket whose
// peer closed reports that cause, even when the call carries a deadline.
//
// VALIDATES: writeFrame returns the write error wrapped as "write frame: %w",
// so CallRPC answers "send request: write frame: write tcp: write: broken pipe"
// and errors.Is finds syscall.EPIPE. It does not answer
// context.DeadlineExceeded, because no deadline passed.
// METHOD: a Conn whose writer accepts the write deadline and then fails the
// write with EPIPE, the answer of a socket whose peer closed, and a call under
// a 5 s deadline that is still live when the write fails.
// PREVENTS: a closed socket read as a timeout because the call had a deadline,
// which hides from every caller that the peer is gone.
//
// MUTATION: restoring "if hasDeadline { return context.DeadlineExceeded }" in
// writeFrame reddens both cause assertions, with "send request: context
// deadline exceeded".
func TestConnWriteToAClosedPeerKeepsItsCause(t *testing.T) {
	t.Parallel()

	conn := NewConn(newBlockingReader(), brokenPipeWriter{})
	defer closeConn(t, conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := conn.CallRPC(ctx, "never-sent", nil)
	require.Error(t, err)
	require.NoError(t, ctx.Err(), "the context must still be live, or ctx.Err decided the answer")
	assert.ErrorIs(t, err, syscall.EPIPE, "the broken pipe must survive as the cause")
	assert.NotErrorIs(t, err, context.DeadlineExceeded, "no deadline passed, so the error is not a timeout")
}

// TestConnWritePastItsDeadlineIsATimeout proves that a write the transport ends
// at its write deadline still answers context.DeadlineExceeded, on the deadline
// transport and on the watchdog transport alike.
//
// VALIDATES: the two forms writeFrame reads as a timeout under a context
// deadline: a write error that is os.ErrDeadlineExceeded (SetWriteDeadline
// expired), and a write the watchdog ended because it stalled past its window
// (fireWatchdog closes the writer, and the write then fails with the writer's
// close error).
// METHOD: each case calls SendOK under a 5 s deadline, so the context is still
// live when the write fails and only writeFrame's reading of the write decides
// the answer. The deadline case uses a writer that answers
// os.ErrDeadlineExceeded; the watchdog case uses the blocking writer of
// conn_watchdog_test.go with a 30 ms window, which returns io.ErrClosedPipe once
// the watchdog closed it.
// PREVENTS: a fix for the closed-pipe case that stops reporting a real timeout.
//
// MUTATION: dropping the os.ErrDeadlineExceeded test in writeFrame reddens the
// deadline case; dropping the watchdog-fired test reddens the watchdog case.
func TestConnWritePastItsDeadlineIsATimeout(t *testing.T) {
	t.Run("write deadline expired", func(t *testing.T) {
		conn := NewConn(newBlockingReader(), expiredDeadlineWriter{})
		defer closeConn(t, conn)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := conn.SendOK(ctx, 1)
		require.Error(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})
	t.Run("watchdog fired", func(t *testing.T) {
		writer := newBlockingWriter()
		conn := NewConn(newBlockingReader(), writer)
		conn.SetWatchdogWindow(30 * time.Millisecond)
		if _, ok := any(writer).(writeDeadliner); ok {
			t.Fatal("blockingWriter must not implement SetWriteDeadline")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := conn.SendOK(ctx, 1)
		require.Error(t, err)
		require.NoError(t, ctx.Err(), "the context must still be live, or ctx.Err decided the answer")
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})
}

// batchEvents is one deliver-batch event, enough to put a frame on the wire.
var batchEvents = [][]byte{[]byte(`{"type":"bgp"}`)}

// TestConnBatchWriteToAClosedPeerKeepsItsCause proves that a deliver-batch write
// to a socket whose peer closed reports that cause, as CallRPC does on the same
// connection, even when the call carries a deadline.
//
// VALIDATES: writeBatchWithDeadline hands the failure to writeFailure, so
// CallBatchRPC answers exactly "send batch: write batch frame: write tcp: write:
// broken pipe", errors.Is finds syscall.EPIPE, and the error is not
// context.DeadlineExceeded. This is the path every event delivery to a plugin
// over a connection takes (process deliverViaConn, ipc SendDeliverBatch).
// METHOD: the brokenPipeWriter of TestConnWriteToAClosedPeerKeepsItsCause, and a
// call under a 5 s deadline that is still live when the write fails.
// PREVENTS: one dead plugin reading as slow on event delivery ("send batch:
// context deadline exceeded") while its RPCs read as gone ("send request: write
// frame: ... broken pipe").
//
// MUTATION: restoring "if hasDeadline { return context.DeadlineExceeded }" in
// writeBatchWithDeadline in place of the writeFailure call reddens all three
// cause assertions, with "send batch: context deadline exceeded".
func TestConnBatchWriteToAClosedPeerKeepsItsCause(t *testing.T) {
	t.Parallel()

	conn := NewConn(newBlockingReader(), brokenPipeWriter{})
	defer closeConn(t, conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := conn.CallBatchRPC(ctx, batchEvents)
	require.Error(t, err)
	require.NoError(t, ctx.Err(), "the context must still be live, or ctx.Err decided the answer")
	assert.EqualError(t, err, "send batch: write batch frame: write tcp: write: broken pipe")
	assert.ErrorIs(t, err, syscall.EPIPE, "the broken pipe must survive as the cause")
	assert.NotErrorIs(t, err, context.DeadlineExceeded, "no deadline passed, so the error is not a timeout")
}

// TestConnBatchWritePastItsDeadlineIsATimeout proves that a deliver-batch write
// that timed out still answers context.DeadlineExceeded, on the deadline
// transport and on the watchdog transport alike.
//
// VALIDATES: writeBatchWithDeadline gives writeFailure the two timeout forms
// writeFrame gives it: a write error that is os.ErrDeadlineExceeded, here
// wrapped by WriteBatchFrame as "write batch frame: %w", and the watchdog-fired
// report from disarmWatchdogLocked on a transport with no write deadline.
// METHOD: each case calls CallBatchRPC under a 5 s deadline, so the context is
// still live when the write fails and only the reading of the write decides the
// answer. The writers are the ones TestConnWritePastItsDeadlineIsATimeout uses.
// PREVENTS: the closed-peer fix for the batch path ceasing to report a real
// timeout, and the batch path dropping the watchdog's verdict.
//
// MUTATION: dropping the os.ErrDeadlineExceeded test in writeFailure reddens the
// deadline case; leaving watchdogFired false in writeBatchWithDeadline (not
// storing disarmWatchdogLocked's answer) reddens the watchdog case, with "send
// batch: write batch frame: io: read/write on closed pipe".
func TestConnBatchWritePastItsDeadlineIsATimeout(t *testing.T) {
	t.Run("write deadline expired", func(t *testing.T) {
		conn := NewConn(newBlockingReader(), expiredDeadlineWriter{})
		defer closeConn(t, conn)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := conn.CallBatchRPC(ctx, batchEvents)
		require.Error(t, err)
		require.NoError(t, ctx.Err(), "the context must still be live, or ctx.Err decided the answer")
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})
	t.Run("watchdog fired", func(t *testing.T) {
		writer := newBlockingWriter()
		conn := NewConn(newBlockingReader(), writer)
		conn.SetWatchdogWindow(30 * time.Millisecond)
		if _, ok := any(writer).(writeDeadliner); ok {
			t.Fatal("blockingWriter must not implement SetWriteDeadline")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := conn.CallBatchRPC(ctx, batchEvents)
		require.Error(t, err)
		require.NoError(t, ctx.Err(), "the context must still be live, or ctx.Err decided the answer")
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})
}
