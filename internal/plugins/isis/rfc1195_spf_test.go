// Design: docs/architecture/isis/isis-9-spf-rib.md -- wire LSDB to IPv4 route selection.
// RFC 1195 Sections 3.4 and 3.10.2; RFC 2966 Section 3.2.
// Goal: keep metric type and level preference intact from received LSP bytes to
// the installed Loc-RIB path, including withdrawal of a preferred advertisement.
package isis

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// metricAdvertisement describes one received route, not a precomputed SPF node.
type metricAdvertisement struct {
	level          lsdb.Level
	id             byte
	cost           uint32
	metric         uint8
	external       bool
	externalMetric bool
	down           bool
}

type metricResolver struct{}

func (metricResolver) ResolveNextHop(_ spf.Level, id types.SystemID) (spf.NextHop, bool) {
	return spf.NextHop{Addr: netip.AddrFrom4([4]byte{192, 0, 2, id[5]}), Interface: "eth0"}, true
}

var metricPrefix = netip.MustParsePrefix("198.51.100.0/24")

func metricSystem(id byte) types.SystemID { return types.SystemID{0, 0, 0, 0, 0, id} }

// metricEngine stores codec-round-tripped LSPs through the receive LSDB path.
func metricEngine(t *testing.T, advertisements []metricAdvertisement) (*engine, *locrib.RIB) {
	t.Helper()
	e := newEngine(transport.New(&fakeBackend{}))
	e.spf.Stop()
	loc := locrib.NewRIB()
	e.spf = spf.NewComputer(spf.Config{
		Source: (*lsdbSPFSource)(e), Resolver: metricResolver{}, Root: metricSystem(1),
		Levels: []spf.Level{spf.Level1, spf.Level2}, Installer: spf.NewInstaller(loc),
	})
	t.Cleanup(e.shutdown)
	for _, level := range []lsdb.Level{lsdb.Level1, lsdb.Level2} {
		var neighbors []lsdb.AdjacencyInfo
		for _, a := range advertisements {
			if a.level != level {
				continue
			}
			metric, err := types.NewMetric(a.cost)
			if err != nil {
				t.Fatal(err)
			}
			neighbors = append(neighbors, lsdb.AdjacencyInfo{Neighbor: types.NewSourceID(metricSystem(a.id), 0), Metric: metric})
		}
		e.originator.Originate(level, lsdb.NodeInfo{SystemID: metricSystem(1), AdvertiseIPv4: true}, lsdb.LevelState{Neighbors: neighbors})
	}
	for _, a := range advertisements {
		metricReceive(t, e, a, 1, true)
	}
	return e, loc
}

func metricReceive(t *testing.T, e *engine, a metricAdvertisement, sequence uint32, reachability bool) {
	t.Helper()
	metric, err := types.NewMetric(a.cost)
	if err != nil {
		t.Fatal(err)
	}
	linkBytes := make([]byte, types.SourceIDLen+types.MetricLen+1)
	off := types.NewSourceID(metricSystem(1), 0).WriteTo(linkBytes, 0)
	metric.WriteTo(linkBytes, off)
	tlvs := []packet.TLV{{Type: packet.TLVExtendedISReach, Value: linkBytes}}
	if reachability {
		prefix := packet.NarrowIPReachTLV{External: a.external, Entries: []packet.NarrowIPReachEntry{{
			DefaultMetricValue: a.metric, ExternalMetric: a.externalMetric, UpDown: a.down,
			DelayMetric: 0x80, ExpenseMetric: 0x80, ErrorMetric: 0x80, Prefix: metricPrefix,
		}}}
		raw := make([]byte, prefix.EncodedLen())
		prefix.WriteTo(raw, 0)
		tlvs = append(tlvs, packet.TLV{Type: raw[0], Value: raw[2:]})
	}
	pt := packet.PDUTypeL1LSP
	if a.level == lsdb.Level2 {
		pt = packet.PDUTypeL2LSP
	}
	lsp := packet.LSP{PDUType: pt, RemainingLifetime: 1200,
		LSPID:          types.NewLSPID(types.NewSourceID(metricSystem(a.id), 0), 0),
		SequenceNumber: types.SequenceNumber(sequence), TLVs: tlvs}
	raw := make([]byte, lsp.EncodedLen())
	lsp.WriteTo(raw, 0)
	decoded, err := packet.DecodePDU(raw)
	if err != nil {
		t.Fatal(err)
	}
	defer packet.ReleaseTLVs(decoded.LSP.TLVs)
	if result := e.lsdb.Receive(a.level, decoded.LSP, raw, false); !result.Stored {
		t.Fatalf("LSP not received: %+v", result)
	}
}

// metricMutate replaces a peer's stored advertisement without changing its
// topology. The copy keeps the LSDB's borrowed raw spans immutable.
func metricMutate(t *testing.T, e *engine, a metricAdvertisement, mutate func([]packet.TLV)) {
	t.Helper()
	entry := e.lsdb.Lookup(a.level, types.NewLSPID(types.NewSourceID(metricSystem(a.id), 0), 0))
	lsp, err := entry.Decode()
	if err != nil {
		t.Fatal(err)
	}
	defer packet.ReleaseTLVs(lsp.TLVs)
	for i := range lsp.TLVs {
		lsp.TLVs[i].Value = append([]byte(nil), lsp.TLVs[i].Value...)
	}
	mutate(lsp.TLVs)
	lsp.SequenceNumber++
	raw := make([]byte, lsp.EncodedLen())
	lsp.WriteTo(raw, 0)
	if result := e.lsdb.Receive(a.level, &lsp, raw, false); !result.Stored {
		t.Fatalf("replacement LSP not stored: %+v", result)
	}
}

func metricWinner(t *testing.T, e *engine, loc *locrib.RIB, id byte, metric uint32) {
	t.Helper()
	e.spf.Run()
	group, ok := loc.Lookup(family.IPv4Unicast, metricPrefix)
	if !ok || len(group.Paths) != 1 {
		t.Fatalf("installed path group = %+v, present=%v", group, ok)
	}
	want := netip.AddrFrom4([4]byte{192, 0, 2, id})
	if group.Paths[0].NextHop != want || group.Paths[0].Metric != metric {
		t.Fatalf("installed path = %+v, want via %s metric %d", group.Paths[0], want, metric)
	}
}

// RFC requirement: RFC1195-3.4-1 positive -- a smaller external metric wins even when its exit is farther away; equal external metrics choose the nearer exit.
func TestRFC1195ExternalMetricBeforeExitCost(t *testing.T) {
	a := metricAdvertisement{level: lsdb.Level2, id: 2, cost: 100, metric: 1, external: true, externalMetric: true}
	b := metricAdvertisement{level: lsdb.Level2, id: 3, cost: 1, metric: 2, external: true, externalMetric: true}
	e, loc := metricEngine(t, []metricAdvertisement{a, b})
	metricWinner(t, e, loc, 2, 1)
	a.metric = 2
	metricReceive(t, e, a, 2, true)
	metricWinner(t, e, loc, 3, 2)
}

// RFC requirement: RFC1195-3.4-1 negative -- a shorter internal path cannot defeat a smaller external metric, but withdrawal of the smaller external route permits the remaining exit.
func TestRFC1195ExternalMetricWithdrawalRecomputesWinner(t *testing.T) {
	a := metricAdvertisement{level: lsdb.Level2, id: 2, cost: 1, metric: 8, external: true, externalMetric: true}
	b := metricAdvertisement{level: lsdb.Level2, id: 3, cost: 200, metric: 7, external: true, externalMetric: true}
	e, loc := metricEngine(t, []metricAdvertisement{a, b})
	metricWinner(t, e, loc, 3, 7)
	metricReceive(t, e, b, 2, false)
	metricWinner(t, e, loc, 2, 8)
}

// RFC requirement: RFC1195-3.10-2 positive -- a costly L2 internal-metric prefix beats a cheap external-metric prefix in the installed Loc-RIB.
// RFC requirement: RFC1195-7-4 positive -- external metric type is retained separately from internal metric type through LSDB ingestion and route selection.
func TestRFC1195InternalMetricOutranksExternal(t *testing.T) {
	a := metricAdvertisement{level: lsdb.Level2, id: 2, cost: 100, metric: 63}
	b := metricAdvertisement{level: lsdb.Level2, id: 3, cost: 1, metric: 1, external: true, externalMetric: true}
	e, loc := metricEngine(t, []metricAdvertisement{a, b})
	metricWinner(t, e, loc, 2, 163)
}

// RFC requirement: RFC1195-3.10-2 negative -- an external TLV using internal metrics is not penalized as an external metric route; its lower internal cost wins.
// RFC requirement: RFC1195-7-4 negative -- external reachability provenance alone does not turn its internal metric into an external metric during selection.
func TestRFC1195ExternalReachabilityWithInternalMetric(t *testing.T) {
	a := metricAdvertisement{level: lsdb.Level2, id: 2, cost: 100, metric: 63}
	b := metricAdvertisement{level: lsdb.Level2, id: 3, cost: 1, metric: 1, external: true}
	e, loc := metricEngine(t, []metricAdvertisement{a, b})
	metricWinner(t, e, loc, 3, 2)
}

// RFC requirement: RFC2966-3.2-1 positive -- every adjacent pair in the six preference classes selects the more preferred route despite its larger metric.
func TestRFC2966SixPreferenceClasses(t *testing.T) {
	classes := []metricAdvertisement{
		{level: lsdb.Level1}, {level: lsdb.Level2}, {level: lsdb.Level1, down: true},
		{level: lsdb.Level1, external: true, externalMetric: true},
		{level: lsdb.Level2, external: true, externalMetric: true},
		{level: lsdb.Level1, down: true, external: true, externalMetric: true},
	}
	for i := 0; i+1 < len(classes); i++ {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			a, b := classes[i], classes[i+1]
			a.id, a.cost, a.metric = 2, 100, 60
			b.id, b.cost, b.metric = 3, 1, 1
			e, loc := metricEngine(t, []metricAdvertisement{b, a})
			want := uint32(160)
			if a.externalMetric {
				want = 60
			}
			metricWinner(t, e, loc, 2, want)
		})
	}
}

// RFC requirement: RFC2966-3.2-1 negative -- an L1 external-metric route does not beat a leaked-down internal-metric route; removing the internal route restores the external route.
func TestRFC2966ExternalL1DoesNotOverrideDownInternal(t *testing.T) {
	a := metricAdvertisement{level: lsdb.Level1, id: 2, cost: 100, metric: 60, down: true}
	b := metricAdvertisement{level: lsdb.Level1, id: 3, cost: 1, metric: 1, external: true, externalMetric: true}
	e, loc := metricEngine(t, []metricAdvertisement{a, b})
	metricWinner(t, e, loc, 2, 160)
	metricReceive(t, e, a, 2, false)
	metricWinner(t, e, loc, 3, 1)
}

// RFC requirement: RFC2966-x-1 positive -- an invalid internal-reachability entry carrying an external metric is ignored, so a valid alternate route is selected.
// RFC requirement: RFC2966-x-1 negative -- clearing the invalid metric-type bit restores the cheaper internal route.
func TestRFC2966InvalidInternalExternalMetricExcluded(t *testing.T) {
	a := metricAdvertisement{level: lsdb.Level2, id: 2, cost: 100, metric: 60}
	b := metricAdvertisement{level: lsdb.Level2, id: 3, cost: 1, metric: 1}
	e, loc := metricEngine(t, []metricAdvertisement{a, b})
	metricMutate(t, e, b, func(tlvs []packet.TLV) {
		for i := range tlvs {
			if tlvs[i].Type == packet.TLVIPInternalReachability {
				tlvs[i].Value[0] |= 0x40
			}
		}
	})
	metricWinner(t, e, loc, 2, 160)
	metricMutate(t, e, b, func(tlvs []packet.TLV) {
		for i := range tlvs {
			if tlvs[i].Type == packet.TLVIPInternalReachability {
				tlvs[i].Value[0] &^= 0x40
			}
		}
	})
	metricWinner(t, e, loc, 3, 2)
}

// RFC requirement: RFC1195-3.10-3 negative -- removing the default metric octet prevents the malformed reachability entry from becoming a route; a valid competing entry remains usable.
// RFC requirement: RFC1195-5.2-4 negative -- a reachability entry without the required metric cannot change the installed next hop.
func TestRFC1195MissingDefaultMetricCannotRoute(t *testing.T) {
	a := metricAdvertisement{level: lsdb.Level2, id: 2, cost: 100, metric: 60}
	b := metricAdvertisement{level: lsdb.Level2, id: 3, cost: 1, metric: 1}
	e, loc := metricEngine(t, []metricAdvertisement{a, b})
	metricMutate(t, e, b, func(tlvs []packet.TLV) {
		for i := range tlvs {
			if tlvs[i].Type == packet.TLVIPInternalReachability {
				tlvs[i].Value = tlvs[i].Value[1:]
			}
		}
	})
	metricWinner(t, e, loc, 2, 160)
}

// metricLeak originates the L1 route in L2 through the production SPF leak and
// PrefixInfo conversion, and decodes the resulting wire advertisement.
func metricLeak(t *testing.T, advertisements ...metricAdvertisement) packet.NarrowIPReachEntry {
	t.Helper()
	e, _ := metricEngine(t, advertisements)
	graphs := make(map[spf.Level]*spf.Graph)
	var results []*spf.Result
	for _, level := range []spf.Level{spf.Level1, spf.Level2} {
		graphs[level] = spf.BuildGraph((*lsdbSPFSource)(e), level)
		results = append(results, spf.Compute(graphs[level], metricSystem(1), level))
	}
	leaked := spf.LeakPrefixes(results, graphs)
	prefixes := leakedToPrefixInfos(leaked.IntoL2)
	e.originator.Originate(lsdb.Level2, lsdb.NodeInfo{SystemID: metricSystem(1), AdvertiseIPv4: true},
		lsdb.LevelState{Prefixes: prefixes})
	var found []packet.NarrowIPReachEntry
	for _, raw := range e.lsdb.RawSnapshot(lsdb.Level2) {
		pdu, err := packet.DecodePDU(raw)
		if err != nil {
			t.Fatal(err)
		}
		if pdu.LSP.LSPID.SystemID() != metricSystem(1) {
			packet.ReleaseTLVs(pdu.LSP.TLVs)
			continue
		}
		for _, tlv := range pdu.LSP.TLVs {
			if tlv.Type != packet.TLVIPInternalReachability && tlv.Type != packet.TLVIPExternalReachability {
				continue
			}
			decoded, err := packet.DecodeNarrowIPReachTLV(tlv.Value, tlv.Type == packet.TLVIPExternalReachability)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range decoded.Entries {
				if entry.Prefix == metricPrefix {
					if decoded.External != advertisements[0].external {
						t.Fatalf("leak changed external reachability type: %+v", decoded)
					}
					found = append(found, entry)
				}
			}
		}
		packet.ReleaseTLVs(pdu.LSP.TLVs)
	}
	if len(found) != 1 {
		t.Fatalf("originated %d L2 entries for %s, want one", len(found), metricPrefix)
	}
	return found[0]
}

// RFC requirement: RFC1195-3.2-1 positive -- a narrow L1 path whose accumulated metric exceeds 63 is reoriginated in L2 with metric 63.
// RFC requirement: RFC1195-5.3.4-1 positive -- reoriginating internal reachability keeps its I/E bit clear.
func TestRFC1195NarrowLeakedMetricClamps(t *testing.T) {
	got := metricLeak(t, metricAdvertisement{level: lsdb.Level1, id: 2, cost: 50, metric: 40})
	if got.DefaultMetricValue != 63 || got.ExternalMetric {
		t.Fatalf("leaked metric = %+v, want internal metric 63", got)
	}
}

// RFC requirement: RFC1195-3.2-1 negative -- a representable narrow metric sum is preserved rather than saturated, and external metrics exclude the internal exit cost.
func TestRFC1195NarrowLeakedMetricBelowCeiling(t *testing.T) {
	got := metricLeak(t, metricAdvertisement{level: lsdb.Level1, id: 2, cost: 10, metric: 5})
	if got.DefaultMetricValue != 15 {
		t.Fatalf("leaked metric = %d, want 15", got.DefaultMetricValue)
	}
	got = metricLeak(t, metricAdvertisement{level: lsdb.Level1, id: 2, cost: 100, metric: 5, external: true, externalMetric: true})
	if got.DefaultMetricValue != 5 || !got.ExternalMetric {
		t.Fatalf("external leak = %+v, want external metric 5", got)
	}
}

// RFC requirement: RFC1195-7-2 positive -- the aging timer's periodic refresh triggers a full LSDB/SPF pass that repairs a stale installed prefix metric.
// RFC requirement: RFC1195-7-2 negative -- recovery does not depend on receiving a topology-change trigger for the changed foreign LSP.
func TestRFC1195PeriodicSPFRecoversMissedUpdate(t *testing.T) {
	// Keep the adjacency at the foreign LSP's level: the P2P runtime chooses
	// only L1 when both levels are enabled, leaving the L2 graph disconnected.
	cfg, err := parseISISConfig(sec(`{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point","level":"l2","metric":"10"}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	e := newEngine(transport.New(&fakeBackend{}))
	t.Cleanup(e.shutdown)
	e.setConfig(cfg)
	e.spf.Stop()
	if err := e.openCircuit(cfg.EnabledCircuits()[0]); err != nil {
		t.Fatal(err)
	}
	circuit := e.circuitByName["eth0"]
	if tr := circuit.Receive(adjacency.SNPA{}, p2pHelloPDU(t, metricSystem(2), cfg.NETs[0].AreaID())); !tr.SessionUp {
		t.Fatalf("peer did not become adjacent: %+v", tr)
	}
	loc := locrib.NewRIB()
	e.spf = spf.NewComputer(spf.Config{
		Source: (*lsdbSPFSource)(e), Resolver: metricResolver{}, Root: cfg.SystemID,
		Levels: []spf.Level{spf.Level1, spf.Level2}, Installer: spf.NewInstaller(loc),
	})
	a := metricAdvertisement{level: lsdb.Level2, id: 2, cost: 10, metric: 1}
	metricReceive(t, e, a, 1, true)
	metricWinner(t, e, loc, 2, 11)
	a.metric = 5
	metricReceive(t, e, a, 2, true) // Intentionally no emitLSPChange / Trigger.
	group, ok := loc.Lookup(family.IPv4Unicast, metricPrefix)
	if !ok || group.Paths[0].Metric != 11 {
		t.Fatalf("route was not stale before periodic update: %+v", group)
	}
	changed := make(chan spf.RouteDelta, 1)
	e.spf.SetOnChange(func(delta spf.RouteDelta) {
		select {
		case changed <- delta:
		default:
		}
	})
	setLastOrigAtPast(e, lsdb.Level2)
	e.ageOnce()
	select {
	case <-changed:
	case <-time.After(3 * time.Second):
		t.Fatal("periodic refresh did not recover the missed foreign-LSP update")
	}
	group, ok = loc.Lookup(family.IPv4Unicast, metricPrefix)
	if !ok || len(group.Paths) != 1 || group.Paths[0].Metric != 15 {
		t.Fatalf("periodic SPF did not repair metric: %+v", group)
	}
}

// RFC requirement: RFC3786-5-1 positive -- a standard source with live fragment zero supplies its additional fragment's route.
// RFC requirement: RFC3786-5-1 negative -- retaining a live nonzero fragment cannot retain that route after fragment zero disappears.
func TestRFC1195StandardFragmentsRequireFragmentZero(t *testing.T) {
	a := metricAdvertisement{level: lsdb.Level2, id: 2, cost: 10, metric: 5}
	e, loc := metricEngine(t, []metricAdvertisement{a})
	id := types.NewLSPID(types.NewSourceID(metricSystem(2), 0), 0)
	zero := e.lsdb.Lookup(lsdb.Level2, id)
	lsp, err := zero.Decode()
	if err != nil {
		t.Fatal(err)
	}
	defer packet.ReleaseTLVs(lsp.TLVs)
	lsp.LSPID = types.NewLSPID(id.SourceID(), 1)
	raw := make([]byte, lsp.EncodedLen())
	lsp.WriteTo(raw, 0)
	e.lsdb.Receive(lsdb.Level2, &lsp, raw, false)
	// Retain only topology in fragment zero; the prefix lives in fragment one.
	metricReceive(t, e, a, 2, false)
	metricWinner(t, e, loc, 2, 15)
	e.lsdb.Delete(lsdb.Level2, id)
	e.spf.Run()
	if group, ok := loc.Lookup(family.IPv4Unicast, metricPrefix); ok && len(group.Paths) != 0 {
		t.Fatalf("orphan fragment retained a route: %+v", group)
	}
}

// RFC requirement: RFC1195-5.3.4-1 negative -- even an internal-prefix producer supplying ExternalMetric cannot set I/E in an originated TLV 128.
func TestRFC1195InternalOriginatorClearsExternalMetric(t *testing.T) {
	d := lsdb.New(nil)
	o := lsdb.NewOriginator(d, nil)
	o.Originate(lsdb.Level2, lsdb.NodeInfo{SystemID: metricSystem(1), AdvertiseIPv4: true},
		lsdb.LevelState{Prefixes: []lsdb.PrefixInfo{{Prefix: metricPrefix, Metric: types.NewPrefixMetric(10), Narrow: true, ExternalMetric: true}}})
	for _, raw := range d.RawSnapshot(lsdb.Level2) {
		pdu, err := packet.DecodePDU(raw)
		if err != nil {
			t.Fatal(err)
		}
		for _, tlv := range pdu.LSP.TLVs {
			if tlv.Type == packet.TLVIPInternalReachability {
				if tlv.Value[0]&0x40 != 0 {
					t.Fatal("internal prefix emitted an external metric")
				}
				packet.ReleaseTLVs(pdu.LSP.TLVs)
				return
			}
		}
		packet.ReleaseTLVs(pdu.LSP.TLVs)
	}
	t.Fatal("internal prefix was not originated")
}

// RFC requirement: RFC1195-3.2-2 positive -- duplicate specific prefixes from two L1 routers become one L2 entry with the minimum accumulated metric.
// RFC requirement: RFC1195-3.2-2 negative -- the more expensive L1 advertisement cannot add a duplicate or replace the cheaper L2 metric.
func TestRFC1195SpecificLeakCollapsesDuplicatePrefix(t *testing.T) {
	got := metricLeak(t,
		metricAdvertisement{level: lsdb.Level1, id: 2, cost: 10, metric: 10},
		metricAdvertisement{level: lsdb.Level1, id: 3, cost: 5, metric: 4})
	if got.DefaultMetricValue != 9 {
		t.Fatalf("duplicate prefix leak metric = %d, want minimum 9", got.DefaultMetricValue)
	}
}

// RFC requirement: RFC1195-5.3.4-2 positive -- a pseudonode originates IS-neighbor connectivity without any IP reachability or IDRPI TLV.
// RFC requirement: RFC1195-5.3.4-2 negative -- narrow internal and external prefixes on the originating router cannot leak into its pseudonode LSP.
func TestRFC1195PseudonodeCannotInheritRouterPrefixes(t *testing.T) {
	d := lsdb.New(nil)
	o := lsdb.NewOriginator(d, nil)
	o.Originate(lsdb.Level2, lsdb.NodeInfo{SystemID: metricSystem(1), AdvertiseIPv4: true},
		lsdb.LevelState{Prefixes: []lsdb.PrefixInfo{
			{Prefix: metricPrefix, Metric: types.NewPrefixMetric(10), Narrow: true},
			{Prefix: netip.MustParsePrefix("203.0.113.0/24"), Metric: types.NewPrefixMetric(5), Narrow: true, External: true, ExternalMetric: true},
		}})
	o.OriginatePseudonode(lsdb.Level2, lsdb.PseudonodeInfo{
		SystemID: metricSystem(1), PseudonodeID: 7, Members: []types.SystemID{metricSystem(1), metricSystem(2)},
	})
	routerTypes := make(map[uint8]bool)
	neighborSeen := false
	for _, raw := range d.RawSnapshot(lsdb.Level2) {
		pdu, err := packet.DecodePDU(raw)
		if err != nil {
			t.Fatal(err)
		}
		for _, tlv := range pdu.LSP.TLVs {
			if pdu.LSP.LSPID.PseudonodeID() == 0 {
				routerTypes[tlv.Type] = true
				continue
			}
			switch tlv.Type {
			case packet.TLVIPInternalReachability, packet.TLVIPExternalReachability, 131:
				t.Fatalf("pseudonode inherited forbidden TLV %d", tlv.Type)
			case packet.TLVExtendedISReach:
				reach, err := packet.DecodeExtendedISReachTLV(tlv.Value)
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range reach.Entries {
					if entry.Neighbor.SystemID() == metricSystem(2) {
						neighborSeen = true
					}
				}
			}
		}
		packet.ReleaseTLVs(pdu.LSP.TLVs)
	}
	if !neighborSeen || !routerTypes[packet.TLVIPInternalReachability] || !routerTypes[packet.TLVIPExternalReachability] {
		t.Fatalf("router prefixes or pseudonode connectivity missing: router=%v neighbor=%v", routerTypes, neighborSeen)
	}
}

// RFC requirement: RFC1195-4.5-1 positive -- an Up peer changing to OSI-only replaces its installed IPv4 forwarding route with a terminal unreachable route through the production LSDB/SPF/installer path.
// RFC requirement: RFC1195-4.5-1 negative -- restoring IPv4 support on that same adjacency replaces the rejection with its learned direct gateway.
func TestRFC1195CapabilityChangesInstalledRouteDisposition(t *testing.T) {
	cfg, err := parseISISConfig(sec(protocolP2PConfig))
	if err != nil {
		t.Fatal(err)
	}
	e := newEngine(transport.New(&fakeBackend{}))
	t.Cleanup(e.shutdown)
	e.setConfig(cfg)
	e.spf.Stop()
	if err := e.openCircuit(cfg.EnabledCircuits()[0]); err != nil {
		t.Fatal(err)
	}
	c := protocolLiveCircuit(t, e, "eth0")
	loc := locrib.NewRIB()
	e.spf = spf.NewComputer(spf.Config{
		Source: (*lsdbSPFSource)(e), Resolver: (*engineNextHopResolver)(e),
		Root: cfg.SystemID, Levels: []spf.Level{spf.Level1},
		Debounce: time.Hour, Installer: spf.NewInstaller(loc),
	})
	metricReceive(t, e, metricAdvertisement{level: lsdb.Level1, id: 2, cost: 10, metric: 5}, 1, true)
	for _, supported := range []bool{true, false, true} {
		var protocols []byte
		if supported {
			protocols = []byte{packet.NLPIDIPv4}
		}
		e.dispatch.dispatch(transport.RawFrame{IfIndex: c.IfIndex(), PDU: protocolPeerIIH(t, protocols)})
		e.spf.Run()
		group, ok := loc.Lookup(family.IPv4Unicast, metricPrefix)
		if !ok || len(group.Paths) != 1 {
			t.Fatalf("capable=%v lost explicit route: %+v", supported, group)
		}
		path := group.Paths[0]
		if !supported {
			if path.RouteType != routetype.Unreachable || path.NextHop.IsValid() {
				t.Fatalf("OSI-only peer remained a forwarding path: %+v", path)
			}
			continue
		}
		if path.RouteType != routetype.Unicast || path.NextHop != netip.MustParseAddr("198.51.100.2") ||
			path.Interface != "eth0" || !path.OnLink {
			t.Fatalf("capable peer did not install its direct adjacency: %+v", path)
		}
	}
}
