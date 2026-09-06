// Design: docs/architecture/diagnostics/packet-capture.md -- dissecting one captured record
// Overview: reassemble.go -- the stream this dissection feeds

package pcap

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
)

// The errors a record's dissection produces. Each one says which layer refused
// the bytes, so a report can tell an unsupported link type from a corrupt
// packet, and neither is reported as an empty result.
var (
	// ErrLinkTypeUnsupported says the file's link type has no dissector here.
	ErrLinkTypeUnsupported = errors.New("pcap: unsupported link type")
	// ErrNotIP says the frame carries something other than IPv4 or IPv6.
	ErrNotIP = errors.New("pcap: frame is not IP")
	// ErrNotTCP says the IP packet carries a protocol other than TCP.
	ErrNotTCP = errors.New("pcap: packet is not TCP")
	// ErrFragmented says the IPv4 packet is a fragment. Reassembling IP
	// fragments is a second reassembly problem, and a BGP session never
	// produces one.
	ErrFragmented = errors.New("pcap: IPv4 packet is fragmented")
	// ErrTruncated says a header claimed more bytes than the record holds.
	ErrTruncated = errors.New("pcap: header runs past the captured bytes")
)

// extensionHeaderMax bounds the IPv6 extension header chain a dissection walks.
// The chain length is attacker-controlled, so the walk is bounded rather than
// run to its end.
const extensionHeaderMax = 8

// The IPv6 extension header numbers this walk skips over (RFC 8200 Section 4).
const (
	extensionHopByHop = 0
	extensionRouting  = 43
	extensionFragment = 44
	extensionDestOpts = 60
)

// The link-layer header lengths this package strips.
const (
	ethernetHeaderLen  = 14
	linuxSLLHeaderLen  = 16
	linuxSLL2HeaderLen = 20
	vlanTagLen         = 4
)

// The EtherType values a dissection follows.
const (
	etherTypeIPv4 = 0x0800
	etherTypeIPv6 = 0x86DD
	etherTypeVLAN = 0x8100
	etherTypeQinQ = 0x88A8
)

// vlanTagMax bounds the stacked VLAN tags a dissection strips. Two covers
// Q-in-Q, which is the deepest stack Ze meets.
const vlanTagMax = 2

// segment is one TCP segment lifted out of a record: which direction it
// belongs to, where it sits in that direction's byte stream, and what it
// carries.
type segment struct {
	flow     Flow
	sequence uint32
	// synchronize is true when the SYN flag was set, which makes sequence the
	// initial sequence number rather than the position of a data byte.
	synchronize bool
	data        []byte
}

// dissect turns one record's bytes into a TCP segment. It returns one of the
// named errors above when the record carries something else, so a caller can
// count what it skipped rather than treat a skip as an empty capture.
func dissect(linkType uint32, data []byte) (segment, error) {
	payload, err := stripLinkLayer(linkType, data)
	if err != nil {
		return segment{}, err
	}
	return dissectIP(payload)
}

// stripLinkLayer removes the link-layer header the file's link type declares
// and returns the IP packet inside it.
func stripLinkLayer(linkType uint32, data []byte) ([]byte, error) {
	switch linkType {
	case LinkTypeRaw, LinkTypeIPv4, LinkTypeIPv6:
		return data, nil
	case LinkTypeEthernet:
		return stripEthernet(data)
	case LinkTypeLinuxSLL:
		return stripCooked(data, linuxSLLHeaderLen, 14)
	case LinkTypeLinuxSLL2:
		return stripCooked(data, linuxSLL2HeaderLen, 0)
	default:
		return nil, fmt.Errorf("%w: %d", ErrLinkTypeUnsupported, linkType)
	}
}

// stripEthernet removes the 14-byte Ethernet header and any stacked VLAN tags,
// and refuses a frame whose EtherType is not IP.
func stripEthernet(data []byte) ([]byte, error) {
	if len(data) < ethernetHeaderLen {
		return nil, ErrTruncated
	}
	etherType := binary.BigEndian.Uint16(data[12:14])
	offset := ethernetHeaderLen

	for tags := 0; etherType == etherTypeVLAN || etherType == etherTypeQinQ; tags++ {
		if tags == vlanTagMax {
			return nil, fmt.Errorf("%w: more than %d stacked VLAN tags", ErrNotIP, vlanTagMax)
		}
		if len(data) < offset+vlanTagLen {
			return nil, ErrTruncated
		}
		etherType = binary.BigEndian.Uint16(data[offset+2 : offset+4])
		offset += vlanTagLen
	}

	if etherType != etherTypeIPv4 && etherType != etherTypeIPv6 {
		return nil, fmt.Errorf("%w: EtherType 0x%04X", ErrNotIP, etherType)
	}
	return data[offset:], nil
}

// stripCooked removes a Linux cooked-capture header. headerLen is the header's
// size and protocolOffset is where its EtherType-equivalent sits; SLL2 carries
// its protocol first, at offset 0.
func stripCooked(data []byte, headerLen, protocolOffset int) ([]byte, error) {
	if len(data) < headerLen {
		return nil, ErrTruncated
	}
	protocol := binary.BigEndian.Uint16(data[protocolOffset : protocolOffset+2])
	if protocol != etherTypeIPv4 && protocol != etherTypeIPv6 {
		return nil, fmt.Errorf("%w: cooked protocol 0x%04X", ErrNotIP, protocol)
	}
	return data[headerLen:], nil
}

// dissectIP reads an IPv4 or IPv6 header, selected by the version nibble of the
// first byte, and hands the TCP segment inside it to dissectTCP.
func dissectIP(data []byte) (segment, error) {
	if len(data) < 1 {
		return segment{}, ErrTruncated
	}
	switch data[0] >> 4 {
	case 4:
		return dissectIPv4(data)
	case 6:
		return dissectIPv6(data)
	default:
		return segment{}, fmt.Errorf("%w: IP version %d", ErrNotIP, data[0]>>4)
	}
}

// dissectIPv4 reads the IPv4 header of RFC 791 Section 3.1. The IHL and the
// total length both come off the wire, so each is checked against the bytes
// present before it indexes anything.
func dissectIPv4(data []byte) (segment, error) {
	if len(data) < IPv4HeaderLen {
		return segment{}, ErrTruncated
	}
	headerLen := int(data[0]&0x0F) * 4
	if headerLen < IPv4HeaderLen {
		return segment{}, fmt.Errorf("%w: IHL %d is below the 5-word minimum", ErrNotIP, headerLen/4)
	}
	if headerLen > len(data) {
		return segment{}, ErrTruncated
	}

	// RFC 791 Section 3.1: a non-zero fragment offset, or the More Fragments
	// flag, marks a fragment. Neither belongs to a BGP session.
	fragment := binary.BigEndian.Uint16(data[6:8])
	if fragment&0x1FFF != 0 || fragment&0x2000 != 0 {
		return segment{}, ErrFragmented
	}

	if data[9] != protocolTCP {
		return segment{}, fmt.Errorf("%w: IP protocol %d", ErrNotTCP, data[9])
	}

	source, _ := netip.AddrFromSlice(data[12:16])
	target, _ := netip.AddrFromSlice(data[16:20])

	// The total length is what was on the network. A truncated record holds
	// fewer bytes, so the shorter of the two bounds the TCP segment.
	totalLen := int(binary.BigEndian.Uint16(data[2:4]))
	end := min(totalLen, len(data))
	if end < headerLen {
		end = len(data)
	}

	return dissectTCP(data[headerLen:end], source, target)
}

// dissectIPv6 reads the IPv6 header of RFC 8200 Section 3 and walks its
// extension header chain, bounded by extensionHeaderMax, to reach TCP.
func dissectIPv6(data []byte) (segment, error) {
	if len(data) < IPv6HeaderLen {
		return segment{}, ErrTruncated
	}

	source, _ := netip.AddrFromSlice(data[8:24])
	target, _ := netip.AddrFromSlice(data[24:40])

	payloadLen := int(binary.BigEndian.Uint16(data[4:6]))
	end := min(IPv6HeaderLen+payloadLen, len(data))
	if end < IPv6HeaderLen {
		end = len(data)
	}

	next := data[6]
	rest := data[IPv6HeaderLen:end]
	for hops := 0; next != protocolTCP; hops++ {
		if hops == extensionHeaderMax {
			return segment{}, fmt.Errorf("%w: more than %d extension headers", ErrNotTCP, extensionHeaderMax)
		}
		switch next {
		case extensionHopByHop, extensionRouting, extensionDestOpts:
			if len(rest) < 8 {
				return segment{}, ErrTruncated
			}
			// RFC 8200 Section 4.3: the length is in 8-octet units, not
			// counting the first eight octets.
			extensionLen := (int(rest[1]) + 1) * 8
			if extensionLen > len(rest) {
				return segment{}, ErrTruncated
			}
			next, rest = rest[0], rest[extensionLen:]
		case extensionFragment:
			return segment{}, ErrFragmented
		default:
			return segment{}, fmt.Errorf("%w: IPv6 next header %d", ErrNotTCP, next)
		}
	}

	return dissectTCP(rest, source, target)
}

// dissectTCP reads the TCP header of RFC 9293 Section 3.1 and returns the
// segment its payload belongs to. The data offset comes off the wire, so it is
// checked against the bytes present before it indexes anything.
func dissectTCP(data []byte, source, target netip.Addr) (segment, error) {
	if len(data) < TCPHeaderLen {
		return segment{}, ErrTruncated
	}
	headerLen := int(data[12]>>4) * 4
	if headerLen < TCPHeaderLen {
		return segment{}, fmt.Errorf("%w: data offset %d is below the 5-word minimum", ErrNotTCP, headerLen/4)
	}
	if headerLen > len(data) {
		return segment{}, ErrTruncated
	}

	return segment{
		flow: Flow{
			SourceAddr: source,
			TargetAddr: target,
			SourcePort: binary.BigEndian.Uint16(data[0:2]),
			TargetPort: binary.BigEndian.Uint16(data[2:4]),
		},
		sequence: binary.BigEndian.Uint32(data[4:8]),
		// RFC 9293 Section 3.1: the SYN flag is bit 1 of the flags octet.
		synchronize: data[13]&0x02 != 0,
		data:        data[headerLen:],
	}, nil
}
