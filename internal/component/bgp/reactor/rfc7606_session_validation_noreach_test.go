// RFC: rfc/short/rfc7606.md -- Section 5.2 escalation, Section 4 structural errors
// Overview: session_validation.go -- enforceRFC7606, the entry point this drives
//
// The third obligation an abandoned attribute walk still owes. Section 3.g is in
// session_validation_dupmp_test.go and Section 5.4 in
// session_validation_nlritype_bypass_test.go; all three were skipped by the same four
// returns, and all three are now paid by one helper.

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// TestRFC7606Section52EscalatesWhenTheAttributeWalkIsAbandoned proves the escalation is not
// skippable by breaking the framing.
//
// VALIDATES: RFC 7606 Section 5.2 -- "An UPDATE message with only path attributes and no
// associated NLRI ... if any path attribute fails the checks ... and the error action is not
// 'attribute discard' ... the session-reset action MUST be used."
//
// PREVENTS: silence where the RFC requires a NOTIFICATION. The escalation was judged after
// the attribute walk, and the walk's four RFC 7606 Section 4 structural returns are inside
// it, so an UPDATE with attributes, no NLRI and a truncated attribute header returned
// treat-as-withdraw. For a body carrying no reachable NLRI,
// message.SynthesizeWithdrawFamilies produces no bodies at all, so ze consumed the UPDATE
// and told the peer nothing. The peer keeps a session RFC 7606 says must be reset, and the
// attribute error that caused it is never signaled.
//
// The fixture carries no NLRI, no withdrawn routes and no MP_REACH, which is exactly the
// Section 5.2 shape. Its ORIGIN is well formed: the only error is the framing one, so the
// escalation cannot be attributed to anything the completed walk would have caught.
//
// RFC requirement: RFC7606-5.2-1 positive -- an UPDATE with attributes and no reachable NLRI is session-reset when a Section 4 framing error abandons the walk, not treated as a withdrawal that withdraws nothing.
func TestRFC7606Section52EscalatesWhenTheAttributeWalkIsAbandoned(t *testing.T) {
	s := nlriTypeTestSession()

	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP, well formed
		0x40, 0x02, 0x00, // AS_PATH (empty), well formed
	}
	// RFC 7606 Section 4: an attribute declaring 64 octets of value with none following.
	attrs = append(attrs, 0x40, 0x02, 0x40)

	body := makeUpdateBody(nil, attrs, nil)
	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.Error(t, err,
		"RFC 7606 Section 5.2 requires session reset, which enforceRFC7606 reports as an error")
	assert.Equal(t, message.RFC7606ActionSessionReset, action,
		"attributes with no reachable NLRI escalate even when the walk never reached its end")
}

// TestRFC7606Section52LeavesAnUpdateWithNLRIAlone is the other side of the same branch, so
// the test above cannot pass by escalating everything.
//
// VALIDATES: the Section 5.2 escalation is conditioned on "no associated NLRI". An UPDATE
// that DOES carry reachable NLRI keeps treat-as-withdraw, which is what lets the routes it
// announced be withdrawn rather than the session torn down.
//
// PREVENTS: a fix that reads "structural error" as "session reset" and turns every truncated
// attribute into a teardown, which would hand any peer a one-octet way to drop the session.
//
// RFC requirement: RFC7606-5.2-1 negative -- an UPDATE carrying reachable NLRI is NOT escalated to session reset by the same framing error.
func TestRFC7606Section52LeavesAnUpdateWithNLRIAlone(t *testing.T) {
	s := nlriTypeTestSession()

	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0x01, 0x01, 0x01, 0x01, // NEXT_HOP 1.1.1.1
	}
	attrs = append(attrs, 0x40, 0x02, 0x40) // the same Section 4 framing error

	nlri := []byte{0x18, 0x0a, 0x00, 0x00} // 10.0.0.0/24
	body := makeUpdateBody(nil, attrs, nlri)
	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionTreatAsWithdraw, action,
		"reachable NLRI means the routes can be withdrawn, so Section 5.2 does not escalate")
}
