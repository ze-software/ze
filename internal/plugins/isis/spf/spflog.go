// Design: docs/architecture/isis/isis-13-cli-diag-interop.md -- `show isis spf-log` history
// Related: computer.go -- Run records one SPFLogEntry per level per run
//
// The SPF log is a bounded ring of recent SPF runs surfaced by `show isis
// spf-log` (spec-isis-13 AC-6). Each entry records when a run happened, the
// level it computed, why it ran (the trigger), how long it took, and how many
// nodes were in the graph. The ring is bounded (spfLogCapacity) so a busy node
// cannot grow it without limit (security review: spf-log render is bounded). The
// log is observational only: recording an entry never changes routing state.

package spf

import (
	"sync"
	"time"
)

// spfLogCapacity bounds the SPF-run history ring. A run on an L1L2 node appends
// one entry per level, so the ring holds roughly spfLogCapacity/2 full runs for
// a dual-level node. The bound caps both memory and the `show isis spf-log`
// render size.
const spfLogCapacity = 64

// SPFLogEntry is one recorded SPF run at one level (the `show isis spf-log`
// row). It is a flat value with no pointers so it crosses the CLI/RPC boundary
// cleanly. DurationSeconds is the wall-clock cost of the Dijkstra pass; Nodes is
// the node count of the graph it ran over; Trigger names why the run happened.
type SPFLogEntry struct {
	// Time is the run start time as a Unix timestamp (seconds). The renderer
	// formats it; the engine keeps it numeric so the value is locale-free.
	TimeUnix int64 `json:"time-unix"`
	// Level is the routing level computed ("l1" | "l2").
	Level string `json:"level"`
	// Trigger names why SPF ran (e.g. "lsdb-change", "manual"). The debounced
	// engine path records "lsdb-change"; a direct Run records "manual".
	Trigger string `json:"trigger"`
	// DurationSeconds is how long the Dijkstra pass took for this level.
	DurationSeconds float64 `json:"duration-seconds"`
	// Nodes is the number of nodes in the level graph this run built.
	Nodes int `json:"nodes"`
}

// spfLog is the bounded ring of recent SPF runs. It is guarded by its own mutex
// so recording (on the SPF goroutine) and reading (on the CLI goroutine) never
// race, independent of the Computer's run lock.
type spfLog struct {
	mu      sync.Mutex
	entries []SPFLogEntry // newest last; capped at spfLogCapacity
	trigger string        // why the NEXT recorded run ran (set by the trigger path)
}

// setTrigger records the reason the next run will report. The engine debounce
// path calls this with "lsdb-change" before triggering; a direct Run with no
// trigger set reports "manual". Concurrency-safe.
func (l *spfLog) setTrigger(reason string) {
	l.mu.Lock()
	l.trigger = reason
	l.mu.Unlock()
}

// record appends one run entry, evicting the oldest when at capacity. now is the
// run start time. The trigger is the most recently set reason, defaulting to
// "manual" when none was set (a direct Run() call, e.g. a test).
func (l *spfLog) record(now time.Time, level string, dur time.Duration, nodes int) {
	l.mu.Lock()
	trigger := l.trigger
	if trigger == "" {
		trigger = "manual"
	}
	e := SPFLogEntry{
		TimeUnix:        now.Unix(),
		Level:           level,
		Trigger:         trigger,
		DurationSeconds: dur.Seconds(),
		Nodes:           nodes,
	}
	if len(l.entries) >= spfLogCapacity {
		// Drop the oldest, keep the most recent spfLogCapacity-1, append the new.
		l.entries = append(l.entries[1:], e)
	} else {
		l.entries = append(l.entries, e)
	}
	l.mu.Unlock()
}

// snapshot returns a copy of the recorded runs, newest first (the `show isis
// spf-log` order: most recent at the top). It never exposes the live slice.
func (l *spfLog) snapshot() []SPFLogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]SPFLogEntry, len(l.entries))
	// Reverse so the newest entry is first.
	for i, e := range l.entries {
		out[len(l.entries)-1-i] = e
	}
	return out
}

// reset clears the recorded history (used by `clear isis counters`, which resets
// observational state without tearing down routing).
func (l *spfLog) reset() {
	l.mu.Lock()
	l.entries = nil
	l.mu.Unlock()
}
