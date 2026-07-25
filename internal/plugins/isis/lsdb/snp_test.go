// Design: plan/spec-isis-7-flooding.md -- CSNP/PSNP build, receive, and pending-request tests.
//
// VALIDATES: (lsdb-package unit level, fake tx + circuit set)
//   - CSNP build carries a TLV 9 entry per LSP over the start/end range and splits
//     into multiple PDUs when the DB exceeds one CSNP (TestCSNPBuildRange, AC-13);
//   - CSNP receive: neighbor-newer-held sets SSN + pending; older sets SRM; equal
//     clears SRM and pending (TestCSNPGapDetection, AC-7/AC-8/AC-13);
//   - a CSNP listing an LSP we do NOT hold records a pending-request (no SSN),
//     a later PSNP carries the request, and the request clears when the LSP
//     arrives (TestCSNPGapRequestPending, AC-15);
//   - PSNP build emits SSN acks (clearing SSN) + pending requests; PSNP receive
//     clears SRM on an ack at our seq and sets SRM on a request
//     (TestPSNPRequestAndAck, AC-9/AC-10);
//   - the P2P initial CSNP fires only on a P2P circuit (TestInitialCSNPP2POnly,
//     AC-11).

package lsdb

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// ownSrc is the test node's own Source ID (System ID byte 1, pseudonode 0).
func ownSrc() types.SourceID { return types.NewSourceID(testSys(1), 0) }

// decodeCSNP parses a built CSNP PDU back into the typed form for assertions.
func decodeCSNP(t *testing.T, pdu []byte) *packet.CSNP {
	t.Helper()
	out, err := packet.DecodePDU(pdu)
	if err != nil || out.CSNP == nil {
		t.Fatalf("decodeCSNP: %v (csnp=%v)", err, out.CSNP)
	}
	return out.CSNP
}

// decodePSNP parses a built PSNP PDU back into the typed form for assertions.
func decodePSNP(t *testing.T, pdu []byte) *packet.PSNP {
	t.Helper()
	out, err := packet.DecodePDU(pdu)
	if err != nil || out.PSNP == nil {
		t.Fatalf("decodePSNP: %v (psnp=%v)", err, out.PSNP)
	}
	return out.PSNP
}

// csnpEntries flattens the TLV 9 entries across all CSNP PDUs.
func csnpEntries(t *testing.T, pdus [][]byte) []packet.LSPEntry {
	t.Helper()
	var out []packet.LSPEntry
	for _, pdu := range pdus {
		out = append(out, decodeLSPEntries(decodeCSNP(t, pdu).TLVs)...)
	}
	return out
}

// buildIncomingCSNP builds a CSNP PDU (as if received) listing the given entries
// over the whole LSP-ID range, sourced from src.
func buildIncomingCSNP(t *testing.T, level Level, src types.SourceID, entries []packet.LSPEntry) *packet.CSNP {
	t.Helper()
	pt := packet.PDUTypeL1CSNP
	if level == Level2 {
		pt = packet.PDUTypeL2CSNP
	}
	c := packet.CSNP{
		PDUType:    pt,
		SourceID:   src,
		StartLSPID: minLSPID(),
		EndLSPID:   maxLSPID(),
		TLVs:       []packet.TLV{lspEntriesTLV(entries)},
	}
	buf := make([]byte, c.EncodedLen())
	n := c.WriteTo(buf, 0)
	out, err := packet.DecodePDU(buf[:n])
	if err != nil {
		t.Fatalf("buildIncomingCSNP decode: %v", err)
	}
	return out.CSNP
}

// buildIncomingPSNP builds a PSNP PDU (as if received) carrying the given entries.
func buildIncomingPSNP(t *testing.T, level Level, src types.SourceID, entries []packet.LSPEntry) *packet.PSNP {
	t.Helper()
	pt := packet.PDUTypeL1PSNP
	if level == Level2 {
		pt = packet.PDUTypeL2PSNP
	}
	p := packet.PSNP{PDUType: pt, SourceID: src, TLVs: []packet.TLV{lspEntriesTLV(entries)}}
	buf := make([]byte, p.EncodedLen())
	n := p.WriteTo(buf, 0)
	out, err := packet.DecodePDU(buf[:n])
	if err != nil {
		t.Fatalf("buildIncomingPSNP decode: %v", err)
	}
	return out.PSNP
}

// TestCSNPBuildRange asserts a CSNP carries a TLV 9 entry per LSP over the
// start/end range, and a DB exceeding one CSNP splits into ordered PDUs whose
// ranges tile the space (AC-13, A-4).
func TestCSNPBuildRange(t *testing.T) {
	d := New(nil)
	f := NewFlooder(d, nil, nil)

	// Single CSNP case: 3 L2 LSPs fit one PDU. The range must span the whole space.
	for _, frag := range []byte{0, 1, 2} {
		id := types.NewLSPID(types.NewSourceID(testSys(1), 0), frag)
		lsp, raw := buildLSP(t, packet.PDUTypeL2LSP, id, types.SequenceNumber(frag+1), 1000, nil)
		d.Insert(Level2, lsp, raw)
	}
	pdus := f.buildCSNPs(Level2, ownSrc())
	if len(pdus) != 1 {
		t.Fatalf("3 LSPs produced %d CSNP PDUs, want 1", len(pdus))
	}
	c := decodeCSNP(t, pdus[0])
	if c.StartLSPID != minLSPID() || c.EndLSPID != maxLSPID() {
		t.Errorf("single CSNP range = [%s..%s], want whole space", c.StartLSPID, c.EndLSPID)
	}
	if got := len(csnpEntries(t, pdus)); got != 3 {
		t.Errorf("CSNP carried %d entries, want 3", got)
	}

	// Multi-PDU case: insert more than maxLSPEntriesPerSNP (15) LSPs so the build
	// splits. Use distinct System IDs so the LSP IDs are well-ordered.
	d2 := New(nil)
	f2 := NewFlooder(d2, nil, nil)
	const total = maxLSPEntriesPerSNP + 5
	for i := range total {
		id := types.NewLSPID(types.NewSourceID(testSys(byte(i+1)), 0), 0)
		lsp, raw := buildLSP(t, packet.PDUTypeL1LSP, id, 1, 1000, nil)
		d2.Insert(Level1, lsp, raw)
	}
	pdus2 := f2.buildCSNPs(Level1, ownSrc())
	if len(pdus2) < 2 {
		t.Fatalf("%d LSPs produced %d CSNP PDUs, want >= 2 (split)", total, len(pdus2))
	}
	// Every LSP appears exactly once across the split PDUs.
	if got := len(csnpEntries(t, pdus2)); got != total {
		t.Errorf("split CSNPs carried %d entries total, want %d", got, total)
	}
	// The first PDU starts at the whole-space minimum; the last ends at the max.
	first := decodeCSNP(t, pdus2[0])
	last := decodeCSNP(t, pdus2[len(pdus2)-1])
	if first.StartLSPID != minLSPID() {
		t.Errorf("first split CSNP start = %s, want whole-space min", first.StartLSPID)
	}
	if last.EndLSPID != maxLSPID() {
		t.Errorf("last split CSNP end = %s, want whole-space max", last.EndLSPID)
	}

	// Empty DB: one CSNP, whole range, no entries (tells the neighbor to send all).
	d3 := New(nil)
	f3 := NewFlooder(d3, nil, nil)
	empty := f3.buildCSNPs(Level1, ownSrc())
	if len(empty) != 1 || len(csnpEntries(t, empty)) != 0 {
		t.Errorf("empty DB CSNP = %d PDUs / %d entries, want 1/0", len(empty), len(csnpEntries(t, empty)))
	}
}

// TestCSNPGapDetection asserts the three held-entry CSNP reconcile outcomes:
// neighbor-newer (SSN + pending), we-newer (SRM), equal (clear SRM + clear
// pending). AC-7, AC-8, AC-13.
func TestCSNPGapDetection(t *testing.T) {
	d := New(nil)
	const cid CircuitID = 1
	f := NewFlooder(d, nil, staticCircuits(l1l2Circuit("c", cid)))

	idNewer := lspID(10, 0) // neighbor advertises a higher seq than we hold
	idOlder := lspID(11, 0) // neighbor advertises a lower seq than we hold
	idEqual := lspID(12, 0) // neighbor advertises our exact seq

	for _, id := range []types.LSPID{idNewer, idOlder, idEqual} {
		lsp, raw := buildLSP(t, packet.PDUTypeL2LSP, id, 5, 1000, nil)
		d.Insert(Level2, lsp, raw)
	}
	// Pre-arm SRM on idEqual so the equal-entry clear is observable.
	d.SetSRM(Level2, idEqual, cid)

	csnp := buildIncomingCSNP(t, Level2, neighborSrc(), []packet.LSPEntry{
		{LSPID: idNewer, SequenceNumber: 9, RemainingLifetime: 1000},
		{LSPID: idOlder, SequenceNumber: 2, RemainingLifetime: 1000},
		{LSPID: idEqual, SequenceNumber: 5, RemainingLifetime: 1000},
	})
	f.ReceiveCSNP(cid, csnp)

	// Neighbor-newer: SSN on the held stale entry AND a pending-request recorded.
	if !d.SSN(Level2, idNewer, cid) {
		t.Error("neighbor-newer CSNP entry did not set SSN on the held entry (AC-7)")
	}
	if f.PendingCount(cid, Level2) != 1 {
		t.Errorf("neighbor-newer CSNP entry did not record a pending-request (AC-7): pending=%d", f.PendingCount(cid, Level2))
	}
	// We-newer: SRM set to send our copy.
	if !d.SRM(Level2, idOlder, cid) {
		t.Error("we-newer CSNP entry did not set SRM to send our copy (AC-8)")
	}
	// Equal: SRM cleared (implicit ack).
	if d.SRM(Level2, idEqual, cid) {
		t.Error("equal CSNP entry did not clear SRM (AC-13)")
	}
}

// TestCSNPGapRequestPending asserts a CSNP listing an LSP we do NOT hold records
// a per-circuit pending-request (no SSN, since no LSDB entry exists); a later
// PSNP carries the request; and the request clears when the LSP arrives (AC-15).
func TestCSNPGapRequestPending(t *testing.T) {
	d := New(nil)
	const cid CircuitID = 1
	f := NewFlooder(d, nil, staticCircuits(l1l2Circuit("c", cid)))

	missing := lspID(20, 0) // we hold nothing for this LSP ID
	csnp := buildIncomingCSNP(t, Level1, neighborSrc(), []packet.LSPEntry{
		{LSPID: missing, SequenceNumber: 7, RemainingLifetime: 1000, Checksum: 0xabcd},
	})
	f.ReceiveCSNP(cid, csnp)

	// No LSDB entry exists, so SSN cannot be set; a pending-request is recorded.
	if d.Lookup(Level1, missing) != nil {
		t.Fatal("a not-held LSP must not create an LSDB entry")
	}
	if d.SSN(Level1, missing, cid) {
		t.Error("SSN set for an LSP with no LSDB entry (impossible by design, AC-15)")
	}
	if f.PendingCount(cid, Level1) != 1 {
		t.Fatalf("missing LSP not recorded as pending (AC-15): pending=%d", f.PendingCount(cid, Level1))
	}

	// A PSNP built for this circuit carries a TLV 9 request for the missing LSP.
	// The request uses the standard "send me this LSP" form -- sequence 0 (NOT the
	// sequence advertised in the CSNP). Echoing the advertised sequence would make
	// the request indistinguishable from an acknowledgement at the holder's current
	// sequence, so the holder would clear SRM and never supply the LSP (ISO/IEC
	// 10589 clause 7.3.15.3, AC-15).
	psnps := f.buildPSNP(cid, Level1, ownSrc())
	if len(psnps) == 0 {
		t.Fatal("no PSNP built despite a pending request (AC-15)")
	}
	var requested bool
	for _, p := range psnps {
		for _, e := range decodeLSPEntries(decodePSNP(t, p).TLVs) {
			if e.LSPID == missing {
				if e.SequenceNumber != 0 {
					t.Errorf("PSNP request carried seq %d, want 0 (a request must not echo the advertised seq, AC-15)", e.SequenceNumber)
				}
				requested = true
			}
		}
	}
	if !requested {
		t.Error("PSNP did not request the pending (not-held) LSP (AC-15)")
	}
	// The request is still pending until the LSP arrives.
	if f.PendingCount(cid, Level1) != 1 {
		t.Error("pending-request cleared before the LSP arrived")
	}

	// When the requested LSP arrives and is stored, the pending entry clears.
	lsp, raw := buildLSP(t, packet.PDUTypeL1LSP, missing, 7, 1000, nil)
	f.ReceiveLSP(cid, false, lsp, raw)
	if f.PendingCount(cid, Level1) != 0 {
		t.Errorf("pending-request not cleared after the LSP arrived and was stored (AC-15): pending=%d", f.PendingCount(cid, Level1))
	}
}

// TestPSNPRequestAndAck asserts PSNP build emits SSN-acks (clearing SSN) and
// pending-requests, and PSNP receive clears SRM on an ack at our seq (AC-9) and
// sets SRM on a request/older entry (AC-10).
func TestPSNPRequestAndAck(t *testing.T) {
	d := New(nil)
	const cid CircuitID = 1
	f := NewFlooder(d, nil, staticCircuits(l1l2Circuit("c", cid)))

	// --- PSNP build: an SSN-set held LSP becomes an ack entry; SSN is cleared. ---
	ack := lspID(30, 0)
	ackLSP, ackRaw := buildLSP(t, packet.PDUTypeL2LSP, ack, 5, 1000, nil)
	d.Insert(Level2, ackLSP, ackRaw)
	d.SetSSN(Level2, ack, cid)

	pdus := f.buildPSNP(cid, Level2, ownSrc())
	if len(pdus) == 0 {
		t.Fatal("no PSNP built for an SSN-set LSP")
	}
	var acked bool
	for _, p := range pdus {
		for _, e := range decodeLSPEntries(decodePSNP(t, p).TLVs) {
			if e.LSPID == ack && e.SequenceNumber == 5 {
				acked = true
			}
		}
	}
	if !acked {
		t.Error("PSNP did not carry the SSN-set LSP as an ack entry")
	}
	if d.SSN(Level2, ack, cid) {
		t.Error("SSN not cleared after the PSNP that acknowledges the LSP was built (clause 7.3.16)")
	}

	// --- PSNP receive: an ack at our seq clears SRM (AC-9). ---
	supplied := lspID(31, 0)
	supLSP, supRaw := buildLSP(t, packet.PDUTypeL2LSP, supplied, 8, 1000, nil)
	d.Insert(Level2, supLSP, supRaw)
	d.SetSRM(Level2, supplied, cid) // we owe this LSP on the circuit
	ackPSNP := buildIncomingPSNP(t, Level2, neighborSrc(), []packet.LSPEntry{
		{LSPID: supplied, SequenceNumber: 8, RemainingLifetime: 1000},
	})
	f.ReceivePSNP(cid, ackPSNP)
	if d.SRM(Level2, supplied, cid) {
		t.Error("PSNP ack at our sequence did not clear SRM (AC-9)")
	}

	// --- PSNP receive: a request (the neighbor lists an older/zero seq) sets SRM
	// so we supply our copy (AC-10). ---
	wanted := lspID(32, 0)
	wantLSP, wantRaw := buildLSP(t, packet.PDUTypeL2LSP, wanted, 6, 1000, nil)
	d.Insert(Level2, wantLSP, wantRaw)
	reqPSNP := buildIncomingPSNP(t, Level2, neighborSrc(), []packet.LSPEntry{
		{LSPID: wanted, SequenceNumber: 0, RemainingLifetime: 0}, // "send me this LSP"
	})
	f.ReceivePSNP(cid, reqPSNP)
	if !d.SRM(Level2, wanted, cid) {
		t.Error("PSNP request did not set SRM to supply the LSP (AC-10)")
	}
}

// TestInitialCSNPP2POnly asserts the initial CSNP fires only on a P2P circuit
// (AC-11). A broadcast circuit's initial CSNP is a no-op (LAN cadence is DIS
// policy, isis-8).
func TestInitialCSNPP2POnly(t *testing.T) {
	d := New(nil)
	// Put one LSP in so a CSNP would carry an entry.
	id := lspID(40, 0)
	lsp, raw := buildLSP(t, packet.PDUTypeL2LSP, id, 1, 1000, nil)
	d.Insert(Level2, lsp, raw)

	recP2P := &recordingTx{}
	fP2P := NewFlooder(d, recP2P.tx, nil)
	fP2P.InitialCSNP(p2pCircuit("p2p", 1), Level2, ownSrc())
	if recP2P.count() == 0 {
		t.Error("P2P initial CSNP sent nothing (AC-11)")
	}
	// A broadcast circuit: InitialCSNP is a no-op.
	recLAN := &recordingTx{}
	fLAN := NewFlooder(d, recLAN.tx, nil)
	fLAN.InitialCSNP(l1l2Circuit("lan", 2), Level2, ownSrc())
	if recLAN.count() != 0 {
		t.Errorf("broadcast initial CSNP sent %d PDUs, want 0 (LAN cadence is DIS policy)", recLAN.count())
	}
}

// TestCSNPPurgeVsPurgeEqual asserts that when OUR held entry is a purge
// (Remaining Lifetime 0) and the neighbor's CSNP entry for the same LSP ID at the
// same sequence is ALSO a purge, compareSNPEntry treats them as equal (returns 0)
// and ReceiveCSNP takes the implicit-ack path: it clears SRM on the circuit and
// clears any pending-request for that LSP. ISO/IEC 10589 clause 7.3.15.2: a CSNP
// entry that matches our copy (here, both are the same purge) acknowledges it, so
// there is nothing left to send (SRM cleared) and nothing left to request.
func TestCSNPPurgeVsPurgeEqual(t *testing.T) {
	d := New(nil)
	const cid CircuitID = 1
	f := NewFlooder(d, nil, staticCircuits(l1l2Circuit("c", cid)))

	id := lspID(50, 0)
	// Hold a PURGE: an LSP at seq 5 with Remaining Lifetime 0 (replaceLocked marks
	// it purged). This is our held copy.
	lsp, raw := buildLSP(t, packet.PDUTypeL2LSP, id, 5, 0, nil)
	d.Insert(Level2, lsp, raw)
	held := d.Lookup(Level2, id)
	if held == nil || !held.IsPurged() {
		t.Fatalf("held entry not in purged state: %+v", held)
	}

	// Unit check: an SNP entry that is also a purge at the same sequence compares
	// EQUAL (0), not newer/older -- the purge tiebreak only fires when exactly one
	// side is a purge.
	neighborEntry := packet.LSPEntry{LSPID: id, SequenceNumber: 5, RemainingLifetime: 0}
	if cmp := compareSNPEntry(neighborEntry, held); cmp != 0 {
		t.Fatalf("compareSNPEntry(purge, held-purge) = %d, want 0 (equal)", cmp)
	}

	// Pre-arm SRM and record a pending-request for the same LSP so the equal-entry
	// clear is observable on both: ReceiveCSNP must clear SRM AND drop the pending.
	d.SetSRM(Level2, id, cid)
	f.recordPending(cid, Level2, id, pendingReq{seq: 5})
	if f.PendingCount(cid, Level2) != 1 {
		t.Fatalf("pre-condition: pending not recorded, count=%d", f.PendingCount(cid, Level2))
	}

	csnp := buildIncomingCSNP(t, Level2, neighborSrc(), []packet.LSPEntry{neighborEntry})
	f.ReceiveCSNP(cid, csnp)

	if d.SRM(Level2, id, cid) {
		t.Error("purge-vs-purge equal CSNP entry did not clear SRM (implicit ack)")
	}
	if f.PendingCount(cid, Level2) != 0 {
		t.Errorf("purge-vs-purge equal CSNP entry did not clear the pending-request: count=%d", f.PendingCount(cid, Level2))
	}
}

// neighborSrc is a fixed neighbor Source ID (System ID byte 2, pseudonode 0),
// distinct from the test node's own Source ID (ownSrc).
func neighborSrc() types.SourceID { return types.NewSourceID(testSys(2), 0) }
