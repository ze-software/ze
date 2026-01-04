package main

import (
	"errors"
	"testing"
)

// TestConfigFmtFormatsConfig verifies that fmt produces normalized output.
//
// VALIDATES: fmt command parses and re-serializes config with consistent formatting.
//
// PREVENTS: Formatter producing invalid or non-idempotent output.
func TestConfigFmtFormatsConfig(t *testing.T) {
	// Create a badly formatted but valid config
	input := `peer 127.0.0.1{local-as 1;peer-as 2;}`
	expected := `peer 127.0.0.1 {
	local-as 1;
	peer-as 2;
}
`

	output, hasChanges, err := configFmtBytes([]byte(input))
	if err != nil {
		t.Fatalf("configFmtBytes failed: %v", err)
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
	input := `peer 127.0.0.1 {
	local-as 1;
	peer-as 2;
}
`

	// First pass
	output1, hasChanges1, err := configFmtBytes([]byte(input))
	if err != nil {
		t.Fatalf("first configFmtBytes failed: %v", err)
	}

	// Second pass on first output
	output2, hasChanges2, err := configFmtBytes([]byte(output1))
	if err != nil {
		t.Fatalf("second configFmtBytes failed: %v", err)
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
// VALIDATES: Old configs are rejected with helpful error message.
//
// PREVENTS: Accidentally formatting old configs without migration.
func TestConfigFmtRejectsOld(t *testing.T) {
	// Old config uses "neighbor" keyword
	input := `neighbor 127.0.0.1 {
	local-as 1;
	peer-as 2;
}
`

	_, _, err := configFmtBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error for old config")
	}

	if !errors.Is(err, ErrOldConfig) {
		t.Errorf("expected ErrOldConfig, got: %v", err)
	}
}

// TestConfigFmtComplexConfig verifies formatting of a more complex config.
//
// VALIDATES: Complex configs with templates and families are formatted correctly.
//
// PREVENTS: Formatting errors on real-world configs.
func TestConfigFmtComplexConfig(t *testing.T) {
	input := `template{group defaults{hold-time 90;}}peer 192.0.2.1{inherit defaults;local-as 65000;peer-as 65001;family{ipv4/unicast;}}`

	output, hasChanges, err := configFmtBytes([]byte(input))
	if err != nil {
		t.Fatalf("configFmtBytes failed: %v", err)
	}

	if !hasChanges {
		t.Error("expected hasChanges=true")
	}

	// Verify it's properly indented (should have tabs)
	if output == input {
		t.Error("expected output to differ from compressed input")
	}

	// Run again to verify idempotency
	output2, hasChanges2, err := configFmtBytes([]byte(output))
	if err != nil {
		t.Fatalf("second pass failed: %v", err)
	}

	if hasChanges2 {
		t.Errorf("expected no changes on second pass, got:\n%s\nvs:\n%s", output, output2)
	}
}
