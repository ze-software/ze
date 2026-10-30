// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package filter_family

import (
	"encoding/json"
	"slices"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestConfigTxUnchangedSubtreeIsNoop proves AC-5 for bgp-filter-family: a reload that
// edits bgp outside the family-filter instances installs nothing, so every filter call keeps
// reading the very state it read before, and the rollback of that
// transaction installs nothing either. Method: the stored pointer is compared
// across a verify, apply and rollback whose candidate differs from the
// running config only in another peer's description.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-5 for bgp-filter-family.
// PREVENTS: a shared-root edit rebuilding the filter's state, which would
// also fail when the plugin's parsed config stops comparing deeply equal.
// MUTATION: delete the reflect.DeepEqual short-circuit in sdk.ConfigTx.Apply
// and the apply check fails.
// Not parallel: instancesByName is package state.
func TestConfigTxUnchangedSubtreeIsNoop(t *testing.T) {
	t.Cleanup(func() { instancesByName.Store(nil) })
	tx := newFamilyConfigTx()
	if err := tx.Configure(withOtherPeer(t, familyFilterSection("A"), "first")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	installed := instancesByName.Load()
	if installed == nil {
		t.Fatal("configure installed nothing")
	}

	if err := tx.Verify(withOtherPeer(t, familyFilterSection("A"), "second")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if instancesByName.Load() != installed {
		t.Fatal("an edit outside the family-filter instances installed a new config")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if instancesByName.Load() != installed {
		t.Fatal("the rollback of an edit outside the family-filter instances installed a config")
	}
}

// withOtherPeer returns sections with a bgp delivery added beside the policy
// one, holding peer "other": a peer no filter names, carrying description, so
// two calls that differ only in description differ only in the bgp root the
// plugin reads for its export-chain check, never in its own subtree.
func withOtherPeer(t *testing.T, sections []sdk.ConfigSection, description string) []sdk.ConfigSection {
	t.Helper()
	tree := map[string]any{configRootBGP: map[string]any{
		"peer": map[string]any{"other": map[string]any{
			"connection":  map[string]any{"remote": map[string]any{"ip": "192.0.2.99"}},
			"description": description,
		}},
	}}
	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return append(slices.Clone(sections), sdk.ConfigSection{Root: configRootBGP, Data: string(data)})
}
