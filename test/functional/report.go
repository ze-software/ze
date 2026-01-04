package functional

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

// PrintFailure prints a detailed failure report for a test.
func (r *Report) PrintFailure(rec *Record) {
	c := r.colors

	// Header
	_, _ = fmt.Fprintln(r.output, c.DoubleSeparator())
	_, _ = fmt.Fprintf(r.output, "%s: %s %s\n",
		c.Red("TEST FAILURE"),
		c.Cyan(rec.Nick),
		rec.Name)
	_, _ = fmt.Fprintln(r.output, c.DoubleSeparator())
	_, _ = fmt.Fprintln(r.output)

	// Config info
	if rec.ConfigFile != "" {
		_, _ = fmt.Fprintf(r.output, "%s  %s\n", c.Yellow("CONFIG:"), rec.ConfigFile)
	}
	if rec.CIFile != "" {
		_, _ = fmt.Fprintf(r.output, "%s %s\n", c.Yellow("CI FILE:"), rec.CIFile)
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
	_, _ = fmt.Fprintf(r.output, "%s    %s\n", c.Yellow("TYPE:"), c.Red(failType))
	_, _ = fmt.Fprintln(r.output)

	// Type-specific output
	switch failType {
	case stateTimeout:
		r.printTimeoutReport(rec)
	case "mismatch":
		r.printMismatchReport(rec)
	default:
		r.printGenericReport(rec)
	}

	// Debug commands
	r.printDebugCommands(rec)

	_, _ = fmt.Fprintln(r.output, c.DoubleSeparator())
	_, _ = fmt.Fprintln(r.output)
}

func (r *Report) printTimeoutReport(rec *Record) {
	c := r.colors

	_, _ = fmt.Fprintln(r.output, c.LineSeparator())
	_, _ = fmt.Fprintln(r.output, c.Yellow("PROGRESS:"))
	_, _ = fmt.Fprintln(r.output, c.LineSeparator())

	expectedCount := len(rec.Messages)
	if expectedCount == 0 {
		expectedCount = len(rec.Expects)
	}
	receivedCount := len(rec.ReceivedRaw)

	_, _ = fmt.Fprintf(r.output, "  %s %d\n", c.Gray("expected messages:"), expectedCount)
	_, _ = fmt.Fprintf(r.output, "  %s %d\n", c.Gray("received messages:"), receivedCount)

	waitingFor := receivedCount + 1
	if waitingFor <= expectedCount {
		_, _ = fmt.Fprintf(r.output, "  %s            %s\n",
			c.Gray("status:"),
			c.Red(fmt.Sprintf("waiting for message %d", waitingFor)))
	}
	_, _ = fmt.Fprintln(r.output)

	// Show last received message
	if len(rec.ReceivedRaw) > 0 {
		lastIdx := len(rec.ReceivedRaw) - 1
		_, _ = fmt.Fprintln(r.output, c.LineSeparator())
		_, _ = fmt.Fprintf(r.output, "%s (message %d):\n", c.Yellow("LAST RECEIVED"), lastIdx+1)
		_, _ = fmt.Fprintln(r.output, c.LineSeparator())

		rawHex := rec.ReceivedRaw[lastIdx]
		_, _ = fmt.Fprintf(r.output, "%s     %s\n", c.Yellow("raw:"), formatHex(rawHex))

		if decoded, err := DecodeMessage(rawHex); err == nil {
			_, _ = fmt.Fprintf(r.output, "%s\n", c.Yellow("decoded:"))
			_, _ = fmt.Fprint(r.output, decoded.ColoredString(c))
		}
		_, _ = fmt.Fprintln(r.output)
	}

	// Show expected next message
	nextIdx := len(rec.ReceivedRaw)
	if nextIdx < len(rec.Messages) {
		msg := rec.Messages[nextIdx]
		_, _ = fmt.Fprintln(r.output, c.LineSeparator())
		_, _ = fmt.Fprintf(r.output, "%s (message %d):\n", c.Yellow("EXPECTED NEXT"), nextIdx+1)
		_, _ = fmt.Fprintln(r.output, c.LineSeparator())

		if msg.Cmd != "" {
			_, _ = fmt.Fprintf(r.output, "%s     %s\n", c.Yellow("cmd:"), msg.Cmd)
		}
		if msg.RawHex != "" {
			_, _ = fmt.Fprintf(r.output, "%s     %s\n", c.Yellow("raw:"), formatHex(msg.RawHex))
		}
		if msg.Decoded != "" {
			_, _ = fmt.Fprintf(r.output, "%s\n%s\n", c.Yellow("decoded:"), indentLines(msg.Decoded, "  "))
		}
		_, _ = fmt.Fprintln(r.output)
	}

	// Client output
	if rec.ClientOutput != "" {
		_, _ = fmt.Fprintln(r.output, c.LineSeparator())
		_, _ = fmt.Fprintln(r.output, c.Yellow("CLIENT OUTPUT:"))
		_, _ = fmt.Fprintln(r.output, c.LineSeparator())
		_, _ = fmt.Fprintln(r.output, truncateOutput(rec.ClientOutput, 20))
		if strings.TrimSpace(rec.ClientOutput) == "" {
			_, _ = fmt.Fprintf(r.output, "%s\n", c.Gray("(no output - likely stuck or missing feature)"))
		}
		_, _ = fmt.Fprintln(r.output)
	}
}

func (r *Report) printMismatchReport(rec *Record) {
	c := r.colors

	msgIdx := rec.LastExpectedIdx
	if msgIdx == 0 {
		msgIdx = 1
	}

	_, _ = fmt.Fprintf(r.output, "%s    %s (message %d)\n",
		c.Yellow("TYPE:"),
		c.Red("mismatch"),
		msgIdx)
	_, _ = fmt.Fprintln(r.output)

	// Expected message
	_, _ = fmt.Fprintln(r.output, c.LineSeparator())
	_, _ = fmt.Fprintf(r.output, "%s %d:\n", c.Cyan("EXPECTED MESSAGE"), msgIdx)
	_, _ = fmt.Fprintln(r.output, c.LineSeparator())

	// Use connection offset for API tests with multiple connections (A, B, C)
	expectedIdx := rec.ConnectionOffset() + msgIdx
	if msg := rec.GetMessage(expectedIdx); msg != nil {
		if msg.Cmd != "" {
			_, _ = fmt.Fprintf(r.output, "%s     %s\n", c.Yellow("cmd:"), msg.Cmd)
		}
		if msg.RawHex != "" {
			_, _ = fmt.Fprintf(r.output, "%s     %s\n", c.Yellow("raw:"), formatHex(msg.RawHex))
		}
		if decoded, err := DecodeMessage(msg.RawHex); err == nil {
			_, _ = fmt.Fprintf(r.output, "%s\n", c.Yellow("decoded:"))
			_, _ = fmt.Fprint(r.output, decoded.ColoredString(c))
		}
	}
	_, _ = fmt.Fprintln(r.output)

	// Received message
	_, _ = fmt.Fprintln(r.output, c.LineSeparator())
	_, _ = fmt.Fprintf(r.output, "%s %d:\n", c.Cyan("RECEIVED MESSAGE"), msgIdx)
	_, _ = fmt.Fprintln(r.output, c.LineSeparator())

	// For multi-connection tests, offset by messages from previous connections
	rcvOffset := rec.ReceivedMessageOffset()
	rcvIdx := rcvOffset + rec.LastReceivedIdx
	// Fallback: if calculated index is out of bounds, use last available message
	if rcvIdx >= len(rec.ReceivedRaw) && len(rec.ReceivedRaw) > 0 {
		rcvIdx = len(rec.ReceivedRaw) - 1
	}
	if rcvIdx < len(rec.ReceivedRaw) {
		rawHex := rec.ReceivedRaw[rcvIdx]
		_, _ = fmt.Fprintf(r.output, "%s     %s\n", c.Yellow("raw:"), formatHex(rawHex))
		if decoded, err := DecodeMessage(rawHex); err == nil {
			_, _ = fmt.Fprintf(r.output, "%s\n", c.Yellow("decoded:"))
			_, _ = fmt.Fprint(r.output, decoded.ColoredString(c))
		}
	}
	_, _ = fmt.Fprintln(r.output)

	// Diff
	_, _ = fmt.Fprintln(r.output, c.LineSeparator())
	_, _ = fmt.Fprintln(r.output, c.Yellow("DIFF:"))
	_, _ = fmt.Fprintln(r.output, c.LineSeparator())

	if msg := rec.GetMessage(expectedIdx); msg != nil && rcvIdx < len(rec.ReceivedRaw) {
		received := rec.ReceivedRaw[rcvIdx]
		diff := ColoredDiff(msg.RawHex, received, c)
		_, _ = fmt.Fprint(r.output, diff)
	}
	_, _ = fmt.Fprintln(r.output)
}

func (r *Report) printGenericReport(rec *Record) {
	c := r.colors

	// Show error if any
	if rec.Error != nil {
		_, _ = fmt.Fprintln(r.output, c.LineSeparator())
		_, _ = fmt.Fprintln(r.output, c.Yellow("ERROR:"))
		_, _ = fmt.Fprintln(r.output, c.LineSeparator())
		_, _ = fmt.Fprintf(r.output, "%s\n", c.Red(rec.Error.Error()))
		_, _ = fmt.Fprintln(r.output)
	}

	// Peer output
	if rec.PeerOutput != "" {
		_, _ = fmt.Fprintln(r.output, c.LineSeparator())
		_, _ = fmt.Fprintln(r.output, c.Yellow("PEER OUTPUT:"))
		_, _ = fmt.Fprintln(r.output, c.LineSeparator())
		_, _ = fmt.Fprintln(r.output, truncateOutput(rec.PeerOutput, 30))
		_, _ = fmt.Fprintln(r.output)
	}

	// Client output
	if rec.ClientOutput != "" {
		_, _ = fmt.Fprintln(r.output, c.LineSeparator())
		_, _ = fmt.Fprintln(r.output, c.Yellow("CLIENT OUTPUT:"))
		_, _ = fmt.Fprintln(r.output, c.LineSeparator())
		_, _ = fmt.Fprintln(r.output, truncateOutput(rec.ClientOutput, 30))
		_, _ = fmt.Fprintln(r.output)
	}
}

func (r *Report) printDebugCommands(rec *Record) {
	c := r.colors

	_, _ = fmt.Fprintln(r.output, c.LineSeparator())
	_, _ = fmt.Fprintln(r.output, c.Yellow("DEBUG:"))
	_, _ = fmt.Fprintln(r.output, c.LineSeparator())

	// Decode commands
	if len(rec.Messages) > 0 && rec.Messages[0].RawHex != "" {
		_, _ = fmt.Fprintf(r.output, "%s\n", c.Gray("# Decode expected:"))
		_, _ = fmt.Fprintf(r.output, "zebgp decode update %s\n\n", rec.Messages[0].RawHex[:min(64, len(rec.Messages[0].RawHex))]+"...")
	}

	if len(rec.ReceivedRaw) > 0 {
		_, _ = fmt.Fprintf(r.output, "%s\n", c.Gray("# Decode received:"))
		_, _ = fmt.Fprintf(r.output, "zebgp decode update %s\n\n", rec.ReceivedRaw[0][:min(64, len(rec.ReceivedRaw[0]))]+"...")
	}

	// Manual test commands
	_, _ = fmt.Fprintf(r.output, "%s\n", c.Gray("# Run test manually:"))
	_, _ = fmt.Fprintf(r.output, "go run ./test/cmd/functional encoding --server %s\n", rec.Nick)
	_, _ = fmt.Fprintf(r.output, "go run ./test/cmd/functional encoding --client %s\n", rec.Nick)
	_, _ = fmt.Fprintln(r.output)
}

// PrintAllFailures prints failure reports for all failed tests.
func (r *Report) PrintAllFailures(tests *Tests) {
	for _, rec := range tests.FailedRecords() {
		r.PrintFailure(rec)
	}
}

// Helper functions

func formatHex(h string) string {
	// Add colons every 2 characters for readability, truncate if too long
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

func indentLines(s string, indent string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = indent + line
		}
	}
	return strings.Join(lines, "\n")
}
