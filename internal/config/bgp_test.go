package config

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
	p := NewParser(YANGSchema())
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

	gr := cap.GetContainer("graceful-restart")
	require.NotNil(t, gr)

	val, _ = gr.Get("restart-time")
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

// TestPerNeighborRIBOut verifies per-group rib { out { ... } } configuration.
//
// VALIDATES: Per-group RIBOut config, template inheritance, defaults, legacy group-updates.
//
// PREVENTS: Unable to configure batching per-peer or broken backward compatibility.
func TestPerNeighborRIBOut(t *testing.T) {
	t.Run("explicit config", func(t *testing.T) {
		input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        rib { out { group-updates true; auto-commit-delay 100ms; max-batch-size 500; } }
    }
    peer 192.0.2.2 {
        local-as 65000;
        peer-as 65002;
        rib { out { group-updates false; auto-commit-delay 50ms; } }
    }
}
`
		cfg := parseConfig(t, input)
		require.Len(t, cfg.Peers, 2)
		m := peersByAddr(cfg.Peers)

		require.True(t, m["192.0.2.1"].RIBOut.GroupUpdates)
		require.Equal(t, 100*time.Millisecond, m["192.0.2.1"].RIBOut.AutoCommitDelay)
		require.Equal(t, 500, m["192.0.2.1"].RIBOut.MaxBatchSize)

		require.False(t, m["192.0.2.2"].RIBOut.GroupUpdates)
		require.Equal(t, 50*time.Millisecond, m["192.0.2.2"].RIBOut.AutoCommitDelay)
		require.Equal(t, 0, m["192.0.2.2"].RIBOut.MaxBatchSize)
	})

	t.Run("defaults", func(t *testing.T) {
		input := `bgp { peer 192.0.2.1 { local-as 65000; peer-as 65001; } }`
		cfg := parseConfig(t, input)
		require.Len(t, cfg.Peers, 1)
		n := cfg.Peers[0]
		require.True(t, n.RIBOut.GroupUpdates)
		require.Equal(t, time.Duration(0), n.RIBOut.AutoCommitDelay)
		require.Equal(t, 0, n.RIBOut.MaxBatchSize)
	})

	t.Run("template inheritance", func(t *testing.T) {
		input := `
template { group rib-tmpl { rib { out { group-updates true; auto-commit-delay 200ms; max-batch-size 1000; } } } }
bgp {
    peer 192.0.2.1 { inherit rib-tmpl; local-as 65000; peer-as 65001; }
    peer 192.0.2.2 { inherit rib-tmpl; local-as 65000; peer-as 65002; rib { out { auto-commit-delay 50ms; } } }
}
`
		cfg := parseConfig(t, input)
		require.Len(t, cfg.Peers, 2)
		m := peersByAddr(cfg.Peers)

		// n1: full template inheritance
		require.True(t, m["192.0.2.1"].RIBOut.GroupUpdates)
		require.Equal(t, 200*time.Millisecond, m["192.0.2.1"].RIBOut.AutoCommitDelay)
		require.Equal(t, 1000, m["192.0.2.1"].RIBOut.MaxBatchSize)

		// n2: template with override
		require.True(t, m["192.0.2.2"].RIBOut.GroupUpdates)
		require.Equal(t, 50*time.Millisecond, m["192.0.2.2"].RIBOut.AutoCommitDelay)
		require.Equal(t, 1000, m["192.0.2.2"].RIBOut.MaxBatchSize)
	})

	t.Run("legacy group-updates", func(t *testing.T) {
		input := `bgp { peer 192.0.2.1 { local-as 65000; peer-as 65001; group-updates false; } }`
		cfg := parseConfig(t, input)
		require.Len(t, cfg.Peers, 1)
		n := cfg.Peers[0]
		require.False(t, n.GroupUpdates)
		require.False(t, n.RIBOut.GroupUpdates)
	})
}

// parseConfig is a test helper that parses BGP config and returns the result.
func parseConfig(t *testing.T, input string) *BGPConfig {
	t.Helper()
	p := NewParser(YANGSchema())
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

// TestIPGlobMatch verifies IP glob pattern matching.
//
// VALIDATES: IP glob patterns correctly match IP addresses.
//
// PREVENTS: Incorrect peer glob matching behavior.
func TestIPGlobMatch(t *testing.T) {
	tests := []struct {
		pattern string
		ip      string
		match   bool
	}{
		// Wildcard all
		{"*", "192.168.1.1", true},
		{"*", "10.0.0.1", true},
		{"*", "2001:db8::1", true},

		// Exact match
		{"192.168.1.1", "192.168.1.1", true},
		{"192.168.1.1", "192.168.1.2", false},

		// Single octet wildcard
		{"192.168.1.*", "192.168.1.1", true},
		{"192.168.1.*", "192.168.1.255", true},
		{"192.168.1.*", "192.168.2.1", false},

		// Middle octet wildcard
		{"192.168.*.1", "192.168.0.1", true},
		{"192.168.*.1", "192.168.255.1", true},
		{"192.168.*.1", "192.168.1.2", false},

		// Multiple wildcards
		{"192.*.*.*", "192.0.0.1", true},
		{"192.*.*.*", "192.255.255.255", true},
		{"192.*.*.*", "10.0.0.1", false},

		// First octet wildcard
		{"*.168.1.1", "192.168.1.1", true},
		{"*.168.1.1", "10.168.1.1", true},
		{"*.168.1.1", "192.169.1.1", false},

		// IPv6 - just exact and wildcard all
		{"2001:db8::1", "2001:db8::1", true},
		{"2001:db8::1", "2001:db8::2", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.ip, func(t *testing.T) {
			result := IPGlobMatch(tt.pattern, tt.ip)
			require.Equal(t, tt.match, result)
		})
	}
}

// =============================================================================
// V3 SYNTAX TESTS: template { group ... } and template { match ... }
// =============================================================================

// TestTemplateGroupBasic verifies template { group <name> { } } syntax.
//
// VALIDATES: Named templates can be defined using "group" instead of "neighbor".
//
// PREVENTS: Unable to use group syntax for named templates.
func TestTemplateGroupBasic(t *testing.T) {
	input := `
plugin { external rib { run ./rib; } }
template {
    group ibgp-rr {
        peer-as 65000;
        hold-time 60;
        capability { route-refresh; }
        process rib { send { update; } }
    }
}

bgp {
    peer 192.0.2.1 {
        inherit ibgp-rr;
        local-as 65000;
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	n := cfg.Peers[0]
	require.Equal(t, uint32(65000), n.PeerAS)
	require.Equal(t, uint16(60), n.HoldTime)
	require.True(t, n.Capabilities.RouteRefresh)
}

// TestTemplateMatchBasic verifies template { match <pattern> { } } syntax.
//
// VALIDATES: Glob patterns can be defined using "match" inside template block.
//
// PREVENTS: Unable to use match syntax for glob patterns.
func TestTemplateMatchBasic(t *testing.T) {
	input := `
template {
    match * {
        rib { out { group-updates false; auto-commit-delay 100ms; } }
    }
}

bgp {
    peer 192.0.2.1 { local-as 65000; peer-as 65001; }
    peer 192.0.2.2 { local-as 65000; peer-as 65002; }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 2)

	for _, n := range cfg.Peers {
		require.False(t, n.RIBOut.GroupUpdates, "match * should apply to %s", n.Address)
		require.Equal(t, 100*time.Millisecond, n.RIBOut.AutoCommitDelay)
	}
}

// TestTemplateMatchOrder verifies match blocks are applied in config order.
//
// VALIDATES: match blocks are applied in the order they appear in the config,
// allowing later matches to override earlier ones.
//
// PREVENTS: Unexpected behavior from specificity-based ordering.
func TestTemplateMatchOrder(t *testing.T) {
	input := `
template {
    match * {
        hold-time 90;
        rib { out { group-updates true; } }
    }
    match 192.0.2.* {
        hold-time 60;
        rib { out { auto-commit-delay 50ms; } }
    }
}

bgp {
    peer 192.0.2.1 { local-as 65000; peer-as 65001; }
    peer 10.0.0.1 { local-as 65000; peer-as 65002; }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 2)
	m := peersByAddr(cfg.Peers)

	// 192.0.2.1: match * first, then match 192.0.2.* overrides hold-time
	require.Equal(t, uint16(60), m["192.0.2.1"].HoldTime)
	require.True(t, m["192.0.2.1"].RIBOut.GroupUpdates)
	require.Equal(t, 50*time.Millisecond, m["192.0.2.1"].RIBOut.AutoCommitDelay)

	// 10.0.0.1: only match * applies
	require.Equal(t, uint16(90), m["10.0.0.1"].HoldTime)
	require.True(t, m["10.0.0.1"].RIBOut.GroupUpdates)
	require.Equal(t, time.Duration(0), m["10.0.0.1"].RIBOut.AutoCommitDelay)
}

// TestTemplateMatchConfigOrderNotSpecificity verifies config order, NOT specificity.
//
// VALIDATES: More specific match appearing BEFORE less specific is applied first,
// and less specific (appearing later) OVERRIDES it. Config order, not specificity!
//
// PREVENTS: Specificity-based ordering instead of config-file ordering.
func TestTemplateMatchConfigOrderNotSpecificity(t *testing.T) {
	// Critical: specific match BEFORE general match
	// Per plan: config order, so 10.0.0.0/8 applies first, then * overrides
	input := `
template {
    match 10.0.0.0/8 {
        hold-time 60;
    }
    match * {
        hold-time 90;
    }
}

bgp {
    peer 10.0.0.1 { local-as 65000; peer-as 65001; }
    peer 192.168.1.1 { local-as 65000; peer-as 65002; }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 2)
	m := peersByAddr(cfg.Peers)

	// 10.0.0.1: match 10.0.0.0/8 first (hold-time=60), then match * (hold-time=90 overrides)
	// Config order means later match wins, regardless of specificity!
	require.Equal(t, uint16(90), m["10.0.0.1"].HoldTime, "config order: * should override 10.0.0.0/8")

	// 192.168.1.1: only match * applies
	require.Equal(t, uint16(90), m["192.168.1.1"].HoldTime)
}

// TestTemplateGroupAndMatchCombined verifies combined group and match usage.
//
// VALIDATES: Both group and match can be used together with proper precedence.
// Order: match (config order) → group (inherit order) → peer
//
// PREVENTS: Incorrect precedence when mixing group and match.
func TestTemplateGroupAndMatchCombined(t *testing.T) {
	input := `
template {
    match * {
        hold-time 90;
    }
    match 192.0.2.* {
        hold-time 60;
    }
    group high-preference {
        rib { out { auto-commit-delay 100ms; } }
    }
}

bgp {
    peer 192.0.2.1 {
        inherit high-preference;
        local-as 65000;
        peer-as 65001;
    }
    peer 10.0.0.1 {
        local-as 65000;
        peer-as 65002;
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 2)
	m := peersByAddr(cfg.Peers)

	// 192.0.2.1: match * (hold-time 90), match 192.0.2.* (hold-time 60), inherit high-preference
	require.Equal(t, uint16(60), m["192.0.2.1"].HoldTime)
	require.Equal(t, 100*time.Millisecond, m["192.0.2.1"].RIBOut.AutoCommitDelay)

	// 10.0.0.1: only match * applies, no inherit
	require.Equal(t, uint16(90), m["10.0.0.1"].HoldTime)
	require.Equal(t, time.Duration(0), m["10.0.0.1"].RIBOut.AutoCommitDelay)
}

// TestPeerKeywordForSessions verifies "peer" keyword works for BGP sessions.
//
// VALIDATES: "peer <IP> { }" syntax works as alias for "neighbor <IP> { }".
//
// PREVENTS: Unable to use peer syntax for BGP sessions.
func TestPeerKeywordForSessions(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        hold-time 90;
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	n := cfg.Peers[0]
	require.Equal(t, "192.0.2.1", n.Address.String())
	require.Equal(t, uint32(65000), n.LocalAS)
	require.Equal(t, uint32(65001), n.PeerAS)
	require.Equal(t, uint16(90), n.HoldTime)
}

// TestSingleInheritance verifies single inherit statement works.
//
// VALIDATES: inherit statement applies template settings to peer.
//
// PREVENTS: Template inheritance not working.
func TestSingleInheritance(t *testing.T) {
	input := `
plugin { external rib { run ./rib; } }
template {
    group ibgp-defaults {
        hold-time 60;
        peer-as 65000;
        capability { route-refresh; }
        process rib { send { update; } }
    }
}

bgp {
    peer 192.0.2.1 {
        inherit ibgp-defaults;
        local-as 65000;
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	n := cfg.Peers[0]

	// Template settings applied
	require.Equal(t, uint16(60), n.HoldTime)
	require.Equal(t, uint32(65000), n.PeerAS)
	require.True(t, n.Capabilities.RouteRefresh)
	// Peer settings
	require.Equal(t, uint32(65000), n.LocalAS)
}

// =============================================================================
// V3 ERROR CASE TESTS
// =============================================================================

// TestInheritRejectedInTemplate verifies inherit is rejected inside template.
//
// VALIDATES: inherit statements inside template { group/match } are rejected.
//
// PREVENTS: Confusing nested inheritance in templates.
func TestInheritRejectedInTemplate(t *testing.T) {
	input := `
template {
    group base {
        hold-time 90;
    }
    group derived {
        inherit base;
        hold-time 60;
    }
}
bgp {
    peer 192.0.2.1 { inherit derived; local-as 65000; peer-as 65001; }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err) // Parse succeeds

	// Validation happens in TreeToConfig
	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "inherit")
}

// TestMatchRejectedAtRoot verifies match is rejected at root level.
//
// VALIDATES: match blocks are only valid inside template { }.
//
// PREVENTS: Confusion between root-level peer globs and template matches.
func TestMatchRejectedAtRoot(t *testing.T) {
	input := `
match * {
    hold-time 90;
}
`
	p := NewParser(YANGSchema())
	_, err := p.Parse(input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "match")
}

// TestMatchRejectedInPeer verifies match is rejected inside peer blocks.
//
// VALIDATES: match blocks cannot appear inside peer { }.
//
// PREVENTS: Invalid nested match syntax.
func TestMatchRejectedInPeer(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        match * {
            hold-time 90;
        }
    }
}
`
	p := NewParser(YANGSchema())
	_, err := p.Parse(input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "match")
}

// TestGroupNameValidation verifies group name validation rules.
//
// VALIDATES: Group names must follow naming rules:
// - Start with letter
// - Contain letters, numbers, hyphens
// - Not end with hyphen
//
// PREVENTS: Invalid group names causing issues.
func TestGroupNameValidation(t *testing.T) {
	validNames := []string{"a", "ibgp", "ibgp-rr", "rr-client-v4", "Route-Reflector-1"}
	invalidNames := []string{"123", "-ibgp", "ibgp-", "ibgp_rr"}

	for _, name := range validNames {
		t.Run("valid:"+name, func(t *testing.T) {
			input := `template { group ` + name + ` { hold-time 90; } }
bgp { peer 192.0.2.1 { inherit ` + name + `; local-as 65000; peer-as 65001; } }`
			p := NewParser(YANGSchema())
			tree, err := p.Parse(input)
			require.NoError(t, err, "group name %q should parse", name)

			// Validation happens in TreeToConfig
			_, err = TreeToConfig(tree)
			require.NoError(t, err, "group name %q should be valid", name)
		})
	}

	for _, name := range invalidNames {
		t.Run("invalid:"+name, func(t *testing.T) {
			input := `template { group ` + name + ` { hold-time 90; } }
bgp { peer 192.0.2.1 { inherit ` + name + `; local-as 65000; peer-as 65001; } }`
			p := NewParser(YANGSchema())
			tree, err := p.Parse(input)
			require.NoError(t, err, "group name %q should parse", name)

			// Validation happens in TreeToConfig
			_, err = TreeToConfig(tree)
			require.Error(t, err, "group name %q should be invalid", name)
		})
	}
}

// =============================================================================
// IPV6 AND CIDR PATTERN MATCHING TESTS
// =============================================================================

// TestIPv6GlobMatch verifies IPv6 glob pattern matching.
//
// VALIDATES: IPv6 glob patterns correctly match IPv6 addresses.
//
// PREVENTS: Unable to use glob patterns with IPv6 peers.
func TestIPv6GlobMatch(t *testing.T) {
	tests := []struct {
		pattern string
		ip      string
		match   bool
	}{
		// Wildcard all
		{"*", "2001:db8::1", true},

		// Exact match
		{"2001:db8::1", "2001:db8::1", true},
		{"2001:db8::1", "2001:db8::2", false},

		// Trailing wildcard
		{"2001:db8::*", "2001:db8::1", true},
		{"2001:db8::*", "2001:db8::ffff", true},
		{"2001:db8::*", "2001:db9::1", false},

		// Prefix wildcard
		{"2001:db8:abcd::*", "2001:db8:abcd::1", true},
		{"2001:db8:abcd::*", "2001:db8:abcd::ffff:1", true},
		{"2001:db8:abcd::*", "2001:db8:abce::1", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.ip, func(t *testing.T) {
			result := IPGlobMatch(tt.pattern, tt.ip)
			require.Equal(t, tt.match, result)
		})
	}
}

// TestCIDRPatternMatch verifies CIDR notation pattern matching.
//
// VALIDATES: CIDR patterns correctly match IP addresses.
//
// PREVENTS: Unable to use CIDR notation for peer matching.
func TestCIDRPatternMatch(t *testing.T) {
	tests := []struct {
		pattern string
		ip      string
		match   bool
	}{
		// IPv4 CIDR
		{"10.0.0.0/8", "10.0.0.1", true},
		{"10.0.0.0/8", "10.255.255.255", true},
		{"10.0.0.0/8", "11.0.0.1", false},
		{"192.168.1.0/24", "192.168.1.1", true},
		{"192.168.1.0/24", "192.168.2.1", false},

		// IPv6 CIDR
		{"2001:db8::/32", "2001:db8::1", true},
		{"2001:db8::/32", "2001:db8:ffff::1", true},
		{"2001:db8::/32", "2001:db9::1", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.ip, func(t *testing.T) {
			result := IPGlobMatch(tt.pattern, tt.ip)
			require.Equal(t, tt.match, result)
		})
	}
}

// TestTemplateCIDRMatch verifies CIDR patterns work in template { match }.
//
// VALIDATES: CIDR patterns can be used in match blocks.
//
// PREVENTS: Unable to use CIDR notation in template matches.
func TestTemplateCIDRMatch(t *testing.T) {
	input := `
template {
    match 10.0.0.0/8 {
        hold-time 60;
    }
    match 192.168.0.0/16 {
        hold-time 90;
    }
}

bgp {
    peer 10.0.0.1 { local-as 65000; peer-as 65001; }
    peer 192.168.1.1 { local-as 65000; peer-as 65002; }
    peer 172.16.0.1 { local-as 65000; peer-as 65003; }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 3)
	m := peersByAddr(cfg.Peers)

	require.Equal(t, uint16(60), m["10.0.0.1"].HoldTime)
	require.Equal(t, uint16(90), m["192.168.1.1"].HoldTime)
	require.Equal(t, uint16(DefaultHoldTime), m["172.16.0.1"].HoldTime) // unset → default 90s
}

// =============================================================================
// TEMPLATE MATCH TESTS
// =============================================================================

// TestTemplateMatchConfig verifies template match patterns apply to peers.
//
// VALIDATES: template { match * { ... } } applies settings to all matching peers.
//
// PREVENTS: Unable to set defaults using template match patterns.
func TestPeerGlobConfig(t *testing.T) {
	t.Run("match * applies to all", func(t *testing.T) {
		input := `
template {
    match * {
        rib { out { group-updates false; auto-commit-delay 100ms; } }
    }
}
bgp {
    peer 192.0.2.1 { local-as 65000; peer-as 65001; }
    peer 192.0.2.2 { local-as 65000; peer-as 65002; }
}
`
		cfg := parseConfig(t, input)
		require.Len(t, cfg.Peers, 2)

		for _, n := range cfg.Peers {
			require.False(t, n.RIBOut.GroupUpdates, "match * should apply to %s", n.Address)
			require.Equal(t, 100*time.Millisecond, n.RIBOut.AutoCommitDelay)
		}
	})

	t.Run("match pattern applies to matching", func(t *testing.T) {
		input := `
template {
    match 192.0.2.* {
        rib { out { group-updates false; } }
    }
}
bgp {
    peer 192.0.2.1 { local-as 65000; peer-as 65001; }
    peer 192.0.2.2 { local-as 65000; peer-as 65002; }
    peer 10.0.0.1 { local-as 65000; peer-as 65003; }
}
`
		cfg := parseConfig(t, input)
		require.Len(t, cfg.Peers, 3)
		m := peersByAddr(cfg.Peers)

		// Matching peers get the match settings
		require.False(t, m["192.0.2.1"].RIBOut.GroupUpdates)
		require.False(t, m["192.0.2.2"].RIBOut.GroupUpdates)

		// Non-matching peer gets defaults
		require.True(t, m["10.0.0.1"].RIBOut.GroupUpdates)
	})

	t.Run("peer overrides match", func(t *testing.T) {
		input := `
template {
    match * {
        rib { out { group-updates false; auto-commit-delay 100ms; } }
    }
}
bgp {
    peer 192.0.2.1 { local-as 65000; peer-as 65001; }
    peer 192.0.2.2 {
        local-as 65000;
        peer-as 65002;
        rib { out { group-updates true; } }
    }
}
`
		cfg := parseConfig(t, input)
		require.Len(t, cfg.Peers, 2)
		m := peersByAddr(cfg.Peers)

		// First peer: match * settings
		require.False(t, m["192.0.2.1"].RIBOut.GroupUpdates)
		require.Equal(t, 100*time.Millisecond, m["192.0.2.1"].RIBOut.AutoCommitDelay)

		// Second peer: explicit override for group-updates, inherits delay
		require.True(t, m["192.0.2.2"].RIBOut.GroupUpdates)
		require.Equal(t, 100*time.Millisecond, m["192.0.2.2"].RIBOut.AutoCommitDelay)
	})

	t.Run("later match overrides earlier (config order)", func(t *testing.T) {
		input := `
template {
    match * {
        rib { out { auto-commit-delay 100ms; } }
    }
    match 192.0.2.* {
        rib { out { auto-commit-delay 50ms; } }
    }
}
bgp {
    peer 192.0.2.1 { local-as 65000; peer-as 65001; }
    peer 10.0.0.1 { local-as 65000; peer-as 65002; }
}
`
		cfg := parseConfig(t, input)
		require.Len(t, cfg.Peers, 2)
		m := peersByAddr(cfg.Peers)

		// 192.0.2.1 matches both, later match wins (config order)
		require.Equal(t, 50*time.Millisecond, m["192.0.2.1"].RIBOut.AutoCommitDelay)

		// 10.0.0.1 only matches *
		require.Equal(t, 100*time.Millisecond, m["10.0.0.1"].RIBOut.AutoCommitDelay)
	})
}

// =============================================================================
// API BINDING TESTS (Phase 1 - per-peer API configuration)
// =============================================================================
// Note: These tests use OLD syntax (api { processes [...] }) which works with Freeform().
// NEW syntax (api <name> { content {...} }) requires parser changes (Phase 2).

// TestPeerProcessBindingOldSyntax verifies API binding parsing with old syntax.
//
// VALIDATES: Config parsing extracts API bindings from processes array.
//
// PREVENTS: Silent failures when api block is malformed.
func TestPeerProcessBindingOldSyntax(t *testing.T) {
	input := `
plugin { external foo { run ./test; encoder text; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process {
            processes [ foo ];
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].ProcessBindings, 1)

	binding := cfg.Peers[0].ProcessBindings[0]
	require.Equal(t, "foo", binding.PluginName)
}

// TestAPIBindingMultipleProcesses verifies multiple processes in old syntax.
//
// VALIDATES: Multiple processes in array create multiple bindings.
//
// PREVENTS: Missing processes when multiple specified.
func TestAPIBindingMultipleProcesses(t *testing.T) {
	input := `
plugin {
    external collector { run ./collector; encoder json; }
    external logger { run ./logger; encoder text; }
}
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process {
            processes [ collector logger ];
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers[0].ProcessBindings, 2)

	// Find bindings by name
	names := make([]string, 0, len(cfg.Peers[0].ProcessBindings))
	for _, b := range cfg.Peers[0].ProcessBindings {
		names = append(names, b.PluginName)
	}
	require.Contains(t, names, "collector")
	require.Contains(t, names, "logger")
}

// TestAPIBindingNeighborChanges verifies neighbor-changes flag sets receive.State.
//
// VALIDATES: neighbor-changes; in old syntax sets receive.State = true.
//
// PREVENTS: State change events being dropped for old-style configs.
func TestAPIBindingNeighborChanges(t *testing.T) {
	input := `
plugin { external foo { run ./test; encoder text; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process {
            processes [ foo ];
            neighbor-changes;
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].ProcessBindings, 1)

	binding := cfg.Peers[0].ProcessBindings[0]
	require.Equal(t, "foo", binding.PluginName)
	require.True(t, binding.Receive.State, "neighbor-changes should set receive.State")
}

// TestAPIBindingReceiveNegotiated verifies negotiated flag in receive config.
//
// VALIDATES: receive { negotiated; } sets Receive.Negotiated = true.
//
// PREVENTS: Negotiated capabilities not being forwarded to plugins.
func TestAPIBindingReceiveNegotiated(t *testing.T) {
	input := `
plugin { external foo { run ./test; encoder json; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process foo {
            receive { negotiated; }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].ProcessBindings, 1)

	binding := cfg.Peers[0].ProcessBindings[0]
	require.Equal(t, "foo", binding.PluginName)
	require.True(t, binding.Receive.Negotiated, "receive { negotiated; } should set Receive.Negotiated")
}

// TestAPIBindingReceiveAll verifies all flag sets all receive options.
//
// VALIDATES: receive { all; } sets all Receive flags including Negotiated.
//
// PREVENTS: Missing negotiated in all shorthand.
func TestAPIBindingReceiveAll(t *testing.T) {
	input := `
plugin { external foo { run ./test; encoder json; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process foo {
            receive { all; }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].ProcessBindings, 1)

	binding := cfg.Peers[0].ProcessBindings[0]
	require.True(t, binding.Receive.Update, "all should set Update")
	require.True(t, binding.Receive.Open, "all should set Open")
	require.True(t, binding.Receive.Notification, "all should set Notification")
	require.True(t, binding.Receive.Keepalive, "all should set Keepalive")
	require.True(t, binding.Receive.Refresh, "all should set Refresh")
	require.True(t, binding.Receive.State, "all should set State")
	require.True(t, binding.Receive.Sent, "all should set Sent")
	require.True(t, binding.Receive.Negotiated, "all should set Negotiated")
}

// TestAPIBindingUndefinedProcess verifies error on undefined plugin reference.
//
// VALIDATES: Error when api references non-existent process.
//
// PREVENTS: Runtime crashes from nil process lookup.
func TestAPIBindingUndefinedProcess(t *testing.T) {
	input := `
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process {
            processes [ nonexistent ];
        }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "undefined plugin")
}

// TestEmptyAPIBlock verifies empty api block creates no bindings.
//
// VALIDATES: Empty api block (no processes) creates no bindings.
//
// PREVENTS: Crash on empty api block.
func TestEmptyAPIBlock(t *testing.T) {
	input := `
plugin { external foo { run ./test; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process { }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers[0].ProcessBindings, 0, "Empty api block should have no bindings")
}

// TestAPIBindingConfigStructs verifies the config struct fields exist.
//
// VALIDATES: PeerProcessBinding struct has all required fields.
//
// PREVENTS: Missing fields for Phase 2 new syntax support.
func TestAPIBindingConfigStructs(t *testing.T) {
	// Verify struct fields exist (compile-time check)
	binding := PeerProcessBinding{
		PluginName: "test",
		Content: PeerContentConfig{
			Encoding: "json",
			Format:   "full",
		},
		Receive: PeerReceiveConfig{
			Update:       true,
			Open:         true,
			Notification: true,
			Keepalive:    true,
			Refresh:      true,
			State:        true,
		},
		Send: PeerSendConfig{
			Update:  true,
			Refresh: true,
		},
	}
	require.Equal(t, "test", binding.PluginName)
	require.Equal(t, "json", binding.Content.Encoding)
	require.True(t, binding.Receive.Update)
	require.True(t, binding.Send.Update)
}

// =============================================================================
// NEW API SYNTAX TESTS (api <process-name> { content {...} receive {...} })
// =============================================================================

// TestPeerProcessBindingNewSyntax verifies API binding parsing with new syntax.
//
// VALIDATES: New syntax (api <name> { content {...} }) parses correctly.
//
// PREVENTS: Silent failures when using new api syntax.
func TestPeerProcessBindingNewSyntax(t *testing.T) {
	input := `
plugin { external foo { run ./test; encoder text; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process foo {
            content { encoding json; format full; }
            receive { update; notification; }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].ProcessBindings, 1)

	binding := cfg.Peers[0].ProcessBindings[0]
	require.Equal(t, "foo", binding.PluginName)
	require.Equal(t, "json", binding.Content.Encoding)
	require.Equal(t, "full", binding.Content.Format)
	require.True(t, binding.Receive.Update)
	require.True(t, binding.Receive.Notification)
	require.False(t, binding.Receive.Open) // Not specified
}

// TestReceiveAllExpansion verifies "all" keyword expands to all message types.
//
// VALIDATES: "all" keyword sets all receive flags true.
//
// PREVENTS: Missing messages when user specifies "all".
func TestReceiveAllExpansion(t *testing.T) {
	input := `
plugin { external foo { run ./test; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process foo {
            receive { all; }
        }
    }
}
`
	cfg := parseConfig(t, input)

	recv := cfg.Peers[0].ProcessBindings[0].Receive
	require.True(t, recv.Update, "all should set Update")
	require.True(t, recv.Open, "all should set Open")
	require.True(t, recv.Notification, "all should set Notification")
	require.True(t, recv.Keepalive, "all should set Keepalive")
	require.True(t, recv.Refresh, "all should set Refresh")
	require.True(t, recv.State, "all should set State")
}

// TestSendAllExpansion verifies "all" keyword in send block.
//
// VALIDATES: "all" keyword sets all send flags true.
//
// PREVENTS: Missing send capabilities when user specifies "all".
func TestSendAllExpansion(t *testing.T) {
	input := `
plugin { external foo { run ./test; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process foo {
            send { all; }
        }
    }
}
`
	cfg := parseConfig(t, input)

	send := cfg.Peers[0].ProcessBindings[0].Send
	require.True(t, send.Update, "all should set Update")
	require.True(t, send.Refresh, "all should set Refresh")
}

// TestEmptyAPIBindingNewSyntax verifies empty new-syntax api block creates binding with defaults.
//
// VALIDATES: Empty api block creates binding with process name but empty config.
//
// PREVENTS: Crash on minimal api binding.
func TestEmptyAPIBindingNewSyntax(t *testing.T) {
	input := `
plugin { external foo { run ./test; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process foo { }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers[0].ProcessBindings, 1)

	binding := cfg.Peers[0].ProcessBindings[0]
	require.Equal(t, "foo", binding.PluginName)
	require.Empty(t, binding.Content.Encoding) // Inherit from process at runtime
	require.Empty(t, binding.Content.Format)   // Default to "parsed" at runtime
	require.False(t, binding.Receive.Update)   // No messages subscribed
}

// TestAPIBindingUndefinedProcessNewSyntax verifies error on undefined plugin in new syntax.
//
// VALIDATES: Error when api references non-existent process.
//
// PREVENTS: Runtime crashes from nil process lookup.
func TestAPIBindingUndefinedProcessNewSyntax(t *testing.T) {
	input := `
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process nonexistent {
            receive { update; }
        }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "undefined plugin")
}

// TestMultipleProcessBindingsNewSyntax verifies multiple api blocks for different processes.
//
// VALIDATES: Multiple api <name> blocks create separate bindings.
//
// PREVENTS: Only first api block being parsed.
func TestMultipleProcessBindingsNewSyntax(t *testing.T) {
	input := `
plugin {
    external collector { run ./collector; encoder json; }
    external logger { run ./logger; encoder text; }
}
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process collector {
            content { encoding json; }
            receive { update; }
        }
        process logger {
            content { encoding text; format full; }
            receive { all; }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers[0].ProcessBindings, 2)

	// Find bindings by name
	var collectorBinding, loggerBinding *PeerProcessBinding
	for i := range cfg.Peers[0].ProcessBindings {
		b := &cfg.Peers[0].ProcessBindings[i]
		switch b.PluginName {
		case "collector":
			collectorBinding = b
		case "logger":
			loggerBinding = b
		}
	}

	require.NotNil(t, collectorBinding, "collector binding not found")
	require.NotNil(t, loggerBinding, "logger binding not found")

	require.Equal(t, "json", collectorBinding.Content.Encoding)
	require.True(t, collectorBinding.Receive.Update)
	require.False(t, collectorBinding.Receive.Open)

	require.Equal(t, "text", loggerBinding.Content.Encoding)
	require.Equal(t, "full", loggerBinding.Content.Format)
	require.True(t, loggerBinding.Receive.Update)
	require.True(t, loggerBinding.Receive.Open) // all expands to true
}

// =============================================================================
// TEMPLATE API BINDING INHERITANCE TESTS
// =============================================================================

// TestTemplateAPIBindingInheritance verifies API bindings are inherited from templates.
//
// VALIDATES: Peer inheriting a template gets the template's API bindings.
//
// PREVENTS: Lost API bindings when using template inheritance.
func TestTemplateAPIBindingInheritance(t *testing.T) {
	input := `
plugin { external collector { run ./collector; encoder json; } }
template {
    group api-template {
        process collector {
            content { encoding json; format full; }
            receive { update; }
        }
    }
}
bgp {
    peer 192.0.2.1 {
        inherit api-template;
        local-as 65000;
        peer-as 65001;
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].ProcessBindings, 1)

	binding := cfg.Peers[0].ProcessBindings[0]
	require.Equal(t, "collector", binding.PluginName)
	require.Equal(t, "json", binding.Content.Encoding)
	require.Equal(t, "full", binding.Content.Format)
	require.True(t, binding.Receive.Update)
}

// TestTemplateAPIBindingPeerOverride verifies peer API bindings override template bindings.
//
// VALIDATES: Peer API binding with same process name replaces template binding.
//
// PREVENTS: Template bindings not being overridden by peer-specific config.
func TestTemplateAPIBindingPeerOverride(t *testing.T) {
	input := `
plugin { external collector { run ./collector; encoder json; } }
template {
    group api-template {
        process collector {
            content { encoding json; format parsed; }
            receive { update; }
        }
    }
}
bgp {
    peer 192.0.2.1 {
        inherit api-template;
        local-as 65000;
        peer-as 65001;
        process collector {
            content { encoding text; format full; }
            receive { all; }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].ProcessBindings, 1, "should have 1 binding (peer overrides template)")

	binding := cfg.Peers[0].ProcessBindings[0]
	require.Equal(t, "collector", binding.PluginName)
	require.Equal(t, "text", binding.Content.Encoding, "peer should override template encoding")
	require.Equal(t, "full", binding.Content.Format, "peer should override template format")
	require.True(t, binding.Receive.Update, "peer 'all' should set Update")
	require.True(t, binding.Receive.Open, "peer 'all' should set Open (template didn't have it)")
}

// TestTemplateAPIBindingMergeMultipleProcesses verifies merging different process bindings.
//
// VALIDATES: Template and peer with different process names both appear in result.
//
// PREVENTS: Missing bindings when template and peer bind different processes.
func TestTemplateAPIBindingMergeMultipleProcesses(t *testing.T) {
	input := `
plugin {
    external collector { run ./collector; encoder json; }
    external logger { run ./logger; encoder text; }
}
template {
    group api-template {
        process collector {
            receive { update; }
        }
    }
}
bgp {
    peer 192.0.2.1 {
        inherit api-template;
        local-as 65000;
        peer-as 65001;
        process logger {
            receive { notification; }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].ProcessBindings, 2, "should have 2 bindings (template + peer)")

	// Find bindings by name
	m := make(map[string]PeerProcessBinding)
	for _, b := range cfg.Peers[0].ProcessBindings {
		m[b.PluginName] = b
	}

	require.Contains(t, m, "collector", "should have collector from template")
	require.Contains(t, m, "logger", "should have logger from peer")
	require.True(t, m["collector"].Receive.Update)
	require.True(t, m["logger"].Receive.Notification)
}

// TestTemplateWithMultipleProcessBindings verifies single template with multiple API bindings.
//
// VALIDATES: A template can have multiple API bindings for different processes.
//
// PREVENTS: Lost bindings when template has multiple process bindings.
//
// Note: Multiple inherit statements are NOT supported (inherit is a Leaf type,
// second inherit overwrites first). This test uses a single template with
// multiple api blocks instead.
func TestTemplateWithMultipleProcessBindings(t *testing.T) {
	input := `
plugin {
    external collector { run ./collector; encoder json; }
    external logger { run ./logger; encoder text; }
}
template {
    group multi-api {
        process collector {
            content { encoding json; }
            receive { update; }
        }
        process logger {
            content { encoding text; }
            receive { notification; }
        }
    }
}
bgp {
    peer 192.0.2.1 {
        inherit multi-api;
        local-as 65000;
        peer-as 65001;
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].ProcessBindings, 2, "should have 2 bindings from template")

	m := make(map[string]PeerProcessBinding)
	for _, b := range cfg.Peers[0].ProcessBindings {
		m[b.PluginName] = b
	}

	require.Equal(t, "json", m["collector"].Content.Encoding)
	require.Equal(t, "text", m["logger"].Content.Encoding)
}

// TestMatchTemplateAPIBinding verifies API bindings from match templates.
//
// VALIDATES: Match patterns can specify API bindings that apply to matching peers.
//
// PREVENTS: Unable to set default API bindings via match patterns.
func TestMatchTemplateAPIBinding(t *testing.T) {
	input := `
plugin { external collector { run ./collector; encoder json; } }
template {
    match * {
        process collector {
            receive { update; }
        }
    }
}
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
    }
    peer 10.0.0.1 {
        local-as 65000;
        peer-as 65002;
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 2)

	for _, peer := range cfg.Peers {
		require.Len(t, peer.ProcessBindings, 1, "match * should apply to %s", peer.Address)
		require.Equal(t, "collector", peer.ProcessBindings[0].PluginName)
		require.True(t, peer.ProcessBindings[0].Receive.Update)
	}
}

// TestMatchAndInheritAPIBindingPrecedence verifies correct precedence: match → inherit → peer.
//
// VALIDATES: Inherit overrides match, peer overrides both.
//
// PREVENTS: Wrong API binding precedence causing unexpected behavior.
func TestMatchAndInheritAPIBindingPrecedence(t *testing.T) {
	input := `
plugin { external collector { run ./collector; encoder json; } }
template {
    match * {
        process collector {
            content { encoding json; }
            receive { update; }
        }
    }
    group override-template {
        process collector {
            content { encoding text; }
            receive { notification; }
        }
    }
}
bgp {
    peer 192.0.2.1 {
        inherit override-template;
        local-as 65000;
        peer-as 65001;
    }
    peer 10.0.0.1 {
        local-as 65000;
        peer-as 65002;
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 2)
	m := peersByAddr(cfg.Peers)

	// 192.0.2.1: match * first, then inherit override-template (inherit wins)
	require.Len(t, m["192.0.2.1"].ProcessBindings, 1)
	require.Equal(t, "text", m["192.0.2.1"].ProcessBindings[0].Content.Encoding, "inherit should override match")
	require.False(t, m["192.0.2.1"].ProcessBindings[0].Receive.Update, "inherit replaces entire binding")
	require.True(t, m["192.0.2.1"].ProcessBindings[0].Receive.Notification)

	// 10.0.0.1: only match * applies
	require.Len(t, m["10.0.0.1"].ProcessBindings, 1)
	require.Equal(t, "json", m["10.0.0.1"].ProcessBindings[0].Content.Encoding)
	require.True(t, m["10.0.0.1"].ProcessBindings[0].Receive.Update)
}

// =============================================================================
// V2 SYNTAX REJECTION TESTS
// =============================================================================

// =============================================================================
// MERGE API BINDINGS TESTS
// =============================================================================

// TestMergeProcessBindingsEmptyNew verifies that empty new returns existing unchanged.
//
// VALIDATES: When new is empty, existing bindings are returned as-is.
//
// PREVENTS: Lost bindings when merging with empty list.
func TestMergeProcessBindingsEmptyNew(t *testing.T) {
	existing := []PeerProcessBinding{
		{PluginName: "foo", Content: PeerContentConfig{Encoding: "json"}},
		{PluginName: "bar", Content: PeerContentConfig{Encoding: "text"}},
	}
	result := mergeProcessBindings(existing, nil)
	require.Equal(t, existing, result)

	result = mergeProcessBindings(existing, []PeerProcessBinding{})
	require.Equal(t, existing, result)
}

// TestMergeProcessBindingsEmptyExisting verifies that empty existing returns new unchanged.
//
// VALIDATES: When existing is empty, new bindings are returned as-is.
//
// PREVENTS: Lost bindings when starting from empty.
func TestMergeProcessBindingsEmptyExisting(t *testing.T) {
	newBindings := []PeerProcessBinding{
		{PluginName: "foo", Content: PeerContentConfig{Encoding: "json"}},
		{PluginName: "bar", Content: PeerContentConfig{Encoding: "text"}},
	}
	result := mergeProcessBindings(nil, newBindings)
	require.Equal(t, newBindings, result)

	result = mergeProcessBindings([]PeerProcessBinding{}, newBindings)
	require.Equal(t, newBindings, result)
}

// TestMergeProcessBindingsAppend verifies that new bindings with different names are appended.
//
// VALIDATES: Bindings with unique names are appended to result.
//
// PREVENTS: Missing bindings when names don't overlap.
func TestMergeProcessBindingsAppend(t *testing.T) {
	existing := []PeerProcessBinding{
		{PluginName: "foo", Content: PeerContentConfig{Encoding: "json"}},
	}
	newBindings := []PeerProcessBinding{
		{PluginName: "bar", Content: PeerContentConfig{Encoding: "text"}},
	}

	result := mergeProcessBindings(existing, newBindings)

	require.Len(t, result, 2)
	require.Equal(t, "foo", result[0].PluginName)
	require.Equal(t, "bar", result[1].PluginName)
}

// TestMergeProcessBindingsReplace verifies that new bindings replace existing with same name.
//
// VALIDATES: Bindings with same ProcessName are replaced (new overrides existing).
//
// PREVENTS: Duplicate bindings for same process, wrong override semantics.
func TestMergeProcessBindingsReplace(t *testing.T) {
	existing := []PeerProcessBinding{
		{PluginName: "foo", Content: PeerContentConfig{Encoding: "json", Format: "parsed"}},
		{PluginName: "bar", Content: PeerContentConfig{Encoding: "text"}},
	}
	newBindings := []PeerProcessBinding{
		{PluginName: "foo", Content: PeerContentConfig{Encoding: "text", Format: "full"}},
	}

	result := mergeProcessBindings(existing, newBindings)

	require.Len(t, result, 2)

	// foo should be replaced with new binding
	require.Equal(t, "foo", result[0].PluginName)
	require.Equal(t, "text", result[0].Content.Encoding)
	require.Equal(t, "full", result[0].Content.Format)

	// bar should remain unchanged
	require.Equal(t, "bar", result[1].PluginName)
	require.Equal(t, "text", result[1].Content.Encoding)
}

// TestMergeProcessBindingsMixed verifies mixed append and replace operations.
//
// VALIDATES: Some bindings replaced, others appended in single merge.
//
// PREVENTS: Incorrect behavior when mix of new and existing names.
func TestMergeProcessBindingsMixed(t *testing.T) {
	existing := []PeerProcessBinding{
		{PluginName: "foo", Content: PeerContentConfig{Encoding: "json"}},
		{PluginName: "bar", Content: PeerContentConfig{Encoding: "json"}},
	}
	newBindings := []PeerProcessBinding{
		{PluginName: "bar", Content: PeerContentConfig{Encoding: "text"}}, // replace
		{PluginName: "baz", Content: PeerContentConfig{Encoding: "json"}}, // append
	}

	result := mergeProcessBindings(existing, newBindings)

	require.Len(t, result, 3)

	// Build map for easier checking
	m := make(map[string]PeerProcessBinding)
	for _, b := range result {
		m[b.PluginName] = b
	}

	require.Equal(t, "json", m["foo"].Content.Encoding) // unchanged
	require.Equal(t, "text", m["bar"].Content.Encoding) // replaced
	require.Equal(t, "json", m["baz"].Content.Encoding) // appended
}

// TestMergeProcessBindingsPreservesOrder verifies that result order is: existing (replaced in place), new appends.
//
// VALIDATES: Order is deterministic: existing items in original positions, new items appended.
//
// PREVENTS: Non-deterministic ordering causing config instability.
func TestMergeProcessBindingsPreservesOrder(t *testing.T) {
	existing := []PeerProcessBinding{
		{PluginName: "a"},
		{PluginName: "b"},
		{PluginName: "c"},
	}
	newBindings := []PeerProcessBinding{
		{PluginName: "b", Content: PeerContentConfig{Encoding: "replaced"}}, // replace in position 1
		{PluginName: "d"}, // append
		{PluginName: "e"}, // append
	}

	result := mergeProcessBindings(existing, newBindings)

	require.Len(t, result, 5)
	require.Equal(t, "a", result[0].PluginName)
	require.Equal(t, "b", result[1].PluginName)
	require.Equal(t, "replaced", result[1].Content.Encoding)
	require.Equal(t, "c", result[2].PluginName)
	require.Equal(t, "d", result[3].PluginName)
	require.Equal(t, "e", result[4].PluginName)
}

// TestMergeProcessBindingsFullReplace verifies complete replacement of all existing bindings.
//
// VALIDATES: When all existing names are in new, all are replaced.
//
// PREVENTS: Stale existing bindings remaining after full override.
func TestMergeProcessBindingsFullReplace(t *testing.T) {
	existing := []PeerProcessBinding{
		{PluginName: "foo", Content: PeerContentConfig{Encoding: "old"}},
		{PluginName: "bar", Content: PeerContentConfig{Encoding: "old"}},
	}
	newBindings := []PeerProcessBinding{
		{PluginName: "foo", Content: PeerContentConfig{Encoding: "new"}},
		{PluginName: "bar", Content: PeerContentConfig{Encoding: "new"}},
	}

	result := mergeProcessBindings(existing, newBindings)

	require.Len(t, result, 2)
	for _, b := range result {
		require.Equal(t, "new", b.Content.Encoding, "binding %s should be replaced", b.PluginName)
	}
}

// TestMergeProcessBindingsReceiveConfig verifies Receive config is properly merged.
//
// VALIDATES: Receive flags are replaced along with the binding.
//
// PREVENTS: Receive config not being properly copied during replace.
func TestMergeProcessBindingsReceiveConfig(t *testing.T) {
	existing := []PeerProcessBinding{
		{
			PluginName: "foo",
			Receive:    PeerReceiveConfig{Update: true, Open: false},
		},
	}
	newBindings := []PeerProcessBinding{
		{
			PluginName: "foo",
			Receive:    PeerReceiveConfig{Update: false, Open: true, Notification: true},
		},
	}

	result := mergeProcessBindings(existing, newBindings)

	require.Len(t, result, 1)
	require.False(t, result[0].Receive.Update, "Update should be false after replace")
	require.True(t, result[0].Receive.Open, "Open should be true after replace")
	require.True(t, result[0].Receive.Notification, "Notification should be true after replace")
}

// =============================================================================
// OLD SYNTAX REJECTION TESTS
// =============================================================================

// TestOldSyntaxRejected verifies that old syntax is rejected by BGPSchema.
//
// VALIDATES: YANGSchema() only accepts current syntax.
//
// PREVENTS: Accidentally accepting deprecated configs.
func TestOldSyntaxRejected(t *testing.T) {
	t.Run("neighbor at root rejected", func(t *testing.T) {
		input := `
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
}
`
		p := NewParser(YANGSchema())
		_, err := p.Parse(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown top-level keyword: neighbor")
	})

	t.Run("peer at root rejected (requires bgp block)", func(t *testing.T) {
		input := `
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
}
`
		p := NewParser(YANGSchema())
		_, err := p.Parse(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown top-level keyword: peer")
	})

	t.Run("peer glob in bgp block rejected", func(t *testing.T) {
		input := `
bgp {
    peer * {
        local-as 65000;
    }
}
`
		p := NewParser(YANGSchema())
		_, err := p.Parse(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid")
	})

	t.Run("peer CIDR pattern in bgp block rejected", func(t *testing.T) {
		input := `
bgp {
    peer 192.0.2.0/24 {
        local-as 65000;
    }
}
`
		p := NewParser(YANGSchema())
		_, err := p.Parse(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid")
	})

	t.Run("template.neighbor rejected", func(t *testing.T) {
		input := `
template {
    neighbor mytemplate {
        local-as 65000;
    }
}
`
		p := NewParser(YANGSchema())
		_, err := p.Parse(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown field in template: neighbor")
	})

	t.Run("current syntax accepted", func(t *testing.T) {
		input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
    }
}
template {
    group mytemplate {
        local-as 65000;
    }
    match * {
        hold-time 90;
    }
}
`
		p := NewParser(YANGSchema())
		tree, err := p.Parse(input)
		require.NoError(t, err)
		require.NotNil(t, tree)
	})
}

// =============================================================================
// NLRI FILTER PARSING TESTS
// =============================================================================

// TestParseNLRIEntriesValid verifies valid NLRI family entries are parsed.
//
// VALIDATES: Space-separated AFI SAFI entries are correctly parsed.
//
// PREVENTS: Valid NLRI filter configs being rejected.
func TestParseNLRIEntriesValid(t *testing.T) {
	tests := []struct {
		name     string
		entries  []string
		wantMode int // 0=all, 1=none, 2=selective
		families []string
	}{
		{"empty returns all", []string{}, 0, nil},
		{"all keyword", []string{"all"}, 0, nil},
		{"none keyword", []string{"none"}, 1, nil},
		{"ipv4/unicast", []string{"ipv4/unicast"}, 2, []string{"ipv4/unicast"}},
		{"ipv6/unicast", []string{"ipv6/unicast"}, 2, []string{"ipv6/unicast"}},
		{"multiple families", []string{"ipv4/unicast", "ipv6/unicast"}, 2, []string{"ipv4/unicast", "ipv6/unicast"}},
		{"case insensitive", []string{"IPv4/Unicast", "IPV6/UNICAST"}, 2, []string{"ipv4/unicast", "ipv6/unicast"}},
		{"l2vpn/evpn", []string{"l2vpn/evpn"}, 2, []string{"l2vpn/evpn"}},
		{"ipv4/flow", []string{"ipv4/flow"}, 2, []string{"ipv4/flow"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := parseNLRIEntries(tt.entries)
			require.NoError(t, err)

			switch tt.wantMode {
			case 0: // all
				require.True(t, filter.IncludesFamily("ipv4/unicast"), "all mode should include ipv4/unicast")
				require.True(t, filter.IncludesFamily("ipv6/unicast"), "all mode should include ipv6/unicast")
			case 1: // none
				require.True(t, filter.IsEmpty(), "none mode should be empty")
			case 2: // selective
				for _, f := range tt.families {
					require.True(t, filter.IncludesFamily(f), "should include %s", f)
				}
			}
		})
	}
}

// TestParseNLRIEntriesInvalid verifies invalid NLRI entries are rejected.
//
// VALIDATES: Invalid entries produce clear errors.
//
// PREVENTS: Silent failures on malformed NLRI config.
func TestParseNLRIEntriesInvalid(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
		wantErr string
	}{
		{"missing safi", []string{"ipv4"}, "unknown family"},
		{"invalid format", []string{"ipv4/unicast extra"}, "unknown family"},
		{"unknown afi", []string{"bogus/unicast"}, "unknown family"},
		{"unknown safi", []string{"ipv4/bogus"}, "unknown family"},
		{"empty entry", []string{"ipv4/unicast", ""}, ""}, // empty entries skipped
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseNLRIEntries(tt.entries)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

// TestAPIConfigNLRIFilter verifies NLRI filter in API config.
//
// VALIDATES: Config with nlri entries parses correctly.
//
// PREVENTS: NLRI filter config not being applied.
func TestAPIConfigNLRIFilter(t *testing.T) {
	input := `
plugin { external foo { run ./test; encoder json; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process foo {
            content {
                encoding json;
                nlri ipv4/unicast;
                nlri ipv6/unicast;
            }
            receive { update; }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].ProcessBindings, 1)

	binding := cfg.Peers[0].ProcessBindings[0]
	require.NotNil(t, binding.Content.NLRI, "NLRI filter should be set")
	require.True(t, binding.Content.NLRI.IncludesFamily("ipv4/unicast"))
	require.True(t, binding.Content.NLRI.IncludesFamily("ipv6/unicast"))
	require.False(t, binding.Content.NLRI.IncludesFamily("l2vpn/evpn"))
}

// TestAPIConfigAttributeFilterError verifies invalid attribute filter is rejected.
//
// VALIDATES: Invalid attribute config produces error.
//
// PREVENTS: Silent failures on malformed attribute config.
func TestAPIConfigAttributeFilterError(t *testing.T) {
	input := `
plugin { external foo { run ./test; encoder json; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process foo {
            content {
                attribute bogus-attribute;
            }
            receive { update; }
        }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid attribute filter")
}

// TestAPIConfigNLRIFilterError verifies invalid NLRI filter is rejected.
//
// VALIDATES: Invalid NLRI config produces error.
//
// PREVENTS: Silent failures on malformed NLRI config.
func TestAPIConfigNLRIFilterError(t *testing.T) {
	input := `
plugin { external foo { run ./test; encoder json; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process foo {
            content {
                nlri bogus family;
            }
            receive { update; }
        }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid nlri filter")
}

// TestConfigValidationRouteRefreshRequiresProcess verifies route-refresh without process fails.
//
// VALIDATES: Config with route-refresh capability but no process binding is rejected.
// PREVENTS: Silent runtime failure when route-refresh request arrives with no plugin to handle it.
func TestConfigValidationRouteRefreshRequiresProcess(t *testing.T) {
	input := `
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        capability { route-refresh; }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)
	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "route-refresh requires process with send { update; }")
	require.Contains(t, err.Error(), "no process bindings configured")
}

// TestConfigValidationGracefulRestartRequiresProcess verifies graceful-restart without process fails.
//
// VALIDATES: Config with graceful-restart capability but no process binding is rejected.
// PREVENTS: Silent runtime failure when peer reconnects and expects routes to be replayed.
func TestConfigValidationGracefulRestartRequiresProcess(t *testing.T) {
	input := `
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        capability { graceful-restart { restart-time 120; } }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)
	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "graceful-restart requires process with send { update; }")
	require.Contains(t, err.Error(), "no process bindings configured")
}

// TestConfigValidationRouteRefreshWithProcess verifies route-refresh with proper process succeeds.
//
// VALIDATES: Config with route-refresh and process with send { update; } is accepted.
// PREVENTS: False positives rejecting valid configurations.
func TestConfigValidationRouteRefreshWithProcess(t *testing.T) {
	input := `
plugin { external rib { run ./rib; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        capability { route-refresh; }
        process rib { send { update; } }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.True(t, cfg.Peers[0].Capabilities.RouteRefresh)
}

// TestConfigValidationGracefulRestartWithProcess verifies graceful-restart with proper process succeeds.
//
// VALIDATES: Config with graceful-restart and process with send { update; } is accepted.
// PREVENTS: False positives rejecting valid configurations.
func TestConfigValidationGracefulRestartWithProcess(t *testing.T) {
	input := `
plugin { external rib { run ./rib; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        capability { graceful-restart { restart-time 120; } }
        process rib { send { update; } }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.True(t, cfg.Peers[0].Capabilities.GracefulRestart)
}

// TestConfigValidationRouteRefreshProcessNoSendUpdate verifies route-refresh with process lacking send { update; } fails.
//
// VALIDATES: Config with route-refresh and process without send { update; } is rejected.
// PREVENTS: Misconfiguration where process cannot respond to route-refresh.
func TestConfigValidationRouteRefreshProcessNoSendUpdate(t *testing.T) {
	input := `
plugin { external logger { run ./logger; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        capability { route-refresh; }
        process logger { receive { update; } }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)
	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "route-refresh requires process with send { update; }")
	require.Contains(t, err.Error(), "configured: process logger")
	require.Contains(t, err.Error(), "none have send { update; }")
}

// TestConfigValidationBothCapabilitiesWithProcess verifies both capabilities with proper process succeeds.
//
// VALIDATES: Config with both route-refresh and graceful-restart with proper process is accepted.
// PREVENTS: False positives when multiple capabilities are configured.
func TestConfigValidationBothCapabilitiesWithProcess(t *testing.T) {
	input := `
plugin { external rib { run ./rib; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        capability {
            route-refresh;
            graceful-restart { restart-time 120; }
        }
        process rib { send { update; } }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.True(t, cfg.Peers[0].Capabilities.RouteRefresh)
	require.True(t, cfg.Peers[0].Capabilities.GracefulRestart)
}

// TestConfigValidationRouteRefreshFromTemplate verifies template with route-refresh, peer without process fails.
//
// VALIDATES: Config where peer inherits route-refresh from template but has no process is rejected.
// PREVENTS: Misconfiguration through template inheritance.
func TestConfigValidationRouteRefreshFromTemplate(t *testing.T) {
	input := `
template {
    group rr {
        capability { route-refresh; }
    }
}
bgp {
    peer 10.0.0.1 {
        inherit rr;
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)
	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "route-refresh requires process with send { update; }")
}

// TestConfigValidationSendAllSatisfiesRequirement verifies send { all; } satisfies the requirement.
//
// VALIDATES: Config with route-refresh and process with send { all; } is accepted.
// PREVENTS: False rejection when using "all" keyword.
func TestConfigValidationSendAllSatisfiesRequirement(t *testing.T) {
	input := `
plugin { external rib { run ./rib; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        capability { route-refresh; }
        process rib { send { all; } }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.True(t, cfg.Peers[0].Capabilities.RouteRefresh)
}

// TestPluginConfigTimeout verifies plugin timeout configuration parsing.
//
// VALIDATES: "timeout 10s;" in plugin block parses to 10 seconds.
// PREVENTS: Plugin timeout config being ignored.
func TestPluginConfigTimeout(t *testing.T) {
	input := `
plugin {
    external myapp {
        run ./myapp;
        encoder json;
        timeout 10s;
    }
}

bgp { }
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Plugins, 1)
	require.Equal(t, "myapp", cfg.Plugins[0].Name)
	require.Equal(t, 10*time.Second, cfg.Plugins[0].StageTimeout)
}

// TestPluginConfigTimeoutDefault verifies default timeout when not specified.
//
// VALIDATES: Missing timeout → 0 (use default in server).
// PREVENTS: Non-zero default breaking existing configs.
func TestPluginConfigTimeoutDefault(t *testing.T) {
	input := `
plugin {
    external myapp {
        run ./myapp;
    }
}

bgp { }
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Plugins, 1)
	require.Equal(t, time.Duration(0), cfg.Plugins[0].StageTimeout)
}

// TestPluginConfigTimeoutInvalid verifies invalid timeout is rejected.
//
// VALIDATES: "timeout abc;" produces parse error.
// PREVENTS: Invalid durations being silently ignored.
func TestPluginConfigTimeoutInvalid(t *testing.T) {
	input := `
plugin {
    external myapp {
        run ./myapp;
        timeout abc;
    }
}

bgp { }
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err) // Parsing succeeds (schema accepts string)

	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid timeout")
}

// TestPluginConfigTimeoutNegative verifies negative timeout is rejected.
//
// VALIDATES: "timeout -5s;" produces error (not silently accepted).
// PREVENTS: Negative duration causing immediate context expiration.
func TestPluginConfigTimeoutNegative(t *testing.T) {
	input := `
plugin {
    external myapp {
        run ./myapp;
        timeout -5s;
    }
}

bgp { }
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err) // Parsing succeeds (schema accepts string)

	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be positive")
}

// TestPluginConfigTimeoutVariants verifies various duration formats.
//
// VALIDATES: Various Go duration formats parse correctly.
// PREVENTS: Only some formats working.
func TestPluginConfigTimeoutVariants(t *testing.T) {
	tests := []struct {
		name     string
		timeout  string
		expected time.Duration
	}{
		{"seconds", "5s", 5 * time.Second},
		{"milliseconds", "500ms", 500 * time.Millisecond},
		{"minutes", "2m", 2 * time.Minute},
		{"combined", "1m30s", 90 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := `
plugin {
    external myapp {
        run ./myapp;
        timeout ` + tt.timeout + `;
    }
}

bgp { }
`
			cfg := parseConfig(t, input)
			require.Len(t, cfg.Plugins, 1)
			require.Equal(t, tt.expected, cfg.Plugins[0].StageTimeout)
		})
	}
}

// TestParseUpdateBlock_Basic verifies basic update block parsing.
//
// VALIDATES: Update block with attributes and NLRI parses correctly into StaticRouteConfig.
// PREVENTS: New native syntax being rejected or misinterpreted.
func TestParseUpdateBlock_Basic(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
            }
            nlri {
                ipv4/unicast 1.0.0.0/24;
            }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].StaticRoutes, 1, "expected 1 static route from update block")

	route := cfg.Peers[0].StaticRoutes[0]
	require.Equal(t, "1.0.0.0/24", route.Prefix.String())
	require.Equal(t, "10.0.0.1", route.NextHop)
	require.Equal(t, "igp", route.Origin)
}

// TestParseUpdateBlock_Attributes verifies all attribute types.
//
// VALIDATES: All supported attributes parse correctly in update block.
// PREVENTS: Missing attribute support in native syntax.
func TestParseUpdateBlock_Attributes(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin egp;
                next-hop 10.0.0.1;
                med 100;
                local-preference 200;
                as-path [ 65001 65002 ];
                community [ 65000:1 65000:2 ];
                large-community [ 65000:0:1 ];
                extended-community [ target:65000:100 ];
            }
            nlri {
                ipv4/unicast 1.0.0.0/24;
            }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].StaticRoutes, 1)

	route := cfg.Peers[0].StaticRoutes[0]
	require.Equal(t, "egp", route.Origin)
	require.Equal(t, "10.0.0.1", route.NextHop)
	require.Equal(t, uint32(100), route.MED)
	require.Equal(t, uint32(200), route.LocalPreference)
	// Value-or-array stores without brackets
	require.Equal(t, "65001 65002", route.ASPath)
	require.Equal(t, "65000:1 65000:2", route.Community)
	require.Equal(t, "65000:0:1", route.LargeCommunity)
	require.Equal(t, "target:65000:100", route.ExtendedCommunity)
}

// TestParseUpdateBlock_MultiplePrefixes verifies multiple prefixes in one family.
//
// VALIDATES: Multiple prefixes in a single nlri line are parsed correctly.
// PREVENTS: Only first prefix being processed.
func TestParseUpdateBlock_MultiplePrefixes(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
            }
            nlri {
                ipv4/unicast 1.0.0.0/24 2.0.0.0/24 3.0.0.0/24;
            }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].StaticRoutes, 3, "expected 3 routes")

	// All routes should have same attributes
	for _, route := range cfg.Peers[0].StaticRoutes {
		require.Equal(t, "igp", route.Origin)
		require.Equal(t, "10.0.0.1", route.NextHop)
	}
}

// TestParseUpdateBlock_NextHopSelf verifies next-hop self.
//
// VALIDATES: next-hop self is correctly parsed.
// PREVENTS: Literal "self" being stored instead of flag.
func TestParseUpdateBlock_NextHopSelf(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop self;
            }
            nlri {
                ipv4/unicast 1.0.0.0/24;
            }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].StaticRoutes, 1)

	route := cfg.Peers[0].StaticRoutes[0]
	require.True(t, route.NextHopSelf, "expected NextHopSelf to be true")
	require.Empty(t, route.NextHop, "expected NextHop to be empty when using self")
}

// TestParseUpdateBlock_MissingNLRI verifies error on missing nlri block.
//
// VALIDATES: Update block without nlri produces clear error.
// PREVENTS: Silent failures on malformed config.
func TestParseUpdateBlock_MissingNLRI(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
            }
        }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "nlri")
}

// TestParseUpdateBlock_InvalidFamily verifies error on invalid family.
//
// VALIDATES: Invalid family name produces clear error.
// PREVENTS: Silent failures or panics on invalid input.
func TestParseUpdateBlock_InvalidFamily(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
            }
            nlri {
                invalid/family 1.0.0.0/24;
            }
        }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid family")
}

// TestParseUpdateBlock_Multiple verifies multiple update blocks per peer.
//
// VALIDATES: Multiple update blocks with different attributes are parsed correctly.
// PREVENTS: Only first update block being processed.
func TestParseUpdateBlock_Multiple(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
            }
            nlri {
                ipv4/unicast 1.0.0.0/24;
            }
        }
        update {
            attribute {
                origin egp;
                next-hop 10.0.0.2;
            }
            nlri {
                ipv4/unicast 2.0.0.0/24;
            }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].StaticRoutes, 2, "expected 2 routes from 2 update blocks")

	// Find routes by prefix and verify attributes
	var found1, found2 bool
	for _, r := range cfg.Peers[0].StaticRoutes {
		switch r.Prefix.String() {
		case "1.0.0.0/24":
			require.Equal(t, "igp", r.Origin)
			require.Equal(t, "10.0.0.1", r.NextHop)
			found1 = true
		case "2.0.0.0/24":
			require.Equal(t, "egp", r.Origin)
			require.Equal(t, "10.0.0.2", r.NextHop)
			found2 = true
		}
	}
	require.True(t, found1, "expected route 1.0.0.0/24")
	require.True(t, found2, "expected route 2.0.0.0/24")
}

// TestParseUpdateBlock_InvalidMED verifies error on invalid MED value.
//
// VALIDATES: Non-numeric MED produces clear error at parse time.
// PREVENTS: Silent failures with MED=0.
func TestParseUpdateBlock_InvalidMED(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
                med abc;
            }
            nlri {
                ipv4/unicast 1.0.0.0/24;
            }
        }
    }
}
`
	p := NewParser(YANGSchema())
	_, err := p.Parse(input)
	// YANG schema validates uint32, so error happens at parse time
	require.Error(t, err)
	require.Contains(t, err.Error(), "med")
}

// TestParseUpdateBlock_IPv6 verifies IPv6 prefix parsing.
//
// VALIDATES: IPv6 prefixes parse correctly in update block.
// PREVENTS: IPv4-only assumption in NLRI parsing.
func TestParseUpdateBlock_IPv6(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 2001:db8::1;
            }
            nlri {
                ipv6/unicast 2001:db8::/32 2001:db8:1::/48;
            }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].StaticRoutes, 2, "expected 2 IPv6 routes")

	// Verify both prefixes parsed correctly
	prefixes := make(map[string]bool)
	for _, route := range cfg.Peers[0].StaticRoutes {
		prefixes[route.Prefix.String()] = true
		require.Equal(t, "igp", route.Origin)
		require.Equal(t, "2001:db8::1", route.NextHop)
	}
	require.True(t, prefixes["2001:db8::/32"], "expected 2001:db8::/32")
	require.True(t, prefixes["2001:db8:1::/48"], "expected 2001:db8:1::/48")
}

// TestParseUpdateBlock_MEDBoundary verifies MED boundary values.
//
// VALIDATES: MED accepts 0 and max uint32 (4294967295).
// PREVENTS: Off-by-one errors in MED validation.
// BOUNDARY: 0 (valid min), 4294967295 (valid max).
func TestParseUpdateBlock_MEDBoundary(t *testing.T) {
	tests := []struct {
		name    string
		med     string
		want    uint32
		wantErr bool
	}{
		{"min_valid_0", "0", 0, false},
		{"max_valid_4294967295", "4294967295", 4294967295, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := fmt.Sprintf(`
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
                med %s;
            }
            nlri {
                ipv4/unicast 1.0.0.0/24;
            }
        }
    }
}
`, tt.med)
			cfg := parseConfig(t, input)
			require.Len(t, cfg.Peers, 1)
			require.Len(t, cfg.Peers[0].StaticRoutes, 1)
			require.Equal(t, tt.want, cfg.Peers[0].StaticRoutes[0].MED)
		})
	}
}

// TestParseUpdateBlock_LocalPrefBoundary verifies local-preference boundary values.
//
// VALIDATES: local-preference accepts 0 and max uint32 (4294967295).
// PREVENTS: Off-by-one errors in local-preference validation.
// BOUNDARY: 0 (valid min), 4294967295 (valid max).
func TestParseUpdateBlock_LocalPrefBoundary(t *testing.T) {
	tests := []struct {
		name    string
		locpref string
		want    uint32
	}{
		{"min_valid_0", "0", 0},
		{"max_valid_4294967295", "4294967295", 4294967295},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := fmt.Sprintf(`
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
                local-preference %s;
            }
            nlri {
                ipv4/unicast 1.0.0.0/24;
            }
        }
    }
}
`, tt.locpref)
			cfg := parseConfig(t, input)
			require.Len(t, cfg.Peers, 1)
			require.Len(t, cfg.Peers[0].StaticRoutes, 1)
			require.Equal(t, tt.want, cfg.Peers[0].StaticRoutes[0].LocalPreference)
		})
	}
}
