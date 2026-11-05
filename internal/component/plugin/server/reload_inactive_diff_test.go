// Design: docs/architecture/config/transaction-protocol.md -- public config diff payloads.
package server

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// TestRunningIntentNeverEntersPublicDiffBodies covers whole added/removed
// subtrees and both sides of a changed pair. Each public body retains its active
// member, but private metadata and disabled members must not cross the transport.
func TestRunningIntentNeverEntersPublicDiffBodies(t *testing.T) {
	before := inactiveReloadTree("automatic:a").ToRunningMap()
	after := inactiveReloadTree("automatic:b").ToRunningMap()
	cases := []struct {
		name string
		diff *config.ConfigDiff
	}{
		{"added", &config.ConfigDiff{Added: map[string]any{"bgp": after["bgp"]}}},
		{"removed", &config.ConfigDiff{Removed: map[string]any{"bgp": before["bgp"]}}},
		{"both changed sides", &config.ConfigDiff{Changed: map[string]config.DiffPair{
			"bgp": {Old: before["bgp"], New: after["bgp"]},
		}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sections := buildDiffSections(tc.diff, []string{"bgp"})
			if len(sections) != 1 || sections[0].Root != "bgp" {
				t.Fatalf("public diff lost root participation: %v", sections)
			}
			section := sections[0]
			body := section.Added + section.Removed + section.Changed
			if !strings.Contains(body, "operator:keep") {
				t.Errorf("public diff lost active member: %s", body)
			}
			for _, private := range []string{"!inactive-members", "automatic:a", "automatic:b"} {
				if strings.Contains(body, private) {
					t.Errorf("public diff leaked %s: %s", private, body)
				}
			}
		})
	}
}
