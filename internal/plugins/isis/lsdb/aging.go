// Design: plan/learned/932-isis-6-lsdb.md -- LSP aging, refresh, and zero-age purge.
// ISO/IEC 10589 clause 7.3.16/7.3.17: Remaining Lifetime decrements once per
// second; an LSP that reaches 0 is a purge -- re-flooded and retained for
// ZeroAgeLifetime, NOT deleted at once -- so a node that missed the purge cannot
// keep a stale copy. clause 7.3.3: sequence-number wraparound suspends
// origination for MaxAge + ZeroAgeLifetime.
//
// This file owns the lifecycle timers: the 1s decrement (Tick), the own-LSP
// refresh decision (Refresh* drive origination.go), the purge garbage-collection
// after the grace period, and the distinction between a received purge (re-flood
// + retain) and a local expiry (garbage-collect) -- spec AC-9, R-2.

package lsdb

import "time"

// Aging timer constants (ISO/IEC 10589 clause 7.3). The MaxAge / refresh values
// are operator-configurable (the lsp-lifetime / lsp-refresh-interval YANG leaves
// resolved by isis-4); these are the RFC defaults the engine passes when no
// override is set. ZeroAgeLifetime is fixed.
const (
	// DefaultMaxAge is the default LSP Remaining Lifetime at origination
	// (ISO/IEC 10589 clause 7.3.16.1: MaxAge, default 1200 s).
	DefaultMaxAge = 1200 * time.Second

	// DefaultRefreshInterval is the default age at which an own LSP is refreshed
	// before MaxAge (clause 7.3.16.1, typically 900 s). The originator bumps the
	// sequence and resets the lifetime so the LSP never ages out in the network.
	DefaultRefreshInterval = 900 * time.Second

	// ZeroAgeLifetime is the grace period a purged LSP (Remaining Lifetime 0) is
	// kept in the database after the purge is flooded, before garbage collection
	// (ISO/IEC 10589 clause 7.3.16.4 / 7.3.17). It is also part of the
	// post-wraparound suspension window. RFC default 60 s.
	ZeroAgeLifetime = 60 * time.Second
)

// TickResult reports the effect of one 1s aging tick so the engine can drive the
// follow-on actions this spec does NOT own: flooding a fresh purge (isis-7) and
// recomputing SPF (isis-9). Per-level lists keep the engine's wiring simple.
type TickResult struct {
	// Purged lists LSP IDs (per level) that reached Remaining Lifetime 0 on this
	// tick and must now be flooded as a purge (isis-7 sets SRM). They remain in
	// the database for the grace period.
	PurgedL1 []PurgeEvent
	PurgedL2 []PurgeEvent
	// DeletedL1 / DeletedL2 list LSP IDs garbage-collected on this tick (their
	// grace period elapsed). SPF (isis-9) drops them from the graph.
	DeletedL1 []deletedEvent
	DeletedL2 []deletedEvent
}

// PurgeEvent names an LSP the tick reports for flooding, with whether it was a
// local expiry or a received purge so the engine can decide flooding behavior
// (ISO/IEC 10589 clause 7.3.16: a received purge is re-flooded and retained; a
// local expiry is only garbage-collected). own marks our own LSP (the engine
// re-originates a suspended own LSP only after the wraparound window; an aged-out
// own LSP is normally refreshed long before 0). ReceivedPurge is true when the
// purge arrived on the wire (the engine re-arms SRM to re-flood it once within
// the grace window, distinct from a local expiry).
type PurgeEvent struct {
	LSPID         string
	Own           bool
	ReceivedPurge bool
}

// deletedEvent names an LSP garbage-collected after its grace period.
type deletedEvent struct {
	LSPID string
}

// Tick advances the database by one second (ISO/IEC 10589 clause 7.3.16.4): it
// decrements every entry's Remaining Lifetime, transitions an entry that reaches
// 0 to the purged state (re-flood + retain, NOT delete), and garbage-collects a
// purged entry whose ZeroAgeLifetime grace period has elapsed. It returns the
// purge and deletion events for the engine to flood / recompute. Single-writer:
// it takes the LSDB write lock.
func (d *LSDB) Tick() TickResult {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.now()
	var res TickResult
	res.PurgedL1, res.DeletedL1 = d.tickLevelLocked(Level1, now)
	res.PurgedL2, res.DeletedL2 = d.tickLevelLocked(Level2, now)
	return res
}

// tickLevelLocked ages one level's database by one second. The caller holds the
// write lock. It returns the LSPs newly purged and those garbage-collected.
func (d *LSDB) tickLevelLocked(level Level, now time.Time) (purged []PurgeEvent, deleted []deletedEvent) {
	store := d.dbFor(level)
	sizeChanged := false
	for id, e := range store.entries {
		switch {
		case e.IsPurged():
			// Already purged: garbage-collect once the grace period elapses
			// (clause 7.3.17). A received purge and a local expiry are both
			// collected here, but only after the grace window -- they differ in
			// flooding (isis-7), not in retention.
			if !e.deleteAt.IsZero() && !now.Before(e.deleteAt) {
				delete(store.entries, id)
				deleted = append(deleted, deletedEvent{LSPID: id.String()})
				sizeChanged = true
				break
			}
			// A received purge still within its grace window is surfaced ONCE so
			// the engine re-arms SRM and re-floods it (ISO/IEC 10589 clause 7.3.16:
			// a received purge is re-flooded, distinct from a local expiry). The
			// receive path floods it on arrival; this single tick-driven re-flood
			// lets a neighbor that missed the first flood converge, without a
			// per-second storm (the guard, spec R-2/R-4).
			if e.receivedPurge && !e.recvPurgeReflooded {
				e.recvPurgeReflooded = true
				purged = append(purged, PurgeEvent{LSPID: id.String(), Own: e.own, ReceivedPurge: true})
			}
		case e.Lifetime() == 0:
			// Reached 0 this tick (e.g. stored already at 0): become purged.
			d.markPurgedLocked(level, e, now)
			purged = append(purged, PurgeEvent{LSPID: id.String(), Own: e.own, ReceivedPurge: e.receivedPurge})
		default:
			e.setLifetime(e.Lifetime() - 1)
			if e.Lifetime() == 0 {
				// Crossed to 0: transition to purged (re-flood + retain).
				d.markPurgedLocked(level, e, now)
				purged = append(purged, PurgeEvent{LSPID: id.String(), Own: e.own, ReceivedPurge: e.receivedPurge})
			}
		}
	}
	if sizeChanged {
		// A garbage-collected purge removed a key, so the sorted-ID cache is stale.
		// sizeChanged is set exactly when an entry was deleted above.
		store.rebuildIDsLocked()
		d.publishSizeMetricLocked(level)
	}
	return purged, deleted
}

// markPurgedLocked transitions an entry to the zero-age purged state: lifetime
// pinned at 0, the grace timer armed, and the per-level purge counter bumped.
// The caller holds the write lock. It does NOT clear the SRM/SSN flags: isis-7
// re-arms SRM to flood the purge.
func (d *LSDB) markPurgedLocked(level Level, e *Entry, now time.Time) {
	if e.IsPurged() {
		return
	}
	e.setLifetime(0)
	e.purged.Store(true)
	e.deleteAt = now.Add(ZeroAgeLifetime)
	d.mPurges.With(level.String()).Inc()
}
