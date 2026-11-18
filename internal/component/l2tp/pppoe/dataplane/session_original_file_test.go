// Design: docs/architecture/l2tp/followup-l2tp-call.md -- original relay transport lifetime.
package dataplane

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// originalRelayTransport is a real file owner, never a native PPP capability.
type originalRelayTransport struct {
	file    *os.File
	failure error
	closes  int
}

func (owner *originalRelayTransport) Close() error {
	owner.closes++
	return errors.Join(owner.file.Close(), owner.failure)
}
func (owner *originalRelayTransport) WithOriginalFile(use func(*os.File) error) error {
	return use(owner.file)
}

// The admitted callback receives the exact owned file, and departure refuses
// subsequent borrowing instead of reconstructing a file from its old number.
func TestOpenedSessionBorrowsExactOriginalFile(t *testing.T) {
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, writer.Close()) })
	owner := &originalRelayTransport{file: reader}
	session := &openedSession{transport: owner}
	want := errors.New("bounded native operation refused")
	err = session.WithOriginalFile(func(original *os.File) error {
		require.Same(t, reader, original)
		if session.transportMu.TryLock() {
			session.transportMu.Unlock()
			t.Error("original borrow did not exclude transport close")
		}
		return want
	})
	require.ErrorIs(t, err, want)
	require.NoError(t, session.CloseTransport())
	err = session.WithOriginalFile(func(*os.File) error { t.Fatal("borrow after departure"); return nil })
	require.Error(t, err)
}

// Deferred physical close has the same one-attempt, retained-error contract as
// an immediate close. The worker that drops its token receives the real cause.
func TestOriginalRelayDepartureRetainsDeferredCloseError(t *testing.T) {
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, writer.Close()) })
	want := errors.New("original transport physical close failed")
	owner := &originalRelayTransport{file: reader, failure: want}
	session := &openedSession{transport: owner}
	lease, err := session.RetainOriginalFile()
	require.NoError(t, err)
	require.Error(t, session.CloseTransport())
	require.ErrorIs(t, lease.Close(), want)
	require.ErrorIs(t, lease.Close(), want)
	require.ErrorIs(t, session.CloseTransport(), want)
	require.Equal(t, 1, owner.closes, "uncertain descriptor close must not retry its reusable number")
}

// Unsupported and empty callbacks are refusals, never successful no-ops.
func TestOpenedSessionOriginalBorrowRefusesUnsupported(t *testing.T) {
	session := &openedSession{transport: io.NopCloser(&emptyReader{})}
	require.Error(t, session.WithOriginalFile(func(*os.File) error { t.Fatal("unsupported borrow"); return nil }))
	require.Error(t, session.WithOriginalFile(nil))
}

// A retained exact relay token makes departure retryable, without closing its
// transport early or permanently caching the pending result. Releasing another
// token copy cannot release the original admission's barrier.
func TestOriginalRelayLeaseBlocksDepartureUntilChannelsClose(t *testing.T) {
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, writer.Close()) })
	session := &openedSession{transport: &originalRelayTransport{file: reader}}
	lease, err := session.RetainOriginalFile()
	require.NoError(t, err)
	require.NoError(t, lease.WithOriginalFile(func(original *os.File) error { require.Same(t, reader, original); return nil }))
	_, err = session.RetainOriginalFile()
	require.Error(t, err, "a second relay cannot claim the same exact transport")
	require.Error(t, session.CloseTransport())
	_, err = reader.Stat()
	require.NoError(t, err, "pending bridge must retain the original transport")
	require.Error(t, lease.WithOriginalFile(func(*os.File) error { t.Fatal("capture after departure"); return nil }))
	copy := *lease.(*relayFileLease)
	require.Error(t, copy.Close())
	require.Error(t, session.CloseTransport(), "copy cannot release original token")
	require.NoError(t, lease.Close())
	require.NoError(t, lease.Close())
	_, err = reader.Stat()
	require.ErrorIs(t, err, os.ErrClosed, "releasing final token MUST fulfill the earlier physical close without a caller retry")
	require.NoError(t, session.CloseTransport())
	require.NoError(t, session.CloseTransport())
	_, err = reader.Stat()
	require.ErrorIs(t, err, os.ErrClosed)
}

type emptyReader struct{}

func (*emptyReader) Read([]byte) (int, error) { return 0, io.EOF }
