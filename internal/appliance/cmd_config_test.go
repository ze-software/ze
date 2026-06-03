package appliance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigMergedOutput(t *testing.T) {
	dir := t.TempDir()
	baseDir = dir

	appDir := filepath.Join(dir, "lab")
	os.MkdirAll(appDir, 0o755) //nolint:errcheck // test

	cfg := DefaultConfig("lab")
	data, _ := json.MarshalIndent(&cfg, "", "  ")
	os.WriteFile(filepath.Join(appDir, "appliance.json"), append(data, '\n'), 0o644) //nolint:errcheck,gosec // test

	overlay := "router bgp 65000\n neighbor 10.0.0.1 remote-as 65001\n"
	os.WriteFile(filepath.Join(appDir, "ze.conf"), []byte(overlay), 0o644) //nolint:errcheck,gosec // test

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := runConfig([]string{"--merged", "lab"})

	w.Close() //nolint:errcheck // test pipe
	os.Stdout = old

	if code != exitOK {
		t.Fatalf("config --merged returned %d, want 0", code)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "router bgp 65000") {
		t.Errorf("output should contain overlay content, got: %q", output)
	}
	if !strings.Contains(output, "neighbor 10.0.0.1") {
		t.Errorf("output should contain neighbor line, got: %q", output)
	}
}

func TestConfigMergedWithBase(t *testing.T) {
	dir := t.TempDir()
	baseDir = dir

	appDir := filepath.Join(dir, "lab")
	os.MkdirAll(appDir, 0o755) //nolint:errcheck // test

	baseConfig := "router bgp 65000\n local-as 65000\n"
	os.WriteFile(filepath.Join(appDir, "base.conf"), []byte(baseConfig), 0o644) //nolint:errcheck,gosec // test

	cfg := DefaultConfig("lab")
	cfg.ConfigBase = "base.conf"
	data, _ := json.MarshalIndent(&cfg, "", "  ")
	os.WriteFile(filepath.Join(appDir, "appliance.json"), append(data, '\n'), 0o644) //nolint:errcheck,gosec // test

	overlay := " neighbor 10.0.0.1 remote-as 65001\n"
	os.WriteFile(filepath.Join(appDir, "ze.conf"), []byte(overlay), 0o644) //nolint:errcheck,gosec // test

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := runConfig([]string{"--merged", "lab"})

	w.Close() //nolint:errcheck // test pipe
	os.Stdout = old

	if code != exitOK {
		t.Fatalf("config --merged returned %d, want 0", code)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "local-as 65000") {
		t.Errorf("output should contain base content, got: %q", output)
	}
	if !strings.Contains(output, "neighbor 10.0.0.1") {
		t.Errorf("output should contain overlay content, got: %q", output)
	}
}
