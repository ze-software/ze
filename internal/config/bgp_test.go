package config

import (
	"testing"

	"github.com/stretchr/testify/require"

	grschema "codeberg.org/thomas-mangin/ze/internal/plugins/bgp-gr/schema"
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
        connection both;
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
// Family is a standard YANG list with key "name" and leaf "mode".
// Multi-entry block: family { ipv4/unicast; ipv6/unicast require; }
//
// VALIDATES: Family/AFI/SAFI config parses correctly as list entries.
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

	// Family is now a list, not a container
	families := n.GetList("family")
	require.Len(t, families, 3)

	// Key-only entries have empty tree (mode defaults to enable)
	require.NotNil(t, families["ipv4/unicast"])
	require.NotNil(t, families["ipv4/multicast"])
	require.NotNil(t, families["ipv6/unicast"])
}

// TestBGPSchemaFamilyWithMode verifies family entries with positional mode.
//
// VALIDATES: family { ipv4/unicast require; } stores mode="require" on the entry.
//
// PREVENTS: Lost mode when using positional syntax.
func TestBGPSchemaFamilyWithMode(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        family {
            ipv4/unicast;
            ipv6/unicast require;
            ipv4/multicast disable;
        }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	bgpContainer := tree.GetContainer("bgp")
	n := bgpContainer.GetList("peer")["192.0.2.1"]

	families := n.GetList("family")
	require.Len(t, families, 3)

	// ipv4/unicast: no mode (default enable)
	ipv4u := families["ipv4/unicast"]
	_, hasMode := ipv4u.Get("mode")
	require.False(t, hasMode, "key-only entry should not have mode set")

	// ipv6/unicast: mode = require
	ipv6u := families["ipv6/unicast"]
	mode, ok := ipv6u.Get("mode")
	require.True(t, ok)
	require.Equal(t, "require", mode)

	// ipv4/multicast: mode = disable
	ipv4m := families["ipv4/multicast"]
	mode, ok = ipv4m.Get("mode")
	require.True(t, ok)
	require.Equal(t, "disable", mode)
}

// TestBGPSchemaFamilyInline verifies individual family entries (non-block).
//
// VALIDATES: family ipv4/unicast; works as a single entry outside a block.
//
// PREVENTS: Only block syntax working for families.
func TestBGPSchemaFamilyInline(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        family ipv4/unicast;
        family ipv6/unicast require;
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	n := tree.GetContainer("bgp").GetList("peer")["192.0.2.1"]
	families := n.GetList("family")
	require.Len(t, families, 2)

	require.NotNil(t, families["ipv4/unicast"])

	ipv6u := families["ipv6/unicast"]
	mode, ok := ipv6u.Get("mode")
	require.True(t, ok)
	require.Equal(t, "require", mode)
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

// TestBGPSchemaNexthopList verifies nexthop parsed as list with positional children.
//
// VALIDATES: nexthop { ipv4/unicast ipv6; } stores nhafi="ipv6" on the entry.
//
// PREVENTS: Lost extended next-hop config after removing ze:allow-unknown-fields.
func TestBGPSchemaNexthopList(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        capability {
            nexthop {
                ipv4/unicast ipv6;
                ipv4/multicast ipv6 require;
            }
        }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	cap := tree.GetContainer("bgp").GetList("peer")["192.0.2.1"].GetContainer("capability")
	nhList := cap.GetList("nexthop")
	require.Len(t, nhList, 2)

	// ipv4/unicast → nhafi=ipv6
	ipv4u := nhList["ipv4/unicast"]
	nhafi, ok := ipv4u.Get("nhafi")
	require.True(t, ok)
	require.Equal(t, "ipv6", nhafi)

	// ipv4/multicast → nhafi=ipv6, mode=require
	ipv4m := nhList["ipv4/multicast"]
	nhafi, ok = ipv4m.Get("nhafi")
	require.True(t, ok)
	require.Equal(t, "ipv6", nhafi)
	mode, ok := ipv4m.Get("mode")
	require.True(t, ok)
	require.Equal(t, "require", mode)
}

// TestBGPSchemaPeerAddPathList verifies peer-level add-path parsed as list.
//
// VALIDATES: add-path { ipv4/unicast send; } stores direction="send" on the entry.
//
// PREVENTS: Lost per-family add-path config after removing ze:allow-unknown-fields.
func TestBGPSchemaPeerAddPathList(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        add-path {
            ipv4/unicast send;
            ipv6/unicast send/receive require;
        }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	n := tree.GetContainer("bgp").GetList("peer")["192.0.2.1"]
	apList := n.GetList("add-path")
	require.Len(t, apList, 2)

	// ipv4/unicast → direction=send
	ipv4u := apList["ipv4/unicast"]
	dir, ok := ipv4u.Get("direction")
	require.True(t, ok)
	require.Equal(t, "send", dir)

	// ipv6/unicast → direction=send/receive, mode=require
	ipv6u := apList["ipv6/unicast"]
	dir, ok = ipv6u.Get("direction")
	require.True(t, ok)
	require.Equal(t, "send/receive", dir)
	mode, ok := ipv6u.Get("mode")
	require.True(t, ok)
	require.Equal(t, "require", mode)
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
    listen 0.0.0.0:179;
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
	require.Equal(t, "0.0.0.0:179", val)
}
