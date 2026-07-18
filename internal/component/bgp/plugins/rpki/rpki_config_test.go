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

// TestRPKISourceAddress verifies the cache-server source-address leaf is parsed
// into cacheServerConfig and threaded through NewRTRSession to the dialer field.
//
// VALIDATES: AC-5/AC-6 -- source-address reaches the RTR session (bound as
// LocalAddr) when configured, and is empty (OS-selected) when omitted.
// PREVENTS: the leaf being parsed but dropped before the RTRSession constructor.
func TestRPKISourceAddress(t *testing.T) {
	jsonStr := `{
		"rpki": {
			"cache-server": {
				"192.0.2.200": {"port": "3323", "source-address": "198.51.100.1"},
				"192.0.2.201": {"port": "3324"}
			}
		}
	}`

	cfg, err := parseRPKIConfig(jsonStr)
	require.NoError(t, err)
	require.Len(t, cfg.CacheServers, 2)

	byAddr := map[string]cacheServerConfig{}
	for _, cs := range cfg.CacheServers {
		byAddr[cs.Address] = cs
	}
	require.Equal(t, "198.51.100.1", byAddr["192.0.2.200"].SourceAddress)
	require.Empty(t, byAddr["192.0.2.201"].SourceAddress, "unconfigured server keeps empty source")

	// Constructor stores the source address for the dialer to bind.
	stopCh := make(chan struct{})
	sess := NewRTRSession("192.0.2.200", 3323, 100, "198.51.100.1", NewROACache(), NewASPACache(), stopCh)
	require.Equal(t, "198.51.100.1", sess.sourceAddress)

	sessNoSrc := NewRTRSession("192.0.2.201", 3324, 100, "", NewROACache(), NewASPACache(), stopCh)
	require.Empty(t, sessNoSrc.sourceAddress)
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

	// RFC 6811 origin-validation policy defaults (the YANG leaf defaults): the exclusion of
	// Invalid routes defaults to reject, NotFound defaults to accept.
	assert.Equal(t, ASPAPolicyReject, cfg.OriginInvalidAction, "default invalid-action is reject")
	assert.Equal(t, ASPAPolicyAccept, cfg.OriginNotFoundAction, "default not-found-action is accept")
}

// TestParseRPKIConfigOriginPolicy verifies the rpki/policy container (RFC 6811 origin-validation
// action) is parsed. The YANG leaf existed but was never read before this wiring, so the
// operator-facing action was a no-op.
func TestParseRPKIConfigOriginPolicy(t *testing.T) {
	cfg, err := parseRPKIConfig(`{
		"rpki": {
			"policy": {"invalid-action": "accept", "not-found-action": "reject"},
			"cache-server": {"10.0.0.1": {}}
		}
	}`)
	require.NoError(t, err)
	assert.Equal(t, ASPAPolicyAccept, cfg.OriginInvalidAction, "invalid-action=accept is parsed")
	assert.Equal(t, ASPAPolicyReject, cfg.OriginNotFoundAction, "not-found-action=reject is parsed")
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

func TestParseRPKIConfigASPAValidationDefault(t *testing.T) {
	jsonStr := `{"rpki": {"cache-server": {"10.0.0.1": {}}}}`

	cfg, err := parseRPKIConfig(jsonStr)
	require.NoError(t, err)
	assert.False(t, cfg.ASPAValidation)
}

func TestParseRPKIConfigASPAValidationEnabled(t *testing.T) {
	jsonStr := `{"rpki": {"aspa": {"validation": "true"}, "cache-server": {"10.0.0.1": {}}}}`

	cfg, err := parseRPKIConfig(jsonStr)
	require.NoError(t, err)
	assert.True(t, cfg.ASPAValidation)
}

func TestParseRPKIConfigASPAValidationDisabled(t *testing.T) {
	jsonStr := `{"rpki": {"aspa": {"validation": "false"}, "cache-server": {"10.0.0.1": {}}}}`

	cfg, err := parseRPKIConfig(jsonStr)
	require.NoError(t, err)
	assert.False(t, cfg.ASPAValidation)
}

func TestParseRPKIConfigASPAPolicy(t *testing.T) {
	jsonStr := `{"rpki": {
		"aspa": {
			"validation": "true",
			"policy": {
				"invalid-action": "reject",
				"unknown-action": "reject"
			}
		},
		"cache-server": {"10.0.0.1": {}}
	}}`

	cfg, err := parseRPKIConfig(jsonStr)
	require.NoError(t, err)
	assert.Equal(t, ASPAPolicyReject, cfg.ASPAInvalidAction)
	assert.Equal(t, ASPAPolicyReject, cfg.ASPAUnknownAction)
}

func TestParseRPKIConfigASPAPolicyDefaults(t *testing.T) {
	jsonStr := `{"rpki": {"cache-server": {"10.0.0.1": {}}}}`

	cfg, err := parseRPKIConfig(jsonStr)
	require.NoError(t, err)
	assert.Equal(t, ASPAPolicyLogOnly, cfg.ASPAInvalidAction)
	assert.Equal(t, ASPAPolicyAccept, cfg.ASPAUnknownAction)
}

func TestParseRPKIConfigASPAPolicyPartial(t *testing.T) {
	jsonStr := `{"rpki": {
		"aspa": {"policy": {"invalid-action": "accept"}},
		"cache-server": {"10.0.0.1": {}}
	}}`

	cfg, err := parseRPKIConfig(jsonStr)
	require.NoError(t, err)
	assert.Equal(t, ASPAPolicyAccept, cfg.ASPAInvalidAction)
	assert.Equal(t, ASPAPolicyAccept, cfg.ASPAUnknownAction) // default preserved
}
