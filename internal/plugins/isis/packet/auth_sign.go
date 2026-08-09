// Design: docs/architecture/isis/isis-10-auth.md -- IS-IS authentication backend (sign side).
//
// RFC: rfc/short/rfc5304.md -- IS-IS Cryptographic Authentication (HMAC-MD5, type 54)
// RFC: rfc/short/rfc5310.md -- IS-IS Generic Cryptographic Authentication (HMAC-SHA, type 3)
//
// SignPDU and the encode/layout/digest/checksum machinery. The digest and layout
// helpers here (computeDigest, the *AuthLayout locators, finalizeLSPChecksum) are
// SHARED with verification (auth_verify.go); the shared types/algorithm helpers
// live in auth_types.go. See auth_types.go for the package overview and the
// per-algorithm pre-image rules.

package packet

import "crypto/hmac"

// SignPDU signs a fully-constructed PDU with key and returns the final on-wire
// bytes. The input pdu MUST be the complete PDU (common header + body + TLVs),
// already padded for an IIH (RFC 5304 sec 2 signs padded Hellos) and WITHOUT a
// TLV 10. SignPDU:
//
//   - inserts TLV 10 as the FIRST TLV (RFC 5304 sec 1), sized to the algorithm,
//     with a zero (placeholder) Authentication Value;
//   - computes the digest over the PDU with the Authentication Value zeroed, and
//     for LSPs with the Checksum and Remaining Lifetime fields also zeroed
//     (RFC 5304 sec 2);
//   - writes the digest into the TLV 10 value (and the Key ID for the generic-
//     crypto family, RFC 5310 sec 3.1);
//   - for an LSP, recomputes the Fletcher checksum LAST so the signing order is
//     build -> sign -> checksum (spec AC-9, R-3); the Remaining Lifetime is
//     restored to its original value before the checksum runs.
//
// Cleartext (auth type 1) carries the password as the value and computes no
// digest (sanity only, not security; spec AC-5).
func SignPDU(pdu []byte, key Key) ([]byte, error) {
	if key.Algorithm == AuthAlgoNone {
		return pdu, nil // no auth configured: leave the PDU unchanged.
	}
	dec, err := DecodePDU(pdu)
	if err != nil {
		return nil, ErrAuthMalformed
	}
	defer dec.Release()
	class, ok := classOf(dec.Header.PDUType)
	if !ok {
		return nil, ErrAuthMalformed
	}
	value, err := placeholderValue(key)
	if err != nil {
		return nil, err
	}
	authType, _ := authTypeFor(key.Algorithm)
	authTLV := TLV{Type: TLVAuthentication, Value: append([]byte{authType}, value...)}

	signed, layout, err := reencodeWithAuthFirst(dec, authTLV)
	if err != nil {
		return nil, err
	}

	// Cleartext carries the password as the value; no digest is computed (the
	// value is already in place from reencodeWithAuthFirst). The checksum (LSP)
	// still needs recomputing because the value was inserted.
	if key.Algorithm == AuthAlgoCleartext {
		if class == classLSP {
			finalizeLSPChecksum(signed)
		}
		return signed, nil
	}

	digest, derr := computeDigest(signed, layout, key, class)
	if derr != nil {
		return nil, derr
	}
	// Write the digest into the Authentication Value (after the Key ID, if any).
	copy(signed[layout.authValueOff+keyIDOctets(key.Algorithm):], digest)

	// LSP: recompute the Fletcher checksum LAST, after the digest is in place
	// (RFC 5304 sec 2 / spec AC-9). The Remaining Lifetime was restored inside
	// computeDigest, so the region holds the final bytes.
	if class == classLSP {
		finalizeLSPChecksum(signed)
	}
	return signed, nil
}

// authLayout records, within an encoded PDU, the offsets the digest computation
// and back-patch need: where the TLV 10 value begins (the auth-type octet sits
// at authValueOff-1, so the zeroed region starts at authValueOff+keyIDOctets),
// the auth value length, and (LSP only) the Checksum and Remaining Lifetime
// field offsets.
type authLayout struct {
	authValueOff int // offset of the first value octet (the auth-type byte)
	authValueLen int // length of the TLV 10 value (auth-type + key-id + digest)
	// LSP-only field offsets (0 for non-LSP).
	checksumOff    int
	remLifetimeOff int
}

// reencodeWithAuthFirst rebuilds the decoded PDU with authTLV prepended as the
// first TLV (RFC 5304 sec 1) and returns the new bytes plus the layout of the
// inserted auth value. It reuses the isis-2 PDU encoders so no field layout is
// duplicated here.
func reencodeWithAuthFirst(dec PDU, authTLV TLV) ([]byte, authLayout, error) {
	switch {
	case dec.LANHello != nil:
		h := *dec.LANHello
		h.TLVs = prependTLV(h.TLVs, authTLV)
		buf := make([]byte, h.EncodedLen())
		n := h.WriteTo(buf, 0)
		return buf[:n], helloAuthLayout(buf[:n]), nil
	case dec.P2PHello != nil:
		h := *dec.P2PHello
		h.TLVs = prependTLV(h.TLVs, authTLV)
		buf := make([]byte, h.EncodedLen())
		n := h.WriteTo(buf, 0)
		return buf[:n], helloAuthLayout(buf[:n]), nil
	case dec.CSNP != nil:
		c := *dec.CSNP
		c.TLVs = prependTLV(c.TLVs, authTLV)
		buf := make([]byte, c.EncodedLen())
		n := c.WriteTo(buf, 0)
		return buf[:n], snpAuthLayout(buf[:n]), nil
	case dec.PSNP != nil:
		p := *dec.PSNP
		p.TLVs = prependTLV(p.TLVs, authTLV)
		buf := make([]byte, p.EncodedLen())
		n := p.WriteTo(buf, 0)
		return buf[:n], snpAuthLayout(buf[:n]), nil
	case dec.LSP != nil:
		l := *dec.LSP
		l.RawBytes = nil
		l.TLVs = prependTLV(l.TLVs, authTLV)
		buf := make([]byte, l.EncodedLen())
		n := l.WriteTo(buf, 0)
		return buf[:n], lspAuthLayout(buf[:n]), nil
	default:
		return nil, authLayout{}, ErrAuthMalformed
	}
}

// prependTLV returns a new slice with t as the first element. The original
// auth-first ordering invariant (RFC 5304 sec 1) is established here.
func prependTLV(tlvs []TLV, t TLV) []TLV {
	out := make([]TLV, 0, len(tlvs)+1)
	out = append(out, t)
	out = append(out, tlvs...)
	return out
}

// helloAuthLayout locates TLV 10 in an encoded IIH (its first TLV, just after the
// fixed header). A LAN IIH fixed body is longer than a P2P IIH; the TLV region
// is found by walking the TLV iterator from the body start. Returns a zero
// layout if no TLV 10 is found (should not happen for a freshly signed PDU).
func helloAuthLayout(pdu []byte) authLayout {
	return firstAuthValueLayout(pdu)
}

// snpAuthLayout locates TLV 10 in an encoded CSNP/PSNP.
func snpAuthLayout(pdu []byte) authLayout {
	return firstAuthValueLayout(pdu)
}

// lspAuthLayout locates TLV 10 in an encoded LSP and records the Checksum and
// Remaining Lifetime field offsets (RFC 5304 sec 2 zeroes all three before the
// digest).
func lspAuthLayout(pdu []byte) authLayout {
	l := firstAuthValueLayout(pdu)
	l.remLifetimeOff = CommonHeaderLen + lspRemLifetimeOff
	l.checksumOff = CommonHeaderLen + lspChecksumFieldBodyOff
	return l
}

// firstAuthValueLayout walks the PDU's TLV region and returns the offset and
// length of the FIRST TLV 10's authentication value (the octets AFTER the
// auth-type byte: the optional 2-octet Key ID then the digest). It locates the
// TLV region by re-decoding the PDU header and stepping over the fixed fields;
// the TLV region start is the PDU body offset reported by the codec. Because a
// freshly signed PDU has TLV 10 first, this finds it immediately.
//
// authValueOff therefore points one octet PAST the auth-type byte, so the digest
// offset is authValueOff+keyIDOctets and the digest end is
// authValueOff+authValueLen. A TLV 10 of length 1 (auth-type byte only, no value)
// yields authValueLen 0.
func firstAuthValueLayout(pdu []byte) authLayout {
	off, ok := tlvRegionStart(pdu)
	if !ok {
		return authLayout{}
	}
	// The first TLV must be TLV 10 (we just prepended it). Validate framing.
	if off+TLVHeaderLen > len(pdu) || pdu[off] != TLVAuthentication {
		return authLayout{}
	}
	length := int(pdu[off+1])
	if length < 1 {
		return authLayout{}
	}
	valStart := off + TLVHeaderLen // the auth-type byte
	if valStart+length > len(pdu) {
		return authLayout{}
	}
	// Skip the auth-type byte: the authentication value begins at valStart+1.
	return authLayout{authValueOff: valStart + 1, authValueLen: length - 1}
}

// tlvRegionStart returns the byte offset where the TLV region begins in a fully
// encoded PDU, by parsing the common header and stepping over the PDU-specific
// fixed fields. It mirrors the fixed-field widths the body encoders use.
func tlvRegionStart(pdu []byte) (int, bool) {
	if len(pdu) < CommonHeaderLen {
		return 0, false
	}
	pt := PDUType(pdu[offPDUType] & pduTypeMask)
	switch pt {
	case PDUTypeL1LANHello, PDUTypeL2LANHello:
		return CommonHeaderLen + lanHelloFixedLen, true
	case PDUTypeP2PHello:
		return CommonHeaderLen + p2pHelloFixedLen, true
	case PDUTypeL1LSP, PDUTypeL2LSP:
		return CommonHeaderLen + lspFixedLen, true
	case PDUTypeL1CSNP, PDUTypeL2CSNP:
		return CommonHeaderLen + csnpFixedLen, true
	case PDUTypeL1PSNP, PDUTypeL2PSNP:
		return CommonHeaderLen + psnpFixedLen, true
	default:
		return 0, false
	}
}

// computeDigest computes the authentication digest over signed with the
// Authentication Value zeroed (and, for an LSP, the Checksum and Remaining
// Lifetime zeroed too -- RFC 5304 sec 2). It saves and restores the three fields
// so the caller's buffer is left holding the real (non-zeroed) values, ready for
// the digest to be written and (LSP) the Fletcher checksum to be computed.
func computeDigest(signed []byte, layout authLayout, key Key, class pduClass) ([]byte, error) {
	h := newHash(key.Algorithm)
	if h == nil {
		return nil, ErrAuthUnsupported
	}
	kidLen := keyIDOctets(key.Algorithm)
	digOff := layout.authValueOff + kidLen
	digEnd := layout.authValueOff + layout.authValueLen
	if digOff > len(signed) || digEnd > len(signed) || digOff > digEnd {
		return nil, ErrAuthMalformed
	}

	// Save the fields the digest computation zeroes, so they can be restored.
	savedDigest := append([]byte(nil), signed[digOff:digEnd]...)
	var savedChecksum, savedLifetime [2]byte
	if class == classLSP {
		copy(savedChecksum[:], signed[layout.checksumOff:layout.checksumOff+2])
		copy(savedLifetime[:], signed[layout.remLifetimeOff:layout.remLifetimeOff+2])
	}

	// Fill the Authentication Data field with the algorithm's agreed pre-image.
	// The two RFCs disagree on what that pre-image is, and an interop peer (FRR)
	// follows the RFC literally, so Ze MUST match per-algorithm:
	//   - HMAC-MD5 (RFC 5304 sec 2): the value is ZEROED before the digest.
	//   - HMAC-SHA family (RFC 5310 sec 3.3 step 1 / sec 3.5): the value is filled
	//     with Apad (0x878FE1F3 repeated) before the digest, on BOTH sign and
	//     verify. Zeroing here would make every Ze<->FRR HMAC-SHA digest mismatch.
	if key.Algorithm == AuthAlgoHMACMD5 {
		for i := digOff; i < digEnd; i++ {
			signed[i] = 0
		}
	} else {
		// RFC 5310 sec 3.3: Authentication Data filled with Apad before hashing.
		fillApad(signed, digOff, digEnd)
	}
	if class == classLSP {
		// RFC 5304 sec 2: zero the Checksum and Remaining Lifetime before the digest.
		signed[layout.checksumOff], signed[layout.checksumOff+1] = 0, 0
		signed[layout.remLifetimeOff], signed[layout.remLifetimeOff+1] = 0, 0
	}

	mac := hmac.New(h, key.Secret)
	mac.Write(signed)
	digest := mac.Sum(nil)

	// Restore the saved fields (the digest is written by the caller; the LSP
	// checksum is computed by the caller after the digest is placed).
	copy(signed[digOff:digEnd], savedDigest)
	if class == classLSP {
		copy(signed[layout.checksumOff:layout.checksumOff+2], savedChecksum[:])
		copy(signed[layout.remLifetimeOff:layout.remLifetimeOff+2], savedLifetime[:])
	}
	return digest, nil
}

// finalizeLSPChecksum recomputes the ISO Fletcher checksum over an encoded LSP's
// checksum region (the octets after Remaining Lifetime) and writes it into the
// Checksum field. Called LAST on the send path so the digest is already in place
// (RFC 5304 sec 2 / spec AC-9). The region offsets mirror lsp.go.
func finalizeLSPChecksum(pdu []byte) {
	regionStart := CommonHeaderLen + lspChecksumRegionStart
	checkPos := CommonHeaderLen + lspChecksumFieldBodyOff
	if len(pdu) < regionStart || checkPos+1 >= len(pdu) {
		return
	}
	region := pdu[regionStart:]
	high, low := Checksum(region, lspChecksumRegionCheckOff)
	pdu[checkPos] = high
	pdu[checkPos+1] = low
}

// StripPurgeBody returns the canonical authenticated-purge form of an LSP: the
// LSP with its body removed (RFC 5304 sec 2: "ISes that ... initiate LSP purges
// MUST remove the body of the LSP and add the authentication TLV"). The returned
// PDU has Remaining Lifetime 0, no TLVs (the caller signs it, which adds TLV 10
// as the only TLV), and a recomputed length. The caller then SignPDUs it. seq is
// the purge sequence number the originator assigns.
func StripPurgeBody(lsp *LSP) *LSP {
	return &LSP{
		PDUType:           lsp.PDUType,
		RemainingLifetime: 0,
		LSPID:             lsp.LSPID,
		SequenceNumber:    lsp.SequenceNumber,
		TypeBlock:         lsp.TypeBlock,
		MaxAreaAddresses:  lsp.MaxAreaAddresses,
		TLVs:              nil,
	}
}
