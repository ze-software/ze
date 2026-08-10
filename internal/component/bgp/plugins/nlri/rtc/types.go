// Design: docs/architecture/wire/nlri.md — route target constraint plugin
// RFC: rfc/short/rfc4684.md
//
// Package bgp_rtc implements Route Target Constraint NLRI (RFC 4684, SAFI 132).
package rtc

import (
	"encoding/binary"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Type aliases for shared nlri types.
type (
	Family = family.Family
	AFI    = family.AFI
	SAFI   = family.SAFI
	NLRI   = nlri.NLRI
)

// Re-export constants.
const (
	AFIIPv4 = family.AFIIPv4
	SAFIRTC = family.SAFIRTC
)

// Family registration for RTC.
var IPv4RTC = family.MustRegister(AFIIPv4, SAFIRTC, "ipv4", "rtc")

// Errors for RTC parsing.
var ErrRTCTruncated = errors.New("rtc: truncated data")

// routeTarget represents a Route Target extended community.
//
// RFC 4360 defines extended communities as 8-octet values.
type routeTarget struct {
	Type  uint16  // Extended community type (2 bytes)
	Value [6]byte // Extended community value (6 bytes)
}

// Bytes returns the wire format of the route target (8 bytes).
func (rt routeTarget) Bytes() []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint16(buf[:2], rt.Type)
	copy(buf[2:], rt.Value[:])
	return buf
}

// String returns a human-readable route target.
//
// RFC 4360 Section 3 defines extended community types.
func (rt routeTarget) String() string {
	var b textbuf.Buffer
	switch rt.Type >> 8 {
	case 0x00: // 2-byte ASN (RFC 4360 Section 3.1)
		asn := binary.BigEndian.Uint16(rt.Value[:2])
		assigned := binary.BigEndian.Uint32(rt.Value[2:6])
		return b.Reset().Uint16(asn).Byte(':').Uint32(assigned).String()
	case 0x01: // IPv4 address (RFC 4360 Section 3.2)
		ip := netip.AddrFrom4([4]byte(rt.Value[:4]))
		assigned := binary.BigEndian.Uint16(rt.Value[4:6])
		return b.Addr(ip).Byte(':').Uint16(assigned).String()
	case 0x02: // 4-byte ASN (RFC 5668)
		asn := binary.BigEndian.Uint32(rt.Value[:4])
		assigned := binary.BigEndian.Uint16(rt.Value[4:6])
		return b.Uint32(asn).Byte(':').Uint16(assigned).String()
	default:
		return b.Str("rt-type").Uint16(rt.Type).Byte(':').Hex(rt.Value[:]).String()
	}
}

// RTC represents a Route Target Constraint NLRI (RFC 4684 Section 4).
//
// The RTC NLRI is used to advertise interest in receiving VPN routes
// with specific Route Targets.
type RTC struct {
	originAS    uint32      // Origin AS number (4 bytes)
	routeTarget routeTarget // Route Target extended community (8 bytes)
}

// newRTC creates a new RTC NLRI.
func newRTC(originAS uint32, rt routeTarget) *RTC {
	return &RTC{
		originAS:    originAS,
		routeTarget: rt,
	}
}

// parseRTC parses an RTC NLRI from wire format.
//
// RFC 4684 Section 4: prefix of 0 to 96 bits.
// A prefix-length of 0 = default route target.
func parseRTC(data []byte) (*RTC, []byte, error) {
	if len(data) < 1 {
		return nil, nil, ErrRTCTruncated
	}

	prefixLen := int(data[0])
	prefixBytes := (prefixLen + 7) / 8

	if len(data) < 1+prefixBytes {
		return nil, nil, ErrRTCTruncated
	}

	rtc := &RTC{}

	if prefixLen == 0 {
		return rtc, data[1:], nil
	}

	if prefixBytes >= 4 {
		rtc.originAS = binary.BigEndian.Uint32(data[1:5])
	}

	if prefixBytes >= 6 {
		rtc.routeTarget.Type = binary.BigEndian.Uint16(data[5:7])
	}
	if prefixBytes >= 12 {
		copy(rtc.routeTarget.Value[:], data[7:13])
	} else if prefixBytes > 6 {
		copy(rtc.routeTarget.Value[:prefixBytes-6], data[7:1+prefixBytes])
	}

	return rtc, data[1+prefixBytes:], nil
}

// Family returns the address family.
func (r *RTC) Family() Family {
	return Family{AFI: AFIIPv4, SAFI: SAFIRTC}
}

// OriginAS returns the origin AS number.
func (r *RTC) OriginAS() uint32 { return r.originAS }

// routeTargetValue returns the route target.
func (r *RTC) routeTargetValue() routeTarget { return r.routeTarget }

// isDefault returns true if this is the default RTC (matches all RTs).
//
// RFC 4684 Section 4: A zero-length prefix = default route target.
func (r *RTC) isDefault() bool {
	return r.originAS == 0 && r.routeTarget.Type == 0 && r.routeTarget.Value == [6]byte{}
}

// Bytes returns the wire-format encoding.
//
// RFC 4684 Section 4: prefix-length is in bits: 96 = 12 bytes.
func (r *RTC) Bytes() []byte {
	buf := make([]byte, r.Len())
	r.WriteTo(buf, 0)
	return buf
}

// Len returns the length in bytes.
// RFC 4684 Section 4: 1 byte for default, 13 bytes otherwise.
func (r *RTC) Len() int {
	if r.isDefault() {
		return 1
	}
	return 13
}

// PathID returns 0.
func (r *RTC) PathID() uint32 { return 0 }

// HasPathID returns false.
func (r *RTC) HasPathID() bool { return false }

// SupportsAddPath returns false - RTC doesn't support ADD-PATH.
func (r *RTC) SupportsAddPath() bool { return false }

// String returns command-style format for API round-trip compatibility.
func (r *RTC) String() string {
	if r.isDefault() {
		return "default"
	}
	var b textbuf.Buffer
	return b.Reset().Str("origin-as ").Uint32(r.originAS).Str(" rt ").Str(r.routeTarget.String()).String()
}

// WriteTo writes the RTC NLRI directly to buf at offset.
func (r *RTC) WriteTo(buf []byte, off int) int {
	if r.isDefault() {
		buf[off] = 0
		return 1
	}

	pos := off
	buf[pos] = 96
	pos++

	binary.BigEndian.PutUint32(buf[pos:], r.originAS)
	pos += 4

	binary.BigEndian.PutUint16(buf[pos:], r.routeTarget.Type)
	pos += 2

	copy(buf[pos:], r.routeTarget.Value[:])
	pos += 6

	return pos - off
}
