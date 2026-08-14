package hub

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/engine"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/ze"
)

func init() {
	skipRootCheck = true
}

type reloadProbeSubsystem struct {
	seen []string
}

func (s *reloadProbeSubsystem) Name() string { return "reload-probe" }

func (s *reloadProbeSubsystem) Start(context.Context, ze.EventBus, ze.ConfigProvider) error {
	return nil
}

func (s *reloadProbeSubsystem) Stop(context.Context) error { return nil }

func (s *reloadProbeSubsystem) Reload(_ context.Context, cfg ze.ConfigProvider) error {
	root, err := cfg.Get("bgp")
	if err != nil {
		return err
	}
	marker, _ := root["marker"].(string)
	s.seen = append(s.seen, marker)
	if marker == "bad" {
		return fmt.Errorf("bad marker")
	}
	return nil
}

// TestRunMissingConfig verifies error handling for missing config.
//
// VALIDATES: Hub returns error for non-existent config.
// PREVENTS: Silent failure when config file not found.
func TestRunMissingConfig(t *testing.T) {
	exit := Run(storage.NewFilesystem(), "/nonexistent/config.conf", nil, 0, -1, false, "", false, "", "")
	assert.Equal(t, 1, exit)
}

// TestEphemeralDaemonStartsSSH verifies that an ephemeral daemon (started by
// config edit) creates an SSH listener and writes the address file even with
// an empty config that has no ssh{} or bgp{} block.
//
// VALIDATES: Ephemeral SSH starts for any config type and writes address file.
// PREVENTS: "command executor not ready" when config edit runs operational commands.
func TestEphemeralDaemonStartsSSH(t *testing.T) {
	// test-relax: ssh is compile-out-able (//go:build ze_ssh). Under the default
	// ze_core unit build ssh is absent, so the ephemeral ssh daemon cannot start;
	// the ze_ssh build (TestBuildTags) runs this test fully. Not weakened coverage.
	if sshBuildStandalone == nil {
		t.Skip("ssh compiled out (ze_ssh off): ephemeral ssh daemon is unavailable")
	}
	addrFile := filepath.Join(t.TempDir(), "ephemeral-ssh.addr")
	require.NoError(t, env.Set("ze.ssh.ephemeral", addrFile))
	defer func() { require.NoError(t, env.Set("ze.ssh.ephemeral", "")) }()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "empty.conf")
	require.NoError(t, os.WriteFile(configPath, []byte{}, 0o600))

	// Run daemon in a goroutine; it blocks on signal wait when startup succeeds.
	// Send SIGINT after a short delay to unblock it.
	exitCh := make(chan int, 1)
	go func() {
		exitCh <- Run(storage.NewFilesystem(), configPath, nil, 0, -1, false, "", false, "", "")
	}()

	// Wait for the ephemeral address file to appear (SSH started).
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if data, readErr := os.ReadFile(addrFile); readErr == nil && len(data) > 0 {
			// SSH started and wrote address file: ephemeral mode works.
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	data, readErr := os.ReadFile(addrFile)
	require.NoError(t, readErr, "ephemeral SSH address file should exist")
	assert.NotEmpty(t, data, "ephemeral SSH address file should contain an address")

	// Send SIGINT to stop the daemon.
	proc, procErr := os.FindProcess(os.Getpid())
	require.NoError(t, procErr)
	require.NoError(t, proc.Signal(os.Interrupt))

	select {
	case exit := <-exitCh:
		assert.Equal(t, 0, exit)
	case <-time.After(10 * time.Second):
		t.Fatal("daemon did not exit after SIGINT")
	}
}

// TestRollbackReloadRestoresProviderOnSubsystemFailure verifies failed subsystem reload restores provider roots.
// VALIDATES: SIGHUP reload rolls the ConfigProvider back to its previous roots after subsystem failure.
// PREVENTS: Provider and subsystems staying on different config versions after a failed reload.
func TestRollbackReloadRestoresProviderOnSubsystemFailure(t *testing.T) {
	cp := zeconfig.NewProvider()
	cp.SetRoot("bgp", map[string]any{"marker": "old"})
	cp.SetRoot("l2tp", map[string]any{"enabled": "true"})

	eng := engine.NewEngine(nil, cp, nil)
	probe := &reloadProbeSubsystem{}
	require.NoError(t, eng.RegisterSubsystem(probe))

	prior, err := snapshotProvider(cp)
	require.NoError(t, err)
	applyLoadedTreeToProvider(cp, map[string]any{
		"bgp": map[string]any{"marker": "bad"},
	})

	err = eng.Reload(context.Background())
	require.Error(t, err)
	assert.Equal(t, []string{"bad"}, probe.seen)

	err = rollbackReload(context.Background(), nil, eng, cp, prior, nil)
	require.NoError(t, err)

	bgpRoot, err := cp.Get("bgp")
	require.NoError(t, err)
	assert.Equal(t, "old", bgpRoot["marker"])
	l2tpRoot, err := cp.Get("l2tp")
	require.NoError(t, err)
	assert.Equal(t, "true", l2tpRoot["enabled"])
	assert.Equal(t, []string{"bad", "old"}, probe.seen)
}

// TestRunInvalidConfig verifies error handling for invalid config.
//
// VALIDATES: Hub returns error for malformed config.
// PREVENTS: Crash on invalid config syntax.
func TestRunInvalidConfig(t *testing.T) {
	// Create temp config with invalid syntax
	dir := t.TempDir()
	configPath := filepath.Join(dir, "invalid.conf")
	err := os.WriteFile(configPath, []byte("invalid { syntax"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	exit := Run(storage.NewFilesystem(), configPath, nil, 0, -1, false, "", false, "", "")
	assert.Equal(t, 1, exit)
}

// TestRunPassesCLIPluginsToConfigLoad pins the contract commit 8d92e9fab shipped
// when it deleted the standalone orchestrator: a plugin-only config reaches
// runYANGConfig like every other config, and the --plugin list goes with it to
// zeconfig.LoadConfig. There is no pre-flight refusal any more.
//
// VALIDATES: --plugin is accepted for a config the old ProbeConfigType called a
// hub config, and the name it carries is resolved by the config loader.
//
// PREVENTS: the predecessor test asserted the refusal 8d92e9fab deleted. Nothing
// updated it, so it kept calling Run on a config that now BOOTS: the call parked
// in waitLoop and cmd/ze/hub hit the 20 minute package timeout on every CI run
// from 2026-08-12.
//
// The unresolvable ze. name is what makes this test terminate. It fails inside
// MergeCliPlugins (internal/component/config/loader.go), which sits downstream of
// the deleted guard and upstream of the daemon, so the failure proves the list
// traveled the whole path. A restored pre-flight refusal reports a different
// message and fails the assertions below; a --plugin dropped on the floor lets
// the config boot, and the deadline reports that in seconds rather than at the
// package timeout.
func TestRunPassesCLIPluginsToConfigLoad(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "hub.conf")
	err := os.WriteFile(configPath, []byte("plugin { external demo { } }\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	defer func() {
		os.Stderr = origStderr
	}()

	captured := make(chan string, 1)
	go func() {
		data, readErr := io.ReadAll(r)
		if readErr != nil {
			captured <- ""
			return
		}
		captured <- string(data)
	}()

	exited := make(chan int, 1)
	go func() {
		exited <- Run(storage.NewFilesystem(), configPath, []string{"ze.no-such-plugin"}, 0, -1, false, "", false, "", "")
	}()

	select {
	case exit := <-exited:
		assert.Equal(t, 1, exit)
	case <-time.After(60 * time.Second):
		t.Fatal("Run did not return: the config booted, so --plugin never reached LoadConfig")
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	stderr := <-captured
	assert.Contains(t, stderr, "load config", "the failure comes from the config load phase")
	assert.Contains(t, stderr, "ze.no-such-plugin", "the CLI plugin name reached plugin resolution")
	assert.Contains(t, stderr, "unknown internal plugin", "resolution refused the name, no pre-flight guard did")
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestReadStdinConfigWithNUL verifies that a NUL sentinel stops reading
// and reports stdin as still open for liveness monitoring.
//
// VALIDATES: Config data before NUL is returned, stdinOpen=true.
// PREVENTS: Ze blocking forever on stdin when upstream keeps pipe open.
func TestReadStdinConfigWithNUL(t *testing.T) {
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	os.Stdin = r
	config := "bgp {\n  local {\n    as 65000;\n  }\n}\n"

	// Keep the write end open until test completes (simulates long-lived upstream).
	done := make(chan struct{})
	go func() {
		if _, wErr := w.WriteString(config); wErr != nil {
			return
		}
		if _, wErr := w.Write([]byte{0}); wErr != nil {
			return
		}
		<-done
	}()

	data, stdinOpen, readErr := readStdinConfig()
	close(done) // Release goroutine.

	assert.NoError(t, readErr)
	assert.True(t, stdinOpen, "stdin should remain open after NUL sentinel")
	assert.Equal(t, config, string(data))

	if closeErr := w.Close(); closeErr != nil {
		t.Log("close pipe writer:", closeErr)
	}
	if closeErr := r.Close(); closeErr != nil {
		t.Log("close pipe reader:", closeErr)
	}
}

// TestReadStdinConfigEOF verifies that plain EOF (no NUL) returns the
// full data with stdinOpen=false — the normal "cat file | ze -" case.
//
// VALIDATES: Full config returned, stdinOpen=false on plain EOF.
// PREVENTS: False liveness monitoring when stdin is a regular file/pipe.
func TestReadStdinConfigEOF(t *testing.T) {
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	// Write and close before reading — pipe buffer holds the data.
	config := "bgp {\n  local {\n    as 65000;\n  }\n}\n"
	if _, wErr := w.WriteString(config); wErr != nil {
		t.Fatal(wErr)
	}
	if closeErr := w.Close(); closeErr != nil {
		t.Log("close pipe writer:", closeErr)
	}

	os.Stdin = r

	data, stdinOpen, readErr := readStdinConfig()
	assert.NoError(t, readErr)
	assert.False(t, stdinOpen, "stdin should be closed after EOF")
	assert.Equal(t, config, string(data))

	if closeErr := r.Close(); closeErr != nil {
		t.Log("close pipe reader:", closeErr)
	}
}

// TestReadStdinConfigEmpty verifies empty stdin returns empty data.
//
// VALIDATES: Empty stdin returns empty slice, no error.
// PREVENTS: Panic or error on empty pipe input.
func TestReadStdinConfigEmpty(t *testing.T) {
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	if closeErr := w.Close(); closeErr != nil {
		t.Log("close pipe writer:", closeErr)
	}
	os.Stdin = r

	data, stdinOpen, readErr := readStdinConfig()
	assert.NoError(t, readErr)
	assert.False(t, stdinOpen)
	assert.Empty(t, data)

	if closeErr := r.Close(); closeErr != nil {
		t.Log("close pipe reader:", closeErr)
	}
}
