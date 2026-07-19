// VALIDATES: spec-ospf-ext-15 AC-2/AC-3/AC-5/AC-6/AC-12 -- the per-AF engine spawn (one
// v6-codec engine per configured RFC 5838 address family), the Instance-ID demux across
// instances, the AF-bit adjacency gate (non-default AF requires it, default AF ignores it),
// and the add/remove engine lifecycle.
// PREVENTS: a mis-ranged Instance ID mapping to the wrong AF, cross-AF LSDB sharing, an
// AF-bit gate that breaks the default IPv6-unicast instance, or a leaked engine on removal.
package ospf

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/transport"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
	ospfv3packet "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/v3/types"
)

// twoAFConfig parses a config with the default IPv6-unicast AF (instance 0) plus an
// IPv4-unicast AF (instance 64), the canonical two-AF multi-AF setup.
func twoAFConfig(t *testing.T) ospfConfig {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1",`+
		`"address-family":{`+
		`"ipv6":{"instance-id":0,"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-point"}}}},`+
		`"ipv4-unicast":{"instance-id":64,"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth1":{"area":"0","network-type":"point-to-point"}}}}`+
		`}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	return cfg
}

// TestMultiAFEngineSpawn pins the wiring: a two-AF config spawns one v6-codec engine per AF,
// each at its configured Instance ID with the correct Loc-RIB install family (AC-2, wiring).
func TestMultiAFEngineSpawn(t *testing.T) {
	cfg := twoAFConfig(t)
	m := newV6EngineSet()
	defer m.shutdownAll()
	m.configure(cfg.v6Families(), cfg.multiAF())

	e6u, ok := m.engineFor(afIPv6Unicast)
	if !ok {
		t.Fatal("no IPv6-unicast engine spawned")
	}
	if e6u.dispatch.instanceID != 0 || e6u.installFamily() != family.IPv6Unicast {
		t.Errorf("v6u engine: instance=%d family=%s, want 0/ipv6-unicast", e6u.dispatch.instanceID, e6u.installFamily())
	}
	e4u, ok := m.engineFor(afIPv4Unicast)
	if !ok {
		t.Fatal("no IPv4-unicast engine spawned")
	}
	if e4u.dispatch.instanceID != 64 || e4u.installFamily() != family.IPv4Unicast {
		t.Errorf("v4u engine: instance=%d family=%s, want 64/ipv4-unicast", e4u.dispatch.instanceID, e4u.installFamily())
	}
	// The default IPv6-unicast instance is multi-AF-aware here, so it emits the AF-bit.
	if !e6u.emitAFBit() || !e4u.emitAFBit() {
		t.Error("both instances must emit the AF-bit when multiple AFs are configured")
	}
}

// TestPerAFLSDBIsolation pins AC-2/A-2/R-5: each AF engine owns a private LSDB, neighbor
// table, and SPF, so per-AF separation is structural (no shared storage to leak across AFs).
func TestPerAFLSDBIsolation(t *testing.T) {
	cfg := twoAFConfig(t)
	m := newV6EngineSet()
	defer m.shutdownAll()
	m.configure(cfg.v6Families(), cfg.multiAF())
	e6u, _ := m.engineFor(afIPv6Unicast)
	e4u, _ := m.engineFor(afIPv4Unicast)
	if e6u.lsdb == nil || e4u.lsdb == nil || e6u.lsdb == e4u.lsdb {
		t.Errorf("AF engines share (or lack) an LSDB: %p vs %p", e6u.lsdb, e4u.lsdb)
	}
	if e6u.neighbors == e4u.neighbors {
		t.Error("AF engines share a neighbor table")
	}
	if e6u.spf == e4u.spf {
		t.Error("AF engines share an SPF computer")
	}
}

// TestAFReconcileAddRemove pins AC-12/R-6: adding an AF spawns its engine; removing an AF
// shuts the engine down and forgets it (no leak), while the other AF is unaffected.
func TestAFReconcileAddRemove(t *testing.T) {
	m := newV6EngineSet()
	defer m.shutdownAll()

	// Start with only the default IPv6-unicast AF.
	only6, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","address-family":{"ipv6":{"instance-id":0,"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}}}}}}`), nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m.apply(only6.v6Families(), only6.multiAF())
	if _, ok := m.engineFor(afIPv4Unicast); ok {
		t.Fatal("IPv4-unicast engine present before it was configured")
	}
	if _, ok := m.engineFor(afIPv6Unicast); !ok {
		t.Fatal("IPv6-unicast engine missing")
	}

	// Add the IPv4-unicast AF.
	both := twoAFConfig(t)
	m.apply(both.v6Families(), both.multiAF())
	if _, ok := m.engineFor(afIPv4Unicast); !ok {
		t.Fatal("IPv4-unicast engine not spawned on add")
	}

	// Remove the IPv4-unicast AF: its engine stops; the IPv6-unicast engine survives.
	m.apply(only6.v6Families(), only6.multiAF())
	if _, ok := m.engineFor(afIPv4Unicast); ok {
		t.Error("IPv4-unicast engine still present after removal")
	}
	if _, ok := m.engineFor(afIPv6Unicast); !ok {
		t.Error("IPv6-unicast engine wrongly removed")
	}
}

// v4uEngine builds a running IPv4-unicast-over-OSPFv3 engine (instance 64) over a fake
// transport, returning the engine and its eth1 ifindex.
func v4uEngine(t *testing.T) (*engine, int) {
	t.Helper()
	cfg := twoAFConfig(t)
	var sub ospfConfig
	fams := cfg.v6Families()
	for i := range fams {
		if fams[i].af == afIPv4Unicast {
			sub = fams[i].cfg
		}
	}
	fb := &fakeBackend{}
	eng := newEngineWithCodecAF(transport.New(fb), v6Codec{}, afIPv4Unicast)
	eng.setMultiAF(true)
	eng.setConfig(sub)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	fb.mu.Lock()
	handle := fb.handles["eth1"]
	fb.mu.Unlock()
	if handle == nil {
		t.Fatal("eth1 transport handle missing")
	}
	return eng, handle.ifindex
}

// TestAFBitGatesFullNonDefault pins AC-5/R-2: on a non-default AF a Hello without the AF-bit
// forms no adjacency, while the same Hello with the AF-bit does.
// RFC requirement: RFC5838-2.4-1 negative -- on a non-default AF a Hello with the AF-bit clear forms no adjacency; it is discarded before the neighbor reaches Full.
// RFC requirement: RFC5838-2.4-1 positive -- on a non-default AF the same Hello carrying the AF-bit is accepted and brings the neighbor up.
func TestAFBitGatesFullNonDefault(t *testing.T) {
	eng, ifindex := v4uEngine(t)
	defer eng.shutdown()
	peer := ridOf("10.0.0.2")
	src := netip.MustParseAddr("fe80::2")
	dst := netip.MustParseAddr("ff02::5")
	area := types.AreaID{}

	noAF := ospfv3packet.Hello{InterfaceID: 2, Priority: 1, Options: ospfv3types.OptV6 | ospfv3types.OptR | ospfv3types.OptE, HelloInterval: DefaultHelloInterval, RouterDeadInterval: DefaultDeadInterval}
	dispatchHelloV6Instance(t, eng, ifindex, peer, area, src, dst, 64, noAF)
	if rows := eng.neighborSnapshot(); len(rows) != 0 {
		t.Fatalf("non-default AF formed %d neighbor(s) from an AF-bit-less Hello; want 0 (RFC 5838 §2.5)", len(rows))
	}

	withAF := noAF
	withAF.Options = withAF.Options.SetAF()
	dispatchHelloV6Instance(t, eng, ifindex, peer, area, src, dst, 64, withAF)
	if rows := eng.neighborSnapshot(); len(rows) != 1 {
		t.Fatalf("non-default AF formed %d neighbor(s) from an AF-bit Hello; want 1", len(rows))
	}
}

// TestAFBitIgnoredDefaultAF pins AC-6/A-8: the default IPv6-unicast instance still forms an
// adjacency with a neighbor whose Hello omits the AF-bit (RFC 5838 §2.6).
// RFC requirement: RFC5838-2.4-2 positive -- the base IPv6-unicast AF does not apply the AF-bit check, so it still forms an adjacency with a neighbor whose Hello omits the AF-bit.
func TestAFBitIgnoredDefaultAF(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-point"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fb := &fakeBackend{}
	eng := newEngineWithCodecAF(transport.New(fb), v6Codec{}, afIPv6Unicast)
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	defer eng.shutdown()
	fb.mu.Lock()
	handle := fb.handles["eth0"]
	fb.mu.Unlock()

	// No AF-bit set: the default AF must still form the adjacency (backward interop).
	noAF := ospfv3packet.Hello{InterfaceID: 2, Priority: 1, Options: ospfv3types.OptV6 | ospfv3types.OptR | ospfv3types.OptE, HelloInterval: DefaultHelloInterval, RouterDeadInterval: DefaultDeadInterval}
	dispatchHelloV6Instance(t, eng, handle.ifindex, ridOf("10.0.0.2"), cfg.Areas[0].AreaID, netip.MustParseAddr("fe80::2"), netip.MustParseAddr("ff02::5"), 0, noAF)
	if rows := eng.neighborSnapshot(); len(rows) != 1 {
		t.Fatalf("default IPv6-unicast AF formed %d neighbor(s) from an AF-bit-less Hello; want 1 (RFC 5838 §2.6)", len(rows))
	}
}

// TestMultiAFInstanceDemux pins AC-3/A-1: two AF instances on one link each accept only their
// own Instance ID. A Hello for Instance 64 reaches the IPv4-unicast instance, not the
// IPv6-unicast instance (RFC 5340 §4.2.2 demux, reused per instance).
func TestMultiAFInstanceDemux(t *testing.T) {
	// IPv6-unicast engine at Instance 0.
	cfg6, _ := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-point"}}}}}`), nil)
	fb6 := &fakeBackend{}
	e6u := newEngineWithCodecAF(transport.New(fb6), v6Codec{}, afIPv6Unicast)
	e6u.setMultiAF(true)
	e6u.setConfig(cfg6)
	if err := e6u.openInterfaces(); err != nil {
		t.Fatalf("v6u openInterfaces: %v", err)
	}
	defer e6u.shutdown()
	fb6.mu.Lock()
	h6 := fb6.handles["eth0"]
	fb6.mu.Unlock()

	// IPv4-unicast engine at Instance 64.
	e4u, if4 := v4uEngine(t)
	defer e4u.shutdown()

	peer := ridOf("10.0.0.2")
	src := netip.MustParseAddr("fe80::2")
	dst := netip.MustParseAddr("ff02::5")
	// A Hello carrying Instance 64 + AF-bit.
	hello := ospfv3packet.Hello{InterfaceID: 2, Priority: 1, Options: (ospfv3types.OptV6 | ospfv3types.OptR | ospfv3types.OptE).SetAF(), HelloInterval: DefaultHelloInterval, RouterDeadInterval: DefaultDeadInterval}

	dispatchHelloV6Instance(t, e6u, h6.ifindex, peer, cfg6.Areas[0].AreaID, src, dst, 64, hello)
	if rows := e6u.neighborSnapshot(); len(rows) != 0 {
		t.Fatalf("Instance-64 Hello formed %d neighbor(s) on the Instance-0 engine; want 0 (demux)", len(rows))
	}
	dispatchHelloV6Instance(t, e4u, if4, peer, types.AreaID{}, src, dst, 64, hello)
	if rows := e4u.neighborSnapshot(); len(rows) != 1 {
		t.Fatalf("Instance-64 Hello formed %d neighbor(s) on the Instance-64 engine; want 1", len(rows))
	}
}
