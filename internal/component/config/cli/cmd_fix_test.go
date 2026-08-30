package cli

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// fixPlanAnswer runs the answer `ze config fix --plan` renders over a config
// file and answers it encoded, which is the record cmdFix prints.
func fixPlanAnswer(t *testing.T, configPath string) ([]byte, int) {
	t.Helper()

	payload, code := resolveFixPlan(configPath)
	if payload == nil {
		return nil, code
	}
	encoded, err := json.Marshal(payload)
	require.NoError(t, err)
	return encoded, code
}

// TestConfigFixPlanJSON verifies the answer path: reads a config file, runs
// validation, and answers a fix-plan envelope with diagnostics.
//
// VALIDATES: AC-9 fix-plan answers a plan-only record with diagnostics and
// repair candidates.
// PREVENTS: the fix plan silently failing or answering a malformed record.
func TestConfigFixPlanJSON(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "ze-fix-test-*.conf")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(tmpFile.Name()) }) //nolint:errcheck,gosec // test cleanup

	_, err = tmpFile.WriteString("invalid config syntax")
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	encoded, code := fixPlanAnswer(t, tmpFile.Name())
	assert.Equal(t, exitOK, code, "the fix plan of a broken config should still answer")

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &raw))
	assert.Contains(t, raw, envelopeKey())
	assert.Contains(t, raw, "diagnostics")

	var result diagnostic.FixPlanResult
	require.NoError(t, json.Unmarshal(encoded, &result))
	assert.NotEmpty(t, result.Diagnostics)
	assert.Equal(t, "config-parse", result.Diagnostics[0].Code)
}

// TestConfigFixPlanRepairIDsFromFix verifies repair IDs appear in the actual
// cmdFix output path (not just runValidation).
//
// VALIDATES: AC-9/AC-10 repair metadata in fix-plan output.
// PREVENTS: Repair metadata lost between runValidation and cmdFix JSON encoding.
func TestConfigFixPlanRepairIDsFromFix(t *testing.T) {
	conf := `environment {
	web {
		enabled true
		server web1 {
			ip 0.0.0.0
			port 8080
		}
		server web2 {
			ip 0.0.0.0
			port 8080
		}
	}
}`
	tmpFile, err := os.CreateTemp("", "ze-fix-repair-*.conf")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(tmpFile.Name()) }) //nolint:errcheck,gosec // test cleanup

	_, err = tmpFile.WriteString(conf)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	encoded, code := fixPlanAnswer(t, tmpFile.Name())
	assert.Equal(t, exitOK, code)

	var result diagnostic.FixPlanResult
	require.NoError(t, json.Unmarshal(encoded, &result))

	var hasRepair bool
	for _, d := range result.Diagnostics {
		if d.Repair != nil && d.Repair.ID != "" {
			hasRepair = true
			break
		}
	}
	assert.True(t, hasRepair, "expected at least one diagnostic with a repair ID in fix-plan output")
}

// TestConfigFixRequiresPlan verifies `ze config fix` refuses to run without
// --plan, and names the option it wants.
//
// VALIDATES: the plan-only guard survived the deletion of --json.
// PREVENTS: the guard being dropped with the rendering flag it sat beside.
func TestConfigFixRequiresPlan(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "ze-fix-guard-*.conf")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(tmpFile.Name()) }) //nolint:errcheck,gosec // test cleanup
	require.NoError(t, tmpFile.Close())

	assert.Equal(t, 1, cmdFix([]string{tmpFile.Name()}))
	assert.Equal(t, exitOK, cmdFix([]string{"--plan", tmpFile.Name()}))
}
