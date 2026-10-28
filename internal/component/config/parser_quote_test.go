// Design: docs/architecture/config/syntax.md — quoted values.

package config

import (
	"reflect"
	"strings"
	"testing"
)

// TestParserRefusesUnterminatedQuote checks that a quote with no closing
// quote is a syntax error naming the line it opens on. The tokenizer used to
// end the string at the end of the input, so `bgp peer A000"0000` parsed
// and `ze config fmt` printed a config that lowered to another map
// (FuzzFormatRoundTrip). Method: each input opens a quote it never closes,
// in a key, a leaf value and a comment-free block, on the line the error
// must name.
func TestParserRefusesUnterminatedQuote(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}
	for _, tc := range []struct {
		name  string
		input string
		line  string
	}{
		{"glued to a list key", "bgp peer A000\"000000000000", "line 1:"},
		{"after a list key", "bgp peer A000 \"000000000000", "line 1:"},
		{"leaf value", "bgp {\n\trouter-id \"192.0.2.1\n}\n", "line 2:"},
		{"single quote", "bgp {\n\n\trouter-id '192.0.2.1;\n}\n", "line 3:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewParser(schema).Parse(tc.input)
			if err == nil {
				t.Fatalf("unterminated quote accepted: %q", tc.input)
			}
			for _, want := range []string{tc.line, "unterminated", "closing"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q lacks %q", err, want)
				}
			}
		})
	}
}

// TestParserRefusesInlineValueOnContainer checks the terminated form of the
// fuzz finding. `peer A000 "x"` is a one-line list entry whose value fell on
// peer's last child, `attach`, a container: the tree stored the text under
// the container's name, no printer read it, and the formatted config lowered
// to another map. Method: each input must be refused naming the child and the
// block form; an input the parser accepts must still round-trip.
func TestParserRefusesInlineValueOnContainer(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}
	for _, tc := range []struct{ input, child string }{
		{"bgp peer A000\"000000000000\"", "attach"}, // first positional slot
		{"bgp peer A000 \"000000000000\"", "attach"},
		{"bgp peer A000 [ x ]", "update"}, // a bracket list goes to the last child
	} {
		input := tc.input
		_, err := NewParser(schema).Parse(input)
		if err == nil {
			t.Errorf("value on a container child accepted: %q", input)
			continue
		}
		for _, want := range []string{"line 1:", "gives", tc.child + ", which holds no value", "write the entry as a block: peer A000 { " + tc.child} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q lacks %q", err, want)
			}
		}
	}
	for _, input := range []string{
		"bgp { peer A000 { } }",
	} {
		source, err := NewParser(schema).Parse(input)
		if err != nil {
			t.Fatalf("%q refused: %v", input, err)
		}
		formatted := Serialize(source, schema)
		reparsed, err := NewParser(schema).Parse(formatted)
		if err != nil {
			t.Fatalf("formatted text of %q does not parse: %v\n%s", input, err, formatted)
		}
		if !reflect.DeepEqual(source.ToMap(), reparsed.ToMap()) {
			t.Errorf("formatted text of %q lowers to another map\nsource %v\nformatted %v\n%s",
				input, source.ToMap(), reparsed.ToMap(), formatted)
		}
	}
}
