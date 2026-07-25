// Design: plan/spec-isis-6-lsdb.md -- engine<->LSDB wiring tests (Wiring Test table).
//
// VALIDATES the end-to-end wiring this spec owns at the engine layer:
//   - an adjacency reaching Up fires origination, the own LSP is stored, and SRM
//     is armed on the eligible circuit (TestISISEngineOriginateOnAdjacencyUp ->
//     the spec Wiring Test "adjacency Up -> origination builds own LSP, stores
//     it, sets SRM");
//   - connected/redistributed prefixes flow into TLV 135 origination
//     (TestISISEngineConnectedPrefixOrigination);
//   - `show isis database` returns the live snapshot
//     (TestISISEngineDatabaseSnapshot).
// The lsdb-package unit tests cover the store/origination/aging internals; these
// prove the root-package glue actually reaches them.

package isis

import (
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// startedEngine opens the engine over a fake backend for the given config JSON
// and returns it (already with circuits open and an initial origination done).
// The caller defers eng.shutdown().
func startedEngine(t *testing.T, cfgJSON string) *engine {
	t.Helper()
	cfg, err := parseISISConfig(sec(cfgJSON))
	if err != nil {
		t.Fatalf("parseISISConfig: %v", err)
	}
	eng := newEngine(transport.New(&fakeBackend{}))
	eng.setConfig(cfg)
	if err := eng.openCircuits(); err != nil {
		t.Fatalf("openCircuits: %v", err)
	}
	return eng
}

// p2pHelloPDU builds a legacy (no TLV 240) point-to-point IIH from sys carrying
// the area TLV 1 so an L1 adjacency can form. A legacy P2P Hello brings the
// adjacency Up on first receipt (implicit two-way, RFC 5303 sec 3.2 fall-back).
func p2pHelloPDU(t *testing.T, sys types.SystemID, area types.AreaID) []byte {
	t.Helper()
	// TLV 1 (Area Addresses): 1-octet length + area octets.
	var areaScratch [13]byte
	an := area.WriteTo(areaScratch[:], 0)
	areaVal := append([]byte{byte(an)}, areaScratch[:an]...)

	h := &packet.P2PHello{
		CircuitType:    packet.CircuitL1L2,
		SystemID:       sys,
		HoldingTime:    30,
		LocalCircuitID: 1,
		TLVs:           []packet.TLV{{Type: packet.TLVAreaAddresses, Value: areaVal}},
	}
	buf := make([]byte, h.EncodedLen())
	n := h.WriteTo(buf, 0)
	return buf[:n]
}

func TestISISEngineOriginateOnAdjacencyUp(t *testing.T) {
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point","metric":"15"}}}}}`)
	defer eng.shutdown()

	node := eng.cfg.SystemID
	area := eng.cfg.NETs[0].AreaID()

	// RFC requirement: RFC1195-5.2-3 positive -- the node originates its own fragment 0 at BOTH Level 1 and Level 2 (checked here at L2, and at L1 below); interface addresses (TLV 132) are collected level-independently (internal/plugins/isis/lsdb_wiring.go:436-448), so the same IP address set is advertised at both levels by construction (RFC 1195 sec 5.2).
	// The initial origination (in openCircuits) already produced the node's own
	// fragment 0 at both levels (l1-l2 default).
	frag0L2 := types.NewLSPID(types.NewSourceID(node, 0), 0)
	if eng.lsdb.Lookup(lsdb.Level2, frag0L2) == nil {
		t.Fatal("no own L2 fragment 0 after start (origination not wired)")
	}

	// Drive an adjacency Up by feeding a legacy P2P Hello to the circuit; the
	// transition hook fires e.originate().
	eng.circuitsMu.RLock()
	c := eng.circuitByName["eth0"]
	eng.circuitsMu.RUnlock()
	if c == nil {
		t.Fatal("eth0 circuit not built")
	}
	neighbor := types.SystemID{0, 0, 0, 0, 0, 2}
	tr := c.Receive(adjacency.SNPA{}, p2pHelloPDU(t, neighbor, area))
	if !tr.SessionUp {
		t.Fatalf("adjacency did not reach Up on a legacy P2P Hello: %+v", tr)
	}

	// AC-1 (wired): the regenerated own LSP now lists the neighbor in TLV 22.
	e := eng.lsdb.Lookup(lsdb.Level1, types.NewLSPID(types.NewSourceID(node, 0), 0))
	if e == nil {
		t.Fatal("own L1 fragment 0 missing after adjacency Up")
	}
	if e.Sequence() == 0 {
		t.Error("own LSP has reserved sequence 0")
	}
	lsp, err := e.Decode()
	if err != nil {
		t.Fatalf("decode own L1 LSP: %v", err)
	}
	if !lsp.VerifyChecksum() {
		t.Error("own LSP checksum invalid")
	}
	var sawNeighbor, sawArea, sawProto, sawHost bool
	for _, tl := range lsp.TLVs {
		switch tl.Type {
		case packet.TLVExtendedISReach:
			ext, _ := packet.DecodeExtendedISReachTLV(tl.Value)
			for _, ent := range ext.Entries {
				if ent.Neighbor == types.NewSourceID(neighbor, 0) {
					sawNeighbor = true
					if ent.Metric.Value() != 15 {
						t.Errorf("TLV 22 metric = %d, want 15 (circuit metric)", ent.Metric.Value())
					}
				}
			}
		case packet.TLVAreaAddresses:
			sawArea = true
		case packet.TLVProtocolsSupported:
			sawProto = true
		case packet.TLVDynamicHostname:
			sawHost = true
		}
	}
	if !sawNeighbor {
		t.Error("Up neighbor not advertised in TLV 22 after origination")
	}
	if !sawArea || !sawProto {
		t.Errorf("fragment 0 missing fixed TLVs: area=%v proto=%v", sawArea, sawProto)
	}
	_ = sawHost // no hostname configured in this test

	// SRM is armed on the circuit for the originated L1 fragment 0 so isis-7
	// floods it (the spec Wiring Test: "sets SRM on eligible circuits").
	cid := eng.circuitIDFor("eth0")
	if !eng.lsdb.SRM(lsdb.Level1, types.NewLSPID(types.NewSourceID(node, 0), 0), cid) {
		t.Error("SRM not armed on eth0 for the originated L1 fragment 0")
	}
}

func TestISISEngineConnectedPrefixOrigination(t *testing.T) {
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"metric":"10"}}}}}`)
	defer eng.shutdown()

	// Inject a connected prefix at L2 (the path isis-11 will use) and re-originate.
	prefix := netip.MustParsePrefix("198.51.100.0/24")
	eng.setPrefixes(lsdb.Level2, []lsdb.PrefixInfo{{
		Prefix: prefix,
		Metric: types.NewPrefixMetric(10),
	}})
	eng.originate()

	node := eng.cfg.SystemID
	e := eng.lsdb.Lookup(lsdb.Level2, types.NewLSPID(types.NewSourceID(node, 0), 0))
	if e == nil {
		t.Fatal("own L2 fragment 0 missing")
	}
	lsp, err := e.Decode()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	var found bool
	for _, tl := range lsp.TLVs {
		if tl.Type == packet.TLVExtendedIPReach {
			ext, err := packet.DecodeExtendedIPReachTLV(tl.Value)
			if err != nil {
				t.Fatalf("decode TLV 135: %v", err)
			}
			for _, ent := range ext.Entries {
				if ent.Prefix == prefix {
					found = true
				}
			}
		}
	}
	if !found {
		t.Errorf("connected prefix %s not advertised in TLV 135", prefix)
	}
}

// TestISISEngineLeakOrigination is the engine-glue regression for RFC 2966
// inter-level leaking (spec AC-4/AC-5): the SPF Computer's leak callback
// (applyLeak) stores the other level's reachable prefixes and re-originates them
// into this level's own LSP, carrying the up/down bit. It proves the engine side
// of the leak -- the spf-package leak math is covered by spf/leak_test.go.
//
// The leak set is injected via applyLeak directly (the exact value the Computer
// hands the engine) so the assertion is deterministic and does not depend on the
// async SPF debounce. It checks: an L2-derived prefix lands in the L1 LSP with the
// up/down bit SET, an L1-derived prefix lands in the L2 LSP with the bit CLEAR,
// and a re-apply of the SAME set does NOT bump the sequence (the fixpoint).
func TestISISEngineLeakOrigination(t *testing.T) {
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"metric":"10"}}}}}`)
	defer eng.shutdown()

	node := eng.cfg.SystemID
	frag0 := func(level lsdb.Level) (*packet.LSP, types.SequenceNumber) {
		t.Helper()
		e := eng.lsdb.Lookup(level, types.NewLSPID(types.NewSourceID(node, 0), 0))
		if e == nil {
			t.Fatalf("own %s fragment 0 missing", level)
		}
		lsp, err := e.Decode()
		if err != nil {
			t.Fatalf("decode %s LSP: %v", level, err)
		}
		return &lsp, e.Sequence()
	}

	// tlv135UpDown returns whether prefix is present in the LSP's TLV 135 and, if
	// so, its up/down bit.
	tlv135UpDown := func(lsp *packet.LSP, prefix netip.Prefix) (present, upDown bool) {
		for _, tl := range lsp.TLVs {
			if tl.Type != packet.TLVExtendedIPReach {
				continue
			}
			ext, err := packet.DecodeExtendedIPReachTLV(tl.Value)
			if err != nil {
				t.Fatalf("decode TLV 135: %v", err)
			}
			for _, ent := range ext.Entries {
				if ent.Prefix == prefix {
					return true, ent.UpDown
				}
			}
		}
		return false, false
	}

	l2Derived := netip.MustParsePrefix("10.2.0.0/24") // leaked DOWN into L1, bit set
	l1Derived := netip.MustParsePrefix("10.1.0.0/24") // leaked UP into L2, bit clear

	leak := spf.LeakResult{
		IntoL1: []spf.LeakedPrefix{{Prefix: l2Derived, Metric: 27, UpDown: true}},
		IntoL2: []spf.LeakedPrefix{{Prefix: l1Derived, Metric: 15, UpDown: false}},
	}
	eng.applyLeak(leak)

	// L1 LSP: the L2-derived prefix is present with the up/down bit SET.
	l1LSP, l1Seq := frag0(lsdb.Level1)
	// RFC requirement: RFC5305-4.1-1 positive -- a prefix leaked from L2 down into L1 carries the TLV 135 up/down bit SET (RFC 5305 sec 4.1: advertised from a higher level to a lower level).
	if present, up := tlv135UpDown(l1LSP, l2Derived); !present || !up {
		t.Errorf("L1 LSP: leaked %s present=%v up/down=%v, want present with up/down SET (RFC 2966 down leak)", l2Derived, present, up)
	}
	// The L1-derived prefix must NOT appear in the L1 LSP (it is leaked into L2).
	if present, _ := tlv135UpDown(l1LSP, l1Derived); present {
		t.Errorf("L1 LSP must not carry the L1-native prefix %s as a leak", l1Derived)
	}

	// L2 LSP: the L1-derived prefix is present with the up/down bit CLEAR.
	l2LSP, l2Seq := frag0(lsdb.Level2)
	// RFC requirement: RFC5305-4.1-1 negative -- a prefix leaked UP from L1 into L2 leaves the up/down bit CLEAR; the bit is set only on a down-the-hierarchy advertisement (RFC 5305 sec 4.1).
	if present, up := tlv135UpDown(l2LSP, l1Derived); !present || up {
		t.Errorf("L2 LSP: leaked %s present=%v up/down=%v, want present with up/down CLEAR (up leak)", l1Derived, present, up)
	}

	// Fixpoint: re-applying the SAME leak set must NOT re-originate (no sequence
	// bump), so the SPF->re-originate->SPF feedback loop terminates without churn.
	eng.applyLeak(leak)
	if _, seq := frag0(lsdb.Level1); seq != l1Seq {
		t.Errorf("L1 sequence bumped on identical leak re-apply: %d -> %d (not a fixpoint)", l1Seq, seq)
	}
	if _, seq := frag0(lsdb.Level2); seq != l2Seq {
		t.Errorf("L2 sequence bumped on identical leak re-apply: %d -> %d (not a fixpoint)", l2Seq, seq)
	}

	// Clearing the leak (empty set) re-originates and withdraws the leaked entry.
	eng.applyLeak(spf.LeakResult{})
	if present, _ := tlv135UpDown(mustFrag0(t, eng, node, lsdb.Level1), l2Derived); present {
		t.Errorf("L1 LSP still carries leaked %s after the leak was cleared", l2Derived)
	}
}

// mustFrag0 decodes the node's own fragment-0 LSP at level, failing the test if
// absent. A small helper for the leak test's clear-path assertion.
func mustFrag0(t *testing.T, eng *engine, node types.SystemID, level lsdb.Level) *packet.LSP {
	t.Helper()
	e := eng.lsdb.Lookup(level, types.NewLSPID(types.NewSourceID(node, 0), 0))
	if e == nil {
		t.Fatalf("own %s fragment 0 missing", level)
	}
	lsp, err := e.Decode()
	if err != nil {
		t.Fatalf("decode %s LSP: %v", level, err)
	}
	return &lsp
}

func TestISISEngineDatabaseSnapshot(t *testing.T) {
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","hostname":"snap-node","interfaces":{"interface":{"eth0":{}}}}}`)
	defer eng.shutdown()

	rows := eng.databaseSnapshot()
	if len(rows) == 0 {
		t.Fatal("show isis database returned no rows after origination")
	}
	// The snapshot crosses the RPC boundary as JSON (OnExecuteCommand returns
	// any); verify the rendered shape carries the lsp-id, sequence, lifetime,
	// checksum, and overload fields isis-13 renders (spec AC-10).
	blob, err := json.Marshal(rows)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	var decoded []struct {
		Level    string `json:"level"`
		LSPID    string `json:"lsp-id"`
		Sequence uint32 `json:"sequence"`
		Lifetime uint16 `json:"lifetime"`
		Checksum uint16 `json:"checksum"`
		Overload bool   `json:"overload"`
	}
	if err := json.Unmarshal(blob, &decoded); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	for _, row := range decoded {
		if row.LSPID == "" {
			t.Error("snapshot row has empty lsp-id")
		}
		if row.Sequence == 0 {
			t.Error("snapshot row has reserved sequence 0")
		}
		if row.Checksum == 0 {
			t.Error("snapshot row has zero checksum")
		}
		if row.Level != "l1" && row.Level != "l2" {
			t.Errorf("snapshot row level = %q, want l1/l2", row.Level)
		}
	}
}

// ownSeq returns the sequence number of the node's own fragment-0 LSP at level, or
// 0 when it is absent.
func ownSeq(eng *engine, node types.SystemID, level lsdb.Level) types.SequenceNumber {
	e := eng.lsdb.Lookup(level, types.NewLSPID(types.NewSourceID(node, 0), 0))
	if e == nil {
		return 0
	}
	return e.Sequence()
}

// TestISISEngineOriginateCoalescesNoChange is the flooding-amplification
// regression (Bundle E finding 1): a re-origination whose input is byte-identical
// to the previous one must NOT re-Insert / bump the sequence / re-flood. An
// adjacency flap fires originate() from several goroutines for the SAME resulting
// state; without coalescing each call bumped the sequence (N steps toward
// wraparound) and re-armed SRM on every circuit (N redundant floods). A REAL change
// must still re-originate (ISO/IEC 10589 clause 7.3.12), so the test also proves a
// changed input bumps the sequence.
func TestISISEngineOriginateCoalescesNoChange(t *testing.T) {
	// A point-to-point circuit with no neighbor: the origination input is stable
	// (no DIS, no adjacency churn), so repeated originate() calls have identical
	// input.
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point","metric":"10"}}}}}`)
	defer eng.shutdown()
	node := eng.cfg.SystemID

	// The initial origination (in openCircuits) produced fragment 0 at both levels.
	seqL1 := ownSeq(eng, node, lsdb.Level1)
	seqL2 := ownSeq(eng, node, lsdb.Level2)
	if seqL1 == 0 || seqL2 == 0 {
		t.Fatalf("own fragment 0 missing after start: L1=%d L2=%d", seqL1, seqL2)
	}

	// Several re-originations with NO topology change: the sequence must not move
	// (the redundant re-floods collapse to nothing).
	for range 5 {
		eng.originate()
	}
	if got := ownSeq(eng, node, lsdb.Level1); got != seqL1 {
		t.Errorf("unchanged re-origination bumped L1 sequence %d -> %d (should coalesce)", seqL1, got)
	}
	if got := ownSeq(eng, node, lsdb.Level2); got != seqL2 {
		t.Errorf("unchanged re-origination bumped L2 sequence %d -> %d (should coalesce)", seqL2, got)
	}

	// A REAL change: inject a connected prefix at L2. The L2 input now differs, so
	// L2 must re-originate (sequence bumps); L1 is unchanged and must stay put.
	eng.setPrefixes(lsdb.Level2, []lsdb.PrefixInfo{{
		Prefix: netip.MustParsePrefix("198.51.100.0/24"),
		Metric: types.NewPrefixMetric(10),
	}})
	eng.originate()
	bumpedL2 := ownSeq(eng, node, lsdb.Level2)
	if bumpedL2 != seqL2+1 {
		t.Errorf("real change at L2: sequence = %d, want %d (one bump)", bumpedL2, seqL2+1)
	}
	if got := ownSeq(eng, node, lsdb.Level1); got != seqL1 {
		t.Errorf("L2 change must not re-originate L1: sequence %d -> %d", seqL1, got)
	}

	// Re-originating with the SAME (now-changed) prefix set must coalesce again: no
	// further bump on either level.
	for range 3 {
		eng.originate()
	}
	if got := ownSeq(eng, node, lsdb.Level2); got != bumpedL2 {
		t.Errorf("re-origination after a change bumped L2 again %d -> %d (should coalesce)", bumpedL2, got)
	}
	if got := ownSeq(eng, node, lsdb.Level1); got != seqL1 {
		t.Errorf("L1 sequence drifted %d -> %d", seqL1, got)
	}
}

// setLastOrigAtPast plants the recorded last-origination time for level far enough
// in the past that a refresh is due (well beyond DefaultLSPRefreshInterval). It is
// the deterministic seam the periodic-refresh tests use instead of sleeping for a
// real refresh interval (900s): the refresh-due decision is a timestamp compare, so
// moving the timestamp back is equivalent to time elapsing.
func setLastOrigAtPast(eng *engine, level lsdb.Level) {
	eng.mu.Lock()
	if eng.lastOrigAt == nil {
		eng.lastOrigAt = make(map[lsdb.Level]time.Time)
	}
	eng.lastOrigAt[level] = time.Now().Add(-2 * DefaultLSPRefreshInterval * time.Second)
	eng.mu.Unlock()
}

// ownLifetime returns the stored Remaining Lifetime of the node's own fragment-0
// LSP at level (0 when absent).
func ownLifetime(eng *engine, node types.SystemID, level lsdb.Level) uint16 {
	e := eng.lsdb.Lookup(level, types.NewLSPID(types.NewSourceID(node, 0), 0))
	if e == nil {
		return 0
	}
	return e.Lifetime().Seconds()
}

// TestISISEngineRefreshDueReoriginates is the periodic own-LSP refresh regression
// (spec-isis-6 AC-3, ISO/IEC 10589 clause 7.3.16.1): in a quiescent network the
// aging loop must re-originate an own LSP once lsp-refresh-interval has elapsed,
// bumping the sequence and resetting the Remaining Lifetime to MaxAge so the LSP
// never ages out. It drives the refresh deterministically by planting lastOrigAt in
// the past, then invoking the same refreshOwnLSPs the aging tick calls -- no 900s
// sleep. A point-to-point circuit with no neighbor keeps the origination input
// stable, so the ONLY reason to re-originate is the elapsed refresh interval.
func TestISISEngineRefreshDueReoriginates(t *testing.T) {
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point","metric":"10"}}}}}`)
	defer eng.shutdown()
	node := eng.cfg.SystemID

	if seqL1, seqL2 := ownSeq(eng, node, lsdb.Level1), ownSeq(eng, node, lsdb.Level2); seqL1 == 0 || seqL2 == 0 {
		t.Fatalf("own fragment 0 missing after start: L1=%d L2=%d", seqL1, seqL2)
	}

	// Make a refresh due on both levels, then run the aging-loop refresh driver.
	// Capture the per-level baseline sequence IMMEDIATELY before the refresh. The
	// assertion is "strictly higher" rather than exactly +1 because the engine's
	// own aging loop and the debounced SPF leak callback run concurrently and may
	// add their own (correct) re-stamps; the AC is that the refresh BUMPS the
	// sequence and resets the lifetime, not that it is the sole bumper.
	baseL1 := ownSeq(eng, node, lsdb.Level1)
	baseL2 := ownSeq(eng, node, lsdb.Level2)
	setLastOrigAtPast(eng, lsdb.Level1)
	setLastOrigAtPast(eng, lsdb.Level2)

	// refreshDueLevels must report both configured levels as due.
	due := eng.refreshDueLevels(time.Now())
	if len(due) != 2 {
		t.Fatalf("refreshDueLevels = %v, want both L1 and L2 due", due)
	}

	eng.refreshOwnLSPs()

	// AC-3: the sequence is incremented (HIGHER than before the refresh) and the
	// Remaining Lifetime is reset to MaxAge (lsp-lifetime, default 1200) on each
	// level.
	if got := ownSeq(eng, node, lsdb.Level1); got <= baseL1 {
		t.Errorf("L1 refresh sequence = %d, want > %d (refresh must bump)", got, baseL1)
	}
	if got := ownSeq(eng, node, lsdb.Level2); got <= baseL2 {
		t.Errorf("L2 refresh sequence = %d, want > %d (refresh must bump)", got, baseL2)
	}
	if got := ownLifetime(eng, node, lsdb.Level1); got != DefaultLSPLifetime {
		t.Errorf("L1 refresh Remaining Lifetime = %d, want %d (reset to MaxAge)", got, DefaultLSPLifetime)
	}
	if got := ownLifetime(eng, node, lsdb.Level2); got != DefaultLSPLifetime {
		t.Errorf("L2 refresh Remaining Lifetime = %d, want %d (reset to MaxAge)", got, DefaultLSPLifetime)
	}

	// The refreshed LSP must still carry a valid Fletcher checksum (R-4: recompute on
	// every sequence increment).
	e := eng.lsdb.Lookup(lsdb.Level1, types.NewLSPID(types.NewSourceID(node, 0), 0))
	lsp, err := e.Decode()
	if err != nil {
		t.Fatalf("decode refreshed L1 LSP: %v", err)
	}
	if !lsp.VerifyChecksum() {
		t.Error("refreshed L1 LSP checksum invalid")
	}
}

// TestISISEngineRefreshNotDueNoReorigination proves the refresh driver is a no-op
// when no refresh is due: a freshly-originated own LSP (lastOrigAt = now) must NOT
// be re-originated by the aging-loop refresh, so a quiescent node does not bump its
// sequence every second (the refresh fires only at lsp-refresh-interval, not every
// tick). This is the complement of TestISISEngineRefreshDueReoriginates.
func TestISISEngineRefreshNotDueNoReorigination(t *testing.T) {
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point","metric":"10"}}}}}`)
	defer eng.shutdown()
	node := eng.cfg.SystemID

	seqL1 := ownSeq(eng, node, lsdb.Level1)
	seqL2 := ownSeq(eng, node, lsdb.Level2)

	// The initial origination recorded lastOrigAt = now, so nothing is due.
	if due := eng.refreshDueLevels(time.Now()); len(due) != 0 {
		t.Fatalf("refreshDueLevels = %v immediately after origination, want none due", due)
	}

	// Several refresh-driver invocations must not move the sequence (no refresh due).
	for range 5 {
		eng.refreshOwnLSPs()
	}
	if got := ownSeq(eng, node, lsdb.Level1); got != seqL1 {
		t.Errorf("not-due refresh bumped L1 sequence %d -> %d", seqL1, got)
	}
	if got := ownSeq(eng, node, lsdb.Level2); got != seqL2 {
		t.Errorf("not-due refresh bumped L2 sequence %d -> %d", seqL2, got)
	}
}

// TestISISEngineRefreshDuePerLevel proves the refresh-due decision is per level: a
// node whose L1 origination is stale but L2 is fresh refreshes ONLY L1. This guards
// the "refresh exactly the due levels" property -- refreshDueLevels reports only L1,
// and originate's per-level coalescing then re-stamps L1 while leaving the fresh L2
// untouched (so a refresh does not gratuitously re-flood every level).
func TestISISEngineRefreshDuePerLevel(t *testing.T) {
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point","metric":"10"}}}}}`)
	defer eng.shutdown()
	node := eng.cfg.SystemID

	// Only L1 is stale. Capture L2's baseline immediately before the refresh so the
	// "L2 unchanged" assertion is exact even if a concurrent async originate() runs:
	// such a pass re-checks every level's freshness and coalesces the fresh L2 away,
	// so L2 must stay put regardless.
	setLastOrigAtPast(eng, lsdb.Level1)
	baseL1 := ownSeq(eng, node, lsdb.Level1)
	baseL2 := ownSeq(eng, node, lsdb.Level2)

	// The pure decision: only L1 is due (deterministic, no async interference).
	due := eng.refreshDueLevels(time.Now())
	if len(due) != 1 || due[0] != lsdb.Level1 {
		t.Fatalf("refreshDueLevels = %v, want only L1 due", due)
	}

	eng.refreshOwnLSPs()
	if got := ownSeq(eng, node, lsdb.Level1); got <= baseL1 {
		t.Errorf("L1 refresh sequence = %d, want > %d (the due level must bump)", got, baseL1)
	}
	if got := ownSeq(eng, node, lsdb.Level2); got != baseL2 {
		t.Errorf("L2 must not refresh (still fresh): sequence %d -> %d", baseL2, got)
	}
}
