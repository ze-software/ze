package spf

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func lookupPaths(t *testing.T, loc *locrib.RIB, pfx netip.Prefix) []locrib.Path {
	t.Helper()
	g, ok := loc.Lookup(family.IPv4Unicast, pfx)
	if !ok {
		return nil
	}
	return g.Paths
}

func TestOSPFInstallPath(t *testing.T) {
	loc := locrib.NewRIB()
	in := NewInstaller(loc)
	pfx := netip.MustParsePrefix("10.20.0.0/24")

	in.Apply([]RouteEntry{{
		AreaID: testArea(), Prefix: pfx, Metric: 42, Type: RouteIntraArea,
		Origin:   testRID(t, "2.2.2.2"),
		NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.2")}, {Addr: netip.MustParseAddr("10.0.0.3")}},
	}})
	paths := lookupPaths(t, loc, pfx)
	if len(paths) != 2 {
		t.Fatalf("inserted %d paths, want 2", len(paths))
	}
	for i, p := range paths {
		if p.Source != ProtocolID() || p.AdminDistance != DefaultAdminDistance || p.Metric != 42 {
			t.Fatalf("path[%d] = %+v", i, p)
		}
	}

	in.Apply([]RouteEntry{{
		AreaID: testArea(), Prefix: pfx, Metric: 42, Type: RouteIntraArea,
		Origin: testRID(t, "2.2.2.2"), NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.2")}},
	}})
	if paths = lookupPaths(t, loc, pfx); len(paths) != 1 || paths[0].NextHop != netip.MustParseAddr("10.0.0.2") {
		t.Fatalf("after shrink paths = %+v, want only 10.0.0.2", paths)
	}

	in.Apply(nil)
	if paths = lookupPaths(t, loc, pfx); len(paths) != 0 {
		t.Fatalf("after withdraw paths = %+v, want none", paths)
	}
}

// TestInstallerFamilyPerAF pins the RFC 5838 per-AF install family: an installer built
// with NewInstallerFamily(fam) inserts into fam's Loc-RIB, one AF per RFC 5838 §2.1.
func TestInstallerFamilyPerAF(t *testing.T) {
	cases := []struct {
		fam family.Family
		pfx netip.Prefix
		nh  netip.Addr
	}{
		{family.IPv6Unicast, netip.MustParsePrefix("2001:db8::/64"), netip.MustParseAddr("fe80::2")},
		{family.IPv4Unicast, netip.MustParsePrefix("10.20.0.0/24"), netip.MustParseAddr("fe80::2")},
		{family.IPv6Multicast, netip.MustParsePrefix("2001:db8:1::/64"), netip.MustParseAddr("fe80::2")},
		{family.IPv4Multicast, netip.MustParsePrefix("10.30.0.0/24"), netip.MustParseAddr("fe80::2")},
	}
	for _, c := range cases {
		loc := locrib.NewRIB()
		in := NewInstallerFamily(loc, c.fam)
		in.Apply([]RouteEntry{{
			AreaID: testArea(), Prefix: c.pfx, Metric: 10, Type: RouteIntraArea,
			Origin: testRID(t, "2.2.2.2"), NextHops: []NextHop{{Addr: c.nh}},
		}})
		g, ok := loc.Lookup(c.fam, c.pfx)
		if !ok || len(g.Paths) != 1 || g.Paths[0].Source != ProtocolID() {
			t.Fatalf("%s: route %s not installed into %s (got %+v, ok=%v)", c.fam, c.pfx, c.fam, g, ok)
		}
	}
}

// TestV6UnicastInstallsIPv6Family pins the IPv6-base fix (RFC 5838): the IPv6-unicast
// instance installs into family.IPv6Unicast, NOT the old family.IPv4Unicast hardcode.
func TestV6UnicastInstallsIPv6Family(t *testing.T) {
	loc := locrib.NewRIB()
	in := NewInstallerFamily(loc, family.IPv6Unicast)
	pfx := netip.MustParsePrefix("2001:db8:abcd::/48")
	in.Apply([]RouteEntry{{
		AreaID: testArea(), Prefix: pfx, Metric: 10, Type: RouteIntraArea,
		Origin: testRID(t, "2.2.2.2"), NextHops: []NextHop{{Addr: netip.MustParseAddr("fe80::2")}},
	}})
	if g, ok := loc.Lookup(family.IPv6Unicast, pfx); !ok || len(g.Paths) != 1 {
		t.Fatalf("route not in IPv6Unicast: got %+v ok=%v", g, ok)
	}
	if g, ok := loc.Lookup(family.IPv4Unicast, pfx); ok && len(g.Paths) > 0 {
		t.Fatalf("route wrongly present in IPv4Unicast: %+v", g)
	}
}

func TestOSPFSPFRoute(t *testing.T) {
	area := testArea()
	loc := locrib.NewRIB()
	db := baseP2PSource(t, area)
	c := NewComputer(Config{Source: db, Root: testRID(t, "1.1.1.1"), Areas: []types.AreaID{area}, Installer: NewInstaller(loc)})
	delta := c.Run()
	pfx := netip.MustParsePrefix("192.0.2.0/24")
	if len(delta.Added) != 1 || delta.Added[0].Prefix != pfx {
		t.Fatalf("delta = %+v, want added %s", delta, pfx)
	}
	paths := lookupPaths(t, loc, pfx)
	if len(paths) != 1 || paths[0].Source != ProtocolID() || paths[0].NextHop != netip.MustParseAddr("10.0.0.2") {
		t.Fatalf("Loc-RIB paths = %+v", paths)
	}
	snap := c.Snapshot()
	if len(snap) != 1 || snap[0].Prefix != pfx.String() || snap[0].Type != RouteIntraArea.String() {
		t.Fatalf("snapshot = %+v", snap)
	}
	c.Stop()
	if paths := lookupPaths(t, loc, pfx); len(paths) != 0 {
		t.Fatalf("after Stop paths = %+v, want none", paths)
	}
}

func TestOSPFInterAreaInstallsViaLocRIB(t *testing.T) {
	area := types.BackboneArea
	loc := locrib.NewRIB()
	root := testRID(t, "1.1.1.1")
	abr := testRID(t, "2.2.2.2")
	db := testSource(t, area,
		routerLSA(t, "1.1.1.1", p2pLink(t, "2.2.2.2", "10.0.0.1", 10)),
		packet.LSA{
			Header: packet.LSAHeader{
				Options:           types.OptionE,
				Type:              types.LSTypeRouter,
				LinkStateID:       types.LinkStateID(abr),
				AdvertisingRouter: abr,
				Sequence:          types.InitialSequenceNumber,
			},
			Router: &packet.RouterLSA{
				Flags: packet.RouterFlagB,
				Links: []packet.RouterLink{p2pLink(t, "1.1.1.1", "10.0.0.2", 10)},
			},
		},
		summaryNetworkLSA(t, "10.70.0.0", "2.2.2.2", 5),
	)
	c := NewComputer(Config{Source: db, Root: root, Areas: []types.AreaID{area}, Installer: NewInstaller(loc)})

	delta := c.Run()
	pfx := netip.MustParsePrefix("10.70.0.0/24")
	if len(delta.Added) != 1 || delta.Added[0].Prefix != pfx || delta.Added[0].Type != RouteInterArea {
		t.Fatalf("delta = %+v, want one inter-area add for %s", delta, pfx)
	}
	paths := lookupPaths(t, loc, pfx)
	if len(paths) != 1 || paths[0].Source != ProtocolID() || paths[0].NextHop != netip.MustParseAddr("10.0.0.2") || paths[0].Metric != 15 {
		t.Fatalf("Loc-RIB paths = %+v", paths)
	}
	snap := c.Snapshot()
	if len(snap) != 1 || snap[0].Prefix != pfx.String() || snap[0].Type != RouteInterArea.String() {
		t.Fatalf("snapshot = %+v", snap)
	}
}

func TestOSPFSPFThrottle(t *testing.T) {
	area := testArea()
	c := NewComputer(Config{Root: testRID(t, "1.1.1.1"), Areas: []types.AreaID{area}, SPFDelay: time.Millisecond, SPFHold: 2 * time.Millisecond, SPFMaxHold: 8 * time.Millisecond})
	now := time.Unix(0, 0)
	c.now = func() time.Time { return now }
	var delays []time.Duration
	var timers []*manualTimer
	c.afterFunc = func(d time.Duration, f func()) timerHandle {
		delays = append(delays, d)
		mt := &manualTimer{fn: f}
		timers = append(timers, mt)
		return mt
	}

	c.TriggerArea(area)
	now = now.Add(500 * time.Microsecond)
	c.TriggerArea(area)
	now = now.Add(500 * time.Microsecond)
	c.TriggerArea(area)
	if len(delays) != 1 || delays[0] != time.Millisecond {
		t.Fatalf("armed delays = %v, want one initial delay", delays)
	}
	if c.currentDelay != 4*time.Millisecond {
		t.Fatalf("currentDelay = %s, want 4ms after burst backoff", c.currentDelay)
	}
	timers[0].fire()
	now = now.Add(20 * time.Millisecond)
	c.TriggerArea(area)
	if len(delays) != 2 || delays[1] != time.Millisecond {
		t.Fatalf("after quiet delays = %v, want reset initial delay", delays)
	}
}

type manualTimer struct {
	fn      func()
	stopped bool
}

func TestOSPFRedistArbitrationFunctional(t *testing.T) {
	loc := locrib.NewRIB()
	in := NewInstaller(loc)
	pfx := netip.MustParsePrefix("10.40.0.0/24")
	staticID := redistevents.RegisterProtocol("static")
	staticPath := locrib.Path{Source: staticID, Instance: 0, NextHop: netip.MustParseAddr("10.0.0.1"), AdminDistance: 1, Metric: 1}
	loc.InsertForward(family.IPv4Unicast, pfx, staticPath, nil)
	in.Apply([]RouteEntry{{
		AreaID: testArea(), Prefix: pfx, Metric: 10, Type: RouteIntraArea,
		Origin: testRID(t, "2.2.2.2"), NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.2")}},
	}})
	best, ok := loc.Best(family.IPv4Unicast, pfx)
	if !ok || best.Source != staticID {
		t.Fatalf("best with static distance 1 = %+v, want static", best)
	}
	staticPath.AdminDistance = 200
	loc.InsertForward(family.IPv4Unicast, pfx, staticPath, nil)
	best, ok = loc.Best(family.IPv4Unicast, pfx)
	if !ok || best.Source != ProtocolID() {
		t.Fatalf("best with static distance 200 = %+v, want OSPF", best)
	}
}
func (t *manualTimer) Stop() bool {
	if t.stopped {
		return false
	}
	t.stopped = true
	return true
}

func (t *manualTimer) fire() {
	if t.stopped || t.fn == nil {
		return
	}
	t.stopped = true
	t.fn()
}
