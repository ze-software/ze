// Design: docs/architecture/wire/l2tp.md — L2TP tunnel authentication (CHAP-MD5)
// RFC: rfc/short/rfc2661.md — RFC 2661 Section 4.2 (Challenge/Response)
// Related: avp.go — MsgSCCRP/MsgSCCCN constants drive ChapID{SCCRP,SCCCN}

package l2tp

import (
	"crypto/md5" //nolint:gosec // RFC 2661 Section 4.2 prescribes MD5 for tunnel authentication; no wire-compatible alternative exists.
	"crypto/subtle"
)

// ChapIDSCCRP is the Message Type byte prepended to the CHAP-MD5 input when
// computing or verifying the response carried in SCCRP (RFC 2661 Section 4.2).
const ChapIDSCCRP byte = byte(MsgSCCRP)

// ChapIDSCCCN is the Message Type byte prepended to the CHAP-MD5 input when
// computing or verifying the response carried in SCCCN.
const ChapIDSCCCN byte = byte(MsgSCCCN)

// ChallengeResponse computes the 16-byte L2TP tunnel authentication response.
//
//	response = MD5(chapID || secret || challenge)
//
// chapID is the Message Type byte of the response-bearing message (2 for
// SCCRP, 3 for SCCCN). Different chapID values prevent cross-direction replay
// even with the same secret and challenge.
//
// The concatenated input is built in a single stack-sized buffer for short
// inputs; longer inputs fall through to a heap allocation (once per tunnel
// setup, not hot-path).
//
// Precondition: len(secret) > 0 and len(challenge) > 0. RFC 2661 Section
// 5.12 requires a Challenge AVP of at least one byte, and a zero-length
// secret reduces the response to MD5(chapID) which is trivially forgeable.
// The subsystem state machine enforces both conditions before calling this;
// the wire helper panics on violation as a programmer-error guard.
func ChallengeResponse(chapID byte, secret, challenge []byte) [16]byte {
	if len(secret) == 0 || len(challenge) == 0 {
		panic("BUG: l2tp ChallengeResponse requires non-empty secret and challenge per RFC 2661")
	}
	total := 1 + len(secret) + len(challenge)
	var stack [128]byte
	var in []byte
	if total <= len(stack) {
		in = stack[:total]
	} else {
		in = make([]byte, total)
	}
	in[0] = chapID
	copy(in[1:], secret)
	copy(in[1+len(secret):], challenge)
	return md5.Sum(in) //nolint:gosec // RFC 2661 Section 4.2 prescribes MD5 for tunnel authentication.
}

// VerifyChallengeResponse returns true iff got equals the expected response
// for the given chapID/secret/challenge. Uses constant-time comparison to
// avoid timing side channels.
func VerifyChallengeResponse(chapID byte, secret, challenge, got []byte) bool {
	if len(got) != 16 {
		return false
	}
	want := ChallengeResponse(chapID, secret, challenge)
	return subtle.ConstantTimeCompare(want[:], got) == 1
}
