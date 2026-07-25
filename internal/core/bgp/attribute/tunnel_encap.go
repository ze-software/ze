// Design: docs/architecture/wire/attributes.md -- path attribute encoding
// RFC: rfc/short/rfc9012.md -- Tunnel Encapsulation Attribute (code 23)
//
// TunnelEncap implements the Tunnel Encapsulation attribute (RFC 9012).
//
// Wire format: concatenated Tunnel Type TLVs, each with:
//
//	[tunnel-type:2][length:2][value:length]
//
// The value of each TLV contains sub-TLVs specific to that tunnel type.
// Unknown tunnel types are preserved for forwarding (RFC 9012 Section 7).

package attribute

import (
	"encoding/binary"
	"fmt"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

const tunnelTLVHeaderLen = 4 // type(2) + length(2)

// TunnelTLV represents one Tunnel Type TLV within the Tunnel Encapsulation attribute.
// The Value field contains the raw sub-TLV bytes for this tunnel type.
type TunnelTLV struct {
	TunnelType uint16
	Value      []byte
}

// TunnelEncap is the Tunnel Encapsulation attribute (RFC 9012, code 23).
// Flags: Optional, Transitive.
type TunnelEncap struct {
	TLVs []TunnelTLV
}

// ParseTunnelEncap parses the attribute value into TunnelEncap.
// Unknown tunnel types are preserved as-is for forwarding.
func ParseTunnelEncap(data []byte) (*TunnelEncap, error) {
	te := &TunnelEncap{}
	for len(data) > 0 {
		if len(data) < tunnelTLVHeaderLen {
			return nil, fmt.Errorf("tunnel-encap: TLV header truncated (need %d, have %d)", tunnelTLVHeaderLen, len(data))
		}
		tunnelType := binary.BigEndian.Uint16(data[0:2])
		length := int(binary.BigEndian.Uint16(data[2:4]))
		if len(data) < tunnelTLVHeaderLen+length {
			return nil, fmt.Errorf("tunnel-encap: TLV value truncated for type %d (need %d, have %d)", tunnelType, length, len(data)-tunnelTLVHeaderLen)
		}
		value := make([]byte, length)
		copy(value, data[tunnelTLVHeaderLen:tunnelTLVHeaderLen+length])
		te.TLVs = append(te.TLVs, TunnelTLV{TunnelType: tunnelType, Value: value})
		data = data[tunnelTLVHeaderLen+length:]
	}
	return te, nil
}

func (te *TunnelEncap) Code() AttributeCode { return AttrTunnelEncap }

func (te *TunnelEncap) Flags() AttributeFlags { return FlagOptional | FlagTransitive }

// Len returns the total value length (all TLVs including their headers).
func (te *TunnelEncap) Len() int {
	n := 0
	for i := range te.TLVs {
		n += tunnelTLVHeaderLen + len(te.TLVs[i].Value)
	}
	return n
}

func (te *TunnelEncap) WriteTo(buf []byte, off int) int {
	pos := off
	for i := range te.TLVs {
		binary.BigEndian.PutUint16(buf[pos:], te.TLVs[i].TunnelType)
		binary.BigEndian.PutUint16(buf[pos+2:], uint16(len(te.TLVs[i].Value)))
		copy(buf[pos+tunnelTLVHeaderLen:], te.TLVs[i].Value)
		pos += tunnelTLVHeaderLen + len(te.TLVs[i].Value)
	}
	return pos - off
}

func (te *TunnelEncap) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return te.WriteTo(buf, off)
}

// Sub-TLV type constants (RFC 9012 Section 13, RFC 9830 Section 2.4).
const (
	SubTLVPreference  uint8 = 12  // RFC 9830 Section 2.4.1
	SubTLVBindingSID  uint8 = 13  // RFC 9830 Section 2.4.2
	SubTLVPriority    uint8 = 15  // RFC 9830 Section 2.4.6
	SubTLVSegmentList uint8 = 128 // RFC 9830 Section 2.4.5 (2-byte length)
)

// SubTLV is a single sub-TLV within a tunnel type's value.
// Returned by on-demand parsing; Value aliases the TunnelTLV.Value slice.
type SubTLV struct {
	Type  uint8
	Value []byte
}

// SubTLVs parses the sub-TLVs within this tunnel TLV on demand.
// RFC 9012: types 0-127 use 1-byte length, 128-255 use 2-byte length.
// Unknown sub-TLVs are included. Returns partial results plus error on truncation.
func (t *TunnelTLV) SubTLVs() ([]SubTLV, error) {
	data := t.Value
	var result []SubTLV
	for len(data) > 0 {
		if len(data) < 2 {
			return result, fmt.Errorf("tunnel-encap: sub-TLV header truncated (have %d)", len(data))
		}
		stype := data[0]
		var length int
		var hdrLen int
		// RFC 9012 Section 3: types 0-127 use 1-byte length, 128-255 use 2-byte length.
		if stype < 128 {
			hdrLen = 2
			length = int(data[1])
		} else {
			hdrLen = 3
			if len(data) < 3 {
				return result, fmt.Errorf("tunnel-encap: sub-TLV long header truncated for type %d (have %d)", stype, len(data))
			}
			length = int(binary.BigEndian.Uint16(data[1:3]))
		}
		if len(data) < hdrLen+length {
			return result, fmt.Errorf("tunnel-encap: sub-TLV type %d value truncated (need %d, have %d)", stype, length, len(data)-hdrLen)
		}
		result = append(result, SubTLV{Type: stype, Value: data[hdrLen : hdrLen+length]})
		data = data[hdrLen+length:]
	}
	return result, nil
}

// preferenceValueLen is the Preference sub-TLV value length RFC 9830 Section 2.4.1
// mandates: Flags(1) + RESERVED(1) + Preference(4).
const preferenceValueLen = 6

// Preference parses sub-TLVs on demand and returns the Preference value
// (RFC 9830 Section 2.4.1) if present. Returns 0, false if not found or
// the sub-TLV value is malformed.
//
// RFC 9012 Section 13: a malformed sub-TLV is treated as an unrecognized one, so a
// Preference sub-TLV whose value is not the mandated 6 octets is skipped rather than
// read at a guessed offset, and the search continues.
func (t *TunnelTLV) Preference() (uint32, bool) {
	stlvs, err := t.SubTLVs()
	if err != nil {
		return 0, false
	}
	for i := range stlvs {
		if stlvs[i].Type == SubTLVPreference && len(stlvs[i].Value) == preferenceValueLen {
			// flags(1) + reserved(1) + preference(4)
			return binary.BigEndian.Uint32(stlvs[i].Value[2:preferenceValueLen]), true
		}
	}
	return 0, false
}
