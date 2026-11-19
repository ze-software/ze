// Design: docs/architecture/l2tp/bng-5-pppoe.md -- bounded punted IPv6 controls
// Related: ipv6_carrier.go -- fixed queue ownership; ipv6_metrics.go -- drops
package vpp

import (
	"encoding/binary"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

var (
	ipv6AllNodes    = netip.MustParseAddr("ff02::1")
	ipv6AllRouters  = netip.MustParseAddr("ff02::2")
	ipv6DHCPServers = netip.MustParseAddr("ff02::1:2")
)

// IPv6 wire layout, RFC 8200 Section 3 (offsets exclude PPP's two octets):
//
//	0             4             6       7       8             24            40
//	+-------------+-------------+-------+-------+-------------+-------------+
//	| ver/TC/flow | payload len | next  | hop   | source (16) | target (16) |
//	+-------------+-------------+-------+-------+-------------+-------------+
//
// ICMPv6 RS (RFC 4861 Section 4.1): type/code/checksum/reserved (8 octets).
// UDP (RFC 768): source/target/length/checksum (two octets apiece).
// Extension headers and fragments are never controls emitted by patch 0014;
// accepting them here would create a second, incomplete IPv6 receive stack.
// RFC 8200 Section 3: "4-bit Internet Protocol version number = 6."
func (s *controlSocket) dispatchIPv6(packet []byte) {
	s.carrierMu.RLock()
	defer s.carrierMu.RUnlock()
	c := s.carrier
	// RFC 1661 Section 3.5: "Any supported network-layer protocol packets
	// received when the corresponding NCP is not in the Opened state MUST be
	// silently discarded."
	if c == nil {
		return
	}
	if len(packet) > ppp.MaxFrameLen {
		s.countIPv6Discard("envelope")
		return
	}
	if len(packet) < 40 {
		s.countIPv6Discard("envelope")
		return
	}
	if packet[0]>>4 != 6 {
		s.countIPv6Discard("envelope")
		return
	}
	if int(binary.BigEndian.Uint16(packet[4:6])) != len(packet)-40 {
		s.countIPv6Discard("envelope")
		return
	}
	source := netip.AddrFrom16([16]byte(packet[8:24]))
	target := netip.AddrFrom16([16]byte(packet[24:40]))
	payload := packet[40:]
	// Only direct ICMPv6 and UDP controls belong to this partial dispatcher.
	//exhaustive:ignore
	switch packet[6] {
	case 58:
		// RFC 4443 Section 2.3.
		if ipv6Checksum(source, target, 58, payload) != 0 {
			s.countIPv6Discard("checksum")
			return
		}
		// RFC 4861 Section 6.1.1. The shared validator also checks options.
		if !ppp.ValidRouterSolicitation(payload, source, int(packet[7])) {
			s.countIPv6Discard("solicitation")
			return
		}
		if target != ipv6AllRouters {
			if target != c.local {
				s.countIPv6Discard("destination")
				return
			}
		}
		select {
		case c.rs <- struct{}{}:
		default:
			s.countIPv6Discard("queue-full")
		}
	case 17:
		// RFC 9915 Section 7.2.
		s.dispatchDHCPv6(c, source, target, packet[7], payload)
	default:
		s.countIPv6Discard("transport")
	}
}

// RFC 9915 Section 7.2: "Servers and relay agents MUST listen for DHCP
// messages on UDP port 547."
func (s *controlSocket) dispatchDHCPv6(c *ipv6Carrier, source, target netip.Addr, hopLimit byte, payload []byte) {
	// RFC 9915 Section 5. IPv6CP's IID is not a DHCPv6 source binding.
	if !validIPv6ControlPeer(source) {
		s.countIPv6Discard("source")
		return
	}
	// RFC 9915 Section 16: "Servers SHOULD NOT accept unicast traffic from
	// clients." This provider has never offered the obsolete Server Unicast
	// option, so it carries no legacy unicast lease to preserve.
	if target != ipv6DHCPServers {
		s.countIPv6Discard("destination")
		return
	}
	if hopLimit == 0 {
		s.countIPv6Discard("envelope")
		return
	}
	if len(payload) < 12 {
		s.countIPv6Discard("datagram")
		return
	}
	// RFC 9915 Section 7.2: "However, clients, servers, and relay agents MAY
	// send DHCP messages from any UDP source port they are allowed to use."
	if binary.BigEndian.Uint16(payload[2:4]) != 547 {
		s.countIPv6Discard("datagram")
		return
	}
	if int(binary.BigEndian.Uint16(payload[4:6])) != len(payload) {
		s.countIPv6Discard("datagram")
		return
	}
	// RFC 8200 Section 8.1: "IPv6 receivers must discard UDP packets
	// containing a zero checksum and should log the error."
	if binary.BigEndian.Uint16(payload[6:8]) == 0 {
		s.countIPv6Discard("checksum")
		return
	}
	// RFC 8200 Section 8.1.
	if ipv6Checksum(source, target, 17, payload) != 0 {
		s.countIPv6Discard("checksum")
		return
	}
	if len(payload)-8 > ipv6DHCPPayloadMax {
		s.countIPv6Discard("datagram")
		return
	}
	// RFC 9915 Section 7.2: "Servers MUST send Relay-reply messages to UDP
	// destination port 547 and client messages to UDP destination port 546."
	message := ipv6DHCPMessage{length: len(payload) - 8, peer: netip.AddrPortFrom(source, 546)}
	copy(message.payload[:], payload[8:])
	select {
	case c.dhcp <- message:
	default:
		s.countIPv6Discard("queue-full")
	}
}

// RFC 9915 Section 5: "The client uses a link-local source address or
// addresses determined through other mechanisms for transmitting and
// receiving DHCP messages."
func validIPv6ControlPeer(address netip.Addr) bool {
	if !address.Is6() {
		return false
	}
	if address.Is4In6() {
		return false
	}
	// RFC 4291 Section 2.5.3: "The loopback address must not be used as the
	// source address in IPv6 packets that are sent outside of a single node."
	if address.IsLoopback() {
		return false
	}
	if address.IsGlobalUnicast() {
		return true
	}
	return address.IsLinkLocalUnicast()
}

// RFC 8200 Section 3: "Length of the IPv6 payload, i.e., the rest of the
// packet following this IPv6 header, in octets."
func writeIPv6Control(buf []byte, source, target netip.Addr, next, hopLimit byte, payload []byte) (int, error) {
	if len(payload) > ppp.MaxFrameLen-40 {
		return 0, errors.New("pppoe-vpp: IPv6 control exceeds the PPP frame bound")
	}
	length := ppp.ProtoFieldLen + 40 + len(payload)
	if len(buf) < length {
		return 0, errors.New("pppoe-vpp: IPv6 output buffer is too short")
	}
	clear(buf[:length])
	binary.BigEndian.PutUint16(buf[:2], ppp.ProtoIPv6)
	packet := buf[2:length]
	packet[0] = 0x60
	binary.BigEndian.PutUint16(packet[4:6], uint16(len(payload)))
	packet[6], packet[7] = next, hopLimit
	from, to := source.As16(), target.As16()
	copy(packet[8:24], from[:])
	copy(packet[24:40], to[:])
	copy(packet[40:], payload)
	return length, nil
}

// RFC 4443 Section 2.3: "The checksum is the 16-bit one's complement of the
// one's complement sum of the entire ICMPv6 message, starting with the ICMPv6
// message type field, and prepended with a "pseudo-header" of IPv6 header
// fields, as specified in [IPv6, Section 8.1]."
// RFC 8200 Section 8.1 uses the same pseudoheader for UDP. Each input is
// bounded by the 1500-octet PPP Information field; the sum cannot overflow.
func ipv6Checksum(source, target netip.Addr, next byte, payload []byte) uint16 {
	from, to := source.As16(), target.As16()
	sum := uint32(len(payload)) + uint32(next)
	for off := 0; off < 16; off += 2 {
		sum += uint32(binary.BigEndian.Uint16(from[off : off+2]))
		sum += uint32(binary.BigEndian.Uint16(to[off : off+2]))
	}
	for off := 0; off+1 < len(payload); off += 2 {
		sum += uint32(binary.BigEndian.Uint16(payload[off : off+2]))
	}
	if len(payload)%2 != 0 {
		sum += uint32(payload[len(payload)-1]) << 8
	}
	for sum > 0xffff {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}
