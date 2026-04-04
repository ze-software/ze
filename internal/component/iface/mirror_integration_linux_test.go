//go:build integration && linux

package iface

import (
	"testing"

	"github.com/vishvananda/netlink"

)

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

// countQdiscs returns the number of qdiscs on a link (excluding the default).
func countQdiscs(t *testing.T, linkName string) int {
	t.Helper()
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		t.Fatalf("LinkByName(%q): %v", linkName, err)
	}
	qdiscs, err := netlink.QdiscList(link)
	if err != nil {
		t.Fatalf("QdiscList(%q): %v", linkName, err)
	}
	return len(qdiscs)
}

func TestIntegrationMirrorIngress(t *testing.T) {
	// VALIDATES: SetupMirror with ingress=true installs an ingress qdisc.
	// PREVENTS: Ingress mirroring silently fails to configure tc.
	withNetNS(t, func() {
		createDummyForTest(t, "src0")
		createDummyForTest(t, "dst0")

		if err := SetupMirror("src0", "dst0", true, false); err != nil {
			t.Fatalf("SetupMirror(ingress): %v", err)
		}
		t.Cleanup(func() { _ = RemoveMirror("src0") })

		if !hasQdisc(t, "src0", "ingress") {
			t.Error("expected ingress qdisc on src0, not found")
		}
	})
}

func TestIntegrationMirrorEgress(t *testing.T) {
	// VALIDATES: SetupMirror with egress=true installs a clsact qdisc.
	// PREVENTS: Egress mirroring silently fails to configure tc.
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
	// VALIDATES: RemoveMirror removes mirroring qdiscs from the interface.
	// PREVENTS: Stale mirroring configuration after removal.
	withNetNS(t, func() {
		createDummyForTest(t, "src0")
		createDummyForTest(t, "dst0")

		if err := SetupMirror("src0", "dst0", true, false); err != nil {
			t.Fatalf("SetupMirror: %v", err)
		}

		if !hasQdisc(t, "src0", "ingress") {
			t.Fatal("ingress qdisc should exist before removal")
		}

		if err := RemoveMirror("src0"); err != nil {
			t.Fatalf("RemoveMirror: %v", err)
		}

		// After removal, no ingress or clsact qdisc should remain.
		if hasQdisc(t, "src0", "ingress") {
			t.Error("ingress qdisc still present after RemoveMirror")
		}
		if hasQdisc(t, "src0", "clsact") {
			t.Error("clsact qdisc still present after RemoveMirror")
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
