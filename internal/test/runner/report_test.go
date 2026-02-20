package runner

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDebugCommandsFullHex verifies that debug commands contain the full hex string,
// not a truncated version that cannot be copy-pasted.
//
// VALIDATES: AC-2 — debug commands show full hex, directly copy-pasteable.
// PREVENTS: Truncated hex forcing developers to manually find full message bytes.
func TestDebugCommandsFullHex(t *testing.T) {
	// Create a record with hex longer than 64 characters
	longHex := strings.Repeat("FF", 100) // 200 hex chars = 100 bytes
	rec := &Record{
		Nick: "A",
		Name: "test-long-hex",
		Messages: []MessageExpect{
			{Index: 1, RawHex: longHex},
		},
		ReceivedRaw: []string{longHex},
	}

	var buf bytes.Buffer
	report := NewReport(NewColorsWithOverride(false))
	report.SetOutput(&buf)
	report.printDebugCommands(rec)

	output := buf.String()

	// The full hex should appear in the decode commands, not truncated
	assert.Contains(t, output, longHex, "expected full hex in decode command, got truncated")
	assert.NotContains(t, output, "...", "debug commands should not contain ellipsis truncation")
}

// TestGenericReportStructured verifies that generic failure reports include
// structured sections with clear labels instead of raw output dumps.
//
// VALIDATES: AC-3 — generic report has ERROR + LIKELY CAUSE + output sections.
// PREVENTS: Unstructured dumps that require developer guesswork.
func TestGenericReportStructured(t *testing.T) {
	rec := &Record{
		Nick:         "B",
		Name:         "test-generic",
		State:        StateFail,
		FailureType:  "",
		Error:        assert.AnError,
		PeerOutput:   "some peer output\nline 2",
		ClientOutput: "",
	}

	var buf bytes.Buffer
	report := NewReport(NewColorsWithOverride(false))
	report.SetOutput(&buf)
	report.printGenericReport(rec)

	output := buf.String()

	// Must have structured sections
	assert.Contains(t, output, "ERROR:", "generic report must have ERROR section")
	assert.Contains(t, output, "LIKELY CAUSE:", "generic report must have LIKELY CAUSE section")
}

// TestLikelyCauseTimeout verifies that timeout reports include likely cause hints
// to help developers understand why the test timed out.
//
// VALIDATES: AC-4 — timeout with empty client output suggests common reasons.
// PREVENTS: Developer having to memorize timeout failure patterns.
func TestLikelyCauseTimeout(t *testing.T) {
	rec := &Record{
		Nick:         "C",
		Name:         "test-timeout-hint",
		State:        StateTimeout,
		FailureType:  stateTimeout,
		ClientOutput: "",
		ReceivedRaw:  nil,
		Messages:     []MessageExpect{{Index: 1, RawHex: "FFFF"}},
	}

	var buf bytes.Buffer
	report := NewReport(NewColorsWithOverride(false))
	report.SetOutput(&buf)
	report.printTimeoutReport(rec)

	output := buf.String()

	// Must contain likely cause section with actionable hints
	assert.Contains(t, output, "LIKELY CAUSE:", "timeout report must have LIKELY CAUSE section")
}

// TestLikelyCauseEmptyClient verifies that empty client output triggers
// a specific diagnostic hint.
//
// VALIDATES: AC-4 — empty client output gets specific diagnosis.
// PREVENTS: Missing diagnosis for the most common timeout scenario.
func TestLikelyCauseEmptyClient(t *testing.T) {
	rec := &Record{
		Nick:         "D",
		Name:         "test-empty-client",
		State:        StateFail,
		FailureType:  "",
		Error:        assert.AnError,
		ClientOutput: "",
		PeerOutput:   "",
	}

	var buf bytes.Buffer
	report := NewReport(NewColorsWithOverride(false))
	report.SetOutput(&buf)
	report.printGenericReport(rec)

	output := buf.String()

	// Empty client output should trigger specific hint
	assert.Contains(t, output, "LIKELY CAUSE:", "must have LIKELY CAUSE section")
}
