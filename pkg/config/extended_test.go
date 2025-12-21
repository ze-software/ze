package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFlagSyntax verifies flag-only capability syntax.
//
// VALIDATES: "graceful-restart;" parses as true.
//
// PREVENTS: Requiring block for simple flags.
func TestFlagSyntax(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    capability {
        graceful-restart;
        route-refresh;
    }
}
`
	schema := BGPSchema()
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
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
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    capability {
        graceful-restart 120;
    }
}
`
	schema := BGPSchema()
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
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
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    capability {
        graceful-restart {
            restart-time 120;
        }
    }
}
`
	schema := BGPSchema()
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	n := neighbors["192.0.2.1"]

	cap := n.GetContainer("capability")
	gr := cap.GetContainer("graceful-restart")
	require.NotNil(t, gr)

	val, ok := gr.Get("restart-time")
	require.True(t, ok)
	require.Equal(t, "120", val)
}

// TestInlineRoute verifies inline route syntax parses.
//
// VALIDATES: "route 10.0.0.0/8 next-hop 1.1.1.1;" parses.
//
// PREVENTS: Requiring block for simple routes.
func TestInlineRoute(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    static {
        route 10.0.0.0/8 next-hop 192.0.2.1;
        route 172.16.0.0/12 next-hop 192.0.2.1 local-preference 200;
    }
}
`
	schema := BGPSchema()
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	n := neighbors["192.0.2.1"]

	// Static is Freeform - just verify it parses
	static := n.GetContainer("static")
	require.NotNil(t, static)
}

// TestInlineRouteWithBlock verifies block routes still work.
//
// VALIDATES: Block syntax still works after inline support.
//
// PREVENTS: Breaking existing block syntax.
func TestInlineRouteWithBlock(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    static {
        route 10.0.0.0/8 {
            next-hop 192.0.2.1;
            local-preference 100;
        }
    }
}
`
	schema := BGPSchema()
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	n := neighbors["192.0.2.1"]

	// Static is Freeform - just verify it parses
	static := n.GetContainer("static")
	require.NotNil(t, static)
}

// TestMixedRouteSyntax verifies mixed inline and block routes.
//
// VALIDATES: Inline and block routes can coexist.
//
// PREVENTS: Syntax mixing issues.
func TestMixedRouteSyntax(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    static {
        route 10.0.0.0/8 next-hop 192.0.2.1;
        route 172.16.0.0/12 {
            next-hop 192.0.2.1;
            community 65000:100;
        }
        route 192.168.0.0/16 next-hop self;
    }
}
`
	schema := BGPSchema()
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	n := neighbors["192.0.2.1"]

	// Static is Freeform - just verify it parses
	static := n.GetContainer("static")
	require.NotNil(t, static)
}

// TestEnableDisable verifies enable/disable as bool.
//
// VALIDATES: "asn4 enable;" parses as true.
//
// PREVENTS: Only accepting true/false.
func TestEnableDisable(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    capability {
        asn4 enable;
    }
}
`
	schema := BGPSchema()
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	n := neighbors["192.0.2.1"]

	cap := n.GetContainer("capability")
	val, ok := cap.Get("asn4")
	require.True(t, ok)
	require.Equal(t, "true", val)
}

// TestArraySyntax verifies array parsing.
//
// VALIDATES: "processes [ name1 name2 ];" parses as array.
//
// PREVENTS: Breaking on bracket syntax.
func TestArraySyntax(t *testing.T) {
	input := `
process watcher {
    run "/usr/bin/watcher";
    encoder json;
}

neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    api {
        processes [ watcher ];
    }
}
`
	schema := BGPSchema()
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	n := neighbors["192.0.2.1"]

	api := n.GetContainer("api")
	require.NotNil(t, api)

	val, ok := api.Get("processes")
	require.True(t, ok)
	require.Equal(t, "watcher", val)
}

// TestArrayMultipleValues verifies multiple array values.
//
// VALIDATES: "processes [ a b c ];" parses all values.
//
// PREVENTS: Lost array elements.
func TestArrayMultipleValues(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    api {
        processes [ watcher announcer receiver ];
    }
}
`
	schema := BGPSchema()
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	n := neighbors["192.0.2.1"]

	api := n.GetContainer("api")
	val, ok := api.Get("processes")
	require.True(t, ok)
	require.Equal(t, "watcher announcer receiver", val)
}

// TestArrayRoundtrip verifies array serialization.
//
// VALIDATES: Arrays survive roundtrip.
//
// PREVENTS: Lost array syntax.
func TestArrayRoundtrip(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    api {
        processes [ watcher ];
    }
}
`
	schema := BGPSchema()
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	output := Serialize(tree, schema)

	tree2, err := p.Parse(output)
	require.NoError(t, err)

	require.True(t, TreeEqual(tree, tree2))
}
