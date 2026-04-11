// Design: rfc/short/rfc5880.md -- BFD Control packet wire format (Section 4.1)
// Related: diag.go -- Diag and State constants
//
// Package packet implements the BFD Control packet codec.
//
// The 24-byte mandatory section is defined by RFC 5880 Section 4.1. An
// optional Authentication Section follows when the A bit is set; see auth.go.
//
// Encoding is buffer-first: WriteTo writes into a caller-provided buffer at
// a given offset and returns the number of bytes written. Decoding is zero
// allocation: ParseControl reads fields from a byte slice into a stack-local
// struct.
package packet

import (
	"encoding/binary"
	"errors"
)

// MandatoryLen is the fixed length of the BFD Control mandatory section.
const MandatoryLen = 24

// Version is the protocol version carried in every BFD Control packet.
// RFC 5880 Section 4.1 fixes the version number at 1.
const Version uint8 = 1

// Flag bits packed into byte 1 of the Control packet alongside State.
//
// Bit layout of byte 1 (MSB first):
//
//	7 6 | 5 4 3 2 1 0
//	Sta | P F C A D M
//
// See RFC 5880 Section 4.1.
const (
	FlagPoll       uint8 = 1 << 5 // P
	FlagFinal      uint8 = 1 << 4 // F
	FlagCPI        uint8 = 1 << 3 // C -- control plane independent
	FlagAuth       uint8 = 1 << 2 // A -- authentication section present
	FlagDemand     uint8 = 1 << 1 // D -- demand mode
	FlagMultipoint uint8 = 1 << 0 // M -- MUST be zero
)

// Control is the decoded representation of a BFD Control packet's mandatory
// section. It is a value type with no hidden allocations; passing Control
// by value across goroutines is safe.
type Control struct {
	Version                   uint8
	Diag                      Diag
	State                     State
	Poll                      bool
	Final                     bool
	CPI                       bool
	Auth                      bool
	Demand                    bool
	Multipoint                bool
	DetectMult                uint8
	Length                    uint8
	MyDiscriminator           uint32
	YourDiscriminator         uint32
	DesiredMinTxInterval      uint32 // microseconds
	RequiredMinRxInterval     uint32 // microseconds
	RequiredMinEchoRxInterval uint32 // microseconds, 0 = no echo
}

// Errors returned by ParseControl. Wire parsing MUST return an error rather
// than panic so fuzzing and untrusted input are safe.
var (
	ErrShortPacket      = errors.New("bfd: packet shorter than 24 bytes")
	ErrBadVersion       = errors.New("bfd: version is not 1")
	ErrLengthTooSmall   = errors.New("bfd: length field below minimum")
	ErrLengthOverBuffer = errors.New("bfd: length field exceeds buffer")
	ErrZeroDetectMult   = errors.New("bfd: detect multiplier is zero")
	ErrMultipointSet    = errors.New("bfd: multipoint bit set")
	ErrZeroMyDisc       = errors.New("bfd: my discriminator is zero")
	ErrShortBuffer      = errors.New("bfd: buffer too small for write")
)

// WriteTo serializes c into buf starting at off and returns the number of
// bytes written (always MandatoryLen).
//
// Callers MUST provide a buffer with at least MandatoryLen bytes available
// from off; this is the buffer-first contract used throughout ze. The pool
// buffers used by the engine are sized for the largest BFD packet (24 + 28
// for SHA1 auth = 52 bytes), so the contract is statically satisfied.
//
// WriteTo does NOT write the authentication section. Call AuthWriteTo (in
// auth.go) afterwards when the A bit is set, and update c.Length before
// transmission.
//
// Field values are taken from c verbatim. WriteTo does not validate the
// State, Diag, DetectMult, or Length fields; the caller is responsible for
// supplying RFC-legal values.
func (c Control) WriteTo(buf []byte, off int) int {
	// Byte 0: Version (3 bits) + Diag (5 bits)
	buf[off] = (c.Version&0x07)<<5 | (uint8(c.Diag) & 0x1F)

	// Byte 1: State (2 bits) + P F C A D M (6 flags)
	b1 := (uint8(c.State) & 0x03) << 6
	if c.Poll {
		b1 |= FlagPoll
	}
	if c.Final {
		b1 |= FlagFinal
	}
	if c.CPI {
		b1 |= FlagCPI
	}
	if c.Auth {
		b1 |= FlagAuth
	}
	if c.Demand {
		b1 |= FlagDemand
	}
	if c.Multipoint {
		b1 |= FlagMultipoint
	}
	buf[off+1] = b1

	buf[off+2] = c.DetectMult
	buf[off+3] = c.Length

	binary.BigEndian.PutUint32(buf[off+4:], c.MyDiscriminator)
	binary.BigEndian.PutUint32(buf[off+8:], c.YourDiscriminator)
	binary.BigEndian.PutUint32(buf[off+12:], c.DesiredMinTxInterval)
	binary.BigEndian.PutUint32(buf[off+16:], c.RequiredMinRxInterval)
	binary.BigEndian.PutUint32(buf[off+20:], c.RequiredMinEchoRxInterval)

	return MandatoryLen
}

// ParseControl parses a BFD Control packet from data. It implements the
// reception-check ordering of RFC 5880 Section 6.8.6: every structural
// error is reported before any semantic processing. Authentication of the
// packet body (A=1 case) is NOT performed; the caller is expected to drive
// authentication via the Control.Auth flag and the Authentication Section
// helpers in auth.go.
//
// ParseControl returns the parsed Control, the consumed mandatory-section
// length, and an error. The authentication section, if any, begins at byte
// MandatoryLen and runs to c.Length.
//
// ParseControl never allocates. The returned Control is a value.
func ParseControl(data []byte) (Control, int, error) {
	if len(data) < MandatoryLen {
		return Control{}, 0, ErrShortPacket
	}

	var c Control
	c.Version = data[0] >> 5
	c.Diag = Diag(data[0] & 0x1F)

	b1 := data[1]
	c.State = State(b1 >> 6)
	c.Poll = b1&FlagPoll != 0
	c.Final = b1&FlagFinal != 0
	c.CPI = b1&FlagCPI != 0
	c.Auth = b1&FlagAuth != 0
	c.Demand = b1&FlagDemand != 0
	c.Multipoint = b1&FlagMultipoint != 0

	c.DetectMult = data[2]
	c.Length = data[3]

	c.MyDiscriminator = binary.BigEndian.Uint32(data[4:])
	c.YourDiscriminator = binary.BigEndian.Uint32(data[8:])
	c.DesiredMinTxInterval = binary.BigEndian.Uint32(data[12:])
	c.RequiredMinRxInterval = binary.BigEndian.Uint32(data[16:])
	c.RequiredMinEchoRxInterval = binary.BigEndian.Uint32(data[20:])

	// RFC 5880 Section 6.8.6 ordering: version, length, detect mult,
	// multipoint, my discriminator.
	if c.Version != Version {
		return c, MandatoryLen, ErrBadVersion
	}
	minLen := uint8(MandatoryLen)
	if c.Auth {
		minLen = MandatoryLen + 2 // Auth Type + Auth Len
	}
	if c.Length < minLen {
		return c, MandatoryLen, ErrLengthTooSmall
	}
	if int(c.Length) > len(data) {
		return c, MandatoryLen, ErrLengthOverBuffer
	}
	if c.DetectMult == 0 {
		return c, MandatoryLen, ErrZeroDetectMult
	}
	if c.Multipoint {
		return c, MandatoryLen, ErrMultipointSet
	}
	if c.MyDiscriminator == 0 {
		return c, MandatoryLen, ErrZeroMyDisc
	}

	return c, MandatoryLen, nil
}
