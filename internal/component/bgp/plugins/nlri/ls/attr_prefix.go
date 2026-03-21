// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS prefix attribute TLVs
// RFC: rfc/short/rfc7752.md — BGP-LS prefix attribute TLV types (Section 3.3.3)
// Overview: attr.go — TLV interface, registration, and iterator
// Related: attr_node.go — node attribute TLV types
// Related: attr_link.go — link attribute TLV types
package ls

import (
	"encoding/binary"
	"fmt"
	"net/netip"
)

// Prefix attribute TLV type codes.
// RFC 7752 Section 3.3.3 defines prefix attribute TLVs.
const (
	TLVIGPFlags         uint16 = 1152 // RFC 7752 Section 3.3.3.1
	TLVPrefixMetric     uint16 = 1155 // RFC 7752 Section 3.3.3.4
	TLVOpaquePrefixAttr uint16 = 1157 // RFC 7752 Section 3.3.3.6
)

// --- TLV 1152: IGP Flags ---

// LsIGPFlags represents BGP-LS IGP Flags (TLV 1152).
// RFC 7752 Section 3.3.3.1: 1 byte of flags.
//
//	+--+--+--+--+--+--+--+--+
//	|D |N |L |P |  Reserved  |
//	+--+--+--+--+--+--+--+--+
type LsIGPFlags struct {
	Flags uint8
}

func (t *LsIGPFlags) Code() uint16 { return TLVIGPFlags }
func (t *LsIGPFlags) Len() int     { return 4 + 1 }

func (t *LsIGPFlags) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVIGPFlags, 1)
	buf[off+4] = t.Flags
	return n
}

func (t *LsIGPFlags) ToJSON() map[string]any {
	return map[string]any{
		"igp-flags": map[string]any{
			"D":   int((t.Flags >> 7) & 1),
			"N":   int((t.Flags >> 6) & 1),
			"L":   int((t.Flags >> 5) & 1),
			"P":   int((t.Flags >> 4) & 1),
			"RSV": int(t.Flags & 0x0F),
		},
	}
}

func decodeIGPFlags(data []byte) (LsAttrTLV, error) {
	if len(data) < 1 {
		return nil, ErrBGPLSTruncated
	}
	return &LsIGPFlags{Flags: data[0]}, nil
}

// --- TLV 1155: Prefix Metric ---

// LsPrefixMetric represents BGP-LS Prefix Metric (TLV 1155).
// RFC 7752 Section 3.3.3.4: 4-byte unsigned integer.
type LsPrefixMetric struct {
	Metric uint32
}

func (t *LsPrefixMetric) Code() uint16 { return TLVPrefixMetric }
func (t *LsPrefixMetric) Len() int     { return 4 + 4 }

func (t *LsPrefixMetric) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVPrefixMetric, 4)
	binary.BigEndian.PutUint32(buf[off+4:], t.Metric)
	return n
}

func (t *LsPrefixMetric) ToJSON() map[string]any {
	return map[string]any{"prefix-metric": t.Metric}
}

func decodePrefixMetric(data []byte) (LsAttrTLV, error) {
	if len(data) < 4 {
		return nil, ErrBGPLSTruncated
	}
	return &LsPrefixMetric{Metric: binary.BigEndian.Uint32(data)}, nil
}

// --- TLV 1157: Opaque Prefix Attribute ---

// LsOpaquePrefixAttr represents BGP-LS Opaque Prefix Attribute (TLV 1157).
// RFC 7752 Section 3.3.3.6: variable-length opaque data.
type LsOpaquePrefixAttr struct {
	Data []byte
}

func (t *LsOpaquePrefixAttr) Code() uint16 { return TLVOpaquePrefixAttr }
func (t *LsOpaquePrefixAttr) Len() int     { return 4 + len(t.Data) }

func (t *LsOpaquePrefixAttr) WriteTo(buf []byte, off int) int {
	return writeTLVBytes(buf, off, TLVOpaquePrefixAttr, t.Data)
}

func (t *LsOpaquePrefixAttr) ToJSON() map[string]any {
	return map[string]any{"opaque-prefix-attr": fmt.Sprintf("0x%X", t.Data)}
}

func decodeOpaquePrefixAttr(data []byte) (LsAttrTLV, error) {
	cp := make([]byte, len(data))
	copy(cp, data)
	return &LsOpaquePrefixAttr{Data: cp}, nil
}

// --- Phase 2: SR-MPLS Prefix Attribute TLVs (RFC 9085) ---

// SR-MPLS prefix attribute TLV type codes.
const (
	TLVPrefixSID      uint16 = 1158 // RFC 9085 Section 2.3.1
	TLVSIDLabel       uint16 = 1161 // RFC 9085 Section 2.1.1
	TLVSRPrefixFlags  uint16 = 1170 // RFC 9085 Section 2.3.2
	TLVSourceRouterID uint16 = 1171 // RFC 9085 Section 2.3.3
)

// --- TLV 1158: Prefix SID ---

// LsPrefixSID represents BGP-LS Prefix SID (TLV 1158).
// RFC 9085 Section 2.3.1: Flags(1) + Algorithm(1) + Reserved(2) + SID (3 or 4 bytes).
type LsPrefixSID struct {
	Flags     uint8
	Algorithm uint8
	SID       uint32
	// wireLen preserves 7 vs 8 byte total value length for round-trip.
	wireLen int
}

func (t *LsPrefixSID) Code() uint16 { return TLVPrefixSID }

func (t *LsPrefixSID) Len() int {
	if t.wireLen > 0 {
		return 4 + t.wireLen
	}
	return 4 + 8 // default: flags(1) + algo(1) + reserved(2) + SID(4)
}

func (t *LsPrefixSID) WriteTo(buf []byte, off int) int {
	valueLen := t.Len() - 4
	n := writeTLV(buf, off, TLVPrefixSID, valueLen)
	vOff := off + 4
	buf[vOff] = t.Flags
	buf[vOff+1] = t.Algorithm
	buf[vOff+2] = 0 // reserved
	buf[vOff+3] = 0 // reserved
	sidLen := valueLen - 4
	if sidLen == 3 {
		buf[vOff+4] = byte(t.SID >> 16)
		buf[vOff+5] = byte(t.SID >> 8)
		buf[vOff+6] = byte(t.SID)
	} else {
		binary.BigEndian.PutUint32(buf[vOff+4:], t.SID)
	}
	return n
}

func (t *LsPrefixSID) ToJSON() map[string]any {
	return map[string]any{
		"prefix-sid": map[string]any{
			"flags":     int(t.Flags),
			"algorithm": int(t.Algorithm),
			"sid":       t.SID,
		},
	}
}

func decodePrefixSID(data []byte) (LsAttrTLV, error) {
	// RFC 9085 Section 2.3.1: 7 or 8 bytes
	if len(data) != 7 && len(data) != 8 {
		return nil, ErrBGPLSTruncated
	}
	ps := &LsPrefixSID{
		Flags:     data[0],
		Algorithm: data[1],
		wireLen:   len(data),
	}
	// data[2:4] reserved
	v := data[4:]
	if len(v) == 4 {
		ps.SID = binary.BigEndian.Uint32(v)
	} else {
		// RFC 9085 Section 2.3.1: 3-byte label, 20 rightmost bits
		ps.SID = (uint32(v[0])<<16 | uint32(v[1])<<8 | uint32(v[2])) & 0xFFFFF
	}
	return ps, nil
}

// --- TLV 1161: SID/Label ---

// LsSIDLabel represents BGP-LS SID/Label (TLV 1161).
// RFC 9085 Section 2.1.1: 3-byte label (20 bits) or 4-byte SID index.
type LsSIDLabel struct {
	SID uint32
	// wireLen preserves 3 vs 4 byte encoding for round-trip.
	wireLen int
}

func (t *LsSIDLabel) Code() uint16 { return TLVSIDLabel }

func (t *LsSIDLabel) Len() int {
	if t.wireLen > 0 {
		return 4 + t.wireLen
	}
	return 4 + 4
}

func (t *LsSIDLabel) WriteTo(buf []byte, off int) int {
	valueLen := t.Len() - 4
	n := writeTLV(buf, off, TLVSIDLabel, valueLen)
	if valueLen == 3 {
		buf[off+4] = byte(t.SID >> 16)
		buf[off+5] = byte(t.SID >> 8)
		buf[off+6] = byte(t.SID)
	} else {
		binary.BigEndian.PutUint32(buf[off+4:], t.SID)
	}
	return n
}

func (t *LsSIDLabel) ToJSON() map[string]any {
	return map[string]any{"sid-label": t.SID}
}

func decodeSIDLabel(data []byte) (LsAttrTLV, error) {
	if len(data) != 3 && len(data) != 4 {
		return nil, ErrBGPLSTruncated
	}
	var sid uint32
	if len(data) == 4 {
		sid = binary.BigEndian.Uint32(data)
	} else {
		sid = (uint32(data[0])<<16 | uint32(data[1])<<8 | uint32(data[2])) & 0xFFFFF
	}
	return &LsSIDLabel{SID: sid, wireLen: len(data)}, nil
}

// --- TLV 1170: SR Prefix Attribute Flags ---

// LsSRPrefixFlags represents BGP-LS SR Prefix Attribute Flags (TLV 1170).
// RFC 9085 Section 2.3.2: 1 byte of flags.
//
//	+--+--+--+--+--+--+--+--+
//	|X |R |N |    Reserved   |
//	+--+--+--+--+--+--+--+--+
type LsSRPrefixFlags struct {
	Flags uint8
}

func (t *LsSRPrefixFlags) Code() uint16 { return TLVSRPrefixFlags }
func (t *LsSRPrefixFlags) Len() int     { return 4 + 1 }

func (t *LsSRPrefixFlags) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVSRPrefixFlags, 1)
	buf[off+4] = t.Flags
	return n
}

func (t *LsSRPrefixFlags) ToJSON() map[string]any {
	return map[string]any{
		"sr-prefix-flags": map[string]any{
			"X":   int((t.Flags >> 7) & 1),
			"R":   int((t.Flags >> 6) & 1),
			"N":   int((t.Flags >> 5) & 1),
			"RSV": int(t.Flags & 0x1F),
		},
	}
}

func decodeSRPrefixFlags(data []byte) (LsAttrTLV, error) {
	if len(data) < 1 {
		return nil, ErrBGPLSTruncated
	}
	return &LsSRPrefixFlags{Flags: data[0]}, nil
}

// --- TLV 1171: Source Router ID ---

// LsSourceRouterID represents BGP-LS Source Router ID (TLV 1171).
// RFC 9085 Section 2.3.3: 4-byte (IPv4) or 16-byte (IPv6) router ID.
type LsSourceRouterID struct {
	ID []byte // 4 or 16 bytes
}

func (t *LsSourceRouterID) Code() uint16 { return TLVSourceRouterID }
func (t *LsSourceRouterID) Len() int     { return 4 + len(t.ID) }

func (t *LsSourceRouterID) WriteTo(buf []byte, off int) int {
	return writeTLVBytes(buf, off, TLVSourceRouterID, t.ID)
}

func (t *LsSourceRouterID) ToJSON() map[string]any {
	switch len(t.ID) {
	case 4:
		addr := netip.AddrFrom4([4]byte(t.ID[:4]))
		return map[string]any{"source-router-id": addr.String()}
	case 16:
		addr := netip.AddrFrom16([16]byte(t.ID[:16]))
		return map[string]any{"source-router-id": addr.String()}
	}
	return map[string]any{"source-router-id": fmt.Sprintf("0x%X", t.ID)}
}

func decodeSourceRouterID(data []byte) (LsAttrTLV, error) {
	if len(data) != 4 && len(data) != 16 {
		return nil, ErrBGPLSTruncated
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	return &LsSourceRouterID{ID: cp}, nil
}
