// Design: docs/architecture/ospf/ospf-4-component-config.md -- OSPF interface output cost.
//
// Auto-cost derivation and its wiring: the `ospf/reference-bandwidth` leaf reaches the cost
// of every interface that configures none, an explicit `cost` leaf still wins, and an
// interface whose link speed the kernel does not report keeps cost 1.

package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/transport"
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

// VALIDATES: a reference-bandwidth change restarts an auto-cost interface and leaves an
// explicitly-costed one alone.
// PREVENTS: a re-costing bouncing an adjacency whose cost did not change.
func TestInterfaceGlobalParamsChangedReferenceBandwidth(t *testing.T) {
	oldCfg := ospfConfig{ReferenceBandwidth: 100000}
	newCfg := ospfConfig{ReferenceBandwidth: 10000}
	auto := interfaceConfig{Name: "eth0"}
	explicit := interfaceConfig{Name: "eth1", Cost: 10, HasCost: true}

	if !interfaceGlobalParamsChanged(oldCfg, newCfg, auto) {
		t.Error("an auto-cost interface was not restarted by a reference-bandwidth change")
	}
	if interfaceGlobalParamsChanged(oldCfg, newCfg, explicit) {
		t.Error("an explicitly-costed interface was restarted by a reference-bandwidth change")
	}
	if interfaceGlobalParamsChanged(oldCfg, oldCfg, auto) {
		t.Error("an unchanged reference bandwidth restarted an interface")
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
