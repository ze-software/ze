// Design: plan/learned/929-isis-3-l2-transport.md -- IEEE 802.3 + LLC frame codec
// Related: multicast.go -- destination MAC groups frames are addressed to
//
// IS-IS runs directly over IEEE 802.3 frames with an LLC header, NOT over an
// Ethernet II ethertype and NOT over IP. Each frame is:
//
//	dst MAC (6) | src MAC (6) | 802.3 length (2) | LLC (3) | IS-IS PDU
//
// The 2-byte field after the source MAC is the IEEE 802.3 LENGTH of the LLC
// header plus the PDU, NOT an ethertype. A receiver distinguishes 802.3 from
// Ethernet II by that field: values below 0x0600 are a length (802.3), values
// >= 0x0600 are an ethertype. Building this field as an ethertype is the classic
// IS-IS framing bug; this codec makes the length explicit and rejects ethertype
// values on receive.
//
// The LLC header (ISO/IEC 10589) is DSAP 0xFE, SSAP 0xFE, control 0x03
// (unnumbered information). 0xFE is the IANA/ISO network-layer SAP for ISO CLNS,
// which carries IS-IS.
//
// This codec adds ONLY framing. It does NOT pad the PDU: padding (TLV 8) is part
// of the PDU and is added by the engine (isis-5) before authentication, per the
// umbrella "Final PDU bytes" contract. BuildFrame writes the caller's PDU bytes
// verbatim.

package transport

import "errors"

// LLC header constants (ISO/IEC 10589). IS-IS uses LLC SAP 0xFE (ISO CLNS) with
// the unnumbered-information control byte 0x03.
const (
	LLCSAP     byte = 0xFE // DSAP and SSAP for ISO CLNS / IS-IS
	LLCControl byte = 0x03 // unnumbered information (UI) frame
)

// Frame layout constants.
const (
	// LLCHeaderLen is the LLC header size: DSAP(1) + SSAP(1) + control(1).
	LLCHeaderLen = 3
	// LengthFieldLen is the IEEE 802.3 length field size.
	LengthFieldLen = 2
	// FrameHeaderLen is everything before the PDU: dst(6) + src(6) +
	// 802.3 length(2) + LLC(3) = 17 octets.
	FrameHeaderLen = 2*MACLen + LengthFieldLen + LLCHeaderLen
	// EthertypeThreshold is the IEEE 802.3 boundary: a length-or-type field
	// below this value is an 802.3 length; at or above it is an Ethernet II
	// ethertype. IS-IS frames MUST keep the field below this.
	EthertypeThreshold = 0x0600
	// MaxLLCAndPDU is the largest LLC+PDU length expressible as a valid 802.3
	// length field (one below the ethertype threshold).
	MaxLLCAndPDU = EthertypeThreshold - 1
)

// Frame codec errors.
var (
	// ErrBufferTooShort is returned by BuildFrame when the destination buffer
	// cannot hold the framed PDU.
	ErrBufferTooShort = errors.New("isis/transport: buffer too short for frame")
	// ErrPDUTooLarge is returned by BuildFrame when LLC+PDU would reach the
	// ethertype threshold and could not be expressed as an 802.3 length.
	ErrPDUTooLarge = errors.New("isis/transport: PDU too large for 802.3 length field")
	// ErrFrameTooShort is returned by ParseFrame when the captured frame is
	// shorter than the minimum 802.3+LLC header.
	ErrFrameTooShort = errors.New("isis/transport: frame shorter than 802.3+LLC header")
	// ErrNotISO is returned by ParseFrame when the length-or-type field is an
	// Ethernet II ethertype (>= 0x0600) rather than an 802.3 length.
	ErrNotISO = errors.New("isis/transport: length field is an ethertype, not an 802.3 length")
	// ErrBadLength is returned when the declared 802.3 length is below the LLC
	// header size or overruns the captured bytes.
	ErrBadLength = errors.New("isis/transport: 802.3 length out of bounds")
	// ErrBadLLC is returned when the LLC DSAP/SSAP/control bytes are not the
	// ISO CLNS values (0xFE/0xFE/0x03).
	ErrBadLLC = errors.New("isis/transport: not an ISO CLNS LLC header (DSAP/SSAP/control != 0xFE/0xFE/0x03)")
)

// BuildFrame writes an 802.3 + LLC frame for pdu into buf and returns the number
// of bytes written. Buffer-first per ai/rules/performance.md: the caller owns
// buf and must size it to at least FrameHeaderLen+len(pdu). The PDU is copied in
// verbatim -- the transport adds only framing and MUST NOT pad or alter it
// (umbrella "Final PDU bytes" contract).
//
// ISO/IEC 10589: the 2-byte field is the LENGTH of LLC+PDU (not an ethertype),
// followed by the LLC header DSAP 0xFE / SSAP 0xFE / control 0x03.
func BuildFrame(buf []byte, dst, src [MACLen]byte, pdu []byte) (int, error) {
	total := FrameHeaderLen + len(pdu)
	if len(buf) < total {
		return 0, ErrBufferTooShort
	}
	llcAndPDU := LLCHeaderLen + len(pdu)
	// The 802.3 length field MUST stay below the ethertype threshold so the
	// frame is parsed as 802.3 and not Ethernet II.
	if llcAndPDU > MaxLLCAndPDU {
		return 0, ErrPDUTooLarge
	}

	copy(buf[0:MACLen], dst[:])
	copy(buf[MACLen:2*MACLen], src[:])
	buf[2*MACLen] = byte(llcAndPDU >> 8)
	buf[2*MACLen+1] = byte(llcAndPDU)
	buf[2*MACLen+2] = LLCSAP     // DSAP
	buf[2*MACLen+3] = LLCSAP     // SSAP
	buf[2*MACLen+4] = LLCControl // control (UI)
	copy(buf[FrameHeaderLen:], pdu)

	return total, nil
}

// Frame is a parsed 802.3 + LLC IS-IS frame. PDU is a ZERO-COPY view into the
// input buffer passed to ParseFrame; the caller must not retain it past the
// lifetime of that buffer (the receive path copies before queueing).
type Frame struct {
	DstMAC [MACLen]byte
	SrcMAC [MACLen]byte
	PDU    []byte // zero-copy view; LLC stripped, no padding added
}

// ParseFrame validates and strips the 802.3 + LLC header from a captured frame
// and returns the inner IS-IS PDU as a zero-copy view. It is the receive-path
// gate: every field is validated before any slice into the PDU, so a crafted or
// truncated frame is rejected rather than over-reading the buffer.
//
// ISO/IEC 10589: the field after the source MAC is an 802.3 length (< 0x0600);
// an ethertype (>= 0x0600) is rejected. The LLC header MUST be 0xFE/0xFE/0x03.
func ParseFrame(frame []byte) (Frame, error) {
	if len(frame) < FrameHeaderLen {
		return Frame{}, ErrFrameTooShort
	}

	declared := int(frame[2*MACLen])<<8 | int(frame[2*MACLen+1])
	// Reject Ethernet II: the field is an ethertype, not an 802.3 length.
	if declared >= EthertypeThreshold {
		return Frame{}, ErrNotISO
	}
	// The declared length covers LLC(3)+PDU; it must leave room for the LLC
	// header and must not overrun the bytes actually captured.
	if declared < LLCHeaderLen {
		return Frame{}, ErrBadLength
	}
	if FrameHeaderLen-LLCHeaderLen+declared > len(frame) {
		return Frame{}, ErrBadLength
	}

	// LLC header validation (ISO CLNS: DSAP 0xFE, SSAP 0xFE, control 0x03).
	if frame[2*MACLen+2] != LLCSAP || frame[2*MACLen+3] != LLCSAP || frame[2*MACLen+4] != LLCControl {
		return Frame{}, ErrBadLLC
	}

	pduLen := declared - LLCHeaderLen
	var f Frame
	copy(f.DstMAC[:], frame[0:MACLen])
	copy(f.SrcMAC[:], frame[MACLen:2*MACLen])
	f.PDU = frame[FrameHeaderLen : FrameHeaderLen+pduLen]
	return f, nil
}
