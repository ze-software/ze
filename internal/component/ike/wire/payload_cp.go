// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — Configuration payload (Section 3.15)
package wire

import (
	"encoding/binary"
	"errors"
)

// CPAttrCountMax bounds the attributes one Configuration payload may carry. The payload is
// shaped by an unauthenticated-until-IKE_AUTH-completes peer, and every attribute costs an
// allocation, so the count is capped before the loop runs to completion. The largest request
// seen from strongSwan, Windows and macOS clients carries fewer than 20 attributes.
const CPAttrCountMax = 64

// ErrCPTooManyAttrs reports a Configuration payload carrying more than CPAttrCountMax
// attributes; the payload is refused as malformed.
var ErrCPTooManyAttrs = errors.New("ike: configuration payload carries more than 64 attributes")

// Configuration types (RFC 7296 Section 3.15).
const (
	CFGTypeRequest uint8 = 1
	CFGTypeReply   uint8 = 2
	CFGTypeSet     uint8 = 3
	CFGTypeACK     uint8 = 4
)

// cpAttrTypeMask isolates the Configuration Attribute Type from the octet pair that carries
// it. RFC 7296 Section 3.15.1 lays that pair out as |R| Attribute Type (15 bits) |, where R
// is "Reserved (1 bit) - This bit MUST be set to zero and MUST be ignored on receipt". A
// sixteen-bit read folds R into the type, so a peer that sets it turns INTERNAL_IP4_ADDRESS
// (1) into 0x8001 and the attribute reads as unrecognized. RFC 7296 Section 2.5 states the
// general rule this enforces: the content of a RESERVED field is ignored on receipt and sent
// as zero.
const cpAttrTypeMask uint16 = 0x7fff

// Configuration attribute types (RFC 7296 Section 3.15.1).
const (
	CPAttrInternalIP4Address uint16 = 1
	CPAttrInternalIP4Netmask uint16 = 2
	CPAttrInternalIP4DNS     uint16 = 3
	CPAttrInternalIP4NBNS    uint16 = 4
	// CPAttrInternalAddressExpiry (5) is obsolete: RFC 7296 Section 1.7 removed it, and a
	// receiver ignores it like any unrecognized attribute.
	CPAttrInternalAddressExpiry uint16 = 5
	CPAttrInternalIP4DHCP       uint16 = 6
	CPAttrApplicationVersion    uint16 = 7
	CPAttrInternalIP6Address    uint16 = 8
	CPAttrInternalIP6DNS        uint16 = 10
	CPAttrInternalIP6DHCP       uint16 = 12
	CPAttrInternalIP4Subnet     uint16 = 13
	CPAttrSupportedAttributes   uint16 = 14
	CPAttrInternalIP6Subnet     uint16 = 15
)

// ConfigAttr is a single configuration attribute.
type ConfigAttr struct {
	Type  uint16
	Value []byte
}

// PayloadCP is the Configuration payload (type 47).
type PayloadCP struct {
	CFGType uint8
	Attrs   []ConfigAttr
}

func (p *PayloadCP) Type() uint8 { return PayloadTypeCP }

func (p *PayloadCP) WriteTo(buf []byte, off int) int {
	buf[off] = p.CFGType
	buf[off+1] = 0 // reserved
	buf[off+2] = 0
	buf[off+3] = 0
	n := 4
	for i := range p.Attrs {
		// Send the Reserved bit as zero whatever the caller put in Type.
		binary.BigEndian.PutUint16(buf[off+n:], p.Attrs[i].Type&cpAttrTypeMask)
		binary.BigEndian.PutUint16(buf[off+n+2:], uint16(len(p.Attrs[i].Value)))
		copy(buf[off+n+4:], p.Attrs[i].Value)
		n += 4 + len(p.Attrs[i].Value)
	}
	return n
}

func (p *PayloadCP) Len() int {
	n := 4
	for i := range p.Attrs {
		n += 4 + len(p.Attrs[i].Value)
	}
	return n
}

// ReadFrom decodes a Configuration payload body. Every attribute is a 4-octet header and its
// value, so a trailing remnant shorter than a header is a truncated attribute and is refused,
// as is a payload carrying more than CPAttrCountMax attributes.
//
//	                     1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|   CFG Type    |                    RESERVED                   |  octets 0-3
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|R|         Attribute Type      |            Length             |  octets 4-7
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	~                             Value                             ~
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// (RFC 7296 Sections 3.15 and 3.15.1.)
func (p *PayloadCP) ReadFrom(data []byte) error {
	if len(data) < 4 {
		return ErrTruncated
	}
	p.CFGType = data[0]
	p.Attrs = nil
	off := 4
	for off < len(data) {
		if off+4 > len(data) {
			return ErrTruncated
		}
		if len(p.Attrs) == CPAttrCountMax {
			return ErrCPTooManyAttrs
		}
		// Ignore the Reserved bit on receipt: only the low 15 bits name the attribute.
		atype := binary.BigEndian.Uint16(data[off:]) & cpAttrTypeMask
		alen := int(binary.BigEndian.Uint16(data[off+2:]))
		off += 4
		if off+alen > len(data) {
			return ErrTruncated
		}
		val := make([]byte, alen)
		copy(val, data[off:off+alen])
		p.Attrs = append(p.Attrs, ConfigAttr{Type: atype, Value: val})
		off += alen
	}
	return nil
}
