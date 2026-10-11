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
// holds the two enumerations the capabilities use, and asserts the error names
// the leaf, the words it takes, and the exact line to write instead.
func TestEnumLeafRefusalNamesTheReplacementLine(t *testing.T) {
	schema := NewSchema()
	schema.Define("route-refresh", &LeafNode{Type: TypeString, Enums: []string{"enable", "disable", "require", "refuse"}})
	schema.Define("link-local-nexthop", &LeafNode{Type: TypeString, Enums: []string{"enable", "disable"}})
	schema.Define("color", &LeafNode{Type: TypeString, Enums: []string{"red", "blue"}})

	tests := []struct {
		input string
		want  []string
	}{
		{"route-refresh;", []string{"route-refresh needs a value, one of enable, disable, require, refuse", "write: route-refresh enable;"}},
		{"route-refresh { }", []string{"route-refresh needs a value", "write: route-refresh enable;"}},
		{"route-refresh true;", []string{`invalid value for route-refresh: invalid enum: "true"`, "write: route-refresh enable;"}},
		{"route-refresh false;", []string{`invalid enum: "false"`, "write: route-refresh disable;"}},
		{"route-refresh maybe;", []string{`invalid enum: "maybe"`, "write: route-refresh <enable|disable|require|refuse>;"}},
		{"link-local-nexthop require;", []string{`invalid enum: "require"`, "write: link-local-nexthop <enable|disable>;"}},
		{"color;", []string{"color needs a value, one of red, blue", "write: color <red|blue>;"}},
		{"color true;", []string{"write: color <red|blue>;"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := NewParser(schema).Parse(tt.input)
			if err == nil {
				t.Fatalf("%q accepted, want a refusal", tt.input)
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("%q refused with %q, want it to contain %q", tt.input, err.Error(), want)
				}
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
