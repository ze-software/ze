// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package filter_community_match

import (
	"encoding/json"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestConfigTxUnchangedSubtreeIsNoop proves AC-5 for bgp-filter-community-match: a reload that
// edits the policy root outside the community-match definitions installs nothing, so every filter call keeps
// reading the very state it read before, and the rollback of that
// transaction installs nothing either. Method: the stored pointer is compared
// across a verify, apply and rollback whose candidate differs from the
// running config only in a sibling list under the policy root.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-5 for bgp-filter-community-match.
// PREVENTS: a shared-root edit rebuilding the filter's state, which would
// also fail when the plugin's parsed config stops comparing deeply equal.
// MUTATION: delete the reflect.DeepEqual short-circuit in sdk.ConfigTx.Apply
// and the apply check fails.
// Not parallel: listsByName is package state.
func TestConfigTxUnchangedSubtreeIsNoop(t *testing.T) {
	t.Cleanup(func() { listsByName.Store(nil) })
	tx := newCommunityMatchConfigTx()
	if err := tx.Configure(withOtherPolicyList(t, definitionsSection("A"), "first")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	installed := listsByName.Load()
	if installed == nil {
		t.Fatal("configure installed nothing")
	}

	if err := tx.Verify(withOtherPolicyList(t, definitionsSection("A"), "second")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if listsByName.Load() != installed {
		t.Fatal("an edit outside the community-match definitions installed a new config")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if listsByName.Load() != installed {
		t.Fatal("the rollback of an edit outside the community-match definitions installed a config")
	}
}

// withOtherPolicyList returns sections with an entry of another filter type added
// to the policy root: config no list of this plugin holds, carrying
// description, so two calls that differ only in description differ only
// outside the plugin's subtree.
func withOtherPolicyList(t *testing.T, sections []sdk.ConfigSection, description string) []sdk.ConfigSection {
	t.Helper()
	var tree map[string]any
	if err := json.Unmarshal([]byte(sections[0].Data), &tree); err != nil {
		t.Fatalf("section data: %v", err)
	}
	policyCfg, ok := tree["policy"].(map[string]any)
	if !ok {
		t.Fatal("section data holds no policy object")
	}
	policyCfg["other-filter-type"] = map[string]any{
		"other": map[string]any{"description": description},
	}
	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return []sdk.ConfigSection{{Root: sections[0].Root, Data: string(data)}}
}
