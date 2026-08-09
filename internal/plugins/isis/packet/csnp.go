// Design: docs/architecture/wire/isis.md -- L1/L2 CSNP body codec (source ID, start/end LSPID)
// ISO/IEC 10589 clause 9.10 (Complete Sequence Numbers PDU).

package packet

import "github.com/ze-software/ze/internal/plugins/isis/types"

// CSNP fixed-header layout after the common header (ISO/IEC 10589 clause 9.10):
// PDU Length (2) + Source ID (7) + Start LSP ID (8) + End LSP ID (8). A CSNP
// carries no Fletcher checksum (only LSPs do).
const csnpFixedLen = 2 + types.SourceIDLen + types.LSPIDLen + types.LSPIDLen

// CSNP is a decoded Level 1 or Level 2 Complete Sequence Numbers PDU (ISO/IEC
// 10589 clause 9.10). It summarizes the sender's LSDB over the LSP-ID range
// [Start, End] via TLV 9 (LSP Entries). The PDU type distinguishes L1 (0x18)
// from L2 (0x19).
type CSNP struct {
	PDUType          PDUType // PDUTypeL1CSNP or PDUTypeL2CSNP
	SourceID         types.SourceID
	StartLSPID       types.LSPID
	EndLSPID         types.LSPID
	MaxAreaAddresses uint8 // common-header field; 0 = the default 3
	TLVs             []TLV
}

// EncodedLen returns the total on-wire size of the CSNP.
func (c *CSNP) EncodedLen() int {
	return CommonHeaderLen + csnpFixedLen + tlvsEncodedLen(c.TLVs)
}

// WriteTo serializes the CSNP into buf at off; PDU Length via skip-and-backfill.
// Buffer-first.
func (c *CSNP) WriteTo(buf []byte, off int) int {
	start := off
	off = writeCommonHeader(buf, off, c.PDUType, CommonHeaderLen+uint8(csnpFixedLen), c.MaxAreaAddresses)
	pduLenPos := off
	off += 2
	off += c.SourceID.WriteTo(buf, off)
	off += c.StartLSPID.WriteTo(buf, off)
	off += c.EndLSPID.WriteTo(buf, off)
	off = writeTLVs(buf, off, c.TLVs)
	total := off - start
	buf[pduLenPos] = byte(total >> 8)
	buf[pduLenPos+1] = byte(total)
	return off
}

// DecodeCSNP parses a CSNP body following the common header.
func DecodeCSNP(pt PDUType, body []byte) (CSNP, error) {
	if len(body) < csnpFixedLen {
		return CSNP{}, ErrTruncated
	}
	c := CSNP{PDUType: pt}
	off := 0
	pduLen := int(body[off])<<8 | int(body[off+1])
	off += 2
	src, _ := types.SourceIDFromBytes(body[off : off+types.SourceIDLen])
	c.SourceID = src
	off += types.SourceIDLen
	start, _ := types.LSPIDFromBytes(body[off : off+types.LSPIDLen])
	c.StartLSPID = start
	off += types.LSPIDLen
	end, _ := types.LSPIDFromBytes(body[off : off+types.LSPIDLen])
	c.EndLSPID = end
	off += types.LSPIDLen

	tlvRegion, err := pduTLVRegion(body, off, pduLen)
	if err != nil {
		return CSNP{}, err
	}
	tlvs, err := DecodeTLVs(tlvRegion)
	if err != nil {
		return CSNP{}, err
	}
	c.TLVs = tlvs
	return c, nil
}
