// Design: docs/architecture/wire/l2tp.md — L2TP hidden AVP cipher
// RFC: rfc/short/rfc2661.md — RFC 2661 Section 4.3 (Hiding of AVP Attribute Values)
// Related: avp.go — AVPs with FlagHidden carry ciphertext decrypted here; skipHiddenAVP
// Related: errors.go — ErrHiddenLenMismatch, ErrInvalidAVPLen
// Related: tunnel_fsm.go — handleMessage reveals each delivered message before dispatch
// Related: reactor.go — the SCCRQ pre-parse reveals before parseSCCRQ
// Related: register_hidden_diagnostic.go — the doctor code and report-bus event of an AVP left hidden

package l2tp

import (
	"crypto/md5" //nolint:gosec // RFC 2661 Section 4.3 prescribes MD5 for the hidden-AVP stream cipher.
	"encoding/binary"
	"errors"
	"log/slog"
	"net/netip"
)

const (
	hiddenBlockSize = 16 // MD5 output block size
	hiddenLenField  = 2  // Original Length field prepended to plaintext
)

// The reasons revealHiddenAVP leaves a hidden AVP hidden, beside the two
// HiddenDecrypt returns for a malformed hidden value (ErrInvalidAVPLen,
// ErrHiddenLenMismatch).
var (
	errHiddenNoSecret       = errors.New("l2tp: hidden AVP not revealed: no shared secret is configured for this tunnel")
	errHiddenNoRandomVector = errors.New("l2tp: hidden AVP not revealed: no non-empty Random Vector AVP precedes it")
	errHiddenRandomVector   = errors.New("l2tp: hidden AVP not revealed: it is a Random Vector AVP, which is never hidden")
)

// revealHiddenAVPs returns body with every hidden AVP it can reveal rewritten
// as the cleartext AVP the sender hid: the H bit clear, the M bit kept, the
// Length 6 plus the Original Length, and the original value, padding
// dropped. Every other AVP is copied unchanged, so an AVP it cannot reveal
// reaches the parser still hidden, and skipHiddenAVP applies the M-bit rule
// to it. Those AVPs are counted in the returned hiddenLeft, which the caller
// logs once for the message (hiddenLeft.log): the caller decides, because the
// SCCRQ path takes unauthenticated datagrams and rate-limits the line. An AVP
// the iterator marks unrecognized is copied unchanged and not counted: the
// parser refuses or skips it by its M bit, hidden or not.
//
// body is returned as is when it holds no hidden AVP, so a message without
// one costs one header walk and no copy. Otherwise the result is a new slice
// the caller owns. It cannot come from the pool, because parsers keep
// subslices of it (a Challenge, a proxy LCP body) after they return, and the
// body itself is never written, because the reorder queue may still hold it.
// From the first malformed AVP on, the stream is copied as it came, so the
// parser meets the same malformation it would have met unrevealed.
//
// secret is the tunnel's shared secret, empty when none is configured.
//
// RFC 2661 Section 4.3: "On receipt, the random vector is taken from the last
// Random Vector AVP encountered in the message prior to the AVP to be
// unhidden.  The above process is then reversed to yield the original value."
func revealHiddenAVPs(body []byte, secret string) (revealedBody []byte, left hiddenLeft) {
	if !carriesHiddenAVP(body) {
		return body, left
	}
	revealed := make([]byte, len(body))
	written := 0
	var rv []byte
	iter := NewAVPIterator(body)
	// Bounded by len(body): each Next consumes at least AVPHeaderLen octets.
	for {
		start := len(body) - iter.Remaining()
		vendorID, attrType, flags, value, ok := iter.Next()
		if !ok {
			// Nothing remains on clean exhaustion; after a malformed AVP the
			// rest goes to the parser as it came.
			written += copy(revealed[written:], body[start:])
			return revealed[:written], left
		}
		raw := body[start : len(body)-iter.Remaining()]
		if flags&FlagUnrecognized != 0 {
			written += copy(revealed[written:], raw)
			continue
		}
		if flags&FlagHidden == 0 {
			// RFC 2661 Section 4.4.1: "More than one Random Vector AVP may
			// appear in a message, in which case a hidden AVP uses the Random
			// Vector AVP most closely preceding it."
			if isRandomVectorAVP(vendorID, attrType) {
				rv = value
			}
			written += copy(revealed[written:], raw)
			continue
		}
		n, err := revealHiddenAVP(revealed[written:], vendorID, attrType, flags, value, secret, rv)
		if err == nil {
			written += n
			continue
		}
		left.note(vendorID, attrType, flags, err)
		written += copy(revealed[written:], raw)
	}
}

// hiddenLeft summarizes the AVPs revealHiddenAVPs left hidden in one message:
// how many, and the first one with its reason. The zero value means none.
type hiddenLeft struct {
	count     int
	vendorID  uint16
	attrType  AVPType
	mandatory bool
	err       error
}

// note records one AVP left hidden; the first one is the one reported.
func (h *hiddenLeft) note(vendorID uint16, attrType AVPType, flags AVPFlags, err error) {
	h.count++
	if h.count > 1 {
		return
	}
	h.vendorID = vendorID
	h.attrType = attrType
	h.mandatory = flags&FlagMandatory != 0
	h.err = err
}

// log writes the one warning line a message with AVPs left hidden earns, the
// local error message RFC 2661 Section 7.1 asks for. One line per message,
// never one per AVP, so a message packed with hidden AVPs costs one line.
// It writes nothing when no AVP was left hidden. peer is the source of the
// message, so the line and the event name the tunnel endpoint to correct; the
// event is raised on the report bus for ze show doctor (reportHiddenLeft).
func (h hiddenLeft) log(logger *slog.Logger, peer netip.AddrPort) {
	if h.count == 0 {
		return
	}
	fix := hiddenRevealFix(h.err)
	logger.Warn("l2tp: hidden AVP left hidden; a mandatory one refuses the message, an optional one is ignored",
		"code", codeL2TPHiddenAVPUnrevealed, "peer", peer.String(),
		"count", h.count, "vendor", h.vendorID, "type", uint16(h.attrType), "mandatory", h.mandatory,
		"error", h.err.Error(), "fix", fix)
	reportHiddenLeft(peer, h.err, fix)
}

// carriesHiddenAVP reports whether a well-formed prefix of body holds an AVP
// with the H bit set. A malformed stream before any hidden AVP answers false,
// and its parser refuses the malformation itself.
func carriesHiddenAVP(body []byte) bool {
	iter := NewAVPIterator(body)
	// Bounded by len(body): each Next consumes at least AVPHeaderLen octets.
	for {
		_, _, flags, _, ok := iter.Next()
		if !ok {
			return false
		}
		if flags&FlagHidden != 0 {
			return true
		}
	}
}

// isRandomVectorAVP reports whether the AVP is the IETF Random Vector AVP.
func isRandomVectorAVP(vendorID uint16, attrType AVPType) bool {
	if vendorID != 0 {
		return false
	}
	return attrType == AVPRandomVector
}

// revealHiddenAVP writes the cleartext form of one hidden AVP at dst[0] and
// returns its length, or an error naming why it cannot be revealed. dst MUST
// have room for AVPHeaderLen + len(ciphertext) octets, which holds whenever
// dst starts at or before the hidden AVP's own offset in a buffer as long as
// its message.
func revealHiddenAVP(dst []byte, vendorID uint16, attrType AVPType, flags AVPFlags, ciphertext []byte, secret string, rv []byte) (int, error) {
	// RFC 2661 Section 4.4.1, Random Vector: "This AVP MUST NOT be hidden
	// (the H-bit MUST be 0)." A hidden vector is no vector, and its value
	// could only be revealed with itself.
	if isRandomVectorAVP(vendorID, attrType) {
		return 0, errHiddenRandomVector
	}
	// RFC 2661 Section 4.3: "The H bit MUST only be set if a shared secret
	// exists between the LAC and LNS."
	if secret == "" {
		return 0, errHiddenNoSecret
	}
	// RFC 2661 Section 4.3: "If the H bit is set in any AVP(s) in a given
	// control message, a Random Vector AVP must also be present in the
	// message and MUST precede the first AVP having an H bit of 1."
	if len(rv) == 0 {
		return 0, errHiddenNoRandomVector
	}
	plain, err := HiddenDecrypt(dst[AVPHeaderLen:], attrType, []byte(secret), rv, ciphertext)
	if err != nil {
		return 0, err
	}
	// HiddenDecrypt leaves the value after the 2-octet Original Length; move
	// it up over that field, where the cleartext AVP's value starts.
	copy(dst[AVPHeaderLen:], plain)
	total := AVPHeaderLen + len(plain)
	WriteAVPHeader(dst, 0, flags&FlagMandatory, vendorID, attrType, total)
	return total, nil
}

// hiddenRevealFix names what the operator does about an AVP left hidden.
func hiddenRevealFix(err error) string {
	if errors.Is(err, errHiddenNoSecret) {
		return "configure the secret this peer hides AVPs under as l2tp shared-secret (or the remote's shared-secret for a dialed tunnel)"
	}
	if errors.Is(err, ErrHiddenLenMismatch) {
		return "check that the configured shared-secret matches the peer's tunnel secret"
	}
	return "the peer hid this AVP against RFC 2661 Section 4.3; correct the peer's L2TP software or configuration"
}

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
