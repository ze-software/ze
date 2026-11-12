// Related: parser.go — parseLeaf and leafValueFix name the line that replaces a refused enumeration leaf
package config

import (
	"strings"
	"testing"
)

// TestEnumLeafRefusalNamesTheReplacementLine covers the refusal an operator
// meets when an enumeration leaf is written without a usable value.
// route-refresh, extended-message and link-local-nexthop were presence
// containers until 2026-10-11, so their old spellings (bare, a block, true,
// false) reach this path. The method parses each spelling against a schema that
// holds the two enumerations the capabilities use, and asserts the whole
// refusal by equality: the line, the leaf named as the problem, every word it
// takes one per line in declared order, and the exact line to write instead
// as the last line. true still means enable and false disable where the
// enumeration offers them; a bare or block spelling, which says nothing about
// the value, offers the alternation (AC-18).
func TestEnumLeafRefusalNamesTheReplacementLine(t *testing.T) {
	schema := NewSchema()
	schema.Define("route-refresh", &LeafNode{Type: TypeString, Enums: []string{"enable", "disable", "require", "refuse"}})
	schema.Define("link-local-nexthop", &LeafNode{Type: TypeString, Enums: []string{"enable", "disable"}})
	schema.Define("color", &LeafNode{Type: TypeString, Enums: []string{"red", "blue"}})

	routeRefresh := "route-refresh accepts only these values:\n  enable\n  disable\n  require\n  refuse\n"
	tests := []struct {
		input string
		want  string
	}{
		{"route-refresh;", "line 1: route-refresh: needs a value\n" + routeRefresh + "fix: route-refresh <enable|disable|require|refuse>;"},
		{"route-refresh { }", "line 1: route-refresh: needs a value\n" + routeRefresh + "fix: route-refresh <enable|disable|require|refuse>;"},
		{"route-refresh true;", "line 1: route-refresh: \"true\" is not an accepted value\n" + routeRefresh + "fix: route-refresh enable;"},
		{"route-refresh false;", "line 1: route-refresh: \"false\" is not an accepted value\n" + routeRefresh + "fix: route-refresh disable;"},
		{"route-refresh maybe;", "line 1: route-refresh: \"maybe\" is not an accepted value\n" + routeRefresh + "fix: route-refresh <enable|disable|require|refuse>;"},
		{"link-local-nexthop require;", "line 1: link-local-nexthop: \"require\" is not an accepted value\nlink-local-nexthop accepts only these values:\n  enable\n  disable\nfix: link-local-nexthop <enable|disable>;"},
		{"color;", "line 1: color: needs a value\ncolor accepts only these values:\n  red\n  blue\nfix: color <red|blue>;"},
		// true is no legacy word where the enumeration holds no enable, and
		// two edits from blue is past the one a four-character value may be
		// corrected by (suggest.Correction): no hint, and the fix is the
		// alternation, never a value the operator did not mean.
		{"color true;", "line 1: color: \"true\" is not an accepted value\ncolor accepts only these values:\n  red\n  blue\nfix: color <red|blue>;"},
		// One edit from red is within that bound, so the suggestion is the fix.
		{"color rad;", "line 1: color: \"rad\" is not an accepted value (did you mean red?)\ncolor accepts only these values:\n  red\n  blue\nfix: color red;"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := NewParser(schema).Parse(tt.input)
			if err == nil {
				t.Fatalf("%q accepted, want a refusal", tt.input)
			}
			if err.Error() != tt.want {
				t.Errorf("%q refused with\n%s\nwant\n%s", tt.input, err.Error(), tt.want)
			}
			if strings.Contains(err.Error(), "write:") {
				t.Errorf("%q refusal still carries the old write: line: %q", tt.input, err.Error())
			}
		})
	}
}

// TestNonEnumLeafRefusalKeepsItsMessage proves the replacement line is
// offered only for an enumeration: a free string leaf has no words to name,
// so its refusal stays the plain one and invents no line.
func TestNonEnumLeafRefusalKeepsItsMessage(t *testing.T) {
	schema := NewSchema()
	schema.Define("description", &LeafNode{Type: TypeString})

	_, err := NewParser(schema).Parse("description;")
	if err == nil {
		t.Fatal("bare string leaf accepted, want a refusal")
	}
	if !strings.Contains(err.Error(), "expected value for description") {
		t.Errorf("refusal %q, want the plain expected-value message", err.Error())
	}
	if strings.Contains(err.Error(), "write:") {
		t.Errorf("refusal %q offers a replacement line for a non-enumeration leaf", err.Error())
	}
}
