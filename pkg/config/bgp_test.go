package config

import (
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
peer 192.0.2.1 {
    description "Transit Provider";
    router-id 1.2.3.4;
    local-address 192.0.2.2;
    local-as 65000;
    peer-as 65001;
    hold-time 90;
    passive false;
}
`
	p := NewParser(BGPSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	neighbors := tree.GetList("peer")
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
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    family {
        ipv4 unicast;
        ipv4 multicast;
        ipv6 unicast;
    }
}
`
	p := NewParser(BGPSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	neighbors := tree.GetList("peer")
	n := neighbors["192.0.2.1"]

	family := n.GetContainer("family")
	require.NotNil(t, family)

	// Families stored as "afi safi" -> true
	val, ok := family.Get("ipv4 unicast")
	require.True(t, ok)
	require.Equal(t, "true", val)

	val, ok = family.Get("ipv4 multicast")
	require.True(t, ok)
	require.Equal(t, "true", val)

	val, ok = family.Get("ipv6 unicast")
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
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    family {
        ipv4 unicast;
        ignore-mismatch enable;
    }
}`,
			expected: true,
		},
		{
			name: "ignore-mismatch true",
			input: `
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    family {
        ipv4 unicast;
        ignore-mismatch true;
    }
}`,
			expected: true,
		},
		{
			name: "ignore-mismatch disabled",
			input: `
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    family {
        ipv4 unicast;
        ignore-mismatch disable;
    }
}`,
			expected: false,
		},
		{
			name: "ignore-mismatch not specified (default false)",
			input: `
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    family {
        ipv4 unicast;
    }
}`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(BGPSchema())
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
// VALIDATES: "ipv4 unicast require;" parses to FamilyConfig with Mode=Require.
//
// PREVENTS: Unable to require specific address families.
func TestFamilyConfigInlineWithMode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []FamilyConfig
	}{
		{
			name: "ipv4 unicast require",
			input: `
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    family {
        ipv4 unicast require;
    }
}`,
			expected: []FamilyConfig{
				{AFI: "ipv4", SAFI: "unicast", Mode: FamilyModeRequire},
			},
		},
		{
			name: "ipv6 unicast disable",
			input: `
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    family {
        ipv6 unicast disable;
    }
}`,
			expected: []FamilyConfig{
				{AFI: "ipv6", SAFI: "unicast", Mode: FamilyModeDisable},
			},
		},
		{
			name: "mixed modes inline",
			input: `
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    family {
        ipv4 unicast;
        ipv4 multicast require;
        ipv6 unicast disable;
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
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    family {
        ipv4 unicast ignore;
        ipv6 unicast;
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
			p := NewParser(BGPSchema())
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
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    family {
        ipv4 {
            unicast;
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
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    family {
        ipv4 {
            unicast;
            multicast require;
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
			input: `peer 192.0.2.1 { local-as 65000; peer-as 65001; family { ipv6 { unicast require; mpls-vpn } } }`,
			expected: []FamilyConfig{
				{AFI: "ipv6", SAFI: "unicast", Mode: FamilyModeRequire},
				{AFI: "ipv6", SAFI: "mpls-vpn", Mode: FamilyModeEnable},
			},
		},
		{
			name: "multiple AFI blocks",
			input: `
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
			p := NewParser(BGPSchema())
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
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    family {
        ipv4 unicast;
        ipv6 {
            unicast require;
            mpls-vpn;
        }
        l2vpn evpn require;
    }
}`
	expected := []FamilyConfig{
		{AFI: "ipv4", SAFI: "unicast", Mode: FamilyModeEnable},
		{AFI: "ipv6", SAFI: "unicast", Mode: FamilyModeRequire},
		{AFI: "ipv6", SAFI: "mpls-vpn", Mode: FamilyModeEnable},
		{AFI: "l2vpn", SAFI: "evpn", Mode: FamilyModeRequire},
	}

	p := NewParser(BGPSchema())
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
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    capability {
        asn4 true;
        route-refresh true;
        graceful-restart {
            restart-time 120;
        }
        add-path {
            send true;
            receive true;
        }
    }
}
`
	p := NewParser(BGPSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	neighbors := tree.GetList("peer")
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

// TestBGPSchemaStatic verifies static route configuration parses.
//
// VALIDATES: Static routes parse without error (using Freeform).
//
// PREVENTS: Broken route injection.
func TestBGPSchemaStatic(t *testing.T) {
	input := `
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    static {
        route 10.0.0.0/8 {
            next-hop 192.0.2.1;
            local-preference 100;
            community 65000:100;
        }
        route 172.16.0.0/12 {
            next-hop 192.0.2.1;
            med 50;
        }
    }
}
`
	p := NewParser(BGPSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	neighbors := tree.GetList("peer")
	n := neighbors["192.0.2.1"]
	require.NotNil(t, n)

	// Static is parsed as Freeform - just verify it exists
	static := n.GetContainer("static")
	require.NotNil(t, static)
}

// TestBGPSchemaProcess verifies API process configuration.
//
// VALIDATES: External process config parses correctly.
//
// PREVENTS: Broken API integration.
func TestBGPSchemaProcess(t *testing.T) {
	input := `
process announce-routes {
    run "/usr/local/bin/exabgp-announce";
    encoder json;
}

process receive-routes {
    run "/usr/local/bin/exabgp-receive";
    encoder text;
}
`
	p := NewParser(BGPSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	procs := tree.GetList("process")
	require.Len(t, procs, 2)

	p1 := procs["announce-routes"]
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
router-id 1.2.3.4;
local-as 65000;
listen 0.0.0.0 179;
`
	p := NewParser(BGPSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	val, ok := tree.Get("router-id")
	require.True(t, ok)
	require.Equal(t, "1.2.3.4", val)

	val, ok = tree.Get("local-as")
	require.True(t, ok)
	require.Equal(t, "65000", val)

	val, ok = tree.Get("listen")
	require.True(t, ok)
	require.Equal(t, "0.0.0.0 179", val)
}

// TestBGPSchemaFullConfig verifies a complete configuration.
//
// VALIDATES: Full realistic config parses correctly.
//
// PREVENTS: Integration issues between config sections.
func TestBGPSchemaFullConfig(t *testing.T) {
	input := `
# Global settings
router-id 10.0.0.1;
local-as 65000;

# API process
process watcher {
    run "/usr/bin/watcher";
    encoder json;
}

# Transit provider
peer 192.0.2.1 {
    description "Transit AS65001";
    local-address 192.0.2.2;
    peer-as 65001;
    hold-time 90;
    family {
        ipv4 unicast;
        ipv6 unicast;
    }
    capability {
        asn4 true;
        route-refresh true;
    }
}

# Peer
peer 192.0.2.10 {
    description "Peer AS65010";
    local-address 192.0.2.2;
    peer-as 65010;
    passive true;
    family {
        ipv4 unicast;
    }
    static {
        route 10.0.0.0/8 {
            next-hop self;
            community 65000:100;
        }
    }
}
`
	p := NewParser(BGPSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	// Check global
	val, _ := tree.Get("router-id")
	require.Equal(t, "10.0.0.1", val)

	// Check process
	procs := tree.GetList("process")
	require.Len(t, procs, 1)

	// Check neighbors
	neighbors := tree.GetList("peer")
	require.Len(t, neighbors, 2)

	n1 := neighbors["192.0.2.1"]
	val, _ = n1.Get("peer-as")
	require.Equal(t, "65001", val)

	n2 := neighbors["192.0.2.10"]
	val, _ = n2.Get("passive")
	require.Equal(t, "true", val)
}

// TestBGPSchemaToConfig verifies conversion to typed Config.
//
// VALIDATES: Tree converts to strongly-typed Config struct.
//
// PREVENTS: Type conversion errors.
func TestBGPSchemaToConfig(t *testing.T) {
	input := `
router-id 10.0.0.1;
local-as 65000;

peer 192.0.2.1 {
    local-address 192.0.2.2;
    peer-as 65001;
    hold-time 90;
}
`
	p := NewParser(BGPSchema())
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

// TestBGPSchemaAnnounce verifies announce block parsing and route extraction.
//
// VALIDATES: Routes in announce { ipv4 { unicast ... } } syntax are parsed
// and converted to StaticRouteConfig entries.
//
// PREVENTS: Missing routes when using announce syntax instead of static syntax.
func TestBGPSchemaAnnounce(t *testing.T) {
	input := `
peer 127.0.0.1 {
    router-id 10.0.0.2;
    local-address 127.0.0.1;
    local-as 65533;
    peer-as 65000;

    family {
        ipv4 unicast;
    }

    announce {
        ipv4 {
            unicast 10.0.0.0/24 next-hop 10.0.1.254 local-preference 200;
            unicast 10.0.1.0/24 next-hop 10.0.1.254 med 100;
        }
    }
}
`
	p := NewParser(BGPSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, err := TreeToConfig(tree)
	require.NoError(t, err)

	require.Len(t, cfg.Peers, 1)
	n := cfg.Peers[0]

	// Should have 2 routes from announce block
	require.Len(t, n.StaticRoutes, 2, "expected 2 routes from announce block")

	// Find routes by prefix (order not guaranteed)
	routeMap := make(map[string]StaticRouteConfig)
	for _, r := range n.StaticRoutes {
		routeMap[r.Prefix.String()] = r
	}

	// Check first route
	r1, ok := routeMap["10.0.0.0/24"]
	require.True(t, ok, "missing route 10.0.0.0/24")
	require.Equal(t, "10.0.1.254", r1.NextHop)
	require.Equal(t, uint32(200), r1.LocalPreference)

	// Check second route
	r2, ok := routeMap["10.0.1.0/24"]
	require.True(t, ok, "missing route 10.0.1.0/24")
	require.Equal(t, "10.0.1.254", r2.NextHop)
	require.Equal(t, uint32(100), r2.MED)
}

// TestBGPSchemaAnnounceIPv6 verifies IPv6 announce block parsing.
//
// VALIDATES: IPv6 routes with communities parse correctly.
//
// PREVENTS: Missing IPv6 routes or community parsing failures.
func TestBGPSchemaAnnounceIPv6(t *testing.T) {
	input := `
peer 127.0.0.1 {
    router-id 10.0.0.2;
    local-address 127.0.0.1;
    local-as 65533;
    peer-as 65533;

    family {
        ipv6 unicast;
    }

    announce {
        ipv6 {
            unicast 2a02:b80:0:1::1/128 next-hop 2a02:b80:0:2::1 community [30740:0 30740:30740] local-preference 200;
        }
    }
}
`
	p := NewParser(BGPSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, err := TreeToConfig(tree)
	require.NoError(t, err)

	require.Len(t, cfg.Peers, 1)
	n := cfg.Peers[0]

	require.Len(t, n.StaticRoutes, 1, "expected 1 IPv6 route from announce block")

	r := n.StaticRoutes[0]
	require.Equal(t, "2a02:b80:0:1::1/128", r.Prefix.String())
	require.Equal(t, "2a02:b80:0:2::1", r.NextHop)
	require.Equal(t, "30740:0 30740:30740", r.Community)
	require.Equal(t, uint32(200), r.LocalPreference)
}

// TestBGPSchemaTemplateInherit verifies template inheritance.
//
// VALIDATES: Routes from inherited templates are merged into group config.
//
// PREVENTS: Missing routes when using template inheritance.
func TestBGPSchemaTemplateInherit(t *testing.T) {
	input := `
template {
    group base-routes {
        announce {
            ipv4 {
                unicast 10.0.1.0/24 next-hop 10.0.255.254 community 30740:0;
                unicast 10.0.2.0/24 next-hop 10.0.255.254 local-preference 100;
            }
        }
    }
}

peer 127.0.0.1 {
    inherit base-routes;
    router-id 10.0.0.2;
    local-address 127.0.0.1;
    local-as 65533;
    peer-as 65533;

    family {
        ipv4 unicast;
    }

    announce {
        ipv4 {
            unicast 10.0.3.0/24 next-hop 10.0.255.254 local-preference 200;
        }
    }
}
`
	p := NewParser(BGPSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cfg, err := TreeToConfig(tree)
	require.NoError(t, err)

	require.Len(t, cfg.Peers, 1)
	n := cfg.Peers[0]

	// Should have 3 routes: 2 from template + 1 from neighbor
	require.Len(t, n.StaticRoutes, 3, "expected 3 routes (2 from template + 1 from neighbor)")

	// Find routes by prefix
	routeMap := make(map[string]StaticRouteConfig)
	for _, r := range n.StaticRoutes {
		routeMap[r.Prefix.String()] = r
	}

	// Check template routes
	r1, ok := routeMap["10.0.1.0/24"]
	require.True(t, ok, "missing route 10.0.1.0/24 from template")
	require.Equal(t, "30740:0", r1.Community)

	r2, ok := routeMap["10.0.2.0/24"]
	require.True(t, ok, "missing route 10.0.2.0/24 from template")
	require.Equal(t, uint32(100), r2.LocalPreference)

	// Check group route
	r3, ok := routeMap["10.0.3.0/24"]
	require.True(t, ok, "missing route 10.0.3.0/24 from neighbor")
	require.Equal(t, uint32(200), r3.LocalPreference)
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
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    hold-time ` + tt.holdTime + `;
}
`
			p := NewParser(BGPSchema())
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
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    local-address auto;
}
`
	p := NewParser(BGPSchema())
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
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    ` + tt.config + `
}
`
			p := NewParser(BGPSchema())
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
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    add-path {
        ipv4 unicast send;
        ipv6 unicast receive;
        ipv4 multicast send/receive;
    }
}
`
	p := NewParser(BGPSchema())
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

	ipv4Uni, ok := familyMap["ipv4 unicast"]
	require.True(t, ok, "missing ipv4 unicast add-path config")
	require.True(t, ipv4Uni.Send, "ipv4 unicast should have send")
	require.False(t, ipv4Uni.Receive, "ipv4 unicast should not have receive")

	ipv6Uni, ok := familyMap["ipv6 unicast"]
	require.True(t, ok, "missing ipv6 unicast add-path config")
	require.False(t, ipv6Uni.Send, "ipv6 unicast should not have send")
	require.True(t, ipv6Uni.Receive, "ipv6 unicast should have receive")

	ipv4Multi, ok := familyMap["ipv4 multicast"]
	require.True(t, ok, "missing ipv4 multicast add-path config")
	require.True(t, ipv4Multi.Send, "ipv4 multicast should have send")
	require.True(t, ipv4Multi.Receive, "ipv4 multicast should have receive")
}

// TestASN4DefaultEnabled verifies ASN4 capability is enabled by default.
//
// VALIDATES: Configs without explicit asn4 setting get ASN4 enabled.
//
// PREVENTS: Missing 4-byte AS capability in OPEN messages.
func TestASN4DefaultEnabled(t *testing.T) {
	// Config without capability block
	input := `
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
}
`
	p := NewParser(BGPSchema())
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
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    capability {
        asn4 false;
    }
}
`
	p := NewParser(BGPSchema())
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
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    rib {
        out {
            auto-commit-delay ` + tt.input + `;
        }
    }
}
`
			p := NewParser(BGPSchema())
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
		input := `peer 192.0.2.1 { local-as 65000; peer-as 65001; }`
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
peer 192.0.2.1 { inherit rib-tmpl; local-as 65000; peer-as 65001; }
peer 192.0.2.2 { inherit rib-tmpl; local-as 65000; peer-as 65002; rib { out { auto-commit-delay 50ms; } } }
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
		input := `peer 192.0.2.1 { local-as 65000; peer-as 65001; group-updates false; }`
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
	p := NewParser(BGPSchema())
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
// PREVENTS: Unable to use v3 group syntax for named templates.
func TestTemplateGroupBasic(t *testing.T) {
	input := `
template {
    group ibgp-rr {
        peer-as 65000;
        hold-time 60;
        capability { route-refresh; }
    }
}

peer 192.0.2.1 {
    inherit ibgp-rr;
    local-as 65000;
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
// PREVENTS: Unable to use v3 match syntax for glob patterns.
func TestTemplateMatchBasic(t *testing.T) {
	input := `
template {
    match * {
        rib { out { group-updates false; auto-commit-delay 100ms; } }
    }
}

peer 192.0.2.1 { local-as 65000; peer-as 65001; }
peer 192.0.2.2 { local-as 65000; peer-as 65002; }
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

peer 192.0.2.1 { local-as 65000; peer-as 65001; }
peer 10.0.0.1 { local-as 65000; peer-as 65002; }
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

peer 10.0.0.1 { local-as 65000; peer-as 65001; }
peer 192.168.1.1 { local-as 65000; peer-as 65002; }
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

// TestTemplateGroupWithStaticRoutes verifies static routes in group templates.
//
// VALIDATES: group templates can contain static routes.
//
// PREVENTS: Missing routes when using v3 group syntax.
func TestTemplateGroupWithStaticRoutes(t *testing.T) {
	input := `
template {
    group customer-routes {
        static {
            route 10.0.0.0/8 next-hop self;
            route 172.16.0.0/12 next-hop self;
        }
    }
}

peer 192.0.2.1 {
    inherit customer-routes;
    local-as 65000;
    peer-as 65001;
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	n := cfg.Peers[0]

	require.Len(t, n.StaticRoutes, 2)
	routeMap := make(map[string]StaticRouteConfig)
	for _, r := range n.StaticRoutes {
		routeMap[r.Prefix.String()] = r
	}
	require.Contains(t, routeMap, "10.0.0.0/8")
	require.Contains(t, routeMap, "172.16.0.0/12")
}

// TestTemplateMatchWithStaticRoutes verifies static routes in match blocks.
//
// VALIDATES: match blocks can contain static routes.
//
// PREVENTS: Missing routes when using match patterns.
func TestTemplateMatchWithStaticRoutes(t *testing.T) {
	input := `
template {
    match * {
        static {
            route 10.0.0.0/8 next-hop self;
        }
    }
    match 192.0.2.* {
        static {
            route 172.16.0.0/12 next-hop self;
        }
    }
}

peer 192.0.2.1 { local-as 65000; peer-as 65001; }
peer 10.0.0.1 { local-as 65000; peer-as 65002; }
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 2)
	m := peersByAddr(cfg.Peers)

	// 192.0.2.1: both matches apply, gets both routes
	require.Len(t, m["192.0.2.1"].StaticRoutes, 2)

	// 10.0.0.1: only match * applies, gets one route
	require.Len(t, m["10.0.0.1"].StaticRoutes, 1)
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

peer 192.0.2.1 {
    inherit high-preference;
    local-as 65000;
    peer-as 65001;
}
peer 10.0.0.1 {
    local-as 65000;
    peer-as 65002;
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
// PREVENTS: Unable to use v3 peer syntax for BGP sessions.
func TestPeerKeywordForSessions(t *testing.T) {
	input := `
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    hold-time 90;
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
template {
    group ibgp-defaults {
        hold-time 60;
        peer-as 65000;
        capability { route-refresh true; }
    }
}

peer 192.0.2.1 {
    inherit ibgp-defaults;
    local-as 65000;
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
`
	p := NewParser(BGPSchema())
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
	p := NewParser(BGPSchema())
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
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    match * {
        hold-time 90;
    }
}
`
	p := NewParser(BGPSchema())
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
			input := `template { group ` + name + ` { hold-time 90; } }`
			p := NewParser(BGPSchema())
			tree, err := p.Parse(input)
			require.NoError(t, err, "group name %q should parse", name)

			// Validation happens in TreeToConfig
			_, err = TreeToConfig(tree)
			require.NoError(t, err, "group name %q should be valid", name)
		})
	}

	for _, name := range invalidNames {
		t.Run("invalid:"+name, func(t *testing.T) {
			input := `template { group ` + name + ` { hold-time 90; } }`
			p := NewParser(BGPSchema())
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

peer 10.0.0.1 { local-as 65000; peer-as 65001; }
peer 192.168.1.1 { local-as 65000; peer-as 65002; }
peer 172.16.0.1 { local-as 65000; peer-as 65003; }
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 3)
	m := peersByAddr(cfg.Peers)

	require.Equal(t, uint16(60), m["10.0.0.1"].HoldTime)
	require.Equal(t, uint16(90), m["192.168.1.1"].HoldTime)
	require.Equal(t, uint16(0), m["172.16.0.1"].HoldTime) // default
}

// =============================================================================
// TEMPLATE MATCH TESTS (v3 syntax)
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
peer 192.0.2.1 { local-as 65000; peer-as 65001; }
peer 192.0.2.2 { local-as 65000; peer-as 65002; }
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
peer 192.0.2.1 { local-as 65000; peer-as 65001; }
peer 192.0.2.2 { local-as 65000; peer-as 65002; }
peer 10.0.0.1 { local-as 65000; peer-as 65003; }
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
peer 192.0.2.1 { local-as 65000; peer-as 65001; }
peer 192.0.2.2 {
    local-as 65000;
    peer-as 65002;
    rib { out { group-updates true; } }
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
peer 192.0.2.1 { local-as 65000; peer-as 65001; }
peer 10.0.0.1 { local-as 65000; peer-as 65002; }
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

// TestPeerAPIBindingOldSyntax verifies API binding parsing with old syntax.
//
// VALIDATES: Config parsing extracts API bindings from processes array.
//
// PREVENTS: Silent failures when api block is malformed.
func TestPeerAPIBindingOldSyntax(t *testing.T) {
	input := `
process foo { run ./test; encoder text; }
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api {
        processes [ foo ];
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].APIBindings, 1)

	binding := cfg.Peers[0].APIBindings[0]
	require.Equal(t, "foo", binding.ProcessName)
}

// TestAPIBindingMultipleProcesses verifies multiple processes in old syntax.
//
// VALIDATES: Multiple processes in array create multiple bindings.
//
// PREVENTS: Missing processes when multiple specified.
func TestAPIBindingMultipleProcesses(t *testing.T) {
	input := `
process collector { run ./collector; encoder json; }
process logger { run ./logger; encoder text; }
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api {
        processes [ collector logger ];
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers[0].APIBindings, 2)

	// Find bindings by name
	names := make([]string, 0)
	for _, b := range cfg.Peers[0].APIBindings {
		names = append(names, b.ProcessName)
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
process foo { run ./test; encoder text; }
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api {
        processes [ foo ];
        neighbor-changes;
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].APIBindings, 1)

	binding := cfg.Peers[0].APIBindings[0]
	require.Equal(t, "foo", binding.ProcessName)
	require.True(t, binding.Receive.State, "neighbor-changes should set receive.State")
}

// TestAPIBindingUndefinedProcess verifies error on undefined process reference.
//
// VALIDATES: Error when api references non-existent process.
//
// PREVENTS: Runtime crashes from nil process lookup.
func TestAPIBindingUndefinedProcess(t *testing.T) {
	input := `
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api {
        processes [ nonexistent ];
    }
}
`
	p := NewParser(BGPSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "undefined process")
}

// TestEmptyAPIBlock verifies empty api block creates no bindings.
//
// VALIDATES: Empty api block (no processes) creates no bindings.
//
// PREVENTS: Crash on empty api block.
func TestEmptyAPIBlock(t *testing.T) {
	input := `
process foo { run ./test; }
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api { }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers[0].APIBindings, 0, "Empty api block should have no bindings")
}

// TestAPIBindingConfigStructs verifies the config struct fields exist.
//
// VALIDATES: PeerAPIBinding struct has all required fields.
//
// PREVENTS: Missing fields for Phase 2 new syntax support.
func TestAPIBindingConfigStructs(t *testing.T) {
	// Verify struct fields exist (compile-time check)
	binding := PeerAPIBinding{
		ProcessName: "test",
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
	require.Equal(t, "test", binding.ProcessName)
	require.Equal(t, "json", binding.Content.Encoding)
	require.True(t, binding.Receive.Update)
	require.True(t, binding.Send.Update)
}

// =============================================================================
// NEW API SYNTAX TESTS (api <process-name> { content {...} receive {...} })
// =============================================================================

// TestPeerAPIBindingNewSyntax verifies API binding parsing with new syntax.
//
// VALIDATES: New syntax (api <name> { content {...} }) parses correctly.
//
// PREVENTS: Silent failures when using new api syntax.
func TestPeerAPIBindingNewSyntax(t *testing.T) {
	input := `
process foo { run ./test; encoder text; }
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api foo {
        content { encoding json; format full; }
        receive { update; notification; }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].APIBindings, 1)

	binding := cfg.Peers[0].APIBindings[0]
	require.Equal(t, "foo", binding.ProcessName)
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
process foo { run ./test; }
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api foo {
        receive { all; }
    }
}
`
	cfg := parseConfig(t, input)

	recv := cfg.Peers[0].APIBindings[0].Receive
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
process foo { run ./test; }
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api foo {
        send { all; }
    }
}
`
	cfg := parseConfig(t, input)

	send := cfg.Peers[0].APIBindings[0].Send
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
process foo { run ./test; }
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api foo { }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers[0].APIBindings, 1)

	binding := cfg.Peers[0].APIBindings[0]
	require.Equal(t, "foo", binding.ProcessName)
	require.Empty(t, binding.Content.Encoding) // Inherit from process at runtime
	require.Empty(t, binding.Content.Format)   // Default to "parsed" at runtime
	require.False(t, binding.Receive.Update)   // No messages subscribed
}

// TestAPIBindingUndefinedProcessNewSyntax verifies error on undefined process in new syntax.
//
// VALIDATES: Error when api references non-existent process.
//
// PREVENTS: Runtime crashes from nil process lookup.
func TestAPIBindingUndefinedProcessNewSyntax(t *testing.T) {
	input := `
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api nonexistent {
        receive { update; }
    }
}
`
	p := NewParser(BGPSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "undefined process")
}

// TestMultipleAPIBindingsNewSyntax verifies multiple api blocks for different processes.
//
// VALIDATES: Multiple api <name> blocks create separate bindings.
//
// PREVENTS: Only first api block being parsed.
func TestMultipleAPIBindingsNewSyntax(t *testing.T) {
	input := `
process collector { run ./collector; encoder json; }
process logger { run ./logger; encoder text; }
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    api collector {
        content { encoding json; }
        receive { update; }
    }
    api logger {
        content { encoding text; format full; }
        receive { all; }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers[0].APIBindings, 2)

	// Find bindings by name
	var collectorBinding, loggerBinding *PeerAPIBinding
	for i := range cfg.Peers[0].APIBindings {
		b := &cfg.Peers[0].APIBindings[i]
		switch b.ProcessName {
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
// V2 SYNTAX REJECTION TESTS
// =============================================================================

// TestV2SyntaxRejected verifies that v2 syntax is rejected by BGPSchema.
//
// VALIDATES: BGPSchema() only accepts v3 syntax.
//
// PREVENTS: Accidentally accepting deprecated v2 configs.
func TestV2SyntaxRejected(t *testing.T) {
	t.Run("neighbor at root rejected", func(t *testing.T) {
		input := `
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
}
`
		p := NewParser(BGPSchema())
		_, err := p.Parse(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown top-level keyword: neighbor")
	})

	t.Run("peer glob at root rejected", func(t *testing.T) {
		input := `
peer * {
    local-as 65000;
}
`
		p := NewParser(BGPSchema())
		_, err := p.Parse(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid")
	})

	t.Run("peer CIDR pattern at root rejected", func(t *testing.T) {
		input := `
peer 192.0.2.0/24 {
    local-as 65000;
}
`
		p := NewParser(BGPSchema())
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
		p := NewParser(BGPSchema())
		_, err := p.Parse(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown field in template: neighbor")
	})

	t.Run("v3 syntax accepted", func(t *testing.T) {
		input := `
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
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
		p := NewParser(BGPSchema())
		tree, err := p.Parse(input)
		require.NoError(t, err)
		require.NotNil(t, tree)
	})
}
