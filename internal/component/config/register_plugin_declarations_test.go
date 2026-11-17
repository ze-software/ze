package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
)

// These tests give the reader `show plugin declarations config` asks a config
// that declares an external plugin whose run string is a real program
// (markingPlugin, declared_schemas_test.go). The reader names the process and
// starts nothing: starting is the command's choice, made from that answer.
// None runs in parallel: each resets the module set the process published.

// writeDeclaringConfig writes, in a directory of its own, a config that
// declares external plugin acme with run, and answers its path.
func writeDeclaringConfig(t *testing.T, run string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ze.conf")
	require.NoError(t, os.WriteFile(path, []byte(configDeclaring("acme", run, "")), 0o600))
	return path
}

// declaredRun answers the run string the reader gave for plugin name, and
// fails the test when the reader did not list the plugin.
func declaredRun(t *testing.T, plugins []plugin.PluginConfig, name string) string {
	t.Helper()
	for _, configured := range plugins {
		if configured.Name == name {
			return configured.Run
		}
	}
	require.Failf(t, "plugin not listed", "the reader did not list plugin %q: %v", name, plugins)
	return ""
}

// TestConfiguredPluginReaderNamesACommandWithNoDaemon: in a process that
// published nothing and keeps no module set, the reader says the file was read
// by a command with no daemon. Method: read a config that declares an external
// plugin, in a process reset to that state.
//
// VALIDATES: the owner ruling "plugin command requiring external plugins which
// can not be started from the code when no daemon is running to pass it to are
// not available", at the seam `show plugin declarations config` decides from:
// the process is ConfigProcessOffline, the plugin is listed with its run
// string, and the reader started nothing.
// PREVENTS: the command reading a missing answer as a running daemon and
// starting the program.
func TestConfiguredPluginReaderNamesACommandWithNoDaemon(t *testing.T) {
	offlineProcess(t)
	run, _, runs := markingPlugin(t)

	configured, err := readConfiguredPlugins(writeDeclaringConfig(t, run))
	require.NoError(t, err)

	assert.Equal(t, plugin.ConfigProcessOffline, configured.Process)
	assert.Equal(t, run, declaredRun(t, configured.Plugins, "acme"))
	assert.Zero(t, runCount(t, runs), "the reader started the declared program")
}

// TestConfiguredPluginReaderNamesARunningDaemon: in a process whose module set
// is fixed, as a running daemon's is, the reader says a daemon read the file.
// Method: record a keeper, as the daemon's plugin server does, then read the
// same config.
//
// VALIDATES: the process is ConfigProcessDaemon, from the one test the config
// step makes (yang.ModuleSetFixed before it fixes anything), and the reader
// itself still starts nothing.
// PREVENTS: a running daemon answering every external plugin as not available.
func TestConfiguredPluginReaderNamesARunningDaemon(t *testing.T) {
	offlineProcess(t)
	run, _, runs := markingPlugin(t)
	yang.KeepsModuleSet("the plugin server of a test")
	require.True(t, yang.ModuleSetFixed())

	configured, err := readConfiguredPlugins(writeDeclaringConfig(t, run))
	require.NoError(t, err)

	assert.Equal(t, plugin.ConfigProcessDaemon, configured.Process)
	assert.Equal(t, run, declaredRun(t, configured.Plugins, "acme"))
	assert.Zero(t, runCount(t, runs), "the reader started the declared program")
}
