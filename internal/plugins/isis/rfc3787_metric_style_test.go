// Design: docs/architecture/isis/isis-6-lsdb.md -- the metric style of the own LSPs.
// Related: rfc3787_absence_test.go -- absencePeerIIH, the neighbor Hello helper
// Related: rfc1195_neighbor_addresses_test.go -- the on-link backend and its addresses
//
// VALIDATES: RFC 3787 section 5, "If not all devices in the IS-IS domain support
// wide metrics, narrow metrics MUST continue to be used.", on the path an
// operator's config takes: the delivered `metric-style` leaf -> parseISISConfig
// -> validateConfig -> setConfig -> the engine's own-LSP origination. Under
// narrow the own LSP carries TLV 2 and TLV 128 and nothing wide, under transition
// it carries both families at the same metric, and a narrow-only router's LSP is
// routed through. A circuit metric no narrow TLV can carry is refused at commit.
// PREVENTS: a node configured for a narrow-metric domain that still advertises
// only TLV 22 / TLV 135, which a narrow-only router cannot read, and a narrow
// metric silently advertised as a different value than the operator configured.
package isis

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// metricStyleConfig is onLinkPeerConfig with the metric-style leaf and a circuit
// metric of 20 on the point-to-point circuit the neighbor comes up on.
func metricStyleConfig(style string) string {
	return `{"isis":{"net":"49.0001.0000.0000.0001.00","metric-style":"` + style + `","interfaces":{"interface":{
"isis-on-a":{"level":"l1","hello-interval":"3600","circuit-type":"point-to-point","metric":"20"},
"isis-on-b":{"level":"l1","hello-interval":"3600","circuit-type":"point-to-point","metric":"20"}}}}}`
}

// metricStyleOwnTLVs brings the IP-capable neighbor up on isis-on-a under the
// given style, originates, and answers the TLVs of the node's own Level-1 LSP
// fragment zero, keyed by type. The values are copies, safe after release.
func metricStyleOwnTLVs(t *testing.T, style string) map[uint8][][]byte {
	t.Helper()
	useOnLinkBackend(t)
	eng, _ := protocolEngine(t, metricStyleConfig(style))
	c := protocolLiveCircuit(t, eng, "isis-on-a")
	eng.dispatch.dispatch(transport.RawFrame{IfIndex: c.IfIndex(), PDU: onLinkPeerIIH(t, []netip.Addr{absencePeerAddr})})
	if rows := c.Table().Snapshot(); len(rows) != 1 || rows[0].State != "up" {
		t.Fatalf("neighbor did not form an Up adjacency: %+v", rows)
	}
	eng.originate()

	id := types.NewLSPID(types.NewSourceID(eng.cfg.SystemID, 0), 0)
	entry := eng.lsdb.Lookup(lsdb.Level1, id)
	if entry == nil {
		t.Fatal("no own Level-1 LSP fragment zero")
	}
	p, err := packet.DecodePDU(entry.Raw())
	if err != nil {
		t.Fatal(err)
	}
	defer packet.ReleaseTLVs(p.LSP.TLVs)
	out := map[uint8][][]byte{}
	for _, tlv := range p.LSP.TLVs {
		out[tlv.Type] = append(out[tlv.Type], append([]byte(nil), tlv.Value...))
	}
	return out
}

// requireNarrowNeighbor asserts exactly one TLV 2 lists the neighbor at metric
// 20 with the three optional metrics marked unsupported.
func requireNarrowNeighbor(t *testing.T, tlvs map[uint8][][]byte) {
	t.Helper()
	values := tlvs[packet.TLVISReachabilityNarrow]
	if len(values) != 1 {
		t.Fatalf("own LSP carries %d TLV 2, want 1", len(values))
	}
	decoded, err := packet.DecodeNarrowISReachTLV(values[0])
	if err != nil {
		t.Fatalf("own TLV 2 does not decode: %v", err)
	}
	if decoded.VirtualFlag != 0 {
		t.Errorf("TLV 2 virtual flag = %d, want 0", decoded.VirtualFlag)
	}
	if len(decoded.Entries) != 1 {
		t.Fatalf("TLV 2 entries = %+v, want the one neighbor", decoded.Entries)
	}
	got := decoded.Entries[0]
	if got.Neighbor != types.NewSourceID(onLinkPeer, 0) {
		t.Errorf("TLV 2 neighbor = %v, want %v", got.Neighbor, types.NewSourceID(onLinkPeer, 0))
	}
	if got.DefaultMetricValue != 20 || got.DefaultMetricExternal {
		t.Errorf("TLV 2 default metric = %d external=%v, want internal 20", got.DefaultMetricValue, got.DefaultMetricExternal)
	}
	if got.DelayMetric != 0x80 || got.ExpenseMetric != 0x80 || got.ErrorMetric != 0x80 {
		t.Errorf("TLV 2 optional metrics = %#x %#x %#x, want 0x80 (unsupported)", got.DelayMetric, got.ExpenseMetric, got.ErrorMetric)
	}
}

// requireNarrowPrefixes asserts the connected prefixes of both circuits appear in
// TLV 128 at metric 20, internal, with no TLV 130.
func requireNarrowPrefixes(t *testing.T, tlvs map[uint8][][]byte) {
	t.Helper()
	want := map[netip.Prefix]bool{
		netip.MustParsePrefix("192.0.2.0/24"):    false,
		netip.MustParsePrefix("198.51.100.0/24"): false,
	}
	for _, value := range tlvs[packet.TLVIPInternalReachability] {
		decoded, err := packet.DecodeNarrowIPReachTLV(value, false)
		if err != nil {
			t.Fatalf("own TLV 128 does not decode: %v", err)
		}
		for _, e := range decoded.Entries {
			if _, known := want[e.Prefix]; !known {
				t.Errorf("TLV 128 carries unexpected prefix %v", e.Prefix)
				continue
			}
			if e.DefaultMetricValue != 20 || e.ExternalMetric {
				t.Errorf("TLV 128 %v metric = %d external=%v, want internal 20", e.Prefix, e.DefaultMetricValue, e.ExternalMetric)
			}
			want[e.Prefix] = true
		}
	}
	for prefix, seen := range want {
		if !seen {
			t.Errorf("connected prefix %v is missing from TLV 128", prefix)
		}
	}
	if n := len(tlvs[packet.TLVIPExternalReachability]); n != 0 {
		t.Errorf("own LSP carries %d TLV 130 for connected prefixes, want none", n)
	}
}

// RFC requirement: RFC3787-5-1 positive -- with `metric-style narrow` the node's
// own LSP lists its Up neighbor in TLV 2 and its connected prefixes in TLV 128,
// each at the configured metric 20, and carries no TLV 22 or TLV 135; with
// `metric-style transition` it carries those narrow TLVs and also TLV 22 and
// TLV 135.
func TestRFC3787NarrowMetricStyleOriginatesNarrowTLVs(t *testing.T) {
	t.Run("narrow", func(t *testing.T) {
		tlvs := metricStyleOwnTLVs(t, "narrow")
		requireNarrowNeighbor(t, tlvs)
		requireNarrowPrefixes(t, tlvs)
		if n := len(tlvs[packet.TLVExtendedISReach]); n != 0 {
			t.Errorf("narrow style own LSP carries %d TLV 22, want none", n)
		}
		if n := len(tlvs[packet.TLVExtendedIPReach]); n != 0 {
			t.Errorf("narrow style own LSP carries %d TLV 135, want none", n)
		}
	})
	t.Run("transition", func(t *testing.T) {
		tlvs := metricStyleOwnTLVs(t, "transition")
		requireNarrowNeighbor(t, tlvs)
		requireNarrowPrefixes(t, tlvs)
		wide := tlvs[packet.TLVExtendedISReach]
		if len(wide) != 1 {
			t.Fatalf("transition style own LSP carries %d TLV 22, want 1", len(wide))
		}
		decoded, err := packet.DecodeExtendedISReachTLV(wide[0])
		if err != nil {
			t.Fatal(err)
		}
		if len(decoded.Entries) != 1 || decoded.Entries[0].Metric.Value() != 20 {
			t.Errorf("TLV 22 entries = %+v, want the neighbor at metric 20, agreeing with TLV 2", decoded.Entries)
		}
		if len(tlvs[packet.TLVExtendedIPReach]) == 0 {
			t.Error("transition style own LSP carries no TLV 135")
		}
	})
}

// TestMetricStyleWideOriginatesNoNarrowTLVs: under `metric-style wide` the own
// LSP is wide only, and a config with no metric-style leaf resolves to wide,
// so an operator who sets nothing keeps the RFC 5305 encoding.
func TestMetricStyleWideOriginatesNoNarrowTLVs(t *testing.T) {
	defaults, err := parseISISConfig(sec(onLinkPeerConfig))
	if err != nil {
		t.Fatal(err)
	}
	if defaults.MetricStyle != lsdb.MetricStyleWide {
		t.Fatalf("metric-style default = %s, want wide", defaults.MetricStyle)
	}
	tlvs := metricStyleOwnTLVs(t, "wide")
	if n := len(tlvs[packet.TLVISReachabilityNarrow]); n != 0 {
		t.Errorf("wide style own LSP carries %d TLV 2, want none", n)
	}
	if n := len(tlvs[packet.TLVIPInternalReachability]); n != 0 {
		t.Errorf("wide style own LSP carries %d TLV 128, want none", n)
	}
	if len(tlvs[packet.TLVExtendedISReach]) != 1 || len(tlvs[packet.TLVExtendedIPReach]) == 0 {
		t.Errorf("wide style own LSP lacks TLV 22 or TLV 135: %d / %d", len(tlvs[packet.TLVExtendedISReach]), len(tlvs[packet.TLVExtendedIPReach]))
	}
}

// RFC requirement: RFC3787-5-1 negative -- under `metric-style narrow` a
// circuit metric of 64, which no narrow-metric TLV can carry, is refused at
// commit naming the interface and the narrow range, whether set circuit-wide or
// as a level override, while 63 is accepted; under `metric-style wide` 64 is
// accepted.
func TestRFC3787NarrowMetricStyleRefusesUnrepresentableMetric(t *testing.T) {
	cases := []struct {
		name    string
		style   string
		iface   string
		refused bool
	}{
		{"narrow circuit metric 64", "narrow", `"metric":"64"`, true},
		{"transition circuit metric 64", "transition", `"metric":"64"`, true},
		{"narrow level-1 metric 64", "narrow", `"level-1":{"metric":"64"}`, true},
		{"narrow level-2 metric 64", "narrow", `"level-2":{"metric":"64"}`, true},
		{"narrow circuit metric 63", "narrow", `"metric":"63"`, false},
		{"wide circuit metric 64", "wide", `"metric":"64"`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := `{"isis":{"net":"49.0001.0000.0000.0001.00","metric-style":"` + tc.style +
				`","interfaces":{"interface":{"eth0":{` + tc.iface + `}}}}}`
			cfg, err := parseISISConfig(sec(data))
			if err != nil {
				t.Fatalf("parseISISConfig: %v", err)
			}
			err = validateConfig(cfg)
			if !tc.refused {
				if err != nil {
					t.Fatalf("validateConfig refused %s: %v", tc.name, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateConfig accepted %s, a metric no narrow TLV can carry", tc.name)
			}
			for _, want := range []string{`"eth0"`, "64", "1 to 63", "metric-style wide"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal %q does not name %q", err, want)
				}
			}
		})
	}
}

// TestRFC3787NarrowOnlyNeighborIsRouted: a neighbor whose LSP lacks every
// wide-metric TLV (only TLV 2 back to Ze and TLV 128 for its prefix) is still
// routed through: its narrow TLV 2 edge completes the two-way check and its
// prefix is installed at the link metric plus the narrow prefix metric (20 + 5).
// It covers Ze's SPF reading narrow metrics, not the origination style, so it
// carries no RFC requirement tag.
func TestRFC3787NarrowOnlyNeighborIsRouted(t *testing.T) {
	e := newEngine(transport.New(&fakeBackend{}))
	e.spf.Stop()
	loc := locrib.NewRIB()
	e.spf = spf.NewComputer(spf.Config{
		Source: (*lsdbSPFSource)(e), Resolver: metricResolver{}, Root: metricSystem(1),
		Levels: []spf.Level{spf.Level1}, Installer: spf.NewInstaller(loc),
	})
	t.Cleanup(e.shutdown)
	if err := e.originator.SetMetricStyle(lsdb.MetricStyleNarrow); err != nil {
		t.Fatal(err)
	}
	link, err := types.NewMetric(20)
	if err != nil {
		t.Fatal(err)
	}
	e.originator.Originate(lsdb.Level1, lsdb.NodeInfo{SystemID: metricSystem(1), AdvertiseIPv4: true},
		lsdb.LevelState{Neighbors: []lsdb.AdjacencyInfo{{Neighbor: types.NewSourceID(metricSystem(2), 0), Metric: link}}})

	// The narrow-only neighbor: TLV 2 back to Ze at metric 20, TLV 128 for its
	// prefix at metric 5, and no TLV 22 or TLV 135.
	back := make([]byte, 1, 1+4+types.SourceIDLen)
	back = append(back, 20, 0x80, 0x80, 0x80)
	back = append(back, make([]byte, types.SourceIDLen)...)
	types.NewSourceID(metricSystem(1), 0).WriteTo(back, 5)
	prefix := packet.NarrowIPReachTLV{Entries: []packet.NarrowIPReachEntry{{
		DefaultMetricValue: 5, DelayMetric: 0x80, ExpenseMetric: 0x80, ErrorMetric: 0x80, Prefix: metricPrefix,
	}}}
	raw := make([]byte, prefix.EncodedLen())
	prefix.WriteTo(raw, 0)
	lsp := packet.LSP{PDUType: packet.PDUTypeL1LSP, RemainingLifetime: 1200,
		LSPID:          types.NewLSPID(types.NewSourceID(metricSystem(2), 0), 0),
		SequenceNumber: 1,
		TLVs: []packet.TLV{
			{Type: packet.TLVISReachabilityNarrow, Value: back},
			{Type: raw[0], Value: raw[2:]},
		}}
	wire := make([]byte, lsp.EncodedLen())
	lsp.WriteTo(wire, 0)
	decoded, err := packet.DecodePDU(wire)
	if err != nil {
		t.Fatal(err)
	}
	defer packet.ReleaseTLVs(decoded.LSP.TLVs)
	if result := e.lsdb.Receive(lsdb.Level1, decoded.LSP, wire, false); !result.Stored {
		t.Fatalf("narrow-only LSP not received: %+v", result)
	}

	metricWinner(t, e, loc, 2, 25)
}

// TestMetricStyleReloadReoriginates proves a commit that changes only
// metric-style re-encodes the own LSP at once: the origination input is
// unchanged, so without dropping the coalescing record the change would wait
// for the refresh interval.
func TestMetricStyleReloadReoriginates(t *testing.T) {
	useOnLinkBackend(t)
	eng, _ := protocolEngine(t, metricStyleConfig("wide"))
	id := types.NewLSPID(types.NewSourceID(eng.cfg.SystemID, 0), 0)
	carriesTLV128 := func() bool {
		t.Helper()
		entry := eng.lsdb.Lookup(lsdb.Level1, id)
		if entry == nil {
			t.Fatal("no own Level-1 LSP fragment zero")
		}
		p, err := packet.DecodePDU(entry.Raw())
		if err != nil {
			t.Fatal(err)
		}
		defer packet.ReleaseTLVs(p.LSP.TLVs)
		for _, tlv := range p.LSP.TLVs {
			if tlv.Type == packet.TLVIPInternalReachability {
				return true
			}
		}
		return false
	}
	if carriesTLV128() {
		t.Fatal("wide style own LSP already carries TLV 128")
	}
	reconcileTo(t, eng, metricStyleConfig("narrow"))
	if !carriesTLV128() {
		t.Fatal("after the reload to metric-style narrow the own LSP still carries no TLV 128")
	}
}

// TestMetricStyleChangeDropsPseudonodeCoalescing proves a metric-style change
// drops the pseudo-node coalescing record, so the next re-election pass
// re-encodes the pseudo-node LSP in the new style instead of skipping it as an
// unchanged input until the refresh interval.
func TestMetricStyleChangeDropsPseudonodeCoalescing(t *testing.T) {
	useOnLinkBackend(t)
	eng, _ := protocolEngine(t, metricStyleConfig("wide"))
	key := disKey{name: "isis-on-a", level: lsdb.Level1}
	eng.setPNLastOrigInput(key, pnInput{pnid: 1})
	reconcileTo(t, eng, metricStyleConfig("narrow"))
	eng.disMu.Lock()
	_, input := eng.pnLastInput[key]
	_, at := eng.pnLastOrigAt[key]
	eng.disMu.Unlock()
	if input || at {
		t.Fatalf("pseudo-node coalescing record survived the metric-style change: input=%v at=%v", input, at)
	}
}
