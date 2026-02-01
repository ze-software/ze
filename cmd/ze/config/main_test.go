package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestConfigFmtFormatsConfig verifies that fmt produces normalized output.
//
// VALIDATES: fmt command parses and re-serializes config with consistent formatting.
//
// PREVENTS: Formatter producing invalid or non-idempotent output.
func TestConfigFmtFormatsConfig(t *testing.T) {
	// Create a badly formatted but valid config
	input := `bgp{peer 127.0.0.1{local-as 1;peer-as 2;}}`
	expected := `bgp {
	peer 127.0.0.1 {
		local-as 1;
		peer-as 2;
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
	peer 127.0.0.1 {
		local-as 1;
		peer-as 2;
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
	local-as 1;
	peer-as 2;
}
`

	_, _, err := ConfigFmtBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error for old config")
	}

	// Old syntax results in parse error (unknown field "neighbor")
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// TestConfigFmtComplexConfig verifies formatting of a more complex config.
//
// VALIDATES: Complex configs with templates and families are formatted correctly.
//
// PREVENTS: Formatting errors on real-world configs.
func TestConfigFmtComplexConfig(t *testing.T) {
	input := `template{bgp{peer *{inherit-name defaults;hold-time 90;}}}bgp{peer 192.0.2.1{inherit defaults;local-as 65000;peer-as 65001;family{ipv4/unicast;}}}`

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

// TestConfigCheckCurrentConfig tests that current configs pass check.
//
// VALIDATES: config check works for valid configs.
//
// PREVENTS: False positives on current syntax.
func TestConfigCheckCurrentConfig(t *testing.T) {
	config := `
bgp {
	peer 192.0.2.1 {
		peer-as 65001;
	}
}
template {
	bgp {
		peer * {
			inherit-name rr;
			peer-as 65000;
		}
	}
}
`
	// Write config to temp file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Run check
	result := configCheckData(configPath)

	if result.err != nil {
		t.Errorf("unexpected error: %v", result.err)
	}
}

// TestConfigCheckRejectsNeighbor tests that old neighbor syntax is rejected.
//
// VALIDATES: config check rejects old syntax.
//
// PREVENTS: Accepting deprecated configs.
func TestConfigCheckRejectsNeighbor(t *testing.T) {
	config := `
neighbor 192.0.2.1 {
	peer-as 65001;
}
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result := configCheckData(configPath)

	// Old syntax should cause parse error
	if result.err == nil {
		t.Error("expected error for old neighbor syntax, got nil")
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
	peer 192.0.2.1 {
		peer-as 65001;
	}
}
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	if err := os.WriteFile(configPath, []byte(input), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, _, err := configMigrateWithWarnings(configPath, "")
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

	_, _, _, err := configMigrateWithWarnings(configPath, "")
	if err == nil {
		t.Error("expected error for invalid syntax, got nil")
	}
}
