package yang

import (
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateEmptyAcceptsOnlyPresence pins that a type-empty leaf accepts
// the presence marker the config tree stores and refuses anything else.
// RFC 7950 Section 9.11.2 gives the type no lexical representation, so this
// is ze's tree contract, not a lexical form.
func TestValidateEmptyAcceptsOnlyPresence(t *testing.T) {
	v := &Validator{}
	typ := lexicalLeafType(t, "flag")
	require.Equal(t, gyang.Yempty, typ.Kind)
	assert.NoError(t, v.validateYangType("flag", typ, "true"))
	for _, value := range []any{"false", "", "yes", true} {
		err := v.validateYangType("flag", typ, value)
		require.Error(t, err, "%v must be refused", value)
	}
}

// TestValidateUncheckedKindsRefused pins that a value of a kind ze has no
// lexical check for (Ynone, which goyang leaves on an unresolved type) is
// refused rather than accepted unread, and that a union holding such a
// member still answers by its other members.
func TestValidateUncheckedKindsRefused(t *testing.T) {
	v := &Validator{}
	typ := &gyang.YangType{Kind: gyang.Ynone, Name: "none"}
	requireValidationErrorType(t, v.validateYangType("x", typ, "anything"), ErrTypeType, "none")
	union := &gyang.YangType{Kind: gyang.Yunion, Name: "u", Type: []*gyang.YangType{
		{Kind: gyang.Ynone, Name: "none"},
		{Kind: gyang.Yuint8, Name: "uint8"},
	}}
	assert.NoError(t, v.validateYangType("x", union, "7"), "the uint8 member must still answer")
	assert.Error(t, v.validateYangType("x", union, "anything"), "an unchecked member must not accept every value")
}

// TestValidateLeafrefNeedsLeaf pins that a leafref value checked with no
// leaf to start its path from is refused, never accepted unread.
func TestValidateLeafrefNeedsLeaf(t *testing.T) {
	v := &Validator{}
	typ := &gyang.YangType{Kind: gyang.Yleafref, Name: "leafref", Path: "../a"}
	requireValidationErrorType(t, v.validateYangType("x", typ, "anything"), ErrTypeType, "leafref")
}

// TestValidateBitsRefusesRepeatedName pins Ze's own rule that a bits value
// naming one bit twice is refused. RFC 7950 Section 9.7.2 does not call a
// repeated name malformed, so this is Ze's choice, not an RFC obligation: a
// bit is set or not, and a repeated name is more likely a typo than intent.
func TestValidateBitsRefusesRepeatedName(t *testing.T) {
	v := &Validator{}
	typ := lexicalLeafType(t, "flags")
	requireValidationErrorType(t, v.validateYangType("flags", typ, "a a"), ErrTypeType, "a a")
}
