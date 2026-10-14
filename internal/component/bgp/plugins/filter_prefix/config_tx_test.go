package filter_prefix

import (
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// prefixSection builds a bgp delivery defining prefix-list L with one entry
// for 10.0.0.0/8 and the given action.
func prefixSection(entryAction string) []sdk.ConfigSection {
	data := `{"bgp":{"policy":{"prefix-list":{"L":{"entry":{"10.0.0.0/8":{"action":"` +
		entryAction + `"}}}}}}}`
	return []sdk.ConfigSection{{Root: configRootBGP, Data: data}}
}

// installedActionOfL reads the action the running definitions give list L's
// only entry, as a filter-update would.
func installedActionOfL(t *testing.T) action {
	t.Helper()
	listsP := listsByName.Load()
	if listsP == nil {
		t.Fatal("no prefix lists installed")
	}
	list, ok := (*listsP)["L"]
	if !ok {
		t.Fatal("prefix-list L not installed")
	}
	if len(list.entries) != 1 {
		t.Fatalf("L has %d entries, want 1", len(list.entries))
	}
	return list.entries[0].action
}

// VALIDATES: spec-plugin-reload-applies-or-refuses AC-1, AC-3, AC-4 for
// bgp-filter-prefix. A reload's verify installs nothing, apply installs the
// edited entry, rollback restores the boot definition, and a candidate the
// parser refuses leaves the running lists.
// PREVENTS: the silent reload: OnConfigure alone answered every config-apply
// OK and kept the startup lists.
// Not parallel: listsByName is package state.
func TestPrefixConfigTxAppliesAReloadedEntry(t *testing.T) {
	t.Cleanup(func() { listsByName.Store(nil) })
	tx := newPrefixConfigTx()
	if err := tx.Configure(prefixSection("accept")); err != nil {
		t.Fatalf("configure: %v", err)
	}

	if err := tx.Verify(prefixSection("reject")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got := installedActionOfL(t); got != actionAccept {
		t.Fatalf("verify installed the candidate: action %v, want accept until apply", got)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := installedActionOfL(t); got != actionReject {
		t.Fatalf("after apply: action %v, want reject", got)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if got := installedActionOfL(t); got != actionAccept {
		t.Fatalf("after rollback: action %v, want accept", got)
	}

	if err := tx.Verify(prefixSection("maybe")); err == nil {
		t.Fatal("verify of an unknown action: err nil, want a parse refusal")
	}
	if got := installedActionOfL(t); got != actionAccept {
		t.Fatalf("a refused verify changed the lists: action %v, want accept", got)
	}
}

// VALIDATES: AC-2 for bgp-filter-prefix. A reload whose bgp section defines no
// prefix-list removes every list, and a reload that carries no bgp section
// leaves the lists as they are (A-2).
// PREVENTS: a removed list still filtering after the reload.
// Not parallel: listsByName is package state.
func TestPrefixConfigTxRemovalAndAbsentRoot(t *testing.T) {
	t.Cleanup(func() { listsByName.Store(nil) })
	tx := newPrefixConfigTx()
	if err := tx.Configure(prefixSection("accept")); err != nil {
		t.Fatalf("configure: %v", err)
	}

	if err := tx.Verify([]sdk.ConfigSection{{Root: "interface", Data: `{}`}}); err != nil {
		t.Fatalf("verify without bgp: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply without bgp: %v", err)
	}
	if got := installedActionOfL(t); got != actionAccept {
		t.Fatalf("a transaction without bgp changed L: action %v", got)
	}

	if err := tx.Verify([]sdk.ConfigSection{{Root: configRootBGP, Data: `{"bgp":{}}`}}); err != nil {
		t.Fatalf("verify of a removal: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply of a removal: %v", err)
	}
	listsP := listsByName.Load()
	if listsP == nil {
		t.Fatal("removal left no definitions installed, want an empty set")
	}
	if n := len(*listsP); n != 0 {
		t.Fatalf("after removal: %d lists, want 0", n)
	}
}
