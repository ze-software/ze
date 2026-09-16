//go:build linux

package transport

import (
	"errors"
	"log/slog"
	"net"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/probe"
)

// readIntOption reads one integer socket option back off the transport's
// socket.
func readIntOption(t *testing.T, tr *UDPTransport, level, opt int) int {
	t.Helper()
	sc, ok := any(tr.Conn()).(syscall.Conn)
	if !ok {
		t.Fatalf("conn %T carries no SyscallConn", tr.Conn())
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		t.Fatalf("SyscallConn: %v", err)
	}
	var value int
	var getErr error
	if err := raw.Control(func(fd uintptr) {
		value, getErr = unix.GetsockoptInt(int(fd), level, opt)
	}); err != nil {
		t.Fatalf("Control: %v", err)
	}
	if getErr != nil {
		t.Fatalf("getsockopt level=%d opt=%d: %v", level, opt, getErr)
	}
	return value
}

// TestDFSendRestoresSocketMode proves the DF send toggles IP_MTU_DISCOVER
// only for its own datagram: the datagram reaches the peer, and after the
// send the socket reads the value it held before, so the next plain Send by
// another SA leaves under the kernel default. A loopback socket carries the
// option, so no privilege is needed.
func TestDFSendRestoresSocketMode(t *testing.T) {
	recvConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	t.Cleanup(func() { _ = recvConn.Close() })
	recvAddr, ok := recvConn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("LocalAddr is not *net.UDPAddr")
	}

	tr, err := NewUDPTransport("127.0.0.1:0", slog.Default())
	if err != nil {
		t.Fatalf("NewUDPTransport: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })
	prior := readIntOption(t, tr, unix.IPPROTO_IP, unix.IP_MTU_DISCOVER)

	for _, df := range []probe.DFMode{probe.DFHonorCache, probe.DFBypassCache, probe.DFOff} {
		msg := make([]byte, 28)
		msg[0] = byte(df)
		if err := tr.SendDF(msg, recvAddr, df); err != nil {
			t.Fatalf("SendDF(%v): %v", df, err)
		}
		if after := readIntOption(t, tr, unix.IPPROTO_IP, unix.IP_MTU_DISCOVER); after != prior {
			t.Errorf("%v: IP_MTU_DISCOVER after SendDF = %d, want the prior %d", df, after, prior)
		}
		buf := make([]byte, MaxMsgSize)
		if err := recvConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatalf("SetReadDeadline: %v", err)
		}
		n, _, err := recvConn.ReadFromUDP(buf)
		if err != nil {
			t.Fatalf("%v: ReadFromUDP: %v", df, err)
		}
		if n != 28 {
			t.Fatalf("%v: received %d bytes, want 28", df, n)
		}
		if buf[0] != byte(df) {
			t.Fatalf("%v: received first byte %#x, want %#x", df, buf[0], byte(df))
		}
	}
	if err := tr.SendDF(make([]byte, 28), recvAddr, probe.DFUnspecified); !errors.Is(err, probe.ErrDFUnspecified) {
		t.Errorf("SendDF with the zero mode answered %v, want ErrDFUnspecified", err)
	}
}

// TestTransportInstallsErrorQueue proves the socket carries IP_RECVERR from
// creation, on the plain and the NAT-T socket alike, so a router's
// Fragmentation Needed for a DF datagram is queued rather than dropped.
func TestTransportInstallsErrorQueue(t *testing.T) {
	for _, c := range []struct {
		name string
		open func(string, *slog.Logger) (*UDPTransport, error)
	}{{"ike", NewUDPTransport}, {"natt", NewNATTTransport}} {
		tr, err := c.open("127.0.0.1:0", slog.Default())
		if err != nil {
			t.Fatalf("%s: open: %v", c.name, err)
		}
		t.Cleanup(func() { _ = tr.Close() })
		if got := readIntOption(t, tr, unix.IPPROTO_IP, unix.IP_RECVERR); got != 1 {
			t.Errorf("%s: IP_RECVERR = %d, want 1", c.name, got)
		}
	}
}

// TestSendSurvivesAnErrorQueuedForAnEarlierDatagram proves the hazard
// IP_RECVERR adds to a shared socket is handled: the kernel hands an ICMP
// error about an earlier datagram to the next send as that send's failure
// (sock_alloc_send_pskb returns the pending sk_err), so a Send to a live
// peer after one to a closed port must still deliver its datagram. Loopback
// answers a closed port with Port Unreachable, so no privilege is needed.
// Run is not started: the pending error is left for the send to meet.
func TestSendSurvivesAnErrorQueuedForAnEarlierDatagram(t *testing.T) {
	recvConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	t.Cleanup(func() { _ = recvConn.Close() })
	recvAddr, ok := recvConn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("LocalAddr is not *net.UDPAddr")
	}
	closedConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	closedAddr, ok := closedConn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("LocalAddr is not *net.UDPAddr")
	}
	if err := closedConn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	tr, err := NewUDPTransport("127.0.0.1:0", slog.Default())
	if err != nil {
		t.Fatalf("NewUDPTransport: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })

	if err := tr.Send(make([]byte, 28), closedAddr); err != nil {
		t.Fatalf("Send to the closed port: %v", err)
	}
	// Loopback delivers the Port Unreachable at once, but on another thread;
	// the pending error is what the next send meets, so give it a moment.
	time.Sleep(50 * time.Millisecond) // sleep(kernel): loopback ICMP delivery runs in softirq, no readiness to wait on without reading the socket, which would consume the error under test

	msg := make([]byte, 28)
	msg[0] = 0xcc
	if err := tr.Send(msg, recvAddr); err != nil {
		t.Fatalf("Send after a queued error: %v", err)
	}
	buf := make([]byte, MaxMsgSize)
	if err := recvConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	n, _, err := recvConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("the datagram after a queued error never arrived: %v", err)
	}
	if n != 28 {
		t.Fatalf("received %d bytes, want 28", n)
	}
	if buf[0] != 0xcc {
		t.Fatalf("received first byte %#x, want 0xcc", buf[0])
	}
	select {
	case got := <-tr.Refusals():
		t.Fatalf("a Port Unreachable reached Refusals as a size refusal: %+v", got)
	default:
	}
}
