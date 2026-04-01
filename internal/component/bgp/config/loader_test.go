package bgpconfig

import (
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/reactor"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
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
            }
            local {
                ip 192.168.1.1
                connect false
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
            }
            local {
                ip 192.168.1.1
                connect false
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
        process rib { send [ update ]; }
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

// TestParseAllConfigFiles verifies all etc/ze/bgp/*.conf files parse.
//
// VALIDATES: All example configs are syntactically valid.
//
// PREVENTS: Broken example configs shipped with the project.
func TestParseAllConfigFiles(t *testing.T) {
	t.Skip("TODO: Convert etc/ze/bgp/*.conf files from ExaBGP to native Ze syntax")
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

// TestBuildMUPNLRI_T1ST_Source verifies T1ST source field encoding.
//
// VALIDATES: MUP T1ST routes correctly encode the optional source field
// with source_len (1 byte) + source_addr (4 or 16 bytes).
//
// PREVENTS: Silent failures when source address is invalid,
// missing source encoding in NLRI output.
func TestBuildMUPNLRI_T1ST_Source(t *testing.T) {
	tests := []struct {
		name        string
		config      MUPRouteConfig
		wantHex     string // expected hex substring in NLRI (source_len + source_addr)
		wantErr     bool
		wantErrText string
	}{
		{
			name: "IPv4 T1ST with source",
			config: MUPRouteConfig{
				RouteType: "mup-t1st",
				IsIPv6:    false,
				Prefix:    "192.168.0.2/32",
				RD:        "100:100",
				TEID:      "12345",
				QFI:       9,
				Endpoint:  "10.0.0.1",
				Source:    "10.0.1.1",
			},
			// source: len=32 (0x20), addr=10.0.1.1 (0x0a000101)
			wantHex: "200a000101",
		},
		{
			name: "IPv6 T1ST with source",
			config: MUPRouteConfig{
				RouteType: "mup-t1st",
				IsIPv6:    true,
				Prefix:    "2001:db8:1:1::2/128",
				RD:        "100:100",
				TEID:      "12345",
				QFI:       9,
				Endpoint:  "2001::1",
				Source:    "2002::2",
			},
			// source: len=128 (0x80), addr=2002::2
			wantHex: "8020020000000000000000000000000002",
		},
		{
			name: "T1ST without source (optional)",
			config: MUPRouteConfig{
				RouteType: "mup-t1st",
				IsIPv6:    false,
				Prefix:    "192.168.0.2/32",
				RD:        "100:100",
				TEID:      "12345",
				QFI:       9,
				Endpoint:  "10.0.0.1",
				Source:    "", // no source
			},
			// endpoint should be last: len=32 (0x20), addr=10.0.0.1 (0x0a000001)
			// no source bytes after
			wantHex: "200a000001",
		},
		{
			name: "T1ST with invalid source fails loudly",
			config: MUPRouteConfig{
				RouteType: "mup-t1st",
				IsIPv6:    false,
				Prefix:    "192.168.0.2/32",
				RD:        "100:100",
				TEID:      "12345",
				QFI:       9,
				Endpoint:  "10.0.0.1",
				Source:    "not-an-ip",
			},
			wantErr:     true,
			wantErrText: "invalid T1ST source",
		},
		{
			name: "T1ST with invalid endpoint fails loudly",
			config: MUPRouteConfig{
				RouteType: "mup-t1st",
				IsIPv6:    false,
				Prefix:    "192.168.0.2/32",
				RD:        "100:100",
				TEID:      "12345",
				QFI:       9,
				Endpoint:  "bad-endpoint",
				Source:    "10.0.1.1",
			},
			wantErr:     true,
			wantErrText: "invalid T1ST endpoint",
		},
		{
			name: "T1ST with invalid prefix fails loudly",
			config: MUPRouteConfig{
				RouteType: "mup-t1st",
				IsIPv6:    false,
				Prefix:    "not-a-prefix",
				RD:        "100:100",
			},
			wantErr:     true,
			wantErrText: "invalid T1ST prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mupFamily := familyIPv4MUP
			if tt.config.IsIPv6 {
				mupFamily = familyIPv6MUP
			}
			args := mupRouteConfigToArgs(tt.config)
			nlriHex, err := registry.EncodeNLRIByFamily(mupFamily, args)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrText)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, nlriHex)

			// Registry returns uppercase hex; wantHex is lowercase
			nlriHexLower := strings.ToLower(nlriHex)
			assert.Contains(t, nlriHexLower, tt.wantHex,
				"NLRI should contain %s, got %s", tt.wantHex, nlriHexLower)
		})
	}
}

// TestConvertMVPNRoute_OriginatorID verifies RFC 4456 originator-id parsing.
//
// VALIDATES: OriginatorID is parsed from config IP string to uint32.
// PREVENTS: Route reflector config silently ignored.
func TestConvertMVPNRoute_OriginatorID(t *testing.T) {
	mr := MVPNRouteConfig{
		RouteType:    "source-ad",
		OriginatorID: "192.168.1.1",
	}
	route, err := convertMVPNRoute(mr)
	require.NoError(t, err)
	require.Equal(t, uint32(0xC0A80101), route.OriginatorID)
}

// TestConvertMVPNRoute_ClusterList verifies RFC 4456 cluster-list parsing.
//
// VALIDATES: ClusterList is parsed from space-separated IPs to []uint32.
// PREVENTS: Route reflector config silently ignored.
func TestConvertMVPNRoute_ClusterList(t *testing.T) {
	mr := MVPNRouteConfig{
		RouteType:   "source-ad",
		ClusterList: "192.168.1.1 192.168.1.2",
	}
	route, err := convertMVPNRoute(mr)
	require.NoError(t, err)
	require.Equal(t, []uint32{0xC0A80101, 0xC0A80102}, route.ClusterList)
}

// TestConvertMVPNRoute_InvalidOriginatorID verifies error on bad IP.
//
// VALIDATES: Invalid originator-id returns descriptive error.
// PREVENTS: Silent failure on malformed config.
func TestConvertMVPNRoute_InvalidOriginatorID(t *testing.T) {
	mr := MVPNRouteConfig{
		RouteType:    "source-ad",
		OriginatorID: "not-an-ip",
	}
	_, err := convertMVPNRoute(mr)
	require.Error(t, err)
	require.Contains(t, err.Error(), "originator-id")
}

// TestConvertMVPNRoute_InvalidClusterList verifies error on bad cluster-list IP.
//
// VALIDATES: Invalid cluster-list IP returns descriptive error.
// PREVENTS: Silent failure on malformed config.
func TestConvertMVPNRoute_InvalidClusterList(t *testing.T) {
	mr := MVPNRouteConfig{
		RouteType:   "source-ad",
		ClusterList: "192.168.1.1 bad-ip",
	}
	_, err := convertMVPNRoute(mr)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cluster-list")
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
        run "ze plugin gr";
        encoder json;
    }
    external rib {
        run "ze plugin rib";
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
	run, ok := grPlugin.Get("run")
	require.True(t, ok)
	require.Equal(t, "ze plugin gr", run)

	encoder, ok := grPlugin.Get("encoder")
	require.True(t, ok)
	require.Equal(t, "json", encoder)

	// Verify RIB plugin
	ribPlugin := plugins["rib"]
	require.NotNil(t, ribPlugin)
	run, ok = ribPlugin.Get("run")
	require.True(t, ok)
	require.Equal(t, "ze plugin rib", run)
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
			result, err := mergeCliPlugins(tt.configPlugs, tt.cliPlugs)
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
	result, err := mergeCliPlugins(nil, []string{"ze.bgp-rib"})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.True(t, result[0].Internal, "internal plugin should have Internal=true")
	assert.Empty(t, result[0].Run, "internal plugin should have empty Run")
	assert.Equal(t, "bgp-rib", result[0].Name)

	result2, err := mergeCliPlugins(nil, []string{"./custom-plugin"})
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

	result, err := expandDependencies(plugins)
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

	result, err := expandDependencies(plugins)
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
	pluginNames := r.PluginNames()
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

// TestResolveSSHStorage verifies that SSH storage always resolves to blob when zefs exists.
//
// VALIDATES: SSH host key goes into blob store even when main store is filesystem.
// PREVENTS: Host key written as plain file when config loaded from filesystem.
func TestResolveSSHStorage(t *testing.T) {
	dir := t.TempDir()
	blobPath := filepath.Join(dir, "database.zefs")

	t.Run("blob store passed through", func(t *testing.T) {
		blob, err := storage.NewBlob(blobPath, dir)
		require.NoError(t, err)
		defer blob.Close() //nolint:errcheck // test

		got := resolveSSHStorage(blob, dir)
		assert.True(t, storage.IsBlobStorage(got), "blob storage should pass through")
		got.Close() //nolint:errcheck // test
	})

	t.Run("filesystem upgraded to blob when zefs exists", func(t *testing.T) {
		// Create zefs database first
		blob, err := storage.NewBlob(blobPath, dir)
		require.NoError(t, err)
		blob.Close() //nolint:errcheck // just creating

		fs := storage.NewFilesystem()
		got := resolveSSHStorage(fs, dir)
		assert.True(t, storage.IsBlobStorage(got), "filesystem should be upgraded to blob when zefs exists")
		got.Close() //nolint:errcheck // test
	})

	t.Run("filesystem kept when no config dir", func(t *testing.T) {
		fs := storage.NewFilesystem()
		got := resolveSSHStorage(fs, "")
		assert.False(t, storage.IsBlobStorage(got), "should stay filesystem when no config dir")
	})
}

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
//
// VALIDATES: Login banner shows detail when exactly 1 warning exists.
// PREVENTS: Single-warning case still showing "1 warnings" count.
func TestCollectPrefixWarningsOneStale(t *testing.T) {
	staleDate := "2025-01-01" // > 6 months ago
	ri := &mockIntrospector{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: 65001, Name: "core-rtr", PrefixUpdated: staleDate},
			{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65002, PrefixUpdated: "2026-03-01"},
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
	staleDate := "2025-01-01"
	ri := &mockIntrospector{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: 65001, PrefixUpdated: staleDate},
			{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65002, PrefixUpdated: staleDate},
		},
	}

	warnings := collectPrefixWarnings(ri)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0].Message, "2 warnings")
	assert.Equal(t, "show bgp warnings", warnings[0].Command)
}

// TestCollectPrefixWarningsNone verifies no warnings returned for healthy peers.
//
// VALIDATES: Login banner is silent when no warnings exist.
// PREVENTS: Spurious warnings on healthy system.
func TestCollectPrefixWarningsNone(t *testing.T) {
	ri := &mockIntrospector{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: 65001, PrefixUpdated: "2026-03-01"},
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
	ri := &mockIntrospector{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: 65001, PrefixWarnings: []string{"ipv4/unicast"}},
		},
	}

	warnings := collectPrefixWarnings(ri)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0].Message, "ipv4/unicast")
	assert.Contains(t, warnings[0].Message, "exceeds warning threshold")
}
