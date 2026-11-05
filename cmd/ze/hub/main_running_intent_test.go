// Design: docs/architecture/config/syntax.md -- running intent and provider active data.
package hub

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// TestHubRunningProjectionKeepsIntentOutOfProvider drives both real hub map
// producers: boot lowering and provider refresh. Internal running reconstruction
// retains inactive root/nested members, while provider roots, watcher data and
// snapshots remain active-only, including a root-level inactive leaf-list.
func TestHubRunningProjectionKeepsIntentOutOfProvider(t *testing.T) {
	root, policy := config.NewTree(), config.NewTree()
	root.SetContainer("policy", policy)
	root.SetMemberStates("root-members", []config.MemberState{{Value: "root-live"}, {Value: "root-off", Inactive: true}})
	policy.SetMemberStates("filters", []config.MemberState{{Value: "operator:keep"}, {Value: "automatic:off", Inactive: true}})
	provider := config.NewProvider()
	running := lowerForPlugins(root, provider)
	restored, err := config.TreeFromPluginMap(running)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.GetMultiValuesState("root-members"), root.GetMultiValuesState("root-members")) {
		t.Error("hub running map lost root inactive intent")
	}
	if !reflect.DeepEqual(restored.GetContainer("policy").GetMultiValuesState("filters"), policy.GetMultiValuesState("filters")) {
		t.Error("hub running map lost nested inactive intent")
	}
	checkProvider := func() {
		t.Helper()
		roots := provider.Roots()
		if !reflect.DeepEqual(roots, []string{"policy"}) {
			t.Errorf("provider roots include private metadata: %v", roots)
		}
		snapshot, err := snapshotProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "operator:keep") {
			t.Errorf("provider lost active data: %s", body)
		}
		for _, private := range []string{"!inactive-members", "root-off", "automatic:off"} {
			if strings.Contains(string(body), private) {
				t.Errorf("provider snapshot leaked %s: %s", private, body)
			}
		}
	}
	checkProvider()
	applyLoadedTreeToProvider(provider, running)
	checkProvider()
}
