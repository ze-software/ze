// Design: docs/architecture/testing/runner-architecture.md -- shared fixture schema publication.

package externalyang

import (
	"slices"
	"testing"
)

// TestConfigDeclarationsRetainFixtureOrigins compares the registered snapshot
// with each owner's exact accepted module and the outcome provider's real name.
func TestConfigDeclarationsRetainFixtureOrigins(t *testing.T) {
	want := []ConfigDeclaration{
		{Plugin: "acme-chrony", Module: Augment()},
		{Plugin: "acme-monitor", Module: Valid()},
		CommandOutcomeWeb(),
	}
	got := ConfigDeclarations()
	if !slices.Equal(got, want) {
		t.Fatalf("registered fixture declarations = %+v, want %+v", got, want)
	}
	if got[2].Plugin != "answer-outcome-provider" {
		t.Errorf("outcome provider = %q, want answer-outcome-provider", got[2].Plugin)
	}
	if got[2].Module.Name != "ze-answer-outcome-web-conf" {
		t.Errorf("outcome module = %q, want ze-answer-outcome-web-conf", got[2].Module.Name)
	}
}

// TestConfigDeclarationRegistryRefusesInvalidInput holds a useful declaration
// beside every refused one, proving a refusal cannot remove or replace it.
func TestConfigDeclarationRegistryRefusesInvalidInput(t *testing.T) {
	base := ConfigDeclaration{Plugin: "fixture", Module: Valid()}
	for _, test := range []struct {
		name   string
		change func(*ConfigDeclaration)
	}{
		{name: "empty origin", change: func(d *ConfigDeclaration) { d.Plugin = "" }},
		{name: "empty name", change: func(d *ConfigDeclaration) { d.Module.Name = "" }},
		{name: "empty namespace", change: func(d *ConfigDeclaration) { d.Module.Namespace = "" }},
		{name: "empty text", change: func(d *ConfigDeclaration) { d.Module.Text = "" }},
		{name: "refused module", change: func(d *ConfigDeclaration) { d.Module.Refused = true }},
		{name: "duplicate name", change: func(d *ConfigDeclaration) { d.Plugin = "other-fixture" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			var registry configDeclarationRegistry
			if err := registry.register(base); err != nil {
				t.Fatalf("register useful control: %v", err)
			}
			refused := base
			test.change(&refused)
			if err := registry.register(refused); err == nil {
				t.Fatal("invalid declaration was accepted")
			}
			if got := registry.snapshot(); !slices.Equal(got, []ConfigDeclaration{base}) {
				t.Errorf("refusal changed the useful declaration: %+v", got)
			}
		})
	}
}

// TestConfigDeclarationSnapshotIsOwned mutates every field in one returned
// declaration, then reads the registry again to exclude shared writable state.
func TestConfigDeclarationSnapshotIsOwned(t *testing.T) {
	base := ConfigDeclaration{Plugin: "fixture", Module: Valid()}
	var registry configDeclarationRegistry
	if err := registry.register(base); err != nil {
		t.Fatal(err)
	}
	snapshot := registry.snapshot()
	snapshot[0].Plugin = "changed"
	snapshot[0].Module.Name = "changed"
	snapshot[0].Module.Namespace = "changed"
	snapshot[0].Module.Text = "changed"
	snapshot[0].Module.Refused = true
	if got := registry.snapshot(); !slices.Equal(got, []ConfigDeclaration{base}) {
		t.Errorf("snapshot mutation changed the declaration: %+v", got)
	}
}
