// Design: docs/architecture/diagnostics/production-diagnostics.md -- diag-7 structured log query

package slogutil

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const defaultLogRingCapacity = 512

// LogEntry is one record in the log ring buffer.
type LogEntry struct {
	Timestamp time.Time
	// Level is the name levelString writes (debug, info, warn, error), the
	// same spelling ListLevels reports and an operator types in a level word.
	Level     string
	Component string
	Message   string
}

// LogRing is a fixed-size circular buffer of log entries.
// Safe for concurrent use.
type LogRing struct {
	mu      sync.Mutex
	records []LogEntry
	head    int
	count   int
}

// newLogRing creates a ring with the given capacity.
func newLogRing(capacity int) *LogRing {
	if capacity <= 0 {
		capacity = defaultLogRingCapacity
	}
	return &LogRing{records: make([]LogEntry, capacity)}
}

func (r *LogRing) append(entry LogEntry) {
	r.mu.Lock()
	r.records[r.head] = entry
	r.head = (r.head + 1) % len(r.records)
	if r.count < len(r.records) {
		r.count++
	}
	r.mu.Unlock()
}

// Recent returns the newest entries at every level and component, newest
// first. Limit <= 0 returns all.
func (r *LogRing) Recent(limit int) []LogEntry {
	return r.collect(limit, "", "")
}

// Snapshot returns log entries newest-first, filtered by level and component.
// Empty strings mean no filter. Limit <= 0 returns all. The level is a word
// parseLevel accepts, in any case, so `err` and `error` both select the
// entries stored as error. A word that names no level is refused, because an
// empty answer would read as an empty ring.
func (r *LogRing) Snapshot(limit int, level, component string) ([]LogEntry, error) {
	if level == "" {
		return r.collect(limit, "", component), nil
	}
	lvl, ok := parseLevel(level)
	if !ok {
		return nil, fmt.Errorf("invalid level %q (valid: %s)", level, textbuf.Join(validLevelNames, ", "))
	}
	return r.collect(limit, levelString(lvl), component), nil
}

// collect walks the ring newest-first and keeps the entries whose level name
// and component equal the non-empty filters. Limit <= 0 keeps all.
func (r *LogRing) collect(limit int, level, component string) []LogEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count == 0 {
		return []LogEntry{}
	}

	out := make([]LogEntry, 0, r.count)
	for i := range r.count {
		idx := (r.head - 1 - i + len(r.records)) % len(r.records)
		rec := r.records[idx]
		if level != "" && rec.Level != level {
			continue
		}
		if component != "" && rec.Component != component {
			continue
		}
		out = append(out, rec)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// ringHandler is a slog.Handler that tees log records to a LogRing
// in addition to an inner handler.
type ringHandler struct {
	inner slog.Handler
	ring  *LogRing
	attrs []slog.Attr
}

func newRingHandler(inner slog.Handler, ring *LogRing) *ringHandler {
	return &ringHandler{inner: inner, ring: ring}
}

// Enabled delegates to the inner handler. The ring only captures entries
// at or above the subsystem's configured level. Lowering a subsystem's
// level via `log set <subsystem> debug` will cause debug entries to
// appear in subsequent `show log recent` output.
func (h *ringHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *ringHandler) Handle(ctx context.Context, r slog.Record) error { //nolint:gocritic // slog.Handler interface requires value receiver for Record
	component := ""
	for _, a := range h.attrs {
		if a.Key == "subsystem" {
			component = a.Value.String()
			break
		}
	}
	if component == "" {
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "subsystem" {
				component = a.Value.String()
				return false
			}
			return true
		})
	}

	h.ring.append(LogEntry{
		Timestamp: r.Time,
		Level:     levelString(r.Level),
		Component: component,
		Message:   r.Message,
	})

	return h.inner.Handle(ctx, r)
}

func (h *ringHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ringHandler{
		inner: h.inner.WithAttrs(attrs),
		ring:  h.ring,
		attrs: append(h.attrs, attrs...),
	}
}

func (h *ringHandler) WithGroup(name string) slog.Handler {
	return &ringHandler{
		inner: h.inner.WithGroup(name),
		ring:  h.ring,
		attrs: h.attrs,
	}
}

// globalLogRing is the singleton log ring for CLI queries.
var globalLogRing = newLogRing(defaultLogRingCapacity)

// GlobalLogRing returns the global log ring buffer.
func GlobalLogRing() *LogRing {
	return globalLogRing
}
