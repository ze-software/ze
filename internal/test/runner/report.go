// Design: docs/architecture/testing/ci-format.md — test runner framework

package runner

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/trace"
)

// Report generates AI-friendly failure output.
type Report struct {
	colors   *Colors
	output   io.Writer
	label    string    // test suite label for debug commands (e.g., "encode", "plugin")
	hostLoad *HostLoad // snapshot at run start; attached to failure groups when contended
}

// newReport creates a new report generator.
func newReport(colors *Colors) *Report {
	return &Report{
		colors: colors,
		output: os.Stdout,
	}
}

// SetOutput sets the output writer.
func (r *Report) SetOutput(w io.Writer) {
	r.output = w
}

// SetLabel sets the test suite label for debug commands.
func (r *Report) SetLabel(label string) {
	r.label = label
}

// setHostLoad records the host load snapshot taken at run start.
func (r *Report) setHostLoad(h *HostLoad) {
	r.hostLoad = h
}

// printFailure prints a detailed failure report for a test.
func (r *Report) printFailure(rec *Record) {
	c := r.colors

	// Header
	r.writeln(c.DoubleSeparator())
	r.writef("%s: %s %s\n", c.Red("TEST FAILURE"), c.Cyan(rec.Nick), rec.Name)
	r.writeln(c.DoubleSeparator())
	r.writeln("")

	// Config info
	if rec.ConfigFile != "" {
		r.writef("%s  %s\n", c.Yellow("CONFIG:"), rec.ConfigFile)
	}
	if rec.CIFile != "" {
		r.writef("%s %s\n", c.Yellow("CI FILE:"), rec.CIFile)
	}

	// Failure type
	failType := rec.FailureType
	if failType == "" {
		if rec.State == StateTimeout {
			failType = stateTimeout
		} else {
			failType = stateUnknown
		}
	}
	r.writef("%s    %s\n", c.Yellow("TYPE:"), c.Red(failType))
	r.writeln("")

	// Type-specific output
	switch failType {
	case stateTimeout:
		r.printTimeoutReport(rec)
	case FailTypeMismatch:
		r.printMismatchReport(rec)
	default:
		r.printGenericReport(rec)
	}

	// Step trace
	if len(rec.StepTrace) > 0 {
		r.writeln(c.Yellow("STEP TRACE:"))
		trace.PrintTrace(r.output, rec.Name, rec.StepTrace, c.Enabled())
		r.writeln("")
	}

	// Debug commands
	r.printDebugCommands(rec)

	r.writeln(c.DoubleSeparator())
	r.writeln("")
}

func (r *Report) printTimeoutReport(rec *Record) {
	c := r.colors

	r.printFailedPeers(rec)

	r.writeln(c.LineSeparator())
	r.writeln(c.Yellow("PROGRESS:"))
	r.writeln(c.LineSeparator())

	expectedCount := len(rec.Messages)
	if expectedCount == 0 {
		expectedCount = len(rec.Expects)
	}
	receivedCount := len(rec.ReceivedRaw)

	r.writef("  %s %d\n", c.Gray("expected messages:"), expectedCount)
	r.writef("  %s %d\n", c.Gray("received messages:"), receivedCount)

	waitingFor := receivedCount + 1
	if waitingFor <= expectedCount {
		r.writef("  %s            %s\n",
			c.Gray("status:"),
			c.Red(textbuf.StrInt("waiting for message ", int64(waitingFor))))
	}
	r.writeln("")

	// Show last received message
	if len(rec.ReceivedRaw) > 0 {
		lastIdx := len(rec.ReceivedRaw) - 1
		r.writeln(c.LineSeparator())
		r.writef("%s (message %d):\n", c.Yellow("LAST RECEIVED"), lastIdx+1)
		r.writeln(c.LineSeparator())

		rawHex := rec.ReceivedRaw[lastIdx]
		r.writef("%s     %s\n", c.Yellow("raw:"), formatHex(rawHex))

		if decoded, err := DecodeMessage(rawHex); err == nil {
			r.writef("%s\n", c.Yellow("decoded:"))
			r.write(ColoredString(decoded, c))
		}
		r.writeln("")
	}

	// Show expected next message
	nextIdx := len(rec.ReceivedRaw)
	if nextIdx < len(rec.Messages) {
		msg := rec.Messages[nextIdx]
		r.writeln(c.LineSeparator())
		r.writef("%s (message %d):\n", c.Yellow("EXPECTED NEXT"), nextIdx+1)
		r.writeln(c.LineSeparator())

		if msg.Cmd != "" {
			r.writef("%s     %s\n", c.Yellow("cmd:"), msg.Cmd)
		}
		if msg.RawHex != "" {
			r.writef("%s     %s\n", c.Yellow("raw:"), formatHex(msg.RawHex))
		}
		if msg.Decoded != "" {
			r.writef("%s\n%s\n", c.Yellow("decoded:"), indentLines(msg.Decoded, "  "))
		}
		r.writeln("")
	}

	// Likely cause hint for timeout
	if hint := likelyCauseTimeout(rec); hint != "" {
		r.writeln(c.LineSeparator())
		r.writeln(c.Yellow("LIKELY CAUSE:"))
		r.writeln(c.LineSeparator())
		r.writef("  %s\n", hint)
		r.writeln("")
	}

	// Client output
	if rec.ClientOutput != "" {
		r.writeln(c.LineSeparator())
		r.writeln(c.Yellow("CLIENT OUTPUT:"))
		r.writeln(c.LineSeparator())
		r.writeln(truncateOutput(rec.ClientOutput, 200))
		if strings.TrimSpace(rec.ClientOutput) == "" {
			r.writef("%s\n", c.Gray("(no output - likely stuck or missing feature)"))
		}
		r.writeln("")
	}
}

// printFailedPeers names the check-mode peers that did not report a clean
// exchange, before any joined output is dumped.
//
// Called from EVERY failure report, not only the mismatch one: in a multi-peer
// test the joined dump interleaves all peers, and the peer that SUCCEEDED is
// usually the loudest thing in it.
func (r *Report) printFailedPeers(rec *Record) {
	if len(rec.FailedPeers) == 0 {
		return
	}
	c := r.colors
	r.writeln(c.LineSeparator())
	var tb textbuf.Buffer
	r.writeln(c.Yellow(tb.Str("FAILED CHECK PEERS: ").Join(rec.FailedPeers, ", ").String()))
}

func (r *Report) printMismatchReport(rec *Record) {
	c := r.colors

	r.printFailedPeers(rec)

	// Debug: show raw peer output to diagnose mismatch
	if rec.PeerOutput != "" {
		r.writeln(c.LineSeparator())
		r.writeln(c.Yellow("RAW PEER OUTPUT (first 3000 chars):"))
		r.writeln(c.LineSeparator())
		out := rec.PeerOutput
		if len(out) > 3000 {
			var tb textbuf.Buffer
			out = tb.Str(out[:3000]).Str("...").String()
		}
		r.writeln(out)
		r.writeln("")
	}

	// Client output: ze's stderr/log is often where the real cause shows up
	// (parser errors, config warnings, plugin failures) even when the failure
	// surfaces as a wire mismatch.
	if rec.ClientOutput != "" {
		r.writeln(c.LineSeparator())
		r.writeln(c.Yellow("CLIENT OUTPUT:"))
		r.writeln(c.LineSeparator())
		r.writeln(truncateOutput(rec.ClientOutput, 30))
		r.writeln("")
	}

	msgIdx := rec.LastExpectedIdx
	if msgIdx == 0 {
		msgIdx = 1
	}

	// Expected message
	r.writeln(c.LineSeparator())
	r.writef("%s %d:\n", c.Cyan("EXPECTED MESSAGE"), msgIdx)
	r.writeln(c.LineSeparator())

	// For multi-connection tests, don't use Nick-based offset since Nick is
	// the test identifier, not the connection letter. Just show first message.
	// Future: parse actual connection info from testpeer output.
	expectedIdx := msgIdx
	if msg := rec.getMessage(expectedIdx); msg != nil {
		if msg.Cmd != "" {
			r.writef("%s     %s\n", c.Yellow("cmd:"), msg.Cmd)
		}
		if msg.RawHex != "" {
			r.writef("%s     %s\n", c.Yellow("raw:"), formatHex(msg.RawHex))
		}
		if decoded, err := DecodeMessage(msg.RawHex); err == nil {
			r.writef("%s\n", c.Yellow("decoded:"))
			r.write(ColoredString(decoded, c))
		}
	}
	r.writeln("")

	// Received message
	r.writeln(c.LineSeparator())
	r.writef("%s %d:\n", c.Cyan("RECEIVED MESSAGE"), msgIdx)
	r.writeln(c.LineSeparator())

	// Use LastReceivedIdx directly (0-based from extractMismatchIndices)
	rcvIdx := rec.LastReceivedIdx
	// Fallback: if calculated index is out of bounds, use last available message
	if rcvIdx >= len(rec.ReceivedRaw) && len(rec.ReceivedRaw) > 0 {
		rcvIdx = len(rec.ReceivedRaw) - 1
	}
	if rcvIdx < len(rec.ReceivedRaw) {
		rawHex := rec.ReceivedRaw[rcvIdx]
		r.writef("%s     %s\n", c.Yellow("raw:"), formatHex(rawHex))
		if decoded, err := DecodeMessage(rawHex); err == nil {
			r.writef("%s\n", c.Yellow("decoded:"))
			r.write(ColoredString(decoded, c))
		}
	}
	r.writeln("")

	// Diff
	r.writeln(c.LineSeparator())
	r.writeln(c.Yellow("DIFF:"))
	r.writeln(c.LineSeparator())

	if msg := rec.getMessage(expectedIdx); msg != nil && rcvIdx < len(rec.ReceivedRaw) {
		received := rec.ReceivedRaw[rcvIdx]
		diff := ColoredDiff(msg.RawHex, received, c)
		r.write(diff)
	}
	r.writeln("")
}

func (r *Report) printGenericReport(rec *Record) {
	c := r.colors

	r.printFailedPeers(rec)

	// Show error if any
	if rec.Error != nil {
		r.writeln(c.LineSeparator())
		r.writeln(c.Yellow("ERROR:"))
		r.writeln(c.LineSeparator())
		r.writef("%s\n", c.Red(rec.Error.Error()))
		r.writeln("")
	}

	// Likely cause hint
	if hint := likelyCause(rec); hint != "" {
		r.writeln(c.LineSeparator())
		r.writeln(c.Yellow("LIKELY CAUSE:"))
		r.writeln(c.LineSeparator())
		r.writef("  %s\n", hint)
		r.writeln("")
	}

	// Peer output
	if rec.PeerOutput != "" {
		r.writeln(c.LineSeparator())
		r.writeln(c.Yellow("PEER OUTPUT:"))
		r.writeln(c.LineSeparator())
		r.writeln(truncateOutput(rec.PeerOutput, 30))
		r.writeln("")
	}

	// Client output
	if rec.ClientOutput != "" {
		r.writeln(c.LineSeparator())
		r.writeln(c.Yellow("CLIENT OUTPUT:"))
		r.writeln(c.LineSeparator())
		r.writeln(truncateOutput(rec.ClientOutput, 30))
		r.writeln("")
	}
}

func (r *Report) printDebugCommands(rec *Record) {
	c := r.colors

	r.writeln(c.LineSeparator())
	r.writeln(c.Yellow("DEBUG:"))
	r.writeln(c.LineSeparator())

	// Decode commands
	if len(rec.Messages) > 0 && rec.Messages[0].RawHex != "" {
		r.writef("%s\n", c.Gray("# Decode expected:"))
		r.writef("ze bgp decode update %s\n\n", rec.Messages[0].RawHex)
	}

	if len(rec.ReceivedRaw) > 0 {
		r.writef("%s\n", c.Gray("# Decode received:"))
		r.writef("ze bgp decode update %s\n\n", rec.ReceivedRaw[0])
	}

	// Rerun commands
	suite := r.label
	if suite == "" {
		suite = defaultFailureSuite
	}
	r.writef("%s\n", c.Gray("# Run single test:"))
	r.writef("%s\n\n", formatRecordRerunCommand(suite, rec))
	if supportsServerClientDebug(suite) {
		r.writef("%s\n", c.Gray("# Run test manually (server/client):"))
		r.writef("%s\n", FormatRerunCommand(suite, []string{"--server", rec.Nick}))
		r.writef("%s\n", FormatRerunCommand(suite, []string{"--client", rec.Nick}))
	}
	r.writeln("")
}

// printAllFailures prints failure reports for all failed tests.
func (r *Report) printAllFailures(tests *Tests) {
	for _, rec := range tests.failedRecords() {
		r.printFailure(rec)
	}
}

// writef writes formatted text to the report output, handling the error.
func (r *Report) writef(format string, args ...any) {
	if _, err := fmt.Fprintf(r.output, format, args...); err != nil { //nolint:errcheck // output
		return // report output is best-effort
	}
}

// writeln writes a line to the report output, handling the error.
func (r *Report) writeln(s string) {
	if _, err := fmt.Fprintln(r.output, s); err != nil { //nolint:errcheck // output
		return // report output is best-effort
	}
}

// write writes a string to the report output without format interpretation.
func (r *Report) write(s string) {
	if _, err := fmt.Fprint(r.output, s); err != nil {
		return // report output is best-effort
	}
}

// likelyCause returns a diagnostic hint based on the failure record.
func likelyCause(rec *Record) string {
	// Empty client output is the most common problem
	if strings.TrimSpace(rec.ClientOutput) == "" && rec.Error != nil {
		errMsg := rec.Error.Error()
		if strings.Contains(errMsg, "exec:") || strings.Contains(errMsg, "not found") {
			return "Binary not found — run 'make build' or check PATH"
		}
		if strings.Contains(errMsg, "connection refused") {
			return "Server not listening — check config address/port"
		}
		return "Client produced no output — may have crashed, missing feature, or wrong config"
	}

	if strings.TrimSpace(rec.ClientOutput) == "" && rec.Error == nil {
		return "Client produced no output — may have crashed or failed silently"
	}

	if rec.Error != nil {
		errMsg := rec.Error.Error()
		if strings.Contains(errMsg, "signal: killed") || strings.Contains(errMsg, "signal: segmentation") {
			return "Process crashed — check for nil pointer or resource exhaustion"
		}
		if strings.Contains(errMsg, "exit status") {
			return "Process exited with error — check CLIENT OUTPUT below for details"
		}
	}

	return ""
}

// likelyCauseTimeout returns a diagnostic hint for timeout failures.
func likelyCauseTimeout(rec *Record) string {
	// A recorded error beats every guess below. awaitDaemonStderr sets
	// rec.Error to the exact needle and budget that expired
	// (await_stderr.go), and the heuristics that followed replaced it with
	// "server likely failed to start or crashed" -- which for an await fence
	// is simply wrong: the daemon started fine and never printed the awaited
	// line. Diagnosing test/plugin/as112-external-refuses.ci cost several
	// reproduction rounds for exactly that reason (ai/rules/cli.md:
	// a failure must say what failed, not guess).
	if rec.Error != nil {
		return rec.Error.Error()
	}

	received := len(rec.ReceivedRaw)
	expected := len(rec.Messages)
	if expected == 0 {
		expected = len(rec.Expects)
	}

	if received == 0 {
		if strings.TrimSpace(rec.ClientOutput) == "" {
			return "No messages received, no client output — server likely failed to start or crashed"
		}
		return "No messages received — check OPEN negotiation (capabilities, ASN, hold-time)"
	}

	if received < expected {
		var b textbuf.Buffer
		return b.Reset().Str("Partial exchange (").Int(int64(received)).Byte('/').Int(int64(expected)).Str(" messages) — check message ").Int(int64(received + 1)).Str(" expectations against config").String()
	}

	return "All expected messages received but test still timed out — check for extra unexpected messages"
}

// Helper functions

func formatHex(h string) string {
	// Truncate long hex for readability in display lines.
	// Full hex is available in the DEBUG section's decode commands.
	h = strings.ReplaceAll(h, ":", "")
	if len(h) > 80 {
		var tb textbuf.Buffer
		return tb.Str(h[:80]).Str("...").String()
	}
	return h
}

// truncateOutput keeps the head AND the tail of a long capture, because a
// failure's diagnosis is usually in neither half alone: startup lines say what
// the daemon became, and the last lines say what it was doing when it failed.
//
// It kept the first maxLines only until 2026-09-04, and that cost a session
// several hours. A fixture printing a per-round diagnostic had every one of
// those lines relayed and then cut, while the single line it printed before its
// loop survived, so the output looked exactly as though the later prints had
// never happened. The session concluded the plugin stderr relay dropped them,
// wrote that in a journal row and a spec, committed both, and had to retract
// them: the relay has no cap and does not stop at ready
// (attachStderrRelay, internal/component/plugin/process/process.go). Head-only
// truncation is indistinguishable from a producer that went silent.
//
// The elision line names how many lines went, so a reader can tell a truncated
// capture from a short one.
func truncateOutput(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	head := maxLines / 2
	tail := maxLines - head
	elided := len(lines) - maxLines
	var tb textbuf.Buffer
	tb.Join(lines[:head], "\n")
	tb.Str("\n... (")
	tb.Int(int64(elided))
	tb.Str(" lines elided; head and tail shown)\n")
	tb.Join(lines[len(lines)-tail:], "\n")
	return tb.String()
}

func indentLines(s, indent string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			var tb textbuf.Buffer
			lines[i] = tb.Str(indent).Str(line).String()
		}
	}
	return textbuf.Join(lines, "\n")
}
