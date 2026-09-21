package yang

import (
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decimal64Type builds a decimal64 YangType with the given fraction-digits
// and range text, through the same goyang parser the loader uses, so the
// Range carries Numbers with the type's fraction digits.
func decimal64Type(t *testing.T, fractionDigits uint8, rangeText string) *gyang.YangType {
	t.Helper()
	ranges, err := gyang.ParseRangesDecimal(rangeText, fractionDigits)
	require.NoError(t, err)
	return &gyang.YangType{Kind: gyang.Ydecimal64, Name: "decimal64", FractionDigits: int(fractionDigits), Range: ranges}
}

// TestRFC7950Decimal64Accepted drives every lexical form of a decimal64 value
// through validateYangType and expects each to pass with the value inside
// the range.
//
// RFC requirement: RFC7950-9.1-1 positive — a decimal64 value with a sign, without a sign, with a fraction and without one is each accepted by validateYangType.
// RFC requirement: RFC7950-9.3.2-1 positive — a decimal64 value with at least one digit before and after the decimal point is accepted.
func TestRFC7950Decimal64Accepted(t *testing.T) {
	v := &Validator{}
	typ := decimal64Type(t, 2, "-10.5..10.5")
	for _, value := range []string{"0.0", "1", "+1.5", "-1.5", "10.50", "-10.5", "0.25"} {
		assert.NoError(t, v.validateYangType("x", typ, value), "%q must be accepted", value)
	}
}

// TestRFC7950Decimal64Refused drives a decimal64 value with no digit on one
// side of the point, a non-numeric value, a value with more fraction digits
// than the type declares, and a value outside the range through
// validateYangType and expects each refusal to name the fault.
//
// RFC requirement: RFC7950-9.1-1 negative — a decimal64 value that is not in the lexical representation ("abc", "1.2.3", a bare sign) is refused with ErrTypeType, and a value beyond the type's fraction-digits or range is refused.
// RFC requirement: RFC7950-9.3.2-1 negative — a decimal64 value with no digit before the point (".5") or none after it ("1.") is refused with ErrTypeType.
func TestRFC7950Decimal64Refused(t *testing.T) {
	v := &Validator{}
	typ := decimal64Type(t, 2, "-10.5..10.5")
	for _, value := range []string{".5", "1.", "abc", "1.2.3", "+", "-", ""} {
		err := v.validateYangType("x", typ, value)
		require.Error(t, err, "%q must be refused", value)
		var verr *ValidationError
		require.ErrorAs(t, err, &verr)
		assert.Equal(t, ErrTypeType, verr.Type, "%q must be a type error", value)
		assert.Contains(t, verr.Message, "expected decimal64")
	}

	err := v.validateYangType("x", typ, "1.234")
	require.Error(t, err, "three fraction digits against fraction-digits 2 must be refused")
	assert.Contains(t, err.Error(), "at most 2 fraction digits")

	err = v.validateYangType("x", typ, "10.51")
	require.Error(t, err, "a value above the range must be refused")
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	assert.Equal(t, ErrTypeRange, verr.Type)

	err = v.validateYangType("x", typ, 1.5)
	require.Error(t, err, "a non-string value must be refused")
}

// TestBooleanConfigSpellingRefused pins that the yang validator takes only
// the RFC 7950 lexical form of a boolean: the config spelling enable and
// disable is rewritten by NormalizeLeafValue before a value reaches it.
func TestBooleanConfigSpellingRefused(t *testing.T) {
	v := &Validator{}
	typ := &gyang.YangType{Kind: gyang.Ybool, Name: "boolean"}
	for _, value := range []string{"enable", "disable"} {
		require.Error(t, v.validateYangType("x", typ, value), "%q must be refused", value)
	}
}

// TestSignedRangeNegativeBound pins that a signed range whose lower bound is
// negative accepts zero and a negative value inside it, and refuses a value
// below it. goyang stores the bound as a magnitude with a sign flag, and the
// check used to read the magnitude as the bound, which put zero outside every
// int32 range.
func TestSignedRangeNegativeBound(t *testing.T) {
	v := &Validator{}
	ranges, err := gyang.ParseRangesInt("-10..10")
	require.NoError(t, err)
	typ := &gyang.YangType{Kind: gyang.Yint32, Name: "int32", Range: ranges}
	for _, value := range []string{"0", "-10", "10", "-3"} {
		assert.NoError(t, v.validateYangType("x", typ, value), "%q must be inside -10..10", value)
	}
	for _, value := range []string{"-11", "11"} {
		require.Error(t, v.validateYangType("x", typ, value), "%q must be outside -10..10", value)
	}
	full, err := gyang.ParseRangesInt("-9223372036854775808..9223372036854775807")
	require.NoError(t, err)
	typ = &gyang.YangType{Kind: gyang.Yint64, Name: "int64", Range: full}
	assert.NoError(t, v.validateYangType("x", typ, "-9223372036854775808"))
	assert.NoError(t, v.validateYangType("x", typ, "9223372036854775807"))
}
