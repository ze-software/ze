package yang

import "testing"

// TestRFC7950EnumRestrictionKeepsTheBaseValue drives a restricted
// enumeration whose value statement differs from the base type's, which RFC
// 7950 Section 9.6.4.2 forbids, one that names an enum the base type does not
// assign, which Section 9.6.4 forbids, and the two restrictions both allow.
// goyang accepts all four, so the refusals are Ze's checkStructure.
//
// RFC requirement: RFC7950-9.6.4.2-1 negative — restricting typedef e (x is 1, y is 2) with "enum x { value 5; }" is refused naming the value.
// RFC requirement: RFC7950-9.6.4.2-1 positive — restricting typedef e with "enum x { value 1; }", the base value, or with "enum x;", no value, loads.
// RFC requirement: RFC7950-9.6.4-2 negative — restricting typedef e (x, y) with "enum z", a name the base type does not assign, is refused naming the assigned name.
// RFC requirement: RFC7950-9.6.4-2 positive — restricting typedef e with "enum x", a name of the base type, keeping or omitting its value, loads.
func TestRFC7950EnumRestrictionKeepsTheBaseValue(t *testing.T) {
	const base = `module m { namespace "urn:m"; prefix m; typedef e { type enumeration { enum x { value 1; } enum y { value 2; } } } `
	requireModuleRefused(t, "changed value", "value", base+`leaf a { type e { enum x { value 5; } } } }`)
	requireModuleRefused(t, "name outside the base", "assigned name", base+`leaf a { type e { enum z; } } }`)
	requireModuleLoaded(t, "same value", base+`leaf a { type e { enum x { value 1; } } } }`)
	requireModuleLoaded(t, "value omitted", base+`leaf a { type e { enum x; } } }`)
}

// TestRFC7950ExtensionUsageSubstatementsAreYANGStatements drives an extension
// USAGE, the statement "m:e" that instantiates an extension, whose
// substatement is the unprefixed non-YANG keyword "bogus", and one whose
// "description" carries no argument against the Section 14 grammar. Section
// 7.19 binds these substatements; the extension definition's own
// substatements are what TestRFC7950ModuleLoadRefused drives. goyang keeps a
// usage as raw text, so the refusals are Ze's checkStructure.
//
// RFC requirement: RFC7950-7.19-1 negative — an extension usage "m:e" whose substatement is "bogus", not a YANG keyword, or "description" with no argument, is each refused naming that substatement.
// RFC requirement: RFC7950-7.19-1 positive — an extension usage whose substatement is "description" with an argument, or another usage of the extension, loads.
func TestRFC7950ExtensionUsageSubstatementsAreYANGStatements(t *testing.T) {
	const head = `module m { namespace "urn:m"; prefix m; extension e { argument text; } leaf a { type string; `
	requireModuleRefused(t, "non-YANG substatement", "bogus", head+`m:e "x" { bogus "y"; } } }`)
	requireModuleRefused(t, "description without argument", "description", head+`m:e "x" { description; } } }`)
	requireModuleLoaded(t, "YANG substatement", head+`m:e "x" { description "d"; } } }`)
	requireModuleLoaded(t, "extension substatement", head+`m:e "x" { m:e "y"; } } }`)
}

// TestRFC7950LengthPartsDisjointAndAscending drives a length whose two parts
// overlap and one whose parts descend, which RFC 7950 Section 9.4.4 forbids,
// and the same parts disjoint and ascending. goyang sorts and coalesces the
// parts before validating them, so the refusals are Ze's checkStructure.
//
// RFC requirement: RFC7950-9.4.4-1 negative — length "1..5 | 3..8", whose parts overlap, and "10..20 | 1..5", whose parts descend, are each refused naming the length.
// RFC requirement: RFC7950-9.4.4-1 positive — length "0..5 | 10..20", disjoint and ascending, loads.
func TestRFC7950LengthPartsDisjointAndAscending(t *testing.T) {
	const head = `module m { namespace "urn:m"; prefix m; leaf a { type string { length "`
	requireModuleRefused(t, "overlapping parts", "length", head+`1..5 | 3..8"; } } }`)
	requireModuleRefused(t, "descending parts", "length", head+`10..20 | 1..5"; } } }`)
	requireModuleLoaded(t, "disjoint ascending parts", head+`0..5 | 10..20"; } } }`)
}
