package sysrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

var (
	ipv4Unicast = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}
	ipv6Unicast = family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast}
)

var testProto = redistevents.RegisterProtocol("test-nhresolver")

func connectedPath(metric uint32) locrib.Path {
	return locrib.Path{
		Source:        testProto,
		Instance:      0,
		AdminDistance: 0,
		Metric:        metric,
	}
}

func recursivePath(nextHop netip.Addr, metric uint32) locrib.Path {
	return locrib.Path{
		Source:        testProto,
		Instance:      0,
		NextHop:       nextHop,
		AdminDistance: 20,
		Metric:        metric,
	}
}

func TestRecursiveNHResolve_DirectlyConnected(t *testing.T) {
	rib := locrib.NewRIB()
	// 10.0.0.0/24 is directly connected (zero next-hop)
	rib.Insert(ipv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), connectedPath(10))

	resolver := newNHResolver(rib)
	res := resolver.Resolve(netip.MustParseAddr("10.0.0.1"))
	if !res.Resolved {
		t.Fatal("expected resolved")
	}
	if res.DirectNH != netip.MustParseAddr("10.0.0.1") {
		t.Errorf("DirectNH = %v, want 10.0.0.1", res.DirectNH)
	}
	if res.Metric != 10 {
		t.Errorf("Metric = %d, want 10", res.Metric)
	}
}

func TestRecursiveNHResolve_OneHopRecursion(t *testing.T) {
	rib := locrib.NewRIB()
	// 192.168.1.0/24 is directly connected (cost 5)
	rib.Insert(ipv4Unicast, netip.MustParsePrefix("192.168.1.0/24"), connectedPath(5))
	// 10.0.0.0/8 reachable via 192.168.1.1 (cost 100)
	rib.Insert(ipv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), recursivePath(netip.MustParseAddr("192.168.1.1"), 100))

	resolver := newNHResolver(rib)
	// Resolve 10.1.2.3: LPM matches 10.0.0.0/8 -> NH 192.168.1.1
	// Then resolve 192.168.1.1: LPM matches 192.168.1.0/24 -> connected
	res := resolver.Resolve(netip.MustParseAddr("10.1.2.3"))
	if !res.Resolved {
		t.Fatal("expected resolved")
	}
	if res.DirectNH != netip.MustParseAddr("192.168.1.1") {
		t.Errorf("DirectNH = %v, want 192.168.1.1", res.DirectNH)
	}
	if res.Metric != 105 {
		t.Errorf("Metric = %d, want 105 (100+5)", res.Metric)
	}
}

func TestRecursiveNHResolve_TwoHopRecursion(t *testing.T) {
	rib := locrib.NewRIB()
	// 172.16.0.0/24 directly connected (cost 1)
	rib.Insert(ipv4Unicast, netip.MustParsePrefix("172.16.0.0/24"), connectedPath(1))
	// 192.168.0.0/16 via 172.16.0.1 (cost 10)
	rib.Insert(ipv4Unicast, netip.MustParsePrefix("192.168.0.0/16"), recursivePath(netip.MustParseAddr("172.16.0.1"), 10))
	// 10.0.0.0/8 via 192.168.1.1 (cost 50)
	rib.Insert(ipv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), recursivePath(netip.MustParseAddr("192.168.1.1"), 50))

	resolver := newNHResolver(rib)
	res := resolver.Resolve(netip.MustParseAddr("10.1.1.1"))
	if !res.Resolved {
		t.Fatal("expected resolved")
	}
	if res.DirectNH != netip.MustParseAddr("172.16.0.1") {
		t.Errorf("DirectNH = %v, want 172.16.0.1", res.DirectNH)
	}
	if res.Metric != 61 {
		t.Errorf("Metric = %d, want 61 (50+10+1)", res.Metric)
	}
}

func TestRecursiveNHResolve_Unreachable(t *testing.T) {
	rib := locrib.NewRIB()
	resolver := newNHResolver(rib)
	res := resolver.Resolve(netip.MustParseAddr("10.0.0.1"))
	if res.Resolved {
		t.Error("expected not resolved for empty RIB")
	}
}

func TestRecursiveNHResolve_InfiniteRecursion(t *testing.T) {
	rib := locrib.NewRIB()
	// Create a loop: 10.0.0.0/8 via 192.168.1.1, 192.168.0.0/16 via 10.1.1.1
	rib.Insert(ipv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), recursivePath(netip.MustParseAddr("192.168.1.1"), 10))
	rib.Insert(ipv4Unicast, netip.MustParsePrefix("192.168.0.0/16"), recursivePath(netip.MustParseAddr("10.1.1.1"), 20))

	resolver := newNHResolver(rib)
	res := resolver.Resolve(netip.MustParseAddr("10.2.3.4"))
	if res.Resolved {
		t.Error("expected not resolved for recursive loop")
	}
}

func TestRecursiveNHResolve_SelfReferencing(t *testing.T) {
	rib := locrib.NewRIB()
	// Route pointing to itself: 10.0.0.0/8 via 10.0.0.1
	rib.Insert(ipv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), recursivePath(netip.MustParseAddr("10.0.0.1"), 10))

	resolver := newNHResolver(rib)
	res := resolver.Resolve(netip.MustParseAddr("10.0.0.1"))
	if res.Resolved {
		t.Error("expected not resolved for self-referencing route")
	}
}

func TestRecursiveNHResolve_IPv6(t *testing.T) {
	rib := locrib.NewRIB()
	rib.Insert(ipv6Unicast, netip.MustParsePrefix("2001:db8:1::/48"), connectedPath(3))

	resolver := newNHResolver(rib)
	res := resolver.Resolve(netip.MustParseAddr("2001:db8:1::1"))
	if !res.Resolved {
		t.Fatal("expected resolved")
	}
	if res.DirectNH != netip.MustParseAddr("2001:db8:1::1") {
		t.Errorf("DirectNH = %v, want 2001:db8:1::1", res.DirectNH)
	}
	if res.Metric != 3 {
		t.Errorf("Metric = %d, want 3", res.Metric)
	}
}

func TestRecursiveNHResolve_InvalidAddr(t *testing.T) {
	rib := locrib.NewRIB()
	resolver := newNHResolver(rib)
	res := resolver.Resolve(netip.Addr{})
	if res.Resolved {
		t.Error("expected not resolved for invalid addr")
	}
}

func TestNHResolver_IGPMetric(t *testing.T) {
	rib := locrib.NewRIB()
	rib.Insert(ipv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), connectedPath(42))

	resolver := newNHResolver(rib)
	metric := resolver.IGPMetric(netip.MustParseAddr("10.0.0.5"))
	if metric != 42 {
		t.Errorf("IGPMetric = %d, want 42", metric)
	}

	metric = resolver.IGPMetric(netip.MustParseAddr("172.16.0.1"))
	if metric != 0 {
		t.Errorf("IGPMetric for unreachable = %d, want 0", metric)
	}
}

func TestNHResolver_CoveredNHs(t *testing.T) {
	rib := locrib.NewRIB()
	resolver := newNHResolver(rib)

	nh1 := netip.MustParseAddr("10.0.0.1")
	nh2 := netip.MustParseAddr("10.0.0.5")
	nh3 := netip.MustParseAddr("172.16.0.1")
	pfx1 := netip.MustParsePrefix("192.168.1.0/24")
	pfx2 := netip.MustParsePrefix("192.168.2.0/24")

	resolver.Track(nh1, pfx1)
	resolver.Track(nh2, pfx2)
	resolver.Track(nh3, pfx1)

	covered := resolver.CoveredNHs(netip.MustParsePrefix("10.0.0.0/24"))
	if len(covered) != 2 {
		t.Fatalf("CoveredNHs(10.0.0.0/24) = %d NHs, want 2", len(covered))
	}

	covered = resolver.CoveredNHs(netip.MustParsePrefix("172.16.0.0/16"))
	if len(covered) != 1 {
		t.Fatalf("CoveredNHs(172.16.0.0/16) = %d NHs, want 1", len(covered))
	}

	covered = resolver.CoveredNHs(netip.MustParsePrefix("0.0.0.0/0"))
	if len(covered) != 3 {
		t.Fatalf("CoveredNHs(0.0.0.0/0) = %d NHs, want 3", len(covered))
	}

	covered = resolver.CoveredNHs(netip.MustParsePrefix("8.8.8.0/24"))
	if len(covered) != 0 {
		t.Fatalf("CoveredNHs(8.8.8.0/24) = %d NHs, want 0", len(covered))
	}
}

func TestNHResolver_Tracking(t *testing.T) {
	rib := locrib.NewRIB()
	resolver := newNHResolver(rib)

	nh := netip.MustParseAddr("10.0.0.1")
	pfx1 := netip.MustParsePrefix("192.168.1.0/24")
	pfx2 := netip.MustParsePrefix("192.168.2.0/24")

	resolver.Track(nh, pfx1)
	resolver.Track(nh, pfx2)

	deps := resolver.Dependents(nh)
	if len(deps) != 2 {
		t.Fatalf("Dependents len = %d, want 2", len(deps))
	}

	resolver.Untrack(nh, pfx1)
	deps = resolver.Dependents(nh)
	if len(deps) != 1 {
		t.Fatalf("Dependents len after untrack = %d, want 1", len(deps))
	}

	resolver.Untrack(nh, pfx2)
	deps = resolver.Dependents(nh)
	if len(deps) != 0 {
		t.Errorf("Dependents len after full untrack = %d, want 0", len(deps))
	}
}
