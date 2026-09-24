//go:build integration && linux

// Design: docs/architecture/mpls/mpls-kernel.md -- scoped push acceptance and cleanup.
// These tests exercise Apply against a real netlink backend in an isolated netns.
// FIB lookups prove selection; the RSVP packet-path carrier owns on-wire proof.
package fibkernel

import (
	"errors"
	"net"
	"net/netip"
	"slices"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
)

func contextPush(table, label uint32) mplsfibevents.Entry {
	return mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPush,
		FEC: netip.MustParsePrefix("10.9.0.9/32"), TableID: table,
		NextHop: netip.MustParseAddr("10.0.0.2"), OutLabels: []uint32{label}}
}

func contextRoutes(t *testing.T, h *netlink.Handle, table uint32) []netlink.Route {
	t.Helper()
	routes, err := h.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Table: int(table)}, netlink.RT_FILTER_TABLE)
	if err != nil {
		t.Fatal(err)
	}
	return routes
}

func contextRules(t *testing.T, h *netlink.Handle, mark uint32) []netlink.Rule {
	t.Helper()
	rules, err := h.RuleList(netlink.FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	return slices.DeleteFunc(rules, func(rule netlink.Rule) bool { return rule.Mark != mark })
}

func requireContextForwarding(t *testing.T, h *netlink.Handle, entry *mplsfibevents.Entry) {
	t.Helper()
	routes, err := h.RouteGetWithOptions(entry.FEC.Addr().AsSlice(), &netlink.RouteGetOptions{
		Mark: entry.TableID, FIBMatch: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatalf("marked lookup returned %d routes", len(routes))
	}
	route := &routes[0]
	encap, ok := route.Encap.(*netlink.MPLSEncap)
	if !ok {
		t.Fatalf("marked lookup selected an unlabeled route: %+v", route)
	}
	if route.Table != int(entry.TableID) || !route.Gw.Equal(entry.NextHop.AsSlice()) ||
		!slices.Equal(encap.Labels, []int{int(entry.OutLabels[0])}) {
		t.Fatalf("marked lookup selected %+v, labels %v; want table %d next hop %s label %d",
			route, encap.Labels, entry.TableID, entry.NextHop, entry.OutLabels[0])
	}
}

func requireContextBlocked(t *testing.T, h *netlink.Handle, entry *mplsfibevents.Entry) {
	t.Helper()
	_, err := h.RouteGetWithOptions(entry.FEC.Addr().AsSlice(), &netlink.RouteGetOptions{Mark: entry.TableID})
	if !errors.Is(err, unix.ENETUNREACH) {
		t.Fatalf("withdrawn context lookup error = %v, want ENETUNREACH", err)
	}
}

// TestMPLSIntegration_ScopedContexts distinguishes two bypasses with the same
// merge point by mark, then withdraws one without changing the other or main FIB.
func TestMPLSIntegration_ScopedContexts(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		if err != nil {
			t.Fatal(err)
		}
		defer h.Close()
		enableNetnsMPLS(t)
		setupDummyLink(t, h)
		backend := &netlinkBackend{handle: h}
		defer func() {
			if closeErr := backend.close(); closeErr != nil {
				t.Error(closeErr)
			}
		}()
		bus := newMPLSApplyBus(t, newFIBKernel(backend))
		first, second := contextPush(0x5a000001, 300), contextPush(0x5a000002, 400)
		second.NextHop = netip.MustParseAddr("10.0.0.3")
		addProtocolRoute(t, h, first.FEC.String(), "10.0.0.4", 100)
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{first, second}); err != nil {
			t.Fatal(err)
		}
		requireContextForwarding(t, h, &first)
		requireContextForwarding(t, h, &second)
		for _, entry := range []mplsfibevents.Entry{first, second} {
			rules := contextRules(t, h, entry.TableID)
			if len(rules) != 1 || rules[0].Mask == nil || *rules[0].Mask != 0xffffffff || rules[0].Table != int(entry.TableID) {
				t.Fatalf("context %#x rules = %+v", entry.TableID, rules)
			}
		}
		ordinary, err := h.RouteGetWithOptions(first.FEC.Addr().AsSlice(), &netlink.RouteGetOptions{FIBMatch: true})
		if err != nil || len(ordinary) != 1 || ordinary[0].Table != unix.RT_TABLE_MAIN || ordinary[0].Encap != nil {
			t.Fatalf("unmarked route changed: %+v, %v", ordinary, err)
		}
		first.OutLabels[0] = 500
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{first}); err != nil {
			t.Fatal(err)
		}
		requireContextForwarding(t, h, &first)
		first.Action = mplsfibevents.ActionRemove
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{first}); err != nil {
			t.Fatal(err)
		}
		if len(contextRoutes(t, h, first.TableID)) != 0 || len(contextRules(t, h, first.TableID)) != 0 {
			t.Fatal("withdrawn context retained its route or rule")
		}
		requireContextBlocked(t, h, &first)
		requireContextForwarding(t, h, &second)
		second.Action = mplsfibevents.ActionRemove
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{second}); err != nil {
			t.Fatal(err)
		}
		// The ordinary route remains usable without a mark. With no private
		// contexts left, both stale marks must still hit the shared guard.
		requireContextBlocked(t, h, &first)
		requireContextBlocked(t, h, &second)
	})
}

// TestMPLSIntegration_ScopedRuleFailureRollback makes selector installation fail
// after route creation, then verifies the route is gone and the foreign rule survives.
func TestMPLSIntegration_ScopedRuleFailureRollback(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		if err != nil {
			t.Fatal(err)
		}
		defer h.Close()
		enableNetnsMPLS(t)
		setupDummyLink(t, h)
		backend := &netlinkBackend{handle: h}
		defer func() {
			if closeErr := backend.close(); closeErr != nil {
				t.Error(closeErr)
			}
		}()
		bus := newMPLSApplyBus(t, newFIBKernel(backend))
		entry := contextPush(0x5a000001, 300)
		mask := uint32(0xffffffff)
		foreign := netlink.NewRule()
		foreign.Family, foreign.Priority, foreign.Table = netlink.FAMILY_V4, 1, 123
		foreign.Mark, foreign.Mask, foreign.Protocol = entry.TableID, &mask, 100
		if err := h.RuleAdd(foreign); err != nil {
			t.Fatal(err)
		}
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}); !errors.Is(err, unix.EEXIST) {
			t.Fatalf("Apply conflict error = %v, want EEXIST", err)
		}
		if routes := contextRoutes(t, h, entry.TableID); len(routes) != 0 {
			t.Fatalf("failed context leaked routes: %+v", routes)
		}
		if rules := contextRules(t, h, entry.TableID); len(rules) != 1 || rules[0].Protocol != 100 || rules[0].Table != 123 {
			t.Fatalf("failed context changed foreign rule: %+v", rules)
		}
		if err := h.RuleDel(foreign); err != nil {
			t.Fatal(err)
		}
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}); err != nil {
			t.Fatal(err)
		}
		requireContextForwarding(t, h, &entry)
	})
}

// TestMPLSIntegration_ScopedForeignRoute preserves another writer's private-table
// route across a refused add and a compensating remove, then permits a clean retry.
func TestMPLSIntegration_ScopedForeignRoute(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		if err != nil {
			t.Fatal(err)
		}
		defer h.Close()
		enableNetnsMPLS(t)
		setupDummyLink(t, h)
		backend := &netlinkBackend{handle: h}
		defer func() {
			if closeErr := backend.close(); closeErr != nil {
				t.Error(closeErr)
			}
		}()
		bus := newMPLSApplyBus(t, newFIBKernel(backend))
		entry := contextPush(0x5a000001, 300)
		link, err := h.LinkByName("ze-mpls0")
		if err != nil {
			t.Fatal(err)
		}
		foreign := &netlink.Route{Dst: &net.IPNet{IP: entry.FEC.Addr().AsSlice(), Mask: net.CIDRMask(32, 32)},
			Table: int(entry.TableID), LinkIndex: link.Attrs().Index, Protocol: 100, Scope: netlink.SCOPE_LINK}
		if err := h.RouteAdd(foreign); err != nil {
			t.Fatal(err)
		}
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}); !errors.Is(err, unix.EEXIST) {
			t.Fatalf("Apply conflict error = %v, want EEXIST", err)
		}
		entry.Action = mplsfibevents.ActionRemove
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}); err != nil {
			t.Fatal(err)
		}
		if routes := contextRoutes(t, h, entry.TableID); len(routes) != 1 || routes[0].Protocol != 100 || routes[0].Encap != nil {
			t.Fatalf("compensating remove changed foreign route: %+v", routes)
		}
		if err := h.RouteDel(foreign); err != nil {
			t.Fatal(err)
		}
		entry.Action = mplsfibevents.ActionAdd
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}); err != nil {
			t.Fatal(err)
		}
		requireContextForwarding(t, h, &entry)
	})
}

// TestMPLSIntegration_ScopedShutdown verifies backend close removes private
// contexts without letting late marked traffic use ordinary or foreign routes.
func TestMPLSIntegration_ScopedShutdown(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		if err != nil {
			t.Fatal(err)
		}
		enableNetnsMPLS(t)
		setupDummyLink(t, h)
		observer, err := netlink.NewHandle()
		if err != nil {
			h.Close()
			t.Fatal(err)
		}
		defer observer.Close()
		backend := &netlinkBackend{handle: h}
		bus := newMPLSApplyBus(t, newFIBKernel(backend))
		entry := contextPush(0x5a000001, 300)
		addProtocolRoute(t, observer, entry.FEC.String(), "10.0.0.4", 100)
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}); err != nil {
			t.Fatal(err)
		}
		requireContextForwarding(t, observer, &entry)
		if err := backend.close(); err != nil {
			t.Fatal(err)
		}
		if len(contextRoutes(t, observer, entry.TableID)) != 0 || len(contextRules(t, observer, entry.TableID)) != 0 {
			t.Fatal("backend close leaked scoped routes or selectors")
		}
		requireContextBlocked(t, observer, &entry)
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}); err == nil {
			t.Fatal("closed backend acknowledged a new context")
		}
		ordinary, err := observer.RouteGetWithOptions(entry.FEC.Addr().AsSlice(), &netlink.RouteGetOptions{FIBMatch: true})
		if err != nil || len(ordinary) != 1 || ordinary[0].Protocol != 100 {
			t.Fatalf("shutdown changed foreign ordinary route: %+v, %v", ordinary, err)
		}
		restartedHandle, err := netlink.NewHandle()
		if err != nil {
			t.Fatal(err)
		}
		restarted := &netlinkBackend{handle: restartedHandle}
		defer func() {
			if err := restarted.close(); err != nil {
				t.Error(err)
			}
		}()
		restartedBus := newMPLSApplyBus(t, newFIBKernel(restarted))
		if err := mplsfibevents.Apply(restartedBus, []mplsfibevents.Entry{entry}); err != nil {
			t.Fatalf("restarted owner cannot reuse the namespace guard: %v", err)
		}
		requireContextForwarding(t, observer, &entry)
		if err := restarted.close(); err != nil {
			t.Fatal(err)
		}
		requireContextBlocked(t, observer, &entry)
	})
}

// TestMPLSIntegration_ScopedSharedTable retains a table's selector until its last
// FEC is removed. Removing a route after link failure must still clean it up.
func TestMPLSIntegration_ScopedSharedTable(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		if err != nil {
			t.Fatal(err)
		}
		defer h.Close()
		enableNetnsMPLS(t)
		setupDummyLink(t, h)
		backend := &netlinkBackend{handle: h}
		defer func() {
			if closeErr := backend.close(); closeErr != nil {
				t.Error(closeErr)
			}
		}()
		bus := newMPLSApplyBus(t, newFIBKernel(backend))
		first, second := contextPush(0x5a000001, 300), contextPush(0x5a000001, 400)
		second.FEC = netip.MustParsePrefix("10.9.0.10/32")
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{first, second}); err != nil {
			t.Fatal(err)
		}
		requireContextForwarding(t, h, &first)
		requireContextForwarding(t, h, &second)
		first.Action = mplsfibevents.ActionRemove
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{first}); err != nil {
			t.Fatal(err)
		}
		requireContextBlocked(t, h, &first)
		requireContextForwarding(t, h, &second)
		if rules := contextRules(t, h, second.TableID); len(rules) != 1 {
			t.Fatalf("shared selector count = %d, want 1", len(rules))
		}
		link, err := h.LinkByName("ze-mpls0")
		if err != nil {
			t.Fatal(err)
		}
		if err := h.LinkSetDown(link); err != nil {
			t.Fatal(err)
		}
		second.Action = mplsfibevents.ActionRemove
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{second}); err != nil {
			t.Fatal(err)
		}
		if len(contextRoutes(t, h, second.TableID)) != 0 || len(contextRules(t, h, second.TableID)) != 0 {
			t.Fatal("last FEC withdrawal retained its private route or selector")
		}
	})
}
