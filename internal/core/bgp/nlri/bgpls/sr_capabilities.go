// Design: docs/architecture/wire/nlri-bgpls.md — SR Capabilities TLV 1034, SR label ranges, shared JSON keys
// RFC: rfc/short/rfc9085.md
// Related: attr_tlv_codes.go — the attribute TLV type codes

package bgpls

import (
	"encoding/binary"
)

const (
	JSONKeyFlags = "flags"
	// JSONKeyReserved carries the reserved bits of a flags octet, so a
	// consumer sees what the wire held even for bits ze does not name.
	JSONKeyReserved = "RSV"
)

// LsSrLabelRange represents a single SRGB or SRLB range entry.
// RFC 9085 Section 3: Range(3 bytes) + SID/Label sub-TLV.
type LsSrLabelRange struct {
	Range    uint32 // number of labels in range
	FirstSID uint32 // first SID/label value
	// SIDLen preserves 3 vs 4 byte SID encoding for round-trip.
	SIDLen int
}

// SRLabelRangesLen returns the wire length of a slice of label ranges (without TLV header or flags).
func SRLabelRangesLen(ranges []LsSrLabelRange) int {
	n := 0
	for _, r := range ranges {
		sidLen := r.SIDLen
		if sidLen == 0 {
			sidLen = 4
		}
		n += 3 + 4 + sidLen // Range(3) + sub-TLV header(4) + SID(3 or 4)
	}
	return n
}

// WriteSRLabelRanges writes label ranges to buf at off. Returns bytes written.
func WriteSRLabelRanges(buf []byte, off int, ranges []LsSrLabelRange) {
	pos := off
	for _, r := range ranges {
		buf[pos] = byte(r.Range >> 16)
		buf[pos+1] = byte(r.Range >> 8)
		buf[pos+2] = byte(r.Range)
		pos += 3

		sidLen := r.SIDLen
		if sidLen == 0 {
			sidLen = 4
		}
		binary.BigEndian.PutUint16(buf[pos:], TLVSIDLabel)
		binary.BigEndian.PutUint16(buf[pos+2:], uint16(sidLen)) //nolint:gosec // bounded
		pos += 4
		if sidLen == 3 {
			buf[pos] = byte(r.FirstSID >> 16)
			buf[pos+1] = byte(r.FirstSID >> 8)
			buf[pos+2] = byte(r.FirstSID)
		} else {
			binary.BigEndian.PutUint32(buf[pos:], r.FirstSID)
		}
		pos += sidLen
	}
}

// DecodeSRLabelRanges parses label ranges from wire bytes (past flags+reserved).
func DecodeSRLabelRanges(data []byte) ([]LsSrLabelRange, error) {
	var ranges []LsSrLabelRange
	rest := data
	for len(rest) > 7 { // 3 (range) + 4 (sub-TLV header) minimum
		rangeVal := uint32(rest[0])<<16 | uint32(rest[1])<<8 | uint32(rest[2])
		rest = rest[3:]

		if len(rest) < 4 {
			return nil, ErrBGPLSTruncated
		}
		subLen := int(binary.BigEndian.Uint16(rest[2:4]))
		rest = rest[4:]

		if len(rest) < subLen {
			return nil, ErrBGPLSTruncated
		}

		var sid uint32
		if subLen == 3 {
			sid = (uint32(rest[0])<<16 | uint32(rest[1])<<8 | uint32(rest[2])) & 0xFFFFF
		} else if subLen >= 4 {
			sid = binary.BigEndian.Uint32(rest[:4])
		}

		ranges = append(ranges, LsSrLabelRange{
			Range:    rangeVal,
			FirstSID: sid,
			SIDLen:   subLen,
		})
		rest = rest[subLen:]
	}
	return ranges, nil
}

// SRLabelRangesToJSON converts label ranges to JSON-friendly slice.
func SRLabelRangesToJSON(ranges []LsSrLabelRange) []map[string]any {
	result := make([]map[string]any, len(ranges))
	for i, r := range ranges {
		result[i] = map[string]any{
			"first-sid": r.FirstSID,
			"range":     r.Range,
		}
	}
	return result
}

// LsSRCapabilities represents BGP-LS SR Capabilities (TLV 1034).
// RFC 9085 Section 3: Flags(1) + Reserved(1) + N x {Range(3) + SID/Label sub-TLV}.
type LsSRCapabilities struct {
	Flags  uint8
	Ranges []LsSrLabelRange
}

func (t *LsSRCapabilities) Code() uint16 { return TLVSRCapabilities }

func (t *LsSRCapabilities) Len() int {
	return 4 + 2 + SRLabelRangesLen(t.Ranges) // header + flags + reserved + ranges
}

func (t *LsSRCapabilities) WriteTo(buf []byte, off int) int {
	valueLen := t.Len() - 4
	n := WriteTLV(buf, off, TLVSRCapabilities, valueLen)
	vOff := off + 4
	buf[vOff] = t.Flags
	buf[vOff+1] = 0 // reserved
	WriteSRLabelRanges(buf, vOff+2, t.Ranges)
	return n
}

func (t *LsSRCapabilities) ToJSON() map[string]any {
	return map[string]any{
		"sr-capabilities": map[string]any{
			JSONKeyFlags: map[string]any{
				"I":             int((t.Flags >> 7) & 1), // IPv4 MPLS
				"V":             int((t.Flags >> 6) & 1), // IPv6 MPLS
				JSONKeyReserved: int(t.Flags & 0x3F),
			},
			"ranges": SRLabelRangesToJSON(t.Ranges),
		},
	}
}

// DecodeSRCapabilities decodes the value of an SR Capabilities TLV (1034,
// RFC 9085 Section 2.1.2). The BGP-LS plugin registers it, through an
// adapter, as the TLV 1034 decoder.
func DecodeSRCapabilities(data []byte) (*LsSRCapabilities, error) {
	// RFC 9085 Section 3: minimum flags(1) + reserved(1) = 2 bytes
	if len(data) < 2 {
		return nil, ErrBGPLSTruncated
	}
	ranges, err := DecodeSRLabelRanges(data[2:])
	if err != nil {
		return nil, err
	}
	return &LsSRCapabilities{Flags: data[0], Ranges: ranges}, nil
}
