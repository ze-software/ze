package yang

import (
	"errors"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lexicalTypesModule declares one leaf of each type whose lexical check
// validator_lexical.go carries, so the tests below judge the YangType goyang
// resolves rather than one built by hand.
const lexicalTypesModule = `module lexical {
    namespace "urn:ze:test:lexical";
    prefix lx;
    leaf blob { type binary { length "0..3"; } }
    leaf flags { type bits { bit a; bit b; bit c; } }
    leaf flag { type empty; }
}`

// lexicalLeafType resolves lexicalTypesModule and answers the type of leaf.
func lexicalLeafType(t *testing.T, leaf string) *gyang.YangType {
	t.Helper()
	loader := NewLoader()
	require.NoError(t, loader.AddModuleFromText("lexical.yang", lexicalTypesModule))
	resolved, err := loader.Resolve()
	require.NoError(t, err)
	entry := resolved.GetEntry("lexical")
	require.NotNil(t, entry)
	child := entry.Dir[leaf]
	require.NotNil(t, child, "leaf %s", leaf)
	require.NotNil(t, child.Type, "leaf %s type", leaf)
	return child.Type
}

// requireValidationErrorType asserts err is a ValidationError of kind want.
func requireValidationErrorType(t *testing.T, err error, want ErrorType, value string) {
	t.Helper()
	require.Error(t, err, "%q must be refused", value)
	var verr *ValidationError
	require.True(t, errors.As(err, &verr), "%q: error %v is no ValidationError", value, err)
	assert.Equal(t, want, verr.Type, "%q: %v", value, err)
}

// TestRFC7950BinaryAccepted drives base64 values of zero to three octets
// through validateYangType for a binary leaf of length 0..3.
//
// RFC requirement: RFC7950-9.1-1 positive -- the binary values "" (zero octets), "AQ==" (one), "AQI=" (two) and "AQID" (three), base64 with padding, are each accepted by validateYangType for a binary type of length 0..3.
func TestRFC7950BinaryAccepted(t *testing.T) {
	v := &Validator{}
	typ := lexicalLeafType(t, "blob")
	require.Equal(t, gyang.Ybinary, typ.Kind)
	for _, value := range []string{"", "AQ==", "AQI=", "AQID"} {
		assert.NoError(t, v.validateYangType("blob", typ, value), "%q must be accepted", value)
	}
}

// TestRFC7950BinaryRefused drives values outside the base64 alphabet or its
// padding rules, and a value of four octets, through validateYangType for a
// binary leaf of length 0..3.
//
// RFC requirement: RFC7950-9.1-1 negative -- the binary values "AQ" and "AQ=" (missing padding), "AR==" (non-zero padding bits), "A Q==", "AQ==\n" and "!!!!" (outside the alphabet) are each refused by validateYangType with ErrTypeType, and "AQIDBA==" (four octets, against length 0..3) with ErrTypeLength.
func TestRFC7950BinaryRefused(t *testing.T) {
	v := &Validator{}
	typ := lexicalLeafType(t, "blob")
	for _, value := range []string{"AQ", "AQ=", "AR==", "A Q==", "AQ==\n", "!!!!"} {
		requireValidationErrorType(t, v.validateYangType("blob", typ, value), ErrTypeType, value)
	}
	requireValidationErrorType(t, v.validateYangType("blob", typ, "AQIDBA=="), ErrTypeLength, "AQIDBA==")
}

// TestRFC7950BitsAccepted drives the empty set, one bit, and several bits in
// any order through validateYangType for a bits leaf of bits a, b and c.
//
// RFC requirement: RFC7950-9.1-1 positive -- the bits values "" (no bit set), "a", "c a" and "a b c", space-separated names of defined bits, are each accepted by validateYangType for a bits type of bits a, b and c.
func TestRFC7950BitsAccepted(t *testing.T) {
	v := &Validator{}
	typ := lexicalLeafType(t, "flags")
	require.Equal(t, gyang.Ybits, typ.Kind)
	for _, value := range []string{"", "a", "c a", "a b c"} {
		assert.NoError(t, v.validateYangType("flags", typ, value), "%q must be accepted", value)
	}
}

// TestRFC7950BitsRefused drives an undefined bit name and a tab separator
// through validateYangType for a bits leaf of bits a, b and c.
//
// RFC requirement: RFC7950-9.1-1 negative -- the bits values "d" and "a d" (a name no bit defines) and "a\tb" (a tab in place of the space separator) are each refused by validateYangType with ErrTypeEnum.
func TestRFC7950BitsRefused(t *testing.T) {
	v := &Validator{}
	typ := lexicalLeafType(t, "flags")
	for _, value := range []string{"d", "a d", "a\tb"} {
		requireValidationErrorType(t, v.validateYangType("flags", typ, value), ErrTypeEnum, value)
	}
}
