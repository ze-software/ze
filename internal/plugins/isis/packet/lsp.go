// Design: docs/architecture/wire/isis.md -- L1/L2 LSP body codec (lifetime, LSPID, sequence, checksum, type block)
// ISO/IEC 10589 clause 9.8 (Link State PDU), clause 7.3.11 (Fletcher checksum).
//
// RFC: rfc/short/rfc3787.md -- LSP-database-overload (OL) bit usage
// RFC: rfc/short/rfc5304.md -- LSP authentication zeroes Checksum + Remaining Lifetime before signing (sec 2)

package packet

import "github.com/ze-software/ze/internal/plugins/isis/types"

// LSP fixed-header layout after the common header (ISO/IEC 10589 clause 9.8):
//
//	PDU Length        (2)
//	Remaining Lifetime(2)  <-- checksum region begins at the NEXT octet
//	LSP ID            (8)
//	Sequence Number   (4)
//	Checksum          (2)
//	Type block        (1)  (P/ATT/OL/IS-type flags)
//	TLVs              ...
const (
	lspFixedLen = 2 + types.LifetimeLen + types.LSPIDLen + types.SequenceNumberLen + 2 + 1

	// lspPDULenOff is the offset of the PDU Length field within the LSP body
	// (immediately after the common header).
	lspPDULenOff = 0
	// lspRemLifetimeOff is the offset of the Remaining Lifetime field within
	// the body. The Fletcher checksum region begins at the octet after it.
	lspRemLifetimeOff = 2
	// lspChecksumRegionStart is the body offset where the checksummed region
	// begins (the octet after Remaining Lifetime): LSP ID onward.
	lspChecksumRegionStart = lspRemLifetimeOff + types.LifetimeLen
	// lspChecksumFieldBodyOff is the body offset of the 2-octet Checksum field.
	lspChecksumFieldBodyOff = lspChecksumRegionStart + types.LSPIDLen + types.SequenceNumberLen
	// lspTypeBlockBodyOff is the body offset of the 1-octet type block.
	lspTypeBlockBodyOff = lspChecksumFieldBodyOff + 2
)

// Type-block bit masks (ISO/IEC 10589 clause 9.8). The high bit P is the
// partition-repair flag; ATT are the four attached-metric bits; OL is the
// LSP-database-overload bit; the low two bits are the IS type.
const (
	LSPFlagPartition = 0x80 // P: partition repair supported
	LSPFlagOverload  = 0x04 // OL: LSP database overload (RFC 3787)
	LSPFlagISTypeL1  = 0x01 // IS type low bit
	LSPFlagISTypeL2  = 0x02 // IS type high bit
	// LSPAttachedMask covers the four ATT bits (default/delay/expense/error
	// metric attached); set by an L1L2 router in its L1 LSP.
	LSPAttachedMask = 0x78
)

// LSP is a decoded Level 1 or Level 2 Link State PDU (ISO/IEC 10589 clause 9.8).
// The PDU type distinguishes L1 (0x12) from L2 (0x14). RawBytes, when set on a
// decode, is the full PDU slice (for the LSDB to store and re-flood verbatim);
// it is nil on a struct built for encoding. TLVs are retained in order so an
// unknown TLV re-floods verbatim (ISO/IEC 10589 clause 7.3.14).
type LSP struct {
	PDUType           PDUType // PDUTypeL1LSP or PDUTypeL2LSP
	RemainingLifetime types.RemainingLifetime
	LSPID             types.LSPID
	SequenceNumber    types.SequenceNumber
	Checksum          uint16 // as decoded; recomputed and backfilled on WriteTo
	TypeBlock         uint8
	MaxAreaAddresses  uint8 // common-header field; 0 = the default 3
	TLVs              []TLV

	// RawBytes is the verbatim PDU as decoded (nil when the struct was built
	// for encoding). The LSDB retains this for re-flood.
	RawBytes []byte
}

// EncodedLen returns the total on-wire size of the LSP.
func (l *LSP) EncodedLen() int {
	return CommonHeaderLen + lspFixedLen + tlvsEncodedLen(l.TLVs)
}

// WriteTo serializes the LSP into buf at off and returns the new offset. The
// PDU Length is written via skip-and-backfill, and the ISO 8473 Fletcher
// checksum is computed over the region from the octet after Remaining Lifetime
// to the end and backfilled LAST (ISO/IEC 10589 clause 7.3.11). The Checksum
// field of the struct is ignored on encode; the computed value is authoritative
// and is also stored back into l.Checksum for the caller. Buffer-first.
//
// Note (RFC 5304 sec 2 / isis-10): when authentication is enabled, isis-10
// signs the PDU with the Checksum and Remaining Lifetime fields zeroed, AFTER
// padding, and BEFORE this checksum is computed. This codec computes the
// checksum over whatever auth TLV bytes are present; the engine's send order
// (build -> sign TLV 10 -> Fletcher checksum) guarantees the auth value is in
// place when the checksum runs.
func (l *LSP) WriteTo(buf []byte, off int) int {
	start := off
	off = writeCommonHeader(buf, off, l.PDUType, CommonHeaderLen+uint8(lspFixedLen), l.MaxAreaAddresses)
	bodyStart := off

	pduLenPos := off // PDU Length (skip-and-backfill)
	off += 2
	off += l.RemainingLifetime.WriteTo(buf, off)
	off += l.LSPID.WriteTo(buf, off)
	off += l.SequenceNumber.WriteTo(buf, off)
	checksumPos := off // Checksum field; zeroed now, backfilled last
	buf[off] = 0
	buf[off+1] = 0
	off += 2
	buf[off] = l.TypeBlock
	off++
	off = writeTLVs(buf, off, l.TLVs)

	total := off - start
	buf[pduLenPos] = byte(total >> 8)
	buf[pduLenPos+1] = byte(total)

	// Fletcher checksum over the region after Remaining Lifetime to the PDU
	// end. The checksum field sits at lspChecksumRegionCheckOff within that
	// region (LSPID 8 + sequence 4 = 12).
	region := buf[bodyStart+lspChecksumRegionStart : off]
	high, low := Checksum(region, lspChecksumRegionCheckOff)
	buf[checksumPos] = high
	buf[checksumPos+1] = low
	l.Checksum = uint16(high)<<8 | uint16(low)

	return off
}

// DecodeLSP parses an LSP body following the common header. body is the slice
// after the common header; full is the entire PDU slice (common header + body)
// so RawBytes can be retained for verbatim re-flood. Every field is
// bound-checked before slicing (security review).
func DecodeLSP(pt PDUType, full, body []byte) (LSP, error) {
	if len(body) < lspFixedLen {
		return LSP{}, ErrTruncated
	}
	l := LSP{PDUType: pt, RawBytes: full}
	pduLen := int(body[lspPDULenOff])<<8 | int(body[lspPDULenOff+1])
	lifetime, _ := types.RemainingLifetimeFromBytes(body[lspRemLifetimeOff : lspRemLifetimeOff+types.LifetimeLen])
	l.RemainingLifetime = lifetime
	lspid, _ := types.LSPIDFromBytes(body[lspChecksumRegionStart : lspChecksumRegionStart+types.LSPIDLen])
	l.LSPID = lspid
	seqOff := lspChecksumRegionStart + types.LSPIDLen
	seq, _ := types.SequenceNumberFromBytes(body[seqOff : seqOff+types.SequenceNumberLen])
	l.SequenceNumber = seq
	l.Checksum = uint16(body[lspChecksumFieldBodyOff])<<8 | uint16(body[lspChecksumFieldBodyOff+1])
	l.TypeBlock = body[lspTypeBlockBodyOff]

	tlvStart := lspTypeBlockBodyOff + 1
	tlvRegion, err := pduTLVRegion(body, tlvStart, pduLen)
	if err != nil {
		return LSP{}, err
	}
	tlvs, err := DecodeTLVs(tlvRegion)
	if err != nil {
		return LSP{}, err
	}
	l.TLVs = tlvs
	return l, nil
}

// LSPChecksumOf reads the 2-octet Checksum field out of an ENCODED LSP PDU
// (common header included) and reports whether the PDU was long enough to hold
// it. It exists because the checksum in the bytes and the Checksum field of the
// LSP struct can diverge: SignPDU inserts TLV 10 and recomputes the checksum in
// the BYTES (finalizeLSPChecksum) without touching the struct WriteTo filled in.
// Anything that stores or advertises an LSP's checksum must read it from the
// bytes it actually stored, never from a struct built before signing.
func LSPChecksumOf(pdu []byte) (uint16, bool) {
	off := CommonHeaderLen + lspChecksumFieldBodyOff
	if off+1 >= len(pdu) {
		return 0, false
	}
	return uint16(pdu[off])<<8 | uint16(pdu[off+1]), true
}

// VerifyChecksum recomputes the Fletcher checksum over this LSP's raw bytes and
// reports whether it is valid (ISO/IEC 10589 clause 7.3.11). It requires
// RawBytes to be set (a decoded LSP); for an LSP built in memory the checksum
// is computed by WriteTo, so verify the encoded output instead. Returns false
// if RawBytes is too short to contain the checksum region.
func (l *LSP) VerifyChecksum() bool {
	// The checksummed region is from the octet after Remaining Lifetime to the
	// PDU end. Within RawBytes (which includes the common header), that begins
	// at CommonHeaderLen + lspChecksumRegionStart.
	regionStart := CommonHeaderLen + lspChecksumRegionStart
	if len(l.RawBytes) < regionStart {
		return false
	}
	return VerifyChecksum(l.RawBytes[regionStart:])
}

// IsOverloaded reports whether the LSP-database-overload (OL) bit is set in the
// type block (RFC 3787); SPF (isis-9) treats an overloaded node as transit-only.
func (l *LSP) IsOverloaded() bool { return l.TypeBlock&LSPFlagOverload != 0 }
