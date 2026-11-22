// Design: docs/architecture/doctor-and-health-checks.md -- the three severities a finding carries

package doctor

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestRenderDoctorInfoIsNotAProblem draws a readiness answer whose only finding
// is informational, in text and in JSON, then one beside a severity no check
// declares.
//
// VALIDATES: an info finding keeps the box ready, draws as INFO, is counted
// neither as an error nor as a warning, exits 0, and reaches JSON as "info" with
// "ready": true; a severity outside the declared three draws and counts as a
// warning, never as INFO.
// PREVENTS: an informational row (the daemon's capability inventory) that makes
// a box read not ready or counts as a warning, and an unrecognized severity
// shown as less than a problem.
func TestRenderDoctorInfoIsNotAProblem(t *testing.T) {
	diags := []diagnostic.Diagnostic{
		{Code: "doctor-fixture-fact", Severity: diagnostic.SeverityInfo, Message: "fixture fact", Fix: "ze show fixture"},
	}
	require.True(t, readyOf(diags), "an info finding failed readiness")
	result := diagnostic.NewDoctorResult(readyOf(diags), diags)

	var code int
	text := captureStdout(t, func() { code = renderDoctor(doctorPath, readinessAnswer{DoctorResult: result}) })
	assert.Equal(t, 0, code, "a ready answer exits 0: %q", text)
	assert.True(t, strings.HasPrefix(text, "INFO  [doctor-fixture-fact] fixture fact\n"), "output: %q", text)
	assert.Contains(t, text, "ready (0 errors, 0 warnings)")

	out := captureStdout(t, func() { renderDoctor(doctorPath, readinessAnswer{DoctorResult: result, asJSON: true}) })
	var decoded struct {
		Ready       bool             `json:"ready"`
		Diagnostics []map[string]any `json:"diagnostics"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &decoded), "output: %q", out)
	assert.True(t, decoded.Ready)
	require.Len(t, decoded.Diagnostics, 1)
	assert.Equal(t, "info", decoded.Diagnostics[0]["severity"])

	undeclared := []diagnostic.Diagnostic{
		diags[0],
		{Code: "doctor-fixture-odd", Severity: "notice", Message: "fixture odd", Fix: "ze show fixture"},
	}
	result = diagnostic.NewDoctorResult(readyOf(undeclared), undeclared)
	text = captureStdout(t, func() { renderDoctor(doctorPath, readinessAnswer{DoctorResult: result}) })
	assert.Contains(t, text, "WARN  [doctor-fixture-odd] fixture odd")
	assert.Contains(t, text, "ready (0 errors, 1 warnings)")
}
