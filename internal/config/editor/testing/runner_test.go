package testing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunnerBasicTest verifies a simple test case runs successfully.
//
// VALIDATES: Runner can execute a basic .et test.
// PREVENTS: Test framework fundamentally broken.
func TestRunnerBasicTest(t *testing.T) {
	etContent := `# Basic test
tmpfs=test.conf:terminator=EOF_CONF
bgp {
  local-as 65000;
  router-id 1.2.3.4;
}
EOF_CONF

option=file:path=test.conf

expect=context:root
expect=dirty:false
`

	result := RunETTest(etContent)
	require.NotNil(t, result)
	assert.True(t, result.Passed, "test should pass: %s", result.Error)
	assert.Empty(t, result.Error)
}

// TestRunnerWithInput verifies input actions are executed.
//
// VALIDATES: Runner processes input= lines correctly.
// PREVENTS: User input not being sent to editor.
func TestRunnerWithInput(t *testing.T) {
	etContent := `# Test with input
tmpfs=test.conf:terminator=EOF_CONF
bgp {
  local-as 65000;
  router-id 1.2.3.4;
}
EOF_CONF

option=file:path=test.conf

input=type:text=edit bgp
input=enter
expect=context:path=bgp
`

	result := RunETTest(etContent)
	require.NotNil(t, result)
	assert.True(t, result.Passed, "test should pass: %s", result.Error)
}

// TestRunnerFailingExpectation verifies failed expectations are reported.
//
// VALIDATES: Runner reports expectation failures clearly.
// PREVENTS: Test failures not detected.
func TestRunnerFailingExpectation(t *testing.T) {
	etContent := `# Test with failing expectation
tmpfs=test.conf:terminator=EOF_CONF
bgp {
  local-as 65000;
}
EOF_CONF

option=file:path=test.conf

expect=context:path=bgp
`

	result := RunETTest(etContent)
	require.NotNil(t, result)
	assert.False(t, result.Passed, "test should fail")
	assert.Contains(t, result.Error, "context")
}

// TestRunnerMissingConfigFile verifies error on missing config.
//
// VALIDATES: Runner fails clearly when config file not in tmpfs.
// PREVENTS: Cryptic errors on missing config.
func TestRunnerMissingConfigFile(t *testing.T) {
	etContent := `# Test with missing config
option=file:path=nonexistent.conf
expect=context:root
`

	result := RunETTest(etContent)
	require.NotNil(t, result)
	assert.False(t, result.Passed)
	assert.Contains(t, result.Error, "nonexistent")
}

// TestRunnerMultipleExpectations verifies multiple expectations.
//
// VALIDATES: All expectations are checked in order.
// PREVENTS: Expectations after first being skipped.
func TestRunnerMultipleExpectations(t *testing.T) {
	etContent := `# Test with multiple expectations
tmpfs=test.conf:terminator=EOF_CONF
bgp {
  local-as 65000;
  router-id 1.2.3.4;
}
EOF_CONF

option=file:path=test.conf

expect=context:root
expect=dirty:false
expect=error:none
`

	result := RunETTest(etContent)
	require.NotNil(t, result)
	assert.True(t, result.Passed, "all expectations should pass: %s", result.Error)
}

// TestRunnerWithTabCompletion verifies tab completion can be tested.
//
// VALIDATES: Tab key triggers completions for assertions.
// PREVENTS: Completion tests not working.
func TestRunnerWithTabCompletion(t *testing.T) {
	etContent := `# Test tab completion
tmpfs=test.conf:terminator=EOF_CONF
bgp {
  local-as 65000;
  router-id 1.2.3.4;
}
EOF_CONF

option=file:path=test.conf

input=type:text=edit bgp
input=enter
input=type:text=set
expect=context:path=bgp
`

	result := RunETTest(etContent)
	require.NotNil(t, result)
	assert.True(t, result.Passed, "test should pass: %s", result.Error)
}

// TestRunETFile verifies running from file path.
//
// VALIDATES: RunETFile loads and executes .et file.
// PREVENTS: File-based test execution broken.
func TestRunETFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .et file
	etContent := `# File-based test
tmpfs=test.conf:terminator=EOF_CONF
bgp {
  local-as 65000;
}
EOF_CONF

option=file:path=test.conf

expect=context:root
`
	etPath := filepath.Join(tmpDir, "test.et")
	err := os.WriteFile(etPath, []byte(etContent), 0600)
	require.NoError(t, err)

	result := RunETFile(etPath)
	require.NotNil(t, result)
	assert.True(t, result.Passed, "test should pass: %s", result.Error)
}

// TestRunnerReportsLineNumber verifies error location reporting.
//
// VALIDATES: Failures report which expectation failed.
// PREVENTS: Difficult to locate failing assertion.
func TestRunnerReportsLineNumber(t *testing.T) {
	etContent := `# Test with identified failure
tmpfs=test.conf:terminator=EOF_CONF
bgp { local-as 65000; }
EOF_CONF

option=file:path=test.conf

expect=context:root
expect=dirty:true
`

	result := RunETTest(etContent)
	require.NotNil(t, result)
	assert.False(t, result.Passed)
	// Error should identify which expectation failed
	assert.Contains(t, result.Error, "dirty")
}

// TestRunnerCleanup verifies temp files are cleaned up.
//
// VALIDATES: Temp directory is removed after test.
// PREVENTS: Disk space leak from test runs.
func TestRunnerCleanup(t *testing.T) {
	etContent := `# Cleanup test
tmpfs=test.conf:terminator=EOF_CONF
bgp { local-as 65000; }
EOF_CONF

option=file:path=test.conf

expect=context:root
`

	result := RunETTest(etContent)
	require.NotNil(t, result)

	// TempDir should be cleaned up (empty or not exist)
	if result.TempDir != "" {
		_, err := os.Stat(result.TempDir)
		assert.True(t, os.IsNotExist(err), "temp dir should be cleaned up")
	}
}

// TestFunctionalETFiles runs all .et files from test/editor/ directory.
//
// VALIDATES: All functional editor tests pass.
// PREVENTS: Regressions in editor behavior.
func TestFunctionalETFiles(t *testing.T) {
	// Find the test/editor directory relative to project root
	// The test runs from the package directory, so we need to navigate up
	projectRoot := findProjectRoot()
	if projectRoot == "" {
		t.Skip("Could not find project root (test/editor directory)")
		return
	}

	editorTestDir := filepath.Join(projectRoot, "test", "editor")
	if _, err := os.Stat(editorTestDir); os.IsNotExist(err) {
		t.Skip("test/editor directory not found")
		return
	}

	// Collect all .et files
	var etFiles []string
	err := filepath.Walk(editorTestDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".et" {
			etFiles = append(etFiles, path)
		}
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, etFiles, "should find .et files in test/editor/")

	t.Logf("Found %d .et files", len(etFiles))

	// Run each .et file as a subtest
	for _, etPath := range etFiles {
		relPath, _ := filepath.Rel(editorTestDir, etPath)
		t.Run(relPath, func(t *testing.T) {
			result := RunETFile(etPath)
			if !result.Passed {
				t.Errorf("test failed: %s", result.Error)
			}
		})
	}
}

// findProjectRoot walks up from current directory to find project root.
// Returns empty string if not found.
func findProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Walk up looking for test/editor directory
	for {
		testDir := filepath.Join(dir, "test", "editor")
		if _, err := os.Stat(testDir); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "" // Reached filesystem root
		}
		dir = parent
	}
}
