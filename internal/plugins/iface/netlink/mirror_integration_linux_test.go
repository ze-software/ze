//go:build integration && linux

// Design: docs/features/interfaces.md -- the mirror shares the clsact qdisc, so
// it owns only its filters. These tests drive setupClsactMirror against a real
// kernel with a destination the kernel refuses, which is the only deterministic
// way to reach the rollback path: SetupMirror resolves its destination by name
// and would refuse a bogus one before any filter is added.

package ifacenetlink

import (
	"runtime"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

const (
	// mirrorTestForeignPriority is the tc filter priority flow-export sampling
	// owns (SampleFilterPriority in internal/plugins/flowexport/sampling). A
	// filter here stands for every subsystem that shares the clsact qdisc.
	mirrorTestForeignPriority uint16 = 100
	// mirrorTestNoSuchIfindex is an ifindex no device carries, so the kernel
	// refuses a mirred action pointing at it with ENODEV.
	mirrorTestNoSuchIfindex = 999999
)

// withMirrorNetNS runs fn inside a fresh named network namespace so tc state
// cannot collide with host links. Skips (not fails) without CAP_NET_ADMIN per
// ai/rules/platform-linux.md.
func withMirrorNetNS(t *testing.T, fn func()) {
	t.Helper()

	runtime.LockOSThread()
	unlocked := false
	unlock := func() {
		if !unlocked {
			runtime.UnlockOSThread()
			unlocked = true
		}
	}

	origNS, err := netns.Get()
	if err != nil {
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: cannot get current namespace: %v", err)
	}

	nsName := mirrorNetNSName(t.Name())
	newNS, err := netns.NewNamed(nsName)
	if err != nil {
		origNS.Close()
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: cannot create namespace: %v", err)
	}

	t.Cleanup(func() {
		if restoreErr := netns.Set(origNS); restoreErr != nil {
			t.Errorf("failed to restore original namespace: %v", restoreErr)
		}
		origNS.Close()
		newNS.Close()
		netns.DeleteNamed(nsName) //nolint:errcheck // best-effort cleanup
		unlock()
	})

	fn()
}

func mirrorNetNSName(testName string) string {
	name := strings.NewReplacer("/", "_", " ", "_", "(", "", ")", "").Replace(testName)
	if len(name) > 8 {
		name = name[len(name)-8:]
	}
	return "zemir_" + name
}

func addMirrorTestDummy(t *testing.T, name string) netlink.Link {
	t.Helper()

	if err := netlink.LinkAdd(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: name}}); err != nil {
		t.Fatalf("add dummy %q: %v", name, err)
	}
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("link %q: %v", name, err)
	}
	return link
}

// addForeignTestFilter attaches a filter belonging to another subsystem to the
// shared qdisc, at the priority flow-export sampling uses.
func addForeignTestFilter(t *testing.T, src netlink.Link, dstIndex int) {
	t.Helper()

	filter := &netlink.MatchAll{
		FilterAttrs: netlink.FilterAttrs{
			LinkIndex: src.Attrs().Index,
			Parent:    netlink.HANDLE_MIN_INGRESS,
			Priority:  mirrorTestForeignPriority,
			Protocol:  unix.ETH_P_ALL,
		},
		Actions: []netlink.Action{
			&netlink.MirredAction{
				ActionAttrs:  netlink.ActionAttrs{Action: netlink.TC_ACT_PIPE},
				MirredAction: netlink.TCA_EGRESS_MIRROR,
				Ifindex:      dstIndex,
			},
		},
	}
	if err := netlink.FilterAdd(filter); err != nil {
		t.Fatalf("add foreign filter on %q: %v", src.Attrs().Name, err)
	}
}

func mirrorTestFilterCount(t *testing.T, link netlink.Link, parent uint32) int {
	t.Helper()

	filters, err := netlink.FilterList(link, parent)
	if err != nil {
		t.Fatalf("FilterList(%q): %v", link.Attrs().Name, err)
	}
	return len(filters)
}

func mirrorTestHasQdisc(t *testing.T, link netlink.Link, qdiscType string) bool {
	t.Helper()

	qdiscs, err := netlink.QdiscList(link)
	if err != nil {
		t.Fatalf("QdiscList(%q): %v", link.Attrs().Name, err)
	}
	for _, q := range qdiscs {
		if q.Type() == qdiscType {
			return true
		}
	}
	return false
}

func TestIntegrationMirrorSetupRollbackKeepsForeignFilter(t *testing.T) {
	// VALIDATES: AC-5 -- a mirror setup that fails leaves the qdisc it did not
	// create, with every foreign filter still attached to it.
	// PREVENTS: the rollback path calling QdiscDel on the shared qdisc, which
	// took flow-export sampling down as a side effect of undoing a mirror.
	withMirrorNetNS(t, func() {
		src := addMirrorTestDummy(t, "msrc0")
		smp := addMirrorTestDummy(t, "msmp0")

		if _, err := ensureClsactQdisc(src.Attrs().Index); err != nil {
			t.Fatalf("pre-create the shared clsact qdisc: %v", err)
		}
		addForeignTestFilter(t, src, smp.Attrs().Index)

		if err := setupClsactMirror(src, mirrorTestNoSuchIfindex, true, true); err == nil {
			t.Fatal("setupClsactMirror succeeded with a destination no device carries")
		}

		if !mirrorTestHasQdisc(t, src, "clsact") {
			t.Error("the rollback deleted a clsact qdisc the mirror did not create")
		}
		if got := mirrorTestFilterCount(t, src, netlink.HANDLE_MIN_INGRESS); got != 1 {
			t.Errorf("ingress hook carries %d filters after the rollback, want 1 (the foreign one)", got)
		}
		if got := mirrorTestFilterCount(t, src, netlink.HANDLE_MIN_EGRESS); got != 0 {
			t.Errorf("egress hook carries %d filters after the rollback, want 0", got)
		}
	})
}

func TestIntegrationMirrorSetupRollbackRemovesTheQdiscItCreated(t *testing.T) {
	// VALIDATES: AC-5 -- a failed setup that created the qdisc removes it, so
	// a failed apply leaves no tc state behind.
	// PREVENTS: trading the qdisc-deletion defect for a qdisc leak.
	withMirrorNetNS(t, func() {
		src := addMirrorTestDummy(t, "msrc0")

		if err := setupClsactMirror(src, mirrorTestNoSuchIfindex, true, false); err == nil {
			t.Fatal("setupClsactMirror succeeded with a destination no device carries")
		}

		if mirrorTestHasQdisc(t, src, "clsact") {
			t.Error("the rollback left behind the clsact qdisc the failed setup created")
		}
	})
}

func TestIntegrationMirrorRemoveKeepsTheQdiscOfAnotherSubsystem(t *testing.T) {
	// VALIDATES: AC-2 and AC-3 -- teardown removes the qdisc when the mirror
	// was its last user, and keeps it when another subsystem still holds a
	// filter on it.
	// PREVENTS: both halves of the ownership rule regressing to "always delete"
	// or to "never delete".
	withMirrorNetNS(t, func() {
		src := addMirrorTestDummy(t, "msrc0")
		dst := addMirrorTestDummy(t, "mdst0")
		smp := addMirrorTestDummy(t, "msmp0")
		b := &netlinkBackend{}

		if err := b.SetupMirror("msrc0", "mdst0", true, true); err != nil {
			t.Fatalf("SetupMirror: %v", err)
		}
		addForeignTestFilter(t, src, smp.Attrs().Index)

		if err := b.RemoveMirror("msrc0"); err != nil {
			t.Fatalf("RemoveMirror with a foreign filter attached: %v", err)
		}
		if !mirrorTestHasQdisc(t, src, "clsact") {
			t.Fatal("the shared qdisc was deleted while a foreign filter was attached")
		}
		if got := mirrorTestFilterCount(t, src, netlink.HANDLE_MIN_INGRESS); got != 1 {
			t.Errorf("ingress hook carries %d filters, want 1 (the foreign one)", got)
		}

		// Remove the foreign filter, install and remove the mirror again: the
		// mirror is now the last user, so the qdisc goes with it.
		if err := netlink.FilterDel(&netlink.MatchAll{
			FilterAttrs: netlink.FilterAttrs{
				LinkIndex: src.Attrs().Index,
				Parent:    netlink.HANDLE_MIN_INGRESS,
				Priority:  mirrorTestForeignPriority,
				Protocol:  unix.ETH_P_ALL,
			},
		}); err != nil {
			t.Fatalf("remove the foreign filter: %v", err)
		}
		if err := b.SetupMirror("msrc0", dst.Attrs().Name, true, true); err != nil {
			t.Fatalf("SetupMirror second pass: %v", err)
		}
		if err := b.RemoveMirror("msrc0"); err != nil {
			t.Fatalf("RemoveMirror as the last user: %v", err)
		}
		if mirrorTestHasQdisc(t, src, "clsact") {
			t.Error("the qdisc survived the removal of its last user")
		}
	})
}
