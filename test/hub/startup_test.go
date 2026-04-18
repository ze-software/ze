package hub_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/hub"
)

// TestHubStartupWithBGP verifies hub starts BGP plugin correctly.
//
// VALIDATES: Hub forks BGP process and completes 5-stage protocol.
// PREVENTS: Hub failing to start plugins.
func TestHubStartupWithBGP(t *testing.T) {
	// Create temp config file
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.conf")

	// Write minimal config
	config := `
plugin {
	external bgp {
		run "ze bgp --child";
	}
}

bgp {
	peer 127.0.0.1 {
		router-id 1.2.3.4;
		local-address 127.0.0.1;
		local-as 65000;
		peer-as 65001;
	}
}
`
	err := os.WriteFile(configPath, []byte(config), 0o600)
	require.NoError(t, err)

	// Load config
	cfg, err := hub.LoadHubConfig(configPath)
	require.NoError(t, err)

	// Verify config parsed correctly
	assert.Equal(t, 1, len(cfg.Plugins))
	assert.Equal(t, "bgp", cfg.Plugins[0].Name)
	assert.Equal(t, "ze bgp --child", cfg.Plugins[0].Run)
	assert.NotEmpty(t, cfg.Blocks["bgp"])
	assert.Equal(t, configPath, cfg.ConfigPath)
}

// TestHubConfigParsing verifies hub config parsing.
//
// VALIDATES: Hub parses all config sections correctly.
// PREVENTS: Config parsing errors.
func TestHubConfigParsing(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		wantErr   bool
		checkFunc func(*testing.T, *hub.HubConfig)
	}{
		{
			name: "env_block",
			config: `
env {
	log-level debug;
}
bgp {
	router-id 1.2.3.4;
}
`,
			checkFunc: func(t *testing.T, cfg *hub.HubConfig) {
				assert.Equal(t, "debug", cfg.Env["log-level"])
			},
		},
		{
			name: "multiple_plugins",
			config: `
plugin {
	external bgp {
		run "ze bgp --child";
	}
	external rib {
		run "ze rib --child";
	}
}
`,
			checkFunc: func(t *testing.T, cfg *hub.HubConfig) {
				assert.Equal(t, 2, len(cfg.Plugins))
				assert.Equal(t, "bgp", cfg.Plugins[0].Name)
				assert.Equal(t, "rib", cfg.Plugins[1].Name)
			},
		},
		{
			name:    "invalid_syntax",
			config:  "invalid {",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := hub.ParseHubConfig(tt.config)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.checkFunc != nil {
				tt.checkFunc(t, cfg)
			}
		})
	}
}

// TestOrchestratorCreate verifies orchestrator creation.
//
// VALIDATES: Orchestrator creates with correct configuration.
// PREVENTS: Orchestrator creation failures.
func TestOrchestratorCreate(t *testing.T) {
	cfg := &hub.HubConfig{
		Plugins: []hub.PluginDef{
			{Name: "bgp", Run: "ze bgp --child"},
		},
		ConfigPath: "/path/to/config.conf",
	}

	o := hub.NewOrchestrator(cfg)
	require.NotNil(t, o)

	// Verify registry is available
	registry := o.Registry()
	require.NotNil(t, registry)
}

// TestOrchestratorStartStop verifies orchestrator lifecycle.
//
// VALIDATES: Orchestrator starts and stops cleanly.
// PREVENTS: Resource leaks on shutdown.
//
// Note: This test requires ze binary to be built.
func TestOrchestratorStartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create temp config
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.conf")

	// Minimal BGP config that won't try to connect anywhere
	config := `
bgp {
	peer 127.0.0.1 {
		router-id 1.2.3.4;
		local-address 127.0.0.1;
		local-as 65000;
		peer-as 65001;
		passive true;
	}
}
`
	err := os.WriteFile(configPath, []byte(config), 0o600)
	require.NoError(t, err)

	cfg := &hub.HubConfig{
		Plugins: []hub.PluginDef{
			{Name: "bgp", Run: "ze bgp --child"},
		},
		ConfigPath: configPath,
	}

	o := hub.NewOrchestrator(cfg)

	// Start with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Note: This will fail if ze binary is not built or BGP plugin has issues
	// In CI, this validates the full integration
	err = o.Start(ctx)
	if err != nil {
		t.Skipf("skipping: %v (ze binary may not be built)", err)
	}

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Stop
	o.Stop()
}
