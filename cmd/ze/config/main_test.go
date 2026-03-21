package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConfigFmtFormatsConfig verifies that fmt produces normalized output.
//
// VALIDATES: fmt command parses and re-serializes config with consistent formatting.
//
// PREVENTS: Formatter producing invalid or non-idempotent output.
func TestConfigFmtFormatsConfig(t *testing.T) {
	// Create a badly formatted but valid config
	input := `bgp{peer peer1{remote{ip 127.0.0.1;as 2;}local{as 1;}}}`
	expected := `bgp {
	peer peer1 {
		local {
			as 1
		}
		remote {
			as 2
			ip 127.0.0.1
		}
	}
}
`

	output, hasChanges, err := ConfigFmtBytes([]byte(input))
	if err != nil {
		t.Fatalf("ConfigFmtBytes failed: %v", err)
	}

	if !hasChanges {
		t.Error("expected hasChanges=true for badly formatted input")
	}

	if output != expected {
		t.Errorf("unexpected output:\ngot:\n%s\nwant:\n%s", output, expected)
	}
}

// TestConfigFmtIdempotent verifies that formatting is idempotent.
//
// VALIDATES: Running fmt twice produces identical output.
//
// PREVENTS: Non-idempotent formatting that would fail --check after -w.
func TestConfigFmtIdempotent(t *testing.T) {
	input := `bgp {
	peer peer1 {
		local {
			as 1
		}
		remote {
			as 2
			ip 127.0.0.1
		}
	}
}
`

	// First pass
	output1, hasChanges1, err := ConfigFmtBytes([]byte(input))
	if err != nil {
		t.Fatalf("first ConfigFmtBytes failed: %v", err)
	}

	// Second pass on first output
	output2, hasChanges2, err := ConfigFmtBytes([]byte(output1))
	if err != nil {
		t.Fatalf("second ConfigFmtBytes failed: %v", err)
	}

	if hasChanges1 {
		t.Errorf("expected no changes for already-formatted input on first pass, output:\n%s", output1)
	}

	if hasChanges2 {
		t.Error("expected no changes on second pass (idempotency)")
	}

	if output1 != output2 {
		t.Errorf("non-idempotent output:\nfirst:\n%s\nsecond:\n%s", output1, output2)
	}
}

// TestConfigFmtRejectsOld verifies that fmt rejects old ExaBGP configs.
//
// VALIDATES: Old configs are rejected with parse error (unknown field).
//
// PREVENTS: Accidentally formatting old configs.
func TestConfigFmtRejectsOld(t *testing.T) {
	// Old config uses "neighbor" keyword which is no longer in YANG schema
	input := `neighbor 127.0.0.1 {
	local-as 1
	peer-as 2
}
`

	_, _, err := ConfigFmtBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error for old config")
		return
	}

	// Old syntax results in parse error (unknown field "neighbor")
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// TestConfigFmtComplexConfig verifies formatting of a more complex config.
//
// VALIDATES: Complex configs with groups and families are formatted correctly.
//
// PREVENTS: Formatting errors on real-world configs.
func TestConfigFmtComplexConfig(t *testing.T) {
	input := `bgp{group defaults{hold-time 90;peer upstream{remote{ip 192.0.2.1;as 65001;}local{as 65000;}family{ipv4/unicast;}}}}`

	output, hasChanges, err := ConfigFmtBytes([]byte(input))
	if err != nil {
		t.Fatalf("ConfigFmtBytes failed: %v", err)
	}

	if !hasChanges {
		t.Error("expected hasChanges=true")
	}

	// Verify it's properly indented (should have tabs)
	if output == input {
		t.Error("expected output to differ from compressed input")
	}

	// Run again to verify idempotency
	output2, hasChanges2, err := ConfigFmtBytes([]byte(output))
	if err != nil {
		t.Fatalf("second pass failed: %v", err)
	}

	if hasChanges2 {
		t.Errorf("expected no changes on second pass, got:\n%s\nvs:\n%s", output, output2)
	}
}

// TestConfigValidateCurrentConfig tests that current configs pass validation.
//
// VALIDATES: config validate works for valid configs.
//
// PREVENTS: False positives on current syntax.
func TestConfigValidateCurrentConfig(t *testing.T) {
	input := `
bgp {
	local {
		as 65000;
	}
	group rr {
		remote {
			as 65000;
		}
		peer upstream {
			remote {
				ip 192.0.2.1;
				as 65001;
			}
			local {
				ip 192.0.2.2;
			}
		}
	}
}
`
	result := runValidation(input, "test.conf")

	if !result.Valid {
		for _, e := range result.Errors {
			t.Errorf("unexpected error: %s", e.Message)
		}
	}
}

// TestConfigValidateRejectsNeighbor tests that old neighbor syntax is rejected.
//
// VALIDATES: config validate rejects old syntax.
//
// PREVENTS: Accepting deprecated configs.
func TestConfigValidateRejectsNeighbor(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
	peer-as 65001
}
`
	result := runValidation(input, "test.conf")

	// Old syntax should cause parse error.
	if result.Valid {
		t.Error("expected invalid result for old neighbor syntax")
	}
}

// TestConfigMigrateNativeConfig tests that native configs migrate successfully.
//
// VALIDATES: config migrate handles native Ze configs.
//
// PREVENTS: Migration failures for valid configs.
func TestConfigMigrateNativeConfig(t *testing.T) {
	input := `
bgp {
	peer peer1 {
		remote {
			ip 192.0.2.1;
			as 65001;
		}
	}
}
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	if err := os.WriteFile(configPath, []byte(input), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, _, err := configMigrateWithWarnings(configPath, "", "set")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestConfigMigrateRejectsInvalid tests that invalid configs are rejected.
//
// VALIDATES: Invalid syntax is rejected.
//
// PREVENTS: Silent failures on bad configs.
func TestConfigMigrateRejectsInvalid(t *testing.T) {
	input := `
invalid { syntax }
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	if err := os.WriteFile(configPath, []byte(input), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, _, err := configMigrateWithWarnings(configPath, "", "set")
	if err == nil {
		t.Error("expected error for invalid syntax, got nil")
	}
}

// TestConfigMigrateSetFormatInput tests that set-format input is parsed correctly.
//
// VALIDATES: configMigrateWithWarnings uses SetParser for set-format input.
//
// PREVENTS: Set-format configs failing to parse because hierarchical parser is used.
func TestConfigMigrateSetFormatInput(t *testing.T) {
	input := `set bgp local as 65000
set bgp peer peer1 remote ip 192.0.2.1
set bgp peer peer1 remote as 65001
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	if err := os.WriteFile(configPath, []byte(input), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	output, _, _, err := configMigrateWithWarnings(configPath, "", "set")
	if err != nil {
		t.Fatalf("unexpected error for set-format input: %v", err)
	}

	// Output should still be set format and contain the original values.
	if !strings.Contains(output, "set bgp local as") {
		t.Errorf("expected set-format output, got:\n%s", output)
	}
	if !strings.Contains(output, "set bgp peer peer1 remote") {
		t.Errorf("expected peer in output, got:\n%s", output)
	}
}

// TestConfigMigrateHierarchicalOutput tests that --format hierarchical works.
//
// VALIDATES: configMigrateWithWarnings produces hierarchical output when requested.
//
// PREVENTS: --format hierarchical flag ignored.
func TestConfigMigrateHierarchicalOutput(t *testing.T) {
	input := `set bgp local as 65000
set bgp peer peer1 remote ip 192.0.2.1
set bgp peer peer1 remote as 65001
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	if err := os.WriteFile(configPath, []byte(input), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	output, _, _, err := configMigrateWithWarnings(configPath, "", "hierarchical")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Hierarchical output should contain braces.
	if !strings.Contains(output, "{") {
		t.Errorf("expected hierarchical output with braces, got:\n%s", output)
	}
	if !strings.Contains(output, "local") {
		t.Errorf("expected local in output, got:\n%s", output)
	}
}

// TestConfigMigrateOutputToFile tests that -o flag writes to file.
//
// VALIDATES: configMigrateWithWarnings writes output to file.
//
// PREVENTS: Output file not created or empty.
func TestConfigMigrateOutputToFile(t *testing.T) {
	input := `bgp {
	peer peer1 {
		remote {
			ip 192.0.2.1;
			as 65001;
		}
	}
}
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	if err := os.WriteFile(configPath, []byte(input), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	outputPath := filepath.Join(tmpDir, "output.conf")

	_, _, _, err := configMigrateWithWarnings(configPath, outputPath, "set")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outputPath) //nolint:gosec // Test file
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}

	if !strings.Contains(string(data), "set bgp peer peer1 remote") {
		t.Errorf("expected set-format in output file, got:\n%s", string(data))
	}
}
