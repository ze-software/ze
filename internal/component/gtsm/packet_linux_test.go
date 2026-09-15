//go:build linux

// Design: rfc/short/rfc5082.md -- the packets a GTSM proof has to build by hand
// Overview: netns_linux_test.go -- the namespace these frames are injected into
//
// The proofs inject packets the socket API cannot express: an ICMP error whose
// outer TTL the test chooses, quoting a TCP header of a session the test never
// opened. So each header is built byte by byte, at the offsets its RFC fixes.

package gtsm

import (
	"encoding/binary"
	"net/netip"

	"golang.org/x/sys/unix"
)

const (
	ethernetHeaderLen = 14
	ipv4HeaderLen     = 20
	ipv6HeaderLen     = 40
	// quotedTCPLen is how much of the offending TCP header an ICMP error
	// carries: RFC 792 asks for "64 bits of Original Data Datagram", which is
	// the ports, the sequence number, and nothing further.
	quotedTCPLen = 8

	ethertypeIPv4 = 0x0800
	ethertypeIPv6 = 0x86DD

	icmpv4DestinationUnreachable = 3
	icmpv4PortUnreachable        = 3
	icmpv6DestinationUnreachable = 1
	icmpv6PortUnreachable        = 4
)

// ethernetFrame wraps a network-layer packet for injection. The destination is
// ze's own interface address, so the kernel takes the frame for itself rather
// than deciding whether to forward it.
func ethernetFrame(destination, source []byte, ethertype uint16, packet []byte) []byte {
	frame := make([]byte, ethernetHeaderLen+len(packet))
	copy(frame[0:6], destination)
	copy(frame[6:12], source)
	binary.BigEndian.PutUint16(frame[12:14], ethertype)
	copy(frame[ethernetHeaderLen:], packet)
	return frame
}

// ipv4Packet builds the header RFC 791 Section 3.1 defines, with no options,
// so the header is 20 bytes and every offset below is fixed:
//
//	byte  0: version and header length (0x45)
//	byte  8: time to live
//	byte  9: protocol
//	byte 10: header checksum
//	byte 12: source address
//	byte 16: destination address
func ipv4Packet(source, destination netip.Addr, ttl, protocol uint8, payload []byte) []byte {
	return ipv4PacketWithOptions(source, destination, ttl, protocol, nil, payload)
}

// ipv4PacketWithOptions is ipv4Packet with an options field after the fixed
// header. RFC 791 Section 3.1 counts the header length in 32-bit words, so the
// options MUST be a multiple of four octets. The version and header length
// byte then reads 0x46 for four octets of options, not 0x45.
// A proof uses it for the one quoted header the fixed-offset reads cannot
// take at face value.
func ipv4PacketWithOptions(source, destination netip.Addr, ttl, protocol uint8, options, payload []byte) []byte {
	if len(options)%4 != 0 {
		panic("BUG: IPv4 options are counted in 32-bit words, so their length is a multiple of four")
	}
	headerLen := ipv4HeaderLen + len(options)
	packet := make([]byte, headerLen+len(payload))
	packet[0] = 0x40 | uint8(headerLen/4)
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	packet[8] = ttl
	packet[9] = protocol
	copy(packet[12:16], source.AsSlice())
	copy(packet[16:20], destination.AsSlice())
	copy(packet[ipv4HeaderLen:headerLen], options)
	binary.BigEndian.PutUint16(packet[10:12], internetChecksum(packet[:headerLen]))
	copy(packet[headerLen:], payload)
	return packet
}

// ipv6Packet builds the header RFC 8200 Section 3 defines:
//
//	byte  0: version, traffic class and flow label
//	byte  4: payload length
//	byte  6: next header
//	byte  7: hop limit
//	byte  8: source address
//	byte 24: destination address
func ipv6Packet(source, destination netip.Addr, hopLimit, nextHeader uint8, payload []byte) []byte {
	packet := make([]byte, ipv6HeaderLen+len(payload))
	packet[0] = 0x60
	binary.BigEndian.PutUint16(packet[4:6], uint16(len(payload)))
	packet[6] = nextHeader
	packet[7] = hopLimit
	copy(packet[8:24], source.AsSlice())
	copy(packet[24:40], destination.AsSlice())
	copy(packet[ipv6HeaderLen:], payload)
	return packet
}

// udpDatagram builds the header RFC 768 defines. The checksum stays zero for
// IPv4, where RFC 768 makes it optional, and is computed for IPv6, where RFC
// 8200 Section 8.1 makes it mandatory and a zero one is discarded.
func udpDatagram(sourcePort, destinationPort uint16, source, destination netip.Addr) []byte {
	datagram := make([]byte, 8)
	binary.BigEndian.PutUint16(datagram[0:2], sourcePort)
	binary.BigEndian.PutUint16(datagram[2:4], destinationPort)
	binary.BigEndian.PutUint16(datagram[4:6], uint16(len(datagram)))
	if source.Is6() {
		sum := pseudoHeaderChecksum(source, destination, unix.IPPROTO_UDP, datagram)
		binary.BigEndian.PutUint16(datagram[6:8], sum)
	}
	return datagram
}

// icmpv4Message builds the header RFC 792 gives every ICMPv4 message: a type,
// a code, a checksum, four bytes the type defines, and then the body. For an
// error message the body is the quoted datagram.
func icmpv4Message(messageType, code uint8, body []byte) []byte {
	message := make([]byte, 8+len(body))
	message[0] = messageType
	message[1] = code
	copy(message[8:], body)
	binary.BigEndian.PutUint16(message[2:4], internetChecksum(message))
	return message
}

// icmpv6Message is the same shape, with the checksum RFC 4443 Section 2.3
// computes over the IPv6 pseudo-header as well as the message.
func icmpv6Message(messageType, code uint8, source, destination netip.Addr, body []byte) []byte {
	message := make([]byte, 8+len(body))
	message[0] = messageType
	message[1] = code
	copy(message[8:], body)
	sum := pseudoHeaderChecksum(source, destination, unix.IPPROTO_ICMPV6, message)
	binary.BigEndian.PutUint16(message[2:4], sum)
	return message
}

// quotedTCPv4Datagram is what an ICMPv4 error carries when the packet that
// caused it was one ze sent over TCP: ze's own IPv4 header, then the first
// eight bytes of the TCP header.
func quotedTCPv4Datagram(source, destination netip.Addr, sourcePort, destinationPort uint16) []byte {
	return ipv4Packet(source, destination, 255, unix.IPPROTO_TCP, tcpHeadEight(sourcePort, destinationPort))
}

// quotedTCPv4DatagramWithOptions is quotedTCPv4Datagram for a packet ze sent
// with IP options in its header. The quoted TCP header then starts after the
// options, not at the twentieth octet.
func quotedTCPv4DatagramWithOptions(source, destination netip.Addr, options []byte, sourcePort, destinationPort uint16) []byte {
	return ipv4PacketWithOptions(source, destination, 255, unix.IPPROTO_TCP, options, tcpHeadEight(sourcePort, destinationPort))
}

// quotedTCPv6Datagram is the IPv6 form, which RFC 4443 Section 3.1 lets the
// sender carry as much of as the MTU allows.
func quotedTCPv6Datagram(source, destination netip.Addr, sourcePort, destinationPort uint16, sequence uint32) []byte {
	head := tcpHeadEight(sourcePort, destinationPort)
	binary.BigEndian.PutUint32(head[4:8], sequence)
	return ipv6Packet(source, destination, 255, unix.IPPROTO_TCP, head)
}

// tcpHeadEight is the first eight bytes of a TCP header: the two ports and the
// sequence number (RFC 9293 Section 3.1).
func tcpHeadEight(sourcePort, destinationPort uint16) []byte {
	head := make([]byte, quotedTCPLen)
	binary.BigEndian.PutUint16(head[0:2], sourcePort)
	binary.BigEndian.PutUint16(head[2:4], destinationPort)
	return head
}

// internetChecksum is the one's complement of the one's complement sum of the
// 16-bit words, which RFC 1071 defines and every header here uses.
func internetChecksum(data []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(data); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(data[i : i+2]))
	}
	if len(data)%2 == 1 {
		sum += uint32(data[len(data)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}
	return ^uint16(sum)
}

// pseudoHeaderChecksum is the IPv6 checksum of RFC 8200 Section 8.1: the
// source and destination addresses, the upper-layer length and the next header
// number, and then the message itself.
func pseudoHeaderChecksum(source, destination netip.Addr, nextHeader uint8, message []byte) uint16 {
	pseudo := make([]byte, 40+len(message))
	copy(pseudo[0:16], source.AsSlice())
	copy(pseudo[16:32], destination.AsSlice())
	binary.BigEndian.PutUint32(pseudo[32:36], uint32(len(message)))
	pseudo[39] = nextHeader
	copy(pseudo[40:], message)
	return internetChecksum(pseudo)
}
