// Design: docs/guide/bfd.md -- a peer's bfd profile is checked at commit
//
// The goal is that a peer naming a BFD profile the candidate config does not
// define, or one its hop mode may not use, is refused at commit. Before, the
// commit succeeded and EnsureSession refused the session at start, so a
// non-strict peer ran with no BFD and a log line as the only trace. The method
// runs the commit check over a bgp tree and the bfd section BGP reads.
package reactor

import (
	"strings"
	"testing"

	// The bfd plugin registers the real profile check (api.CheckProfile).
	_ "github.com/ze-software/ze/internal/component/bfd"
)

const commitBFDSection = `{"bfd":{"profile":{"fast":{"detect-multiplier":"5"},` +
	`"quiet":{"passive":"true"},"echoing":{"echo":{}}}}}`

func TestPeerBFDProfileCheckedAtCommit(t *testing.T) {
	cases := []struct {
		name    string
		bfd     map[string]any
		section string
		refusal string
	}{
		{"defined profile", map[string]any{"enabled": "true", "profile": "fast"}, commitBFDSection, ""},
		{"no profile named", map[string]any{"enabled": "true"}, "", ""},
		{"bfd disabled on the peer", map[string]any{"enabled": "false", "profile": "typo"}, commitBFDSection, ""},
		{"undefined profile", map[string]any{"enabled": "true", "profile": "typo"}, commitBFDSection, `profile "typo" is not defined`},
		{"no bfd section", map[string]any{"enabled": "true", "profile": "fast"}, "", "the config has no bfd section"},
		{"passive single-hop", map[string]any{"enabled": "true", "profile": "quiet"}, commitBFDSection, `profile "quiet" sets passive`},
		{"passive multi-hop", map[string]any{"enabled": "true", "profile": "quiet", "mode": "multi-hop"}, commitBFDSection, ""},
		{"echo multi-hop", map[string]any{"enabled": "true", "profile": "echoing", "mode": "multi-hop"}, commitBFDSection, `profile "echoing" enables echo`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyPeerBFDProfiles(bfdStrictTree(tc.bfd), tc.section)
			if tc.refusal == "" {
				if err != nil {
					t.Fatalf("commit refused a usable profile: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.refusal) {
				t.Fatalf("commit error = %v, want %q", err, tc.refusal)
			}
			if !strings.Contains(err.Error(), "192.0.2.1") {
				t.Errorf("commit error = %v, want the peer named", err)
			}
		})
	}
}
