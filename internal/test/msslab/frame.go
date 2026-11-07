// Design: docs/guide/firewall.md -- tcp-mss-set, the clamp a forwarded SYN meets
// Related: msslab_linux.go -- the router that forwards these frames

// Package msslab forwards a crafted TCP SYN through a real Linux router
// namespace and reports the MSS option the next hop receives. It is the one
// implementation behind the nft integration test of `tcp-mss-set` and the
// policy-routing fixture of `tcp-mss`, so both proofs read the packet the
// kernel forwarded, never the ruleset that was installed.
package msslab

import (
	"encoding/binary"
	"fmt"
)

// Family selects the IP version of the probe.
type Family uint8

// Family values. Zero is Unspecified, so a forgotten family is refused.
const (
	FamilyUnspecified Family = iota
	FamilyIPv4
	FamilyIPv6
)

// String names the family for a test log.
func (f Family) String() string {
	switch f {
	case FamilyIPv4:
		return "ipv4"
	case FamilyIPv6:
		return "ipv6"
	case FamilyUnspecified:
		return "unspecified"
	}
	return "unknown family"
}

// Option is the MSS option of a SYN: absent, or present with its value.
type Option struct {
	Present bool
	Value   uint16
}

// Seen is what the egress peer read from the forwarded SYN.
type Seen struct {
	MSS Option
	// HeaderOctets is the TCP header length the forwarded SYN declares, so a
	// test can show that no option was added to a SYN that carried none.
	HeaderOctets int
}

// TCP option kinds and the SYN's fixed fields (RFC 9293 Section 3.1).
const (
	optionEnd     = 0
	optionNOP     = 1
	optionMSS     = 2
	optionMSSLen  = 4
	tcpHeaderMin  = 20
	tcpFlagSYN    = 0x02
	tcpTargetPort = 80
	protoTCP      = 6
	ethHeaderLen  = 14
	ipv4HeaderLen = 20
	ipv6HeaderLen = 40
	etherTypeIPv4 = 0x0800
	etherTypeIPv6 = 0x86dd
)

var (
	// The ingress pair's addresses: the injected frame's destination and
	// source, written here so the frame is built on any host.
	ingressRouterMAC = [6]byte{0x02, 0, 0, 0, 0x01, 0x01}
	ingressPeerMAC   = [6]byte{0x02, 0, 0, 0, 0x01, 0x02}

	sourceV4 = [4]byte{192, 0, 2, 10}
	targetV4 = [4]byte{198, 51, 100, 20}
	sourceV6 = [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0x0a, 15: 0x10}
	targetV6 = [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0x0b, 15: 0x20}
)

func etherType(family Family) (uint16, error) {
	switch family {
	case FamilyIPv4:
		return etherTypeIPv4, nil
	case FamilyIPv6:
		return etherTypeIPv6, nil
	case FamilyUnspecified:
	}
	return 0, fmt.Errorf("msslab: no ether type for family %s", family)
}

// synFrame builds the Ethernet frame of one SYN from the ingress peer to the
// router's ingress side, addressed to the target behind the egress side.
//
//	Ethernet: dst(6) src(6) type(2)
//	IPv4: ver/ihl(1) tos(1) total(2) id(2) frag(2) ttl(1) proto(1) csum(2) src(4) dst(4)
//	IPv6: ver/tc/flow(4) payload(2) next(1) hops(1) src(16) dst(16)
//	TCP: sport(2) dport(2) seq(4) ack(4) off(1) flags(1) win(2) csum(2) urg(2) [02 04 mss(2)]
func synFrame(family Family, sent Option, sourcePort uint16) ([]byte, error) {
	kind, err := etherType(family)
	if err != nil {
		return nil, err
	}
	tcpLen := tcpHeaderMin
	if sent.Present {
		tcpLen += optionMSSLen
	}
	ipLen := ipv4HeaderLen
	if family == FamilyIPv6 {
		ipLen = ipv6HeaderLen
	}
	frame := make([]byte, ethHeaderLen+ipLen+tcpLen)
	copy(frame[0:6], ingressRouterMAC[:])
	copy(frame[6:12], ingressPeerMAC[:])
	binary.BigEndian.PutUint16(frame[12:14], kind)

	tcp := frame[ethHeaderLen+ipLen:]
	binary.BigEndian.PutUint16(tcp[0:2], sourcePort)
	binary.BigEndian.PutUint16(tcp[2:4], tcpTargetPort)
	binary.BigEndian.PutUint32(tcp[4:8], 0x5a450001)
	tcp[12] = byte(tcpLen/4) << 4
	tcp[13] = tcpFlagSYN
	binary.BigEndian.PutUint16(tcp[14:16], 64240)
	if sent.Present {
		tcp[20] = optionMSS
		tcp[21] = optionMSSLen
		binary.BigEndian.PutUint16(tcp[22:24], sent.Value)
	}

	ip := frame[ethHeaderLen : ethHeaderLen+ipLen]
	var pseudo uint32
	if family == FamilyIPv4 {
		ip[0] = 0x45
		binary.BigEndian.PutUint16(ip[2:4], uint16(ipLen+tcpLen))
		binary.BigEndian.PutUint16(ip[6:8], 0x4000) // DF
		ip[8] = 64
		ip[9] = protoTCP
		copy(ip[12:16], sourceV4[:])
		copy(ip[16:20], targetV4[:])
		binary.BigEndian.PutUint16(ip[10:12], checksum(0, ip))
		pseudo = sum(sum(0, sourceV4[:]), targetV4[:])
	} else {
		ip[0] = 0x60
		binary.BigEndian.PutUint16(ip[4:6], uint16(tcpLen))
		ip[6] = protoTCP
		ip[7] = 64
		copy(ip[8:24], sourceV6[:])
		copy(ip[24:40], targetV6[:])
		pseudo = sum(sum(0, sourceV6[:]), targetV6[:])
	}
	pseudo += protoTCP + uint32(tcpLen)
	binary.BigEndian.PutUint16(tcp[16:18], checksum(pseudo, tcp))
	return frame, nil
}

// parseSYN answers the MSS option of frame when it is this probe's SYN: the
// family's ether type, TCP, from sourcePort. Any other frame answers false.
func parseSYN(frame []byte, family Family, sourcePort uint16) (Seen, bool) {
	kind, err := etherType(family)
	if err != nil {
		return Seen{}, false
	}
	if len(frame) < ethHeaderLen {
		return Seen{}, false
	}
	if binary.BigEndian.Uint16(frame[12:14]) != kind {
		return Seen{}, false
	}
	ip := frame[ethHeaderLen:]
	var tcp []byte
	if family == FamilyIPv4 {
		if len(ip) < ipv4HeaderLen {
			return Seen{}, false
		}
		if ip[9] != protoTCP {
			return Seen{}, false
		}
		tcp = ip[int(ip[0]&0x0f)*4:]
	} else {
		if len(ip) < ipv6HeaderLen {
			return Seen{}, false
		}
		if ip[6] != protoTCP {
			return Seen{}, false
		}
		tcp = ip[ipv6HeaderLen:]
	}
	if len(tcp) < tcpHeaderMin {
		return Seen{}, false
	}
	if binary.BigEndian.Uint16(tcp[0:2]) != sourcePort {
		return Seen{}, false
	}
	headerOctets := int(tcp[12]>>4) * 4
	if headerOctets < tcpHeaderMin {
		return Seen{}, false
	}
	if headerOctets > len(tcp) {
		return Seen{}, false
	}
	return Seen{MSS: mssOption(tcp[tcpHeaderMin:headerOctets]), HeaderOctets: headerOctets}, true
}

// mssOption walks the option bytes of a TCP header. The walk is bounded by
// the header length, at most 40 octets.
func mssOption(options []byte) Option {
	for i := 0; i < len(options); {
		switch options[i] {
		case optionEnd:
			return Option{}
		case optionNOP:
			i++
			continue
		}
		if i+1 >= len(options) {
			return Option{}
		}
		length := int(options[i+1])
		if length < 2 {
			return Option{}
		}
		if i+length > len(options) {
			return Option{}
		}
		if options[i] == optionMSS && length == optionMSSLen {
			return Option{Present: true, Value: binary.BigEndian.Uint16(options[i+2 : i+4])}
		}
		i += length
	}
	return Option{}
}

func sum(acc uint32, data []byte) uint32 {
	for i := 0; i+1 < len(data); i += 2 {
		acc += uint32(binary.BigEndian.Uint16(data[i : i+2]))
	}
	if len(data)%2 == 1 {
		acc += uint32(data[len(data)-1]) << 8
	}
	return acc
}

// checksum answers the Internet checksum of data with acc already added (the
// pseudo-header for TCP, zero for the IPv4 header).
func checksum(acc uint32, data []byte) uint16 {
	acc = sum(acc, data)
	for acc>>16 != 0 {
		acc = acc&0xffff + acc>>16
	}
	return ^uint16(acc)
}
