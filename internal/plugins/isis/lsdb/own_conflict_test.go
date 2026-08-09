// Design: docs/architecture/isis/isis-6-lsdb.md -- own-LSP sequence conflicts.
//
// VALIDATES: ISO/IEC 10589 clause 7.3.16.1 and clause 7.3.16.4 c) at the store /
// originator layer: another system's claim on one of this system's own LSP IDs is
// never written into the database, and the next origination of that ID moves
// STRICTLY ABOVE the claimed sequence. Before this, the originator computed the
// next sequence from its own private counter alone, so a purge carrying a
// sequence this system never issued was never superseded.
//
// One test per claim:
//   - the floor is a floor (TestISISRaiseSequenceFloorOriginatesAbove)
//   - a claim below ours is refused (TestISISRaiseSequenceFloorIgnoresLowerClaim)
//   - the floor composes with the wraparound suspension, in both directions
//     (TestISISRaiseSequenceFloorComposesWithWraparound)
//   - the receive-side classification of an own LSP (TestISISOwnConflictStates)

package lsdb

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// ownFrag0 is the node's own non-pseudonode fragment 0 for sampleNode.
func ownFrag0(sys types.SystemID) types.LSPID {
	return types.NewLSPID(types.NewSourceID(sys, 0), 0)
}

// TestISISRaiseSequenceFloorOriginatesAbove is the core claim: after another
// system claims sequence N for one of our LSP IDs, the next origination emits
// N+1, not lastSeq+1. ISO/IEC 10589 clause 7.3.16.1 requires exactly that: "When
// S receives this LSP it shall change its sequence number to be the next number
// greater than the new one received, and shall generate a link state PDU"
// (quoted from RFC 1142 section 7.3.16.1, whose text is identical; see
// iso/README.md on why this repository holds no copy of the ISO document).
func TestISISRaiseSequenceFloorOriginatesAbove(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	node := sampleNode(t)
	state := LevelState{InterfaceAddrs: []netip.Addr{netip.MustParseAddr("10.0.0.1")}}
	frag0 := ownFrag0(node.SystemID)

	o.Originate(Level1, node, state)
	before := d.Lookup(Level1, frag0).Sequence()
	if before != types.FirstSequenceNumber {
		t.Fatalf("first origination sequence = %d, want %d", before, types.FirstSequenceNumber)
	}

	// A neighbor claims a sequence far above anything this node has issued.
	const claimed types.SequenceNumber = 4096
	if !o.RaiseSequenceFloor(frag0, claimed) {
		t.Fatal("RaiseSequenceFloor refused a claim above our own sequence")
	}
	o.Originate(Level1, node, state)

	got := d.Lookup(Level1, frag0).Sequence()
	if got != claimed+1 {
		t.Errorf("origination after a claim of %d produced sequence %d, want %d", claimed, got, claimed+1)
	}
	if got <= claimed {
		t.Errorf("origination did not supersede the claim: %d <= %d", got, claimed)
	}
}

// TestISISRaiseSequenceFloorIgnoresLowerClaim proves the floor only ever rises.
// A claim BELOW a sequence this node has already issued is refused, so a stale
// retransmission cannot drive a sequence-bump storm: the copy this node already
// holds is what corrects the sender.
func TestISISRaiseSequenceFloorIgnoresLowerClaim(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	node := sampleNode(t)
	state := LevelState{InterfaceAddrs: []netip.Addr{netip.MustParseAddr("10.0.0.1")}}
	frag0 := ownFrag0(node.SystemID)

	// Reach sequence 3.
	for range 3 {
		o.Originate(Level1, node, state)
		state.InterfaceAddrs = append(state.InterfaceAddrs, netip.MustParseAddr("10.0.0.2"))
	}
	if got := d.Lookup(Level1, frag0).Sequence(); got != 3 {
		t.Fatalf("setup sequence = %d, want 3", got)
	}

	if o.RaiseSequenceFloor(frag0, 2) {
		t.Error("RaiseSequenceFloor accepted a claim below our own sequence")
	}
	// The equal case IS accepted: clause 7.3.16.2 (LSP confusion) resolves an
	// equal sequence with a differing checksum by moving above it.
	if !o.RaiseSequenceFloor(frag0, 3) {
		t.Error("RaiseSequenceFloor refused a claim equal to our own sequence")
	}
	o.Originate(Level1, node, state)
	if got := d.Lookup(Level1, frag0).Sequence(); got != 4 {
		t.Errorf("sequence after an equal claim = %d, want 4", got)
	}
}

// TestISISRaiseSequenceFloorComposesWithWraparound proves the two mechanisms
// compose in BOTH directions, which is the hazard a sequence floor introduces:
//
//   - a floor raised to the maximum makes the next origination WRAP (purge at the
//     maximum, suspend, count the wrap) rather than emitting a bogus value, so
//     raising a sequence can never silently defeat the wraparound handling;
//   - a claim arriving WHILE an ID is suspended cannot raise anything at all, so
//     it can neither shorten the window nor survive it: origination restarts
//     from 1, which is what ISO/IEC 10589 clause 7.3.16.1 requires once every
//     copy carrying the high sequence has expired ("When it is re-enabled the IS
//     shall start again with sequence number 1"). The wrap deletes the ID's
//     sequence state, and a claim on an ID with no state is refused.
func TestISISRaiseSequenceFloorComposesWithWraparound(t *testing.T) {
	clk := newFakeClock()
	d := New(clk.now)
	o := NewOriginator(d, clk.now)
	node := sampleNode(t)
	state := LevelState{InterfaceAddrs: []netip.Addr{netip.MustParseAddr("10.0.0.1")}}
	frag0 := ownFrag0(node.SystemID)

	o.Originate(Level1, node, state)

	// Direction 1: a claim at the maximum sequence must WRAP, not originate.
	if !o.RaiseSequenceFloor(frag0, types.MaxSequenceNumber) {
		t.Fatal("RaiseSequenceFloor refused a maximum-sequence claim")
	}
	res := o.Originate(Level1, node, state)
	if !res.Wrapped {
		t.Fatal("a floor raised to MaxSequenceNumber did not wrap the next origination")
	}
	e := d.Lookup(Level1, frag0)
	if !e.IsPurged() || e.Sequence() != types.MaxSequenceNumber {
		t.Errorf("wrap did not purge at the maximum: purged=%v seq=%d", e.IsPurged(), e.Sequence())
	}

	// Direction 2: a claim arriving during the suspension raises nothing. The wrap
	// deleted the ID's sequence state, and a claim on an ID this node is not
	// currently originating is refused, so the window cannot be shortened.
	if o.RaiseSequenceFloor(frag0, 9000) {
		t.Fatal("a claim was answered while the ID was suspended after a wraparound")
	}
	clk.advance(DefaultMaxAge) // still inside MaxAge + ZeroAgeLifetime
	if res = o.Originate(Level1, node, state); len(res.Originated) != 0 {
		t.Errorf("a floor raised during the suspension re-originated it early: %+v", res.Originated)
	}
	if got := d.Lookup(Level1, frag0).Sequence(); got != types.MaxSequenceNumber {
		t.Errorf("suspended LSP rewritten at %d during the suspension window", got)
	}

	// Plant a sequence for the suspended ID the way purgeStaleFragmentsLocked
	// would: it writes lastSeq for any fragment still present in the LSDB, and a
	// wrapped fragment IS present (purged at the maximum). That is the reachable
	// path by which a suspended ID regains a sequence, and the reason the
	// suspension-expiry branch clears lastSeq as well as suspendUntil. Without
	// that clear the restart below would be from 9001, not 1.
	o.mu.Lock()
	o.lastSeq[frag0] = 9000
	o.mu.Unlock()

	// ... and does not survive it: the restart is from 1.
	clk.advance(DefaultMaxAge + ZeroAgeLifetime)
	if res = o.Originate(Level1, node, state); len(res.Originated) == 0 {
		t.Fatal("LSP not re-originated after the suspension window")
	}
	if got := d.Lookup(Level1, frag0).Sequence(); got != types.FirstSequenceNumber {
		t.Errorf("post-suspension sequence = %d, want %d (the raised floor must not survive)", got, types.FirstSequenceNumber)
	}
}

// TestISISOwnConflictStates covers the receive-side classification: which
// arrivals bearing one of THIS system's own LSP IDs are conflicts it must answer
// by re-originating, and which are not (ISO/IEC 10589 clause 7.3.16.4 a) / c) and
// clause 7.3.16.2). Every conflict is refused storage -- clause 7.3.16.4 c-1,
// "shall not overwrite with the received LSP".
func TestISISOwnConflictStates(t *testing.T) {
	node := sampleNode(t)
	frag0 := ownFrag0(node.SystemID)

	// heldSeq/heldChecksum describe the copy this system originated.
	const heldSeq types.SequenceNumber = 10

	tests := []struct {
		name         string
		noEntry      bool
		heldPurged   bool
		inSeq        types.SequenceNumber
		inLifetime   types.RemainingLifetime
		sameChecksum bool
		// forceHeldChecksum stamps OUR stored checksum onto the arrival, whatever
		// its sequence. sameChecksum only leaves buildLSP's natural checksum in
		// place, which still differs once the sequence differs (the sequence is
		// inside the checksummed region), so it cannot express "higher sequence,
		// same checksum".
		forceHeldChecksum bool
		wantConflict      bool
	}{
		{name: "purge above our sequence", inSeq: heldSeq + 5, inLifetime: 0, wantConflict: true},
		// The pure-SEQUENCE branch. Every other above-our-sequence row also
		// differs in checksum or is a purge, so deleting that branch would leave
		// them caught by the later one -- and this arrival would then be STORED,
		// not merely unanswered. forceChecksum pins our own checksum onto a higher
		// sequence, which the natural encoding never produces (the sequence is
		// inside the checksummed region).
		{name: "live copy above our sequence, our checksum", inSeq: heldSeq + 5, inLifetime: 1000, forceHeldChecksum: true, wantConflict: true},
		{name: "purge at our sequence", inSeq: heldSeq, inLifetime: 0, sameChecksum: true, wantConflict: true},
		{name: "live copy above our sequence", inSeq: heldSeq + 5, inLifetime: 1000, wantConflict: true},
		{name: "same sequence differing checksum", inSeq: heldSeq, inLifetime: 1000, wantConflict: true},
		{name: "purge with no entry held", noEntry: true, inSeq: 4096, inLifetime: 0, wantConflict: true},
		{name: "our own echo", inSeq: heldSeq, inLifetime: 1000, sameChecksum: true, wantConflict: false},
		{name: "stale copy below our sequence", inSeq: heldSeq - 1, inLifetime: 1000, wantConflict: false},
		{name: "echo of a purge we issued", heldPurged: true, inSeq: heldSeq, inLifetime: 0, sameChecksum: true, wantConflict: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := New(nil)
			var held *Entry
			if !tc.noEntry {
				lifetime := types.RemainingLifetime(1000)
				if tc.heldPurged {
					lifetime = 0
				}
				lsp, raw := buildLSP(t, packet.PDUTypeL1LSP, frag0, heldSeq, lifetime, nil)
				d.Insert(Level1, lsp, raw)
				held = d.Lookup(Level1, frag0)
			}

			in, _ := buildLSP(t, packet.PDUTypeL1LSP, frag0, tc.inSeq, tc.inLifetime, nil)
			switch {
			case held == nil:
			case tc.forceHeldChecksum:
				in.Checksum = held.Checksum()
			case !tc.sameChecksum:
				in.Checksum = held.Checksum() ^ 0xFFFF
			}

			res, conflict := ownConflictResult(held, in)
			if conflict != tc.wantConflict {
				t.Fatalf("ownConflictResult conflict = %v, want %v", conflict, tc.wantConflict)
			}
			if !conflict {
				return
			}
			if res.Stored {
				t.Error("a conflicting own LSP was reported as stored (clause 7.3.16.4 c-1 forbids the overwrite)")
			}
			if !res.OwnConflict {
				t.Error("conflict result does not carry OwnConflict")
			}
			if res.ConflictSequence != tc.inSeq {
				t.Errorf("ConflictSequence = %d, want %d", res.ConflictSequence, tc.inSeq)
			}
		})
	}
}

// TestISISRaiseSequenceFloorAnswersAClaimOnce proves the answer is bounded even
// when it cannot advance the floor by itself. A retransmission of the SAME stale
// LSP must not re-originate once per copy: raising the floor writes
// lastSeq[id] = claimed, so the "already above it" test cannot damp a claim whose
// answer never writes that ID -- a fragment the current state no longer produces
// is exactly that case. A HIGHER claim afterwards is still answered.
func TestISISRaiseSequenceFloorAnswersAClaimOnce(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	node := sampleNode(t)
	state := LevelState{InterfaceAddrs: []netip.Addr{netip.MustParseAddr("10.0.0.1")}}
	frag0 := ownFrag0(node.SystemID)
	o.Originate(Level1, node, state)

	if !o.RaiseSequenceFloor(frag0, 4096) {
		t.Fatal("first claim not answered")
	}
	if o.RaiseSequenceFloor(frag0, 4096) {
		t.Error("the SAME claim was answered twice: a retransmission re-originates")
	}
	if o.RaiseSequenceFloor(frag0, 4000) {
		t.Error("a claim below the one already answered was answered")
	}
	if !o.RaiseSequenceFloor(frag0, 5000) {
		t.Error("a HIGHER claim after an answered one was refused")
	}
}

// TestISISRaiseSequenceFloorIgnoresUnoriginatedID proves a claim on an LSP ID
// this node does not originate is refused. Clause 7.3.16.4 c) needs "an
// un-expired LSP from S ... in memory" to change the sequence OF; clause
// 7.3.16.4 a) asks only for an acknowledgement. Refusing also stops a remote
// party creating per-LSP-ID state here by claiming IDs across our own System
// ID's 65280-entry fragment space.
func TestISISRaiseSequenceFloorIgnoresUnoriginatedID(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	node := sampleNode(t)
	o.Originate(Level1, node, LevelState{})

	// Fragment 7 of our own System ID: never originated (state fits fragment 0).
	never := types.NewLSPID(types.NewSourceID(node.SystemID, 0), 7)
	if o.RaiseSequenceFloor(never, 4096) {
		t.Error("a claim on an LSP ID this node never originated was answered")
	}
	o.mu.Lock()
	_, tracked := o.lastSeq[never]
	claims := len(o.answeredClaim)
	o.mu.Unlock()
	if tracked {
		t.Error("a refused claim still created lastSeq state a remote party controls")
	}
	if claims != 0 {
		t.Errorf("a refused claim created %d answeredClaim entries, want 0", claims)
	}
}

// TestISISSignedOwnLSPChecksumMatchesStoredBytes pins the invariant the stored
// entry depends on: an entry's checksum metadata must be the checksum of the
// bytes that entry stores and floods.
//
// packet.LSP.WriteTo fills the struct with the PRE-signature checksum, and
// packet.SignPDU recomputes it inside the BYTES without touching the struct.
// Storing the struct's value gives a signed own LSP an entry whose checksum
// nothing can reproduce from its own bytes. Two things break at once: the CSNP
// this node sources advertises that unreproducible value (ISO/IEC 10589 clause
// 7.3.16.2 then has a receiver treat our own LSP as confused), and this node
// reads the echo of its own flood as a same-sequence checksum mismatch and
// answers it under clause 7.3.16.4, bumping its sequence once per echo.
func TestISISSignedOwnLSPChecksumMatchesStoredBytes(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	node := sampleNode(t)
	frag0 := ownFrag0(node.SystemID)

	// A signer standing in for spec-isis-10: it mutates the bytes after WriteTo
	// exactly as SignPDU does -- insert a TLV, then recompute the checksum over
	// the new bytes.
	o.SetSigner(func(pdu []byte) []byte {
		lsp, err := packet.DecodePDU(pdu)
		if err != nil || lsp.LSP == nil {
			t.Fatalf("signer could not decode the LSP: %v", err)
		}
		signed := *lsp.LSP
		signed.TLVs = append([]packet.TLV{{
			Type:  packet.TLVAuthentication,
			Value: []byte{0x01, 'k', 'e', 'y'},
		}}, signed.TLVs...)
		buf := make([]byte, signed.EncodedLen())
		return buf[:signed.WriteTo(buf, 0)]
	})

	o.Originate(Level1, node, LevelState{})
	e := d.Lookup(Level1, frag0)
	if e == nil {
		t.Fatal("no own fragment 0 after a signed origination")
	}
	stored, ok := packet.LSPChecksumOf(e.Raw())
	if !ok {
		t.Fatal("stored bytes too short to hold a checksum")
	}
	if e.Checksum() != stored {
		t.Errorf("entry checksum 0x%04x does not match its own bytes 0x%04x", e.Checksum(), stored)
	}
	decoded, err := e.Decode()
	if err != nil {
		t.Fatalf("decode stored bytes: %v", err)
	}
	if !decoded.VerifyChecksum() {
		t.Error("the stored signed LSP does not verify its own Fletcher checksum")
	}
}

// TestISISOwnLSPNotHeldIsAcknowledged covers ISO/IEC 10589 clause 7.3.16.4 a):
// an LSP arriving for a source this database holds NOTHING for is acknowledged
// and not retained. For one of our own LSP IDs the SSN flag cannot carry that
// acknowledgement, because SSN lives on an LSDB entry and there is none, so the
// ack goes out in the next PSNP at the ARRIVED sequence.
//
// The two halves are asserted together on purpose. "Not retained" alone would
// also hold if the arrival were dropped on the floor, and "acknowledged" alone
// would also hold if it were stored.
func TestISISOwnLSPNotHeldIsAcknowledged(t *testing.T) {
	const cid CircuitID = 1
	sys := testSys(1)
	id := types.NewLSPID(types.NewSourceID(sys, 0), 0)

	d := New(nil)
	f := NewFlooder(d, nil, staticCircuits(l1l2Circuit("in", cid)))
	f.SetSystemID(sys)

	// A purge of OUR LSP ID arrives while this database holds nothing for it.
	purge, raw := buildLSP(t, packet.PDUTypeL1LSP, id, 4096, 0, nil)
	res := f.ReceiveLSP(cid, false, purge, raw)

	if res.Stored {
		t.Error("clause 7.3.16.4 a-2: the LSP was retained")
	}
	if d.Lookup(Level1, id) != nil {
		t.Error("clause 7.3.16.4 a-2: an entry was created for an LSP we must not retain")
	}
	if !res.OwnConflict || res.ConflictSequence != 4096 {
		t.Errorf("the arrival was not reported as an own conflict: %+v", res)
	}

	pdus := f.buildPSNP(cid, Level1, types.NewSourceID(sys, 0))
	if len(pdus) != 1 {
		t.Fatalf("clause 7.3.16.4 a-1: %d PSNPs built, want 1 carrying the acknowledgement", len(pdus))
	}
	psnp := decodePSNP(t, pdus[0])
	var acked bool
	for _, tlv := range psnp.TLVs {
		if tlv.Type != packet.TLVLSPEntries {
			continue
		}
		entries, err := packet.DecodeLSPEntriesTLV(tlv.Value)
		if err != nil {
			t.Fatalf("decode TLV 9: %v", err)
		}
		for _, e := range entries.Entries {
			if e.LSPID != id {
				continue
			}
			acked = true
			// An ACK echoes the arrival. A REQUEST goes out at sequence 0 so the
			// holder reads it as older and supplies the LSP; at the sender's own
			// sequence it reads as an acknowledgement and clears SRM.
			if e.SequenceNumber != 4096 {
				t.Errorf("acknowledged at sequence %d, want 4096 (0 would be a REQUEST)", e.SequenceNumber)
			}
		}
	}
	if !acked {
		t.Error("clause 7.3.16.4 a-1: the PSNP carries no acknowledgement of the arrival")
	}

	// a-2: nothing survives the acknowledgement. A second PSNP repeats nothing.
	if pdus = f.buildPSNP(cid, Level1, types.NewSourceID(sys, 0)); len(pdus) != 0 {
		t.Errorf("the acknowledgement was retained and re-sent: %d PSNPs", len(pdus))
	}
}

// TestISISReceiveRefusesOwnLSPOverwrite drives the refusal through the exported
// Receive path (not ownConflictResult directly) so the wiring between the two is
// covered: a neighbor's purge of our un-expired own LSP must leave the database
// holding OUR copy, un-purged.
func TestISISReceiveRefusesOwnLSPOverwrite(t *testing.T) {
	d := New(nil)
	node := sampleNode(t)
	frag0 := ownFrag0(node.SystemID)

	mine, raw := buildLSP(t, packet.PDUTypeL1LSP, frag0, 7, 1000, nil)
	d.Insert(Level1, mine, raw)

	purge, praw := buildLSP(t, packet.PDUTypeL1LSP, frag0, 4096, 0, nil)
	res := d.Receive(Level1, purge, praw, true)

	if res.Stored {
		t.Fatal("a neighbor's purge of our own LSP was stored")
	}
	if !res.OwnConflict || res.ConflictSequence != 4096 {
		t.Errorf("Receive did not report the own conflict: %+v", res)
	}
	e := d.Lookup(Level1, frag0)
	if e.IsPurged() || e.Sequence() != 7 {
		t.Errorf("our own LSP was displaced: purged=%v seq=%d, want purged=false seq=7", e.IsPurged(), e.Sequence())
	}
}
