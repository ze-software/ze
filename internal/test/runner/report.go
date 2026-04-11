// Design: docs/architecture/testing/ci-format.md — test runner framework

package runner

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Report generates AI-friendly failure output.
type Report struct {
	colors *Colors
	output io.Writer
	label  string // test suite label for debug commands (e.g., "encode", "plugin")
}

// NewReport creates a new report generator.
func NewReport(colors *Colors) *Report {
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

// PrintFailure prints a detailed failure report for a test.
func (r *Report) PrintFailure(rec *Record) {
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

	// Debug commands
	r.printDebugCommands(rec)

	r.writeln(c.DoubleSeparator())
	r.writeln("")
}

func (r *Report) printTimeoutReport(rec *Record) {
	c := r.colors

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
			c.Red(fmt.Sprintf("waiting for message %d", waitingFor)))
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

func (r *Report) printMismatchReport(rec *Record) {
	c := r.colors

	// Debug: show raw peer output to diagnose mismatch
	if rec.PeerOutput != "" {
		r.writeln(c.LineSeparator())
		r.writeln(c.Yellow("RAW PEER OUTPUT (first 3000 chars):"))
		r.writeln(c.LineSeparator())
		out := rec.PeerOutput
		if len(out) > 3000 {
			out = out[:3000] + "..."
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
	if msg := rec.GetMessage(expectedIdx); msg != nil {
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

	if msg := rec.GetMessage(expectedIdx); msg != nil && rcvIdx < len(rec.ReceivedRaw) {
		received := rec.ReceivedRaw[rcvIdx]
		diff := ColoredDiff(msg.RawHex, received, c)
		r.write(diff)
	}
	r.writeln("")
}

func (r *Report) printGenericReport(rec *Record) {
	c := r.colors

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
		suite = "encode"
	}
	r.writef("%s\n", c.Gray("# Run single test:"))
	r.writef("ze-test bgp %s %s\n\n", suite, rec.Nick)
	r.writef("%s\n", c.Gray("# Run test manually (server/client):"))
	r.writef("ze-test bgp %s --server %s\n", suite, rec.Nick)
	r.writef("ze-test bgp %s --client %s\n", suite, rec.Nick)
	r.writeln("")
}

// PrintAllFailures prints failure reports for all failed tests.
func (r *Report) PrintAllFailures(tests *Tests) {
	for _, rec := range tests.FailedRecords() {
		r.PrintFailure(rec)
	}
}

// writef writes formatted text to the report output, handling the error.
func (r *Report) writef(format string, args ...any) {
	if _, err := fmt.Fprintf(r.output, format, args...); err != nil {
		return // report output is best-effort
	}
}

// writeln writes a line to the report output, handling the error.
func (r *Report) writeln(s string) {
	if _, err := fmt.Fprintln(r.output, s); err != nil {
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
		return fmt.Sprintf("Partial exchange (%d/%d messages) — check message %d expectations against config",
			received, expected, received+1)
	}

	return "All expected messages received but test still timed out — check for extra unexpected messages"
}

// Helper functions

func formatHex(h string) string {
	// Truncate long hex for readability in display lines.
	// Full hex is available in the DEBUG section's decode commands.
	h = strings.ReplaceAll(h, ":", "")
	if len(h) > 80 {
		return h[:80] + "..."
	}
	return h
}

func truncateOutput(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n") + "\n... (truncated)"
}

func indentLines(s, indent string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = indent + line
		}
	}
	return strings.Join(lines, "\n")
}
