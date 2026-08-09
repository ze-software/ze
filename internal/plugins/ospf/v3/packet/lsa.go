// Design: docs/architecture/ospf/ospfv3-2-wire.md -- 20-octet OSPFv3 LSA header and lazy LSA view.
// RFC: rfc/short/rfc5340.md (§A.4.2.1 LSA header)

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/v3/types"

// LSA header field offsets (RFC 5340 §A.4.2.1). There is NO Options byte in the
// OSPFv3 LSA header: the OSPFv2 Options@2 + 8-bit Type@3 become a single 16-bit
// LS Type@2.
const (
	lsaAgeOff       = 0
	lsaTypeOff      = 2
	lsaLSIDOff      = 4
	lsaAdvRouterOff = 8
	lsaSequenceOff  = 12
	lsaChecksumOff  = 16
	lsaLengthOff    = 18
	// LSAHeaderLen is the fixed OSPFv3 LSA header width in octets.
	LSAHeaderLen = 20
)

// LSAHeader is the decoded common OSPFv3 LSA header. Length covers this 20-octet
// header plus the body (RFC 5340 §A.4.2.1).
type LSAHeader struct {
	Age               types.LSAge
	Type              types.LSType
	LinkStateID       types.LinkStateID
	AdvertisingRouter types.RouterID
	Sequence          types.LSSequenceNumber
	Checksum          uint16
	Length            uint16
}

// DecodeLSAHeader parses the first 20 bytes of buf as an OSPFv3 LSA header. The
// LS Type is kept even when unrecognized: RFC 5340 §A.4.2.1's U-bit governs
// whether an unknown LSA is flooded, so the codec retains every type and leaves
// the policy to the LSDB.
func DecodeLSAHeader(buf []byte) (LSAHeader, error) {
	if len(buf) < LSAHeaderLen {
		return LSAHeader{}, ErrShortBuffer
	}
	lt, err := types.LSTypeFromBytes(buf, lsaTypeOff)
	if err != nil {
		return LSAHeader{}, err
	}
	lsid, err := types.LinkStateIDFromBytes(buf[lsaLSIDOff : lsaLSIDOff+types.IDLen])
	if err != nil {
		return LSAHeader{}, err
	}
	adv, err := types.RouterIDFromBytes(buf[lsaAdvRouterOff : lsaAdvRouterOff+types.IDLen])
	if err != nil {
		return LSAHeader{}, err
	}
	length := readUint16(buf, lsaLengthOff)
	// RFC 5340 §A.4.2.1: "Length -- The length in bytes of the LSA. This includes
	// the 20-byte LSA header."
	if length < LSAHeaderLen {
		return LSAHeader{}, ErrLength
	}
	return LSAHeader{
		Age:               types.LSAge(readUint16(buf, lsaAgeOff)),
		Type:              lt,
		LinkStateID:       lsid,
		AdvertisingRouter: adv,
		Sequence:          types.LSSequenceNumber(int32(readUint32(buf, lsaSequenceOff))),
		Checksum:          readUint16(buf, lsaChecksumOff),
		Length:            length,
	}, nil
}

// Key returns the LSDB identity tuple for this LSA header (RFC 5340 §A.4.2.1: an
// LSA is identified by LS Type, Link State ID, Advertising Router).
func (h LSAHeader) Key() types.LSAKey {
	return types.LSAKey{Type: h.Type, LinkStateID: h.LinkStateID, AdvertisingRouter: h.AdvertisingRouter}
}

// WriteTo writes the LSA header as-is into buf at off and returns off+20.
func (h LSAHeader) WriteTo(buf []byte, off int) int {
	off += h.Age.WriteTo(buf, off)
	off += h.Type.WriteTo(buf, off)
	off += h.LinkStateID.WriteTo(buf, off)
	off += h.AdvertisingRouter.WriteTo(buf, off)
	off += writeUint32(buf, off, uint32(h.Sequence))
	off += writeUint16(buf, off, h.Checksum)
	off += writeUint16(buf, off, h.Length)
	return off
}

// LSA is a lazily decoded LSA or a struct ready for encoding. DecodeLSA fills
// RawBytes and Body as caller-owned slices. Body-specific Decode* methods parse
// Body on demand. WriteTo re-emits RawBytes verbatim when no typed body is set,
// enabling opaque passthrough of received and unknown LSAs; constructed LSAs
// recompute Length and the Fletcher checksum.
type LSA struct {
	Header   LSAHeader
	Body     []byte
	RawBytes []byte

	Router       *RouterLSA
	Network      *NetworkLSA
	InterAreaPfx *InterAreaPrefixLSA
	InterAreaRtr *InterAreaRouterLSA
	External     *ExternalLSA
	Link         *LinkLSA
	IntraAreaPfx *IntraAreaPrefixLSA
	Grace        *GraceLSA
}

// hasTypedBody reports whether a typed body is attached (so encode must
// re-marshal rather than re-emit RawBytes).
func (l *LSA) hasTypedBody() bool {
	return l.Router != nil || l.Network != nil || l.InterAreaPfx != nil ||
		l.InterAreaRtr != nil || l.External != nil || l.Link != nil || l.IntraAreaPfx != nil ||
		l.Grace != nil
}

// DecodeLSA parses one LSA from the front of buf, driven by the Length field. The
// typed body is NOT eagerly decoded; RawBytes and Body retain caller-owned spans
// so flooding can re-emit a received LSA byte-for-byte.
func DecodeLSA(buf []byte) (LSA, error) {
	if len(buf) < LSAHeaderLen {
		return LSA{}, ErrShortBuffer
	}
	header, err := DecodeLSAHeader(buf[:LSAHeaderLen])
	if err != nil {
		return LSA{}, err
	}
	if int(header.Length) > len(buf) {
		return LSA{}, ErrTruncated
	}
	raw := buf[:header.Length]
	return LSA{Header: header, Body: raw[LSAHeaderLen:], RawBytes: raw}, nil
}

// EncodedLen returns the total on-wire length of the LSA.
func (l *LSA) EncodedLen() int {
	if len(l.RawBytes) != 0 && !l.hasTypedBody() {
		return len(l.RawBytes)
	}
	return LSAHeaderLen + l.bodyLen()
}

func (l *LSA) bodyLen() int {
	switch {
	case l.Router != nil:
		return l.Router.EncodedLen()
	case l.Network != nil:
		return l.Network.EncodedLen()
	case l.InterAreaPfx != nil:
		return l.InterAreaPfx.EncodedLen()
	case l.InterAreaRtr != nil:
		return l.InterAreaRtr.EncodedLen()
	case l.External != nil:
		return l.External.EncodedLen()
	case l.Link != nil:
		return l.Link.EncodedLen()
	case l.IntraAreaPfx != nil:
		return l.IntraAreaPfx.EncodedLen()
	case l.Grace != nil:
		return l.Grace.EncodedLen()
	default:
		return len(l.Body)
	}
}

// WriteTo serializes the LSA into buf at off and returns the new offset. A
// received LSA with no typed body is re-emitted verbatim from RawBytes so a
// flooded LSA is byte-for-byte identical (RFC 5340 §4.5: LSAs are flooded
// unchanged). A constructed LSA recomputes Length and the Fletcher checksum.
func (l *LSA) WriteTo(buf []byte, off int) int {
	if len(l.RawBytes) != 0 && !l.hasTypedBody() {
		copy(buf[off:off+len(l.RawBytes)], l.RawBytes)
		return off + len(l.RawBytes)
	}
	start := off
	h := l.Header
	h.Length = 0
	h.Checksum = 0
	off = h.WriteTo(buf, off)
	switch {
	case l.Router != nil:
		off = l.Router.WriteTo(buf, off)
	case l.Network != nil:
		off = l.Network.WriteTo(buf, off)
	case l.InterAreaPfx != nil:
		off = l.InterAreaPfx.WriteTo(buf, off)
	case l.InterAreaRtr != nil:
		off = l.InterAreaRtr.WriteTo(buf, off)
	case l.External != nil:
		off = l.External.WriteTo(buf, off)
	case l.Link != nil:
		off = l.Link.WriteTo(buf, off)
	case l.IntraAreaPfx != nil:
		off = l.IntraAreaPfx.WriteTo(buf, off)
	case l.Grace != nil:
		off = l.Grace.WriteTo(buf, off)
	default:
		copy(buf[off:off+len(l.Body)], l.Body)
		off += len(l.Body)
	}
	length := off - start
	// LS Length is a 16-bit field (RFC 5340 sec A.4.1). A valid OSPFv3 LSA is bounded well below
	// this by origination (an interface advertises only its handful of prefixes), so this only
	// guards a hypothetical bug: clamp to the max rather than silently wrapping to a small value
	// that a receiver would misparse -- the over-size LSA then fails the receiver's length /
	// checksum check instead of corrupting its LSDB.
	length = min(length, 0xFFFF)
	writeUint16(buf, start+lsaLengthOff, uint16(length))
	checksum := FinalizeLSAChecksum(buf[start:off])
	l.Header.Length = uint16(length)
	l.Header.Checksum = checksum
	return off
}

// VerifyChecksum verifies this decoded LSA's Fletcher checksum.
func (l *LSA) VerifyChecksum() bool {
	if len(l.RawBytes) == 0 {
		return false
	}
	return VerifyLSAChecksum(l.RawBytes)
}

// DecodeRouter parses a Router-LSA body on demand.
func (l *LSA) DecodeRouter() (RouterLSA, error) { return DecodeRouterLSA(l.Body) }

// DecodeNetwork parses a Network-LSA body on demand.
func (l *LSA) DecodeNetwork() (NetworkLSA, error) { return DecodeNetworkLSA(l.Body) }

// DecodeInterAreaPrefix parses an Inter-Area-Prefix-LSA body on demand.
func (l *LSA) DecodeInterAreaPrefix() (InterAreaPrefixLSA, error) {
	return decodeInterAreaPrefixLSA(l.Body)
}

// DecodeInterAreaRouter parses an Inter-Area-Router-LSA body on demand.
func (l *LSA) DecodeInterAreaRouter() (InterAreaRouterLSA, error) {
	return decodeInterAreaRouterLSA(l.Body)
}

// DecodeExternal parses an AS-External / NSSA body on demand.
func (l *LSA) DecodeExternal() (ExternalLSA, error) { return DecodeExternalLSA(l.Body) }

// DecodeLink parses a Link-LSA body on demand.
func (l *LSA) DecodeLink() (LinkLSA, error) { return decodeLinkLSA(l.Body) }

// DecodeIntraAreaPrefix parses an Intra-Area-Prefix-LSA body on demand.
func (l *LSA) DecodeIntraAreaPrefix() (IntraAreaPrefixLSA, error) {
	return decodeIntraAreaPrefixLSA(l.Body)
}

// DecodeGrace parses a Grace-LSA body (RFC 5187 §2.2) on demand.
func (l *LSA) DecodeGrace() (GraceLSA, error) { return decodeGraceLSA(l.Body) }

// LSAIterator walks a region of consecutive LSAs using each LSA's Length field.
// It never panics on malformed input; Err reports truncation or a bad length.
type LSAIterator struct {
	data []byte
	off  int
	cur  LSA
	err  error
}

// NewLSAIterator returns an iterator over a consecutive LSA region.
func NewLSAIterator(data []byte) LSAIterator { return LSAIterator{data: data} }

// Next advances to the next LSA, bound-checking the Length against the buffer.
func (it *LSAIterator) Next() bool {
	if it.err != nil || it.off == len(it.data) {
		return false
	}
	if len(it.data)-it.off < LSAHeaderLen {
		it.err = ErrTruncated
		return false
	}
	length := int(readUint16(it.data, it.off+lsaLengthOff))
	// RFC 5340 §A.4.2.1: an LSA Length below the 20-byte header is malformed.
	if length < LSAHeaderLen {
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
