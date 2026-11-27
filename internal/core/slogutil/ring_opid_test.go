package slogutil

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opid"
)

type redactedSecretValue struct {
	Secret string
}

func (redactedSecretValue) MarshalText() ([]byte, error) {
	return []byte("[REDACTED]"), nil
}

// VALIDATES: AC-1 -- ringHandler extracts op-id from context, populates LogEntry.OpID and attributes, and preserves redaction.
// PREVENTS: log entries losing operation correlation IDs or leaking redacted values.
func TestRingHandlerOpIDFromContext(t *testing.T) {
	ring := newLogRing(16)
	var text strings.Builder
	handler := newRingHandler(slog.NewTextHandler(&text, &slog.HandlerOptions{Level: slog.LevelDebug}), ring)
	logger := slog.New(handler).With("subsystem", "hub.reload")

	ctx := opid.With(context.Background(), "op-42")
	logger.InfoContext(ctx, "reload applied", "secret", redactedSecretValue{Secret: "top-secret"})

	entries := ring.Recent(0)
	if len(entries) != 1 {
		t.Fatalf("count = %d, want 1", len(entries))
	}
	if entries[0].OpID != "op-42" {
		t.Errorf("OpID = %q, want op-42", entries[0].OpID)
	}
	attrs, err := entries[0].Attributes()
	if err != nil {
		t.Fatalf("Attributes: %v", err)
	}
	if attrs["operation-id"] != "op-42" {
		t.Errorf("attrs[operation-id] = %v, want op-42", attrs["operation-id"])
	}
	if attrs["secret"] != "[REDACTED]" {
		t.Errorf("attrs[secret] = %v, want [REDACTED]", attrs["secret"])
	}
	if !strings.Contains(text.String(), "operation-id=op-42") {
		t.Errorf("text output missing operation-id=op-42: %q", text.String())
	}
}

// VALIDATES: AC-1 -- ringHandler extracts explicit "operation-id" from record attributes and pre-bound logger attributes without duplication.
// PREVENTS: callers logging operation-id as an attribute being missed by SnapshotWithOp or emitting duplicate keys.
func TestRingHandlerExplicitOpIDAttr(t *testing.T) {
	ring := newLogRing(16)
	var text strings.Builder
	handler := newRingHandler(slog.NewTextHandler(&text, &slog.HandlerOptions{Level: slog.LevelDebug}), ring)
	logger := slog.New(handler).With("subsystem", "plugin.server")

	// Explicit attr on record without context operation-id.
	logger.Warn("explicit record", "operation-id", "op-77")
	// Pre-bound attr via With without context operation-id.
	logger.With("operation-id", "op-88").Error("bound record")
	// Context operation-id matching explicit attr must not duplicate "operation-id" in text or JSON.
	ctx := opid.With(context.Background(), "op-99")
	logger.InfoContext(ctx, "both set", "operation-id", "op-99")

	entries := ring.Recent(0)
	if len(entries) != 3 {
		t.Fatalf("count = %d, want 3", len(entries))
	}
	if entries[0].OpID != "op-99" || entries[1].OpID != "op-88" || entries[2].OpID != "op-77" {
		t.Fatalf("OpIDs = [%q, %q, %q], want [op-99, op-88, op-77]",
			entries[0].OpID, entries[1].OpID, entries[2].OpID)
	}
	if strings.Count(text.String(), "operation-id=op-99") != 1 {
		t.Errorf("duplicate operation-id=op-99 in text output: %q", text.String())
	}
	for _, entry := range entries {
		attrs, err := entry.Attributes()
		if err != nil {
			t.Fatalf("Attributes: %v", err)
		}
		if attrs["operation-id"] != entry.OpID {
			t.Errorf("entry %q attrs[operation-id] = %v, want %q", entry.Message, attrs["operation-id"], entry.OpID)
		}
	}
}

// VALIDATES: AC-1 -- SnapshotWithOp filters newest-first across both general and faults sub-rings while preserving Snapshot when opID is empty.
// PREVENTS: op-id queries missing warnings/errors in the fault sub-ring or breaking unfiltered queries.
func TestLogRingSnapshotWithOp(t *testing.T) {
	ring := newLogRing(16)
	handler := newRingHandler(slog.NewTextHandler(discardWriter{}, &slog.HandlerOptions{Level: slog.LevelDebug}), ring)
	reloadLog := slog.New(handler).With("subsystem", "hub.reload")
	bgpLog := slog.New(handler).With("subsystem", "bgp.reactor")

	ctx1 := opid.With(context.Background(), "op-1")
	ctx2 := opid.With(context.Background(), "op-2")

	reloadLog.InfoContext(ctx1, "op1-info-1")
	bgpLog.WarnContext(ctx2, "op2-warn")
	reloadLog.ErrorContext(ctx1, "op1-err")
	reloadLog.Info("no-op-info")
	bgpLog.InfoContext(ctx1, "op1-info-2")

	// Empty opID matches Snapshot exactly.
	allSnap, err := ring.Snapshot(0, "", "")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	allWithOp, err := ring.SnapshotWithOp(0, "", "", "")
	if err != nil {
		t.Fatalf("SnapshotWithOp empty: %v", err)
	}
	if len(allWithOp) != len(allSnap) {
		t.Fatalf("SnapshotWithOp('') len = %d, want %d", len(allWithOp), len(allSnap))
	}

	// Cross-sub-ring filtering (general + faults), newest-first.
	op1Entries, err := ring.SnapshotWithOp(0, "", "", "op-1")
	if err != nil {
		t.Fatalf("SnapshotWithOp(op-1): %v", err)
	}
	if len(op1Entries) != 3 {
		t.Fatalf("op-1 len = %d, want 3", len(op1Entries))
	}
	if op1Entries[0].Message != "op1-info-2" || op1Entries[1].Message != "op1-err" || op1Entries[2].Message != "op1-info-1" {
		t.Errorf("op-1 order = [%q, %q, %q], want [op1-info-2, op1-err, op1-info-1]",
			op1Entries[0].Message, op1Entries[1].Message, op1Entries[2].Message)
	}

	// Limit applies after op-id matching.
	limited, err := ring.SnapshotWithOp(2, "", "", "op-1")
	if err != nil {
		t.Fatalf("SnapshotWithOp limit: %v", err)
	}
	if len(limited) != 2 || limited[0].Message != "op1-info-2" || limited[1].Message != "op1-err" {
		t.Errorf("limited op-1 = %+v, want [op1-info-2, op1-err]", limited)
	}

	// Combined level + component + opID filter (fault sub-ring).
	errOnly, err := ring.SnapshotWithOp(0, "err", "hub.reload", "op-1")
	if err != nil {
		t.Fatalf("SnapshotWithOp err: %v", err)
	}
	if len(errOnly) != 1 || errOnly[0].Message != "op1-err" {
		t.Errorf("errOnly = %+v, want [op1-err]", errOnly)
	}

	// Invalid level is still refused.
	if _, err := ring.SnapshotWithOp(0, "bogus", "", "op-1"); err == nil {
		t.Error("SnapshotWithOp(bogus) succeeded, want error")
	}
}
