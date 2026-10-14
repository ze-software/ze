package filter_family

import (
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// familyFilterSection builds a bgp delivery defining one family-filter
// instance per name, each removing ipv4/flow.
func familyFilterSection(names ...string) []sdk.ConfigSection {
	data := `{"bgp":{"policy":{"family-filter":{`
	for i, name := range names {
		if i > 0 {
			data += ","
		}
		data += `"` + name + `":{"family":"ipv4/flow","action":"remove"}`
	}
	data += `}}}}`
	return []sdk.ConfigSection{{Root: configRootBGP, Data: data}}
}

// installedInstances counts the instances a filter-update would read.
func installedInstances(t *testing.T) int {
	t.Helper()
	instances := instancesByName.Load()
	if instances == nil {
		t.Fatal("no instances installed")
	}
	return len(*instances)
}

// VALIDATES: spec-plugin-reload-applies-or-refuses AC-1 to AC-4 for
// bgp-filter-family. A reload's verify installs nothing, apply installs the
// candidate, rollback restores the boot instances, and a candidate that
// places a tear-down instance in an export chain is refused at verify and
// leaves them.
// PREVENTS: the silent reload: verify only checked the candidate and no apply
// handler existed, so a reload answered OK and kept the startup instances.
// Not parallel: instancesByName is package state.
func TestConfigTxAppliesReloadedFamilyFilters(t *testing.T) {
	t.Cleanup(func() { instancesByName.Store(nil) })
	tx := newFamilyConfigTx()
	if err := tx.Configure(familyFilterSection("A")); err != nil {
		t.Fatalf("configure: %v", err)
	}

	if err := tx.Verify(familyFilterSection("A", "B")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if n := installedInstances(t); n != 1 {
		t.Fatalf("verify installed the candidate: %d instances, want 1 until apply", n)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if n := installedInstances(t); n != 2 {
		t.Fatalf("after apply: %d instances, want 2", n)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if n := installedInstances(t); n != 1 {
		t.Fatalf("after rollback: %d instances, want 1", n)
	}

	tearDownInExport := []sdk.ConfigSection{{Root: configRootBGP, Data: `{"bgp":{
		"policy":{"family-filter":{"Kill":{"family":"ipv4/flow","action":"tear-down"}}},
		"filter":{"export":["bgp-filter-family:Kill"]}}}`}}
	if err := tx.Verify(tearDownInExport); err == nil {
		t.Fatal("verify of a tear-down instance in an export chain: err nil, want a refusal")
	}
	if n := installedInstances(t); n != 1 {
		t.Fatalf("a refused verify changed the instances: %d, want 1", n)
	}
}

// VALIDATES: AC-2 for bgp-filter-family: a reload that removes every instance
// leaves none defined.
// Not parallel: instancesByName is package state.
func TestConfigTxRemovedFamilyFiltersUnconfigure(t *testing.T) {
	t.Cleanup(func() { instancesByName.Store(nil) })
	tx := newFamilyConfigTx()
	if err := tx.Configure(familyFilterSection("A")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.Verify([]sdk.ConfigSection{{Root: configRootBGP, Data: `{"bgp":{}}`}}); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if n := installedInstances(t); n != 0 {
		t.Fatalf("after removal: %d instances, want 0", n)
	}
}
