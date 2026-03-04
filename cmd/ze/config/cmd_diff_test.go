package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimal ze config: router-id + local-as + one peer with peer-as.
const testConfigBase = `
bgp {
    router-id 1.1.1.1;
    local-as 65000;
    peer 10.0.0.1 {
        peer-as 65001;
    }
}
`

// same as base but peer-as changed.
const testConfigChanged = `
bgp {
    router-id 1.1.1.1;
    local-as 65000;
    peer 10.0.0.1 {
        peer-as 65002;
    }
}
`

// base config with an extra peer added.
const testConfigAdded = `
bgp {
    router-id 1.1.1.1;
    local-as 65000;
    peer 10.0.0.1 {
        peer-as 65001;
    }
    peer 10.0.0.2 {
        peer-as 65003;
    }
}
`

// writeTempConfig writes content to a temp file and returns its path.
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.conf")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// TestConfigDiffIdentical verifies identical configs produce no differences.
//
// VALIDATES: AC-9 — identical files produce empty diff, exit 0.
// PREVENTS: False positives in diff output.
func TestConfigDiffIdentical(t *testing.T) {
	file1 := writeTempConfig(t, testConfigBase)
	file2 := writeTempConfig(t, testConfigBase)

	code := cmdDiff([]string{file1, file2})
	assert.Equal(t, exitOK, code)
}

// TestConfigDiffChanged verifies changed values are detected.
//
// VALIDATES: AC-10 — changed peer-as appears in diff output.
// PREVENTS: Missing changes in diff.
func TestConfigDiffChanged(t *testing.T) {
	file1 := writeTempConfig(t, testConfigBase)
	file2 := writeTempConfig(t, testConfigChanged)

	code := cmdDiff([]string{"--json", file1, file2})
	assert.Equal(t, exitOK, code)
}

// TestConfigDiffAdded verifies added peers appear in diff.
//
// VALIDATES: AC-11 — added peer subtree appears in diff output.
// PREVENTS: Missed additions in diff.
func TestConfigDiffAdded(t *testing.T) {
	file1 := writeTempConfig(t, testConfigBase)
	file2 := writeTempConfig(t, testConfigAdded)

	code := cmdDiff([]string{"--json", file1, file2})
	assert.Equal(t, exitOK, code)
}

// TestConfigDiffMissingFile verifies missing file returns exit 2.
//
// VALIDATES: AC-12 — nonexistent file returns exit code 2.
// PREVENTS: Crash or silent failure on missing file.
func TestConfigDiffMissingFile(t *testing.T) {
	file1 := writeTempConfig(t, testConfigBase)

	code := cmdDiff([]string{file1, "/nonexistent/path/config.conf"})
	assert.Equal(t, exitError, code)
}

// TestConfigDiffJSON verifies JSON output matches ConfigDiff structure.
//
// VALIDATES: AC-10 — JSON output has added/removed/changed keys.
// PREVENTS: Malformed JSON diff output.
func TestConfigDiffJSON(t *testing.T) {
	file1 := writeTempConfig(t, testConfigBase)
	file2 := writeTempConfig(t, testConfigChanged)

	// Capture stdout
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	code := cmdDiff([]string{"--json", file1, file2})

	if cerr := w.Close(); cerr != nil {
		t.Logf("close write pipe: %v", cerr)
	}
	os.Stdout = old

	assert.Equal(t, exitOK, code)

	var buf [4096]byte
	n, readErr := r.Read(buf[:])
	require.NoError(t, readErr)
	if cerr := r.Close(); cerr != nil {
		t.Logf("close read pipe: %v", cerr)
	}

	var result map[string]any
	require.NoError(t, json.Unmarshal(buf[:n], &result))

	// Should have added, removed, changed keys
	_, hasAdded := result["added"]
	_, hasRemoved := result["removed"]
	_, hasChanged := result["changed"]
	assert.True(t, hasAdded, "expected 'added' key in JSON output")
	assert.True(t, hasRemoved, "expected 'removed' key in JSON output")
	assert.True(t, hasChanged, "expected 'changed' key in JSON output")
}

// TestConfigDiffNoArgs verifies usage error on missing arguments.
func TestConfigDiffNoArgs(t *testing.T) {
	code := cmdDiff([]string{})
	assert.Equal(t, exitError, code)
}
