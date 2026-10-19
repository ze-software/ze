// Design: docs/architecture/wire/attributes.md — AS_PATH segment types
// RFC: rfc/short/rfc9774.md — Deprecation of AS_SET and AS_CONFED_SET in BGP
// Related: rfc7606.go — validateASPath, the AS_PATH caller of the set scan
// Related: rfc7607.go — validateAS4PathAttr, the AS4_PATH caller of the set scan
//
// RFC 9774 deprecates two of the four AS_PATH segment types. The scan lives in its own
// file so the AS_PATH and AS4_PATH validators share one answer and cannot disagree.

package message

import "github.com/ze-software/ze/internal/core/textbuf"

// asPathSetVerdict builds the treat-as-withdraw verdict for a set segment found in the
// AS_PATH-shaped attribute code. attrName is the attribute name the operator reads in the
// log line ("AS_PATH" or "AS4_PATH").
//
// RFC 9774 Section 3: "MUST use the "treat-as-withdraw" error handling behavior per
// [RFC7606] upon reception of BGP UPDATE messages containing AS_SETs or AS_CONFED_SETs
// in the AS_PATH or AS4_PATH [RFC6793]." The verdict carries that procedure.
func asPathSetVerdict(code uint8, attrName string, segType uint8) *RFC7606ValidationResult {
	segName := "AS_SET"
	if segType == asPathTypeConfedSet {
		segName = "AS_CONFED_SET"
	}
	var b textbuf.Buffer
	return &RFC7606ValidationResult{
		Action:      RFC7606ActionTreatAsWithdraw,
		AttrCode:    code,
		Description: b.Reset().Str("RFC 9774 Section 3: ").Str(segName).Str(" in ").Str(attrName).String(),
	}
}

// ASPathSetSegment reports the type of the first AS_SET (1) or AS_CONFED_SET (4)
// segment in AS_PATH-shaped segment data, and whether one was found. asnOctets is 2
// for a two-octet encoding and 4 for a four-octet one.
//
// RFC 9774 Section 3: "BGP speakers: MUST NOT advertise BGP UPDATE messages containing
// AS_SETs or AS_CONFED_SETs and MUST use the "treat-as-withdraw" error handling behavior
// per [RFC7606] upon reception of BGP UPDATE messages containing AS_SETs or
// AS_CONFED_SETs in the AS_PATH or AS4_PATH [RFC6793]." Both halves ask this question.
//
// The scan judges a fully framed path only: every segment carries a known type, a
// nonzero count and its AS numbers, and the last segment ends exactly at len(data).
// Any structural fault answers found false, because a misframed path is a different
// malformation with a different owner (RFC 7606 Section 7.2 for AS_PATH, RFC 6793
// Section 6 for AS4_PATH), and a byte read past a fault could be a length mistaken
// for a type. The walk is bounded by len(data), and pos rises by at least two each
// iteration, so hostile input cannot make it run forever.
//
// Exported for the reactor's send guard (reactor/session_rfc9774.go), which asks the
// same question of a transmitted path so the two halves of Section 3 cannot disagree.
func ASPathSetSegment(data []byte, asnOctets int) (segType uint8, found bool) {
	pos := 0
	for pos < len(data) {
		if pos+2 > len(data) {
			return 0, false
		}
		current := data[pos]
		count := int(data[pos+1])
		if current < asPathTypeASSet {
			return 0, false
		}
		if current > asPathTypeConfedSet {
			return 0, false
		}
		if count == 0 {
			return 0, false
		}
		pos += 2 + count*asnOctets
		if pos > len(data) {
			return 0, false
		}
		if found {
			continue
		}
		if current == asPathTypeASSet {
			segType, found = current, true
			continue
		}
		if current == asPathTypeConfedSet {
			segType, found = current, true
		}
	}
	return segType, found
}
