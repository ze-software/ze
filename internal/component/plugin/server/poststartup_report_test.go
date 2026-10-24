package server

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// TestPostStartupFailureWarnsWhileRunning proves a post-startup callback that
// fails while the daemon runs is reported at Warn, naming the plugin, the
// cause and the command, and that the shutdown race alone stays at Debug.
// Method: capture the package logger at Info, report one failure with a live
// server context and one with a cancelled one, and read what reached Info.
func TestPostStartupFailureWarnsWhileRunning(t *testing.T) {
	orig := logger
	t.Cleanup(func() { logger = orig })

	var buf bytes.Buffer
	captured := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger = func() *slog.Logger { return captured }

	running := &Server{ctx: context.Background()}
	running.reportPostStartupFailure("conf-watchdog", errors.New("exec: le: not found in PATH"))
	out := buf.String()
	for _, want := range []string{"level=WARN", "conf-watchdog", "exec: le: not found in PATH", "ze show plugin list"} {
		if !strings.Contains(out, want) {
			t.Fatalf("running daemon: log %q lacks %q", out, want)
		}
	}

	buf.Reset()
	stopped, cancel := context.WithCancel(context.Background())
	cancel()
	stopping := &Server{ctx: stopped}
	stopping.reportPostStartupFailure("conf-watchdog", errors.New("connection closed"))
	if buf.Len() != 0 {
		t.Fatalf("stopping daemon: the expected close race reached Info: %q", buf.String())
	}

	running.reportPostStartupFailure("conf-watchdog", nil)
	if buf.Len() != 0 {
		t.Fatalf("a delivered callback logged: %q", buf.String())
	}
}
