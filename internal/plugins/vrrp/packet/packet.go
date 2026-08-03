// RFC: rfc/short/rfc9568.md -- VRRPv3 Advertisement wire format (Section 5.1-5.2)
// RFC: rfc/short/rfc3768.md -- VRRPv2 Advertisement wire format (Section 5.1-5.3)
// Design: ai/rules/performance.md -- WriteTo(buf, off) int contract
//
// Package packet implements the VRRP Advertisement codec for VRRPv3 (RFC 9568,
// IPv4 + IPv6) and VRRPv2 (RFC 3768, IPv4 only). It has no sockets, no build
// tags, and no goroutines: the FSM (spec-vrrp-2) fills an Advertisement and
// calls WriteTo/FillChecksum; the engine (spec-vrrp-5) calls Decode; the
// transport (spec-vrrp-4) calls StripIPv4Header. The single internal time unit
// is MILLISECONDS (umbrella R-2); conversion to wire units (v3 centiseconds,
// v2 whole seconds) happens ONLY in this package's four conversion helpers.
//
// checksum.go holds the RFC 1071 core, the pseudo-header builders, FillChecksum
// and the rx dual-accept; validate.go holds RxMeta, Decode with the 13-row
// receive-validation ladder, the typed error sentinels, Reason() and
// StripIPv4Header.
//
// Encoding is buffer-first: WriteTo writes into a caller-provided buffer at an
// offset and returns the number of bytes written. Decoding is zero allocation:
// Decode reads fields into a value struct and exposes the Virtual IP addresses
// lazily as a view over the receive buffer (see the AppendVIPs copy helper for
// callers that persist past the buffer's lifetime).
package packet

import (
	"errors"
	"net/netip"
)

// Protocol constants. These are defined locally in the vrrp-owned package and
// are NOT added to any central registry (ai/rules/plugins.md).
const (
	// ProtoNumber is the IP protocol / IPv6 Next Header value for VRRP.
	// RFC 9568 Section 5.1: "Protocol / Next Header | 112".
	ProtoNumber uint8 = 112

	// VersionV2 / VersionV3 are the values carried in the high nibble of byte 0.
	VersionV2 uint8 = 2 // RFC 3768 Section 5.3.1
	VersionV3 uint8 = 3 // RFC 9568 Section 5.2.1

	// TypeAdvertisement is the only defined VRRP packet type (low nibble of
	// byte 0). RFC 9568 Section 5.2.2 / RFC 3768 Section 5.3.2.
	TypeAdvertisement uint8 = 1

	// HeaderLen is the fixed VRRP header length before the address list, for
	// both versions. RFC 9568 Section 5.2 / RFC 3768 Section 5.3.
	HeaderLen = 8

	// v2AuthTrailerLen is the mandatory 8-byte Authentication Data trailer that
	// VRRPv2 always carries. RFC 3768 Section 5.3.10.
	v2AuthTrailerLen = 8

	// MaxVIPs is the configured maximum number of Virtual IP addresses per
	// group (umbrella boundary table, max-elements 16).
	MaxVIPs = 16
)

// Address family selectors. Family is a plain uint8 so callers can copy an
// Advertisement by value with no hidden state.
const (
	V4 uint8 = 4
	V6 uint8 = 6
)

// Maximum encoded lengths for a 16-VIP advertisement. Exported so the transport
// (spec-vrrp-4) can size its fixed per-instance tx buffer.
//
//	MaxLenV2   = 8 + 16*4 + 8 = 80
//	MaxLenV3v4 = 8 + 16*4     = 72
//	MaxLenV3v6 = 8 + 16*16    = 264
const (
	MaxLenV2   = HeaderLen + MaxVIPs*4 + v2AuthTrailerLen
	MaxLenV3v4 = HeaderLen + MaxVIPs*4
	MaxLenV3v6 = HeaderLen + MaxVIPs*16
)

// Virtual Router MAC address prefixes. RFC 9568 Section 7.3 / RFC 3768
// Section 7.3: the first five octets are the IANA OUI plus the VRRP IPv4
// (00-00-5E-00-01) or IPv6 (00-00-5E-00-02) block; the last octet is the VRID.
// Consumed by the transport / plugin (children 4/5); the pure FSM never touches
// MACs.
var (
	VirtualMACPrefixV4 = [5]byte{0x00, 0x00, 0x5e, 0x00, 0x01}
	VirtualMACPrefixV6 = [5]byte{0x00, 0x00, 0x5e, 0x00, 0x02}
)

// Link-local multicast destinations for VRRP advertisements.
// RFC 9568 Section 5.1 / RFC 3768 Section 5.2.2.
var (
	MulticastV4 = netip.AddrFrom4([4]byte{224, 0, 0, 18})
	MulticastV6 = netip.AddrFrom16([16]byte{0xff, 0x02, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x12})
)

// VirtualMAC returns the Virtual Router MAC address for the given family and
// VRID. RFC 9568 Section 7.3: 00-00-5E-00-01-{VRID} (IPv4) /
// 00-00-5E-00-02-{VRID} (IPv6).
func VirtualMAC(family, vrid uint8) [6]byte {
	prefix := VirtualMACPrefixV4
	if family == V6 {
		prefix = VirtualMACPrefixV6
	}
	var mac [6]byte
	copy(mac[:5], prefix[:])
	mac[5] = vrid
	return mac
}

// Advertisement is the decoded/encodable representation of a VRRP ADVERTISEMENT
// message. One struct serves both encode and decode. It is a value type; the
// only reference field is wireVIPs, a view into the receive buffer that is
// valid ONLY until the next socket read (see AppendVIPs).
type Advertisement struct {
	Version         uint8        // 2 or 3
	Family          uint8        // V4 or V6
	VRID            uint8        // 1..255
	Priority        uint8        // 0 resign, 1..254 backup, 255 owner
	Count           uint8        // number of Virtual IP addresses (decode output)
	AdverIntervalMS uint32       // MILLISECONDS ALWAYS (umbrella R-2)
	VIPs            []netip.Addr // encode source; nil on decode
	// MsgOnlyChecksum is set on decode when a v3/IPv4 packet matched only the
	// RFC 9568 message-only checksum, not the RFC 5798 pseudo-header form ze and
	// the deployed base send. Accepted either way; the flag exists so the engine
	// can count strict-RFC-9568 peers (ReasonMsgOnlyChecksum).
	MsgOnlyChecksum bool

	// wireVIPs is the decode-side lazy view into the receive buffer. It is a
	// sub-slice of the payload and is valid only until the next socket read
	// (A-3). Callers that persist VIPs beyond that MUST AppendVIPs-copy first.
	wireVIPs []byte
}

// addrSize returns the on-wire size of one Virtual IP address for a.Family.
func (a Advertisement) addrSize() int {
	if a.Family == V6 {
		return 16
	}
	return 4
}

// VIPCount returns the number of Virtual IP addresses. On the decode path it is
// derived from the lazy wire view; on the encode path from the VIPs slice.
func (a Advertisement) VIPCount() int {
	if a.wireVIPs != nil {
		return len(a.wireVIPs) / a.addrSize()
	}
	return len(a.VIPs)
}

// VIPAt returns the i-th Virtual IP address (bounds-checked; the zero Addr for
// an out-of-range index). Allocation-free: it copies into a fixed-size array
// and uses netip's value constructors (A-2).
func (a Advertisement) VIPAt(i int) netip.Addr {
	if i < 0 {
		return netip.Addr{}
	}
	if a.wireVIPs != nil {
		sz := a.addrSize()
		off := i * sz
		if off+sz > len(a.wireVIPs) {
			return netip.Addr{}
		}
		if a.Family == V6 {
			var b [16]byte
			copy(b[:], a.wireVIPs[off:off+16])
			return netip.AddrFrom16(b)
		}
		var b [4]byte
		copy(b[:], a.wireVIPs[off:off+4])
		return netip.AddrFrom4(b)
	}
	if i >= len(a.VIPs) {
		return netip.Addr{}
	}
	return a.VIPs[i]
}

// AppendVIPs appends every Virtual IP address to dst and returns the extended
// slice. This is the explicit copy callers use before persisting VIPs past the
// receive buffer's lifetime (A-3).
func (a Advertisement) AppendVIPs(dst []netip.Addr) []netip.Addr {
	n := a.VIPCount()
	for i := range n {
		dst = append(dst, a.VIPAt(i))
	}
	return dst
}

// Encode-side validation errors (defense-in-depth against engine bugs, A-4).
// These are NOT part of the receive reason taxonomy: Reason() does not map them
// (they can never label a received packet).
var (
	ErrVRIDRange     = errors.New("vrrp: vrid out of range 1..255")
	ErrCountRange    = errors.New("vrrp: vip count out of range 1..16")
	ErrIntervalRange = errors.New("vrrp: advertise interval out of wire range")
)

// Validate checks that an Advertisement's fields are encodable for its version
// before WriteTo. The engine (spec-vrrp-5) verifies these at config time; this
// method is defense-in-depth (A-4) and is exercised by the encode boundary
// tests. WriteTo assumes a validated Advertisement.
func (a Advertisement) Validate() error {
	if a.VRID < 1 {
		return ErrVRIDRange
	}
	if len(a.VIPs) < 1 || len(a.VIPs) > MaxVIPs {
		return ErrCountRange
	}
	if a.Version == VersionV2 {
		// RFC 3768 Section 5.3.7: Adver Int is whole seconds 1..255.
		if a.AdverIntervalMS < 1000 || a.AdverIntervalMS > 255000 || a.AdverIntervalMS%1000 != 0 {
			return ErrIntervalRange
		}
		return nil
	}
	// RFC 9568 Section 6.1 / erratum 8301: Advertisement_Interval is
	// centiseconds 1..4095 -> milliseconds 10..40950, multiples of 10.
	if a.AdverIntervalMS < 10 || a.AdverIntervalMS > 40950 || a.AdverIntervalMS%10 != 0 {
		return ErrIntervalRange
	}
	return nil
}

// -----------------------------------------------------------------------------
// Unit conversion helpers -- the ONLY places wire units meet milliseconds
// (umbrella R-2). Keeping them isolated kills the s/cs/ms confusion class.
// -----------------------------------------------------------------------------

// msToV3Centiseconds converts milliseconds to the 12-bit centisecond wire unit.
// RFC 9568 Section 5.2.7: Max Advertise Interval is in centiseconds.
func msToV3Centiseconds(ms uint32) uint16 { return uint16(ms / 10) }

// v3CentisecondsToMS converts the 12-bit centisecond wire unit to milliseconds.
func v3CentisecondsToMS(cs uint16) uint32 { return uint32(cs) * 10 }

// msToV2Seconds converts milliseconds to the 8-bit whole-second wire unit.
// RFC 3768 Section 5.3.7: Adver Int is in whole seconds.
func msToV2Seconds(ms uint32) uint8 { return uint8(ms / 1000) }

// v2SecondsToMS converts the 8-bit whole-second wire unit to milliseconds.
func v2SecondsToMS(sec uint8) uint32 { return uint32(sec) * 1000 }

// WriteTo serializes a into buf starting at off with the Checksum field ZERO,
// and returns the number of bytes written. Call FillChecksum afterwards to
// backfill the checksum (skip-and-backfill, ai/rules/performance.md).
//
// The caller MUST provide a buffer with at least the version-specific maximum
// (MaxLenV2 / MaxLenV3v4 / MaxLenV3v6) bytes from off; WriteTo indexes directly
// and panics on a short buffer, like the rest of ze's buffer-first code.
// WriteTo assumes a validated Advertisement (see Validate); the Count byte is
// derived from len(VIPs) and unit conversion happens in the isolated helpers.
func (a Advertisement) WriteTo(buf []byte, off int) int {
	count := uint8(len(a.VIPs))

	// Byte 0: Version (high nibble) | Type (low nibble).
	buf[off] = (a.Version << 4) | TypeAdvertisement
	buf[off+1] = a.VRID
	buf[off+2] = a.Priority
	buf[off+3] = count

	if a.Version == VersionV2 {
		// RFC 3768 Section 5.3.6: Auth Type; only 0 (No Authentication) is sent.
		buf[off+4] = 0
		buf[off+5] = msToV2Seconds(a.AdverIntervalMS)
	} else {
		cs := msToV3Centiseconds(a.AdverIntervalMS)
		// RFC 9568 Section 5.2.6: "The Reserve field MUST be set to zero on
		// transmission"; the 12-bit Max Advertise Interval fills the low 12
		// bits of bytes 4-5.
		buf[off+4] = byte(cs>>8) & 0x0F
		buf[off+5] = byte(cs)
	}

	// Checksum field zeroed here; FillChecksum backfills off+6..7.
	buf[off+6] = 0
	buf[off+7] = 0

	o := off + HeaderLen
	for _, vip := range a.VIPs {
		if a.Family == V6 {
			b := vip.As16()
			copy(buf[o:o+16], b[:])
			o += 16
		} else {
			b := vip.As4()
			copy(buf[o:o+4], b[:])
			o += 4
		}
	}

	if a.Version == VersionV2 {
		// RFC 3768 Section 5.3.10: 8-byte Authentication Data trailer, zeroed
		// on transmission.
		clear(buf[o : o+v2AuthTrailerLen])
		o += v2AuthTrailerLen
	}

	return o - off
}
