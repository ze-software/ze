// RFC: rfc/short/rfc7606.md -- Section 3.g duplicate MP attributes
// Overview: session_validation.go -- enforceRFC7606, the entry point this drives
//
// The withdrawal half of Section 3.g, framed so that Section 5.2 cannot answer for it.
//
// It lives in its own file rather than beside its MP_REACH twin because the twin's file is
// RFC-tagged and the repository refuses edits to a tagged test without the owner's approval
// (.claude/hooks/pretool-writeedit.py). That guard is right, and the cost of respecting it
// here is one extra file. See the note in the test's own comment.

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// TestRFC7606Section3gDuplicateMPUnreachResetsWithRoutesPresent proves the duplicate
// MP_UNREACH verdict on its own, with no other rule able to produce the same answer.
//
// VALIDATES: RFC 7606 Section 3.g -- "If the MP_REACH_NLRI attribute or the MP_UNREACH_NLRI
// attribute appears more than once in the UPDATE message, then a NOTIFICATION message MUST
// be sent with the Error Subcode 'Malformed Attribute List'" -- for MP_UNREACH, reached
// before a Section 4 framing error can abandon the walk.
//
// PREVENTS: a fix that moved only the MP_REACH branch. Both counters feed one verdict, and
// Section 5.4 applies to withdrawals on the same terms as announcements, so the withdrawal
// side needs its own proof.
//
// WHY THE FIXTURE CARRIES A ROUTE. `TestRFC7606Section3gDuplicateMPUnreachBeatsAnAbandonedWalk`
// (session_validation_dupmp_test.go) asserts the same outcome from a body with no NLRI and
// no MP_REACH. That is ALSO the RFC 7606 Section 5.2 shape, so `structuralError`
// (message/rfc7606.go) escalates to session reset there whether or not the duplicate was
// ever judged: it stays green with the Section 3.g fix reverted and therefore proves
// nothing. The independent RFC audit measured that and recorded it in rfc/audit/rfc7606.json.
// Reachable NLRI removes the Section 5.2 escape, so Section 3.g is the only rule left that
// can produce this verdict, and reverting the in-loop check turns this test red.
//
// The twin's fixture WAS corrected on 2026-08-04 under Thomas's standing authorisation, so
// both now carry a route and both now redden when the in-loop check is reverted (measured:
// each fails with the verdict moved back after the loop). This file is therefore redundant
// with its twin and is a fold-back candidate, not a workaround. It is kept rather than
// deleted because removing a tracked test is the owner's call, and two passing proofs of one
// MUST cost less than a deletion nobody asked for.
//
// rfc-test-change-approved: 2026-08-04 -- Thomas standing authorisation for
// correctness-only test edits. POLARITY CORRECTED, positive -> negative. A duplicate MP
// attribute is the violation, so rejecting it is a negative (ai/skills/ze-rfc.md). No
// assertion changes.
//
// RFC requirement: RFC7606-3.g-1 negative -- a second MP_UNREACH_NLRI is a session reset even when a later attribute's framing abandons the Section 4 walk, in an UPDATE that carries reachable NLRI so no other rule can reach the same verdict.
func TestRFC7606Section3gDuplicateMPUnreachResetsWithRoutesPresent(t *testing.T) {
	s := nlriTypeTestSession()

	// Two MP_UNREACH attributes, ipv4/unicast, withdrawing 10.0.1.0/24.
	unreach := []byte{0x00, 0x01, 0x01, 0x18, 0x0a, 0x00, 0x01}
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0x01, 0x01, 0x01, 0x01, // NEXT_HOP 1.1.1.1, for the announced route
	}
	for range 2 {
		attrs = append(attrs, 0x80, 0x0f, byte(len(unreach)))
		attrs = append(attrs, unreach...)
	}
	// RFC 7606 Section 4: an attribute declaring 64 octets of value with none following.
	attrs = append(attrs, 0x40, 0x02, 0x40)

	nlri := []byte{0x18, 0x0a, 0x00, 0x00} // 10.0.0.0/24 announced, so Section 5.2 cannot fire
	body := makeUpdateBody(nil, attrs, nlri)
	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.Error(t, err,
		"RFC 7606 Section 3.g requires a NOTIFICATION, which enforceRFC7606 reports as an error")
	assert.Equal(t, message.RFC7606ActionSessionReset, action,
		"the duplicate MP_UNREACH is the only rule that can reset this session")
}
