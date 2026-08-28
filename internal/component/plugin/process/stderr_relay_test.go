package process

import (
	"bufio"
	"errors"
	"log/slog"
	"os/exec"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestStderrRelayKeepsTheLineWrittenJustBeforeExit drives the same three
// primitives startExternal and monitorCmd use -- attachStderrRelay, a relay
// goroutine reading to EOF, and drainStderrRelay after Cmd.Wait -- against a
// child that writes one line and exits at once. That is the shape of every
// plugin verdict: internal/test/cli/cmd_engine_steps.go logs "all steps passed"
// or "ZE-OBSERVER-FAIL" and then asks the daemon to shut down, which kills it.
//
// VALIDATES: a line written immediately before the child exits still reaches the
// relay, on every one of the runs below.
// PREVENTS: the regression that made test/ipsec/ipsec-clear-sa.ci and
// ipsec-sa-show.ci red on linux CI (run 31225029268) while passing on an idle
// machine. Cmd.StderrPipe hands Cmd.Wait the read end, and Wait closes it as
// soon as the child exits ("it is thus incorrect to call Wait before all reads
// from the pipe have completed", os/exec), so whatever the reader had not yet
// consumed was discarded. The verdict line vanished and the .ci reported only
// that its stderr did not contain the text.
//
// DISCRIMINATES: swap attachStderrRelay for cmd.StderrPipe() and this test
// fails, because that is the wiring the bug lived in.
func TestStderrRelayKeepsTheLineWrittenJustBeforeExit(t *testing.T) {
	const (
		runs    = 60
		verdict = "plugin verdict written just before exit"
	)
	for run := range runs {
		cmd := exec.CommandContext(t.Context(), "/bin/sh", "-c", "printf '"+verdict+"\\n' >&2") //nolint:gosec // fixed argv, no user input
		read, write, err := attachStderrRelay(cmd)
		if err != nil {
			t.Fatalf("run %d: attachStderrRelay: %v", run, err)
		}
		if startErr := cmd.Start(); startErr != nil {
			write.Close() //nolint:errcheck,gosec // test cleanup
			read.Close()  //nolint:errcheck,gosec // test cleanup
			t.Fatalf("run %d: start: %v", run, startErr)
		}
		// The child holds its own descriptor. Closing the parent's copy is what
		// turns the child's exit into EOF for the reader.
		if closeErr := write.Close(); closeErr != nil {
			t.Fatalf("run %d: close write end: %v", run, closeErr)
		}

		relayed := make(chan string, 4)
		relayDone := make(chan struct{})
		go func() {
			defer close(relayDone)
			scanner := bufio.NewScanner(read)
			for scanner.Scan() {
				select {
				case relayed <- scanner.Text():
				default:
				}
			}
		}()

		_ = cmd.Wait() //nolint:errcheck // the child's exit status is not what this test asserts
		drainStderrRelay(relayDone, read, stderrDrainGrace)
		<-relayDone
		close(relayed)

		var seen bool
		for line := range relayed {
			if strings.Contains(line, verdict) {
				seen = true
			}
		}
		if !seen {
			t.Fatalf("run %d: the child's last stderr line never reached the relay; it was discarded when the child exited", run)
		}
	}
}

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

// TestClassifyStderrLineRelaysPlainText verifies that non-slog plugin stderr
// is surfaced at the default WARN relay threshold.
//
// VALIDATES: plain stderr lines are treated as WARN.
// PREVENTS: observer success/failure text disappearing behind the default filter.
func TestClassifyStderrLineRelaysPlainText(t *testing.T) {
	line := "OK: cursor commands accepted"
	level, msg, _, inPanic, skip := classifyStderrLine(line, false, slog.LevelWarn)
	if skip {
		t.Fatal("plain stderr line should be relayed at WARN relayLevel")
	}
	if level != slog.LevelWarn {
		t.Fatalf("level = %v, want WARN", level)
	}
	if msg != line {
		t.Fatalf("msg = %q, want %q", msg, line)
	}
	if inPanic {
		t.Fatal("plain stderr line should not enter panic mode")
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

// observerSentinelReason is a failure reason carrying both characters the slog
// text format escapes: a double quote and a backslash. The quoted token is the
// shape the plugin engine really produces -- run 31225029268 relayed
// `expected 'add' or 'del' before prefix: got "destination-ipv4"`.
const observerSentinelReason = `RPC error: got "destination-ipv4" and a back\slash`

// observerSentinelLine is the exact stderr line fixture.ReportFailure
// (internal/test/fixture/fixture.go) writes for observerSentinelReason.
//
// The literal pins the compiled producer's slog text contract at the relay
// boundary. Before this test existed, a writer that emitted an unescaped quote
// truncated every relayed reason at the quote and nothing noticed.
const observerSentinelLine = `time=runtime level=ERROR msg="ZE-OBSERVER-FAIL: RPC error: got \"destination-ipv4\" and a back\\slash" subsystem=test.observer`

// TestClassifyStderrLineDecodesObserverSentinelEscapes drives the relay's own
// classifier over the observer sentinel as the observer writes it.
//
// VALIDATES: a reason containing a double quote and a backslash reaches the
// relay intact, with the trailing subsystem attribute still an attribute.
// PREVENTS: the loss seen in run 31225029268, where msg closed at the reason's
// own quote and the remainder became the attribute key
// `original.destination-ipv4\"\" subsystem`. The ZE-OBSERVER-FAIL marker
// survived that, so the test still failed -- but the reason a reader needs to
// act on was cut off exactly where it named the cause.
//
// DISCRIMINATES: drop the escape decoding from slogutil.ParseLogLine and msg
// comes back carrying literal backslashes, which slog then escapes again on
// re-emit; revert findClosingQuote to its byte-before test and the backslash
// before the closing quote swallows the subsystem attribute.
func TestClassifyStderrLineDecodesObserverSentinelEscapes(t *testing.T) {
	level, msg, attrs, inPanic, skip := classifyStderrLine(observerSentinelLine, false, slog.LevelWarn)

	if skip {
		t.Fatal("an ERROR sentinel must never be dropped by the relay filter")
	}
	if level != slog.LevelError {
		t.Fatalf("level = %v, want ERROR", level)
	}
	if inPanic {
		t.Fatal("a valid slog line must not open a panic block")
	}

	want := "ZE-OBSERVER-FAIL: " + observerSentinelReason
	if msg != want {
		t.Fatalf("msg = %q, want %q", msg, want)
	}

	var subsystem string
	for i := 0; i+1 < len(attrs); i += 2 {
		if attrs[i] == "subsystem" {
			subsystem, _ = attrs[i+1].(string)
		}
	}
	if subsystem != "test.observer" {
		t.Fatalf("subsystem attr = %q, want %q -- the reason's escapes must not "+
			"swallow the attributes that follow it", subsystem, "test.observer")
	}
}

// failingStderr yields prefix, then fails. It models the plugin's stderr pipe
// breaking part way through a line.
type failingStderr struct {
	prefix []byte
	err    error
}

func (f *failingStderr) Read(p []byte) (int, error) {
	if len(f.prefix) == 0 {
		return 0, f.err
	}
	n := copy(p, f.prefix)
	f.prefix = f.prefix[n:]
	return n, nil
}

// captureRelay swaps the package's relay logger for one writing into buf, and
// restores it when the test ends. stderrLogger is a package var of func type,
// which is what makes the relay's own output assertable.
func captureRelay(t *testing.T, buf *strings.Builder) {
	t.Helper()
	orig := stderrLogger
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	stderrLogger = func() *slog.Logger { return logger }
	t.Cleanup(func() { stderrLogger = orig })
}

// TestRelayStderrReportsAReadFailure drives a stderr pipe that breaks part way
// through a line.
//
// VALIDATES: the relay says it stopped.
// PREVENTS: the relay ending for the life of the plugin with nothing logged, so
// that a panic printed later never reaches the engine log. That silent loss is
// the exact failure the panic-forcing rule in relayStderrFrom exists to stop,
// and an unchecked scanner reopened it one layer down.
func TestRelayStderrReportsAReadFailure(t *testing.T) {
	var out strings.Builder
	captureRelay(t, &out)

	p := &Process{config: plugin.PluginConfig{Name: "test-plugin"}}
	p.relayStderrFrom(&failingStderr{
		prefix: []byte("time=2026-01-01 level=ERROR msg=\"first line\"\npartial"),
		err:    errors.New("pipe broke"),
	})

	got := out.String()
	if !strings.Contains(got, "relay stopped") {
		t.Fatalf("the relay did not report that it stopped:\n%s", got)
	}
	if !strings.Contains(got, "pipe broke") {
		t.Fatalf("the relay did not name the read failure:\n%s", got)
	}
	if !strings.Contains(got, "test-plugin") {
		t.Fatalf("the relay did not name the plugin:\n%s", got)
	}
}

// TestRelayStderrReportsAnOverLongLine covers the failure mode with no
// underlying I/O error: one plugin log line above bufio.MaxScanTokenSize.
func TestRelayStderrReportsAnOverLongLine(t *testing.T) {
	var out strings.Builder
	captureRelay(t, &out)

	p := &Process{config: plugin.PluginConfig{Name: "test-plugin"}}
	p.relayStderrFrom(strings.NewReader(strings.Repeat("x", bufio.MaxScanTokenSize+1)))

	if !strings.Contains(out.String(), "relay stopped") {
		t.Fatalf("an over-long line ended the relay with nothing logged:\n%s", out.String())
	}
}

// TestRelayStderrIsQuietOnAWholeStream pins the other side: a stream that ends
// at EOF reports no stoppage.
func TestRelayStderrIsQuietOnAWholeStream(t *testing.T) {
	var out strings.Builder
	captureRelay(t, &out)

	p := &Process{config: plugin.PluginConfig{Name: "test-plugin"}}
	p.relayStderrFrom(strings.NewReader("time=2026-01-01 level=ERROR msg=\"a whole line\"\n"))

	got := out.String()
	if strings.Contains(got, "relay stopped") {
		t.Fatalf("a clean EOF was reported as a stopped relay:\n%s", got)
	}
	if !strings.Contains(got, "a whole line") {
		t.Fatalf("the relayed line is missing:\n%s", got)
	}
}
