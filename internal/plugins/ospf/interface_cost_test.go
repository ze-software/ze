// Design: docs/architecture/ospf/ospf-4-component-config.md -- OSPF interface output cost.
//
// Auto-cost derivation and its wiring: the `ospf/reference-bandwidth` leaf reaches the cost
// of every interface that configures none, an explicit `cost` leaf still wins, and an
// interface whose link speed the kernel does not report keeps cost 1.

package ospf

import (
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
	if !res.changed["eth0"] {
		t.Errorf("reconcile journal = %+v, want eth0 restarted: its cost changed", res)
	}
	if res.changed["eth1"] {
		t.Errorf("reconcile journal = %+v, want eth1 untouched: an explicit cost is not re-priced", res)
	}
}

// VALIDATES: the restart decision follows the derived COST, not the reference bandwidth. An
// interface whose cost changes is restarted; one whose cost is unchanged is left alone,
// whether it sets an explicit `cost`, sits on a link fast enough that the new quotient
// truncates to the old one, or sits on a link the kernel does not price.
// PREVENTS: a bounce that buys nothing. Every adjacency on the router re-forms and the
// advertised metric is byte-identical, which on a VPP dataplane or a non-Linux host is
// EVERY interface, because none of them is priced at any reference bandwidth.
func TestInterfaceGlobalParamsChangedReferenceBandwidth(t *testing.T) {
	stubLinkSpeed(t, map[string]uint64{"eth0": 1000, "eth1": 1000, "eth2": 10000})
	oldCfg := ospfConfig{ReferenceBandwidth: 100000}
	newCfg := ospfConfig{ReferenceBandwidth: 10000}
	auto := interfaceConfig{Name: "eth0"}
	explicit := interfaceConfig{Name: "eth1", Cost: 10, HasCost: true}

	// 100000 over 1000 is 100; 10000 over 1000 is 10. eth0 is re-priced, so it restarts.
	if !interfaceGlobalParamsChanged(oldCfg, newCfg, auto) {
		t.Error("an auto-cost interface was not restarted by a reference-bandwidth change that re-prices it")
	}
	if interfaceGlobalParamsChanged(oldCfg, newCfg, explicit) {
		t.Error("an explicitly-costed interface was restarted by a reference-bandwidth change")
	}
	if interfaceGlobalParamsChanged(oldCfg, oldCfg, auto) {
		t.Error("an unchanged reference bandwidth restarted an interface")
	}

	// 100000 and 105000 both price a 10 Gbit/s link at 10: the quotient truncates.
	sameQuotient := ospfConfig{ReferenceBandwidth: 105000}
	tenGig := interfaceConfig{Name: "eth2"}
	if interfaceGlobalParamsChanged(oldCfg, sameQuotient, tenGig) {
		t.Error("a reference-bandwidth change that leaves the cost at 10 restarted the interface")
	}
	// veth0 has no speed in the table, which is what the kernel reports for a loopback or a
	// tunnel, and what a VPP dataplane and a non-Linux host report for every interface.
	unpriced := interfaceConfig{Name: "veth0"}
	if interfaceGlobalParamsChanged(oldCfg, newCfg, unpriced) {
		t.Error("a reference-bandwidth change restarted an interface whose link speed the kernel does not report")
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

// VALIDATES: AC-8b -- a reload that re-prices an interface drops the adjacency that
// interface held, and the runtime that replaces it advertises the new cost. The path is the
// production one: parseOSPFConfig, reconcile, interfaceGlobalParamsChanged and
// startInterfaceLocked, over an interface that holds a neighbor.
// PREVENTS: a re-price that leaves the old runtime and its neighbor records in place, and a
// restart that drops the adjacency without changing the cost it re-forms at.
// The neighbor reaches 2-Way rather than Full: Full needs a database exchange with a peer,
// and ospf-auto-cost-frr is where a peer exists.
func TestReferenceBandwidthReloadDropsAdjacencyAndReprices(t *testing.T) {
	stubLinkSpeed(t, map[string]uint64{"eth0": 1000})
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","reference-bandwidth":"100000","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	eng := newEngine(transport.New(&fakeBackend{}))
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	defer eng.shutdown()

	before := eng.interfaces["eth0"]
	if before == nil {
		t.Fatal("eth0 has no interface runtime after openInterfaces")
	}
	detail := before.Snapshot()
	hello := types.Hello{
		HelloInterval: detail.HelloInterval,
		DeadInterval:  uint32(detail.DeadInterval),
		Options:       types.OptionE,
		Priority:      1,
	}
	if reason := before.ReceiveHello(types.RouterID{10, 0, 0, 2}, hello, time.Now()); reason != "" {
		t.Fatalf("ReceiveHello: %s", reason)
	}
	if got := before.Snapshot().NeighborCount; got != 1 {
		t.Fatalf("neighbors before the reload = %d, want 1: the reload needs an adjacency to drop", got)
	}
	if got := topologyCost(t, eng, "eth0"); got != 100 {
		t.Fatalf("eth0 advertised cost = %d, want 100 before the reload", got)
	}

	lowered, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","reference-bandwidth":"10000","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(lowered): %v", err)
	}
	res := eng.reconcile(lowered)

	if !res.changed["eth0"] {
		t.Fatalf("reconcile journal = %+v, want eth0 restarted: 10000 over a 1G link prices it 10, not 100", res)
	}
	if got := before.Snapshot().NeighborCount; got != 0 {
		t.Errorf("the replaced runtime kept %d neighbors, want 0: the re-price drops the adjacency", got)
	}
	after := eng.interfaces["eth0"]
	if after == before {
		t.Fatal("reconcile kept the interface runtime it re-priced, so no adjacency was dropped")
	}
	if got := after.Snapshot().NeighborCount; got != 0 {
		t.Errorf("the replacement runtime starts with %d neighbors, want 0", got)
	}
	if got := after.Snapshot().Cost; got != 10 {
		t.Errorf("the replacement runtime advertises cost %d, want the re-priced 10", got)
	}
	if got := topologyCost(t, eng, "eth0"); got != 10 {
		t.Errorf("eth0 advertised cost = %d after the reload, want 10", got)
	}
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
	teMetric, found := 0, false
	for _, o := range eng.teOriginateType1(cfg.RouterID) {
		lsa := decodeOrigTELSA(t, o)
		if !lsa.IsLink || !lsa.Link.HasTEMetric {
			continue
		}
		teMetric, found = int(lsa.Link.TEMetric), true
	}
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
	teMetric, found = 0, false
	for _, o := range eng.teOriginateType1(cfg.RouterID) {
		// eth1 left the topology on this pass, so its LSA is withdrawn: a MaxAge flush
		// carries no body to decode.
		if o.Withdraw {
			continue
		}
		lsa := decodeOrigTELSA(t, o)
		if !lsa.IsLink || !lsa.Link.HasTEMetric {
			continue
		}
		teMetric, found = int(lsa.Link.TEMetric), true
	}
	if !found {
		t.Fatal("no TE Link LSA carrying a TE metric was originated for eth0")
	}
	if teMetric != 100 {
		t.Errorf("eth0 TE metric = %d, want the derived 100: one interface reads the same cost four ways", teMetric)
	}
}
