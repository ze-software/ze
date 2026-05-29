package appliance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

func setupConfigPushAppliance(t *testing.T, name, address string) string {
	t.Helper()
	dir := t.TempDir()
	baseDir = dir

	appDir := filepath.Join(dir, name)
	os.MkdirAll(appDir, 0o755) //nolint:errcheck // test

	cfg := DefaultConfig(name)
	cfg.Device.Address = address
	data, _ := json.MarshalIndent(&cfg, "", "  ")
	os.WriteFile(filepath.Join(appDir, "appliance.json"), append(data, '\n'), 0o644) //nolint:errcheck,gosec // test

	overlay := "router bgp 65000\n neighbor 10.0.0.1 remote-as 65001\n"
	os.WriteFile(filepath.Join(appDir, "ze.conf"), []byte(overlay), 0o644) //nolint:errcheck,gosec // test

	return dir
}

func TestConfigPushUploadsConfig(t *testing.T) {
	dir := setupConfigPushAppliance(t, "lab", "10.0.0.100")
	baseDir = dir

	var commands []string
	var stdinData []string
	old := sshExecFunc
	sshExecFunc = func(addr, user, command, stdin string) sshResult {
		commands = append(commands, command)
		stdinData = append(stdinData, stdin)
		return sshResult{}
	}
	defer func() { sshExecFunc = old }()

	code := runConfigPush([]string{"lab"})
	if code != exitOK {
		t.Fatalf("config-push returned %d, want 0", code)
	}

	if len(commands) < 3 {
		t.Fatalf("expected at least 3 SSH commands, got %d", len(commands))
	}
	if commands[0] != "config stage" {
		t.Errorf("first command should stage config, got: %q", commands[0])
	}
	if commands[1] != "config validate staged" {
		t.Errorf("second command should validate staged, got: %q", commands[1])
	}
	if commands[2] != "config apply staged" {
		t.Errorf("third command should apply staged, got: %q", commands[2])
	}
	if !strings.Contains(stdinData[0], "router bgp 65000") {
		t.Errorf("stdin should contain config content, got: %q", stdinData[0])
	}
}

func TestConfigPushUsesConfiguredPortAndUser(t *testing.T) {
	dir := t.TempDir()
	baseDir = dir
	appDir := filepath.Join(dir, "lab")
	os.MkdirAll(appDir, 0o755) //nolint:errcheck // test

	cfg := DefaultConfig("lab")
	cfg.Device.Address = "10.0.0.100"
	cfg.SSH.Port = "2222"
	cfg.Credentials.Username = "operator"
	data, _ := json.MarshalIndent(&cfg, "", "  ")
	os.WriteFile(filepath.Join(appDir, "appliance.json"), append(data, '\n'), 0o644)    //nolint:errcheck,gosec // test
	os.WriteFile(filepath.Join(appDir, "ze.conf"), []byte("router bgp 65000\n"), 0o644) //nolint:errcheck,gosec // test

	var gotAddr, gotUser string
	old := sshExecFunc
	sshExecFunc = func(addr, user, command, stdin string) sshResult {
		gotAddr = addr
		gotUser = user
		return sshResult{}
	}
	defer func() { sshExecFunc = old }()

	if code := runConfigPush([]string{"lab"}); code != exitOK {
		t.Fatalf("config-push returned %d, want 0", code)
	}
	if gotAddr != "10.0.0.100:2222" {
		t.Errorf("addr = %q, want 10.0.0.100:2222 (cfg.SSH.Port honored)", gotAddr)
	}
	if gotUser != "operator" {
		t.Errorf("user = %q, want operator (cfg.Credentials.Username honored)", gotUser)
	}
}

func TestConfigPushInvalidConfigReverts(t *testing.T) {
	dir := setupConfigPushAppliance(t, "lab", "10.0.0.100")
	baseDir = dir

	old := sshExecFunc
	sshExecFunc = func(addr, user, command, stdin string) sshResult {
		if command == "config validate staged" {
			return sshResult{Output: "syntax error at line 3", Err: fmt.Errorf("exit 1")}
		}
		return sshResult{}
	}
	defer func() { sshExecFunc = old }()

	code := runConfigPush([]string{"lab"})
	if code != exitError {
		t.Errorf("config-push should fail on validation error, got %d", code)
	}
}

func TestConfigPushUnreachableDevice(t *testing.T) {
	dir := setupConfigPushAppliance(t, "edge-01", "192.0.2.99")
	baseDir = dir

	old := sshExecFunc
	sshExecFunc = func(addr, user, command, stdin string) sshResult {
		return sshResult{Err: fmt.Errorf("dial tcp %s: connection refused", addr)}
	}
	defer func() { sshExecFunc = old }()

	code := runConfigPush([]string{"edge-01"})
	if code != exitError {
		t.Errorf("config-push should fail for unreachable device, got %d", code)
	}
}

func TestConfigPushDryRun(t *testing.T) {
	dir := setupConfigPushAppliance(t, "lab", "10.0.0.100")
	baseDir = dir

	sshCalled := false
	old := sshExecFunc
	sshExecFunc = func(addr, user, command, stdin string) sshResult {
		sshCalled = true
		return sshResult{}
	}
	defer func() { sshExecFunc = old }()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := runConfigPush([]string{"--dry-run", "lab"})

	w.Close() //nolint:errcheck // test pipe
	os.Stdout = oldStdout

	if code != exitOK {
		t.Fatalf("config-push --dry-run returned %d, want 0", code)
	}

	if sshCalled {
		t.Error("--dry-run should not make SSH connections")
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])
	if !strings.Contains(output, "router bgp 65000") {
		t.Errorf("--dry-run should print merged config, got: %q", output)
	}
}

func TestConfigPushAllDevices(t *testing.T) {
	dir := t.TempDir()
	baseDir = dir

	for _, name := range []string{"dev1", "dev2", "dev3"} {
		appDir := filepath.Join(dir, name)
		os.MkdirAll(appDir, 0o755) //nolint:errcheck // test

		cfg := DefaultConfig(name)
		cfg.Device.Address = "10.0.0." + name[3:]
		data, _ := json.MarshalIndent(&cfg, "", "  ")
		os.WriteFile(filepath.Join(appDir, "appliance.json"), append(data, '\n'), 0o644)    //nolint:errcheck,gosec // test
		os.WriteFile(filepath.Join(appDir, "ze.conf"), []byte("router bgp 65000\n"), 0o644) //nolint:errcheck,gosec // test
	}

	pushed := make(map[string]bool)
	old := sshExecFunc
	sshExecFunc = func(addr, user, command, stdin string) sshResult {
		pushed[addr] = true
		return sshResult{}
	}
	defer func() { sshExecFunc = old }()

	code := runConfigPush([]string{"--all"})
	if code != exitOK {
		t.Fatalf("config-push --all returned %d, want 0", code)
	}

	if len(pushed) < 3 {
		t.Errorf("expected 3 devices pushed to, got %d", len(pushed))
	}
}

func TestConfigPushApplyStep(t *testing.T) {
	dir := setupConfigPushAppliance(t, "lab", "10.0.0.100")
	baseDir = dir

	var commands []string
	old := sshExecFunc
	sshExecFunc = func(addr, user, command, stdin string) sshResult {
		commands = append(commands, command)
		return sshResult{}
	}
	defer func() { sshExecFunc = old }()

	code := runConfigPush([]string{"lab"})
	if code != exitOK {
		t.Fatalf("config-push returned %d, want 0", code)
	}

	if !slices.Contains(commands, "config apply staged") {
		t.Error("config-push should issue 'config apply staged' command")
	}
}

func TestConfigPushAllParallel(t *testing.T) {
	dir := t.TempDir()
	baseDir = dir

	for i := range 4 {
		name := fmt.Sprintf("dev%d", i)
		appDir := filepath.Join(dir, name)
		os.MkdirAll(appDir, 0o755) //nolint:errcheck // test

		cfg := DefaultConfig(name)
		cfg.Device.Address = fmt.Sprintf("10.0.0.%d", i+1)
		data, _ := json.MarshalIndent(&cfg, "", "  ")
		os.WriteFile(filepath.Join(appDir, "appliance.json"), append(data, '\n'), 0o644)    //nolint:errcheck,gosec // test
		os.WriteFile(filepath.Join(appDir, "ze.conf"), []byte("router bgp 65000\n"), 0o644) //nolint:errcheck,gosec // test
	}

	pushed := make(map[string]bool)
	var mu sync.Mutex
	old := sshExecFunc
	sshExecFunc = func(addr, user, command, stdin string) sshResult {
		mu.Lock()
		pushed[addr] = true
		mu.Unlock()
		return sshResult{}
	}
	defer func() { sshExecFunc = old }()

	code := runConfigPush([]string{"--all", "--parallel", "4"})
	if code != exitOK {
		t.Fatalf("config-push --all --parallel 4 returned %d, want 0", code)
	}

	if len(pushed) < 4 {
		t.Errorf("expected 4 devices pushed to, got %d", len(pushed))
	}
}
