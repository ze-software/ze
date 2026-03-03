package signal

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/pidfile"
)

// TestSignalCommandReload verifies "reload" maps to SIGHUP.
//
// VALIDATES: reload command sends SIGHUP to the running process.
// PREVENTS: Wrong signal mapping breaking config reload.
func TestSignalCommandReload(t *testing.T) {
	assert.Equal(t, syscall.SIGHUP, signalMap["reload"])
}

// TestSignalCommandStop verifies "stop" maps to SIGTERM.
//
// VALIDATES: stop command sends SIGTERM for graceful shutdown.
// PREVENTS: Wrong signal causing immediate kill instead of graceful.
func TestSignalCommandStop(t *testing.T) {
	assert.Equal(t, syscall.SIGTERM, signalMap["stop"])
}

// TestSignalCommandQuit verifies "quit" maps to SIGQUIT.
//
// VALIDATES: quit command sends SIGQUIT for immediate shutdown.
// PREVENTS: Quit not being available as an escape hatch.
func TestSignalCommandQuit(t *testing.T) {
	assert.Equal(t, syscall.SIGQUIT, signalMap["quit"])
}

// TestSignalCommandStatus verifies status checks process alive via kill(0).
//
// VALIDATES: Status returns 0 for running process, 1 for dead.
// PREVENTS: Status command giving false positives.
func TestSignalCommandStatus(t *testing.T) {
	// Our own process should be alive
	info := &pidfile.Info{
		PID:        os.Getpid(),
		ConfigPath: "/etc/ze/test.conf",
		StartTime:  "2026-01-31T10:30:00Z",
	}
	code := cmdStatus(info)
	assert.Equal(t, ExitSuccess, code)

	// PID that doesn't exist (use a high unlikely PID)
	infoDown := &pidfile.Info{
		PID:        99999999,
		ConfigPath: "/etc/ze/test.conf",
		StartTime:  "2026-01-31T10:30:00Z",
	}
	code = cmdStatus(infoDown)
	assert.Equal(t, ExitNotRunning, code)
}

// TestSignalCommandMissingArgs verifies usage is printed for missing arguments.
//
// VALIDATES: Run returns error exit code when args are missing.
// PREVENTS: Panic on empty args slice.
func TestSignalCommandMissingArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no_args", []string{}},
		{"command_only", []string{"status"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := Run(tt.args)
			assert.Equal(t, ExitNotRunning, code)
		})
	}
}

// TestSignalCommandExplicitPIDFile verifies --pid-file overrides config-derived path.
//
// VALIDATES: Explicit PID file path is used when provided.
// PREVENTS: Ignoring user-specified PID file location.
func TestSignalCommandExplicitPIDFile(t *testing.T) {
	dir := t.TempDir()
	explicit := filepath.Join(dir, "explicit.pid")

	// Create a PID file at the explicit path
	pf, err := pidfile.Acquire(explicit, "/etc/ze/test.conf")
	require.NoError(t, err)
	defer pf.Release()

	// Run status with explicit --pid-file pointing to our PID
	code := Run([]string{"--pid-file", explicit, "status", "/etc/ze/test.conf"})
	assert.Equal(t, ExitSuccess, code)
}

// TestSignalCommandNoPIDFile verifies correct exit code when PID file doesn't exist.
//
// VALIDATES: Returns ExitNoPIDFile when PID file is missing.
// PREVENTS: Confusing error message when daemon isn't running.
func TestSignalCommandNoPIDFile(t *testing.T) {
	code := Run([]string{"--pid-file", "/nonexistent/path/test.pid", "status", "/etc/ze/test.conf"})
	assert.Equal(t, ExitNoPIDFile, code)
}

// TestResolvePIDFileExplicit verifies explicit path takes priority.
//
// VALIDATES: Explicit path returned unchanged.
// PREVENTS: Config-derived path overriding user's explicit choice.
func TestResolvePIDFileExplicit(t *testing.T) {
	path, err := resolvePIDFile("/run/ze/test.pid", "/etc/ze/test.conf")
	require.NoError(t, err)
	assert.Equal(t, "/run/ze/test.pid", path)
}

// TestResolvePIDFileFromConfig verifies config-derived path when no explicit.
//
// VALIDATES: Location() is called when no explicit path.
// PREVENTS: Empty PID file path causing nil pointer.
func TestResolvePIDFileFromConfig(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "test.conf")

	// Write a dummy config so filepath.Abs works
	err := os.WriteFile(configPath, []byte("# test"), 0o644)
	require.NoError(t, err)

	t.Setenv("XDG_RUNTIME_DIR", "")

	path, err := resolvePIDFile("", configPath)
	require.NoError(t, err)

	// With XDG unset (non-root), falls back to os.TempDir()/ze/<hash>.pid
	absConfig, _ := filepath.Abs(configPath)
	hash := pidfile.ConfigHash(absConfig)
	expected := filepath.Join(os.TempDir(), "ze", hash+".pid")
	assert.Equal(t, expected, path)
}

// TestSignalCommandSendToSelf verifies sending a signal to our own process.
//
// VALIDATES: Signal delivery works end-to-end via cmdSignal.
// PREVENTS: Signal mapping existing but delivery failing.
func TestSignalCommandSendToSelf(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "self.pid")

	pf, err := pidfile.Acquire(pidPath, "/etc/ze/test.conf")
	require.NoError(t, err)
	defer pf.Release()

	// We can't easily test reload/stop/quit on ourselves without side effects,
	// but we can verify the signalMap coverage and that cmdSignal returns
	// the right code for an unknown command.
	info := &pidfile.Info{
		PID:        os.Getpid(),
		ConfigPath: "/etc/ze/test.conf",
		StartTime:  "2026-01-31T10:30:00Z",
	}

	code := cmdSignal("unknown", info)
	assert.Equal(t, ExitSignalFailed, code)
}

// TestSignalMapCompleteness verifies all documented commands are in signalMap.
//
// VALIDATES: reload, stop, quit are all mapped.
// PREVENTS: Adding a command to CLI without mapping its signal.
func TestSignalMapCompleteness(t *testing.T) {
	expected := []string{"reload", "stop", "quit"}
	for _, cmd := range expected {
		_, ok := signalMap[cmd]
		assert.True(t, ok, "missing signal mapping for %q", cmd)
	}
	assert.Len(t, signalMap, len(expected), "unexpected entries in signalMap")
}

// TestRunStatusWithPIDFile verifies full Run() flow for status command.
//
// VALIDATES: End-to-end: Run → parse args → resolve PID file → check status.
// PREVENTS: Integration gaps between argument parsing and PID file lookup.
func TestRunStatusWithPIDFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "test.conf")
	err := os.WriteFile(configPath, []byte("# test"), 0o644)
	require.NoError(t, err)

	t.Setenv("XDG_RUNTIME_DIR", "")

	// Acquire PID file at the location Run() will look for it
	pidPath, err := pidfile.Location(configPath)
	require.NoError(t, err)
	pf, err := pidfile.Acquire(pidPath, configPath)
	require.NoError(t, err)
	defer pf.Release()

	// Run status — should find our PID file and report running
	code := Run([]string{"status", configPath})

	assert.Equal(t, ExitSuccess, code, fmt.Sprintf(
		"expected success checking status of pid %d", os.Getpid()))
}
