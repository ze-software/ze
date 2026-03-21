// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS link attribute TLVs
// RFC: rfc/short/rfc7752.md — BGP-LS link attribute TLV types (Section 3.3.2)
// Overview: attr.go — TLV interface, registration, and iterator
// Related: attr_node.go — node attribute TLV types
// Related: attr_prefix.go — prefix attribute TLV types
// Related: attr_srv6.go — SRv6 attribute TLV types
package ls

import (
	"encoding/binary"
	"math"
	"net/netip"
)

// Link attribute TLV type codes.
// RFC 7752 Section 3.3.2 defines link attribute TLVs.
const (
	TLVIPv4RouterIDRemote uint16 = 1030 // RFC 7752 Section 3.3.2.1
	TLVIPv6RouterIDRemote uint16 = 1031 // RFC 7752 Section 3.3.2.1
	TLVAdminGroup         uint16 = 1088 // RFC 7752 Section 3.3.2.5
	TLVMaxLinkBandwidth   uint16 = 1089 // RFC 7752 Section 3.3.2.3
	TLVMaxReservableBW    uint16 = 1090 // RFC 7752 Section 3.3.2.4
	TLVUnreservedBW       uint16 = 1091 // RFC 7752 Section 3.3.2.4
	TLVTEDefaultMetric    uint16 = 1092 // RFC 7752 Section 3.3.2.7
	TLVIGPMetric          uint16 = 1095 // RFC 7752 Section 3.3.2.4
	TLVSRLG               uint16 = 1096 // RFC 7752 Section 3.3.2.6
	TLVOpaqueLinkAttr     uint16 = 1097 // RFC 7752 Section 3.3.2.10
	TLVLinkName           uint16 = 1098 // RFC 7752 Section 3.3.2.7
)

// --- TLV 1030: IPv4 Router-ID (Remote) ---

// LsIPv4RouterIDRemote represents BGP-LS IPv4 Remote Router-ID (TLV 1030).
// RFC 7752 Section 3.3.2.1: 4-byte IPv4 address.
type LsIPv4RouterIDRemote struct {
	Addr netip.Addr
}

func (t *LsIPv4RouterIDRemote) Code() uint16 { return TLVIPv4RouterIDRemote }
func (t *LsIPv4RouterIDRemote) Len() int     { return 4 + 4 }

func (t *LsIPv4RouterIDRemote) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVIPv4RouterIDRemote, 4)
	b := t.Addr.As4()
	copy(buf[off+4:], b[:])
	return n
}

func (t *LsIPv4RouterIDRemote) ToJSON() map[string]any {
	return map[string]any{
		"remote-router-ids": []string{t.Addr.String()},
	}
}

func decodeIPv4RouterIDRemote(data []byte) (LsAttrTLV, error) {
	if len(data) != 4 {
		return nil, ErrBGPLSTruncated
	}
	return &LsIPv4RouterIDRemote{Addr: netip.AddrFrom4([4]byte(data[:4]))}, nil
}

// --- TLV 1031: IPv6 Router-ID (Remote) ---

// LsIPv6RouterIDRemote represents BGP-LS IPv6 Remote Router-ID (TLV 1031).
// RFC 7752 Section 3.3.2.1: 16-byte IPv6 address.
type LsIPv6RouterIDRemote struct {
	Addr netip.Addr
}

func (t *LsIPv6RouterIDRemote) Code() uint16 { return TLVIPv6RouterIDRemote }
func (t *LsIPv6RouterIDRemote) Len() int     { return 4 + 16 }

func (t *LsIPv6RouterIDRemote) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVIPv6RouterIDRemote, 16)
	b := t.Addr.As16()
	copy(buf[off+4:], b[:])
	return n
}

func (t *LsIPv6RouterIDRemote) ToJSON() map[string]any {
	return map[string]any{
		"remote-router-ids": []string{t.Addr.String()},
	}
}

func decodeIPv6RouterIDRemote(data []byte) (LsAttrTLV, error) {
	if len(data) != 16 {
		return nil, ErrBGPLSTruncated
	}
	return &LsIPv6RouterIDRemote{Addr: netip.AddrFrom16([16]byte(data[:16]))}, nil
}

// --- TLV 1088: Administrative Group ---

// LsAdminGroup represents BGP-LS Administrative Group (TLV 1088).
// RFC 7752 Section 3.3.2.5: 4-byte bit mask.
type LsAdminGroup struct {
	Mask uint32
}

func (t *LsAdminGroup) Code() uint16 { return TLVAdminGroup }
func (t *LsAdminGroup) Len() int     { return 4 + 4 }

func (t *LsAdminGroup) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVAdminGroup, 4)
	binary.BigEndian.PutUint32(buf[off+4:], t.Mask)
	return n
}

func (t *LsAdminGroup) ToJSON() map[string]any {
	return map[string]any{"admin-group": t.Mask}
}

func decodeAdminGroup(data []byte) (LsAttrTLV, error) {
	if len(data) < 4 {
		return nil, ErrBGPLSTruncated
	}
	return &LsAdminGroup{Mask: binary.BigEndian.Uint32(data)}, nil
}

// --- TLV 1089: Maximum Link Bandwidth ---

// LsMaxLinkBandwidth represents BGP-LS Max Link Bandwidth (TLV 1089).
// RFC 7752 Section 3.3.2.3: 4-byte IEEE 754 float32 (bytes/sec).
type LsMaxLinkBandwidth struct {
	Bandwidth float32
}

func (t *LsMaxLinkBandwidth) Code() uint16 { return TLVMaxLinkBandwidth }
func (t *LsMaxLinkBandwidth) Len() int     { return 4 + 4 }

func (t *LsMaxLinkBandwidth) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVMaxLinkBandwidth, 4)
	binary.BigEndian.PutUint32(buf[off+4:], math.Float32bits(t.Bandwidth))
	return n
}

func (t *LsMaxLinkBandwidth) ToJSON() map[string]any {
	return map[string]any{"max-link-bandwidth": float64(t.Bandwidth)}
}

func decodeMaxLinkBandwidth(data []byte) (LsAttrTLV, error) {
	if len(data) < 4 {
		return nil, ErrBGPLSTruncated
	}
	return &LsMaxLinkBandwidth{Bandwidth: math.Float32frombits(binary.BigEndian.Uint32(data))}, nil
}

// --- TLV 1090: Maximum Reservable Link Bandwidth ---

// LsMaxReservableBW represents BGP-LS Max Reservable Bandwidth (TLV 1090).
// RFC 7752 Section 3.3.2.4: 4-byte IEEE 754 float32 (bytes/sec).
type LsMaxReservableBW struct {
	Bandwidth float32
}

func (t *LsMaxReservableBW) Code() uint16 { return TLVMaxReservableBW }
func (t *LsMaxReservableBW) Len() int     { return 4 + 4 }

func (t *LsMaxReservableBW) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVMaxReservableBW, 4)
	binary.BigEndian.PutUint32(buf[off+4:], math.Float32bits(t.Bandwidth))
	return n
}

func (t *LsMaxReservableBW) ToJSON() map[string]any {
	return map[string]any{"max-reservable-bandwidth": float64(t.Bandwidth)}
}

func decodeMaxReservableBW(data []byte) (LsAttrTLV, error) {
	if len(data) < 4 {
		return nil, ErrBGPLSTruncated
	}
	return &LsMaxReservableBW{Bandwidth: math.Float32frombits(binary.BigEndian.Uint32(data))}, nil
}

// --- TLV 1091: Unreserved Bandwidth ---

// LsUnreservedBW represents BGP-LS Unreserved Bandwidth (TLV 1091).
// RFC 7752 Section 3.3.2.4: 8 x IEEE 754 float32 (one per priority level).
type LsUnreservedBW struct {
	Bandwidth [8]float32
}

func (t *LsUnreservedBW) Code() uint16 { return TLVUnreservedBW }
func (t *LsUnreservedBW) Len() int     { return 4 + 32 }

func (t *LsUnreservedBW) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVUnreservedBW, 32)
	for i := range 8 {
		binary.BigEndian.PutUint32(buf[off+4+i*4:], math.Float32bits(t.Bandwidth[i]))
	}
	return n
}

func (t *LsUnreservedBW) ToJSON() map[string]any {
	bws := make([]float64, 8)
	for i := range 8 {
		bws[i] = float64(t.Bandwidth[i])
	}
	return map[string]any{"unreserved-bandwidth": bws}
}

func decodeUnreservedBW(data []byte) (LsAttrTLV, error) {
	if len(data) < 32 {
		return nil, ErrBGPLSTruncated
	}
	var bw [8]float32
	for i := range 8 {
		bw[i] = math.Float32frombits(binary.BigEndian.Uint32(data[i*4:]))
	}
	return &LsUnreservedBW{Bandwidth: bw}, nil
}

// --- TLV 1092: TE Default Metric ---

// LsTEDefaultMetric represents BGP-LS TE Default Metric (TLV 1092).
// RFC 7752 Section 3.3.2.7: 4-byte unsigned integer.
type LsTEDefaultMetric struct {
	Metric uint32
}

func (t *LsTEDefaultMetric) Code() uint16 { return TLVTEDefaultMetric }
func (t *LsTEDefaultMetric) Len() int     { return 4 + 4 }

func (t *LsTEDefaultMetric) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVTEDefaultMetric, 4)
	binary.BigEndian.PutUint32(buf[off+4:], t.Metric)
	return n
}

func (t *LsTEDefaultMetric) ToJSON() map[string]any {
	return map[string]any{"te-metric": t.Metric}
}

func decodeTEDefaultMetric(data []byte) (LsAttrTLV, error) {
	if len(data) < 4 {
		return nil, ErrBGPLSTruncated
	}
	return &LsTEDefaultMetric{Metric: binary.BigEndian.Uint32(data)}, nil
}

// --- TLV 1095: IGP Metric ---

// LsIGPMetric represents BGP-LS IGP Metric (TLV 1095).
// RFC 7752 Section 3.3.2.4: variable-length (1-4 bytes).
// IS-IS small metric: 1 byte (6 bits), OSPF: 2 bytes, IS-IS wide: 3 bytes, generic: 4 bytes.
type LsIGPMetric struct {
	Metric uint32
	// wireLen preserves original encoding length for round-trip fidelity.
	wireLen int
}

func (t *LsIGPMetric) Code() uint16 { return TLVIGPMetric }

func (t *LsIGPMetric) Len() int {
	if t.wireLen > 0 {
		return 4 + t.wireLen
	}
	return 4 + igpMetricValueLen(t.Metric)
}

// igpMetricValueLen returns the smallest encoding length for the given metric value.
// RFC 7752 Section 3.3.2.4: 1 byte (IS-IS small, 6 bits), 2 bytes (OSPF), 3 bytes (IS-IS wide).
func igpMetricValueLen(metric uint32) int {
	if metric <= 0x3F {
		return 1
	}
	if metric <= 0xFFFF {
		return 2
	}
	return 3 // IS-IS wide metric (max 24 bits)
}

func (t *LsIGPMetric) WriteTo(buf []byte, off int) int {
	valueLen := t.Len() - 4
	n := writeTLV(buf, off, TLVIGPMetric, valueLen)
	writeIGPMetricValue(buf, off+4, t.Metric, valueLen)
	return n
}

// writeIGPMetricValue encodes the metric at the given offset with the specified length.
func writeIGPMetricValue(buf []byte, off int, metric uint32, valueLen int) {
	switch valueLen {
	case 1:
		buf[off] = byte(metric & 0x3F)
	case 2:
		binary.BigEndian.PutUint16(buf[off:], uint16(metric)) //nolint:gosec // value checked by caller
	case 3:
		buf[off] = byte(metric >> 16)
		buf[off+1] = byte(metric >> 8)
		buf[off+2] = byte(metric)
	case 4:
		binary.BigEndian.PutUint32(buf[off:], metric)
	}
}

func (t *LsIGPMetric) ToJSON() map[string]any {
	return map[string]any{"igp-metric": int(t.Metric)}
}

func decodeIGPMetric(data []byte) (LsAttrTLV, error) {
	// RFC 7752 Section 3.3.2.4: 1 byte (IS-IS small), 2 bytes (OSPF), or 3 bytes (IS-IS wide)
	if len(data) == 0 || len(data) > 3 {
		return nil, ErrBGPLSTruncated
	}
	var metric uint32
	switch len(data) {
	case 1:
		metric = uint32(data[0] & 0x3F)
	case 2:
		metric = uint32(binary.BigEndian.Uint16(data))
	case 3:
		metric = uint32(data[0])<<16 | uint32(data[1])<<8 | uint32(data[2])
	}
	return &LsIGPMetric{Metric: metric, wireLen: len(data)}, nil
}

// --- TLV 1096: SRLG ---

// LsSRLG represents BGP-LS Shared Risk Link Group (TLV 1096).
// RFC 7752 Section 3.3.2.6: array of 4-byte uint32 values.
type LsSRLG struct {
	Values []uint32
}

func (t *LsSRLG) Code() uint16 { return TLVSRLG }
func (t *LsSRLG) Len() int     { return 4 + len(t.Values)*4 }

func (t *LsSRLG) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVSRLG, len(t.Values)*4)
	for i, v := range t.Values {
		binary.BigEndian.PutUint32(buf[off+4+i*4:], v)
	}
	return n
}

func (t *LsSRLG) ToJSON() map[string]any {
	return map[string]any{"srlgs": t.Values}
}

func decodeSRLG(data []byte) (LsAttrTLV, error) {
	if len(data)%4 != 0 {
		return nil, ErrBGPLSTruncated
	}
	values := make([]uint32, len(data)/4)
	for i := range values {
		values[i] = binary.BigEndian.Uint32(data[i*4:])
	}
	return &LsSRLG{Values: values}, nil
}

// --- TLV 1097: Opaque Link Attribute ---

// LsOpaqueLinkAttr represents BGP-LS Opaque Link Attribute (TLV 1097).
// RFC 7752 Section 3.3.2.10: variable-length opaque data.
type LsOpaqueLinkAttr struct {
	Data []byte
}

func (t *LsOpaqueLinkAttr) Code() uint16 { return TLVOpaqueLinkAttr }
func (t *LsOpaqueLinkAttr) Len() int     { return 4 + len(t.Data) }

func (t *LsOpaqueLinkAttr) WriteTo(buf []byte, off int) int {
	return writeTLVBytes(buf, off, TLVOpaqueLinkAttr, t.Data)
}

func (t *LsOpaqueLinkAttr) ToJSON() map[string]any {
	return map[string]any{"opaque-link-attr": formatHex(t.Data)}
}

func decodeOpaqueLinkAttr(data []byte) (LsAttrTLV, error) {
	cp := make([]byte, len(data))
	copy(cp, data)
	return &LsOpaqueLinkAttr{Data: cp}, nil
}

// --- TLV 1098: Link Name ---

// LsLinkName represents BGP-LS Link Name (TLV 1098).
// RFC 7752 Section 3.3.2.7: variable-length UTF-8 string.
type LsLinkName struct {
	Name string
}

func (t *LsLinkName) Code() uint16 { return TLVLinkName }
func (t *LsLinkName) Len() int     { return 4 + len(t.Name) }

func (t *LsLinkName) WriteTo(buf []byte, off int) int {
	return writeTLVBytes(buf, off, TLVLinkName, []byte(t.Name))
}

func (t *LsLinkName) ToJSON() map[string]any {
	return map[string]any{"link-name": t.Name}
}

func decodeLinkName(data []byte) (LsAttrTLV, error) {
	return &LsLinkName{Name: string(data)}, nil
}

// --- Phase 2: SR-MPLS Link Attribute TLV (RFC 9085) ---

// SR-MPLS link attribute TLV type code.
const (
	TLVAdjacencySID uint16 = 1099 // RFC 9085 Section 2.2.1
)

// --- TLV 1099: Adjacency SID ---

// LsAdjacencySID represents BGP-LS Adjacency SID (TLV 1099).
// RFC 9085 Section 2.2.1: Flags(1) + Weight(1) + Reserved(2) + SID (3 or 4 bytes).
// V=1,L=1: 3-byte label (20 bits). V=0,L=0: 4-byte index.
type LsAdjacencySID struct {
	Flags  uint8
	Weight uint8
	SID    uint32
	// wireLen preserves 7 vs 8 byte total value for round-trip.
	wireLen int
}

func (t *LsAdjacencySID) Code() uint16 { return TLVAdjacencySID }

func (t *LsAdjacencySID) Len() int {
	if t.wireLen > 0 {
		return 4 + t.wireLen
	}
	return 4 + 8 // flags(1) + weight(1) + reserved(2) + SID(4)
}

func (t *LsAdjacencySID) WriteTo(buf []byte, off int) int {
	valueLen := t.Len() - 4
	n := writeTLV(buf, off, TLVAdjacencySID, valueLen)
	vOff := off + 4
	buf[vOff] = t.Flags
	buf[vOff+1] = t.Weight
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

func (t *LsAdjacencySID) ToJSON() map[string]any {
	return map[string]any{
		"adj-sids": []map[string]any{{
			"flags": map[string]any{
				"F":   int((t.Flags >> 7) & 1),
				"B":   int((t.Flags >> 6) & 1),
				"V":   int((t.Flags >> 5) & 1),
				"L":   int((t.Flags >> 4) & 1),
				"S":   int((t.Flags >> 3) & 1),
				"P":   int((t.Flags >> 2) & 1),
				"RSV": int(t.Flags & 0x03),
			},
			"weight":         int(t.Weight),
			"sids":           []int{int(t.SID)},
			"undecoded-sids": []string{},
		}},
	}
}

func decodeAdjacencySID(data []byte) (LsAttrTLV, error) {
	// RFC 9085 Section 2.2.1: 7 or 8 bytes
	if len(data) != 7 && len(data) != 8 {
		return nil, ErrBGPLSTruncated
	}
	adj := &LsAdjacencySID{
		Flags:   data[0],
		Weight:  data[1],
		wireLen: len(data),
	}
	// data[2:4] reserved
	v := data[4:]
	if len(v) == 4 {
		adj.SID = binary.BigEndian.Uint32(v)
	} else {
		// RFC 9085 Section 2.2.1: 3-byte label, 20 rightmost bits
		adj.SID = (uint32(v[0])<<16 | uint32(v[1])<<8 | uint32(v[2])) & 0xFFFFF
	}
	return adj, nil
}

// --- Phase 3: EPE Peer SIDs (RFC 9086) ---

const (
	TLVPeerNodeSID uint16 = 1101 // RFC 9086 Section 5
	TLVPeerAdjSID  uint16 = 1102 // RFC 9086 Section 5
	TLVPeerSetSID  uint16 = 1103 // RFC 9086 Section 5
)

// LsPeerSID represents a BGP-EPE Peer SID (TLVs 1101-1103).
// RFC 9086 Section 5: Flags(1) + Weight(1) + Reserved(2) + SID (3 or 4 bytes).
// Same wire format as Adjacency SID but different TLV codes and JSON keys.
type LsPeerSID struct {
	TLVCode uint16
	Flags   uint8
	Weight  uint8
	SID     uint32
	wireLen int
}

func (t *LsPeerSID) Code() uint16 { return t.TLVCode }

func (t *LsPeerSID) Len() int {
	if t.wireLen > 0 {
		return 4 + t.wireLen
	}
	return 4 + 8
}

func (t *LsPeerSID) WriteTo(buf []byte, off int) int {
	valueLen := t.Len() - 4
	n := writeTLV(buf, off, t.TLVCode, valueLen)
	vOff := off + 4
	buf[vOff] = t.Flags
	buf[vOff+1] = t.Weight
	buf[vOff+2] = 0
	buf[vOff+3] = 0
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

// peerSIDKey returns the JSON key for a Peer SID TLV code.
var peerSIDKeys = map[uint16]string{
	TLVPeerNodeSID: "peer-node-sid",
	TLVPeerAdjSID:  "peer-adj-sid",
	TLVPeerSetSID:  "peer-set-sid",
}

func (t *LsPeerSID) ToJSON() map[string]any {
	key := peerSIDKeys[t.TLVCode]
	return map[string]any{
		key: map[string]any{
			"flags":  int(t.Flags),
			"weight": int(t.Weight),
			"sid":    t.SID,
		},
	}
}

func decodePeerSID(code uint16) LsAttrTLVDecoder {
	return func(data []byte) (LsAttrTLV, error) {
		if len(data) != 7 && len(data) != 8 {
			return nil, ErrBGPLSTruncated
		}
		ps := &LsPeerSID{
			TLVCode: code,
			Flags:   data[0],
			Weight:  data[1],
			wireLen: len(data),
		}
		v := data[4:]
		if len(v) == 4 {
			ps.SID = binary.BigEndian.Uint32(v)
		} else {
			ps.SID = (uint32(v[0])<<16 | uint32(v[1])<<8 | uint32(v[2])) & 0xFFFFF
		}
		return ps, nil
	}
}

// --- Phase 3: SRv6 End.X SID (RFC 9514) ---

const (
	TLVSRv6EndXSID     uint16 = 1106 // RFC 9514 Section 4
	TLVSRv6LANEndXISIS uint16 = 1107 // RFC 9514 Section 4 (IS-IS)
	TLVSRv6LANEndXOSPF uint16 = 1108 // RFC 9514 Section 4 (OSPFv3)
)

// srv6EndXKeys maps SRv6 End.X SID TLV codes to JSON keys.
var srv6EndXKeys = map[uint16]string{
	TLVSRv6EndXSID:     "srv6-endx-sids",
	TLVSRv6LANEndXISIS: "srv6-lan-endx-isis",
	TLVSRv6LANEndXOSPF: "srv6-lan-endx-ospf",
}

// LsSRv6EndXSID represents SRv6 End.X SID (TLV 1106) or LAN variants (1107/1108).
// RFC 9514: Behavior(2) + Flags(1) + Algorithm(1) + Weight(1) + Reserved(1) +
// [NeighborID(variable)] + SID(16) + [sub-TLVs].
type LsSRv6EndXSID struct {
	TLVCode          uint16
	EndpointBehavior uint16
	Flags            uint8
	Algorithm        uint8
	Weight           uint8
	NeighborID       []byte // 0 for End.X, 6 for IS-IS LAN, 4 for OSPFv3 LAN
	SID              [16]byte
	SIDStructure     [4]uint8 // loc_block, loc_node, func, arg (from sub-TLV 1252)
	hasSIDStructure  bool
}

func (t *LsSRv6EndXSID) Code() uint16 { return t.TLVCode }

func (t *LsSRv6EndXSID) Len() int {
	n := 4 + 6 + len(t.NeighborID) + 16 // header + fixed fields + neighbor + SID
	if t.hasSIDStructure {
		n += 4 + 4 // sub-TLV header + 4 bytes
	}
	return n
}

func (t *LsSRv6EndXSID) WriteTo(buf []byte, off int) int {
	valueLen := t.Len() - 4
	n := writeTLV(buf, off, t.TLVCode, valueLen)
	vOff := off + 4
	binary.BigEndian.PutUint16(buf[vOff:], t.EndpointBehavior)
	buf[vOff+2] = t.Flags
	buf[vOff+3] = t.Algorithm
	buf[vOff+4] = t.Weight
	buf[vOff+5] = 0 // reserved
	vOff += 6

	if len(t.NeighborID) > 0 {
		copy(buf[vOff:], t.NeighborID)
		vOff += len(t.NeighborID)
	}

	copy(buf[vOff:], t.SID[:])
	vOff += 16

	if t.hasSIDStructure {
		binary.BigEndian.PutUint16(buf[vOff:], 1252) // SRv6 SID Structure sub-TLV
		binary.BigEndian.PutUint16(buf[vOff+2:], 4)  // length
		buf[vOff+4] = t.SIDStructure[0]              // loc_block_len
		buf[vOff+5] = t.SIDStructure[1]              // loc_node_len
		buf[vOff+6] = t.SIDStructure[2]              // func_len
		buf[vOff+7] = t.SIDStructure[3]              // arg_len
	}

	return n
}

func (t *LsSRv6EndXSID) ToJSON() map[string]any {
	addr := netip.AddrFrom16(t.SID)
	entry := map[string]any{
		"behavior":  int(t.EndpointBehavior),
		"algorithm": int(t.Algorithm),
		"weight":    int(t.Weight),
		"flags": map[string]any{
			"B":   int((t.Flags >> 7) & 1),
			"S":   int((t.Flags >> 6) & 1),
			"P":   int((t.Flags >> 5) & 1),
			"RSV": int(t.Flags & 0x1F),
		},
		"sid": addr.String(),
	}
	if len(t.NeighborID) > 0 {
		entry["neighbor-id"] = formatHex(t.NeighborID)[2:] // without 0x prefix
	}
	if t.hasSIDStructure {
		entry["srv6-sid-structure"] = map[string]any{
			"loc-block-len": int(t.SIDStructure[0]),
			"loc-node-len":  int(t.SIDStructure[1]),
			"func-len":      int(t.SIDStructure[2]),
			"arg-len":       int(t.SIDStructure[3]),
		}
	}

	key := srv6EndXKeys[t.TLVCode]
	return map[string]any{key: []map[string]any{entry}}
}

func decodeSRv6EndXSID(code uint16, neighborIDLen int) LsAttrTLVDecoder {
	return func(data []byte) (LsAttrTLV, error) {
		minLen := 6 + neighborIDLen + 16
		if len(data) < minLen {
			return nil, ErrBGPLSTruncated
		}
		t := &LsSRv6EndXSID{
			TLVCode:          code,
			EndpointBehavior: binary.BigEndian.Uint16(data[0:2]),
			Flags:            data[2],
			Algorithm:        data[3],
			Weight:           data[4],
		}
		off := 6
		if neighborIDLen > 0 {
			t.NeighborID = make([]byte, neighborIDLen)
			copy(t.NeighborID, data[off:off+neighborIDLen])
			off += neighborIDLen
		}
		copy(t.SID[:], data[off:off+16])
		off += 16

		// Parse optional sub-TLVs (SRv6 SID Structure = 1252)
		if off+8 <= len(data) {
			subType := binary.BigEndian.Uint16(data[off : off+2])
			subLen := binary.BigEndian.Uint16(data[off+2 : off+4])
			if subType == 1252 && subLen == 4 {
				t.SIDStructure[0] = data[off+4]
				t.SIDStructure[1] = data[off+5]
				t.SIDStructure[2] = data[off+6]
				t.SIDStructure[3] = data[off+7]
				t.hasSIDStructure = true
			}
		}
		return t, nil
	}
}

// --- Phase 3: Delay Metrics (RFC 8571) ---

const (
	TLVUnidirectionalDelay uint16 = 1114 // RFC 8571 Section 3
	TLVMinMaxDelay         uint16 = 1115 // RFC 8571 Section 4
	TLVDelayVariation      uint16 = 1116 // RFC 8571 Section 5
)

// LsUnidirectionalDelay represents unidirectional link delay (TLV 1114).
// RFC 8571 Section 3: A-flag(1 bit) + Reserved(7 bits) + Delay(24 bits, microseconds).
type LsUnidirectionalDelay struct {
	Anomalous bool
	Delay     uint32 // microseconds, 24 bits
}

func (t *LsUnidirectionalDelay) Code() uint16 { return TLVUnidirectionalDelay }
func (t *LsUnidirectionalDelay) Len() int     { return 4 + 4 }

func (t *LsUnidirectionalDelay) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVUnidirectionalDelay, 4)
	vOff := off + 4
	var flagByte uint8
	if t.Anomalous {
		flagByte = 0x80
	}
	buf[vOff] = flagByte
	buf[vOff+1] = byte(t.Delay >> 16)
	buf[vOff+2] = byte(t.Delay >> 8)
	buf[vOff+3] = byte(t.Delay)
	return n
}

func (t *LsUnidirectionalDelay) ToJSON() map[string]any {
	return map[string]any{
		"link-delay": map[string]any{
			"anomalous": t.Anomalous,
			"delay":     t.Delay,
		},
	}
}

func decodeUnidirectionalDelay(data []byte) (LsAttrTLV, error) {
	if len(data) != 4 {
		return nil, ErrBGPLSTruncated
	}
	return &LsUnidirectionalDelay{
		Anomalous: data[0]&0x80 != 0,
		Delay:     uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3]),
	}, nil
}

// LsMinMaxDelay represents min/max unidirectional link delay (TLV 1115).
// RFC 8571 Section 4: A-flag + Reserved + MinDelay(24) + Reserved(8) + MaxDelay(24).
type LsMinMaxDelay struct {
	Anomalous bool
	MinDelay  uint32
	MaxDelay  uint32
}

func (t *LsMinMaxDelay) Code() uint16 { return TLVMinMaxDelay }
func (t *LsMinMaxDelay) Len() int     { return 4 + 8 }

func (t *LsMinMaxDelay) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVMinMaxDelay, 8)
	vOff := off + 4
	var flagByte uint8
	if t.Anomalous {
		flagByte = 0x80
	}
	buf[vOff] = flagByte
	buf[vOff+1] = byte(t.MinDelay >> 16)
	buf[vOff+2] = byte(t.MinDelay >> 8)
	buf[vOff+3] = byte(t.MinDelay)
	buf[vOff+4] = 0 // reserved
	buf[vOff+5] = byte(t.MaxDelay >> 16)
	buf[vOff+6] = byte(t.MaxDelay >> 8)
	buf[vOff+7] = byte(t.MaxDelay)
	return n
}

func (t *LsMinMaxDelay) ToJSON() map[string]any {
	return map[string]any{
		"link-delay-minmax": map[string]any{
			"anomalous": t.Anomalous,
			"min-delay": t.MinDelay,
			"max-delay": t.MaxDelay,
		},
	}
}

func decodeMinMaxDelay(data []byte) (LsAttrTLV, error) {
	if len(data) != 8 {
		return nil, ErrBGPLSTruncated
	}
	return &LsMinMaxDelay{
		Anomalous: data[0]&0x80 != 0,
		MinDelay:  uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3]),
		MaxDelay:  uint32(data[5])<<16 | uint32(data[6])<<8 | uint32(data[7]),
	}, nil
}

// LsDelayVariation represents unidirectional delay variation (TLV 1116).
// RFC 8571 Section 5: Reserved(8) + Variation(24 bits, microseconds).
type LsDelayVariation struct {
	Variation uint32
}

func (t *LsDelayVariation) Code() uint16 { return TLVDelayVariation }
func (t *LsDelayVariation) Len() int     { return 4 + 4 }

func (t *LsDelayVariation) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVDelayVariation, 4)
	vOff := off + 4
	buf[vOff] = 0 // reserved
	buf[vOff+1] = byte(t.Variation >> 16)
	buf[vOff+2] = byte(t.Variation >> 8)
	buf[vOff+3] = byte(t.Variation)
	return n
}

func (t *LsDelayVariation) ToJSON() map[string]any {
	return map[string]any{"delay-variation": t.Variation}
}

func decodeDelayVariation(data []byte) (LsAttrTLV, error) {
	if len(data) != 4 {
		return nil, ErrBGPLSTruncated
	}
	return &LsDelayVariation{
		Variation: uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3]),
	}, nil
}
