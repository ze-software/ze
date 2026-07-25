// Design: plan/learned/928-isis-2-wire.md -- common 8-byte header, PDU type constants, dispatch
// ISO/IEC 10589 clause 9: common header and PDU-specific headers.

package packet

import (
	"errors"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// PDUType is the 1-octet PDU type field of the IS-IS common header. Only the
// low 5 bits are the type code (ISO/IEC 10589 clause 9.5); the high 3 bits are
// reserved and MUST be zero on transmit.
type PDUType uint8

// PDU type code octets (ISO/IEC 10589 clause 9).
//
// These are the AUTHORITATIVE values from ISO/IEC 10589 clause 9 and the
// umbrella contract. The research guide (docs/research/isis-implementation-guide.md
// sec 2) transcribes the L1 codes incorrectly (it lists L1 LSP 0x18, L1 CSNP
// 0x24, L1 PSNP 0x26); those are typos. TestISISPDUConstants pins these exact
// values so a regression cannot silently break interop (spec A-5).
const (
	PDUTypeL1LANHello PDUType = 0x0f // 15: Level 1 LAN IS-IS Hello
	PDUTypeL2LANHello PDUType = 0x10 // 16: Level 2 LAN IS-IS Hello
	PDUTypeP2PHello   PDUType = 0x11 // 17: Point-to-Point IS-IS Hello
	PDUTypeL1LSP      PDUType = 0x12 // 18: Level 1 Link State PDU
	PDUTypeL2LSP      PDUType = 0x14 // 20: Level 2 Link State PDU
	PDUTypeL1CSNP     PDUType = 0x18 // 24: Level 1 Complete Sequence Numbers PDU
	PDUTypeL2CSNP     PDUType = 0x19 // 25: Level 2 Complete Sequence Numbers PDU
	PDUTypeL1PSNP     PDUType = 0x1a // 26: Level 1 Partial Sequence Numbers PDU
	PDUTypeL2PSNP     PDUType = 0x1b // 27: Level 2 Partial Sequence Numbers PDU
)

// pduTypeMask isolates the 5-bit PDU type code from the type octet
// (ISO/IEC 10589 clause 9.5: the top three bits are reserved).
const pduTypeMask = 0x1f

// Common-header constants (ISO/IEC 10589 clause 9.5/9.6/9.7/9.8).
const (
	// CommonHeaderLen is the fixed length of the IS-IS common header in octets.
	CommonHeaderLen = 8

	// ProtocolDiscriminator is the Intra-domain Routeing Protocol Discriminator
	// (NLPID 0x83 for IS-IS) carried in octet 0 of every PDU.
	ProtocolDiscriminator = 0x83

	// VersionProtocolIDExtension is the "version/protocol ID extension" octet
	// (octet 2); always 1.
	VersionProtocolIDExtension = 0x01

	// Version is the "version" octet (octet 5); always 1.
	Version = 0x01

	// IDLength is the System ID length advertised in the common header (octet
	// 3). Ze fixes the System ID at 6 octets; on the wire the value 0 also
	// means "6 octets" (ISO/IEC 10589 clause 9.5), so both 0 and 6 are accepted
	// on receive and 6 is sent.
	IDLength = types.SystemIDLen
)

// Common-header field offsets within the 8-octet header.
const (
	offDiscriminator   = 0 // Intradomain Routeing Protocol Discriminator (0x83)
	offLengthIndicator = 1 // Length Indicator (length of the fixed header for this PDU)
	offVersionProtoExt = 2 // Version/Protocol ID Extension (0x01)
	offIDLength        = 3 // ID Length
	offPDUType         = 4 // PDU Type (low 5 bits)
	offVersion         = 5 // Version (0x01)
	offReserved        = 6 // Reserved (0x00)
	offMaxAreaAddr     = 7 // Maximum Area Addresses
)

// Errors returned by the codec. They are typed sentinels so callers can match
// without parsing strings, and they never echo attacker-controlled bytes
// (security review: error leakage).
var (
	// ErrShortBuffer reports a buffer too short to hold the structure being
	// decoded or encoded.
	ErrShortBuffer = errors.New("isis packet: buffer too short")
	// ErrBadDiscriminator reports a common header whose protocol discriminator
	// is not 0x83.
	ErrBadDiscriminator = errors.New("isis packet: bad protocol discriminator")
	// ErrBadVersion reports a common header whose version octets are not 1.
	ErrBadVersion = errors.New("isis packet: unsupported version")
	// ErrBadIDLength reports a common header whose ID length is neither 0 nor 6
	// (Ze fixes the System ID at 6 octets).
	ErrBadIDLength = errors.New("isis packet: unsupported ID length")
	// ErrUnknownPDUType reports a PDU type code that is not one of the 9 known
	// types.
	ErrUnknownPDUType = errors.New("isis packet: unknown PDU type")
	// ErrTruncated reports a PDU body shorter than its fixed header requires.
	ErrTruncated = errors.New("isis packet: truncated PDU")
	// ErrLength reports a length field that is out of range (e.g. a TLV value
	// longer than 255 octets, or a sub-TLV block that overflows its TLV).
	ErrLength = errors.New("isis packet: invalid length")
)

// Header is the parsed common 8-octet IS-IS header (ISO/IEC 10589 clause 9.5).
// It is a small value copied out of the wire bytes; the body parsers take the
// remaining slice.
type Header struct {
	// LengthIndicator is the length of this PDU's fixed header (common header
	// plus the PDU-specific fixed fields), as carried in octet 1.
	LengthIndicator uint8
	// IDLength is the System ID length octet as received (0 or 6).
	IDLength uint8
	// PDUType is the 5-bit PDU type code.
	PDUType PDUType
	// MaxAreaAddresses is the maximum number of area addresses (octet 7); 0
	// means the default of 3 (ISO/IEC 10589 clause 9.5).
	MaxAreaAddresses uint8
}

// Level reports the IS-IS level (1 or 2) implied by a PDU type, and ok=false
// for the P2P Hello (which is level-agnostic; its circuit type carries the
// level) or an unknown type.
func (t PDUType) Level() (level uint8, ok bool) {
	switch t {
	case PDUTypeL1LANHello, PDUTypeL1LSP, PDUTypeL1CSNP, PDUTypeL1PSNP:
		return 1, true
	case PDUTypeL2LANHello, PDUTypeL2LSP, PDUTypeL2CSNP, PDUTypeL2PSNP:
		return 2, true
	default:
		return 0, false
	}
}

// known reports whether t is one of the 9 defined PDU type codes.
func (t PDUType) known() bool {
	switch t {
	case PDUTypeL1LANHello, PDUTypeL2LANHello, PDUTypeP2PHello,
		PDUTypeL1LSP, PDUTypeL2LSP,
		PDUTypeL1CSNP, PDUTypeL2CSNP,
		PDUTypeL1PSNP, PDUTypeL2PSNP:
		return true
	default:
		return false
	}
}

// String renders the PDU type as a stable short token (for CLI/JSON). Cold
// path; the small switch returns interned literals (no allocation).
func (t PDUType) String() string {
	switch t {
	case PDUTypeL1LANHello:
		return "l1-lan-hello"
	case PDUTypeL2LANHello:
		return "l2-lan-hello"
	case PDUTypeP2PHello:
		return "p2p-hello"
	case PDUTypeL1LSP:
		return "l1-lsp"
	case PDUTypeL2LSP:
		return "l2-lsp"
	case PDUTypeL1CSNP:
		return "l1-csnp"
	case PDUTypeL2CSNP:
		return "l2-csnp"
	case PDUTypeL1PSNP:
		return "l1-psnp"
	case PDUTypeL2PSNP:
		return "l2-psnp"
	default:
		return "unknown"
	}
}

// DecodeHeader parses and validates the common 8-octet header at the start of
// buf. It is bound-checked before every read (security review: input
// validation) and rejects a bad discriminator, version, ID length, or unknown
// PDU type. On success it returns the parsed header and the offset at which the
// PDU-specific body begins (always CommonHeaderLen).
//
// ISO/IEC 10589 clause 9.5: "Intradomain Routeing Protocol Discriminator" =
// 0x83, "Version/Protocol ID Extension" = 1, "Version" = 1.
func DecodeHeader(buf []byte) (Header, int, error) {
	if len(buf) < CommonHeaderLen {
		return Header{}, 0, ErrShortBuffer
	}
	if buf[offDiscriminator] != ProtocolDiscriminator {
		return Header{}, 0, ErrBadDiscriminator
	}
	if buf[offVersionProtoExt] != VersionProtocolIDExtension || buf[offVersion] != Version {
		return Header{}, 0, ErrBadVersion
	}
	idLen := buf[offIDLength]
	// ID length 0 is the on-wire shorthand for the default 6 (ISO/IEC 10589
	// clause 9.5); Ze fixes System IDs at 6 octets, so only 0 and 6 are valid.
	if idLen != 0 && idLen != types.SystemIDLen {
		return Header{}, 0, ErrBadIDLength
	}
	pt := PDUType(buf[offPDUType] & pduTypeMask)
	if !pt.known() {
		return Header{}, 0, ErrUnknownPDUType
	}
	h := Header{
		LengthIndicator:  buf[offLengthIndicator],
		IDLength:         idLen,
		PDUType:          pt,
		MaxAreaAddresses: buf[offMaxAreaAddr],
	}
	return h, CommonHeaderLen, nil
}

// writeCommonHeader writes the 8-octet common header into buf at off and
// returns off+CommonHeaderLen. lengthIndicator is the length of this PDU's
// fixed header (common + PDU-specific fixed fields). Buffer-first; the caller
// guarantees room. maxAreaAddr 0 means the default 3.
func writeCommonHeader(buf []byte, off int, pt PDUType, lengthIndicator, maxAreaAddr uint8) int {
	buf[off+offDiscriminator] = ProtocolDiscriminator
	buf[off+offLengthIndicator] = lengthIndicator
	buf[off+offVersionProtoExt] = VersionProtocolIDExtension
	buf[off+offIDLength] = IDLength
	buf[off+offPDUType] = byte(pt) & pduTypeMask
	buf[off+offVersion] = Version
	buf[off+offReserved] = 0x00
	buf[off+offMaxAreaAddr] = maxAreaAddr
	return off + CommonHeaderLen
}
