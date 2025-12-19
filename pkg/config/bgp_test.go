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
