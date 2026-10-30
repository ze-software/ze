// VALIDATES: applyRouteInstall/applyRouteRemove apply a forked plugin's route
// batch to the engine Loc-RIB, resolving the protocol NAME to this process's
// ProtocolID (AC-1/2/3/6/7).
// PREVENTS: a numeric ProtocolID trusted across processes misattributing routes;
// a malformed entry producing a zero-Source Path or a partial batch apply.

package server

import (
	"encoding/json"
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func v4u() family.Family { return family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast} }

func mustPfx(t *testing.T, s string) netip.Prefix {
	t.Helper()
	p, err := netip.ParsePrefix(s)
	if err != nil {
		t.Fatalf("parse prefix %q: %v", s, err)
	}
	return p
}

func init() {
	// resolveProtocol now LOOKS UP the protocol (ProtocolIDOf) and rejects unknown
	// names, so the test protocol names must be registered in this process first --
	// mirroring how the engine binary registers in-tree protocols (e.g. "ospf") at
	// package init. Names not registered here exercise the reject-unknown path.
	for _, n := range []string{"test-proto-install", "test-proto-remove", "test-batch", "test-ecmp", "test-disconnect"} {
		redistevents.RegisterProtocol(n)
	}
}

// TestApplyRouteInstallRejectsUnknownProtocol: ISSUE 1 -- a protocol name the
// engine never registered is REJECTED (not registered-on-demand), so wire input
// cannot pollute or exhaust (panic) the process-global redistevents registry.
func TestApplyRouteInstallRejectsUnknownProtocol(t *testing.T) {
	rib := locrib.NewRIB()
	in := rpc.RouteInstallInput{Routes: []rpc.RouteInstallEntry{{
		Protocol: "totally-unregistered-xyz", AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast),
		Prefix: "10.7.0.0/24", NextHop: "192.0.2.9", DistanceOverride: distanceOf(110),
	}}}
	keys, err := applyRouteInstall(rib, in)
	if err == nil {
		t.Fatal("expected error for an unknown (unregistered) protocol name")
	}
	if len(keys) != 0 {
		t.Errorf("installed %d routes; want 0 for unknown protocol", len(keys))
	}
	if _, ok := rib.Best(v4u(), mustPfx(t, "10.7.0.0/24")); ok {
		t.Error("route inserted despite unknown protocol")
	}
}

// TestApplyRouteInstallInsertsPath: AC-1 -- a route-install entry becomes a valid
// locrib.Path in the engine RIB with every field carried through.
func TestApplyRouteInstallInsertsPath(t *testing.T) {
	rib := locrib.NewRIB()
	in := rpc.RouteInstallInput{Routes: []rpc.RouteInstallEntry{{
		Protocol:         "test-proto-install",
		AFI:              uint16(family.AFIIPv4),
		SAFI:             uint8(family.SAFIUnicast),
		Prefix:           "10.1.0.0/24",
		Instance:         0,
		NextHop:          "192.0.2.1",
		DistanceOverride: distanceOf(110),
		Metric:           42,
	}}}
	n, err := applyRouteInstall(rib, in)
	if err != nil {
		t.Fatalf("applyRouteInstall: %v", err)
	}
	if len(n) != 1 {
		t.Fatalf("installed = %d, want 1", len(n))
	}
	best, ok := rib.Best(v4u(), mustPfx(t, "10.1.0.0/24"))
	if !ok {
		t.Fatal("route not present in Loc-RIB")
	}
	if best.NextHop != netip.MustParseAddr("192.0.2.1") {
		t.Errorf("next-hop = %v, want 192.0.2.1", best.NextHop)
	}
	if best.AdminDistance != 110 || best.Metric != 42 {
		t.Errorf("admin/metric = %d/%d, want 110/42", best.AdminDistance, best.Metric)
	}
}

// TestApplyRouteInstallResolvesProtocolByName: AC-2 -- the wire NAME is resolved
// to THIS process's ProtocolID; the numeric id is never trusted from the wire.
func TestApplyRouteInstallResolvesProtocolByName(t *testing.T) {
	rib := locrib.NewRIB()
	const name = "test-proto-resolve"
	want := redistevents.RegisterProtocol(name) // this process's id for the name
	in := rpc.RouteInstallInput{Routes: []rpc.RouteInstallEntry{{
		Protocol: name, AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast),
		Prefix: "10.2.0.0/24", NextHop: "192.0.2.2", DistanceOverride: distanceOf(110),
	}}}
	if _, err := applyRouteInstall(rib, in); err != nil {
		t.Fatalf("applyRouteInstall: %v", err)
	}
	best, ok := rib.Best(v4u(), mustPfx(t, "10.2.0.0/24"))
	if !ok {
		t.Fatal("route not present")
	}
	if best.Source != want {
		t.Errorf("Source = %d, want %d (resolved from name %q)", best.Source, want, name)
	}
	if got := redistevents.ProtocolName(best.Source); got != name {
		t.Errorf("ProtocolName(Source) = %q, want %q", got, name)
	}
}

// TestApplyRouteRemoveWithdrawsPath: AC-3 -- remove withdraws exactly the named
// (Source, Instance) path.
func TestApplyRouteRemoveWithdrawsPath(t *testing.T) {
	rib := locrib.NewRIB()
	const name = "test-proto-remove"
	install := rpc.RouteInstallInput{Routes: []rpc.RouteInstallEntry{{
		Protocol: name, AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast),
		Prefix: "10.3.0.0/24", NextHop: "192.0.2.3", DistanceOverride: distanceOf(110),
	}}}
	if _, err := applyRouteInstall(rib, install); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, ok := rib.Best(v4u(), mustPfx(t, "10.3.0.0/24")); !ok {
		t.Fatal("precondition: route should be present after install")
	}
	remove := rpc.RouteRemoveInput{Routes: []rpc.RouteRemoveEntry{{
		Protocol: name, AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast),
		Prefix: "10.3.0.0/24", Instance: 0,
	}}}
	n, err := applyRouteRemove(rib, remove)
	if err != nil {
		t.Fatalf("applyRouteRemove: %v", err)
	}
	if len(n) != 1 {
		t.Fatalf("removed = %d, want 1", len(n))
	}
	if _, ok := rib.Best(v4u(), mustPfx(t, "10.3.0.0/24")); ok {
		t.Error("route still present after remove")
	}
}

// TestApplyRouteInstallRejectsBadProtocolName: AC-6 -- empty or over-long protocol
// names are rejected and nothing is inserted (never a zero-Source Path).
func TestApplyRouteInstallRejectsBadProtocolName(t *testing.T) {
	cases := map[string]string{
		"empty":     "",
		"oversized": string(make([]byte, maxProtocolNameLen+1)),
	}
	for label, name := range cases {
		t.Run(label, func(t *testing.T) {
			rib := locrib.NewRIB()
			in := rpc.RouteInstallInput{Routes: []rpc.RouteInstallEntry{{
				Protocol: name, AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast),
				Prefix: "10.4.0.0/24", NextHop: "192.0.2.4", DistanceOverride: distanceOf(110),
			}}}
			n, err := applyRouteInstall(rib, in)
			if err == nil {
				t.Fatal("expected error for bad protocol name")
			}
			if len(n) != 0 {
				t.Errorf("installed = %d, want 0 on error", len(n))
			}
			if _, ok := rib.Best(v4u(), mustPfx(t, "10.4.0.0/24")); ok {
				t.Error("route inserted despite bad protocol name")
			}
		})
	}
}

// TestApplyRouteInstallBatchAtomicOnBadEntry: a malformed entry fails the whole
// batch before ANY partial apply (build-all-then-apply).
func TestApplyRouteInstallBatchAtomicOnBadEntry(t *testing.T) {
	rib := locrib.NewRIB()
	in := rpc.RouteInstallInput{Routes: []rpc.RouteInstallEntry{
		{Protocol: "test-batch", AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast),
			Prefix: "10.5.0.0/24", NextHop: "192.0.2.5", DistanceOverride: distanceOf(110)},
		{Protocol: "test-batch", AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast),
			Prefix: "not-a-prefix", NextHop: "192.0.2.6", DistanceOverride: distanceOf(110)},
	}}
	if _, err := applyRouteInstall(rib, in); err == nil {
		t.Fatal("expected error for malformed prefix in batch")
	}
	if _, ok := rib.Best(v4u(), mustPfx(t, "10.5.0.0/24")); ok {
		t.Error("first (valid) entry applied despite batch error; want all-or-nothing")
	}
}

// TestApplyRouteInstallECMP: AC-7 -- two entries for one prefix with distinct
// Instances both land as distinct (Source, Instance) paths.
func TestApplyRouteInstallECMP(t *testing.T) {
	rib := locrib.NewRIB()
	const name = "test-ecmp"
	in := rpc.RouteInstallInput{Routes: []rpc.RouteInstallEntry{
		{Protocol: name, AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast),
			Prefix: "10.6.0.0/24", Instance: 0, NextHop: "192.0.2.7", DistanceOverride: distanceOf(110), Metric: 5},
		{Protocol: name, AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast),
			Prefix: "10.6.0.0/24", Instance: 1, NextHop: "192.0.2.8", DistanceOverride: distanceOf(110), Metric: 5},
	}}
	if _, err := applyRouteInstall(rib, in); err != nil {
		t.Fatalf("applyRouteInstall: %v", err)
	}
	g, ok := rib.Lookup(v4u(), mustPfx(t, "10.6.0.0/24"))
	if !ok {
		t.Fatal("prefix not present")
	}
	if len(g.Paths) != 2 {
		t.Fatalf("path count = %d, want 2 (distinct instances)", len(g.Paths))
	}
}

// TestWithdrawPluginRoutesOnDisconnect: AC-8 -- a forked plugin's routes are
// withdrawn from the engine Loc-RIB when it disconnects (no stale routes). Uses
// locrib.Default() (the engine singleton, non-nil in tests) with a TEST-NET prefix
// so it does not collide with other tests' entries.
func TestWithdrawPluginRoutesOnDisconnect(t *testing.T) {
	s := &Server{}
	rib := locrib.Default()
	in := rpc.RouteInstallInput{Routes: []rpc.RouteInstallEntry{{
		Protocol: "test-disconnect", AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast),
		Prefix: "198.51.100.0/24", NextHop: "192.0.2.20", DistanceOverride: distanceOf(110),
	}}}
	keys, err := applyRouteInstall(rib, in)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	s.recordInstalled("ospf-disc", keys)
	if _, ok := rib.Best(v4u(), mustPfx(t, "198.51.100.0/24")); !ok {
		t.Fatal("precondition: route should be installed")
	}
	s.withdrawPluginRoutes("ospf-disc") // simulate disconnect
	if _, ok := rib.Best(v4u(), mustPfx(t, "198.51.100.0/24")); ok {
		t.Error("route still present after plugin disconnect; AC-8 stale-route cleanup failed")
	}
}

// TestUnrecordStopsWithdrawal: a route the plugin explicitly withdrew (unrecord) is
// no longer withdrawn again on disconnect (bookkeeping is exact).
func TestUnrecordStopsWithdrawal(t *testing.T) {
	s := &Server{}
	keys := []routeKey{{fam: v4u(), prefix: mustPfx(t, "203.0.113.0/24"), source: redistevents.RegisterProtocol("test-unrecord"), instance: 0}}
	s.recordInstalled("p", keys)
	s.unrecordInstalled("p", keys)
	s.routeMu.Lock()
	_, tracked := s.installedByPlugin["p"]
	s.routeMu.Unlock()
	if tracked {
		t.Error("plugin still tracked after all its routes were unrecorded")
	}
}

// routeChain adapts a function into a ribpolicy.Chain for the gate tests.
type routeChain func(r ribpolicy.Route) ribpolicy.Verdict

func (f routeChain) Evaluate(r ribpolicy.Route) ribpolicy.Verdict { return f(r) } //nolint:gocritic // hugeParam: the Chain signature passes Route by value

// TestRouteInstallTagReachesGate: AC-11 forked and R-8. A forked producer's
// route-install entry carries tag, has-tag, level and ospf-route-type over
// the JSON wire, and the engine rebuilds them on the Path, so the Loc-RIB gate
// judges the tag the plugin sent: a central chain holding the compiled
// `match { action reject; tag [ 4242 ]; }` hides the tagged route. An entry
// that omits the fields (an older plugin) reaches the gate as a route that
// carries no tag, never as tag 0. A level or path type outside the numbering
// fails the batch.
func TestRouteInstallTagReachesGate(t *testing.T) {
	reject4242, err := policy.CompileRouteMatch(map[string]any{"action": "reject", "tag": []any{"4242"}}, false, nil)
	if err != nil {
		t.Fatalf("compile match: %v", err)
	}
	var seen []ribpolicy.Route
	var mu sync.Mutex
	rejectTag := routeChain(func(r ribpolicy.Route) ribpolicy.Verdict {
		mu.Lock()
		seen = append(seen, r)
		mu.Unlock()
		if reject4242.Rejects(&r) {
			return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
		}
		return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
	})
	snap, err := ribpolicy.NewSnapshot(nil, rejectTag)
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	ribpolicy.Set(snap)
	t.Cleanup(func() { ribpolicy.Set(nil) })

	// The entries cross the JSON wire, as from a forked plugin.
	wire, err := json.Marshal(rpc.RouteInstallInput{Routes: []rpc.RouteInstallEntry{
		{Protocol: "test-proto-install", AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast),
			Prefix: "10.42.0.0/24", NextHop: "192.0.2.1", Tag: 4242, HasTag: true, Level: 2, OSPFRouteType: 4},
		{Protocol: "test-proto-install", AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast),
			Prefix: "10.43.0.0/24", NextHop: "192.0.2.1"},
	}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var in rpc.RouteInstallInput
	if err := json.Unmarshal(wire, &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	rib := locrib.NewRIB()
	if _, err := applyRouteInstall(rib, in); err != nil {
		t.Fatalf("applyRouteInstall: %v", err)
	}
	if best, ok := rib.Best(v4u(), mustPfx(t, "10.42.0.0/24")); ok {
		t.Fatalf("tagged route selected (%+v): the gate never saw tag 4242", best)
	}
	if _, ok := rib.Best(v4u(), mustPfx(t, "10.43.0.0/24")); !ok {
		t.Fatal("untagged route hidden: an omitted tag reached the gate as a tag")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 2 {
		t.Fatalf("gate judged %d routes, want 2", len(seen))
	}
	tagged, untagged := seen[0], seen[1]
	if tagged.Prefix != mustPfx(t, "10.42.0.0/24") {
		tagged, untagged = untagged, tagged
	}
	if tagged.Level != ribpolicy.ISISLevel2 || tagged.OSPFRouteType != ribpolicy.OSPFExternal2 {
		t.Errorf("tagged route reached the gate at level %d route type %d, want 2 and 4", tagged.Level, tagged.OSPFRouteType)
	}
	if untagged.HasTag || untagged.Level != ribpolicy.ISISLevelUnspecified || untagged.OSPFRouteType != ribpolicy.OSPFRouteTypeUnspecified {
		t.Errorf("an entry omitting the fields reached the gate carrying them: %+v", untagged)
	}

	for _, bad := range []rpc.RouteInstallEntry{
		{Protocol: "test-proto-install", AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast), Prefix: "10.44.0.0/24", NextHop: "192.0.2.1", Level: 3},
		{Protocol: "test-proto-install", AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast), Prefix: "10.44.0.0/24", NextHop: "192.0.2.1", OSPFRouteType: 7},
	} {
		if _, err := applyRouteInstall(rib, rpc.RouteInstallInput{Routes: []rpc.RouteInstallEntry{bad}}); err == nil {
			t.Errorf("entry with level %d route type %d accepted, want refused", bad.Level, bad.OSPFRouteType)
		}
	}
}
