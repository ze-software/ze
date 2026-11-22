// Design: docs/architecture/doctor-and-health-checks.md -- the fix field on a doctor diagnostic

package diagnostic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDiagnosticFixKey checks the fix field's JSON contract: the command alone
// under "fix", omitted when empty, and separate from fix-safety and repair.
//
// VALIDATES: Diagnostic.Fix marshals as "fix", omitempty.
// PREVENTS: the fix command missing from ze doctor --json and show doctor, or
// riding a key an API client reads as a repair grade.
func TestDiagnosticFixKey(t *testing.T) {
	d := Diagnostic{
		Code:     "doctor-runtime-not-checked",
		Severity: SeverityWarning,
		Message:  "runtime checks did not run",
		Fix:      "ze start",
	}
	data, err := json.Marshal(d)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Equal(t, "ze start", raw["fix"])
	assert.NotContains(t, raw, "fix-safety")
	assert.NotContains(t, raw, "repair")

	data, err = json.Marshal(Diagnostic{Code: "doctor-x", Severity: SeverityError, Message: "m"})
	require.NoError(t, err)
	raw = map[string]any{}
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.NotContains(t, raw, "fix")
}

// TestBuiltinCodesExplainRuntimeCodes checks that the cross-cutting runtime
// code this package owns is explained, so ze show diagnostic answers for it.
//
// VALIDATES: doctor-recent-events resolves through Lookup.
// PREVENTS: a doctor row whose code ze show diagnostic does not know.
func TestBuiltinCodesExplainRuntimeCodes(t *testing.T) {
	t.Cleanup(SnapshotForTest())
	ResetForTest()
	RegisterBuiltinCodes()

	for _, code := range []string{CodeDoctorRecentEvents} {
		assert.NotNil(t, Lookup(code), "code %s has no explanation", code)
	}
}
