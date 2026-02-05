package config

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
    local-as 65000;
    listen 127.0.0.1:1179;

    peer 192.0.2.1 {
        peer-as 65001;
        hold-time 90;
    }

    peer 192.0.2.2 {
        peer-as 65002;
        passive true;
    }
}
`

	r, err := LoadReactor(input)
	require.NoError(t, err)
	require.NotNil(t, r)

	peers := r.Peers()
	require.Len(t, peers, 2)
}

// TestLoadReactorInheritance verifies local-as inheritance.
//
// VALIDATES: Neighbors inherit global local-as.
//
// PREVENTS: Zero AS numbers in neighbors.
func TestLoadReactorInheritance(t *testing.T) {
	input := `
bgp {
    router-id 10.0.0.1;
    local-as 65000;

    peer 192.0.2.1 {
        peer-as 65001;
    }
}
`

	r, err := LoadReactor(input)
	require.NoError(t, err)

	peers := r.Peers()
	require.Len(t, peers, 1)

	// Neighbor should inherit local-as from global
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
    local-as 65000;

    peer 192.0.2.1 {
        peer-as 65001;
        passive true;
    }
}
`

	r, err := LoadReactor(input)
	require.NoError(t, err)

	peers := r.Peers()
	require.Len(t, peers, 1)

	n := peers[0].Settings()
	require.True(t, n.Passive)
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
    local-as 65000;

    peer 192.0.2.1 {
        peer-as 65001;
        capability {
            route-refresh;
        }
        process rib { send { update; } }
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

// TestLoadReactorConfig verifies reactor config settings.
//
// VALIDATES: Listen address and router-id are set.
//
// PREVENTS: Missing reactor configuration.
func TestLoadReactorConfig(t *testing.T) {
	input := `
bgp {
    router-id 10.0.0.1;
    local-as 65000;
    listen 0.0.0.0:179;

    peer 192.0.2.1 {
        peer-as 65001;
    }
}
`

	cfg, r, err := LoadReactorWithConfig(input)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, r)

	require.Equal(t, uint32(0x0a000001), cfg.RouterID) // 10.0.0.1
	require.Equal(t, uint32(65000), cfg.LocalAS)
}

// TestLoadReactorError verifies error handling.
//
// VALIDATES: Invalid config returns error.
//
// PREVENTS: Silent config failures.
func TestLoadReactorError(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as not-a-number;
    }
}
`

	_, err := LoadReactor(input)
	require.Error(t, err)
}

// TestCreateReactorFromConfig verifies direct Config to Reactor.
//
// VALIDATES: CreateReactor works with typed Config.
//
// PREVENTS: Only string-based loading working.
func TestCreateReactorFromConfig(t *testing.T) {
	cfg := &BGPConfig{
		RouterID: 0x0a000001,
		LocalAS:  65000,
		Listen:   "127.0.0.1:1179",
	}

	r, err := CreateReactor(cfg)
	require.NoError(t, err)
	require.NotNil(t, r)
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
		require.Contains(t, err.Error(), "ze bgp config migrate")
	})

	t.Run("template.neighbor triggers hint", func(t *testing.T) {
		input := `template { neighbor test { local-as 65000; } }`
		_, err := LoadReactor(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown field in template: neighbor")
		require.Contains(t, err.Error(), "ze bgp config migrate")
	})

	t.Run("current syntax no hint", func(t *testing.T) {
		// Valid current config should parse without error (no hint needed)
		input := `bgp { peer 192.0.2.1 { local-as 65000; peer-as 65001; } }`
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
			nlri, err := buildMUPNLRI(tt.config)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrText)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, nlri)

			// Check that the expected hex is present in the NLRI
			nlriHex := hex.EncodeToString(nlri)
			assert.Contains(t, nlriHex, tt.wantHex,
				"NLRI should contain %s, got %s", tt.wantHex, nlriHex)
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
// TestPluginOnlySchema verifies that PluginOnlySchema() only parses plugin blocks.
//
// VALIDATES: First phase parsing extracts only plugin definitions.
// PREVENTS: Non-plugin config blocks being parsed in first phase.
func TestPluginOnlySchema(t *testing.T) {
	// Input with ONLY plugin blocks (no peer block)
	input := `
plugin {
    external gr {
        run "ze bgp plugin gr";
        encoder json;
    }
    external rib {
        run "ze bgp plugin rib";
    }
}
`
	// PluginOnlySchema should only parse plugin blocks
	p := NewParser(PluginOnlySchema())
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
	require.Equal(t, "ze bgp plugin gr", run)

	encoder, ok := grPlugin.Get("encoder")
	require.True(t, ok)
	require.Equal(t, "json", encoder)

	// Verify RIB plugin
	ribPlugin := plugins["rib"]
	require.NotNil(t, ribPlugin)
	run, ok = ribPlugin.Get("run")
	require.True(t, ok)
	require.Equal(t, "ze bgp plugin rib", run)
}

// TestPluginOnlySchemaRejectsUnknown verifies unknown blocks are rejected.
//
// VALIDATES: PluginOnlySchema only accepts plugin blocks.
// PREVENTS: Accidental parsing of peer/template blocks in first phase.
func TestPluginOnlySchemaRejectsUnknown(t *testing.T) {
	input := `
peer 192.0.2.1 {
    peer-as 65001;
}
`
	p := NewParser(PluginOnlySchema())
	_, err := p.Parse(input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown top-level keyword: peer")
}

// TestSchemaExtendCapability verifies dynamic schema extension.
//
// VALIDATES: Schema.ExtendCapability adds new capability sub-blocks.
// PREVENTS: Plugin-declared capabilities being rejected as unknown.
func TestSchemaExtendCapability(t *testing.T) {
	schema := YANGSchema()

	// Before extension, custom capability should fail
	inputBefore := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        capability {
            custom-cap {
                some-value 42;
            }
        }
    }
}
`
	p := NewParser(schema)
	_, err := p.Parse(inputBefore)
	require.Error(t, err, "custom-cap should be unknown before extension")

	// Extend schema with custom capability
	err = schema.ExtendCapability("custom-cap",
		Field("some-value", Leaf(TypeUint32)),
	)
	require.NoError(t, err)

	// After extension, custom capability should parse
	inputAfter := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        capability {
            custom-cap {
                some-value 42;
            }
        }
    }
}
`
	p = NewParser(schema)
	tree, err := p.Parse(inputAfter)
	require.NoError(t, err)

	// Verify the capability was parsed
	bgpContainer := tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer)

	peers := bgpContainer.GetList("peer")
	require.Len(t, peers, 1)

	peer := peers["192.0.2.1"]
	require.NotNil(t, peer)

	cap := peer.GetContainer("capability")
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
    local-as 65000;
    listen 127.0.0.1:1179;

    peer 192.0.2.1 {
        peer-as 65001;
        hold-time 90;
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
local-as 65000;

peer 192.0.2.1 {
    peer-as 65001;
}
`

	_, err := LoadReactor(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown top-level keyword")
}

// TestParseTemplateNewSyntax verifies new template syntax with peer patterns.
//
// VALIDATES: template { bgp { peer <pattern> { } } } syntax.
// PREVENTS: Template config parsing failures with new syntax.
func TestParseTemplateNewSyntax(t *testing.T) {
	input := `
template {
    bgp {
        peer * {
            hold-time 90;
        }
    }
}

bgp {
    router-id 10.0.0.1;
    local-as 65000;

    peer 192.0.2.1 {
        peer-as 65001;
    }
}
`

	r, err := LoadReactor(input)
	require.NoError(t, err)
	require.NotNil(t, r)

	peers := r.Peers()
	require.Len(t, peers, 1)

	// Peer should inherit hold-time from template
	settings := peers[0].Settings()
	assert.Equal(t, 90*1000000000, int(settings.HoldTime.Nanoseconds()))
}

// TestInheritNameKeyword verifies inherit-name in templates.
//
// VALIDATES: template with inherit-name creates named template.
// PREVENTS: Named template lookup failures.
func TestInheritNameKeyword(t *testing.T) {
	input := `
template {
    bgp {
        peer * {
            inherit-name backbone;
            hold-time 90;
        }
    }
}

bgp {
    router-id 10.0.0.1;
    local-as 65000;

    peer 192.0.2.1 {
        inherit backbone;
        peer-as 65001;
    }
}
`

	r, err := LoadReactor(input)
	require.NoError(t, err)
	require.NotNil(t, r)

	peers := r.Peers()
	require.Len(t, peers, 1)

	// Peer should inherit hold-time from backbone template
	settings := peers[0].Settings()
	assert.Equal(t, 90*1000000000, int(settings.HoldTime.Nanoseconds()))
}

// TestInheritNonExistent verifies error for missing template.
//
// VALIDATES: inherit with non-existent template name fails.
// PREVENTS: Silent failure when template doesn't exist.
func TestInheritNonExistent(t *testing.T) {
	input := `
bgp {
    router-id 10.0.0.1;
    local-as 65000;

    peer 192.0.2.1 {
        inherit nonexistent;
        peer-as 65001;
    }
}
`

	_, err := LoadReactor(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

// TestInheritPatternValidation verifies inherit pattern matching.
//
// VALIDATES: inherit only works when peer matches template pattern.
// PREVENTS: Applying templates to non-matching peers.
func TestInheritPatternValidation(t *testing.T) {
	input := `
template {
    bgp {
        peer 10.* {
            inherit-name internal;
            hold-time 90;
        }
    }
}

bgp {
    router-id 10.0.0.1;
    local-as 65000;

    # This peer IP doesn't match 10.* pattern
    peer 192.168.1.1 {
        inherit internal;
        peer-as 65001;
    }
}
`

	_, err := LoadReactor(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pattern")
}

// TestMergeCliPlugins verifies CLI plugin merging with config plugins.
//
// VALIDATES: CLI plugins are correctly resolved and merged.
// PREVENTS: Plugin duplication or incorrect resolution.
func TestMergeCliPlugins(t *testing.T) {
	tests := []struct {
		name        string
		configPlugs []PluginConfig
		cliPlugs    []string
		wantNames   []string // Expected plugin names in order
		wantErr     bool
	}{
		{
			name:        "cli_only_internal",
			configPlugs: nil,
			cliPlugs:    []string{"ze.rib"},
			wantNames:   []string{"rib"},
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
			cliPlugs:    []string{"ze.rib", "ze.gr"},
			wantNames:   []string{"rib", "gr"},
		},
		{
			name: "cli_plus_config",
			configPlugs: []PluginConfig{
				{Name: "existing", Run: "some-plugin"},
			},
			cliPlugs:  []string{"ze.rib"},
			wantNames: []string{"rib", "existing"}, // CLI first
		},
		{
			name: "dedup_cli_matches_config",
			configPlugs: []PluginConfig{
				{Name: "rib", Run: "ze bgp plugin rib"},
			},
			cliPlugs:  []string{"ze.rib"},
			wantNames: []string{"rib"}, // Only one rib
		},
		{
			name:        "cli_command_with_args",
			configPlugs: nil,
			cliPlugs:    []string{"ze bgp plugin rr"},
			wantNames:   []string{"rr"},
		},
		{
			name:        "empty_cli",
			configPlugs: []PluginConfig{{Name: "existing", Run: "x"}},
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
			cfg := &BGPConfig{
				Plugins: tt.configPlugs,
			}

			err := mergeCliPlugins(cfg, tt.cliPlugs)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			var gotNames []string
			for _, p := range cfg.Plugins {
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
	cfg := &BGPConfig{}
	err := mergeCliPlugins(cfg, []string{"ze.rib"})
	require.NoError(t, err)
	require.Len(t, cfg.Plugins, 1)
	assert.True(t, cfg.Plugins[0].Internal, "internal plugin should have Internal=true")
	assert.Empty(t, cfg.Plugins[0].Run, "internal plugin should have empty Run")
	assert.Equal(t, "rib", cfg.Plugins[0].Name)

	cfg2 := &BGPConfig{}
	err = mergeCliPlugins(cfg2, []string{"./custom-plugin"})
	require.NoError(t, err)
	require.Len(t, cfg2.Plugins, 1)
	assert.False(t, cfg2.Plugins[0].Internal, "external plugin should have Internal=false")
	assert.Equal(t, "./custom-plugin", cfg2.Plugins[0].Run)
}

// TestHostnameAlwaysAvailable verifies hostname/domain-name are always parseable.
//
// VALIDATES: host-name/domain-name parse with default schema (internal plugin YANG always loaded).
// PREVENTS: Regression in internal plugin YANG loading.
func TestHostnameAlwaysAvailable(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        host-name my-host-name;
    }
}
`
	// YANGSchema() now includes all internal plugin YANG (hostname, gr, etc.)
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)

	// Parsing should succeed - internal plugin YANG is always loaded
	require.NoError(t, err)
	require.NotNil(t, tree)
}

// TestHostnameWithPluginYANG verifies hostname plugin YANG enables parsing.
//
// VALIDATES: With plugin YANG, host-name/domain-name parse and populate RawCapabilityConfig.
// PREVENTS: Plugin not receiving config values.
func TestHostnameWithPluginYANG(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        host-name my-host-name;
        domain-name my-domain.com;
    }
}
`
	// Load plugin YANG
	pluginYANG := map[string]string{
		"ze-hostname.yang": `module ze-hostname {
    namespace "urn:ze:hostname";
    prefix hostname;
    import ze-bgp-conf { prefix bgp; }
    revision 2025-01-29 { description "Test"; }
    augment "/bgp:bgp/bgp:peer" {
        leaf host-name { type string; }
        leaf domain-name { type string; }
    }
}`,
	}

	// With plugin YANG, parsing should succeed
	schema := YANGSchemaWithPlugins(pluginYANG)
	require.NotNil(t, schema, "schema should load")

	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err, "parsing should succeed with plugin YANG")

	cfg, err := TreeToConfig(tree)
	require.NoError(t, err)
	require.Len(t, cfg.Peers, 1)

	peer := cfg.Peers[0]

	// Verify RawCapabilityConfig for plugin delivery
	require.NotNil(t, peer.RawCapabilityConfig, "RawCapabilityConfig should be populated")
	require.NotNil(t, peer.RawCapabilityConfig["hostname"], "hostname scope should exist")
	assert.Equal(t, "my-host-name", peer.RawCapabilityConfig["hostname"]["host"])
	assert.Equal(t, "my-domain.com", peer.RawCapabilityConfig["hostname"]["domain"])
}

// TestHostnameNewSyntax verifies the new capability { hostname { ... } } syntax.
//
// VALIDATES: New syntax capability { hostname { host ...; domain ...; } } populates CapabilityConfigJSON.
// PREVENTS: New syntax not being extracted for plugin delivery.
func TestHostnameNewSyntax(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        capability {
            hostname {
                host my-host-name;
                domain my-domain.com;
            }
        }
    }
}
`
	// Load plugin YANG
	pluginYANG := map[string]string{
		"ze-hostname.yang": `module ze-hostname {
    namespace "urn:ze:hostname";
    prefix hostname;
    import ze-bgp-conf { prefix bgp; }
    revision 2025-01-29 { description "Test"; }
    augment "/bgp:bgp/bgp:peer/bgp:capability" {
        container hostname {
            leaf host { type string; }
            leaf domain { type string; }
        }
    }
}`,
	}

	// With plugin YANG, parsing should succeed
	schema := YANGSchemaWithPlugins(pluginYANG)
	require.NotNil(t, schema, "schema should load")

	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err, "parsing should succeed with plugin YANG")

	cfg, err := TreeToConfig(tree)
	require.NoError(t, err)
	require.Len(t, cfg.Peers, 1)

	peer := cfg.Peers[0]

	// Verify CapabilityConfigJSON for plugin delivery (new JSON-based approach)
	require.NotEmpty(t, peer.CapabilityConfigJSON, "CapabilityConfigJSON should be populated")
	assert.Contains(t, peer.CapabilityConfigJSON, `"hostname"`)
	assert.Contains(t, peer.CapabilityConfigJSON, `"host":"my-host-name"`)
	assert.Contains(t, peer.CapabilityConfigJSON, `"domain":"my-domain.com"`)
}

// TestHoldTimeZeroPreserved verifies hold-time 0 is preserved, not defaulted.
//
// VALIDATES: RFC 4271 allows hold-time 0 (disables keepalives).
// PREVENTS: Explicit hold-time 0 being overwritten with default 90s.
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
    local-as 65000;
    peer 192.0.2.1 {
        peer-as 65001;
        hold-time 0;
    }
}`,
			wantHoldTime: 0,
		},
		{
			name: "unset_defaults_to_90",
			config: `
bgp {
    router-id 10.0.0.1;
    local-as 65000;
    peer 192.0.2.1 {
        peer-as 65001;
    }
}`,
			wantHoldTime: 90,
		},
		{
			name: "explicit_30_preserved",
			config: `
bgp {
    router-id 10.0.0.1;
    local-as 65000;
    peer 192.0.2.1 {
        peer-as 65001;
        hold-time 30;
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
			gotSec := int(settings.HoldTime.Seconds())
			assert.Equal(t, tt.wantHoldTime, gotSec, "hold-time mismatch")
		})
	}
}
