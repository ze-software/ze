// Design: docs/architecture/isis/isis-6-lsdb.md -- LSP aging / purge / grace tests.
//
// VALIDATES: the 1s decrement (TestISISLSDBAgeDecrement); lifetime-0 ->
// purged-not-deleted, deleted after the ZeroAgeLifetime grace
// (TestISISLSDBAgeToPurge); and the received-purge vs local-expiry distinction
// (TestISISLSDBPurgeVsExpiry) -- spec AC-2, AC-9, R-2.

package lsdb

import (
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// fakeClock is a settable clock for deterministic aging tests.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func TestISISLSDBAgeDecrement(t *testing.T) {
	d := New(nil)
	id := lspID(1, 0)
	lsp, raw := buildLSP(t, packet.PDUTypeL2LSP, id, 1, 5, nil)
	d.Insert(Level2, lsp, raw)

	// Each Tick decrements the Remaining Lifetime by exactly 1 (clause 7.3.16.4).
	for want := uint16(4); want >= 1; want-- {
		d.Tick()
		if got := d.Lookup(Level2, id).Lifetime().Seconds(); got != want {
			t.Fatalf("after tick, lifetime = %d, want %d", got, want)
		}
	}
}

func TestISISLSDBAgeToPurge(t *testing.T) {
	clk := newFakeClock()
	d := New(clk.now)
	id := lspID(2, 0)
	// Lifetime 2 so two ticks reach 0.
	lsp, raw := buildLSP(t, packet.PDUTypeL2LSP, id, 1, 2, nil)
	d.Insert(Level2, lsp, raw)

	d.Tick() // 2 -> 1
	if d.Lookup(Level2, id) == nil || d.Lookup(Level2, id).IsPurged() {
		t.Fatal("LSP purged too early (lifetime 1)")
	}
	res := d.Tick() // 1 -> 0: purged, NOT deleted
	e := d.Lookup(Level2, id)
	if e == nil {
		t.Fatal("LSP deleted at the instant lifetime hit 0 (must purge, not delete)")
	}
	if !e.IsPurged() {
		t.Error("LSP not marked purged at lifetime 0")
	}
	if len(res.PurgedL2) != 1 || res.PurgedL2[0].LSPID != id.String() {
		t.Errorf("tick did not report the purge: %+v", res.PurgedL2)
	}

	// Within the grace period the entry stays (a node that missed the purge must
	// still see it). Advance just short of ZeroAgeLifetime and tick.
	clk.advance(ZeroAgeLifetime - time.Second)
	d.Tick()
	if d.Lookup(Level2, id) == nil {
		t.Error("purged LSP garbage-collected before the grace period elapsed")
	}

	// After the grace period it is deleted and reported.
	clk.advance(2 * time.Second)
	res = d.Tick()
	if d.Lookup(Level2, id) != nil {
		t.Error("purged LSP not garbage-collected after the grace period")
	}
	if len(res.DeletedL2) != 1 || res.DeletedL2[0].LSPID != id.String() {
		t.Errorf("tick did not report the deletion: %+v", res.DeletedL2)
	}
}

func TestISISLSDBPurgeVsExpiry(t *testing.T) {
	clk := newFakeClock()
	d := New(clk.now)

	// (a) A RECEIVED purge: arrives on the wire at lifetime 0. It must be stored,
	// marked purged, and retained for the grace period (re-flooded by isis-7),
	// NOT dropped on arrival (spec AC-9, clause 7.3.16/17).
	rid := lspID(3, 0)
	rpurge, rraw := buildLSP(t, packet.PDUTypeL2LSP, rid, 5, 0, nil)
	r := d.Receive(Level2, rpurge, rraw, false)
	if !r.Stored {
		t.Fatal("received purge was not stored (must retain to re-flood)")
	}
	e := d.Lookup(Level2, rid)
	if e == nil || !e.IsPurged() {
		t.Fatal("received purge not retained as a purged entry")
	}
	if !e.receivedPurge {
		t.Error("received purge not marked receivedPurge (distinct from local expiry)")
	}

	// (b) A LOCAL expiry: a normal LSP whose lifetime decays to 0 here. It is
	// marked purged (local), retained for the grace period, then collected. Its
	// receivedPurge flag stays false (the distinct path).
	lid := lspID(4, 0)
	llsp, lraw := buildLSP(t, packet.PDUTypeL2LSP, lid, 5, 1, nil)
	d.Insert(Level2, llsp, lraw)
	d.Tick() // 1 -> 0: local expiry becomes a purge
	le := d.Lookup(Level2, lid)
	if le == nil || !le.IsPurged() {
		t.Fatal("locally expired LSP not retained as purged")
	}
	if le.receivedPurge {
		t.Error("locally expired LSP wrongly marked receivedPurge")
	}

	// Both are garbage-collected after the grace period (retention is identical;
	// only the flooding decision differs, which isis-7 owns).
	clk.advance(ZeroAgeLifetime + time.Second)
	d.Tick()
	if d.Lookup(Level2, rid) != nil {
		t.Error("received purge not collected after grace")
	}
	if d.Lookup(Level2, lid) != nil {
		t.Error("local expiry not collected after grace")
	}
}

// TestISISReceivedPurgeRefloodSurfaced asserts the aging tick SURFACES a received
// purge ONCE (with ReceivedPurge=true) so the engine re-arms SRM and re-floods it
// within the grace window, distinctly from a local expiry (ISO/IEC 10589 clause
// 7.3.16, spec AC-9/R-4). The one-shot guard prevents a per-second re-flood storm
// (R-2). Regression for finding B2-5: the receivedPurge flag was set but never
// read, so the engine could not re-flood received purges distinctly.
func TestISISReceivedPurgeRefloodSurfaced(t *testing.T) {
	clk := newFakeClock()
	d := New(clk.now)

	// A received purge (lifetime 0 on the wire) and a local expiry (lifetime 1,
	// decays to 0 on the first tick).
	rid := lspID(7, 0)
	rpurge, rraw := buildLSP(t, packet.PDUTypeL2LSP, rid, 5, 0, nil)
	if r := d.Receive(Level2, rpurge, rraw, false); !r.Stored {
		t.Fatal("received purge not stored")
	}
	if !d.Lookup(Level2, rid).receivedPurge {
		t.Fatal("receivedPurge false for a wire-received purge")
	}

	lid := lspID(8, 0)
	llsp, lraw := buildLSP(t, packet.PDUTypeL2LSP, lid, 5, 1, nil)
	d.Insert(Level2, llsp, lraw)
	if d.Lookup(Level2, lid).receivedPurge {
		t.Fatal("receivedPurge true for a locally-originated LSP")
	}

	// First tick: the received purge is surfaced for re-flood (ReceivedPurge=true)
	// and the local LSP expires to a purge (ReceivedPurge=false).
	res := d.Tick()
	var sawReceived, sawLocal bool
	for _, p := range res.PurgedL2 {
		switch p.LSPID {
		case rid.String():
			sawReceived = true
			if !p.ReceivedPurge {
				t.Error("received purge surfaced with ReceivedPurge=false (must be true, AC-9)")
			}
		case lid.String():
			sawLocal = true
			if p.ReceivedPurge {
				t.Error("local expiry surfaced with ReceivedPurge=true (must be false, AC-9)")
			}
		}
	}
	if !sawReceived {
		t.Error("aging tick did not surface the received purge for re-flood (finding B2-5)")
	}
	if !sawLocal {
		t.Error("aging tick did not surface the local expiry as a purge")
	}

	// Second tick (still within the grace window): the received purge is NOT
	// surfaced again -- the one-shot guard prevents a per-second re-flood storm
	// (R-2). The already-purged local expiry is likewise not re-surfaced.
	res = d.Tick()
	for _, p := range res.PurgedL2 {
		if p.LSPID == rid.String() {
			t.Error("received purge surfaced a SECOND time (re-flood storm, R-2)")
		}
		if p.LSPID == lid.String() {
			t.Error("local expiry re-surfaced after it was already purged")
		}
	}
}

// TestISISLSDBEntryAccessorsAreRaceFree drives the aging tick and the unlocked
// metadata accessors concurrently, which is the exact shape that produced the
// reported DATA RACE.
//
// VALIDATES: fixit-isis-lsdb-entry-race -- every exported *Entry accessor is
// safe to call without the LSDB lock, which for Lifetime() and IsPurged() holds
// only because those two fields are atomic.
//
// PREVENTS: the shipped race. Lookup hands out a LIVE *Entry and its doc invited
// unlocked metadata reads ("callers that only read metadata are fine"), but the
// aging tick decrements Entry.lifetime and markPurgedLocked sets Entry.purged on
// entries that are already published. Reading a field another goroutine is
// writing is a race whether or not the reader mutates, so SNP generation on the
// flooding goroutine raced the tick.
//
// It does NOT rely on TestISISDISElection, which surfaced the race only by
// scheduling luck and did not reproduce on demand.
//
// EVERY exported accessor is read, not just the two atomic ones. The field
// discipline documented on the Entry struct is a claim about the whole accessor
// set (an accessor is the evidence for "read without the lock"), so the test
// exercises the whole set: an accessor added later over a field that is mutated
// after publication fails here rather than shipping.
//
// The writer RE-ORIGINATES the LSPs once the tick reports them purged, and that
// is load-bearing rather than tidiness. All eight LSPs reach the purge state
// within the first few tick iterations; markPurgedLocked then early-returns and
// the tick writes no entry field at all for the remaining ~150 ms, so the
// reader/writer overlap this test exists to create collapses to a few
// microseconds. Measured on 2026-07-27 against the pre-fix code (lifetime
// reverted to a plain uint32), the version without the re-arm caught the
// regression in only 6 of 12 single runs -- a coin flip in a -count=1 suite,
// which is what ze-verify runs. Re-originating keeps both post-publication
// writes (the decrement and markPurgedLocked) flowing for the whole run.
func TestISISLSDBEntryAccessorsAreRaceFree(t *testing.T) {
	d := New(nil)

	// A SHORT lifetime on purpose: the run must reach markPurgedLocked often, so
	// the purged write is exercised as heavily as the lifetime decrement. A long
	// lifetime would cover the decrement well and the purge hardly at all.
	const lsps = 8
	const lifetime = 3
	ids := make([]types.LSPID, 0, lsps)
	built := make([]*packet.LSP, 0, lsps)
	raws := make([][]byte, 0, lsps)
	for i := range lsps {
		id := lspID(uint8(i+1), 0)
		lsp, raw := buildLSP(t, packet.PDUTypeL2LSP, id, 1, lifetime, nil)
		d.Insert(Level2, lsp, raw)
		ids = append(ids, id)
		// Kept for the writer's re-origination below. buildLSP needs *testing.T,
		// which may only be used from the test goroutine, so the PDUs are built
		// here once rather than inside the writer.
		built = append(built, lsp)
		raws = append(raws, raw)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Writer: the aging tick -- setLifetime on the decrement path and
	// markPurgedLocked on the crossing-to-zero path. These are the two writes
	// that land on an ALREADY-PUBLISHED entry, which is condition 1 of the field
	// discipline.
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				if res := d.Tick(); len(res.PurgedL2) > 0 {
					// Re-originate: a database of purged entries is a database
					// the tick no longer writes to (see the note above).
					for i := range ids {
						d.Insert(Level2, built[i], raws[i])
					}
				}
			}
		}
	})

	// Readers: the unlocked reads that SNP generation, show and SPF perform on
	// the live pointer Lookup returns, after the LSDB read lock is released.
	for range 4 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					for _, id := range ids {
						e := d.Lookup(Level2, id)
						if e == nil {
							continue // garbage-collected after the grace period
						}
						_ = e.Lifetime()
						_ = e.IsPurged()
						_ = e.Sequence()
						_ = e.Checksum()
						_ = e.LSPID()
						_ = e.IsOverloaded()
						_ = e.isOwn()
						_ = e.Raw()
						if lsp, err := e.Decode(); err == nil {
							packet.ReleaseTLVs(lsp.TLVs)
						}
					}
				}
			}
		})
	}

	time.Sleep(150 * time.Millisecond) // let the goroutines interleave; -race is the oracle
	close(stop)
	wg.Wait()
}
