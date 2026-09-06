// Design: docs/architecture/core-design.md -- tc ingress policer translation

//go:build linux

package trafficnetlink

import (
	"context"
	"errors"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/traffic"
)

// TestIngressPolicerFilterCarriesRateAndDrop checks the translation of a ze
// Policer into the matchall filter the kernel takes. The rate is converted to
// bytes per second because that is the unit TCA_POLICE_TBF carries, and traffic
// above it is dropped, because ingress has no queue to delay it into.
func TestIngressPolicerFilterCarriesRateAndDrop(t *testing.T) {
	filter, err := ingressPolicerFilter(traffic.NewPolicer(8_000_000), 7)
	if err != nil {
		t.Fatalf("ingressPolicerFilter: %v", err)
	}
	if filter.LinkIndex != 7 {
		t.Fatalf("LinkIndex = %d, want 7", filter.LinkIndex)
	}
	if filter.Parent != netlink.HANDLE_MIN_INGRESS {
		t.Fatalf("Parent = %#x, want HANDLE_MIN_INGRESS %#x", filter.Parent, uint32(netlink.HANDLE_MIN_INGRESS))
	}
	if filter.Priority != policerFilterPriority {
		t.Fatalf("Priority = %d, want %d", filter.Priority, policerFilterPriority)
	}
	if len(filter.Actions) != 1 {
		t.Fatalf("Actions = %d, want 1", len(filter.Actions))
	}
	police, ok := filter.Actions[0].(*netlink.PoliceAction)
	if !ok {
		t.Fatalf("action type = %T, want *netlink.PoliceAction", filter.Actions[0])
	}
	if police.Rate != 1_000_000 {
		t.Fatalf("police Rate = %d bytes/s, want 1000000 (8 Mbit/s)", police.Rate)
	}
	if uint64(police.Burst) != traffic.PolicerBurstBytes(8_000_000) {
		t.Fatalf("police Burst = %d, want %d", police.Burst, traffic.PolicerBurstBytes(8_000_000))
	}
	if police.ExceedAction != netlink.TC_POLICE_SHOT {
		t.Fatalf("ExceedAction = %v, want drop", police.ExceedAction)
	}
	if police.Mtu != policerMTUBytes {
		t.Fatalf("Mtu = %d, want %d", police.Mtu, policerMTUBytes)
	}
}

// TestIngressPolicerFilterRefusesUnrepresentableRate checks the bound: the
// kernel carries the policed rate in a uint32 of bytes per second, so a rate
// above 34.359 Gbit/s cannot be expressed. Truncating it would police at a
// wrapped rate, which is silently wrong.
func TestIngressPolicerFilterRefusesUnrepresentableRate(t *testing.T) {
	if _, err := ingressPolicerFilter(traffic.NewPolicer(policerRateBpsMax), 1); err != nil {
		t.Fatalf("the largest representable rate was refused: %v", err)
	}
	if _, err := ingressPolicerFilter(traffic.NewPolicer(policerRateBpsMax+8), 1); err == nil {
		t.Fatal("a rate above the uint32 byte-per-second bound was accepted")
	}
}

// TestIngressPolicerFilterRefusesBurstlessPolicer checks that a policer the
// model would refuse never reaches the kernel. A zero burst is accepted by tc
// and drops every packet.
func TestIngressPolicerFilterRefusesBurstlessPolicer(t *testing.T) {
	if _, err := ingressPolicerFilter(traffic.Policer{RateBps: 1_000_000}, 1); err == nil {
		t.Fatal("a policer with no burst was accepted")
	}
}

// TestApplyInstallsIngressPolicer checks the whole backend path: an
// InterfaceQoS carrying an Ingress policer reaches the kernel as a clsact qdisc
// plus one matchall filter on the ingress hook. Before this, InterfaceQoS held
// a root qdisc alone, so no upload enforcement was reachable through Backend.
func TestApplyInstallsIngressPolicer(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["ppp0"] = testLink("ppp0", 11)
	ops.qdiscs["ppp0"] = []netlink.Qdisc{originalFQ(11)}
	b := testBackend(t, ops)

	qos := desiredHTB("ppp0")
	qos.Ingress = traffic.NewPolicer(5_000_000)
	if err := b.Apply(context.Background(), map[string]traffic.InterfaceQoS{"ppp0": qos}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	var clsact *netlink.Clsact
	for _, q := range ops.replaced {
		if c, ok := q.(*netlink.Clsact); ok {
			clsact = c
		}
	}
	if clsact != nil {
		t.Fatal("the clsact qdisc was REPLACED; replacing it drops every filter the mirror and sampling paths own on that hook")
	}
	if len(ops.addedQdiscs) != 1 {
		t.Fatalf("qdiscAdd calls = %d, want 1 (the clsact hook)", len(ops.addedQdiscs))
	}
	if _, ok := ops.addedQdiscs[0].(*netlink.Clsact); !ok {
		t.Fatalf("added qdisc type = %T, want *netlink.Clsact", ops.addedQdiscs[0])
	}

	var policer *netlink.MatchAll
	for _, f := range ops.added {
		if m, ok := f.(*netlink.MatchAll); ok && m.Priority == policerFilterPriority {
			policer = m
		}
	}
	if policer == nil {
		t.Fatal("no ingress policer filter was installed; the upload rate reaches no interface")
	}
	if policer.Parent != netlink.HANDLE_MIN_INGRESS {
		t.Fatalf("policer parent = %#x, want the ingress hook", policer.Parent)
	}
}

// TestApplyWithoutIngressPolicerTouchesNoIngressHook checks that an interface
// asking for no upload enforcement leaves the shared clsact hook alone. The
// mirror and sampling paths own filters there, so touching it without being
// asked is a change to another subsystem's state.
func TestApplyWithoutIngressPolicerTouchesNoIngressHook(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["eth9"] = testLink("eth9", 12)
	ops.qdiscs["eth9"] = []netlink.Qdisc{originalFQ(12)}
	b := testBackend(t, ops)

	if err := b.Apply(context.Background(), map[string]traffic.InterfaceQoS{"eth9": desiredHTB("eth9")}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(ops.addedQdiscs) != 0 {
		t.Fatalf("qdiscAdd calls = %d, want 0", len(ops.addedQdiscs))
	}
	for _, f := range ops.added {
		if m, ok := f.(*netlink.MatchAll); ok && m.Priority == policerFilterPriority {
			t.Fatal("an ingress policer filter was installed for an interface that asked for none")
		}
	}
}

// TestApplyToleratesExistingClsact checks that an interface whose ingress hook
// another subsystem already created still gets its policer. EEXIST means the
// hook the policer needs is present, which is the outcome asked for.
func TestApplyToleratesExistingClsact(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["ppp1"] = testLink("ppp1", 13)
	ops.qdiscs["ppp1"] = []netlink.Qdisc{originalFQ(13)}
	ops.qdiscAddErr = unix.EEXIST
	b := testBackend(t, ops)

	qos := desiredHTB("ppp1")
	qos.Ingress = traffic.NewPolicer(2_000_000)
	if err := b.Apply(context.Background(), map[string]traffic.InterfaceQoS{"ppp1": qos}); err != nil {
		t.Fatalf("Apply refused an ingress hook another subsystem already created: %v", err)
	}
	found := false
	for _, f := range ops.added {
		if m, ok := f.(*netlink.MatchAll); ok && m.Priority == policerFilterPriority {
			found = true
		}
	}
	if !found {
		t.Fatal("no policer filter installed after EEXIST on the clsact qdisc")
	}
}

// TestRestoreOriginalRemovesPolicerButKeepsHook checks session teardown. The
// policer filter goes, and the clsact qdisc stays, because a teardown cannot
// know who else attached to it.
func TestRestoreOriginalRemovesPolicerButKeepsHook(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["ppp2"] = testLink("ppp2", 14)
	ops.qdiscs["ppp2"] = []netlink.Qdisc{originalFQ(14)}
	b := testBackend(t, ops)

	qos := desiredHTB("ppp2")
	qos.Ingress = traffic.NewPolicer(3_000_000)
	if err := b.Apply(context.Background(), map[string]traffic.InterfaceQoS{"ppp2": qos}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := b.RestoreOriginal(context.Background(), "ppp2"); err != nil {
		t.Fatalf("RestoreOriginal: %v", err)
	}

	removed := false
	for _, f := range ops.deletedFilters {
		if m, ok := f.(*netlink.MatchAll); ok && m.Priority == policerFilterPriority {
			removed = true
		}
	}
	if !removed {
		t.Fatal("the ingress policer survived RestoreOriginal; the next session on this pppN inherits it")
	}
	for _, q := range ops.deleted {
		if _, ok := q.(*netlink.Clsact); ok {
			t.Fatal("the clsact qdisc was deleted; that drops every mirror and sampling filter on the hook")
		}
	}
}

// TestRestoreOriginalToleratesMissingPolicer checks the two errnos the kernel
// answers a FilterDel that matches nothing with. Both mean "there was nothing
// to delete", which is the state asked for.
func TestRestoreOriginalToleratesMissingPolicer(t *testing.T) {
	for _, errno := range []error{unix.ENOENT, unix.EINVAL} {
		ops := newFakeTCOps()
		ops.links["ppp3"] = testLink("ppp3", 15)
		ops.qdiscs["ppp3"] = []netlink.Qdisc{originalFQ(15)}
		ops.filterDelErr = errno
		b := testBackend(t, ops)

		if err := b.Apply(context.Background(), map[string]traffic.InterfaceQoS{"ppp3": desiredHTB("ppp3")}); err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if err := b.RestoreOriginal(context.Background(), "ppp3"); err != nil {
			t.Fatalf("RestoreOriginal did not tolerate %v: %v", errno, err)
		}
	}
}

// TestRestoreOriginalReportsPolicerRemovalFailure checks that an errno which is
// NOT "nothing to delete" reaches the caller. A policer left behind rate-limits
// whoever gets this interface next.
func TestRestoreOriginalReportsPolicerRemovalFailure(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["ppp4"] = testLink("ppp4", 16)
	ops.qdiscs["ppp4"] = []netlink.Qdisc{originalFQ(16)}
	ops.filterDelErr = unix.EPERM
	b := testBackend(t, ops)

	if err := b.Apply(context.Background(), map[string]traffic.InterfaceQoS{"ppp4": desiredHTB("ppp4")}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	err := b.RestoreOriginal(context.Background(), "ppp4")
	if !errors.Is(err, unix.EPERM) {
		t.Fatalf("RestoreOriginal swallowed a real removal failure: %v", err)
	}
}
