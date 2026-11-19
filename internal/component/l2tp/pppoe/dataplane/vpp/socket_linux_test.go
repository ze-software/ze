// Design: docs/architecture/l2tp/bng-5-pppoe.md -- socket polling and close ownership
// VALIDATES: actual Linux poller wakeup and complete datagram bounds.

//go:build linux

package vpp

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"golang.org/x/sys/unix"
)

func nativeSocketFixture(t *testing.T) (*controlSocket, *os.File) {
	t.Helper()
	pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	file := os.NewFile(uintptr(pair[0]), "vpp-control-test")
	peer := os.NewFile(uintptr(pair[1]), "vpp-peer-test")
	t.Cleanup(func() {
		if err := peer.Close(); err != nil {
			t.Error(err)
		}
	})
	raw, err := file.SyscallConn()
	if err != nil {
		t.Fatal(errors.Join(err, file.Close()))
	}
	socket := newControlSocket(&socketFile{file: file, raw: raw})
	t.Cleanup(func() {
		if err := socket.Close(); err != nil {
			t.Error(err)
		}
	})
	return socket, peer
}

// A blocked read owns its descriptor reference through the runtime poller.
// Close wakes it and remains idempotent; no raw second close is issued.
func TestVPPNativeSocketCloseWakesPolledReader(t *testing.T) {
	socket, _ := nativeSocketFixture(t)
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() { close(started); var buf [ppp.MaxFrameBufLen]byte; _, err := socket.Read(buf[:]); done <- err }()
	<-started
	if err := socket.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("closed reader succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("polled reader stayed blocked")
	}
	if err := socket.Close(); err != nil {
		t.Fatal(err)
	}
}

// An overlong datagram must not turn into a syntactically valid truncated PPP
// packet. The following valid datagram proves the endpoint remains usable.
func TestVPPNativeSocketRejectsTruncationThenReadsNextFrame(t *testing.T) {
	socket, peer := nativeSocketFixture(t)
	oversized := make([]byte, ppp.MaxFrameBufLen+1)
	if _, err := peer.Write(oversized); err != nil {
		t.Fatal(err)
	}
	var buf [ppp.MaxFrameBufLen]byte
	if _, err := socket.Read(buf[:]); err == nil {
		t.Fatal("truncated datagram was accepted")
	}
	frame := []byte{0xc0, 0x21, 9, 1, 0, 8, 0, 0, 0, 1}
	if _, err := peer.Write(frame); err != nil {
		t.Fatal(err)
	}
	n, err := socket.Read(buf[:])
	if err != nil || n != len(frame) {
		t.Fatalf("next frame n=%d, err=%v", n, err)
	}
	if string(buf[:n]) != string(frame) {
		t.Fatalf("next frame %x", buf[:n])
	}
}

// Quiescing a PPP reader uses a deadline, retaining writes for the final control
// exchange until its one lifetime owner closes the transport.
func TestVPPNativeSocketInterruptRetainsWritePath(t *testing.T) {
	socket, peer := nativeSocketFixture(t)
	if err := socket.InterruptRead(); err != nil {
		t.Fatal(err)
	}
	var buf [ppp.MaxFrameBufLen]byte
	if _, err := socket.Read(buf[:]); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("interrupted read %v", err)
	}
	frame := []byte{0xc0, 0x21, 9, 1, 0, 8, 0, 0, 0, 1}
	if _, err := socket.Write(frame); err != nil {
		t.Fatal(err)
	}
	if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if n, err := peer.Read(buf[:]); err != nil || n != len(frame) {
		t.Fatalf("retained write n=%d, err=%v", n, err)
	}
}
