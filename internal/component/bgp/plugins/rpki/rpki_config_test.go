package rpki

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
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
	sess := newRTRSession("192.0.2.200", 3323, 100, "198.51.100.1", newROACache(), newASPACache(), stopCh)
	require.Equal(t, "198.51.100.1", sess.sourceAddress)

	sessNoSrc := newRTRSession("192.0.2.201", 3324, 100, "", newROACache(), newASPACache(), stopCh)
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
	// RFC requirement: RFC8210-9-1 positive -- RFC 8210 Section 9 keeps port 323 unprotected TCP as the
	// mandatory-to-implement transport for v1, and that is what a default-configured cache server uses.
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
	set, ok := cfg.PeerActions[configjson.PeerConfigKey{ID: "192.0.2.1"}]
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
	set, ok := cfg.PeerActions[configjson.PeerConfigKey{ID: "198.51.100.2"}]
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
	set := cfg.PeerActions[configjson.PeerConfigKey{ID: "198.51.100.2"}]
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
	set := cfg.PeerActions[configjson.PeerConfigKey{ID: "203.0.113.5"}]
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
	set := cfg.PeerActions[configjson.PeerConfigKey{ID: "203.0.113.9"}]
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

// TestParseRPKIConfig_NamedPeerWithNoStaticIPSkipped verifies a NAMED peer with no
// static remote IP cannot be keyed and is skipped, so its override is not recorded.
// The reactor never builds such a peer from the group's template -- it is named, so
// it has no address any consumer produces -- and an entry under a key no reader can
// make would read as in force and do nothing.
//
// The group around it states no action of its own here, so nothing else is keyed:
// the assertion is about the named peer alone.
func TestParseRPKIConfig_NamedPeerWithNoStaticIPSkipped(t *testing.T) {
	cfg, err := parseRPKIConfig(`{
		"rpki": {"cache-server": {"10.0.0.1": {}}},
		"group": {"dyn": {
			"connection": {"remote": {"ip": "dynamic"}},
			"peer": {"d": {"rpki": {"action": {"invalid": "accept"}}}}
		}}
	}`)
	require.NoError(t, err)
	assert.Nil(t, cfg.PeerActions, "a named peer with no static IP is not keyed")
}

// TestParseRPKIConfig_KeysWhatARuntimeReaderProduces verifies the per-peer map is
// keyed by the string buildDecisions holds at decision time, which is an address
// rendered by netip.Addr.String().
//
// Two spellings reach the same key. A peer whose NAME is its address and which
// states no connection > remote > ip is keyed by that name, because the name is
// what a reader produces for it. A peer whose address the operator wrote in a
// non-canonical form is keyed canonically, because the reader never sees the
// operator's spelling.
//
// The first subtest drives parseRPKIConfig directly, and the config it passes is
// one the LOADER refuses: config.validatePeerName (bgp/config/resolve.go) rejects
// a peer name that netip.ParseAddr accepts. So it pins configjson.PeerKey's
// name-as-address branch as a unit contract, and it is NOT evidence that an
// operator can name a peer by its address. Read the second subtest for the
// spelling case that a real configuration reaches.
func TestParseRPKIConfig_KeysWhatARuntimeReaderProduces(t *testing.T) {
	t.Run("a peer named by its own address, a shape the loader refuses", func(t *testing.T) {
		cfg, err := parseRPKIConfig(`{
			"rpki": {"cache-server": {"10.0.0.1": {}}},
			"peer": {"203.0.113.4": {"rpki": {"action": {"invalid": "accept"}}}}
		}`)
		require.NoError(t, err)
		set, ok := cfg.PeerActions[configjson.PeerConfigKey{ID: "203.0.113.4"}]
		require.True(t, ok, "the peer is not keyed, got %v", cfg.PeerActions)
		assert.Equal(t, ASPAPolicyAccept, set.OriginInvalid.Action)
	})

	t.Run("a non-canonical address is keyed canonically", func(t *testing.T) {
		cfg, err := parseRPKIConfig(`{
			"rpki": {"cache-server": {"10.0.0.1": {}}},
			"peer": {"v6": {
				"connection": {"remote": {"ip": "2001:0DB8::1"}},
				"rpki": {"action": {"invalid": "accept"}}
			}}
		}`)
		require.NoError(t, err)
		_, ok := cfg.PeerActions[configjson.PeerConfigKey{ID: "2001:db8::1"}]
		require.True(t, ok, "the peer is keyed by the operator's spelling, got %v", cfg.PeerActions)
	})
}

// TestRPKIPeerActionsForDynamicGroup verifies AC-6: a listen-range group's rpki
// action is keyed under the GROUP, which is the one identity a peer built from
// that template shares with the config document.
//
// Before this, parsePeerActions returned early for the template visit ("no static
// remote ip"), so an operator's action on the object an IXP actually configures
// reached nobody and every member fell back to the global actions.
func TestRPKIPeerActionsForDynamicGroup(t *testing.T) {
	t.Run("the group's action is keyed under the group", func(t *testing.T) {
		cfg, err := parseRPKIConfig(`{
			"rpki": {"cache-server": {"10.0.0.1": {}}},
			"group": {"ix": {
				"connection": {"remote": {"ip": "dynamic", "range": ["192.0.2.0/24"]}},
				"rpki": {"action": {"invalid": "accept"}}
			}}
		}`)
		require.NoError(t, err)
		set, ok := cfg.PeerActions[configjson.GroupKey("ix")]
		require.True(t, ok, "the template is not keyed, got %v", cfg.PeerActions)
		assert.Equal(t, ASPAPolicyAccept, set.OriginInvalid.Action)
		assert.Equal(t, sourceGroup, set.OriginInvalid.Source)
	})

	t.Run("a named member keeps its own entry and its own leaf wins", func(t *testing.T) {
		cfg, err := parseRPKIConfig(`{
			"rpki": {"cache-server": {"10.0.0.1": {}}},
			"group": {"ix": {
				"connection": {"remote": {"ip": "dynamic", "range": ["192.0.2.0/24"]}},
				"rpki": {"action": {"invalid": "reject"}},
				"peer": {"member": {
					"connection": {"remote": {"ip": "192.0.2.9"}},
					"rpki": {"action": {"invalid": "accept"}}
				}}
			}}
		}`)
		require.NoError(t, err)
		peer, ok := cfg.PeerActions[configjson.PeerConfigKey{ID: "192.0.2.9"}]
		require.True(t, ok, "the named peer lost its entry, got %v", cfg.PeerActions)
		assert.Equal(t, ASPAPolicyAccept, peer.OriginInvalid.Action, "the group's action beat the peer's")
		assert.Equal(t, sourcePeer, peer.OriginInvalid.Source)

		tmpl, ok := cfg.PeerActions[configjson.GroupKey("ix")]
		require.True(t, ok, "the template stopped being keyed once the group listed a peer")
		assert.Equal(t, ASPAPolicyReject, tmpl.OriginInvalid.Action)
	})

	t.Run("the blackhole agreement resolves for the template too", func(t *testing.T) {
		cfg, err := parseRPKIConfig(`{
			"rpki": {"cache-server": {"10.0.0.1": {}}},
			"group": {"ix": {
				"connection": {"remote": {"ip": "dynamic", "range": ["192.0.2.0/24"]}},
				"rpki": {"blackhole-exempt": "true"},
				"blackhole": {"communities": ["65001:666"]}
			}}
		}`)
		require.NoError(t, err)
		set, ok := cfg.PeerActions[configjson.GroupKey("ix")]
		require.True(t, ok, "the template is not keyed, got %v", cfg.PeerActions)
		assert.True(t, set.BlackholeExempt)
		assert.Equal(t, []attribute.Community{attribute.Community(65001<<16 | 666)}, set.BlackholeCommunities,
			"the RFC 7999 Section 3.3 agreement was read from a key the template does not hold")
	})
}

// TestRPKIDynamicGroupActionsMatchAStaticPeer verifies AC-8: one config states the
// same rpki leaves on a static peer and on a dynamic group, and both resolve to the
// same effective action set. The template must diverge from a static peer nowhere it
// was not told to.
//
// Source is normalized away before the comparison: it names the config LEVEL a leaf
// came from, which is peer for one and group for the other by construction, and it
// is display metadata rather than the enforced value.
func TestRPKIDynamicGroupActionsMatchAStaticPeer(t *testing.T) {
	cfg, err := parseRPKIConfig(`{
		"rpki": {"cache-server": {"10.0.0.1": {}}},
		"peer": {"static": {
			"connection": {"remote": {"ip": "198.51.100.1"}},
			"rpki": {
				"action": {"invalid": "accept", "not-found": "reject"},
				"aspa": {"action": {"invalid": "reject", "unknown": "log-only"}},
				"blackhole-exempt": "true"
			},
			"blackhole": {"communities": ["65001:666"]}
		}},
		"group": {"ix": {
			"connection": {"remote": {"ip": "dynamic", "range": ["192.0.2.0/24"]}},
			"rpki": {
				"action": {"invalid": "accept", "not-found": "reject"},
				"aspa": {"action": {"invalid": "reject", "unknown": "log-only"}},
				"blackhole-exempt": "true"
			},
			"blackhole": {"communities": ["65001:666"]}
		}}
	}`)
	require.NoError(t, err)

	effective := func(s peerActionSet) peerActionSet {
		s.OriginInvalid.Source = sourceGlobal
		s.OriginNotFound.Source = sourceGlobal
		s.ASPAInvalid.Source = sourceGlobal
		s.ASPAUnknown.Source = sourceGlobal
		return s
	}

	peer, ok := cfg.PeerActions[configjson.PeerConfigKey{ID: "198.51.100.1"}]
	require.True(t, ok, "the static peer is not keyed, got %v", cfg.PeerActions)
	tmpl, ok := cfg.PeerActions[configjson.GroupKey("ix")]
	require.True(t, ok, "the dynamic group's template is not keyed, got %v", cfg.PeerActions)

	assert.Equal(t, effective(peer), effective(tmpl),
		"the template resolved a different action set than the static peer stating the same leaves")
}
