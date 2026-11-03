//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- raw descriptor ordering.
// Related: receive_linux.go -- production readPacket, not a channel priority fake.
package pppoeclient

import (
	"bytes"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// TestOrderedDescriptorDispatch puts both packets on actual nonblocking kernel
// descriptors before authorizing production input. The channel must win; the
// unit packet must remain on its descriptor until dispatch authorizes another
// read. Socket pairs preserve packet boundaries without requiring a PPP device.
// TestDialAcknowledgesFirstEagerIPCP retains the separate real-PPP oracle.
func TestOrderedDescriptorDispatch(t *testing.T) {
	control, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(control[0])
	defer unix.Close(control[1])
	unit, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(unit[0])
	defer unix.Close(unit[1])
	if err := prepareClientDescriptors(control[0], unit[0]); err != nil {
		t.Fatal(err)
	}
	link := &sessionLink{channelFD: control[0], unitFD: unit[0], unitConnected: true}
	stop := make(chan struct{})
	reader, err := newFrameReader(link.readPacket, stop)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		close(stop)
		link.mu.Lock()
		link.closed = true
		link.mu.Unlock()
		<-reader.done
	}()
	controlPacket := []byte{0xc2, 0x23, 3, 63, 0, 4}
	unitPacket := []byte{0x80, 0x21, 1, 71, 0, 10, 3, 6, 192, 0, 2, 1}
	for _, item := range []struct {
		fd     int
		packet []byte
	}{{unit[1], unitPacket}, {control[1], controlPacket}} {
		if n, err := unix.Write(item.fd, item.packet); err != nil || n != len(item.packet) {
			t.Fatalf("enqueue: %d %v", n, err)
		}
	}
	select {
	case frame := <-reader.next():
		if frame.err != nil || !bytes.Equal(frame.data, controlPacket) {
			t.Fatalf("channel did not precede unit: %+v", frame)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("channel read blocked")
	}
	var peek [64]byte
	n, _, err := unix.Recvfrom(unit[0], peek[:], unix.MSG_PEEK|unix.MSG_DONTWAIT)
	if err != nil || !bytes.Equal(peek[:n], unitPacket) {
		t.Fatalf("unit read ahead before dispatch: %d %v", n, err)
	}
	reader.received()
	select {
	case frame := <-reader.next():
		reader.received()
		if frame.err != nil || !bytes.Equal(frame.data, unitPacket) {
			t.Fatalf("unit packet lost: %+v", frame)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("unit read blocked")
	}
}
