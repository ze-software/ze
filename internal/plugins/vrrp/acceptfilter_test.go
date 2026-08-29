// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- Accept_Mode dataplane enforcement tests
// RFC: rfc/short/rfc9568.md (VRRPv3) -- Section 6.1 Accept_Mode, Section 6.4.3 Active
//
// VALIDATES: the acceptance decision RFC 9568 Section 6.4.3 attaches to an
// Active router reaches the dataplane, in all three directions the sentence
// names -- the address owner accepts, Accept_Mode True accepts, and neither one
// MUST NOT accept -- and that the Section 6.1 carve-out keeps IPv6 Neighbor
// Solicitations and Advertisements out of the resulting filter.
// PREVENTS: the defect these tests were written against, in which Accept_Mode
// reached the show snapshot and nothing else, so a non-owner Active answered on
// the virtual address whichever way the operator set the leaf.
//
// The tests drive the state machine and read what the executor handed the
// dataplane. None of them reads the configuration back out of the config.

package vrrp

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
	"github.com/ze-software/ze/internal/test/sim"
)

// dropTermNames returns the names of every Drop term in the built table, in
// rule order, so a test can say what the kernel would refuse.
func dropTermNames(t *testing.T, tables []firewall.Table) []string {
	t.Helper()
	var names []string
	for _, tbl := range tables {
		for _, chain := range tbl.Chains {
			for _, term := range chain.Terms {
				for _, action := range term.Actions {
					if _, ok := action.(firewall.Drop); ok {
						names = append(names, term.Name)
					}
				}
			}
		}
	}
	return names
}

// firstChain returns the single base chain the filter builds. It fails the test
// rather than returning a zero value, because a zero chain has no terms and
// every ordering assertion below would then pass vacuously.
func firstChain(t *testing.T, tables []firewall.Table) firewall.Chain {
	t.Helper()
	if len(tables) != 1 {
		t.Fatalf("tables = %d, want exactly 1", len(tables))
	}
	if len(tables[0].Chains) != 1 {
		t.Fatalf("chains = %d, want exactly 1", len(tables[0].Chains))
	}
	return tables[0].Chains[0]
}

// TestAcceptFilterTableDropsEveryAddressItIsGiven proves the built table refuses
// local delivery for each suppressed address and for nothing else, and that it
// is hooked where local delivery happens.
func TestAcceptFilterTableDropsEveryAddressItIsGiven(t *testing.T) {
	v4 := netip.MustParseAddr("192.0.2.1")
	v6 := netip.MustParseAddr("2001:db8::1")

	tables := acceptFilterTables([]netip.Addr{v4, v6})
	chain := firstChain(t, tables)

	if tables[0].Name != acceptFilterTableName || tables[0].Family != firewall.FamilyInet {
		t.Errorf("table = %q/%v, want %q/inet", tables[0].Name, tables[0].Family, acceptFilterTableName)
	}
	if !chain.IsBase || chain.Hook != firewall.HookInput {
		t.Errorf("chain hook = %v (base %v), want a base chain at the input hook: local delivery is what RFC 9568 Section 6.4.3 calls accepting", chain.Hook, chain.IsBase)
	}
	if chain.Policy != firewall.PolicyAccept {
		t.Errorf("chain policy = %v, want accept: this table decides only what it names", chain.Policy)
	}

	got := dropTermNames(t, tables)
	want := []string{"drop-192.0.2.1", "drop-2001-db8--1"}
	if len(got) != len(want) {
		t.Fatalf("drop terms = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("drop term %d = %q, want %q", i, got[i], want[i])
		}
	}

	for _, term := range chain.Terms {
		for _, match := range term.Matches {
			dst, ok := match.(firewall.MatchDestinationAddress)
			if !ok {
				continue
			}
			if !dst.Prefix.IsSingleIP() {
				t.Errorf("term %q matches %v, want a host prefix: the rule names one virtual address and no other", term.Name, dst.Prefix)
			}
		}
	}
}

// TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop proves the Section 6.1
// carve-out is placed where it works: a packet takes the verdict of the first
// rule it matches, so the two ICMPv6 accepts must precede every drop.
//
// RFC requirement: RFC9568-6.1-1 positive -- with Accept_Mode False in force for a virtual address, ICMPv6 Neighbor Solicitation (135) and Neighbor Advertisement (136) are accepted by the filter ahead of every address drop, so neither is dropped (acceptFilterTables acceptfilter.go)
// RFC requirement: RFC9568-6.1-1 negative -- contrast: the same table DOES drop a non-ND packet addressed to that virtual address, so the carve-out is specific to Neighbor Discovery and is not the chain being uniformly permissive (acceptFilterTables acceptfilter.go).
func TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop(t *testing.T) {
	v6 := netip.MustParseAddr("2001:db8::1")
	chain := firstChain(t, acceptFilterTables([]netip.Addr{v6}))

	firstDrop := -1
	acceptedICMPv6 := map[uint8]int{}
	for i, term := range chain.Terms {
		for _, action := range term.Actions {
			if _, ok := action.(firewall.Drop); ok && firstDrop < 0 {
				firstDrop = i
			}
			if _, ok := action.(firewall.Accept); !ok {
				continue
			}
			for _, match := range term.Matches {
				if icmp, ok := match.(firewall.MatchICMPv6Type); ok {
					acceptedICMPv6[icmp.Type] = i
				}
			}
		}
	}

	if firstDrop < 0 {
		t.Fatal("no drop term: the negative half of this test would pass vacuously")
	}
	for _, icmpType := range []uint8{icmpv6TypeNeighborSolicit, icmpv6TypeNeighborAdvert} {
		at, ok := acceptedICMPv6[icmpType]
		if !ok {
			t.Fatalf("ICMPv6 type %d is not accepted: RFC 9568 Section 6.1 says it MUST NOT be dropped when Accept_Mode is False", icmpType)
		}
		if at >= firstDrop {
			t.Errorf("ICMPv6 type %d accepted at rule %d, after the first drop at %d: the first matching rule decides, so the carve-out has to come first", icmpType, at, firstDrop)
		}
	}
}

// TestAcceptFilterWithdrawsWhenNothingIsSuppressed proves the last instance to
// stop suppressing takes the kernel table with it, rather than leaving an empty
// one that a later reader would take for a live policy.
func TestAcceptFilterWithdrawsWhenNothingIsSuppressed(t *testing.T) {
	if tables := acceptFilterTables(nil); tables != nil {
		t.Errorf("tables = %+v, want none: an empty suppression set withdraws the table", tables)
	}
}

// TestAcceptFilterTermNamesAreValidFirewallNames proves the name derived from an
// address is one the firewall registry accepts. An IPv6 address carries colons,
// which firewall.ValidateName refuses, so a name built without the substitution
// would be rejected at registration and no rule would reach the kernel at all.
func TestAcceptFilterTermNamesAreValidFirewallNames(t *testing.T) {
	addrs := []netip.Addr{
		netip.MustParseAddr("192.0.2.1"),
		netip.MustParseAddr("2001:db8::1"),
		netip.MustParseAddr("fe80::1"),
	}
	for _, addr := range addrs {
		name := acceptFilterTermName(addr)
		if err := firewall.ValidateName(name); err != nil {
			t.Errorf("term name %q for %v: %v", name, addr, err)
		}
	}
}

// TestAcceptFilterDeduplicatesAndSortsAddresses proves two instances suppressing
// one address produce one rule, and that the rule order does not depend on map
// iteration order.
func TestAcceptFilterDeduplicatesAndSortsAddresses(t *testing.T) {
	acceptFilterMu.Lock()
	acceptFilterState = map[string][]netip.Addr{
		"vrrp:zv4-b": {netip.MustParseAddr("192.0.2.9"), netip.MustParseAddr("192.0.2.1")},
		"vrrp:zv4-a": {netip.MustParseAddr("192.0.2.1")},
	}
	got := suppressedAddressesLocked()
	acceptFilterState = map[string][]netip.Addr{}
	acceptFilterMu.Unlock()

	want := []netip.Addr{netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("192.0.2.9")}
	if len(got) != len(want) {
		t.Fatalf("addresses = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("address %d = %v, want %v", i, got[i], want[i])
		}
	}
}

// TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery drives a non-owner
// group into Active and reads what the executor handed the dataplane.
//
// RFC requirement: RFC9568-6.4.3-7 positive -- an Active router that is neither the address owner nor configured with Accept_Mode True hands the dataplane a suppression for its virtual addresses, so it does not accept packets addressed to them (doInstallVIPs instance.go, EffectiveAcceptMode groups.go)
// RFC requirement: RFC9568-6.4.3-6 negative -- contrast: this router does NOT accept packets addressed to the virtual address, because it satisfies neither of the two conditions Section 6.4.3 makes acceptance conditional on (EffectiveAcceptMode groups.go).
func TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery(t *testing.T) {
	spec := testSpec()
	spec.IsOwner = false
	spec.AcceptMode = false
	in, f, clk := newTestInstance(t, spec)

	in.dispatch(fsm.Startup{Config: in.fsmConfig()})
	promoteToActive(t, in, clk)

	got := f.snapshot()
	if in.machine.State() != fsm.StateMaster {
		t.Fatalf("state = %v, want Master: the suppression assertion below needs an Active router", in.machine.State())
	}
	if len(got.installs) != 1 {
		t.Fatalf("address installs = %d, want 1: the Active router still installs the address, because Section 6.4.3 requires it to answer ARP and ND for it", len(got.installs))
	}
	if len(got.filters) != 1 {
		t.Fatalf("acceptance-filter calls = %+v, want exactly 1", got.filters)
	}
	if got.filters[0].accept {
		t.Errorf("filter accept = true, want false: neither owner nor Accept_Mode True, so RFC 9568 Section 6.4.3 says this router MUST NOT accept packets addressed to the virtual address")
	}
	if got.filters[0].owner != in.own {
		t.Errorf("filter owner = %q, want %q", got.filters[0].owner, in.own)
	}
	if len(got.filters[0].vips) != 1 || got.filters[0].vips[0] != spec.VIPs[0] {
		t.Errorf("filter addresses = %v, want %v", got.filters[0].vips, spec.VIPs)
	}
}

// TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery proves the operator's
// opt-in reaches the dataplane, and is the discriminating contrast for the test
// above: the same router, the same promotion, one leaf changed.
//
// RFC requirement: RFC9568-6.4.3-6 positive -- an Active router configured with Accept_Mode True hands the dataplane no suppression, so it accepts packets addressed to the virtual address (doInstallVIPs instance.go, EffectiveAcceptMode groups.go)
// RFC requirement: RFC9568-6.4.3-7 negative -- contrast: the prohibition does NOT reach an Active router whose Accept_Mode is True, so the suppression is bound to the condition rather than applied to every non-owner (EffectiveAcceptMode groups.go).
func TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery(t *testing.T) {
	spec := testSpec()
	spec.IsOwner = false
	spec.AcceptMode = true
	in, f, clk := newTestInstance(t, spec)

	in.dispatch(fsm.Startup{Config: in.fsmConfig()})
	promoteToActive(t, in, clk)

	got := f.snapshot()
	if len(got.filters) != 1 {
		t.Fatalf("acceptance-filter calls = %+v, want exactly 1", got.filters)
	}
	if !got.filters[0].accept {
		t.Errorf("filter accept = false, want true: Accept_Mode True is one of the two conditions RFC 9568 Section 6.4.3 accepts on")
	}
}

// TestActiveAddressOwnerAcceptsWhateverAcceptModeSays proves the Section 6.1
// exemption survives the new filter: the address owner accepts on its own
// address even with the leaf set false, and `show vrrp` reports that.
//
// RFC requirement: RFC9568-6.4.3-6 positive -- the IPvX address owner accepts packets addressed to the virtual address, and hands the dataplane no suppression, whatever Accept_Mode was configured (EffectiveAcceptMode groups.go)
// RFC requirement: RFC9568-6.4.3-7 negative -- contrast: the prohibition does NOT reach the address owner, which is the first of the two exemptions the sentence names (EffectiveAcceptMode groups.go).
func TestActiveAddressOwnerAcceptsWhateverAcceptModeSays(t *testing.T) {
	spec := testSpec()
	spec.IsOwner = true
	spec.AcceptMode = false
	in, f, _ := newTestInstance(t, spec)

	in.dispatch(fsm.Startup{Config: in.fsmConfig()})

	got := f.snapshot()
	if in.machine.State() != fsm.StateMaster {
		t.Fatalf("owner state = %v, want Master", in.machine.State())
	}
	if len(got.filters) != 1 {
		t.Fatalf("acceptance-filter calls = %+v, want exactly 1", got.filters)
	}
	if !got.filters[0].accept {
		t.Errorf("filter accept = false, want true: the address owner accepts on its own address, whatever Accept_Mode says (RFC 9568 Section 6.1)")
	}
	if !in.snapshot().AcceptMode {
		t.Error("show vrrp accept-mode = false for the address owner, want true: the reported value is the effective one")
	}
}

// TestAcceptFilterInstalledBeforeTheAddressAndWithdrawnAfterIt pins the order of
// the two dataplane calls. A filter that lands after the address leaves a window
// in which the kernel accepts what the router MUST NOT accept, and a filter
// withdrawn before the address leaves the same window at the other end.
func TestAcceptFilterInstalledBeforeTheAddressAndWithdrawnAfterIt(t *testing.T) {
	spec := testSpec()
	spec.IsOwner = false
	spec.AcceptMode = false
	in, f, clk := newTestInstance(t, spec)

	in.dispatch(fsm.Startup{Config: in.fsmConfig()})
	promoteToActive(t, in, clk)
	in.dispatch(fsm.Shutdown{})

	got := f.snapshot()
	want := []string{"accept-filter-on", "install-addresses", "remove-addresses", "accept-filter-withdrawn"}
	if len(got.dataplane) != len(want) {
		t.Fatalf("dataplane calls = %v, want %v", got.dataplane, want)
	}
	for i := range want {
		if got.dataplane[i] != want[i] {
			t.Errorf("dataplane call %d = %q, want %q", i, got.dataplane[i], want[i])
		}
	}
}

// TestAcceptModeChangeOnARunningActiveReachesTheDataplane proves the operator can
// flip the leaf on a live Active router. The address set has not moved, so
// nothing else in the executor would have run.
func TestAcceptModeChangeOnARunningActiveReachesTheDataplane(t *testing.T) {
	spec := testSpec()
	spec.IsOwner = false
	spec.AcceptMode = false
	in, f, clk := newTestInstance(t, spec)

	in.dispatch(fsm.Startup{Config: in.fsmConfig()})
	promoteToActive(t, in, clk)

	relaxed := spec
	relaxed.AcceptMode = true
	in.reconfigure(relaxed)

	got := f.snapshot()
	if len(got.filters) != 2 {
		t.Fatalf("acceptance-filter calls = %+v, want 2 (one on promotion, one on the config change)", got.filters)
	}
	if got.filters[0].accept {
		t.Error("first filter call accepted, want suppression: the group started with accept-mode false")
	}
	if !got.filters[1].accept {
		t.Error("second filter call suppressed, want acceptance: accept-mode was set true on a running Active router")
	}
	if got.announces != 1 {
		t.Errorf("announcements = %d, want 1: no address changed hands, so no gratuitous ARP is owed for the accept-mode change", got.announces)
	}
}

// promoteToActive drives a Backup instance into Active by letting its own
// master-down timer fire, so the tests above exercise the promotion path an
// election actually takes. Writing the state in by hand would skip the executor,
// which is the half under test.
func promoteToActive(t *testing.T, in *instance, clk *sim.FakeClock) {
	t.Helper()
	if in.machine.State() == fsm.StateMaster {
		return
	}
	clk.Add(10 * time.Second)
	deadline := time.After(2 * time.Second)
	for in.machine.State() != fsm.StateMaster {
		select {
		case ev := <-in.events:
			in.dispatch(ev)
		case <-deadline:
			t.Fatal("the master-down timer never fired: the router never became Active")
		}
	}
}

// TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress covers the VRRPv2 form of the
// same prohibition. RFC 3768 has no Accept_Mode, and ze rejects the leaf under
// version 2 (validateGroup groups.go), so ownership is the whole of the
// condition there and EffectiveAcceptMode reduces to IsOwner.
//
// RFC requirement: RFC3768-6.4.3-3 positive -- a VRRPv2 Master that is not the IP address owner hands the dataplane a suppression for the virtual address, so it does not accept packets addressed to it (doInstallVIPs instance.go, EffectiveAcceptMode groups.go)
// RFC requirement: RFC3768-6.4.3-3 negative -- contrast: the VRRPv2 Master that IS the address owner hands the dataplane no suppression and accepts, so the prohibition is bound to ownership rather than applied to every Master (EffectiveAcceptMode groups.go).
func TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress(t *testing.T) {
	tests := []struct {
		name       string
		isOwner    bool
		wantAccept bool
	}{
		{name: "non-owner", isOwner: false, wantAccept: false},
		{name: "address-owner", isOwner: true, wantAccept: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spec := testSpec()
			spec.Version = versionV2
			spec.IsOwner = tc.isOwner

			in, f, clk := newTestInstance(t, spec)
			in.dispatch(fsm.Startup{Config: in.fsmConfig()})
			promoteToActive(t, in, clk)

			got := f.snapshot()
			if len(got.filters) != 1 {
				t.Fatalf("acceptance-filter calls = %+v, want exactly 1", got.filters)
			}
			if got.filters[0].accept != tc.wantAccept {
				t.Errorf("filter accept = %v, want %v", got.filters[0].accept, tc.wantAccept)
			}
		})
	}
}

// withRecordedAcceptFilterPublish substitutes the publish step and returns the
// slice each reconcile appends to, so a test can see what this package hands the
// firewall component and how often. The original is restored on cleanup.
func withRecordedAcceptFilterPublish(t *testing.T) *[][]firewall.Table {
	t.Helper()
	original := acceptFilterPublish
	var published [][]firewall.Table
	acceptFilterPublish = func(tables []firewall.Table) error {
		published = append(published, tables)
		return nil
	}
	t.Cleanup(func() {
		acceptFilterPublish = original
		acceptFilterMu.Lock()
		acceptFilterState = map[string][]netip.Addr{}
		acceptFilterMu.Unlock()
	})
	return &published
}

// TestAcceptFilterShareTheTableAndWithdrawIndependently proves two groups build
// one table, that withdrawing one leaves the other's rule standing, and that the
// last withdrawal takes the table away.
func TestAcceptFilterShareTheTableAndWithdrawIndependently(t *testing.T) {
	published := withRecordedAcceptFilterPublish(t)

	first := netip.MustParseAddr("192.0.2.1")
	second := netip.MustParseAddr("192.0.2.2")
	if err := setAcceptFilter("vrrp:zv4-a", []netip.Addr{first}, false); err != nil {
		t.Fatalf("suppress first group: %v", err)
	}
	if err := setAcceptFilter("vrrp:zv4-b", []netip.Addr{second}, false); err != nil {
		t.Fatalf("suppress second group: %v", err)
	}
	if err := clearAcceptFilter("vrrp:zv4-a"); err != nil {
		t.Fatalf("withdraw first group: %v", err)
	}

	if len(*published) != 3 {
		t.Fatalf("published %d table sets, want 3", len(*published))
	}
	if got := dropTermNames(t, (*published)[1]); len(got) != 2 {
		t.Errorf("drop terms after both groups = %v, want one for each group", got)
	}
	got := dropTermNames(t, (*published)[2])
	if len(got) != 1 || got[0] != acceptFilterTermName(second) {
		t.Errorf("drop terms after one withdrawal = %v, want only %q: withdrawing one group must not unblock another group's address", got, acceptFilterTermName(second))
	}

	if err := clearAcceptFilter("vrrp:zv4-b"); err != nil {
		t.Fatalf("withdraw second group: %v", err)
	}
	if last := (*published)[len(*published)-1]; last != nil {
		t.Errorf("last publish = %+v, want no table: the final withdrawal takes the kernel table with it", last)
	}
}

// TestAcceptFilterDoesNotReachTheFirewallWhenNothingChanges proves a group that
// accepts never publishes anything. Every promotion calls setAcceptFilter, so
// without the short circuit a deployment that configured no firewall would load
// the firewall backend on its first VRRP transition.
func TestAcceptFilterDoesNotReachTheFirewallWhenNothingChanges(t *testing.T) {
	published := withRecordedAcceptFilterPublish(t)
	vips := []netip.Addr{netip.MustParseAddr("192.0.2.1")}

	if err := setAcceptFilter("vrrp:zv4-a", vips, true); err != nil {
		t.Fatalf("accepting group: %v", err)
	}
	if err := clearAcceptFilter("vrrp:zv4-a"); err != nil {
		t.Fatalf("withdraw a group that never suppressed: %v", err)
	}
	if len(*published) != 0 {
		t.Fatalf("published %d table sets, want 0: nothing was ever suppressed", len(*published))
	}

	if err := setAcceptFilter("vrrp:zv4-a", vips, false); err != nil {
		t.Fatalf("suppress: %v", err)
	}
	if err := setAcceptFilter("vrrp:zv4-a", vips, false); err != nil {
		t.Fatalf("re-suppress the same addresses: %v", err)
	}
	if len(*published) != 1 {
		t.Errorf("published %d table sets, want 1: re-stating an unchanged suppression publishes nothing", len(*published))
	}
}
