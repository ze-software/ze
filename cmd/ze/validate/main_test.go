package validate

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// validConfig is a minimal valid BGP configuration for testing.
const validConfig = `bgp {
	peer 127.0.0.1 {
		local-address 127.0.0.1;
		local-as 65533;
		peer-as 65533;
	}
}`

// TestRunValidConfig verifies valid config returns exit code 0.
//
// VALIDATES: Valid config produces success exit code.
// PREVENTS: Regression in config validation acceptance.
func TestRunValidConfig(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "ze-validate-test-*.conf")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(tmpFile.Name()) }) //nolint:errcheck,gosec // test cleanup

	if _, err := tmpFile.WriteString(validConfig); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	code := Run([]string{"-q", tmpFile.Name()})
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

// TestRunInvalidConfig verifies invalid config returns exit code 1.
//
// VALIDATES: Invalid config produces error exit code.
// PREVENTS: Silent acceptance of broken configs.
func TestRunInvalidConfig(t *testing.T) {
	content := `not valid config syntax`
	tmpFile, err := os.CreateTemp("", "ze-validate-test-*.conf")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(tmpFile.Name()) }) //nolint:errcheck,gosec // test cleanup

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	code := Run([]string{"-q", tmpFile.Name()})
	if code != 1 {
		t.Errorf("expected exit code 1 for invalid config, got %d", code)
	}
}

// TestRunMissingFile verifies missing file returns exit code 2.
//
// VALIDATES: Missing file produces file error exit code.
// PREVENTS: Confusing error for missing vs invalid.
func TestRunMissingFile(t *testing.T) {
	code := Run([]string{"-q", "/nonexistent/path/config.conf"})
	if code != 2 {
		t.Errorf("expected exit code 2 for missing file, got %d", code)
	}
}

// TestRunNoArgs verifies missing args returns exit code 1.
//
// VALIDATES: Missing arguments shows usage.
// PREVENTS: Panic on empty args.
func TestRunNoArgs(t *testing.T) {
	code := Run([]string{})
	if code != 1 {
		t.Errorf("expected exit code 1 for no args, got %d", code)
	}
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
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r

	// Write content in goroutine
	go func() {
		io.Copy(w, bytes.NewReader([]byte(validConfig))) //nolint:errcheck,gosec // test helper
		w.Close()                                        //nolint:errcheck,gosec // test helper
	}()

	code := Run([]string{"-q", "-"})
	if code != 0 {
		t.Errorf("expected exit code 0 for valid stdin, got %d", code)
	}
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
		if got != tt.want {
			t.Errorf("extractLine(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// TestUint32ToIP verifies IP address formatting.
//
// VALIDATES: uint32 correctly converted to dotted quad.
// PREVENTS: Wrong byte order in router-id display.
func TestUint32ToIP(t *testing.T) {
	tests := []struct {
		input uint32
		want  string
	}{
		{0x0A000001, "10.0.0.1"},
		{0xC0A80001, "192.168.0.1"},
		{0x7F000001, "127.0.0.1"},
		{0x00000000, "0.0.0.0"},
		{0xFFFFFFFF, "255.255.255.255"},
	}

	for _, tt := range tests {
		got := uint32ToIP(tt.input)
		if got != tt.want {
			t.Errorf("uint32ToIP(0x%08X) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestValidationResultValid verifies validation result for valid config.
//
// VALIDATES: Valid config produces Valid=true result.
// PREVENTS: False negatives in validation.
func TestValidationResultValid(t *testing.T) {
	result := validateConfig(validConfig, "test.conf")

	if !result.Valid {
		t.Errorf("expected Valid=true, got false")
		for _, e := range result.Errors {
			t.Logf("error: %s", e.Message)
		}
	}

	if result.Path != "test.conf" {
		t.Errorf("expected Path=%q, got %q", "test.conf", result.Path)
	}
}

// TestValidationResultInvalid verifies validation result for invalid config.
//
// VALIDATES: Invalid config produces Valid=false with errors.
// PREVENTS: Silent failures in validation.
func TestValidationResultInvalid(t *testing.T) {
	content := `invalid syntax here`
	result := validateConfig(content, "bad.conf")

	if result.Valid {
		t.Errorf("expected Valid=false for invalid config")
	}

	if len(result.Errors) == 0 {
		t.Errorf("expected at least one error")
	}
}

// TestSemanticValidationWarnings verifies semantic checks produce warnings.
//
// VALIDATES: Missing router-id and local-as produce warnings.
// PREVENTS: Silent config issues.
func TestSemanticValidationWarnings(t *testing.T) {
	result := validateConfig(validConfig, "test.conf")

	if !result.Valid {
		t.Fatalf("expected valid config")
	}

	// Should have warning about missing router-id
	hasRouterIDWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w.Message, "router-id") {
			hasRouterIDWarning = true
			break
		}
	}
	if !hasRouterIDWarning {
		t.Errorf("expected warning about missing router-id")
	}
}
