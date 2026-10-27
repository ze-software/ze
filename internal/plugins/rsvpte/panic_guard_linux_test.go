// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- panic containment at the signaling and receive tasks
// Related: panic_guard_test.go -- snapshotGuard, requireContained
// Related: transport_linux.go -- rawTransport.readLoop and receive, the code under test

package rsvpte

import (
	"bytes"
	"encoding/binary"
	"net"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/crashlog"
)

// panickingRawConn delegates to a real socket's RawConn and panics once, right
// after the first successful Read: the first datagram is consumed by recvmsg
// and then lost to the injected defect, as a panic in the receive path would
// lose it.
type panickingRawConn struct {
	real     syscall.RawConn
	panicked atomic.Bool
}

func (c *panickingRawConn) Control(f func(fd uintptr)) error { return c.real.Control(f) }
func (c *panickingRawConn) Write(f func(fd uintptr) bool) error {
	return c.real.Write(f)
}

func (c *panickingRawConn) Read(f func(fd uintptr) bool) error {
	err := c.real.Read(f)
	if err == nil && c.panicked.CompareAndSwap(false, true) {
		panic("injected test panic after the first datagram was read")
	}
	return err
}

// rsvpDatagram is what a raw IPPROTO_RSVP socket reads: an IPv4 header with
// protocol 46, then the RSVP octets.
func rsvpDatagram(payload []byte) []byte {
	datagram := make([]byte, 20+len(payload))
	datagram[0] = 0x45
	binary.BigEndian.PutUint16(datagram[2:4], uint16(len(datagram)))
	datagram[8] = 1
	datagram[9] = rsvpProtocol
	copy(datagram[12:16], []byte{10, 0, 0, 1})
	copy(datagram[16:20], []byte{10, 0, 0, 9})
	copy(datagram[20:], payload)
	return datagram
}

// TestRSVPReadPanicContained runs the real readLoop over a UDP socket that
// carries IP_PKTINFO, standing in for the raw RSVP socket. VALIDATES: a panic
// while one datagram is received drops that datagram, the next one is still
// delivered on the same socket, and the panic is counted, logged and recorded.
// PREVENTS: one bad datagram ending RSVP-TE reception on the node.
func TestRSVPReadPanicContained(t *testing.T) {
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	before := snapshotGuard(t, taskReceive)

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	raw, err := conn.SyscallConn()
	if err != nil {
		t.Fatalf("SyscallConn: %v", err)
	}
	var optErr error
	if err := raw.Control(func(fd uintptr) {
		optErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_PKTINFO, 1)
	}); err != nil {
		t.Fatalf("Control: %v", err)
	}
	if optErr != nil {
		t.Fatalf("set IP_PKTINFO: %v", optErr)
	}

	tr := &rawTransport{
		rxRaw:  &panickingRawConn{real: raw},
		recvCh: make(chan Packet, 4),
		closeC: make(chan struct{}),
		done:   make(chan struct{}),
	}
	go tr.readLoop()
	t.Cleanup(func() {
		_ = conn.Close()
		<-tr.done
	})

	local, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("the listening socket's address is not *net.UDPAddr")
	}
	sender, err := net.DialUDP("udp4", nil, local)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	t.Cleanup(func() { _ = sender.Close() })
	dropped, delivered := []byte{1, 2, 3, 4}, []byte{5, 6, 7, 8}
	for _, payload := range [][]byte{dropped, delivered} {
		if _, err := sender.Write(rsvpDatagram(payload)); err != nil {
			t.Fatalf("send a datagram: %v", err)
		}
	}

	select {
	case packet, ok := <-tr.recvCh:
		if !ok {
			t.Fatal("the receive worker stopped: the panic was not contained")
		}
		if !bytes.Equal(packet.Payload, delivered) {
			t.Fatalf("first delivered payload = %v, want %v: the datagram read when the panic hit was not dropped", packet.Payload, delivered)
		}
		if packet.IfIndex <= 0 {
			t.Errorf("delivered packet interface index = %d, want the loopback's", packet.IfIndex)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no datagram was delivered after the panic: the receive worker did not survive")
	}
	requireContained(t, taskReceive, before)
}
