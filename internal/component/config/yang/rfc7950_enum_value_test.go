package yang

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// enumModule wraps body, the statements of one module, in module m.
func enumModule(body string) string {
	return `module m { namespace "urn:m"; prefix m; ` + body + ` }`
}

// TestRFC7950EnumImplicitValueFollowsTheHighest drives the value RFC 7950
// Section 9.6.4.2 assigns an enum with no value statement, through a
// restriction that restates it: the restriction loads when it states the
// assigned value and is refused when it states any other. Each base type
// exercises one clause of the rule: after an explicit -5 the next value is
// -4, where goyang v1.6.3 assigned 0; after 10 then 3 it is 11; an enum made
// conditional by if-feature still takes its value; and two typedef levels
// down the value is still the root enumeration's.
//
// VALIDATES: the implicit enum value is one greater than the highest before it.
// PREVENTS: any value but the one the RFC assigns deciding a restriction.
//
// RFC requirement: RFC7950-9.6.4.2-1 negative — restricting a base type whose enum q follows "value -5" with "enum q { value 0; }", whose c follows values 10 and 3 with "value 4", whose c follows a and if-feature b with "value 1", or two typedef levels below the -5 base with q "value 0", is each refused naming the value.
// RFC requirement: RFC7950-9.6.4.2-1 positive — the same restrictions stating q "value -4", c "value 11", c "value 2", and q "value -4" two typedef levels down each load.
func TestRFC7950EnumImplicitValueFollowsTheHighest(t *testing.T) {
	const negative = `typedef e { type enumeration { enum p { value -5; } enum q; } } `
	requireModuleLoaded(t, "after a negative", enumModule(negative+`leaf a { type e { enum q { value -4; } } }`))
	requireModuleRefused(t, "after a negative, zero", "value 0",
		enumModule(negative+`leaf a { type e { enum q { value 0; } } }`))

	const descending = `typedef e { type enumeration { enum a { value 10; } enum b { value 3; } enum c; } } `
	requireModuleLoaded(t, "after the highest", enumModule(descending+`leaf x { type e { enum c { value 11; } } }`))
	requireModuleRefused(t, "after the latest", "value 4",
		enumModule(descending+`leaf x { type e { enum c { value 4; } } }`))

	const feature = `feature f; typedef e { type enumeration { enum a; enum b { if-feature f; } enum c; } } `
	requireModuleLoaded(t, "if-feature counted", enumModule(feature+`leaf x { type e { enum c { value 2; } } }`))
	requireModuleRefused(t, "if-feature skipped", "value 1",
		enumModule(feature+`leaf x { type e { enum c { value 1; } } }`))

	const chain = negative + `typedef e2 { type e { enum p; enum q; } } `
	requireModuleLoaded(t, "two levels down", enumModule(chain+`leaf a { type e2 { enum q { value -4; } } }`))
	requireModuleRefused(t, "two levels down, zero", "value 0",
		enumModule(chain+`leaf a { type e2 { enum q { value 0; } } }`))
}

// TestRFC7950EnumValueRangeAndUniqueness drives the two refusals RFC 7950
// Section 9.6.4.2 makes of the values an enumeration assigns: an enum with no
// value after one holding 2147483647, which the next value would carry past
// the int32 range, and an explicit value that repeats an implicitly assigned
// one. The refusals asserted here are Ze's own, by ErrEnumValue.
//
// VALIDATES: an enumeration's values stay within int32 and unique.
// PREVENTS: an implicit value past 2147483647, or one an explicit value repeats.
//
// RFC requirement: RFC7950-9.6.4.2-1 negative — "enum a { value 2147483647; } enum b;", and "enum p { value -5; } enum q; enum r { value -4; }" whose r repeats q's implicit -4, are each refused by Ze's enum value check.
// RFC requirement: RFC7950-9.6.4.2-1 positive — "enum a { value 2147483646; } enum b;", whose b is 2147483647, and "enum p { value -5; } enum q; enum r { value -3; }" each load.
func TestRFC7950EnumValueRangeAndUniqueness(t *testing.T) {
	refused := func(name, enums string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			err := loadModuleTexts(t, enumModule(`leaf x { type enumeration { `+enums+` } }`))
			require.Error(t, err, "loader must refuse the violating module")
			assert.True(t, errors.Is(err, ErrEnumValue), "Ze's enum value check must refuse it: %v", err)
		})
	}
	refused("past int32", `enum a { value 2147483647; } enum b;`)
	refused("repeats the implicit value", `enum p { value -5; } enum q; enum r { value -4; }`)
	requireModuleLoaded(t, "reaches int32 max", enumModule(`leaf x { type enumeration { enum a { value 2147483646; } enum b; } }`))
	requireModuleLoaded(t, "distinct after a negative", enumModule(`leaf x { type enumeration { enum p { value -5; } enum q; enum r { value -3; } } }`))
}

// TestEnumNamesFollowTheAssignedValues proves the two surfaces that order an
// enumeration's names by value, the schema node (EnumNamesDeclared) and the
// command argument (argDefFor), read the values RFC 7950 assigns: q after
// "value -5" is -4, so it sorts before r at -3. goyang v1.6.3 numbered q 0
// and would have put it last.
//
// VALIDATES: names offered to an operator follow the RFC-assigned values.
// PREVENTS: a surface ordering names by anything but the RFC-assigned values.
func TestEnumNamesFollowTheAssignedValues(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.AddModuleFromText("m.yang",
		enumModule(`leaf x { type enumeration { enum p { value -5; } enum q; enum r { value -3; } } }`)))
	require.NoError(t, loader.Resolve())
	leaf := loader.GetEntry("m").Dir["x"]
	require.NotNil(t, leaf)

	names, err := EnumNamesDeclared(leaf)
	require.NoError(t, err)
	assert.Equal(t, []string{"p", "q", "r"}, names)

	def, ok := argDefFor(leaf, "x")
	require.True(t, ok)
	assert.Equal(t, []string{"p", "q", "r"}, def.EnumValues)
}
