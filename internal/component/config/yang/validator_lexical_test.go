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

// TestValidateUncheckedKindsRefused pins that a value of a type ze has no
// lexical check for is refused rather than accepted unread, and that a union
// holding such a member still answers by its other members.
func TestValidateUncheckedKindsRefused(t *testing.T) {
	v := &Validator{}
	for _, kind := range []gyang.TypeKind{gyang.Yleafref, gyang.Yidentityref, gyang.YinstanceIdentifier, gyang.Ynone} {
		typ := &gyang.YangType{Kind: kind, Name: kind.String()}
		requireValidationErrorType(t, v.validateYangType("x", typ, "anything"), ErrTypeType, kind.String())
	}
	union := &gyang.YangType{Kind: gyang.Yunion, Name: "u", Type: []*gyang.YangType{
		{Kind: gyang.Yleafref, Name: "leafref"},
		{Kind: gyang.Yuint8, Name: "uint8"},
	}}
	assert.NoError(t, v.validateYangType("x", union, "7"), "the uint8 member must still answer")
	assert.Error(t, v.validateYangType("x", union, "anything"), "a leafref member must not accept every value")
}

// TestLoaderRefusesUncheckedTypes pins that Resolve refuses a module
// declaring leafref, identityref or instance-identifier, including as a
// typedef base and as a union member, naming the type, and loads the twin
// module without them.
func TestLoaderRefusesUncheckedTypes(t *testing.T) {
	const head = `module unchecked { namespace "urn:ze:test:unchecked"; prefix uc; identity base1; leaf a { type string; } `
	for name, body := range map[string]string{
		"leafref":             `leaf b { type leafref { path "../a"; } }`,
		"identityref":         `leaf b { type identityref { base base1; } }`,
		"instance-identifier": `leaf b { type instance-identifier; }`,
		"typedef base":        `typedef ref { type leafref { path "/uc:a"; } } leaf b { type ref; }`,
		"union member":        `leaf b { type union { type uint8; type leafref { path "../a"; } } }`,
	} {
		t.Run(name, func(t *testing.T) {
			err := loadModuleTexts(t, head+body+`}`)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrUncheckedType)
			assert.Contains(t, err.Error(), "ze does not check")
		})
	}
	require.NoError(t, loadModuleTexts(t, head+`leaf b { type string; } }`))
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
