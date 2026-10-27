//go:build integration && linux

// Design: rfc/short/rfc5082.md -- GTSM, the listen-port floor before the handshake
// Overview: gtsm.go -- listenPortTerm, the rule these proofs drive
// Related: netns_linux_test.go, packet_linux_test.go -- the namespace and the frames
//
// These proofs drive a real TCP listener in a namespace of its own, so they
// need CAP_NET_ADMIN, which the merge gate host does not hold. They are tagged
// `integration` so `./le rfc record` runs them in the QEMU guest that boots
// Ze's kernel (docs/contributing/rfc-conformance-gates.md, the guest unit),
// where a skip cannot be read as a pass.
//
// A SYN is the main protocol packet RFC 5082 Section 3 classifies. The
// accepted socket's floor (network.SetIPMinTTL) exists only after the
// handshake, so the SYN itself is judged by the rule SetPeers publishes at the
// input hook. Each proof injects SYNs from the peer's side of the veth pair and
// reads what the kernel answers on ze's side.

package gtsm

import (
	"encoding/binary"
	"net"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/firewall"
)

// The TCP fields the proofs write. A SYN carries no options, so the header is
// the fixed 20 octets of RFC 9293 Section 3.1 and the flags sit at octet 13.
const (
	tcpHeaderLen = 20
	tcpFlagSYN   = 0x02
	tcpFlagACK   = 0x10
	// listenerOtherPort is a second port ze listens on, which no GTSM session
	// runs on, so a SYN to it from the GTSM peer is Unknown.
	listenerOtherPort = 1179
	// noAnswerWait is how long a proof waits to be sure the kernel sent no
	// SYN-ACK. It starts after the drop rule's counter already moved, and the
	// kernel answers a SYN it accepts within microseconds on an idle veth.
	noAnswerWait = 500 * time.Millisecond
)

// unconfiguredV4 and unconfiguredV6 are sources no peer uses. otherHostV6 is
// the IPv6 host no GTSM session names.
var (
	unconfiguredV4 = netip.MustParseAddr("192.0.2.4")
	otherHostV6    = netip.MustParseAddr("2001:db8::3")
)

// TestGTSMListenerSYNBelowFloorDropped is AC-1 and AC-3 for IPv4. A GTSM peer
// with floor 255 sends a SYN to ze's listen port at TTL 254: the listen-port
// rule drops it, so the kernel sends no SYN-ACK and no other rule of the chain
// claims it. The same peer then sends a SYN at TTL 255, the kernel answers it,
// and the handshake completes into an accepted connection.
//
// RFC requirement: RFC5082-3-7 positive -- with GTSM enabled for a peer at
// floor 255, a TCP SYN from the peer's address to ze's listen port at TTL 254
// is dropped by the listen-port rule ze installs and receives no SYN-ACK,
// while the same SYN at TTL 255 is answered and the handshake completes.
func TestGTSMListenerSYNBelowFloorDropped(t *testing.T) {
	listenerFloorProof(t, zeV4, peerV4)
}

// TestGTSMListenerSYNBelowFloorDroppedIPv6 is AC-2 and AC-3: the same proof
// for an IPv6 peer, where the floor is read from the Hop Limit.
//
// RFC requirement: RFC5082-3-7 positive -- with GTSM enabled for an IPv6 peer
// at floor 255, a TCP SYN from the peer's address to ze's listen port at hop
// limit 254 is dropped by the listen-port rule ze installs and receives no
// SYN-ACK, while the same SYN at hop limit 255 is answered and the handshake
// completes.
func TestGTSMListenerSYNBelowFloorDroppedIPv6(t *testing.T) {
	listenerFloorProof(t, zeV6, peerV6)
}

// listenerFloorProof runs the drop and the completed handshake for one family.
func listenerFloorProof(t *testing.T, ze, peer netip.Addr) {
	bed := newTestbed(t)
	listener := listenTCP(t, ze, bgpPort)
	publishGTSMPeers(t, Peer{Addr: peer, Port: bgpPort, HopLimit: 255, Floor: 255})
	listenTerm := listenTermName(netip.PrefixFrom(peer, peer.BitLen()))

	bed.injectTCP(peer, ze, 254, tcpSegment(peer, ze, 40001, bgpPort, 1000, 0, tcpFlagSYN))
	awaitTermMatched(t, listenTerm)
	if got := termsMatched(t); !slices.Equal(got, []string{listenTerm}) {
		t.Fatalf("terms %q matched the SYN, want exactly %q", got, listenTerm)
	}
	if _, answered := bed.synACK(ze, peer, 40001, noAnswerWait); answered {
		t.Fatalf("the kernel answered a SYN from %s below the floor: the Dangerous SYN reached the listener", peer)
	}

	bed.injectTCP(peer, ze, 255, tcpSegment(peer, ze, 40002, bgpPort, 2000, 0, tcpFlagSYN))
	segment, answered := bed.synACK(ze, peer, 40002, captureWait)
	if !answered {
		t.Fatalf("the kernel sent no SYN-ACK to a SYN from %s at the floor: a Trusted SYN was dropped", peer)
	}
	serverSeq := binary.BigEndian.Uint32(segment[4:8])
	bed.injectTCP(peer, ze, 255, tcpSegment(peer, ze, 40002, bgpPort, 2001, serverSeq+1, tcpFlagACK))
	awaitAccept(t, listener, peer)

	if got := listenTermPackets(t, listenTerm); got != 1 {
		t.Fatalf("the listen-port rule matched %d packets, want only the one SYN below the floor", got)
	}
}

// TestGTSMListenerSYNAtFloorAccepted is AC-3 for IPv4, the Trusted half of the
// listen-port rule on its own. A GTSM peer at floor 255 sends a SYN at TTL 255,
// exactly the floor: the kernel answers it, the handshake completes into an
// accepted connection, and no rule of the chain counts it. A compare that
// drops at the floor as well as below it turns this unit red.
//
// RFC requirement: RFC5082-3-4 positive -- with GTSM enabled for a peer at
// floor 255, a TCP SYN from the peer's address to ze's listen port at TTL 255
// is answered with a SYN-ACK, the handshake completes into an accepted
// connection, and no rule of the chain matches it, so GTSM processing drops no
// SYN classified Trusted.
func TestGTSMListenerSYNAtFloorAccepted(t *testing.T) {
	listenerAtFloorProof(t, zeV4, peerV4)
}

// TestGTSMListenerSYNAtFloorAcceptedIPv6 is AC-3 for IPv6: the same proof,
// where the floor is read from the Hop Limit.
//
// RFC requirement: RFC5082-3-4 positive -- with GTSM enabled for an IPv6 peer
// at floor 255, a TCP SYN from the peer's address to ze's listen port at hop
// limit 255 is answered with a SYN-ACK, the handshake completes into an
// accepted connection, and no rule of the chain matches it, so GTSM processing
// drops no SYN classified Trusted.
func TestGTSMListenerSYNAtFloorAcceptedIPv6(t *testing.T) {
	listenerAtFloorProof(t, zeV6, peerV6)
}

// listenerAtFloorProof completes a handshake whose SYN arrives at the floor,
// with the listen-port rule published, and reads that no rule matched.
func listenerAtFloorProof(t *testing.T, ze, peer netip.Addr) {
	bed := newTestbed(t)
	listener := listenTCP(t, ze, bgpPort)
	publishGTSMPeers(t, Peer{Addr: peer, Port: bgpPort, HopLimit: 255, Floor: 255})

	bed.injectTCP(peer, ze, 255, tcpSegment(peer, ze, 40004, bgpPort, 4000, 0, tcpFlagSYN))
	segment, answered := bed.synACK(ze, peer, 40004, captureWait)
	if !answered {
		t.Fatalf("the kernel sent no SYN-ACK to a SYN from %s at the floor: a Trusted SYN was dropped", peer)
	}
	serverSeq := binary.BigEndian.Uint32(segment[4:8])
	bed.injectTCP(peer, ze, 255, tcpSegment(peer, ze, 40004, bgpPort, 4001, serverSeq+1, tcpFlagACK))
	awaitAccept(t, listener, peer)
	assertNoTermMatched(t)
}

// TestGTSMListenerUnknownNotDropped is AC-4. With the listen-port rule
// published for one GTSM peer, a SYN at TTL 1 from a configured peer that has
// no GTSM, from an address that is no peer, and from the GTSM peer to a port
// no GTSM session runs on is Unknown, so each is answered by TCP and no rule of
// the chain matches it.
//
// RFC requirement: RFC5082-3-4 positive -- with the listen-port rule published
// for a GTSM peer, a TCP SYN at TTL or hop limit 1 from a host with no GTSM
// session, from an unconfigured address, or from the GTSM peer to a port its
// session does not use, is answered with a SYN-ACK and matched by no rule of
// the chain, so GTSM processing drops no SYN classified Unknown.
func TestGTSMListenerUnknownNotDropped(t *testing.T) {
	cases := []struct {
		name   string
		ze     netip.Addr
		source netip.Addr
		port   uint16
	}{
		{name: "a peer without GTSM", ze: zeV4, source: otherHostV4, port: bgpPort},
		{name: "an unconfigured address", ze: zeV4, source: unconfiguredV4, port: bgpPort},
		{name: "the GTSM peer to another port", ze: zeV4, source: peerV4, port: listenerOtherPort},
		{name: "an IPv6 host without GTSM", ze: zeV6, source: otherHostV6, port: bgpPort},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bed := newTestbed(t)
			bed.pinNeighbor(tc.source)
			listenTCP(t, tc.ze, tc.port)
			publishGTSMPeers(t,
				Peer{Addr: peerV4, Port: bgpPort, HopLimit: 255, Floor: 255},
				Peer{Addr: peerV6, Port: bgpPort, HopLimit: 255, Floor: 255},
			)

			bed.injectTCP(tc.source, tc.ze, 1, tcpSegment(tc.source, tc.ze, 40003, tc.port, 3000, 0, tcpFlagSYN))
			if _, answered := bed.synACK(tc.ze, tc.source, 40003, captureWait); !answered {
				t.Fatalf("the kernel sent no SYN-ACK to an Unknown SYN from %s to port %d", tc.source, tc.port)
			}
			assertNoTermMatched(t)
		})
	}
}

// publishGTSMPeers publishes the filter for the given peers through SetPeers,
// the entry point the BGP reactor calls, and forgets the package's reconcile
// state when the test ends, as publishGTSMFilter does.
func publishGTSMPeers(t *testing.T, peers ...Peer) {
	t.Helper()

	if err := SetPeers(peers, nil); err != nil {
		t.Fatalf("SetPeers: %v", err)
	}
	t.Cleanup(func() {
		current = nil
		currentRanges = nil
		if err := firewall.RegisterTables(filterOwner, nil); err != nil {
			t.Errorf("withdraw the table registration: %v", err)
		}
	})
}

// listenTCP opens a listener inside the test's namespace. The calling
// goroutine is locked to the namespace's thread by newTestbed, and the socket
// belongs to the namespace it was created in.
func listenTCP(t *testing.T, addr netip.Addr, port uint16) *net.TCPListener {
	t.Helper()

	listener, err := net.ListenTCP("tcp", net.TCPAddrFromAddrPort(netip.AddrPortFrom(addr, port)))
	if err != nil {
		t.Fatalf("listen on %s port %d: %v", addr, port, err)
	}
	t.Cleanup(func() { listener.Close() }) //nolint:errcheck // best-effort cleanup
	return listener
}

// awaitAccept requires the listener to hand over the connection the injected
// handshake completed, from the peer's address.
func awaitAccept(t *testing.T, listener *net.TCPListener, peer netip.Addr) {
	t.Helper()

	if err := listener.SetDeadline(time.Now().Add(captureWait)); err != nil {
		t.Fatalf("listener deadline: %v", err)
	}
	conn, err := listener.AcceptTCP()
	if err != nil {
		t.Fatalf("no connection from %s was accepted: %v", peer, err)
	}
	defer conn.Close() //nolint:errcheck // best-effort cleanup

	address, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("accepted a connection whose remote address is a %T, want a *net.TCPAddr", conn.RemoteAddr())
	}
	if remote := address.AddrPort().Addr().Unmap(); remote != peer {
		t.Fatalf("accepted a connection from %s, want %s", remote, peer)
	}
}

// listenTermPackets is the packet count of the named term's rules.
func listenTermPackets(t *testing.T, term string) uint64 {
	t.Helper()

	var packets uint64
	for _, rule := range gtsmRules(t) {
		if rule.term == term {
			packets += rule.packets
		}
	}
	return packets
}

// pinNeighbor gives a further source a permanent neighbor entry on ze's end,
// so the SYN-ACK the kernel sends it leaves without address resolution.
func (b *testbed) pinNeighbor(addr netip.Addr) {
	b.t.Helper()

	link, err := netlink.LinkByIndex(b.zeIndex)
	if err != nil {
		b.t.Fatalf("link by index %d: %v", b.zeIndex, err)
	}
	b.addNeighbor(link, addr)
}

// injectTCP wraps a TCP segment in an IPv4 packet at the given TTL, or an IPv6
// packet at the given hop limit, and injects it toward ze.
func (b *testbed) injectTCP(source, destination netip.Addr, ttl uint8, segment []byte) {
	b.t.Helper()

	if source.Is4() {
		packet := ipv4Packet(source, destination, ttl, unix.IPPROTO_TCP, segment)
		b.inject(ethernetFrame(b.zeMAC, peerMAC, ethertypeIPv4, packet))
		return
	}
	packet := ipv6Packet(source, destination, ttl, unix.IPPROTO_TCP, segment)
	b.inject(ethernetFrame(b.zeMAC, peerMAC, ethertypeIPv6, packet))
}

// synACK reads frames leaving ze until a SYN-ACK from ze to the source's
// port arrives, or wait passes. It answers the TCP segment and true, or false
// when the kernel sent none, which a drop proof needs to observe without
// failing.
func (b *testbed) synACK(ze, source netip.Addr, sourcePort uint16, wait time.Duration) ([]byte, bool) {
	b.t.Helper()

	buf := make([]byte, 2048)
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		n, _, err := unix.Recvfrom(b.captureFD, buf, 0)
		if err != nil {
			continue // the read timeout, or a signal; the deadline is the bound
		}
		if n <= ethernetHeaderLen {
			continue
		}
		segment, ok := tcpFromTo(buf[ethernetHeaderLen:n], ze, source)
		if !ok {
			continue
		}
		if len(segment) < tcpHeaderLen {
			continue
		}
		if binary.BigEndian.Uint16(segment[2:4]) != sourcePort {
			continue
		}
		if segment[13]&(tcpFlagSYN|tcpFlagACK) == tcpFlagSYN|tcpFlagACK {
			return slices.Clone(segment), true
		}
	}
	return nil, false
}

// tcpFromTo answers the TCP segment of a packet sent from one address to
// another, or false for any other packet. Both headers are the fixed lengths
// the kernel writes for a SYN-ACK.
func tcpFromTo(packet []byte, source, destination netip.Addr) ([]byte, bool) {
	if source.Is4() {
		if len(packet) < ipv4HeaderLen {
			return nil, false
		}
		if packet[0] != 0x45 {
			return nil, false
		}
		if packet[9] != unix.IPPROTO_TCP {
			return nil, false
		}
		if netip.AddrFrom4([4]byte(packet[12:16])) != source {
			return nil, false
		}
		if netip.AddrFrom4([4]byte(packet[16:20])) != destination {
			return nil, false
		}
		return packet[ipv4HeaderLen:], true
	}
	if len(packet) < ipv6HeaderLen {
		return nil, false
	}
	if packet[0]>>4 != 6 {
		return nil, false
	}
	if packet[6] != unix.IPPROTO_TCP {
		return nil, false
	}
	if netip.AddrFrom16([16]byte(packet[8:24])) != source {
		return nil, false
	}
	if netip.AddrFrom16([16]byte(packet[24:40])) != destination {
		return nil, false
	}
	return packet[ipv6HeaderLen:], true
}

// tcpSegment builds a TCP header with no options and no payload (RFC 9293
// Section 3.1), with the checksum over the family's pseudo-header:
//
//	byte  0: source port
//	byte  2: destination port
//	byte  4: sequence number
//	byte  8: acknowledgment number
//	byte 12: data offset (5 words)
//	byte 13: flags
//	byte 14: window
//	byte 16: checksum
func tcpSegment(source, destination netip.Addr, sourcePort, destinationPort uint16, seq, ack uint32, flags uint8) []byte {
	segment := make([]byte, tcpHeaderLen)
	binary.BigEndian.PutUint16(segment[0:2], sourcePort)
	binary.BigEndian.PutUint16(segment[2:4], destinationPort)
	binary.BigEndian.PutUint32(segment[4:8], seq)
	binary.BigEndian.PutUint32(segment[8:12], ack)
	segment[12] = (tcpHeaderLen / 4) << 4
	segment[13] = flags
	binary.BigEndian.PutUint16(segment[14:16], 65535)
	if source.Is4() {
		binary.BigEndian.PutUint16(segment[16:18], ipv4PseudoHeaderChecksum(source, destination, segment))
		return segment
	}
	binary.BigEndian.PutUint16(segment[16:18], pseudoHeaderChecksum(source, destination, unix.IPPROTO_TCP, segment))
	return segment
}

// ipv4PseudoHeaderChecksum is the TCP checksum over the IPv4 pseudo-header of
// RFC 9293 Section 3.1: source, destination, a zero octet, the protocol, and
// the TCP length, then the segment.
func ipv4PseudoHeaderChecksum(source, destination netip.Addr, segment []byte) uint16 {
	pseudo := make([]byte, 12+len(segment))
	copy(pseudo[0:4], source.AsSlice())
	copy(pseudo[4:8], destination.AsSlice())
	pseudo[9] = unix.IPPROTO_TCP
	binary.BigEndian.PutUint16(pseudo[10:12], uint16(len(segment)))
	copy(pseudo[12:], segment)
	return internetChecksum(pseudo)
}
