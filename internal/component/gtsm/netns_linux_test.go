//go:build linux

// Design: docs/architecture/testing/qemu-integration.md -- a kernel test owns a namespace of its own
// Overview: gtsm_rfc5082_linux_test.go -- the RFC 5082 proofs this harness carries
//
// The proofs need a kernel that really generates an ICMP error, and really
// evaluates an nftables input chain, so they need a network namespace of their
// own: programming the firewall or a route in the host namespace would reach
// the machine the test runs on.
//
// The namespace holds one veth pair. Ze's side carries the addresses and is
// where the kernel under test answers; the other side is where the test
// injects frames, because a frame sent out of one end of a veth pair arrives
// as input on the other. Both ends live in the same namespace, which is what
// lets one test process play both roles.
//
// Every test that uses this harness runs on a locked OS thread. A network
// namespace is a property of the thread, so a goroutine that moved would
// program the host.

package gtsm

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

// The addresses the harness configures. Both prefixes are documentation
// ranges (RFC 5737, RFC 3849), so a frame that escapes the namespace names
// nothing real.
var (
	zeV4   = netip.MustParseAddr("192.0.2.1")
	peerV4 = netip.MustParseAddr("192.0.2.2")
	zeV6   = netip.MustParseAddr("2001:db8::1")
	peerV6 = netip.MustParseAddr("2001:db8::2")
)

// peerMAC is the hardware address the harness gives the peer. It is written
// into the neighbor table as permanent, so the kernel never waits for ARP or
// Neighbor Discovery before it sends the ICMP error the test is waiting for.
var peerMAC = net.HardwareAddr{0x02, 0x00, 0x00, 0x00, 0x00, 0x02}

// captureWait bounds every read of the capture socket. A kernel that does not
// answer is the failure the test reports, so the bound is the test's own and
// is several times the time an answer takes on an idle veth.
const captureWait = 3 * time.Second

type testbed struct {
	t         *testing.T
	zeIndex   int
	zeMAC     net.HardwareAddr
	injectFD  int
	captureFD int
}

// newTestbed enters a fresh network namespace, builds the veth pair, and opens
// the two packet sockets the tests read and write through.
//
// It SKIPS rather than fails when the namespace or the veth cannot be created.
// One test file runs unprivileged in the merge gate and privileged in the
// QEMU and container runs, and a missing capability is not a broken product.
func newTestbed(t *testing.T) *testbed {
	t.Helper()

	runtime.LockOSThread()
	origin, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Skipf("needs a network namespace of its own: %v", err)
	}
	if err := unix.Unshare(unix.CLONE_NEWNET); err != nil {
		origin.Close() //nolint:errcheck // best-effort cleanup
		runtime.UnlockOSThread()
		t.Skipf("needs CAP_NET_ADMIN to unshare a network namespace: %v", err)
	}
	t.Cleanup(func() {
		if err := netns.Set(origin); err != nil {
			t.Errorf("cannot return to the original network namespace: %v", err)
		}
		origin.Close() //nolint:errcheck // best-effort cleanup
		runtime.UnlockOSThread()
	})

	bed := &testbed{t: t}
	bed.buildVeth()
	bed.silenceICMPRateLimit()
	bed.captureFD = bed.openPacketSocket(bed.zeIndex)
	bed.injectFD = bed.openPacketSocket(bed.peerLinkIndex())
	return bed
}

// buildVeth creates the pair, addresses ze's end, and pins the peer's
// neighbor entry so no address resolution stands between the kernel's answer
// and the capture socket.
func (b *testbed) buildVeth() {
	b.t.Helper()

	veth := &netlink.Veth{
		Name:     "zegtsm0",
		PeerName: "zegtsm1",
	}
	if err := netlink.LinkAdd(veth); err != nil {
		b.t.Skipf("needs CAP_NET_ADMIN to create a veth pair: %v", err)
	}

	zeLink, err := netlink.LinkByName("zegtsm0")
	if err != nil {
		b.t.Fatalf("veth end zegtsm0: %v", err)
	}
	peerLink, err := netlink.LinkByName("zegtsm1")
	if err != nil {
		b.t.Fatalf("veth end zegtsm1: %v", err)
	}
	for _, link := range []netlink.Link{zeLink, peerLink} {
		if err := netlink.LinkSetUp(link); err != nil {
			b.t.Fatalf("link up %s: %v", link.Attrs().Name, err)
		}
	}

	b.zeIndex = zeLink.Attrs().Index
	b.zeMAC = zeLink.Attrs().HardwareAddr

	b.addAddress(zeLink, zeV4, 24)
	b.addAddress(zeLink, zeV6, 64)
	b.addNeighbor(zeLink, peerV4)
	b.addNeighbor(zeLink, peerV6)

	// An IPv6 address is tentative until duplicate address detection finishes,
	// and the kernel sends nothing from a tentative address. The namespace has
	// one neighbor and it is the test's own injector, so waiting for the timer
	// is waiting for nobody.
	b.waitForIPv6(zeV6)
}

func (b *testbed) addAddress(link netlink.Link, addr netip.Addr, bits int) {
	b.t.Helper()

	ip := net.IP(addr.AsSlice())
	err := netlink.AddrAdd(link, &netlink.Addr{
		IPNet: &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, addr.BitLen())},
	})
	if err != nil {
		b.t.Fatalf("address %s on %s: %v", addr, link.Attrs().Name, err)
	}
}

func (b *testbed) addNeighbor(link netlink.Link, addr netip.Addr) {
	b.t.Helper()

	err := netlink.NeighSet(&netlink.Neigh{
		LinkIndex:    link.Attrs().Index,
		Family:       neighFamily(addr),
		State:        netlink.NUD_PERMANENT,
		IP:           net.IP(addr.AsSlice()),
		HardwareAddr: peerMAC,
	})
	if err != nil {
		b.t.Fatalf("neighbor %s: %v", addr, err)
	}
}

func neighFamily(addr netip.Addr) int {
	if addr.Is4() {
		return unix.AF_INET
	}
	return unix.AF_INET6
}

// waitForIPv6 waits for the address to leave the tentative state.
func (b *testbed) waitForIPv6(addr netip.Addr) {
	b.t.Helper()

	link, err := netlink.LinkByIndex(b.zeIndex)
	if err != nil {
		b.t.Fatalf("link by index %d: %v", b.zeIndex, err)
	}
	deadline := time.Now().Add(captureWait)
	for time.Now().Before(deadline) {
		addrs, err := netlink.AddrList(link, unix.AF_INET6)
		if err != nil {
			// An interrupted dump is this poll's normal condition: the
			// kernel changed the address set (DAD finishing, autoconf adding
			// an address) while it was being listed. The addresses it did
			// return are read, and the next iteration lists again.
			if !errors.Is(err, netlink.ErrDumpInterrupted) {
				b.t.Fatalf("address list: %v", err)
			}
		}
		for _, a := range addrs {
			if !a.IP.Equal(net.IP(addr.AsSlice())) {
				continue
			}
			if a.Flags&unix.IFA_F_TENTATIVE == 0 {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	b.t.Fatalf("address %s stayed tentative for %s", addr, captureWait)
}

// silenceICMPRateLimit turns off the kernel's ICMP rate limiter inside this
// namespace. The limiter is per namespace, and its default lets one
// destination-unreachable through per second: a test that sends two would read
// the first answer twice or wait for nothing.
func (b *testbed) silenceICMPRateLimit() {
	b.t.Helper()

	for _, knob := range []string{
		"/proc/sys/net/ipv4/icmp_ratelimit",
		"/proc/sys/net/ipv6/icmp/ratelimit",
	} {
		if err := os.WriteFile(knob, []byte("0"), 0o644); err != nil {
			b.t.Fatalf("write %s: %v", knob, err)
		}
	}
}

func (b *testbed) peerLinkIndex() int {
	b.t.Helper()

	link, err := netlink.LinkByName("zegtsm1")
	if err != nil {
		b.t.Fatalf("veth end zegtsm1: %v", err)
	}
	return link.Attrs().Index
}

// openPacketSocket opens a raw packet socket bound to one interface, seeing
// every frame in both directions. The read timeout is what turns "the kernel
// never answered" into a test failure rather than a hung run.
func (b *testbed) openPacketSocket(ifIndex int) int {
	b.t.Helper()

	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_ALL)))
	if err != nil {
		b.t.Skipf("needs CAP_NET_RAW to open a packet socket: %v", err)
	}
	b.t.Cleanup(func() { unix.Close(fd) }) //nolint:errcheck // best-effort cleanup

	addr := &unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ALL),
		Ifindex:  ifIndex,
	}
	if err := unix.Bind(fd, addr); err != nil {
		b.t.Fatalf("bind packet socket to interface %d: %v", ifIndex, err)
	}
	timeout := unix.NsecToTimeval(int64(200 * time.Millisecond))
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &timeout); err != nil {
		b.t.Fatalf("set receive timeout: %v", err)
	}
	return fd
}

// htons is the byte order a packet socket's protocol field is given in.
func htons(v uint16) uint16 { return v<<8 | v>>8 }

// inject writes one ethernet frame out of the peer's end of the veth pair, so
// the kernel sees it arrive on ze's end.
func (b *testbed) inject(frame []byte) {
	b.t.Helper()

	addr := &unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ALL),
		Ifindex:  b.peerLinkIndex(),
		Halen:    6,
	}
	copy(addr.Addr[:], b.zeMAC)
	if err := unix.Sendto(b.injectFD, frame, 0, addr); err != nil {
		b.t.Fatalf("inject frame: %v", err)
	}
}

// awaitPacket reads frames off ze's interface until one satisfies want, or
// until the wait is over. It returns the network-layer bytes, which is the
// frame without its ethernet header.
func (b *testbed) awaitPacket(what string, want func(packet []byte) bool) []byte {
	b.t.Helper()

	buf := make([]byte, 2048)
	deadline := time.Now().Add(captureWait)
	for time.Now().Before(deadline) {
		n, _, err := unix.Recvfrom(b.captureFD, buf, 0)
		if err != nil {
			continue // the read timeout, or a signal; the deadline is the bound
		}
		if n <= 14 {
			continue
		}
		packet := buf[14:n]
		if want(packet) {
			out := make([]byte, len(packet))
			copy(out, packet)
			return out
		}
	}
	b.t.Fatalf("the kernel sent no %s within %s", what, captureWait)
	return nil
}

// awaitICMPv4 waits for an ICMPv4 message of the given type addressed to the
// peer, and returns the whole IPv4 packet, TTL byte included.
func (b *testbed) awaitICMPv4(icmpType uint8) []byte {
	b.t.Helper()

	what := fmt.Sprintf("ICMPv4 type %d to %s", icmpType, peerV4)
	return b.awaitPacket(what, func(packet []byte) bool {
		if len(packet) < 28 || packet[0]>>4 != 4 || packet[9] != unix.IPPROTO_ICMP {
			return false
		}
		if !net.IP(packet[16:20]).Equal(net.IP(peerV4.AsSlice())) {
			return false
		}
		return packet[20] == icmpType
	})
}

// awaitICMPv6 waits for an ICMPv6 message of the given type addressed to the
// peer, and returns the whole IPv6 packet, hop-limit byte included.
func (b *testbed) awaitICMPv6(icmpType uint8) []byte {
	b.t.Helper()

	what := fmt.Sprintf("ICMPv6 type %d to %s", icmpType, peerV6)
	return b.awaitPacket(what, func(packet []byte) bool {
		if len(packet) < 48 || packet[0]>>4 != 6 || packet[6] != unix.IPPROTO_ICMPV6 {
			return false
		}
		if !net.IP(packet[24:40]).Equal(net.IP(peerV6.AsSlice())) {
			return false
		}
		return packet[40] == icmpType
	})
}
