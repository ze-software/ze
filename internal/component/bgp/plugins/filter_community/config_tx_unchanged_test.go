// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package filter_community

import (
	"encoding/json"
	"reflect"
	"testing"
	"unsafe"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// installedIdentity is where the installed definitions, per-peer configs and
// blackhole agreements live: an install replaces all three, so a changed
// address proves an install ran.
type installedIdentity struct {
	definitions unsafe.Pointer
	peerConfigs unsafe.Pointer
	agreements  *blackholeAgreements
}

func installedNow() installedIdentity {
	mu.RLock()
	defer mu.RUnlock()
	return installedIdentity{
		definitions: reflect.ValueOf(definitions).UnsafePointer(),
		peerConfigs: reflect.ValueOf(peerConfigs).UnsafePointer(),
		agreements:  agreements,
	}
}

// TestConfigTxUnchangedSubtreeIsNoop proves AC-5 for bgp-filter-community: a
// reload that edits bgp outside the community definitions, the peers'
// community filters and their blackhole agreements installs nothing, and the
// rollback of that transaction installs nothing either. Method: the addresses
// of the three installed values are compared across a verify, apply and
// rollback whose candidate differs from the running config only in another
// peer's description.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-5 for bgp-filter-community.
// PREVENTS: a shared-root edit rebuilding the filter's state, which would
// also fail when communityConfig stops comparing deeply equal.
// MUTATION: delete the reflect.DeepEqual short-circuit in sdk.ConfigTx.Apply
// and the apply check fails.
// Not parallel: definitions, peerConfigs and agreements are package state.
func TestConfigTxUnchangedSubtreeIsNoop(t *testing.T) {
	t.Cleanup(func() {
		if err := installCommunityConfig(communityConfig{}); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	tx := newCommunityConfigTx()
	if err := tx.Configure(withOtherPeer(t, tagSection("65000:1"), "first")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	installed := installedNow()
	if installed.definitions == nil {
		t.Fatal("configure installed no definitions")
	}
	if installed.peerConfigs == nil {
		t.Fatal("configure installed no peer configs")
	}

	if err := tx.Verify(withOtherPeer(t, tagSection("65000:1"), "second")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if installedNow() != installed {
		t.Fatal("an edit outside the community config installed a new config")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if installedNow() != installed {
		t.Fatal("the rollback of an edit outside the community config installed a config")
	}
}

// withOtherPeer returns sections with peer "other" added to the bgp subtree:
// a peer no filter names, carrying description, so two calls that differ only
// in description differ only outside the plugin's subtree.
func withOtherPeer(t *testing.T, sections []sdk.ConfigSection, description string) []sdk.ConfigSection {
	t.Helper()
	var tree map[string]any
	if err := json.Unmarshal([]byte(sections[0].Data), &tree); err != nil {
		t.Fatalf("section data: %v", err)
	}
	bgpCfg, ok := tree["bgp"].(map[string]any)
	if !ok {
		t.Fatal("section data holds no bgp object")
	}
	peers, ok := bgpCfg["peer"].(map[string]any)
	if !ok {
		peers = map[string]any{}
		bgpCfg["peer"] = peers
	}
	peers["other"] = map[string]any{
		"connection":  map[string]any{"remote": map[string]any{"ip": "192.0.2.99"}},
		"description": description,
	}
	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return []sdk.ConfigSection{{Root: sections[0].Root, Data: string(data)}}
}
