// Design: docs/architecture/mrt.md -- shared BGP connection helpers for inject/replay/serve

package analyze

import (
	"encoding/binary"
	"net"
)

// bgpWrite sends a complete BGP message (marker + length + type + body).
func bgpWrite(conn net.Conn, msgType byte, body []byte) error {
	totalLen := 19 + len(body)
	msg := make([]byte, totalLen)
	for i := range 16 {
		msg[i] = 0xff
	}
	binary.BigEndian.PutUint16(msg[16:18], uint16(totalLen)) //nolint:gosec // bounded
	msg[18] = msgType
	if len(body) > 0 {
		copy(msg[19:], body)
	}
	_, err := conn.Write(msg)
	return err
}

// bgpReadMsg reads one complete BGP message and returns the type and body.
func bgpReadMsg(conn net.Conn) (msgType byte, body []byte, err error) {
	hdr := make([]byte, 19)
	if err := bgpReadFull(conn, hdr); err != nil {
		return 0, nil, err
	}
	msgLen := int(binary.BigEndian.Uint16(hdr[16:18]))
	if msgLen < 19 {
		return 0, nil, errBGPProtocol
	}
	if msgLen == 19 {
		return hdr[18], nil, nil
	}
	body = make([]byte, msgLen-19)
	if err := bgpReadFull(conn, body); err != nil {
		return 0, nil, err
	}
	return hdr[18], body, nil
}

// bgpReadFull reads exactly len(buf) bytes from conn.
func bgpReadFull(conn net.Conn, buf []byte) error {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return err
		}
	}
	return nil
}

// bgpBuildOpen constructs a BGP OPEN message body (without the 19-byte header).
// Advertises 4-byte ASN, IPv4 unicast, and IPv6 unicast capabilities.
func bgpBuildOpen(localAS uint32, holdTime uint16, routerID net.IP) []byte {
	as2 := uint16(localAS)
	if localAS > 65535 {
		as2 = 23456 // AS_TRANS per RFC 6793
	}
	rid := routerID.To4()
	if rid == nil {
		rid = net.IP{1, 0, 0, 1}
	}

	// RFC 5492: all capabilities in a single optional parameter (type 2).
	// Cap code 1 (MP): AFI(2) + reserved(1) + SAFI(1)
	capMP4 := []byte{1, 4, 0, 1, 0, 1} // IPv4 unicast
	capMP6 := []byte{1, 4, 0, 2, 0, 1} // IPv6 unicast
	cap4AS := []byte{65, 4, byte(localAS >> 24), byte(localAS >> 16), byte(localAS >> 8), byte(localAS)}

	caps := make([]byte, 0, len(capMP4)+len(capMP6)+len(cap4AS))
	caps = append(caps, capMP4...)
	caps = append(caps, capMP6...)
	caps = append(caps, cap4AS...)

	// Single optional parameter type 2 containing all capabilities
	optParams := make([]byte, 0, 2+len(caps))
	optParams = append(optParams, 2, byte(len(caps)))
	optParams = append(optParams, caps...)

	body := make([]byte, 10+len(optParams))
	body[0] = 4
	binary.BigEndian.PutUint16(body[1:3], as2)
	binary.BigEndian.PutUint16(body[3:5], holdTime)
	copy(body[5:9], rid)
	body[9] = byte(len(optParams))
	copy(body[10:], optParams)
	return body
}

// bgpExtractAS4 extracts the 4-byte ASN from optional parameters if present.
func bgpExtractAS4(opts []byte, fallback uint32) uint32 {
	off := 0
	for off+2 <= len(opts) {
		pType := opts[off]
		pLen := int(opts[off+1])
		off += 2
		if off+pLen > len(opts) {
			break
		}
		if pType != 2 {
			off += pLen
			continue
		}
		capData := opts[off : off+pLen]
		cOff := 0
		for cOff+2 <= len(capData) {
			code := capData[cOff]
			cLen := int(capData[cOff+1])
			cOff += 2
			if cOff+cLen > len(capData) {
				break
			}
			if code == 65 && cLen == 4 {
				return binary.BigEndian.Uint32(capData[cOff : cOff+4])
			}
			cOff += cLen
		}
		off += pLen
	}
	return fallback
}

// BMP v3 header sizes (RFC 7854).
const (
	BMPCommonHdrLen = 6
	BMPPeerHdrLen   = 42
)

var errBGPProtocol = net.UnknownNetworkError("bgp protocol error")
