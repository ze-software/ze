// Design: docs/architecture/isis/isis-7-flooding.md -- RFC 1195 Annex B ordering of
// the LSP Entries carried by a CSNP and a PSNP.
//
// Goal: prove that every Sequence Numbers PDU this node builds lists its LSP
// entries in ascending LSP ID order, with the LSP number octet as the least
// significant octet. Method: feed the LSDB and the flooder's request and
// acknowledgement lists out of order, build the PDUs, decode them, and compare
// each entry against its predecessor.

package lsdb

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// assertAscendingLSPIDs fails when any entry does not sort strictly after its
// predecessor, and returns the entries so the caller can check the count.
func assertAscendingLSPIDs(t *testing.T, pdu string, entries []packet.LSPEntry) {
	t.Helper()
	for i := 1; i < len(entries); i++ {
		if !entries[i-1].LSPID.Less(entries[i].LSPID) {
			t.Fatalf("%s entry %d %s does not sort after entry %d %s",
				pdu, i, entries[i].LSPID, i-1, entries[i-1].LSPID)
		}
	}
}

// RFC requirement: RFC1195-7-1 positive -- a CSNP built from LSPs inserted in
// descending order lists its TLV 9 entries in ascending LSP ID order, and
// fragment 0 of one system precedes fragment 1 of the same system (the LSP
// number octet is the least significant octet).
func TestRFC1195CSNPEntriesAscending(t *testing.T) {
	d := New(nil)
	f := NewFlooder(d, nil, staticCircuits(l1l2Circuit("c", 1)))

	// Insert in the reverse of the wire order the CSNP must show.
	inserted := []types.LSPID{lspID(40, 0), lspID(20, 1), lspID(20, 0), lspID(30, 0)}
	for _, id := range inserted {
		lsp, raw := buildLSP(t, packet.PDUTypeL1LSP, id, 1, 1000, nil)
		d.Insert(Level1, lsp, raw)
	}

	entries := csnpEntries(t, f.buildCSNPs(Level1, ownSrc()))
	if len(entries) != len(inserted) {
		t.Fatalf("CSNP carries %d entries, want %d", len(entries), len(inserted))
	}
	assertAscendingLSPIDs(t, "CSNP", entries)
	want := []types.LSPID{lspID(20, 0), lspID(20, 1), lspID(30, 0), lspID(40, 0)}
	for i, id := range want {
		if entries[i].LSPID != id {
			t.Fatalf("CSNP entry %d = %s, want %s", i, entries[i].LSPID, id)
		}
	}
}

// RFC requirement: RFC1195-7-1 negative -- a PSNP that carries an acknowledgement,
// a request and an ack-only entry never lists them in list order (ack, request,
// ack-only) when that order is not ascending: the decoded entries are in
// ascending LSP ID order regardless of which list each came from.
func TestRFC1195PSNPEntriesAscendingAcrossLists(t *testing.T) {
	d := New(nil)
	const cid CircuitID = 1
	f := NewFlooder(d, nil, staticCircuits(l1l2Circuit("c", cid)))

	// The ACK list: a held LSP with SSN set, the HIGHEST LSP ID of the three.
	ack := lspID(30, 0)
	ackLSP, ackRaw := buildLSP(t, packet.PDUTypeL2LSP, ack, 5, 1000, nil)
	d.Insert(Level2, ackLSP, ackRaw)
	d.setSSN(Level2, ack, cid)

	// The REQUEST list: an LSP this node does not hold, the LOWEST LSP ID.
	request := lspID(10, 0)
	f.recordPending(cid, Level2, request, pendingReq{seq: 3, lifetime: 900, checksum: 0x1234})

	// The ACK-ONLY list: an LSP refused on receipt, the MIDDLE LSP ID.
	ackOnly := lspID(20, 0)
	f.recordAckOnly(cid, Level2, ackOnly, packet.LSPEntry{LSPID: ackOnly, SequenceNumber: 2, RemainingLifetime: 800})

	pdus := f.buildPSNP(cid, Level2, ownSrc())
	if len(pdus) != 1 {
		t.Fatalf("built %d PSNPs, want 1", len(pdus))
	}
	entries := decodeLSPEntries(decodePSNP(t, pdus[0]).TLVs)
	if len(entries) != 3 {
		t.Fatalf("PSNP carries %d entries, want 3", len(entries))
	}
	// The list-order output (30, 10, 20) is the forbidden one.
	if entries[0].LSPID == ack {
		t.Fatalf("PSNP lists the acknowledgement %s first: entries follow list order, not LSP ID order", ack)
	}
	assertAscendingLSPIDs(t, "PSNP", entries)
	want := []types.LSPID{request, ackOnly, ack}
	for i, id := range want {
		if entries[i].LSPID != id {
			t.Fatalf("PSNP entry %d = %s, want %s", i, entries[i].LSPID, id)
		}
	}
}
