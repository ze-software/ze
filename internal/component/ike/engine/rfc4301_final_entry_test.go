// VALIDATES: what Ze installs as the final entry of every SPD, from the `vpn ipsec
// unmatched` leaf through ParseIPsecConfig to installUnmatched, the path apply.go runs.
// With the leaf absent the final entry is a BYPASS, a disclosed deviation from the
// RFC 4301 Section 4.4.1 SHOULD ("Every SPD SHOULD have a nominal, final entry that
// matches anything that is otherwise unmatched, and discards it."); with `unmatched
// discard` the final entry is the DISCARD the section asks for.
// PREVENTS: the default silently changing in either direction, and the RFC route
// (`unmatched discard`) installing anything but a final discard entry.

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/test/rfcgap"
)

// finalEntriesFor parses a configuration whose `unmatched` leaf holds value (absent
// when value is empty) and returns the catch-all entries Ze installs for it.
func finalEntriesFor(tb testing.TB, value string) []dataplane.SPParams {
	tb.Helper()
	tree := config.NewTree()
	vpn := tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec")
	if value != "" {
		vpn.Set("unmatched", value)
	}
	cfg, err := ipsec.ParseIPsecConfig(tree)
	if err != nil {
		tb.Fatalf("ParseIPsecConfig(unmatched %q): %v", value, err)
	}
	dp := &bypassDP{}
	if err := installUnmatched(dp, cfg.Unmatched, slogutil.Logger("test")); err != nil {
		tb.Fatalf("install the catch-all: %v", err)
	}
	if len(dp.installed) != 6 {
		tb.Fatalf("unmatched %q: installed %d entries, want 3 directions x 2 families", value, len(dp.installed))
	}
	for _, p := range dp.installed {
		if p.Priority != dataplane.PriorityUnmatched {
			tb.Fatalf("unmatched %q dir %d: priority %d, want the final rank %d",
				value, p.Dir, p.Priority, uint32(dataplane.PriorityUnmatched))
		}
	}
	return dp.installed
}

// TestRFC4301FinalSPDEntryFollowsTheUnmatchedLeaf asserts both dispositions Ze can
// install as the final entry: bypass when the leaf is absent (the default), discard
// when the operator writes `unmatched discard`.
func TestRFC4301FinalSPDEntryFollowsTheUnmatchedLeaf(t *testing.T) {
	for _, p := range finalEntriesFor(t, "") {
		if p.Action != dataplane.SPActionBypass {
			t.Errorf("default dir %d: final entry action %d, want bypass (%d)", p.Dir, p.Action, dataplane.SPActionBypass)
		}
	}
	for _, p := range finalEntriesFor(t, "discard") {
		if p.Action != dataplane.SPActionDiscard {
			t.Errorf("unmatched discard dir %d: final entry action %d, want discard (%d)", p.Dir, p.Action, dataplane.SPActionDiscard)
		}
	}
}

// RFC requirement: RFC4301-4.4.1-3 gap -- with the default configuration the final entry of every SPD discards what nothing else matched.
func TestRFC4301DefaultFinalSPDEntryDiscards(t *testing.T) {
	rfcgap.Demonstrate(t, "RFC4301-4.4.1-3", func(tb testing.TB) {
		for _, p := range finalEntriesFor(tb, "") {
			if p.Action != dataplane.SPActionDiscard {
				tb.Errorf("default dir %d: final entry action %d, want discard (%d)", p.Dir, p.Action, dataplane.SPActionDiscard)
			}
		}
	})
}
