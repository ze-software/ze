package yang

import (
	"strings"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requireModuleRefused loads the module set in a subtest named after the case
// and requires a refusal whose text names the fault, so the case fails when
// the loader accepts the set or refuses it for another reason.
func requireModuleRefused(t *testing.T, name, fault string, texts ...string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		err := loadModuleTexts(t, texts...)
		require.Error(t, err, "loader must refuse the violating module set")
		assert.Contains(t, strings.ToLower(err.Error()), fault)
	})
}

// requireModuleLoaded loads the module set in a subtest named after the case
// and requires no refusal.
func requireModuleLoaded(t *testing.T, name string, texts ...string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		require.NoError(t, loadModuleTexts(t, texts...), "loader must accept the conforming module set")
	})
}

// TestRFC7950LeafTypeMustNameAnExistingType drives a leaf whose type
// statement names no built-in and no defined type, and the conforming twins
// naming a local derived type and an imported one. Every module differs from
// its twin in the type name alone, so the refusal is pinned to the name.
//
// RFC requirement: RFC7950-7.6.3-1 negative — a leaf whose type statement names "nosuch", neither a built-in nor a defined type, is refused as an unknown type.
// RFC requirement: RFC7950-7.6.3-1 positive — a leaf whose type statement names a local typedef, or an imported typedef with its prefix, loads.
func TestRFC7950LeafTypeMustNameAnExistingType(t *testing.T) {
	requireModuleRefused(t, "unknown type name", "unknown type",
		`module m { namespace "urn:m"; prefix m; typedef t { type uint8; } leaf a { type nosuch; } }`)
	requireModuleLoaded(t, "local derived type",
		`module m { namespace "urn:m"; prefix m; typedef t { type uint8; } leaf a { type t; } }`)
	requireModuleLoaded(t, "imported derived type", rfc7950Base,
		`module m { namespace "urn:m"; prefix m; import ext { prefix ext; } leaf a { type ext:port; } }`)
}

// TestRFC7950TypedefTypeMustBePresent drives a typedef with no type
// substatement, refused, and its twin with one, which loads and gives the leaf
// that uses it the base type the typedef names.
//
// RFC requirement: RFC7950-7.3.2-1 negative — a typedef that carries a description and no type substatement is refused naming the type.
// RFC requirement: RFC7950-7.3.2-1 positive — the same typedef with "type uint8" loads, and the leaf using it resolves to the uint8 base type.
func TestRFC7950TypedefTypeMustBePresent(t *testing.T) {
	requireModuleRefused(t, "typedef without type", "type",
		`module m { namespace "urn:m"; prefix m; typedef t { description "d"; } leaf a { type t; } }`)

	loader := NewLoader()
	require.NoError(t, loader.AddModuleFromText("m.yang",
		`module m { namespace "urn:m"; prefix m; typedef t { type uint8; description "d"; } leaf a { type t; } }`))
	schema, err := loader.Resolve()
	require.NoError(t, err)
	entry := schema.GetEntry("m")
	require.NotNil(t, entry)
	leaf := entry.Dir["a"]
	require.NotNil(t, leaf)
	require.NotNil(t, leaf.Type)
	assert.Equal(t, gyang.Yuint8, leaf.Type.Kind, "the typedef's type statement defines the leaf's base type")
}

// TestRFC7950CaseIdentifierUniqueWithinAChoice drives a choice with two
// cases named x, refused, and two twins that load: the same choice with
// cases x and y, and two choices that each hold a case named x, which proves
// the uniqueness is scoped to one choice.
//
// RFC requirement: RFC7950-7.9.2-2 negative — a choice holding two cases both named x is refused as a duplicate.
// RFC requirement: RFC7950-7.9.2-2 positive — a choice with cases x and y loads, and two choices that each hold one case named x load.
func TestRFC7950CaseIdentifierUniqueWithinAChoice(t *testing.T) {
	requireModuleRefused(t, "duplicate case in one choice", "duplicate",
		`module m { namespace "urn:m"; prefix m; choice c { case x { leaf a { type string; } } case x { leaf b { type uint8; } } } }`)
	requireModuleLoaded(t, "distinct cases",
		`module m { namespace "urn:m"; prefix m; choice c { case x { leaf a { type string; } } case y { leaf b { type uint8; } } } }`)
	requireModuleLoaded(t, "same case name in two choices",
		`module m { namespace "urn:m"; prefix m; choice c { case x { leaf a { type string; } } } choice d { case x { leaf b { type uint8; } } } }`)
}

// TestRFC7950LengthRestrictionMustNotWiden drives a derived type whose
// length widens its base type's "2..5", refused, and three restrictions the
// sentence allows: an equal one, a raised lower and reduced upper bound, and
// a split into two ranges with a gap.
//
// RFC requirement: RFC7950-9.4.4-2 negative — a leaf restricting a typedef of length "2..5" to length "1..10" is refused naming the length.
// RFC requirement: RFC7950-9.4.4-2 positive — restricting that typedef to "2..5", to "3..4", or to "2..3 | 5..5" loads.
func TestRFC7950LengthRestrictionMustNotWiden(t *testing.T) {
	const base = `module m { namespace "urn:m"; prefix m; typedef s { type string { length "2..5"; } } `
	requireModuleRefused(t, "widened", "length", base+`leaf a { type s { length "1..10"; } } }`)
	requireModuleLoaded(t, "equal", base+`leaf a { type s { length "2..5"; } } }`)
	requireModuleLoaded(t, "narrowed", base+`leaf a { type s { length "3..4"; } } }`)
	requireModuleLoaded(t, "split with a gap", base+`leaf a { type s { length "2..3 | 5..5"; } } }`)
}

// TestRFC7950EnumValueWithinInt32 drives an enum value one below the int32
// minimum, refused, against the minimum itself, which loads.
//
// RFC requirement: RFC7950-9.6.4.2-1 negative — an enum value of -2147483649, one below the int32 minimum, is refused naming the value.
// RFC requirement: RFC7950-9.6.4.2-1 positive — an enum value of -2147483648, the int32 minimum, loads.
func TestRFC7950EnumValueWithinInt32(t *testing.T) {
	requireModuleRefused(t, "below int32", "value",
		`module m { namespace "urn:m"; prefix m; leaf a { type enumeration { enum x { value -2147483649; } } } }`)
	requireModuleLoaded(t, "int32 minimum",
		`module m { namespace "urn:m"; prefix m; leaf a { type enumeration { enum x { value -2147483648; } } } }`)
}
