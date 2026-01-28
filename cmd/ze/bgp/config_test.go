package bgp

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCmdConfigCheck tests the config check command.
//
// VALIDATES: config check works for valid configs and rejects old syntax.
//
// PREVENTS: User confusion about config format version.
func TestCmdConfigCheck(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		wantError bool
	}{
		{
			name: "current config is valid",
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
			wantError: false,
		},
		{
			name: "neighbor rejected",
			config: `
neighbor 192.0.2.1 {
	peer-as 65001;
}
`,
			wantError: true,
		},
		{
			name: "announce block rejected",
			config: `
bgp {
	peer 192.0.2.1 {
		peer-as 65001;
		announce {
			ipv4 {
				unicast 10.0.0.0/8 next-hop 192.0.2.254;
			}
		}
	}
}
`,
			wantError: true,
		},
		{
			name: "static block rejected",
			config: `
bgp {
	peer 192.0.2.1 {
		peer-as 65001;
		static {
			route 10.0.0.0/8 next-hop 192.0.2.254;
		}
	}
}
`,
			wantError: true,
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

			// Verify error status
			if tt.wantError && result.err == nil {
				t.Error("expected error for old syntax, got nil")
			}
			if !tt.wantError && result.err != nil {
				t.Errorf("unexpected error: %v", result.err)
			}
		})
	}
}

// TestCmdConfigMigrate tests the config migrate command.
//
// VALIDATES: config migrate rejects old ExaBGP syntax with helpful error.
//
// PREVENTS: Confusion about unsupported migration.
func TestCmdConfigMigrate(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{
			name: "current config works",
			input: `
bgp {
	peer 192.0.2.1 {
		peer-as 65001;
	}
}
`,
			wantError: false,
		},
		{
			name: "old neighbor syntax rejected",
			input: `
neighbor 192.0.2.1 {
	peer-as 65001;
}
`,
			wantError: true,
		},
		{
			name: "announce block rejected",
			input: `
bgp {
	peer 192.0.2.1 {
		peer-as 65001;
		announce {
			ipv4 {
				unicast 10.0.0.0/8 next-hop 192.0.2.254;
			}
		}
	}
}
`,
			wantError: true,
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
			_, err := runConfigMigrate(configPath, "")
			if tt.wantError && err == nil {
				t.Error("expected error for old syntax, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// runConfigCheck is a helper that runs config check and returns the result.
func runConfigCheck(path string) checkResult {
	return configCheck(path)
}

// runConfigMigrate is a helper that runs config migrate.
func runConfigMigrate(inputPath, outputPath string) (string, error) {
	return configMigrate(inputPath, outputPath)
}
