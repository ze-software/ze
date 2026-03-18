package rpki

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: parseRPKIConfig extracts cache-server list from BGP config JSON
// PREVENTS: config delivered via OnConfigure being silently ignored

func TestParseRPKIConfigBasic(t *testing.T) {
	jsonStr := `{
		"rpki": {
			"cache-server": {
				"127.0.0.1": {
					"port": "3323",
					"preference": "50"
				}
			},
			"validation-timeout": "60"
		}
	}`

	cfg, err := parseRPKIConfig(jsonStr)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Len(t, cfg.CacheServers, 1)

	cs := cfg.CacheServers[0]
	assert.Equal(t, "127.0.0.1", cs.Address)
	assert.Equal(t, uint16(3323), cs.Port)
	assert.Equal(t, uint8(50), cs.Preference)
	assert.Equal(t, uint16(60), cfg.ValidationTimeout)
}

func TestParseRPKIConfigDefaults(t *testing.T) {
	// Cache server with no port/preference uses YANG defaults
	jsonStr := `{
		"rpki": {
			"cache-server": {
				"10.0.0.1": {}
			}
		}
	}`

	cfg, err := parseRPKIConfig(jsonStr)
	require.NoError(t, err)
	require.Len(t, cfg.CacheServers, 1)

	cs := cfg.CacheServers[0]
	assert.Equal(t, "10.0.0.1", cs.Address)
	assert.Equal(t, uint16(323), cs.Port)             // RTR default
	assert.Equal(t, uint8(100), cs.Preference)        // YANG default
	assert.Equal(t, uint16(0), cfg.ValidationTimeout) // not set
}

func TestParseRPKIConfigMultipleServers(t *testing.T) {
	jsonStr := `{
		"rpki": {
			"cache-server": {
				"10.0.0.1": {"port": "3323"},
				"10.0.0.2": {"port": "3324", "preference": "200"}
			}
		}
	}`

	cfg, err := parseRPKIConfig(jsonStr)
	require.NoError(t, err)
	assert.Len(t, cfg.CacheServers, 2)

	// Verify both servers present (order may vary -- map iteration)
	addrs := map[string]bool{}
	for _, cs := range cfg.CacheServers {
		addrs[cs.Address] = true
	}
	assert.True(t, addrs["10.0.0.1"])
	assert.True(t, addrs["10.0.0.2"])
}

func TestParseRPKIConfigNoRPKISection(t *testing.T) {
	// BGP config without rpki section returns empty config
	jsonStr := `{
		"peer": {
			"127.0.0.1": {"peer-as": "65001"}
		}
	}`

	cfg, err := parseRPKIConfig(jsonStr)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Empty(t, cfg.CacheServers)
}

func TestParseRPKIConfigNoCacheServers(t *testing.T) {
	// RPKI section with no cache-server list
	jsonStr := `{
		"rpki": {
			"validation-timeout": "45"
		}
	}`

	cfg, err := parseRPKIConfig(jsonStr)
	require.NoError(t, err)
	assert.Empty(t, cfg.CacheServers)
	assert.Equal(t, uint16(45), cfg.ValidationTimeout)
}

func TestParseRPKIConfigInvalidJSON(t *testing.T) {
	_, err := parseRPKIConfig("not-json")
	assert.Error(t, err)
}
