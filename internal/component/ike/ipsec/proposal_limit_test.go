package ipsec

import (
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// plimIKETree builds vpn { ipsec { ike-group g { proposal N { ... } x count } } }.
func plimIKETree(count int) *config.Tree {
	tree := config.NewTree()
	ipsec := tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec")
	group := config.NewTree()
	for n := 1; n <= count; n++ {
		prop := config.NewTree()
		prop.Set("encryption", "aes256")
		prop.Set("hash", "sha256")
		prop.Set("dh-group", "14")
		group.AddListEntry("proposal", strconv.Itoa(n), prop)
	}
	ipsec.AddListEntry("ike-group", "g", group)
	return tree
}

// plimESPTree builds vpn { ipsec { esp-group g { proposal N { ... } x count } } }.
func plimESPTree(count int) *config.Tree {
	tree := config.NewTree()
	ipsec := tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec")
	group := config.NewTree()
	for n := 1; n <= count; n++ {
		prop := config.NewTree()
		prop.Set("encryption", "aes256")
		prop.Set("hash", "sha256")
		group.AddListEntry("proposal", strconv.Itoa(n), prop)
	}
	ipsec.AddListEntry("esp-group", "g", group)
	return tree
}

// VALIDATES: a group of at most 255 proposals parses, and a group of 256 is refused
// with an error naming the limit and the count.
// PREVENTS: silent truncation on the wire. The Proposal Num field is one octet, so a
// 256th proposal has no exact encoding. Ze rejects the config rather than send a
// number that wrapped (ai/rules/exact-or-reject.md).
//
// RFC 7296 Section 3.3.1 gives Proposal Num one octet, and Section 3.3 numbers an
// offer one upward, so 255 is the largest number a conforming offer can carry.
func TestPlimGroupProposalCountIsBounded(t *testing.T) {
	for _, tc := range []struct {
		name  string
		tree  func(int) *config.Tree
		group string
	}{
		{"ike-group", plimIKETree, "ike-group"},
		{"esp-group", plimESPTree, "esp-group"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Positive. The largest group the one-octet field can number parses.
			if _, err := ParseIPsecConfig(tc.tree(MaxProposalsPerGroup)); err != nil {
				t.Fatalf("a group of %d proposals was refused: %v", MaxProposalsPerGroup, err)
			}

			// Negative. One more than the field can number is refused.
			_, err := ParseIPsecConfig(tc.tree(MaxProposalsPerGroup + 1))
			if err == nil {
				t.Fatalf("a group of %d proposals parsed, want a refusal", MaxProposalsPerGroup+1)
			}
			for _, want := range []string{tc.group, "256", "255"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal %q does not name %q", err, want)
				}
			}
		})
	}
}
