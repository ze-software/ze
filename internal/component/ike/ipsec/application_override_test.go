// Design: docs/guide/ipsec.md -- application override control
// Related: config.go -- parseApplicationOverride, the parser these cases drive

package ipsec

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// TestApplicationOverrideLeafParses proves AC-13: the `application-override` leaf
// parses to its two values, an absent leaf inside `vpn ipsec` is the YANG default
// deny (D8-2), a box with no `vpn ipsec` block is allow, and any other word is
// refused naming both allowed values.
//
// VALIDATES: parseApplicationOverride and EmptyConfig.
// PREVENTS: an unset leaf reaching the apply as the zero value, which names no
// choice, or a mistyped value read as either disposition.
func TestApplicationOverrideLeafParses(t *testing.T) {
	for _, tc := range []struct {
		name    string
		written string // empty: the leaf is absent
		want    ApplicationOverride
	}{
		{"absent", "", ApplicationOverrideDeny},
		{"deny", "deny", ApplicationOverrideDeny},
		{"allow", "allow", ApplicationOverrideAllow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := config.NewTree()
			block := tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec")
			if tc.written != "" {
				block.Set("application-override", tc.written)
			}
			cfg, err := ParseIPsecConfig(tree)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if cfg.ApplicationOverride != tc.want {
				t.Fatalf("application-override = %v, want %v", cfg.ApplicationOverride, tc.want)
			}
		})
	}

	if got := EmptyConfig().ApplicationOverride; got != ApplicationOverrideAllow {
		t.Fatalf("no vpn ipsec block: application-override = %v, want allow", got)
	}

	tree := config.NewTree()
	tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec").Set("application-override", "permit")
	_, err := ParseIPsecConfig(tree)
	if err == nil {
		t.Fatal("parse accepted application-override permit")
	}
	for _, word := range []string{"permit", "deny", "allow"} {
		if !strings.Contains(err.Error(), word) {
			t.Errorf("refusal %q does not name %q", err, word)
		}
	}
}
