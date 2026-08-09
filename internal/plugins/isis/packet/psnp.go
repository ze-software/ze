// Design: docs/architecture/wire/isis.md -- L1/L2 PSNP body codec (source ID)
// ISO/IEC 10589 clause 9.11 (Partial Sequence Numbers PDU).

package packet

import "github.com/ze-software/ze/internal/plugins/isis/types"

// PSNP fixed-header layout after the common header (ISO/IEC 10589 clause 9.11):
// PDU Length (2) + Source ID (7). A PSNP carries no Fletcher checksum and no
// start/end range (unlike a CSNP); it requests or acknowledges specific LSPs
// via TLV 9.
const psnpFixedLen = 2 + types.SourceIDLen

// PSNP is a decoded Level 1 or Level 2 Partial Sequence Numbers PDU (ISO/IEC
// 10589 clause 9.11). It carries TLV 9 (LSP Entries) to acknowledge or request
// individual LSPs. The PDU type distinguishes L1 (0x1a) from L2 (0x1b).
type PSNP struct {
	PDUType          PDUType // PDUTypeL1PSNP or PDUTypeL2PSNP
	SourceID         types.SourceID
	MaxAreaAddresses uint8 // common-header field; 0 = the default 3
	TLVs             []TLV
}

// EncodedLen returns the total on-wire size of the PSNP.
func (p *PSNP) EncodedLen() int {
	return CommonHeaderLen + psnpFixedLen + tlvsEncodedLen(p.TLVs)
}

// WriteTo serializes the PSNP into buf at off; PDU Length via skip-and-backfill.
// Buffer-first.
func (p *PSNP) WriteTo(buf []byte, off int) int {
	start := off
	off = writeCommonHeader(buf, off, p.PDUType, CommonHeaderLen+uint8(psnpFixedLen), p.MaxAreaAddresses)
	pduLenPos := off
	off += 2
	off += p.SourceID.WriteTo(buf, off)
	off = writeTLVs(buf, off, p.TLVs)
	total := off - start
	buf[pduLenPos] = byte(total >> 8)
	buf[pduLenPos+1] = byte(total)
	return off
}

// DecodePSNP parses a PSNP body following the common header.
func DecodePSNP(pt PDUType, body []byte) (PSNP, error) {
	if len(body) < psnpFixedLen {
		return PSNP{}, ErrTruncated
	}
	p := PSNP{PDUType: pt}
	off := 0
	pduLen := int(body[off])<<8 | int(body[off+1])
	off += 2
	src, _ := types.SourceIDFromBytes(body[off : off+types.SourceIDLen])
	p.SourceID = src
	off += types.SourceIDLen

	tlvRegion, err := pduTLVRegion(body, off, pduLen)
	if err != nil {
		return PSNP{}, err
	}
	tlvs, err := DecodeTLVs(tlvRegion)
	if err != nil {
		return PSNP{}, err
	}
	p.TLVs = tlvs
	return p, nil
}
