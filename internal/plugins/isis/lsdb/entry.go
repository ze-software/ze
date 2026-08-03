// Design: plan/learned/932-isis-6-lsdb.md -- per-LSP database entry (raw bytes + metadata).
// ISO/IEC 10589 clause 7.3: an LSP is identified by its LSP ID and versioned by
// its Sequence Number; the Remaining Lifetime ages it down; the Fletcher
// checksum (clause 7.3.11) protects it.
//
// RFC: rfc/short/rfc3787.md -- the LSP-database-overload (OL) bit (clause 9.8 type block).
// RFC: rfc/short/rfc3786.md -- the 256-fragment model: LSP number 0..255, fragment 0 is special.
//
// The entry follows Ze's buffer-first / lazy model (ai/rules/performance.md):
// it stores the verbatim PDU bytes (a single OWNED copy, never an alias of a
// reused receive buffer) plus a small parsed metadata header. TLVs are parsed
// ON DEMAND (Decode), never eagerly into structs, so an LSP carrying a TLV the
// node does not understand re-floods byte-for-byte (ISO/IEC 10589 clause
// 7.3.14). The per-circuit SRM/SSN flag sets live on the entry (the data model
// the flooding spec, isis-7, drives) but this spec only stores and exposes them.

package lsdb

import (
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// CircuitID is a stable, small per-circuit identifier the LSDB uses to index the
// SRM/SSN flag sets (spec A-3: one bit per circuit). The engine assigns one ID
// per open circuit; the flooding spec (isis-7) sets and clears the flags by this
// ID. It is deliberately not the kernel ifindex (which is large and sparse): the
// engine maps interface name -> CircuitID so the flag sets stay compact.
type CircuitID uint16

// Entry is one LSP in the database: the verbatim PDU bytes plus the parsed
// freshness metadata and the per-circuit flooding flags. It is owned by the
// LSDB (guarded by the LSDB mutex). Snapshot hands out copies, but Lookup
// returns the LIVE pointer, which is what makes the field discipline below
// load-bearing rather than advisory.
//
// FIELD DISCIPLINE (read before adding a field or an accessor).
//
// A field needs an atomic when BOTH of these are true. Neither alone is
// enough, and conflating them is what produced the DATA RACE this discipline
// was written for (TestISISDISElection, lifetime read off-lock by SNP
// generation while the aging tick decremented it):
//
//  1. It is mutated AFTER the entry is published into store.entries. Most
//     fields are not: replaceLocked (lsdb.go) builds a FRESH Entry and swaps
//     it in, so sequence, checksum, typeBlock, own, raw and receivedPurge are
//     written once before the entry is reachable and never again.
//  2. It is read WITHOUT the LSDB lock. Today that means an exported accessor
//     on *Entry reaches it, since Lookup hands out a live pointer and a caller
//     holding one can call any method after the lock is released. But the
//     condition is the UNLOCKED READ, not the accessor: an in-package read
//     outside d.mu would race just as well. The accessor set is the current
//     evidence for this condition, not the definition of it.
//
// Mutated post-publication, so condition 1 holds for: lifetime (aging tick,
// and the clause 7.3.16 duplicate refresh), purged (markPurgedLocked),
// recvPurgeReflooded (the one-shot re-flood guard) and deleteAt (the grace
// timer). Of those, only lifetime and purged also satisfy condition 2, so
// only those two are atomic. recvPurgeReflooded and deleteAt stay plain
// PRECISELY BECAUSE no accessor exposes them -- adding one would be a race,
// not a convenience. The srm/ssn/srmSent maps are likewise reachable only
// through LSDB methods that take the lock themselves.
type Entry struct {
	// id is the LSP ID (the database key, duplicated here for convenience).
	id types.LSPID
	// raw is the verbatim PDU (common header + body), a SINGLE owned copy. On a
	// received LSP it is copied out of the decode buffer (never aliased) so the
	// transport may reuse its receive buffer without corrupting the entry
	// (security review: memory safety). On an originated LSP it is the build
	// buffer copied once. Re-flood (isis-7) sends these bytes unchanged.
	raw []byte
	// sequence is the LSP Sequence Number (ISO/IEC 10589 clause 7.3). The
	// freshness compare keys on this first.
	sequence types.SequenceNumber
	// lifetime is the Remaining Lifetime in seconds, decremented once per second
	// by the aging tick. 0 marks a purge (clause 7.3.16/17).
	//
	// ATOMIC because it is one of only two fields that are BOTH mutated after
	// the entry is published in the database AND read without the lock: the
	// aging tick decrements it (aging.go) and a duplicate LSP refreshes it
	// (lsdb.go, clause 7.3.16), both under the LSDB write lock, while
	// Lifetime() is called with NO lock from SNP generation on the flooding
	// goroutine. Holds a uint16 value.
	//
	// The two conditions are separate and BOTH are required -- see the field
	// discipline note above the struct. Post-publication mutation alone does
	// not need an atomic (recvPurgeReflooded and deleteAt are mutated after
	// publication and stay plain, because nothing reads them off-lock), and an
	// off-lock read alone does not either (sequence, checksum, typeBlock, own
	// and raw are read off-lock and stay plain, because replaceLocked writes
	// them once before the entry is reachable).
	lifetime atomic.Uint32
	// checksum is the LSP's stored Fletcher checksum (clause 7.3.11), the value
	// CSNP/PSNP (isis-7) compare and the freshness compare uses as a tiebreak.
	checksum uint16
	// typeBlock is the LSP type-block octet (clause 9.8): P/ATT/OL/IS-type. The
	// overload (OL) bit is read from here for the snapshot and SPF (isis-9).
	typeBlock uint8

	// own reports whether this node originated the LSP (its System ID matches
	// ours). An own LSP is refreshed before MaxAge and re-originated after a
	// topology change; a foreign LSP is only aged and re-flooded.
	own bool

	// purged marks an LSP whose Remaining Lifetime has reached 0. It is kept in
	// the database for the ZeroAgeLifetime grace period (NOT deleted at once) so
	// a node that missed the purge cannot keep a stale copy (clause 7.3.16/17).
	//
	// ATOMIC for the same reason as lifetime: markPurgedLocked sets it on an
	// already-published entry while IsPurged() is read with no lock from the
	// show, SPF and DIS paths.
	purged atomic.Bool
	// receivedPurge distinguishes a purge that arrived on the wire (re-flood and
	// retain) from a local expiry (garbage-collect): the two are handled by
	// distinct paths (spec AC-9, R-2). Plain: written only by replaceLocked
	// before publication, and read only under the LSDB lock.
	receivedPurge bool
	// recvPurgeReflooded guards the one-shot tick-driven re-flood of a received
	// purge: the receive path floods it once on arrival (SRM on other circuits),
	// and the aging tick surfaces it ONCE more (re-arming SRM) so a neighbor that
	// missed the first flood still converges within the grace window, without a
	// per-second re-flood storm (ISO/IEC 10589 clause 7.3.16, spec R-2/R-4). Set
	// when the tick first surfaces the received purge.
	//
	// LOCK-ONLY: mutated after publication (aging.go, the tick) but touched
	// nowhere else, so it stays plain. Do NOT add an accessor -- that would
	// give it an off-lock reader and make it a race (see the discipline note).
	recvPurgeReflooded bool
	// deleteAt is when a purged entry is garbage-collected (set when lifetime
	// hits 0; zero while the entry is live).
	//
	// LOCK-ONLY, same as recvPurgeReflooded: written post-publication by
	// markPurgedLocked and read by the aging tick, both under the write lock.
	// Do NOT add an accessor.
	deleteAt time.Time

	// srm / ssn are the per-circuit Send-Routeing-Message and Send-Sequence-
	// Number flag sets (ISO/IEC 10589 clause 7.3.4/7.3.5). The LSDB owns the
	// storage; the flooding spec (isis-7) drives them. A map (not a fixed bitmap)
	// keeps the entry compact when few circuits are flagged and lets a circuit be
	// removed cleanly (ClearCircuit).
	srm map[CircuitID]struct{}
	ssn map[CircuitID]struct{}

	// srmSent records, per circuit, that the LSP has already been transmitted on
	// that circuit since SRM was most recently armed. It distinguishes the FIRST
	// flood of an armed LSP from a true RE-send: ze_isis_srm_resends_total counts
	// only the unacknowledged retransmissions on the periodic timer (the 2nd and
	// later sends while SRM stays set), not the first send. SetSRM clears the
	// circuit's bit when it (re-)arms, so a freshly armed LSP's first send is never
	// miscounted as a resend (ISO/IEC 10589 clause 7.3.15.1: periodic
	// retransmission while SRM remains set).
	srmSent map[CircuitID]struct{}
}

// LSPID returns the entry's LSP ID.
func (e *Entry) LSPID() types.LSPID { return e.id }

// Sequence returns the entry's LSP Sequence Number.
func (e *Entry) Sequence() types.SequenceNumber { return e.sequence }

// Lifetime returns the entry's current Remaining Lifetime in seconds. Safe to
// call without the LSDB lock (see the lifetime field).
func (e *Entry) Lifetime() types.RemainingLifetime {
	return types.RemainingLifetime(e.lifetime.Load())
}

// setLifetime stores the Remaining Lifetime. Callers hold the LSDB write lock;
// the atomic is for the benefit of unlocked READERS, not for write ordering.
func (e *Entry) setLifetime(l types.RemainingLifetime) { e.lifetime.Store(uint32(l)) }

// Checksum returns the entry's stored Fletcher checksum.
func (e *Entry) Checksum() uint16 { return e.checksum }

// IsOverloaded reports whether the LSP-database-overload (OL) bit is set in the
// type block (RFC 3787 sec 4): the originator is reachable but must not be used
// as a transit router (enforced in isis-9 SPF).
func (e *Entry) IsOverloaded() bool { return e.typeBlock&packet.LSPFlagOverload != 0 }

// IsOwn reports whether this node originated the LSP.
func (e *Entry) IsOwn() bool { return e.own }

// IsPurged reports whether the entry is in the zero-age purge state (Remaining
// Lifetime 0, retained for the grace period).
func (e *Entry) IsPurged() bool { return e.purged.Load() }

// Raw returns the verbatim PDU bytes for re-flood (isis-7). The slice is the
// entry's owned copy; callers MUST NOT mutate it (the flooding path only reads
// and frames it). It is returned directly (not copied) because the flooding hot
// path sends it unchanged; the LSDB never mutates raw in place after store.
func (e *Entry) Raw() []byte { return e.raw }

// Decode parses the stored raw bytes into the typed LSP on demand (lazy parse,
// ai/rules/performance.md). SPF (isis-9) and `show isis database detail`
// (isis-13) call this to read TLVs; the LSDB never holds the parsed form. The
// returned LSP's TLV value slices alias the entry's raw bytes, which are stable
// for the entry's lifetime, so the caller need not copy unless it outlives the
// entry. A malformed stored PDU (which the codec validated before store) returns
// the codec error.
func (e *Entry) Decode() (packet.LSP, error) {
	pdu, err := packet.DecodePDU(e.raw)
	if err != nil {
		return packet.LSP{}, err
	}
	if pdu.LSP == nil {
		return packet.LSP{}, packet.ErrUnknownPDUType
	}
	return *pdu.LSP, nil
}

// Freshness is the result of comparing a received LSP against the stored entry
// (ISO/IEC 10589 clause 7.3.15/7.3.16): the received copy is Newer, Equal, or
// Older than what the database holds.
type Freshness uint8

// Freshness outcomes.
const (
	// Older means the received LSP is staler than the stored entry; the database
	// keeps its copy and (isis-7) sends the newer copy back on the receiving
	// circuit (SRM on that circuit).
	Older Freshness = iota
	// Equal means the received LSP matches the stored entry; the database updates
	// the Remaining Lifetime and (isis-7) acknowledges via PSNP (SSN on the
	// receiving circuit).
	Equal
	// Newer means the received LSP supersedes the stored entry; it replaces the
	// stored copy and (isis-7) floods it on all other circuits (SRM).
	Newer
)

// compareFreshness compares an incoming (sequence, lifetime, checksum) tuple
// against the stored entry per ISO/IEC 10589 clause 7.3.16:
//
//   - A higher Sequence Number is unambiguously newer.
//   - On EQUAL sequence numbers, an LSP with Remaining Lifetime 0 (a purge) is
//     treated as newer than one with a non-zero lifetime: the purge must win so
//     a node cannot resurrect an LSP the originator has withdrawn (clause
//     7.3.16.1). When neither or both are purges and the checksums match, the
//     two are Equal (a duplicate, to be acknowledged). A differing checksum at
//     the same sequence is a corrupted/ambiguous copy; the stored entry is kept
//     (Older for the incoming) so a bit-flipped duplicate cannot displace a good
//     LSP -- the originator will bump the sequence to resolve it.
//
// in* is the incoming LSP; the receiver is the stored entry.
func (e *Entry) compareFreshness(inSeq types.SequenceNumber, inLifetime types.RemainingLifetime, inChecksum uint16) Freshness {
	switch {
	case inSeq > e.sequence:
		return Newer
	case inSeq < e.sequence:
		return Older
	}
	// Equal sequence numbers: the purge tiebreak (clause 7.3.16.1).
	inPurge := inLifetime.IsPurge()
	havePurge := e.Lifetime().IsPurge()
	switch {
	case inPurge && !havePurge:
		return Newer
	case !inPurge && havePurge:
		return Older
	case inChecksum == e.checksum:
		return Equal
	default:
		// Same sequence, differing checksum, same purge state: keep ours.
		return Older
	}
}
