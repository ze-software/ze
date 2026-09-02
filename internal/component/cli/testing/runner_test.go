package testing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestRunnerBasicTest verifies a simple test case runs successfully.
//
// VALIDATES: Runner can execute a basic .et test.
// PREVENTS: Test framework fundamentally broken.
func TestRunnerBasicTest(t *testing.T) {
	etContent := `# Basic test
tmpfs=test.conf:terminator=EOF_CONF
bgp {
  session {
    asn {
      local 65000
    }
  }
  router-id 1.2.3.4
}
EOF_CONF

option=file:path=test.conf

expect=context:root
expect=dirty:false
`

	result := runETTest(etContent)
	require.NotNil(t, result)
	assert.True(t, result.Passed, "test should pass: %s", result.Error)
	assert.Empty(t, result.Error)
	assert.Len(t, result.Steps, 2, "should record 2 expect steps")
	for _, s := range result.Steps {
		assert.True(t, s.Passed, "step %d should pass", s.Step)
		assert.Equal(t, "expect", s.Kind)
	}
}

// TestRunnerWithInput verifies input actions are executed.
//
// VALIDATES: Runner processes input= lines correctly.
// PREVENTS: User input not being sent to cli.
func TestRunnerWithInput(t *testing.T) {
	etContent := `# Test with input
tmpfs=test.conf:terminator=EOF_CONF
bgp {
  session {
    asn {
      local 65000
    }
  }
  router-id 1.2.3.4
}
EOF_CONF

option=file:path=test.conf

input=type:text=edit bgp
input=enter
expect=context:path=bgp
`

	result := runETTest(etContent)
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
  session {
    asn {
      local 65000
    }
  }
}
EOF_CONF

option=file:path=test.conf

expect=context:path=bgp
`

	result := runETTest(etContent)
	require.NotNil(t, result)
	assert.False(t, result.Passed, "test should fail")
	assert.Contains(t, result.Error, "context")
	require.NotEmpty(t, result.Steps, "should record steps before failure")
	last := result.Steps[len(result.Steps)-1]
	assert.False(t, last.Passed, "last step should be the failure")
	assert.Equal(t, "expect", last.Kind)
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

	result := runETTest(etContent)
	require.NotNil(t, result)
	assert.False(t, result.Passed)
	assert.Contains(t, result.Error, "nonexistent")
}

// TestRunnerBlobStorage verifies option=storage:value=blob runs the editor on a
// zefs blob, as the daemon does, and that the tmpfs config is reachable there.
//
// VALIDATES: an .et test can exercise blob-only editor behavior.
// PREVENTS: a blob-only defect staying invisible to the whole .et suite, which
// ran every test on filesystem storage while the daemon runs on a blob.
func TestRunnerBlobStorage(t *testing.T) {
	etContent := `# Blob-backed editor
tmpfs=test.conf:terminator=EOF_CONF
bgp {
  session {
    asn {
      local 65000
    }
  }
  router-id 1.2.3.4
}
EOF_CONF

option=file:path=test.conf
option=storage:value=blob
option=session:user=thomas:origin=local

input=type:text=set bgp router-id 5.6.7.8
input=enter
expect=dirty:true
expect=error:none
`

	result := runETTest(etContent)
	require.NotNil(t, result)
	assert.True(t, result.Passed, "test should pass: %s", result.Error)
}

// TestRunnerBlobStorageWritesBlobNotFile commits an edit under
// option=storage:value=blob and then reads both stores, to say where the edit
// landed: the blob holds it and the config file on disk does not.
//
// The method is to run the test case in a directory this test owns, because the
// runner removes its own temp directory and takes the evidence with it.
//
// VALIDATES: AC-5 -- the editor under test reads and writes a zefs blob.
// PREVENTS: the option resolving to filesystem storage while every assertion
// stays green. A pass, a dirty flag and an .et run prove the editor did not
// crash; only the two stores read afterwards prove which one it wrote.
func TestRunnerBlobStorageWritesBlobNotFile(t *testing.T) {
	etContent := `# Blob-backed editor, committed
tmpfs=test.conf:terminator=EOF_CONF
bgp {
  session {
    asn {
      local 65000
    }
  }
  router-id 1.2.3.4
}
EOF_CONF

option=file:path=test.conf
option=storage:value=blob
option=session:user=thomas:origin=local

input=type:text=set bgp router-id 5.6.7.8
input=enter
expect=dirty:true
expect=error:none

input=type:text=commit
input=enter
expect=dirty:false
expect=error:none
`

	tc, err := parseETFile(etContent)
	require.NoError(t, err)

	tmpDir := t.TempDir()
	result := runTestCaseIn(tc, tmpDir)
	require.NotNil(t, result)
	require.True(t, result.Passed, "test should pass: %s", result.Error)

	onDisk, err := os.ReadFile(filepath.Join(tmpDir, "test.conf"))
	require.NoError(t, err)
	assert.Contains(t, string(onDisk), "1.2.3.4",
		"the config file on disk keeps its migrated content")
	assert.NotContains(t, string(onDisk), "5.6.7.8",
		"a blob-backed editor writes the blob, so the commit must not reach the file")

	blobStore, err := storage.NewBlob(filepath.Join(tmpDir, "database.zefs"), "")
	require.NoError(t, err)
	defer blobStore.Close() //nolint:errcheck // test cleanup

	inBlob, err := blobStore.ReadFile("test.conf")
	require.NoError(t, err)
	assert.Contains(t, string(inBlob), "5.6.7.8", "the blob holds the committed edit")
}

// TestRunnerBlobStorageRefusesFileExpectation verifies the two options that
// cannot mean anything together are refused.
//
// VALIDATES: expect=file: with blob storage stops the test.
// PREVENTS: a file expectation asserting against the pre-migration copy in the
// temp directory and passing for content the editor never wrote there.
func TestRunnerBlobStorageRefusesFileExpectation(t *testing.T) {
	etContent := `# Blob storage with a file expectation
tmpfs=test.conf:terminator=EOF_CONF
bgp {
  router-id 1.2.3.4
}
EOF_CONF

option=file:path=test.conf
option=storage:value=blob

expect=file:path=test.conf:contains=router-id
`

	result := runETTest(etContent)
	require.NotNil(t, result)
	assert.False(t, result.Passed)
	assert.Contains(t, result.Error, "expect=file:")
}

// TestRunnerUnknownStorageBackendFails verifies the storage option fails closed.
//
// VALIDATES: an unrecognized backend name stops the test.
// PREVENTS: a typo silently running on the filesystem while the test claims to
// prove blob behavior.
func TestRunnerUnknownStorageBackendFails(t *testing.T) {
	etContent := `# Unknown storage backend
tmpfs=test.conf:terminator=EOF_CONF
bgp {
  router-id 1.2.3.4
}
EOF_CONF

option=file:path=test.conf
option=storage:value=zefs

expect=context:root
`

	result := runETTest(etContent)
	require.NotNil(t, result)
	assert.False(t, result.Passed)
	assert.Contains(t, result.Error, "unknown option=storage:value=zefs")
}

// TestRunnerMultipleExpectations verifies multiple expectations.
//
// VALIDATES: All expectations are checked in order.
// PREVENTS: Expectations after first being skipped.
func TestRunnerMultipleExpectations(t *testing.T) {
	etContent := `# Test with multiple expectations
tmpfs=test.conf:terminator=EOF_CONF
bgp {
  session {
    asn {
      local 65000
    }
  }
  router-id 1.2.3.4
}
EOF_CONF

option=file:path=test.conf

expect=context:root
expect=dirty:false
expect=error:none
`

	result := runETTest(etContent)
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
  session {
    asn {
      local 65000
    }
  }
  router-id 1.2.3.4
}
EOF_CONF

option=file:path=test.conf

input=type:text=edit bgp
input=enter
input=type:text=set
expect=context:path=bgp
`

	result := runETTest(etContent)
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
  session {
    asn {
      local 65000
    }
  }
}
EOF_CONF

option=file:path=test.conf

expect=context:root
`
	etPath := filepath.Join(tmpDir, "test.et")
	err := os.WriteFile(etPath, []byte(etContent), 0o600)
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
bgp { session { asn { local 65000; } } }
EOF_CONF

option=file:path=test.conf

expect=context:root
expect=dirty:true
`

	result := runETTest(etContent)
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
bgp { session { asn { local 65000; } } }
EOF_CONF

option=file:path=test.conf

expect=context:root
`

	result := runETTest(etContent)
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
	t.Parallel()
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

	// Run each .et file as a parallel subtest — each creates its own
	// temp dir and headless model, so there are no shared resources.
	for _, etPath := range etFiles {
		relPath, _ := filepath.Rel(editorTestDir, etPath)
		t.Run(relPath, func(t *testing.T) {
			t.Parallel()
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

// TestETFileAssertsHintAndExplanation runs the new expectation kinds from .et
// text, through the parser and the runner an editor test file takes.
//
// VALIDATES: an .et file can name hint and explanation, and the run FAILS when
// the screen does not match.
// PREVENTS: an expectation kind that parses and then passes whatever the CLI
// rendered, which would report coverage the file never had.
func TestETFileAssertsHintAndExplanation(t *testing.T) {
	// The idle info line is what the second message line carries with nothing
	// completed, and no explanation is revealed until Tab asks for one.
	const passing = `option=mode:value=command
expect=hint:contains=Ze CLI
expect=explanation:empty
`
	// The same file asking for text the screen does not carry.
	const failing = `option=mode:value=command
expect=explanation:contains=no command declares this sentence
`

	tc, err := parseETFile(passing)
	require.NoError(t, err)
	result := runTestCase(tc)
	assert.Empty(t, result.Error)
	assert.True(t, result.Passed, "expected the matching file to pass")

	tc, err = parseETFile(failing)
	require.NoError(t, err)
	result = runTestCase(tc)
	assert.False(t, result.Passed, "expected the mismatching file to fail")
	assert.Contains(t, result.Error, "no command declares this sentence")
}
