package runner

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		Messages: []messageExpect{
			{Index: 1, RawHex: longHex},
		},
		ReceivedRaw: []string{longHex},
	}

	var buf bytes.Buffer
	report := newReport(NewColorsWithOverride(false))
	report.SetOutput(&buf)
	report.printDebugCommands(rec)

	output := buf.String()

	// The full hex should appear in the decode commands, not truncated
	assert.Contains(t, output, longHex, "expected full hex in decode command, got truncated")
	assert.NotContains(t, output, "...", "debug commands should not contain ellipsis truncation")
}

func TestReportDebugCommandsUseSuiteSpecificCommands(t *testing.T) {
	rec := &Record{Nick: "A", Name: "ui-failure"}

	var buf bytes.Buffer
	report := newReport(NewColorsWithOverride(false))
	report.SetOutput(&buf)
	report.SetLabel("ui")
	report.printDebugCommands(rec)

	output := buf.String()
	assert.Contains(t, output, "ze-test ui A")
	assert.NotContains(t, output, "ze-test bgp ui")
	assert.NotContains(t, output, "--server")
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
	report := newReport(NewColorsWithOverride(false))
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
		Messages:     []messageExpect{{Index: 1, RawHex: "FFFF"}},
	}

	var buf bytes.Buffer
	report := newReport(NewColorsWithOverride(false))
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
	report := newReport(NewColorsWithOverride(false))
	report.SetOutput(&buf)
	report.printGenericReport(rec)

	output := buf.String()

	// Empty client output should trigger specific hint
	assert.Contains(t, output, "LIKELY CAUSE:", "must have LIKELY CAUSE section")
}

// TestReportGatesVerifyGroups verifies that verify-only failure-group
// metadata is separate from the always-shown failure details.
//
// VALIDATES: PrintFailureGroups emits VERIFY FAILURE GROUP blocks;
//
//	PrintAllFailures emits TEST FAILURE blocks.
//
// PREVENTS: Machine-oriented verify metadata leaking into normal CLI failures.
func TestReportGatesVerifyGroups(t *testing.T) {
	tests := NewEncodingTests(t.TempDir())
	rec := tests.Add("ui-failure")
	rec.Active = true
	rec.State = StateFail
	rec.Error = errors.New("broken")
	rec.CIFile = "test/ui/ui-failure.ci"

	report := newReport(NewColorsWithOverride(false))
	report.SetLabel("ui")

	var buf bytes.Buffer
	report.SetOutput(&buf)

	report.printAllFailures(tests.Tests)
	normal := buf.String()
	if strings.Contains(normal, "VERIFY FAILURE GROUP:") {
		t.Fatalf("unexpected verify failure group in PrintAllFailures output:\n%s", normal)
	}
	if !strings.Contains(normal, "TEST FAILURE:") {
		t.Fatalf("missing failure report in PrintAllFailures output:\n%s", normal)
	}

	buf.Reset()
	report.printFailureGroups(tests.Tests)
	verify := buf.String()
	if !strings.Contains(verify, "VERIFY FAILURE GROUP:") {
		t.Fatalf("missing verify failure group in PrintFailureGroups output:\n%s", verify)
	}
}

// TestRunReportNamesTheArgvItRan runs a real child, so the argv it asserts on is
// the argv the runner built rather than the one a test constructed.
//
// VALIDATES: AC-8 of spec-fixit-ci-runner-cannot-test-stdin. The report states
// the argv the runner executed and whether standard input was piped, on a
// failing run (printFailure) and on a passing one under -v (printStepTraces).
// PREVENTS: the rewrite class this spec exists for staying invisible. The
// runner replaces a `-` with a file for a daemon launch and appends one for a
// ze-peer line, and until this step existed a .ci whose stimulus was replaced
// read exactly like one whose stimulus was honored.
func TestRunReportNamesTheArgvItRan(t *testing.T) {
	baseDir := t.TempDir()
	et := NewEncodingTests(baseDir)
	r, err := NewRunner(et, baseDir)
	require.NoError(t, err)
	defer r.Cleanup()

	success := 0
	rec := et.Add("argv-probe")
	rec.Active = true
	rec.Conf = map[string]any{}
	rec.Extra = map[string]string{"timeout": "30s"}
	rec.ExpectExitCode = &success
	rec.StdinBlocks = map[string][]byte{"payload": []byte("piped-marker\n")}
	rec.RunCommands = []RunCommand{{Mode: modeForeground, Seq: 1, Exec: "/bin/cat", Stdin: "payload"}}

	require.True(t, r.runTest(t.Context(), rec, &RunOptions{}), "the probe must run: %v", rec.Error)
	require.Contains(t, rec.ClientOutput, "piped-marker",
		"the block must reach the child's standard input, or this test is asserting about a claim rather than a run")

	var executed []string
	for _, step := range rec.StepTrace {
		if step.Kind == stepKindExec {
			executed = append(executed, step.Assert)
		}
	}
	require.Len(t, executed, 1, "one command ran, so one exec step is recorded")
	assert.Contains(t, executed[0], "/bin/cat", "the step names the argv the runner executed")
	assert.Contains(t, executed[0], "[stdin=payload piped]", "the step says where the named block went")

	// The failing reader's route.
	var failure bytes.Buffer
	failureReport := newReport(NewColors())
	failureReport.SetOutput(&failure)
	failureReport.printFailure(rec)
	assert.Contains(t, failure.String(), "/bin/cat  [stdin=payload piped]",
		"a failure report states what ran, which is the first question its reader asks")

	// The passing reader's route, which is -v.
	var verbose bytes.Buffer
	verboseReport := newReport(NewColors())
	verboseReport.SetOutput(&verbose)
	verboseReport.printStepTraces(et.Tests)
	assert.Contains(t, verbose.String(), "/bin/cat  [stdin=payload piped]",
		"-v states what ran for a test that passed, which is the only place a replaced stimulus shows")
}
