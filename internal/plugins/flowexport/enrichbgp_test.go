package flowexport

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/ribevents"
	"codeberg.org/thomas-mangin/ze/internal/plugins/flowexport/enrich"
)

// TestBGPEnrichBuilderApplyAndRebuild verifies that best-change batches fold
// into the prefix table and that a rebuild publishes a tree the Enricher can
// look up by longest-prefix-match.
func TestBGPEnrichBuilderApplyAndRebuild(t *testing.T) {
	en := enrich.NewEnricher()
	b := newBGPEnrichBuilder(en)
	defer b.Stop()

	nh := netip.MustParseAddr("10.0.0.254")
	b.applyBatch(&ribevents.BestChangeBatch{
		Changes: []ribevents.BestChangeEntry{
			{
				Action:   ribevents.BestChangeAdd,
				Prefix:   netip.MustParsePrefix("192.0.2.0/24"),
				NextHop:  nh,
				OriginAS: 64512,
				ASPath:   []uint32{65001, 65002, 64512},
			},
		},
	})

	// Before rebuild the published tree is still empty.
	if _, ok := en.Lookup(netip.MustParseAddr("192.0.2.5")); ok {
		t.Fatal("lookup succeeded before rebuild; tree should be empty")
	}

	b.rebuild()

	entry, ok := en.Lookup(netip.MustParseAddr("192.0.2.5"))
	if !ok {
		t.Fatal("lookup failed after rebuild; expected 192.0.2.0/24 match")
	}
	if entry.NextHop != nh {
		t.Errorf("next-hop = %v, want %v", entry.NextHop, nh)
	}
	if entry.AS != 64512 {
		t.Errorf("origin AS = %d, want 64512", entry.AS)
	}
	if len(entry.ASPath) != 3 || entry.ASPath[2] != 64512 {
		t.Errorf("AS path = %v, want [65001 65002 64512]", entry.ASPath)
	}
}

// TestBGPEnrichBuilderWithdraw verifies a withdrawal removes a prefix.
func TestBGPEnrichBuilderWithdraw(t *testing.T) {
	en := enrich.NewEnricher()
	b := newBGPEnrichBuilder(en)
	defer b.Stop()

	pfx := netip.MustParsePrefix("198.51.100.0/24")
	b.applyBatch(&ribevents.BestChangeBatch{
		Changes: []ribevents.BestChangeEntry{
			{Action: ribevents.BestChangeAdd, Prefix: pfx, NextHop: netip.MustParseAddr("10.0.0.1")},
		},
	})
	b.rebuild()
	if _, ok := en.Lookup(netip.MustParseAddr("198.51.100.7")); !ok {
		t.Fatal("prefix not present after add")
	}

	b.applyBatch(&ribevents.BestChangeBatch{
		Changes: []ribevents.BestChangeEntry{
			{Action: ribevents.BestChangeWithdraw, Prefix: pfx},
		},
	})
	b.rebuild()
	if _, ok := en.Lookup(netip.MustParseAddr("198.51.100.7")); ok {
		t.Fatal("prefix still present after withdraw")
	}
}

// TestBGPEnrichBuilderRebuildNotDirty verifies rebuild is a no-op when no
// changes have been applied (the published tree pointer is unchanged).
func TestBGPEnrichBuilderRebuildNotDirty(t *testing.T) {
	en := enrich.NewEnricher()
	b := newBGPEnrichBuilder(en)
	defer b.Stop()

	b.rebuild() // not dirty: must not panic, must leave an empty tree
	if _, ok := en.Lookup(netip.MustParseAddr("203.0.113.1")); ok {
		t.Fatal("unexpected lookup hit on empty tree")
	}
}
