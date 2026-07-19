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
	// RFC requirement: RFC6810-7-1 positive -- a cache server configured with no explicit port
	// defaults to the rpki-rtr port 323, the unprotected-TCP transport ze dials (rtr_session.go
	// DialContext "tcp"). This is the mandatory-to-implement transport for an RTR router.
	assert.Equal(t, uint16(323), cs.Port)             // RTR default
	assert.Equal(t, uint8(100), cs.Preference)        // YANG default
	assert.Equal(t, uint16(0), cfg.ValidationTimeout) // not set

	// RFC 6811 origin-validation policy defaults (the YANG leaf defaults): the exclusion of
	// Invalid routes defaults to reject, NotFound defaults to accept.
	assert.Equal(t, ASPAPolicyReject, cfg.OriginInvalidAction, "default invalid-action is reject")
	assert.Equal(t, ASPAPolicyAccept, cfg.OriginNotFoundAction, "default not-found-action is accept")
}

// TestParseRPKIConfigOriginAction verifies the rpki/action container (RFC 6811 origin-validation
// action) is parsed under the new `action { invalid; not-found }` syntax.
func TestParseRPKIConfigOriginAction(t *testing.T) {
	cfg, err := parseRPKIConfig(`{
		"rpki": {
			"action": {"invalid": "accept", "not-found": "reject"},
			"cache-server": {"10.0.0.1": {}}
		}
	}`)
	require.NoError(t, err)
	assert.Equal(t, ASPAPolicyAccept, cfg.OriginInvalidAction, "action/invalid=accept is parsed")
	assert.Equal(t, ASPAPolicyReject, cfg.OriginNotFoundAction, "action/not-found=reject is parsed")
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

func TestParseRPKIConfigASPAAction(t *testing.T) {
	jsonStr := `{"rpki": {
		"aspa": {
			"validation": "true",
			"action": {
				"invalid": "reject",
				"unknown": "reject"
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

func TestParseRPKIConfigASPAActionPartial(t *testing.T) {
	jsonStr := `{"rpki": {
		"aspa": {"action": {"invalid": "accept"}},
		"cache-server": {"10.0.0.1": {}}
	}}`

	cfg, err := parseRPKIConfig(jsonStr)
	require.NoError(t, err)
	assert.Equal(t, ASPAPolicyAccept, cfg.ASPAInvalidAction)
	assert.Equal(t, ASPAPolicyAccept, cfg.ASPAUnknownAction) // default preserved
}

// TestParseRPKIConfig_PerPeerOverride verifies a per-peer action override is parsed and keyed
// by the peer's remote IP (connection>remote>ip), with unset leaves falling back to global (AC-2/5).
func TestParseRPKIConfig_PerPeerOverride(t *testing.T) {
	cfg, err := parseRPKIConfig(`{
		"rpki": {"cache-server": {"10.0.0.1": {}}},
		"peer": {
			"customer-a": {
				"connection": {"remote": {"ip": "192.0.2.1"}},
				"rpki": {"action": {"invalid": "accept"}}
			}
		}
	}`)
	require.NoError(t, err)
	require.NotNil(t, cfg.PeerActions)
	set, ok := cfg.PeerActions["192.0.2.1"]
	require.True(t, ok, "per-peer map is keyed by remote IP, not peer name")
	assert.Equal(t, ASPAPolicyAccept, set.OriginInvalid.Action)
	assert.Equal(t, sourcePeer, set.OriginInvalid.Source)
	// Unset not-found falls through to the global default (accept).
	assert.Equal(t, ASPAPolicyAccept, set.OriginNotFound.Action)
	assert.Equal(t, sourceGlobal, set.OriginNotFound.Source)
}

// TestParseRPKIConfig_GroupInheritance verifies a group-level action applies to a member peer
// that sets nothing of its own (AC-3).
func TestParseRPKIConfig_GroupInheritance(t *testing.T) {
	cfg, err := parseRPKIConfig(`{
		"rpki": {"cache-server": {"10.0.0.1": {}}},
		"group": {
			"transit": {
				"rpki": {"action": {"invalid": "reject"}},
				"peer": {"member": {"connection": {"remote": {"ip": "198.51.100.2"}}}}
			}
		}
	}`)
	require.NoError(t, err)
	set, ok := cfg.PeerActions["198.51.100.2"]
	require.True(t, ok)
	assert.Equal(t, ASPAPolicyReject, set.OriginInvalid.Action)
	assert.Equal(t, sourceGroup, set.OriginInvalid.Source)
}

// TestParseRPKIConfig_PeerBeatsGroup verifies peer overrides win over group (AC-4).
func TestParseRPKIConfig_PeerBeatsGroup(t *testing.T) {
	cfg, err := parseRPKIConfig(`{
		"rpki": {"cache-server": {"10.0.0.1": {}}},
		"group": {
			"transit": {
				"rpki": {"action": {"invalid": "reject"}},
				"peer": {"member": {
					"connection": {"remote": {"ip": "198.51.100.2"}},
					"rpki": {"action": {"invalid": "accept"}}
				}}
			}
		}
	}`)
	require.NoError(t, err)
	set := cfg.PeerActions["198.51.100.2"]
	assert.Equal(t, ASPAPolicyAccept, set.OriginInvalid.Action)
	assert.Equal(t, sourcePeer, set.OriginInvalid.Source)
}

// TestParseRPKIConfig_PerLeafFallback verifies unset leaves fall through per-leaf: a peer that
// overrides only invalid keeps the global not-found (AC-5).
func TestParseRPKIConfig_PerLeafFallback(t *testing.T) {
	cfg, err := parseRPKIConfig(`{
		"rpki": {"action": {"not-found": "reject"}, "cache-server": {"10.0.0.1": {}}},
		"peer": {"p": {
			"connection": {"remote": {"ip": "203.0.113.5"}},
			"rpki": {"action": {"invalid": "accept"}}
		}}
	}`)
	require.NoError(t, err)
	set := cfg.PeerActions["203.0.113.5"]
	assert.Equal(t, ASPAPolicyAccept, set.OriginInvalid.Action)
	assert.Equal(t, sourcePeer, set.OriginInvalid.Source)
	// not-found inherits the configured GLOBAL value (reject), not the YANG default.
	assert.Equal(t, ASPAPolicyReject, set.OriginNotFound.Action)
	assert.Equal(t, sourceGlobal, set.OriginNotFound.Source)
}

// TestParseRPKIConfig_ASPAPerPeer verifies a per-peer ASPA action override (AC-6).
func TestParseRPKIConfig_ASPAPerPeer(t *testing.T) {
	cfg, err := parseRPKIConfig(`{
		"rpki": {"aspa": {"validation": "true"}, "cache-server": {"10.0.0.1": {}}},
		"peer": {"p": {
			"connection": {"remote": {"ip": "203.0.113.9"}},
			"rpki": {"aspa": {"action": {"invalid": "accept"}}}
		}}
	}`)
	require.NoError(t, err)
	set := cfg.PeerActions["203.0.113.9"]
	assert.Equal(t, ASPAPolicyAccept, set.ASPAInvalid.Action)
	assert.Equal(t, sourcePeer, set.ASPAInvalid.Source)
}

// TestParseRPKIConfig_NoOverrideNotInMap verifies a peer with no override is absent from the
// per-peer map (falls back to global at decision time), keeping the map minimal.
func TestParseRPKIConfig_NoOverrideNotInMap(t *testing.T) {
	cfg, err := parseRPKIConfig(`{
		"rpki": {"cache-server": {"10.0.0.1": {}}},
		"peer": {"p": {"connection": {"remote": {"ip": "203.0.113.1"}}}}
	}`)
	require.NoError(t, err)
	assert.Nil(t, cfg.PeerActions, "no overrides -> nil per-peer map")
}

// TestParseRPKIConfig_DynamicPeerSkipped verifies a peer without a static remote IP cannot be
// keyed and is skipped (documented Known Limitation), so its override is not recorded.
func TestParseRPKIConfig_DynamicPeerSkipped(t *testing.T) {
	cfg, err := parseRPKIConfig(`{
		"rpki": {"cache-server": {"10.0.0.1": {}}},
		"group": {"dyn": {
			"connection": {"remote": {"ip": "dynamic"}},
			"rpki": {"action": {"invalid": "accept"}},
			"peer": {"d": {"rpki": {"action": {"invalid": "accept"}}}}
		}}
	}`)
	require.NoError(t, err)
	assert.Nil(t, cfg.PeerActions, "dynamic peer without static IP is not keyed")
}
