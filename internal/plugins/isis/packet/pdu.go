// Design: docs/architecture/wire/isis.md -- top-level PDU dispatch (header parse -> body decoder)
// ISO/IEC 10589 clause 9: the PDU type octet selects the body layout.

package packet

// PDU is the decoded union returned by DecodePDU. Exactly one of the typed
// pointers is non-nil, selected by Header.PDUType. The runtime dispatcher
// (isis-4 server.go) keys off Header.PDUType; this struct lets a single decode
// call return both the parsed header and the typed body without the caller
// re-switching on the type. The IIH/LSP/SNP body parsers can also be called
// directly when the PDU type is already known.
type PDU struct {
	Header   Header
	LANHello *LANHello
	P2PHello *P2PHello
	LSP      *LSP
	CSNP     *CSNP
	PSNP     *PSNP
}

// Release returns the decoded TLV slice to the pool. Callers must not access
// the PDU's TLVs after this call.
func (p *PDU) Release() {
	switch {
	case p.LANHello != nil:
		ReleaseTLVs(p.LANHello.TLVs)
	case p.P2PHello != nil:
		ReleaseTLVs(p.P2PHello.TLVs)
	case p.LSP != nil:
		ReleaseTLVs(p.LSP.TLVs)
	case p.CSNP != nil:
		ReleaseTLVs(p.CSNP.TLVs)
	case p.PSNP != nil:
		ReleaseTLVs(p.PSNP.TLVs)
	}
}

// DecodePDU parses a complete IS-IS PDU: the common header followed by the
// type-specific body. It validates the common header (discriminator, version,
// ID length, known PDU type), dispatches by the 5-bit PDU type to the matching
// body decoder, and threads the common-header Max Area Addresses field into the
// typed body. It never panics on arbitrary input (AC-11, R-3): a bad header or
// a truncated body returns a typed error.
//
// buf is one whole PDU (the 802.3 + LLC framing already stripped by isis-3, or
// supplied directly by the offline decode CLI). Decoded TLV value slices alias
// buf; the caller copies any it retains.
func DecodePDU(buf []byte) (PDU, error) {
	h, bodyOff, err := DecodeHeader(buf)
	if err != nil {
		return PDU{}, err
	}
	body := buf[bodyOff:]
	out := PDU{Header: h}

	switch h.PDUType {
	case PDUTypeL1LANHello, PDUTypeL2LANHello:
		hello, err := DecodeLANHello(h.PDUType, body)
		if err != nil {
			return PDU{}, err
		}
		hello.MaxAreaAddresses = h.MaxAreaAddresses
		out.LANHello = &hello
	case PDUTypeP2PHello:
		hello, err := DecodeP2PHello(body)
		if err != nil {
			return PDU{}, err
		}
		hello.MaxAreaAddresses = h.MaxAreaAddresses
		out.P2PHello = &hello
	case PDUTypeL1LSP, PDUTypeL2LSP:
		lsp, err := DecodeLSP(h.PDUType, buf, body)
		if err != nil {
			return PDU{}, err
		}
		lsp.MaxAreaAddresses = h.MaxAreaAddresses
		out.LSP = &lsp
	case PDUTypeL1CSNP, PDUTypeL2CSNP:
		csnp, err := DecodeCSNP(h.PDUType, body)
		if err != nil {
			return PDU{}, err
		}
		csnp.MaxAreaAddresses = h.MaxAreaAddresses
		out.CSNP = &csnp
	case PDUTypeL1PSNP, PDUTypeL2PSNP:
		psnp, err := DecodePSNP(h.PDUType, body)
		if err != nil {
			return PDU{}, err
		}
		psnp.MaxAreaAddresses = h.MaxAreaAddresses
		out.PSNP = &psnp
	default:
		// DecodeHeader already rejected unknown types; this is unreachable but
		// kept for exhaustiveness.
		return PDU{}, ErrUnknownPDUType
	}
	return out, nil
}
