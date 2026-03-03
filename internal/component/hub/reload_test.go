package hub

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDiffPluginDefs verifies plugin definition diffing by name.
//
// VALIDATES: diffPluginDefs correctly identifies added, removed, and unchanged plugins.
// PREVENTS: Incorrect reload actions (starting already-running plugins, not stopping removed ones).
func TestDiffPluginDefs(t *testing.T) {
	tests := []struct {
		name      string
		old       []PluginDef
		new       []PluginDef
		wantAdded []string
		wantRemov []string
		wantKept  []string
	}{
		{
			name:      "no_changes",
			old:       []PluginDef{{Name: "bgp", Run: "ze bgp"}, {Name: "rib", Run: "ze rib"}},
			new:       []PluginDef{{Name: "bgp", Run: "ze bgp"}, {Name: "rib", Run: "ze rib"}},
			wantAdded: nil,
			wantRemov: nil,
			wantKept:  []string{"bgp", "rib"},
		},
		{
			name:      "plugin_added",
			old:       []PluginDef{{Name: "bgp", Run: "ze bgp"}},
			new:       []PluginDef{{Name: "bgp", Run: "ze bgp"}, {Name: "rib", Run: "ze rib"}},
			wantAdded: []string{"rib"},
			wantRemov: nil,
			wantKept:  []string{"bgp"},
		},
		{
			name:      "plugin_removed",
			old:       []PluginDef{{Name: "bgp", Run: "ze bgp"}, {Name: "rib", Run: "ze rib"}},
			new:       []PluginDef{{Name: "bgp", Run: "ze bgp"}},
			wantAdded: nil,
			wantRemov: []string{"rib"},
			wantKept:  []string{"bgp"},
		},
		{
			name:      "add_and_remove",
			old:       []PluginDef{{Name: "bgp", Run: "ze bgp"}, {Name: "rib", Run: "ze rib"}},
			new:       []PluginDef{{Name: "bgp", Run: "ze bgp"}, {Name: "gr", Run: "ze gr"}},
			wantAdded: []string{"gr"},
			wantRemov: []string{"rib"},
			wantKept:  []string{"bgp"},
		},
		{
			name:      "empty_to_plugins",
			old:       nil,
			new:       []PluginDef{{Name: "bgp", Run: "ze bgp"}},
			wantAdded: []string{"bgp"},
			wantRemov: nil,
			wantKept:  nil,
		},
		{
			name:      "plugins_to_empty",
			old:       []PluginDef{{Name: "bgp", Run: "ze bgp"}},
			new:       nil,
			wantAdded: nil,
			wantRemov: []string{"bgp"},
			wantKept:  nil,
		},
		{
			name:      "both_empty",
			old:       nil,
			new:       nil,
			wantAdded: nil,
			wantRemov: nil,
			wantKept:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			added, removed, kept := diffPluginDefs(tt.old, tt.new)
			assert.ElementsMatch(t, tt.wantAdded, pluginDefNames(added), "added")
			assert.ElementsMatch(t, tt.wantRemov, removed, "removed")
			assert.ElementsMatch(t, tt.wantKept, kept, "kept")
		})
	}
}

// TestOrchestratorReloadNoChanges verifies SIGHUP with identical config is a no-op.
//
// VALIDATES: Reload with unchanged config doesn't restart anything.
// PREVENTS: Unnecessary plugin restarts on SIGHUP.
func TestOrchestratorReloadNoChanges(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.conf")
	configData := `plugin {
	external bgp {
		run "echo bgp";
	}
}
`
	require.NoError(t, os.WriteFile(configPath, []byte(configData), 0o644))

	cfg := &HubConfig{
		Plugins:    []PluginDef{{Name: "bgp", Run: "echo bgp"}},
		Env:        map[string]string{},
		Blocks:     map[string]any{},
		ConfigPath: configPath,
	}

	o := NewOrchestrator(cfg)
	err := o.Reload(configPath)
	require.NoError(t, err)

	// Config should be unchanged
	assert.Equal(t, 1, len(o.config.Plugins))
	assert.Equal(t, "bgp", o.config.Plugins[0].Name)
}

// TestOrchestratorReloadAddPlugin verifies new plugin definition registers new subsystem.
// Note: Start is skipped because o.ctx is nil (no Start() call) — only registration is tested.
//
// VALIDATES: Added plugin is registered in SubsystemManager after reload.
// PREVENTS: New plugins being silently ignored on reload.
func TestOrchestratorReloadAddPlugin(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.conf")

	cfg := &HubConfig{
		Plugins:    []PluginDef{{Name: "bgp", Run: "echo bgp"}},
		Env:        map[string]string{},
		Blocks:     map[string]any{},
		ConfigPath: configPath,
	}

	newConfig := `plugin {
	external bgp {
		run "echo bgp";
	}
	external rib {
		run "echo rib";
	}
}
`
	require.NoError(t, os.WriteFile(configPath, []byte(newConfig), 0o644))

	o := NewOrchestrator(cfg)
	err := o.Reload(configPath)
	require.NoError(t, err)

	assert.Equal(t, 2, len(o.config.Plugins))
	assert.NotNil(t, o.subsystems.Get("rib"))
}

// TestOrchestratorReloadRemovePlugin verifies removed plugin definition stops subsystem.
//
// VALIDATES: Removed plugin is unregistered from SubsystemManager after reload.
// PREVENTS: Zombie plugins running after config removal.
func TestOrchestratorReloadRemovePlugin(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.conf")

	cfg := &HubConfig{
		Plugins: []PluginDef{
			{Name: "bgp", Run: "echo bgp"},
			{Name: "rib", Run: "echo rib"},
		},
		Env:        map[string]string{},
		Blocks:     map[string]any{},
		ConfigPath: configPath,
	}

	newConfig := `plugin {
	external bgp {
		run "echo bgp";
	}
}
`
	require.NoError(t, os.WriteFile(configPath, []byte(newConfig), 0o644))

	o := NewOrchestrator(cfg)
	err := o.Reload(configPath)
	require.NoError(t, err)

	assert.Equal(t, 1, len(o.config.Plugins))
	assert.Nil(t, o.subsystems.Get("rib"))
}

// TestOrchestratorReloadConfigParseError verifies parse error preserves running config.
//
// VALIDATES: Malformed config file doesn't disrupt running orchestrator.
// PREVENTS: Bad config file crashing the daemon.
func TestOrchestratorReloadConfigParseError(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.conf")

	cfg := &HubConfig{
		Plugins:    []PluginDef{{Name: "bgp", Run: "echo bgp"}},
		Env:        map[string]string{},
		Blocks:     map[string]any{},
		ConfigPath: configPath,
	}

	require.NoError(t, os.WriteFile(configPath, []byte("{{invalid config"), 0o644))

	o := NewOrchestrator(cfg)
	err := o.Reload(configPath)
	require.Error(t, err)

	// Original config preserved
	assert.Equal(t, 1, len(o.config.Plugins))
	assert.Equal(t, "bgp", o.config.Plugins[0].Name)
}

// TestOrchestratorReloadFileNotFound verifies missing file preserves running config.
//
// VALIDATES: Non-existent config file doesn't disrupt running orchestrator.
// PREVENTS: File deletion crashing the daemon.
func TestOrchestratorReloadFileNotFound(t *testing.T) {
	cfg := &HubConfig{
		Plugins:    []PluginDef{{Name: "bgp", Run: "echo bgp"}},
		Env:        map[string]string{},
		Blocks:     map[string]any{},
		ConfigPath: "/nonexistent/config.conf",
	}

	o := NewOrchestrator(cfg)
	err := o.Reload("/nonexistent/config.conf")
	require.Error(t, err)

	// Original config preserved
	assert.Equal(t, 1, len(o.config.Plugins))
}

// TestOrchestratorReloadEnvChangeWarning verifies env changes are detected but not applied.
//
// VALIDATES: Changed env block is detected during reload.
// PREVENTS: Silent env changes that require restart going unnoticed.
func TestOrchestratorReloadEnvChangeWarning(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.conf")

	cfg := &HubConfig{
		Plugins:    []PluginDef{{Name: "bgp", Run: "echo bgp"}},
		Env:        map[string]string{"socket": "/tmp/ze.sock"},
		Blocks:     map[string]any{},
		ConfigPath: configPath,
	}

	newConfig := `env {
	socket /tmp/ze-new.sock;
}
plugin {
	external bgp {
		run "echo bgp";
	}
}
`
	require.NoError(t, os.WriteFile(configPath, []byte(newConfig), 0o644))

	o := NewOrchestrator(cfg)
	err := o.Reload(configPath)
	require.NoError(t, err)

	// Env should NOT be updated (env changes require restart)
	assert.Equal(t, "/tmp/ze.sock", o.config.Env["socket"])
}

// pluginDefNames extracts names from a slice of PluginDef.
func pluginDefNames(defs []PluginDef) []string {
	if len(defs) == 0 {
		return nil
	}
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
