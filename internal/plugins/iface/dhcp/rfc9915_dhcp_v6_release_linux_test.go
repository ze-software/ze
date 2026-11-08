//go:build linux

package ifacedhcp

import (
	"net/netip"
	"testing"
	"time"
)

// VALIDATES: RFC 9915 Section 6.3 (RFC9915-6.3-3), spec AC-12: the Release a
// client sends at shutdown leaves out each delegated prefix a LAN unit took
// space from, keeps the addresses and the prefixes no unit took, and a lease
// left with nothing to release sends no Release.
// METHOD: filter a lease through releasableLease with a chosen in-use answer.
// PREVENTS: releasing a /56 while LAN hosts still hold addresses from it.
func TestRFC9915NoReleaseWhileDelegated(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	used := netip.MustParsePrefix("2001:db8:aa00::/56")
	spare := netip.MustParsePrefix("2001:db8:bb00::/56")
	address := netip.MustParsePrefix("2001:db8:ff::5/128")
	binding := func(kind v6Kind, p netip.Prefix) v6Binding {
		return v6Binding{kind: kind, prefix: p, preferred: time.Hour, valid: 2 * time.Hour, received: now}
	}
	lease := &v6Lease{bindings: []v6Binding{
		binding(v6KindAddress, address),
		binding(v6KindPrefix, used),
		binding(v6KindPrefix, spare),
	}}
	inUse := func(p netip.Prefix) bool { return p == used }

	release := releasableLease(lease, inUse)
	prefixes := release.live(v6KindPrefix, now)
	if len(prefixes) != 1 || prefixes[0].prefix != spare {
		t.Fatalf("released prefixes = %v, want only %s", prefixes, spare)
	}
	if got := release.live(v6KindAddress, now); len(got) != 1 {
		t.Fatalf("released addresses = %v, want the one address", got)
	}
	if len(lease.bindings) != 3 {
		t.Fatal("the lease itself must not change")
	}

	onlyUsed := &v6Lease{bindings: []v6Binding{binding(v6KindPrefix, used)}}
	none := releasableLease(onlyUsed, inUse)
	if len(none.live(v6KindPrefix, now)) != 0 || len(none.live(v6KindAddress, now)) != 0 {
		t.Fatal("a lease whose only prefix is in use must leave nothing to release")
	}

	all := releasableLease(lease, func(netip.Prefix) bool { return false })
	if len(all.live(v6KindPrefix, now)) != 2 {
		t.Fatal("with nothing in use every prefix is released")
	}
}
