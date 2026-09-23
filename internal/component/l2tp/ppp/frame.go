// Design: docs/research/l2tpv2-implementation-guide.md -- PPP frame format on /dev/ppp

package ppp

import (
	"encoding/binary"
	"errors"
)

// PPP protocol field values used by the control plane. Data plane
// protocols (0x0021 IPv4, 0x0057 IPv6) are kernel-handled and never
// reach userspace via /dev/ppp.
//
// RFC 1661 Section 2: Protocol field is two octets; protocols below
// 0x4000 are network-layer; protocols 0xC000-0xFFFF are link-layer
// control. RFC 1700 (PPP DLL Protocol Numbers) is the registry.
const (
	ProtoLCP    uint16 = 0xC021 // RFC 1661
	ProtoPAP    uint16 = 0xC023 // RFC 1334
	ProtoCHAP   uint16 = 0xC223 // RFC 1994
	ProtoIPCP   uint16 = 0x8021 // RFC 1332
	ProtoIPv6CP uint16 = 0x8057 // RFC 5072
	ProtoCCP    uint16 = 0x80FD // RFC 1962 (out of umbrella scope)
	ProtoIPv4   uint16 = 0x0021 // kernel-handled
	ProtoIPv6   uint16 = 0x0057 // kernel-handled
)

// MaxFrameLen is the largest Information field ze accepts or produces:
// the LCP default MRU of 1500 octets, RFC 1661 Section 6.1. It bounds the
// Length of every packet parser in this package.
//
// MaxFrameBufLen bounds a whole frame as it arrives from the chan fd: the
// Protocol field plus a full MaxFrameLen Information field. RFC 1661
// Section 6.1: "If smaller packets are requested, an implementation MUST
// still be able to receive the full 1500 octet information field in case
// link synchronization is lost." So the read buffer and ParseFrame are
// sized to the full field whatever MRU was negotiated, and the two octets
// of Protocol field are counted on top of it, never inside it.
const (
	ProtoFieldLen  = 2
	MinFrameLen    = 64 + ProtoFieldLen // MRU floor + protocol field
	MaxFrameLen    = 1500
	MaxFrameBufLen = ProtoFieldLen + MaxFrameLen
)

// errFrameTooShort is returned when a buffer is smaller than the
// two-byte protocol field minimum.
var errFrameTooShort = errors.New("ppp: frame too short for two-byte protocol field")

// errFrameTooLong is returned when a buffer exceeds MaxFrameBufLen.
var errFrameTooLong = errors.New("ppp: information field exceeds 1500 octets")

// ParseFrame extracts the protocol field and returns the payload slice
// (a sub-slice of buf, no copy).
//
// 6a accepts ONLY the two-byte (uncompressed) protocol form. PFC
// (Protocol-Field-Compression, RFC 1661 §6.5) is intentionally NOT
// auto-detected on receive even when the LCP option is negotiated:
// every protocol the userspace control plane handles (LCP 0xC021,
// PAP 0xC023, CHAP 0xC223, IPCP 0x8021, IPv6CP 0x8057) has a
// non-zero high byte and is NEVER eligible for PFC. Data plane
// protocols that ARE PFC-eligible (IPv4 0x0021, IPv6 0x0057) never
// reach userspace -- the kernel intercepts them on the unit fd. So
// PFC parsing would only encounter buggy peers; rejecting cleanly
// is safer than guessing.
//
// Returns the protocol value, the payload sub-slice, and the number
// of header bytes consumed (always 2).
func ParseFrame(buf []byte) (proto uint16, payload []byte, headerLen int, err error) {
	if len(buf) < 2 {
		return 0, nil, 0, errFrameTooShort
	}
	// RFC 1661 Section 6.1: "an implementation MUST still be able to
	// receive the full 1500 octet information field", so the bound is the
	// Protocol field plus that field, not the MRU alone.
	if len(buf) > MaxFrameBufLen {
		return 0, nil, 0, errFrameTooLong
	}
	return binary.BigEndian.Uint16(buf[:2]), buf[2:], 2, nil
}

// WriteFrame encodes a PPP frame with the given protocol and payload
// at buf[off:] using the two-byte (uncompressed) protocol form.
// Returns the number of bytes written. The caller MUST ensure
// buf[off:] has cap >= 2 + len(payload).
//
// 6a does NOT emit PFC-compressed frames even when negotiated. PFC
// support on transmit is a future improvement; receivers handle both
// forms via ParseFrame.
//
// RFC 1661 Section 6.5: LCP packets MUST be sent with the two-byte
// protocol field; this implementation extends that conservatism to
// every protocol it emits.
func WriteFrame(buf []byte, off int, proto uint16, payload []byte) int {
	binary.BigEndian.PutUint16(buf[off:], proto)
	n := copy(buf[off+2:], payload)
	return 2 + n
}

// frameLen returns the wire size of a frame with the given payload
// length, using the two-byte protocol form WriteFrame produces.
func frameLen(payloadLen int) int {
	return 2 + payloadLen
}
