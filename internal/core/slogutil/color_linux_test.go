package slogutil

import (
	"bufio"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/creack/pty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/env"
)

// VALIDATES: A terminal reached through a relay gets color independently of a file.
func TestConsoleDestinationsSelectColorIndependently(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	require.NoError(t, os.Unsetenv("NO_COLOR"))
	t.Setenv("TERM", "xterm")
	t.Setenv("COLORFGBG", "")
	t.Setenv("ze.log.color", "")
	t.Setenv("ze.log.backend", "stderr,stdout")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	master, terminal, err := pty.Open()
	require.NoError(t, err)
	defer master.Close()
	defer terminal.Close()
	relayRead, relayWrite, err := os.Pipe()
	require.NoError(t, err)
	defer relayRead.Close()
	defer relayWrite.Close()
	file, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	require.NoError(t, err)
	defer file.Close()

	oldOut, oldErr := os.Stdout, os.Stderr
	oldRelay := logStderrRelay.Load()
	os.Stdout, os.Stderr = file, relayWrite
	SetStderrRelay(relayWrite, terminal)
	defer func() {
		os.Stdout, os.Stderr = oldOut, oldErr
		logStderrRelay.Store(oldRelay)
	}()

	logger := slog.New(createHandler(slog.LevelInfo, "test"))
	logger.Warn("peer changed", "state", "down")
	require.NoError(t, relayWrite.Close())

	line, err := bufio.NewReader(relayRead).ReadString('\n')
	require.NoError(t, err)
	assert.Contains(t, line, ansiLogPalette.warning+"WARN"+ansiReset)
	assert.Contains(t, line, ansiLogPalette.failure+"down"+ansiReset)
	plain, err := os.ReadFile(file.Name())
	require.NoError(t, err)
	assert.Contains(t, string(plain), `msg="peer changed" state=down`)
	assert.NotContains(t, string(plain), "\x1b")
}
