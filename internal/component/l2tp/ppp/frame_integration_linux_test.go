//go:build integration && linux

// Design: docs/guide/l2tp.md -- blocking PPP descriptor lifetime

package ppp

import (
	"errors"
	"io"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

// TestPPPFDClosingRejectsFurtherIO exercises a real blocking descriptor. Closing
// the PPP wrapper must make subsequent reads, writes and closes fail through
// the wrapper rather than issuing syscalls against its old descriptor number.
func TestPPPFDClosingRejectsFurtherIO(t *testing.T) {
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	peer := os.NewFile(uintptr(fds[1]), "ppp-peer")
	t.Cleanup(func() { _ = peer.Close() })
	wrapped := NewFDFile(fds[0], "ppp-channel")
	t.Cleanup(func() { _ = wrapped.Close() })
	if _, err := wrapped.Write([]byte{0x5a}); err != nil {
		t.Fatal(err)
	}
	var payload [1]byte
	if _, err := io.ReadFull(peer, payload[:]); err != nil || payload[0] != 0x5a {
		t.Fatalf("live write: payload=%x error=%v", payload, err)
	}
	if _, err := peer.Write([]byte{0xa5}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(wrapped, payload[:]); err != nil || payload[0] != 0xa5 {
		t.Fatalf("live read: payload=%x error=%v", payload, err)
	}
	unit, err := os.Open("/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unit.Close() })
	session := &pppSession{chanFile: wrapped, unitFile: unit}
	session.closeFiles()
	if _, err := unit.Read(payload[:]); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("unit read after session close: got %v, want os.ErrClosed", err)
	}
	session.closeFiles()
	if _, err := wrapped.Read(payload[:]); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("read after Close: got %v, want os.ErrClosed", err)
	}
	if _, err := wrapped.Write(payload[:]); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("write after Close: got %v, want os.ErrClosed", err)
	}
	if err := wrapped.Close(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("repeated Close: got %v, want os.ErrClosed", err)
	}
}

// Queue delivery transfers ownership even when Stop prevents session dispatch.
func TestPPPStoppedDispatcherClosesQueuedDescriptors(t *testing.T) {
	channel, err := unix.Open(os.DevNull, unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	unit, err := unix.Open(os.DevNull, unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		_ = unix.Close(channel)
		t.Fatal(err)
	}
	driver := &Driver{
		sessionsIn: make(chan StartSession, 1),
		stopCh: make(chan struct{}),
		dispatchDone: make(chan struct{}),
		stopped: true,
	}
	driver.sessionsIn <- StartSession{ChanFD: channel, UnitFD: unit}
	close(driver.stopCh)
	driver.dispatch()
	for _, fd := range []int{channel, unit} {
		if _, err := unix.Write(fd, []byte{1}); !errors.Is(err, unix.EBADF) {
			_ = unix.Close(fd)
			t.Errorf("stopped dispatcher retained owned descriptor %d: %v", fd, err)
		}
	}
}
