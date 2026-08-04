// RFC: rfc/short/rfc7606.md -- Section 3.g duplicate MP attributes, Section 4 structural errors
// Overview: session_validation.go -- enforceRFC7606, the entry point this drives
//
// One test, two defects, both reachable only when the validator's attribute walk is
// abandoned before its end. Kept apart from session_validation_nlritype_bypass_test.go
// because the requirement under test is Section 3.g, not Section 5.4: Section 5.4 is what
// the skipped verdict went on to corrupt, not what it violated.

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// TestRFC7606Section3gDuplicateMPBeatsAnAbandonedWalk proves the duplicate verdict is
// reached before anything can outrun it.
//
// VALIDATES: RFC 7606 Section 3.g -- "If the MP_REACH_NLRI attribute or the MP_UNREACH_NLRI
// attribute appears more than once in the UPDATE message, then a NOTIFICATION message MUST
// be sent with the Error Subcode 'Malformed Attribute List'" -- reported when the duplicate
// is SEEN, rather than after a loop a later framing error can abandon.
//
// PREVENTS: two defects at once.
//
// The MUST itself was skippable. `ValidateUpdateRFC7606AddPath` (message/rfc7606.go) counted
// MP attributes in its walk and judged the count afterwards, so appending one attribute
// whose declared length overruns the section returned treat-as-withdraw from inside the loop
// and the duplicate was never reported. The peer kept a session RFC 7606 says must be reset.
//
// And the skipped verdict corrupted Section 5.4. `mpReachNLRI` holds the LAST MP_REACH the
// walk saw, while `attribute.AttrFind` returns the FIRST. With the duplicate unreported,
// `Session.typedNLRIEdit` (session_validation_nlritype.go) received a location describing one
// attribute and bytes belonging to another: it picked the recognizer for the recorded
// family and applied it to a different family's NLRI, then rewrote the wire from that
// misread. The fixture below is that exact shape -- the recorded family is ipv4/unicast,
// which has no Section 5.4 ruling, so the EVPN attribute carrying an unimplemented route
// type was never judged at all and rode out inside the synthesized withdrawal.
//
// Deciding the duplicate inside the loop removes the class rather than the instance: no exit
// added to that loop later can outrun a verdict already returned.
//
// RFC requirement: RFC7606-3.g-1 positive -- a second MP_REACH_NLRI is a session reset even when a later attribute's framing abandons the Section 4 walk before the end of the attribute section.
func TestRFC7606Section3gDuplicateMPBeatsAnAbandonedWalk(t *testing.T) {
	registerEVPNRecognizer(t)
	s := nlriTypeTestSession()

	// MP_REACH #1: l2vpn/evpn, one implemented route type and one ze does not implement.
	nlri := append(evpnWireNLRI(2, 0xaa), evpnWireNLRI(99, 0xbb)...)
	attrs := mpReachAttrs(evpnFam, nlri)

	// MP_REACH #2: ipv4/unicast, 10.0.0.0/24. Its family has no Section 5.4 ruling, which is
	// what made this shape dangerous rather than merely wrong.
	v2 := []byte{0x00, 0x01, 0x01, 0x04, 0xc0, 0x00, 0x02, 0x01, 0x00, 0x18, 0x0a, 0x00, 0x00}
	attrs = append(attrs, 0x80, 0x0e, byte(len(v2)))
	attrs = append(attrs, v2...)

	// RFC 7606 Section 4: an attribute declaring 64 octets of value with none following.
	// This is what abandoned the walk before the post-loop duplicate check.
	attrs = append(attrs, 0x40, 0x02, 0x40)

	body := makeUpdateBody(nil, attrs, nil)
	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.Error(t, err,
		"RFC 7606 Section 3.g requires a NOTIFICATION, which enforceRFC7606 reports as an error")
	assert.Equal(t, message.RFC7606ActionSessionReset, action,
		"a duplicate MP_REACH resets the session whether or not the walk ran to the end")
}

// TestRFC7606Section3gDuplicateMPUnreachBeatsAnAbandonedWalk is the MP_UNREACH half. Both
// counters feed one verdict, and a fix that moved only the MP_REACH branch would leave the
// same hole open on the withdrawal side, where Section 5.4 also applies.
//
// RFC requirement: RFC7606-3.g-1 positive -- a second MP_UNREACH_NLRI is a session reset on the same terms as a second MP_REACH_NLRI.
func TestRFC7606Section3gDuplicateMPUnreachBeatsAnAbandonedWalk(t *testing.T) {
	s := nlriTypeTestSession()

	// Two MP_UNREACH attributes, ipv4/unicast, withdrawing 10.0.0.0/24.
	unreach := []byte{0x00, 0x01, 0x01, 0x18, 0x0a, 0x00, 0x00}
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
	}
	for range 2 {
		attrs = append(attrs, 0x80, 0x0f, byte(len(unreach)))
		attrs = append(attrs, unreach...)
	}
	attrs = append(attrs, 0x40, 0x02, 0x40) // Section 4 framing error

	body := makeUpdateBody(nil, attrs, nil)
	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.Error(t, err)
	assert.Equal(t, message.RFC7606ActionSessionReset, action,
		"a duplicate MP_UNREACH resets the session whether or not the walk ran to the end")
}
