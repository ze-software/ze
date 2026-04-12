// Design: docs/architecture/core-design.md -- BMP wire format
//
// Related: bmp.go -- plugin lifecycle and session handling
// Related: tlv.go -- TLV encode/decode
// Related: msg.go -- message type encode/decode

package bmp

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// RFC 7854 Section 4.1: Common Header.
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+
//	|    Version    |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                        Message Length                         |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|   Msg. Type   |
//	+-+-+-+-+-+-+-+-+

const (
	Version          = 3
	CommonHeaderSize = 6
	PeerHeaderSize   = 42
	DefaultPort      = 11019
)

// Message types (RFC 7854 Section 4).
const (
	MsgRouteMonitoring  uint8 = 0
	MsgStatisticsReport uint8 = 1
	MsgPeerDownNotify   uint8 = 2
	MsgPeerUpNotify     uint8 = 3
	MsgInitiation       uint8 = 4
	MsgTermination      uint8 = 5
	MsgRouteMirroring   uint8 = 6
)

// Peer types (RFC 7854 Section 4.2).
const (
	PeerTypeGlobal uint8 = 0
	PeerTypeL3VPN  uint8 = 1
	PeerTypeLocal  uint8 = 2
	PeerTypeLocRIB uint8 = 3
)

// Peer flags (RFC 7854 Section 4.2, RFC 8671, RFC 9069).
const (
	PeerFlagV uint8 = 1 << 7 // IPv6
	PeerFlagL uint8 = 1 << 6 // post-policy
	PeerFlagA uint8 = 1 << 5 // 2-byte AS (legacy)
	PeerFlagO uint8 = 1 << 4 // Adj-RIB-Out (RFC 8671)
)

var (
	errShortHeader = errors.New("bmp: common header too short")
	errBadVersion  = errors.New("bmp: unsupported version")
	errShortPeer   = errors.New("bmp: per-peer header too short")
)

// CommonHeader is the 6-byte BMP common header.
type CommonHeader struct {
	Version uint8
	Length  uint32
	Type    uint8
}

// DecodeCommonHeader parses a CommonHeader from buf at the given offset.
// Returns the number of bytes consumed (always CommonHeaderSize on success).
func DecodeCommonHeader(buf []byte, off int) (CommonHeader, int, error) {
	if len(buf)-off < CommonHeaderSize {
		return CommonHeader{}, 0, errShortHeader
	}
	h := CommonHeader{
		Version: buf[off],
		Length:  binary.BigEndian.Uint32(buf[off+1 : off+5]),
		Type:    buf[off+5],
	}
	if h.Version != Version {
		return CommonHeader{}, 0, fmt.Errorf("%w: got %d", errBadVersion, h.Version)
	}
	return h, CommonHeaderSize, nil
}

// WriteCommonHeader writes a CommonHeader into buf at off.
// Returns the number of bytes written (always CommonHeaderSize).
func WriteCommonHeader(buf []byte, off int, h CommonHeader) int {
	buf[off] = h.Version
	binary.BigEndian.PutUint32(buf[off+1:off+5], h.Length)
	buf[off+5] = h.Type
	return CommonHeaderSize
}

// PeerHeader is the 42-byte BMP per-peer header.
//
// RFC 7854 Section 4.2:
//
//	Peer Type (1) + Flags (1) + Peer Distinguisher (8) +
//	Peer Address (16) + Peer AS (4) + Peer BGP ID (4) +
//	Timestamp seconds (4) + Timestamp microseconds (4) = 42 bytes.
type PeerHeader struct {
	PeerType      uint8
	Flags         uint8
	Distinguisher uint64
	Address       [16]byte // IPv4 as ::ffff:x.x.x.x
	PeerAS        uint32
	PeerBGPID     uint32
	TimestampSec  uint32
	TimestampUsec uint32
}

// IsIPv6 returns true if the V flag is set (and peer type is not Loc-RIB).
func (h *PeerHeader) IsIPv6() bool {
	return h.PeerType != PeerTypeLocRIB && h.Flags&PeerFlagV != 0
}

// IsPostPolicy returns true if the L flag is set.
func (h *PeerHeader) IsPostPolicy() bool {
	return h.Flags&PeerFlagL != 0
}

// Is2ByteAS returns true if the A flag is set (legacy 2-byte ASN).
func (h *PeerHeader) Is2ByteAS() bool {
	return h.Flags&PeerFlagA != 0
}

// IsAdjRIBOut returns true if the O flag is set (RFC 8671).
func (h *PeerHeader) IsAdjRIBOut() bool {
	return h.Flags&PeerFlagO != 0
}

// DecodePeerHeader parses a PeerHeader from buf at the given offset.
// Returns the number of bytes consumed (always PeerHeaderSize on success).
func DecodePeerHeader(buf []byte, off int) (PeerHeader, int, error) {
	if len(buf)-off < PeerHeaderSize {
		return PeerHeader{}, 0, errShortPeer
	}
	p := PeerHeader{
		PeerType:      buf[off],
		Flags:         buf[off+1],
		Distinguisher: binary.BigEndian.Uint64(buf[off+2 : off+10]),
		PeerAS:        binary.BigEndian.Uint32(buf[off+26 : off+30]),
		PeerBGPID:     binary.BigEndian.Uint32(buf[off+30 : off+34]),
		TimestampSec:  binary.BigEndian.Uint32(buf[off+34 : off+38]),
		TimestampUsec: binary.BigEndian.Uint32(buf[off+38 : off+42]),
	}
	copy(p.Address[:], buf[off+10:off+26])
	return p, PeerHeaderSize, nil
}

// WritePeerHeader writes a PeerHeader into buf at off.
// Returns the number of bytes written (always PeerHeaderSize).
func WritePeerHeader(buf []byte, off int, p PeerHeader) int {
	buf[off] = p.PeerType
	buf[off+1] = p.Flags
	binary.BigEndian.PutUint64(buf[off+2:off+10], p.Distinguisher)
	copy(buf[off+10:off+26], p.Address[:])
	binary.BigEndian.PutUint32(buf[off+26:off+30], p.PeerAS)
	binary.BigEndian.PutUint32(buf[off+30:off+34], p.PeerBGPID)
	binary.BigEndian.PutUint32(buf[off+34:off+38], p.TimestampSec)
	binary.BigEndian.PutUint32(buf[off+38:off+42], p.TimestampUsec)
	return PeerHeaderSize
}

// HasPeerHeader returns true if the message type includes a per-peer header.
// RFC 7854: Initiation (4) and Termination (5) have no per-peer header.
func HasPeerHeader(msgType uint8) bool {
	return msgType != MsgInitiation && msgType != MsgTermination
}
