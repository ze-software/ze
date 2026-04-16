// Design: docs/architecture/wire/nlri.md — route target constraint plugin
// RFC: rfc/short/rfc4684.md
//
// Package bgp_rtc implements Route Target Constraint NLRI (RFC 4684, SAFI 132).
package rtc

import (
	"encoding/binary"
	"errors"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
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

// RouteTarget represents a Route Target extended community.
//
// RFC 4360 defines extended communities as 8-octet values.
type RouteTarget struct {
	Type  uint16  // Extended community type (2 bytes)
	Value [6]byte // Extended community value (6 bytes)
}

// Bytes returns the wire format of the route target (8 bytes).
func (rt RouteTarget) Bytes() []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint16(buf[:2], rt.Type)
	copy(buf[2:], rt.Value[:])
	return buf
}

// String returns a human-readable route target.
//
// RFC 4360 Section 3 defines extended community types.
func (rt RouteTarget) String() string {
	switch rt.Type >> 8 {
	case 0x00: // 2-byte ASN (RFC 4360 Section 3.1)
		asn := binary.BigEndian.Uint16(rt.Value[:2])
		assigned := binary.BigEndian.Uint32(rt.Value[2:6])
		return fmt.Sprintf("%d:%d", asn, assigned)
	case 0x01: // IPv4 address (RFC 4360 Section 3.2)
		ip := fmt.Sprintf("%d.%d.%d.%d", rt.Value[0], rt.Value[1], rt.Value[2], rt.Value[3])
		assigned := binary.BigEndian.Uint16(rt.Value[4:6])
		return fmt.Sprintf("%s:%d", ip, assigned)
	case 0x02: // 4-byte ASN (RFC 5668)
		asn := binary.BigEndian.Uint32(rt.Value[:4])
		assigned := binary.BigEndian.Uint16(rt.Value[4:6])
		return fmt.Sprintf("%d:%d", asn, assigned)
	default:
		return fmt.Sprintf("rt-type%d:%x", rt.Type, rt.Value)
	}
}

// RTC represents a Route Target Constraint NLRI (RFC 4684 Section 4).
//
// The RTC NLRI is used to advertise interest in receiving VPN routes
// with specific Route Targets.
type RTC struct {
	originAS    uint32      // Origin AS number (4 bytes)
	routeTarget RouteTarget // Route Target extended community (8 bytes)
}

// NewRTC creates a new RTC NLRI.
func NewRTC(originAS uint32, rt RouteTarget) *RTC {
	return &RTC{
		originAS:    originAS,
		routeTarget: rt,
	}
}

// ParseRTC parses an RTC NLRI from wire format.
//
// RFC 4684 Section 4: prefix of 0 to 96 bits.
// A prefix-length of 0 = default route target.
func ParseRTC(data []byte) (*RTC, []byte, error) {
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

// RouteTargetValue returns the route target.
func (r *RTC) RouteTargetValue() RouteTarget { return r.routeTarget }

// IsDefault returns true if this is the default RTC (matches all RTs).
//
// RFC 4684 Section 4: A zero-length prefix = default route target.
func (r *RTC) IsDefault() bool {
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
	if r.IsDefault() {
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
	if r.IsDefault() {
		return "default"
	}
	return fmt.Sprintf("origin-as %d rt %s", r.originAS, r.routeTarget)
}

// WriteTo writes the RTC NLRI directly to buf at offset.
func (r *RTC) WriteTo(buf []byte, off int) int {
	if r.IsDefault() {
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
