package filter_irr

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/resolve/irr"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve/peeringdb"
)

// VALIDATES: AC-2 -- prefix in IRR list accepted.
// VALIDATES: AC-3 -- prefix NOT in IRR list rejected.
// VALIDATES: AC-13 -- aggregated prefixes preserved from IRR.
// PREVENTS: IRR-to-prefix-list conversion drops or corrupts entries.

func TestPrefixListFromIRR(t *testing.T) {
	pl := irr.PrefixList{
		IPv4: []netip.Prefix{
			netip.MustParsePrefix("10.0.0.0/24"),
			netip.MustParsePrefix("172.16.0.0/16"),
		},
		IPv6: []netip.Prefix{
			netip.MustParsePrefix("2001:db8::/32"),
		},
	}

	entries := prefixListFromIRR(pl)

	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(entries))
	}

	if entries[0].prefix != netip.MustParsePrefix("10.0.0.0/24") {
		t.Errorf("entry[0].prefix = %s, want 10.0.0.0/24", entries[0].prefix)
	}
	if entries[0].ge != 24 || entries[0].le != 32 {
		t.Errorf("entry[0] ge=%d le=%d, want ge=24 le=32", entries[0].ge, entries[0].le)
	}

	if entries[2].prefix != netip.MustParsePrefix("2001:db8::/32") {
		t.Errorf("entry[2].prefix = %s, want 2001:db8::/32", entries[2].prefix)
	}
	if entries[2].ge != 32 || entries[2].le != 128 {
		t.Errorf("entry[2] ge=%d le=%d, want ge=32 le=128", entries[2].ge, entries[2].le)
	}
}

func TestPrefixListFromIRRAcceptReject(t *testing.T) {
	pl := irr.PrefixList{
		IPv4: []netip.Prefix{
			netip.MustParsePrefix("10.0.0.0/24"),
			netip.MustParsePrefix("10.0.1.0/24"),
		},
	}
	entries := prefixListFromIRR(pl)
	list := &irrPrefixList{entries: entries}

	if !list.evaluateUpdate("ipv4/unicast add 10.0.0.0/24") {
		t.Error("matching prefix should be accepted")
	}
	if list.evaluateUpdate("ipv4/unicast add 192.168.0.0/24") {
		t.Error("non-matching prefix should be rejected")
	}
}

// VALIDATES: AC-4 -- empty list rejects all (fail-closed).
func TestPrefixListFromIRREmpty(t *testing.T) {
	entries := prefixListFromIRR(irr.PrefixList{})
	list := &irrPrefixList{entries: entries}

	if list.evaluateUpdate("ipv4/unicast add 10.0.0.0/24") {
		t.Error("empty IRR result should reject all")
	}
}

// VALIDATES: AC-2, AC-3 -- filter name -> ASN extraction.
func TestExtractASNFromFilter(t *testing.T) {
	tests := []struct {
		filter string
		want   uint32
	}{
		{"bgp-filter-irr:65001", 65001},
		{"65001", 65001},
		{"bgp-filter-irr:", 0},
		{"", 0},
		{"bgp-filter-irr:abc", 0},
	}
	for _, tt := range tests {
		got := extractASNFromFilter(tt.filter)
		if got != tt.want {
			t.Errorf("extractASNFromFilter(%q) = %d, want %d", tt.filter, got, tt.want)
		}
	}
}

// VALIDATES: when the AS-SET cannot be determined, `update bgp irr asn` fails
// (returns statusError) and leaves the existing prefix-list untouched (no
// configuration/state change). PREVENTS: a failed refresh wiping last-known-good
// data and silently reporting success.
func TestUpdateASNFailurePreservesState(t *testing.T) {
	existing := &irrPrefixList{entries: prefixListFromIRR(irr.PrefixList{
		IPv4: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})}
	plug := &irrPlugin{
		byASN: map[uint32]*asnState{
			65010: {asn: 65010, asSet: "", list: existing},
		},
		// Unreachable endpoints so AS-SET discovery fails fast (refused, local).
		pdbClient: peeringdb.NewPeeringDB("http://127.0.0.1:1"),
		irrClient: irr.NewIRR("127.0.0.1:1"),
	}

	status, _, err := plug.updateASN([]string{"65010"})
	if status != statusError || err == nil {
		t.Fatalf("updateASN status=%q err=%v, want statusError + non-nil err", status, err)
	}

	plug.mu.RLock()
	got := plug.byASN[65010].list
	lastErr := plug.byASN[65010].lastErr
	plug.mu.RUnlock()
	if got != existing {
		t.Error("prefix-list was replaced on a failed refresh; want last-known-good preserved")
	}
	if lastErr == "" {
		t.Error("lastErr not recorded for a failed refresh")
	}
}

// VALIDATES: `update bgp irr asn <asn>` for an ASN with no IRR-filtered peer
// fails rather than silently reporting success.
func TestUpdateASNUnknownASN(t *testing.T) {
	plug := &irrPlugin{byASN: map[uint32]*asnState{}}
	status, _, err := plug.updateASN([]string{"65099"})
	if status != statusError || err == nil {
		t.Fatalf("updateASN unknown ASN status=%q err=%v, want statusError + non-nil err", status, err)
	}
}

// VALIDATES: `update bgp irr as-set <as-set>` for an AS-SET used by no peer fails.
func TestUpdateASSetUnused(t *testing.T) {
	plug := &irrPlugin{byASN: map[uint32]*asnState{
		65010: {asn: 65010, asSet: "AS-OTHER"},
	}}
	status, _, err := plug.updateASSet([]string{"AS-NOBODY"})
	if status != statusError || err == nil {
		t.Fatalf("updateASSet unused AS-SET status=%q err=%v, want statusError + non-nil err", status, err)
	}
}

// VALIDATES: a mixed UPDATE (one in-list prefix, one out-of-list) is partitioned
// so only the in-list prefix is kept and the modify delta carries just that
// subset. PREVENTS: a single unauthorized prefix collaterally dropping the
// legitimate routes that share the same UPDATE (all-or-nothing reject).
func TestPartitionUpdateMixed(t *testing.T) {
	list := &irrPrefixList{entries: prefixListFromIRR(irr.PrefixList{
		IPv4: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})}
	p := list.partitionUpdate("ipv4/unicast add 10.0.0.0/24 192.168.0.0/24")
	if len(p.accepted) != 1 || p.accepted[0] != "10.0.0.0/24" {
		t.Errorf("accepted = %v, want [10.0.0.0/24]", p.accepted)
	}
	if len(p.rejected) != 1 || p.rejected[0] != "192.168.0.0/24" {
		t.Errorf("rejected = %v, want [192.168.0.0/24]", p.rejected)
	}
	if got := buildModifyDelta(p); got != "nlri ipv4/unicast add 10.0.0.0/24" {
		t.Errorf("delta = %q, want %q", got, "nlri ipv4/unicast add 10.0.0.0/24")
	}
}

// VALIDATES: an UPDATE whose prefixes are all out-of-list yields an empty
// accepted set (whole-update reject), and an all-in-list UPDATE yields no
// rejected entries (accept unmodified).
func TestPartitionUpdateAllOrNone(t *testing.T) {
	list := &irrPrefixList{entries: prefixListFromIRR(irr.PrefixList{
		IPv4: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})}
	allBad := list.partitionUpdate("ipv4/unicast add 192.168.0.0/24 203.0.113.0/24")
	if len(allBad.accepted) != 0 || len(allBad.rejected) != 2 {
		t.Errorf("all-out-of-list: accepted=%v rejected=%v, want 0 accepted / 2 rejected", allBad.accepted, allBad.rejected)
	}
	allGood := list.partitionUpdate("ipv4/unicast add 10.0.0.0/24")
	if len(allGood.accepted) != 1 || len(allGood.rejected) != 0 {
		t.Errorf("all-in-list: accepted=%v rejected=%v, want 1 accepted / 0 rejected", allGood.accepted, allGood.rejected)
	}
}
