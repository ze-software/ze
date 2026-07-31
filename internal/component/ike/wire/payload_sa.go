// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — SA payload (Section 3.3)
package wire

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Protocol IDs (RFC 7296 Section 3.3.1).
const (
	ProtocolIKE uint8 = 1
	ProtocolAH  uint8 = 2
	ProtocolESP uint8 = 3
)

// MaxTransformAttrs bounds the attributes parsed from one transform. RFC 7296
// Section 3.3.5 defines one attribute type, so a conforming transform carries at
// most one attribute. The SA payload arrives before any peer is authenticated, so
// the bound stops a peer-supplied attribute list from growing the parse.
const MaxTransformAttrs = 16

var (
	// ErrDuplicateAttr reports two attributes of one type in a single transform
	// (RFC 7296 Section 3.3).
	ErrDuplicateAttr = errors.New("ike: transform repeats an attribute type")
	// ErrAttrEncoding reports an attribute encoded in the format its type forbids
	// (RFC 7296 Section 3.3.5).
	ErrAttrEncoding = errors.New("ike: transform attribute uses the wrong encoding format")
	// ErrKeyLengthNotAllowed reports a Key Length attribute on a transform that uses
	// a fixed-length key (RFC 7296 Section 3.3.5).
	ErrKeyLengthNotAllowed = errors.New("ike: key length attribute on a fixed-length-key transform")
	// ErrTooManyAttrs reports an attribute list longer than MaxTransformAttrs.
	ErrTooManyAttrs = errors.New("ike: transform exceeds the maximum attribute count")
	// ErrProposalNumbering reports an offer whose proposal numbers do not start at
	// one and step by one (RFC 7296 Section 3.3).
	ErrProposalNumbering = errors.New("ike: proposal numbers are not sequential from one")
	// ErrTransformRejected marks one refused transform inside a proposal that stays
	// valid. RFC 7296 Section 3.3.6: "other transforms with the same Transform Type
	// are processed as usual". Proposal.ReadFrom drops the transform, keeps the rest,
	// and reports the reason through this wrapper.
	ErrTransformRejected = errors.New("ike: transform rejected")
	// ErrProposalRejected marks one refused proposal inside an SA payload that stays
	// valid. RFC 7296 Section 3.3.6: "however, other proposals in the same SA payload
	// are processed as usual". PayloadSA.ReadFrom drops the proposal and keeps the
	// rest.
	ErrProposalRejected = errors.New("ike: proposal rejected")
)

// rejectTransform records why one transform is refused. The wrapper carries
// ErrTransformRejected for the caller that drops the transform, and the cause for the
// caller that reports it.
func rejectTransform(cause error) error {
	return fmt.Errorf("%w: %w", ErrTransformRejected, cause)
}

// rejectProposal records why one proposal is refused, in the shape rejectTransform uses.
func rejectProposal(cause error) error {
	return fmt.Errorf("%w: %w", ErrProposalRejected, cause)
}

// isItemRejected reports an error that names one refused proposal or transform inside an
// SA payload whose framing is sound. RFC 7296 Section 3.3.6 keeps the siblings of a
// refused item, so the payload still holds what survived and the message stands. Every
// other error means the octets after it cannot be trusted.
func isItemRejected(err error) bool {
	return errors.Is(err, ErrTransformRejected) || errors.Is(err, ErrProposalRejected)
}

// Encryption Transform IDs whose key is fixed length, so RFC 7296 Section 3.3.5
// forbids a Key Length attribute on them. The section names these two.
const (
	encrDES  uint16 = 2
	encrIDEA uint16 = 5
)

// attrSpec records the encoding one attribute type must use. RFC 7296
// Section 3.3.5 defines one attribute type, Key Length, which is fixed length and
// two octets wide.
type attrSpec struct {
	variable bool
	fixedLen int
}

// attrFormats maps an attribute type to the encoding it must use. A type absent
// from the map is one this implementation does not understand: the parse keeps it
// and negotiation refuses the transform (RFC 7296 Section 3.3.6).
var attrFormats = map[uint16]attrSpec{
	AttrTypeKeyLength: {variable: false, fixedLen: 2},
}

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

// TransformAttr is a transform attribute (RFC 7296 Section 3.3.5). An attribute
// uses the TV form, a two-octet value in Value, or the TLV form, a variable-length
// value in Data. Variable selects the form, and its zero value is the TV form that
// the only defined attribute type, Key Length, requires.
type TransformAttr struct {
	Type     uint16
	Value    uint16
	Variable bool
	Data     []byte
}

// attrLen reports the octets this attribute occupies on the wire.
func (a *TransformAttr) attrLen() int {
	if a.Variable {
		return 4 + len(a.Data)
	}
	return 4
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
		a := &t.Attrs[i]
		if a.Variable {
			// TLV format: the AF bit is zero and the second half-word is the
			// value length (RFC 7296 Section 3.3.5).
			binary.BigEndian.PutUint16(buf[off:], a.Type&0x7fff)
			binary.BigEndian.PutUint16(buf[off+2:], uint16(len(a.Data)))
			copy(buf[off+4:], a.Data)
			off += 4 + len(a.Data)
			continue
		}
		// TV format: the AF bit is one and the second half-word is the value.
		binary.BigEndian.PutUint16(buf[off:], a.Type|0x8000)
		binary.BigEndian.PutUint16(buf[off+2:], a.Value)
		off += 4
	}
	length := off - start
	binary.BigEndian.PutUint16(buf[start+2:], uint16(length))
	return length
}

// length reports the bytes Transform.WriteTo writes: the fixed transform header
// plus each attribute in the form it declares.
func (t *Transform) length() int {
	n := transformHeaderLen
	for i := range t.Attrs {
		n += t.Attrs[i].attrLen()
	}
	return n
}

// keyLengthAllowed reports whether a Key Length attribute CAN sit on this transform.
// RFC 7296 Section 3.3.5: "The Key Length attribute MUST NOT be used with transforms
// that use a fixed-length key.  For example, this includes ENCR_DES, ENCR_IDEA, and all
// the Type 2 (Pseudorandom Function) and Type 3 (Integrity Algorithm) transforms
// specified in this document."
//
// The obligation reaches those carriers and no others. Section 3.3.5 never names Type 4
// (Diffie-Hellman Group) or Type 5 (Extended Sequence Numbers), so the attribute on
// either of those breaks no MUST. The parser keeps such a transform and leaves it to
// negotiation, which refuses what it cannot use.
func (t *Transform) keyLengthAllowed() bool {
	switch t.Type {
	case TransformTypeENCR:
		return t.ID != encrDES && t.ID != encrIDEA
	case TransformTypePRF, TransformTypeINTG:
		return false
	default:
		return true
	}
}

// checkAttr rejects an attribute whose encoding or carrier the RFC forbids. An
// attribute type this implementation does not understand is kept rather than
// refused. RFC 7296 Section 3.3.6 makes that transform unacceptable, and it leaves
// the rest of the payload parseable.
func (t *Transform) checkAttr(a *TransformAttr) error {
	spec, known := attrFormats[a.Type]
	if !known {
		return nil
	}
	if spec.variable && !a.Variable {
		return ErrAttrEncoding
	}
	if !spec.variable && a.Variable && spec.fixedLen <= 2 {
		return ErrAttrEncoding
	}
	if a.Type == AttrTypeKeyLength && !t.keyLengthAllowed() {
		return ErrKeyLengthNotAllowed
	}
	return nil
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
	for off < tlen {
		if len(t.Attrs) >= MaxTransformAttrs {
			return rejectTransform(ErrTooManyAttrs)
		}
		if off+4 > tlen {
			return ErrTruncated
		}
		head := binary.BigEndian.Uint16(data[off:])
		attr := TransformAttr{
			Type:     head & 0x7fff,
			Variable: head&0x8000 == 0,
		}
		if attr.Variable {
			alen := int(binary.BigEndian.Uint16(data[off+2:]))
			if alen > tlen-off-4 {
				return ErrTruncated
			}
			attr.Data = make([]byte, alen)
			copy(attr.Data, data[off+4:off+4+alen])
			off += 4 + alen
		} else {
			attr.Value = binary.BigEndian.Uint16(data[off+2:])
			off += 4
		}
		if err := t.checkAttr(&attr); err != nil {
			return rejectTransform(err)
		}
		for i := range t.Attrs {
			if t.Attrs[i].Type == attr.Type {
				return rejectTransform(ErrDuplicateAttr)
			}
		}
		t.Attrs = append(t.Attrs, attr)
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

// checkSPISize rejects an SPI Size the protocol never uses. RFC 7296
// Section 3.3.1 makes the field zero for an initial IKE SA negotiation, because
// the SPI comes from the outer header. For a later negotiation the field holds the
// SPI size of the protocol: 8 octets for IKE, and 4 octets for ESP and AH. A
// protocol identifier this implementation does not know carries no size rule. Its
// proposal is left for negotiation to refuse.
func (p *Proposal) checkSPISize() error {
	switch p.ProtocolID {
	case ProtocolIKE:
		if p.SPISize != 0 && p.SPISize != 8 {
			return ErrInvalidSPISize
		}
	case ProtocolESP, ProtocolAH:
		if p.SPISize != 4 {
			return ErrInvalidSPISize
		}
	}
	return nil
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
	// The bound comes first. An SPI Size that does not fit the declared proposal
	// length is a length inconsistency (RFC 7296 Section 3.3). That reading stays
	// true whatever the protocol identifier says.
	if int(p.SPISize) > plen-off {
		return ErrTruncated
	}
	if err := p.checkSPISize(); err != nil {
		return rejectProposal(err)
	}
	if p.SPISize > 0 {
		p.SPI = make([]byte, p.SPISize)
		copy(p.SPI, data[off:off+int(p.SPISize)])
		off += int(p.SPISize)
	}
	p.Transforms = make([]Transform, 0, numTransforms)
	var rejected error
	for i := 0; i < numTransforms && off < plen; i++ {
		if i >= MaxNestingDepth {
			// The SA payload is parsed before any peer is authenticated, so the count
			// of transforms read from one proposal is bounded. A proposal past the
			// bound is not an offer a conforming peer sends. The whole proposal goes.
			return rejectProposal(ErrTooManyPayloads)
		}
		var t Transform
		err := t.ReadFrom(data[off:plen])
		if err != nil && !errors.Is(err, ErrTransformRejected) {
			return err
		}
		// Transform.ReadFrom validates the length field before it refuses a transform,
		// so this step is sound for a dropped transform too.
		tlen := int(binary.BigEndian.Uint16(data[off+2 : off+4]))
		// A refused transform is dropped, and the transforms beside it are processed as
		// usual (RFC 7296 Section 3.3.6). The first reason reaches the caller.
		if err == nil {
			p.Transforms = append(p.Transforms, t)
		} else if rejected == nil {
			rejected = err
		}
		off += tlen
	}
	if rejected != nil && len(p.Transforms) == 0 {
		// Every transform was refused, so this proposal offers nothing to negotiate.
		// RFC 7296 Section 3.3.6 then makes the proposal itself unacceptable.
		return rejectProposal(rejected)
	}
	return rejected
}

// PayloadSA is the Security Association payload (type 33).
type PayloadSA struct {
	Proposals []Proposal
}

func (p *PayloadSA) Type() uint8 { return PayloadTypeSA }

// ValidateOfferNumbering checks the Proposal Num sequence of an SA payload that
// offers proposals. RFC 7296 Section 3.3: "Each structure MUST have a proposal
// number one (1) greater than the previous structure. The first Proposal in the
// initiator's SA payload MUST have a Proposal Num of one (1)."
//
// The rule governs an offer. A response carries the number of the proposal that
// was accepted (RFC 7296 Section 3.3.1), and that number is usually not one. The
// caller therefore applies this check to a received offer, never to a response.
func (p *PayloadSA) ValidateOfferNumbering() error {
	for i := range p.Proposals {
		if int(p.Proposals[i].Number) != i+1 {
			return ErrProposalNumbering
		}
	}
	return nil
}

// ValidateInitialSPISize checks the SPI Size of every proposal in the SA payload of an
// INITIAL IKE SA negotiation. RFC 7296 Section 3.3.1: "For an initial IKE SA negotiation,
// this field MUST be zero". The SPI comes from the outer header.
//
// The rule is conditional, and the condition is the exchange. checkSPISize runs at parse
// time, where the exchange is unknown. It therefore accepts every size the protocol ever
// uses, which is 0 or 8 for IKE. Only a caller that knows the message is an IKE_SA_INIT
// can narrow that set to zero. That caller applies this check, and the parse layer does
// not.
func (p *PayloadSA) ValidateInitialSPISize() error {
	for i := range p.Proposals {
		if p.Proposals[i].SPISize != 0 {
			return ErrInvalidSPISize
		}
	}
	return nil
}

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
	var rejected error
	for off < len(data) {
		if len(p.Proposals) >= MaxNestingDepth {
			return ErrTooManyPayloads
		}
		if off+proposalHeaderLen > len(data) {
			return ErrTruncated
		}
		var prop Proposal
		err := prop.ReadFrom(data[off:])
		if err != nil && !isItemRejected(err) {
			return err
		}
		// Proposal.ReadFrom validates the length field before it refuses a proposal or
		// a transform, so this step is sound for a dropped proposal too.
		plen := int(binary.BigEndian.Uint16(data[off+2 : off+4]))
		// A proposal refused for itself is dropped. One that only lost a transform
		// keeps the transforms that survived, and it stays in the offer. Either way the
		// proposals beside it are processed as usual (RFC 7296 Section 3.3.6).
		if !errors.Is(err, ErrProposalRejected) {
			p.Proposals = append(p.Proposals, prop)
		}
		if err != nil && rejected == nil {
			rejected = err
		}
		if prop.IsLast {
			break
		}
		off += plen
	}
	return rejected
}
