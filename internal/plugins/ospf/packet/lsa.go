// Design: docs/architecture/ospf/ospf-2-wire.md -- common 20-byte LSA header and lazy LSA view
// RFC 2328 Appendix A.4.1: LSA header.

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/types"

const (
	lsaAgeOff       = 0
	lsaOptionsOff   = 2
	lsaTypeOff      = 3
	lsaLSIDOff      = 4
	lsaAdvRouterOff = 8
	lsaSequenceOff  = 12
	lsaChecksumOff  = 16
	lsaLengthOff    = 18
)

const lsaHeaderMinLen = types.LSAHeaderLen

// LSAHeader is the common OSPF LSA header. The canonical struct lives in the shared
// types leaf (one type across the engine, LSDB, neighbor FSM, SPF, and both wire
// codecs); this alias preserves the packet.LSAHeader spelling for OSPFv2 callers.
// RFC 2328 Appendix A.4.1: Length includes this 20-byte header.
type LSAHeader = types.LSAHeader

// DecodeLSAHeader parses the first 20 bytes of buf as an LSA header.
func DecodeLSAHeader(buf []byte) (LSAHeader, error) {
	if len(buf) < types.LSAHeaderLen {
		return LSAHeader{}, ErrShortBuffer
	}
	age, err := types.LSAgeFromBytes(buf[lsaAgeOff : lsaAgeOff+types.LSAgeLen])
	if err != nil {
		return LSAHeader{}, err
	}
	opts, err := types.OptionsFromBytes(buf[lsaOptionsOff : lsaOptionsOff+types.OptionsLen])
	if err != nil {
		return LSAHeader{}, err
	}
	lt := types.LSTypeFromByte(buf[lsaTypeOff])
	if !lt.Known() {
		return LSAHeader{}, ErrUnknownLSAType
	}
	lsid, err := types.LinkStateIDFromBytes(buf[lsaLSIDOff : lsaLSIDOff+types.LinkStateIDLen])
	if err != nil {
		return LSAHeader{}, err
	}
	adv, err := types.RouterIDFromBytes(buf[lsaAdvRouterOff : lsaAdvRouterOff+types.RouterIDLen])
	if err != nil {
		return LSAHeader{}, err
	}
	seq, err := types.LSSequenceNumberFromBytes(buf[lsaSequenceOff : lsaSequenceOff+types.LSSequenceNumberLen])
	if err != nil {
		return LSAHeader{}, err
	}
	length := readUint16(buf, lsaLengthOff)
	if length < types.LSAHeaderLen {
		return LSAHeader{}, ErrLength
	}
	return LSAHeader{
		Age:               age,
		Options:           opts,
		Type:              lt,
		LinkStateID:       lsid,
		AdvertisingRouter: adv,
		Sequence:          seq,
		Checksum:          readUint16(buf, lsaChecksumOff),
		Length:            length,
	}, nil
}

// writeLSAHeader writes the OSPFv2 LSA header into buf at off and returns off+20. This
// is the OSPFv2 wire encoding and so lives in the codec layer, not on the shared
// types.LSAHeader (the OSPFv3 codec encodes the header differently -- 16-bit scope-typed
// LS Type). The Key() identity method lives on types.LSAHeader.
func writeLSAHeader(h LSAHeader, buf []byte, off int) int {
	off += h.Age.WriteTo(buf, off)
	off += h.Options.WriteTo(buf, off)
	off += h.Type.WriteTo(buf, off)
	off += h.LinkStateID.WriteTo(buf, off)
	off += h.AdvertisingRouter.WriteTo(buf, off)
	off += h.Sequence.WriteTo(buf, off)
	off += writeUint16(buf, off, h.Checksum)
	off += writeUint16(buf, off, h.Length)
	return off
}

// RefreshLSAInPlace re-stamps an already-encoded LSA's LS Age, LS Sequence Number, and LS
// Checksum in place, returning the new checksum. These three fields occupy identical header
// offsets in OSPFv2 and OSPFv3 and the Fletcher checksum is byte-identical across both, so
// this re-stamps a stored LSA (e.g. a MaxAge self-flush) address-family-agnostically --
// without decoding the body, which the OSPFv2 codec could not do for an OSPFv3 LSA's 16-bit
// LS Type. It returns false if the buffer is shorter than an LSA header.
func RefreshLSAInPlace(lsa []byte, age types.LSAge, seq types.LSSequenceNumber) (uint16, bool) {
	if len(lsa) < types.LSAHeaderLen {
		return 0, false
	}
	age.WriteTo(lsa, lsaAgeOff)
	seq.WriteTo(lsa, lsaSequenceOff)
	return FinalizeLSAChecksum(lsa), true
}

// LSA is a lazy decoded LSA or a struct ready for encoding. DecodeLSA fills
// RawBytes and Body as caller-owned slices. Body-specific methods parse Body on
// demand. WriteTo re-emits RawBytes verbatim when no typed body is set, enabling
// opaque passthrough; constructed LSAs recompute Length and Fletcher checksum.
type LSA struct {
	Header   LSAHeader
	Body     []byte
	RawBytes []byte

	Router   *RouterLSA
	Network  *NetworkLSA
	Summary  *SummaryLSA
	External *ExternalLSA
	Opaque   *OpaqueLSA
}

// DecodeLSA parses one LSA from the front of buf, driven by the Length field.
func DecodeLSA(buf []byte) (LSA, error) {
	if len(buf) < types.LSAHeaderLen {
		return LSA{}, ErrShortBuffer
	}
	header, err := DecodeLSAHeader(buf[:types.LSAHeaderLen])
	if err != nil {
		return LSA{}, err
	}
	if int(header.Length) > len(buf) {
		return LSA{}, ErrTruncated
	}
	raw := buf[:header.Length]
	lsa := LSA{Header: header, Body: raw[types.LSAHeaderLen:], RawBytes: raw}
	return lsa, nil
}

// EncodedLen returns the total on-wire length of the LSA.
func (l LSA) EncodedLen() int {
	if len(l.RawBytes) != 0 && l.Router == nil && l.Network == nil && l.Summary == nil && l.External == nil && l.Opaque == nil {
		return len(l.RawBytes)
	}
	return types.LSAHeaderLen + l.bodyLen()
}

func (l LSA) bodyLen() int {
	switch {
	case l.Router != nil:
		return l.Router.EncodedLen()
	case l.Network != nil:
		return l.Network.EncodedLen()
	case l.Summary != nil:
		return l.Summary.EncodedLen()
	case l.External != nil:
		return l.External.EncodedLen()
	case l.Opaque != nil:
		return len(l.Opaque.Data)
	default:
		return len(l.Body)
	}
}

// WriteTo serializes the LSA into buf at off and returns the new offset.
func (l *LSA) WriteTo(buf []byte, off int) int {
	if len(l.RawBytes) != 0 && l.Router == nil && l.Network == nil && l.Summary == nil && l.External == nil && l.Opaque == nil {
		copy(buf[off:off+len(l.RawBytes)], l.RawBytes)
		return off + len(l.RawBytes)
	}
	start := off
	h := l.Header
	h.Length = 0
	h.Checksum = 0
	off = writeLSAHeader(h, buf, off)
	switch {
	case l.Router != nil:
		off = l.Router.WriteTo(buf, off)
	case l.Network != nil:
		off = l.Network.WriteTo(buf, off)
	case l.Summary != nil:
		off = l.Summary.WriteTo(buf, off)
	case l.External != nil:
		off = l.External.WriteTo(buf, off)
	case l.Opaque != nil:
		copy(buf[off:off+len(l.Opaque.Data)], l.Opaque.Data)
		off += len(l.Opaque.Data)
	default:
		copy(buf[off:off+len(l.Body)], l.Body)
		off += len(l.Body)
	}
	length := off - start
	writeUint16(buf, start+lsaLengthOff, uint16(length))
	checksum := FinalizeLSAChecksum(buf[start:off])
	l.Header.Length = uint16(length)
	l.Header.Checksum = checksum
	return off
}

// VerifyChecksum verifies this decoded LSA's Fletcher checksum.
func (l LSA) VerifyChecksum() bool {
	if len(l.RawBytes) == 0 {
		return false
	}
	return VerifyLSAChecksum(l.RawBytes)
}

// DecodeRouter parses a Type 1 Router-LSA body on demand.
func (l LSA) DecodeRouter() (RouterLSA, error) {
	return DecodeRouterLSA(l.Body)
}

// DecodeNetwork parses a Type 2 Network-LSA body on demand.
func (l LSA) DecodeNetwork() (NetworkLSA, error) {
	return DecodeNetworkLSA(l.Body)
}

// DecodeSummary parses a Type 3/4 Summary-LSA body on demand.
func (l LSA) DecodeSummary() (SummaryLSA, error) {
	return DecodeSummaryLSA(l.Body)
}

// DecodeExternal parses a Type 5/7 External/NSSA body on demand.
func (l LSA) DecodeExternal() (ExternalLSA, error) {
	return DecodeExternalLSA(l.Body)
}

// LSAIterator walks a region containing consecutive LSAs using each LSA's Length
// field. It never panics on malformed input; Err reports truncation or bad length.
type LSAIterator struct {
	data []byte
	off  int
	cur  LSA
	err  error
}

// NewLSAIterator returns an iterator over a consecutive LSA region.
func NewLSAIterator(data []byte) LSAIterator { return LSAIterator{data: data} }

// Next advances to the next LSA.
func (it *LSAIterator) Next() bool {
	if it.err != nil || it.off == len(it.data) {
		return false
	}
	if len(it.data)-it.off < types.LSAHeaderLen {
		it.err = ErrTruncated
		return false
	}
	length := int(readUint16(it.data, it.off+lsaLengthOff))
	if length < types.LSAHeaderLen {
		it.err = ErrLength
		return false
	}
	if it.off+length > len(it.data) {
		it.err = ErrTruncated
		return false
	}
	lsa, err := DecodeLSA(it.data[it.off : it.off+length])
	if err != nil {
		it.err = err
		return false
	}
	it.cur = lsa
	it.off += length
	return true
}

// LSA returns the current iterator item.
func (it *LSAIterator) LSA() LSA { return it.cur }

// Err returns the first iterator error, if any.
func (it *LSAIterator) Err() error { return it.err }
