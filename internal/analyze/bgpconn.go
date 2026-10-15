// Design: docs/architecture/mrt.md -- shared BGP connection helpers for inject/replay/serve

package analyze

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"

	"github.com/ze-software/ze/internal/core/bgp/capability"
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
func bgpBuildOpen(localAS uint32, holdTime uint16, routerID net.IP) ([]byte, error) {
	as2 := uint16(localAS)
	if localAS > 65535 {
		as2 = 23456 // AS_TRANS per RFC 6793
	}
	rid := routerID.To4()
	if rid == nil {
		rid = net.IP{1, 0, 0, 1}
	}

	// One Capabilities parameter, packed by the one packer. RFC 9072 Section 2
	caps := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
		&capability.ASN4{ASN: localAS},
	}
	optParams, extended, err := capability.PackOptionalParams([]capability.OptionalParam{capability.CapabilitiesParam(caps)})
	if err != nil {
		return nil, err
	}
	// This body has no RFC 9072 envelope, so it refuses rather than write the
	// extended parameters behind a one-octet Opt Parm Len.
	if extended {
		return nil, errAnalyzeOpenTooLarge
	}

	body := make([]byte, 10+len(optParams))
	body[0] = 4
	binary.BigEndian.PutUint16(body[1:3], as2)
	binary.BigEndian.PutUint16(body[3:5], holdTime)
	copy(body[5:9], rid)
	body[9] = byte(len(optParams))
	copy(body[10:], optParams)
	return body, nil
}

// errAnalyzeOpenTooLarge reports capabilities past the 255 octets the RFC
// 4271 OPEN body bgpBuildOpen writes can carry.
var errAnalyzeOpenTooLarge = errors.New("analyze: OPEN capabilities exceed 255 octets of Optional Parameters")

// errAnalyzePeerOpenShort reports a peer OPEN body shorter than its fixed
// fields or than the Optional Parameters length it states.
var errAnalyzePeerOpenShort = errors.New("analyze: peer OPEN is shorter than the length it states")

// bgpPeerAS answers the AS a peer's OPEN body advertises: the Four-octet AS
// capability when the OPEN carries one, else the two-octet My Autonomous System.
//
// The capabilities are read by capability.ParseFromOptionalParams, the one
// Optional Parameters decoder, in the framing the OPEN states. An OPEN that
// decoder refuses is an error, never the My AS fallback: an AS answered for an
// OPEN nobody could read is a wrong AS.
func bgpPeerAS(body []byte) (uint32, error) {
	if len(body) < 10 {
		return 0, errAnalyzePeerOpenShort
	}
	peerAS := uint32(binary.BigEndian.Uint16(body[1:3]))
	params, extended, err := bgpPeerOptionalParams(body)
	if err != nil {
		return 0, err
	}
	// RFC 9072 Section 2
	caps, err := capability.ParseFromOptionalParams(params, extended)
	if err != nil {
		return 0, fmt.Errorf("analyze: peer OPEN: %w", err)
	}
	for _, c := range caps {
		if asn4, ok := c.(*capability.ASN4); ok {
			return asn4.ASN, nil
		}
	}
	return peerAS, nil
}

// bgpPeerOptionalParams cuts the Optional Parameters out of an OPEN body and
// reports whether they carry the RFC 9072 two-octet framing.
//
// RFC 9072 Section 2: "In parsing an OPEN message, if the one-octet Optional
// Parameters Length field (labeled "Non-Ext OP Len." in Figure 1) is non-zero,
// a BGP speaker MUST use the value of the octet following the one-octet
// Optional Parameters Length field (labeled "Non-Ext OP Type" in Figure 1) to
// determine both the encoding of the Optional Parameters length and the size
// of the Parameter Length field of individual Optional Parameters.".
func bgpPeerOptionalParams(body []byte) ([]byte, bool, error) {
	paramsLen := int(body[9])
	if paramsLen > 0 && len(body) > 10 && body[10] == 255 {
		if len(body) < 13 {
			return nil, false, errAnalyzePeerOpenShort
		}
		extendedLen := int(binary.BigEndian.Uint16(body[11:13]))
		if len(body) < 13+extendedLen {
			return nil, false, errAnalyzePeerOpenShort
		}
		return body[13 : 13+extendedLen], true, nil
	}
	if len(body) < 10+paramsLen {
		return nil, false, errAnalyzePeerOpenShort
	}
	return body[10 : 10+paramsLen], false, nil
}

// BMP v3 header sizes (RFC 7854).
const (
	BMPCommonHdrLen = 6
	BMPPeerHdrLen   = 42
)

var errBGPProtocol = net.UnknownNetworkError("bgp protocol error")
