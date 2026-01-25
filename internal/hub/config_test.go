package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseHubConfigBasic verifies basic config parsing.
//
// VALIDATES: Basic 3-section config parses correctly.
// PREVENTS: Parse failure on valid config.
func TestParseHubConfigBasic(t *testing.T) {
	input := `
env {
    api-socket /var/run/ze/api.sock;
    log-level info;
}

plugin {
    external bgp {
        run "ze bgp --child";
    }
    external rib {
        run "ze rib --child";
    }
}

bgp {
    router-id 1.2.3.4;
    local-as 65000;
}
`
	cfg, err := ParseHubConfig(input)
	require.NoError(t, err)

	// Check env
	assert.Equal(t, "/var/run/ze/api.sock", cfg.Env["api-socket"])
	assert.Equal(t, "info", cfg.Env["log-level"])

	// Check plugins
	require.Len(t, cfg.Plugins, 2)
	assert.Equal(t, "bgp", cfg.Plugins[0].Name)
	assert.Equal(t, "ze bgp --child", cfg.Plugins[0].Run)
	assert.Equal(t, "rib", cfg.Plugins[1].Name)

	// Check remaining blocks stored
	require.NotNil(t, cfg.Blocks)
	assert.Contains(t, cfg.Blocks, "bgp")
}

// TestParseHubConfigEnvOnly verifies config with only env block.
//
// VALIDATES: Env-only config works.
// PREVENTS: Failure when plugin block missing.
func TestParseHubConfigEnvOnly(t *testing.T) {
	input := `
env {
    api-socket /tmp/ze.sock;
}
`
	cfg, err := ParseHubConfig(input)
	require.NoError(t, err)
	assert.Equal(t, "/tmp/ze.sock", cfg.Env["api-socket"])
	assert.Empty(t, cfg.Plugins)
}

// TestParseHubConfigPluginOnly verifies config with only plugin block.
//
// VALIDATES: Plugin-only config works.
// PREVENTS: Failure when env block missing.
func TestParseHubConfigPluginOnly(t *testing.T) {
	input := `
plugin {
    external test {
        run "echo test";
    }
}
`
	cfg, err := ParseHubConfig(input)
	require.NoError(t, err)
	require.Len(t, cfg.Plugins, 1)
	assert.Equal(t, "test", cfg.Plugins[0].Name)
}

// TestParseHubConfigEmpty verifies empty config.
//
// VALIDATES: Empty config returns defaults.
// PREVENTS: Panic on empty input.
func TestParseHubConfigEmpty(t *testing.T) {
	cfg, err := ParseHubConfig("")
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Empty(t, cfg.Plugins)
}

// TestParseHubConfigMultiplePlugins verifies multiple plugin definitions.
//
// VALIDATES: Multiple external plugins parse correctly.
// PREVENTS: Only first plugin parsed.
func TestParseHubConfigMultiplePlugins(t *testing.T) {
	input := `
plugin {
    external bgp {
        run "ze bgp --child";
    }
    external rib {
        run "ze rib --child";
    }
    external gr {
        run "ze gr --child";
    }
}
`
	cfg, err := ParseHubConfig(input)
	require.NoError(t, err)
	require.Len(t, cfg.Plugins, 3)
	assert.Equal(t, "bgp", cfg.Plugins[0].Name)
	assert.Equal(t, "rib", cfg.Plugins[1].Name)
	assert.Equal(t, "gr", cfg.Plugins[2].Name)
}

// TestParseHubConfigFile verifies loading from file.
//
// VALIDATES: File loading works.
// PREVENTS: File path handling errors.
func TestParseHubConfigFile(t *testing.T) {
	// This test uses a non-existent file to test error handling
	_, err := LoadHubConfig("/nonexistent/path/config.conf")
	require.Error(t, err)
}

// TestParseHubConfigBlocks verifies non-env/plugin blocks are captured.
//
// VALIDATES: BGP, RIB and other blocks stored for routing.
// PREVENTS: Config blocks lost during parsing.
func TestParseHubConfigBlocks(t *testing.T) {
	input := `
env {
    log-level debug;
}

bgp {
    router-id 10.0.0.1;
    peer 192.168.1.1 {
        remote-as 65001;
    }
}

rib {
    max-routes 100000;
}
`
	cfg, err := ParseHubConfig(input)
	require.NoError(t, err)
	assert.Contains(t, cfg.Blocks, "bgp")
	assert.Contains(t, cfg.Blocks, "rib")
}
