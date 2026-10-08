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
// substatements break the Section 14 grammar in each way the grammar
// defines: an unprefixed non-YANG keyword "bogus", a "description" with no
// argument, a "config" whose argument is "maybe" where config-arg reads
// "true" or "false", and a "description" whose block holds a "leaf", which
// description-stmt ("description-keyword sep string stmtend") admits no
// statement for. Section 7.19 binds these substatements; the extension
// definition's own substatements are what TestRFC7950ModuleLoadRefused
// drives. goyang keeps a usage as raw text, so the refusals are Ze's
// checkStructure.
//
// RFC requirement: RFC7950-7.19-1 negative — an extension usage "m:e" whose substatement is "bogus", not a YANG keyword, "description" with no argument, "config maybe", or "description" holding a "leaf", is each refused naming that substatement.
// RFC requirement: RFC7950-7.19-1 positive — an extension usage whose substatement is "description" with an argument, another usage of the extension, "config true", or a "leaf" holding its "type", loads.
func TestRFC7950ExtensionUsageSubstatementsAreYANGStatements(t *testing.T) {
	const head = `module m { namespace "urn:m"; prefix m; extension e { argument text; } leaf a { type string; `
	requireModuleRefused(t, "non-YANG substatement", "bogus", head+`m:e "x" { bogus "y"; } } }`)
	requireModuleRefused(t, "description without argument", "description", head+`m:e "x" { description; } } }`)
	requireModuleRefused(t, "config argument outside config-arg", "maybe", head+`m:e "x" { config "maybe"; } } }`)
	requireModuleRefused(t, "leaf inside description", "leaf is not a substatement of description",
		head+`m:e "x" { description "d" { leaf q { type string; } } } } }`)
	requireModuleLoaded(t, "YANG substatement", head+`m:e "x" { description "d"; } } }`)
	requireModuleLoaded(t, "extension substatement", head+`m:e "x" { m:e "y"; } } }`)
	requireModuleLoaded(t, "config true", head+`m:e "x" { config "true"; } } }`)
	requireModuleLoaded(t, "leaf with its type", head+`m:e "x" { leaf q { type string { length "1..5"; } } } } }`)
}

// TestRFC7950LengthMinMaxAreTheRestrictedTypeBounds drives a length over a
// typedef whose own length is "2..10", where RFC 7950 Section 9.4.4 makes
// "min" 2 and "max" 10: ""min" and "max" mean the minimum and maximum lengths
// accepted for the type being restricted". "min | 2" then names 2 twice and
// "10 | max" names 10 twice, so neither is disjoint, while "min..4 | 6..max"
// and "min..max" are. Read as 0 and the largest uint64 instead, the two
// refused lengths would pass.
//
// RFC requirement: RFC7950-9.4.4-1 negative — over a typedef of length "2..10", length "min | 2" and "10 | max", whose parts coincide once min is 2 and max is 10, are each refused naming the length.
// RFC requirement: RFC7950-9.4.4-1 positive — over that typedef, length "min..4 | 6..max", ascending and disjoint, and "min..max" load.
func TestRFC7950LengthMinMaxAreTheRestrictedTypeBounds(t *testing.T) {
	const head = `module m { namespace "urn:m"; prefix m; typedef s { type string { length "2..10"; } } leaf a { type s { length "`
	requireModuleRefused(t, "min names the lower bound", "length", head+`min | 2"; } } }`)
	requireModuleRefused(t, "max names the upper bound", "length", head+`10 | max"; } } }`)
	requireModuleLoaded(t, "ascending disjoint over the bounds", head+`min..4 | 6..max"; } } }`)
	requireModuleLoaded(t, "the whole restricted span", head+`min..max"; } } }`)
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

// TestLengthInAnUnusedGroupingReadsTheTypedefBounds drives a length over a
// typedef of length "2..10" inside a grouping no schema node uses. goyang
// resolves that type too, so "min" and "max" read 2 and 10 there as well:
// "min | 2" is refused, while "min..4 | 6..max" loads.
//
// VALIDATES: restrictedLengthSpan on a type in an unused grouping.
func TestLengthInAnUnusedGroupingReadsTheTypedefBounds(t *testing.T) {
	const head = `module m { namespace "urn:m"; prefix m; typedef s { type string { length "2..10"; } } grouping g { leaf a { type s { length "`
	requireModuleRefused(t, "min names the lower bound", "does not start after", head+`min | 2"; } } } }`)
	requireModuleLoaded(t, "ascending disjoint over the bounds", head+`min..4 | 6..max"; } } } }`)
}
