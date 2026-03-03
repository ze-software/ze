package config

import (
	"testing"

	"github.com/stretchr/testify/require"

	grschema "codeberg.org/thomas-mangin/ze/internal/plugins/bgp-gr/schema"

	// Blank import triggers init() registration of all plugin YANG modules.
	// Needed by TestArraySyntax et al. for the "plugin" top-level keyword.
	_ "codeberg.org/thomas-mangin/ze/internal/plugin/all"
)

// extendedSchemaWithGR returns schema with GR plugin YANG for extended tests.
func extendedSchemaWithGR() *Schema {
	return YANGSchemaWithPlugins(map[string]string{
		"ze-graceful-restart.yang": grschema.ZeGracefulRestartYANG,
	})
}

// TestFlagSyntax verifies flag-only capability syntax.
//
// VALIDATES: "graceful-restart;" parses as true.
//
// PREVENTS: Requiring block for simple flags.
func TestFlagSyntax(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        capability {
            graceful-restart;
            route-refresh;
        }
    }
}
`
	schema := extendedSchemaWithGR() // Use schema with GR plugin YANG
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	bgpContainer := tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer)
	neighbors := bgpContainer.GetList("peer")
	n := neighbors["192.0.2.1"]

	cap := n.GetContainer("capability")
	require.NotNil(t, cap)

	// Flags should be set to "true"
	val, ok := cap.Get("graceful-restart")
	require.True(t, ok, "graceful-restart should exist")
	require.Equal(t, "true", val)

	val, ok = cap.Get("route-refresh")
	require.True(t, ok, "route-refresh should exist")
	require.Equal(t, "true", val)
}

// TestFlagWithValue verifies flag with optional value.
//
// VALIDATES: "graceful-restart 120;" parses value.
//
// PREVENTS: Breaking existing value syntax.
func TestFlagWithValue(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        capability {
            graceful-restart 120;
        }
    }
}
`
	schema := extendedSchemaWithGR() // Use schema with GR plugin YANG
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	bgpContainer := tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer)
	neighbors := bgpContainer.GetList("peer")
	n := neighbors["192.0.2.1"]

	cap := n.GetContainer("capability")
	val, ok := cap.GetFlex("graceful-restart")
	require.True(t, ok)
	require.Equal(t, "120", val)
}

// TestFlagWithBlock verifies flag with block still works.
//
// VALIDATES: "graceful-restart { restart-time 120; }" parses.
//
// PREVENTS: Breaking block syntax.
func TestFlagWithBlock(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        capability {
            graceful-restart {
                restart-time 120;
            }
        }
    }
}
`
	schema := extendedSchemaWithGR() // Use schema with GR plugin YANG
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	bgpContainer := tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer)
	neighbors := bgpContainer.GetList("peer")
	n := neighbors["192.0.2.1"]

	cap := n.GetContainer("capability")
	gr := cap.GetContainer("graceful-restart")
	require.NotNil(t, gr)

	val, ok := gr.Get("restart-time")
	require.True(t, ok)
	require.Equal(t, "120", val)
}

// TestEnableDisable verifies enable/disable as bool.
//
// VALIDATES: "asn4 enable;" parses as true.
//
// PREVENTS: Only accepting true/false.
func TestEnableDisable(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        capability {
            asn4 enable;
        }
    }
}
`
	schema := YANGSchema()
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	bgpContainer := tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer)
	neighbors := bgpContainer.GetList("peer")
	n := neighbors["192.0.2.1"]

	cap := n.GetContainer("capability")
	val, ok := cap.Get("asn4")
	require.True(t, ok)
	require.Equal(t, "true", val)
}

// TestArraySyntax verifies array parsing in old migration syntax.
//
// VALIDATES: "processes [ name1 name2 ];" parses as array.
//
// PREVENTS: Breaking on bracket syntax.
func TestArraySyntax(t *testing.T) {
	input := `
plugin {
    external watcher {
        run "/usr/bin/watcher";
        encoder json;
    }
}

bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        process {
            processes [ watcher ];
        }
    }
}
`
	schema := YANGSchema()
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	bgpContainer := tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer)
	neighbors := bgpContainer.GetList("peer")
	n := neighbors["192.0.2.1"]

	// process is now List(TypeString, ...), so use GetList with KeyDefault key
	processList := n.GetList("process")
	require.NotNil(t, processList)
	processBlock := processList[KeyDefault]
	require.NotNil(t, processBlock, "anonymous process block should exist")

	items := processBlock.GetSlice("processes")
	require.NotEmpty(t, items)
	require.Equal(t, []string{"watcher"}, items)
}

// TestArrayMultipleValues verifies multiple array values.
//
// VALIDATES: "processes [ a b c ];" parses all values.
//
// PREVENTS: Lost array elements.
func TestArrayMultipleValues(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        process {
            processes [ watcher announcer receiver ];
        }
    }
}
`
	schema := YANGSchema()
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	bgpContainer := tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer)
	neighbors := bgpContainer.GetList("peer")
	n := neighbors["192.0.2.1"]

	// api is now List(TypeString, ...), so use GetList with KeyDefault key
	apiList := n.GetList("process")
	require.NotNil(t, apiList)
	api := apiList[KeyDefault]
	require.NotNil(t, api, "anonymous process block should exist")

	items := api.GetSlice("processes")
	require.NotEmpty(t, items)
	require.Equal(t, []string{"watcher", "announcer", "receiver"}, items)
}

// TestArrayRoundtrip verifies array serialization.
//
// VALIDATES: Arrays survive roundtrip.
//
// PREVENTS: Lost array syntax.
func TestArrayRoundtrip(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        process {
            processes [ watcher ];
        }
    }
}
`
	schema := YANGSchema()
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	output := Serialize(tree, schema)

	tree2, err := p.Parse(output)
	require.NoError(t, err)

	require.True(t, TreeEqual(tree, tree2))
}
