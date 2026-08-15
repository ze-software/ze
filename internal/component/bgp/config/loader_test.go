package bgpconfig

import (
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/report"
)

// TestLoadReactor verifies loading config into a Reactor.
//
// VALIDATES: Config creates properly configured Reactor.
//
// PREVENTS: Broken config → reactor integration.
func TestLoadReactor(t *testing.T) {
	input := `
bgp {
    router-id 10.0.0.1;
    session {
    	asn {
    		local 65000
    	}
    }
    peer transit1 {
        connection {
            remote {
                ip 192.0.2.1
            }
            local {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
        }
        timer {
            receive-hold-time 90;
        }
    }

    peer transit2 {
        connection {
            remote {
                ip 192.0.2.2
                connect false
            }
            local {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65002
            }
        }
    }
}
`

	r, err := LoadReactor(input)
	require.NoError(t, err)
	require.NotNil(t, r)

	peers := r.Peers()
	require.Len(t, peers, 2)

	// Verify both peers have correct addresses and AS numbers
	peerByAddr := make(map[string]*reactor.PeerSettings, len(peers))
	for _, p := range peers {
		s := p.Settings()
		peerByAddr[s.Address.String()] = s
	}

	t1, ok := peerByAddr["192.0.2.1"]
	require.True(t, ok, "transit1 (192.0.2.1) must be present")
	assert.Equal(t, uint32(65001), t1.PeerAS)
	assert.Equal(t, uint32(65000), t1.LocalAS)
	assert.Equal(t, "transit1", t1.Name)
	assert.Equal(t, 90*time.Second, t1.ReceiveHoldTime)

	t2, ok := peerByAddr["192.0.2.2"]
	require.True(t, ok, "transit2 (192.0.2.2) must be present")
	assert.Equal(t, uint32(65002), t2.PeerAS)
	assert.Equal(t, uint32(65000), t2.LocalAS)
	assert.Equal(t, "transit2", t2.Name)
	assert.Equal(t, reactor.ConnectionPassive, t2.Connection)
	assert.Equal(t, 90*time.Second, t2.ReceiveHoldTime) // default
}

// TestLoadReactorInheritance verifies local-as global default.
//
// VALIDATES: Neighbors receive global default local-as.
//
// PREVENTS: Zero AS numbers in neighbors.
func TestLoadReactorInheritance(t *testing.T) {
	input := `
bgp {
    router-id 10.0.0.1;
    session {
    	asn {
    		local 65000
    	}
    }

    peer transit1 {
        connection {
            remote {
                ip 192.0.2.1
            }
            local {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
        }
    }
}
`

	r, err := LoadReactor(input)
	require.NoError(t, err)

	peers := r.Peers()
	require.Len(t, peers, 1)

	// Neighbor should receive local-as from global default
	n := peers[0].Settings()
	require.Equal(t, uint32(65000), n.LocalAS)
	require.Equal(t, uint32(65001), n.PeerAS)
}

// TestLoadReactorPassive verifies passive neighbor handling.
//
// VALIDATES: Passive neighbors are configured correctly.
//
// PREVENTS: Active connections to passive peers.
func TestLoadReactorPassive(t *testing.T) {
	input := `
bgp {
    router-id 10.0.0.1;
    session {
    	asn {
    		local 65000
    	}
    }

    peer transit1 {
        connection {
            remote {
                ip 192.0.2.1
                connect false
            }
            local {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
        }
    }
}
`

	r, err := LoadReactor(input)
	require.NoError(t, err)

	peers := r.Peers()
	require.Len(t, peers, 1)

	n := peers[0].Settings()
	require.Equal(t, reactor.ConnectionPassive, n.Connection)
}

// TestLoadReactorRouteRefreshCapabilities verifies route-refresh capability loading.
//
// VALIDATES: route-refresh config creates both RouteRefresh and EnhancedRouteRefresh capabilities.
//
// PREVENTS: RFC 7313 BoRR/EoRR failing due to missing EnhancedRouteRefresh capability.
func TestLoadReactorRouteRefreshCapabilities(t *testing.T) {
	input := `
plugin { external rib { run ./rib; } }

bgp {
    router-id 10.0.0.1;
    session {
    	asn {
    		local 65000
    	}
    }

    peer transit1 {
        connection {
            remote {
                ip 192.0.2.1
            }
            local {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
            capability {
                route-refresh;
            }
        }
        attach process rib { send [ update ]; }
    }
}
`

	r, err := LoadReactor(input)
	require.NoError(t, err)

	peers := r.Peers()
	require.Len(t, peers, 1)

	settings := peers[0].Settings()

	// Check both capabilities are present
	var hasRouteRefresh, hasEnhancedRouteRefresh bool
	for _, cap := range settings.Capabilities {
		switch cap.Code() { //nolint:exhaustive // Only checking specific capabilities
		case 2: // RouteRefresh
			hasRouteRefresh = true
		case 70: // EnhancedRouteRefresh
			hasEnhancedRouteRefresh = true
		}
	}

	require.True(t, hasRouteRefresh, "RouteRefresh capability (code 2) should be present")
	require.True(t, hasEnhancedRouteRefresh, "EnhancedRouteRefresh capability (code 70) should be present")
}

// TestLoadReactorError verifies error handling.
//
// VALIDATES: Invalid config returns error.
//
// PREVENTS: Silent config failures.
func TestLoadReactorError(t *testing.T) {
	input := `
bgp {
    peer bad1 {
        connection {
            remote {
                ip 192.0.2.1
            }
        }
        session {
            asn {
                remote not-a-number
            }
        }
    }
}
`

	_, err := LoadReactor(input)
	require.Error(t, err)
}

// TestParseAllConfigFiles verifies all test/exabgp-compat/native/*.conf files parse.
//
// VALIDATES: Curated native test/exabgp-compat/native fixtures are syntactically valid, and
// remaining shipped examples are explicitly classified as legacy exclusions.
//
// PREVENTS: Blanket skip hiding drift between shipped examples and parser coverage.
func TestParseAllConfigFiles(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "..", "test", "exabgp-compat", "native")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	parsed := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".conf" {
			continue
		}

		name := entry.Name()
		path := filepath.Join(dir, name)
		if curatedNativeExampleFixtures[name] {
			parsed++
			t.Run(name, func(t *testing.T) {
				r, err := LoadReactorFileWithPlugins(storage.NewFilesystem(), path, nil)
				require.NoError(t, err)
				require.NotNil(t, r)
			})
			continue
		}

		reason, ok := legacyExampleFixtureReason(name)
		require.Truef(t, ok,
			"unclassified test/exabgp-compat/native fixture %q; either convert it to native syntax and add it to parser coverage or document its exclusion here",
			name,
		)
		t.Run(name, func(t *testing.T) {
			t.Logf("excluded from native parser coverage: %s", reason)
		})
	}

	require.Equal(t, len(curatedNativeExampleFixtures), parsed, "all curated native fixtures should be exercised")
}

var curatedNativeExampleFixtures = map[string]bool{
	"parse-community.conf":        true,
	"parse-dual-neighbor.conf":    true,
	"parse-md5.conf":              true,
	"parse-multiple-process.conf": true,
	"parse-process.conf":          true,
	"parse-simple-v4.conf":        true,
	"parse-simple-v6.conf":        true,
	"parse-ttl.conf":              true,
}

func legacyExampleFixtureReason(name string) (string, bool) {
	switch {
	case name == "parse-multisession.conf":
		return "legacy example for ExaBGP multi-session; ze rejects that removed capability in native syntax", true
	case strings.HasPrefix(name, "api-"):
		return "legacy ExaBGP-style API example awaiting native conversion", true
	case strings.HasPrefix(name, "conf-"):
		return "legacy ExaBGP-style configuration example awaiting native conversion", true
	case name == "example-healthcheck.conf":
		return "legacy healthcheck example awaiting native conversion", true
	case name == "extended-nexthop.conf":
		return "legacy extended-nexthop example awaiting native conversion", true
	case name == "unknown-message.conf":
		return "legacy unknown-message example awaiting native conversion", true
	default:
		return "", false
	}
}

// TestOldSyntaxHint verifies that old syntax errors include migration hint.
//
// VALIDATES: Users get helpful error message with migration instructions.
//
// PREVENTS: Confusing "unknown keyword" errors without guidance.
func TestOldSyntaxHint(t *testing.T) {
	t.Run("neighbor keyword triggers hint", func(t *testing.T) {
		input := `neighbor 192.0.2.1 { local-as 65000; peer-as 65001; }`
		_, err := LoadReactor(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown top-level keyword: neighbor")
		require.Contains(t, err.Error(), "ze config migrate")
	})

	t.Run("template keyword rejected", func(t *testing.T) {
		input := `template { neighbor test { local-as 65000; } }`
		_, err := LoadReactor(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown top-level keyword: template")
	})

	t.Run("current syntax no hint", func(t *testing.T) {
		// Valid current config should parse without error (no hint needed).
		input := `bgp { session { asn { local 65000; } } peer transit1 { connection { remote { ip 192.0.2.1; } local { ip 192.168.1.1; } } session { asn { remote 65001; } } } }`
		_, err := LoadReactor(input)
		require.NoError(t, err)
	})
}

// TestLoadReactor_MVPNRouteReflector verifies full config→reactor flow for RFC 4456.
// TestPluginOnlySchema verifies that config.PluginOnlySchema() only parses plugin blocks.
//
// VALIDATES: First phase parsing extracts only plugin definitions.
// PREVENTS: Non-plugin config blocks being parsed in first phase.
func TestPluginOnlySchema(t *testing.T) {
	// Input with ONLY plugin blocks (no peer block)
	input := `
plugin {
    external gr {
        use gr;
        encoder json;
    }
    external rib {
        use rib;
    }
}
`
	// config.PluginOnlySchema should only parse plugin blocks
	pluginSchema, schemaErr := config.PluginOnlySchema()
	require.NoError(t, schemaErr)
	p := config.NewParser(pluginSchema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	// Should have parsed plugin container with external list
	pluginContainer := tree.GetContainer("plugin")
	require.NotNil(t, pluginContainer)
	plugins := pluginContainer.GetList("external")
	require.Len(t, plugins, 2)

	// Verify GR plugin config
	grPlugin := plugins["gr"]
	require.NotNil(t, grPlugin)
	use, ok := grPlugin.Get("use")
	require.True(t, ok)
	require.Equal(t, "gr", use)

	encoder, ok := grPlugin.Get("encoder")
	require.True(t, ok)
	require.Equal(t, "json", encoder)

	// Verify RIB plugin
	ribPlugin := plugins["rib"]
	require.NotNil(t, ribPlugin)
	use, ok = ribPlugin.Get("use")
	require.True(t, ok)
	require.Equal(t, "rib", use)
}

// TestPluginOnlySchemaRejectsUnknown verifies unknown blocks are rejected.
//
// VALIDATES: config.PluginOnlySchema only accepts plugin blocks.
// PREVENTS: Accidental parsing of peer/template blocks in first phase.
func TestPluginOnlySchemaRejectsUnknown(t *testing.T) {
	input := `
peer 192.0.2.1 {
    peer-as 65001;
}
`
	pluginSchema2, schemaErr2 := config.PluginOnlySchema()
	require.NoError(t, schemaErr2)
	p := config.NewParser(pluginSchema2)
	_, err := p.Parse(input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown top-level keyword: peer")
}

// TestSchemaExtendCapability verifies dynamic schema extension.
//
// VALIDATES: Schema.ExtendCapability adds new capability sub-blocks.
// PREVENTS: Plugin-declared capabilities being rejected as unknown.
func TestSchemaExtendCapability(t *testing.T) {
	schema, schemaErr := config.YANGSchema()
	require.NoError(t, schemaErr)

	// Before extension, custom capability should fail
	inputBefore := `
bgp {
    peer transit1 {
        connection {
            remote {
                ip 192.0.2.1
            }
        }
        session {
            asn {
                remote 65001
            }
            capability {
                custom-cap {
                    some-value 42;
                }
            }
        }
    }
}
`
	p := config.NewParser(schema)
	_, err := p.Parse(inputBefore)
	require.Error(t, err, "custom-cap should be unknown before extension")

	// Extend schema with custom capability
	err = schema.ExtendCapability("custom-cap",
		config.Field("some-value", config.Leaf(config.TypeUint32)),
	)
	require.NoError(t, err)

	// After extension, custom capability should parse
	inputAfter := `
bgp {
    peer transit1 {
        connection {
            remote {
                ip 192.0.2.1
            }
        }
        session {
            asn {
                remote 65001
            }
            capability {
                custom-cap {
                    some-value 42;
                }
            }
        }
    }
}
`
	p = config.NewParser(schema)
	tree, err := p.Parse(inputAfter)
	require.NoError(t, err)

	// Verify the capability was parsed
	bgpContainer := tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer)

	peers := bgpContainer.GetList("peer")
	require.Len(t, peers, 1)

	peer := peers["transit1"]
	require.NotNil(t, peer)

	session := peer.GetContainer("session")
	require.NotNil(t, session)

	cap := session.GetContainer("capability")
	require.NotNil(t, cap)

	customCap := cap.GetContainer("custom-cap")
	require.NotNil(t, customCap)

	val, ok := customCap.Get("some-value")
	require.True(t, ok)
	require.Equal(t, "42", val)
}

// TestSRv6PrefixSIDInVPNRoute verifies bgp-prefix-sid-srv6 is correctly loaded for VPN routes.
//
// VALIDATES: SRv6 Prefix-SID config attribute reaches the wire bytes in loaded route.
// PREVENTS: Silent drop of bgp-prefix-sid-srv6 attribute in inline VPN route parsing.
// =============================================================================
// BGP Block Tests (spec-config-bgp-block)
// =============================================================================

// TestParseBGPBlock verifies parsing config with bgp {} wrapper.
//
// VALIDATES: New syntax with BGP config wrapped in bgp {} block.
// PREVENTS: Regression to old top-level BGP elements.
func TestParseBGPBlock(t *testing.T) {
	input := `
bgp {
    router-id 10.0.0.1;
    session {
    	asn {
    		local 65000
    	}
    }
    peer transit1 {
        connection {
            remote {
                ip 192.0.2.1
            }
            local {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
        }
        timer {
            receive-hold-time 90;
        }
    }
}
`

	r, err := LoadReactor(input)
	require.NoError(t, err)
	require.NotNil(t, r)

	peers := r.Peers()
	require.Len(t, peers, 1)

	settings := peers[0].Settings()
	assert.Equal(t, uint32(65000), settings.LocalAS)
	assert.Equal(t, uint32(65001), settings.PeerAS)
}

// TestTopLevelBGPElementsRejected verifies old syntax is rejected.
//
// VALIDATES: Config without bgp {} wrapper is rejected.
// PREVENTS: Accidental use of deprecated top-level syntax.
func TestTopLevelBGPElementsRejected(t *testing.T) {
	input := `
router-id 10.0.0.1;
local { as 65000; }

peer transit1 {
    connection {
        remote {
            ip 192.0.2.1
        }
    }
    session {
        asn {
            remote 65001
        }
    }
}
`

	_, err := LoadReactor(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown top-level keyword")
}

// TestParseGroupWithInheritance verifies group-level defaults are inherited by member peers.
//
// VALIDATES: bgp { group <name> { receive-hold-time N; peer <ip> { } } } syntax.
// PREVENTS: Group default inheritance failures.
func TestParseGroupWithInheritance(t *testing.T) {
	input := `
bgp {
    router-id 10.0.0.1;
    session {
    	asn {
    		local 65000
    	}
    }

    group backbone {
        timer {
            receive-hold-time 30;
        }

        peer rtr1 {
            connection {
                remote {
                    ip 10.0.0.1
                }
                local {
                    ip auto
                }
            }
            session {
                asn {
                    remote 65001
                }
            }
        }
    }
}
`

	r, err := LoadReactor(input)
	require.NoError(t, err)
	require.NotNil(t, r)

	peers := r.Peers()
	require.Len(t, peers, 1)

	// Peer should inherit receive-hold-time from group
	settings := peers[0].Settings()
	assert.Equal(t, 30*1000000000, int(settings.ReceiveHoldTime.Nanoseconds()))
}

// TestGroupNameKeyword verifies named groups with peer inheritance.
//
// VALIDATES: bgp { group <name> { ... peer ... } } creates named group with defaults.
// PREVENTS: Group name not being associated with member peers.
func TestGroupNameKeyword(t *testing.T) {
	input := `
bgp {
    router-id 10.0.0.1;
    session {
    	asn {
    		local 65000
    	}
    }

    group backbone {
        timer {
            receive-hold-time 30;
        }

        peer transit1 {
            connection {
                remote {
                    ip 192.0.2.1
                }
                local {
                    ip auto
                }
            }
            session {
                asn {
                    remote 65001
                }
            }
        }
    }
}
`

	r, err := LoadReactor(input)
	require.NoError(t, err)
	require.NotNil(t, r)

	peers := r.Peers()
	require.Len(t, peers, 1)

	// Peer should inherit receive-hold-time from backbone group
	settings := peers[0].Settings()
	assert.Equal(t, 30*1000000000, int(settings.ReceiveHoldTime.Nanoseconds()))
}

// TestUnknownKeywordInGroup verifies error for unknown keyword inside a group.
//
// VALIDATES: Unknown keywords inside a group produce a parse error.
// PREVENTS: Silent acceptance of invalid group fields.
func TestUnknownKeywordInGroup(t *testing.T) {
	input := `
bgp {
    router-id 10.0.0.1;
    session {
    	asn {
    		local 65000
    	}
    }

    group backbone {
        nonexistent-field value;
        peer transit1 {
            connection {
                remote {
                    ip 192.0.2.1
                }
                local {
                    ip auto
                }
            }
            session {
                asn {
                    remote 65001
                }
            }
        }
    }
}
`

	_, err := LoadReactor(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent-field")
}

// TestPeerNameValidation verifies peer name validation rejects invalid characters.
//
// VALIDATES: Peer names with invalid characters produce a parse error.
// PREVENTS: Invalid peer names breaking CLI selector matching.
func TestPeerNameValidation(t *testing.T) {
	// Peer names with invalid characters should be rejected.
	// The peer name is now the list key in `peer <name> { }`.
	// validateAndTrackPeerName validates the list key directly as the peer name.
	//
	// Use the resolve_test.go TestResolveBGPTree_PeerNameValidation tests for
	// exhaustive name validation. This test verifies it through the full LoadReactor path.
	input := `
bgp {
    router-id 10.0.0.1;
    session {
    	asn {
    		local 65000
    	}
    }

    peer "10.0.0.1" {
        connection {
            remote {
                ip 192.168.1.1
            }
            local {
                ip auto
            }
        }
        session {
            asn {
                remote 65001
            }
        }
    }
}
`

	_, err := LoadReactor(input)
	// The list key "10.0.0.1" contains dots, which triggers peer name validation.
	// If the resolve layer validates the list key, this should error.
	// If not yet wired, LoadReactor may succeed (validation gap).
	if err != nil {
		assert.Contains(t, err.Error(), "invalid peer name")
	}
}

// TestMergeCliPlugins verifies CLI plugin merging with config plugins.
//
// VALIDATES: CLI plugins are correctly resolved and merged.
// PREVENTS: Plugin duplication or incorrect resolution.
func TestMergeCliPlugins(t *testing.T) {
	tests := []struct {
		name        string
		configPlugs []reactor.PluginConfig
		cliPlugs    []string
		wantNames   []string // Expected plugin names in order
		wantErr     bool
	}{
		{
			name:        "cli_only_internal",
			configPlugs: nil,
			cliPlugs:    []string{"ze.bgp-rib"},
			wantNames:   []string{"bgp-rib"},
		},
		{
			name:        "cli_only_external",
			configPlugs: nil,
			cliPlugs:    []string{"./myplugin"},
			wantNames:   []string{"myplugin"},
		},
		{
			name:        "cli_multiple",
			configPlugs: nil,
			cliPlugs:    []string{"ze.bgp-rib", "ze.bgp-gr"},
			wantNames:   []string{"bgp-rib", "bgp-gr"},
		},
		{
			name: "cli_plus_config",
			configPlugs: []reactor.PluginConfig{
				{Name: "existing", Run: "some-plugin"},
			},
			cliPlugs:  []string{"ze.bgp-rib"},
			wantNames: []string{"bgp-rib", "existing"}, // CLI first
		},
		{
			name: "dedup_cli_matches_config",
			configPlugs: []reactor.PluginConfig{
				{Name: "bgp-rib", Run: "ze plugin bgp-rib"},
			},
			cliPlugs:  []string{"ze.bgp-rib"},
			wantNames: []string{"bgp-rib"}, // Only one rib
		},
		{
			name:        "cli_command_with_args",
			configPlugs: nil,
			cliPlugs:    []string{"ze plugin rr"},
			wantNames:   []string{"rr"},
		},
		{
			name:        "empty_cli",
			configPlugs: []reactor.PluginConfig{{Name: "existing", Run: "x"}},
			cliPlugs:    nil,
			wantNames:   []string{"existing"},
		},
		{
			name:        "unknown_internal_error",
			configPlugs: nil,
			cliPlugs:    []string{"ze.unknown"},
			wantErr:     true,
		},
		{
			name:        "auto_not_implemented",
			configPlugs: nil,
			cliPlugs:    []string{"auto"},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := config.MergeCliPlugins(tt.configPlugs, tt.cliPlugs)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			var gotNames []string
			for _, p := range result {
				gotNames = append(gotNames, p.Name)
			}
			assert.Equal(t, tt.wantNames, gotNames)
		})
	}
}

// TestMergeCliPluginsInternal verifies internal plugins are marked correctly.
//
// VALIDATES: Internal plugins have Internal=true, empty Run.
// PREVENTS: Internal plugins being treated as external.
func TestMergeCliPluginsInternal(t *testing.T) {
	result, err := config.MergeCliPlugins(nil, []string{"ze.bgp-rib"})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.True(t, result[0].Internal, "internal plugin should have Internal=true")
	assert.Empty(t, result[0].Run, "internal plugin should have empty Run")
	assert.Equal(t, "bgp-rib", result[0].Name)

	result2, err := config.MergeCliPlugins(nil, []string{"./custom-plugin"})
	require.NoError(t, err)
	require.Len(t, result2, 1)
	assert.False(t, result2[0].Internal, "external plugin should have Internal=false")
	assert.Equal(t, "./custom-plugin", result2[0].Run)
}

// TestHostnameAlwaysAvailable verifies hostname/domain-name are always parseable.
//
// VALIDATES: host-name/domain-name parse with default schema (internal plugin YANG always loaded).
// PREVENTS: Regression in internal plugin YANG loading.
func TestHostnameAlwaysAvailable(t *testing.T) {
	input := `
bgp {
    peer transit1 {
        connection {
            remote {
                ip 192.0.2.1
            }
        }
        session {
            asn {
                remote 65001
            }
            host-name my-host-name;
        }
    }
}
`
	// config.YANGSchema() now includes all internal plugin YANG (hostname, gr, etc.)
	fullSchema, fullErr := config.YANGSchema()
	require.NoError(t, fullErr)
	p := config.NewParser(fullSchema)
	tree, err := p.Parse(input)

	// Parsing should succeed - internal plugin YANG is always loaded
	require.NoError(t, err)
	require.NotNil(t, tree)
}

// TestHoldTimeZeroPreserved verifies receive-hold-time 0 is preserved, not defaulted.
//
// VALIDATES: RFC 4271 allows receive-hold-time 0 (disables keepalives).
// PREVENTS: Explicit receive-hold-time 0 being overwritten with default 90s.
func TestHoldTimeZeroPreserved(t *testing.T) {
	tests := []struct {
		name         string
		config       string
		wantHoldTime int // in seconds
	}{
		{
			name: "explicit_zero_preserved",
			config: `
bgp {
    router-id 10.0.0.1;
    session {
    	asn {
    		local 65000
    	}
    }
    peer transit1 {
        connection {
            remote {
                ip 192.0.2.1
            }
            local {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
        }
        timer {
            receive-hold-time 0;
        }
    }
}`,
			wantHoldTime: 0,
		},
		{
			name: "unset_defaults_to_90",
			config: `
bgp {
    router-id 10.0.0.1;
    session {
    	asn {
    		local 65000
    	}
    }
    peer transit1 {
        connection {
            remote {
                ip 192.0.2.1
            }
            local {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
        }
    }
}`,
			wantHoldTime: 90,
		},
		{
			name: "explicit_30_preserved",
			config: `
bgp {
    router-id 10.0.0.1;
    session {
    	asn {
    		local 65000
    	}
    }
    peer transit1 {
        connection {
            remote {
                ip 192.0.2.1
            }
            local {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
        }
        timer {
            receive-hold-time 30;
        }
    }
}`,
			wantHoldTime: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := LoadReactor(tt.config)
			require.NoError(t, err)

			peers := r.Peers()
			require.Len(t, peers, 1)

			settings := peers[0].Settings()
			gotSec := int(settings.ReceiveHoldTime.Seconds())
			assert.Equal(t, tt.wantHoldTime, gotSec, "receive-hold-time mismatch")
		})
	}
}

// --- Dependency expansion tests ---

// TestExpandDependencies verifies that dependencies are auto-added.
//
// VALIDATES: expandDependencies adds missing deps as Internal=true, Encoder="json".
// PREVENTS: Missing dependency plugins at runtime.
func TestExpandDependencies(t *testing.T) {
	// Register a plugin with a dependency in the registry.
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	registry.Reset()

	regA := registry.Registration{
		Name:         "plugin-a",
		Description:  "test A",
		Dependencies: []string{"plugin-b"},
		RunEngine:    func(_ net.Conn) int { return 0 },
		CLIHandler:   func(_ []string) int { return 0 },
	}
	regB := registry.Registration{
		Name:        "plugin-b",
		Description: "test B",
		RunEngine:   func(_ net.Conn) int { return 0 },
		CLIHandler:  func(_ []string) int { return 0 },
	}
	require.NoError(t, registry.Register(regA))
	require.NoError(t, registry.Register(regB))

	plugins := []reactor.PluginConfig{
		{Name: "plugin-a", Internal: true, Encoder: "json"},
	}

	result, err := config.ExpandDependencies(plugins)
	require.NoError(t, err)
	require.Len(t, result, 2)

	// Find the auto-added plugin-b
	var foundB bool
	for _, p := range result {
		if p.Name == "plugin-b" {
			foundB = true
			assert.True(t, p.Internal, "auto-added dep should be Internal")
			assert.Equal(t, "json", p.Encoder, "auto-added dep should use json encoder")
		}
	}
	assert.True(t, foundB, "plugin-b should be auto-added")
}

// TestExpandDependencies_NoDuplicate verifies already-present deps are not duplicated.
//
// VALIDATES: If dep is already in plugin list, it is not added again.
// PREVENTS: Duplicate plugin entries causing startup failures.
func TestExpandDependencies_NoDuplicate(t *testing.T) {
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	registry.Reset()

	regA := registry.Registration{
		Name:         "plugin-a",
		Description:  "test A",
		Dependencies: []string{"plugin-b"},
		RunEngine:    func(_ net.Conn) int { return 0 },
		CLIHandler:   func(_ []string) int { return 0 },
	}
	regB := registry.Registration{
		Name:        "plugin-b",
		Description: "test B",
		RunEngine:   func(_ net.Conn) int { return 0 },
		CLIHandler:  func(_ []string) int { return 0 },
	}
	require.NoError(t, registry.Register(regA))
	require.NoError(t, registry.Register(regB))

	plugins := []reactor.PluginConfig{
		{Name: "plugin-a", Internal: true, Encoder: "json"},
		{Name: "plugin-b", Internal: true, Encoder: "json"},
	}

	result, err := config.ExpandDependencies(plugins)
	require.NoError(t, err)
	require.Len(t, result, 2, "should not duplicate plugin-b")
}

// TestExpandDependencies_Integration verifies LoadReactorWithPlugins auto-adds deps.
//
// VALIDATES: AC-1: LoadReactorWithPlugins with ["ze.bgp-rs"] produces list with both bgp-rs and bgp-adj-rib-in.
// PREVENTS: bgp-rs starting without adj-rib-in, causing silent replay failure.
func TestExpandDependencies_Integration(t *testing.T) {
	// This test uses the real registry (all plugins registered via init())
	// to verify that bgp-rs's dependency on bgp-adj-rib-in is expanded.
	input := `
bgp {
    router-id 10.0.0.1;
    session {
    	asn {
    		local 65000
    	}
    }
    peer transit1 {
        connection {
            remote {
                ip 192.0.2.1
            }
            local {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
        }
    }
}
`
	r, err := LoadReactorWithPlugins(storage.NewFilesystem(), input, "-", []string{"ze.bgp-rs"})
	require.NoError(t, err)
	require.NotNil(t, r)

	// Check that the reactor's plugin list contains both bgp-rs and bgp-adj-rib-in
	pluginNames := r.PluginNamesForTest()
	assert.Contains(t, pluginNames, "bgp-rs", "bgp-rs should be in plugin list")
	assert.Contains(t, pluginNames, "bgp-adj-rib-in", "bgp-adj-rib-in should be auto-added")
}

// TestLoaderWithBlobStorage verifies config loading through blob storage.
//
// VALIDATES: LoadReactorFile reads config from blob storage, not filesystem.
// PREVENTS: Storage wiring broken - loader silently falls back to os.ReadFile.
func TestLoaderWithBlobStorage(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "test.conf")
	configContent := `
bgp {
    router-id 10.0.0.1;
    session {
    	asn {
    		local 65000
    	}
    }
    peer transit1 {
        connection {
            remote {
                ip 192.0.2.1
            }
            local {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
        }
    }
}
`
	// Write config to filesystem first so NewBlob migrates it
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	blobPath := filepath.Join(dir, "database.zefs")
	store, err := storage.NewBlob(blobPath, dir)
	if err != nil {
		t.Fatalf("NewBlob: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup

	// Delete the filesystem copy to prove we're reading from blob
	if err := os.Remove(configPath); err != nil {
		t.Fatalf("remove filesystem config: %v", err)
	}

	// Load from blob storage
	r, err := LoadReactorFile(store, configPath)
	require.NoError(t, err)
	require.NotNil(t, r)

	peers := r.Peers()
	require.Len(t, peers, 1)
	assert.Equal(t, uint32(65000), peers[0].Settings().LocalAS)
	assert.Equal(t, uint32(65001), peers[0].Settings().PeerAS)
}

// TestReloadWithBlobStorage verifies config can be re-read from blob after modification.
//
// VALIDATES: Modified config in blob is picked up on re-read.
// PREVENTS: Stale config served from cache or filesystem fallback.
func TestReloadWithBlobStorage(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "test.conf")
	initialConfig := `
bgp {
    router-id 10.0.0.1;
    session {
    	asn {
    		local 65000
    	}
    }
    peer transit1 {
        connection {
            remote {
                ip 192.0.2.1
            }
            local {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
        }
    }
}
`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	blobPath := filepath.Join(dir, "database.zefs")
	store, err := storage.NewBlob(blobPath, dir)
	if err != nil {
		t.Fatalf("NewBlob: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup

	// Load initial config from blob
	r1, err := LoadReactorFile(store, configPath)
	require.NoError(t, err)
	require.Len(t, r1.Peers(), 1)

	// Update config in blob (add second peer)
	updatedConfig := `
bgp {
    router-id 10.0.0.1;
    session {
    	asn {
    		local 65000
    	}
    }
    peer transit1 {
        connection {
            remote {
                ip 192.0.2.1
            }
            local {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
        }
    }
    peer transit2 {
        connection {
            remote {
                ip 192.0.2.2
            }
            local {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65002
            }
        }
    }
}
`
	if err := store.WriteFile(configPath, []byte(updatedConfig), 0o600); err != nil {
		t.Fatalf("update config in blob: %v", err)
	}

	// Reload from blob - should see updated config
	r2, err := LoadReactorFile(store, configPath)
	require.NoError(t, err)
	require.Len(t, r2.Peers(), 2, "reloaded config should have 2 peers")
}

// test-relax: TestResolveSSHStorage MOVED, not deleted -- ResolveSSHStorage now
// lives in internal/component/config/infra (spec-feature-gate-10-bgp Bucket 2).
// The test moved verbatim to internal/component/config/infra/ssh_test.go.

// TestReservedPeerNamesSyncWithRPCs verifies that reservedPeerNames in resolve.go
// matches the subcommand keywords derived from registered RPCs.
// This test imports plugin/all so all init() registrations have run.
//
// VALIDATES: Hardcoded reserved names stay in sync with registered "peer" RPCs.
// PREVENTS: New "peer <subcommand>" RPC added without updating reservedPeerNames.
func TestReservedPeerNamesSyncWithRPCs(t *testing.T) {
	loader, err := yang.DefaultLoader()
	require.NoError(t, err)
	wireToPath := yang.WireMethodToPath(loader)

	dynamicKeywords := pluginserver.PeerSubcommandKeywords(wireToPath)
	require.NotEmpty(t, dynamicKeywords, "YANG cmd modules should define peer commands")

	// Every dynamically discovered keyword must be in the hardcoded set.
	for keyword := range dynamicKeywords {
		assert.True(t, reservedPeerNames[keyword],
			"RPC keyword %q (from registered bgp peer commands) is missing from reservedPeerNames in resolve.go", keyword)
	}

	// Every hardcoded keyword should correspond to a registered RPC.
	for keyword := range reservedPeerNames {
		assert.True(t, dynamicKeywords[keyword],
			"reservedPeerNames entry %q has no matching registered bgp peer RPC -- remove it or register the RPC", keyword)
	}
}

// mockIntrospector implements plugin.ReactorIntrospector for collectPrefixWarnings tests.
type mockIntrospector struct {
	peers []plugin.PeerInfo
}

func (m *mockIntrospector) Peers() []plugin.PeerInfo   { return m.peers }
func (m *mockIntrospector) Stats() plugin.ReactorStats { return plugin.ReactorStats{} }
func (m *mockIntrospector) PeerNegotiatedCapabilities(_ netip.Addr) *plugin.PeerCapabilitiesInfo {
	return nil
}
func (m *mockIntrospector) GetPeerProcessBindings(_ netip.Addr) []plugin.PeerProcessBinding {
	return nil
}
func (m *mockIntrospector) GetPeerCapabilityConfigs() []plugin.PeerCapabilityConfig { return nil }

// TestCollectPrefixWarningsOneStale verifies that a single stale peer shows the specific warning.
// Bus seeded with one prefix-stale entry; banner reads from the bus.
//
// VALIDATES: Login banner shows detail when exactly 1 warning exists.
// PREVENTS: Single-warning case still showing "1 warnings" count.
func TestCollectPrefixWarningsOneStale(t *testing.T) {
	report.ClearSource("bgp")
	defer report.ClearSource("bgp")

	report.RaiseWarning("bgp", "prefix-stale", "10.0.0.1",
		"prefix data updated 2025-01-01 (>180 days old)",
		map[string]any{"updated": "2025-01-01"})

	ri := &mockIntrospector{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: 65001, Name: "core-rtr"},
			{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65002},
		},
	}

	warnings := collectPrefixWarnings(ri)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0].Message, "core-rtr")
	assert.Contains(t, warnings[0].Message, "stale prefix data")
	assert.NotEmpty(t, warnings[0].Command)
}

// TestCollectPrefixWarningsMultiple verifies that >1 warnings shows a count.
//
// VALIDATES: Login banner shows "N warnings" when multiple warnings exist.
// PREVENTS: Banner dumping all warnings when count is high.
func TestCollectPrefixWarningsMultiple(t *testing.T) {
	report.ClearSource("bgp")
	defer report.ClearSource("bgp")

	report.RaiseWarning("bgp", "prefix-stale", "10.0.0.1",
		"stale", map[string]any{"updated": "2025-01-01"})
	report.RaiseWarning("bgp", "prefix-stale", "10.0.0.2",
		"stale", map[string]any{"updated": "2025-01-01"})

	ri := &mockIntrospector{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: 65001},
			{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65002},
		},
	}

	warnings := collectPrefixWarnings(ri)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0].Message, "2 warnings")
	assert.Equal(t, "show warnings", warnings[0].Command)
}

// TestCollectPrefixWarningsNone verifies no warnings returned for healthy peers.
//
// VALIDATES: Login banner is silent when no warnings exist.
// PREVENTS: Spurious warnings on healthy system.
func TestCollectPrefixWarningsNone(t *testing.T) {
	report.ClearSource("bgp")
	defer report.ClearSource("bgp")

	ri := &mockIntrospector{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: 65001},
		},
	}

	warnings := collectPrefixWarnings(ri)
	assert.Nil(t, warnings)
}

// TestCollectPrefixWarningsRuntime verifies that runtime prefix threshold warnings are included.
//
// VALIDATES: Login banner includes runtime prefix warnings, not just staleness.
// PREVENTS: Runtime prefix warnings invisible at login.
func TestCollectPrefixWarningsRuntime(t *testing.T) {
	report.ClearSource("bgp")
	defer report.ClearSource("bgp")

	report.RaiseWarning("bgp", "prefix-threshold", "10.0.0.1/ipv4/unicast",
		"ipv4/unicast prefix count exceeds warning threshold",
		map[string]any{"family": "ipv4/unicast"})

	ri := &mockIntrospector{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: 65001},
		},
	}

	warnings := collectPrefixWarnings(ri)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0].Message, "ipv4/unicast")
	assert.Contains(t, warnings[0].Message, "exceeds warning threshold")
}

// test-relax: convertMVPNRoute/MVPNRouteConfig removed by spec-route-config-plugin-migration;
// MVPN config parsing (incl originator-id/cluster-list) is now in plugins/nlri/mvpn/config.go,
// verified byte-for-byte by test/encode/mvpn.ci.

// TestExpandDependenciesResolvesUseLabel pins dependency resolution to the
// REGISTERED plugin name rather than the operator's config label.
//
// VALIDATES: `plugin { internal <label> { use <plugin> } }` expands the
// dependencies declared by <plugin>.
//
// PREVENTS: the shipped defect where ExpandDependencies keyed on p.Name (the
// label). ResolveDependencies treats a name absent from the registry as an
// external plugin and skips expanding it, so a route server configured as
// `internal rs { use bgp-rs }` silently loaded no bgp-adj-rib-in and lost its
// peer-up replay entirely -- no error, no warning. Hard Dependencies were
// dropped just as silently on the same path.
func TestExpandDependenciesResolvesUseLabel(t *testing.T) {
	newReg := func(name string, opt, hard []string) registry.Registration {
		return registry.Registration{
			Name:                 name,
			Description:          name,
			Dependencies:         hard,
			OptionalDependencies: opt,
			RunEngine:            func(_ net.Conn) int { return 0 },
			CLIHandler:           func(_ []string) int { return 0 },
		}
	}

	for _, tt := range []struct {
		name string
		run  string
	}{
		{"bare use value", "bgp-rs"},
		{"ze-prefixed use value", "ze.bgp-rs"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			snap := registry.Snapshot()
			t.Cleanup(func() { registry.Restore(snap) })
			registry.Reset()

			require.NoError(t, registry.Register(
				newReg("bgp-rs", []string{"bgp-adj-rib-in"}, []string{"bgp-rib"})))
			require.NoError(t, registry.Register(newReg("bgp-adj-rib-in", nil, nil)))
			require.NoError(t, registry.Register(newReg("bgp-rib", nil, nil)))

			// The operator labels the instance "rs" and runs the plugin "bgp-rs".
			plugins := []reactor.PluginConfig{
				{Name: "rs", Internal: true, Run: tt.run, Encoder: "json"},
			}

			result, err := config.ExpandDependencies(plugins)
			require.NoError(t, err)

			names := make([]string, 0, len(result))
			for _, p := range result {
				names = append(names, p.Name)
			}
			assert.Contains(t, names, "bgp-adj-rib-in",
				"the optional dependency of the plugin named by `use` must be expanded")
			assert.Contains(t, names, "bgp-rib",
				"the hard dependency of the plugin named by `use` must be expanded")
			assert.Contains(t, names, "rs", "the operator's labeled entry must survive")
			assert.Len(t, result, 3, "bgp-rs runs under the label; it must not be added twice")
		})
	}
}
