//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- channel-first attempts.
// Related: client_linux.go -- private real-socket algorithm, no PPP grants.

package kernel

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func clientReadSocket(t *testing.T, kind int) [2]int {
	t.Helper()
	fds, err := unix.Socketpair(unix.AF_UNIX, kind|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, fd := range fds {
			if err := unix.Close(fd); err != nil {
				t.Error(err)
			}
		}
	})
	return fds
}

func clientReadSend(t *testing.T, fd int, bytes []byte) {
	t.Helper()
	n, err := unix.Write(fd, bytes)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(bytes) {
		t.Fatalf("socket write %d/%d", n, len(bytes))
	}
}

// The private descriptor algorithm must consume channel first when both have
// ready packets, retain a preconnection unit packet, and deliver it after
// admission. The wire bytes include IPCP so the control has a useful payload.
func TestClientPairOrderedReadAndDelayedIPCP(t *testing.T) {
	channel := clientReadSocket(t, unix.SOCK_DGRAM)
	unit := clientReadSocket(t, unix.SOCK_DGRAM)
	channelPacket := []byte{0xc0, 0x21, 1, 7, 0, 4}
	unitPacket := []byte{0x80, 0x21, 1, 9, 0, 4}
	clientReadSend(t, unit[1], unitPacket)
	buffer := make([]byte, 128)
	n, err := rawOrderedClientAttempt(channel[0], unit[0], false, buffer)
	if n != 0 {
		t.Fatalf("preconnection unit packet consumed: %d %v", n, err)
	}
	if !errors.Is(err, unix.EAGAIN) {
		t.Fatalf("preconnection unit packet consumed: %d %v", n, err)
	}
	clientReadSend(t, channel[1], channelPacket)
	n, err = rawOrderedClientAttempt(channel[0], unit[0], true, buffer)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buffer[:n], channelPacket) {
		t.Fatalf("unit overtook channel: %x", buffer[:n])
	}
	n, err = rawOrderedClientAttempt(channel[0], unit[0], true, buffer)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buffer[:n], unitPacket) {
		t.Fatalf("IPCP was lost: %x", buffer[:n])
	}
}

// Real socket readiness and terminal conditions must survive the attempt;
// a channel EOF cannot be hidden by a ready unit packet.
func TestClientPairOrderedReadChannelEOF(t *testing.T) {
	channel := clientReadSocket(t, unix.SOCK_STREAM)
	unit := clientReadSocket(t, unix.SOCK_DGRAM)
	if err := unix.Shutdown(channel[1], unix.SHUT_WR); err != nil {
		t.Fatal(err)
	}
	clientReadSend(t, unit[1], []byte{0x80, 0x21})
	buffer := make([]byte, 128)
	n, err := rawOrderedClientAttempt(channel[0], unit[0], true, buffer)
	if n != 0 {
		t.Fatalf("channel EOF lost: %d %v", n, err)
	}
	if !errors.Is(err, io.EOF) {
		t.Fatalf("channel EOF lost: %d %v", n, err)
	}
	n, err = unix.Read(unit[0], buffer)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("channel EOF consumed unit: %d", n)
	}
}

// A real idle poll returns EAGAIN once and releases pair.mu, allowing a
// concurrent split close without a reader retry holding the lifecycle lock.
func TestClientPairIdleAttemptUnlocksForClose(t *testing.T) {
	client, _, released := clientPairFixture(t)
	channel := clientReadSocket(t, unix.SOCK_DGRAM)
	unit := clientReadSocket(t, unix.SOCK_DGRAM)
	entered := make(chan struct{})
	client.ops.read = func(_ *claimedSessionPair, readUnit bool, buffer []byte) (int, error) {
		close(entered)
		return rawOrderedClientAttempt(channel[0], unit[0], readUnit, buffer)
	}
	readDone := make(chan error, 1)
	closeDone := make(chan error, 1)
	go func() { _, err := client.ReadPacket(make([]byte, 128)); readDone <- err }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("idle attempt did not enter")
	}
	go func() { closeDone <- client.CloseChannel() }()
	select {
	case err := <-readDone:
		if !errors.Is(err, unix.EAGAIN) {
			t.Fatalf("idle attempt %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("bounded idle attempt retained its locks")
	}
	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("close could not acquire released lifecycle lock")
	}
	if len(*released) != 1 {
		t.Fatalf("idle close released %v", *released)
	}
}

// Connected unit EOF remains terminal; the disconnected unit is never read.
func TestClientPairOrderedReadUnitEOF(t *testing.T) {
	channel := clientReadSocket(t, unix.SOCK_DGRAM)
	unit := clientReadSocket(t, unix.SOCK_STREAM)
	if err := unix.Shutdown(unit[1], unix.SHUT_WR); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 128)
	if _, err := rawOrderedClientAttempt(channel[0], unit[0], false, buffer); !errors.Is(err, unix.EAGAIN) {
		t.Fatalf("disconnected unit EOF was read: %v", err)
	}
	if _, err := rawOrderedClientAttempt(channel[0], unit[0], true, buffer); !errors.Is(err, io.EOF) {
		t.Fatalf("connected unit EOF lost: %v", err)
	}
}
