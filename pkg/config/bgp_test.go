package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBGPSchemaNeighbor verifies neighbor configuration parsing.
//
// VALIDATES: Full neighbor config parses correctly.
//
// PREVENTS: Missing or broken neighbor fields.
func TestBGPSchemaNeighbor(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
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

	neighbors := tree.GetList("neighbor")
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
neighbor 192.0.2.1 {
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

	neighbors := tree.GetList("neighbor")
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
neighbor 192.0.2.1 {
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
neighbor 192.0.2.1 {
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
neighbor 192.0.2.1 {
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
neighbor 192.0.2.1 {
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
			require.Len(t, cfg.Neighbors, 1)
			require.Equal(t, tt.expected, cfg.Neighbors[0].IgnoreFamilyMismatch)
		})
	}
}

// TestBGPSchemaCapability verifies capability configuration.
//
// VALIDATES: BGP capabilities are parsed.
//
// PREVENTS: Missing capability negotiation config.
func TestBGPSchemaCapability(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
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

	neighbors := tree.GetList("neighbor")
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
neighbor 192.0.2.1 {
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

	neighbors := tree.GetList("neighbor")
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
neighbor 192.0.2.1 {
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
neighbor 192.0.2.10 {
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
	neighbors := tree.GetList("neighbor")
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

neighbor 192.0.2.1 {
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

	require.Len(t, cfg.Neighbors, 1)
	n := cfg.Neighbors[0]
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
neighbor 127.0.0.1 {
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

	require.Len(t, cfg.Neighbors, 1)
	n := cfg.Neighbors[0]

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
neighbor 127.0.0.1 {
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

	require.Len(t, cfg.Neighbors, 1)
	n := cfg.Neighbors[0]

	require.Len(t, n.StaticRoutes, 1, "expected 1 IPv6 route from announce block")

	r := n.StaticRoutes[0]
	require.Equal(t, "2a02:b80:0:1::1/128", r.Prefix.String())
	require.Equal(t, "2a02:b80:0:2::1", r.NextHop)
	require.Equal(t, "30740:0 30740:30740", r.Community)
	require.Equal(t, uint32(200), r.LocalPreference)
}

// TestBGPSchemaTemplateInherit verifies template inheritance.
//
// VALIDATES: Routes from inherited templates are merged into neighbor config.
//
// PREVENTS: Missing routes when using template inheritance.
func TestBGPSchemaTemplateInherit(t *testing.T) {
	input := `
template {
    neighbor base-routes {
        announce {
            ipv4 {
                unicast 10.0.1.0/24 next-hop 10.0.255.254 community 30740:0;
                unicast 10.0.2.0/24 next-hop 10.0.255.254 local-preference 100;
            }
        }
    }
}

neighbor 127.0.0.1 {
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

	require.Len(t, cfg.Neighbors, 1)
	n := cfg.Neighbors[0]

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

	// Check neighbor route
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
neighbor 192.0.2.1 {
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
neighbor 192.0.2.1 {
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

	require.Len(t, cfg.Neighbors, 1)
	n := cfg.Neighbors[0]
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
neighbor 192.0.2.1 {
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

			require.Len(t, cfg.Neighbors, 1)
			n := cfg.Neighbors[0]
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
neighbor 192.0.2.1 {
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

	require.Len(t, cfg.Neighbors, 1)
	n := cfg.Neighbors[0]

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
