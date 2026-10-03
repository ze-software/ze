package yang

import (
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/assert"
)

// TestRFC7950IntegerSignedLexicalForms drives the explicitly signed lexical
// form of an integer, "+17", through validateYangType for an unsigned and a
// signed type, beside the unsigned form "17" and the negative form "-17".
//
// RFC requirement: RFC7950-9.1-1 positive — the integer value 17 written "+17" is accepted for uint8 and int8, as "17" is, and "-17" is accepted for int8.
func TestRFC7950IntegerSignedLexicalForms(t *testing.T) {
	v := &Validator{}
	unsigned := &gyang.YangType{Kind: gyang.Yuint8, Name: "uint8"}
	signed := &gyang.YangType{Kind: gyang.Yint8, Name: "int8"}
	for _, value := range []string{"+17", "17"} {
		assert.NoError(t, v.validateYangType("x", unsigned, value), "%q must be accepted for uint8", value)
		assert.NoError(t, v.validateYangType("x", signed, value), "%q must be accepted for int8", value)
	}
	assert.NoError(t, v.validateYangType("x", signed, "-17"))
}
