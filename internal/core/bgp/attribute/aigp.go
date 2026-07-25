// Design: docs/architecture/wire/attributes.md - path attribute encoding
// RFC: rfc/short/rfc7311.md - AIGP attribute (Accumulated IGP Metric)

package attribute

import (
	"encoding/binary"
	"strconv"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// AIGP TLV type codes (RFC 7311 Section 3).
const (
	aigpTLVTypeMetric = 1
	aigpTLVHeaderLen  = 3  // type (1) + length (2)
	aigpTLVMetricLen  = 11 // header (3) + metric (8)
)

// AIGPTLV represents a single TLV within an AIGP attribute.
type AIGPTLV struct {
	Type uint8
	Data []byte // for type 1: 8-byte big-endian metric; for unknown types: opaque
}

// AIGP represents the AIGP path attribute (RFC 7311).
//
// RFC 7311 Section 3: The AIGP attribute is an optional transitive attribute
// (type code 26) that carries accumulated IGP metric as a sequence of TLVs.
// Only TLV type 1 (AIGP metric) is defined; unknown types are preserved.
type AIGP struct {
	TLVs []AIGPTLV
}

func (a *AIGP) Code() AttributeCode   { return AttrAIGP }
func (a *AIGP) Flags() AttributeFlags { return FlagOptional | FlagTransitive }

// Len returns the total value length (sum of all TLV lengths).
func (a *AIGP) Len() int {
	n := 0
	for i := range a.TLVs {
		n += aigpTLVHeaderLen + len(a.TLVs[i].Data)
	}
	return n
}

// WriteTo writes the AIGP TLVs into buf at offset.
// RFC 7311 Section 3: each TLV is type(1) + length(2) + value(variable).
func (a *AIGP) WriteTo(buf []byte, off int) int {
	start := off
	for i := range a.TLVs {
		tlv := &a.TLVs[i]
		totalLen := aigpTLVHeaderLen + len(tlv.Data)
		buf[off] = tlv.Type
		binary.BigEndian.PutUint16(buf[off+1:], uint16(totalLen)) //nolint:gosec // bounded by BGP attr max
		off += aigpTLVHeaderLen
		off += copy(buf[off:], tlv.Data)
	}
	return off - start
}

// WriteToWithContext writes AIGP TLVs (context-independent).
func (a *AIGP) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return a.WriteTo(buf, off)
}

// CheckedWriteTo validates capacity before writing.
func (a *AIGP) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := a.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return a.WriteTo(buf, off), nil
}

// Metric returns the AIGP metric value from the first type-1 TLV.
// Returns 0, false if no metric TLV is present.
func (a *AIGP) Metric() (uint64, bool) {
	for i := range a.TLVs {
		if a.TLVs[i].Type == aigpTLVTypeMetric && len(a.TLVs[i].Data) == 8 {
			return binary.BigEndian.Uint64(a.TLVs[i].Data), true
		}
	}
	return 0, false
}

// ParseAIGP parses an AIGP attribute from wire bytes.
// RFC 7311 Section 3: sequence of TLVs, each with type(1) + length(2) + value.
// Type 1 TLV must have length 11. Unknown types are preserved as opaque.
func ParseAIGP(data []byte) (*AIGP, error) {
	if len(data) == 0 {
		return nil, ErrShortData
	}

	var tlvs []AIGPTLV
	off := 0
	for off < len(data) {
		// RFC 7311 Section 3: need at least 3 bytes for TLV header
		if off+aigpTLVHeaderLen > len(data) {
			return nil, ErrMalformedValue
		}

		tlvType := data[off]
		tlvLen := int(binary.BigEndian.Uint16(data[off+1:]))

		// RFC 7311: length includes the type and length fields themselves
		if tlvLen < aigpTLVHeaderLen {
			return nil, ErrMalformedValue
		}
		if off+tlvLen > len(data) {
			return nil, ErrMalformedValue
		}

		valueLen := tlvLen - aigpTLVHeaderLen

		// RFC 7311 Section 3: type 1 MUST have length 11 (3 header + 8 metric)
		if tlvType == aigpTLVTypeMetric && tlvLen != aigpTLVMetricLen {
			return nil, ErrMalformedValue
		}

		valueData := make([]byte, valueLen)
		copy(valueData, data[off+aigpTLVHeaderLen:off+tlvLen])

		tlvs = append(tlvs, AIGPTLV{
			Type: tlvType,
			Data: valueData,
		})

		off += tlvLen
	}

	return &AIGP{TLVs: tlvs}, nil
}

// NewAIGPMetric creates an AIGP attribute with a single type-1 metric TLV.
func NewAIGPMetric(metric uint64) *AIGP {
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, metric)
	return &AIGP{
		TLVs: []AIGPTLV{{Type: aigpTLVTypeMetric, Data: data}},
	}
}

// AIGPWireLen is the wire length of a single AIGP metric TLV value (no attr header).
const AIGPWireLen = aigpTLVMetricLen // 11 bytes: type(1) + length(2) + metric(8)

// WriteAIGPMetric writes a single AIGP metric TLV into buf at off.
// Returns 11 (aigpTLVMetricLen).
func WriteAIGPMetric(buf []byte, off int, metric uint64) int {
	buf[off] = aigpTLVTypeMetric
	binary.BigEndian.PutUint16(buf[off+1:], aigpTLVMetricLen)
	binary.BigEndian.PutUint64(buf[off+3:], metric)
	return aigpTLVMetricLen
}

// AppendText appends "aigp <metric>" to buf.
// If the attribute has no metric TLV, returns buf unchanged.
func (a *AIGP) AppendText(buf []byte) []byte {
	metric, ok := a.Metric()
	if !ok {
		return buf
	}
	buf = append(buf, "aigp "...)
	buf = strconv.AppendUint(buf, metric, 10)
	return buf
}
