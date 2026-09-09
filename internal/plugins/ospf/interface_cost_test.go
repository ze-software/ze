// Design: docs/architecture/ospf/ospf-4-component-config.md -- OSPF interface output cost.
//
// Auto-cost derivation and its wiring: the `ospf/reference-bandwidth` leaf reaches the cost
// of every interface that configures none, an explicit `cost` leaf still wins, and an
// interface whose link speed the kernel does not report keeps cost 1.

package ospf

import (
	"slices"
	"testing"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3transport "github.com/ze-software/ze/internal/plugins/ospf/v3/transport"
)

// stubLinkSpeed makes interfaceLinkSpeedMbps answer from a table for the duration of one
// test. The interface names below do not exist on the host, and a real one would report
// whatever the kernel decides, so a case that wants a speed has to state it.
func stubLinkSpeed(t *testing.T, speeds map[string]uint64) {
	t.Helper()
	previous := interfaceLinkSpeedMbps
	interfaceLinkSpeedMbps = func(name string) uint64 { return speeds[name] }
	t.Cleanup(func() { interfaceLinkSpeedMbps = previous })
}

// VALIDATES: an interface with no `cost` leaf takes reference-bandwidth / link-speed, an
// explicit `cost` leaf wins over the derivation, and an unknown link speed takes cost 1.
// PREVENTS: the `reference-bandwidth` leaf going unread again, auto-cost overriding an
// operator's explicit cost, and a virtual or down link deriving cost 0 from a zero speed.
func TestInterfaceCostAutoDerivation(t *testing.T) {
	stubLinkSpeed(t, map[string]uint64{
		"eth0": 1000,   // 1 Gbit/s
		"eth1": 10000,  // 10 Gbit/s
		"eth2": 100000, // 100 Gbit/s
		"eth3": 10,     // 10 Mbit/s
		// veth0 is absent: the kernel reports no speed for it.
	})
	cases := []struct {
		name          string
		ic            interfaceConfig
		referenceMbps uint32
		want          uint16
	}{
		{name: "1G under the 100G default", ic: interfaceConfig{Name: "eth0"}, referenceMbps: 100000, want: 100},
		{name: "10G under the 100G default", ic: interfaceConfig{Name: "eth1"}, referenceMbps: 100000, want: 10},
		{name: "100G reaches the floor", ic: interfaceConfig{Name: "eth2"}, referenceMbps: 100000, want: 1},
		{name: "10M under the 100G default", ic: interfaceConfig{Name: "eth3"}, referenceMbps: 100000, want: 10000},
		{name: "10M reaches the ceiling", ic: interfaceConfig{Name: "eth3"}, referenceMbps: 4294967, want: 65535},
		{name: "1G under a 1G reference", ic: interfaceConfig{Name: "eth0"}, referenceMbps: 1000, want: 1},
		{name: "10G under a 4294967 reference", ic: interfaceConfig{Name: "eth1"}, referenceMbps: 4294967, want: 429},
		{name: "explicit cost wins", ic: interfaceConfig{Name: "eth0", Cost: 7, HasCost: true}, referenceMbps: 100000, want: 7},
		{name: "explicit cost wins on an unknown speed", ic: interfaceConfig{Name: "veth0", Cost: 7, HasCost: true}, referenceMbps: 100000, want: 7},
		{name: "unknown link speed", ic: interfaceConfig{Name: "veth0"}, referenceMbps: 100000, want: costLinkSpeedUnknown},
		{name: "unset reference bandwidth", ic: interfaceConfig{Name: "eth0"}, referenceMbps: 0, want: costLinkSpeedUnknown},
	}
	for _, tc := range cases {
		if got := interfaceCost(tc.ic, tc.referenceMbps); got != tc.want {
			t.Errorf("%s: interfaceCost(%q, %d) = %d, want %d", tc.name, tc.ic.Name, tc.referenceMbps, got, tc.want)
		}
	}
}

// VALIDATES: the production link-speed reader answers 0 rather than failing for a name the
// kernel does not know.
// PREVENTS: the stub in every other test hiding a reader that panics or reports a speed for
// an interface that does not exist.
func TestInterfaceLinkSpeedMbpsUnknownName(t *testing.T) {
	if got := interfaceLinkSpeedMbps("ze-no-such-link0"); got != 0 {
		t.Fatalf("interfaceLinkSpeedMbps(unknown) = %d, want 0", got)
	}
}

// VALIDATES: the `reference-bandwidth` config leaf reaches the cost the Router-LSA
// advertises and the cost `show ospf interface` reports, and a reload re-prices an
// auto-cost interface.
// PREVENTS: the leaf being stored and never read -- the state this spec exists to close.
func TestOSPFTopologyCostFollowsReferenceBandwidth(t *testing.T) {
	stubLinkSpeed(t, map[string]uint64{"eth0": 1000, "eth1": 1000})
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","reference-bandwidth":"100000","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"},"eth1":{"area":"0","cost":"10"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	eng := newEngine(transport.New(&fakeBackend{}))
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	defer eng.shutdown()

	// 100000 Mbps over a 1000 Mbps link is cost 100; eth1 configures 10 and keeps it.
	if got := topologyCost(t, eng, "eth0"); got != 100 {
		t.Fatalf("eth0 advertised cost = %d, want 100 from reference-bandwidth 100000 over a 1G link", got)
	}
	if got := topologyCost(t, eng, "eth1"); got != 10 {
		t.Fatalf("eth1 advertised cost = %d, want the explicit 10", got)
	}
	if got := snapshotByName(t, eng.interfaceSnapshot(), "eth0").Cost; got != 100 {
		t.Fatalf("eth0 runtime cost = %d, want 100: `show ospf interface` must report the advertised cost", got)
	}

	lowered, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","reference-bandwidth":"10000","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"},"eth1":{"area":"0","cost":"10"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(lowered): %v", err)
	}
	res := eng.reconcile(lowered)
	if got := topologyCost(t, eng, "eth0"); got != 10 {
		t.Fatalf("eth0 advertised cost = %d after lowering the reference bandwidth to 10000, want 10", got)
	}
	if got := snapshotByName(t, eng.interfaceSnapshot(), "eth0").Cost; got != 10 {
		t.Fatalf("eth0 runtime cost = %d after the reload, want 10: the snapshot must not keep the old cost", got)
	}
	if res.changed["eth0"] || res.changed["eth1"] {
		t.Errorf("reconcile journal = %+v, want no interface restarted: a re-pricing publishes through the next origination pass", res)
	}
}

// VALIDATES: the restart decision covers what an interface STAMPS into the packets it
// sends, and nothing else. A Router ID or an area-type change recreates the runtime; a
// reference-bandwidth change never does, whether or not it re-prices the interface.
// PREVENTS: a bounce that buys nothing. The cost reaches the Router-LSA through
// lsdbTopology on the next origination pass, so restarting an interface to publish it drops
// every neighbor on it for a number the wire was going to carry anyway. On a router with
// forty auto-costed links that is forty adjacencies for one commit.
func TestInterfaceGlobalParamsChangedIgnoresReferenceBandwidth(t *testing.T) {
	stubLinkSpeed(t, map[string]uint64{"eth0": 1000})
	backbone := []areaConfig{{AreaID: types.BackboneArea, AreaType: areaTypeNormal}}
	oldCfg := ospfConfig{RouterID: types.RouterID{10, 0, 0, 1}, ReferenceBandwidth: 100000, Areas: backbone}
	auto := interfaceConfig{Name: "eth0", AreaID: types.BackboneArea}

	// 100000 over 1000 is cost 100; 10000 over 1000 is cost 10. The metric moves and the
	// interface still keeps its adjacency.
	lowered := oldCfg
	lowered.ReferenceBandwidth = 10000
	if interfaceGlobalParamsChanged(oldCfg, lowered, auto) {
		t.Error("a reference-bandwidth change restarted an auto-cost interface: the Router-LSA carries the new cost without one")
	}
	if interfaceGlobalParamsChanged(oldCfg, oldCfg, auto) {
		t.Error("an unchanged config restarted an interface")
	}

	// The two changes that DO reach the wire through the interface runtime: the Router ID is
	// stamped into every Hello, and the area type decides the E-bit and the N-bit.
	renamed := oldCfg
	renamed.RouterID = types.RouterID{10, 0, 0, 9}
	if !interfaceGlobalParamsChanged(oldCfg, renamed, auto) {
		t.Error("a Router ID change did not restart the interface: its Hellos would carry the old identity")
	}
	stub := oldCfg
	stub.Areas = []areaConfig{{AreaID: types.BackboneArea, AreaType: areaTypeStub}}
	if !interfaceGlobalParamsChanged(oldCfg, stub, auto) {
		t.Error("an area-type change did not restart the interface: its Hellos would carry the old E-bit")
	}
}

// VALIDATES: one `reference-bandwidth` leaf prices a link the same way in the OSPFv2 family
// and in every RFC 5838 OSPFv3 address family, so both Router-LSAs advertise one cost for
// one physical link.
// PREVENTS: an address-family engine keeping the seeded default while the operator's leaf
// reaches IPv4 alone. That advertises two costs for the same link, 47 and 10 under the
// config below, and no configuration makes them agree: the `ospf-af-topology` grouping
// carries no `reference-bandwidth` leaf of its own.
func TestReferenceBandwidthReachesEveryAddressFamily(t *testing.T) {
	stubLinkSpeed(t, map[string]uint64{"eth0": 10000})
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","reference-bandwidth":"470000","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}},"address-family":{"ipv6":{"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}}},"ipv4-multicast":{"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if cfg.V6 == nil {
		t.Fatal("address-family ipv6 was not parsed into cfg.V6")
	}

	// 470000 Mbps over a 10 Gbit/s link is cost 47, the number ospf-auto-cost-frr reads out
	// of Ze's Router-LSA. The v6 default is 100000, so an uninherited numerator gives 10.
	const wantCost = 47

	eng4 := newEngine(transport.New(&fakeBackend{}))
	eng4.setConfig(cfg)
	if err := eng4.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces(v4): %v", err)
	}
	defer eng4.shutdown()
	if got := topologyCost(t, eng4, "eth0"); got != wantCost {
		t.Errorf("OSPFv2 advertised cost = %d, want %d from reference-bandwidth 470000 over a 10G link", got, wantCost)
	}

	eng6 := newEngineWithCodecAF(ospfv3transport.New(&fakeV6Backend{}), v6Codec{}, afIPv6Unicast)
	eng6.setConfig(*cfg.V6)
	if err := eng6.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces(v6): %v", err)
	}
	defer eng6.shutdown()
	if got := topologyCost(t, eng6, "eth0"); got != wantCost {
		t.Errorf("OSPFv3 advertised cost = %d, want %d: both families price one link alike", got, wantCost)
	}

	// v6Families is what register_multiaf hands to each address-family engine, so every
	// entry in it carries the numerator the operator configured.
	for _, fam := range cfg.v6Families() {
		if fam.cfg.ReferenceBandwidth != 470000 {
			t.Errorf("address family %s reference bandwidth = %d, want the router-wide 470000", fam.af.String(), fam.cfg.ReferenceBandwidth)
		}
	}
}

// VALIDATES: AC-8b and AC-11 on a reload -- an OSPFv3 address family re-prices a link the
// way the OSPFv2 family does. The numerator reaches the address-family engine through
// v6Families, which is what v6EngineSet.apply hands to (*engine).reconcile, so the interface
// restarts and the OSPFv3 origination topology carries the new cost.
// PREVENTS: an OSPFv3 family that keeps the cost it started with after a reload. The two
// families would then advertise two costs for one link until the daemon restarts, which is
// the round 1 BLOCKER re-appearing on the reload path rather than on the initial config.
func TestReferenceBandwidthReloadRepricesEveryAddressFamily(t *testing.T) {
	stubLinkSpeed(t, map[string]uint64{"eth0": 10000})
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","reference-bandwidth":"470000","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}},"address-family":{"ipv6":{"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if cfg.V6 == nil {
		t.Fatal("address-family ipv6 was not parsed into cfg.V6")
	}
	eng6 := newEngineWithCodecAF(ospfv3transport.New(&fakeV6Backend{}), v6Codec{}, afIPv6Unicast)
	eng6.setConfig(*cfg.V6)
	if err := eng6.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces(v6): %v", err)
	}
	defer eng6.shutdown()
	if got := topologyCost(t, eng6, "eth0"); got != 47 {
		t.Fatalf("OSPFv3 cost before the reload = %d, want 47 from reference-bandwidth 470000 over a 10G link", got)
	}

	lowered, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","reference-bandwidth":"235000","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}},"address-family":{"ipv6":{"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(lowered): %v", err)
	}
	loweredV6, found := ospfConfig{}, false
	for _, fam := range lowered.v6Families() {
		if fam.af == afIPv6Unicast {
			loweredV6, found = fam.cfg, true
		}
	}
	if !found {
		t.Fatal("v6Families carries no ipv6-unicast entry to reconcile with")
	}
	before := eng6.interfaces["eth0"]
	res := eng6.reconcile(loweredV6)
	if res.changed["eth0"] {
		t.Errorf("OSPFv3 reconcile journal = %+v, want eth0 untouched: a re-pricing must not restart the interface", res)
	}
	if eng6.interfaces["eth0"] != before {
		t.Error("the OSPFv3 reconcile replaced the interface runtime it re-priced, so it dropped the adjacency")
	}
	// 23 is 235000 over 10000. No default produces it: the seeded 100000 gives 10, so a
	// family that failed to inherit the reload's numerator cannot read 23 by accident.
	if got := topologyCost(t, eng6, "eth0"); got != 23 {
		t.Errorf("OSPFv3 cost after the reload = %d, want the re-priced 23", got)
	}
	if got := snapshotByName(t, eng6.interfaceSnapshot(), "eth0").Cost; got != 23 {
		t.Errorf("OSPFv3 `show ospf interface` cost after the reload = %d, want the re-priced 23", got)
	}
}

// VALIDATES: AC-8b -- a reload that re-prices an interface keeps the 2-Way neighbor that
// interface holds, keeps the runtime holding it, and still re-prices the Router-LSA metric
// and the cost `show ospf interface` reports. A reload that changes the Router ID restarts
// the same interface, which is what proves the re-price is a decision rather than a lost
// restart.
// PREVENTS: the outage the owner refused on 2026-09-09. A router with forty auto-costed
// links drops forty neighbors at the commit, each re-forming over a dead interval and a
// database exchange, to publish a metric lsdbTopology derives from e.cfg with no restart at
// all.
// The neighbor reaches 2-Way rather than Full: the Hello names this router, which is what
// receiveHello reads for TwoWay, while Full needs a database exchange with a peer and
// ospf-auto-cost-frr is where a peer exists.
func TestReferenceBandwidthReloadKeepsNeighborAndReprices(t *testing.T) {
	stubLinkSpeed(t, map[string]uint64{"eth0": 1000})
	// min-ls-interval-ms 1 lets the reload's origination install inside one test. RFC 2328
	// Appendix B sets MinLSInterval to 5 seconds, which defers a second origination of one
	// LSA whatever produced it, so the default would hide the reconcile pass rather than the
	// rate limit.
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","reference-bandwidth":"100000","timers":{"min-ls-interval-ms":"1"},"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if cfg.Timers.MinLSIntervalMS != 1 {
		t.Fatalf("min-ls-interval-ms = %d, want 1: the reload origination would be rate-limited", cfg.Timers.MinLSIntervalMS)
	}
	eng := newEngine(transport.New(&fakeBackend{}))
	eng.setConfig(cfg)
	addressedTopology(eng)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	defer eng.shutdown()

	before := eng.interfaces["eth0"]
	if before == nil {
		t.Fatal("eth0 has no interface runtime after openInterfaces")
	}
	peer := types.RouterID{10, 0, 0, 2}
	detail := before.Snapshot()
	// The Hello lists this router, so helloHasNeighbor answers true and the neighbor reaches
	// 2-Way. A Hello that lists nobody leaves it one-way in Init, and NeighborCount counts a
	// one-way neighbor the same, so the neighbor state is read out of the neighbor table.
	hello := types.Hello{
		HelloInterval: detail.HelloInterval,
		DeadInterval:  uint32(detail.DeadInterval),
		Options:       types.OptionE,
		Priority:      1,
		Neighbors:     []types.RouterID{cfg.RouterID},
	}
	if reason := before.ReceiveHello(peer, hello, time.Now()); reason != "" {
		t.Fatalf("ReceiveHello: %s", reason)
	}
	if snap, ok := eng.neighbors.Lookup("eth0", peer); !ok || snap.State != "2-way" {
		t.Fatalf("neighbor state before the reload = %q (present %v), want 2-way: the Hello names this router", snap.State, ok)
	}
	if got := topologyCost(t, eng, "eth0"); got != 100 {
		t.Fatalf("eth0 advertised cost = %d, want 100 before the reload", got)
	}
	eng.originateSelfLSAs()
	if got := selfRouterLSAMetric(t, eng, cfg.RouterID); got != 100 {
		t.Fatalf("Router-LSA link metric = %d, want 100 before the reload", got)
	}

	lowered, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","reference-bandwidth":"10000","timers":{"min-ls-interval-ms":"1"},"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(lowered): %v", err)
	}
	// Past the 1 ms MinLSInterval set above, so the reconcile's origination installs rather
	// than being deferred by RFC 2328 Appendix B rate limiting.
	time.Sleep(2 * time.Millisecond)
	res := eng.reconcile(lowered)

	if res.changed["eth0"] {
		t.Errorf("reconcile journal = %+v, want eth0 untouched: a re-pricing must not restart the interface", res)
	}
	if eng.interfaces["eth0"] != before {
		t.Fatal("reconcile replaced the interface runtime it re-priced, so it dropped the adjacency")
	}
	if snap, ok := eng.neighbors.Lookup("eth0", peer); !ok || snap.State != "2-way" {
		t.Errorf("neighbor state after the re-pricing reload = %q (present %v), want 2-way: the neighbor must survive it", snap.State, ok)
	}
	if got := before.Snapshot().NeighborCount; got != 1 {
		t.Errorf("the re-priced runtime holds %d neighbors, want 1", got)
	}
	if got := topologyCost(t, eng, "eth0"); got != 10 {
		t.Errorf("eth0 advertised cost = %d after the reload, want the re-priced 10", got)
	}
	if got := snapshotByName(t, eng.interfaceSnapshot(), "eth0").Cost; got != 10 {
		t.Errorf("eth0 `show ospf interface` cost = %d after the reload, want the re-priced 10", got)
	}
	// The Router-LSA an OSPFv2 peer reads. reconcile originates before it returns, so the
	// commit publishes the new metric rather than waiting for an unrelated event: no
	// interface restarted, so no neighbor transition drives an origination pass.
	if got := selfRouterLSAMetric(t, eng, cfg.RouterID); got != 10 {
		t.Errorf("Router-LSA link metric = %d after the reload, want the re-priced 10", got)
	}

	// A Router ID change is stamped into every Hello the interface sends, so it still
	// recreates the runtime and drops the neighbor. Without this arm the assertions above
	// would also pass against a reconcile that never restarts anything.
	renamed, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.9","reference-bandwidth":"10000","timers":{"min-ls-interval-ms":"1"},"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(renamed): %v", err)
	}
	if res := eng.reconcile(renamed); !res.changed["eth0"] {
		t.Fatalf("reconcile journal = %+v, want eth0 restarted by a Router ID change", res)
	}
	if eng.interfaces["eth0"] == before {
		t.Error("a Router ID change kept the interface runtime, so the Hellos carry the old identity")
	}
	// InterfaceDown keeps the table entry and drops it to Down (RFC 2328 sec 10.2 KillNbr),
	// so the 2-Way the Hello reached is gone rather than the row.
	if held, ok := eng.neighbors.Lookup("eth0", peer); !ok || held.State != "down" {
		t.Errorf("eth0 neighbor after the Router ID restart = %q (present %v), want down", held.State, ok)
	}
}

// teMetricForLocalAddress returns the TE metric of the originated TE Link LSA whose local
// interface address is addr (RFC 3630 section 2.5.3 Local Interface IP Address sub-TLV).
// The LSA is selected by interface identity rather than by taking the last match: both
// interfaces below enable traffic-engineering, so a loop that overwrites its result would
// let the emission order decide which interface an assertion reads. A withdrawal carries no
// body, because a MaxAge flush has nothing to decode.
func teMetricForLocalAddress(t *testing.T, originations []opaqueOrigination, addr [4]byte) (int, bool) {
	t.Helper()
	for _, o := range originations {
		if o.Withdraw {
			continue
		}
		lsa := decodeOrigTELSA(t, o)
		if !lsa.IsLink || !lsa.Link.HasTEMetric {
			continue
		}
		if !slices.Contains(lsa.Link.LocalIPs, addr) {
			continue
		}
		return int(lsa.Link.TEMetric), true
	}
	return 0, false
}

// addressedTopology wires the engine's own origination topology into its LSDB with one
// substitution: an IPv4 address for every interface. eth0 does not exist on the test host,
// so interfaceIPv4Address reads none and the Router-LSA carries no link to read a metric
// off. Everything else in the topology, the derived cost included, is what lsdbTopology
// produced.
func addressedTopology(eng *engine) {
	eng.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo {
		topology := eng.lsdbTopology()
		for idx := range topology {
			topology[idx].Address = [4]byte{10, 0, 0, 1}
			topology[idx].NetworkMask = [4]byte{255, 255, 255, 0}
		}
		return topology
	})
}

// selfRouterLSAMetric returns the metric of the one link in this router's own Router-LSA,
// which is what an OSPFv2 peer reads off the wire.
func selfRouterLSAMetric(t *testing.T, eng *engine, router types.RouterID) types.Metric {
	t.Helper()
	key := types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(router), AdvertisingRouter: router}
	lsa, ok := eng.lsdb.LookupLSA(types.BackboneArea, key)
	if !ok {
		t.Fatal("this router originated no Router-LSA in the backbone area")
	}
	body, err := lsa.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	if len(body.Links) != 1 {
		t.Fatalf("Router-LSA carries %d links, want 1", len(body.Links))
	}
	return body.Links[0].Metric
}

// topologyCost returns the cost the LSA-origination topology carries for one interface.
func topologyCost(t *testing.T, eng *engine, name string) uint16 {
	t.Helper()
	topology := eng.lsdbTopology()
	for idx := range topology {
		if topology[idx].Name == name {
			return topology[idx].Cost
		}
	}
	t.Fatalf("interface %q is absent from the origination topology", name)
	return 0
}

// VALIDATES: the two AC-9 consumers no test observed -- the LDP-sync restore value and the
// RFC 3630 section 2.5.5 TE metric fallback -- carry the derived cost. eth0 is priced 100
// (100000 Mbps over a 1 Gbit/s link) and eth1 is priced 10 (over a 10 Gbit/s link), and
// both numbers are 1 without the derivation.
// PREVENTS: a consumer that reads ic.Cost again and prices a link at 1 while the Router-LSA
// advertises the derived cost, which would restore the wrong metric after LDP converges and
// publish a TE metric that disagrees with the link's own cost.
func TestDerivedCostReachesLDPSyncAndTEMetric(t *testing.T) {
	stubLinkSpeed(t, map[string]uint64{"eth0": 1000, "eth1": 10000})
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"1.1.1.1","router-address":"9.9.9.9","opaque":true,"reference-bandwidth":"100000","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"name":"eth0","area":"0","network-type":"broadcast","ldp-sync":{"enable":true},"traffic-engineering":{"enable":true}},"eth1":{"name":"eth1","area":"0","network-type":"point-to-point","traffic-engineering":{"enable":true}}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	eng := newEngine(transport.New(&fakeBackend{}))
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	defer eng.shutdown()

	// The LDP-sync restore value, read through the `show ospf ldp-sync` row. eth0 is
	// broadcast, so the row reports the stored cost rather than the point-to-point
	// LSInfinity cost-out.
	rows := eng.ldpSyncSnapshot()
	if len(rows) != 1 {
		t.Fatalf("ldp-sync rows = %d, want 1 for eth0", len(rows))
	}
	row, ok := rows[0].(ldpSyncSnapshotEntry)
	if !ok {
		t.Fatalf("ldp-sync row type = %T, want ldpSyncSnapshotEntry", rows[0])
	}
	if row.Interface != "eth0" {
		t.Fatalf("ldp-sync row interface = %q, want eth0", row.Interface)
	}
	if row.EffectiveMetric != 100 {
		t.Errorf("ldp-sync restore value = %d, want the derived 100: LDP-sync must restore the auto-cost, not 1", row.EffectiveMetric)
	}

	// The RFC 3630 section 2.5.5 TE metric fallback, read out of the originated Link LSA.
	eng.teOrig.setTopology(func() []ospflsdb.InterfaceInfo {
		return []ospflsdb.InterfaceInfo{p2pTopo("eth1", [4]byte{10, 0, 1, 1}, types.RouterID{2, 2, 2, 2}, "10.0.1.2")}
	})
	teMetric, found := teMetricForLocalAddress(t, eng.teOriginateType1(cfg.RouterID), [4]byte{10, 0, 1, 1})
	if !found {
		t.Fatal("no TE Link LSA carrying a TE metric was originated for eth1")
	}
	if teMetric != 10 {
		t.Errorf("TE metric = %d, want the derived 10: the RFC 3630 fallback must carry the cost the Router-LSA advertises", teMetric)
	}

	// AC-9 reads ONE interface four ways. eth0 carries both consumers above, so its
	// Router-LSA metric, its `show ospf interface` cost, its LDP-sync restore value and its
	// TE metric are the one derived 100.
	if got := topologyCost(t, eng, "eth0"); got != 100 {
		t.Errorf("eth0 Router-LSA cost = %d, want the derived 100", got)
	}
	if got := snapshotByName(t, eng.interfaceSnapshot(), "eth0").Cost; got != 100 {
		t.Errorf("eth0 `show ospf interface` cost = %d, want the derived 100", got)
	}
	// eth0 is broadcast, so its TE Link TLV takes the RFC 3630 section 2.5.2 multi-access
	// Link ID: this router is the DR, so the Link ID is its own interface address.
	eng.teOrig.setTopology(func() []ospflsdb.InterfaceInfo {
		return []ospflsdb.InterfaceInfo{{
			Name: "eth0", AreaID: types.BackboneArea, NetworkType: networkBroadcast,
			State: "dr", Address: [4]byte{10, 0, 0, 1},
			RouterID: types.RouterID{1, 1, 1, 1}, DR: types.RouterID{1, 1, 1, 1},
		}}
	})
	teMetric, found = teMetricForLocalAddress(t, eng.teOriginateType1(cfg.RouterID), [4]byte{10, 0, 0, 1})
	if !found {
		t.Fatal("no TE Link LSA carrying a TE metric was originated for eth0")
	}
	if teMetric != 100 {
		t.Errorf("eth0 TE metric = %d, want the derived 100: one interface reads the same cost four ways", teMetric)
	}
}
