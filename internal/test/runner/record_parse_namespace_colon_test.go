// Design: docs/architecture/testing/ci-format.md -- literal reject operands.
// Related: record_parse.go -- directive parsing through parseAndAdd.
// Related: runner_validate.go -- the output assertion and its step trace.

package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRejectStdoutNamespaceColonSurvivesParsing drives the unchanged namespace
// rejection used by help-ai-json-methods-answer from its .ci line to a verdict.
// The declared ze-system-dns method is permitted; an invented ze-system method
// must still be rejected, so preserving the colon cannot erase the assertion.
func TestRejectStdoutNamespaceColonSurvivesParsing(t *testing.T) {
	const needle = `"wire-method": "ze-system:`
	for _, tc := range []struct {
		name   string
		output string
		passed bool
	}{
		{"declared namespace", `{"wire-method": "ze-system-dns:show-system-dns"}`, true},
		{"invented namespace", `{"wire-method": "ze-system:show-system-dns"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec, err := parseCILines(t,
				"cmd=foreground:seq=1:exec=ze help ai --json",
				"reject=stdout:contains="+needle)
			require.NoError(t, err)
			require.Len(t, rec.RunCommands, 1)
			assert.Equal(t, []string{needle}, rec.RunCommands[0].RejectStdout)
			setClientOutput(rec, tc.output, "")
			rec.RunCommands[0].stdoutFrom = 0
			rec.RunCommands[0].stdoutTo = len(tc.output)

			assert.Equal(t, tc.passed, checkOutputAssertions(rec))
			require.Len(t, rec.StepTrace, 1, "the parsed rejection must reach the verdict")
			assert.Equal(t, "stdout-not-contains", rec.StepTrace[0].Assert)
			assert.Equal(t, tc.passed, rec.StepTrace[0].Passed)
		})
	}
}
