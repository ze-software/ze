package validate

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validConfig is a minimal valid BGP configuration for testing.
const validConfig = `bgp {
	peer peer1 {
		remote {
			ip 127.0.0.1;
			as 65533;
		}
		local {
			ip 127.0.0.1;
			as 65533;
		}
	}
}`

// TestRunValidConfig verifies valid config returns exit code 0.
//
// VALIDATES: Valid config produces success exit code.
// PREVENTS: Regression in config validation acceptance.
func TestRunValidConfig(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "ze-validate-test-*.conf")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(tmpFile.Name()) }) //nolint:errcheck,gosec // test cleanup

	_, err = tmpFile.WriteString(validConfig)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	code := Run([]string{"-q", tmpFile.Name()})
	assert.Equal(t, 0, code, "valid config should return exit code 0")
}

// TestRunInvalidConfig verifies invalid config returns exit code 1.
//
// VALIDATES: Invalid config produces error exit code.
// PREVENTS: Silent acceptance of broken configs.
func TestRunInvalidConfig(t *testing.T) {
	content := `not valid config syntax`
	tmpFile, err := os.CreateTemp("", "ze-validate-test-*.conf")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(tmpFile.Name()) }) //nolint:errcheck,gosec // test cleanup

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	code := Run([]string{"-q", tmpFile.Name()})
	assert.Equal(t, 1, code, "invalid config should return exit code 1")
}

// TestRunMissingFile verifies missing file returns exit code 2.
//
// VALIDATES: Missing file produces file error exit code.
// PREVENTS: Confusing error for missing vs invalid.
func TestRunMissingFile(t *testing.T) {
	code := Run([]string{"-q", "/nonexistent/path/config.conf"})
	assert.Equal(t, 2, code, "missing file should return exit code 2")
}

// TestRunNoArgs verifies missing args returns exit code 1.
//
// VALIDATES: Missing arguments shows usage.
// PREVENTS: Panic on empty args.
func TestRunNoArgs(t *testing.T) {
	code := Run([]string{})
	assert.Equal(t, 1, code, "no args should return exit code 1")
}

// TestRunStdin verifies reading config from stdin works.
//
// VALIDATES: "-" argument reads from stdin.
// PREVENTS: Regression in stdin handling.
func TestRunStdin(t *testing.T) {
	// Save original stdin
	oldStdin := os.Stdin
	t.Cleanup(func() { os.Stdin = oldStdin })

	// Create pipe for stdin
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdin = r

	// Write content in goroutine
	go func() {
		io.Copy(w, bytes.NewReader([]byte(validConfig))) //nolint:errcheck,gosec // test helper
		w.Close()                                        //nolint:errcheck,gosec // test helper
	}()

	code := Run([]string{"-q", "-"})
	assert.Equal(t, 0, code, "valid stdin should return exit code 0")
}

// TestExtractLine verifies line number extraction from error messages.
//
// VALIDATES: Line numbers extracted correctly from parser errors.
// PREVENTS: Missing line info in error output.
func TestExtractLine(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"line 5: unexpected token", 5},
		{"line 123: syntax error", 123},
		{"error at line 42: missing semicolon", 42},
		{"no line number here", 0},
		{"", 0},
	}

	for _, tt := range tests {
		got := extractLine(tt.input)
		assert.Equal(t, tt.want, got, "extractLine(%q)", tt.input)
	}
}

// TestValidationResultValid verifies validation result for valid config.
//
// VALIDATES: Valid config produces Valid=true result.
// PREVENTS: False negatives in validation.
func TestValidationResultValid(t *testing.T) {
	result := validateConfig(validConfig, "test.conf")

	if !result.Valid {
		for _, e := range result.Errors {
			t.Logf("error: %s", e.Message)
		}
	}
	require.True(t, result.Valid, "expected Valid=true")
	assert.Equal(t, "test.conf", result.Path)
}

// TestValidationResultInvalid verifies validation result for invalid config.
//
// VALIDATES: Invalid config produces Valid=false with errors.
// PREVENTS: Silent failures in validation.
func TestValidationResultInvalid(t *testing.T) {
	content := `invalid syntax here`
	result := validateConfig(content, "bad.conf")

	assert.False(t, result.Valid, "expected Valid=false for invalid config")
	assert.NotEmpty(t, result.Errors, "expected at least one error")
}

// TestSemanticValidationWarnings verifies semantic checks produce warnings.
//
// VALIDATES: Missing router-id and local-as produce warnings.
// PREVENTS: Silent config issues.
func TestSemanticValidationWarnings(t *testing.T) {
	result := validateConfig(validConfig, "test.conf")
	require.True(t, result.Valid, "expected valid config")

	// Should have warning about missing router-id
	hasRouterIDWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w.Message, "router-id") {
			hasRouterIDWarning = true
			break
		}
	}
	assert.True(t, hasRouterIDWarning, "expected warning about missing router-id")
}
