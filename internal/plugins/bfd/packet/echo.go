// Design: rfc/short/rfc5880.md -- Echo Function (Section 6.4)
//
// BFD Echo packet codec. RFC 5880 Section 6.4 leaves the echo
// format "a local matter"; the only requirement is that the sender
// can match a returning echo to the one it sent and compute the
// round-trip time. ze picks a compact 16-byte envelope:
//
//	0..3:   Magic "ZEEC"
//	4..7:   Local Discriminator (big-endian)
//	8..11:  Sequence Number (big-endian, monotonic per session)
//	12..15: Timestamp (big-endian uint32 millisecond-granularity
//	        of a process-local monotonic clock, for RTT math)
//
// The magic prefix is deliberately NOT a valid BFD Control version
// (1 in the top 3 bits of byte 0 would be 0x20; "ZEEC" starts with
// 0x5A which has the top bits 010, so a peer that accidentally
// treats an echo as a Control gets immediate ErrBadVersion). The
// 16-byte payload sits inside the UDP 3785 datagram; reflected
// packets are bit-exact copies of the original.
package packet

import (
	"encoding/binary"
	"errors"
)

// EchoLen is the fixed length of a ze BFD echo packet.
const EchoLen = 16

// echoMagic is the 4-byte prefix on every echo packet.
var echoMagic = [4]byte{'Z', 'E', 'E', 'C'}

// Echo is the decoded representation of a ze BFD echo packet.
type Echo struct {
	LocalDiscriminator uint32
	Sequence           uint32
	TimestampMs        uint32
}

// ErrShortEcho is returned when an echo buffer is smaller than the
// fixed envelope length.
var ErrShortEcho = errors.New("bfd: echo packet shorter than 16 bytes")

// ErrBadEchoMagic is returned when the magic prefix does not match.
// Callers use this to distinguish "random UDP noise on port 3785"
// from "a real echo packet I need to reflect".
var ErrBadEchoMagic = errors.New("bfd: echo magic mismatch")

// WriteEcho serializes e into buf[off:off+EchoLen] and returns the
// number of bytes written. The caller provides a buffer large
// enough for the envelope.
func WriteEcho(buf []byte, off int, e Echo) int {
	copy(buf[off:off+4], echoMagic[:])
	binary.BigEndian.PutUint32(buf[off+4:], e.LocalDiscriminator)
	binary.BigEndian.PutUint32(buf[off+8:], e.Sequence)
	binary.BigEndian.PutUint32(buf[off+12:], e.TimestampMs)
	return EchoLen
}

// ParseEcho decodes buf[:EchoLen] into an Echo. Returns ErrShortEcho
// or ErrBadEchoMagic for malformed input; never allocates.
func ParseEcho(buf []byte) (Echo, error) {
	if len(buf) < EchoLen {
		return Echo{}, ErrShortEcho
	}
	if buf[0] != echoMagic[0] || buf[1] != echoMagic[1] || buf[2] != echoMagic[2] || buf[3] != echoMagic[3] {
		return Echo{}, ErrBadEchoMagic
	}
	return Echo{
		LocalDiscriminator: binary.BigEndian.Uint32(buf[4:]),
		Sequence:           binary.BigEndian.Uint32(buf[8:]),
		TimestampMs:        binary.BigEndian.Uint32(buf[12:]),
	}, nil
}
