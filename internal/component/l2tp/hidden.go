// Design: docs/architecture/wire/l2tp.md — L2TP hidden AVP cipher
// RFC: rfc/short/rfc2661.md — RFC 2661 Section 4.3 (Hiding of AVP Attribute Values)
// Related: avp.go — AVPs with FlagHidden carry ciphertext decrypted here
// Related: errors.go — ErrHiddenLenMismatch, ErrInvalidAVPLen

package l2tp

import (
	"crypto/md5" //nolint:gosec // RFC 2661 Section 4.3 prescribes MD5 for the hidden-AVP stream cipher.
	"encoding/binary"
)

const (
	hiddenBlockSize = 16 // MD5 output block size
	hiddenLenField  = 2  // Original Length field prepended to plaintext
)

// HiddenEncrypt encrypts a hidden AVP value under RFC 2661 Section 4.3 and
// writes the ciphertext into dst. It returns the populated slice of dst.
//
// Parameters:
//   - dst: caller-provided scratch, MUST have room for hiddenLenField + len(plaintext).
//   - attrType: the AVP Attribute Type (used in the first MD5 block's input).
//   - secret: tunnel shared secret.
//   - rv: the Random Vector from the preceding Random Vector AVP.
//   - plaintext: the original (cleartext) AVP value.
//
// No padding is added. Callers that want random padding should enlarge
// plaintext themselves before calling.
//
// Precondition: len(secret) > 0 and len(rv) > 0. An empty secret reduces the
// first block key to MD5(attrType) (trivially decryptable by any peer that
// knows the attribute type). An empty Random Vector makes the keystream
// identical for every hidden AVP of the same type across messages, which
// leaks plaintext under known-plaintext attack. Both are enforced by the
// subsystem config / state machine; the wire helper panics on violation as
// a programmer-error guard.
func HiddenEncrypt(dst []byte, attrType AVPType, secret, rv, plaintext []byte) []byte {
	if len(secret) == 0 || len(rv) == 0 {
		panic("BUG: l2tp HiddenEncrypt requires non-empty secret and random vector per RFC 2661")
	}
	sublen := hiddenLenField + len(plaintext)
	out := dst[:sublen]
	// Subformat: 2-byte Original Length || plaintext.
	binary.BigEndian.PutUint16(out[:2], uint16(len(plaintext)))
	copy(out[hiddenLenField:], plaintext)

	var attrBytes [2]byte
	binary.BigEndian.PutUint16(attrBytes[:], uint16(attrType))
	key := firstBlockKey(attrBytes[:], secret, rv)

	var prevCipher [hiddenBlockSize]byte
	for i := 0; i < sublen; i += hiddenBlockSize {
		end := min(i+hiddenBlockSize, sublen)
		// XOR block: plaintext XOR key → ciphertext (in-place in out).
		for j := range end - i {
			out[i+j] ^= key[j]
			prevCipher[j] = out[i+j]
		}
		if end < sublen {
			// Next block key: MD5(secret || prev_ciphertext_block).
			// RFC: "b(i) = MD5(S || c(i-1))" for i >= 2.
			key = chainBlockKey(secret, prevCipher[:end-i])
		}
	}
	return out
}

// HiddenDecrypt reverses HiddenEncrypt. Writes the decrypted plaintext
// (subformat) into dst, extracts the original value by Original Length, and
// returns the value subslice.
//
// Returns ErrInvalidAVPLen if ciphertext is shorter than 2 bytes; returns
// ErrHiddenLenMismatch if the recovered Original Length exceeds the
// ciphertext payload.
//
// Precondition: len(secret) > 0 and len(rv) > 0 (same rationale as
// HiddenEncrypt). Panics on violation.
func HiddenDecrypt(dst []byte, attrType AVPType, secret, rv, ciphertext []byte) ([]byte, error) {
	if len(secret) == 0 || len(rv) == 0 {
		panic("BUG: l2tp HiddenDecrypt requires non-empty secret and random vector per RFC 2661")
	}
	if len(ciphertext) < hiddenLenField {
		return nil, ErrInvalidAVPLen
	}
	out := dst[:len(ciphertext)]
	copy(out, ciphertext)

	var attrBytes [2]byte
	binary.BigEndian.PutUint16(attrBytes[:], uint16(attrType))
	key := firstBlockKey(attrBytes[:], secret, rv)

	for i := 0; i < len(ciphertext); i += hiddenBlockSize {
		end := min(i+hiddenBlockSize, len(ciphertext))
		// For decrypt, the ciphertext that the next chain block consumes is
		// the ORIGINAL ciphertext, not the plaintext we just produced.
		if end < len(ciphertext) {
			nextKey := chainBlockKey(secret, ciphertext[i:end])
			// XOR current block.
			for j := range end - i {
				out[i+j] ^= key[j]
			}
			key = nextKey
		} else {
			for j := range end - i {
				out[i+j] ^= key[j]
			}
		}
	}

	origLen := int(binary.BigEndian.Uint16(out[:2]))
	if origLen > len(ciphertext)-hiddenLenField {
		return nil, ErrHiddenLenMismatch
	}
	return out[hiddenLenField : hiddenLenField+origLen], nil
}

// firstBlockKey returns MD5(attrType || secret || rv). The concat is built
// in a stack-sized buffer for the common case; large secrets/RVs fall back
// to the heap (occurs only during tunnel setup, not hot-path).
func firstBlockKey(attrBytes, secret, rv []byte) [hiddenBlockSize]byte {
	total := len(attrBytes) + len(secret) + len(rv)
	var stack [256]byte
	var in []byte
	if total <= len(stack) {
		in = stack[:total]
	} else {
		in = make([]byte, total)
	}
	n := copy(in, attrBytes)
	n += copy(in[n:], secret)
	copy(in[n:], rv)
	return md5.Sum(in) //nolint:gosec // RFC 2661 Section 4.3 fixed algorithm.
}

// chainBlockKey returns MD5(secret || prevCipher).
func chainBlockKey(secret, prevCipher []byte) [hiddenBlockSize]byte {
	total := len(secret) + len(prevCipher)
	var stack [256]byte
	var in []byte
	if total <= len(stack) {
		in = stack[:total]
	} else {
		in = make([]byte, total)
	}
	n := copy(in, secret)
	copy(in[n:], prevCipher)
	return md5.Sum(in) //nolint:gosec // RFC 2661 Section 4.3 fixed algorithm.
}
