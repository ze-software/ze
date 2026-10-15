// Design: docs/architecture/bfd.md -- "One session per neighbor": the zone reconciled on receive and on send
// RFC: rfc/short/rfc5881.md
// Related: bfd_fixture_link_local_linux.go -- the veth links, the AF_PACKET sockets and the scenario
// Related: bfd_fixture_pktinfo.go -- the first-packet observer shape this follows
//
// bfd_fixture_link_local.go holds the platform-free half of the link-local
// kernel proof: the frame the fixture injects, the reading of a frame it
// captures, and the reading of `show bfd sessions`. The sockets that move those
// frames are AF_PACKET, which only Linux has, so they live in the _linux file;
// this half carries its own unit test, which runs on every host.

package fixture

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// The addresses test/bfd/bfd-first-packet-link-local.ci configures. Ze's end of
// each link holds bfdLinkLocalLocal; the far end holds NO address, so a packet
// Ze sends to bfdLinkLocalPeer can only leave on the wire, never through lo, and
// a packet the fixture injects from bfdLinkLocalPeer is ordinary ingress on Ze's
// link. The same peer address on two links is the case the zone exists for.
const (
	bfdLinkLocalPeer  = "fe80::b"
	bfdLinkLocalLocal = "fe80::a"

	// bfdLinkLocalSourcePort is the injected packets' UDP source port, which
	// RFC 5881 Section 4 bounds: "The source port MUST be in the range 49152
	// through 65535." It is fixed, so a capture can tell the two directions apart.
	bfdLinkLocalSourcePort = 49252

	// bfdLinkLocalHopLimit is the hop limit both directions carry.
	// RFC 5881 Section 5: "If BFD authentication is not in use on a session,
	// all BFD Control packets for the session MUST be sent with a Time to
	// Live (TTL) or Hop Limit value of 255." Ze's receive gate drops any other.
	bfdLinkLocalHopLimit = 255

	bfdLinkLocalEthernetOctets = 14
	bfdLinkLocalIPv6Octets     = 40
	bfdLinkLocalUDPOctets      = 8
	bfdLinkLocalEtherTypeIPv6  = 0x86dd
	bfdLinkLocalNextHeaderUDP  = 17

	// bfdLinkLocalFrameOctets is the whole injected frame: no IPv6 extension
	// header and no BFD authentication section.
	bfdLinkLocalFrameOctets = bfdLinkLocalEthernetOctets + bfdLinkLocalIPv6Octets +
		bfdLinkLocalUDPOctets + packet.MandatoryLen
)

// bfdLinkLocalLink is one of the two links: the interface Ze's session names,
// the far end the fixture injects from and captures on, the two fixed MACs the
// setup gives them, and the My Discriminator the fixture's packets on this link
// carry. The two discriminators differ, so a frame names the link it belongs to.
type bfdLinkLocalLink struct {
	near          string
	far           string
	nearMAC       net.HardwareAddr
	farMAC        net.HardwareAddr
	discriminator uint32
}

// bfdLinkLocalLinks returns the two links the .ci builds. A function rather
// than a package variable, so no caller can change the MACs another caller
// reads.
func bfdLinkLocalLinks() [2]bfdLinkLocalLink {
	return [2]bfdLinkLocalLink{
		{
			near:          "zbl0",
			far:           "zbl0p",
			nearMAC:       net.HardwareAddr{0x02, 0x5a, 0x62, 0x6c, 0x00, 0x01},
			farMAC:        net.HardwareAddr{0x02, 0x5a, 0x62, 0x6c, 0x00, 0x02},
			discriminator: 0x6c6c0001,
		},
		{
			near:          "zbl1",
			far:           "zbl1p",
			nearMAC:       net.HardwareAddr{0x02, 0x5a, 0x62, 0x6c, 0x01, 0x01},
			farMAC:        net.HardwareAddr{0x02, 0x5a, 0x62, 0x6c, 0x01, 0x02},
			discriminator: 0x6c6c0002,
		},
	}
}

// bfdLinkLocalFrame writes into frame one Ethernet, IPv6, UDP and BFD Control
// frame from the link's far end to Ze's end, as a neighbor whose session is
// still Down sends it: Your Discriminator zero, so Ze can select a session for
// it only by the source address and the link it arrived on. It returns the
// octets written. frame MUST hold bfdLinkLocalFrameOctets.
//
// RFC 5880 Section 6.8.6: "If the Your Discriminator field is zero, the
// session MUST be selected based on some combination of other fields."
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	| 0  Ethernet: target MAC (6), source MAC (6), type 0x86dd (2)  |
//	| 14 IPv6 (RFC 8200 Section 3): version 6, payload length,      |
//	|    next header 17, hop limit 255, source, destination         |
//	| 54 UDP (RFC 768): source port, port 3784, length, checksum    |
//	| 62 BFD Control (RFC 5880 Section 4.1), 24 octets              |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
func bfdLinkLocalFrame(frame []byte, link *bfdLinkLocalLink) (int, error) {
	if len(frame) < bfdLinkLocalFrameOctets {
		return 0, fmt.Errorf("frame buffer holds %d octets, the frame needs %d", len(frame), bfdLinkLocalFrameOctets)
	}
	source := netip.MustParseAddr(bfdLinkLocalPeer)
	target := netip.MustParseAddr(bfdLinkLocalLocal)

	copy(frame[0:6], link.nearMAC)
	copy(frame[6:12], link.farMAC)
	binary.BigEndian.PutUint16(frame[12:14], bfdLinkLocalEtherTypeIPv6)

	ip := frame[bfdLinkLocalEthernetOctets:]
	binary.BigEndian.PutUint32(ip[0:4], 6<<28)
	binary.BigEndian.PutUint16(ip[4:6], bfdLinkLocalUDPOctets+packet.MandatoryLen)
	ip[6] = bfdLinkLocalNextHeaderUDP
	ip[7] = bfdLinkLocalHopLimit
	sourceOctets, targetOctets := source.As16(), target.As16()
	copy(ip[8:24], sourceOctets[:])
	copy(ip[24:40], targetOctets[:])

	udp := ip[bfdLinkLocalIPv6Octets : bfdLinkLocalIPv6Octets+bfdLinkLocalUDPOctets+packet.MandatoryLen]
	binary.BigEndian.PutUint16(udp[0:2], bfdLinkLocalSourcePort)
	binary.BigEndian.PutUint16(udp[2:4], bfdPktinfoSingleHop)
	binary.BigEndian.PutUint16(udp[4:6], uint16(len(udp)))
	binary.BigEndian.PutUint16(udp[6:8], 0)

	control := packet.Control{
		Version:               packet.Version,
		State:                 packet.StateDown,
		DetectMult:            3,
		Length:                packet.MandatoryLen,
		MyDiscriminator:       link.discriminator,
		YourDiscriminator:     0,
		DesiredMinTxInterval:  300_000,
		RequiredMinRxInterval: 300_000,
	}
	control.WriteTo(udp, bfdLinkLocalUDPOctets)
	binary.BigEndian.PutUint16(udp[6:8], bfdLinkLocalUDPChecksum(source, target, udp))
	return bfdLinkLocalFrameOctets, nil
}

// bfdLinkLocalUDPChecksum is the UDP checksum over the IPv6 pseudo-header and
// segment, whose checksum field MUST be zero when it is called.
//
// RFC 8200 Section 8.1: "Unlike IPv4, the default behavior when UDP packets
// are originated by an IPv6 node is that the UDP checksum is not optional."
// The pseudo-header is the source and destination addresses, the upper-layer
// length as 32 bits, three zero octets and the next header.
func bfdLinkLocalUDPChecksum(source, target netip.Addr, segment []byte) uint16 {
	var sum uint32
	sourceOctets, targetOctets := source.As16(), target.As16()
	sum = bfdLinkLocalSumWords(sum, sourceOctets[:])
	sum = bfdLinkLocalSumWords(sum, targetOctets[:])
	sum += uint32(len(segment))
	sum += bfdLinkLocalNextHeaderUDP
	sum = bfdLinkLocalSumWords(sum, segment)
	for sum > 0xffff {
		sum = sum>>16 + sum&0xffff
	}
	checksum := ^uint16(sum)
	// RFC 768: "If the computed checksum is zero, it is transmitted as all
	// ones." Over IPv6 a zero checksum is refused, so the case matters.
	if checksum == 0 {
		return 0xffff
	}
	return checksum
}

// bfdLinkLocalSumWords adds data to sum as big-endian 16-bit words, padding an
// odd final octet with zero.
func bfdLinkLocalSumWords(sum uint32, data []byte) uint32 {
	for i := 0; i+1 < len(data); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(data[i : i+2]))
	}
	if len(data)%2 == 1 {
		sum += uint32(data[len(data)-1]) << 8
	}
	return sum
}

// errBfdLinkLocalNotControl reports a captured frame that is not a BFD
// single-hop Control packet over IPv6: ND, an MLD report, anything else on the
// link. The capture skips it.
var errBfdLinkLocalNotControl = errors.New("not a BFD single-hop Control packet over IPv6")

// bfdLinkLocalSentYourDiscriminator reads one frame captured on a far end and
// returns the Your Discriminator of the BFD Control packet Ze sent in it.
//
// A frame that is not a BFD single-hop Control packet over IPv6 returns
// errBfdLinkLocalNotControl. A frame that is one returns an error when it
// breaks what AC-3 asserts of every frame Ze sends to this peer: the source
// address the session names, the peer as destination, hop limit 255.
func bfdLinkLocalSentYourDiscriminator(frame []byte) (uint32, error) {
	headers := bfdLinkLocalEthernetOctets + bfdLinkLocalIPv6Octets + bfdLinkLocalUDPOctets
	if len(frame) < headers {
		return 0, errBfdLinkLocalNotControl
	}
	if binary.BigEndian.Uint16(frame[12:14]) != bfdLinkLocalEtherTypeIPv6 {
		return 0, errBfdLinkLocalNotControl
	}
	ip := frame[bfdLinkLocalEthernetOctets:]
	if ip[6] != bfdLinkLocalNextHeaderUDP {
		return 0, errBfdLinkLocalNotControl
	}
	udp := ip[bfdLinkLocalIPv6Octets:]
	if binary.BigEndian.Uint16(udp[2:4]) != bfdPktinfoSingleHop {
		return 0, errBfdLinkLocalNotControl
	}

	source := netip.AddrFrom16([16]byte(ip[8:24]))
	target := netip.AddrFrom16([16]byte(ip[24:40]))
	if source != netip.MustParseAddr(bfdLinkLocalLocal) {
		return 0, fmt.Errorf("a BFD frame left with source %s, not the session's local %s", source, bfdLinkLocalLocal)
	}
	if target != netip.MustParseAddr(bfdLinkLocalPeer) {
		return 0, fmt.Errorf("a BFD frame left for %s, not the session's peer %s", target, bfdLinkLocalPeer)
	}
	if ip[7] != bfdLinkLocalHopLimit {
		return 0, fmt.Errorf("a BFD frame left with hop limit %d, not %d", ip[7], bfdLinkLocalHopLimit)
	}

	control, _, err := packet.ParseControl(udp[bfdLinkLocalUDPOctets:])
	if err != nil {
		return 0, fmt.Errorf("a frame to port %d does not parse as BFD Control: %w", bfdPktinfoSingleHop, err)
	}
	return control.YourDiscriminator, nil
}

// bfdLinkLocalRemoteDiscriminator reads, from the decoded answer of `show bfd
// sessions`, the remote-discriminator of the session to bfdLinkLocalPeer on
// the interface named iface. It reports false when no such session exists
// yet, and an error when two do or when the field is not a number.
//
// The sessions are told apart by interface, never by `show bfd session address`,
// which matches on the peer alone and answers with whichever of the two sessions
// it meets first (plan/journal/key-omits-a-fact-the-builder-uses.md, 2026-10-13).
func bfdLinkLocalRemoteDiscriminator(sessions []map[string]any, iface string) (uint32, bool, error) {
	var found map[string]any
	for _, session := range sessions {
		if session["peer"] != bfdLinkLocalPeer {
			continue
		}
		if session["interface"] != iface {
			continue
		}
		if found != nil {
			return 0, false, fmt.Errorf("show bfd sessions: two sessions to %s on %s", bfdLinkLocalPeer, iface)
		}
		found = session
	}
	if found == nil {
		return 0, false, nil
	}
	seen, ok := found["remote-discriminator"].(float64)
	if !ok {
		return 0, false, fmt.Errorf("show bfd sessions: the session to %s on %s has remote-discriminator %#v, not a number",
			bfdLinkLocalPeer, iface, found["remote-discriminator"])
	}
	return uint32(seen), true, nil
}
