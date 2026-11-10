// Design: docs/architecture/doctor-and-health-checks.md -- the fix line every doctor surface draws

package doctor

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestRenderDoctorPrintsFixOnItsOwnLine draws a readiness answer holding one
// finding with a fix and one without, in text and in JSON.
//
// VALIDATES: AC-3 text shape, the fix sits on a line of its own right after
// its finding's message, behind a "fix: " label, and a finding with no fix
// draws no fix line; JSON carries the command alone under "fix".
// PREVENTS: the fixing command lost in the message line, or drawn under the
// wrong finding, or JSON that colors the command.
func TestRenderDoctorPrintsFixOnItsOwnLine(t *testing.T) {
	diags := []diagnostic.Diagnostic{
		{Code: "doctor-fixture-down", Severity: diagnostic.SeverityError, Message: "fixture is down", Fix: "ze start"},
		{Code: "doctor-fixture-slow", Severity: diagnostic.SeverityWarning, Message: "fixture is slow"},
	}
	result := diagnostic.NewDoctorResult(readyOf(diags), diags)

	var code int
	text := captureStdout(t, func() { code = renderDoctor(doctorPath, readinessAnswer{DoctorResult: result}) })
	assert.Equal(t, 1, code, "a not-ready answer exits 1 on the local drawing")
	lines := strings.Split(text, "\n")
	require.GreaterOrEqual(t, len(lines), 3, "output: %q", text)
	assert.Equal(t, "ERROR [doctor-fixture-down] fixture is down", lines[0])
	assert.Equal(t, "      fix: ze start", lines[1])
	assert.Equal(t, "WARN  [doctor-fixture-slow] fixture is slow", lines[2])
	assert.Equal(t, 1, strings.Count(text, "fix: "), "a finding with no fix drew a fix line: %q", text)

	out := captureStdout(t, func() { renderDoctor(doctorPath, readinessAnswer{DoctorResult: result, asJSON: true}) })
	var decoded struct {
		Diagnostics []map[string]any `json:"diagnostics"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &decoded), "output: %q", out)
	require.Len(t, decoded.Diagnostics, 2)
	assert.Equal(t, "ze start", decoded.Diagnostics[0]["fix"])
	assert.NotContains(t, decoded.Diagnostics[1], "fix")
	assert.NotContains(t, out, "\x1b[", "JSON carries no color")
}
