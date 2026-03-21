// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS node attribute TLVs
// RFC: rfc/short/rfc7752.md — BGP-LS node attribute TLV types (Section 3.3.1)
// Overview: attr.go — TLV interface, registration, and iterator
// Related: attr_link.go — link attribute TLV types
// Related: attr_prefix.go — prefix attribute TLV types
package ls

import (
	"encoding/binary"
	"fmt"
	"net/netip"
)

// Node attribute TLV type codes.
// RFC 7752 Section 3.3.1 defines node attribute TLVs.
const (
	TLVNodeFlagBits      uint16 = 1024 // RFC 7752 Section 3.3.1.1
	TLVOpaqueNodeAttr    uint16 = 1025 // RFC 7752 Section 3.3.1.5
	TLVNodeName          uint16 = 1026 // RFC 7752 Section 3.3.1.3
	TLVISISAreaID        uint16 = 1027 // RFC 7752 Section 3.3.1.2
	TLVIPv4RouterIDLocal uint16 = 1028 // RFC 7752 Section 3.3.1.4
	TLVIPv6RouterIDLocal uint16 = 1029 // RFC 7752 Section 3.3.1.4
)

// --- TLV 1024: Node Flag Bits ---

// LsNodeFlagBits represents BGP-LS Node Flag Bits (TLV 1024).
// RFC 7752 Section 3.3.1.1: 1 byte of flags.
//
//	+--+--+--+--+--+--+--+--+
//	|O |T |E |B |R |V |  RSV |
//	+--+--+--+--+--+--+--+--+
type LsNodeFlagBits struct {
	Flags uint8
}

func (t *LsNodeFlagBits) Code() uint16 { return TLVNodeFlagBits }
func (t *LsNodeFlagBits) Len() int     { return 4 + 1 }

func (t *LsNodeFlagBits) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVNodeFlagBits, 1)
	buf[off+4] = t.Flags
	return n
}

func (t *LsNodeFlagBits) ToJSON() map[string]any {
	return map[string]any{
		"node-flags": map[string]any{
			"O":   int((t.Flags >> 7) & 1),
			"T":   int((t.Flags >> 6) & 1),
			"E":   int((t.Flags >> 5) & 1),
			"B":   int((t.Flags >> 4) & 1),
			"R":   int((t.Flags >> 3) & 1),
			"V":   int((t.Flags >> 2) & 1),
			"RSV": int(t.Flags & 0x03),
		},
	}
}

func decodeNodeFlagBits(data []byte) (LsAttrTLV, error) {
	// RFC 7752 Section 3.3.1.1: exactly 1 byte
	if len(data) < 1 {
		return nil, ErrBGPLSTruncated
	}
	return &LsNodeFlagBits{Flags: data[0]}, nil
}

// --- TLV 1025: Opaque Node Attribute ---

// LsOpaqueNodeAttr represents BGP-LS Opaque Node Attribute (TLV 1025).
// RFC 7752 Section 3.3.1.5: variable-length opaque data.
type LsOpaqueNodeAttr struct {
	Data []byte
}

func (t *LsOpaqueNodeAttr) Code() uint16 { return TLVOpaqueNodeAttr }
func (t *LsOpaqueNodeAttr) Len() int     { return 4 + len(t.Data) }

func (t *LsOpaqueNodeAttr) WriteTo(buf []byte, off int) int {
	return writeTLVBytes(buf, off, TLVOpaqueNodeAttr, t.Data)
}

func (t *LsOpaqueNodeAttr) ToJSON() map[string]any {
	return map[string]any{
		"opaque-node-attr": formatHex(t.Data),
	}
}

func decodeOpaqueNodeAttr(data []byte) (LsAttrTLV, error) {
	cp := make([]byte, len(data))
	copy(cp, data)
	return &LsOpaqueNodeAttr{Data: cp}, nil
}

// --- TLV 1026: Node Name ---

// LsNodeName represents BGP-LS Node Name (TLV 1026).
// RFC 7752 Section 3.3.1.3: variable-length UTF-8 string.
type LsNodeName struct {
	Name string
}

func (t *LsNodeName) Code() uint16 { return TLVNodeName }
func (t *LsNodeName) Len() int     { return 4 + len(t.Name) }

func (t *LsNodeName) WriteTo(buf []byte, off int) int {
	return writeTLVBytes(buf, off, TLVNodeName, []byte(t.Name))
}

func (t *LsNodeName) ToJSON() map[string]any {
	return map[string]any{
		"node-name": t.Name,
	}
}

func decodeNodeName(data []byte) (LsAttrTLV, error) {
	return &LsNodeName{Name: string(data)}, nil
}

// --- TLV 1027: IS-IS Area Identifier ---

// LsISISAreaID represents BGP-LS IS-IS Area Identifier (TLV 1027).
// RFC 7752 Section 3.3.1.2: variable-length IS-IS area ID.
type LsISISAreaID struct {
	AreaID []byte
}

func (t *LsISISAreaID) Code() uint16 { return TLVISISAreaID }
func (t *LsISISAreaID) Len() int     { return 4 + len(t.AreaID) }

func (t *LsISISAreaID) WriteTo(buf []byte, off int) int {
	return writeTLVBytes(buf, off, TLVISISAreaID, t.AreaID)
}

func (t *LsISISAreaID) ToJSON() map[string]any {
	return map[string]any{
		"area-id": fmt.Sprintf("0x%X", t.AreaID),
	}
}

func decodeISISAreaID(data []byte) (LsAttrTLV, error) {
	cp := make([]byte, len(data))
	copy(cp, data)
	return &LsISISAreaID{AreaID: cp}, nil
}

// --- TLV 1028: IPv4 Router-ID (Local) ---

// LsIPv4RouterIDLocal represents BGP-LS IPv4 Local Router-ID (TLV 1028).
// RFC 7752 Section 3.3.1.4: 4-byte IPv4 address.
type LsIPv4RouterIDLocal struct {
	Addr netip.Addr
}

func (t *LsIPv4RouterIDLocal) Code() uint16 { return TLVIPv4RouterIDLocal }
func (t *LsIPv4RouterIDLocal) Len() int     { return 4 + 4 }

func (t *LsIPv4RouterIDLocal) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVIPv4RouterIDLocal, 4)
	b := t.Addr.As4()
	copy(buf[off+4:], b[:])
	return n
}

func (t *LsIPv4RouterIDLocal) ToJSON() map[string]any {
	return map[string]any{
		"local-router-ids": []string{t.Addr.String()},
	}
}

func decodeIPv4RouterIDLocal(data []byte) (LsAttrTLV, error) {
	// RFC 7752 Section 3.3.1.4: exactly 4 bytes
	if len(data) != 4 {
		return nil, ErrBGPLSTruncated
	}
	addr := netip.AddrFrom4([4]byte(data[:4]))
	return &LsIPv4RouterIDLocal{Addr: addr}, nil
}

// --- TLV 1029: IPv6 Router-ID (Local) ---

// LsIPv6RouterIDLocal represents BGP-LS IPv6 Local Router-ID (TLV 1029).
// RFC 7752 Section 3.3.1.4: 16-byte IPv6 address.
type LsIPv6RouterIDLocal struct {
	Addr netip.Addr
}

func (t *LsIPv6RouterIDLocal) Code() uint16 { return TLVIPv6RouterIDLocal }
func (t *LsIPv6RouterIDLocal) Len() int     { return 4 + 16 }

func (t *LsIPv6RouterIDLocal) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVIPv6RouterIDLocal, 16)
	b := t.Addr.As16()
	copy(buf[off+4:], b[:])
	return n
}

func (t *LsIPv6RouterIDLocal) ToJSON() map[string]any {
	return map[string]any{
		"local-router-ids": []string{t.Addr.String()},
	}
}

func decodeIPv6RouterIDLocal(data []byte) (LsAttrTLV, error) {
	// RFC 7752 Section 3.3.1.4: exactly 16 bytes
	if len(data) != 16 {
		return nil, ErrBGPLSTruncated
	}
	addr := netip.AddrFrom16([16]byte(data[:16]))
	return &LsIPv6RouterIDLocal{Addr: addr}, nil
}

// --- Phase 2: SR-MPLS Node Attribute TLVs (RFC 9085) ---

// SR-MPLS node attribute TLV type codes.
const (
	TLVSRCapabilities uint16 = 1034 // RFC 9085 Section 3
	TLVSRAlgorithm    uint16 = 1035 // RFC 9085 Section 4
	TLVSRLocalBlock   uint16 = 1036 // RFC 9085 Section 5
)

// --- TLV 1034: SR Capabilities ---

// LsSrLabelRange represents a single SRGB or SRLB range entry.
// RFC 9085 Section 3: Range(3 bytes) + SID/Label sub-TLV.
type LsSrLabelRange struct {
	Range    uint32 // number of labels in range
	FirstSID uint32 // first SID/label value
	// sidLen preserves 3 vs 4 byte SID encoding for round-trip.
	sidLen int
}

// srLabelRangesLen returns the wire length of a slice of label ranges (without TLV header or flags).
func srLabelRangesLen(ranges []LsSrLabelRange) int {
	n := 0
	for _, r := range ranges {
		sidLen := r.sidLen
		if sidLen == 0 {
			sidLen = 4
		}
		n += 3 + 4 + sidLen // Range(3) + sub-TLV header(4) + SID(3 or 4)
	}
	return n
}

// writeSrLabelRanges writes label ranges to buf at off. Returns bytes written.
func writeSrLabelRanges(buf []byte, off int, ranges []LsSrLabelRange) {
	pos := off
	for _, r := range ranges {
		buf[pos] = byte(r.Range >> 16)
		buf[pos+1] = byte(r.Range >> 8)
		buf[pos+2] = byte(r.Range)
		pos += 3

		sidLen := r.sidLen
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

// decodeSrLabelRanges parses label ranges from wire bytes (past flags+reserved).
func decodeSrLabelRanges(data []byte) ([]LsSrLabelRange, error) {
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
			sidLen:   subLen,
		})
		rest = rest[subLen:]
	}
	return ranges, nil
}

// srLabelRangesToJSON converts label ranges to JSON-friendly slice.
func srLabelRangesToJSON(ranges []LsSrLabelRange) []map[string]any {
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
	return 4 + 2 + srLabelRangesLen(t.Ranges) // header + flags + reserved + ranges
}

func (t *LsSRCapabilities) WriteTo(buf []byte, off int) int {
	valueLen := t.Len() - 4
	n := writeTLV(buf, off, TLVSRCapabilities, valueLen)
	vOff := off + 4
	buf[vOff] = t.Flags
	buf[vOff+1] = 0 // reserved
	writeSrLabelRanges(buf, vOff+2, t.Ranges)
	return n
}

func (t *LsSRCapabilities) ToJSON() map[string]any {
	return map[string]any{
		"sr-capabilities": map[string]any{
			"flags": map[string]any{
				"I":   int((t.Flags >> 7) & 1), // IPv4 MPLS
				"V":   int((t.Flags >> 6) & 1), // IPv6 MPLS
				"RSV": int(t.Flags & 0x3F),
			},
			"ranges": srLabelRangesToJSON(t.Ranges),
		},
	}
}

func decodeSRCapabilities(data []byte) (LsAttrTLV, error) {
	// RFC 9085 Section 3: minimum flags(1) + reserved(1) = 2 bytes
	if len(data) < 2 {
		return nil, ErrBGPLSTruncated
	}
	ranges, err := decodeSrLabelRanges(data[2:])
	if err != nil {
		return nil, err
	}
	return &LsSRCapabilities{Flags: data[0], Ranges: ranges}, nil
}

// --- TLV 1035: SR Algorithm ---

// LsSRAlgorithm represents BGP-LS SR Algorithm (TLV 1035).
// RFC 9085 Section 4: variable-length array of algorithm IDs.
type LsSRAlgorithm struct {
	Algorithms []uint8
}

func (t *LsSRAlgorithm) Code() uint16 { return TLVSRAlgorithm }
func (t *LsSRAlgorithm) Len() int     { return 4 + len(t.Algorithms) }

func (t *LsSRAlgorithm) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVSRAlgorithm, len(t.Algorithms))
	copy(buf[off+4:], t.Algorithms)
	return n
}

func (t *LsSRAlgorithm) ToJSON() map[string]any {
	algos := make([]int, len(t.Algorithms))
	for i, a := range t.Algorithms {
		algos[i] = int(a)
	}
	return map[string]any{"sr-algorithms": algos}
}

func decodeSRAlgorithm(data []byte) (LsAttrTLV, error) {
	algos := make([]uint8, len(data))
	copy(algos, data)
	return &LsSRAlgorithm{Algorithms: algos}, nil
}

// --- TLV 1036: SR Local Block ---

// LsSRLocalBlock represents BGP-LS SR Local Block (TLV 1036).
// RFC 9085 Section 5: Flags(1) + Reserved(1) + N x {Range(3) + SID/Label sub-TLV}.
// Same format as SR Capabilities but for SRLB ranges.
type LsSRLocalBlock struct {
	Flags  uint8
	Ranges []LsSrLabelRange
}

func (t *LsSRLocalBlock) Code() uint16 { return TLVSRLocalBlock }

func (t *LsSRLocalBlock) Len() int {
	return 4 + 2 + srLabelRangesLen(t.Ranges) // header + flags + reserved + ranges
}

func (t *LsSRLocalBlock) WriteTo(buf []byte, off int) int {
	valueLen := t.Len() - 4
	n := writeTLV(buf, off, TLVSRLocalBlock, valueLen)
	vOff := off + 4
	buf[vOff] = t.Flags
	buf[vOff+1] = 0
	writeSrLabelRanges(buf, vOff+2, t.Ranges)
	return n
}

func (t *LsSRLocalBlock) ToJSON() map[string]any {
	return map[string]any{"sr-local-block": map[string]any{"ranges": srLabelRangesToJSON(t.Ranges)}}
}

func decodeSRLocalBlock(data []byte) (LsAttrTLV, error) {
	if len(data) < 2 {
		return nil, ErrBGPLSTruncated
	}
	ranges, err := decodeSrLabelRanges(data[2:])
	if err != nil {
		return nil, err
	}
	return &LsSRLocalBlock{Flags: data[0], Ranges: ranges}, nil
}
