// Design: docs/architecture/config/syntax.md — corrective parser diagnostics.

package config

import (
	"slices"
	"strings"
	"testing"
)

// TestSchemaRefusalCarriesOriginalPath exercises a keyed ancestor and checks
// schema-derived corrections without pinning the entire error message. A
// presence container given a value keeps its slash path and write: line; an
// enumeration leaf's refusal names the same path, keyed ancestor included, in
// the words the operator types, and its fix: line is the corrected line.
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
		{"peer edge { policy [ one ]; }", "peer/edge/policy", "write: policy { filters [ VALUE ]; final <accept|reject>; }"},
		{"peer edge { policy { final permit; } }", "peer edge policy final: \"permit\" is not an accepted value", "\nfix: final <accept|reject>;"},
	} {
		_, err := NewParser(schema).Parse(tc.input)
		if err == nil {
			t.Fatalf("invalid input accepted: %s", tc.input)
		}
		for _, want := range []string{tc.path, tc.fix} {
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
	for input, wants := range map[string][]string{
		"set peer edge policy [ one ]":      {"peer/edge/policy", "write:", "accept|reject"},
		"set peer edge policy final permit": {"peer edge policy final: \"permit\" is not an accepted value", "\nfix: set peer edge policy final <accept|reject>"},
	} {
		_, plainErr := NewSetParser(schema).Parse(input)
		_, _, metaErr := NewSetParser(schema).ParseWithMeta(input)
		for _, refusal := range []error{plainErr, metaErr} {
			if refusal == nil {
				t.Fatalf("invalid set input accepted: %s", input)
			}
			for _, want := range wants {
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
