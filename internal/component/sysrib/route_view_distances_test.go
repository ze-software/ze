// Design: docs/architecture/rib/unified-locrib.md -- daemon distance authority.
// Related: route_view_distances.go -- ordered declaration application.

package sysrib

import (
	"context"
	"errors"
	"maps"
	"net/netip"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func routeViewDistanceInput(t *testing.T, token string, operation uint64, changes map[string]int) rpc.RouteViewDistancesInput {
	t.Helper()
	declared, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	maps.Copy(declared, changes)
	input := rpc.RouteViewDistancesInput{Session: token, Operation: operation}
	for protocol, value := range declared {
		input.Distances = append(input.Distances, rpc.RouteViewDistance{Protocol: protocol, Distance: uint8(value)}) //nolint:gosec // test values come from uint8 schema leaves
	}
	return input
}

// A retry acknowledges the original declaration without judging any path again.
func TestRouteViewDistanceRetryDoesNotReselect(t *testing.T) {
	previous := ribpolicy.Current()
	evaluations := 0
	snapshot, err := ribpolicy.NewSnapshot(nil, candidateChain(func(r ribpolicy.Route) ribpolicy.Verdict {
		evaluations++
		return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
	}))
	if err != nil {
		t.Fatal(err)
	}
	ribpolicy.Set(snapshot)
	t.Cleanup(func() { ribpolicy.Set(previous) })
	defaults, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	publishDistances(defaults)
	t.Cleanup(func() { publishDistances(defaults) })
	rib := locrib.NewRIB()
	rib.Insert(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.0/24"), locrib.Path{Source: redistevents.RegisterProtocol("static")})
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, rib)
	input := routeViewDistanceInput(t, opened.Session, 1, map[string]int{"static": 30})
	evaluations = 0
	ack, err := authority.applyDistances(context.Background(), owner, input)
	if err != nil {
		t.Fatal(err)
	}
	if evaluations == 0 {
		t.Fatal("no path was judged by the first declaration")
	}
	evaluations = 0
	for left, right := 0, len(input.Distances)-1; left < right; left, right = left+1, right-1 {
		input.Distances[left], input.Distances[right] = input.Distances[right], input.Distances[left]
	}
	retry, err := authority.applyDistances(context.Background(), owner, input)
	if err != nil || retry != ack || evaluations != 0 {
		t.Fatalf("retry reran selection: reply %+v, error %v, evaluations %d", retry, err, evaluations)
	}
}

// A normal full-table distance update can exceed the delta retention depth.
// It applies successfully, fences old output and acknowledges the reset marker.
func TestRouteViewDistanceOverflowAcknowledgesReset(t *testing.T) {
	defaults, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	publishDistances(defaults)
	t.Cleanup(func() { publishDistances(defaults) })
	rib := locrib.NewRIB()
	static := redistevents.RegisterProtocol("static")
	bgp := redistevents.RegisterProtocol("bgp")
	for index := range 4097 {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(index >> 8), byte(index), 0}), 24)
		rib.Insert(family.IPv4Unicast, prefix, locrib.Path{Source: static, NextHop: netip.MustParseAddr("192.0.2.1")})
		rib.Insert(family.IPv4Unicast, prefix, locrib.Path{Source: bgp, IsBGP: true, IsEBGP: true, NextHop: netip.MustParseAddr("192.0.2.2")})
	}
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, rib)
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 4096)
	for range 3 {
		if page.Complete {
			break
		}
		page = routeViewReadTest(t, &authority, owner, opened.Session, page.Cursor, 4096)
	}
	if !page.Complete {
		t.Fatal("initial selected view never completed")
	}
	fenced := 0
	if err := authority.withSession(context.Background(), owner, opened.Session, func(session *routeViewSession) error {
		return session.setFence(func() error { fenced++; return nil })
	}); err != nil {
		t.Fatal(err)
	}
	ack, err := authority.applyDistances(context.Background(), owner, routeViewDistanceInput(t, opened.Session, 1, map[string]int{"static": 30}))
	if err != nil {
		t.Fatalf("valid full-table declaration was refused instead of requiring resnapshot: %v", err)
	}
	if ack.Cursor <= page.Cursor || fenced == 0 {
		t.Fatalf("reset has no post-apply marker or withdrawal: ack %+v, consumed %d, fences %d", ack, page.Cursor, fenced)
	}
	routeViewDistanceWinner(t, rib, netip.MustParsePrefix("10.0.0.0/24"), bgp, 20)
	routeViewDistanceWinner(t, rib, netip.MustParsePrefix("10.16.0.0/24"), bgp, 20)
	reset := routeViewReadTest(t, &authority, owner, opened.Session, page.Cursor, 4096)
	if reset.Phase != rpc.RouteViewPhaseReset || reset.Cursor != ack.Cursor {
		t.Fatalf("reset marker differs from applied declaration: %+v, ack %+v", reset, ack)
	}
	if err := authority.withSession(context.Background(), owner, opened.Session, func(session *routeViewSession) error {
		return session.validateConsumed(page.Cursor)
	}); err == nil {
		t.Fatal("old consumed view still permits publication")
	}
}

// A failed fence restores the declaration from the previous owner generation.
func TestRouteViewDistanceFailureRestoresDeclaration(t *testing.T) {
	defaults, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	publishDistances(defaults)
	t.Cleanup(func() { publishDistances(defaults) })
	rib := locrib.NewRIB()
	static := redistevents.RegisterProtocol("static")
	bgp := redistevents.RegisterProtocol("bgp")
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	rib.Insert(family.IPv4Unicast, prefix, locrib.Path{Source: static})
	rib.Insert(family.IPv4Unicast, prefix, locrib.Path{Source: bgp, IsBGP: true, IsEBGP: true})
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, rib)
	if _, err := authority.applyDistances(context.Background(), owner, routeViewDistanceInput(t, opened.Session, 1, map[string]int{"static": 30})); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.close(owner, opened.Session); err != nil {
		t.Fatal(err)
	}
	owner, opened = routeViewOpenTest(t, &authority, rib)
	failed := true
	sentinel := errors.New("controlled output fence failure")
	t.Cleanup(func() { failed = false })
	if err := authority.withSession(context.Background(), owner, opened.Session, func(session *routeViewSession) error {
		if err := session.setFence(func() error {
			if failed {
				return sentinel
			}
			return nil
		}); err != nil {
			return err
		}
		session.overflow.Store(true)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.applyDistances(context.Background(), owner, routeViewDistanceInput(t, opened.Session, 1, map[string]int{"static": 5})); !errors.Is(err, sentinel) {
		t.Fatalf("failed apply did not preserve fence error: %v", err)
	}
	routeViewDistanceWinner(t, rib, prefix, bgp, 20)
	rib.Remove(family.IPv4Unicast, prefix, bgp, 0)
	routeViewDistanceWinner(t, rib, prefix, static, 30)
	if authority.distanceTable["static"] != 30 {
		t.Fatalf("previous generation's declaration lost: %v", authority.distanceTable)
	}
}

// Cancellation after mutation restores the acknowledged table and winner.
func TestRouteViewDistanceCancellationRestoresDeclaration(t *testing.T) {
	defaults, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	publishDistances(defaults)
	t.Cleanup(func() { publishDistances(defaults) })
	rib := locrib.NewRIB()
	static := redistevents.RegisterProtocol("static")
	bgp := redistevents.RegisterProtocol("bgp")
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	rib.Insert(family.IPv4Unicast, prefix, locrib.Path{Source: static})
	rib.Insert(family.IPv4Unicast, prefix, locrib.Path{Source: bgp, IsBGP: true, IsEBGP: true})
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, rib)
	if _, err := authority.applyDistances(context.Background(), owner, routeViewDistanceInput(t, opened.Session, 1, map[string]int{"static": 30})); err != nil {
		t.Fatal(err)
	}
	work, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := authority.withSession(context.Background(), owner, opened.Session, func(session *routeViewSession) error {
		session.beforeReconcile = cancel
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.applyDistances(work, owner, routeViewDistanceInput(t, opened.Session, 2, map[string]int{"static": 5})); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled apply did not report cancellation: %v", err)
	}
	routeViewDistanceWinner(t, rib, prefix, bgp, 20)
	rib.Remove(family.IPv4Unicast, prefix, bgp, 0)
	routeViewDistanceWinner(t, rib, prefix, static, 30)
}

func routeViewDistanceWinner(t *testing.T, rib *locrib.RIB, prefix netip.Prefix, source redistevents.ProtocolID, distance uint8) {
	t.Helper()
	path, ok := rib.Best(family.IPv4Unicast, prefix)
	if !ok {
		t.Fatalf("no winner for %s", prefix)
	}
	if path.Source != source || path.AdminDistance != distance {
		t.Fatalf("winner %+v, want source %d distance %d", path, source, distance)
	}
}

func TestRouteViewDistanceApplyRollback(t *testing.T) {
	defaults, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	publishDistances(defaults)
	t.Cleanup(func() { publishDistances(defaults) })
	rib := locrib.NewRIB()
	static := redistevents.RegisterProtocol("static")
	bgp := redistevents.RegisterProtocol("bgp")
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	override := netip.MustParsePrefix("198.51.100.0/24")
	connected := netip.MustParsePrefix("203.0.113.0/24")
	staticPath := locrib.Path{Source: static, NextHop: netip.MustParseAddr("10.0.0.1")}
	bgpPath := locrib.Path{Source: bgp, NextHop: netip.MustParseAddr("10.0.0.2"), IsBGP: true, IsEBGP: true}
	rib.Insert(family.IPv4Unicast, prefix, staticPath)
	rib.Insert(family.IPv4Unicast, prefix, bgpPath)
	staticPath.DistanceOverride = 7
	staticPath.HasDistanceOverride = true
	rib.Insert(family.IPv4Unicast, override, staticPath)
	rib.Insert(family.IPv4Unicast, override, bgpPath)
	connectedID := redistevents.RegisterProtocol("connected")
	rib.Insert(family.IPv4Unicast, connected, locrib.Path{Source: connectedID})
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, rib)
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 4096)
	input := routeViewDistanceInput(t, opened.Session, 1, map[string]int{"static": 30})
	ack, err := authority.applyDistances(context.Background(), owner, input)
	if err != nil {
		t.Fatal(err)
	}
	if ack.Operation != 1 || ack.Session != opened.Session || ack.Cursor <= page.Cursor {
		t.Fatalf("distance acknowledgement lacks post-reselection barrier: %+v after %+v", ack, page)
	}
	routeViewDistanceWinner(t, rib, prefix, bgp, 20)
	routeViewDistanceWinner(t, rib, override, static, 7)
	routeViewDistanceWinner(t, rib, connected, connectedID, 0)
	revision := rib.Revision()
	retry, err := authority.applyDistances(context.Background(), owner, input)
	if err != nil || retry != ack || rib.Revision() != revision {
		t.Fatalf("retry mutated authority: reply %+v, error %v, revision %d/%d", retry, err, rib.Revision(), revision)
	}
	input.Distances[0].Distance++
	if _, err := authority.applyDistances(context.Background(), owner, input); err == nil {
		t.Fatal("reused operation accepted changed values")
	}
	input = routeViewDistanceInput(t, opened.Session, 2, nil)
	rollback, err := authority.applyDistances(context.Background(), owner, input)
	if err != nil || rollback.Operation != 2 || rollback.Cursor <= ack.Cursor {
		t.Fatalf("rollback: %+v, %v", rollback, err)
	}
	routeViewDistanceWinner(t, rib, prefix, static, 10)
	before := routeViewReadTest(t, &authority, owner, opened.Session, page.Cursor, 4096)
	invalid := []rpc.RouteViewDistancesInput{
		routeViewDistanceInput(t, opened.Session, 1, nil),
		routeViewDistanceInput(t, "foreign-token", 3, map[string]int{"static": 2}),
		{Session: opened.Session, Operation: 3},
	}
	unknown := routeViewDistanceInput(t, opened.Session, 3, nil)
	unknown.Distances = append(unknown.Distances, rpc.RouteViewDistance{Protocol: "unknown", Distance: 1})
	invalid = append(invalid, unknown)
	zero := routeViewDistanceInput(t, opened.Session, 3, map[string]int{"static": 0})
	invalid = append(invalid, zero)
	for _, refused := range invalid {
		if _, err := authority.applyDistances(context.Background(), owner, refused); err == nil {
			t.Fatalf("invalid distance operation accepted: %+v", refused)
		}
		routeViewDistanceWinner(t, rib, prefix, static, 10)
	}
	if _, err := authority.applyDistances(context.Background(), routeViewOwner(), routeViewDistanceInput(t, opened.Session, 3, nil)); err == nil {
		t.Fatal("another process generation changed distances")
	}
	after := routeViewReadTest(t, &authority, owner, opened.Session, before.Cursor, 4096)
	if !reflect.DeepEqual(after.Entries, []rpc.RouteViewEntry(nil)) || after.Cursor != before.Cursor {
		t.Fatalf("refusals changed selected view: %+v", after)
	}
}
