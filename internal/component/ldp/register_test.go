// Design: plan/spec-mpls-2-ldp.md -- discovery interface resolution tests
package ldp

import (
	"context"
	"log/slog"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// VALIDATES: waitForInterface returns immediately for an interface that exists.
func TestWaitForInterfaceFound(t *testing.T) {
	ifaces, err := net.Interfaces()
	if err != nil || len(ifaces) == 0 {
		t.Skip("no interfaces available")
	}
	name := ifaces[0].Name

	ifi := waitForInterface(context.Background(), slogutil.DiscardLogger(), name, time.Second)
	if ifi == nil {
		t.Fatalf("waitForInterface(%q) returned nil for an existing interface", name)
	}
	if ifi.Name != name {
		t.Errorf("got interface %q, want %q", ifi.Name, name)
	}
}

// VALIDATES: waitForInterface returns nil (does not block forever) when the
// context is canceled before a missing interface appears -- the retry loop is
// cancellation-safe, so a goroutine for an absent interface unwinds cleanly.
func TestWaitForInterfaceCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled

	ifi := waitForInterface(ctx, slogutil.DiscardLogger(), "ze-nonexistent-iface-xyz", time.Second)
	if ifi != nil {
		t.Errorf("waitForInterface returned %v, want nil on canceled context", ifi)
	}
}

// warnCounter is a slog.Handler that counts WARN+ records.
type warnCounter struct{ warns atomic.Int64 }

func (w *warnCounter) Enabled(context.Context, slog.Level) bool { return true }
func (w *warnCounter) Handle(_ context.Context, r slog.Record) error { //nolint:gocritic // slog.Handler interface mandates Record by value
	if r.Level >= slog.LevelWarn {
		w.warns.Add(1)
	}
	return nil
}
func (w *warnCounter) WithAttrs([]slog.Attr) slog.Handler { return w }
func (w *warnCounter) WithGroup(string) slog.Handler      { return w }

// VALIDATES: a permanently-missing interface is warned about once, not on every
// retry -- so a misconfigured interface name does not spam the log.
func TestWaitForInterfaceWarnsOnce(t *testing.T) {
	wc := &warnCounter{}
	log := slog.New(wc)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		// 1ms retry so several iterations elapse before we cancel.
		waitForInterface(ctx, log, "ze-nonexistent-iface-xyz", time.Millisecond)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond) // allow many retry cycles
	cancel()
	<-done

	if got := wc.warns.Load(); got != 1 {
		t.Errorf("warn count = %d, want exactly 1 (log-once across retries)", got)
	}
}
