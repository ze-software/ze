// Design: docs/architecture/isis/isis-10-auth.md -- IS-IS authentication backend (verify side).
//
// RFC: rfc/short/rfc5304.md -- IS-IS Cryptographic Authentication (HMAC-MD5, type 54)
// RFC: rfc/short/rfc5310.md -- IS-IS Generic Cryptographic Authentication (HMAC-SHA, type 3)
//
// VerifyPDU and its receive-side helpers. The shared types/algorithm helpers
// live in auth_types.go; the encode/layout/digest/checksum machinery (shared
// with verification) lives in auth_sign.go. See auth_types.go for the package
// overview and the per-algorithm pre-image rules.

package packet

import "crypto/hmac"

// VerifyPDU verifies a received PDU against the candidate keys. It enforces the
// downgrade and ordering rules and, on a match, returns nil; on any failure it
// returns a typed error and the caller drops the PDU and increments
// ze_isis_auth_failures_total (spec AC-1, AC-2). The candidate keys are every
// currently-valid key in the relevant chain (the active signing key plus all
// accepted-on-receive keys, for hitless rotation -- spec AC-4).
//
// Behavior:
//   - No candidate keys: authentication is not configured for this PDU class;
//     VerifyPDU returns nil (unauthenticated operation, the default).
//   - TLV 10 absent under configured auth: ErrAuthMissing (R-6 downgrade).
//   - TLV 10 present but not first: ErrAuthNotFirst (RFC 5304 sec 1, AC-8).
//   - For each candidate key whose algorithm matches the received auth type,
//     recompute the digest with the proper field zeroing and constant-time
//     compare (AC-12); any match accepts.
//   - For an LSP that is a purge (Remaining Lifetime 0), additionally enforce
//     RFC 5304 sec 2: the purge MUST be authenticated (handled by the digest
//     check) and MUST NOT carry any TLV other than TLV 10 (ErrAuthPurgeExtraTLV).
func VerifyPDU(pdu []byte, keys []Key) error {
	if len(keys) == 0 {
		return nil // authentication not configured for this PDU class.
	}
	dec, err := DecodePDU(pdu)
	if err != nil {
		return ErrAuthMalformed
	}
	defer dec.Release()
	class, ok := classOf(dec.Header.PDUType)
	if !ok {
		return ErrAuthMalformed
	}
	tlvs := pduTLVs(dec)

	idx := AuthTLVIndex(tlvs)
	if idx < 0 {
		return ErrAuthMissing
	}
	if idx != 0 {
		return ErrAuthNotFirst
	}

	// Authenticated-purge rule (RFC 5304 sec 2): a purge carries ONLY TLV 10.
	if class == classLSP && dec.LSP != nil && dec.LSP.RemainingLifetime.IsPurge() {
		if len(tlvs) > 1 {
			return ErrAuthPurgeExtraTLV
		}
	}

	authTLV, derr := decodeAuthTLV(tlvs[0].Value)
	if derr != nil {
		return ErrAuthMalformed
	}

	// One scratch buffer for the whole key chain. computeDigest restores every
	// field it zeroes (auth value, and for an LSP the Checksum + Remaining
	// Lifetime), so after each candidate the scratch holds the original PDU bytes
	// again and can be reused. We copy ONCE here instead of per candidate key:
	// a multi-key chain (hitless rotation, spec AC-4) would otherwise allocate N
	// full PDU copies. The caller's receive buffer (pdu) is never mutated.
	var scratch []byte
	matched := false
	for _, key := range keys {
		t, valid := authTypeFor(key.Algorithm)
		if !valid || t != authTLV.AuthType {
			continue
		}
		matched = true
		if scratch == nil {
			scratch = append([]byte(nil), pdu...)
		}
		if verifyKey(scratch, dec, class, authTLV, key) {
			return nil
		}
	}
	if !matched {
		return ErrAuthTypeMismatch
	}
	return ErrAuthMismatch
}

// verifyKey recomputes the digest for one candidate key and constant-time
// compares it against the received value. Cleartext is a constant-time compare
// of the password. Returns true on a match.
//
// scratch is a writable COPY of the received PDU owned by VerifyPDU and shared
// across the candidate-key loop: the caller's receive buffer is never mutated
// (zero-copy preserved). computeDigest restores every field it zeroes, so on
// return scratch again holds the original PDU bytes, ready for the next key.
func verifyKey(scratch []byte, dec PDU, class pduClass, authTLV AuthTLV, key Key) bool {
	if key.Algorithm == AuthAlgoCleartext {
		// Cleartext (sanity only): constant-time compare the password against the
		// received value (RFC: ISO/IEC 10589 cleartext password). AC-5, AC-12.
		// This branch does not touch scratch.
		return hmac.Equal(authTLV.Value, key.Secret)
	}

	// For the generic-crypto family, the received value is Key-ID(2) || digest.
	kidLen := keyIDOctets(key.Algorithm)
	if len(authTLV.Value) < kidLen+digestLen(key.Algorithm) {
		return false
	}
	if kidLen == 2 {
		recvKeyID := uint16(authTLV.Value[0])<<8 | uint16(authTLV.Value[1])
		if recvKeyID != key.KeyID {
			return false // a different SA; not this key (RFC 5310 sec 3.5).
		}
	}
	recvDigest := authTLV.Value[kidLen : kidLen+digestLen(key.Algorithm)]

	// Recompute over the scratch copy with the proper zeroing. We reconstruct the
	// same layout the sender used: TLV 10 first, value at its received length.
	// Because the received PDU already has TLV 10 first (validated above), we
	// locate the auth value by re-deriving the layout from scratch.
	layout := authLayoutForReceived(scratch, dec, class)
	if layout.authValueLen == 0 {
		return false
	}
	digest, err := computeDigest(scratch, layout, key, class)
	if err != nil {
		return false
	}
	// Constant-time compare (AC-12, R-1): hmac.Equal is constant time.
	return hmac.Equal(digest, recvDigest)
}

// authLayoutForReceived derives the auth layout for a RECEIVED PDU (TLV 10 first,
// validated). Unlike the freshly-signed case, the received TLV region begins
// after the fixed fields and the first TLV is TLV 10; we reuse the same locator.
func authLayoutForReceived(pdu []byte, dec PDU, class pduClass) authLayout {
	switch class {
	case classLSP:
		return lspAuthLayout(pdu)
	default:
		_ = dec
		return firstAuthValueLayout(pdu)
	}
}

// pduTLVs returns the decoded TLV list of whichever body the PDU carries.
func pduTLVs(dec PDU) []TLV {
	switch {
	case dec.LANHello != nil:
		return dec.LANHello.TLVs
	case dec.P2PHello != nil:
		return dec.P2PHello.TLVs
	case dec.LSP != nil:
		return dec.LSP.TLVs
	case dec.CSNP != nil:
		return dec.CSNP.TLVs
	case dec.PSNP != nil:
		return dec.PSNP.TLVs
	default:
		return nil
	}
}
