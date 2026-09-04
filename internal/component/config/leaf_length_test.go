package config

import (
	"strings"
	"testing"
)

// TestValidateLeafValueLength covers the boundary of a `certificate` leaf: a
// PKI store name is 1..255 characters.
//
// The constraint is declared in YANG (`type string { length "1..255"; }`) on
// the three certificate leaves (web, as112, geodns) and on ten leaves that
// predate them (mrt, geodns, ddos/flowtriq, exabgp bridge). Until this test,
// the schema carried patterns and numeric ranges but no string length at all,
// so every one of those declarations was decorative: a 256-character value
// validated clean.
func TestValidateLeafValueLength(t *testing.T) {
	node := &LeafNode{
		Type:     TypeString,
		Lengths:  []NumericRange{{Min: "1", Max: "255"}},
		Patterns: []string{"[A-Za-z0-9._-]+"},
	}

	t.Run("last valid length is accepted", func(t *testing.T) {
		if err := ValidateLeafValue(node, strings.Repeat("a", 255)); err != nil {
			t.Fatalf("255 characters must be accepted: %v", err)
		}
	})

	t.Run("one over the maximum is rejected", func(t *testing.T) {
		err := ValidateLeafValue(node, strings.Repeat("a", 256))
		if err == nil {
			t.Fatal("256 characters must be rejected")
		}
		if !strings.Contains(err.Error(), "length") {
			t.Fatalf("error %q does not explain the length constraint", err)
		}
	})

	t.Run("empty is rejected below the minimum", func(t *testing.T) {
		if err := ValidateLeafValue(node, ""); err == nil {
			t.Fatal("an empty value must be rejected by length 1..255")
		}
	})

	t.Run("a leaf with no length constraint is unbounded", func(t *testing.T) {
		free := &LeafNode{Type: TypeString}
		if err := ValidateLeafValue(free, strings.Repeat("a", 4096)); err != nil {
			t.Fatalf("an unconstrained leaf must accept any length: %v", err)
		}
	})

	t.Run("length is measured in characters, not bytes", func(t *testing.T) {
		// YANG length counts characters (RFC 7950 Section 9.4.4). A 3-character
		// multi-byte value must not be rejected by a 1..4 constraint just
		// because its UTF-8 encoding is longer.
		short := &LeafNode{Type: TypeString, Lengths: []NumericRange{{Min: "1", Max: "4"}}}
		if err := ValidateLeafValue(short, "héé"); err != nil {
			t.Fatalf("a 3-character value must fit 1..4: %v", err)
		}
	})
}

// TestCertificateLeafLengthFromYANG proves the constraint reaches the built
// schema from the YANG module, not merely that the validator can enforce one.
func TestCertificateLeafLengthFromYANG(t *testing.T) {
	// A schema that does not build is a defect, not a reason to stop asking.
	// This used to skip, which would have retired the check with no test red
	// and nothing said (ai/rules/principles.md).
	schema, err := YANGSchema()
	if err != nil {
		t.Fatalf("the YANG schema must build: %v", err)
	}
	node := lookupLeaf(t, schema.Get("environment"), "web", "certificate")
	if len(node.Lengths) == 0 {
		t.Fatal("environment.web.certificate must carry its YANG length constraint")
	}
	if node.Lengths[0].Min != "1" || node.Lengths[0].Max != "255" {
		t.Fatalf("length = %v, want 1..255", node.Lengths)
	}
	if err := ValidateLeafValue(node, strings.Repeat("a", 256)); err == nil {
		t.Fatal("a 256-character certificate name must be rejected by the built schema")
	}
}

func lookupLeaf(t *testing.T, root Node, path ...string) *LeafNode {
	t.Helper()
	current := root
	for i, seg := range path {
		container, ok := current.(*ContainerNode)
		if !ok {
			t.Fatalf("%v: %s is not a container", path[:i], seg)
		}
		child := container.Get(seg)
		if child == nil {
			t.Fatalf("%v: no child %q", path[:i], seg)
		}
		current = child
	}
	leaf, ok := current.(*LeafNode)
	if !ok {
		t.Fatalf("%v is not a leaf", path)
	}
	return leaf
}

// TestCertificateLeafLengthFromYANGLG proves the looking-glass certificate leaf
// carries the same constraint as the web one, from the YANG module rather than
// from Go. The two leaves name the same kind of value, a PKI store key, so a
// name one listener accepts and the other refuses would be a trap.
func TestCertificateLeafLengthFromYANGLG(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatalf("the YANG schema must build: %v", err)
	}
	node := lookupLeaf(t, schema.Get("environment"), "looking-glass", "certificate")
	if len(node.Lengths) == 0 {
		t.Fatal("environment.looking-glass.certificate must carry its YANG length constraint")
	}
	if node.Lengths[0].Min != "1" || node.Lengths[0].Max != "255" {
		t.Fatalf("length = %v, want 1..255", node.Lengths)
	}
	if err := ValidateLeafValue(node, strings.Repeat("a", 256)); err == nil {
		t.Fatal("a 256-character certificate name must be rejected by the built schema")
	}
	if err := ValidateLeafValue(node, "lan/../etc"); err == nil {
		t.Fatal("the YANG pattern must refuse a path separator in a store name")
	}
}
