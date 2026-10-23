// Design: docs/architecture/config/syntax.md — corrective parser diagnostics.

package config

import (
	"slices"
	"strings"
	"testing"
)

// TestSchemaRefusalCarriesOriginalPath exercises a keyed ancestor and checks
// schema-derived corrections without pinning the entire error message.
func TestSchemaRefusalCarriesOriginalPath(t *testing.T) {
	policy := &ContainerNode{
		Presence: true,
		children: map[string]Node{
			"filters": &BracketLeafListNode{Type: TypeString},
			"final":   &LeafNode{Type: TypeString, Enums: []string{"accept", "reject"}},
		},
		order: []string{"filters", "final"},
	}
	schema := NewSchema()
	schema.Define("peer", &ListNode{
		KeyType:  TypeString,
		children: map[string]Node{"policy": policy},
		order:    []string{"policy"},
	})
	for _, tc := range []struct {
		input string
		path  string
		fix   string
	}{
		{"peer edge { policy [ one ]; }", "peer/edge/policy", "policy { filters [ VALUE ]; final <accept|reject>; }"},
		{"peer edge { policy { final permit; } }", "peer/edge/policy/final", "final <accept|reject>;"},
	} {
		_, err := NewParser(schema).Parse(tc.input)
		if err == nil {
			t.Fatalf("invalid input accepted: %s", tc.input)
		}
		for _, want := range []string{tc.path, "write:", tc.fix} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal %q lacks %q", err, want)
			}
		}
	}
	if _, err := NewParser(schema).Parse("peer edge { policy { filters [ one ]; final reject; } }"); err != nil {
		t.Fatalf("positive control refused: %v", err)
	}
	_, err := NewParser(schema).ParseAt("policy [ one ];", []string{"peer", "edge"})
	if err == nil {
		t.Fatal("invalid relative fragment accepted")
	}
	if !strings.Contains(err.Error(), "peer/edge/policy") {
		t.Fatalf("relative refusal lost editor context: %v", err)
	}
	for _, input := range []string{
		"set peer edge policy [ one ]",
		"set peer edge policy final permit",
	} {
		_, plainErr := NewSetParser(schema).Parse(input)
		_, _, metaErr := NewSetParser(schema).ParseWithMeta(input)
		for _, refusal := range []error{plainErr, metaErr} {
			if refusal == nil {
				t.Fatalf("invalid set input accepted: %s", input)
			}
			for _, want := range []string{"peer/edge/policy", "write:", "accept|reject"} {
				if !strings.Contains(refusal.Error(), want) {
					t.Errorf("set refusal %q lacks %q", refusal, want)
				}
			}
		}
	}
}

// TestTreeValidatorRegistrationRefusesInvalidHandlers checks programmer errors
// without adding callbacks to the process-global registry.
func TestTreeValidatorRegistrationRefusesInvalidHandlers(t *testing.T) {
	if err := RegisterTreeValidator("", func(*Tree) error { return nil }); err == nil {
		t.Fatal("unnamed tree validator registered")
	}
	if err := RegisterTreeValidator("invalid-nil-handler", nil); err == nil {
		t.Fatal("nil tree validator registered")
	}
}

// TestTreeValidatorRegistrationRejectsDuplicate exercises the registry's error
// boundary, independently of whether any production component registered itself.
func TestTreeValidatorRegistrationRejectsDuplicate(t *testing.T) {
	name := t.Name()
	if err := RegisterTreeValidator(name, func(*Tree) error { return nil }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		treeValidators.Lock()
		defer treeValidators.Unlock()
		for i, registered := range treeValidators.functions {
			if registered.name == name {
				treeValidators.functions = slices.Delete(treeValidators.functions, i, i+1)
				return
			}
		}
	})
	if err := RegisterTreeValidator(name, func(*Tree) error { return nil }); err == nil {
		t.Fatal("duplicate tree validator registration accepted")
	}
}
