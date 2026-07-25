// Design: plan/spec-isis-7-flooding.md -- flooding receive algorithm + periodic SRM TX tests.
//
// VALIDATES: (lsdb-package unit level, fake tx + circuit set)
//   - the freshness-to-flag mapping for every ISO/IEC 10589 clause 7.3.15 outcome
//     (TestFreshnessCompareMatrix): higher seq (accept + SRM-others + SSN-in),
//     equal seq diff checksum (SSN-in request, AC-2), equal seq same checksum
//     (duplicate, AC-3), lower seq (SRM-in send back, AC-4), same-seq purge
//     (accept + re-flood, AC-16);
//   - the periodic flood timer transmits exactly the SRM-set LSPs and clears SRM
//     on a P2P send / leaves it on a LAN (TestSRMDrivenSend, AC-5), and a passive
//     circuit is skipped;
//   - an un-acked LAN SRM resends on the next tick (TestSRMResendOnNoAck, R-2);
//   - a zero-lifetime purge with seq >= ours is accepted, marked purged, SRM on
//     other circuits, not deleted (TestZeroLifetimePurgeReflood, AC-12);
//   - L1 and L2 SRM/SSN flags and pending sets never cross (TestLevelIsolation,
//     AC-14).

package lsdb

import (
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// sentPDU records one transmitted PDU for assertions.
type sentPDU struct {
	circuit string
	level   Level
	pdu     []byte
}

// recordingTx is a TxFunc that records every transmission and can be queried by
// circuit. Safe for concurrent use (the flood path is single-goroutine in tests,
// but the mutex keeps -race quiet).
type recordingTx struct {
	mu   sync.Mutex
	sent []sentPDU
}

func (r *recordingTx) tx(circuit string, level Level, pdu []byte) error {
	r.mu.Lock()
	cp := make([]byte, len(pdu))
	copy(cp, pdu)
	r.sent = append(r.sent, sentPDU{circuit: circuit, level: level, pdu: cp})
	r.mu.Unlock()
	return nil
}

func (r *recordingTx) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.sent)
}

// onCircuit returns the PDUs sent on a named circuit (any level). Tests using it
// flood a single level, so the circuit name alone selects the relevant sends.
func (r *recordingTx) onCircuit(name string) []sentPDU {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []sentPDU
	for _, s := range r.sent {
		if s.circuit == name {
			out = append(out, s)
		}
	}
	return out
}

func (r *recordingTx) reset() {
	r.mu.Lock()
	r.sent = nil
	r.mu.Unlock()
}

// staticCircuits returns a CircuitsFunc yielding a fixed set.
func staticCircuits(cs ...FloodCircuit) CircuitsFunc {
	return func() []FloodCircuit { return cs }
}

// l1l2Circuit builds an L1L2 broadcast circuit with the given name and id.
func l1l2Circuit(name string, id CircuitID) FloodCircuit {
	return FloodCircuit{Name: name, ID: id, FormsL1: true, FormsL2: true}
}

// p2pCircuit builds an L1L2 point-to-point circuit.
func p2pCircuit(name string, id CircuitID) FloodCircuit {
	c := l1l2Circuit(name, id)
	c.P2P = true
	return c
}

// TestFreshnessCompareMatrix drives ReceiveLSP through every clause-7.3.15
// outcome and asserts the resulting SRM/SSN flags on the receiving circuit
// (cIn=1) and the other circuit (cOut=2).
func TestFreshnessCompareMatrix(t *testing.T) {
	const cIn, cOut CircuitID = 1, 2
	id := lspID(10, 0)

	newHarness := func() (*LSDB, *Flooder) {
		d := New(nil)
		f := NewFlooder(d, nil, staticCircuits(
			l1l2Circuit("in", cIn),
			l1l2Circuit("out", cOut),
		))
		return d, f
	}

	// Case 1: higher sequence -> accept, SRM on cOut, SSN on cIn (AC-1).
	t.Run("higher-seq", func(t *testing.T) {
		d, f := newHarness()
		first, firstRaw := buildLSP(t, packet.PDUTypeL2LSP, id, 5, 1000, nil)
		f.ReceiveLSP(cIn, false, first, firstRaw) // first sighting is also "newer"
		if !d.SRM(Level2, id, cOut) {
			t.Error("SRM not set on the other circuit after a newer LSP")
		}
		if d.SRM(Level2, id, cIn) {
			t.Error("SRM set on the incoming circuit (must never re-flood there)")
		}
		if !d.SSN(Level2, id, cIn) {
			t.Error("SSN not set on the incoming circuit (no acknowledge)")
		}
		// A strictly higher sequence replaces and re-arms.
		d.ClearSRM(Level2, id, cOut)
		hi, hiRaw := buildLSP(t, packet.PDUTypeL2LSP, id, 9, 1000, nil)
		f.ReceiveLSP(cIn, false, hi, hiRaw)
		if d.Lookup(Level2, id).Sequence() != 9 {
			t.Errorf("stored seq = %d, want 9", d.Lookup(Level2, id).Sequence())
		}
		if !d.SRM(Level2, id, cOut) {
			t.Error("SRM not re-set on the other circuit after a higher seq")
		}
	})

	// Case 2: equal seq, DIFFERENT checksum -> discard, SSN on cIn to request the
	// correct copy (AC-2). We craft two distinct LSP bodies at the same sequence.
	t.Run("equal-seq-diff-checksum", func(t *testing.T) {
		d, f := newHarness()
		// Stored copy with one TLV body.
		a := &packet.LSP{PDUType: packet.PDUTypeL2LSP, LSPID: id, SequenceNumber: 5, RemainingLifetime: 1000,
			TLVs: []packet.TLV{{Type: 222, Value: []byte{0x01}}}}
		abuf := make([]byte, a.EncodedLen())
		an := a.WriteTo(abuf, 0)
		apdu, _ := packet.DecodePDU(abuf[:an])
		d.Insert(Level2, apdu.LSP, abuf[:an])
		storedCksum := d.Lookup(Level2, id).Checksum()

		// Incoming copy: same seq, different body -> different checksum.
		b := &packet.LSP{PDUType: packet.PDUTypeL2LSP, LSPID: id, SequenceNumber: 5, RemainingLifetime: 1000,
			TLVs: []packet.TLV{{Type: 222, Value: []byte{0x02}}}}
		bbuf := make([]byte, b.EncodedLen())
		bn := b.WriteTo(bbuf, 0)
		bpdu, _ := packet.DecodePDU(bbuf[:bn])
		if bpdu.LSP.Checksum == storedCksum {
			t.Skip("checksum collision in fixture; adjust bytes")
		}

		f.ReceiveLSP(cIn, false, bpdu.LSP, bbuf[:bn])
		if d.Lookup(Level2, id).Checksum() != storedCksum {
			t.Error("stored copy was replaced by a same-seq differing-checksum LSP")
		}
		if !d.SSN(Level2, id, cIn) {
			t.Error("SSN not set on the incoming circuit for a same-seq diff-checksum LSP (AC-2)")
		}
		if d.SRM(Level2, id, cIn) {
			t.Error("SRM wrongly set on incoming for a same-seq diff-checksum LSP")
		}
	})

	// Case 3: equal seq, SAME checksum -> duplicate. On a LAN, SSN set to ack; no
	// SRM anywhere (AC-3).
	t.Run("duplicate", func(t *testing.T) {
		d, f := newHarness()
		first, firstRaw := buildLSP(t, packet.PDUTypeL2LSP, id, 5, 1000, nil)
		d.Insert(Level2, first, firstRaw)
		// Re-receive the identical LSP on a LAN circuit.
		f.ReceiveLSP(cIn, false, first, firstRaw)
		if d.SRM(Level2, id, cIn) || d.SRM(Level2, id, cOut) {
			t.Error("SRM set on a duplicate LSP")
		}
		if !d.SSN(Level2, id, cIn) {
			t.Error("LAN duplicate did not set SSN to acknowledge (AC-3)")
		}
	})

	// Case 3b: duplicate on a P2P circuit sets NO SSN (the sender already cleared
	// SRM on send; no explicit ack needed).
	t.Run("duplicate-p2p", func(t *testing.T) {
		d := New(nil)
		f := NewFlooder(d, nil, staticCircuits(p2pCircuit("in", cIn), p2pCircuit("out", cOut)))
		first, firstRaw := buildLSP(t, packet.PDUTypeL2LSP, id, 5, 1000, nil)
		d.Insert(Level2, first, firstRaw)
		f.ReceiveLSP(cIn, true, first, firstRaw)
		if d.SSN(Level2, id, cIn) {
			t.Error("P2P duplicate set SSN; should not")
		}
	})

	// Case 4: lower sequence -> SRM on cIn to send our newer copy back (AC-4).
	t.Run("lower-seq", func(t *testing.T) {
		d, f := newHarness()
		stored, storedRaw := buildLSP(t, packet.PDUTypeL2LSP, id, 9, 1000, nil)
		d.Insert(Level2, stored, storedRaw)
		old, oldRaw := buildLSP(t, packet.PDUTypeL2LSP, id, 4, 1000, nil)
		f.ReceiveLSP(cIn, false, old, oldRaw)
		if d.Lookup(Level2, id).Sequence() != 9 {
			t.Error("a lower-seq LSP replaced our newer copy")
		}
		if !d.SRM(Level2, id, cIn) {
			t.Error("SRM not set on the incoming circuit to send our newer copy back (AC-4)")
		}
		if d.SSN(Level2, id, cIn) {
			t.Error("SSN wrongly set on a lower-seq LSP")
		}
	})

	// Case 5: same-seq PURGE (lifetime 0) over a held non-zero copy -> accept,
	// mark purged, SRM on cOut (AC-16). isis-6 compareFreshness gives Newer.
	t.Run("same-seq-purge", func(t *testing.T) {
		d, f := newHarness()
		stored, storedRaw := buildLSP(t, packet.PDUTypeL2LSP, id, 7, 1000, nil)
		d.Insert(Level2, stored, storedRaw)
		purge, purgeRaw := buildLSP(t, packet.PDUTypeL2LSP, id, 7, 0, nil)
		f.ReceiveLSP(cIn, false, purge, purgeRaw)
		if !d.Lookup(Level2, id).IsPurged() {
			t.Error("same-seq purge not marked purged (AC-16)")
		}
		if !d.SRM(Level2, id, cOut) {
			t.Error("purge not re-flooded (SRM) on the other circuit (AC-16)")
		}
		if d.SRM(Level2, id, cIn) {
			t.Error("purge re-flooded on the incoming circuit")
		}
	})
}

// TestSRMDrivenSend asserts the periodic flood timer transmits exactly the
// SRM-armed LSPs, LEAVES SRM set on BOTH a P2P and a LAN send (cleared only on
// acknowledgement, ISO/IEC 10589 clause 7.3.15.1, AC-5/AC-6/R-2), and skips a
// passive circuit. Clearing SRM on send -- before the neighbor acknowledges --
// would silently drop an LSP whose first transmission the neighbor missed, which
// is the reliable-flooding regression the three-node wiring test guards against.
func TestSRMDrivenSend(t *testing.T) {
	d := New(nil)
	rec := &recordingTx{}
	const cP2P, cLAN, cPassive CircuitID = 1, 2, 3
	f := NewFlooder(d, rec.tx, staticCircuits(
		p2pCircuit("p2p", cP2P),
		l1l2Circuit("lan", cLAN),
		FloodCircuit{Name: "pass", ID: cPassive, Passive: true, FormsL1: true, FormsL2: true},
	))

	id := lspID(20, 0)
	lsp, raw := buildLSP(t, packet.PDUTypeL1LSP, id, 3, 1000, nil)
	d.Insert(Level1, lsp, raw)
	// Arm SRM on all three circuits at L1.
	d.SetSRM(Level1, id, cP2P)
	d.SetSRM(Level1, id, cLAN)
	d.SetSRM(Level1, id, cPassive)

	f.FloodTick()

	// The P2P and LAN circuits transmit; the passive one does not.
	if got := len(rec.onCircuit("p2p")); got != 1 {
		t.Errorf("p2p sent %d LSPs, want 1", got)
	}
	if got := len(rec.onCircuit("lan")); got != 1 {
		t.Errorf("lan sent %d LSPs, want 1", got)
	}
	if got := len(rec.onCircuit("pass")); got != 0 {
		t.Errorf("passive circuit sent %d LSPs, want 0 (AC-5: passive skipped)", got)
	}
	// The transmitted bytes are the stored raw LSP, verbatim (clause 7.3.14).
	sent := rec.onCircuit("p2p")[0].pdu
	if len(sent) != len(raw) {
		t.Fatalf("flooded PDU len %d != stored raw %d (not verbatim)", len(sent), len(raw))
	}
	for i := range raw {
		if sent[i] != raw[i] {
			t.Fatalf("flooded PDU differs from stored raw at byte %d", i)
		}
	}

	// BOTH P2P and LAN SRM are left set after the send: SRM is cleared only on
	// acknowledgement (PSNP at our seq / equal CSNP entry), so an unacknowledged
	// flood is retried on the next tick (ISO/IEC 10589 clause 7.3.15.1).
	if !d.SRM(Level1, id, cP2P) {
		t.Error("P2P SRM cleared on send; must wait for ack (AC-6, clause 7.3.15.1)")
	}
	if !d.SRM(Level1, id, cLAN) {
		t.Error("LAN SRM cleared on send; must wait for ack (AC-5, R-2)")
	}
	// The passive circuit's SRM is untouched (never serviced).
	if !d.SRM(Level1, id, cPassive) {
		t.Error("passive circuit SRM should be untouched")
	}

	// A P2P send with no ack resends on the next tick (the reliability the
	// three-node line depends on). After a PSNP ack clears SRM, the next tick is
	// silent.
	rec.reset()
	f.FloodTick()
	if got := len(rec.onCircuit("p2p")); got != 1 {
		t.Errorf("P2P resent %d on the next tick, want 1 (no ack -> resend)", got)
	}
	d.ClearSRM(Level1, id, cP2P)
	rec.reset()
	f.FloodTick()
	if got := len(rec.onCircuit("p2p")); got != 0 {
		t.Errorf("P2P sent %d after SRM cleared by ack, want 0", got)
	}
}

// TestSRMResendOnNoAck asserts an un-acked LAN SRM resends on the next tick (R-2).
func TestSRMResendOnNoAck(t *testing.T) {
	d := New(nil)
	rec := &recordingTx{}
	const cLAN CircuitID = 1
	f := NewFlooder(d, rec.tx, staticCircuits(l1l2Circuit("lan", cLAN)))

	id := lspID(21, 0)
	lsp, raw := buildLSP(t, packet.PDUTypeL1LSP, id, 3, 1000, nil)
	d.Insert(Level1, lsp, raw)
	d.SetSRM(Level1, id, cLAN)

	f.FloodTick()
	f.FloodTick() // SRM still set (no ack) -> resend

	if got := len(rec.onCircuit("lan")); got != 2 {
		t.Errorf("LAN sent %d times across two ticks, want 2 (resend on no ack)", got)
	}

	// After a PSNP ack clears SRM, the next tick sends nothing.
	d.ClearSRM(Level1, id, cLAN)
	rec.reset()
	f.FloodTick()
	if got := rec.count(); got != 0 {
		t.Errorf("sent %d after SRM cleared, want 0", got)
	}
}

// TestZeroLifetimePurgeReflood asserts a received zero-lifetime purge with seq >=
// ours is accepted, marked purged, SRM set on other circuits, and not deleted
// (AC-12 receive path; deletion is isis-6's grace-period job).
func TestZeroLifetimePurgeReflood(t *testing.T) {
	d := New(nil)
	const cIn, cOut CircuitID = 1, 2
	f := NewFlooder(d, nil, staticCircuits(l1l2Circuit("in", cIn), l1l2Circuit("out", cOut)))

	id := lspID(22, 0)
	// We hold seq 5, lifetime 1000.
	stored, storedRaw := buildLSP(t, packet.PDUTypeL2LSP, id, 5, 1000, nil)
	d.Insert(Level2, stored, storedRaw)

	// A purge arrives at a HIGHER sequence (6, lifetime 0).
	purge, purgeRaw := buildLSP(t, packet.PDUTypeL2LSP, id, 6, 0, nil)
	f.ReceiveLSP(cIn, false, purge, purgeRaw)

	entry := d.Lookup(Level2, id)
	if entry == nil {
		t.Fatal("purged entry deleted on receipt; must be retained for the grace period (AC-12)")
	}
	if !entry.IsPurged() {
		t.Error("entry not marked purged after a zero-lifetime LSP (AC-12)")
	}
	if entry.Sequence() != 6 {
		t.Errorf("purge sequence not stored: got %d, want 6", entry.Sequence())
	}
	if !d.SRM(Level2, id, cOut) {
		t.Error("purge not re-flooded (SRM) on the other circuit (AC-12)")
	}
	if d.SRM(Level2, id, cIn) {
		t.Error("purge re-flooded on the incoming circuit")
	}
}

// TestLevelIsolation asserts L1 and L2 SRM/SSN flags and pending-request sets do
// not cross when the same LSP ID is active at both levels on one circuit (AC-14).
func TestLevelIsolation(t *testing.T) {
	d := New(nil)
	const cIn, cOut CircuitID = 1, 2
	f := NewFlooder(d, nil, staticCircuits(l1l2Circuit("in", cIn), l1l2Circuit("out", cOut)))

	id := lspID(23, 0)
	// An L1 LSP arrives newer; only L1 flags must change.
	l1, l1raw := buildLSP(t, packet.PDUTypeL1LSP, id, 4, 1000, nil)
	f.ReceiveLSP(cIn, false, l1, l1raw)

	if !d.SRM(Level1, id, cOut) {
		t.Error("L1 SRM not set on the other circuit")
	}
	if d.SRM(Level2, id, cOut) {
		t.Error("L2 SRM set by an L1 LSP (level cross-contamination, AC-14)")
	}
	if !d.SSN(Level1, id, cIn) || d.SSN(Level2, id, cIn) {
		t.Error("SSN crossed levels (AC-14)")
	}

	// Pending-request sets are per level too: record an L1 pending and assert L2
	// is unaffected.
	f.recordPending(cIn, Level1, lspID(24, 0), pendingReq{seq: 1})
	if f.PendingCount(cIn, Level1) != 1 {
		t.Error("L1 pending not recorded")
	}
	if f.PendingCount(cIn, Level2) != 0 {
		t.Error("L2 pending count nonzero after only an L1 pending (AC-14)")
	}
}

// TestReceiveLSPReturnsFreshness asserts ReceiveLSP surfaces the LSDB
// freshness/store outcome so the engine wiring can decide whether the receive
// changed the topology (only Newer/Stored) before emitting an LSP-change event
// and re-running SPF. An Older or Equal LSP must report Stored=false so the
// engine skips the "add"/SPF path (ISO/IEC 10589 clause 7.3.15). Regression for
// finding B2-2: handleLSP emitted "add" and triggered SPF on EVERY received LSP,
// including Older/Equal duplicates.
func TestReceiveLSPReturnsFreshness(t *testing.T) {
	d := New(nil)
	const cid CircuitID = 1
	f := NewFlooder(d, nil, staticCircuits(l1l2Circuit("c", cid)))
	id := lspID(50, 0)

	// First sighting: Newer + Stored.
	first, firstRaw := buildLSP(t, packet.PDUTypeL1LSP, id, 5, 1000, nil)
	if r := f.ReceiveLSP(cid, false, first, firstRaw); r.Freshness != Newer || !r.Stored {
		t.Errorf("first sighting: got %+v, want Newer+Stored", r)
	}

	// Higher sequence: Newer + Stored.
	hi, hiRaw := buildLSP(t, packet.PDUTypeL1LSP, id, 9, 1000, nil)
	if r := f.ReceiveLSP(cid, false, hi, hiRaw); r.Freshness != Newer || !r.Stored {
		t.Errorf("higher seq: got %+v, want Newer+Stored", r)
	}

	// Same seq, same bytes (duplicate): Equal + NOT Stored (only lifetime refresh).
	if r := f.ReceiveLSP(cid, false, hi, hiRaw); r.Freshness != Equal || r.Stored {
		t.Errorf("duplicate: got %+v, want Equal+!Stored", r)
	}

	// Lower sequence: Older + NOT Stored (held copy kept).
	old, oldRaw := buildLSP(t, packet.PDUTypeL1LSP, id, 4, 1000, nil)
	if r := f.ReceiveLSP(cid, false, old, oldRaw); r.Freshness != Older || r.Stored {
		t.Errorf("lower seq: got %+v, want Older+!Stored", r)
	}

	// A wrong-PDU-type LSP reports Older+!Stored (dropped, no topology change).
	bad := &packet.LSP{PDUType: packet.PDUTypeP2PHello, LSPID: id, SequenceNumber: 1, RemainingLifetime: 1000}
	if r := f.ReceiveLSP(cid, false, bad, nil); r.Stored {
		t.Errorf("wrong-pdu-type: got %+v, want !Stored", r)
	}
}

// distinctLSPID builds the n-th distinct LSP-ID (fragment 0) by spreading n
// across the low two System-ID octets, so a test can mint MaxLSPsPerLevel unique
// IDs without collision.
func distinctLSPID(n int) types.LSPID {
	sys := types.SystemID{0, 0, 0, 0, byte(n >> 8), byte(n)}
	return types.NewLSPID(types.NewSourceID(sys, 0), 0)
}

// TestLSDBFullDropPath fills a level to MaxLSPsPerLevel and then feeds a
// brand-new LSP-ID through the full receive path (ReceiveLSP). Because the level
// is full, the first sighting of a new LSP-ID is rejected (lsdb-full): it MUST be
// dropped (not stored), ze_isis_lsps_dropped_total{level,reason="lsdb-full"} must
// increment, and NEITHER SRM (flood onward) NOR SSN (acknowledge) may be set --
// there is no entry to flag and the LSP was not accepted (ISO/IEC 10589 resource-
// exhaustion guard, isis-6 MaxLSPsPerLevel). Regression for finding B2-6: the
// lsdb-full path must drop cleanly and stay observable.
func TestLSDBFullDropPath(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	d := New(nil)
	const cIn, cOut CircuitID = 1, 2
	rec := &recordingTx{}
	f := NewFlooder(d, rec.tx, staticCircuits(
		l1l2Circuit("in", cIn),
		l1l2Circuit("out", cOut),
	))
	f.SetMetrics(reg)

	// Fill Level1 to exactly MaxLSPsPerLevel with distinct LSP-IDs (Insert skips
	// the freshness compare; each is a first sighting that stores).
	for i := range MaxLSPsPerLevel {
		id := distinctLSPID(i)
		lsp, raw := buildLSP(t, packet.PDUTypeL1LSP, id, 1, 1000, nil)
		d.Insert(Level1, lsp, raw)
	}
	if got := d.Len(Level1); got != MaxLSPsPerLevel {
		t.Fatalf("setup: Level1 has %d entries, want %d (full)", got, MaxLSPsPerLevel)
	}

	// A brand-new LSP-ID arrives on cIn while the level is full: first sighting is
	// rejected (treated as Older so isis-7 does not flood it).
	newID := distinctLSPID(MaxLSPsPerLevel + 1)
	if d.Lookup(Level1, newID) != nil {
		t.Fatalf("setup: the new LSP-ID was already present")
	}
	newLSP, newRaw := buildLSP(t, packet.PDUTypeL1LSP, newID, 9, 1000, nil)
	res := f.ReceiveLSP(cIn, false, newLSP, newRaw)

	// Not stored: the database stayed full and the new ID is absent.
	if res.Stored {
		t.Error("a first-sight LSP on a full level reported Stored (must be dropped)")
	}
	if d.Lookup(Level1, newID) != nil {
		t.Error("a first-sight LSP on a full level was stored (lsdb-full must drop it)")
	}
	if got := d.Len(Level1); got != MaxLSPsPerLevel {
		t.Errorf("Level1 grew to %d past the cap %d", got, MaxLSPsPerLevel)
	}

	// Not flooded and not acknowledged: neither SRM nor SSN is set for the dropped
	// ID on any circuit (no entry exists to flag, and the LSP was not accepted).
	for _, cid := range []CircuitID{cIn, cOut} {
		if d.SRM(Level1, newID, cid) {
			t.Errorf("SRM set for a dropped lsdb-full LSP on circuit %d (must not flood)", cid)
		}
		if d.SSN(Level1, newID, cid) {
			t.Errorf("SSN set for a dropped lsdb-full LSP on circuit %d (must not acknowledge)", cid)
		}
	}
	// And nothing was transmitted as a result of the drop.
	if got := rec.count(); got != 0 {
		t.Errorf("the dropped lsdb-full LSP caused %d transmissions, want 0", got)
	}

	// The drop is observable on the canonical counter with reason="lsdb-full".
	out := scrape(t, reg)
	if !strings.Contains(out, `ze_isis_lsps_dropped_total{level="l1",reason="lsdb-full"}`) {
		t.Errorf("lsdb-full drop not counted on ze_isis_lsps_dropped_total{reason=\"lsdb-full\"}:\n%s", out)
	}
}

// TestSRMResendCountsOnlySecondSend asserts ze_isis_srm_resends_total is bumped
// only on the 2nd-and-later unacknowledged sends, NOT on the first flood of a
// freshly armed LSP (ISO/IEC 10589 clause 7.3.15.1: periodic retransmission while
// SRM remains set). Regression for finding B2-4: the first send must not be
// miscounted as a resend.
func TestSRMResendCountsOnlySecondSend(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	d := New(nil)
	rec := &recordingTx{}
	const cLAN CircuitID = 1
	f := NewFlooder(d, rec.tx, staticCircuits(l1l2Circuit("lan", cLAN)))
	f.SetMetrics(reg)

	id := lspID(40, 0)
	lsp, raw := buildLSP(t, packet.PDUTypeL1LSP, id, 3, 1000, nil)
	d.Insert(Level1, lsp, raw)
	d.SetSRM(Level1, id, cLAN)

	// First tick: the LSP is transmitted, but this is the FIRST send since SRM was
	// armed -> NOT a resend. transmitted counts every send (including the first);
	// srm_resends counts only re-sends, so it stays 0 here.
	f.FloodTick()
	if got := len(rec.onCircuit("lan")); got != 1 {
		t.Fatalf("first tick sent %d, want 1", got)
	}
	if c := counterValue(t, reg, `ze_isis_lsps_transmitted_total{level="l1"}`); c != 1 {
		t.Errorf("transmitted = %v after the first send, want 1 (every send is counted)", c)
	}
	// srm_resends has never incremented (the first send is not a resend), so its
	// labeled series is absent from the scrape -- semantically 0.
	if c := counterValueOrZero(t, reg, `ze_isis_srm_resends_total{level="l1"}`); c != 0 {
		t.Errorf("srm_resends = %v after the first send, want 0 (first send is not a resend)", c)
	}

	// Second tick: SRM is still set (no ack) -> this IS a resend, counted once.
	f.FloodTick()
	if c := counterValue(t, reg, `ze_isis_srm_resends_total{level="l1"}`); c != 1 {
		t.Errorf("srm_resends = %v after the second send, want 1", c)
	}

	// Third tick: still unacknowledged -> a second resend.
	f.FloodTick()
	if c := counterValue(t, reg, `ze_isis_srm_resends_total{level="l1"}`); c != 2 {
		t.Errorf("srm_resends = %v after the third send, want 2", c)
	}

	// Re-arming SRM (e.g. a newer version) resets the per-circuit marker, so the
	// next send is again a FIRST send, not a resend.
	d.SetSRM(Level1, id, cLAN)
	f.FloodTick()
	if c := counterValue(t, reg, `ze_isis_srm_resends_total{level="l1"}`); c != 2 {
		t.Errorf("srm_resends = %v after re-arming SRM and one send, want 2 (re-arm resets resend tracking)", c)
	}
	// Four sends total occurred; transmitted counts them all even though only two
	// were re-sends.
	if c := counterValue(t, reg, `ze_isis_lsps_transmitted_total{level="l1"}`); c != 4 {
		t.Errorf("transmitted = %v after four sends, want 4", c)
	}
}

// counterValue scrapes reg and returns the value on the metric line whose
// "name{labels}" prefix matches series. It fails if the series is absent (use it
// only where the series MUST have been observed). A CounterVec exposes a labeled
// series only once a combo is observed; counterValueOrZero handles the "never
// incremented => 0" case.
func counterValue(t *testing.T, reg *metrics.PrometheusRegistry, series string) float64 {
	t.Helper()
	v, ok := lookupCounter(t, reg, series)
	if !ok {
		t.Fatalf("series %q not found in scrape", series)
	}
	return v
}

// counterValueOrZero returns the series value, or 0 when the series is absent (a
// CounterVec combo that has never been incremented does not appear in the scrape;
// for a counter, absent is semantically 0).
func counterValueOrZero(t *testing.T, reg *metrics.PrometheusRegistry, series string) float64 {
	t.Helper()
	v, _ := lookupCounter(t, reg, series)
	return v
}

// lookupCounter scrapes reg and returns (value, found) for the metric line whose
// "name{labels}" prefix matches series.
func lookupCounter(t *testing.T, reg *metrics.PrometheusRegistry, series string) (float64, bool) {
	t.Helper()
	for line := range strings.SplitSeq(scrape(t, reg), "\n") {
		if rest, ok := strings.CutPrefix(line, series+" "); ok {
			v, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
			if err != nil {
				t.Fatalf("parsing %q: %v", line, err)
			}
			return v, true
		}
	}
	return 0, false
}
