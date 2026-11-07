// Design: docs/architecture/testing/interop.md -- the access chain.
// Related: check_access.go -- the checkers that read these captures.
package topology

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
)

// The PPPoE discovery codes (RFC 2516 Section 5) and the Service-Name tag
// (Appendix A) the access chain's discovery verdict reads.
const (
	pppoeCodePADO       = 0x07
	pppoeCodePADS       = 0x65
	pppoeTagServiceName = 0x0101
	etherTypeDiscovery  = 0x8863
	etherTypeIPv4       = 0x0800
	ipProtocolUDP       = 17
	l2tpPort            = 1701
	pppProtocolLCP      = 0xc021
	lcpConfigureRequest = 1
	lcpOptionMRU        = 1
)

// The fixed sizes the capture parsers check every length against.
const (
	pcapHeaderOctets   = 24
	pcapRecordOctets   = 16
	ethernetOctets     = 14
	pppoeHeaderOctets  = 6
	pppoeTagOctets     = 4
	udpHeaderOctets    = 8
	l2tpFlagsOctets    = 2
	lcpHeaderOctets    = 4
	lcpOptionMinOctets = 2
)

// errCaptureEmpty says a capture holds no frame the verdict reads yet, so a
// checker keeps waiting rather than failing on the first read.
var errCaptureEmpty = errors.New("the capture holds no frame of the kind the check reads")

// pcapFrames splits a classic pcap file (either byte order, Ethernet link
// type) into its frames. The global header is 24 octets; each record header
// is 16, with the captured length at octets 8..12. Every length is checked
// against the octets that remain before it is used.
func pcapFrames(capture []byte) ([][]byte, error) {
	if len(capture) < pcapHeaderOctets {
		return nil, fmt.Errorf("capture of %d octets has no pcap header", len(capture))
	}
	var order binary.ByteOrder = binary.LittleEndian
	if bytes.Equal(capture[:4], []byte{0xa1, 0xb2, 0xc3, 0xd4}) {
		order = binary.BigEndian
	}
	var frames [][]byte
	rest := capture[pcapHeaderOctets:]
	for len(rest) >= pcapRecordOctets {
		captured := int(order.Uint32(rest[8:12]))
		if captured > len(rest)-pcapRecordOctets {
			return nil, fmt.Errorf("capture record claims %d octets, %d remain", captured, len(rest)-pcapRecordOctets)
		}
		frames = append(frames, rest[pcapRecordOctets:pcapRecordOctets+captured])
		rest = rest[pcapRecordOctets+captured:]
	}
	return frames, nil
}

// discoveryVerdict judges AC-1 over ze-lac's access capture. The frames are
// untagged Ethernet (the capture runs on the VLAN device), ethertype 0x8863,
// then the PPPoE header (RFC 2516 Section 4):
//
//	0       1       2       3       4       5       6
//	+-------+-------+-------+-------+-------+-------+----
//	|VER|TYP| CODE  |  SESSION_ID   |    LENGTH     | tags
//	+-------+-------+-------+-------+-------+-------+----
//
// and each tag is TAG_TYPE(2), TAG_LENGTH(2), value. It passes when a PADO
// carries a Service-Name tag of length 0, the answer to a subscriber that
// asked for no service, and a PADS carries a non-zero SESSION_ID. A capture
// with neither is errCaptureEmpty; a PADO whose Service-Name is not empty, or
// a PADS with SESSION_ID 0, is refused.
func discoveryVerdict(frames [][]byte) error {
	var pado, pads bool
	for _, frame := range frames {
		code, session, tags, ok := discoveryOfFrame(frame)
		if !ok {
			continue
		}
		if code == pppoeCodePADO {
			length, found := serviceNameLength(tags)
			if !found {
				return errors.New("ze-lac sent a PADO with no Service-Name tag, which RFC 2516 Section 5.2 requires")
			}
			if length != 0 {
				return fmt.Errorf("ze-lac's PADO carries a Service-Name of %d octets; the CPE asked for none, so the relay keyed by \"\" did not answer", length)
			}
			pado = true
		}
		if code == pppoeCodePADS {
			if session == 0 {
				return errors.New("ze-lac's PADS carries SESSION_ID 0, which RFC 2516 Section 5.4 reserves for a refusal")
			}
			pads = true
		}
	}
	if !pado {
		return fmt.Errorf("%w: no PADO from ze-lac", errCaptureEmpty)
	}
	if !pads {
		return fmt.Errorf("%w: a PADO but no PADS from ze-lac", errCaptureEmpty)
	}
	return nil
}

// discoveryOfFrame answers one discovery frame's code, session id and tag
// octets, bounded by the header's LENGTH.
func discoveryOfFrame(frame []byte) (code byte, session uint16, tags []byte, ok bool) {
	if len(frame) < ethernetOctets+pppoeHeaderOctets {
		return 0, 0, nil, false
	}
	if binary.BigEndian.Uint16(frame[12:14]) != etherTypeDiscovery {
		return 0, 0, nil, false
	}
	header := frame[ethernetOctets:]
	length := int(binary.BigEndian.Uint16(header[4:6]))
	if length > len(header)-pppoeHeaderOctets {
		return 0, 0, nil, false
	}
	return header[1], binary.BigEndian.Uint16(header[2:4]), header[pppoeHeaderOctets : pppoeHeaderOctets+length], true
}

// serviceNameLength answers the TAG_LENGTH of the first Service-Name tag.
func serviceNameLength(tags []byte) (int, bool) {
	for len(tags) >= pppoeTagOctets {
		kind := binary.BigEndian.Uint16(tags[0:2])
		length := int(binary.BigEndian.Uint16(tags[2:4]))
		if length > len(tags)-pppoeTagOctets {
			return 0, false
		}
		if kind == pppoeTagServiceName {
			return length, true
		}
		tags = tags[pppoeTagOctets+length:]
	}
	return 0, false
}

// lcpMRUVerdict judges the core-link half of AC-11 over ze-lns's capture: an
// LCP Configure-Request sent from source to port 1701 carries the MRU option
// with value want. The frames are untagged Ethernet, IPv4, UDP, then the
// L2TPv2 data header (RFC 2661 Section 3.1):
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|T|L|x|x|S|x|O|P|x|x|x|x|  Ver  |          Length (opt)         |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|           Tunnel ID           |           Session ID          |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|             Ns (opt)          |             Nr (opt)          |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|      Offset Size (opt)        |    Offset pad... (opt)
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// then the PPP frame: an optional FF 03 address and control, the protocol
// (0xC021 for LCP), and the LCP packet (code, identifier, length, options of
// type, length, value). A capture with no such request is errCaptureEmpty; a
// request with no MRU option or another value is refused.
func lcpMRUVerdict(frames [][]byte, source netip.Addr, want uint16) error {
	for _, frame := range frames {
		options, ok := lcpRequestOfFrame(frame, source)
		if !ok {
			continue
		}
		mru, found := lcpMRUOption(options)
		if !found {
			return fmt.Errorf("%s sent an LCP Configure-Request with no MRU option; ze-lns is configured mru %d", source, want)
		}
		if mru != want {
			return fmt.Errorf("%s sent an LCP Configure-Request with MRU %d, want %d", source, mru, want)
		}
		return nil
	}
	return fmt.Errorf("%w: no LCP Configure-Request from %s", errCaptureEmpty, source)
}

// lcpRequestOfFrame answers the options of the LCP Configure-Request one
// L2TP data frame from source carries.
func lcpRequestOfFrame(frame []byte, source netip.Addr) ([]byte, bool) {
	udp, ok := udpFrom(frame, source)
	if !ok {
		return nil, false
	}
	ppp, ok := l2tpPayload(udp)
	if !ok {
		return nil, false
	}
	if len(ppp) >= 2 && ppp[0] == 0xff && ppp[1] == 0x03 {
		ppp = ppp[2:]
	}
	if len(ppp) < 2+lcpHeaderOctets {
		return nil, false
	}
	if binary.BigEndian.Uint16(ppp[0:2]) != pppProtocolLCP {
		return nil, false
	}
	lcp := ppp[2:]
	if lcp[0] != lcpConfigureRequest {
		return nil, false
	}
	length := int(binary.BigEndian.Uint16(lcp[2:4]))
	if length < lcpHeaderOctets {
		return nil, false
	}
	if length > len(lcp) {
		return nil, false
	}
	return lcp[lcpHeaderOctets:length], true
}

// udpFrom answers the UDP payload of an IPv4 datagram from source to port
// 1701, bounded by the UDP length so Ethernet padding is not read.
func udpFrom(frame []byte, source netip.Addr) ([]byte, bool) {
	if len(frame) < ethernetOctets+20 {
		return nil, false
	}
	if binary.BigEndian.Uint16(frame[12:14]) != etherTypeIPv4 {
		return nil, false
	}
	ip := frame[ethernetOctets:]
	headerOctets := int(ip[0]&0x0f) * 4
	if headerOctets < 20 {
		return nil, false
	}
	if ip[9] != ipProtocolUDP {
		return nil, false
	}
	if netip.AddrFrom4([4]byte(ip[12:16])) != source {
		return nil, false
	}
	if len(ip) < headerOctets+udpHeaderOctets {
		return nil, false
	}
	udp := ip[headerOctets:]
	if binary.BigEndian.Uint16(udp[2:4]) != l2tpPort {
		return nil, false
	}
	udpOctets := int(binary.BigEndian.Uint16(udp[4:6]))
	if udpOctets < udpHeaderOctets {
		return nil, false
	}
	if udpOctets > len(udp) {
		return nil, false
	}
	return udp[udpHeaderOctets:udpOctets], true
}

// l2tpPayload answers the PPP frame an L2TPv2 data message carries, and
// refuses a control message (T bit) or another version.
func l2tpPayload(message []byte) ([]byte, bool) {
	if len(message) < l2tpFlagsOctets {
		return nil, false
	}
	flags := binary.BigEndian.Uint16(message[0:2])
	if flags&0x8000 != 0 {
		return nil, false
	}
	if flags&0x000f != 2 {
		return nil, false
	}
	offset := l2tpFlagsOctets
	if flags&0x4000 != 0 {
		offset += 2
	}
	offset += 4 // Tunnel ID and Session ID.
	if flags&0x0800 != 0 {
		offset += 4
	}
	if flags&0x0200 != 0 {
		if len(message) < offset+2 {
			return nil, false
		}
		offset += 2 + int(binary.BigEndian.Uint16(message[offset:offset+2]))
	}
	if offset > len(message) {
		return nil, false
	}
	return message[offset:], true
}

// lcpMRUOption answers the value of the first MRU option (type 1, length 4).
func lcpMRUOption(options []byte) (uint16, bool) {
	for len(options) >= lcpOptionMinOctets {
		kind := options[0]
		length := int(options[1])
		if length < lcpOptionMinOctets {
			return 0, false
		}
		if length > len(options) {
			return 0, false
		}
		if kind == lcpOptionMRU {
			if length != 4 {
				return 0, false
			}
			return binary.BigEndian.Uint16(options[2:4]), true
		}
		options = options[length:]
	}
	return 0, false
}
