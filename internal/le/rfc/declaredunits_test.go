// Design: docs/contributing/rfc-conformance-gates.md -- the one Go unit inventory.
// Related: goscope.go -- DeclaredUnits and readFunctionUnits, its RFC projection.
package rfc

import (
	"slices"
	"testing"
)

// TestReadFunctionUnitsKeepsConservativeSpansWhereSourceDoesNotParse proves
// the partial syntax tree the weakening check reads never reaches the RFC
// gate: source that does not parse still answers the line-walk spans and
// false, and white space answers no unit.
//
// METHOD: white-box, the expected units built from goFuncSpans directly.
// PREVENTS: an RFC unit, and so an approval digest, that moves because the
// shared inventory learned to read past a syntax error.
func TestReadFunctionUnitsKeepsConservativeSpansWhereSourceDoesNotParse(t *testing.T) {
	for _, content := range []string{
		"package p\nfunc TestA() {}\nfunc TestB() {\n",
		"package p\nfunc TestA() {}\n�\nfunc TestB() {}\n",
		"func TestA() {}\n",
		"package p\n/* unterminated",
	} {
		units, exact := readFunctionUnits(content)
		if exact {
			t.Fatalf("%q: source that does not parse answered exact units %#v", content, units)
		}
		var want []FunctionUnit
		for _, one := range goFuncSpans(content) {
			text := content[one.begin:one.end]
			want = append(want, FunctionUnit{Name: funcNameIn(text), Text: text})
		}
		if !slices.Equal(units, want) {
			t.Fatalf("%q: units = %#v, want the conservative spans %#v", content, units, want)
		}
	}
	for _, content := range []string{"", " \n\t\r\n"} {
		units, exact := readFunctionUnits(content)
		if !exact || len(units) != 0 {
			t.Fatalf("%q: white space answered %#v, %v; want no unit, exactly", content, units, exact)
		}
	}
}

// TestDeclaredUnitsAreExactOnlyBeforeTheFirstSyntaxError proves the inventory
// keeps a declaration the parser read whole before any error, and marks every
// declaration from the first error on as inexact, with no text.
//
// METHOD: partial-tree shapes: a truncated last body, an illegal byte after a
// closed declaration, an error inside the first of two declarations, a
// bodyless signature ended by the error token, and a bodyless declaration
// before an error, which is not exact even though the error lies after it.
// PREVENTS: a declaration the parser recovered into being trusted as an exact
// boundary, and an unrelated malformed byte voiding a declaration before it.
func TestDeclaredUnitsAreExactOnlyBeforeTheFirstSyntaxError(t *testing.T) {
	for _, test := range []struct {
		content string
		exact   []string
	}{
		{content: "package p\nfunc TestA() {}\nfunc TestB() {\n", exact: []string{"TestA"}},
		{content: "package p\nfunc TestA() {}\n�\nfunc TestB() {}\n", exact: []string{"TestA"}},
		{content: "package p\nfunc TestA() { ] }\nfunc TestB() {}\n"},
		{content: "package p\nfunc TestA() )\nfunc TestB() {}\n"},
		{content: "package p\nfunc external()\nfunc TestA() {}\n\ufffd\n", exact: []string{"TestA"}},
	} {
		units, parsed := DeclaredUnits(test.content)
		if parsed {
			t.Fatalf("%q: source with a syntax error parsed", test.content)
		}
		var exact []string
		for _, unit := range units {
			if !unit.Exact {
				if unit.Text != "" {
					t.Fatalf("%q: inexact %s carries text %q", test.content, unit.Name, unit.Text)
				}
				continue
			}
			if unit.Text != test.content[unit.Start:unit.End] {
				t.Fatalf("%q: %s text %q is not its span", test.content, unit.Name, unit.Text)
			}
			exact = append(exact, unit.Name)
		}
		if !slices.Equal(exact, test.exact) {
			t.Fatalf("%q: exact units = %v, want %v (inventory %#v)", test.content, exact, test.exact, units)
		}
	}
}
