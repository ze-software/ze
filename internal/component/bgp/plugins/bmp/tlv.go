// Design: docs/architecture/core-design.md -- BMP TLV encoding
//
// Related: bmp.go -- plugin lifecycle and session handling
// Related: header.go -- common and per-peer header encode/decode
// Related: msg.go -- message type encode/decode

package bmp

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// TLV sizes.
const (
	TLVHeaderSize = 4 // Type (2) + Length (2)
)

// Initiation TLV types (RFC 7854 Section 4.3).
const (
	InitTLVString   uint16 = 0
	InitTLVSysDescr uint16 = 1
	InitTLVSysName  uint16 = 2
)

// Termination TLV types (RFC 7854 Section 4.5).
const (
	TermTLVString uint16 = 0
	TermTLVReason uint16 = 1
)

// Termination reason codes (RFC 7854 Section 4.5).
const (
	TermReasonAdminDown     uint16 = 0
	TermReasonUnspecified   uint16 = 1
	TermReasonOutOfRes      uint16 = 2
	TermReasonRedundant     uint16 = 3
	TermReasonPermAdminDown uint16 = 4
)

// Route Mirroring TLV types (RFC 7854 Section 4.7).
const (
	MirrorTLVBGPMsg   uint16 = 0
	MirrorTLVMsgsLost uint16 = 1
)

// Statistics Report types (RFC 7854 Section 4.8).
const (
	StatPrefixesRejected        uint16 = 0
	StatDuplicatePrefix         uint16 = 1
	StatDuplicateWithdraw       uint16 = 2
	StatClusterListLoop         uint16 = 3
	StatASPathLoop              uint16 = 4
	StatOriginatorID            uint16 = 5
	StatASConfedLoop            uint16 = 6
	StatRoutesAdjRIBIn          uint16 = 7
	StatRoutesLocRIB            uint16 = 8
	StatRoutesPerAFIAdjRIBIn    uint16 = 9
	StatRoutesPerAFILocRIB      uint16 = 10
	StatTreatAsWithdrawUpdates  uint16 = 11
	StatTreatAsWithdrawPrefixes uint16 = 12
	StatDuplicateUpdates        uint16 = 13
)

var errShortTLV = errors.New("bmp: TLV too short")

// TLV represents a BMP information TLV (Type-Length-Value).
type TLV struct {
	Type   uint16
	Length uint16
	Value  []byte
}

// DecodeTLV parses a single TLV from buf at off.
// Returns the TLV and bytes consumed.
func DecodeTLV(buf []byte, off int) (TLV, int, error) {
	if len(buf)-off < TLVHeaderSize {
		return TLV{}, 0, errShortTLV
	}
	t := TLV{
		Type:   binary.BigEndian.Uint16(buf[off : off+2]),
		Length: binary.BigEndian.Uint16(buf[off+2 : off+4]),
	}
	end := off + TLVHeaderSize + int(t.Length)
	if end > len(buf) {
		return TLV{}, 0, fmt.Errorf("%w: need %d bytes, have %d", errShortTLV, t.Length, len(buf)-off-TLVHeaderSize)
	}
	t.Value = buf[off+TLVHeaderSize : end]
	return t, TLVHeaderSize + int(t.Length), nil
}

// WriteTLV writes a TLV into buf at off.
// Returns bytes written.
func WriteTLV(buf []byte, off int, t TLV) int {
	binary.BigEndian.PutUint16(buf[off:off+2], t.Type)
	binary.BigEndian.PutUint16(buf[off+2:off+4], t.Length)
	copy(buf[off+TLVHeaderSize:], t.Value)
	return TLVHeaderSize + int(t.Length)
}

// DecodeTLVs parses all TLVs from buf[off:end].
func DecodeTLVs(buf []byte, off, end int) ([]TLV, error) {
	var tlvs []TLV
	for off < end {
		t, n, err := DecodeTLV(buf, off)
		if err != nil {
			return tlvs, err
		}
		tlvs = append(tlvs, t)
		off += n
	}
	return tlvs, nil
}

// WriteTLVs writes multiple TLVs into buf at off.
// Returns total bytes written.
func WriteTLVs(buf []byte, off int, tlvs []TLV) int {
	total := 0
	for i := range tlvs {
		n := WriteTLV(buf, off, tlvs[i])
		off += n
		total += n
	}
	return total
}

// maxTLVValueLen is the maximum value length a TLV can encode (uint16).
const maxTLVValueLen = 65535

// MakeStringTLV creates a TLV with a string value.
// Strings longer than 65535 bytes are truncated to fit the uint16 length field.
func MakeStringTLV(typ uint16, s string) TLV {
	if len(s) > maxTLVValueLen {
		s = s[:maxTLVValueLen]
	}
	return TLV{Type: typ, Length: uint16(len(s)), Value: []byte(s)}
}
