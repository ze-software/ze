package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestConfigFixPlanJSON verifies the full cmdFix path: reads a config file,
// runs validation, and emits a fix-plan JSON envelope with diagnostics.
//
// VALIDATES: AC-9 fix-plan emits plan-only JSON with diagnostics and repair candidates.
// PREVENTS: cmdFix silently failing or producing malformed output.
func TestConfigFixPlanJSON(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "ze-fix-test-*.conf")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(tmpFile.Name()) }) //nolint:errcheck,gosec // test cleanup

	_, err = tmpFile.WriteString("invalid config syntax")
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := cmdFix([]string{"--plan", "--json", tmpFile.Name()})

	w.Close() //nolint:errcheck // test pipe
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck // test

	assert.Equal(t, 0, code, "fix --plan --json should return 0")

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(buf.Bytes(), &raw))
	assert.Contains(t, raw, envelopeKey())
	assert.Contains(t, raw, "diagnostics")

	var result diagnostic.FixPlanResult
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
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

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := cmdFix([]string{"--plan", "--json", tmpFile.Name()})

	w.Close() //nolint:errcheck // test pipe
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck // test

	assert.Equal(t, 0, code)

	var result diagnostic.FixPlanResult
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	var hasRepair bool
	for _, d := range result.Diagnostics {
		if d.Repair != nil && d.Repair.ID != "" {
			hasRepair = true
			break
		}
	}
	assert.True(t, hasRepair, "expected at least one diagnostic with a repair ID in fix-plan output")
}
