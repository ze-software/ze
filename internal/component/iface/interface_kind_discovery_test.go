// Design: docs/architecture/iface/logical-name-resolution.md -- schema-owned interface kinds

package iface

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/yang"
)

// Goal: a kind added by the current schema reaches registered completion and
// commit validation without a second kind declaration. Method: warm the source,
// register a list and a reference leaf, then ask the actual reference walker.
func TestConfiguredInterfaceReferenceDiscoversRegisteredKind(t *testing.T) {
	root := map[string]any{"interface": map[string]any{
		"ethernet": map[string]any{"eth0": map[string]any{}},
		"probe-port": map[string]any{"probe0": map[string]any{}},
		"probe-targets": map[string]any{"destination": "probe0"},
	}}
	if got := configuredInterfaceReference(root, nil).Names; !slices.Equal(got, []string{"eth0"}) {
		t.Fatalf("before registration names = %v", got)
	}
	t.Cleanup(yang.RegisterModuleForTest("ze-probe-interface-kind.yang", `module ze-probe-interface-kind {
    yang-version 1.1;
    namespace "urn:ze:probe:interface-kind";
    prefix probe;
    import ze-iface-conf { prefix iface; }
    import ze-extensions { prefix ze; }
    augment "/iface:interface" {
        list probe-port {
            key "name";
            description "An interface kind registered for the reference-source test.";
            ze:help "Test interface kind.";
            leaf name {
                type string;
                description "The declared test interface name.";
                ze:help "Test interface name.";
            }
        }
        container probe-targets {
            description "A reference consumer that is not itself an interface kind.";
            ze:help "Test reference consumer.";
            leaf destination {
                type string;
                ze:reference "configured-interface";
                description "The interface the test consumer names.";
                ze:help "Test destination interface.";
            }
        }
    }
}`))
	validator, err := config.YANGValidator()
	if err != nil {
		t.Fatal(err)
	}
	path := []string{"interface", "probe-targets", "destination"}
	if got := validator.ReferenceInstances(root, path); !slices.Equal(got, []string{"eth0", "probe0"}) {
		t.Fatalf("registered completion = %v", got)
	}
	if failures := validator.ValidateReferences(root); len(failures) != 0 {
		t.Fatalf("declared registered-kind target was refused: %v", failures)
	}
	root["interface"].(map[string]any)["probe-targets"].(map[string]any)["destination"] = "not-declared"
	if failures := validator.ValidateReferences(root); len(failures) != 1 {
		t.Fatalf("undeclared target failures = %v, want one refusal", failures)
	}
}

// Goal: unavailable schema knowledge cannot be presented as an empty or valid
// interface set. Method: warm the source, register an invalid extension, then
// require the actual reference judge to retain the loader's error and fix.
func TestConfiguredInterfaceReferenceRetainsSchemaFailure(t *testing.T) {
	root := map[string]any{"interface": map[string]any{
		"ethernet": map[string]any{"eth0": map[string]any{}},
	}}
	if err := configuredInterfaceReference(root, nil).Check("eth0"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(yang.RegisterModuleForTest("ze-probe-interface-kind-error.yang", `module ze-probe-interface-kind-error {
    namespace "urn:ze:probe:interface-kind-error";
    prefix probe;
    import ze-extensions { prefix ze; }
    leaf probe {
        type string;
        ze:hepl "Invalid extension for the failure test.";
    }
}`))
	answer := configuredInterfaceReference(root, nil)
	if len(answer.Names) != 0 {
		t.Errorf("unreadable schema offered unverified names: %v", answer.Names)
	}
	err := answer.Check("eth0")
	if !errors.Is(err, yang.ErrUndeclaredExtension) {
		t.Fatalf("reference judge = %v, want the schema failure", err)
	}
	if !strings.Contains(err.Error(), "ze:hepl") {
		t.Errorf("refusal lost the producing extension: %v", err)
	}
	if !strings.Contains(err.Error(), "ze doctor") {
		t.Errorf("refusal has no operator fix: %v", err)
	}
}
