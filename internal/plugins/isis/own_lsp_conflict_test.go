// Design: plan/spec-isis-6-lsdb.md -- engine reaction to a claim on an own LSP ID.
//
// VALIDATES: the end-to-end engine wiring for ISO/IEC 10589 clause 7.3.16.4 c),
// which the lsdb-package unit tests cover only in pieces. A purge of this node's
// own LSP, arriving on the wire at a sequence the node never issued, must leave
// the node's own LSP live in the database at a sequence STRICTLY ABOVE the purge,
// with SRM armed so it floods.
//
// PREVENTS: the regression where the originator computed the next sequence from
// its own private counter (lastSeq) alone and never consulted what the network
// claimed, so a neighbor's purge at a higher sequence won permanently and the
// node stayed withdrawn from every peer's database.

package isis

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// ownLSPFrame builds the on-wire bytes of an L1 LSP for id at seq/lifetime, with
// a valid Fletcher checksum (WriteTo backfills it). A zero lifetime and no TLVs
// is a purge.
func ownLSPFrame(t *testing.T, id types.LSPID, seq types.SequenceNumber, lifetime types.RemainingLifetime) []byte {
	t.Helper()
	lsp := &packet.LSP{
		PDUType:           packet.PDUTypeL1LSP,
		RemainingLifetime: lifetime,
		LSPID:             id,
		SequenceNumber:    seq,
		TypeBlock:         packet.LSPFlagISTypeL1,
	}
	buf := make([]byte, lsp.EncodedLen())
	return buf[:lsp.WriteTo(buf, 0)]
}

// TestISISEngineReoriginatesAboveReceivedPurge is the defect's regression test.
// A neighbor floods a purge of THIS node's own fragment 0 at sequence 4096, a
// value this node has never issued. Clause 7.3.16.4 c) requires the node to not
// overwrite, to move its sequence above the received one, to generate a new LSP,
// and to set SRM on all circuits.
func TestISISEngineReoriginatesAboveReceivedPurge(t *testing.T) {
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1","interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point","metric":"10"}}}}}`)
	defer eng.shutdown()

	frag0 := types.NewLSPID(types.NewSourceID(eng.cfg.SystemID, 0), 0)
	before := eng.lsdb.Lookup(lsdb.Level1, frag0)
	if before == nil {
		t.Fatal("no own L1 fragment 0 after start")
	}
	const claimed types.SequenceNumber = 4096
	if before.Sequence() >= claimed {
		t.Fatalf("test premise broken: own sequence %d already >= the claim %d", before.Sequence(), claimed)
	}

	// Clear SRM so the assertion below cannot pass on the flag left by the
	// origination that ran at start.
	cid := eng.circuitIDFor("eth0")
	eng.lsdb.ClearSRM(lsdb.Level1, frag0, cid)

	eng.handleLSP(transport.RawFrame{IfIndex: 1, PDU: ownLSPFrame(t, frag0, claimed, 0)})

	after := eng.lsdb.Lookup(lsdb.Level1, frag0)
	if after == nil {
		t.Fatal("own fragment 0 gone after a neighbor purged it: the purge won")
	}
	if after.IsPurged() || after.Lifetime() == 0 {
		t.Fatal("own fragment 0 left purged: the received purge was stored (clause 7.3.16.4 c-1)")
	}
	if after.Sequence() <= claimed {
		t.Errorf("re-originated at sequence %d, which does not supersede the claim %d", after.Sequence(), claimed)
	}
	if after.Sequence() != claimed+1 {
		t.Errorf("re-originated at sequence %d, want %d (the next number greater)", after.Sequence(), claimed+1)
	}
	if !eng.lsdb.SRM(lsdb.Level1, frag0, cid) {
		t.Error("SRM not armed for the regenerated LSP (clause 7.3.16.4 c-4)")
	}
}

// TestISISEngineIgnoresStaleClaimOnOwnLSP proves the answer is bounded: an LSP
// bearing our own LSP ID at a sequence BELOW ours is not answered with a
// re-origination, so an in-flight retransmission of a copy the sender is behind
// on cannot drive a sequence-bump storm.
func TestISISEngineIgnoresStaleClaimOnOwnLSP(t *testing.T) {
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1","interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point","metric":"10"}}}}}`)
	defer eng.shutdown()

	frag0 := types.NewLSPID(types.NewSourceID(eng.cfg.SystemID, 0), 0)

	// Move our own sequence up so a stale claim is unambiguously below it.
	eng.reoriginateAboveClaim(lsdb.Level1, frag0, 100)
	seq := eng.lsdb.Lookup(lsdb.Level1, frag0).Sequence()
	if seq != 101 {
		t.Fatalf("setup sequence = %d, want 101", seq)
	}

	eng.handleLSP(transport.RawFrame{IfIndex: 1, PDU: ownLSPFrame(t, frag0, 50, 0)})

	if got := eng.lsdb.Lookup(lsdb.Level1, frag0).Sequence(); got != seq {
		t.Errorf("a stale claim bumped our sequence from %d to %d", seq, got)
	}
}

// TestISISEngineOwnLSPEchoIsNotAConflict proves the ordinary case is untouched:
// a neighbor flooding our own LSP straight back at us -- the same sequence, the
// same bytes -- must not be read as a claim, or every flood would answer itself
// with a sequence bump.
func TestISISEngineOwnLSPEchoIsNotAConflict(t *testing.T) {
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1","interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point","metric":"10"}}}}}`)
	defer eng.shutdown()

	frag0 := types.NewLSPID(types.NewSourceID(eng.cfg.SystemID, 0), 0)
	held := eng.lsdb.Lookup(lsdb.Level1, frag0)
	seq := held.Sequence()

	// Echo the exact bytes this node originated.
	eng.handleLSP(transport.RawFrame{IfIndex: 1, PDU: held.Raw()})

	after := eng.lsdb.Lookup(lsdb.Level1, frag0)
	if after.Sequence() != seq {
		t.Errorf("an echo of our own LSP bumped the sequence from %d to %d", seq, after.Sequence())
	}
	if after.IsPurged() {
		t.Error("an echo of our own LSP purged it")
	}
}
