//go:build integration && linux

package dataplane

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// Phase 2 evidence for plan/spec-ipsec-esp-dual-form-receive.md.
//
// Phase 1 chose route A: the kernel serves the EXPECTED ESP form for a Child SA and a
// userspace reader catches the other one. That design has one unproven half per
// direction, and the halves are not symmetric:
//
//	M1  Under NAT the inbound state carries an ESP-in-UDP template, so the expected form
//	    is encapsulated and the UNEXPECTED form is bare ESP. Bare ESP is IP protocol 50
//	    and goes straight to XFRM, which refuses it with XfrmInStateMismatch. Does a
//	    userspace raw IPPROTO_ESP reader see the datagram at all, or does the drop happen
//	    where no userspace code can reach it?
//
//	M2  With no NAT the inbound state is template-free, so the expected form is bare and
//	    the UNEXPECTED form is encapsulated. TestEncapReinjectedBareESPAccepted already
//	    measured that userspace CAN read it when the port-4500 socket carries no
//	    UDP_ENCAP option. Ze sets that option today (engine/register.go, EnableESPInUDP).
//	    Is the datagram still visible to userspace while the option is set?
//
// Together they decide whether the per-SA hybrid is buildable, because UDP_ENCAP is a
// per-SOCKET option and Ze holds ONE port-4500 socket for every SA.
//
// Neither probe carries an RFC requirement tag. They record what the kernel does. They
// do not assert that Ze meets an obligation.

const (
	encapSPIHybridTemplated  = 0x00ABCDE6
	encapSPIHybridBare       = 0x00ABCDE7
	encapSPIDualBoth         = 0x00ABCDE8
	encapHybridReadDeadline  = 2 * time.Second
	encapHybridPeerAddrOther = "127.0.0.3"
)

// encapReadRawESP reports whether a raw IPPROTO_ESP reader received a datagram carrying
// the SPI given, within a bounded wait. Go strips the IPv4 header from an "ip4:" socket
// read, so the first four octets are the ESP SPI.
//
// A read timeout is the ANSWER here, not a failure: the probes below ask whether the
// datagram is visible at all, so "nothing arrived" is a measurement.
func encapReadRawESP(t *testing.T, c net.PacketConn, spi uint32) bool {
	t.Helper()
	if err := c.SetReadDeadline(time.Now().Add(encapHybridReadDeadline)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	buf := make([]byte, 2048)
	for {
		n, _, err := c.ReadFrom(buf)
		if err != nil {
			return false
		}
		if n < 4 {
			continue
		}
		got := uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
		if got == spi {
			return true
		}
	}
}

// encapInjectRaw writes one pre-built IPv4 datagram into the local input path.
// IPPROTO_RAW implies IP_HDRINCL, so the header the caller built is the one that goes on
// the wire and the kernel fills only the header checksum.
func encapInjectRaw(t *testing.T, dst string, pkt []byte) {
	t.Helper()
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW, unix.IPPROTO_RAW)
	if err != nil {
		// test-relax: capability gate, not a relaxation. CAP_NET_RAW is absent on some
		// hosts and every sibling probe in this package skips the same way
		// (ai/rules/os-specific-tests.md). encapOwnProcess propagates a child SKIP as a
		// parent SKIP rather than reading it as PASS.
		t.Skipf("no raw socket (needs CAP_NET_RAW): %v", err)
	}
	defer func() {
		if cerr := unix.Close(fd); cerr != nil {
			t.Errorf("close raw fd: %v", cerr)
		}
	}()
	var addr unix.SockaddrInet4
	copy(addr.Addr[:], net.ParseIP(dst).To4())
	if err := unix.Sendto(fd, pkt, 0, &addr); err != nil {
		t.Fatalf("inject re-presented datagram to %s: %v", dst, err)
	}
}

// VALIDATES: AC-1, AC-2 and AC-3 together. ONE inbound XFRM state, ONE SPI, and BOTH ESP
// wire forms reach the crypto check. The encapsulated form arrives on the kernel fast
// path; the bare form is refused by XFRM, read off a raw IPPROTO_ESP socket, re-presented
// by the PRODUCTION writeESPForm, and reaches the same state.
// PREVENTS: shipping the re-presentation on the strength of M1 alone. M1 proves userspace
// can SEE the refused datagram. It does not prove that feeding the datagram back through
// the port-4500 socket reaches the state, which is the half the receive path depends on.
//
// This is the dual-form assertion the spec's Wiring Test table names. The row that must
// flip is the third one: today a templated state refuses bare ESP and nothing recovers it.
//
// One goroutine, no subtests. runtime.LockOSThread binds the NAMESPACE to this goroutine's
// thread, and a t.Run subtest would read the HOST namespace instead.
func TestEncapOneStateAcceptsBothForms(t *testing.T) {
	if !encapOwnProcess(t) {
		return
	}
	encapNetns(t)
	encapNetnsUsable(t)

	// ONE state, templated, as an SA on port 4500 installs it.
	if err := netlink.XfrmStateAdd(encapStateFor(encapSPIDualBoth, true, encapLoopbackPeerAddr, encapLoopbackAddr)); err != nil {
		t.Fatalf("add templated state: %v", err)
	}

	// The port-4500 socket in the state production runs it: UDP_ENCAP set, so the kernel
	// decapsulates the expected form.
	uc, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP(encapLoopbackAddr), Port: encapNATTPortForTests})
	if err != nil {
		t.Fatalf("bind udp %d: %v", encapNATTPortForTests, err)
	}
	defer func() {
		if cerr := uc.Close(); cerr != nil {
			t.Errorf("close udp: %v", cerr)
		}
	}()
	rc, err := uc.SyscallConn()
	if err != nil {
		t.Fatalf("syscall conn: %v", err)
	}
	var setErr error
	if ctlErr := rc.Control(func(fd uintptr) {
		setErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_UDP, unix.UDP_ENCAP, unix.UDP_ENCAP_ESPINUDP)
	}); ctlErr != nil {
		t.Fatalf("control: %v", ctlErr)
	}
	if setErr != nil {
		t.Fatalf("UDP_ENCAP: %v", setErr)
	}

	sender, err := net.DialUDP("udp4",
		&net.UDPAddr{IP: net.ParseIP(encapLoopbackPeerAddr), Port: encapNATTPortForTests},
		&net.UDPAddr{IP: net.ParseIP(encapLoopbackAddr), Port: encapNATTPortForTests})
	if err != nil {
		t.Fatalf("dial udp from the peer address: %v", err)
	}
	defer func() {
		if cerr := sender.Close(); cerr != nil {
			t.Errorf("close sender: %v", cerr)
		}
	}()

	reader, err := net.ListenPacket("ip4:esp", encapLoopbackAddr)
	if err != nil {
		// test-relax: capability gate, not a relaxation. See encapInjectRaw above.
		t.Skipf("no raw ESP socket (needs CAP_NET_RAW): %v", err)
	}
	defer func() {
		if cerr := reader.Close(); cerr != nil {
			t.Errorf("close raw reader: %v", cerr)
		}
	}()

	// Row 1: the EXPECTED form takes the kernel fast path and reaches the crypto check.
	if got := encapVerdictOf(t, func() {
		if _, werr := sender.Write(encapESPBytes(encapSPIDualBoth)); werr != nil {
			t.Fatalf("send encapsulated ESP: %v", werr)
		}
	}); got != encapStatProtoError {
		t.Errorf("AC-2: encapsulated ESP raised %s, want %s; the kernel fast path for the expected form is broken",
			got, encapStatProtoError)
	}

	// Row 2: the UNEXPECTED form is refused by XFRM, exactly as the truth table records.
	if got := encapVerdictOf(t, func() {
		encapInjectBare(t, encapLoopbackPeerAddr, encapLoopbackAddr, encapESPBytes(encapSPIDualBoth))
	}); got != encapStatMismatch {
		t.Errorf("bare ESP against a templated state raised %s, want %s; this test is not measuring what it claims",
			got, encapStatMismatch)
	}

	// Row 3, and it is the row that must flip. Userspace reads the refused datagram and
	// re-presents it through the PRODUCTION builder. It must now reach the SAME state.
	if err := reader.SetReadDeadline(time.Now().Add(encapHybridReadDeadline)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	espBuf := make([]byte, 2048)
	n, _, err := reader.ReadFrom(espBuf)
	if err != nil {
		t.Fatalf("AC-1: the refused bare ESP datagram never reached the userspace reader: %v", err)
	}
	esp := espBuf[:n]
	if spi, ok := espFormSPI(esp); !ok || spi != encapSPIDualBoth {
		t.Fatalf("read a datagram with spi=%#x ok=%v, want %#x", spi, ok, uint32(encapSPIDualBoth))
	}

	out := make([]byte, espFormPacketLen(esp))
	wrote := writeESPForm(out, netip.MustParseAddr(encapLoopbackPeerAddr), netip.MustParseAddr(encapLoopbackAddr), esp)
	if wrote == 0 {
		t.Fatal("writeESPForm refused a datagram read off the wire")
	}

	if got := encapVerdictOf(t, func() {
		encapInjectRaw(t, encapLoopbackAddr, out[:wrote])
	}); got != encapStatProtoError {
		t.Errorf("AC-1: the re-presented bare ESP datagram raised %s, want %s. One state does NOT accept both forms, and the receive path does not work",
			got, encapStatProtoError)
	}

	// AC-3, the discrimination control, on the same goroutine and namespace. An SPI with
	// no state raises a THIRD counter, so the three rows above are real readings.
	if got := encapVerdictOf(t, func() {
		encapInjectBare(t, encapLoopbackPeerAddr, encapLoopbackAddr, encapESPBytes(encapSPIUnknown))
	}); got != encapStatNoStates {
		t.Errorf("AC-3: an SPI with no state raised %s, want %s; the rows above prove nothing",
			got, encapStatNoStates)
	}
}

// VALIDATES: M1. Whether a bare ESP datagram that XFRM refuses against a TEMPLATED state
// is visible to a userspace raw IPPROTO_ESP reader.
// PREVENTS: building the under-NAT half of the hybrid receive path on the assumption that
// userspace can catch the unexpected bare form. If the kernel consumes the datagram before
// any raw socket sees it, that half cannot exist and the design must say so.
//
// ip_protocol_deliver_rcu calls raw_local_deliver BEFORE the protocol handler, so a raw
// reader should get a clone even when xfrm4_rcv then drops the original. That is a source
// reading, and a source reading is a hypothesis until a kernel runs it.
//
// One goroutine, no subtests. runtime.LockOSThread binds the NAMESPACE to this goroutine's
// thread, and a t.Run subtest would read the HOST namespace instead.
func TestEncapBareESPVisibleToUserspaceWhenStateIsTemplated(t *testing.T) {
	if !encapOwnProcess(t) {
		return
	}
	encapNetns(t)
	encapNetnsUsable(t)

	// The state the under-NAT case installs: templated, so the kernel serves the
	// encapsulated form and bare ESP is the unexpected one.
	if err := netlink.XfrmStateAdd(encapStateFor(encapSPIHybridTemplated, true, encapLoopbackPeerAddr, encapLoopbackAddr)); err != nil {
		t.Fatalf("add templated state: %v", err)
	}

	reader, err := net.ListenPacket("ip4:esp", encapLoopbackAddr)
	if err != nil {
		t.Skipf("no raw ESP socket (needs CAP_NET_RAW): %v", err)
	}
	defer func() {
		if cerr := reader.Close(); cerr != nil {
			t.Errorf("close raw reader: %v", cerr)
		}
	}()

	verdict := encapVerdictOf(t, func() {
		encapInjectBare(t, encapLoopbackPeerAddr, encapLoopbackAddr, encapESPBytes(encapSPIHybridTemplated))
	})
	t.Logf("M1 kernel verdict for bare ESP against a templated state: %s", verdict)

	// The kernel half must be the refusal this probe is about. If the templated state
	// ACCEPTED bare ESP, the whole spec's premise is wrong and the reader result below
	// would be measuring a different question.
	if verdict != encapStatMismatch {
		t.Errorf("bare ESP against a templated state raised %s, want %s; this probe is not measuring what it claims",
			verdict, encapStatMismatch)
	}

	saw := encapReadRawESP(t, reader, encapSPIHybridTemplated)
	t.Logf("M1 RESULT: a userspace raw IPPROTO_ESP reader saw the refused bare ESP datagram: %v", saw)

	// A failure here is a FINDING, not a regression. It would mean the under-NAT half of
	// the hybrid receive path cannot be built, and the spec must record what compliance
	// that leaves rather than ship a design whose second half does not work.
	if !saw {
		t.Errorf("M1 NEGATIVE: bare ESP refused by a templated state never reached a userspace raw ESP reader. The under-NAT half of the hybrid receive path is unreachable; record the evidence and re-cost the design")
	}

	// Discrimination control, on the same goroutine and namespace. An SPI with no state
	// raises a THIRD counter, so the verdict above is a real reading and not a counter
	// that never moves.
	if got := encapVerdictOf(t, func() {
		encapInjectBare(t, encapLoopbackPeerAddr, encapLoopbackAddr, encapESPBytes(encapSPIUnknown))
	}); got != encapStatNoStates {
		t.Errorf("an injected SPI with no state raised %s, want %s; the verdict above proves nothing",
			got, encapStatNoStates)
	}
}

// VALIDATES: M2. Whether a UDP-encapsulated ESP datagram is visible to the userspace owner
// of the port-4500 socket while that socket carries UDP_ENCAP_ESPINUDP.
// PREVENTS: believing the per-SA hybrid is buildable. The no-NAT half needs userspace to
// catch the encapsulated form, and the under-NAT half needs the kernel to keep serving it.
// UDP_ENCAP is a per-SOCKET option and Ze holds ONE port-4500 socket, so both halves ask
// that single option for opposite settings.
//
// __xfrm4_udp_encap_rcv returns 1 only when the socket carries no encap type
// (net/ipv4/xfrm4_input.c). With the option set it strips the header and hands the packet
// to XFRM instead, so userspace should see nothing. That is a source reading, and this
// probe runs it.
//
// One goroutine, no subtests, for the namespace reason above.
func TestEncapEncapsulatedESPHiddenFromUserspaceWhenSocketDecapsulates(t *testing.T) {
	if !encapOwnProcess(t) {
		return
	}
	encapNetns(t)
	encapNetnsUsable(t)

	// The state the no-NAT case installs: template-free, so the kernel serves bare ESP and
	// the encapsulated form is the unexpected one.
	if err := netlink.XfrmStateAdd(encapStateFor(encapSPIHybridBare, false, encapLoopbackPeerAddr, encapLoopbackAddr)); err != nil {
		t.Fatalf("add template-free state: %v", err)
	}

	uc, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP(encapLoopbackAddr), Port: encapNATTPortForTests})
	if err != nil {
		t.Fatalf("bind udp %d: %v", encapNATTPortForTests, err)
	}
	defer func() {
		if cerr := uc.Close(); cerr != nil {
			t.Errorf("close udp: %v", cerr)
		}
	}()

	// The socket state Ze runs in production today (transport.EnableESPInUDP).
	rc, err := uc.SyscallConn()
	if err != nil {
		t.Fatalf("syscall conn: %v", err)
	}
	var setErr error
	if ctlErr := rc.Control(func(fd uintptr) {
		setErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_UDP, unix.UDP_ENCAP, unix.UDP_ENCAP_ESPINUDP)
	}); ctlErr != nil {
		t.Fatalf("control: %v", ctlErr)
	}
	if setErr != nil {
		t.Fatalf("UDP_ENCAP: %v", setErr)
	}

	sender, err := net.DialUDP("udp4",
		&net.UDPAddr{IP: net.ParseIP(encapLoopbackPeerAddr), Port: encapNATTPortForTests},
		&net.UDPAddr{IP: net.ParseIP(encapLoopbackAddr), Port: encapNATTPortForTests})
	if err != nil {
		t.Fatalf("dial udp from the peer address: %v", err)
	}
	defer func() {
		if cerr := sender.Close(); cerr != nil {
			t.Errorf("close sender: %v", cerr)
		}
	}()

	verdict := encapVerdictOf(t, func() {
		if _, werr := sender.Write(encapESPBytes(encapSPIHybridBare)); werr != nil {
			t.Fatalf("send encapsulated ESP: %v", werr)
		}
	})
	t.Logf("M2 kernel verdict for encapsulated ESP against a template-free state: %s", verdict)

	// The kernel must have taken the datagram and refused it. That is what makes the
	// userspace reading below meaningful.
	if verdict != encapStatMismatch {
		t.Errorf("encapsulated ESP against a template-free state raised %s, want %s; this probe is not measuring what it claims",
			verdict, encapStatMismatch)
	}

	if err := uc.SetReadDeadline(time.Now().Add(encapHybridReadDeadline)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	buf := make([]byte, 2048)
	n, _, readErr := uc.ReadFromUDP(buf)
	saw := readErr == nil && n > 0
	t.Logf("M2 RESULT: userspace read the encapsulated datagram while UDP_ENCAP was set: %v (n=%d err=%v)", saw, n, readErr)

	// The recorded expectation: the kernel consumed it, so userspace sees nothing. A
	// failure here is a FINDING that would make the per-SA hybrid buildable after all.
	if saw {
		t.Errorf("M2 UNEXPECTED: userspace read an encapsulated ESP datagram while UDP_ENCAP was set. The per-SA hybrid may be buildable; re-read xfrm4_input.c and re-cost the design")
	}
}
