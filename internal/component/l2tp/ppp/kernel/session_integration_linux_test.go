//go:build integration && linux

// Design: docs/research/l2tpv2-ze-integration.md -- kernel session reader shutdown

package kernel

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"

	"golang.org/x/sys/unix"
)

// Real poll-backed borrowed streams must wake on owner closure without peer
// traffic. The readiness signal occurs inside both held RawConn leases, so
// Close cannot finish by merely winning a race before either reader starts.
func TestKernelSessionCloseInterruptsReaders(t *testing.T) {
	control, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Close(control[1]); err != nil {
			t.Error(err)
		}
	})
	opened, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		if closeErr := unix.Close(control[0]); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Close(opened[1]); err != nil {
			t.Error(err)
		}
	})
	cfg := sessionTestConfig()
	cfg.ChanFD = control[0]
	cfg.UnitFD = opened[0]
	cfg.InterfaceName = "lo"
	identity, err := net.InterfaceByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	sessionTestChannel(cfg).fd = cfg.ChanFD
	sessionTestUnit(cfg).info.fd = cfg.UnitFD
	sessionTestUnit(cfg).info.identity = ppp.SessionIdentity{InterfaceName: cfg.InterfaceName, NativeIndex: uint32(identity.Index)}
	started := make(chan struct{}, 2)
	ops := productionSessionOps()
	wrap := ops.wrap
	ops.wrap = func(fd int, name string) (io.ReadWriteCloser, error) {
		file, err := wrap(fd, name)
		if err != nil {
			return nil, err
		}
		reader, ok := file.(*interruptibleFDFile)
		if !ok {
			t.Fatalf("kernel reader has type %T, want *interruptibleFDFile", file)
		}
		reader.readFrame = func(channel, wake int, buffer []byte) (int, error) {
			started <- struct{}{}
			return pollThenRead(channel, wake, buffer)
		}
		return reader, nil
	}
	frames, handle, err := sessionTestNewSession(cfg, ops)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	})
	done := make(chan error, 2)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	// Two lifetime readers own distinct physical streams and are joined below.
	for _, reader := range []io.Reader{frames.Control(), frames.Opened()} {
		go func() { var buffer [8]byte; _, err := reader.Read(buffer[:]); done <- err }()
	}
	for range 2 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("readers never acquired their descriptor leases")
		}
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		select {
		case err := <-done:
			if !errors.Is(err, errReadInterrupted) {
				t.Errorf("reader did not return through the eventfd wake: %v", err)
			}
		case <-ctx.Done():
			t.Fatal("reader remained blocked after owner close")
		}
	}
	if _, err := frames.Control().Write([]byte{1}); err == nil {
		t.Error("closed borrowed stream accepted a write")
	}
	if err := handle.Close(); err != nil {
		t.Errorf("repeated owner close: %v", err)
	}
}
