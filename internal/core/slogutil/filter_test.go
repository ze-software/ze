// VALIDATES: AC-9 -- debug messages filtered by flag when flags are active.
// VALIDATES: AC-10 -- no overhead when debug is off (filterHandler passes through).
// PREVENTS: filterHandler blocking non-matching messages or adding overhead when inactive.

package slogutil

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func newTestFilterLogger(buf *bytes.Buffer) (*slog.Logger, *filterHandler) {
	base := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	fh := newFilterHandler(base)
	return slog.New(fh), fh
}

func TestFilterHandlerNoFlagsPassesAll(t *testing.T) {
	var buf bytes.Buffer
	logger, _ := newTestFilterLogger(&buf)

	logger.Debug("hello", "flag", "update")
	if !strings.Contains(buf.String(), "hello") {
		t.Error("expected message to pass through when no flags configured")
	}
}

func TestFilterHandlerPassesMatchingFlag(t *testing.T) {
	var buf bytes.Buffer
	logger, fh := newTestFilterLogger(&buf)

	fh.setFlags([]string{"update"})

	logger.Debug("matched", "flag", "update")
	if !strings.Contains(buf.String(), "matched") {
		t.Error("expected message with matching flag to pass")
	}
}

func TestFilterHandlerBlocksNonMatchingFlag(t *testing.T) {
	var buf bytes.Buffer
	logger, fh := newTestFilterLogger(&buf)

	fh.setFlags([]string{"update"})

	logger.Debug("blocked", "flag", "keepalive")
	if strings.Contains(buf.String(), "blocked") {
		t.Error("expected message with non-matching flag to be blocked")
	}
}

func TestFilterHandlerBlocksNoFlagAttr(t *testing.T) {
	var buf bytes.Buffer
	logger, fh := newTestFilterLogger(&buf)

	fh.setFlags([]string{"update"})

	logger.Debug("no-flag-attr")
	if strings.Contains(buf.String(), "no-flag-attr") {
		t.Error("expected message without flag attribute to be blocked when flags are active")
	}
}

func TestFilterHandlerDirectionAsScope(t *testing.T) {
	var buf bytes.Buffer
	logger, fh := newTestFilterLogger(&buf)

	fh.setFlags([]string{"update"})
	fh.setScopes(map[string]string{"direction": "receive"})

	logger.Debug("recv", "flag", "update", "direction", "receive")
	if !strings.Contains(buf.String(), "recv") {
		t.Error("expected receive direction to pass via scope")
	}

	buf.Reset()
	logger.Debug("send", "flag", "update", "direction", "send")
	if strings.Contains(buf.String(), "send") {
		t.Error("expected send direction to be blocked via scope filter")
	}
}

func TestFilterHandlerNoScopesPassesAll(t *testing.T) {
	var buf bytes.Buffer
	logger, fh := newTestFilterLogger(&buf)

	fh.setFlags([]string{"update"})

	logger.Debug("send", "flag", "update", "direction", "send")
	if !strings.Contains(buf.String(), "send") {
		t.Error("expected send to pass when no direction filter set")
	}

	buf.Reset()
	logger.Debug("recv", "flag", "update", "direction", "receive")
	if !strings.Contains(buf.String(), "recv") {
		t.Error("expected receive to pass when no direction filter set")
	}
}

func TestFilterHandlerScopeNeighbor(t *testing.T) {
	var buf bytes.Buffer
	logger, fh := newTestFilterLogger(&buf)

	fh.setScopes(map[string]string{"neighbor": "192.0.2.1"})

	logger.Debug("match", "neighbor", "192.0.2.1")
	if !strings.Contains(buf.String(), "match") {
		t.Error("expected matching neighbor scope to pass")
	}

	buf.Reset()
	logger.Debug("other", "neighbor", "192.0.2.2")
	if strings.Contains(buf.String(), "other") {
		t.Error("expected non-matching neighbor to be blocked")
	}
}

func TestFilterHandlerScopeWithoutAttr(t *testing.T) {
	var buf bytes.Buffer
	logger, fh := newTestFilterLogger(&buf)

	fh.setScopes(map[string]string{"neighbor": "192.0.2.1"})

	logger.Debug("no-neighbor")
	if strings.Contains(buf.String(), "no-neighbor") {
		t.Error("expected message without neighbor attribute to be blocked when scope is set")
	}
}

func TestFilterHandlerClearFilters(t *testing.T) {
	var buf bytes.Buffer
	logger, fh := newTestFilterLogger(&buf)

	fh.setFlags([]string{"update"})
	fh.clearFilters()

	logger.Debug("after-clear", "flag", "keepalive")
	if !strings.Contains(buf.String(), "after-clear") {
		t.Error("expected all messages to pass after ClearFilters")
	}
}

func TestFilterHandlerWithAttrsPreserved(t *testing.T) {
	var buf bytes.Buffer
	logger, fh := newTestFilterLogger(&buf)

	fh.setFlags([]string{"update"})

	child := logger.With("extra", "val")
	child.Debug("with-attrs", "flag", "update")
	if !strings.Contains(buf.String(), "with-attrs") {
		t.Error("expected WithAttrs child to inherit filter behavior")
	}
}

func TestFilterHandlerEnabled(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	fh := newFilterHandler(base)

	if fh.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected debug to be disabled when base handler is at warn level")
	}
	if !fh.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("expected warn to be enabled")
	}
}

func TestFilterHandlerMultipleFlags(t *testing.T) {
	var buf bytes.Buffer
	logger, fh := newTestFilterLogger(&buf)

	fh.setFlags([]string{"update", "open"})

	logger.Debug("update-msg", "flag", "update")
	if !strings.Contains(buf.String(), "update-msg") {
		t.Error("expected update flag to match")
	}

	buf.Reset()
	logger.Debug("open-msg", "flag", "open")
	if !strings.Contains(buf.String(), "open-msg") {
		t.Error("expected open flag to match")
	}

	buf.Reset()
	logger.Debug("keepalive-msg", "flag", "keepalive")
	if strings.Contains(buf.String(), "keepalive-msg") {
		t.Error("expected keepalive to be blocked")
	}
}

func TestFilterHandlerMultiScopeAllMustMatch(t *testing.T) {
	var buf bytes.Buffer
	logger, fh := newTestFilterLogger(&buf)

	fh.setScopes(map[string]string{"neighbor": "192.0.2.1", "direction": "receive"})

	logger.Debug("both-match", "neighbor", "192.0.2.1", "direction", "receive")
	if !strings.Contains(buf.String(), "both-match") {
		t.Error("expected message with both scopes matching to pass")
	}

	buf.Reset()
	logger.Debug("neighbor-only", "neighbor", "192.0.2.1", "direction", "send")
	if strings.Contains(buf.String(), "neighbor-only") {
		t.Error("expected message with wrong direction to be blocked even if neighbor matches")
	}

	buf.Reset()
	logger.Debug("direction-only", "neighbor", "192.0.2.2", "direction", "receive")
	if strings.Contains(buf.String(), "direction-only") {
		t.Error("expected message with wrong neighbor to be blocked even if direction matches")
	}

	buf.Reset()
	logger.Debug("missing-scope", "neighbor", "192.0.2.1")
	if strings.Contains(buf.String(), "missing-scope") {
		t.Error("expected message missing direction attribute to be blocked when both scopes required")
	}
}
