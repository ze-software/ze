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
func addForeignFilter(t *testing.T, srcName, dstName string, parent uint32) {
	t.Helper()
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

// test-relax: countQdiscs is gone. It was a HELPER and it never had a caller:
// `git log -S countQdiscs` returns only ad18e8dd9, the commit that added it.
// Deleting never-called test code drops no coverage, so this token records a
// deletion rather than excusing a relaxation. It is here because the helper
// carried two fatal-on-failure calls of its own, which the edit hook counts as
// assertions going to zero. Nothing was given up.

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

		// test-relax: the pre-condition asserted an "ingress" qdisc, which is
		// the mechanism an ingress-only mirror used before this spec. Ze now
		// installs clsact for every mirror, so the qdisc kind no longer says
		// whether a mirror is installed. The filter does, and that is what
		// this now asserts.
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
		// test-relax: this asserted the clsact qdisc was GONE, which is the
		// contract AC-3 carried until 2026-08-14. Teardown is filter-scoped
		// now and deliberately leaves the qdisc, so the assertion is inverted
		// rather than dropped: deleting the shared qdisc here reds this test.
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
		addForeignFilter(t, "src0", "smp0", netlink.HANDLE_MIN_INGRESS)

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
		addForeignFilter(t, "src0", "smp0", netlink.HANDLE_MIN_INGRESS)

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
		addForeignFilter(t, "src0", "smp0", netlink.HANDLE_MIN_INGRESS)

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

// test-relax: AC-5's rollback test was drafted here and moved, unweakened, to
// internal/plugins/iface/netlink/mirror_integration_linux_test.go
// (TestIntegrationMirrorSetupRollbackKeepsForeignFilter). It needed a mirror
// setup that fails after its first filter lands. From this package the only
// lever is a pre-existing ingress qdisc, and sch_ingress's ingress_find accepts
// any minor handle, so the egress filter add would succeed instead of failing.
// The netlink package can pass an unresolvable destination ifindex to
// setupClsactMirror, which the kernel refuses with ENODEV. Same assertions,
// deterministic failure.

func TestIntegrationMirrorSetupIsIdempotent(t *testing.T) {
	// VALIDATES: AC-7 -- applying the same mirror twice succeeds and leaves
	// one filter per hook.
	// PREVENTS: a second commit of an unchanged config failing with EEXIST, or
	// stacking a second mirred filter on the same hook.
	withNetNS(t, func() {
		createDummyForTest(t, "src0")
		createDummyForTest(t, "dst0")

		for i := 0; i < 2; i++ {
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
		// test-relax: this asserted the qdisc was gone. A config delete is a
		// RemoveMirror, so it leaves the shared qdisc for the same reason
		// RemoveMirror does. What the operator asked to stop is the
		// duplication, and the two filter assertions above are what prove it
		// stopped.
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
		addForeignFilter(t, "mir0", "smp0", netlink.HANDLE_MIN_INGRESS)

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
