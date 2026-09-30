// Design: docs/architecture/isis/isis-9-spf-rib.md -- wire LSDB to IPv4 route selection.
// Related: rfc1195_spf_test.go -- the metricEngine fixture these tests reuse.
//
// Goal: prove the RFC 1195, RFC 2966 and RFC 3786 route-selection clauses that
// the older tests in rfc1195_spf_test.go left unproven, each in its own test.
// Method: feed codec-round-tripped LSPs through the receive LSDB, run the
// production SPF computer, and read the installed Loc-RIB path or the
// originated wire bytes.
package isis

import (
	"errors"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// dropDefaultMetric removes the default metric octet, the first octet of each
// 12-octet entry, from every narrow reachability TLV of the given type.
func dropDefaultMetric(tlvType uint8) func([]packet.TLV) {
	return func(tlvs []packet.TLV) {
		for i := range tlvs {
			if tlvs[i].Type == tlvType {
				tlvs[i].Value = tlvs[i].Value[1:]
			}
		}
	}
}

// requireNoDefaultMetricRefused asserts that the decoder refuses a narrow
// reachability entry whose default metric octet is absent, and accepts the
// same entry with the octet present.
func requireNoDefaultMetricRefused(t *testing.T, external bool) {
	t.Helper()
	entry := packet.NarrowIPReachTLV{External: external, Entries: []packet.NarrowIPReachEntry{{
		DefaultMetricValue: 9, DelayMetric: 0x80, ExpenseMetric: 0x80, ErrorMetric: 0x80, Prefix: metricPrefix,
	}}}
	raw := make([]byte, entry.EncodedLen())
	entry.WriteTo(raw, 0)
	whole, err := packet.DecodeNarrowIPReachTLV(raw[2:], external)
	if err != nil || len(whole.Entries) != 1 || whole.Entries[0].DefaultMetricValue != 9 {
		t.Fatalf("complete entry decoded as %+v, err %v; want one entry with default metric 9", whole, err)
	}
	if got, err := packet.DecodeNarrowIPReachTLV(raw[3:], external); !errors.Is(err, packet.ErrLength) {
		t.Fatalf("entry without its default metric decoded as %+v, err %v; want ErrLength", got, err)
	}
}

// RFC requirement: RFC1195-3.10-3 negative -- a TLV 128 entry whose default metric octet is removed is refused by the decoder with ErrLength, and the LSP carrying it installs no route: the competing entry with a default metric stays installed.
func TestRFC1195InternalEntryWithoutDefaultMetricRefused(t *testing.T) {
	requireNoDefaultMetricRefused(t, false)
	a := metricAdvertisement{level: lsdb.Level2, id: 2, cost: 100, metric: 60}
	b := metricAdvertisement{level: lsdb.Level2, id: 3, cost: 1, metric: 1}
	e, loc := metricEngine(t, []metricAdvertisement{a, b})
	metricWinner(t, e, loc, 3, 2)
	metricMutate(t, e, b, dropDefaultMetric(packet.TLVIPInternalReachability))
	metricWinner(t, e, loc, 2, 160)
}

// RFC requirement: RFC1195-5.2-4 negative -- a TLV 130 entry whose default metric octet is removed is refused by the decoder with ErrLength, and the L2 LSP carrying it installs no route: the competing entry with a default metric stays installed.
func TestRFC1195ExternalEntryWithoutDefaultMetricRefused(t *testing.T) {
	requireNoDefaultMetricRefused(t, true)
	a := metricAdvertisement{level: lsdb.Level2, id: 2, cost: 100, metric: 60}
	b := metricAdvertisement{level: lsdb.Level2, id: 3, cost: 1, metric: 1, external: true}
	e, loc := metricEngine(t, []metricAdvertisement{a, b})
	metricWinner(t, e, loc, 3, 2)
	metricMutate(t, e, b, dropDefaultMetric(packet.TLVIPExternalReachability))
	metricWinner(t, e, loc, 2, 160)
}

// RFC requirement: RFC1195-5.2-4 positive -- every TLV 130 entry the originator emits carries its prefix's configured default metric, for internal and external metric types alike.
func TestRFC1195OriginatedExternalEntriesCarryDefaultMetric(t *testing.T) {
	second := netip.MustParsePrefix("203.0.113.0/24")
	want := map[netip.Prefix]uint8{metricPrefix: 7, second: 33}
	d := lsdb.New(nil)
	o := lsdb.NewOriginator(d, nil)
	o.Originate(lsdb.Level2, lsdb.NodeInfo{SystemID: metricSystem(1), AdvertiseIPv4: true},
		lsdb.LevelState{Prefixes: []lsdb.PrefixInfo{
			{Prefix: metricPrefix, Metric: types.NewPrefixMetric(7), Narrow: true, External: true},
			{Prefix: second, Metric: types.NewPrefixMetric(33), Narrow: true, External: true, ExternalMetric: true},
		}})
	got := make(map[netip.Prefix]uint8)
	for _, raw := range d.RawSnapshot(lsdb.Level2) {
		pdu, err := packet.DecodePDU(raw)
		if err != nil {
			t.Fatal(err)
		}
		for _, tlv := range pdu.LSP.TLVs {
			if tlv.Type != packet.TLVIPExternalReachability {
				continue
			}
			decoded, err := packet.DecodeNarrowIPReachTLV(tlv.Value, true)
			if err != nil {
				t.Fatalf("originated TLV 130 does not decode: %v", err)
			}
			for _, entry := range decoded.Entries {
				got[entry.Prefix] = entry.DefaultMetricValue
			}
		}
		packet.ReleaseTLVs(pdu.LSP.TLVs)
	}
	if len(got) != len(want) {
		t.Fatalf("originated TLV 130 entries = %v, want %v", got, want)
	}
	for prefix, metric := range want {
		if got[prefix] != metric {
			t.Fatalf("TLV 130 entry %s default metric = %d, want %d", prefix, got[prefix], metric)
		}
	}
}

// RFC requirement: RFC1195-3.2-2 negative -- three L1 routers announce one prefix and the cheapest is received last: the L2 LSP carries exactly one entry, at the minimum metric 9, and neither more expensive announcement (20 or 12) appears.
func TestRFC1195SpecificLeakKeepsOnlyMinimum(t *testing.T) {
	got := metricLeak(t,
		metricAdvertisement{level: lsdb.Level1, id: 2, cost: 10, metric: 10},
		metricAdvertisement{level: lsdb.Level1, id: 4, cost: 3, metric: 9},
		metricAdvertisement{level: lsdb.Level1, id: 3, cost: 5, metric: 4})
	if got.DefaultMetricValue != 9 {
		t.Fatalf("duplicate prefix leak metric = %d, want minimum 9", got.DefaultMetricValue)
	}
}

// RFC requirement: RFC1195-7-4 negative -- keeping the external set separate retains it: once the costlier internal route is withdrawn, the external-metric route is installed at its own external metric 1, not at an internal cost.
func TestRFC1195ExternalSetRetainedBesideInternal(t *testing.T) {
	a := metricAdvertisement{level: lsdb.Level2, id: 2, cost: 100, metric: 63}
	b := metricAdvertisement{level: lsdb.Level2, id: 3, cost: 1, metric: 1, external: true, externalMetric: true}
	e, loc := metricEngine(t, []metricAdvertisement{a, b})
	metricWinner(t, e, loc, 2, 163)
	metricReceive(t, e, a, 2, false)
	metricWinner(t, e, loc, 3, 1)
}

// periodicEngine builds a started engine with an L2 point-to-point adjacency to
// system 2 and a Loc-RIB installer, with the SPF debounce loop stopped so only
// an explicit Run or the periodic refresh computes routes.
func periodicEngine(t *testing.T) (*engine, *locrib.RIB) {
	t.Helper()
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
	return e, loc
}

// RFC requirement: RFC1195-7-2 negative -- a prefix withdrawn from a foreign LSP with no topology-change trigger stays installed until the periodic full update, which removes it.
func TestRFC1195PeriodicSPFRemovesMissedWithdrawal(t *testing.T) {
	e, loc := periodicEngine(t)
	a := metricAdvertisement{level: lsdb.Level2, id: 2, cost: 10, metric: 1}
	metricReceive(t, e, a, 1, true)
	metricWinner(t, e, loc, 2, 11)
	metricReceive(t, e, a, 2, false) // Intentionally no emitLSPChange / Trigger.
	if group, ok := loc.Lookup(family.IPv4Unicast, metricPrefix); !ok || len(group.Paths) != 1 {
		t.Fatalf("route was not stale before the periodic update: %+v", group)
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
		t.Fatal("periodic refresh did not recover the missed withdrawal")
	}
	if group, ok := loc.Lookup(family.IPv4Unicast, metricPrefix); ok && len(group.Paths) != 0 {
		t.Fatalf("periodic SPF kept the withdrawn prefix: %+v", group)
	}
}

// rfc2966Classes lists the six RFC 2966 section 3.2 preference classes, most
// preferred first, each with every member the metricAdvertisement fixture can
// express. An L1->L2 inter-area route is an L2 route to its receiver, so it
// shares the L2 member of classes 2 and 5.
var rfc2966Classes = [][]metricAdvertisement{
	{{level: lsdb.Level1}, {level: lsdb.Level1, external: true}},
	{{level: lsdb.Level2}, {level: lsdb.Level2, external: true}},
	{{level: lsdb.Level1, down: true}, {level: lsdb.Level1, down: true, external: true}},
	{{level: lsdb.Level1, external: true, externalMetric: true}},
	{{level: lsdb.Level2, external: true, externalMetric: true}},
	{{level: lsdb.Level1, down: true, external: true, externalMetric: true}},
}

// rfc2966Installed is the metric the installer reports for a route with the
// given advertised metric and path cost: the advertised metric alone for an
// external metric, the sum for an internal one.
func rfc2966Installed(a metricAdvertisement) uint32 {
	if a.externalMetric {
		return uint32(a.metric)
	}
	return a.cost + uint32(a.metric)
}

// RFC requirement: RFC2966-3.2-1 positive -- every member of each preference class, including the equal-preference members TestRFC2966SixPreferenceClasses leaves out, beats every member of the next class despite a larger metric.
func TestRFC2966EveryClassMemberOutranksNextClass(t *testing.T) {
	for i := 0; i+1 < len(rfc2966Classes); i++ {
		for _, preferred := range rfc2966Classes[i] {
			for _, next := range rfc2966Classes[i+1] {
				t.Run("", func(t *testing.T) {
					a, b := preferred, next
					a.id, a.cost, a.metric = 2, 100, 60
					b.id, b.cost, b.metric = 3, 1, 1
					e, loc := metricEngine(t, []metricAdvertisement{b, a})
					metricWinner(t, e, loc, 2, rfc2966Installed(a))
				})
			}
		}
	}
}

// RFC requirement: RFC2966-3.2-1 negative -- two members the list places in one class are not ranked against each other: whichever has the lower metric is selected, in both orders.
func TestRFC2966EqualClassMembersCompareByMetric(t *testing.T) {
	for _, class := range rfc2966Classes {
		if len(class) < 2 {
			continue
		}
		for _, swap := range []bool{false, true} {
			t.Run("", func(t *testing.T) {
				a, b := class[0], class[1]
				if swap {
					a, b = b, a
				}
				a.id, a.cost, a.metric = 2, 100, 60
				b.id, b.cost, b.metric = 3, 1, 1
				e, loc := metricEngine(t, []metricAdvertisement{a, b})
				metricWinner(t, e, loc, 3, rfc2966Installed(b))
			})
		}
	}
}

// RFC requirement: RFC3786-5-1 negative -- fragment zero present with RemainingLifetime zero: the source's live fragment one, which still carries the link back and the prefix, is not considered in SPF.
func TestRFC3786ExpiredFragmentZeroExcludesSet(t *testing.T) {
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
	metricWinner(t, e, loc, 2, 15)
	purge := packet.LSP{PDUType: packet.PDUTypeL2LSP, RemainingLifetime: 0, LSPID: id,
		SequenceNumber: zero.Sequence() + 1}
	purgeRaw := make([]byte, purge.EncodedLen())
	purge.WriteTo(purgeRaw, 0)
	if result := e.lsdb.Receive(lsdb.Level2, &purge, purgeRaw, false); !result.Stored {
		t.Fatalf("zero-lifetime fragment zero not stored: %+v", result)
	}
	if held := e.lsdb.Lookup(lsdb.Level2, id); held == nil {
		t.Fatal("zero-lifetime fragment zero is absent: the test would repeat the missing-fragment case")
	}
	e.spf.Run()
	if group, ok := loc.Lookup(family.IPv4Unicast, metricPrefix); ok && len(group.Paths) != 0 {
		t.Fatalf("fragment one of a set with expired fragment zero supplied a route: %+v", group)
	}
}

// ownInterfaceAddrs returns the TLV 132 addresses of the node's own fragment
// zero at one level, sorted.
func ownInterfaceAddrs(t *testing.T, eng *engine, level lsdb.Level) []netip.Addr {
	t.Helper()
	entry := eng.lsdb.Lookup(level, types.NewLSPID(types.NewSourceID(eng.cfg.SystemID, 0), 0))
	if entry == nil {
		t.Fatalf("no own fragment zero at level %d", level)
	}
	lsp, err := entry.Decode()
	if err != nil {
		t.Fatal(err)
	}
	defer packet.ReleaseTLVs(lsp.TLVs)
	var addrs []netip.Addr
	for _, tlv := range lsp.TLVs {
		if tlv.Type != packet.TLVIPInterfaceAddress {
			continue
		}
		for off := 0; off+4 <= len(tlv.Value); off += 4 {
			addrs = append(addrs, netip.AddrFrom4([4]byte(tlv.Value[off:off+4])))
		}
	}
	slices.SortFunc(addrs, netip.Addr.Compare)
	return addrs
}

// RFC requirement: RFC1195-5.2-3 positive -- a level 1 and level 2 router with two circuits originates fragment zero at both levels, and each LSP's TLV 132 set is exactly both circuit addresses, 192.0.2.9 and 198.51.100.9.
// RFC requirement: RFC1195-5.2-2 positive -- the engine collects the router's own interface addresses into its LSPs: fragment zero at each level carries TLV 132 listing exactly the two circuit addresses, 192.0.2.9 and 198.51.100.9.
func TestRFC1195SameInterfaceAddressesBothLevels(t *testing.T) {
	if err := registerConnectedPrefixBackend(); err != nil {
		t.Fatal(err)
	}
	previous := iface.ActiveBackendName()
	if err := iface.LoadBackend("isis-connected-prefix-test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := iface.CloseBackend(); err != nil {
			t.Error(err)
		}
		if previous != "" {
			if err := iface.LoadBackend(previous); err != nil {
				t.Error(err)
			}
		}
	})
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1-l2","interfaces":{"interface":{`+
		`"isis-conn-a":{"circuit-type":"point-to-point","level":"l1-l2","metric":"15"},`+
		`"isis-conn-b":{"circuit-type":"point-to-point","level":"l1-l2","metric":"15"}}}}}`)
	defer eng.shutdown()
	want := []netip.Addr{netip.MustParseAddr("192.0.2.9"), netip.MustParseAddr("198.51.100.9")}
	for _, level := range []lsdb.Level{lsdb.Level1, lsdb.Level2} {
		if got := ownInterfaceAddrs(t, eng, level); !slices.Equal(got, want) {
			t.Fatalf("level %d LSP interface addresses = %v, want %v", level, got, want)
		}
	}
}
