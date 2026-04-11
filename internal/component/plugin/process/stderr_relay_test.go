package process

import (
	"log/slog"
	"testing"
)

// TestClassifyStderrLineDropsBelowRelayLevel verifies the existing filter
// behavior: a low-priority slog line is dropped when relayLevel is higher.
//
// VALIDATES: debug lines below WARN are skipped.
// PREVENTS: Regression of ze.log.relay filter being bypassed by refactor.
func TestClassifyStderrLineDropsBelowRelayLevel(t *testing.T) {
	line := `time=2026-04-07T10:00:00 level=DEBUG msg="fine-grained trace" peer=10.0.0.1`
	_, _, _, _, skip := classifyStderrLine(line, false, slog.LevelWarn)
	if !skip {
		t.Fatal("expected DEBUG line below WARN relayLevel to be skipped")
	}
}

// TestClassifyStderrLinePassesAtOrAboveRelayLevel verifies a WARN line
// clears the filter when relayLevel=WARN.
//
// VALIDATES: Lines at the configured level are relayed.
// PREVENTS: Off-by-one mis-filtering in level comparison.
func TestClassifyStderrLinePassesAtOrAboveRelayLevel(t *testing.T) {
	line := `time=2026-04-07T10:00:00 level=WARN msg="slow handler" peer=10.0.0.1`
	level, msg, _, inPanic, skip := classifyStderrLine(line, false, slog.LevelWarn)
	if skip {
		t.Fatal("expected WARN line to be relayed at WARN relayLevel")
	}
	if level != slog.LevelWarn {
		t.Fatalf("level = %v, want WARN", level)
	}
	if msg != "slow handler" {
		t.Fatalf("msg = %q, want %q", msg, "slow handler")
	}
	if inPanic {
		t.Fatal("valid slog line should not set inPanic")
	}
}

// TestClassifyStderrLinePanicForcedToError is the core regression guard for
// the bug documented in known-failures entry "SDK NewFromTLSEnv missing
// initCallbackDefaults": a plugin process panic (panic: ... + goroutine
// trace) parsed as LevelInfo and was silently dropped by the default WARN
// relay filter. classifyStderrLine must force panic-block lines to ERROR so
// they always reach the engine logs.
//
// VALIDATES: Panic prefix forces ERROR and skip=false at WARN relayLevel.
// PREVENTS: A plugin panic being silently swallowed by the relay filter.
func TestClassifyStderrLinePanicForcedToError(t *testing.T) {
	level, _, _, inPanic, skip := classifyStderrLine(
		"panic: runtime error: index out of range [5] with length 3",
		false,
		slog.LevelWarn,
	)
	if skip {
		t.Fatal("panic line must not be filtered out by relayLevel")
	}
	if level != slog.LevelError {
		t.Fatalf("level = %v, want ERROR", level)
	}
	if !inPanic {
		t.Fatal("panic line must set inPanic=true so follow-up lines also reach ERROR")
	}
}

// TestClassifyStderrLinePanicBlockContinuation verifies the stack-trace
// lines that follow "panic:" (goroutine header, function frames, file
// paths, exit status) inherit ERROR level even though they are plain text.
//
// VALIDATES: Continuation lines in an active panic block stay at ERROR.
// PREVENTS: Partial relay where only the "panic:" line reaches logs but
// the stack trace needed to diagnose it is dropped.
func TestClassifyStderrLinePanicBlockContinuation(t *testing.T) {
	inPanic := true
	// Realistic Go runtime panic follow-up lines. None of these contain
	// level=/msg= so they all parse as LevelInfo in isolation.
	for _, line := range []string{
		"",
		"goroutine 1 [running]:",
		"main.doStuff(...)",
		"\t/path/to/file.go:42 +0x1f",
		"exit status 2",
	} {
		var (
			level slog.Level
			skip  bool
		)
		level, _, _, inPanic, skip = classifyStderrLine(line, inPanic, slog.LevelWarn)
		if skip {
			t.Fatalf("panic-block continuation %q was filtered out", line)
		}
		if level != slog.LevelError {
			t.Fatalf("line %q level = %v, want ERROR", line, level)
		}
		if !inPanic {
			t.Fatalf("line %q cleared inPanic mid-stack", line)
		}
	}
}

// TestClassifyStderrLinePanicBlockEndsOnValidSlog verifies that if a
// plugin emits a valid slog line after a panic prefix (unlikely, but
// possible e.g. when two plugins share a stderr pipe), the classifier
// exits panic mode and resumes normal level-based filtering.
//
// VALIDATES: A well-formed slog line resets inPanic.
// PREVENTS: A spurious early "panic:" line permanently forcing every
// subsequent line to ERROR for the lifetime of the process.
func TestClassifyStderrLinePanicBlockEndsOnValidSlog(t *testing.T) {
	slogLine := `time=2026-04-07T10:00:00 level=INFO msg="resumed" peer=10.0.0.1`
	level, _, _, inPanic, skip := classifyStderrLine(slogLine, true, slog.LevelWarn)
	if !skip {
		t.Fatal("INFO line below WARN relayLevel should be skipped after panic block ends")
	}
	if level != slog.LevelInfo {
		t.Fatalf("level = %v, want INFO", level)
	}
	if inPanic {
		t.Fatal("valid slog line must clear inPanic")
	}
}

// TestClassifyStderrLineFatalErrorPrefix verifies that "fatal error:"
// (emitted by the Go runtime for unrecoverable errors like deadlock
// detection and out-of-memory) is recognized alongside "panic:".
//
// VALIDATES: fatal error: prefix also triggers ERROR forcing.
// PREVENTS: Go runtime fatal errors being filtered out while panics are not.
func TestClassifyStderrLineFatalErrorPrefix(t *testing.T) {
	line := "fatal error: all goroutines are asleep - deadlock!"
	level, _, _, inPanic, skip := classifyStderrLine(line, false, slog.LevelWarn)
	if skip {
		t.Fatal("fatal error line must not be filtered out")
	}
	if level != slog.LevelError {
		t.Fatalf("level = %v, want ERROR", level)
	}
	if !inPanic {
		t.Fatal("fatal error must set inPanic=true")
	}
}

// TestClassifyStderrLinePanicInMessageNotMatched verifies that a plugin
// log line whose msg contains the word "panic" is NOT treated as a panic
// start. Only lines that begin with "panic:" or "fatal error:" should
// trigger the forced-ERROR override.
//
// VALIDATES: Mid-message "panic" does not trigger panic-block mode.
// PREVENTS: Noisy over-escalation of plugin log messages that happen to
// discuss panics in their prose.
func TestClassifyStderrLinePanicInMessageNotMatched(t *testing.T) {
	line := `time=2026-04-07T10:00:00 level=INFO msg="recovered from panic in handler" peer=10.0.0.1`
	level, _, _, inPanic, skip := classifyStderrLine(line, false, slog.LevelWarn)
	if !skip {
		t.Fatal("INFO line below WARN relayLevel should be skipped")
	}
	if level != slog.LevelInfo {
		t.Fatalf("level = %v, want INFO (not ERROR)", level)
	}
	if inPanic {
		t.Fatal("mid-message 'panic' must not trigger panic block")
	}
}
