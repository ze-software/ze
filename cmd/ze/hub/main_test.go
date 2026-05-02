package hub

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/engine"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

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

	err = rollbackReload(context.Background(), nil, eng, cp, prior)
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

// TestRunHubConfigRejectsCLIPlugins verifies hub/orchestrator configs reject
// the global --plugin startup flag with actionable guidance.
func TestRunHubConfigRejectsCLIPlugins(t *testing.T) {
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

	exit := Run(storage.NewFilesystem(), configPath, []string{"bgp-rib"}, 0, -1, false, "", false, "", "")
	assert.Equal(t, 1, exit)

	_ = w.Close()
	data, readErr := io.ReadAll(r)
	assert.NoError(t, readErr)
	assert.Contains(t, string(data), "--plugin")
	assert.Contains(t, string(data), "plugin { external ... }")
	_ = r.Close()
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
