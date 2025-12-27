package functional

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEncodingTestsDiscover verifies test discovery from encode directory.
//
// VALIDATES: All .ci files are discovered and registered.
// PREVENTS: Missing tests due to discovery bugs.
func TestEncodingTestsDiscover(t *testing.T) {
	ResetNickCounter()
	// Find project root
	baseDir := findBaseDir(t)
	encodeDir := filepath.Join(baseDir, "test", "data", "encode")

	et := NewEncodingTests(baseDir)
	if err := et.Discover(encodeDir); err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	// Should have discovered tests
	if et.Count() == 0 {
		t.Error("Discover() found no tests")
	}
}

// TestEncodingTestsParseCIFile verifies .ci file parsing.
//
// VALIDATES: Options and expected messages are extracted correctly.
// PREVENTS: Misconfigured tests due to parse errors.
func TestEncodingTestsParseCIFile(t *testing.T) {
	ResetNickCounter()
	// Create temp test file
	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "test.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	// Write minimal CI file
	ciContent := `option:file:test.conf
option:asn:65000
1:raw:FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:0017:02:00000000
`
	if err := os.WriteFile(ciFile, []byte(ciContent), 0o600); err != nil { //nolint:gosec // Test file
		t.Fatalf("WriteFile(ci) error = %v", err)
	}

	// Write minimal conf file
	confContent := `process test { run ./test.sh; }`
	if err := os.WriteFile(confFile, []byte(confContent), 0o600); err != nil { //nolint:gosec // Test file
		t.Fatalf("WriteFile(conf) error = %v", err)
	}

	et := NewEncodingTests(tmpDir)
	if err := et.Discover(tmpDir); err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if et.Count() != 1 {
		t.Fatalf("Count() = %d, want 1", et.Count())
	}

	r := et.GetByNick("0")
	if r == nil {
		t.Fatal("GetByNick(0) = nil")
	}

	// Verify options parsed
	if r.Extra["asn"] != "65000" {
		t.Errorf("Extra[asn] = %q, want %q", r.Extra["asn"], "65000")
	}

	// Verify expects parsed
	if len(r.Expects) != 1 {
		t.Errorf("Expects = %d entries, want 1", len(r.Expects))
	}
}

// TestEncodingTestsConfigPath verifies config file path resolution.
//
// VALIDATES: Config path is correctly resolved from CI file directory.
// PREVENTS: "Config not found" errors.
func TestEncodingTestsConfigPath(t *testing.T) {
	ResetNickCounter()
	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "test.ci")
	confFile := filepath.Join(tmpDir, "myconfig.conf")

	ciContent := `option:file:myconfig.conf
`
	if err := os.WriteFile(ciFile, []byte(ciContent), 0o600); err != nil { //nolint:gosec // Test file
		t.Fatalf("WriteFile(ci) error = %v", err)
	}
	if err := os.WriteFile(confFile, []byte("{}"), 0o600); err != nil { //nolint:gosec // Test file
		t.Fatalf("WriteFile(conf) error = %v", err)
	}

	et := NewEncodingTests(tmpDir)
	if err := et.Discover(tmpDir); err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	r := et.GetByNick("0")
	if r == nil {
		t.Fatal("GetByNick(0) = nil")
	}

	configPath, ok := r.Conf["config"].(string)
	if !ok {
		t.Fatal("Conf[config] is not a string")
	}

	if configPath != confFile {
		t.Errorf("Conf[config] = %q, want %q", configPath, confFile)
	}
}

// findBaseDir locates the project root for tests.
func findBaseDir(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	// Walk up to find go.mod
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("Could not find project root from %s", dir)
		}
		dir = parent
	}
}
