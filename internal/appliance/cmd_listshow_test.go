package appliance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

func TestListShowsAppliances(t *testing.T) {
	dir := t.TempDir()
	baseDir = dir
	t.Setenv("ZE_APPLIANCE_SSH_PASSWORD", "pw")
	env.ResetCache()

	for _, name := range []string{"edge-01", "edge-02"} {
		cfg := DefaultConfig(name)
		cfgPath := filepath.Join(dir, name+"-input.json")
		data, _ := json.MarshalIndent(&cfg, "", "  ")
		os.WriteFile(cfgPath, data, 0o644) //nolint:errcheck,gosec // test
		code := runInit([]string{"--config", cfgPath, name})
		if code != exitOK {
			t.Fatalf("init %s returned %d", name, code)
		}
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := runList(nil)

	w.Close() //nolint:errcheck // test
	os.Stdout = oldStdout

	if code != exitOK {
		t.Fatalf("list returned %d", code)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "edge-01") {
		t.Error("output should contain edge-01")
	}
	if !strings.Contains(output, "edge-02") {
		t.Error("output should contain edge-02")
	}
	if !strings.Contains(output, "amd64") {
		t.Error("output should contain arch column")
	}
}

func TestShowDisplaysConfigAndCertExpiry(t *testing.T) {
	dir := initTestAppliance(t, "showme", nil)
	baseDir = dir

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := runShow([]string{"showme"})

	w.Close() //nolint:errcheck // test
	os.Stdout = oldStdout

	if code != exitOK {
		t.Fatalf("show returned %d", code)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "showme") {
		t.Error("output should contain appliance name")
	}
	if !strings.Contains(output, "expires:") {
		t.Error("output should contain cert expiry")
	}
	if !strings.Contains(output, "admin") {
		t.Error("output should contain username")
	}
}

func TestShowManagedExplanation(t *testing.T) {
	dir := initTestAppliance(t, "mgd", nil)
	baseDir = dir

	cfg, _ := LoadConfig(ConfigPath(dir, "mgd"))
	cfg.Managed = true
	saveConfig(ConfigPath(dir, "mgd"), cfg) //nolint:errcheck // test

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := runShow([]string{"mgd"})

	w.Close() //nolint:errcheck // test
	os.Stdout = oldStdout

	if code != exitOK {
		t.Fatalf("show returned %d", code)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "fleet mode") {
		t.Error("managed appliance should show fleet mode explanation")
	}
}

func TestMainDispatchAppliance(t *testing.T) {
	code := Run([]string{"help"})
	if code != exitOK {
		t.Errorf("help returned %d, want 0", code)
	}

	code = Run([]string{"--help"})
	if code != exitOK {
		t.Errorf("--help returned %d, want 0", code)
	}

	code = Run(nil)
	if code != exitError {
		t.Errorf("no args should return error, got %d", code)
	}

	code = Run([]string{"nonexistent"})
	if code != exitError {
		t.Errorf("unknown command should return error, got %d", code)
	}
}
