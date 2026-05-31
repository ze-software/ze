// Related: parser.go — parseLeaf handles YANG "type empty" presence flags
// Related: schema.go — TypeEmpty value type and ValidateValue
package config

import (
	"strings"
	"testing"
)

// TestParseTypeEmptyLeaf covers YANG "type empty" leaves: presence flags such as
// "no-default-route;". parseLeaf used to demand a value token for every leaf and
// rejected the bare flag with "expected value for ..., got SEMICOLON" (the
// linux-only pppoe-client config failure). The explicit value form
// ("no-default-route true;") is also accepted so serialized output and the
// pre-TypeEmpty value form keep working.
func TestParseTypeEmptyLeaf(t *testing.T) {
	schema := NewSchema()
	schema.Define("no-default-route", &LeafNode{Type: TypeEmpty})

	t.Run("bare flag records presence as true", func(t *testing.T) {
		tree, err := NewParser(schema).Parse("no-default-route;")
		if err != nil {
			t.Fatalf("bare type-empty flag rejected: %v", err)
		}
		v, ok := tree.Get("no-default-route")
		if !ok {
			t.Fatal("no-default-route not recorded in tree")
		}
		if v != configTrue {
			t.Fatalf("presence flag = %q, want %q", v, configTrue)
		}
	})

	t.Run("explicit value form is accepted", func(t *testing.T) {
		tree, err := NewParser(schema).Parse("no-default-route true;")
		if err != nil {
			t.Fatalf("explicit value form rejected: %v", err)
		}
		if v, ok := tree.Get("no-default-route"); !ok || v != configTrue {
			t.Fatalf("explicit value form = %q (ok=%v), want %q", v, ok, configTrue)
		}
	})

	t.Run("absent flag stays unset", func(t *testing.T) {
		tree, err := NewParser(schema).Parse("")
		if err != nil {
			t.Fatalf("empty input rejected: %v", err)
		}
		if _, ok := tree.Get("no-default-route"); ok {
			t.Fatal("absent type-empty flag should not be set")
		}
	})

	t.Run("trailing flag without explicit ; is accepted (ASI)", func(t *testing.T) {
		// The tokenizer inserts a statement terminator at EOF/newline, so a
		// bare flag as the final statement needs no explicit ';'.
		tree, err := NewParser(schema).Parse("no-default-route")
		if err != nil {
			t.Fatalf("ASI-terminated flag rejected: %v", err)
		}
		if v, ok := tree.Get("no-default-route"); !ok || v != configTrue {
			t.Fatalf("ASI flag presence = %q (ok=%v), want %q", v, ok, configTrue)
		}
	})
}

// TestSerializeTypeEmptyRoundTrip ensures a present type-empty leaf serializes
// to a bare presence flag (not "name enable;") and survives a serialize -> parse
// round-trip (config show output must re-load).
func TestSerializeTypeEmptyRoundTrip(t *testing.T) {
	schema := NewSchema()
	schema.Define("no-default-route", &LeafNode{Type: TypeEmpty})

	tree, err := NewParser(schema).Parse("no-default-route;")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	out := Serialize(tree, schema)
	if strings.Contains(out, "enable") || strings.Contains(out, "true") {
		t.Fatalf("type-empty leaf serialized with a value, want bare flag, got: %q", out)
	}
	tree2, err := NewParser(schema).Parse(out)
	if err != nil {
		t.Fatalf("serialized output %q does not re-parse: %v", out, err)
	}
	if v, ok := tree2.Get("no-default-route"); !ok || v != configTrue {
		t.Fatalf("round-trip lost presence: v=%q ok=%v (serialized: %q)", v, ok, out)
	}
}

// TestSerializeTypeEmptyInContainerRoundTrip exercises a type-empty leaf nested
// in a container (the shape of the pppoe-client config) through a full
// serialize -> parse round-trip, including the inline-container emit path.
func TestSerializeTypeEmptyInContainerRoundTrip(t *testing.T) {
	schema := NewSchema()
	opts := &ContainerNode{
		children: map[string]Node{"no-default-route": &LeafNode{Type: TypeEmpty}},
		order:    []string{"no-default-route"},
	}
	schema.Define("options", opts)

	tree, err := NewParser(schema).Parse("options { no-default-route; }")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	out := Serialize(tree, schema)
	tree2, err := NewParser(schema).Parse(out)
	if err != nil {
		t.Fatalf("serialized output %q does not re-parse: %v", out, err)
	}
	sub := tree2.GetContainer("options")
	if sub == nil {
		t.Fatalf("options container lost on round-trip, serialized: %q", out)
	}
	if v, ok := sub.Get("no-default-route"); !ok || v != configTrue {
		t.Fatalf("nested presence lost: v=%q ok=%v (serialized: %q)", v, ok, out)
	}
}

// TestSetParserTypeEmpty verifies the set-style parser accepts a bare type-empty
// flag ("set no-default-route") and records presence as true.
func TestSetParserTypeEmpty(t *testing.T) {
	schema := NewSchema()
	schema.Define("no-default-route", &LeafNode{Type: TypeEmpty})

	tree, err := NewSetParser(schema).Parse("set no-default-route")
	if err != nil {
		t.Fatalf("set-style bare type-empty flag rejected: %v", err)
	}
	if v, ok := tree.Get("no-default-route"); !ok || v != configTrue {
		t.Fatalf("set flag = %q (ok=%v), want %q", v, ok, configTrue)
	}
}

// TestValidateTypeEmpty pins the validation contract for TypeEmpty: presence is
// stored as configTrue and is valid; any other stored value is malformed.
func TestValidateTypeEmpty(t *testing.T) {
	if err := ValidateValue(TypeEmpty, configTrue); err != nil {
		t.Fatalf("presence value %q rejected: %v", configTrue, err)
	}
	if err := ValidateValue(TypeEmpty, "bogus"); err == nil {
		t.Fatal("non-presence value on a type-empty leaf should be rejected")
	}
	if got := TypeEmpty.String(); got != valueTypeEmpty {
		t.Fatalf("TypeEmpty.String() = %q, want %q", got, valueTypeEmpty)
	}
}
