// Design: docs/architecture/wire/messages.md -- RFC 7606 UPDATE validation.
// Related: rfc7606.go -- ValidateUpdateRFC7606AddPath, the Section 5.2 escalation under test.
// Related: ../plugins/nlri/srpolicy/register.go -- the RFC 9830 Section 5 NLRI syntax rule.
//
// The package is message_test rather than message because the SR Policy syntax rule is a
// plugin registration, and a plugin imports the message package. The blank import puts the
// rule in the nlrisyntax registry, as the generated composition root does in a shipped ze.
package message_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/family"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/srpolicy"
)

// mpUnreachSRPolicyAttr builds an MP_UNREACH_NLRI attribute for IPv4 SR Policy carrying
// one NLRI whose length octet says 104 bits followed by 13 octets. The NLRI walks, but 13
// octets is not the 12 RFC 9830 Section 4.2.1 requires under AFI 1, so the registered
// syntax rule answers treat-as-withdraw.
func mpUnreachSRPolicyAttr() []byte {
	value := []byte{0x00, byte(family.AFIIPv4), byte(family.SAFISRPolicy), 104}
	for i := range 13 {
		value = append(value, byte(i+1))
	}
	return append([]byte{0x80, 0x0f, byte(len(value))}, value...)
}

// TestRFC7606Section52MPUnreachOnlyNotEscalated checks which UPDATE shapes the RFC 7606
// Section 5.2 escalation binds, using a walkable malformed SR Policy NLRI in MP_UNREACH_NLRI
// and no reachable NLRI in each case.
//
// VALIDATES: an UPDATE whose only path attribute is MP_UNREACH_NLRI keeps the
// treat-as-withdraw verdict of its malformed NLRI; adding any other path attribute, valid
// or malformed, brings the UPDATE under Section 5.2 and the verdict becomes session reset.
// PREVENTS: the escalation firing on "doesn't encode any reachable NLRI" alone, which reset
// the session over a withdrawal the RFC lets ze skip.
//
// RFC 7606 Section 5.2: "Thus, if an UPDATE message is encountered that does contain path
// attributes other than MP_UNREACH_NLRI and doesn't encode any reachable NLRI, we cannot be
// confident that the NLRI have been successfully parsed as Section 3 (j) requires."
//
// RFC requirement: RFC7606-5.2-1 positive -- an UPDATE whose only path attribute is MP_UNREACH_NLRI, with a walkable malformed SR Policy NLRI and no reachable NLRI, is treat-as-withdraw and not escalated to session reset
// RFC requirement: RFC7606-5.2-1 negative -- the same MP_UNREACH_NLRI beside a malformed ORIGIN, or beside a valid ORIGIN, contains path attributes other than MP_UNREACH_NLRI, so the error is escalated to session reset
// RFC requirement: RFC9830-5-2 positive -- a walkable malformed SR Policy NLRI in an UPDATE whose only path attribute is MP_UNREACH_NLRI is handled as treat-as-withdraw, not session reset.
func TestRFC7606Section52MPUnreachOnlyNotEscalated(t *testing.T) {
	unreach := mpUnreachSRPolicyAttr()

	got := message.ValidateUpdateRFC7606(unreach, false, true, true)
	require.Equal(t, message.RFC7606ActionTreatAsWithdraw, got.Action, got.Description)
	require.True(t, got.MPUnreachNLRI.Present, "the withdrawal must stay locatable for typedNLRIEdit")

	malformedOrigin := append([]byte{0x40, 0x01, 0x02, 0x00, 0x00}, unreach...)
	got = message.ValidateUpdateRFC7606(malformedOrigin, false, true, true)
	require.Equal(t, message.RFC7606ActionSessionReset, got.Action, got.Description)
	require.Contains(t, got.Description, "Section 5.2")

	validOrigin := append([]byte{0x40, 0x01, 0x01, 0x00}, unreach...)
	got = message.ValidateUpdateRFC7606(validOrigin, false, true, true)
	require.Equal(t, message.RFC7606ActionSessionReset, got.Action, got.Description)
	require.Contains(t, got.Description, "Section 5.2")
}
