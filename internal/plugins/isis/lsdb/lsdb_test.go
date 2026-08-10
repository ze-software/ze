// Design: docs/architecture/isis/isis-6-lsdb.md -- LSDB store/freshness/flags/snapshot tests.
//
// VALIDATES: store/retrieve with per-level isolation (TestISISLSDBStoreRetrieve);
// the freshness compare newer/equal/older (TestISISLSDBReceiveNewer); verbatim
// raw-byte storage of an unknown TLV (TestISISLSDBStoreVerbatim); per-circuit
// SRM/SSN flag ops (TestISISSRMSSNFlagOps); and the database snapshot
// (TestISISLSDBSnapshot). These are the AC-7, AC-8, AC-10 store behaviors.

package lsdb

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// testSys builds a System ID from a single discriminator byte (the last octet).
func testSys(b byte) types.SystemID { return types.SystemID{0, 0, 0, 0, 0, b} }

// lspID builds an LSPID for a System ID byte and fragment number.
func lspID(sysByte, frag byte) types.LSPID {
	return types.NewLSPID(types.NewSourceID(testSys(sysByte), 0), frag)
}

// buildLSP encodes an LSP with the codec and returns the parsed form plus the
// raw bytes, exactly as the receive path would hand them to the LSDB. seq and
// lifetime set the freshness fields; tlvs are the body TLVs.
func buildLSP(t *testing.T, pt packet.PDUType, id types.LSPID, seq types.SequenceNumber, lifetime types.RemainingLifetime, tlvs []packet.TLV) (*packet.LSP, []byte) {
	t.Helper()
	in := &packet.LSP{
		PDUType:           pt,
		RemainingLifetime: lifetime,
		LSPID:             id,
		SequenceNumber:    seq,
		TLVs:              tlvs,
	}
	buf := make([]byte, in.EncodedLen())
	n := in.WriteTo(buf, 0)
	raw := buf[:n]
	pdu, err := packet.DecodePDU(raw)
	if err != nil {
		t.Fatalf("buildLSP: DecodePDU: %v", err)
	}
	return pdu.LSP, raw
}

func TestISISLSDBStoreRetrieve(t *testing.T) {
	d := New(nil)

	// Store an L1 LSP and an L2 LSP with the same LSP ID; the two levels are
	// independent databases (spec: per-level isolation).
	id := lspID(1, 0)
	l1lsp, l1raw := buildLSP(t, packet.PDUTypeL1LSP, id, 5, 1000, nil)
	l2lsp, l2raw := buildLSP(t, packet.PDUTypeL2LSP, id, 9, 1000, nil)

	d.Insert(Level1, l1lsp, l1raw)
	d.Insert(Level2, l2lsp, l2raw)

	if got := d.Lookup(Level1, id); got == nil {
		t.Fatal("L1 lookup returned nil after insert")
	} else if got.Sequence() != 5 {
		t.Errorf("L1 sequence = %d, want 5", got.Sequence())
	}
	if got := d.Lookup(Level2, id); got == nil {
		t.Fatal("L2 lookup returned nil after insert")
	} else if got.Sequence() != 9 {
		t.Errorf("L2 sequence = %d, want 9 (level isolation broken)", got.Sequence())
	}

	if d.Len(Level1) != 1 || d.Len(Level2) != 1 {
		t.Errorf("len L1=%d L2=%d, want 1 each", d.Len(Level1), d.Len(Level2))
	}

	// Retrieve the stored raw bytes byte-for-byte.
	got := d.Lookup(Level1, id)
	if len(got.Raw()) != len(l1raw) {
		t.Fatalf("stored raw len %d != %d", len(got.Raw()), len(l1raw))
	}
	for i := range l1raw {
		if got.Raw()[i] != l1raw[i] {
			t.Fatalf("stored raw differs at byte %d", i)
		}
	}

	// Delete removes only the requested level.
	if !d.Delete(Level1, id) {
		t.Error("Delete(L1) returned false")
	}
	if d.Lookup(Level1, id) != nil {
		t.Error("L1 entry still present after delete")
	}
	if d.Lookup(Level2, id) == nil {
		t.Error("L2 entry wrongly removed by L1 delete")
	}
}

func TestISISLSDBReceiveNewer(t *testing.T) {
	d := New(nil)
	id := lspID(2, 0)

	// First sighting: stored, reported Newer.
	first, firstRaw := buildLSP(t, packet.PDUTypeL2LSP, id, 10, 1000, nil)
	if r := d.Receive(Level2, first, firstRaw, false); !r.Stored || r.Freshness != Newer {
		t.Fatalf("first sighting: got %+v, want Stored Newer", r)
	}

	// A higher sequence replaces (Newer, stored).
	newer, newerRaw := buildLSP(t, packet.PDUTypeL2LSP, id, 11, 800, nil)
	if r := d.Receive(Level2, newer, newerRaw, false); !r.Stored || r.Freshness != Newer {
		t.Fatalf("newer: got %+v, want Stored Newer", r)
	}
	if got := d.Lookup(Level2, id).Sequence(); got != 11 {
		t.Errorf("after newer, stored sequence = %d, want 11", got)
	}

	// An equal sequence (same checksum) is Equal and refreshes the lifetime only.
	equal, equalRaw := buildLSP(t, packet.PDUTypeL2LSP, id, 11, 1200, nil)
	if r := d.Receive(Level2, equal, equalRaw, false); r.Stored || r.Freshness != Equal {
		t.Fatalf("equal: got %+v, want not-Stored Equal", r)
	}
	if got := d.Lookup(Level2, id).Lifetime(); got != 1200 {
		t.Errorf("equal did not refresh lifetime: got %d, want 1200", got)
	}

	// A lower sequence is Older and changes nothing.
	older, olderRaw := buildLSP(t, packet.PDUTypeL2LSP, id, 7, 1200, nil)
	if r := d.Receive(Level2, older, olderRaw, false); r.Stored || r.Freshness != Older {
		t.Fatalf("older: got %+v, want not-Stored Older", r)
	}
	if got := d.Lookup(Level2, id).Sequence(); got != 11 {
		t.Errorf("older wrongly replaced stored entry: sequence = %d, want 11", got)
	}

	// A purge (lifetime 0) at the SAME sequence is Newer (clause 7.3.16.1): the
	// withdrawal must win over a held non-zero-lifetime copy.
	purge, purgeRaw := buildLSP(t, packet.PDUTypeL2LSP, id, 11, 0, nil)
	if r := d.Receive(Level2, purge, purgeRaw, false); !r.Stored || r.Freshness != Newer {
		t.Fatalf("same-seq purge: got %+v, want Stored Newer", r)
	}
	if !d.Lookup(Level2, id).IsPurged() {
		t.Error("entry not marked purged after receiving a purge")
	}
}

func TestISISLSDBStoreVerbatim(t *testing.T) {
	d := New(nil)
	id := lspID(3, 0)

	// An LSP carrying a TLV the node does not understand (type 222) plus a known
	// TLV 135 must be stored and retrievable byte-for-byte (AC-7, clause 7.3.14).
	unknown := packet.TLV{Type: 222, Value: []byte{0xde, 0xad, 0xbe, 0xef}}
	known := ext135TLV(t, netip.MustParsePrefix("10.0.0.0/24"), 10)
	lsp, raw := buildLSP(t, packet.PDUTypeL2LSP, id, 1, 1000, []packet.TLV{unknown, known})

	// Independent copy of the raw bytes BEFORE store, then mutate the original
	// buffer to prove the LSDB took an owned copy (security review: no alias).
	orig := make([]byte, len(raw))
	copy(orig, raw)
	d.Receive(Level2, lsp, raw, false)
	for i := range raw {
		raw[i] = 0xff // scribble the caller's buffer
	}

	got := d.Lookup(Level2, id)
	if got == nil {
		t.Fatal("entry missing after verbatim store")
	}
	stored := got.Raw()
	if len(stored) != len(orig) {
		t.Fatalf("stored len %d != %d", len(stored), len(orig))
	}
	for i := range orig {
		if stored[i] != orig[i] {
			t.Fatalf("stored bytes corrupted at %d (alias of caller buffer?): %#02x != %#02x", i, stored[i], orig[i])
		}
	}

	// The unknown TLV survives a lazy decode in order.
	decoded, err := got.Decode()
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded.TLVs) != 2 || decoded.TLVs[0].Type != 222 {
		t.Fatalf("unknown TLV lost on decode: %+v", decoded.TLVs)
	}
}

func TestISISSRMSSNFlagOps(t *testing.T) {
	d := New(nil)
	id := lspID(4, 0)
	lsp, raw := buildLSP(t, packet.PDUTypeL1LSP, id, 1, 1000, nil)
	d.Insert(Level1, lsp, raw)

	const cA, cB CircuitID = 1, 2

	// SRM and SSN are independent per circuit.
	d.SetSRM(Level1, id, cA)
	d.setSSN(Level1, id, cB)
	if !d.SRM(Level1, id, cA) {
		t.Error("SRM(cA) false after SetSRM(cA)")
	}
	if d.SRM(Level1, id, cB) {
		t.Error("SRM(cB) true but only cA was set (cross-circuit leak)")
	}
	if !d.SSN(Level1, id, cB) {
		t.Error("SSN(cB) false after SetSSN(cB)")
	}
	if d.SSN(Level1, id, cA) {
		t.Error("SSN(cA) true but only cB was set")
	}

	// Clearing one circuit's flag leaves the other.
	d.ClearSRM(Level1, id, cA)
	if d.SRM(Level1, id, cA) {
		t.Error("SRM(cA) still set after ClearSRM(cA)")
	}

	// ClearCircuit drops all of a circuit's flags across the LSP.
	d.SetSRM(Level1, id, cB)
	d.ClearCircuit(cB)
	if d.SRM(Level1, id, cB) || d.SSN(Level1, id, cB) {
		t.Error("ClearCircuit(cB) did not clear cB's flags")
	}

	// Flags on a missing LSP are no-ops, not panics.
	d.SetSRM(Level1, lspID(99, 0), cA)
	if d.SRM(Level1, lspID(99, 0), cA) {
		t.Error("SRM set on a non-existent LSP")
	}

	// A replace (newer version) resets the flags to empty (isis-7 re-arms SRM).
	newer, newerRaw := buildLSP(t, packet.PDUTypeL1LSP, id, 2, 1000, nil)
	d.SetSRM(Level1, id, cA)
	d.Receive(Level1, newer, newerRaw, false)
	if d.SRM(Level1, id, cA) {
		t.Error("SRM survived a newer-version replace; expected reset")
	}
}

func TestISISLSDBSnapshot(t *testing.T) {
	d := New(nil)

	// Two L1 LSPs (one overloaded) and one L2 LSP. The snapshot is per level,
	// sorted by LSP ID, and carries seq/lifetime/checksum/overload (AC-10).
	a, araw := buildLSP(t, packet.PDUTypeL1LSP, lspID(2, 0), 3, 1000, nil)
	b := &packet.LSP{
		PDUType: packet.PDUTypeL1LSP, RemainingLifetime: 900,
		LSPID: lspID(1, 0), SequenceNumber: 7, TypeBlock: packet.LSPFlagOverload | packet.LSPFlagISTypeL1,
	}
	bbuf := make([]byte, b.EncodedLen())
	bn := b.WriteTo(bbuf, 0)
	c, craw := buildLSP(t, packet.PDUTypeL2LSP, lspID(5, 0), 4, 600, nil)

	d.Insert(Level1, a, araw)
	d.Insert(Level1, b, bbuf[:bn])
	d.Insert(Level2, c, craw)

	l1 := d.Snapshot(Level1)
	if len(l1) != 2 {
		t.Fatalf("L1 snapshot has %d rows, want 2", len(l1))
	}
	// Sorted by LSP ID: 0000.0000.0001.00-00 before 0000.0000.0002.00-00.
	if l1[0].LSPID >= l1[1].LSPID {
		t.Errorf("L1 snapshot not sorted by LSP ID: %q then %q", l1[0].LSPID, l1[1].LSPID)
	}
	// The overloaded LSP (sys 1) reports Overload.
	var sawOverload bool
	for _, row := range l1 {
		if row.Sequence == 7 {
			sawOverload = row.Overload
			if row.Checksum == 0 {
				t.Error("snapshot checksum is 0 for an encoded LSP")
			}
		}
	}
	if !sawOverload {
		t.Error("overloaded LSP not reported as Overload in snapshot")
	}

	if len(d.Snapshot(Level2)) != 1 {
		t.Errorf("L2 snapshot has %d rows, want 1", len(d.Snapshot(Level2)))
	}
}

// TestISISLSDBSnapshotOwn proves LSPSnapshot.Own reports which LSPs this node
// originated, which is what `show isis database` marks for the operator (FRR
// prints the same fact as an asterisk beside the LSP ID).
//
// VALIDATES: Snapshot reads Entry.IsOwn per row.
// PREVENTS:  Own hardcoded, dropped from the snapshot literal, or wired to the
//
//	wrong entry -- each of which makes every row claim the same
//	ownership and destroys the answer to "is this one mine?".
//
// The database deliberately holds BOTH kinds at the same level: Insert is the
// origination path (own), Receive with own=false is the wire path (foreign). A
// test over own LSPs alone passes against `Own: true` hardcoded in the snapshot
// literal, so the foreign row is what makes the assertion discriminate.
func TestISISLSDBSnapshotOwn(t *testing.T) {
	d := New(nil)

	mine, mineRaw := buildLSP(t, packet.PDUTypeL1LSP, lspID(1, 0), 3, 1000, nil)
	theirs, theirsRaw := buildLSP(t, packet.PDUTypeL1LSP, lspID(9, 0), 4, 1000, nil)

	d.Insert(Level1, mine, mineRaw)
	if r := d.Receive(Level1, theirs, theirsRaw, false); !r.Stored {
		t.Fatalf("foreign LSP not stored: %+v", r)
	}

	want := map[string]bool{
		lspID(1, 0).String(): true,
		lspID(9, 0).String(): false,
	}
	rows := d.Snapshot(Level1)
	if len(rows) != len(want) {
		t.Fatalf("L1 snapshot has %d rows, want %d", len(rows), len(want))
	}
	for _, row := range rows {
		expect, known := want[row.LSPID]
		if !known {
			t.Fatalf("unexpected snapshot row %q", row.LSPID)
		}
		if row.Own != expect {
			t.Errorf("snapshot row %q: Own = %v, want %v", row.LSPID, row.Own, expect)
		}
	}
}

// ext135TLV builds an opaque TLV 135 (Extended IP Reachability) carrying one
// prefix, for use as an LSP body TLV in tests.
func ext135TLV(t *testing.T, prefix netip.Prefix, metric uint32) packet.TLV {
	t.Helper()
	in := packet.ExtendedIPReachTLV{Entries: []packet.ExtIPReachEntry{{
		Metric: types.NewPrefixMetric(metric),
		Prefix: prefix,
	}}}
	buf := make([]byte, in.EncodedLen())
	n := in.WriteTo(buf, 0)
	it := packet.NewTLVIterator(buf[:n])
	_, value, _ := it.Next()
	cp := make([]byte, len(value))
	copy(cp, value)
	return packet.TLV{Type: packet.TLVExtendedIPReach, Value: cp}
}
