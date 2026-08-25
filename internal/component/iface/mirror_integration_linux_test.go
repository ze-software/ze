//go:build integration && linux

package iface

import (
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const (
	// testMirrorPriority is the tc filter priority ze's mirror owns
	// (mirrorFilterPriority in internal/plugins/iface/netlink).
	testMirrorPriority uint16 = 1
	// testForeignPriority is the priority flow-export sampling owns
	// (SampleFilterPriority in internal/plugins/flowexport/sampling). A filter
	// here stands for every subsystem that shares the clsact qdisc: what makes
	// it foreign to the mirror is the priority and the shared attachment
	// point, not the action it carries.
	testForeignPriority uint16 = 100
)

// filterAt returns the filter attached to one clsact hook at the given
// priority, or nil when there is none.
func filterAt(t *testing.T, linkName string, parent uint32, priority uint16) netlink.Filter {
	t.Helper()
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		t.Fatalf("LinkByName(%q): %v", linkName, err)
	}
	filters, err := netlink.FilterList(link, parent)
	if err != nil {
		return nil
	}
	for _, f := range filters {
		if f.Attrs().Priority == priority {
			return f
		}
	}
	return nil
}

// mirredDst returns the destination ifindex of the mirred action carried by a
// filter, or 0 when it carries none.
func mirredDst(f netlink.Filter) int {
	matchall, ok := f.(*netlink.MatchAll)
	if !ok {
		return 0
	}
	for _, a := range matchall.Actions {
		if mirred, ok := a.(*netlink.MirredAction); ok {
			return mirred.Ifindex
		}
	}
	return 0
}

// addForeignFilter attaches a filter belonging to another subsystem to the
// shared qdisc, at the priority flow-export sampling uses.
func addForeignFilter(t *testing.T, srcName string) {
	t.Helper()
	// The foreign filter always contends for the ingress qdisc; that contention
	// IS what these tests exercise.
	parent := uint32(netlink.HANDLE_MIN_INGRESS)
	// The mirror destination is fixed: every caller creates smp0 and redirects to
	// it, so a parameter here only offered a way for the two to disagree.
	const dstName = "smp0"
	src, err := netlink.LinkByName(srcName)
	if err != nil {
		t.Fatalf("LinkByName(%q): %v", srcName, err)
	}
	dst, err := netlink.LinkByName(dstName)
	if err != nil {
		t.Fatalf("LinkByName(%q): %v", dstName, err)
	}
	filter := &netlink.MatchAll{
		FilterAttrs: netlink.FilterAttrs{
			LinkIndex: src.Attrs().Index,
			Parent:    parent,
			Priority:  testForeignPriority,
			Protocol:  unix.ETH_P_ALL,
		},
		Actions: []netlink.Action{
			&netlink.MirredAction{
				ActionAttrs:  netlink.ActionAttrs{Action: netlink.TC_ACT_PIPE},
				MirredAction: netlink.TCA_EGRESS_MIRROR,
				Ifindex:      dst.Attrs().Index,
			},
		},
	}
	if err := netlink.FilterAdd(filter); err != nil {
		t.Fatalf("add foreign filter on %q: %v", srcName, err)
	}
}

// addClsactQdisc creates the shared clsact qdisc the way flow-export sampling
// creates it, before ze's mirror ever touches the interface.
func addClsactQdisc(t *testing.T, linkName string) {
	t.Helper()
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		t.Fatalf("LinkByName(%q): %v", linkName, err)
	}
	qdisc := &netlink.Clsact{
		QdiscAttrs: netlink.QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    netlink.MakeHandle(0xffff, 0),
			Parent:    netlink.HANDLE_CLSACT,
		},
	}
	if err := netlink.QdiscAdd(qdisc); err != nil {
		t.Fatalf("add clsact qdisc on %q: %v", linkName, err)
	}
}

// hasQdisc returns true if any qdisc on the link matches the given type string.
func hasQdisc(t *testing.T, linkName, qdiscType string) bool {
	t.Helper()
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		t.Fatalf("LinkByName(%q): %v", linkName, err)
	}
	qdiscs, err := netlink.QdiscList(link)
	if err != nil {
		t.Fatalf("QdiscList(%q): %v", linkName, err)
	}
	for _, q := range qdiscs {
		if q.Type() == qdiscType {
			return true
		}
	}
	return false
}

func TestIntegrationMirrorIngress(t *testing.T) {
	// VALIDATES: SetupMirror with ingress=true installs a filter on the
	// ingress hook of the clsact qdisc.
	// PREVENTS: Ingress mirroring silently fails to configure tc.
	// The qdisc is clsact, not the older ingress kind: clsact carries both
	// hooks, so one qdisc serves an ingress-only mirror, an egress-only
	// mirror, and a mirror with a different destination per direction. The
	// ingress qdisc has no egress hook and cannot serve the third.
	withNetNS(t, func() {
		createDummyForTest(t, "src0")
		createDummyForTest(t, "dst0")

		if err := SetupMirror("src0", "dst0", true, false); err != nil {
			t.Fatalf("SetupMirror(ingress): %v", err)
		}
		t.Cleanup(func() { _ = RemoveMirror("src0") })

		if !hasQdisc(t, "src0", "clsact") {
			t.Error("expected clsact qdisc on src0, not found")
		}
		if filterAt(t, "src0", netlink.HANDLE_MIN_INGRESS, testMirrorPriority) == nil {
			t.Error("expected a mirror filter on the ingress hook, not found")
		}
	})
}

func TestIntegrationMirrorEgress(t *testing.T) {
	// VALIDATES: SetupMirror with egress=true installs a filter on the EGRESS
	// hook, and leaves the ingress hook alone.
	// PREVENTS: egress mirroring silently failing to configure tc. Asserting the
	// qdisc alone stopped proving that once every mirror began creating clsact
	// in ensureClsactQdisc before either hook is touched: the qdisc appears even
	// if the egress branch of setupClsactMirror does nothing.
	withNetNS(t, func() {
		createDummyForTest(t, "src0")
		createDummyForTest(t, "dst0")

		if err := SetupMirror("src0", "dst0", false, true); err != nil {
			t.Fatalf("SetupMirror(egress): %v", err)
		}
		t.Cleanup(func() { _ = RemoveMirror("src0") })

		if !hasQdisc(t, "src0", "clsact") {
			t.Error("expected clsact qdisc on src0, not found")
		}
		egress := filterAt(t, "src0", netlink.HANDLE_MIN_EGRESS, testMirrorPriority)
		if egress == nil {
			t.Fatal("expected a mirror filter on the egress hook, not found")
		}
		dst, err := netlink.LinkByName("dst0")
		if err != nil {
			t.Fatalf("LinkByName(dst0): %v", err)
		}
		if got := mirredDst(egress); got != dst.Attrs().Index {
			t.Errorf("egress filter mirrors to ifindex %d, want %d", got, dst.Attrs().Index)
		}
		if filterAt(t, "src0", netlink.HANDLE_MIN_INGRESS, testMirrorPriority) != nil {
			t.Error("an egress-only mirror installed a filter on the ingress hook")
		}
	})
}

func TestIntegrationMirrorBoth(t *testing.T) {
	// VALIDATES: SetupMirror with both ingress and egress uses clsact qdisc.
	// PREVENTS: Combined mirror setup fails or only configures one direction.
	withNetNS(t, func() {
		createDummyForTest(t, "src0")
		createDummyForTest(t, "dst0")

		if err := SetupMirror("src0", "dst0", true, true); err != nil {
			t.Fatalf("SetupMirror(both): %v", err)
		}
		t.Cleanup(func() { _ = RemoveMirror("src0") })

		if !hasQdisc(t, "src0", "clsact") {
			t.Error("expected clsact qdisc on src0, not found")
		}

		// Verify filters exist on the clsact qdisc by checking filter list.
		link, err := netlink.LinkByName("src0")
		if err != nil {
			t.Fatalf("LinkByName: %v", err)
		}

		// Check ingress filters.
		ingressFilters, err := netlink.FilterList(link, netlink.HANDLE_MIN_INGRESS)
		if err != nil {
			t.Fatalf("FilterList(ingress): %v", err)
		}
		if len(ingressFilters) == 0 {
			t.Error("expected ingress filters on clsact, got none")
		}

		// Check egress filters.
		egressFilters, err := netlink.FilterList(link, netlink.HANDLE_MIN_EGRESS)
		if err != nil {
			t.Fatalf("FilterList(egress): %v", err)
		}
		if len(egressFilters) == 0 {
			t.Error("expected egress filters on clsact, got none")
		}
	})
}

func TestIntegrationMirrorRemove(t *testing.T) {
	// VALIDATES: AC-3 -- RemoveMirror removes the mirror's own filter from both
	// hooks and LEAVES the shared qdisc standing, even when the mirror was its
	// only user.
	// PREVENTS: teardown reaching for QdiscDel again. The qdisc at ffff: is
	// shared with flow-export sampling, and RemoveMirror cannot know who
	// created it (RemoveMirror, internal/plugins/iface/netlink/mirror_linux.go).
	withNetNS(t, func() {
		createDummyForTest(t, "src0")
		createDummyForTest(t, "dst0")

		if err := SetupMirror("src0", "dst0", true, false); err != nil {
			t.Fatalf("SetupMirror: %v", err)
		}

		// Ze installs clsact for every mirror, so the qdisc kind does not say whether a
		// mirror is installed. The filter does, and that is what this asserts.
		if filterAt(t, "src0", netlink.HANDLE_MIN_INGRESS, testMirrorPriority) == nil {
			t.Fatal("mirror filter should exist before removal")
		}

		if err := RemoveMirror("src0"); err != nil {
			t.Fatalf("RemoveMirror: %v", err)
		}

		// The mirror's filters go from both hooks.
		if filterAt(t, "src0", netlink.HANDLE_MIN_INGRESS, testMirrorPriority) != nil {
			t.Error("the mirror ingress filter survived RemoveMirror")
		}
		if filterAt(t, "src0", netlink.HANDLE_MIN_EGRESS, testMirrorPriority) != nil {
			t.Error("the mirror egress filter survived RemoveMirror")
		}
		// The older ingress qdisc is never installed now that every mirror uses
		// clsact, so it must not appear at any point.
		if hasQdisc(t, "src0", "ingress") {
			t.Error("ingress qdisc still present after RemoveMirror")
		}
		// Teardown is filter-scoped and deliberately leaves the qdisc, so this asserts the
		// qdisc is still there: deleting the shared qdisc reds this test.
		if !hasQdisc(t, "src0", "clsact") {
			t.Error("RemoveMirror deleted the shared clsact qdisc it does not own")
		}
	})
}

func TestIntegrationMirrorRemoveIdempotent(t *testing.T) {
	// VALIDATES: RemoveMirror succeeds even when no mirror is configured.
	// PREVENTS: Error returned for idempotent cleanup of unconfigured interface.
	withNetNS(t, func() {
		createDummyForTest(t, "src0")

		// No mirror configured -- RemoveMirror should succeed.
		if err := RemoveMirror("src0"); err != nil {
			t.Errorf("RemoveMirror on unconfigured interface: %v", err)
		}
	})
}

func TestIntegrationMirrorRemoveKeepsForeignFilter(t *testing.T) {
	// VALIDATES: AC-2 -- removing a mirror leaves every filter another
	// subsystem attached to the shared clsact qdisc, and leaves the qdisc.
	// PREVENTS: mirror teardown silently disabling flow-export sampling, which
	// is what deleting the whole qdisc did.
	withNetNS(t, func() {
		createDummyForTest(t, "src0")
		createDummyForTest(t, "dst0")
		createDummyForTest(t, "smp0")

		if err := SetupMirror("src0", "dst0", true, true); err != nil {
			t.Fatalf("SetupMirror(both): %v", err)
		}
		addForeignFilter(t, "src0")

		if err := RemoveMirror("src0"); err != nil {
			t.Fatalf("RemoveMirror: %v", err)
		}

		foreign := filterAt(t, "src0", netlink.HANDLE_MIN_INGRESS, testForeignPriority)
		if foreign == nil {
			t.Fatal("the foreign filter was removed with the mirror")
		}
		smp, err := netlink.LinkByName("smp0")
		if err != nil {
			t.Fatalf("LinkByName(smp0): %v", err)
		}
		if got := mirredDst(foreign); got != smp.Attrs().Index {
			t.Errorf("foreign filter destination = %d, want %d", got, smp.Attrs().Index)
		}
		if !hasQdisc(t, "src0", "clsact") {
			t.Error("the clsact qdisc was removed while a foreign filter was still attached")
		}
		if filterAt(t, "src0", netlink.HANDLE_MIN_INGRESS, testMirrorPriority) != nil {
			t.Error("the mirror ingress filter survived RemoveMirror")
		}
		if filterAt(t, "src0", netlink.HANDLE_MIN_EGRESS, testMirrorPriority) != nil {
			t.Error("the mirror egress filter survived RemoveMirror")
		}
	})
}

func TestIntegrationMirrorRemoveLeavesForeignQdiscUntouched(t *testing.T) {
	// VALIDATES: AC-8 -- RemoveMirror on an interface ze never mirrored takes
	// nothing away from the subsystem that created the qdisc.
	// PREVENTS: a config commit that mentions no mirror deleting a clsact
	// qdisc flow-export sampling created.
	withNetNS(t, func() {
		createDummyForTest(t, "src0")
		createDummyForTest(t, "smp0")

		addClsactQdisc(t, "src0")
		addForeignFilter(t, "src0")

		if err := RemoveMirror("src0"); err != nil {
			t.Fatalf("RemoveMirror: %v", err)
		}

		if filterAt(t, "src0", netlink.HANDLE_MIN_INGRESS, testForeignPriority) == nil {
			t.Error("the foreign filter was removed by a mirror teardown that had nothing to remove")
		}
		if !hasQdisc(t, "src0", "clsact") {
			t.Error("the foreign clsact qdisc was removed")
		}
	})
}

func TestIntegrationMirrorSetupOnExistingClsact(t *testing.T) {
	// VALIDATES: AC-4 -- a mirror can be configured on an interface whose
	// clsact qdisc another subsystem already created.
	// PREVENTS: mirror setup failing with EEXIST on an interface that already
	// exports flow samples.
	withNetNS(t, func() {
		createDummyForTest(t, "src0")
		createDummyForTest(t, "dst0")
		createDummyForTest(t, "smp0")

		addClsactQdisc(t, "src0")
		addForeignFilter(t, "src0")

		if err := SetupMirror("src0", "dst0", true, true); err != nil {
			t.Fatalf("SetupMirror on an existing clsact qdisc: %v", err)
		}
		t.Cleanup(func() { _ = RemoveMirror("src0") })

		if filterAt(t, "src0", netlink.HANDLE_MIN_INGRESS, testForeignPriority) == nil {
			t.Error("the foreign filter was lost when the mirror was installed")
		}
		if filterAt(t, "src0", netlink.HANDLE_MIN_INGRESS, testMirrorPriority) == nil {
			t.Error("no mirror filter on the ingress hook")
		}
		if filterAt(t, "src0", netlink.HANDLE_MIN_EGRESS, testMirrorPriority) == nil {
			t.Error("no mirror filter on the egress hook")
		}
	})
}

// AC-5's rollback test lives in the netlink package, as
// TestIntegrationMirrorSetupRollbackKeepsForeignFilter. It needs a mirror setup
// that fails after its first filter lands, and from this package the only lever is
// a pre-existing ingress qdisc: sch_ingress's ingress_find accepts any minor
// handle, so the egress filter add succeeds instead of failing. The netlink
// package can pass an unresolvable destination ifindex to setupClsactMirror, which
// the kernel refuses with ENODEV.

func TestIntegrationMirrorSetupIsIdempotent(t *testing.T) {
	// VALIDATES: AC-7 -- applying the same mirror twice succeeds and leaves
	// one filter per hook.
	// PREVENTS: a second commit of an unchanged config failing with EEXIST, or
	// stacking a second mirred filter on the same hook.
	withNetNS(t, func() {
		createDummyForTest(t, "src0")
		createDummyForTest(t, "dst0")

		for i := range 2 {
			if err := SetupMirror("src0", "dst0", true, true); err != nil {
				t.Fatalf("SetupMirror pass %d: %v", i+1, err)
			}
		}
		t.Cleanup(func() { _ = RemoveMirror("src0") })

		link, err := netlink.LinkByName("src0")
		if err != nil {
			t.Fatalf("LinkByName(src0): %v", err)
		}
		for _, hook := range []uint32{netlink.HANDLE_MIN_INGRESS, netlink.HANDLE_MIN_EGRESS} {
			filters, err := netlink.FilterList(link, hook)
			if err != nil {
				t.Fatalf("FilterList: %v", err)
			}
			if len(filters) != 1 {
				t.Errorf("hook %#x carries %d filters after two applies, want 1", hook, len(filters))
			}
		}
	})
}

func TestIntegrationMirrorTwoDestinations(t *testing.T) {
	// VALIDATES: a mirror that sends ingress and egress traffic to different
	// interfaces installs both filters.
	// PREVENTS: the second direction failing with EEXIST, which is what the
	// ingress qdisc plus an intolerant QdiscAdd produced.
	withNetNS(t, func() {
		createDummyForTest(t, "src0")
		createDummyForTest(t, "dst0")
		createDummyForTest(t, "dst1")

		if err := SetupMirror("src0", "dst0", true, false); err != nil {
			t.Fatalf("SetupMirror(ingress -> dst0): %v", err)
		}
		if err := SetupMirror("src0", "dst1", false, true); err != nil {
			t.Fatalf("SetupMirror(egress -> dst1): %v", err)
		}
		t.Cleanup(func() { _ = RemoveMirror("src0") })

		dst0, err := netlink.LinkByName("dst0")
		if err != nil {
			t.Fatalf("LinkByName(dst0): %v", err)
		}
		dst1, err := netlink.LinkByName("dst1")
		if err != nil {
			t.Fatalf("LinkByName(dst1): %v", err)
		}
		ingress := filterAt(t, "src0", netlink.HANDLE_MIN_INGRESS, testMirrorPriority)
		if ingress == nil {
			t.Fatal("no mirror filter on the ingress hook")
		}
		if got := mirredDst(ingress); got != dst0.Attrs().Index {
			t.Errorf("ingress mirror destination = %d, want %d (dst0)", got, dst0.Attrs().Index)
		}
		egress := filterAt(t, "src0", netlink.HANDLE_MIN_EGRESS, testMirrorPriority)
		if egress == nil {
			t.Fatal("no mirror filter on the egress hook")
		}
		if got := mirredDst(egress); got != dst1.Attrs().Index {
			t.Errorf("egress mirror destination = %d, want %d (dst1)", got, dst1.Attrs().Index)
		}
	})
}

func TestIntegrationApplyConfigMirrorRemovedOnConfigDelete(t *testing.T) {
	// VALIDATES: AC-1 end to end -- the operator deletes the mirror leaves,
	// commits, and the kernel stops duplicating traffic.
	// PREVENTS: a removed mirror living on in the kernel, which applyMirror's
	// empty-destination early return guaranteed.
	withNetNS(t, func() {
		b := GetBackend()
		previous := &ifaceConfig{
			Backend: "netlink",
			Dummy: []ifaceEntry{
				{Name: "mir0", Units: []unitEntry{{MirrorIngress: "cap0", MirrorEgress: "cap0"}}},
				{Name: "cap0"},
			},
		}
		t.Cleanup(func() { _ = DeleteInterface("mir0") })
		t.Cleanup(func() { _ = DeleteInterface("cap0") })

		if errs := applyConfig(previous, nil, b); len(errs) > 0 {
			t.Fatalf("apply previous config: %v", errs)
		}
		if filterAt(t, "mir0", netlink.HANDLE_MIN_INGRESS, testMirrorPriority) == nil {
			t.Fatal("the mirror was not installed by the first apply")
		}

		current := &ifaceConfig{
			Backend: "netlink",
			Dummy: []ifaceEntry{
				{Name: "mir0", Units: []unitEntry{{}}},
				{Name: "cap0"},
			},
		}
		if errs := applyConfig(current, previous, b); len(errs) > 0 {
			t.Fatalf("apply current config: %v", errs)
		}

		if filterAt(t, "mir0", netlink.HANDLE_MIN_INGRESS, testMirrorPriority) != nil {
			t.Error("the mirror ingress filter survived a config that no longer asks for it")
		}
		if filterAt(t, "mir0", netlink.HANDLE_MIN_EGRESS, testMirrorPriority) != nil {
			t.Error("the mirror egress filter survived a config that no longer asks for it")
		}
		// A config delete is a RemoveMirror, so it leaves the shared qdisc for the same
		// reason RemoveMirror does. What the operator asked to stop is the duplication,
		// and the two filter assertions above are what prove it stopped.
		if !hasQdisc(t, "mir0", "clsact") {
			t.Error("the config delete took the shared clsact qdisc with the mirror")
		}
	})
}

func TestIntegrationApplyConfigMirrorKeepsForeignFilterOnConfigDelete(t *testing.T) {
	// VALIDATES: AC-1 and AC-2 together -- deleting the mirror from the config
	// leaves an interface that also samples still sampling.
	// PREVENTS: a config commit taking flow-export sampling down as a side
	// effect of a mirror the operator removed.
	withNetNS(t, func() {
		b := GetBackend()
		previous := &ifaceConfig{
			Backend: "netlink",
			Dummy: []ifaceEntry{
				{Name: "mir0", Units: []unitEntry{{MirrorIngress: "cap0"}}},
				{Name: "cap0"},
				{Name: "smp0"},
			},
		}
		t.Cleanup(func() { _ = DeleteInterface("mir0") })
		t.Cleanup(func() { _ = DeleteInterface("cap0") })
		t.Cleanup(func() { _ = DeleteInterface("smp0") })

		if errs := applyConfig(previous, nil, b); len(errs) > 0 {
			t.Fatalf("apply previous config: %v", errs)
		}
		addForeignFilter(t, "mir0")

		current := &ifaceConfig{
			Backend: "netlink",
			Dummy: []ifaceEntry{
				{Name: "mir0", Units: []unitEntry{{}}},
				{Name: "cap0"},
				{Name: "smp0"},
			},
		}
		if errs := applyConfig(current, previous, b); len(errs) > 0 {
			t.Fatalf("apply current config: %v", errs)
		}

		if filterAt(t, "mir0", netlink.HANDLE_MIN_INGRESS, testForeignPriority) == nil {
			t.Error("the foreign filter was removed by a mirror the operator deleted from the config")
		}
		if filterAt(t, "mir0", netlink.HANDLE_MIN_INGRESS, testMirrorPriority) != nil {
			t.Error("the mirror filter survived a config that no longer asks for it")
		}
	})
}

func TestIntegrationListMirrorsReportsTheLiveMirror(t *testing.T) {
	// VALIDATES: the netlink backend reads its own mirror back out of the
	// kernel, naming the source and the destination DEVICE in each direction.
	// PREVENTS: a reconcile deciding from a read that answers "no mirror" for
	// an interface the kernel is copying every packet from. The reconcile then
	// leaves the mirror installed and reports that the dataplane matches the
	// config.
	withNetNS(t, func() {
		createDummyForTest(t, "src0")
		createDummyForTest(t, "dst0")
		b := GetBackend()

		empty, err := b.ListMirrors()
		if err != nil {
			t.Fatalf("ListMirrors before setup: %v", err)
		}
		if len(empty) != 0 {
			t.Fatalf("ListMirrors before setup = %v, want none", empty)
		}

		if err := SetupMirror("src0", "dst0", true, false); err != nil {
			t.Fatalf("SetupMirror(ingress): %v", err)
		}
		t.Cleanup(func() { _ = RemoveMirror("src0") })

		live, err := b.ListMirrors()
		if err != nil {
			t.Fatalf("ListMirrors after setup: %v", err)
		}
		if len(live) != 1 {
			t.Fatalf("ListMirrors after setup = %v, want one entry", live)
		}
		want := MirrorState{Interface: "src0", Ingress: "dst0"}
		if live[0] != want {
			t.Fatalf("ListMirrors after setup = %+v, want %+v", live[0], want)
		}

		if err := RemoveMirror("src0"); err != nil {
			t.Fatalf("RemoveMirror: %v", err)
		}
		gone, err := b.ListMirrors()
		if err != nil {
			t.Fatalf("ListMirrors after removal: %v", err)
		}
		if len(gone) != 0 {
			t.Fatalf("ListMirrors after removal = %v, want none", gone)
		}
	})
}

func TestIntegrationListMirrorsIgnoresAForeignFilter(t *testing.T) {
	// VALIDATES: a filter another subsystem installed on the shared clsact
	// qdisc is not reported as a mirror.
	// PREVENTS: a reconcile retiring flow-export sampling because it read the
	// sampler's filter as a mirror the configuration does not ask for.
	withNetNS(t, func() {
		createDummyForTest(t, "src0")
		createDummyForTest(t, "smp0")
		addClsactQdisc(t, "src0")
		addForeignFilter(t, "src0")

		live, err := GetBackend().ListMirrors()
		if err != nil {
			t.Fatalf("ListMirrors: %v", err)
		}
		if len(live) != 0 {
			t.Fatalf("ListMirrors = %v, want none: the sampler's filter is not a mirror", live)
		}
	})
}

func TestIntegrationApplyConfigRetiresAMirrorNoConfigAsksFor(t *testing.T) {
	// VALIDATES: the restart hole, against a real kernel -- a mirror installed
	// in tc and absent from the configuration is torn down by an apply that has
	// NO previous config to derive it from.
	// PREVENTS: a mirror the operator deleted while ze was down surviving every
	// boot, copying each packet to a destination that no longer exists in the
	// configuration.
	withNetNS(t, func() {
		createDummyForTest(t, "mir0")
		createDummyForTest(t, "cap0")

		// Installed by an earlier ze, whose memory this boot does not have.
		if err := SetupMirror("mir0", "cap0", true, true); err != nil {
			t.Fatalf("SetupMirror: %v", err)
		}
		t.Cleanup(func() { _ = RemoveMirror("mir0") })

		// Both hooks carry a filter BEFORE the apply. Without this the two
		// assertions below are satisfied by a mirror that was never installed,
		// which is the whole failure mode of asserting an absence.
		if filterAt(t, "mir0", netlink.HANDLE_MIN_INGRESS, testMirrorPriority) == nil {
			t.Fatal("the stranded ingress mirror was not installed by the setup")
		}
		if filterAt(t, "mir0", netlink.HANDLE_MIN_EGRESS, testMirrorPriority) == nil {
			t.Fatal("the stranded egress mirror was not installed by the setup")
		}

		current := &ifaceConfig{
			Backend: "netlink",
			Dummy: []ifaceEntry{
				{Name: "mir0", Units: []unitEntry{{}}},
				{Name: "cap0"},
			},
		}
		if errs := applyConfig(current, nil, GetBackend()); len(errs) > 0 {
			t.Fatalf("apply current config: %v", errs)
		}

		if filterAt(t, "mir0", netlink.HANDLE_MIN_INGRESS, testMirrorPriority) != nil {
			t.Error("the ingress mirror filter survived a boot whose configuration does not ask for it")
		}
		if filterAt(t, "mir0", netlink.HANDLE_MIN_EGRESS, testMirrorPriority) != nil {
			t.Error("the egress mirror filter survived a boot whose configuration does not ask for it")
		}
	})
}

func TestIntegrationApplyConfigKeepsAForeignFilterWhileRetiringAStrandedMirror(t *testing.T) {
	// VALIDATES: the kernel-state reconcile removes ze's own filter and leaves
	// every other subsystem's filter on the shared qdisc alone.
	// PREVENTS: the reconcile taking flow-export sampling down as a side effect
	// of retiring a mirror it found stranded in the kernel.
	withNetNS(t, func() {
		createDummyForTest(t, "mir0")
		createDummyForTest(t, "cap0")
		createDummyForTest(t, "smp0")

		if err := SetupMirror("mir0", "cap0", true, false); err != nil {
			t.Fatalf("SetupMirror: %v", err)
		}
		t.Cleanup(func() { _ = RemoveMirror("mir0") })
		addForeignFilter(t, "mir0")

		if filterAt(t, "mir0", netlink.HANDLE_MIN_INGRESS, testMirrorPriority) == nil {
			t.Fatal("the stranded mirror was not installed by the setup")
		}

		current := &ifaceConfig{
			Backend: "netlink",
			Dummy: []ifaceEntry{
				{Name: "mir0", Units: []unitEntry{{}}},
				{Name: "cap0"},
				{Name: "smp0"},
			},
		}
		if errs := applyConfig(current, nil, GetBackend()); len(errs) > 0 {
			t.Fatalf("apply current config: %v", errs)
		}

		if filterAt(t, "mir0", netlink.HANDLE_MIN_INGRESS, testMirrorPriority) != nil {
			t.Error("the stranded mirror filter survived the reconcile")
		}
		if filterAt(t, "mir0", netlink.HANDLE_MIN_INGRESS, testForeignPriority) == nil {
			t.Error("the reconcile removed the sampler's filter along with the mirror")
		}
	})
}

func TestIntegrationApplyConfigLeavesAMirrorOnAnInterfaceItDoesNotConfigure(t *testing.T) {
	// VALIDATES: against a real kernel, the reconcile retires ze's mirror on a
	// configured interface and leaves an identically shaped filter on an
	// interface the configuration does not name.
	// PREVENTS: the kernel-state pass reading priority 1, matchall and mirred
	// as ownership. Another tool installs that same shape, and removing every
	// match would tear an operator's own capture down on every apply.
	withNetNS(t, func() {
		createDummyForTest(t, "mir0")
		createDummyForTest(t, "cap0")
		createDummyForTest(t, "other0")
		createDummyForTest(t, "tap0")

		if err := SetupMirror("mir0", "cap0", true, false); err != nil {
			t.Fatalf("SetupMirror(mir0): %v", err)
		}
		t.Cleanup(func() { _ = RemoveMirror("mir0") })
		// Stands for another tool's capture: the same filter shape, on an
		// interface the configuration below never names.
		if err := SetupMirror("other0", "tap0", true, false); err != nil {
			t.Fatalf("SetupMirror(other0): %v", err)
		}
		t.Cleanup(func() { _ = RemoveMirror("other0") })

		if filterAt(t, "mir0", netlink.HANDLE_MIN_INGRESS, testMirrorPriority) == nil {
			t.Fatal("the configured interface's mirror was not installed by the setup")
		}
		if filterAt(t, "other0", netlink.HANDLE_MIN_INGRESS, testMirrorPriority) == nil {
			t.Fatal("the unmanaged interface's filter was not installed by the setup")
		}

		current := &ifaceConfig{
			Backend: "netlink",
			Dummy: []ifaceEntry{
				{Name: "mir0", Units: []unitEntry{{}}},
				{Name: "cap0"},
			},
		}
		if errs := applyConfig(current, nil, GetBackend()); len(errs) > 0 {
			t.Fatalf("apply current config: %v", errs)
		}

		if filterAt(t, "mir0", netlink.HANDLE_MIN_INGRESS, testMirrorPriority) != nil {
			t.Error("the stranded mirror on a configured interface survived the reconcile")
		}
		if filterAt(t, "other0", netlink.HANDLE_MIN_INGRESS, testMirrorPriority) == nil {
			t.Error("the reconcile removed a filter on an interface the configuration does not name")
		}
	})
}
