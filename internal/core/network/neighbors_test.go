package network

import (
	"context"
	"errors"
	"net/netip"
	"testing"
)

// TestNeighborLinkLocalsPairing proves the global-to-Link-Local derivation the
// RFC 2545 Section 3 third-party next hop depends on: the global's entry names
// a device and a link-layer address, and the one resolved fe80::/10 entry with
// the same pair is that router's Link-Local. Each miss names its own outcome,
// so a caller can tell "no entry" from "two candidates" from "unresolved".
func TestNeighborLinkLocalsPairing(t *testing.T) {
	const (
		macN = "02:00:00:00:00:09"
		macM = "02:00:00:00:00:08"
		macA = "02:00:00:00:00:0a"
	)
	entries := []Neighbor{
		// N: global and Link-Local on eth0 with one MAC.
		{Address: netip.MustParseAddr("2001:db8:a::9"), LinkAddress: macN, Device: "eth0", Binding: NeighborBindingValid},
		{Address: netip.MustParseAddr("fe80::9"), LinkAddress: macN, Device: "eth0", Binding: NeighborBindingValid},
		// The same MAC on another device is another link, never N's.
		{Address: netip.MustParseAddr("fe80::99"), LinkAddress: macN, Device: "eth1", Binding: NeighborBindingValid},
		// M: a global with no Link-Local entry.
		{Address: netip.MustParseAddr("2001:db8:a::8"), LinkAddress: macM, Device: "eth0", Binding: NeighborBindingValid},
		// An unresolved global.
		{Address: netip.MustParseAddr("2001:db8:a::7"), Device: "eth0", Binding: NeighborBindingInvalid},
		// A: two Link-Locals share one MAC.
		{Address: netip.MustParseAddr("2001:db8:a::a"), LinkAddress: macA, Device: "eth0", Binding: NeighborBindingValid},
		{Address: netip.MustParseAddr("fe80::a"), LinkAddress: macA, Device: "eth0", Binding: NeighborBindingValid},
		{Address: netip.MustParseAddr("fe80::aa"), LinkAddress: macA, Device: "eth0", Binding: NeighborBindingValid},
		// An unresolved Link-Local is never a candidate.
		{Address: netip.MustParseAddr("2001:db8:a::6"), LinkAddress: "02:00:00:00:00:06", Device: "eth0", Binding: NeighborBindingValid},
		{Address: netip.MustParseAddr("fe80::6"), LinkAddress: "02:00:00:00:00:06", Device: "eth0", Binding: NeighborBindingInvalid},
	}
	links := NewNeighborLinkLocals(entries)

	cases := []struct {
		global  string
		want    string
		outcome LinkLocalOutcome
	}{
		{global: "2001:db8:a::9", want: "fe80::9", outcome: LinkLocalFound},
		{global: "2001:db8:a::8", outcome: LinkLocalNoCandidate},
		{global: "2001:db8:a::7", outcome: LinkLocalGlobalUnbound},
		{global: "2001:db8:a::a", outcome: LinkLocalAmbiguous},
		{global: "2001:db8:a::6", outcome: LinkLocalNoCandidate},
		{global: "2001:db8:a::5", outcome: LinkLocalNoGlobalEntry},
	}
	for _, tc := range cases {
		got, outcome := links.LinkLocalOf(netip.MustParseAddr(tc.global))
		if outcome != tc.outcome {
			t.Errorf("%s: outcome %v, want %v", tc.global, outcome, tc.outcome)
		}
		if tc.want == "" {
			if got.IsValid() {
				t.Errorf("%s: address %v with outcome %v, want none", tc.global, got, outcome)
			}
			continue
		}
		if got != netip.MustParseAddr(tc.want) {
			t.Errorf("%s: Link-Local %v, want %s", tc.global, got, tc.want)
		}
	}
}

// TestNeighborLinkLocalsOneGlobalTwoRouters proves a global seen on two
// devices with different link-layer addresses is ambiguous, not the first or
// last router read.
func TestNeighborLinkLocalsOneGlobalTwoRouters(t *testing.T) {
	entries := []Neighbor{
		{Address: netip.MustParseAddr("2001:db8:a::9"), LinkAddress: "02:00:00:00:00:01", Device: "eth0", Binding: NeighborBindingValid},
		{Address: netip.MustParseAddr("fe80::1"), LinkAddress: "02:00:00:00:00:01", Device: "eth0", Binding: NeighborBindingValid},
		{Address: netip.MustParseAddr("2001:db8:a::9"), LinkAddress: "02:00:00:00:00:02", Device: "eth1", Binding: NeighborBindingValid},
		{Address: netip.MustParseAddr("fe80::2"), LinkAddress: "02:00:00:00:00:02", Device: "eth1", Binding: NeighborBindingValid},
	}
	got, outcome := NewNeighborLinkLocals(entries).LinkLocalOf(netip.MustParseAddr("2001:db8:a::9"))
	if outcome != LinkLocalAmbiguous || got.IsValid() {
		t.Fatalf("got %v %v, want no address and %v", got, outcome, LinkLocalAmbiguous)
	}
}

// TestNeighborLinkLocalsEqual proves Equal compares answers, which is what the
// reactor's refresh skip relies on.
func TestNeighborLinkLocalsEqual(t *testing.T) {
	one := []Neighbor{
		{Address: netip.MustParseAddr("2001:db8:a::9"), LinkAddress: "02:00:00:00:00:09", Device: "eth0", Binding: NeighborBindingValid},
		{Address: netip.MustParseAddr("fe80::9"), LinkAddress: "02:00:00:00:00:09", Device: "eth0", Binding: NeighborBindingValid},
	}
	changed := []Neighbor{one[0], {Address: netip.MustParseAddr("fe80::19"), LinkAddress: "02:00:00:00:00:09", Device: "eth0", Binding: NeighborBindingValid}}
	if !NewNeighborLinkLocals(one).Equal(NewNeighborLinkLocals(one)) {
		t.Fatal("identical tables compare unequal")
	}
	if NewNeighborLinkLocals(one).Equal(NewNeighborLinkLocals(changed)) {
		t.Fatal("a changed Link-Local compares equal")
	}
	var none *NeighborLinkLocals
	if !none.Equal(nil) || none.Equal(NewNeighborLinkLocals(one)) {
		t.Fatal("nil pairing comparison is wrong")
	}
}

// TestNeighborTableUnregistered proves a build with no neighbor table says so
// rather than answering an empty table, or a watch that never signals.
func TestNeighborTableUnregistered(t *testing.T) {
	neighborTable.Lock()
	savedRead, savedWatch := neighborTable.read, neighborTable.watch
	neighborTable.read, neighborTable.watch = nil, nil
	neighborTable.Unlock()
	t.Cleanup(func() {
		neighborTable.Lock()
		neighborTable.read, neighborTable.watch = savedRead, savedWatch
		neighborTable.Unlock()
	})
	entries, err := ReadNeighbors()
	if !errors.Is(err, ErrNeighborTableNotRegistered) || entries != nil {
		t.Fatalf("read: got %v, %v; want nil and ErrNeighborTableNotRegistered", entries, err)
	}
	changes, err := WatchNeighbors(context.Background())
	if !errors.Is(err, ErrNeighborTableNotRegistered) || changes != nil {
		t.Fatalf("watch: got %v, %v; want nil and ErrNeighborTableNotRegistered", changes, err)
	}
}
