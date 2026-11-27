package slogutil

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLogRingAppendAndSnapshot(t *testing.T) {
	r := newLogRing(10)
	r.append(LogEntry{Level: levelError, Component: "bgp", Message: "peer down"})
	r.append(LogEntry{Level: levelWarn, Component: "l2tp", Message: "echo loss"})
	r.append(LogEntry{Level: levelError, Component: "l2tp", Message: "tunnel closed"})

	snap := r.Recent(0)
	if len(snap) != 3 {
		t.Fatalf("count = %d, want 3", len(snap))
	}
	if snap[0].Message != "tunnel closed" {
		t.Errorf("newest = %q, want 'tunnel closed'", snap[0].Message)
	}
}

// TestLogRingFilterLevel drives the level filter with every spelling an
// operator can type for one level: the enumeration word `err`, the long
// form `error`, and slog's own upper case. Each selects the entries stored
// under the canonical name, and a word that names no level is refused.
func TestLogRingFilterLevel(t *testing.T) {
	r := newLogRing(10)
	r.append(LogEntry{Level: levelError, Component: "bgp", Message: "a"})
	r.append(LogEntry{Level: levelWarn, Component: "bgp", Message: "b"})
	r.append(LogEntry{Level: levelError, Component: "l2tp", Message: "c"})

	for _, word := range []string{"err", "error", "ERROR"} {
		snap, err := r.Snapshot(0, word, "")
		if err != nil {
			t.Fatalf("Snapshot(level %q): %v", word, err)
		}
		if len(snap) != 2 {
			t.Fatalf("Snapshot(level %q): count = %d, want 2", word, len(snap))
		}
		for _, e := range snap {
			if e.Level != levelError {
				t.Errorf("Snapshot(level %q) kept a %s entry", word, e.Level)
			}
		}
	}

	if _, err := r.Snapshot(0, "verbose", ""); err == nil {
		t.Fatal("Snapshot(level verbose) answered a list for a word that names no level")
	}
}

func TestLogRingFilterComponent(t *testing.T) {
	r := newLogRing(10)
	r.append(LogEntry{Level: levelError, Component: "bgp", Message: "a"})
	r.append(LogEntry{Level: levelWarn, Component: "l2tp", Message: "b"})

	snap, err := r.Snapshot(0, "", "l2tp")
	if err != nil {
		t.Fatalf("Snapshot(component l2tp): %v", err)
	}
	if len(snap) != 1 {
		t.Fatalf("count = %d, want 1", len(snap))
	}
	if snap[0].Component != "l2tp" {
		t.Errorf("component = %s, want l2tp", snap[0].Component)
	}
}

func TestLogRingLimit(t *testing.T) {
	r := newLogRing(10)
	for range 8 {
		r.append(LogEntry{Level: levelInfo, Message: "x"})
	}
	snap := r.Recent(3)
	if len(snap) != 3 {
		t.Fatalf("count = %d, want 3", len(snap))
	}
}

func TestLogRingOverflow(t *testing.T) {
	r := newLogRing(3)
	for i := range 5 {
		r.append(LogEntry{Level: levelInfo, Message: string(rune('a' + i))})
	}
	snap := r.Recent(0)
	if len(snap) != 3 {
		t.Fatalf("count = %d, want 3", len(snap))
	}
	if snap[0].Message != "e" {
		t.Errorf("newest = %q, want 'e'", snap[0].Message)
	}
}

func TestRingHandlerCapturesSubsystem(t *testing.T) {
	r := newLogRing(10)
	inner := slog.NewTextHandler(discardWriter{}, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := newRingHandler(inner, r)
	logger := slog.New(h).With("subsystem", "bgp.reactor")

	logger.Info("test message")

	snap := r.Recent(0)
	if len(snap) != 1 {
		t.Fatalf("count = %d, want 1", len(snap))
	}
	if snap[0].Component != "bgp.reactor" {
		t.Errorf("component = %q, want 'bgp.reactor'", snap[0].Component)
	}
	if snap[0].Message != "test message" {
		t.Errorf("message = %q, want 'test message'", snap[0].Message)
	}
	if snap[0].Level != levelInfo {
		t.Errorf("level = %q, want %q, the name ListLevels reports and an operator types", snap[0].Level, levelInfo)
	}
}

func TestRingHandlerEnabled(t *testing.T) {
	r := newLogRing(10)
	inner := slog.NewTextHandler(discardWriter{}, &slog.HandlerOptions{Level: slog.LevelWarn})
	h := newRingHandler(inner, r)

	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("DEBUG should not be enabled when inner handler is WARN")
	}
	if !h.Enabled(context.Background(), slog.LevelError) {
		t.Error("ERROR should be enabled when inner handler is WARN")
	}
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// VALIDATES: derived handlers can capture concurrently without crossing their
// structured fields, and ring snapshots retain independently owned values.
// PREVENTS: one logger's serialized attributes being attached to another record.
func TestRingHandlerConcurrentAttributes(t *testing.T) {
	const workers = 32
	ring := newLogRing(workers)
	handler := newRingHandler(slog.NewTextHandler(discardWriter{}, nil), ring)
	logger := slog.New(handler).With("subsystem", "concurrent")
	var wg sync.WaitGroup
	for i := range workers {
		name := strconv.Itoa(i)
		child := logger.WithGroup("attempt").With("owner", name)
		wg.Go(func() {
			child.Warn(name, "observed", name)
		})
	}
	wg.Wait()
	entries := ring.Recent(0)
	if len(entries) != workers {
		t.Fatalf("captured %d records, want %d", len(entries), workers)
	}
	for _, entry := range entries {
		attrs, err := entry.Attributes()
		if err != nil {
			t.Fatal(err)
		}
		attempt, ok := attrs["attempt"].(map[string]any)
		if !ok {
			t.Fatalf("record %q has no attempt group: %v", entry.Message, attrs)
		}
		if attempt["owner"] != entry.Message || attempt["observed"] != entry.Message {
			t.Errorf("record %q received another logger's fields: %v", entry.Message, attrs)
		}
	}
	// Overwrite every slot while retaining the previous snapshot.
	for range workers {
		logger.Warn("replacement", "owner", "replacement")
	}
	for _, entry := range entries {
		attrs, err := entry.Attributes()
		if err != nil {
			t.Fatal(err)
		}
		attempt, ok := attrs["attempt"].(map[string]any)
		if !ok {
			t.Fatalf("ring record %q has attempt %T, want an object", entry.Message, attrs["attempt"])
		}
		if attempt["owner"] != entry.Message {
			t.Fatalf("ring overwrite changed retained record %q: %v", entry.Message, attrs)
		}
	}
}

type nestedLogValue struct {
	logger *slog.Logger
}

func (v nestedLogValue) LogValue() slog.Value {
	v.logger.Warn("nested LogValuer")
	return slog.StringValue("resolved")
}

type nestedLogText struct {
	logger *slog.Logger
	Secret string
}

func (v nestedLogText) MarshalText() ([]byte, error) {
	v.logger.Warn("nested TextMarshaler")
	return []byte("[REDACTED]"), nil
}

// VALIDATES: user value callbacks execute outside shared capture locks.
// PREVENTS: a LogValuer or TextMarshaler deadlocking when it uses the same logger,
// or JSON querying exposing a field its text marshaler intentionally redacts.
func TestRingHandlerReentrantValues(t *testing.T) {
	ring := newLogRing(10)
	logger := slog.New(newRingHandler(slog.NewTextHandler(discardWriter{}, nil), ring))
	finished := make(chan struct{})
	go func() {
		logger.Warn("outer",
			"value", nestedLogValue{logger: logger},
			"opaque", nestedLogText{logger: logger, Secret: "must-not-be-exposed"})
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(10 * time.Second):
		t.Fatal("value callback could not log through its own handler")
	}
	entries := ring.Recent(0)
	if len(entries) != 3 || entries[0].Message != "outer" {
		t.Fatalf("expected outer and both nested records: %+v", entries)
	}
	attrs, err := entries[0].Attributes()
	if err != nil {
		t.Fatal(err)
	}
	if attrs["value"] != "resolved" || attrs["opaque"] != "[REDACTED]" {
		t.Fatalf("value callbacks were lost or exposed raw fields: %v", attrs)
	}
}

// VALIDATES: attributes omitted by the text handler do not discard their record.
// PREVENTS: empty source metadata panicking during record or logger capture.
func TestRingHandlerOmittedSource(t *testing.T) {
	for _, source := range []struct {
		name  string
		value *slog.Source
	}{
		{name: "nil"},
		{name: "empty", value: &slog.Source{}},
	} {
		for _, bound := range []bool{false, true} {
			t.Run(source.name+"/bound="+strconv.FormatBool(bound), func(t *testing.T) {
				ring := newLogRing(2)
				var text strings.Builder
				logger := slog.New(newRingHandler(slog.NewTextHandler(&text, nil), ring))
				if bound {
					logger.With("source", source.value).Warn("kept event", "peer", "responder")
				} else {
					logger.Warn("kept event", "source", source.value, "peer", "responder")
				}
				entries := ring.Recent(0)
				if len(entries) != 1 || entries[0].Message != "kept event" {
					t.Fatalf("source omission lost its record: %+v", entries)
				}
				attrs, err := entries[0].Attributes()
				if err != nil {
					t.Fatal(err)
				}
				if _, present := attrs["source"]; present || attrs["peer"] != "responder" {
					t.Fatalf("source omission changed retained fields: %v", attrs)
				}
				if strings.Contains(text.String(), "source=") || !strings.Contains(text.String(), "peer=responder") {
					t.Fatalf("source omission changed the text record: %q", text.String())
				}
			})
		}
	}
}

// TestLogRingFaultsSurviveInfoBurst verifies that a burst of info entries
// exceeding defaultLogRingCapacity does not evict earlier warn or error
// entries from the log ring.
//
// VALIDATES: AC-5 -- warn and error entries survive a burst of
// defaultLogRingCapacity+100 info entries in Snapshot("warn"), Snapshot("err"),
// and Recent(0).
// PREVENTS: high-volume info/debug traffic silently evicting pre-incident
// warnings and errors before triage or operator inspection.
func TestLogRingFaultsSurviveInfoBurst(t *testing.T) {
	r := newLogRing(defaultLogRingCapacity)
	r.append(LogEntry{Level: levelWarn, Component: "bgp", Message: "early warning"})
	r.append(LogEntry{Level: levelError, Component: "bgp", Message: "early error"})

	const infoCount = defaultLogRingCapacity + 100
	for i := range infoCount {
		r.append(LogEntry{Level: levelInfo, Component: "bgp", Message: "info-" + strconv.Itoa(i)})
	}

	warns, err := r.Snapshot(0, "warn", "")
	if err != nil {
		t.Fatalf("Snapshot(warn): %v", err)
	}
	if len(warns) != 1 || warns[0].Message != "early warning" {
		t.Fatalf("Snapshot(warn) = %+v, want [early warning]", warns)
	}

	errs, err := r.Snapshot(0, "err", "")
	if err != nil {
		t.Fatalf("Snapshot(err): %v", err)
	}
	if len(errs) != 1 || errs[0].Message != "early error" {
		t.Fatalf("Snapshot(err) = %+v, want [early error]", errs)
	}

	recent := r.Recent(0)
	if len(recent) != defaultLogRingCapacity+2 {
		t.Fatalf("Recent(0) len = %d, want %d", len(recent), defaultLogRingCapacity+2)
	}
	if recent[0].Message != "info-"+strconv.Itoa(infoCount-1) {
		t.Errorf("Recent(0)[0] = %q, want newest info entry", recent[0].Message)
	}
	if recent[len(recent)-2].Message != "early error" || recent[len(recent)-1].Message != "early warning" {
		t.Errorf("Recent(0) tail = [%q, %q], want [early error, early warning]",
			recent[len(recent)-2].Message, recent[len(recent)-1].Message)
	}

	// Fault sub-ring rotates independently at defaultFaultRingCapacity (256),
	// and a subsequent info burst still evicts zero faults.
	rf := newLogRing(defaultLogRingCapacity)
	const faultCount = defaultFaultRingCapacity + 10
	for i := range faultCount {
		rf.append(LogEntry{Level: levelError, Component: "bgp", Message: "fault-" + strconv.Itoa(i)})
	}
	for i := range infoCount {
		rf.append(LogEntry{Level: levelInfo, Component: "bgp", Message: "burst-" + strconv.Itoa(i)})
	}
	faultSnap, err := rf.Snapshot(0, "err", "")
	if err != nil {
		t.Fatalf("Snapshot(err) after fault rotation: %v", err)
	}
	if len(faultSnap) != defaultFaultRingCapacity {
		t.Fatalf("retained faults = %d, want %d", len(faultSnap), defaultFaultRingCapacity)
	}
	if faultSnap[0].Message != "fault-"+strconv.Itoa(faultCount-1) {
		t.Errorf("newest retained fault = %q, want fault-%d", faultSnap[0].Message, faultCount-1)
	}
	if faultSnap[len(faultSnap)-1].Message != "fault-10" {
		t.Errorf("oldest retained fault = %q, want fault-10", faultSnap[len(faultSnap)-1].Message)
	}
}
