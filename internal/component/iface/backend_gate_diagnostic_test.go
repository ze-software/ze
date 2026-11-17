// Design: docs/architecture/core-design.md — commit-time backend capability gate
// Related: register.go — validateBackendGate preserves verifier causes

package iface

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// setBackendGateDiagnosticSchema gives the real gate a small deterministic
// schema, so these questions concern its error propagation rather than any
// platform's interface inventory. It restores the previously initialized cache.
// Not safe for parallel tests: it temporarily replaces the package schema.
func setBackendGateDiagnosticSchema(t *testing.T) {
	t.Helper()
	backendGateSchemaOnce.Do(func() {
		backendGateSchema, backendGateSchemaErr = config.YANGSchema()
	})
	previousSchema := backendGateSchema
	previousError := backendGateSchemaErr
	first := config.Leaf(config.TypeString)
	first.Backend = []string{"netlink"}
	second := config.Leaf(config.TypeString)
	second.Backend = []string{"netlink"}
	schema := config.NewSchema()
	schema.Define(configRootInterface, config.Container(
		config.Field("first", first),
		config.Field("second", second),
	))
	backendGateSchema = schema
	backendGateSchemaErr = nil
	t.Cleanup(func() {
		backendGateSchema = previousSchema
		backendGateSchemaErr = previousError
	})
}

// TestBackendGatePreservesHostVerdict checks the actual empty-backend producer
// through the gate, so offline validation can classify this host refusal.
// MUTATION: rendering the refusal without retaining its cause loses the marker.
func TestBackendGatePreservesHostVerdict(t *testing.T) {
	setBackendGateDiagnosticSchema(t)
	sections := []sdk.ConfigSection{{Root: configRootInterface, Data: `{"interface":{}}`}}
	err := validateBackendGate(sections, "")
	if err == nil {
		t.Fatal("empty backend was accepted")
	}
	if !diagnostic.IsHostVerdict(err) {
		t.Fatalf("empty-backend refusal lost its host classification: %v", err)
	}
	want := "interface commit rejected:\n  backend feature gate: no backend configured at /interface/backend"
	if err.Error() != want {
		t.Fatalf("refusal text changed: got %q, want %q", err.Error(), want)
	}
}

// TestBackendGatePreservesOrdinaryCause checks that malformed JSON remains a
// config refusal with its concrete parser cause, rather than a host refusal.
// MUTATION: retaining only rendered text hides the parser's typed cause.
func TestBackendGatePreservesOrdinaryCause(t *testing.T) {
	setBackendGateDiagnosticSchema(t)
	sections := []sdk.ConfigSection{{Root: configRootInterface, Data: `{"interface":`}}
	err := validateBackendGate(sections, "netlink")
	if err == nil {
		t.Fatal("malformed config JSON was accepted")
	}
	if diagnostic.IsHostVerdict(err) {
		t.Fatalf("ordinary JSON refusal was classified as a host failure: %v", err)
	}
	if _, ok := errors.AsType[*json.SyntaxError](err); !ok {
		t.Fatalf("JSON refusal lost its concrete parse cause: %v", err)
	}
	want := "interface commit rejected:\n  backend feature gate: parse config JSON: unexpected end of JSON input"
	if err.Error() != want {
		t.Fatalf("parse refusal text changed: got %q, want %q", err.Error(), want)
	}
}

// TestBackendGatePreservesEveryCause checks that aggregation keeps both
// independently rejected nodes reachable and renders their ordered messages.
// MUTATION: wrapping only the first cause drops the second node's cause.
func TestBackendGatePreservesEveryCause(t *testing.T) {
	setBackendGateDiagnosticSchema(t)
	sections := []sdk.ConfigSection{{Root: configRootInterface, Data: `{"interface":{"first":"yes","second":"yes"}}`}}
	err := validateBackendGate(sections, "vpp")
	if err == nil {
		t.Fatal("two unsupported nodes were accepted")
	}
	if diagnostic.IsHostVerdict(err) {
		t.Fatalf("feature mismatches were classified as a host failure: %v", err)
	}
	wantCauses := []string{
		"/interface/first: feature not supported by backend \"vpp\" (supported: netlink)",
		"/interface/second: feature not supported by backend \"vpp\" (supported: netlink)",
	}
	want := "interface commit rejected:\n  " + wantCauses[0] + "\n  " + wantCauses[1]
	if err.Error() != want {
		t.Fatalf("aggregate text changed: got %q, want %q", err.Error(), want)
	}
	var aggregate interface{ Unwrap() []error }
	if !errors.As(err, &aggregate) {
		t.Fatalf("aggregate exposes no joined causes: %v", err)
	}
	causes := aggregate.Unwrap()
	if len(causes) != 2 {
		t.Fatalf("aggregate has %d causes, want both rejected nodes", len(causes))
	}
	for index, cause := range causes {
		if cause.Error() != wantCauses[index] {
			t.Fatalf("cause %d = %q, want %q", index, cause.Error(), wantCauses[index])
		}
		if !errors.Is(err, cause) {
			t.Fatalf("aggregate lost reachable cause %v", cause)
		}
	}
}

// TestBackendGateAcceptsSupportedNodes proves that retaining causes does not
// turn supported configuration into a refusal.
func TestBackendGateAcceptsSupportedNodes(t *testing.T) {
	setBackendGateDiagnosticSchema(t)
	sections := []sdk.ConfigSection{{Root: configRootInterface, Data: `{"interface":{"first":"yes","second":"yes"}}`}}
	if err := validateBackendGate(sections, "netlink"); err != nil {
		t.Fatalf("supported nodes were rejected: %v", err)
	}
}

// TestBackendGateIgnoresOtherSections proves that an unrelated section is not
// parsed or classified by the interface gate, even with no active backend.
func TestBackendGateIgnoresOtherSections(t *testing.T) {
	setBackendGateDiagnosticSchema(t)
	sections := []sdk.ConfigSection{{Root: "other-component", Data: "not JSON"}}
	if err := validateBackendGate(sections, ""); err != nil {
		t.Fatalf("unrelated section reached the interface gate: %v", err)
	}
}

// TestBackendGateAcceptsNoSections proves the gate remains a no-op when no
// configuration reaches it, even when this host has no active backend.
func TestBackendGateAcceptsNoSections(t *testing.T) {
	setBackendGateDiagnosticSchema(t)
	if err := validateBackendGate(nil, ""); err != nil {
		t.Fatalf("absent configuration was rejected: %v", err)
	}
}
