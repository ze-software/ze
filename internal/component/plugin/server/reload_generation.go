// Design: docs/architecture/api/commands.md -- show system reload status observability surface
// Related: reload.go -- Server.reloadConfig, the plugin-server half of a reload
// Related: reload_history.go -- ReloadRecord and ReloadTrace types stored under reloadGen.mu

package server

import (
	"sync"
	"time"
)

// Reload outcome strings reported by ReloadStatus. A reload that ran to
// completion is "applied"; one that returned an error is "failed". Both
// advance the generation counter: the counter answers "was a reload
// PROCESSED", not "did it change anything".
const (
	ReloadOutcomeApplied = "applied"
	ReloadOutcomeFailed  = "failed"
	// ReloadOutcomeNone is reported before the first reload is processed,
	// while Generation is still 0.
	ReloadOutcomeNone = "none"
)

// reloadGeneration is the monotonic "reload processed" fence.
//
// It exists so an observer can wait for a reload to have been PROCESSED
// without sleeping. A rejected or no-op reload changes no other observable
// state -- the l2tp listener rebind rejection, for instance, logs a WARN and
// leaves the bound endpoint exactly as it was -- so before this counter the
// only evidence a reload had run at all was a stderr line, which a plugin
// cannot poll. Every processed reload advances generation regardless of
// outcome, which is what makes it a usable fence for the reject and no-op
// cases.
//
// The counter is observational only. Nothing reads it to make a decision, so
// it cannot alter what a reload accepts or rejects.
type reloadGeneration struct {
	mu         sync.Mutex
	generation uint64
	outcome    string
	at         time.Time
	lastOpID   string
	history    [maxReloadHistory]ReloadRecord
	histHead   int
	histCount  int
}

const reloadTriggerUnknown = "unknown"

// mark records that one reload finished, whatever its outcome.
func (g *reloadGeneration) mark(applied bool) {
	outcome := ReloadOutcomeFailed
	if applied {
		outcome = ReloadOutcomeApplied
	}
	now := time.Now().UTC()
	g.markWithRecord(ReloadRecord{
		Trigger:    reloadTriggerUnknown,
		Outcome:    outcome,
		StartedAt:  now,
		FinishedAt: now,
	})
}

// markWithRecord increments generation, updates the latest status and operation-id,
// stamps rec.Generation, appends rec into the 32-slot circular history ring,
// and returns the assigned generation. Acquires only g.mu (leaf lock).
func (g *reloadGeneration) markWithRecord(rec ReloadRecord) uint64 {
	if rec.Outcome == "" {
		if rec.Error == "" {
			rec.Outcome = ReloadOutcomeApplied
		} else {
			rec.Outcome = ReloadOutcomeFailed
		}
	}
	if rec.FinishedAt.IsZero() {
		rec.FinishedAt = time.Now().UTC()
	}
	if rec.StartedAt.IsZero() {
		rec.StartedAt = rec.FinishedAt
	}
	if rec.DurationMS < 0 {
		rec.DurationMS = 0
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	g.generation++
	g.outcome = rec.Outcome
	g.at = rec.FinishedAt
	g.lastOpID = rec.OpID
	rec.Generation = g.generation
	g.history[g.histHead] = cloneReloadRecord(rec)
	g.histHead = (g.histHead + 1) % maxReloadHistory
	if g.histCount < maxReloadHistory {
		g.histCount++
	}
	return g.generation
}

// statusWithOp returns a consistent snapshot of the four fields under one lock.
func (g *reloadGeneration) statusWithOp() (generation uint64, outcome string, at time.Time, opID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.generation == 0 {
		return 0, ReloadOutcomeNone, time.Time{}, ""
	}
	return g.generation, g.outcome, g.at, g.lastOpID
}

// snapshotHistory returns up to limit records newest-first, filtered by opID
// when non-empty. Always returns a non-nil slice.
func (g *reloadGeneration) snapshotHistory(limit int, opID string) []ReloadRecord {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.histCount == 0 {
		return []ReloadRecord{}
	}
	out := make([]ReloadRecord, 0, g.histCount)
	for i := range g.histCount {
		idx := (g.histHead - 1 - i + maxReloadHistory) % maxReloadHistory
		rec := g.history[idx]
		if opID != "" {
			if rec.OpID != opID {
				continue
			}
		}
		out = append(out, cloneReloadRecord(rec))
		if limit > 0 {
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

// MarkReloadProcessed records that a reload sequence completed and advances the
// generation counter. `applied` is false when the reload returned an error.
//
// MUST be called only once the ENTIRE reload sequence has run, not merely the
// plugin-server half: the config knobs a reload rejects are diffed by the
// subsystems that engine.Reload fans out to, which runs AFTER
// Server.ReloadConfig. Marking any earlier would advance the fence before the
// rejection it is meant to fence had happened, and an observer could then read
// state the reload had not yet touched. cmd/ze/hub/main_reload.go doReload is
// the correct (and only) caller.
//
// A reload refused with ErrReloadInProgress was never processed -- it is queued
// and replayed -- so its caller must not mark it.
func (s *Server) MarkReloadProcessed(applied bool) {
	s.reloadGen.mark(applied)
}

// MarkReloadProcessedWithRecord records a processed reload with its full
// transaction history entry, advances generation, and returns the assigned
// generation. Safe for concurrent use; acquires only the leaf reloadGen.mu lock.
func (s *Server) MarkReloadProcessedWithRecord(rec ReloadRecord) uint64 {
	return s.reloadGen.markWithRecord(rec)
}

// ReloadStatus returns the number of reloads processed since daemon start, the
// outcome of the most recent one, and when it finished. Before the first
// reload: (0, ReloadOutcomeNone, zero time).
//
// An observer fences on the generation: read it, trigger a reload, then poll
// until it advances. At that point every reload step has run, so the resulting
// state (or the deliberate absence of a change) is safe to assert.
func (s *Server) ReloadStatus() (generation uint64, outcome string, at time.Time) {
	generation, outcome, at, _ = s.reloadGen.statusWithOp()
	return generation, outcome, at
}

// ReloadStatusWithOp returns the latest reload generation, outcome, timestamp,
// and operation-id under the leaf reloadGen.mu lock.
func (s *Server) ReloadStatusWithOp() (generation uint64, outcome string, at time.Time, opID string) {
	return s.reloadGen.statusWithOp()
}

// ReloadHistory returns the newest-first bounded reload history (up to 32 entries),
// optionally filtered by opID and capped by limit when > 0. Always returns a
// non-nil slice and never acquires Server.txLock.
func (s *Server) ReloadHistory(limit int, opID string) []ReloadRecord {
	return s.reloadGen.snapshotHistory(limit, opID)
}
