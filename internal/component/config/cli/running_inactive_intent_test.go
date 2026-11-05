// Design: docs/architecture/config/syntax.md -- public active running configuration.
package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestShowConfigRunningExcludesInternalInactiveIntent reaches the production
// command with a running projection. Active data remains present; neither the
// inactive member nor internal metadata is exposed in the JSON response.
func TestShowConfigRunningExcludesInternalInactiveIntent(t *testing.T) {
	root, policy := config.NewTree(), config.NewTree()
	root.SetContainer("policy", policy)
	policy.SetMemberStates("filters", []config.MemberState{{Value: "operator:keep"}, {Value: "automatic:off", Inactive: true}})
	coordinator := plugin.NewCoordinator(root.ToRunningMap())
	response := dispatchRunning(t, coordinator)
	if response.Status != plugin.StatusDone {
		t.Fatal(response.Error)
	}
	body, err := json.Marshal(response.Data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "operator:keep") {
		t.Fatalf("public response lost the active member: %s", body)
	}
	for _, private := range []string{"!inactive-members", "automatic:off"} {
		if strings.Contains(string(body), private) {
			t.Errorf("public response leaked %s: %s", private, body)
		}
	}
}
