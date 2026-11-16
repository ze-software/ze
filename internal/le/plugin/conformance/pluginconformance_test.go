// VALIDATES: `le plugin conformance check run <command> [name <name>]` is
// registered at its path, declares itself heavy, and answers the checker's
// report with the checker's exit code.
// PREVENTS: a development entry to the checker that drifts from the report
// `ze show plugin conformance run` answers, or that a subagent could start.

package pluginconformance

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/conformance"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// conformingRun builds a run string whose program writes the declaration line
// the protocol's own writer produces, and exits 0. The line holds no single
// quote, so the shell passes it to printf verbatim.
func conformingRun(t *testing.T) string {
	t.Helper()
	var line bytes.Buffer
	require.NoError(t, rpc.WriteDeclaration(&line, &rpc.DeclareRegistrationInput{
		ProtocolVersion: rpc.ProtocolVersion,
		Commands:        []rpc.CommandDecl{{Name: "show query answer", ShortHelp: "Show the answer."}},
	}))
	require.NotContains(t, line.String(), "'")
	return "printf '%s' '" + line.String() + "'"
}

// answeredReport runs the check action over args and answers its report.
func answeredReport(t *testing.T, args ...string) (conformance.Report, int) {
	t.Helper()
	payload, code := Answer(append([]string{"check"}, args...))
	report, isReport := payload.(conformance.Report)
	require.True(t, isReport, "the action answered %T, and its answer is the checker's report", payload)
	return report, code
}

// TestTheAreaIsRegisteredAtItsPath asks whether le holds the area under the
// name a developer types, with the action table its init registered.
func TestTheAreaIsRegisteredAtItsPath(t *testing.T) {
	require.True(t, leroot.Owns(area), "le registers no command at %q", leroot.CommandPath(area))

	list, declared := leroot.ActionsOf(area)
	require.True(t, declared, "the area %q registered no action table", area)
	assert.Equal(t, area, list.Area)
	require.Len(t, list.Actions, 1)
	assert.Equal(t, "check", list.Actions[0].Verb)
}

// TestTheCheckActionIsHeavyAndTakesRunAndName reads the published action row:
// the action starts a program, so it is heavy, and its grammar is the required
// run string and the optional plugin name.
func TestTheCheckActionIsHeavyAndTakesRunAndName(t *testing.T) {
	rows := Actions().Actions
	require.Len(t, rows, 1)
	check := rows[0]

	assert.True(t, check.Heavy, "the check action starts a program and waits for it, so it is heavy")
	assert.True(t, leroot.HeavyLine([]string{"plugin", "conformance", "check"}),
		"le does not read `plugin conformance check` as heavy")
	assert.False(t, check.Writes)

	require.Len(t, check.Parameters, 2)
	run, name := check.Parameters[0], check.Parameters[1]

	assert.Equal(t, keywordRun, run.Keyword)
	assert.Equal(t, leaction.KindString, run.Kind)
	assert.Equal(t, leaction.Required, run.Requirement)

	assert.Equal(t, keywordName, name.Keyword)
	assert.Equal(t, leaction.KindLabel, name.Kind)
	assert.Equal(t, leaction.Optional, name.Requirement)
}

// TestCheckAnswersZeroForAConformingProgram runs a program that writes the
// declaration line alone and exits 0.
func TestCheckAnswersZeroForAConformingProgram(t *testing.T) {
	report, code := answeredReport(t, keywordRun, conformingRun(t))

	assert.Equal(t, conformance.ExitConforms, code)
	assert.Equal(t, conformance.VerdictPass, report.Verdict)
	// The run string starts printf, so the default name is the program's.
	assert.Equal(t, "printf", report.Plugin)
	for _, row := range report.Rows {
		assert.NotEqual(t, conformance.VerdictFail, row.Verdict, "%s: %s", row.Check, row.Reason)
	}
}

// TestCheckWritesTheReportForTheNameItIsGiven asks whether the optional
// keyword replaces the default name.
func TestCheckWritesTheReportForTheNameItIsGiven(t *testing.T) {
	report, code := answeredReport(t, keywordRun, conformingRun(t), keywordName, "acme-monitor")

	assert.Equal(t, conformance.ExitConforms, code)
	assert.Equal(t, "acme-monitor", report.Plugin)
}

// TestCheckAnswersOneForAFailedRow runs a program that exits 0 and writes no
// declaration, which fails the declaration rows and nothing before them.
func TestCheckAnswersOneForAFailedRow(t *testing.T) {
	report, code := answeredReport(t, keywordRun, "exit 0", keywordName, "silent")

	assert.Equal(t, conformance.ExitViolation, code)
	assert.Equal(t, conformance.VerdictFail, report.Verdict)

	failed := map[string]bool{}
	for _, row := range report.Rows {
		if row.Verdict == conformance.VerdictFail {
			failed[row.Check] = true
			assert.NotEmpty(t, row.Fix, "%s: a failed row names the change that fixes it", row.Check)
		}
	}
	assert.True(t, failed["declaration-line"], "the rows that failed are %v", failed)
	assert.False(t, failed["program-starts"], "a program that ran was reported as not started")
}

// TestCheckAnswersTwoWhenTheRunCannotHappen gives a run string that names no
// program. There is no report then, and the code is not the code of a failed
// row.
func TestCheckAnswersTwoWhenTheRunCannotHappen(t *testing.T) {
	payload, code := Answer([]string{"check", keywordRun, "   "})

	assert.Equal(t, conformance.ExitNoRun, code)
	assert.Nil(t, payload)
}
