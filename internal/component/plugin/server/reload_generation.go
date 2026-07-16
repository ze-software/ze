// Design: docs/architecture/api/commands.md -- show reload-status observability surface
// Related: reload.go -- Server.reloadConfig, the plugin-server half of a reload

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
}

// mark records that one reload finished, whatever its outcome.
func (g *reloadGeneration) mark(applied bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.generation++
	if applied {
		g.outcome = ReloadOutcomeApplied
	} else {
		g.outcome = ReloadOutcomeFailed
	}
	g.at = time.Now()
}

// status returns a consistent snapshot of the three fields. Taken under one
// lock so an observer can never pair a generation from one reload with the
// outcome of another.
func (g *reloadGeneration) status() (generation uint64, outcome string, at time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.generation == 0 {
		return 0, ReloadOutcomeNone, time.Time{}
	}
	return g.generation, g.outcome, g.at
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

// ReloadStatus returns the number of reloads processed since daemon start, the
// outcome of the most recent one, and when it finished. Before the first
// reload: (0, ReloadOutcomeNone, zero time).
//
// An observer fences on the generation: read it, trigger a reload, then poll
// until it advances. At that point every reload step has run, so the resulting
// state (or the deliberate absence of a change) is safe to assert.
func (s *Server) ReloadStatus() (generation uint64, outcome string, at time.Time) {
	return s.reloadGen.status()
}
