package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// TestLogShowHandler verifies handler returns subsystem list from registry.
//
// VALIDATES: AC-1 — bgp log show returns JSON map of subsystem names to levels.
// PREVENTS: Handler returning empty when loggers exist.
func TestLogShowHandler(t *testing.T) {
	slogutil.ResetLevelRegistry()
	defer slogutil.ResetLevelRegistry()

	t.Setenv("ze.log.showtest", "info")
	_ = slogutil.Logger("showtest")

	ctx := &pluginserver.CommandContext{}
	resp, err := handleLogShow(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "expected map[string]any data")

	levels, ok := data["levels"].(map[string]string)
	require.True(t, ok, "expected map[string]string in levels field")
	assert.Contains(t, levels, "showtest")
	assert.Equal(t, "info", levels["showtest"])
}

// TestLogSetHandler verifies handler changes level via SetLevel().
//
// VALIDATES: AC-2 — bgp log set changes subsystem to specified level.
// PREVENTS: Level change having no effect.
func TestLogSetHandler(t *testing.T) {
	slogutil.ResetLevelRegistry()
	defer slogutil.ResetLevelRegistry()

	t.Setenv("ze.log.sethandlertest", "warn")
	_ = slogutil.Logger("sethandlertest")

	ctx := &pluginserver.CommandContext{}
	resp, err := handleLogSet(ctx, []string{"sethandlertest", "debug"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "sethandlertest", data["subsystem"])
	assert.Equal(t, "debug", data["level"])

	// Verify level actually changed
	levels := slogutil.ListLevels()
	assert.Equal(t, "debug", levels["sethandlertest"])
}

// TestLogSetMissingArgs verifies handler returns usage error with no args.
//
// VALIDATES: AC-5 — bgp log set with missing args returns usage error.
// PREVENTS: Panic on missing arguments.
func TestLogSetMissingArgs(t *testing.T) {
	ctx := &pluginserver.CommandContext{}

	// No args
	resp, err := handleLogSet(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "usage")

	// One arg
	resp, err = handleLogSet(ctx, []string{"server"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "usage")
}

// TestLogSetInvalidLevel verifies handler returns error for bad level string.
//
// VALIDATES: AC-4 — bgp log set with invalid level returns error.
// PREVENTS: Accepting typos silently.
func TestLogSetInvalidLevel(t *testing.T) {
	slogutil.ResetLevelRegistry()
	defer slogutil.ResetLevelRegistry()

	t.Setenv("ze.log.invalidsettest", "info")
	_ = slogutil.Logger("invalidsettest")

	ctx := &pluginserver.CommandContext{}
	resp, err := handleLogSet(ctx, []string{"invalidsettest", "badlevel"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "invalid level")
}

// TestLogSetUnknownSubsystem verifies handler returns error for unknown subsystem.
//
// VALIDATES: AC-3 — bgp log set with unknown subsystem returns error.
// PREVENTS: Silent no-op for wrong subsystem name.
func TestLogSetUnknownSubsystem(t *testing.T) {
	slogutil.ResetLevelRegistry()
	defer slogutil.ResetLevelRegistry()

	ctx := &pluginserver.CommandContext{}
	resp, err := handleLogSet(ctx, []string{"nonexistent", "info"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "unknown subsystem")
}
