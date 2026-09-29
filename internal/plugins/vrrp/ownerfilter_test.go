// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- the address owner's physical-MAC answers
package vrrp

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
)

// TestOwnerFilterTablesDropParentAnswersForOwnedAddresses checks the two tables
// the owner filter builds. The IPv4 owned address gets an arp-family output
// rule dropping a reply that leaves the parent with it as sender; the IPv6 one
// gets an ip6 output rule dropping an advertisement that leaves the parent for
// it. The firewall validator accepts both, so the kernel apply is not where
// they fail.
func TestOwnerFilterTablesDropParentAnswersForOwnedAddresses(t *testing.T) {
	v4 := netip.MustParseAddr("192.0.2.254")
	v6 := netip.MustParseAddr("2001:db8::254")
	tables := ownerFilterTables([]ownerFilterEntry{{parent: "eth0", vips: []netip.Addr{v6, v4}}})
	if len(tables) != 2 {
		t.Fatalf("tables = %+v, want one arp and one ip6 table", tables)
	}
	if err := firewall.ValidateTables(tables); err != nil {
		t.Fatalf("owner filter tables refused by the firewall validator: %v", err)
	}

	arp := tables[0]
	if arp.Name != ownerFilterARPTableName || arp.Family != firewall.FamilyARP {
		t.Fatalf("first table = %s/%s, want %s/arp", arp.Name, arp.Family, ownerFilterARPTableName)
	}
	wantARP := []firewall.Match{
		firewall.MatchOutputInterface{Name: "eth0"},
		firewall.MatchARPOperation{Operation: firewall.ARPOperationReply},
		firewall.MatchARPSenderAddress{Addr: v4},
	}
	assertOwnerChain(t, arp, wantARP)

	nd := tables[1]
	if nd.Name != ownerFilterNDTableName || nd.Family != firewall.FamilyIP6 {
		t.Fatalf("second table = %s/%s, want %s/ip6", nd.Name, nd.Family, ownerFilterNDTableName)
	}
	wantND := []firewall.Match{
		firewall.MatchOutputInterface{Name: "eth0"},
		firewall.MatchICMPv6Type{Type: icmpv6TypeNeighborAdvert},
		firewall.MatchNDTargetAddress{Addr: v6},
	}
	assertOwnerChain(t, nd, wantND)
}

// assertOwnerChain checks that tbl holds one output base chain, accepting by
// policy, whose single term carries want and drops.
func assertOwnerChain(t *testing.T, tbl firewall.Table, want []firewall.Match) {
	t.Helper()
	if len(tbl.Chains) != 1 {
		t.Fatalf("%s chains = %d, want 1", tbl.Name, len(tbl.Chains))
	}
	ch := tbl.Chains[0]
	if !ch.IsBase || ch.Hook != firewall.HookOutput || ch.Policy != firewall.PolicyAccept {
		t.Fatalf("%s chain = %+v, want an output base chain with policy accept", tbl.Name, ch)
	}
	if len(ch.Terms) != 1 {
		t.Fatalf("%s terms = %+v, want 1", tbl.Name, ch.Terms)
	}
	term := ch.Terms[0]
	if !slices.Equal(term.Matches, want) {
		t.Errorf("%s matches = %+v, want %+v", tbl.Name, term.Matches, want)
	}
	if len(term.Actions) != 1 || term.Actions[0] != (firewall.Drop{}) {
		t.Errorf("%s actions = %+v, want drop", tbl.Name, term.Actions)
	}
}

// TestOwnerFilterPublishesAndWithdraws drives set and clear through the publish
// seam: an owner's entry publishes its tables, an empty address list publishes
// nothing, and the clear withdraws the tables.
func TestOwnerFilterPublishesAndWithdraws(t *testing.T) {
	var published [][]firewall.Table
	saved := ownerFilterPublish
	ownerFilterPublish = func(tables []firewall.Table) error {
		published = append(published, tables)
		return nil
	}
	t.Cleanup(func() { ownerFilterPublish = saved })

	if err := setOwnerFilter("vrrp:test-none", "eth0", nil); err != nil {
		t.Fatal(err)
	}
	if len(published) != 0 {
		t.Fatalf("a group that owns nothing published %d times, want 0", len(published))
	}
	vips := []netip.Addr{netip.MustParseAddr("192.0.2.254")}
	if err := setOwnerFilter("vrrp:test-owner", "eth0", vips); err != nil {
		t.Fatal(err)
	}
	if err := setOwnerFilter("vrrp:test-owner", "eth0", vips); err != nil {
		t.Fatal(err)
	}
	if len(published) != 1 || len(published[0]) != 1 {
		t.Fatalf("publishes after set = %+v, want one arp table once", published)
	}
	if err := clearOwnerFilter("vrrp:test-owner"); err != nil {
		t.Fatal(err)
	}
	if len(published) != 2 || published[1] != nil {
		t.Fatalf("publishes after clear = %+v, want the tables withdrawn", published)
	}
}

// TestOwnerFilterInstalledBeforeTheAddressAndWithdrawnAfterIt pins the order for
// the address owner. The filter lands before the address so the parent's
// physical-MAC answer is never the only one; it is withdrawn after the address
// so the parent answers again once it is the address's only holder. The filter
// names the parent device and exactly the owned address.
func TestOwnerFilterInstalledBeforeTheAddressAndWithdrawnAfterIt(t *testing.T) {
	spec := testSpec()
	spec.IsOwner = true
	spec.Priority = ownerPriority
	spec.realAddresses = []netip.Addr{spec.VIPs[0]}
	in, f, clk := newTestInstance(t, spec)

	in.dispatch(fsm.Startup{Config: in.fsmConfig()})
	promoteToActive(t, in, clk)
	in.dispatch(fsm.Shutdown{})

	got := f.snapshot()
	want := []string{"accept-filter-off", "owner-filter-on", "install-addresses",
		"remove-addresses", "accept-filter-withdrawn", "owner-filter-withdrawn"}
	if !slices.Equal(got.dataplane, want) {
		t.Fatalf("dataplane calls = %v, want %v", got.dataplane, want)
	}
	set := got.ownerFilters[0]
	if set.parent != "eth0" || !slices.Equal(set.vips, spec.VIPs) {
		t.Errorf("owner filter set = %+v, want parent eth0 and %v", set, spec.VIPs)
	}
}

// TestOwnerFilterNamesNothingForANonOwner proves the other side: a group whose
// virtual address is not a real address of the parent asks the filter for no
// address, so the parent's answers for its own addresses are untouched.
func TestOwnerFilterNamesNothingForANonOwner(t *testing.T) {
	spec := testSpec()
	spec.IsOwner = false
	spec.realAddresses = []netip.Addr{netip.MustParseAddr("192.0.2.254")}
	in, f, clk := newTestInstance(t, spec)

	in.dispatch(fsm.Startup{Config: in.fsmConfig()})
	promoteToActive(t, in, clk)

	got := f.snapshot()
	if len(got.ownerFilters) != 1 || len(got.ownerFilters[0].vips) != 0 {
		t.Fatalf("owner filter calls = %+v, want one call naming no address", got.ownerFilters)
	}
}

// TestOwnerFilterWiredOnPromotionForEveryFamily proves the product route to
// the owner filter: an address owner that becomes Master hands the dataplane
// the filter for its parent and exactly its owned addresses, before any
// address is installed, for IPv4 VRRPv2, IPv4 VRRPv3 and IPv6. The filter's
// effect on the wire, that the parent's physical-MAC answers are dropped, is
// TestVRRPOwnerAnswersWithVirtualMACOnly.
//
// Method: each owner starts and is promoted through the FSM. The first owner
// filter call must name the parent and the owned address, and must come before
// the address install. The same group as a non-owner names no address, so the
// filter is bound to ownership.
//
// RFC requirement: RFC3768-8.2-1 positive -- a VRRPv2 address owner promoted to Master hands the dataplane the owner filter for its parent and its owned IPv4 address before installing it, so the parent's physical-MAC ARP answer is dropped whenever the Master holds the address (doInstallVIPs instance.go, ownedVIPs)
// RFC requirement: RFC3768-8.2-1 negative -- contrast: the same VRRPv2 group as a non-owner names no address to the filter, so the drop is bound to the owner, the only case where the parent holds the virtual address (doInstallVIPs instance.go).
// RFC requirement: RFC5798-8.2.2-1 positive -- an IPv6 address owner promoted to Master hands the dataplane the owner filter for its parent and its owned IPv6 address before installing it, so the parent's physical-MAC Neighbor Advertisement is dropped whenever the Master holds the address (doInstallVIPs instance.go, ownedVIPs)
// RFC requirement: RFC5798-8.2.2-1 negative -- contrast: the same IPv6 group as a non-owner names no address to the filter, so the drop is bound to the owner (doInstallVIPs instance.go).
// RFC requirement: RFC9568-8.2.2-1 positive -- an IPv6 address owner promoted to Active hands the dataplane the owner filter for its parent and its owned IPv6 address before installing it, so the parent's physical-MAC Neighbor Advertisement is dropped whenever the Active router holds the address (doInstallVIPs instance.go, ownedVIPs)
// RFC requirement: RFC9568-8.2.2-1 negative -- contrast: the same IPv6 group as a non-owner names no address to the filter, so the drop is bound to the owner (doInstallVIPs instance.go).
func TestOwnerFilterWiredOnPromotionForEveryFamily(t *testing.T) {
	v2 := testSpec()
	v2.Version = versionV2
	v6 := testSpecV6()
	cases := []struct {
		name  string
		spec  GroupSpec
		owned netip.Addr
	}{
		{"ipv4-v2", v2, v2.VIPs[0]},
		{"ipv4-v3", testSpec(), testSpec().VIPs[0]},
		{"ipv6", v6, v6.VIPs[1]},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := tc.spec
			spec.realAddresses = []netip.Addr{tc.owned}
			spec.IsOwner = true
			spec.Priority = ownerPriority
			in, f, clk := newTestInstance(t, spec)
			in.dispatch(fsm.Startup{Config: in.fsmConfig()})
			promoteToActive(t, in, clk)

			got := f.snapshot()
			if len(got.ownerFilters) == 0 {
				t.Fatal("the owner Master handed the dataplane no owner filter")
			}
			set := got.ownerFilters[0]
			if set.parent != spec.ParentDevice || !slices.Equal(set.vips, []netip.Addr{tc.owned}) {
				t.Fatalf("owner filter = %+v, want parent %s and exactly %v", set, spec.ParentDevice, tc.owned)
			}
			filterAt := slices.Index(got.dataplane, "owner-filter-on")
			installAt := slices.Index(got.dataplane, "install-addresses")
			if filterAt < 0 || installAt < 0 || filterAt > installAt {
				t.Fatalf("dataplane calls = %v, want owner-filter-on before install-addresses", got.dataplane)
			}

			// The non-owner's parent holds another address of the subnet, never
			// the virtual one.
			spec.realAddresses = []netip.Addr{netip.MustParseAddr("192.0.2.254")}
			if tc.owned.Is6() {
				spec.realAddresses = []netip.Addr{netip.MustParseAddr("2001:db8::254")}
			}
			spec.IsOwner = false
			spec.Priority = 200
			inN, fN, clkN := newTestInstance(t, spec)
			inN.dispatch(fsm.Startup{Config: inN.fsmConfig()})
			promoteToActive(t, inN, clkN)
			gotN := fN.snapshot()
			if len(gotN.ownerFilters) != 1 || len(gotN.ownerFilters[0].vips) != 0 {
				t.Fatalf("non-owner owner filter calls = %+v, want one call naming no address", gotN.ownerFilters)
			}
		})
	}
}
