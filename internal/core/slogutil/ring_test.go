package slogutil

import (
	"context"
	"log/slog"
	"testing"
)

func TestLogRingAppendAndSnapshot(t *testing.T) {
	r := NewLogRing(10)
	r.append(LogEntry{Level: "ERROR", Component: "bgp", Message: "peer down"})
	r.append(LogEntry{Level: "WARN", Component: "l2tp", Message: "echo loss"})
	r.append(LogEntry{Level: "ERROR", Component: "l2tp", Message: "tunnel closed"})

	snap := r.Snapshot(0, "", "")
	if len(snap) != 3 {
		t.Fatalf("count = %d, want 3", len(snap))
	}
	if snap[0].Message != "tunnel closed" {
		t.Errorf("newest = %q, want 'tunnel closed'", snap[0].Message)
	}
}

func TestLogRingFilterLevel(t *testing.T) {
	r := NewLogRing(10)
	r.append(LogEntry{Level: "ERROR", Component: "bgp", Message: "a"})
	r.append(LogEntry{Level: "WARN", Component: "bgp", Message: "b"})
	r.append(LogEntry{Level: "ERROR", Component: "l2tp", Message: "c"})

	snap := r.Snapshot(0, "ERROR", "")
	if len(snap) != 2 {
		t.Fatalf("count = %d, want 2", len(snap))
	}
	for _, e := range snap {
		if e.Level != "ERROR" {
			t.Errorf("non-ERROR in filtered result: %s", e.Level)
		}
	}
}

func TestLogRingFilterComponent(t *testing.T) {
	r := NewLogRing(10)
	r.append(LogEntry{Level: "ERROR", Component: "bgp", Message: "a"})
	r.append(LogEntry{Level: "WARN", Component: "l2tp", Message: "b"})

	snap := r.Snapshot(0, "", "l2tp")
	if len(snap) != 1 {
		t.Fatalf("count = %d, want 1", len(snap))
	}
	if snap[0].Component != "l2tp" {
		t.Errorf("component = %s, want l2tp", snap[0].Component)
	}
}

func TestLogRingLimit(t *testing.T) {
	r := NewLogRing(10)
	for range 8 {
		r.append(LogEntry{Level: "INFO", Message: "x"})
	}
	snap := r.Snapshot(3, "", "")
	if len(snap) != 3 {
		t.Fatalf("count = %d, want 3", len(snap))
	}
}

func TestLogRingOverflow(t *testing.T) {
	r := NewLogRing(3)
	for i := range 5 {
		r.append(LogEntry{Level: "INFO", Message: string(rune('a' + i))})
	}
	snap := r.Snapshot(0, "", "")
	if len(snap) != 3 {
		t.Fatalf("count = %d, want 3", len(snap))
	}
	if snap[0].Message != "e" {
		t.Errorf("newest = %q, want 'e'", snap[0].Message)
	}
}

func TestRingHandlerCapturesSubsystem(t *testing.T) {
	r := NewLogRing(10)
	inner := slog.NewTextHandler(discardWriter{}, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := newRingHandler(inner, r)
	logger := slog.New(h).With("subsystem", "bgp.reactor")

	logger.Info("test message")

	snap := r.Snapshot(0, "", "")
	if len(snap) != 1 {
		t.Fatalf("count = %d, want 1", len(snap))
	}
	if snap[0].Component != "bgp.reactor" {
		t.Errorf("component = %q, want 'bgp.reactor'", snap[0].Component)
	}
	if snap[0].Message != "test message" {
		t.Errorf("message = %q, want 'test message'", snap[0].Message)
	}
	if snap[0].Level != "INFO" {
		t.Errorf("level = %q, want 'INFO'", snap[0].Level)
	}
}

func TestRingHandlerEnabled(t *testing.T) {
	r := NewLogRing(10)
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
