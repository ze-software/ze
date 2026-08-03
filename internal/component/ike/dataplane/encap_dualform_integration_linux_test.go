//go:build integration && linux

package dataplane

import (
	"encoding/binary"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// Phase 1 evidence for plan/spec-ipsec-esp-dual-form-receive.md.
//
// The sibling file's TestEncapKernelBindsOneESPFormPerState records that ONE inbound
// XFRM state binds exactly ONE ESP wire form. Two of that spec's assumptions ask
// whether a cheaper way around the limit exists, and neither had been measured:
//
//	A-1  Do TWO states on ONE SPI let both forms through? engine/child.go calls this
//	     REASONED and not measured, and the sibling probe states it as fact while
//	     installing its two states on two DISTINCT SPIs.
//	A-3  Can a userspace reader present an encapsulated datagram to a template-free
//	     state as bare ESP, preserving the outer source address XFRM needs?
//
// Linux 6.19.11 decides both in one line, net/xfrm/xfrm_input.c:634:
//
//	if ((x->encap ? x->encap->encap_type : 0) != encap_type) {
//	        XFRM_INC_STATS(net, LINUX_MIB_XFRMINSTATEMISMATCH);
//
// It is symmetric, so a template-free state demands encap_type == 0 and a templated
// state demands its own type. These probes measure that the kernel behaves as that
// line reads, because a source read is a hypothesis until a kernel runs it.
//
// Neither probe carries an RFC requirement tag. They record what the kernel does.
// They do not assert that Ze meets an obligation.

const (
	encapSPIDualForm      = 0x00ABCDE3
	encapSPIReinjected    = 0x00ABCDE4
	encapSPIReinjectNoSA  = 0x00ABCDE5
	encapLoopbackPeerAddr = "127.0.0.2"
)

// encapChildEnv marks the re-executed probe process.
const encapChildEnv = "ZE_ENCAP_PROBE_OWN_PROCESS"

// encapOwnProcess gives the calling probe a process to itself. It returns true in the
// child, where the probe body must run, and false in the parent, where the body must be
// skipped because the child has already run it.
//
// MEASURED, and it is the reason this exists: this package supports exactly ONE network
// namespace probe per test BINARY. Each probe passes when it runs alone. Run two in one
// process and the first passes while every later one reports that no xfrm counter moved
// at all. It is not a property of the new probes: the pre-existing
// TestEncapKernelBindsOneESPFormPerState under `-count=2` passes its first iteration and
// fails its second with no new code involved.
//
// The mechanism is NOT diagnosed. What is measured is the boundary: one unshare per
// process is reliable and two are not. So the contract is enforced structurally rather
// than left to an ordering assumption, because `make ze-qemu-integration-test` runs the
// whole package and a later probe would otherwise report a product defect that does not
// exist.
//
// Every namespace probe in this package MUST open with this call. A probe that skips it
// works only while it happens to run first.
func encapOwnProcess(t *testing.T) bool {
	t.Helper()
	if os.Getenv(encapChildEnv) == "1" {
		return true
	}

	cmd := exec.Command(os.Args[0], "-test.run", "^"+t.Name()+"$", "-test.v")
	cmd.Env = append(os.Environ(), encapChildEnv+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("probe in its own process failed: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "--- SKIP") {
		// test-relax: this is skip PROPAGATION, not relaxation. The probe body still
		// runs every assertion in the child; the child skips only where it always did,
		// when the kernel refuses CLONE_NEWNET or a raw socket. Reporting the child's
		// skip as a parent PASS would be the fail-open reading of the same result.
		t.Skipf("probe skipped in its own process:\n%s", out)
	}
	if !strings.Contains(string(out), "--- PASS") {
		t.Fatalf("probe in its own process reported neither PASS nor SKIP:\n%s", out)
	}
	t.Logf("ran in its own process:\n%s", out)
	return false
}

// encapNetnsUsable fails the probe when loopback is not carrying traffic in the namespace
// it just entered. Without it a broken fixture reads as "the kernel accepted nothing",
// because datagrams that die before XFRM move no counter at all
// (ai/rules/evidence.md). netlink answers for the CALLING thread's namespace,
// which is the one the probe runs in.
func encapNetnsUsable(t *testing.T) {
	t.Helper()
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("lo lookup: %v", err)
	}
	if attrs := lo.Attrs(); attrs.Flags&net.FlagUp == 0 {
		t.Fatalf("loopback is down; datagrams would never reach XFRM and every counter would read zero")
	}
}

// encapStateFor builds one inbound ESP state between the two addresses given, with or
// without the ESP-in-UDP template. encapAddState is the loopback-to-loopback case that
// fails the test on error. These probes must SEE the kernel's error rather than die on
// it, and they must vary the addresses.
func encapStateFor(spi int, withEncap bool, src, dst string) *netlink.XfrmState {
	state := &netlink.XfrmState{
		Src:   net.ParseIP(src),
		Dst:   net.ParseIP(dst),
		Proto: netlink.XFRM_PROTO_ESP,
		Mode:  netlink.XFRM_MODE_TUNNEL,
		Spi:   spi,
		Aead: &netlink.XfrmStateAlgo{
			Name:   "rfc4106(gcm(aes))",
			Key:    append([]byte(nil), encapTestAEADKey...),
			ICVLen: 128,
		},
	}
	if withEncap {
		state.Encap = &netlink.XfrmStateEncap{
			Type:    netlink.XFRM_ENCAP_ESPINUDP,
			SrcPort: encapNATTPortForTests,
			DstPort: encapNATTPortForTests,
		}
	}
	return state
}

// encapVerdictOf runs one send and returns the name of the single xfrm counter it
// moved. It is encapKernelVerdict's generic half: these probes build their own
// datagrams, so they cannot use that function's two fixed shapes.
//
// A send syscall returning success proves nothing here. sendto reports that the packet
// left userspace, never that XFRM accepted it, so every verdict below is read from
// /proc/net/xfrm_stat rather than from a write result.
func encapVerdictOf(t *testing.T, send func()) string {
	t.Helper()
	names := []string{encapStatNoStates, encapStatMismatch, encapStatProtoError}
	before := make(map[string]int, len(names))
	for _, n := range names {
		before[n] = encapStat(t, n)
	}

	send()

	var raised []string
	for _, n := range names {
		if encapStat(t, n) > before[n] {
			raised = append(raised, n)
		}
	}
	if len(raised) != 1 {
		t.Fatalf("expected exactly one counter to move, got %v", raised)
	}
	return raised[0]
}

// encapInjectBare writes one bare ESP datagram into the local input path carrying the
// outer addresses the caller names. IPPROTO_RAW implies IP_HDRINCL, so the header below
// is the one that goes on the wire, and a userspace receive path can therefore preserve
// the peer's source address when it re-injects. Route A would use exactly this
// mechanism, so the probe uses it rather than a convenience socket that would pick its
// own source address and prove nothing about the real case.
func encapInjectBare(t *testing.T, src, dst string, esp []byte) {
	t.Helper()
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW, unix.IPPROTO_RAW)
	if err != nil {
		t.Skipf("no raw socket (needs CAP_NET_RAW): %v", err)
	}
	defer func() {
		if cerr := unix.Close(fd); cerr != nil {
			t.Errorf("close raw fd: %v", cerr)
		}
	}()

	srcIP, dstIP := net.ParseIP(src).To4(), net.ParseIP(dst).To4()
	if srcIP == nil || dstIP == nil {
		t.Fatalf("probe addresses must be IPv4, got src=%q dst=%q", src, dst)
	}

	pkt := make([]byte, 20+len(esp))
	pkt[0] = 0x45 // IPv4, five-word header
	binary.BigEndian.PutUint16(pkt[2:4], uint16(len(pkt)))
	pkt[8] = 64               // TTL
	pkt[9] = unix.IPPROTO_ESP // protocol 50
	copy(pkt[12:16], srcIP)
	copy(pkt[16:20], dstIP)
	copy(pkt[20:], esp)
	// The header checksum stays zero on purpose. The kernel fills it for an
	// IP_HDRINCL socket.

	var addr unix.SockaddrInet4
	copy(addr.Addr[:], dstIP)
	if err := unix.Sendto(fd, pkt, 0, &addr); err != nil {
		t.Fatalf("inject bare ESP %s->%s: %v", src, dst, err)
	}
}

// VALIDATES: assumption A-1. Whether a SECOND XFRM state on the SAME SPI lets the other
// ESP form reach the crypto check.
// PREVENTS: a userspace receive seam being built to route around a limit that a second
// state would have lifted for free, and equally the opposite error of believing a second
// state works when the kernel refuses to install one.
//
// One goroutine, no subtests. runtime.LockOSThread binds the NAMESPACE to this
// goroutine's thread, and a t.Run subtest would read the HOST namespace instead.
func TestEncapTwoStatesOneSPI(t *testing.T) {
	if !encapOwnProcess(t) {
		return
	}
	encapNetns(t)
	encapNetnsUsable(t)

	if err := netlink.XfrmStateAdd(encapStateFor(encapSPIDualForm, false, encapLoopbackAddr, encapLoopbackAddr)); err != nil {
		t.Fatalf("add template-free state: %v", err)
	}

	// Two ways a second state might coexist. Identical addresses first, then a differing
	// source: __xfrm_state_lookup keys on destination, SPI, protocol, family and mark
	// (xfrm_state.c:1184-1192), so the source is the only field left that could separate
	// two states on one SPI.
	sameAddrErr := netlink.XfrmStateAdd(encapStateFor(encapSPIDualForm, true, encapLoopbackAddr, encapLoopbackAddr))
	t.Logf("A-1: second state, same SPI, identical addresses: err=%v", sameAddrErr)

	diffSrcErr := netlink.XfrmStateAdd(encapStateFor(encapSPIDualForm, true, encapLoopbackPeerAddr, encapLoopbackAddr))
	t.Logf("A-1: second state, same SPI, differing source: err=%v", diffSrcErr)
	t.Logf("A-1: a second state on one SPI was installed: %v", sameAddrErr == nil || diffSrcErr == nil)

	bare := encapVerdictOf(t, func() {
		encapInjectBare(t, encapLoopbackAddr, encapLoopbackAddr, encapESPBytes(encapSPIDualForm))
	})
	encapsulated := encapKernelVerdict(t, encapSPIDualForm, true)
	t.Logf("A-1 verdicts: bare=%s encapsulated=%s", bare, encapsulated)

	// The template-free state must still take bare ESP whatever happened above. If it
	// does not, this probe is measuring something other than what it claims.
	if bare != encapStatProtoError {
		t.Errorf("bare ESP raised %s, want %s; the first state is not serving its own form, so the encapsulated verdict means nothing",
			bare, encapStatProtoError)
	}

	// The recorded belief: a second state does not lift the limit. A failure here is a
	// FINDING, not a regression. It would mean a cheap route exists and the spec's route
	// selection must be redone.
	if encapsulated != encapStatMismatch {
		t.Errorf("A-1 BROKEN: with a second state present the encapsulated form raised %s, want %s. Two states on one SPI DO serve both forms; record it in the spec and re-cost route A",
			encapsulated, encapStatMismatch)
	}

	// Discrimination control, on the same goroutine and namespace. An SPI with no state
	// raises a THIRD counter, so the two verdicts above are real readings rather than a
	// counter that never moves.
	if got := encapKernelVerdict(t, encapSPIUnknown, false); got != encapStatNoStates {
		t.Errorf("an SPI with no state raised %s, want %s; the counters cannot discriminate and this test proves nothing",
			got, encapStatNoStates)
	}
}

// VALIDATES: assumption A-3. Whether a userspace reader can take a UDP-encapsulated ESP
// datagram off a socket carrying NO UDP_ENCAP option and present it to a template-free
// XFRM state as bare ESP, preserving the outer source address.
// PREVENTS: route A being adopted on the strength of the sibling probe's bare-ESP row,
// which sends from a socket that picks its own source and therefore never tests whether
// the peer's address survives re-injection.
//
// The kernel hands those datagrams to userspace already: __xfrm4_udp_encap_rcv returns 1
// when the socket carries no encap type (net/ipv4/xfrm4_input.c:91-94), which is the
// socket state route A would keep.
//
// One goroutine, no subtests, for the namespace reason above.
func TestEncapReinjectedBareESPAccepted(t *testing.T) {
	if !encapOwnProcess(t) {
		return
	}
	encapNetns(t)
	encapNetnsUsable(t)

	// The state a route-A receive path would install: template-free, so the kernel takes
	// bare ESP natively and userspace supplies the other form.
	if err := netlink.XfrmStateAdd(encapStateFor(encapSPIReinjected, false, encapLoopbackPeerAddr, encapLoopbackAddr)); err != nil {
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
	if _, err := sender.Write(encapESPBytes(encapSPIReinjected)); err != nil {
		t.Fatalf("send encapsulated ESP: %v", err)
	}

	buf := make([]byte, 2048)
	n, from, err := uc.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("read the encapsulated datagram in userspace: %v", err)
	}
	if from.IP.String() != encapLoopbackPeerAddr {
		t.Fatalf("datagram arrived from %s, want %s; the probe cannot test source preservation",
			from.IP, encapLoopbackPeerAddr)
	}

	verdict := encapVerdictOf(t, func() {
		encapInjectBare(t, from.IP.String(), encapLoopbackAddr, buf[:n])
	})
	t.Logf("A-3 verdict: re-injected bare ESP raised %s", verdict)
	if verdict != encapStatProtoError {
		t.Errorf("A-3 BROKEN: re-injected bare ESP raised %s, want %s. Route A cannot present the encapsulated form to a template-free state, and the leading candidate is gone",
			verdict, encapStatProtoError)
	}

	// Discrimination control. Without it a counter that never moves would read as success.
	if got := encapVerdictOf(t, func() {
		encapInjectBare(t, encapLoopbackPeerAddr, encapLoopbackAddr, encapESPBytes(encapSPIReinjectNoSA))
	}); got != encapStatNoStates {
		t.Errorf("an injected SPI with no state raised %s, want %s; the verdict above proves nothing",
			got, encapStatNoStates)
	}
}
