package rpc

import (
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// blockingWriter is an io.WriteCloser whose Write blocks until Close is called.
// It deliberately does NOT implement SetWriteDeadline, so a Conn wrapping it
// takes the non-deadline write path guarded by the write watchdog.
type blockingWriter struct {
	unblocked chan struct{}
	once      sync.Once
}

func newBlockingWriter() *blockingWriter {
	return &blockingWriter{unblocked: make(chan struct{})}
}

func (b *blockingWriter) Write(_ []byte) (int, error) {
	<-b.unblocked
	return 0, io.ErrClosedPipe
}

func (b *blockingWriter) Close() error {
	b.once.Do(func() { close(b.unblocked) })
	return nil
}

// blockingReader is an io.ReadCloser whose Read blocks until Close is called.
// The Conn's readCloser is closed by fireWatchdog; this lets that Close unblock
// any pending Read too.
type blockingReader struct {
	unblocked chan struct{}
	once      sync.Once
}

func newBlockingReader() *blockingReader {
	return &blockingReader{unblocked: make(chan struct{})}
}

func (b *blockingReader) Read(p []byte) (int, error) {
	<-b.unblocked
	return 0, io.EOF
}

func (b *blockingReader) Close() error {
	b.once.Do(func() { close(b.unblocked) })
	return nil
}

// TestConnWriteWatchdogStuckStdio verifies that a write on a non-deadline
// transport that blocks past the watchdog window fires the watchdog: the
// connection is closed (fail-fast) and the write returns an error promptly
// instead of blocking indefinitely.
//
// VALIDATES: AC-2 -- stuck write on non-writeDeadliner transport detected,
// connection closed after the watchdog window.
func TestConnWriteWatchdogStuckStdio(t *testing.T) {
	w := newBlockingWriter()
	r := newBlockingReader()
	conn := NewConn(r, w)
	conn.SetWatchdogWindow(30 * time.Millisecond)
	conn.SetLabel("test-plugin")

	// Sanity: the blocking writer must not be a deadline transport, otherwise
	// the deadline path (not the watchdog) would govern the write.
	if _, ok := any(w).(writeDeadliner); ok {
		t.Fatal("blockingWriter must not implement SetWriteDeadline")
	}

	start := time.Now()
	err := conn.SendOK(context.Background(), 1)
	elapsed := time.Since(start)

	require.Error(t, err, "write must return an error after the watchdog fires")
	require.GreaterOrEqual(t, elapsed, 20*time.Millisecond, "returned before the watchdog window elapsed")
	require.Less(t, elapsed, 5*time.Second, "watchdog did not fire promptly")
}

// TestConnWatchdogNoopOnDeadlineTransport verifies zero behavior change for
// deadline-capable transports: the watchdog timer is never even created.
//
// VALIDATES: AC-2 -- deadline-capable transports: zero behavior change.
func TestConnWatchdogNoopOnDeadlineTransport(t *testing.T) {
	a, b := net.Pipe()
	defer func() {
		if err := a.Close(); err != nil {
			t.Logf("close a: %v", err)
		}
	}()
	defer func() {
		if err := b.Close(); err != nil {
			t.Logf("close b: %v", err)
		}
	}()

	// net.Pipe endpoints are net.Conn, hence writeDeadliner.
	if _, ok := any(a).(writeDeadliner); !ok {
		t.Fatal("net.Pipe endpoint must implement SetWriteDeadline")
	}

	conn := NewConn(a, a)

	// Drain the peer so the write completes.
	go func() {
		buf := make([]byte, 256)
		for {
			if _, err := b.Read(buf); err != nil {
				return
			}
		}
	}()

	require.NoError(t, conn.SendOK(context.Background(), 1))

	conn.mu.Lock()
	wd := conn.watchdog
	conn.mu.Unlock()
	require.Nil(t, wd, "watchdog must never arm on a deadline-capable transport")
}

// TestWatchdogMetric verifies the package write-watchdog hook is invoked with
// the transport kind and connection label when the watchdog fires.
//
// VALIDATES: AC-2 -- Prometheus counter (via hook) increments; transport kind
// and plugin name are reported.
func TestWatchdogMetric(t *testing.T) {
	done := make(chan [2]string, 1)
	SetWriteWatchdogHook(func(transport, label string) {
		select {
		case done <- [2]string{transport, label}:
		default:
		}
	})
	defer SetWriteWatchdogHook(nil)

	w := newBlockingWriter()
	r := newBlockingReader()
	conn := NewConn(r, w)
	conn.SetWatchdogWindow(30 * time.Millisecond)
	conn.SetLabel("metric-plugin")

	go func() { _ = conn.SendOK(context.Background(), 1) }()

	select {
	case got := <-done:
		require.Equal(t, "metric-plugin", got[1], "hook label")
		require.Equal(t, "stream", got[0], "hook transport kind")
	case <-time.After(5 * time.Second):
		t.Fatal("watchdog hook was not called")
	}
}

// TestConnWatchdogDisabledWhenWindowZero verifies the boundary case: a window
// of 0 disables the watchdog, so arming is a no-op and no timer is created.
//
// VALIDATES: AC-2 boundary -- watchdog window 0 => disabled.
func TestConnWatchdogDisabledWhenWindowZero(t *testing.T) {
	w := newBlockingWriter()
	r := newBlockingReader()
	conn := NewConn(r, w)
	conn.SetWatchdogWindow(0)

	conn.mu.Lock()
	conn.armWatchdogLocked()
	wd := conn.watchdog
	conn.mu.Unlock()

	require.Nil(t, wd, "window <= 0 must disable the watchdog (no timer created)")
}
