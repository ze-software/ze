package config

import (
	"strings"
	"testing"
)

// TestUnknownKeyNamesClosestValidKey proves every parser path that refuses an
// unknown key names the closest valid key at that position.
//
// VALIDATES: the hierarchical parser at the root, inside a container and
// inside a list entry, ParseAt at a context, and the set parser, each say
// "did you mean <key>?" for a near miss, and say nothing for a name close to
// no key.
// PREVENTS: an operator who mistyped one letter being told only that the key
// is unknown, with no way to learn the spelling (AC-5, and
// docs/contributing/ze-go-style.md, "Fail on an unknown key and suggest the
// closest valid one").
func TestUnknownKeyNamesClosestValidKey(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}

	cases := []struct {
		name    string
		parse   func() error
		want    string
		notWant string
	}{
		{
			name: "root",
			parse: func() error {
				_, err := NewParser(schema).Parse("bgpp {\n}\n")
				return err
			},
			want: "did you mean bgp?",
		},
		{
			name: "container",
			parse: func() error {
				_, err := NewParser(schema).Parse("bgp {\n  router-idd 1.2.3.4;\n}\n")
				return err
			},
			want: "did you mean router-id?",
		},
		{
			name: "list entry",
			parse: func() error {
				_, err := NewParser(schema).Parse("bgp {\n  peer peer1 {\n    timerx {\n    }\n  }\n}\n")
				return err
			},
			want: "did you mean timer?",
		},
		{
			name: "context",
			parse: func() error {
				_, err := NewParser(schema).ParseAt("timerx {\n}\n", []string{"bgp", "peer", "peer1"})
				return err
			},
			want: "did you mean timer?",
		},
		{
			name: "set",
			parse: func() error {
				_, err := NewSetParser(schema).Parse("set bgp router-idd 1.2.3.4\n")
				return err
			},
			want: "did you mean router-id?",
		},
		{
			name: "far from every key",
			parse: func() error {
				_, err := NewParser(schema).Parse("bgp {\n  zzzzzzzzzzzzzzzz 1;\n}\n")
				return err
			},
			notWant: "did you mean",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.parse()
			if err == nil {
				t.Fatal("an unknown key parsed")
			}
			msg := err.Error()
			if c.want != "" && !strings.Contains(msg, c.want) {
				t.Errorf("error %q does not contain %q", msg, c.want)
			}
			if c.notWant != "" && strings.Contains(msg, c.notWant) {
				t.Errorf("error %q suggests a key for a name close to none", msg)
			}
		})
	}
}
