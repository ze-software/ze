// Design: docs/architecture/ospf/ospf-ext-11-ldp-igp-sync.md -- LDP-IGP synchronization at origination.
// Related: ldp_sync.go -- applyLDPSyncOverride, the producer these tests drive.
// Related: ldp_sync_test.go -- the state-machine tests over the same manager.
//
// These tests read the metric the engine ORIGINATES, not the helper that names it. Each
// builds a real engine from config, drives its LDP-sync machine, runs the production
// applyLDPSyncOverride over the interface snapshot, originates the Router-LSA into a
// fresh LSDB, and decodes the installed LSA. The TE tests also originate the RFC 3630
// TE Link LSA through the engine and decode its TE metric sub-TLV.
//
// VALIDATES: RFC 5443 sections 2 and 4 on the originated LSAs: LSInfinity 0xFFFF on the
// point-to-point link while LDP is not fully operational, the configured cost after, and
// an unchanged TE metric throughout.
// PREVENTS: a cost-out that only a show helper reports, a max cost other than 0xFFFF, and
// a cost-out that reaches the TE link cost.
package ospf

import (
	"testing"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// rfc5443LocalAddr is the local interface address of the point-to-point link under test.
var rfc5443LocalAddr = [4]byte{10, 0, 1, 1}

// rfc5443Engine builds an engine whose only interface, eth1, is point-to-point with
// ldp-sync enabled and TE enabled. teMetric, when not empty, is the explicit te-metric
// leaf. The LDP-sync machine runs on fake timers so the test controls the hold-down.
func rfc5443Engine(t *testing.T, cost, teMetric string) (*engine, *fakeTimers) {
	t.Helper()
	te := `"traffic-engineering":{"enable":true}`
	if teMetric != "" {
		te = `"traffic-engineering":{"enable":true,"te-metric":"` + teMetric + `"}`
	}
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"1.1.1.1","router-address":"9.9.9.9","opaque":true,`+
		`"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth1":{"name":"eth1","area":"0",`+
		`"network-type":"point-to-point","cost":"`+cost+`","ldp-sync":{"enable":true,"holddown":"30"},`+te+`}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	eng := newEngine(nil)
	ft := &fakeTimers{}
	eng.ldpSync.afterFunc = ft.afterFunc
	eng.setConfig(cfg)
	rfc5443Reconcile(t, eng, &cfg)
	return eng, ft
}

// rfc5443Reconcile reconciles the engine's LDP-sync machines to cfg the
// way updateLDPSyncMachines does for the running set, without opening sockets.
func rfc5443Reconcile(t *testing.T, e *engine, cfg *ospfConfig) {
	t.Helper()
	desired := map[string]ldpSyncConfig{}
	for _, ic := range cfg.Interfaces {
		if !ic.LDPSyncEnabled {
			continue
		}
		desired[ic.Name] = ldpSyncConfig{
			HoldDown:    time.Duration(ic.LDPSyncHoldDown) * time.Second,
			Cost:        interfaceCost(ic, cfg.ReferenceBandwidth),
			NetworkType: string(ic.NetworkType),
		}
	}
	e.ldpSync.reconcileTo(desired)
}

// rfc5443Snapshot is eth1's origination-time snapshot after the production LDP-sync
// override: one Full point-to-point neighbor, the configured cost, and a /30 subnet.
func rfc5443Snapshot(t *testing.T, eng *engine) ospflsdb.InterfaceInfo {
	t.Helper()
	info := p2pTopo("eth1", rfc5443LocalAddr, types.RouterID{2, 2, 2, 2}, "10.0.1.2")
	info.NetworkMask = [4]byte{255, 255, 255, 252}
	info.Options = types.OptionE
	ic := eng.cfg.Interfaces[0]
	info.Cost = interfaceCost(ic, eng.cfg.ReferenceBandwidth)
	eng.applyLDPSyncOverride(&info, ic)
	return info
}

// rfc5443RouterMetrics originates the Router-LSA for info into a fresh LSDB and
// returns the metric of the point-to-point link and of the subnet stub link, decoded from
// the installed LSA.
func rfc5443RouterMetrics(t *testing.T, info ospflsdb.InterfaceInfo) (p2p, stub uint16) {
	t.Helper()
	in := ospflsdb.OriginInput{AreaID: types.BackboneArea, RouterID: types.RouterID{1, 1, 1, 1}, Options: types.OptionE, Interfaces: []ospflsdb.InterfaceInfo{info}}
	db := ospflsdb.New(func() time.Time { return time.Unix(0, 0) })
	h, ok := db.OriginateRouter(in)
	if !ok {
		t.Fatal("OriginateRouter returned false")
	}
	lsa, ok := db.LookupLSA(types.BackboneArea, h.Key())
	if !ok {
		t.Fatal("originated Router-LSA not installed")
	}
	body, err := lsa.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	p2pCount, stubCount := 0, 0
	for _, l := range body.Links {
		switch l.Type {
		case packet.RouterLinkTypeP2P:
			p2p = uint16(l.Metric)
			p2pCount++
		case packet.RouterLinkTypeStub:
			stub = uint16(l.Metric)
			stubCount++
		}
	}
	if p2pCount != 1 {
		t.Fatalf("point-to-point links = %d, want 1 (the cost-out raises the link, never withholds it)", p2pCount)
	}
	if stubCount != 1 {
		t.Fatalf("stub links = %d, want the one subnet stub", stubCount)
	}
	return p2p, stub
}

// rfc5443TEMetric originates the TE Link LSA for info through the engine and returns the
// TE metric it carries for eth1's local address.
func rfc5443TEMetric(t *testing.T, eng *engine, info ospflsdb.InterfaceInfo) int {
	t.Helper()
	eng.teOrig.setTopology(func() []ospflsdb.InterfaceInfo { return []ospflsdb.InterfaceInfo{info} })
	metric, found := teMetricForLocalAddress(t, eng.teOriginateType1(eng.cfg.RouterID), rfc5443LocalAddr)
	if !found {
		t.Fatal("no TE Link LSA carrying a TE metric was originated for eth1")
	}
	return metric
}

// rfc5443Synchronize drives eth1 to Synchronized: LDP session up, then hold-down expiry.
func rfc5443Synchronize(t *testing.T, eng *engine, ft *fakeTimers) {
	t.Helper()
	eng.ldpSync.onSessionUp("eth1")
	ft.fireLast(t)
	if state, _ := eng.ldpSync.stateFor("eth1"); state != ldpSyncSynchronized {
		t.Fatalf("eth1 state = %s, want synchronized after hold-down", ldpSyncStateName(state))
	}
}

// RFC requirement: RFC5443-2-1 positive -- while LDP is not fully operational (the link just
// came up, and again after LDP session-up but before the hold-down expires) the originated
// Router-LSA advertises the point-to-point link at LSInfinity instead of the configured 100.
// RFC requirement: RFC5443-2-2 positive -- the maximum cost the originated Router-LSA carries
// for the costed-out link is exactly the 16-bit value 0xFFFF, while the subnet stub keeps 100.
func TestRFC5443NotOperationalLinkAdvertisedAtMaxCost(t *testing.T) {
	// Goal: the advertised metric, not a helper value, is raised while LDP is not fully
	// operational. Method: originate before session-up and again during hold-down.
	eng, _ := rfc5443Engine(t, "100", "")

	p2p, stub := rfc5443RouterMetrics(t, rfc5443Snapshot(t, eng))
	if p2p != 0xFFFF {
		t.Fatalf("not-synchronized p2p metric = %#x, want 0xFFFF", p2p)
	}
	if stub != 100 {
		t.Fatalf("stub metric = %d, want the configured 100 (only the link is costed out)", stub)
	}

	eng.ldpSync.onSessionUp("eth1")
	p2p, _ = rfc5443RouterMetrics(t, rfc5443Snapshot(t, eng))
	if p2p != 0xFFFF {
		t.Fatalf("hold-down p2p metric = %#x, want 0xFFFF until LDP is fully operational", p2p)
	}
}

// RFC requirement: RFC5443-2-1 negative -- once LDP is fully operational (session up and the
// hold-down expired) the originated Router-LSA no longer advertises the maximum cost: the
// point-to-point link carries the configured 100.
func TestRFC5443OperationalLinkAdvertisedAtConfiguredCost(t *testing.T) {
	// Goal: the cost-out is confined to the not-operational period. Method: synchronize,
	// then originate and decode.
	eng, ft := rfc5443Engine(t, "100", "")
	rfc5443Synchronize(t, eng, ft)

	p2p, stub := rfc5443RouterMetrics(t, rfc5443Snapshot(t, eng))
	if p2p != 100 {
		t.Fatalf("synchronized p2p metric = %d, want the configured 100", p2p)
	}
	if stub != 100 {
		t.Fatalf("stub metric = %d, want the configured 100", stub)
	}
}

// RFC requirement: RFC5443-2-2 negative -- a configured cost one below the maximum (0xFFFE)
// is not the cost-out value: while not synchronized the link is advertised at 0xFFFF, not at
// 0xFFFE, and once synchronized it returns to 0xFFFE, so the two values are told apart.
func TestRFC5443MaxCostIsNotTheConfiguredNearMaxCost(t *testing.T) {
	// Goal: the substituted cost is LSInfinity itself, never the configured cost. Method: a
	// configured 0xFFFE link, originated before and after synchronization.
	eng, ft := rfc5443Engine(t, "65534", "")

	p2p, _ := rfc5443RouterMetrics(t, rfc5443Snapshot(t, eng))
	if p2p != 0xFFFF {
		t.Fatalf("not-synchronized p2p metric = %#x, want 0xFFFF, not the configured 0xFFFE", p2p)
	}

	rfc5443Synchronize(t, eng, ft)
	p2p, _ = rfc5443RouterMetrics(t, rfc5443Snapshot(t, eng))
	if p2p != 0xFFFE {
		t.Fatalf("synchronized p2p metric = %#x, want the configured 0xFFFE", p2p)
	}
}

// RFC requirement: RFC5443-4-1 positive -- the mechanism is applied to the IP link cost only:
// while not synchronized the Router-LSA link is raised to 0xFFFF, and the TE Link LSA still
// carries the TE metric defaulted from the configured IP cost, 100.
func TestRFC5443CostOutRaisesIPCostNotDefaultTEMetric(t *testing.T) {
	// Goal: the IP cost moves and the TE metric does not. Method: originate both LSAs for
	// the same not-synchronized snapshot.
	eng, _ := rfc5443Engine(t, "100", "")
	info := rfc5443Snapshot(t, eng)

	p2p, _ := rfc5443RouterMetrics(t, info)
	if p2p != 0xFFFF {
		t.Fatalf("IP link metric = %#x, want 0xFFFF while not synchronized", p2p)
	}
	if te := rfc5443TEMetric(t, eng, info); te != 100 {
		t.Fatalf("TE metric = %d, want 100: the cost-out MUST NOT reach the TE link cost", te)
	}
}

// RFC requirement: RFC5443-4-1 negative -- an explicitly configured TE metric is not raised
// by the cost-out either: while the IP link is advertised at 0xFFFF the TE Link LSA carries
// the configured te-metric 50, so no TE tunnel is rerouted.
func TestRFC5443CostOutLeavesExplicitTEMetric(t *testing.T) {
	// Goal: the cost-out never writes a TE metric. Method: an explicit te-metric distinct
	// from the IP cost, originated while not synchronized.
	eng, _ := rfc5443Engine(t, "100", "50")
	info := rfc5443Snapshot(t, eng)

	p2p, _ := rfc5443RouterMetrics(t, info)
	if p2p != 0xFFFF {
		t.Fatalf("IP link metric = %#x, want 0xFFFF while not synchronized", p2p)
	}
	if te := rfc5443TEMetric(t, eng, info); te != 50 {
		t.Fatalf("TE metric = %d, want the configured te-metric 50", te)
	}
}
