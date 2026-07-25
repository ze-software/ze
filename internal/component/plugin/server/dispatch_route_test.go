// VALIDATES: applyRouteInstall/applyRouteRemove apply a forked plugin's route
// batch to the engine Loc-RIB, resolving the protocol NAME to this process's
// ProtocolID (AC-1/2/3/6/7).
// PREVENTS: a numeric ProtocolID trusted across processes misattributing routes;
// a malformed entry producing a zero-Source Path or a partial batch apply.

package server

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
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
		Prefix: "10.7.0.0/24", NextHop: "192.0.2.9", AdminDistance: 110,
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
		Protocol:      "test-proto-install",
		AFI:           uint16(family.AFIIPv4),
		SAFI:          uint8(family.SAFIUnicast),
		Prefix:        "10.1.0.0/24",
		Instance:      0,
		NextHop:       "192.0.2.1",
		AdminDistance: 110,
		Metric:        42,
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
		Prefix: "10.2.0.0/24", NextHop: "192.0.2.2", AdminDistance: 110,
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
		Prefix: "10.3.0.0/24", NextHop: "192.0.2.3", AdminDistance: 110,
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
				Prefix: "10.4.0.0/24", NextHop: "192.0.2.4", AdminDistance: 110,
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
			Prefix: "10.5.0.0/24", NextHop: "192.0.2.5", AdminDistance: 110},
		{Protocol: "test-batch", AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast),
			Prefix: "not-a-prefix", NextHop: "192.0.2.6", AdminDistance: 110},
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
			Prefix: "10.6.0.0/24", Instance: 0, NextHop: "192.0.2.7", AdminDistance: 110, Metric: 5},
		{Protocol: name, AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast),
			Prefix: "10.6.0.0/24", Instance: 1, NextHop: "192.0.2.8", AdminDistance: 110, Metric: 5},
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
		Prefix: "198.51.100.0/24", NextHop: "192.0.2.20", AdminDistance: 110,
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
