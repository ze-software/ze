// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — Configuration payload (Section 3.15)
package wire

import "encoding/binary"

// Configuration types (RFC 7296 Section 3.15).
const (
	CFGTypeRequest uint8 = 1
	CFGTypeReply   uint8 = 2
	CFGTypeSet     uint8 = 3
	CFGTypeACK     uint8 = 4
)

// Configuration attribute types (RFC 7296 Section 3.15.1).
const (
	CPAttrInternalIP4Address uint16 = 1
	CPAttrInternalIP4Netmask uint16 = 2
	CPAttrInternalIP4DNS     uint16 = 3
	CPAttrInternalIP4NBNS    uint16 = 4
	CPAttrInternalIP4DHCP    uint16 = 6
	CPAttrApplicationVersion uint16 = 7
	CPAttrInternalIP6Address uint16 = 8
	CPAttrInternalIP6DNS     uint16 = 10
	CPAttrInternalIP6DHCP    uint16 = 12
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
		binary.BigEndian.PutUint16(buf[off+n:], p.Attrs[i].Type)
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

func (p *PayloadCP) ReadFrom(data []byte) error {
	if len(data) < 4 {
		return ErrTruncated
	}
	p.CFGType = data[0]
	p.Attrs = nil
	off := 4
	for off+4 <= len(data) {
		atype := binary.BigEndian.Uint16(data[off:])
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
