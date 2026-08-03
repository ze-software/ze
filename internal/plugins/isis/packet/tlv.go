// Design: plan/learned/928-isis-2-wire.md -- generic TLV iterator + encode helper
// ISO/IEC 10589 clause 9: TLVs are Type (1 octet) + Length (1 octet) + Value
// (Length octets). The same framing nests as sub-TLVs (RFC 5305 sec 2).
//
// RFC: rfc/short/rfc5305.md -- sub-TLV framing (sec 2), TLV 22/135 type codes
// RFC: rfc/short/rfc5308.md -- TLV 232/236 type codes
// RFC: rfc/short/rfc5301.md -- TLV 137 type code
// RFC: rfc/short/rfc5303.md -- TLV 240 type code
// RFC: rfc/short/rfc5304.md -- TLV 10 authentication type code

package packet

// TLV type codes handled by this codec (ISO/IEC 10589 clause 9 and the
// extension RFCs). The umbrella TLV inventory enumerates the full set; this
// codec encodes/decodes 1, 6, 8, 9, 10, 22, 129, 132, 135, 137, 232, 236, 240
// and decode-only TLV 2.
const (
	TLVAreaAddresses        = 1   // ISO/IEC 10589 clause 9.2
	TLVISReachabilityNarrow = 2   // ISO/IEC 10589 clause 9.3 (decode-only)
	TLVISNeighbors          = 6   // ISO/IEC 10589 clause 9.4 (LAN SNPA list)
	TLVPadding              = 8   // ISO/IEC 10589 clause 9.10
	TLVLSPEntries           = 9   // ISO/IEC 10589 clause 9.14
	TLVAuthentication       = 10  // ISO/IEC 10589 clause 9.8 / RFC 5304
	TLVExtendedISReach      = 22  // RFC 5305 sec 3
	TLVProtocolsSupported   = 129 // RFC 1195
	TLVIPInterfaceAddress   = 132 // RFC 1195
	TLVExtendedIPReach      = 135 // RFC 5305 sec 4
	TLVDynamicHostname      = 137 // RFC 5301
	TLVIPv6InterfaceAddress = 232 // RFC 5308 sec 3
	TLVIPv6Reachability     = 236 // RFC 5308 sec 2
	TLVP2PThreeWay          = 240 // RFC 5303 sec 3.1
)

// TLVHeaderLen is the fixed type+length framing of one TLV or sub-TLV.
const TLVHeaderLen = 2

// MaxTLVValueLen is the largest value a single TLV can carry: the length field
// is one octet (ISO/IEC 10589 clause 9). A value longer than this must be
// split across multiple TLVs by the caller (e.g. fragmentation in isis-6).
const MaxTLVValueLen = 255

// TLVIterator walks a TLV region lazily, yielding (type, value-slice) pairs
// without copying. Decode is zero-copy per ai/rules/performance.md: the value
// slices alias the caller's buffer and are valid only while it is stable.
//
// The iterator never panics on malformed input (spec R-3, AC-11): a TLV whose
// declared length runs past the end of the region terminates iteration and is
// reported via Err. The same type is used for sub-TLV regions (RFC 5305 sec 2).
type TLVIterator struct {
	buf []byte
	off int
	err error
}

// NewTLVIterator returns an iterator over the TLV region buf. The region is the
// raw TLV bytes only (no PDU header); callers slice it out of the PDU first.
func NewTLVIterator(buf []byte) TLVIterator {
	return TLVIterator{buf: buf}
}

// Next returns the next (type, value) pair and ok=true, or ok=false at the end
// of the region or on a truncated TLV. Every read is bound-checked before
// slicing (security review: input validation). When a TLV's declared length
// exceeds the remaining bytes, Next stops and records ErrTruncated; the partial
// bytes are not yielded.
func (it *TLVIterator) Next() (typ uint8, value []byte, ok bool) {
	if it.err != nil {
		return 0, nil, false
	}
	// A clean end is exactly at the region boundary.
	if it.off == len(it.buf) {
		return 0, nil, false
	}
	// A single trailing octet (type with no length) is a truncation.
	if it.off+TLVHeaderLen > len(it.buf) {
		it.err = ErrTruncated
		return 0, nil, false
	}
	typ = it.buf[it.off]
	length := int(it.buf[it.off+1])
	valStart := it.off + TLVHeaderLen
	valEnd := valStart + length
	if valEnd > len(it.buf) {
		it.err = ErrTruncated
		return 0, nil, false
	}
	it.off = valEnd
	return typ, it.buf[valStart:valEnd], true
}

// Err returns the first error encountered (nil on a clean walk to the region
// boundary). It is set when a TLV's declared length overruns the region.
func (it *TLVIterator) Err() error { return it.err }

// writeTLV writes one TLV (type + length + value) into buf at off and returns
// the new offset. The value MUST be <= MaxTLVValueLen octets; longer values are
// a caller error (the length field is one octet) and panic in tests is avoided
// by the encoders validating length before calling. Buffer-first: the caller
// guarantees room (off + TLVHeaderLen + len(value) <= len(buf)).
func writeTLV(buf []byte, off int, typ uint8, value []byte) int {
	buf[off] = typ
	buf[off+1] = byte(len(value))
	off += TLVHeaderLen
	off += copy(buf[off:], value)
	return off
}

// tlvOverhead returns the encoded size of a TLV carrying a value of n octets.
func tlvOverhead(n int) int { return TLVHeaderLen + n }

// maxSubTLVs is the fixed initial capacity for decoded sub-TLV slices.
// A sub-TLV block fits in at most 255 bytes (single TLV value), so 8
// is a comfortable fixed size for pooling (real entries carry 1-3).
const maxSubTLVs = 8

// SubTLV is one sub-TLV (RFC 5305 sec 2): the same Type(1)+Length(1)+Value
// framing as a TLV, nested inside a TLV 22/135/236 entry. Value aliases the
// source buffer on decode. Unknown sub-TLVs are retained and re-emitted
// verbatim, matching the unknown-TLV passthrough contract one level down (RFC
// 5305 sec 2: "Unknown sub-TLVs are to be ignored and skipped upon receipt" --
// the codec keeps them so the entry round-trips, and leaves the ignore policy
// to the consumer).
type SubTLV struct {
	Type  uint8
	Value []byte
}

// EncodedLen returns the on-wire size of this sub-TLV.
func (s SubTLV) EncodedLen() int { return tlvOverhead(len(s.Value)) }

// decodeSubTLVs parses a sub-TLV block (the bytes after the 1-octet sub-TLV
// length field) into SubTLVs in order, retaining values as opaque spans. A
// truncated sub-TLV is reported via the error; sub-TLVs decoded before it are
// returned. Reused by TLV 22 / 135 / 236.
func decodeSubTLVs(block []byte) ([]SubTLV, error) {
	if len(block) == 0 {
		return nil, nil
	}
	out := make([]SubTLV, 0, maxSubTLVs)
	it := NewTLVIterator(block)
	for {
		typ, value, ok := it.Next()
		if !ok {
			break
		}
		out = append(out, SubTLV{Type: typ, Value: value})
	}
	return out, it.Err()
}

// subTLVsEncodedLen returns the total encoded size of a sub-TLV block.
func subTLVsEncodedLen(subs []SubTLV) int {
	n := 0
	for _, s := range subs {
		n += s.EncodedLen()
	}
	return n
}

// writeSubTLVs writes a sub-TLV block (no leading length octet) into buf at off
// and returns the new offset.
func writeSubTLVs(buf []byte, off int, subs []SubTLV) int {
	for _, s := range subs {
		off = writeTLV(buf, off, s.Type, s.Value)
	}
	return off
}
