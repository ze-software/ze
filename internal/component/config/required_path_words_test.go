// Design: docs/architecture/config/syntax.md -- ze:required enforcement.
// Related: required.go -- CheckRequired

package config

import (
	"slices"
	"testing"
)

// TestRequiredViolationKeepsAncestorKeys proves a missing descendant keeps
// every list instance in its fix path; a slash in a key stays one token.
func TestRequiredViolationKeepsAncestorKeys(t *testing.T) {
	schema := NewSchema()
	schema.Define("probe", Container(
		Field("site", List(TypeString,
			Field("target", listWithRequired(
				[][]string{{"address"}},
				Field("address", Leaf(TypeString)),
			)),
		)),
	))
	data := map[string]any{"probe": map[string]any{
		"site": map[string]any{"edge/west": map[string]any{
			"target": map[string]any{"one": map[string]any{}},
		}},
	}}
	violations := CheckRequired(schema, data)
	if len(violations) != 1 {
		t.Fatalf("violations = %v, want one", violations)
	}
	want := []string{"probe", "site", "edge/west", "target", "one", "address"}
	if !slices.Equal(violations[0].Path, want) {
		t.Fatalf("path = %q, want %q", violations[0].Path, want)
	}
	if violations[0].SetHint != "set probe site edge/west target one address <value>" {
		t.Fatalf("set hint lost the ancestor key: %s", violations[0].SetHint)
	}
}

// TestRequiredViolationExpandsCompositeEntry checks the path and printable hint
// of a required field on an entry addressed by two YANG key leaves.
func TestRequiredViolationExpandsCompositeEntry(t *testing.T) {
	target := listWithRequired([][]string{{"address"}},
		Field("name", Leaf(TypeString)), Field("second", Leaf(TypeString)),
		Field("address", Leaf(TypeString)),
	)
	target.KeyName = "name second"
	schema := NewSchema()
	schema.Define("target", target)
	tree, err := NewParser(schema).Parse("target alpha { second beta; }")
	if err != nil {
		t.Fatal(err)
	}
	violations := CheckRequired(schema, tree.ToMap())
	if len(violations) != 1 {
		t.Fatalf("violations = %v, want one", violations)
	}
	want := []string{"target", "alpha", "second", "beta", "address"}
	if !slices.Equal(violations[0].Path, want) {
		t.Errorf("path = %q, want %q", violations[0].Path, want)
	}
	if violations[0].SetHint != "set target alpha second beta address <value>" {
		t.Errorf("set hint = %q", violations[0].SetHint)
	}
}

// TestRequiredViolationExpandsCompositeAncestor checks a nested required field
// keeps each key leaf of its ancestor as a separate command word.
func TestRequiredViolationExpandsCompositeAncestor(t *testing.T) {
	site := List(TypeString,
		Field("name", Leaf(TypeString)), Field("second", Leaf(TypeString)),
		Field("target", listWithRequired([][]string{{"address"}},
			Field("address", Leaf(TypeString)),
		)),
	)
	site.KeyName = "name second"
	schema := NewSchema()
	schema.Define("site", site)
	tree, err := NewParser(schema).Parse("site alpha { second beta; target one {} }")
	if err != nil {
		t.Fatal(err)
	}
	violations := CheckRequired(schema, tree.ToMap())
	if len(violations) != 1 {
		t.Fatalf("violations = %v, want one", violations)
	}
	want := []string{"site", "alpha", "second", "beta", "target", "one", "address"}
	if !slices.Equal(violations[0].Path, want) {
		t.Errorf("path = %q, want %q", violations[0].Path, want)
	}
	if violations[0].SetHint != "set site alpha second beta target one address <value>" {
		t.Errorf("set hint = %q", violations[0].SetHint)
	}
}

// TestRequiredViolationQuotesSingleKey checks a single key containing a space
// stays one path word and is quoted in the standalone set hint.
func TestRequiredViolationQuotesSingleKey(t *testing.T) {
	schema := NewSchema()
	schema.Define("target", listWithRequired([][]string{{"address"}},
		Field("address", Leaf(TypeString)),
	))
	tree, err := NewParser(schema).Parse(`target "edge routers" {}`)
	if err != nil {
		t.Fatal(err)
	}
	violations := CheckRequired(schema, tree.ToMap())
	if len(violations) != 1 {
		t.Fatalf("violations = %v, want one", violations)
	}
	want := []string{"target", "edge routers", "address"}
	if !slices.Equal(violations[0].Path, want) {
		t.Errorf("path = %q, want %q", violations[0].Path, want)
	}
	if violations[0].SetHint != `set target "edge routers" address <value>` {
		t.Errorf("set hint = %q", violations[0].SetHint)
	}
}

// TestRequiredViolationQuotesLiteralPipe checks a literal pipe key stays one
// path word and is quoted as data in the standalone command hint.
func TestRequiredViolationQuotesLiteralPipe(t *testing.T) {
	schema := NewSchema()
	schema.Define("target", listWithRequired([][]string{{"address"}},
		Field("address", Leaf(TypeString)),
	))
	tree, err := NewParser(schema).Parse(`target "|" {}`)
	if err != nil {
		t.Fatal(err)
	}
	violations := CheckRequired(schema, tree.ToMap())
	if len(violations) != 1 {
		t.Fatalf("violations = %v, want one", violations)
	}
	want := []string{"target", "|", "address"}
	if !slices.Equal(violations[0].Path, want) {
		t.Errorf("path = %q, want %q", violations[0].Path, want)
	}
	if violations[0].SetHint != `set target "|" address <value>` {
		t.Errorf("set hint = %q", violations[0].SetHint)
	}
}

// TestRequiredViolationPreservesNumericSuffix reads parsed entry identities
// directly, so a literal #N in a key cannot become a positional display suffix.
func TestRequiredViolationPreservesNumericSuffix(t *testing.T) {
	single := listWithRequired([][]string{{"address"}}, Field("address", Leaf(TypeString)))
	target := listWithRequired([][]string{{"address"}},
		Field("name", Leaf(TypeString)), Field("second", Leaf(TypeString)),
		Field("address", Leaf(TypeString)),
	)
	target.KeyName = "name second"
	site := List(TypeString,
		Field("name", Leaf(TypeString)), Field("second", Leaf(TypeString)),
		Field("target", single),
	)
	site.KeyName = "name second"
	schema := NewSchema()
	schema.Define("single", single)
	schema.Define("target", target)
	schema.Define("single-site", List(TypeString, Field("target", single)))
	schema.Define("site", site)
	for _, tc := range []struct {
		name    string
		content string
		path    []string
		hint    string
	}{
		{
			name: "single entry", content: `single "edge#3" {}`,
			path: []string{"single", "edge#3", "address"},
			hint: `set single "edge#3" address <value>`,
		},
		{
			name: "composite entry", content: `target alpha { second "beta#3"; }`,
			path: []string{"target", "alpha", "second", "beta#3", "address"},
			hint: `set target alpha second "beta#3" address <value>`,
		},
		{
			name: "single ancestor", content: `single-site "edge#3" { target one {} }`,
			path: []string{"single-site", "edge#3", "target", "one", "address"},
			hint: `set single-site "edge#3" target one address <value>`,
		},
		{
			name: "composite ancestor", content: `site alpha { second "beta#3"; target one {} }`,
			path: []string{"site", "alpha", "second", "beta#3", "target", "one", "address"},
			hint: `set site alpha second "beta#3" target one address <value>`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := NewParser(schema).Parse(tc.content)
			if err != nil {
				t.Fatal(err)
			}
			violations := CheckRequired(schema, tree.ToMap())
			if len(violations) != 1 {
				t.Fatalf("violations = %v, want one", violations)
			}
			if !slices.Equal(violations[0].Path, tc.path) {
				t.Errorf("path = %q, want %q", violations[0].Path, tc.path)
			}
			if violations[0].SetHint != tc.hint {
				t.Errorf("set hint = %q, want %q", violations[0].SetHint, tc.hint)
			}
		})
	}
}
