// Design: docs/architecture/diagnostics/production-diagnostics.md -- diag-7 structured log query

package slogutil

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
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
	// attributes is immutable JSON, captured before caller-owned values can
	// change. Snapshots can share it without sharing mutable attribute data.
	attributes string
}

// Attributes decodes this entry's structured fields for a query. The result
// belongs to the caller; changing it cannot change this or later snapshots.
func (e LogEntry) Attributes() (map[string]any, error) {
	if e.attributes == "" {
		return nil, nil
	}
	var record struct {
		Attributes map[string]any `json:"attributes"`
	}
	decoder := json.NewDecoder(strings.NewReader(e.attributes))
	decoder.UseNumber()
	if err := decoder.Decode(&record); err != nil {
		return nil, fmt.Errorf("decode log attributes: %w", err)
	}
	return record.Attributes, nil
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
	inner        slog.Handler
	ring         *LogRing
	fields       slog.Handler
	capture      *logAttributeCapture
	component    string
	hasComponent bool
}

func newRingHandler(inner slog.Handler, ring *LogRing) *ringHandler {
	capture := &logAttributeCapture{}
	// Keep standard slog group and scalar encoding. Caller callbacks have
	// already been resolved by snapshotLogRecord/snapshotLogAttrs.
	fields := slog.NewJSONHandler(capture, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if len(groups) == 0 {
				return slog.Attr{}
			}
			return attr
		},
	}).WithGroup("attributes")
	return &ringHandler{inner: inner, ring: ring, fields: fields, capture: capture}
}

// Enabled delegates to the inner handler. The ring only captures entries
// at or above the subsystem's configured level. Lowering a subsystem's
// level via `log set <subsystem> debug` will cause debug entries to
// appear in subsequent `show log recent` output.
func (h *ringHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *ringHandler) Handle(ctx context.Context, r slog.Record) error { //nolint:gocritic // slog.Handler interface requires value receiver for Record
	r = snapshotLogRecord(r)
	component := h.component
	if component == "" {
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "subsystem" {
				component = a.Value.Resolve().String()
				return false
			}
			return true
		})
	}
	attributes, err := h.capture.encode(ctx, h.fields, r)
	if err != nil {
		return err
	}

	h.ring.append(LogEntry{
		Timestamp:  r.Time,
		Level:      levelString(r.Level),
		Component:  component,
		Message:    r.Message,
		attributes: attributes,
	})

	return h.inner.Handle(ctx, r)
}

func (h *ringHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	attrs = snapshotLogAttrs(attrs)
	component, hasComponent := h.component, h.hasComponent
	if !hasComponent {
		for _, attr := range attrs {
			if attr.Key == "subsystem" {
				component, hasComponent = attr.Value.Resolve().String(), true
				break
			}
		}
	}
	return &ringHandler{
		inner:        h.inner.WithAttrs(attrs),
		ring:         h.ring,
		fields:       h.fields.WithAttrs(attrs),
		capture:      h.capture,
		component:    component,
		hasComponent: hasComponent,
	}
}

func (h *ringHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &ringHandler{
		inner:        h.inner.WithGroup(name),
		ring:         h.ring,
		fields:       h.fields.WithGroup(name),
		capture:      h.capture,
		component:    h.component,
		hasComponent: h.hasComponent,
	}
}

// snapshotLogRecord MUST resolve caller callbacks before a capture lock is
// held. Both the ring and text backend then receive the same frozen values.
func snapshotLogRecord(record slog.Record) slog.Record { //nolint:gocritic // slog records are passed by value.
	if record.NumAttrs() == 0 {
		return record
	}
	frozen := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		frozen.AddAttrs(snapshotLogAttr(attr))
		return true
	})
	return frozen
}

func snapshotLogAttrs(attrs []slog.Attr) []slog.Attr {
	frozen := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		frozen[i] = snapshotLogAttr(attr)
	}
	return frozen
}

func snapshotLogAttr(attr slog.Attr) slog.Attr {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return attr
	}
	switch attr.Value.Kind() {
	case slog.KindGroup:
		children := snapshotLogAttrs(attr.Value.Group())
		attr.Value = slog.GroupValue(children...)
	case slog.KindAny:
		// Preserve TextHandler's TextAppender/TextMarshaler/Stringer
		// presentation, including redaction. Reflecting the original object
		// into JSON could reveal fields its text representation hides.
		text, present := snapshotLogAny(attr.Value)
		if !present {
			return slog.Attr{}
		}
		attr.Value = slog.StringValue(text)
	}
	return attr
}

// opaqueLogFormatter is borrowed exclusively, so caller formatting may itself
// log without waiting for a lock held by the outer record.
type opaqueLogFormatter struct {
	handler *slog.TextHandler
	encoded string
}

var opaqueLogFormatters = sync.Pool{New: func() any {
	formatter := &opaqueLogFormatter{}
	formatter.handler = slog.NewTextHandler(formatter, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key != "value" {
				return slog.Attr{}
			}
			return attr
		},
	})
	return formatter
}}

func (f *opaqueLogFormatter) Write(p []byte) (int, error) {
	f.encoded = string(p)
	return len(p), nil
}

func snapshotLogAny(value slog.Value) (text string, present bool) {
	// The borrowed formatter MUST stay exclusive until its output is detached.
	formatter := opaqueLogFormatters.Get().(*opaqueLogFormatter)
	record := slog.NewRecord(time.Time{}, slog.LevelInfo, "", 0)
	record.AddAttrs(slog.Attr{Key: "value", Value: value})
	// TextHandler encodes formatting failures in the value. Its only returned
	// error is from Write, and this owned sink always succeeds.
	_ = formatter.handler.Handle(context.Background(), record)
	encoded := formatter.encoded
	formatter.encoded = ""
	// Put MUST follow detaching the output and clearing the reusable sink.
	opaqueLogFormatters.Put(formatter)
	// TextHandler omits some values, including nil and empty *slog.Source.
	// Preserve that omission rather than inventing an empty string field.
	if encoded == "\n" {
		return "", false
	}
	encoded = encoded[len("value=") : len(encoded)-1]
	if len(encoded) >= 2 && encoded[0] == '"' {
		encoded = unquoteValue(encoded[1 : len(encoded)-1])
	}
	return encoded, true
}

// logAttributeCapture owns the JSON handler's one-write-per-record output.
// Derived handlers share this sink, so encode MUST hold mu across Handle and
// taking the result. Write MUST only be called by that protected Handle.
type logAttributeCapture struct {
	mu      sync.Mutex
	encoded string
}

func (c *logAttributeCapture) encode(ctx context.Context, handler slog.Handler, record slog.Record) (string, error) { //nolint:gocritic // slog.Handler takes Record by value.
	c.mu.Lock()
	defer c.mu.Unlock()
	err := handler.Handle(ctx, record)
	encoded := c.encoded
	c.encoded = ""
	return encoded, err
}

func (c *logAttributeCapture) Write(p []byte) (int, error) {
	// The formatter reuses its bytes. This single retained copy makes every
	// ring entry independent without decoding or allocating maps on the log path.
	c.encoded = string(p)
	return len(p), nil
}

// globalLogRing is the singleton log ring for CLI queries.
var globalLogRing = newLogRing(defaultLogRingCapacity)

// GlobalLogRing returns the global log ring buffer.
func GlobalLogRing() *LogRing {
	return globalLogRing
}
