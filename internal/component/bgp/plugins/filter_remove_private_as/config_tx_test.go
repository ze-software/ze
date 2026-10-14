package filter_remove_private_as

import (
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// definitionsSection builds a bgp delivery defining one remove-private-as entry per
// name.
func definitionsSection(names ...string) []sdk.ConfigSection {
	data := `{"bgp":{"policy":{"remove-private-as":{`
	for i, name := range names {
		if i > 0 {
			data += ","
		}
		data += `"` + name + `":{}`
	}
	data += `}}}}`
	return []sdk.ConfigSection{{Root: configRootBGP, Data: data}}
}

// installedDefinitions counts the definitions a filter-update would read.
func installedDefinitions(t *testing.T) int {
	t.Helper()
	defs := defsByName.Load()
	if defs == nil {
		t.Fatal("no definitions installed")
	}
	return len(*defs)
}

// VALIDATES: spec-plugin-reload-applies-or-refuses AC-1 to AC-4 for
// bgp-filter-remove-private-as. A reload's verify installs nothing, apply installs the
// candidate, rollback restores the boot definitions, and a candidate that is
// not JSON is refused and leaves them.
// PREVENTS: the silent reload: OnConfigure alone answered every config-apply
// OK and kept the startup definitions.
// Not parallel: defsByName is package state.
func TestConfigTxAppliesReloadedDefinitions(t *testing.T) {
	t.Cleanup(func() { defsByName.Store(nil) })
	tx := newRemovePrivateASConfigTx()
	if err := tx.Configure(definitionsSection("A")); err != nil {
		t.Fatalf("configure: %v", err)
	}

	if err := tx.Verify(definitionsSection("A", "B")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if n := installedDefinitions(t); n != 1 {
		t.Fatalf("verify installed the candidate: %d definitions, want 1 until apply", n)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if n := installedDefinitions(t); n != 2 {
		t.Fatalf("after apply: %d definitions, want 2", n)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if n := installedDefinitions(t); n != 1 {
		t.Fatalf("after rollback: %d definitions, want 1", n)
	}

	if err := tx.Verify([]sdk.ConfigSection{{Root: configRootBGP, Data: "{not json"}}); err == nil {
		t.Fatal("verify of invalid JSON: err nil, want a refusal")
	}
	if n := installedDefinitions(t); n != 1 {
		t.Fatalf("a refused verify changed the definitions: %d, want 1", n)
	}
}
