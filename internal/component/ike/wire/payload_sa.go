// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — SA payload (Section 3.3)
package wire

import "encoding/binary"

// Protocol IDs (RFC 7296 Section 3.3.1).
const (
	ProtocolIKE uint8 = 1
	ProtocolAH  uint8 = 2
	ProtocolESP uint8 = 3
)

// Transform types (RFC 7296 Section 3.3.2).
const (
	TransformTypeENCR uint8 = 1
	TransformTypePRF  uint8 = 2
	TransformTypeINTG uint8 = 3
	TransformTypeDH   uint8 = 4
	TransformTypeESN  uint8 = 5
)

// Transform attribute type (RFC 7296 Section 3.3.5).
const AttrTypeKeyLength uint16 = 14

// TransformAttr is a transform attribute (type-value, always TV format for key length).
type TransformAttr struct {
	Type  uint16
	Value uint16
}

// Transform is a single cryptographic transform within a proposal.
type Transform struct {
	IsLast bool
	Type   uint8
	ID     uint16
	Attrs  []TransformAttr
}

const transformHeaderLen = 8

func (t *Transform) WriteTo(buf []byte, off int) int {
	start := off
	if t.IsLast {
		buf[off] = 0
	} else {
		buf[off] = 3
	}
	buf[off+1] = 0
	// skip length at off+2..off+3, backfill
	buf[off+4] = t.Type
	buf[off+5] = 0
	binary.BigEndian.PutUint16(buf[off+6:], t.ID)
	off += transformHeaderLen
	for i := range t.Attrs {
		// TV format: high bit set on type
		binary.BigEndian.PutUint16(buf[off:], t.Attrs[i].Type|0x8000)
		binary.BigEndian.PutUint16(buf[off+2:], t.Attrs[i].Value)
		off += 4
	}
	length := off - start
	binary.BigEndian.PutUint16(buf[start+2:], uint16(length))
	return length
}

// length reports the bytes Transform.WriteTo writes: the fixed transform
// header plus 4 bytes per TV-format attribute.
func (t *Transform) length() int {
	return transformHeaderLen + 4*len(t.Attrs)
}

func (t *Transform) ReadFrom(data []byte) error {
	if len(data) < transformHeaderLen {
		return ErrTruncated
	}
	t.IsLast = data[0] == 0
	tlen := int(binary.BigEndian.Uint16(data[2:4]))
	if tlen < transformHeaderLen || tlen > len(data) {
		return ErrTruncated
	}
	t.Type = data[4]
	t.ID = binary.BigEndian.Uint16(data[6:8])
	off := transformHeaderLen
	t.Attrs = nil
	for off+4 <= tlen {
		atype := binary.BigEndian.Uint16(data[off:])
		aval := binary.BigEndian.Uint16(data[off+2:])
		t.Attrs = append(t.Attrs, TransformAttr{
			Type:  atype & 0x7fff,
			Value: aval,
		})
		off += 4
	}
	return nil
}

// Proposal is a single proposal within an SA payload.
type Proposal struct {
	IsLast     bool
	Number     uint8
	ProtocolID uint8
	SPISize    uint8
	SPI        []byte
	Transforms []Transform
}

const proposalHeaderLen = 8

func (p *Proposal) WriteTo(buf []byte, off int) int {
	start := off
	if p.IsLast {
		buf[off] = 0
	} else {
		buf[off] = 2
	}
	buf[off+1] = 0
	// skip length at off+2..off+3, backfill
	buf[off+4] = p.Number
	buf[off+5] = p.ProtocolID
	buf[off+6] = p.SPISize
	buf[off+7] = byte(len(p.Transforms))
	off += proposalHeaderLen
	if p.SPISize > 0 && len(p.SPI) >= int(p.SPISize) {
		copy(buf[off:], p.SPI[:p.SPISize])
		off += int(p.SPISize)
	} else if p.SPISize > 0 {
		buf[start+6] = 0
	}
	for i := range p.Transforms {
		if i == len(p.Transforms)-1 {
			p.Transforms[i].IsLast = true
		} else {
			p.Transforms[i].IsLast = false
		}
		off += p.Transforms[i].WriteTo(buf, off)
	}
	length := off - start
	binary.BigEndian.PutUint16(buf[start+2:], uint16(length))
	return length
}

// length reports the bytes Proposal.WriteTo writes. The SPI is written only
// when SPISize>0 and the SPI slice is long enough (WriteTo otherwise zeroes
// SPISize and writes no SPI bytes), so the SPI contribution mirrors that guard.
func (p *Proposal) length() int {
	n := proposalHeaderLen
	if p.SPISize > 0 && len(p.SPI) >= int(p.SPISize) {
		n += int(p.SPISize)
	}
	for i := range p.Transforms {
		n += p.Transforms[i].length()
	}
	return n
}

func (p *Proposal) ReadFrom(data []byte) error {
	if len(data) < proposalHeaderLen {
		return ErrTruncated
	}
	p.IsLast = data[0] == 0
	plen := int(binary.BigEndian.Uint16(data[2:4]))
	if plen < proposalHeaderLen || plen > len(data) {
		return ErrTruncated
	}
	p.Number = data[4]
	p.ProtocolID = data[5]
	p.SPISize = data[6]
	numTransforms := int(data[7])
	off := proposalHeaderLen
	if int(p.SPISize) > plen-off {
		return ErrTruncated
	}
	if p.SPISize > 0 {
		p.SPI = make([]byte, p.SPISize)
		copy(p.SPI, data[off:off+int(p.SPISize)])
		off += int(p.SPISize)
	}
	p.Transforms = make([]Transform, 0, numTransforms)
	for i := 0; i < numTransforms && off < plen; i++ {
		if i >= MaxNestingDepth {
			return ErrTooManyPayloads
		}
		var t Transform
		if err := t.ReadFrom(data[off:plen]); err != nil {
			return err
		}
		tlen := int(binary.BigEndian.Uint16(data[off+2 : off+4]))
		p.Transforms = append(p.Transforms, t)
		off += tlen
	}
	return nil
}

// PayloadSA is the Security Association payload (type 33).
type PayloadSA struct {
	Proposals []Proposal
}

func (p *PayloadSA) Type() uint8 { return PayloadTypeSA }

func (p *PayloadSA) WriteTo(buf []byte, off int) int {
	start := off
	for i := range p.Proposals {
		if i == len(p.Proposals)-1 {
			p.Proposals[i].IsLast = true
		} else {
			p.Proposals[i].IsLast = false
		}
		off += p.Proposals[i].WriteTo(buf, off)
	}
	return off - start
}

func (p *PayloadSA) Len() int {
	n := 0
	for i := range p.Proposals {
		n += p.Proposals[i].length()
	}
	return n
}

func (p *PayloadSA) ReadFrom(data []byte) error {
	if len(data) == 0 {
		return ErrNoProposals
	}
	p.Proposals = nil
	off := 0
	for off < len(data) {
		if len(p.Proposals) >= MaxNestingDepth {
			return ErrTooManyPayloads
		}
		if off+proposalHeaderLen > len(data) {
			return ErrTruncated
		}
		var prop Proposal
		if err := prop.ReadFrom(data[off:]); err != nil {
			return err
		}
		plen := int(binary.BigEndian.Uint16(data[off+2 : off+4]))
		p.Proposals = append(p.Proposals, prop)
		if prop.IsLast {
			break
		}
		off += plen
	}
	return nil
}
