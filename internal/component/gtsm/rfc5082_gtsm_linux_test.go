//go:build linux

// Design: rfc/short/rfc5082.md -- GTSM, the TTL 255 rule for related ICMP messages
// Overview: gtsm.go, route_linux.go -- the kernel state these proofs assert
// Related: netns_linux_test.go, packet_linux_test.go -- the namespace and the frames
//
// RFC 5082 Section 3: "The TTL field in all IP packets used for transmission
// of messages associated with GTSM-enabled protocol sessions MUST be set to
// 255. This also applies to the related ICMP error handling messages."
// Section 6.1: "This specification mandates setting and verifying TTL=255 of
// those as well as the main protocol packets."
//
// The requirement has four cells: transmit and receive, in each family. Every
// one of them is answered by the kernel, on state ze installs, so each proof
// here reads what ze installed and then what the kernel did with it.
//
// These tests are NOT tagged `integration`. `./le rfc discriminate-record`
// runs a tagged unit with an empty build-tag set, so a unit behind that tag
// matches nothing, `go test` exits 0, and no break can ever be observed to
// redden it (plan/journal/gate-excludes-part-of-its-population.md, 2026-09-01).
// They skip when the namespace, the veth or the packet socket is refused.

package gtsm

import (
	"bufio"
	"encoding/binary"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/internal/core/rtproto"

	// The nft backend registers itself, which is what lets firewall.ApplyAll
	// reach the kernel from this test binary. The receive proofs assert what
	// the KERNEL does with the rules ze publishes, so the real backend is the
	// point rather than an implementation detail.
	_ "github.com/ze-software/ze/internal/plugins/firewall/nft"
)

// The ports the proofs use. bgpPort is the port a GTSM peer's session runs on,
// which is what the filter terms associate an ICMP error with. otherPort is a
// port of that same peer that no GTSM session claims.
const (
	bgpPort   = 179
	otherPort = 22
	localPort = 50179
	closedUDP = 40001
)

// ---------------------------------------------------------------------------
// Transmit: the ICMP errors ze sends a GTSM peer
// ---------------------------------------------------------------------------

// TestGTSMTransmittedICMPErrorCarriesTTL255 installs the kernel state ze
// installs for a GTSM peer, makes the kernel generate an ICMPv4 error toward
// that peer, and reads the TTL off the packet as it leaves.
//
// RFC requirement: RFC5082-3-2 positive -- with the hop-limit host route ze
// installs for a GTSM peer, an ICMPv4 error the kernel generates toward that
// peer leaves with TTL exactly 255, which is what RFC 5082 Section 3 requires
// of the related ICMP error handling messages of a GTSM-enabled session.
func TestGTSMTransmittedICMPErrorCarriesTTL255(t *testing.T) {
	bed := newTestbed(t)
	installGTSMRoute(t, peerV4)

	bed.triggerICMPv4Error()
	packet := bed.awaitICMPv4(icmpv4DestinationUnreachable)

	if packet[8] != 255 {
		t.Fatalf("the ICMP error left with TTL %d, want exactly 255", packet[8])
	}
}

// TestGTSMTransmittedICMPErrorWithoutTheRouteMetricIsNot255 is the
// discrimination for the test above. The route is installed and then
// withdrawn, so the two tests differ in one piece of kernel state, and the 255
// there is bound to the metric ze installs rather than to a system default
// that happens to be 255.
//
// RFC requirement: RFC5082-3-2 negative -- once the hop-limit route is
// withdrawn, an ICMPv4 error the kernel generates toward the same peer leaves
// with the system default TTL instead of 255.
func TestGTSMTransmittedICMPErrorWithoutTheRouteMetricIsNot255(t *testing.T) {
	bed := newTestbed(t)
	installGTSMRoute(t, peerV4)
	withdrawGTSMRoute(t, peerV4)

	bed.triggerICMPv4Error()
	packet := bed.awaitICMPv4(icmpv4DestinationUnreachable)

	want := systemDefaultTTL(t)
	if packet[8] != want {
		t.Fatalf("the ICMP error left with TTL %d, want the system default %d", packet[8], want)
	}
	if packet[8] == 255 {
		t.Fatal("the TTL is 255 with no hop-limit route installed, so the positive test proves nothing")
	}
}

// TestGTSMTransmittedICMPv6ErrorCarriesHopLimit255 is the IPv6 half of the
// transmit cell. The metric is the same one: ip6_dst_hoplimit reads
// RTAX_HOPLIMIT off the route exactly as ip_select_ttl does for IPv4.
//
// RFC requirement: RFC5082-3-2 positive -- with the hop-limit host route ze
// installs for a GTSM peer, an ICMPv6 error the kernel generates toward that
// peer leaves with hop limit exactly 255.
func TestGTSMTransmittedICMPv6ErrorCarriesHopLimit255(t *testing.T) {
	bed := newTestbed(t)
	installGTSMRoute(t, peerV6)

	bed.triggerICMPv6Error()
	packet := bed.awaitICMPv6(icmpv6DestinationUnreachable)

	if packet[7] != 255 {
		t.Fatalf("the ICMPv6 error left with hop limit %d, want exactly 255", packet[7])
	}
}

// TestGTSMTransmittedICMPv6ErrorWithoutTheRouteMetricIsNot255 is the
// discrimination for the IPv6 transmit proof, by the same withdraw.
//
// RFC requirement: RFC5082-3-2 negative -- once the hop-limit route is
// withdrawn, an ICMPv6 error the kernel generates toward the same peer leaves
// with the interface's default hop limit instead of 255.
func TestGTSMTransmittedICMPv6ErrorWithoutTheRouteMetricIsNot255(t *testing.T) {
	bed := newTestbed(t)
	installGTSMRoute(t, peerV6)
	withdrawGTSMRoute(t, peerV6)

	bed.triggerICMPv6Error()
	packet := bed.awaitICMPv6(icmpv6DestinationUnreachable)

	if packet[7] == 255 {
		t.Fatal("the hop limit is 255 with no hop-limit route installed, so the positive test proves nothing")
	}
}

// ---------------------------------------------------------------------------
// Receive, IPv4: the ICMP errors ze receives about a GTSM peer's session
// ---------------------------------------------------------------------------

// otherHostV4 is a host that is neither ze nor the peer. The drop proofs use
// it as the source of a forged error. The delivery proof uses it as the
// destination of a quoted packet no GTSM session claims. It sits on the
// testbed's own subnet, so no route and no reverse-path filter stands between
// the injected frame and ze's chain. A message the chain never saw is then a
// failure the proof reports, and never one it mistakes for a drop.
var otherHostV4 = netip.MustParseAddr("192.0.2.3")

// quotedICMPv4Case is one injected ICMPv4 error: the address it comes from and
// the datagram it quotes. The three receive proofs below each run every case
// in a namespace of its own, so the counters one case reads are its own.
type quotedICMPv4Case struct {
	name   string
	source netip.Addr
	quoted []byte
}

// sessionQuote is the datagram an ICMPv4 error about the peer's BGP session
// quotes. It is ze's own IPv4 header toward the peer, then the TCP ports of
// that session.
func sessionQuote() []byte {
	return quotedTCPv4Datagram(zeV4, peerV4, localPort, bgpPort)
}

// TestGTSMDropsADangerousQuotedICMPError publishes the filter ze publishes for
// a GTSM peer and injects the message RFC 5082 classifies as Dangerous. That
// message is an ICMPv4 error quoting a TCP header of the peer's BGP session,
// arriving below the session's TTL floor. It comes once from the peer's
// address and once from a third host. An ICMP error is generated by whichever
// router met the problem. The kernel ties it to the session by the header it
// quotes, never by its source (tcp_v4_err, net/ipv4/tcp_ipv4.c).
//
// The observable is the kernel's own count of the ICMPv4 messages it received
// of that type. The input hook runs before icmp_rcv, so a message the rule
// drops is never counted there, and a message the rule passes always is. The
// second assertion names WHICH rule stopped it, by the term after it never
// being reached. The third reads TCPMinTTLDrop, the counter tcp_v4_err
// increments when it judges the error itself: a message dropped before the
// TCP stack leaves it untouched.
//
// RFC requirement: RFC5082-3-2 positive -- an ICMPv4 error quoting a TCP
// header of a GTSM peer's BGP session, arriving with an outer TTL below the
// session's floor, is dropped by the nftables rule ze installs and never
// reaches the kernel's ICMP receive path, whether its source is the peer's
// address or another host's, which is the verification RFC 5082 Section 6.1
// mandates for related messages.
func TestGTSMDropsADangerousQuotedICMPError(t *testing.T) {
	cases := []quotedICMPv4Case{
		{name: "from the peer", source: peerV4, quoted: sessionQuote()},
		{name: "from another host", source: otherHostV4, quoted: sessionQuote()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bed := newTestbed(t)
			publishGTSMFilter(t)

			beforeICMP := icmpInType(t, icmpv4DestinationUnreachable)
			beforeMinTTL := tcpMinTTLDrop(t)
			bed.injectICMPv4Error(tc.source, 1, tc.quoted)
			awaitChainEvaluated(t)

			if after := icmpInType(t, icmpv4DestinationUnreachable); after != beforeICMP {
				t.Fatalf("the kernel received %d ICMP type 3 messages, want the %d it had: the Dangerous message was delivered", after, beforeICMP)
			}
			wantTerm := termName(peerV4, icmpv4DestinationUnreachable, firewall.QuotedPortDestination)
			if got := lastTermReached(t); got != wantTerm {
				t.Fatalf("the chain stopped at term %q, want %q: another rule claimed the message", got, wantTerm)
			}
			if after := tcpMinTTLDrop(t); after != beforeMinTTL {
				t.Fatalf("TCPMinTTLDrop is %d, want the %d it was: the message reached the TCP stack", after, beforeMinTTL)
			}
		})
	}
}

// TestGTSMDeliversAQuotedICMPErrorAtTTL255 is the discrimination for the test
// above. It injects the same messages, from the same two addresses, quoting
// the same session, and only the outer TTL differs. A rule that dropped one of
// these would refuse every related message, not only the Dangerous ones.
//
// RFC requirement: RFC5082-3-2 negative -- the same ICMPv4 error arriving with
// an outer TTL of 255, from the peer's address or from another host, is
// matched by no drop rule and reaches the kernel's ICMP receive path, so a
// related message carrying the TTL RFC 5082 Section 3 mandates is delivered.
func TestGTSMDeliversAQuotedICMPErrorAtTTL255(t *testing.T) {
	cases := []quotedICMPv4Case{
		{name: "from the peer", source: peerV4, quoted: sessionQuote()},
		{name: "from another host", source: otherHostV4, quoted: sessionQuote()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bed := newTestbed(t)
			publishGTSMFilter(t)

			before := icmpInType(t, icmpv4DestinationUnreachable)
			bed.injectICMPv4Error(tc.source, 255, tc.quoted)
			after := awaitICMPInType(t, icmpv4DestinationUnreachable, before+1)

			if after != before+1 {
				t.Fatalf("the kernel received %d ICMP type 3 messages, want one more than %d: a Trusted message was dropped", after, before)
			}
			assertWholeChainEvaluated(t)
		})
	}
}

// quotedOptionsSpellingTheBGPPort is four octets of IP options for a quoted
// header. They are chosen so the octets at the port offsets of a 20-octet
// quoted header read as the BGP port on both sides. A lowering that read
// those offsets before it checked the version and header length byte would
// claim this message. The 0x45 guard is what keeps it Unknown. As options
// they are an end-of-option-list octet and padding, which the kernel never
// reads out of a quoted header.
var quotedOptionsSpellingTheBGPPort = []byte{0x00, bgpPort, 0x00, bgpPort}

// TestGTSMDeliversAnICMPErrorNoSessionClaims holds the line RFC 5082 Section 3
// draws around the drop: "MUST NOT drop (as part of GTSM processing) packets
// classified as Trusted or Unknown". An ICMP error that quotes another port of
// the peer, a packet toward another host, or a header the filter cannot read
// at its fixed offsets belongs to no GTSM session. So it is Unknown, and a low
// TTL from any source is not enough to drop it.
//
// It is the test that keeps RFC5082-3-4, proven elsewhere for the main packet
// path, true of this filter as well.
//
// RFC requirement: RFC5082-3-4 positive -- with the filter published for a
// GTSM peer, an ICMPv4 error arriving from another host with an outer TTL
// below the floor is delivered to the kernel's ICMP receive path, and every
// rule of the chain is evaluated without matching, when it quotes the peer on
// a port other than the BGP port, when it quotes the BGP port toward another
// destination, or when its quoted header carries IP options, so GTSM
// processing drops no message classified Unknown.
func TestGTSMDeliversAnICMPErrorNoSessionClaims(t *testing.T) {
	cases := []quotedICMPv4Case{
		{
			name:   "another port of the peer",
			source: otherHostV4,
			quoted: quotedTCPv4Datagram(zeV4, peerV4, localPort, otherPort),
		},
		{
			name:   "the BGP port of another host",
			source: otherHostV4,
			quoted: quotedTCPv4Datagram(zeV4, otherHostV4, localPort, bgpPort),
		},
		{
			name:   "a quoted header with options",
			source: otherHostV4,
			quoted: quotedTCPv4DatagramWithOptions(zeV4, peerV4, quotedOptionsSpellingTheBGPPort, localPort, bgpPort),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bed := newTestbed(t)
			publishGTSMFilter(t)

			before := icmpInType(t, icmpv4DestinationUnreachable)
			bed.injectICMPv4Error(tc.source, 1, tc.quoted)
			after := awaitICMPInType(t, icmpv4DestinationUnreachable, before+1)

			if after != before+1 {
				t.Fatalf("the kernel received %d ICMP type 3 messages, want one more than %d: an Unknown message was dropped", after, before)
			}
			assertWholeChainEvaluated(t)
		})
	}
}

// ---------------------------------------------------------------------------
// Receive, IPv6: the socket option is the whole mechanism
// ---------------------------------------------------------------------------

// TestGTSMMinHopCountDropsALowHopLimitICMPv6Error drives the IPv6 receive cell,
// which the kernel answers on state ze already installs: tcp_v6_err compares
// the ICMPv6 error's OWN hop limit against the socket's min_hopcount, which is
// the IPV6_MINHOPCOUNT that network.SetIPMinTTL sets.
//
// The observable is the testbed namespace's TCPMinTTLDrop, the counter the kernel
// increments on exactly that comparison.
//
// RFC requirement: RFC5082-3-2 positive -- on a socket carrying the
// IPV6_MINHOPCOUNT ze installs for a GTSM peer, an ICMPv6 error whose own hop
// limit is below the floor is discarded by the kernel and counted in
// TCPMinTTLDrop, so it never reaches the session it claims to be about.
func TestGTSMMinHopCountDropsALowHopLimitICMPv6Error(t *testing.T) {
	bed := newTestbed(t)
	fd := bed.connectWithMinHopCount(t, 255)

	before := tcpMinTTLDrop(t)
	bed.injectQuotedICMPv6Error(t, 1)
	after := awaitMinTTLDrop(t, before+1)

	if after != before+1 {
		t.Fatalf("TCPMinTTLDrop went from %d to %d, want one more: the error was not refused on its own hop limit", before, after)
	}
	if err := socketError(t, fd); err != 0 {
		t.Fatalf("the connection took error %v from a message the kernel should have discarded", unix.Errno(err))
	}
}

// TestGTSMWithoutMinHopCountDeliversTheSameICMPv6Error is the discrimination
// for the test above. The socket carries the same floor and the message is the
// same message; only its hop limit differs. So the drop there is bound to the
// comparison ze's socket option asks for, and not to the message being
// malformed, unroutable, or ignored for some other reason.
//
// RFC requirement: RFC5082-3-2 negative -- the same ICMPv6 error arriving with
// its own hop limit at 255 passes the IPV6_MINHOPCOUNT comparison, is not
// counted in TCPMinTTLDrop, and reaches the session, which reports the error.
func TestGTSMWithoutMinHopCountDeliversTheSameICMPv6Error(t *testing.T) {
	bed := newTestbed(t)
	fd := bed.connectWithMinHopCount(t, 255)

	before := tcpMinTTLDrop(t)
	bed.injectQuotedICMPv6Error(t, 255)
	err := awaitSocketError(t, fd)

	if after := tcpMinTTLDrop(t); after != before {
		t.Fatalf("TCPMinTTLDrop went from %d to %d, want no change: a message at hop limit 255 is not below the floor", before, after)
	}
	if err != int(unix.ECONNREFUSED) {
		t.Fatalf("the connection took error %v, want ECONNREFUSED: the error was not delivered", unix.Errno(err))
	}
}

// ---------------------------------------------------------------------------
// The kernel state, installed and read back
// ---------------------------------------------------------------------------

// installGTSMRoute installs the peer's hop-limit route through the product
// path and reads the metric back off the kernel. The read-back is the
// assertion that matters: applyHopLimitRoutes reports a failure and continues,
// so a test that only called it would pass with no route in the kernel.
func installGTSMRoute(t *testing.T, addr netip.Addr) {
	t.Helper()

	peer := Peer{Addr: addr, Port: bgpPort, HopLimit: 255, Floor: 255}
	if err := applyHopLimitRoutes([]Peer{peer}, nil); err != nil {
		t.Fatalf("applyHopLimitRoutes: %v", err)
	}
	if got := routeHopLimit(t, addr); got != 255 {
		t.Fatalf("the kernel route to %s carries hop limit %d, want 255", addr, got)
	}
}

// withdrawGTSMRoute removes the peer's route through the product path and
// confirms the kernel no longer holds it.
func withdrawGTSMRoute(t *testing.T, addr netip.Addr) {
	t.Helper()

	peer := Peer{Addr: addr, Port: bgpPort, HopLimit: 255, Floor: 255}
	if err := applyHopLimitRoutes(nil, []Peer{peer}); err != nil {
		t.Fatalf("applyHopLimitRoutes withdraw: %v", err)
	}
	if got := routeHopLimit(t, addr); got != 0 {
		t.Fatalf("the kernel still holds a route to %s with hop limit %d", addr, got)
	}
}

// routeHopLimit answers with the RTAX_HOPLIMIT metric of ze's own host route
// to the address, or 0 when ze holds no such route.
func routeHopLimit(t *testing.T, addr netip.Addr) int {
	t.Helper()

	family := unix.AF_INET
	if addr.Is6() {
		family = unix.AF_INET6
	}
	routes, err := netlink.RouteList(nil, family)
	if err != nil {
		t.Fatalf("route list: %v", err)
	}
	for i := range routes {
		if routes[i].Protocol != netlink.RouteProtocol(rtproto.GTSM) {
			continue
		}
		if routes[i].Dst == nil || !routes[i].Dst.IP.Equal(net.IP(addr.AsSlice())) {
			continue
		}
		return routes[i].Hoplimit
	}
	return 0
}

// publishGTSMFilter publishes the filter for one GTSM peer through the entry
// point the BGP reactor calls, so the rules under test are the rules a
// configured peer produces.
func publishGTSMFilter(t *testing.T) {
	t.Helper()

	peer := Peer{Addr: peerV4, Port: bgpPort, HopLimit: 255, Floor: 255}
	if err := SetPeers([]Peer{peer}); err != nil {
		t.Fatalf("SetPeers: %v", err)
	}
	t.Cleanup(func() {
		// The namespace goes away with the test, so the kernel needs no
		// withdraw. The package's own reconcile state does, or the next test
		// in this binary would read its peer set as already applied.
		current = nil
		if err := firewall.RegisterTables(filterOwner, nil); err != nil {
			t.Errorf("withdraw the table registration: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// The packets each proof needs
// ---------------------------------------------------------------------------

// triggerICMPv4Error sends ze a UDP datagram addressed to a port nothing
// listens on, which is the simplest way to make the kernel generate an ICMP
// error of its own toward the peer.
func (b *testbed) triggerICMPv4Error() {
	b.t.Helper()

	udp := udpDatagram(40000, closedUDP, peerV4, zeV4)
	packet := ipv4Packet(peerV4, zeV4, 64, unix.IPPROTO_UDP, udp)
	b.inject(ethernetFrame(b.zeMAC, peerMAC, ethertypeIPv4, packet))
}

// triggerICMPv6Error is the same trigger in the other family.
func (b *testbed) triggerICMPv6Error() {
	b.t.Helper()

	udp := udpDatagram(40000, closedUDP, peerV6, zeV6)
	packet := ipv6Packet(peerV6, zeV6, 64, unix.IPPROTO_UDP, udp)
	b.inject(ethernetFrame(b.zeMAC, peerMAC, ethertypeIPv6, packet))
}

// injectICMPv4Error sends ze an ICMPv4 port-unreachable from source, carrying
// the outer TTL the caller chooses and quoting the datagram the caller built.
// The frame leaves the peer's end of the veth whoever the source is. That is
// the on-link position every forger of a related message holds toward ze.
func (b *testbed) injectICMPv4Error(source netip.Addr, outerTTL uint8, quoted []byte) {
	b.t.Helper()

	message := icmpv4Message(icmpv4DestinationUnreachable, icmpv4PortUnreachable, quoted)
	packet := ipv4Packet(source, zeV4, outerTTL, unix.IPPROTO_ICMP, message)
	b.inject(ethernetFrame(b.zeMAC, peerMAC, ethertypeIPv4, packet))
}

// connectWithMinHopCount opens a TCP connection toward the peer's BGP port
// with the IPV6_MINHOPCOUNT floor ze installs for a GTSM peer, and returns the
// socket. The peer answers nothing, so the connection stays in progress and
// the only thing that can end it is the ICMPv6 error the test injects.
//
// The socket is built by hand rather than dialed, because a dial on another
// goroutine could run on a thread this test never moved into the namespace.
func (b *testbed) connectWithMinHopCount(t *testing.T, floor uint8) int {
	t.Helper()

	fd, err := unix.Socket(unix.AF_INET6, unix.SOCK_STREAM|unix.SOCK_NONBLOCK, 0)
	if err != nil {
		t.Fatalf("socket: %v", err)
	}
	t.Cleanup(func() { unix.Close(fd) }) //nolint:errcheck // best-effort cleanup

	if err := network.SetIPMinTTL(fd, net.IP(peerV6.AsSlice()), floor); err != nil {
		t.Fatalf("SetIPMinTTL: %v", err)
	}

	sa := &unix.SockaddrInet6{Port: bgpPort}
	copy(sa.Addr[:], peerV6.AsSlice())
	if err := unix.Connect(fd, sa); err != nil && err != unix.EINPROGRESS {
		t.Fatalf("connect: %v", err)
	}
	return fd
}

// injectQuotedICMPv6Error sends ze an ICMPv6 port-unreachable from the peer's
// address, carrying the hop limit the caller chooses and quoting the SYN the
// socket just sent, which is what ties the message to that session.
func (b *testbed) injectQuotedICMPv6Error(t *testing.T, hopLimit uint8) {
	t.Helper()

	sourcePort, sequence := b.awaitOutgoingSYN(t)

	quoted := quotedTCPv6Datagram(zeV6, peerV6, sourcePort, bgpPort, sequence)
	message := icmpv6Message(icmpv6DestinationUnreachable, icmpv6PortUnreachable, peerV6, zeV6, quoted)
	packet := ipv6Packet(peerV6, zeV6, hopLimit, unix.IPPROTO_ICMPV6, message)
	b.inject(ethernetFrame(b.zeMAC, peerMAC, ethertypeIPv6, packet))
}

// awaitOutgoingSYN reads the connection's own SYN off the wire, which is where
// the source port and the sequence number the quoted header has to carry come
// from.
func (b *testbed) awaitOutgoingSYN(t *testing.T) (uint16, uint32) {
	t.Helper()

	packet := b.awaitPacket("TCP SYN to the peer", func(packet []byte) bool {
		if len(packet) < ipv6HeaderLen+20 || packet[0]>>4 != 6 || packet[6] != unix.IPPROTO_TCP {
			return false
		}
		return net.IP(packet[24:40]).Equal(net.IP(peerV6.AsSlice()))
	})
	head := packet[ipv6HeaderLen:]
	return binary.BigEndian.Uint16(head[0:2]), binary.BigEndian.Uint32(head[4:8])
}

// ---------------------------------------------------------------------------
// The observables
// ---------------------------------------------------------------------------

// awaitChainEvaluated waits until ze's chain has seen the injected message.
// The counter ze puts on the FIRST rule counts every packet that REACHES that
// rule, so its first packet is the moment the chain ran.
func awaitChainEvaluated(t *testing.T) {
	t.Helper()

	deadline := time.Now().Add(captureWait)
	for time.Now().Before(deadline) {
		if lastTermReached(t) != "" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("ze's GTSM chain never saw the injected message")
}

// assertWholeChainEvaluated fails unless every rule of ze's chain was reached,
// which is what a message no rule matched looks like: nothing stopped it.
func assertWholeChainEvaluated(t *testing.T) {
	t.Helper()

	awaitChainEvaluated(t)
	for _, rule := range gtsmRules(t) {
		if rule.packets != 0 {
			continue
		}
		t.Fatalf("term %q was never reached, so an earlier rule matched a message GTSM may not drop", rule.term)
	}
}

// lastTermReached names the last rule of ze's GTSM chain the message reached,
// which is the rule that matched it: a rule's verdict ends the evaluation, so
// every rule after the matching one keeps a counter of zero. It answers with
// the empty string when the chain has seen nothing at all.
//
// The counter ze programs sits at the FRONT of the rule (applyChain,
// internal/plugins/firewall/nft/backend_linux.go), so it counts the packets a
// rule was REACHED by rather than the packets it matched. That is what makes
// the last non-zero rule the answer, and it is why no test here reads a single
// rule's counter as a match count
// (plan/journal/counter-counts-the-wrong-packets.md).
func lastTermReached(t *testing.T) string {
	t.Helper()

	last := ""
	for _, rule := range gtsmRules(t) {
		if rule.packets == 0 {
			break
		}
		last = rule.term
	}
	return last
}

// gtsmRule is one rule of ze's GTSM chain: the term it was built from, and the
// number of packets that reached it.
type gtsmRule struct {
	term    string
	packets uint64
}

// gtsmRules reads ze's GTSM chain back out of the kernel, in evaluation order.
func gtsmRules(t *testing.T) []gtsmRule {
	t.Helper()

	conn, err := nftables.New()
	if err != nil {
		t.Fatalf("nftables connection: %v", err)
	}

	tables, err := conn.ListTables()
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}

	var out []gtsmRule
	for _, table := range tables {
		if table.Name != filterTableName {
			continue
		}
		chains, err := conn.ListChainsOfTableFamily(table.Family)
		if err != nil {
			t.Fatalf("list chains: %v", err)
		}
		for _, chain := range chains {
			if chain.Table.Name != filterTableName {
				continue
			}
			rules, err := conn.GetRules(table, chain)
			if err != nil {
				t.Fatalf("get rules: %v", err)
			}
			for _, rule := range rules {
				out = append(out, gtsmRule{term: string(rule.UserData), packets: rulePackets(rule)})
			}
		}
	}
	if len(out) == 0 {
		t.Fatal("ze published no GTSM rules to the kernel, so nothing here can be observed")
	}
	return out
}

// rulePackets is the packet count of a rule's counter expression.
func rulePackets(rule *nftables.Rule) uint64 {
	for _, e := range rule.Exprs {
		counter, ok := e.(*expr.Counter)
		if !ok {
			continue
		}
		return counter.Packets
	}
	return 0
}

// awaitICMPInType waits for the kernel's count of received ICMPv4 messages of
// one type to reach want.
func awaitICMPInType(t *testing.T, messageType uint8, want uint64) uint64 {
	t.Helper()

	var got uint64
	deadline := time.Now().Add(captureWait)
	for time.Now().Before(deadline) {
		got = icmpInType(t, messageType)
		if got >= want {
			return got
		}
		time.Sleep(50 * time.Millisecond)
	}
	return got
}

// icmpInType is the kernel's count of the ICMPv4 messages of one type it has
// RECEIVED, from the IcmpMsg line of the testbed namespace's snmp file. The
// input hook runs before icmp_rcv, so a message an nftables rule drops is never
// counted here.
func icmpInType(t *testing.T, messageType uint8) uint64 {
	t.Helper()

	column := "InType" + strconv.Itoa(int(messageType))
	value, found := procNetColumn(t, "/proc/thread-self/net/snmp", "IcmpMsg:", column)
	if !found {
		// No message of this type has been received since boot, so the kernel
		// has not created the column yet. That is a zero, not a missing
		// counter: the namespace is seconds old.
		return 0
	}
	return value
}

// awaitMinTTLDrop waits for the kernel's TCPMinTTLDrop counter to reach want.
func awaitMinTTLDrop(t *testing.T, want uint64) uint64 {
	t.Helper()

	var got uint64
	deadline := time.Now().Add(captureWait)
	for time.Now().Before(deadline) {
		got = tcpMinTTLDrop(t)
		if got >= want {
			return got
		}
		time.Sleep(50 * time.Millisecond)
	}
	return got
}

// tcpMinTTLDrop reads the counter the kernel increments when it refuses a
// packet, or an ICMP error, on the socket's minimum TTL or hop limit.
func tcpMinTTLDrop(t *testing.T) uint64 {
	t.Helper()

	value, found := procNetColumn(t, "/proc/thread-self/net/netstat", "TcpExt:", "TCPMinTTLDrop")
	if !found {
		t.Fatal("this kernel reports no TCPMinTTLDrop counter, so the receive check cannot be observed")
	}
	return value
}

// procNetColumn reads one named column of one named section of a /proc
// network statistics file. Those files carry pairs of lines: a header naming
// the columns, then the values, both opening with the section name.
//
// The path MUST be under /proc/thread-self/net. /proc/net is /proc/self/net,
// and /proc/self is the thread-group leader, which stays in the host network
// namespace: newTestbed unshares the namespace on the locked test thread only,
// so a counter read through /proc/net counts the host's packets and never the
// testbed's. /proc/thread-self resolves to the calling thread, which is the
// one that unshared.
func procNetColumn(t *testing.T, path, section, column string) (uint64, bool) {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close() //nolint:errcheck // read-only

	scanner := bufio.NewScanner(file)
	var names []string
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 || fields[0] != section {
			continue
		}
		if names == nil {
			names = fields
			continue
		}
		for i, name := range names {
			if name != column || i >= len(fields) {
				continue
			}
			value, err := strconv.ParseUint(fields[i], 10, 64)
			if err != nil {
				t.Fatalf("parse %s %s %q: %v", section, column, fields[i], err)
			}
			return value, true
		}
		names = nil
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return 0, false
}

// awaitSocketError waits for a pending connection to take an error, and
// answers with it.
func awaitSocketError(t *testing.T, fd int) int {
	t.Helper()

	deadline := time.Now().Add(captureWait)
	for time.Now().Before(deadline) {
		if err := socketError(t, fd); err != 0 {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	return 0
}

// socketError reads the pending error of a socket without consuming it twice.
func socketError(t *testing.T, fd int) int {
	t.Helper()

	value, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_ERROR)
	if err != nil {
		t.Fatalf("read the socket error: %v", err)
	}
	return value
}

// systemDefaultTTL is the TTL the kernel gives a locally generated packet when
// no route says otherwise, which is what the transmit negative asserts against.
func systemDefaultTTL(t *testing.T) uint8 {
	t.Helper()

	raw, err := os.ReadFile("/proc/sys/net/ipv4/ip_default_ttl")
	if err != nil {
		t.Fatalf("read ip_default_ttl: %v", err)
	}
	value, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 8)
	if err != nil {
		t.Fatalf("parse ip_default_ttl %q: %v", raw, err)
	}
	return uint8(value)
}
