package slogutil

import (
	"context"
	"log/slog"
	"testing"
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
