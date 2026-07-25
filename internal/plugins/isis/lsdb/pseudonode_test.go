// Design: plan/spec-isis-8-dis-broadcast.md -- pseudo-node LSP origination tests.
//
// VALIDATES: the DIS-originated pseudo-node LSP (ISO/IEC 10589 clause 8.4.5):
//   - it has a NON-ZERO pseudonode LAN ID and lists every member as a TLV 22
//     neighbor at metric 0, the DIS included (TestPseudoNodeLSPBuild, AC-3);
//   - a zero pseudonode ID is rejected (TestPseudoNodeZeroIDRejected, boundary);
//   - the pseudonode ID boundary 1..255 originates (TestPseudoNodeIDBoundary);
//   - losing the DIS role purges every fragment at a bumped sequence with
//     Remaining Lifetime 0 and releases the ID (TestPseudoNodePurgeOnRoleLoss,
//     R-2, AC-5/AC-6);
//   - a re-origination bumps the sequence and reflects the current member set
//     (TestPseudoNodeReOriginateOnMemberChange, AC-4);
//   - a member list larger than one fragment splits with no entry dropped
//     (TestPseudoNodeFragmentation, A-4).
// PREVENTS: a regression where the pseudo-node LSP is built with pseudonode 0,
// omits a member, uses a non-zero metric, or does not purge cleanly on role loss.

package lsdb

import (
	"maps"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// pnMembers builds a member System ID list from final octets.
func pnMembers(last ...byte) []types.SystemID {
	out := make([]types.SystemID, len(last))
	for i, b := range last {
		out[i] = testSys(b)
	}
	return out
}

// decodePseudonodeFrag0 looks up and decodes fragment 0 of the pseudo-node LSP
// <sys>.<pnid> at level.
func decodePseudonodeFrag0(t *testing.T, d *LSDB, level Level, sys types.SystemID, pnid uint8) packet.LSP {
	t.Helper()
	id := types.NewLSPID(types.NewSourceID(sys, pnid), 0)
	e := d.Lookup(level, id)
	if e == nil {
		t.Fatalf("pseudo-node fragment 0 missing at %s for %s.%02x", level, sys, pnid)
	}
	lsp, err := e.Decode()
	if err != nil {
		t.Fatalf("decode pseudo-node fragment 0: %v", err)
	}
	return lsp
}

// pseudonodeMembers decodes the TLV 22 entries across an LSP into a map of member
// Source ID -> metric, for assertions.
func pseudonodeMembers(t *testing.T, lsp packet.LSP) map[types.SourceID]uint32 {
	t.Helper()
	out := map[types.SourceID]uint32{}
	for _, tlv := range lsp.TLVs {
		if tlv.Type != packet.TLVExtendedISReach {
			continue
		}
		dec, err := packet.DecodeExtendedISReachTLV(tlv.Value)
		if err != nil {
			t.Fatalf("decode TLV 22: %v", err)
		}
		for _, e := range dec.Entries {
			out[e.Neighbor] = e.Metric.Value()
		}
	}
	return out
}

func TestPseudoNodeLSPBuild(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)

	// DIS is system ..01; segment members are ..01 (the DIS), ..02, ..03.
	const pnid = 0x07
	info := PseudonodeInfo{
		SystemID:     testSys(0x01),
		PseudonodeID: pnid,
		Members:      pnMembers(0x01, 0x02, 0x03),
		MaxLifetime:  1200,
	}
	res := o.OriginatePseudonode(Level1, info)
	if len(res.Originated) == 0 {
		t.Fatal("pseudo-node origination produced no LSP")
	}

	// The LSP ID must carry the non-zero pseudonode octet (AC-3).
	frag0ID := res.Originated[0]
	if frag0ID.PseudonodeID() != pnid {
		t.Fatalf("pseudo-node LSP ID has pseudonode %02x, want %02x", frag0ID.PseudonodeID(), pnid)
	}
	if frag0ID.SystemID() != testSys(0x01) {
		t.Fatalf("pseudo-node LSP ID system %s, want %s", frag0ID.SystemID(), testSys(0x01))
	}

	lsp := decodePseudonodeFrag0(t, d, Level1, testSys(0x01), pnid)
	members := pseudonodeMembers(t, lsp)
	// All three members present, each at metric 0 (clause 8.4.5).
	for _, m := range pnMembers(0x01, 0x02, 0x03) {
		src := types.NewSourceID(m, 0)
		metric, ok := members[src]
		if !ok {
			t.Fatalf("pseudo-node LSP missing member %s", src)
		}
		if metric != 0 {
			t.Fatalf("member %s metric = %d, want 0", src, metric)
		}
	}
	if len(members) != 3 {
		t.Fatalf("pseudo-node LSP has %d members, want 3", len(members))
	}

	// A pseudo-node LSP carries no area/protocols/hostname TLVs (it is a pure hub).
	for _, tlv := range lsp.TLVs {
		switch tlv.Type {
		case packet.TLVAreaAddresses, packet.TLVProtocolsSupported, packet.TLVDynamicHostname:
			t.Fatalf("pseudo-node LSP unexpectedly carries TLV %d", tlv.Type)
		}
	}
}

func TestPseudoNodeZeroIDRejected(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	res := o.OriginatePseudonode(Level1, PseudonodeInfo{
		SystemID:     testSys(0x01),
		PseudonodeID: 0, // invalid: 0 means a real router, not a pseudo-node
		Members:      pnMembers(0x01, 0x02),
	})
	if len(res.Originated) != 0 || len(res.Purged) != 0 {
		t.Fatal("a zero pseudonode ID must originate nothing (it would collide with the router LSP)")
	}
	// Nothing must be stored at pseudonode 0 by this call (that is the router LSP
	// space, owned by Originate, not OriginatePseudonode).
	id := types.NewLSPID(types.NewSourceID(testSys(0x01), 0), 0)
	if d.Lookup(Level1, id) != nil {
		t.Fatal("OriginatePseudonode must not write the pseudonode-0 (router) LSP")
	}
}

func TestPseudoNodeIDBoundary(t *testing.T) {
	// Pseudonode ID 1 (lowest valid) and 255 (highest) both originate.
	for _, pnid := range []uint8{1, 255} {
		d := New(nil)
		o := NewOriginator(d, nil)
		res := o.OriginatePseudonode(Level1, PseudonodeInfo{
			SystemID:     testSys(0x01),
			PseudonodeID: pnid,
			Members:      pnMembers(0x01),
		})
		if len(res.Originated) == 0 {
			t.Fatalf("pseudonode ID %d should originate", pnid)
		}
		if res.Originated[0].PseudonodeID() != pnid {
			t.Fatalf("originated LSP pseudonode %02x, want %02x", res.Originated[0].PseudonodeID(), pnid)
		}
	}
}

func TestPseudoNodeReOriginateOnMemberChange(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	const pnid = 0x05
	base := PseudonodeInfo{SystemID: testSys(0x01), PseudonodeID: pnid, MaxLifetime: 1200}

	// First origination: two members.
	base.Members = pnMembers(0x01, 0x02)
	o.OriginatePseudonode(Level1, base)
	id := types.NewLSPID(types.NewSourceID(testSys(0x01), pnid), 0)
	seq1 := d.Lookup(Level1, id).Sequence()

	// A new member joins (AC-4): re-originate; the sequence bumps and the member
	// set now includes ..03.
	base.Members = pnMembers(0x01, 0x02, 0x03)
	o.OriginatePseudonode(Level1, base)
	seq2 := d.Lookup(Level1, id).Sequence()
	if seq2 <= seq1 {
		t.Fatalf("re-origination must bump the sequence: %d -> %d", seq1, seq2)
	}
	members := pseudonodeMembers(t, decodePseudonodeFrag0(t, d, Level1, testSys(0x01), pnid))
	if _, ok := members[types.NewSourceID(testSys(0x03), 0)]; !ok {
		t.Fatal("re-originated pseudo-node LSP must include the new member ..03")
	}
}

func TestPseudoNodePurgeOnRoleLoss(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	const pnid = 0x09

	// Originate, then purge (the local node lost the DIS role, R-2).
	o.OriginatePseudonode(Level1, PseudonodeInfo{
		SystemID:     testSys(0x01),
		PseudonodeID: pnid,
		Members:      pnMembers(0x01, 0x02, 0x03),
		MaxLifetime:  1200,
	})
	id := types.NewLSPID(types.NewSourceID(testSys(0x01), pnid), 0)
	liveSeq := d.Lookup(Level1, id).Sequence()

	purged := o.PurgePseudonode(Level1, testSys(0x01), pnid)
	if len(purged) == 0 {
		t.Fatal("PurgePseudonode must report the purged fragment(s)")
	}

	e := d.Lookup(Level1, id)
	if e == nil {
		t.Fatal("a purged pseudo-node LSP is retained (zero-age grace), not deleted at once")
	}
	if !e.Lifetime().IsPurge() {
		t.Fatalf("purged pseudo-node LSP must have Remaining Lifetime 0, got %d", e.Lifetime().Seconds())
	}
	if !e.IsPurged() {
		t.Fatal("purged entry must report IsPurged")
	}
	if e.Sequence() <= liveSeq {
		t.Fatalf("purge must bump the sequence so peers accept it: live=%d purge=%d", liveSeq, e.Sequence())
	}
}

func TestPseudoNodePurgeZeroIDNoop(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	if purged := o.PurgePseudonode(Level1, testSys(0x01), 0); purged != nil {
		t.Fatal("purging a zero pseudonode ID must be a no-op")
	}
}

func TestPseudoNodeFragmentation(t *testing.T) {
	// Force a tiny LSP size so a handful of members must split across fragments.
	// Each TLV 22 entry is 11 octets; minLSPSize floors the budget at minTLVBudget
	// (64), so ~5 entries fit one fragment. 30 members must produce >1 fragment and
	// every member must appear (A-4, no entry dropped).
	d := New(nil)
	o := NewOriginator(d, nil)
	const pnid = 0x03
	members := make([]types.SystemID, 0, 30)
	for i := range 30 {
		members = append(members, testSys(byte(0x20+i)))
	}
	res := o.OriginatePseudonode(Level1, PseudonodeInfo{
		SystemID:     testSys(0x01),
		PseudonodeID: pnid,
		Members:      members,
		MaxLSPSize:   minLSPSize, // smallest allowed -> small per-fragment budget
		MaxLifetime:  1200,
	})
	if len(res.Originated) < 2 {
		t.Fatalf("30 members at the minimum LSP size must span >1 fragment, got %d", len(res.Originated))
	}

	// Collect every member across all fragments; all 30 must be present, each at 0.
	all := map[types.SourceID]uint32{}
	for _, id := range res.Originated {
		e := d.Lookup(Level1, id)
		if e == nil {
			t.Fatalf("fragment %s missing", id)
		}
		lsp, err := e.Decode()
		if err != nil {
			t.Fatalf("decode fragment %s: %v", id, err)
		}
		maps.Copy(all, pseudonodeMembers(t, lsp))
	}
	for _, m := range members {
		src := types.NewSourceID(m, 0)
		if metric, ok := all[src]; !ok {
			t.Fatalf("member %s dropped during fragmentation", src)
		} else if metric != 0 {
			t.Fatalf("member %s metric %d, want 0", src, metric)
		}
	}
	if len(all) != 30 {
		t.Fatalf("got %d distinct members across fragments, want 30", len(all))
	}
}
