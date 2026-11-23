//go:build linux

// Design: docs/architecture/core-design.md -- Linux netlink backend ownership, ESRCH, and partial-apply tests
// Related: backend_linux.go -- delOwnedRoute, replaceOwnedRoute
// Related: nexthop_linux.go -- delRichRoute, replaceRichRoute, wrapSEG6InstallError

package fibkernel

import (
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
)

type fakeJournalNetlinkHandle struct {
	netlinkHandle
	listed     []netlink.Route
	added      []netlink.Route
	replaced   []netlink.Route
	deleted    []netlink.Route
	addErr     error
	replaceErr error
	delErr     error
	delFn      func(route *netlink.Route) error
}

func (h *fakeJournalNetlinkHandle) RouteAdd(route *netlink.Route) error {
	if h.addErr != nil {
		return h.addErr
	}
	h.added = append(h.added, *route)
	return nil
}

func (h *fakeJournalNetlinkHandle) RouteReplace(route *netlink.Route) error {
	if h.replaceErr != nil {
		return h.replaceErr
	}
	h.replaced = append(h.replaced, *route)
	return nil
}

func (h *fakeJournalNetlinkHandle) RouteDel(route *netlink.Route) error {
	h.deleted = append(h.deleted, *route)
	if h.delFn != nil {
		return h.delFn(route)
	}
	return h.delErr
}

func (h *fakeJournalNetlinkHandle) RouteListFiltered(_ int, _ *netlink.Route, _ uint64) ([]netlink.Route, error) {
	out := make([]netlink.Route, len(h.listed))
	copy(out, h.listed)
	return out, nil
}

// VALIDATES: delOwnedRoute, netlinkBackend.delRoute, and netlinkBackend.delRichRoute
// distinguish a real kernel delete (true, nil) from ESRCH (false, nil) and other
// kernel errors (false, err).
// PREVENTS: ESRCH being counted as a removed orphan route during startup sweep.
func TestDelOwnedRouteDistinguishesESRCHFromDelete(t *testing.T) {
	route, err := ownedRouteKey("198.18.180.0/24", 0)
	if err != nil {
		t.Fatalf("ownedRouteKey: %v", err)
	}

	t.Run("deleted route returns true and nil", func(t *testing.T) {
		handle := &fakeJournalNetlinkHandle{}
		removed, delErr := delOwnedRoute(handle, route)
		if delErr != nil {
			t.Fatalf("delOwnedRoute error = %v, want nil", delErr)
		}
		if !removed {
			t.Fatal("delOwnedRoute removed = false, want true")
		}
		backend := &netlinkBackend{handle: handle}
		removed, delErr = backend.delRoute("198.18.180.0/24")
		if delErr != nil || !removed {
			t.Fatalf("delRoute = (%v, %v), want (true, nil)", removed, delErr)
		}
		removed, delErr = backend.delRichRoute(netip.MustParsePrefix("198.18.180.0/24"), 100)
		if delErr != nil || !removed {
			t.Fatalf("delRichRoute = (%v, %v), want (true, nil)", removed, delErr)
		}
	})

	t.Run("ESRCH returns false and nil", func(t *testing.T) {
		handle := &fakeJournalNetlinkHandle{delErr: unix.ESRCH}
		removed, delErr := delOwnedRoute(handle, route)
		if delErr != nil {
			t.Fatalf("delOwnedRoute error = %v, want nil on ESRCH", delErr)
		}
		if removed {
			t.Fatal("delOwnedRoute removed = true on ESRCH, want false")
		}
		backend := &netlinkBackend{handle: handle}
		removed, delErr = backend.delRoute("198.18.180.0/24")
		if delErr != nil || removed {
			t.Fatalf("delRoute = (%v, %v), want (false, nil) on ESRCH", removed, delErr)
		}
		removed, delErr = backend.delRichRoute(netip.MustParsePrefix("198.18.180.0/24"), 100)
		if delErr != nil || removed {
			t.Fatalf("delRichRoute = (%v, %v), want (false, nil) on ESRCH", removed, delErr)
		}
	})

	t.Run("non-ESRCH error returns false and error", func(t *testing.T) {
		handle := &fakeJournalNetlinkHandle{delErr: unix.EPERM}
		removed, delErr := delOwnedRoute(handle, route)
		if !errors.Is(delErr, unix.EPERM) {
			t.Fatalf("delOwnedRoute error = %v, want EPERM", delErr)
		}
		if removed {
			t.Fatal("delOwnedRoute removed = true on EPERM, want false")
		}
	})
}

// VALIDATES: replaceOwnedRoute attempts every superseded delete when a metric
// change leaves old Ze routes behind, returns *supersededDeleteError without
// wrapSEG6InstallError misclassifying it, records the newly installed route in
// fibKernel, keeps supersededTables untouched by sweepStale, and deletes both
// the new route and every superseded route on flushRoutes.
// PREVENTS: partial apply discarding ownership of the newly installed route or
// leaking superseded routes on shutdown flush.
func TestReplaceOwnedRoutePartialApplyRecordsNewRouteAndRetainsSupersededForFlush(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	prefix := netip.MustParsePrefix("198.18.181.0/24")
	_, cidr, err := net.ParseCIDR(prefix.String())
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}

	handle := &fakeJournalNetlinkHandle{
		listed: []netlink.Route{
			{Dst: cidr, Protocol: rtprotZE, Priority: 10, Table: unix.RT_TABLE_MAIN},
			{Dst: cidr, Protocol: rtprotZE, Priority: 20, Table: unix.RT_TABLE_MAIN},
		},
		delErr: unix.EPERM,
	}
	backend := &netlinkBackend{handle: handle}
	fib := newFIBKernel(backend)
	fib.installed[prefix.String()] = "192.0.2.1"
	fib.forwarding[prefix.String()] = incomingChange{
		Action:  routeaction.Add,
		Prefix:  prefix,
		NextHop: netip.MustParseAddr("192.0.2.1"),
		Metric:  10,
	}

	// Verify wrapSEG6InstallError passes *supersededDeleteError through unchanged
	// even if RouteDel returned EOPNOTSUPP on a SEG6 route.
	seg6Handle := &fakeJournalNetlinkHandle{
		listed: []netlink.Route{{Dst: cidr, Protocol: rtprotZE, Priority: 10, Table: unix.RT_TABLE_MAIN}},
		delErr: unix.EOPNOTSUPP,
	}
	seg6Backend := &netlinkBackend{handle: seg6Handle}
	seg6Err := seg6Backend.replaceRichRoute(RichRoute{
		Prefix:  prefix,
		NextHop: netip.MustParseAddr("192.0.2.2"),
		Metric:  100,
		SRv6SID: netip.MustParseAddr("2001:db8:1::2"),
	})
	if _, ok := errors.AsType[*supersededDeleteError](seg6Err); !ok {
		t.Fatalf("SEG6 partial apply returned %T (%v), want *supersededDeleteError", seg6Err, seg6Err)
	}
	if _, ok := errors.AsType[*opererr.Error](seg6Err); ok {
		t.Fatalf("wrapSEG6InstallError misclassified *supersededDeleteError as not-installed opererr: %v", seg6Err)
	}

	// Drive partial apply through fibKernel.processEvent.
	fib.processEvent(makeSysribPayload([]incomingChange{{
		Action:   routeaction.Update,
		Prefix:   prefix,
		NextHop:  netip.MustParseAddr("192.0.2.2"),
		Metric:   100,
		Protocol: "bgp",
	}}))

	if len(handle.added) != 1 {
		t.Fatalf("RouteAdd calls = %d, want 1 for the new priority", len(handle.added))
	}
	if len(handle.deleted) != 2 {
		t.Fatalf("RouteDel calls = %d, want 2 (must attempt every superseded route)", len(handle.deleted))
	}
	if got := fib.installed[prefix.String()]; got != "192.0.2.2" {
		t.Fatalf("fib.installed[%s] = %q, want newly installed next hop 192.0.2.2", prefix, got)
	}
	if got := fib.forwarding[prefix.String()].Metric; got != 100 {
		t.Fatalf("fib.forwarding[%s].Metric = %d, want 100", prefix, got)
	}
	if got := fib.supersededTables[prefix.String()]; len(got) != 2 || got[0] != 0 || got[1] != 0 {
		t.Fatalf("fib.supersededTables[%s] = %v, want [0 0]", prefix, got)
	}
	if _, pending := fib.pending[prefix.String()]; !pending {
		t.Fatal("partial apply did not park prefix in fib.pending")
	}
	if _, parked := fib.pendingChanges[prefix.String()]; !parked {
		t.Fatal("partial apply did not park prefix in fib.pendingChanges")
	}
	if !hasSyncFailureWarning(prefix.String()) {
		t.Fatal("partial apply did not raise fib-sync-failure warning")
	}

	// sweepStale must not touch supersededTables.
	fib.sweepStale(map[string]string{prefix.String(): "192.0.2.1"})
	if got := fib.supersededTables[prefix.String()]; len(got) != 2 {
		t.Fatalf("sweepStale modified supersededTables: got %v, want [0 0]", got)
	}

	// flushRoutes deletes the newly installed route plus both superseded routes
	// in table 0 (3 deletes total) and clears supersededTables.
	handle.deleted = nil
	handle.delErr = nil
	handle.listed = nil
	fib.flushRoutes()

	if len(handle.deleted) != 3 {
		t.Fatalf("flushRoutes performed %d deletes, want 3 (1 installed + 2 superseded)", len(handle.deleted))
	}
	if len(fib.supersededTables) != 0 {
		t.Fatalf("flushRoutes kept supersededTables: %v", fib.supersededTables)
	}
}

// VALIDATES: A partial apply (*supersededDeleteError) parked in pendingChanges
// clears supersededTables, pending state, and fib-sync-failure once retry
// re-runs replaceOwnedRoute and deletes the superseded route cleanly.
// PREVENTS: Permanent fib-sync-failure or leaked superseded routes after a
// transient superseded-delete failure.
func TestReplaceOwnedRoutePartialApplyClearedOnRetry(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	prefix := netip.MustParsePrefix("198.18.182.0/24")
	_, cidr, err := net.ParseCIDR(prefix.String())
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}

	handle := &fakeJournalNetlinkHandle{
		listed: []netlink.Route{
			{Dst: cidr, Protocol: rtprotZE, Priority: 10, Table: unix.RT_TABLE_MAIN},
		},
		delErr: unix.EPERM,
	}
	fib := newFIBKernel(&netlinkBackend{handle: handle})
	fib.installed[prefix.String()] = "192.0.2.1"
	fib.forwarding[prefix.String()] = incomingChange{
		Action:  routeaction.Add,
		Prefix:  prefix,
		NextHop: netip.MustParseAddr("192.0.2.1"),
		Metric:  10,
	}

	fib.processEvent(makeSysribPayload([]incomingChange{{
		Action:   routeaction.Update,
		Prefix:   prefix,
		NextHop:  netip.MustParseAddr("192.0.2.2"),
		Metric:   100,
		Protocol: "bgp",
	}}))

	if len(fib.supersededTables[prefix.String()]) != 1 {
		t.Fatalf("setup: supersededTables = %v, want [0]", fib.supersededTables[prefix.String()])
	}
	if !hasSyncFailureWarning(prefix.String()) {
		t.Fatal("setup: expected fib-sync-failure warning after partial apply")
	}

	// Simulate the kernel now holding both priorities and permitting deletes.
	handle.listed = []netlink.Route{
		{Dst: cidr, Protocol: rtprotZE, Priority: 10, Table: unix.RT_TABLE_MAIN},
		{Dst: cidr, Protocol: rtprotZE, Priority: 100, Table: unix.RT_TABLE_MAIN},
	}
	handle.delErr = nil
	handle.deleted = nil

	fib.retryDuePending(time.Now().Add(time.Hour))

	if len(handle.replaced) != 1 {
		t.Fatalf("retry RouteReplace calls = %d, want 1", len(handle.replaced))
	}
	if len(handle.deleted) != 1 || handle.deleted[0].Priority != 10 {
		t.Fatalf("retry RouteDel calls = %+v, want one delete of priority 10", handle.deleted)
	}
	if len(fib.supersededTables) != 0 {
		t.Fatalf("retry left supersededTables = %v, want empty", fib.supersededTables)
	}
	if len(fib.pending) != 0 || len(fib.pendingChanges) != 0 {
		t.Fatalf("retry left pending=%v pendingChanges=%v, want empty", fib.pending, fib.pendingChanges)
	}
	if hasSyncFailureWarning(prefix.String()) {
		t.Fatal("retry did not clear fib-sync-failure warning")
	}
}

// VALIDATES: replaceOwnedRoute ignores unix.ESRCH when deleting a superseded Ze
// route that disappeared concurrently, returning nil rather than
// *supersededDeleteError.
// PREVENTS: False partial-apply errors when a superseded route was already
// flushed before RouteDel ran.
func TestReplaceOwnedRouteIgnoresESRCHOnSupersededDelete(t *testing.T) {
	prefix := netip.MustParsePrefix("198.18.183.0/24")
	_, cidr, err := net.ParseCIDR(prefix.String())
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}

	handle := &fakeJournalNetlinkHandle{
		listed: []netlink.Route{
			{Dst: cidr, Protocol: rtprotZE, Priority: 10, Table: 100},
		},
		delErr: unix.ESRCH,
	}
	backend := &netlinkBackend{handle: handle}

	err = backend.replaceRichRoute(RichRoute{
		Prefix:  prefix,
		NextHop: netip.MustParseAddr("192.0.2.2"),
		Metric:  20,
		TableID: 100,
	})
	if err != nil {
		t.Fatalf("replaceRichRoute returned %v on superseded ESRCH, want nil", err)
	}
	if len(handle.added) != 1 {
		t.Fatalf("RouteAdd calls = %d, want 1", len(handle.added))
	}
	if len(handle.deleted) != 1 || handle.deleted[0].Priority != 10 {
		t.Fatalf("RouteDel calls = %+v, want one attempt on priority 10", handle.deleted)
	}
}
