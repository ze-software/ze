// Design: reserved RTR PDU fields are zero because the WRITER zeroes them, and
// only a dirty output buffer can tell that apart from a buffer that was already
// zero.
// Related: rtr_pdu_test.go -- the rest of the RTR PDU wire-format tests.
// Related: rtr_pdu.go -- writeResetQuery, the producer under test.
package rpki

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
)

// dirtyBuf returns a buffer whose every octet is 0xFF.
//
// Every RTR writer is handed a buffer it did not allocate, so "the field is
// zero" and "the writer zeroed the field" are two different facts. On a
// make([]byte, n) they are indistinguishable, and only the second one is what an
// RFC means by "MUST be zero on transmission". Starting from 0xFF makes the
// weaker fact impossible to reach by accident.
func dirtyBuf(size int) []byte {
	buf := make([]byte, size)
	for i := range buf {
		buf[i] = 0xFF
	}
	return buf
}

// TestWriteResetQueryZeroesReservedOverGarbage proves writeResetQuery ZEROES the
// two header octets that carry no Session ID in a Reset Query, instead of
// leaving whatever the caller's buffer held there.
//
// RFC 6810 Section 5: "Fields with unspecified content MUST be zero on
// transmission and MAY be ignored on receipt."
// RFC 8210 Section 5: "Reserved fields (marked "zero" in PDU diagrams) MUST be
// zero on transmission and MUST be ignored on receipt."
//
// VALIDATES: octets 2-3 of a Reset Query leave the writer as zero even when the
// output buffer arrives full of 0xFF, and the specified fields around them still
// carry their real values.
// PREVENTS: a Reset Query that ships stack or pool residue in its reserved
// field. RFC 8210 makes the receive side "MUST be ignored", but RFC 6810 makes
// it only "MAY be ignored", so a v0 cache is free to reject the PDU.
//
// TestWriteResetQuery asserts the same two octets on a make([]byte, 16), where
// they were already zero before the call. Deleting both `buf[off+2] = 0` and
// `buf[off+3] = 0` from writeResetQuery leaves that test green; it turns this one
// red (mutation-tested, 2026-08-30).
//
// RFC requirement: RFC6810-5-1 positive -- the Session ID field is unspecified
// content in a Reset Query, and writeResetQuery emits it as zero on transmission
// over a buffer that held 0xFF, so the zero is written rather than inherited.
// RFC requirement: RFC8210-5-1 positive -- v1 keeps the rule for reserved fields
// marked zero in the PDU diagrams, and the same dirty-buffer write satisfies it.
func TestWriteResetQueryZeroesReservedOverGarbage(t *testing.T) {
	buf := dirtyBuf(16)

	n := writeResetQuery(buf, 0, rtrVersionMax)

	assert.Equal(t, pduResetQueryLen, n)
	assert.Equal(t, uint16(0), binary.BigEndian.Uint16(buf[2:4]),
		"reserved octets 2-3 must be zeroed by the writer, not inherited from the buffer")

	// The specified fields are still written, so the zeroing is targeted at the
	// reserved octets rather than a blanket wipe of the caller's buffer.
	assert.Equal(t, rtrVersionMax, buf[0])
	assert.Equal(t, pduResetQuery, buf[1])
	assert.Equal(t, uint32(pduResetQueryLen), binary.BigEndian.Uint32(buf[4:8]))

	// The obligation stops at the PDU. Octets past it belong to the caller.
	assert.Equal(t, byte(0xFF), buf[pduResetQueryLen],
		"the writer must not touch octets past the 8-octet PDU")
}
