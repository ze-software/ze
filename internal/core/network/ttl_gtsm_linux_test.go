//go:build linux

// Design: rfc/short/rfc5082.md -- GTSM, the TTL 255 transmit rule and the receive floor
// Overview: ttl.go, ttl_linux.go -- SetIPTTL, SetIPMinTTL, the options ze installs
//
// RFC 5082 is met through the kernel: ze installs a socket option and Linux
// performs the check. ai/rules/rfc-compliance.md requires the proof to assert
// the thing ZE produces, so every test here reads back the option ze installed
// or the TTL ze put on the wire, at the boundary ze owns.
//
// These live outside ttl_integration_linux_test.go on purpose. That file is
// guarded `//go:build integration && linux`, and `./le rfc discriminate-record`
// runs a tagged unit with `gotoolchain.TestOptions{}`, whose empty Tags field
// leaves `integration` off. The unit then matches nothing, `go test` exits 0,
// and no break can ever be observed to redden it
// (plan/journal/gate-excludes-part-of-its-population.md, 2026-09-01).

package network

import (
	"errors"
	"net"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// loopbackV4 is the peer address every test here passes to the TTL helpers.
// SetIPTTL and SetIPMinTTL pick the IPv4 or the IPv6 option from it.
var loopbackV4 = net.IPv4(127, 0, 0, 1)

// TestGTSMDialerSetsOutgoingTTLTo255 drives RealDialer, the dialer the BGP
// reactor builds for a peer, and reads the option back off the connected
// socket. NewSession copies PeerSettings.OutTTL into RealDialer.OutTTL, and
// parseTTLSettings derives 255 for `ttl max N`, so 255 here is the value a
// GTSM peer's configuration produces.
//
// RFC requirement: RFC5082-3-1 positive -- a dialer carrying the GTSM outgoing
// TTL leaves the connected socket with IP_TTL exactly 255, so every IP packet
// of the session is transmitted with the value RFC 5082 Section 3 mandates.
func TestGTSMDialerSetsOutgoingTTLTo255(t *testing.T) {
	listener := listenTCPV4(t)
	defer closeOrLog(t, listener)

	conn := dialTCPV4(t, RealDialer{OutTTL: 255}, listener.Addr().String())
	defer closeOrLog(t, conn)

	if got := socketOption(t, conn, unix.IPPROTO_IP, unix.IP_TTL); got != 255 {
		t.Fatalf("IP_TTL = %d, want exactly 255", got)
	}
}

// TestGTSMDialerWithoutOutTTLLeavesTheDefault is the discrimination for the
// test above. The two dialers differ in one field, so a 255 read back there is
// bound to OutTTL and not to a hard-wired constant or to the system default.
//
// RFC requirement: RFC5082-3-1 negative -- a dialer with no GTSM outgoing TTL
// installs no IP_TTL, so the socket keeps the system default and is not 255.
// The Section 3 obligation binds a GTSM-enabled session and nothing else.
func TestGTSMDialerWithoutOutTTLLeavesTheDefault(t *testing.T) {
	listener := listenTCPV4(t)
	defer closeOrLog(t, listener)

	conn := dialTCPV4(t, RealDialer{}, listener.Addr().String())
	defer closeOrLog(t, conn)

	got := socketOption(t, conn, unix.IPPROTO_IP, unix.IP_TTL)
	if got == 255 {
		t.Fatalf("IP_TTL = 255 with no OutTTL configured; the GTSM value must come from configuration")
	}
	if got != systemDefaultTTL(t) {
		t.Fatalf("IP_TTL = %d, want the system default %d", got, systemDefaultTTL(t))
	}
}

// TestGTSMTransmittedTTLArrivesUndecremented reads the TTL off the wire rather
// than off the sending socket. The receiver reports what the stack delivered,
// so a value of 255 at the peer is the stack-level answer RFC 5082 Section 3
// asks for: nothing between ze and the peer decremented it.
//
// RFC requirement: RFC5082-3-3 positive -- a datagram ze transmits after
// SetIPTTL(255) is delivered to the peer with an inbound TTL of exactly 255,
// so the TTL of a GTSM-enabled session was not decremented in transit.
func TestGTSMTransmittedTTLArrivesUndecremented(t *testing.T) {
	receiver, sender := udpPairV4(t)
	defer closeOrLog(t, receiver)
	defer closeOrLog(t, sender)

	reportInboundTTL(t, receiver)
	controlUDP(t, sender, func(fd int) error {
		return SetIPTTL(fd, loopbackV4, 255)
	})

	sendByte(t, sender, receiver, 0x42)
	payload, ttl := receiveWithTTL(t, receiver)
	if payload != 0x42 {
		t.Fatalf("payload = %#x, want 0x42", payload)
	}
	if ttl != 255 {
		t.Fatalf("inbound TTL = %d, want exactly 255", ttl)
	}
}

// TestGTSMTransmittedTTLReportsTheValueSet is the discrimination for the test
// above. The two datagrams differ in the TTL ze sets, so a 255 observed there
// is bound to what SetIPTTL wrote and not to a receiver that answers 255 for
// everything.
//
// RFC requirement: RFC5082-3-3 negative -- a datagram ze transmits after
// SetIPTTL(254) is delivered with an inbound TTL of exactly 254. The receive
// path reports the true TTL, and no layer rewrites it upward to 255.
func TestGTSMTransmittedTTLReportsTheValueSet(t *testing.T) {
	receiver, sender := udpPairV4(t)
	defer closeOrLog(t, receiver)
	defer closeOrLog(t, sender)

	reportInboundTTL(t, receiver)
	controlUDP(t, sender, func(fd int) error {
		return SetIPTTL(fd, loopbackV4, 254)
	})

	sendByte(t, sender, receiver, 0x43)
	payload, ttl := receiveWithTTL(t, receiver)
	if payload != 0x43 {
		t.Fatalf("payload = %#x, want 0x43", payload)
	}
	if ttl != 254 {
		t.Fatalf("inbound TTL = %d, want exactly 254", ttl)
	}
}

// TestGTSMFloorDeliversATrustedPacket covers the Trusted half of the Section 3
// receive rule. IP_MINTTL is a floor rather than an equality, so a packet whose
// TTL is inside the session's expected range reaches the protocol.
//
// RFC requirement: RFC5082-3-4 positive -- a packet arriving at the TTL floor
// ze installed is classified Trusted and is delivered, so GTSM processing does
// not drop a packet RFC 5082 Section 3 forbids it to drop.
func TestGTSMFloorDeliversATrustedPacket(t *testing.T) {
	receiver, sender := udpPairV4(t)
	defer closeOrLog(t, receiver)
	defer closeOrLog(t, sender)

	controlUDP(t, receiver, func(fd int) error {
		return SetIPMinTTL(fd, loopbackV4, 255)
	})
	controlUDP(t, sender, func(fd int) error {
		return SetIPTTL(fd, loopbackV4, 255)
	})

	sendByte(t, sender, receiver, 0x44)
	if got := receiveByte(t, receiver); got != 0x44 {
		t.Fatalf("delivered payload = %#x, want 0x44", got)
	}
}

// TestGTSMNoFloorDeliversAnUnknownPacket covers the Unknown half of the same
// rule. Ze installs a floor only where a session configured one:
// tuneTCPConnectionForSettings calls SetIPMinTTL when PeerSettings.MinTTL is
// non-zero, and SetIPMinTTL itself returns before the setsockopt when the value
// is zero. A packet no GTSM-enabled session claims therefore meets no gate.
//
// RFC requirement: RFC5082-3-4 negative -- a socket for which ze configured no
// TTL floor carries IP_MINTTL 0 and delivers a packet whose TTL is far below
// 255, so a packet classified Unknown is not dropped by GTSM processing.
func TestGTSMNoFloorDeliversAnUnknownPacket(t *testing.T) {
	receiver, sender := udpPairV4(t)
	defer closeOrLog(t, receiver)
	defer closeOrLog(t, sender)

	controlUDP(t, receiver, func(fd int) error {
		return SetIPMinTTL(fd, loopbackV4, 0)
	})
	if got := socketOption(t, receiver, unix.IPPROTO_IP, unix.IP_MINTTL); got != 0 {
		t.Fatalf("IP_MINTTL = %d with no floor configured, want exactly 0", got)
	}

	controlUDP(t, sender, func(fd int) error {
		return SetIPTTL(fd, loopbackV4, 1)
	})
	sendByte(t, sender, receiver, 0x45)
	if got := receiveByte(t, receiver); got != 0x45 {
		t.Fatalf("delivered payload = %#x, want 0x45", got)
	}
}

func listenTCPV4(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("listen tcp4 127.0.0.1:0: %v", err)
	}
	return listener
}

func dialTCPV4(t *testing.T, dialer RealDialer, address string) *net.TCPConn {
	t.Helper()
	conn, err := dialer.DialContext(t.Context(), "tcp4", address)
	if err != nil {
		t.Fatalf("DialContext %s: %v", address, err)
	}
	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		closeOrLog(t, conn)
		t.Fatalf("dialed connection is %T, want *net.TCPConn", conn)
	}
	return tcp
}

func udpPairV4(t *testing.T) (*net.UDPConn, *net.UDPConn) {
	t.Helper()
	receiver, err := net.ListenUDP("udp4", &net.UDPAddr{IP: loopbackV4})
	if err != nil {
		t.Skipf("listen udp4 127.0.0.1:0: %v", err)
	}
	sender, err := net.ListenUDP("udp4", &net.UDPAddr{IP: loopbackV4})
	if err != nil {
		closeOrLog(t, receiver)
		t.Skipf("listen udp4 sender: %v", err)
	}
	return receiver, sender
}

// reportInboundTTL asks the kernel to deliver each datagram's IP TTL as a
// control message, which is how the receiver observes what arrived rather than
// what was sent.
func reportInboundTTL(t *testing.T, conn *net.UDPConn) {
	t.Helper()
	controlUDP(t, conn, func(fd int) error {
		return unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_RECVTTL, 1)
	})
}

func sendByte(t *testing.T, sender, receiver *net.UDPConn, payload byte) {
	t.Helper()
	target, ok := receiver.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("receiver address is %T, want *net.UDPAddr", receiver.LocalAddr())
	}
	if _, err := sender.WriteToUDP([]byte{payload}, target); err != nil {
		t.Fatalf("WriteToUDP: %v", err)
	}
}

func receiveByte(t *testing.T, conn *net.UDPConn) byte {
	t.Helper()
	deadline(t, conn)
	buf := make([]byte, 1)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			t.Fatal("no datagram delivered before the deadline; the packet was dropped")
		}
		t.Fatalf("ReadFromUDP: %v", err)
	}
	if n != 1 {
		t.Fatalf("read %d byte(s), want 1", n)
	}
	return buf[0]
}

// receiveWithTTL answers the payload byte and the inbound TTL the kernel
// reported in the IP_RECVTTL control message.
func receiveWithTTL(t *testing.T, conn *net.UDPConn) (byte, int) {
	t.Helper()
	deadline(t, conn)
	buf := make([]byte, 1)
	oob := make([]byte, unix.CmsgSpace(4))
	n, oobn, _, _, err := conn.ReadMsgUDP(buf, oob)
	if err != nil {
		t.Fatalf("ReadMsgUDP: %v", err)
	}
	if n != 1 {
		t.Fatalf("read %d byte(s), want 1", n)
	}
	messages, err := unix.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		t.Fatalf("ParseSocketControlMessage: %v", err)
	}
	for _, message := range messages {
		if message.Header.Level != unix.IPPROTO_IP || message.Header.Type != unix.IP_TTL {
			continue
		}
		if len(message.Data) == 0 {
			t.Fatal("IP_TTL control message carries no data")
		}
		return buf[0], int(message.Data[0])
	}
	t.Fatal("no IP_TTL control message; the inbound TTL was not reported")
	return 0, 0
}

func deadline(t *testing.T, conn *net.UDPConn) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
}

func controlUDP(t *testing.T, conn *net.UDPConn, fn func(fd int) error) {
	t.Helper()
	raw, err := conn.SyscallConn()
	if err != nil {
		t.Fatalf("SyscallConn: %v", err)
	}
	var inner error
	if err := raw.Control(func(fd uintptr) {
		inner = fn(int(fd))
	}); err != nil {
		t.Fatalf("Control: %v", err)
	}
	if inner != nil {
		t.Fatalf("socket option: %v", inner)
	}
}

// rawConn is the one thing socketOption needs from a connection: the raw file
// descriptor. *net.TCPConn and *net.UDPConn both satisfy it.
type rawConn interface {
	SyscallConn() (syscall.RawConn, error)
}

// socketOption reads one integer socket option off any connection that exposes
// a raw file descriptor.
func socketOption(t *testing.T, conn rawConn, level, option int) int {
	t.Helper()
	raw, err := conn.SyscallConn()
	if err != nil {
		t.Fatalf("SyscallConn: %v", err)
	}
	var value int
	var inner error
	if err := raw.Control(func(fd uintptr) {
		value, inner = unix.GetsockoptInt(int(fd), level, option)
	}); err != nil {
		t.Fatalf("Control: %v", err)
	}
	if inner != nil {
		t.Fatalf("getsockopt level %d option %d: %v", level, option, inner)
	}
	return value
}

// systemDefaultTTL answers the IP_TTL a socket carries with nothing configured,
// which is what the GTSM-off case must still read back.
func systemDefaultTTL(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: loopbackV4})
	if err != nil {
		t.Skipf("listen udp4 for the default TTL: %v", err)
	}
	defer closeOrLog(t, conn)
	return socketOption(t, conn, unix.IPPROTO_IP, unix.IP_TTL)
}
