// VALIDATES: AC-2 / C-2 -- resolveNexthopIndex, when no iface backend is loaded,
// returns an error that names BOTH the next-hop interface and the missing
// `interface { backend ... }` stanza, instead of the bare "iface: no backend
// loaded" the shared resolver produces.
// PREVENTS: an operator seeing an unactionable error and not knowing an
// interface backend stanza is required for interface-only static next-hops.

//go:build linux

package static

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestStaticRemoveRouteToleratesESRCH is A-7 for static's own removal owner:
// when a device is deleted the kernel flushes every route through it, and the
// removal static sends next must be done, not a failure. Method, in a user
// and network namespace with no iface backend loaded: (1) a named-table route
// is installed through a dummy device, the device is deleted (the kernel
// flushes the route), and removeRoute of the route as static last installed
// it, naming that device, must succeed; (2) a route still present is removed
// although the member it names resolves to no device, and the kernel must no
// longer hold it.
// PREVENTS: a session drop logged as a removal failure, the removal failing
// because it resolves a device that is gone, and a key-only removal that
// leaves the route installed.
func TestStaticRemoveRouteToleratesESRCH(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	if iface.ActiveBackendName() != "" {
		_ = iface.CloseBackend()
	}
	withStaticNetNS(t, func() {
		h, err := netlink.NewHandle()
		if err != nil {
			t.Fatalf("netlink handle: %v", err)
		}
		defer h.Close()
		b := &netlinkStaticBackend{handle: h}

		const dev = "zeesrch0" // unique: no iface backend resolves it
		dummy := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: dev}}
		if err := h.LinkAdd(dummy); err != nil {
			t.Fatalf("create dummy %s: %v", dev, err)
		}
		link, err := h.LinkByName(dev)
		if err != nil {
			t.Fatalf("lookup %s: %v", dev, err)
		}
		if err := h.LinkSetUp(link); err != nil {
			t.Fatalf("set %s up: %v", dev, err)
		}

		const table = 100
		flushed := netip.MustParsePrefix("10.99.70.0/24")
		if err := h.RouteAdd(&netlink.Route{
			Dst: prefixToIPNet(flushed), LinkIndex: link.Attrs().Index,
			Protocol: rtprotStatic, Table: table,
		}); err != nil {
			t.Fatalf("install %v through %s: %v", flushed, dev, err)
		}
		if err := h.LinkDel(link); err != nil {
			t.Fatalf("delete %s: %v", dev, err)
		}
		if got := staticRoutesIn(t, h, table, flushed); got != 0 {
			t.Fatalf("kernel holds %d routes for %v after deleting %s, want the flush A-7 assumes", got, flushed, dev)
		}
		installed := staticRoute{
			Prefix: flushed, Table: table, Action: actionForward,
			NextHops: []nextHop{{Interface: dev, Weight: 1, Dynamic: true}},
		}
		if err := b.removeRoute(installed); err != nil {
			t.Errorf("removeRoute of a route the kernel flushed with its device: %v, want done", err)
		}

		present := netip.MustParsePrefix("10.99.71.0/24")
		lo, err := h.LinkByName("lo")
		if err != nil {
			t.Fatalf("lookup lo: %v", err)
		}
		if err := h.LinkSetUp(lo); err != nil {
			t.Fatalf("set lo up: %v", err)
		}
		if err := h.RouteAdd(&netlink.Route{
			Dst: prefixToIPNet(present), LinkIndex: lo.Attrs().Index,
			Protocol: rtprotStatic, Table: table, Type: unix.RTN_UNICAST,
		}); err != nil {
			t.Fatalf("install %v through lo: %v", present, err)
		}
		gone := staticRoute{
			Prefix: present, Table: table, Action: actionForward,
			NextHops: []nextHop{{Interface: dev, Weight: 1, Dynamic: true}},
		}
		if err := b.removeRoute(gone); err != nil {
			t.Errorf("removeRoute naming a member that no longer resolves: %v, want the route removed", err)
		}
		if got := staticRoutesIn(t, h, table, present); got != 0 {
			t.Errorf("kernel still holds %d routes for %v after removeRoute", got, present)
		}
	})
}

// staticRoutesIn counts the proto-static routes the kernel holds for prefix in
// table.
func staticRoutesIn(t *testing.T, h *netlink.Handle, table int, prefix netip.Prefix) int {
	t.Helper()
	filter := &netlink.Route{Table: table, Dst: prefixToIPNet(prefix)}
	routes, err := h.RouteListFiltered(netlink.FAMILY_V4, filter, netlink.RT_FILTER_TABLE|netlink.RT_FILTER_DST)
	if err != nil {
		t.Fatalf("list table %d: %v", table, err)
	}
	n := 0
	for i := range routes {
		if routes[i].Protocol == rtprotStatic {
			n++
		}
	}
	return n
}

func TestResolveNexthopIndexNoBackendErrorIsDiagnosable(t *testing.T) {
	// Ensure the no-backend precondition regardless of test ordering.
	if iface.ActiveBackendName() != "" {
		_ = iface.CloseBackend()
	}

	const dev = "nostaticbackend0" // unique: iface.Resolve caches per logical name
	_, err := resolveNexthopIndex(dev)
	if err == nil {
		t.Fatal("resolveNexthopIndex: want an error with no iface backend loaded")
	}
	msg := err.Error()
	if !strings.Contains(msg, dev) {
		t.Errorf("error must name the interface %q: %q", dev, msg)
	}
	if !strings.Contains(msg, "interface { backend") {
		t.Errorf("error must name the missing `interface { backend ... }` stanza: %q", msg)
	}
}
