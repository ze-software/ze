// VALIDATES: the PPPoE dialer's real kernel paths — Dial rejects a non-existent
// source interface at resolution and, on loopback, opens the PF_PACKET discovery
// socket and returns (no PADO) rather than hanging. Auto-enrolled in the QEMU
// integration run via the derived `integration && linux` package list.
// PREVENTS: a discovery-socket or interface-resolution regression, or a Dial that
// ignores the stop signal and blocks a caller indefinitely.

//go:build integration && linux

package pppoeclient

import (
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
)

func isPermErr(err error) bool {
	return errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) || errors.Is(err, os.ErrPermission)
}

// dialWithWatchdog runs Dial in a goroutine and fails if it does not return
// within the timeout, so a stop-signal regression cannot hang the suite.
func dialWithWatchdog(t *testing.T, cfg iface.PPPoEClientConfig) error {
	t.Helper()
	stop := make(chan struct{})
	close(stop) // already stopped: Dial must not block waiting for a PADO
	type result struct{ err error }
	ch := make(chan result, 1)
	go func() {
		d := &Dialer{}
		_, err := d.Dial(cfg, stop, slog.Default())
		ch <- result{err}
	}()
	select {
	case r := <-ch:
		return r.err
	case <-time.After(30 * time.Second):
		t.Fatal("Dial did not return within 30s (stop signal ignored?)")
		return nil
	}
}

func TestDialBogusInterface(t *testing.T) {
	err := dialWithWatchdog(t, iface.PPPoEClientConfig{SourceInterface: "ze-nope-xyz0"})
	if err == nil {
		t.Error("Dial on a non-existent interface should return an error")
	}
}

func TestDialLoopbackDiscovery(t *testing.T) {
	err := dialWithWatchdog(t, iface.PPPoEClientConfig{SourceInterface: "lo"})
	if err == nil {
		t.Error("Dial on loopback with no PPPoE server should return an error")
	}
	if isPermErr(err) {
		t.Skipf("PF_PACKET discovery socket needs CAP_NET_RAW: %v", err)
	}
}
