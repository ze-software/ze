// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS node attribute TLVs
// RFC: rfc/short/rfc7752.md — BGP-LS node attribute TLV types (Section 3.3.1)
// Overview: attr.go — TLV interface, registration, and iterator
// Related: attr_link.go — link attribute TLV types
// Related: attr_prefix.go — prefix attribute TLV types
package ls

import (
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/nlri/bgpls"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// --- TLV 1024: Node Flag Bits ---

// lsNodeFlagBits represents BGP-LS Node Flag Bits (TLV 1024).
// RFC 7752 Section 3.3.1.1: 1 byte of flags.
//
//	+--+--+--+--+--+--+--+--+
//	|O |T |E |B |R |V |  RSV |
//	+--+--+--+--+--+--+--+--+
type lsNodeFlagBits struct {
	Flags uint8
}

func (t *lsNodeFlagBits) Code() uint16 { return bgpls.TLVNodeFlagBits }
func (t *lsNodeFlagBits) Len() int     { return 4 + 1 }

func (t *lsNodeFlagBits) WriteTo(buf []byte, off int) int {
	n := bgpls.WriteTLV(buf, off, bgpls.TLVNodeFlagBits, 1)
	buf[off+4] = t.Flags
	return n
}

func (t *lsNodeFlagBits) ToJSON() map[string]any {
	return map[string]any{
		"node-flags": map[string]any{
			"O":                   int((t.Flags >> 7) & 1),
			"T":                   int((t.Flags >> 6) & 1),
			"E":                   int((t.Flags >> 5) & 1),
			"B":                   int((t.Flags >> 4) & 1),
			"R":                   int((t.Flags >> 3) & 1),
			"V":                   int((t.Flags >> 2) & 1),
			bgpls.JSONKeyReserved: int(t.Flags & 0x03),
		},
	}
}

func decodeNodeFlagBits(data []byte) (lsAttrTLV, error) {
	// RFC 7752 Section 3.3.1.1: exactly 1 byte
	if len(data) < 1 {
		return nil, bgpls.ErrBGPLSTruncated
	}
	return &lsNodeFlagBits{Flags: data[0]}, nil
}

// --- TLV 1025: Opaque Node Attribute ---

// lsOpaqueNodeAttr represents BGP-LS Opaque Node Attribute (TLV 1025).
// RFC 7752 Section 3.3.1.5: variable-length opaque data.
type lsOpaqueNodeAttr struct {
	Data []byte
}

func (t *lsOpaqueNodeAttr) Code() uint16 { return bgpls.TLVOpaqueNodeAttr }
func (t *lsOpaqueNodeAttr) Len() int     { return 4 + len(t.Data) }

func (t *lsOpaqueNodeAttr) WriteTo(buf []byte, off int) int {
	return bgpls.WriteTLVBytes(buf, off, bgpls.TLVOpaqueNodeAttr, t.Data)
}

func (t *lsOpaqueNodeAttr) ToJSON() map[string]any {
	return map[string]any{
		"opaque-node-attr": formatHex(t.Data),
	}
}

func decodeOpaqueNodeAttr(data []byte) (lsAttrTLV, error) {
	cp := make([]byte, len(data))
	copy(cp, data)
	return &lsOpaqueNodeAttr{Data: cp}, nil
}

// --- TLV 1026: Node Name ---

// lsNodeName represents BGP-LS Node Name (TLV 1026).
// RFC 7752 Section 3.3.1.3: variable-length UTF-8 string.
type lsNodeName struct {
	Name string
}

func (t *lsNodeName) Code() uint16 { return bgpls.TLVNodeName }
func (t *lsNodeName) Len() int     { return 4 + len(t.Name) }

func (t *lsNodeName) WriteTo(buf []byte, off int) int {
	return bgpls.WriteTLVBytes(buf, off, bgpls.TLVNodeName, []byte(t.Name))
}

func (t *lsNodeName) ToJSON() map[string]any {
	return map[string]any{
		"node-name": t.Name,
	}
}

func decodeNodeName(data []byte) (lsAttrTLV, error) {
	return &lsNodeName{Name: string(data)}, nil
}

// --- TLV 1027: IS-IS Area Identifier ---

// lsISISAreaID represents BGP-LS IS-IS Area Identifier (TLV 1027).
// RFC 7752 Section 3.3.1.2: variable-length IS-IS area ID.
type lsISISAreaID struct {
	AreaID []byte
}

func (t *lsISISAreaID) Code() uint16 { return bgpls.TLVISISAreaID }
func (t *lsISISAreaID) Len() int     { return 4 + len(t.AreaID) }

func (t *lsISISAreaID) WriteTo(buf []byte, off int) int {
	return bgpls.WriteTLVBytes(buf, off, bgpls.TLVISISAreaID, t.AreaID)
}

func (t *lsISISAreaID) ToJSON() map[string]any {
	return map[string]any{
		"area-id": "0x" + strings.ToUpper(textbuf.StringHex(t.AreaID)),
	}
}

func decodeISISAreaID(data []byte) (lsAttrTLV, error) {
	cp := make([]byte, len(data))
	copy(cp, data)
	return &lsISISAreaID{AreaID: cp}, nil
}

// --- TLV 1028: IPv4 Router-ID (Local) ---

// lsIPv4RouterIDLocal represents BGP-LS IPv4 Local Router-ID (TLV 1028).
// RFC 7752 Section 3.3.1.4: 4-byte IPv4 address.
type lsIPv4RouterIDLocal struct {
	Addr netip.Addr
}

func (t *lsIPv4RouterIDLocal) Code() uint16 { return bgpls.TLVIPv4RouterIDLocal }
func (t *lsIPv4RouterIDLocal) Len() int     { return 4 + 4 }

func (t *lsIPv4RouterIDLocal) WriteTo(buf []byte, off int) int {
	n := bgpls.WriteTLV(buf, off, bgpls.TLVIPv4RouterIDLocal, 4)
	b := t.Addr.As4()
	copy(buf[off+4:], b[:])
	return n
}

func (t *lsIPv4RouterIDLocal) ToJSON() map[string]any {
	return map[string]any{
		"local-router-ids": []string{t.Addr.String()},
	}
}

func decodeIPv4RouterIDLocal(data []byte) (lsAttrTLV, error) {
	// RFC 7752 Section 3.3.1.4: exactly 4 bytes
	if len(data) != 4 {
		return nil, bgpls.ErrBGPLSTruncated
	}
	addr := netip.AddrFrom4([4]byte(data[:4]))
	return &lsIPv4RouterIDLocal{Addr: addr}, nil
}

// --- TLV 1029: IPv6 Router-ID (Local) ---

// lsIPv6RouterIDLocal represents BGP-LS IPv6 Local Router-ID (TLV 1029).
// RFC 7752 Section 3.3.1.4: 16-byte IPv6 address.
type lsIPv6RouterIDLocal struct {
	Addr netip.Addr
}

func (t *lsIPv6RouterIDLocal) Code() uint16 { return bgpls.TLVIPv6RouterIDLocal }
func (t *lsIPv6RouterIDLocal) Len() int     { return 4 + 16 }

func (t *lsIPv6RouterIDLocal) WriteTo(buf []byte, off int) int {
	n := bgpls.WriteTLV(buf, off, bgpls.TLVIPv6RouterIDLocal, 16)
	b := t.Addr.As16()
	copy(buf[off+4:], b[:])
	return n
}

func (t *lsIPv6RouterIDLocal) ToJSON() map[string]any {
	return map[string]any{
		"local-router-ids": []string{t.Addr.String()},
	}
}

func decodeIPv6RouterIDLocal(data []byte) (lsAttrTLV, error) {
	// RFC 7752 Section 3.3.1.4: exactly 16 bytes
	if len(data) != 16 {
		return nil, bgpls.ErrBGPLSTruncated
	}
	addr := netip.AddrFrom16([16]byte(data[:16]))
	return &lsIPv6RouterIDLocal{Addr: addr}, nil
}

// --- Phase 2: SR-MPLS Node Attribute TLVs (RFC 9085) ---

// --- TLV 1034: SR Capabilities ---

// --- TLV 1035: SR Algorithm ---

// lsSRAlgorithm represents BGP-LS SR Algorithm (TLV 1035).
// RFC 9085 Section 4: variable-length array of algorithm IDs.
type lsSRAlgorithm struct {
	Algorithms []uint8
}

func (t *lsSRAlgorithm) Code() uint16 { return bgpls.TLVSRAlgorithm }
func (t *lsSRAlgorithm) Len() int     { return 4 + len(t.Algorithms) }

func (t *lsSRAlgorithm) WriteTo(buf []byte, off int) int {
	n := bgpls.WriteTLV(buf, off, bgpls.TLVSRAlgorithm, len(t.Algorithms))
	copy(buf[off+4:], t.Algorithms)
	return n
}

func (t *lsSRAlgorithm) ToJSON() map[string]any {
	algos := make([]int, len(t.Algorithms))
	for i, a := range t.Algorithms {
		algos[i] = int(a)
	}
	return map[string]any{"sr-algorithms": algos}
}

func decodeSRAlgorithm(data []byte) (lsAttrTLV, error) {
	algos := make([]uint8, len(data))
	copy(algos, data)
	return &lsSRAlgorithm{Algorithms: algos}, nil
}

// --- TLV 1036: SR Local Block ---

// lsSRLocalBlock represents BGP-LS SR Local Block (TLV 1036).
// RFC 9085 Section 5: Flags(1) + Reserved(1) + N x {Range(3) + SID/Label sub-TLV}.
// Same format as SR Capabilities but for SRLB ranges.
type lsSRLocalBlock struct {
	Flags  uint8
	Ranges []bgpls.LsSrLabelRange
}

func (t *lsSRLocalBlock) Code() uint16 { return bgpls.TLVSRLocalBlock }

func (t *lsSRLocalBlock) Len() int {
	return 4 + 2 + bgpls.SRLabelRangesLen(t.Ranges) // header + flags + reserved + ranges
}

func (t *lsSRLocalBlock) WriteTo(buf []byte, off int) int {
	valueLen := t.Len() - 4
	n := bgpls.WriteTLV(buf, off, bgpls.TLVSRLocalBlock, valueLen)
	vOff := off + 4
	buf[vOff] = t.Flags
	buf[vOff+1] = 0
	bgpls.WriteSRLabelRanges(buf, vOff+2, t.Ranges)
	return n
}

func (t *lsSRLocalBlock) ToJSON() map[string]any {
	return map[string]any{"sr-local-block": map[string]any{"ranges": bgpls.SRLabelRangesToJSON(t.Ranges)}}
}

func decodeSRLocalBlock(data []byte) (lsAttrTLV, error) {
	if len(data) < 2 {
		return nil, bgpls.ErrBGPLSTruncated
	}
	ranges, err := bgpls.DecodeSRLabelRanges(data[2:])
	if err != nil {
		return nil, err
	}
	return &lsSRLocalBlock{Flags: data[0], Ranges: ranges}, nil
}
