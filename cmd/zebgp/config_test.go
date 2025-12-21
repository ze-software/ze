package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCmdConfigCheck tests the config check command.
//
// VALIDATES: config check shows version and deprecated patterns.
//
// PREVENTS: User confusion about config format version.
func TestCmdConfigCheck(t *testing.T) {
	tests := []struct {
		name           string
		config         string
		wantVersion    string
		wantDeprecated []string
		wantCurrent    bool
	}{
		{
			name: "v3 config shows current",
			config: `
peer 192.0.2.1 {
	peer-as 65001;
}
template {
	group rr {
		peer-as 65000;
	}
	match * {
		hold-time 90;
	}
}
`,
			wantVersion: "v3",
			wantCurrent: true,
		},
		{
			name: "v2 neighbor detected",
			config: `
neighbor 192.0.2.1 {
	peer-as 65001;
}
`,
			wantVersion:    "v2",
			wantDeprecated: []string{"neighbor"},
			wantCurrent:    false,
		},
		{
			name: "v2 peer glob at root detected",
			config: `
peer * {
	hold-time 90;
}
peer 192.0.2.1 {
	peer-as 65001;
}
`,
			wantVersion:    "v2",
			wantDeprecated: []string{"peer *"},
			wantCurrent:    false,
		},
		{
			name: "v2 template.neighbor detected",
			config: `
template {
	neighbor rr {
		peer-as 65000;
	}
}
peer 192.0.2.1 {
	peer-as 65001;
}
`,
			wantVersion:    "v2",
			wantDeprecated: []string{"template.neighbor"},
			wantCurrent:    false,
		},
		{
			name: "v2 static block detected",
			config: `
peer 192.0.2.1 {
	peer-as 65001;
	static {
		route 10.0.0.0/8 next-hop 192.0.2.254;
	}
}
`,
			wantVersion:    "v2",
			wantDeprecated: []string{"static"},
			wantCurrent:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write config to temp file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "test.conf")
			if err := os.WriteFile(configPath, []byte(tt.config), 0o600); err != nil { //nolint:gosec // test file
				t.Fatalf("write config: %v", err)
			}

			// Run check command
			result := runConfigCheck(configPath)

			// Verify version
			if !strings.Contains(result.version, tt.wantVersion) {
				t.Errorf("version = %q, want contains %q", result.version, tt.wantVersion)
			}

			// Verify current status
			if result.isCurrent != tt.wantCurrent {
				t.Errorf("isCurrent = %v, want %v", result.isCurrent, tt.wantCurrent)
			}

			// Verify deprecated patterns
			for _, dep := range tt.wantDeprecated {
				found := false
				for _, d := range result.deprecated {
					if strings.Contains(d, dep) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("deprecated = %v, want contains %q", result.deprecated, dep)
				}
			}

			// If config is current, deprecated list must be empty
			if tt.wantCurrent && len(result.deprecated) > 0 {
				t.Errorf("current config should have no deprecated patterns, got: %v", result.deprecated)
			}
		})
	}
}

// TestCmdConfigMigrate tests the config migrate command.
//
// VALIDATES: config migrate converts v2 to v3 format.
//
// PREVENTS: Data loss during migration, incorrect output.
func TestCmdConfigMigrate(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantOutput []string // Substrings that must appear in output
	}{
		{
			name: "neighbor to peer",
			input: `
neighbor 192.0.2.1 {
	peer-as 65001;
}
`,
			wantOutput: []string{"peer 192.0.2.1", "peer-as 65001"},
		},
		{
			name: "peer glob to template.match",
			input: `
peer * {
	hold-time 90;
}
peer 192.0.2.1 {
	peer-as 65001;
}
`,
			wantOutput: []string{"template {", "match *", "hold-time 90", "peer 192.0.2.1"},
		},
		{
			name: "template.neighbor to template.group",
			input: `
template {
	neighbor rr {
		peer-as 65000;
	}
}
peer 192.0.2.1 {
	inherit rr;
}
`,
			wantOutput: []string{"template {", "group rr", "peer-as 65000"},
		},
		{
			name: "static to announce",
			input: `
peer 192.0.2.1 {
	peer-as 65001;
	static {
		route 10.0.0.0/8 next-hop 192.0.2.254;
	}
}
`,
			// InlineListNode serializes as: unicast 10.0.0.0/8 next-hop ...;
			wantOutput: []string{"announce {", "ipv4 {", "unicast 10.0.0.0/8"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write config to temp file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "test.conf")
			if err := os.WriteFile(configPath, []byte(tt.input), 0o600); err != nil { //nolint:gosec // test file
				t.Fatalf("write config: %v", err)
			}

			// Run migrate command
			output, err := runConfigMigrate(configPath, "")
			if err != nil {
				t.Fatalf("runConfigMigrate: %v", err)
			}

			// Verify output contains expected substrings
			for _, want := range tt.wantOutput {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q:\n%s", want, output)
				}
			}
		})
	}
}

// TestCmdConfigMigrateFile tests -o flag for output file.
//
// VALIDATES: config migrate -o writes to specified file.
//
// PREVENTS: Output to wrong location.
func TestCmdConfigMigrateFile(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
	peer-as 65001;
}
`
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "input.conf")
	outputPath := filepath.Join(tmpDir, "output.conf")

	if err := os.WriteFile(inputPath, []byte(input), 0o600); err != nil { //nolint:gosec // test file
		t.Fatalf("write input: %v", err)
	}

	// Run migrate with -o
	_, err := runConfigMigrate(inputPath, outputPath)
	if err != nil {
		t.Fatalf("runConfigMigrate: %v", err)
	}

	// Verify output file exists and contains migrated config
	data, err := os.ReadFile(outputPath) //nolint:gosec // test file path from TempDir
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	if !strings.Contains(string(data), "peer 192.0.2.1") {
		t.Errorf("output file missing 'peer 192.0.2.1':\n%s", data)
	}
}

// TestCmdConfigMigrateInPlace tests --in-place flag with backup.
//
// VALIDATES: config migrate --in-place modifies file in place with backup.
//
// PREVENTS: Data loss without backup.
func TestCmdConfigMigrateInPlace(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
	peer-as 65001;
}
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.conf")

	if err := os.WriteFile(configPath, []byte(input), 0o600); err != nil { //nolint:gosec // test file
		t.Fatalf("write config: %v", err)
	}

	// Run migrate with --in-place
	backupPath, err := runConfigMigrateInPlace(configPath)
	if err != nil {
		t.Fatalf("runConfigMigrateInPlace: %v", err)
	}

	// Verify backup exists
	if _, err := os.Stat(backupPath); err != nil {
		t.Errorf("backup file not found: %v", err)
	}

	// Verify backup contains original
	backupData, err := os.ReadFile(backupPath) //nolint:gosec // test file path from TempDir
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if !strings.Contains(string(backupData), "neighbor 192.0.2.1") {
		t.Errorf("backup missing original content")
	}

	// Verify original file is now v3
	data, err := os.ReadFile(configPath) //nolint:gosec // test file path from TempDir
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "peer 192.0.2.1") {
		t.Errorf("config file not migrated:\n%s", data)
	}
}

// runConfigCheck runs the config check command and returns results.
func runConfigCheck(path string) checkResult {
	return configCheck(path)
}

// runConfigMigrate runs the config migrate command and returns migrated output.
// If outputPath is empty, returns the migrated content as string.
func runConfigMigrate(inputPath, outputPath string) (string, error) {
	return configMigrate(inputPath, outputPath)
}

// runConfigMigrateInPlace runs config migrate --in-place and returns backup path.
func runConfigMigrateInPlace(path string) (string, error) {
	return configMigrateInPlace(path)
}

// TestCmdConfigCheckErrors tests error handling in config check.
//
// VALIDATES: config check returns errors for invalid input.
//
// PREVENTS: Silent failures on bad input.
func TestCmdConfigCheckErrors(t *testing.T) {
	t.Run("file not found", func(t *testing.T) {
		result := runConfigCheck("/nonexistent/path/config.conf")
		if result.err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("parse error", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "bad.conf")
		if err := os.WriteFile(configPath, []byte("invalid { syntax"), 0o600); err != nil { //nolint:gosec // test
			t.Fatalf("write config: %v", err)
		}

		result := runConfigCheck(configPath)
		if result.err == nil {
			t.Error("expected error for invalid syntax")
		}
	})
}

// TestCmdConfigMigrateErrors tests error handling in config migrate.
//
// VALIDATES: config migrate returns errors for invalid input.
//
// PREVENTS: Silent failures on bad input.
func TestCmdConfigMigrateErrors(t *testing.T) {
	t.Run("file not found", func(t *testing.T) {
		_, err := runConfigMigrate("/nonexistent/path/config.conf", "")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("parse error", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "bad.conf")
		if err := os.WriteFile(configPath, []byte("invalid { syntax"), 0o600); err != nil { //nolint:gosec // test
			t.Fatalf("write config: %v", err)
		}

		_, err := runConfigMigrate(configPath, "")
		if err == nil {
			t.Error("expected error for invalid syntax")
		}
	})

	t.Run("output dir not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		inputPath := filepath.Join(tmpDir, "input.conf")
		if err := os.WriteFile(inputPath, []byte("peer 192.0.2.1 { }"), 0o600); err != nil { //nolint:gosec // test
			t.Fatalf("write config: %v", err)
		}

		_, err := runConfigMigrate(inputPath, "/nonexistent/dir/output.conf")
		if err == nil {
			t.Error("expected error for nonexistent output dir")
		}
	})
}
