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

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
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
