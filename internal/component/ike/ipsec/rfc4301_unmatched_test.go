// VALIDATES: the `unmatched` leaf under vpn ipsec is the management interface for the
// catch-all disposition of RFC 4301 Section 5: an absent leaf is the bypass default,
// discard is read as DISCARD, and any other word is refused rather than defaulted.

package ipsec

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

func unmatchedTree(value string) *config.Tree {
	tree := config.NewTree()
	ipsec := tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec")
	if value != "" {
		ipsec.Set("unmatched", value)
	}
	return tree
}

// RFC requirement: RFC4301-5-1 positive -- `unmatched discard` is read as the DISCARD
// disposition of the SPD catch-all, and an absent leaf is the bypass default.
func TestRFC4301UnmatchedLeafIsReadAsWritten(t *testing.T) {
	cfg, err := ParseIPsecConfig(unmatchedTree("discard"))
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	if cfg.Unmatched != dataplane.SPActionDiscard {
		t.Fatalf("unmatched discard read as %d, want discard (%d)", cfg.Unmatched, dataplane.SPActionDiscard)
	}
	cfg, err = ParseIPsecConfig(unmatchedTree(""))
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	if cfg.Unmatched != dataplane.SPActionBypass {
		t.Fatalf("absent unmatched read as %d, want bypass (%d)", cfg.Unmatched, dataplane.SPActionBypass)
	}
}

// RFC requirement: RFC4301-5-1 negative -- a word that is neither discard nor bypass
// is refused by name, and never reaches the configuration as a disposition.
func TestRFC4301UnmatchedLeafRefusesAnUnknownWord(t *testing.T) {
	cfg, err := ParseIPsecConfig(unmatchedTree("protect"))
	if err == nil {
		t.Fatalf("unmatched protect was accepted as disposition %d", cfg.Unmatched)
	}
}
