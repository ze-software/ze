package yang

import "testing"

// TestRFC7950EnumRestrictionKeepsTheBaseValue drives a restricted
// enumeration whose value statement differs from the base type's, which RFC
// 7950 Section 9.6.4.2 forbids, and the two restrictions it allows.
//
// It is RED: goyang accepts the changed value and Ze adds no check of its
// own. The refusal belongs to plan/pre-release/spec-config-yang-loader-structural-checks.md,
// and the test carries no RFC tag until that refusal exists.
func TestRFC7950EnumRestrictionKeepsTheBaseValue(t *testing.T) {
	const base = `module m { namespace "urn:m"; prefix m; typedef e { type enumeration { enum x { value 1; } enum y { value 2; } } } `
	requireModuleRefused(t, "changed value", "value", base+`leaf a { type e { enum x { value 5; } } } }`)
	requireModuleLoaded(t, "same value", base+`leaf a { type e { enum x { value 1; } } } }`)
	requireModuleLoaded(t, "value omitted", base+`leaf a { type e { enum x; } } }`)
}

// TestRFC7950ExtensionUsageSubstatementsAreYANGStatements drives an extension
// USAGE, the statement "m:e" that instantiates an extension, whose
// substatement is the unprefixed non-YANG keyword "bogus", and one whose
// "description" carries no argument against the Section 14 grammar. Section
// 7.19 binds these substatements; the extension definition's own
// substatements are what TestRFC7950ModuleLoadRefused drives.
//
// It is RED: goyang accepts both usages and Ze adds no check of its own. The
// refusal belongs to plan/pre-release/spec-config-yang-loader-structural-checks.md,
// and the test carries no RFC tag until that refusal exists.
func TestRFC7950ExtensionUsageSubstatementsAreYANGStatements(t *testing.T) {
	const head = `module m { namespace "urn:m"; prefix m; extension e { argument text; } leaf a { type string; `
	requireModuleRefused(t, "non-YANG substatement", "bogus", head+`m:e "x" { bogus "y"; } } }`)
	requireModuleRefused(t, "description without argument", "description", head+`m:e "x" { description; } } }`)
	requireModuleLoaded(t, "YANG substatement", head+`m:e "x" { description "d"; } } }`)
	requireModuleLoaded(t, "extension substatement", head+`m:e "x" { m:e "y"; } } }`)
}

// TestRFC7950LengthPartsDisjointAndAscending drives a length whose two parts
// overlap and one whose parts descend, which RFC 7950 Section 9.4.4 forbids,
// and the same parts disjoint and ascending.
//
// It is RED: goyang accepts both lengths and Ze adds no check of its own.
// The refusal belongs to plan/pre-release/spec-config-yang-loader-structural-checks.md,
// and the test carries no RFC tag until that refusal exists.
func TestRFC7950LengthPartsDisjointAndAscending(t *testing.T) {
	const head = `module m { namespace "urn:m"; prefix m; leaf a { type string { length "`
	requireModuleRefused(t, "overlapping parts", "length", head+`1..5 | 3..8"; } } }`)
	requireModuleRefused(t, "descending parts", "length", head+`10..20 | 1..5"; } } }`)
	requireModuleLoaded(t, "disjoint ascending parts", head+`0..5 | 10..20"; } } }`)
}
