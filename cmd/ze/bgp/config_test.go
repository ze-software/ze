package bgp

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
		name               string
		config             string
		wantDeprecated     []string
		wantNeedsMigration bool
	}{
		{
			name: "current config shows no migration needed",
			config: `
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
`,
			wantNeedsMigration: false,
		},
		{
			name: "neighbor detected",
			config: `
neighbor 192.0.2.1 {
	peer-as 65001;
}
`,
			wantDeprecated:     []string{"neighbor"},
			wantNeedsMigration: true,
		},
		{
			name: "peer glob at root detected",
			config: `
peer * {
	hold-time 90;
}
peer 192.0.2.1 {
	peer-as 65001;
}
`,
			wantDeprecated:     []string{"peer *"},
			wantNeedsMigration: true,
		},
		{
			name: "template.neighbor detected",
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
			wantDeprecated:     []string{"template.neighbor"},
			wantNeedsMigration: true,
		},
		{
			name: "static block detected",
			config: `
peer 192.0.2.1 {
	peer-as 65001;
	static {
		route 10.0.0.0/8 next-hop 192.0.2.254;
	}
}
`,
			wantDeprecated:     []string{"static"},
			wantNeedsMigration: true,
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

			// Verify migration status
			if result.needsMigration != tt.wantNeedsMigration {
				t.Errorf("needsMigration = %v, want %v", result.needsMigration, tt.wantNeedsMigration)
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

			// If config is current (no migration needed), deprecated list must be empty
			if !tt.wantNeedsMigration && len(result.deprecated) > 0 {
				t.Errorf("current config should have no deprecated patterns, got: %v", result.deprecated)
			}
		})
	}
}

// TestCmdConfigMigrate tests the config migrate command.
//
// VALIDATES: config migrate converts old ExaBGP to current format.
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
			wantOutput: []string{"bgp {", "peer 192.0.2.1", "peer-as 65001"},
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
			wantOutput: []string{"template {", "peer *", "hold-time 90", "bgp {", "peer 192.0.2.1"},
		},
		{
			name: "template.neighbor to inherit-name",
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
			wantOutput: []string{"template {", "inherit-name rr", "peer-as 65000", "bgp {"},
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
			wantOutput: []string{"bgp {", "announce {", "ipv4 {", "unicast 10.0.0.0/8"},
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

	if !strings.Contains(string(data), "bgp {") {
		t.Errorf("output file missing 'bgp {':\n%s", data)
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

	// Verify original file is now current syntax
	data, err := os.ReadFile(configPath) //nolint:gosec // test file path from TempDir
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "bgp {") {
		t.Errorf("config file not migrated (missing bgp block):\n%s", data)
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

// TestCmdConfigCheckUnsupported tests unsupported feature warnings.
//
// VALIDATES: config check shows warnings for unsupported ExaBGP features.
//
// PREVENTS: Silent failure when importing configs that use features ZeBGP doesn't support.
func TestCmdConfigCheckUnsupported(t *testing.T) {
	tests := []struct {
		name         string
		config       string
		wantWarnings []string // Substrings that must appear in warnings
	}{
		{
			name: "multi-session capability unsupported",
			config: `
peer 192.0.2.1 {
	peer-as 65001;
	capability {
		multi-session true;
	}
}
`,
			wantWarnings: []string{"multi-session"},
		},
		{
			name: "operational capability unsupported",
			config: `
peer 192.0.2.1 {
	peer-as 65001;
	capability {
		operational true;
	}
}
`,
			wantWarnings: []string{"operational"},
		},
		{
			name: "operational block unsupported",
			config: `
peer 192.0.2.1 {
	peer-as 65001;
	operational {
		asm ipv4/unicast "test";
	}
}
`,
			wantWarnings: []string{"operational"},
		},
		{
			name: "multiple unsupported features",
			config: `
peer 192.0.2.1 {
	peer-as 65001;
	capability {
		multi-session true;
		operational true;
	}
}
`,
			wantWarnings: []string{"multi-session", "operational"},
		},
		{
			name: "unsupported in template.group",
			config: `
template {
	group rr {
		capability {
			multi-session true;
		}
	}
}
peer 192.0.2.1 {
	inherit rr;
}
`,
			wantWarnings: []string{"multi-session"},
		},
		{
			name: "no warnings for supported features",
			config: `
peer 192.0.2.1 {
	peer-as 65001;
	capability {
		route-refresh true;
		graceful-restart 120;
	}
}
`,
			wantWarnings: nil,
		},
		{
			name: "unsupported in old neighbor block",
			config: `
neighbor 192.0.2.1 {
	peer-as 65001;
	capability {
		operational true;
	}
}
`,
			wantWarnings: []string{"operational"},
		},
		{
			name: "unsupported in template.match",
			config: `
template {
	match * {
		capability {
			multi-session true;
		}
	}
}
peer 192.0.2.1 {
	peer-as 65001;
}
`,
			wantWarnings: []string{"multi-session"},
		},
		{
			name: "unsupported in template.neighbor (old)",
			config: `
template {
	neighbor rr {
		capability {
			operational true;
		}
	}
}
peer 192.0.2.1 {
	inherit rr;
}
`,
			wantWarnings: []string{"operational"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write config to temp file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "test.conf")
			if err := os.WriteFile(configPath, []byte(tt.config), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			// Run check command
			result := runConfigCheck(configPath)
			if result.err != nil {
				t.Fatalf("runConfigCheck: %v", result.err)
			}

			// Verify warnings
			for _, want := range tt.wantWarnings {
				found := false
				for _, w := range result.unsupported {
					if strings.Contains(w, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("unsupported = %v, want contains %q", result.unsupported, want)
				}
			}

			// If no warnings expected, verify list is empty
			if tt.wantWarnings == nil && len(result.unsupported) > 0 {
				t.Errorf("expected no unsupported warnings, got: %v", result.unsupported)
			}
		})
	}
}

// TestCmdConfigMigrateWarnings tests unsupported feature warnings from migrate.
//
// VALIDATES: config migrate returns warnings for unsupported features.
//
// PREVENTS: Silent migration of configs with unsupported features.
func TestCmdConfigMigrateWarnings(t *testing.T) {
	tests := []struct {
		name         string
		config       string
		wantWarnings []string
	}{
		{
			name: "old config with unsupported features",
			config: `
neighbor 192.0.2.1 {
	peer-as 65001;
	capability {
		multi-session true;
	}
}
`,
			wantWarnings: []string{"multi-session"},
		},
		{
			name: "current config with unsupported features",
			config: `
peer 192.0.2.1 {
	peer-as 65001;
	operational {
		asm ipv4/unicast "test";
	}
}
`,
			wantWarnings: []string{"operational"},
		},
		{
			name: "no warnings for clean config",
			config: `
peer 192.0.2.1 {
	peer-as 65001;
}
`,
			wantWarnings: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "test.conf")
			if err := os.WriteFile(configPath, []byte(tt.config), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			_, _, warnings, err := configMigrateWithWarnings(configPath, "")
			if err != nil {
				t.Fatalf("configMigrateWithWarnings: %v", err)
			}

			for _, want := range tt.wantWarnings {
				found := false
				for _, w := range warnings {
					if strings.Contains(w, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("warnings = %v, want contains %q", warnings, want)
				}
			}

			if tt.wantWarnings == nil && len(warnings) > 0 {
				t.Errorf("expected no warnings, got: %v", warnings)
			}
		})
	}
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

// TestCmdConfigCheckEnv tests the --env flag for environment validation.
//
// VALIDATES: config check --env validates environment variables.
//
// PREVENTS: Invalid environment variables causing runtime failures.
func TestCmdConfigCheckEnv(t *testing.T) {
	t.Run("valid environment", func(t *testing.T) {
		// No env vars set - defaults should be valid
		exitCode := checkEnvironment(false)
		if exitCode != exitOK {
			t.Errorf("expected exitOK for valid environment, got %d", exitCode)
		}
	})

	t.Run("valid environment json", func(t *testing.T) {
		exitCode := checkEnvironment(true)
		if exitCode != exitOK {
			t.Errorf("expected exitOK for valid environment, got %d", exitCode)
		}
	})

	t.Run("invalid port in environment", func(t *testing.T) {
		t.Setenv("ze.bgp.tcp.port", "invalid")
		exitCode := checkEnvironment(false)
		if exitCode != exitError {
			t.Errorf("expected exitError for invalid port, got %d", exitCode)
		}
	})

	t.Run("invalid boolean in environment", func(t *testing.T) {
		t.Setenv("ze.bgp.bgp.passive", "maybe")
		exitCode := checkEnvironment(false)
		if exitCode != exitError {
			t.Errorf("expected exitError for invalid boolean, got %d", exitCode)
		}
	})

	t.Run("invalid log level in environment", func(t *testing.T) {
		t.Setenv("ze.bgp.log.level", "BOGUS")
		exitCode := checkEnvironment(false)
		if exitCode != exitError {
			t.Errorf("expected exitError for invalid log level, got %d", exitCode)
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
