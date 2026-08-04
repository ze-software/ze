// Design: plan/learned/932-isis-6-lsdb.md -- the Link-State Database store.
// ISO/IEC 10589 clause 7.3: the LSDB is the per-level record of every LSP the
// node knows. It is held separately for Level 1 and Level 2.
//
// RFC: rfc/short/rfc3787.md -- overload bit surfaced in the snapshot.
//
// The LSDB stores each LSP per Ze's buffer-first philosophy (entry.go): raw PDU
// bytes plus parsed freshness metadata, TLVs parsed lazily. A single RWMutex
// makes the store the single writer (spec R-5): the aging tick, origination, and
// the receive path all mutate under the write lock; snapshot reads take the read
// lock so the CLI never blocks a writer for long. This spec owns the store, the
// freshness compare, the per-circuit SRM/SSN flag model, and the snapshot;
// flooding (isis-7) drives the flags and the wire, SPF (isis-9) reads the store.

package lsdb

import (
	"sort"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// Level is the routing level a database holds LSPs for (1 or 2). The LSDB keeps
// one independent database per level; an L1L2 node populates both.
type Level uint8

// Database levels.
const (
	Level1 Level = 1
	Level2 Level = 2
)

// String renders the level as the metric/CLI token "l1"/"l2".
func (l Level) String() string {
	if l == Level2 {
		return "l2"
	}
	return "l1"
}

// MaxLSPsPerLevel caps the number of LSPs stored per level so a flood of
// distinct LSP IDs from a misbehaving or hostile peer cannot exhaust memory
// (security review: resource exhaustion). A real area never approaches this; an
// own LSP set is at most 256 fragments. A received LSP for a brand-new LSP ID is
// rejected once the level is full (an existing entry is always updatable so a
// refresh/purge of a known LSP is never dropped).
const MaxLSPsPerLevel = 16384

// db is one per-level database keyed by LSP ID.
//
// idsSorted caches the LSP IDs of entries in LSP-ID (CSNP range) order so the
// hot flooding/SNP paths (LSPIDs, called per circuit per level per FloodTick,
// per buildPSNP, per reconcileCSNPRange) do not re-sort the whole key set on
// every call. It is rebuilt eagerly whenever the KEY SET changes (a new LSP ID
// stored, or an entry deleted) -- always under the LSDB write lock, so a reader
// holding the read lock copies a stable, already-sorted slice. Overwriting an
// existing key (a freshness replace/refresh) does NOT change the key set and
// leaves the cache valid.
type db struct {
	entries   map[types.LSPID]*Entry
	idsSorted []types.LSPID
}

func newDB() *db { return &db{entries: make(map[types.LSPID]*Entry)} }

// rebuildIDsLocked recomputes the sorted LSP-ID cache from the current entries.
// The caller holds the LSDB write lock. Called only when the key set changed.
func (s *db) rebuildIDsLocked() {
	ids := make([]types.LSPID, 0, len(s.entries))
	for id := range s.entries {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].Less(ids[j]) })
	s.idsSorted = ids
}

// LSDB is the two-level Link-State Database.
type LSDB struct {
	mu sync.RWMutex
	l1 *db
	l2 *db

	// now supplies the current time (time.Now in production, a fake clock in
	// tests) so the purge grace period and aging are testable without sleeping.
	now func() time.Time

	// Per-level metrics (umbrella canonical series, owner isis-6). lsps is a
	// gauge of the current LSDB size; fragments a gauge of own LSP fragments;
	// originations/wraps/purges are counters. Registered via SetMetrics; a
	// no-op registry leaves them inert.
	mLSPs         metrics.GaugeVec   // ze_isis_lsps{level}
	mFragments    metrics.GaugeVec   // ze_isis_lsp_fragments{level}
	mOriginations metrics.CounterVec // ze_isis_lsp_originations_total{level}
	mWraps        metrics.CounterVec // ze_isis_sequence_wraps_total{level}
	mPurges       metrics.CounterVec // ze_isis_purges_total{level}
}

// New constructs an empty LSDB. now may be nil (defaults to time.Now). Metrics
// start as no-ops until SetMetrics wires a real registry.
func New(now func() time.Time) *LSDB {
	if now == nil {
		now = time.Now
	}
	nop := metrics.NopRegistry{}
	return &LSDB{
		l1:            newDB(),
		l2:            newDB(),
		now:           now,
		mLSPs:         nop.GaugeVec("", "", nil),
		mFragments:    nop.GaugeVec("", "", nil),
		mOriginations: nop.CounterVec("", "", nil),
		mWraps:        nop.CounterVec("", "", nil),
		mPurges:       nop.CounterVec("", "", nil),
	}
}

// SetMetrics registers the LSDB-owned Prometheus series on reg. This spec OWNS
// and registers exactly these rows from the umbrella canonical Metrics table
// (owner isis-6); other ze_isis_* series are registered by their owning specs.
func (d *LSDB) SetMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	d.mu.Lock()
	d.mLSPs = reg.GaugeVec("ze_isis_lsps", "Current number of LSPs in the IS-IS link-state database, by level.", []string{"level"})
	d.mFragments = reg.GaugeVec("ze_isis_lsp_fragments", "Current number of own-LSP fragments in the IS-IS link-state database, by level.", []string{"level"})
	d.mOriginations = reg.CounterVec("ze_isis_lsp_originations_total", "Total IS-IS own-LSP originations (including refreshes and regenerations), by level.", []string{"level"})
	d.mWraps = reg.CounterVec("ze_isis_sequence_wraps_total", "Total IS-IS LSP sequence-number wraparounds, by level.", []string{"level"})
	d.mPurges = reg.CounterVec("ze_isis_purges_total", "Total IS-IS LSPs that reached zero Remaining Lifetime and were purged, by level.", []string{"level"})
	d.mu.Unlock()
	d.publishSizeMetrics()
}

// dbFor returns the per-level database for level (defaulting L1 for any value
// other than Level2). The caller holds the appropriate lock.
func (d *LSDB) dbFor(level Level) *db {
	if level == Level2 {
		return d.l2
	}
	return d.l1
}

// ReceiveResult reports what the freshness compare decided for a received LSP so
// the flooding spec (isis-7) can set SRM/SSN per ISO/IEC 10589 clause 7.3.16.
type ReceiveResult struct {
	// Freshness is the compare outcome (Newer / Equal / Older).
	Freshness Freshness
	// Stored is true when the received LSP was actually written to the database
	// (Newer, or a first sighting). Equal updates only the lifetime; Older stores
	// nothing.
	Stored bool
}

// Receive applies a received, codec-validated LSP to the database (the data path
// the flooding spec, isis-7, calls after the wire codec has parsed the header
// and validated lengths). It compares the incoming LSP against any stored entry
// (clause 7.3.16):
//
//   - Newer: replace the stored copy (a single OWNED copy of raw is taken so the
//     transport may reuse its receive buffer) and report Stored.
//   - Equal: refresh the stored Remaining Lifetime to the received value and
//     acknowledge (no re-store of bytes).
//   - Older: keep the stored copy untouched.
//
// A received LSP with Remaining Lifetime 0 is a purge: it is stored (when newer)
// and marked receivedPurge so the aging path re-floods and retains it for the
// grace period rather than garbage-collecting it (clause 7.3.16/17, spec AC-9).
// own marks an LSP whose System ID is ours (the originator); the engine passes
// this so a received copy of our own LSP is recognized.
//
// raw is the verbatim PDU; Receive copies it (never aliases the caller's buffer).
func (d *LSDB) Receive(level Level, lsp *packet.LSP, raw []byte, own bool) ReceiveResult {
	d.mu.Lock()
	defer d.mu.Unlock()

	store := d.dbFor(level)
	existing, ok := store.entries[lsp.LSPID]
	if ok {
		fr := existing.compareFreshness(lsp.SequenceNumber, lsp.RemainingLifetime, lsp.Checksum)
		switch fr {
		case Newer:
			d.replaceLocked(level, store, existing.LSPID(), lsp, raw, own, true)
			return ReceiveResult{Freshness: Newer, Stored: true}
		case Equal:
			// Clause 7.3.16: a duplicate refreshes the held Remaining Lifetime.
			existing.setLifetime(lsp.RemainingLifetime)
			return ReceiveResult{Freshness: Equal, Stored: false}
		default:
			return ReceiveResult{Freshness: Older, Stored: false}
		}
	}
	// First sighting of this LSP ID.
	if len(store.entries) >= MaxLSPsPerLevel {
		// Level full: reject a brand-new LSP ID (an existing one is always
		// updatable above). Treated as Older so isis-7 does not flood it.
		return ReceiveResult{Freshness: Older, Stored: false}
	}
	d.replaceLocked(level, store, lsp.LSPID, lsp, raw, own, true)
	d.publishSizeMetricLocked(level)
	return ReceiveResult{Freshness: Newer, Stored: true}
}

// replaceLocked writes (or overwrites) the entry for id in store from a parsed
// LSP and its verbatim bytes. received marks the bytes as arriving on the wire
// (vs originated): a received Remaining-Lifetime-0 LSP becomes a receivedPurge.
// The caller holds the write lock. raw is copied into an entry-owned slice so no
// alias of a reused buffer survives (security review: memory safety). When this
// write newly transitions the LSP ID into the purged state (a received or
// originated purge of an LSP that was not already purged), the per-level
// ze_isis_purges_total counter is bumped so it counts purge events on every path
// (the aging-to-zero path counts in markPurgedLocked).
func (d *LSDB) replaceLocked(level Level, store *db, id types.LSPID, lsp *packet.LSP, raw []byte, own, received bool) {
	prev := store.entries[id]
	wasPurged := prev != nil && prev.IsPurged()
	wasReceivedPurge := prev != nil && prev.receivedPurge
	owned := make([]byte, len(raw))
	copy(owned, raw)
	e := &Entry{
		id:        id,
		raw:       owned,
		sequence:  lsp.SequenceNumber,
		checksum:  lsp.Checksum,
		typeBlock: lsp.TypeBlock,
		own:       own,
	}
	e.setLifetime(lsp.RemainingLifetime)
	if lsp.RemainingLifetime.IsPurge() {
		e.purged.Store(true)
		// A purge is a received purge when this write arrived on the wire OR the
		// entry it replaces was already a received purge (ISO/IEC 10589 clause
		// 7.3.16: a received purge is re-flooded and retained distinctly from a
		// local expiry). Preserving the prior flag keeps the received-purge
		// distinction across a replace so the engine still re-floods it (AC-9, R-2).
		e.receivedPurge = received || wasReceivedPurge
		e.deleteAt = d.now().Add(ZeroAgeLifetime)
		if !wasPurged {
			d.mPurges.With(level.String()).Inc()
		}
	}
	// Preserve the flooding flags across a replace: a newer version still needs
	// to be (re-)flooded, and isis-7 owns clearing them. Start fresh sets so the
	// caller (isis-7) re-arms SRM for the new version (and the per-circuit
	// already-sent markers start empty so the new version's first send is a first
	// send, not a resend).
	e.srm = make(map[CircuitID]struct{})
	e.ssn = make(map[CircuitID]struct{})
	e.srmSent = make(map[CircuitID]struct{})
	store.entries[id] = e
	if prev == nil {
		// A brand-new LSP ID: the key set changed, so refresh the sorted-ID cache.
		// An overwrite (prev != nil) leaves the key set -- and the cache -- intact.
		store.rebuildIDsLocked()
	}
}

// Insert stores an originated own LSP (the origination path, origination.go).
// Unlike Receive it performs no freshness compare: the originator is
// authoritative for its own LSP IDs and assigns monotonically increasing
// sequence numbers. raw is copied into an entry-owned slice. An originated
// Remaining-Lifetime-0 LSP is a locally-generated purge (NOT a receivedPurge):
// it is re-flooded and retained for the grace period, then garbage-collected.
func (d *LSDB) Insert(level Level, lsp *packet.LSP, raw []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	store := d.dbFor(level)
	_, existed := store.entries[lsp.LSPID]
	d.replaceLocked(level, store, lsp.LSPID, lsp, raw, true, false)
	if !existed {
		d.publishSizeMetricLocked(level)
	}
}

// Lookup returns the stored entry for id at level, or nil. The returned pointer
// is the LIVE entry, so a caller holding it after Lookup returns holds no lock.
//
// What is safe to call on it without the LSDB lock: the metadata accessors
// (LSPID, Sequence, Checksum, IsOverloaded, IsOwn, Raw, Decode) because
// replaceLocked writes those fields once, before the entry is reachable; and
// Lifetime/IsPurged because those two fields are atomic precisely for this.
// The aging tick, markPurgedLocked and the clause 7.3.16 duplicate refresh
// write them.
//
// Those two are the only ACCESSOR-REACHABLE fields mutated after publication.
// They are NOT the only fields mutated after publication. recvPurgeReflooded,
// deleteAt and the srm/ssn/srmSent maps are too. Those stay plain because no
// accessor reaches them. Read the FIELD DISCIPLINE note on Entry for the
// enumeration. This paragraph is not one.
//
// What is NOT safe: mutating anything, and reading any other mutable field
// directly rather than through an accessor. The earlier wording here -- that
// "callers that only read metadata are fine" -- was the invitation that
// produced a DATA RACE between the aging tick and SNP generation: a read races
// a concurrent write whether or not the reader also writes.
//
// (isis-7 flooding uses the flag methods, which lock internally; SPF uses
// Snapshot/Decode.)
func (d *LSDB) Lookup(level Level, id types.LSPID) *Entry {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.dbFor(level).entries[id]
}

// Delete removes the entry for id at level, returning whether it existed. Used
// by the aging path after the purge grace period and by the engine on shutdown.
func (d *LSDB) Delete(level Level, id types.LSPID) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	store := d.dbFor(level)
	if _, ok := store.entries[id]; !ok {
		return false
	}
	delete(store.entries, id)
	store.rebuildIDsLocked() // key set changed: refresh the sorted-ID cache.
	d.publishSizeMetricLocked(level)
	return true
}

// Len returns the number of LSPs stored at level.
func (d *LSDB) Len(level Level) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.dbFor(level).entries)
}

// LSPIDs returns the LSP IDs stored at level, sorted in LSP-ID (CSNP range)
// order. It is the read-only enumerator the flooding spec (isis-7) uses to drain
// SRM flags and to reconcile a CSNP range without holding a live entry pointer:
// the caller then uses SRM/SSN/Lookup by ID. Sorting makes the flood/CSNP order
// deterministic (ISO/IEC 10589 clause 9.10 CSNP ordering). It takes the read lock
// and returns a COPY of the cached sorted-ID slice (idsSorted), so it never sorts
// on the hot path (the sort happens once per key-set change, under the write
// lock), never blocks the single writer for long, and never exposes the store or
// the live cache slice (the caller may iterate while a later writer rebuilds it).
func (d *LSDB) LSPIDs(level Level) []types.LSPID {
	d.mu.RLock()
	defer d.mu.RUnlock()
	cached := d.dbFor(level).idsSorted
	out := make([]types.LSPID, len(cached))
	copy(out, cached)
	return out
}

// LSPEntries returns one packet.LSPEntry (TLV 9 record) per LSP at level, in
// LSP-ID (CSNP range) order, built directly from the typed entry metadata under
// a single read lock. It is the source for CSNP/PSNP build (isis-7): no string
// round-trip (the old Snapshot-then-ParseLSPID path stringified every LSP ID and
// reparsed it), and one lock acquisition for the whole set rather than a Lookup
// per ID. LSPEntry is a small fixed value with no pointer into the store, so the
// returned slice never exposes a live entry (ISO/IEC 10589 clause 9.10/9.14).
func (d *LSDB) LSPEntries(level Level) []packet.LSPEntry {
	d.mu.RLock()
	defer d.mu.RUnlock()
	store := d.dbFor(level)
	out := make([]packet.LSPEntry, 0, len(store.idsSorted))
	for _, id := range store.idsSorted {
		e := store.entries[id]
		if e == nil {
			continue // cache/entries momentarily divergent: skip defensively.
		}
		out = append(out, packet.LSPEntry{
			RemainingLifetime: e.Lifetime(),
			LSPID:             id,
			SequenceNumber:    e.sequence,
			Checksum:          e.checksum,
		})
	}
	return out
}

// ---- Per-circuit SRM/SSN flags (ISO/IEC 10589 clause 7.3.4/7.3.5) ----
//
// The flooding spec (isis-7) drives these; the LSDB only stores and exposes
// them. Each is a per-LSP, per-circuit boolean. SetSRM/SetSSN arm a flag,
// ClearSRM/ClearSSN clear it, and SRM/SSN query it. ClearCircuit drops a
// circuit's flags from every LSP when the circuit goes away (so a closed
// circuit's index is never re-flagged). All take the LSDB lock.

// SetSRM arms the Send-Routeing-Message flag for (level, id) on circuit cid: the
// LSP must be (re-)sent on that circuit. No-op when the LSP is absent. Arming (or
// re-arming) SRM clears the circuit's "already sent since armed" marker so the
// NEXT transmit counts as a first send, not a resend (ISO/IEC 10589 clause
// 7.3.15.1: the periodic-retransmission counter tracks unacknowledged re-sends).
func (d *LSDB) SetSRM(level Level, id types.LSPID, cid CircuitID) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if e := d.dbFor(level).entries[id]; e != nil {
		e.srm[cid] = struct{}{}
		delete(e.srmSent, cid)
	}
}

// noteSRMTransmit records that (level, id) was just transmitted on circuit cid
// and reports whether this is a RE-send (a transmit while SRM stayed set, with a
// prior transmit since SRM was last armed) versus the FIRST send after arming.
// The flood timer (flooding.go) calls it right after a successful transmit so
// ze_isis_srm_resends_total counts only the 2nd-and-later unacknowledged sends
// (ISO/IEC 10589 clause 7.3.15.1). It takes the write lock (it mutates srmSent).
func (d *LSDB) noteSRMTransmit(level Level, id types.LSPID, cid CircuitID) (resend bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	e := d.dbFor(level).entries[id]
	if e == nil {
		return false
	}
	if e.srmSent == nil {
		e.srmSent = make(map[CircuitID]struct{})
	}
	if _, sent := e.srmSent[cid]; sent {
		return true // a prior send since SRM was armed: this is a true resend.
	}
	e.srmSent[cid] = struct{}{}
	return false // first send since SRM was armed: not counted as a resend.
}

// ClearSRM clears the SRM flag for (level, id) on circuit cid (isis-7 clears it
// once the LSP has been sent and, on a P2P link, acknowledged).
func (d *LSDB) ClearSRM(level Level, id types.LSPID, cid CircuitID) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if e := d.dbFor(level).entries[id]; e != nil {
		delete(e.srm, cid)
	}
}

// SRM reports whether the SRM flag is set for (level, id) on circuit cid.
func (d *LSDB) SRM(level Level, id types.LSPID, cid CircuitID) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if e := d.dbFor(level).entries[id]; e != nil {
		_, ok := e.srm[cid]
		return ok
	}
	return false
}

// SetSSN arms the Send-Sequence-Number flag for (level, id) on circuit cid: a
// PSNP acknowledging/requesting the LSP must be sent on that circuit.
func (d *LSDB) SetSSN(level Level, id types.LSPID, cid CircuitID) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if e := d.dbFor(level).entries[id]; e != nil {
		e.ssn[cid] = struct{}{}
	}
}

// ClearSSN clears the SSN flag for (level, id) on circuit cid.
func (d *LSDB) ClearSSN(level Level, id types.LSPID, cid CircuitID) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if e := d.dbFor(level).entries[id]; e != nil {
		delete(e.ssn, cid)
	}
}

// SSN reports whether the SSN flag is set for (level, id) on circuit cid.
func (d *LSDB) SSN(level Level, id types.LSPID, cid CircuitID) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if e := d.dbFor(level).entries[id]; e != nil {
		_, ok := e.ssn[cid]
		return ok
	}
	return false
}

// ClearCircuit drops circuit cid from every LSP's SRM/SSN sets at both levels.
// The engine calls it when a circuit closes so a stale circuit index is never
// left flagged (spec: flags cleared on entry removal / circuit removal).
func (d *LSDB) ClearCircuit(cid CircuitID) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, store := range []*db{d.l1, d.l2} {
		for _, e := range store.entries {
			delete(e.srm, cid)
			delete(e.ssn, cid)
			delete(e.srmSent, cid)
		}
	}
}

// ---- Snapshot (`show isis database`, isis-13) ----

// LSPSnapshot is one row of the `show isis database` view (rendered by isis-13).
// It is a flat value with no pointers so it crosses the CLI/RPC boundary cleanly
// (JSON-tagged for the show output). It carries the freshness metadata plus the
// overload flag (spec AC-10).
type LSPSnapshot struct {
	LSPID    string `json:"lsp-id"`
	Sequence uint32 `json:"sequence"`
	Lifetime uint16 `json:"lifetime"`
	Checksum uint16 `json:"checksum"`
	Overload bool   `json:"overload"`
	Purged   bool   `json:"purged,omitempty"`
}

// Snapshot returns a stable, sorted copy of the LSPs at level for the CLI. It
// takes the read lock so it never exposes a live entry pointer across the
// boundary and never blocks the single writer for long. Rows are ordered by LSP
// ID (the CSNP order), so the CLI output is deterministic.
func (d *LSDB) Snapshot(level Level) []LSPSnapshot {
	d.mu.RLock()
	defer d.mu.RUnlock()
	store := d.dbFor(level)
	out := make([]LSPSnapshot, 0, len(store.entries))
	for _, e := range store.entries {
		out = append(out, LSPSnapshot{
			LSPID:    e.id.String(),
			Sequence: uint32(e.sequence),
			Lifetime: e.Lifetime().Seconds(),
			Checksum: e.checksum,
			Overload: e.IsOverloaded(),
			Purged:   e.IsPurged(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LSPID < out[j].LSPID })
	return out
}

// incOriginations bumps ze_isis_lsp_originations_total{level} through the LSDB
// lock. The Originator (origination.go / pseudonode.go) calls it while holding
// its OWN mutex, not d.mu; SetMetrics rebinds the metric handle under d.mu, so
// the handle MUST be read under d.mu to avoid a data race on the interface value
// (the Inc itself is concurrency-safe). The LSDB never acquires the Originator's
// mutex, so taking d.mu here introduces no lock-ordering cycle.
func (d *LSDB) incOriginations(level Level) {
	d.mu.RLock()
	m := d.mOriginations
	d.mu.RUnlock()
	m.With(level.String()).Inc()
}

// incWraps bumps ze_isis_sequence_wraps_total{level} through the LSDB lock, for
// the same reason as incOriginations: the originator reads the handle while
// holding o.mu, while SetMetrics rebinds it under d.mu.
func (d *LSDB) incWraps(level Level) {
	d.mu.RLock()
	m := d.mWraps
	d.mu.RUnlock()
	m.With(level.String()).Inc()
}

// publishSizeMetrics refreshes the per-level size gauges from the current store
// (called after a bulk change such as SetMetrics).
func (d *LSDB) publishSizeMetrics() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.publishSizeMetricLocked(Level1)
	d.publishSizeMetricLocked(Level2)
}

// publishSizeMetricLocked sets the ze_isis_lsps gauge for one level. The caller
// holds the write lock. It also refreshes the own-fragment gauge.
func (d *LSDB) publishSizeMetricLocked(level Level) {
	store := d.dbFor(level)
	frags := 0
	for _, e := range store.entries {
		if e.own {
			frags++
		}
	}
	d.mLSPs.With(level.String()).Set(float64(len(store.entries)))
	d.mFragments.With(level.String()).Set(float64(frags))
}
