// Design: docs/architecture/isis/isis-10-auth.md -- ISO 9542 per-link passwords.
// RFC 1195 Annex D uses TLV133 with authentication type1 for cleartext ISH passwords.

package packet

import "crypto/hmac"

// TLVISHAuthentication is RFC 1195 Annex D's Authentication Information code.
// Its value is one authentication-type octet followed by the password bytes.
const TLVISHAuthentication = 133

// SignISH appends a cleartext password to a complete ISH and recomputes its
// Fletcher checksum. It mutates the caller's buffer and returns the expanded
// slice. The caller MUST retain capacity for 3+len(key.Secret) additional octets,
// and MUST NOT pass a PDU already containing Authentication Information.
// HMAC formats from RFC 5304/5310 apply to IS-IS PDUs, not ISO 9542 ISHs.
func SignISH(pdu []byte, key Key) ([]byte, error) {
	if key.Algorithm != AuthAlgoCleartext {
		return nil, ErrAuthUnsupported
	}
	ish, err := DecodeISH(pdu)
	if err != nil {
		return nil, ErrAuthMalformed
	}
	defer ReleaseTLVs(ish.TLVs)
	for _, tlv := range ish.TLVs {
		if tlv.Type == TLVISHAuthentication {
			return nil, ErrAuthMalformed
		}
	}
	if len(key.Secret) > MaxTLVValueLen-1 {
		return nil, ErrLength
	}
	length := int(pdu[1])
	end := length + TLVHeaderLen + 1 + len(key.Secret)
	if end > ishLengthMax {
		return nil, ErrLength
	}
	if end > cap(pdu) {
		return nil, ErrShortBuffer
	}
	pdu = pdu[:end]
	// RFC 1195 Annex D.2: "IS-IS Hello and 9542 IS Hello packets shall
	// contain the per-link password".
	pdu[length] = TLVISHAuthentication
	pdu[length+1] = byte(1 + len(key.Secret))
	pdu[length+2] = AuthTypeCleartext
	copy(pdu[length+3:], key.Secret)
	pdu[1] = byte(end)
	hi, lo := Checksum(pdu, ishChecksumOffset)
	pdu[ishChecksumOffset] = hi
	pdu[ishChecksumOffset+1] = lo
	return pdu, nil
}

// VerifyISH checks a received ISH against the configured cleartext receive keys.
// An empty key set means authentication is not configured; the runtime MUST
// distinguish that from a configured key chain with no currently valid keys.
func VerifyISH(pdu []byte, keys []Key) error {
	ish, err := DecodeISH(pdu)
	if err != nil {
		return ErrAuthMalformed
	}
	defer ReleaseTLVs(ish.TLVs)
	if len(keys) == 0 {
		return nil
	}
	authIndex := -1
	for i, tlv := range ish.TLVs {
		if tlv.Type != TLVISHAuthentication {
			continue
		}
		if authIndex >= 0 {
			return ErrAuthMalformed
		}
		authIndex = i
	}
	if authIndex < 0 {
		return ErrAuthMissing
	}
	auth, err := decodeAuthTLV(ish.TLVs[authIndex].Value)
	if err != nil {
		return ErrAuthMalformed
	}
	if auth.AuthType != AuthTypeCleartext {
		return ErrAuthTypeMismatch
	}
	matchedType := false
	for _, key := range keys {
		if key.Algorithm != AuthAlgoCleartext {
			continue
		}
		matchedType = true
		// RFC 1195 Annex D.2: "However, any password contained in the
		// receive password set will be accepted on receipt."
		if hmac.Equal(auth.Value, key.Secret) {
			return nil
		}
	}
	if !matchedType {
		return ErrAuthTypeMismatch
	}
	return ErrAuthMismatch
}
