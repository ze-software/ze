package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	grschema "codeberg.org/thomas-mangin/ze/internal/plugin/gr/schema"
)

// schemaWithGR returns a YANG schema with the GR plugin YANG loaded.
// Use this for tests that need graceful-restart config.
func schemaWithGR() *Schema {
	return YANGSchemaWithPlugins(map[string]string{
		"ze-graceful-restart.yang": grschema.ZeGracefulRestartYANG,
	})
}

// TestBGPSchemaNeighbor verifies group configuration parsing.
//
// VALIDATES: Full group config parses correctly.
//
// PREVENTS: Missing or broken group fields.
func TestBGPSchemaNeighbor(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        description "Transit Provider";
        router-id 1.2.3.4;
        local-address 192.0.2.2;
        local-as 65000;
        peer-as 65001;
        hold-time 90;
        passive false;
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	bgpContainer := tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer)

	neighbors := bgpContainer.GetList("peer")
	require.Len(t, neighbors, 1)

	n := neighbors["192.0.2.1"]
	require.NotNil(t, n)

	val, _ := n.Get("description")
	require.Equal(t, "Transit Provider", val)

	val, _ = n.Get("local-as")
	require.Equal(t, "65000", val)

	val, _ = n.Get("peer-as")
	require.Equal(t, "65001", val)

	val, _ = n.Get("hold-time")
	require.Equal(t, "90", val)
}

// TestBGPSchemaFamily verifies address family configuration.
//
// VALIDATES: Family/AFI/SAFI config parses correctly.
//
// PREVENTS: Broken multiprotocol support.
func TestBGPSchemaFamily(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        family {
            ipv4/unicast;
            ipv4/multicast;
            ipv6/unicast;
        }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	bgpContainer := tree.GetContainer("bgp")
	neighbors := bgpContainer.GetList("peer")
	n := neighbors["192.0.2.1"]

	family := n.GetContainer("family")
	require.NotNil(t, family)

	// Families stored as "afi safi" -> true
	val, ok := family.Get("ipv4/unicast")
	require.True(t, ok)
	require.Equal(t, "true", val)

	val, ok = family.Get("ipv4/multicast")
	require.True(t, ok)
	require.Equal(t, "true", val)

	val, ok = family.Get("ipv6/unicast")
	require.True(t, ok)
	require.Equal(t, "true", val)
}

// TestBGPSchemaFamilyIgnoreMismatch verifies ignore-mismatch parsing in family block.
//
// VALIDATES: ignore-mismatch option is parsed from family config.
//
// PREVENTS: Unable to configure lenient mode for buggy peers.
func TestBGPSchemaFamilyIgnoreMismatch(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name: "ignore-mismatch enabled",
			input: `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        family {
            ipv4/unicast;
            ignore-mismatch enable;
        }
    }
}`,
			expected: true,
		},
		{
			name: "ignore-mismatch true",
			input: `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        family {
            ipv4/unicast;
            ignore-mismatch true;
        }
    }
}`,
			expected: true,
		},
		{
			name: "ignore-mismatch disabled",
			input: `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        family {
            ipv4/unicast;
            ignore-mismatch disable;
        }
    }
}`,
			expected: false,
		},
		{
			name: "ignore-mismatch not specified (default false)",
			input: `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        family {
            ipv4/unicast;
        }
    }
}`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(YANGSchema())
			tree, err := p.Parse(tt.input)
			require.NoError(t, err)

			cfg, err := TreeToConfig(tree)
			require.NoError(t, err)
			require.Len(t, cfg.Peers, 1)
			require.Equal(t, tt.expected, cfg.Peers[0].IgnoreFamilyMismatch)
		})
	}
}

// TestFamilyModeTypes verifies FamilyMode type parsing.
//
// VALIDATES: FamilyMode parses enable/disable/require correctly.
//
// PREVENTS: Wrong mode assignment for family configuration.
func TestFamilyModeTypes(t *testing.T) {
	tests := []struct {
		input    string
		expected FamilyMode
	}{
		{"", FamilyModeEnable},
		{"true", FamilyModeEnable},
		{"enable", FamilyModeEnable},
		{"false", FamilyModeDisable},
		{"disable", FamilyModeDisable},
		{"require", FamilyModeRequire},
		{"REQUIRE", FamilyModeRequire}, // case insensitive
		{"Enable", FamilyModeEnable},   // case insensitive
		{"ignore", FamilyModeIgnore},
		{"IGNORE", FamilyModeIgnore},  // case insensitive
		{"unknown", FamilyModeEnable}, // unknown defaults to enable
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseFamilyMode(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestFamilyModeString verifies FamilyMode.String().
//
// VALIDATES: FamilyMode converts to string correctly.
//
// PREVENTS: Broken logging/serialization of family modes.
func TestFamilyModeString(t *testing.T) {
	require.Equal(t, "enable", FamilyModeEnable.String())
	require.Equal(t, "disable", FamilyModeDisable.String())
	require.Equal(t, "require", FamilyModeRequire.String())
	require.Equal(t, "ignore", FamilyModeIgnore.String())
	require.Equal(t, "unknown", FamilyMode(99).String())
}

// TestFamilyConfigInlineWithMode verifies inline family syntax with mode.
//
// VALIDATES: "ipv4/unicast require;" parses to FamilyConfig with Mode=Require.
//
// PREVENTS: Unable to require specific address families.
func TestFamilyConfigInlineWithMode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []FamilyConfig
	}{
		{
			name: "ipv4/unicast require",
			input: `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        family {
            ipv4/unicast require;
        }
    }
}`,
			expected: []FamilyConfig{
				{AFI: "ipv4", SAFI: "unicast", Mode: FamilyModeRequire},
			},
		},
		{
			name: "ipv6/unicast disable",
			input: `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        family {
            ipv6/unicast disable;
        }
    }
}`,
			expected: []FamilyConfig{
				{AFI: "ipv6", SAFI: "unicast", Mode: FamilyModeDisable},
			},
		},
		{
			name: "mixed modes inline",
			input: `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        family {
            ipv4/unicast;
            ipv4/multicast require;
            ipv6/unicast disable;
        }
    }
}`,
			expected: []FamilyConfig{
				{AFI: "ipv4", SAFI: "unicast", Mode: FamilyModeEnable},
				{AFI: "ipv4", SAFI: "multicast", Mode: FamilyModeRequire},
				{AFI: "ipv6", SAFI: "unicast", Mode: FamilyModeDisable},
			},
		},
		{
			name: "ignore mode",
			input: `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        family {
            ipv4/unicast ignore;
            ipv6/unicast;
        }
    }
}`,
			expected: []FamilyConfig{
				{AFI: "ipv4", SAFI: "unicast", Mode: FamilyModeIgnore},
				{AFI: "ipv6", SAFI: "unicast", Mode: FamilyModeEnable},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(YANGSchema())
			tree, err := p.Parse(tt.input)
			require.NoError(t, err)

			cfg, err := TreeToConfig(tree)
			require.NoError(t, err)
			require.Len(t, cfg.Peers, 1)
			require.ElementsMatch(t, tt.expected, cfg.Peers[0].FamilyConfigs)
		})
	}
}

// TestFamilyConfigBlockSyntax verifies block family syntax.
//
// VALIDATES: "ipv4 { unicast; multicast require; }" parses correctly.
//
// PREVENTS: Unable to group SAFIs under single AFI block.
func TestFamilyConfigBlockSyntax(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []FamilyConfig
	}{
		{
			name: "single SAFI block",
			input: `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        family {
            ipv4 {
                unicast;
            }
        }
    }
}`,
			expected: []FamilyConfig{
				{AFI: "ipv4", SAFI: "unicast", Mode: FamilyModeEnable},
			},
		},
		{
			name: "multiple SAFIs block",
			input: `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        family {
            ipv4 {
                unicast;
                multicast require;
            }
        }
    }
}`,
			expected: []FamilyConfig{
				{AFI: "ipv4", SAFI: "unicast", Mode: FamilyModeEnable},
				{AFI: "ipv4", SAFI: "multicast", Mode: FamilyModeRequire},
			},
		},
		{
			name:  "one-liner block",
			input: `bgp { peer 192.0.2.1 { local-as 65000; peer-as 65001; family { ipv6 { unicast require; mpls-vpn } } } }`,
			expected: []FamilyConfig{
				{AFI: "ipv6", SAFI: "unicast", Mode: FamilyModeRequire},
				{AFI: "ipv6", SAFI: "mpls-vpn", Mode: FamilyModeEnable},
			},
		},
		{
			name: "multiple AFI blocks",
			input: `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        family {
            ipv4 {
                unicast require;
            }
            ipv6 {
                unicast;
                mpls-vpn require;
            }
        }
    }
}`,
			expected: []FamilyConfig{
				{AFI: "ipv4", SAFI: "unicast", Mode: FamilyModeRequire},
				{AFI: "ipv6", SAFI: "unicast", Mode: FamilyModeEnable},
				{AFI: "ipv6", SAFI: "mpls-vpn", Mode: FamilyModeRequire},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(YANGSchema())
			tree, err := p.Parse(tt.input)
			require.NoError(t, err)

			cfg, err := TreeToConfig(tree)
			require.NoError(t, err)
			require.Len(t, cfg.Peers, 1)
			require.ElementsMatch(t, tt.expected, cfg.Peers[0].FamilyConfigs)
		})
	}
}

// TestFamilyConfigMixedSyntax verifies mixed inline and block syntax.
//
// VALIDATES: Inline and block family syntax can coexist.
//
// PREVENTS: Parser confusion when mixing syntax styles.
func TestFamilyConfigMixedSyntax(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        family {
            ipv4/unicast;
            ipv6 {
                unicast require;
                mpls-vpn;
            }
            l2vpn/evpn require;
        }
    }
}`
	expected := []FamilyConfig{
		{AFI: "ipv4", SAFI: "unicast", Mode: FamilyModeEnable},
		{AFI: "ipv6", SAFI: "unicast", Mode: FamilyModeRequire},
		{AFI: "ipv6", SAFI: "mpls-vpn", Mode: FamilyModeEnable},
		{AFI: "l2vpn", SAFI: "evpn", Mode: FamilyModeRequire},
	}

	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, err := TreeToConfig(tree)
	require.NoError(t, err)
	require.Len(t, cfg.Peers, 1)
	require.ElementsMatch(t, expected, cfg.Peers[0].FamilyConfigs)
}

// TestBGPSchemaCapability verifies capability configuration.
//
// VALIDATES: BGP capabilities are parsed.
//
// PREVENTS: Missing capability negotiation config.
func TestBGPSchemaCapability(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        capability {
            asn4 true;
            route-refresh;
            graceful-restart {
                restart-time 120;
            }
            add-path {
                send true;
                receive true;
            }
        }
    }
}
`
	p := NewParser(schemaWithGR()) // Use schema with GR plugin YANG
	tree, err := p.Parse(input)
	require.NoError(t, err)

	bgpContainer := tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer)

	neighbors := bgpContainer.GetList("peer")
	n := neighbors["192.0.2.1"]

	cap := n.GetContainer("capability")
	require.NotNil(t, cap)

	val, _ := cap.Get("asn4")
	require.Equal(t, "true", val)

	grCap := cap.GetContainer("graceful-restart")
	require.NotNil(t, grCap)

	val, _ = grCap.Get("restart-time")
	require.Equal(t, "120", val)

	addPath := cap.GetContainer("add-path")
	require.NotNil(t, addPath)

	val, _ = addPath.Get("send")
	require.Equal(t, "true", val)
}

// TestBGPSchemaProcess verifies API process configuration.
//
// VALIDATES: External process config parses correctly.
//
// PREVENTS: Broken API integration.
func TestBGPSchemaProcess(t *testing.T) {
	input := `
plugin {
    external announce-routes {
        run "/usr/local/bin/exabgp-announce";
        encoder json;
    }
    external receive-routes {
        run "/usr/local/bin/exabgp-receive";
        encoder text;
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	pluginContainer := tree.GetContainer("plugin")
	require.NotNil(t, pluginContainer)

	plugins := pluginContainer.GetList("external")
	require.Len(t, plugins, 2)

	p1 := plugins["announce-routes"]
	val, _ := p1.Get("run")
	require.Equal(t, "/usr/local/bin/exabgp-announce", val)

	val, _ = p1.Get("encoder")
	require.Equal(t, "json", val)
}

// TestBGPSchemaGlobal verifies global settings.
//
// VALIDATES: Global BGP settings parse correctly.
//
// PREVENTS: Missing global config.
func TestBGPSchemaGlobal(t *testing.T) {
	input := `
bgp {
    router-id 1.2.3.4;
    local-as 65000;
    listen 0.0.0.0 179;
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	bgpContainer := tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer)

	val, ok := bgpContainer.Get("router-id")
	require.True(t, ok)
	require.Equal(t, "1.2.3.4", val)

	val, ok = bgpContainer.Get("local-as")
	require.True(t, ok)
	require.Equal(t, "65000", val)

	val, ok = bgpContainer.Get("listen")
	require.True(t, ok)
	require.Equal(t, "0.0.0.0 179", val)
}

// TestBGPSchemaToConfig verifies conversion to typed Config.
//
// VALIDATES: Tree converts to strongly-typed Config struct.
//
// PREVENTS: Type conversion errors.
func TestBGPSchemaToConfig(t *testing.T) {
	input := `
bgp {
    router-id 10.0.0.1;
    local-as 65000;

    peer 192.0.2.1 {
        local-address 192.0.2.2;
        peer-as 65001;
        hold-time 90;
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, err := TreeToConfig(tree)
	require.NoError(t, err)

	require.Equal(t, uint32(0x0a000001), cfg.RouterID) // 10.0.0.1
	require.Equal(t, uint32(65000), cfg.LocalAS)

	require.Len(t, cfg.Peers, 1)
	n := cfg.Peers[0]
	require.Equal(t, "192.0.2.1", n.Address.String())
	require.Equal(t, uint32(65001), n.PeerAS)
	require.Equal(t, uint16(90), n.HoldTime)
}

// TestHoldTimeRFCValidation verifies that hold-time 1-2 seconds is rejected.
//
// RFC 4271 Section 4.2: "Hold Time MUST be either zero or at least three seconds."
//
// VALIDATES: Config rejects invalid hold times (1-2 seconds) per RFC 4271.
//
// PREVENTS: Configuration of non-compliant hold times.
func TestHoldTimeRFCValidation(t *testing.T) {
	tests := []struct {
		name      string
		holdTime  string
		wantError bool
	}{
		{"zero is valid", "0", false},
		{"one is invalid", "1", true},
		{"two is invalid", "2", true},
		{"three is valid", "3", false},
		{"ninety is valid", "90", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        hold-time ` + tt.holdTime + `;
    }
}
`
			p := NewParser(YANGSchema())
			tree, err := p.Parse(input)
			require.NoError(t, err)

			_, err = TreeToConfig(tree)
			if tt.wantError {
				require.Error(t, err, "hold-time %s should be rejected", tt.holdTime)
				require.Contains(t, err.Error(), "hold-time")
			} else {
				require.NoError(t, err, "hold-time %s should be valid", tt.holdTime)
			}
		})
	}
}

// TestLocalAddressAuto verifies 'auto' keyword for local-address.
//
// VALIDATES: local-address 'auto' is parsed as special value for auto-binding.
//
// PREVENTS: Unable to configure automatic local address selection.
func TestLocalAddressAuto(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        local-address auto;
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, err := TreeToConfig(tree)
	require.NoError(t, err)

	require.Len(t, cfg.Peers, 1)
	n := cfg.Peers[0]
	require.True(t, n.LocalAddressAuto, "local-address auto should set LocalAddressAuto=true")
	require.False(t, n.LocalAddress.IsValid(), "local-address auto should leave LocalAddress unset")
}

// TestExtendedMessageCapabilityConfig verifies extended-message capability config.
//
// RFC 8654 Extended Message Support for BGP.
//
// VALIDATES: extended-message capability can be enabled/disabled in config.
//
// PREVENTS: Unable to control extended message negotiation.
func TestExtendedMessageCapabilityConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		expected bool
	}{
		{
			"explicit enable",
			`capability { extended-message enable; }`,
			true,
		},
		{
			"explicit disable",
			`capability { extended-message disable; }`,
			false,
		},
		{
			"true value",
			`capability { extended-message true; }`,
			true,
		},
		{
			"false value",
			`capability { extended-message false; }`,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        ` + tt.config + `
    }
}
`
			p := NewParser(YANGSchema())
			tree, err := p.Parse(input)
			require.NoError(t, err)

			cfg, err := TreeToConfig(tree)
			require.NoError(t, err)

			require.Len(t, cfg.Peers, 1)
			n := cfg.Peers[0]
			require.Equal(t, tt.expected, n.Capabilities.ExtendedMessage,
				"extended-message capability mismatch")
		})
	}
}

// TestPerFamilyAddPathConfig verifies per-family add-path configuration.
//
// RFC 7911 ADD-PATH capability per AFI/SAFI.
//
// VALIDATES: add-path can be configured per address family.
//
// PREVENTS: Global add-path setting applying to all families.
func TestPerFamilyAddPathConfig(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        add-path {
            ipv4/unicast send;
            ipv6/unicast receive;
            ipv4/multicast send/receive;
        }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, err := TreeToConfig(tree)
	require.NoError(t, err)

	require.Len(t, cfg.Peers, 1)
	n := cfg.Peers[0]

	// Check per-family add-path settings
	require.Len(t, n.AddPathFamilies, 3, "expected 3 family add-path configs")

	// Find configs by family
	familyMap := make(map[string]AddPathFamilyConfig)
	for _, f := range n.AddPathFamilies {
		familyMap[f.Family] = f
	}

	ipv4Uni, ok := familyMap["ipv4/unicast"]
	require.True(t, ok, "missing ipv4/unicast add-path config")
	require.True(t, ipv4Uni.Send, "ipv4/unicast should have send")
	require.False(t, ipv4Uni.Receive, "ipv4/unicast should not have receive")

	ipv6Uni, ok := familyMap["ipv6/unicast"]
	require.True(t, ok, "missing ipv6/unicast add-path config")
	require.False(t, ipv6Uni.Send, "ipv6/unicast should not have send")
	require.True(t, ipv6Uni.Receive, "ipv6/unicast should have receive")

	ipv4Multi, ok := familyMap["ipv4/multicast"]
	require.True(t, ok, "missing ipv4/multicast add-path config")
	require.True(t, ipv4Multi.Send, "ipv4/multicast should have send")
	require.True(t, ipv4Multi.Receive, "ipv4/multicast should have receive")
}

// TestASN4DefaultEnabled verifies ASN4 capability is enabled by default.
//
// VALIDATES: Configs without explicit asn4 setting get ASN4 enabled.
//
// PREVENTS: Missing 4-byte AS capability in OPEN messages.
func TestASN4DefaultEnabled(t *testing.T) {
	// Config without capability block
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, err := TreeToConfig(tree)
	require.NoError(t, err)

	require.Len(t, cfg.Peers, 1)
	require.True(t, cfg.Peers[0].Capabilities.ASN4, "ASN4 should be enabled by default")
}

// TestASN4ExplicitlyDisabled verifies ASN4 can be disabled.
//
// VALIDATES: Explicit asn4 false disables the capability.
//
// PREVENTS: Unable to connect to peers that don't support 4-byte AS.
func TestASN4ExplicitlyDisabled(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        capability {
            asn4 false;
        }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, err := TreeToConfig(tree)
	require.NoError(t, err)

	require.Len(t, cfg.Peers, 1)
	require.False(t, cfg.Peers[0].Capabilities.ASN4, "ASN4 should be disabled when explicitly set to false")
}

// TestRIBOutConfigAutoCommitDelayFormats verifies duration parsing in per-group rib.
//
// VALIDATES: Auto-commit-delay accepts various duration formats.
//
// PREVENTS: Configuration errors from different duration syntaxes.
func TestRIBOutConfigAutoCommitDelayFormats(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
	}{
		{"milliseconds", "100ms", 100 * time.Millisecond},
		{"seconds", "5s", 5 * time.Second},
		{"fractional seconds", "0.5s", 500 * time.Millisecond},
		{"zero", "0", 0},
		{"zero ms", "0ms", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        rib {
            out {
                auto-commit-delay ` + tt.input + `;
            }
        }
    }
}
`
			p := NewParser(YANGSchema())
			tree, err := p.Parse(input)
			require.NoError(t, err)

			cfg, err := TreeToConfig(tree)
			require.NoError(t, err)

			require.Len(t, cfg.Peers, 1)
			require.Equal(t, tt.expected, cfg.Peers[0].RIBOut.AutoCommitDelay)
		})
	}
}

// parseConfig parses BGP config with the standard YANG schema.
func parseConfig(t *testing.T, input string) *BGPConfig {
	t.Helper()
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)
	cfg, err := TreeToConfig(tree)
	require.NoError(t, err)
	return cfg
}

// parseConfigWithGR parses config using schema with GR plugin YANG.
func parseConfigWithGR(t *testing.T, input string) *BGPConfig {
	t.Helper()
	p := NewParser(schemaWithGR())
	tree, err := p.Parse(input)
	require.NoError(t, err)
	cfg, err := TreeToConfig(tree)
	require.NoError(t, err)
	return cfg
}

// peersByAddr returns a map of neighbors keyed by IP address string.
func peersByAddr(neighbors []PeerConfig) map[string]PeerConfig {
	m := make(map[string]PeerConfig)
	for _, n := range neighbors {
		m[n.Address.String()] = n
	}
	return m
}
