package config

import (
	"testing"

	"github.com/stretchr/testify/require"

	gryang "github.com/ze-software/ze/internal/component/bgp/plugins/gr/yang"
	// Blank import triggers init() registration of all plugin YANG modules.
	// Needed by TestArraySyntax et al. for the "plugin" top-level keyword.
)

// extendedSchemaWithGR returns schema with GR plugin YANG for extended tests.
func extendedSchemaWithGR(t *testing.T) *Schema {
	t.Helper()
	schema, err := YANGSchemaWithPlugins(map[string]string{
		"ze-graceful-restart.yang": gryang.ZeGracefulRestartYANG,
	})
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

// TestFlagSyntax verifies flag-only capability syntax.
//
// VALIDATES: "graceful-restart;" parses as true.
//
// PREVENTS: Requiring block for simple flags.
func TestFlagSyntax(t *testing.T) {
	input := `
bgp {
    peer peer1 {
        connection {
            remote {
                ip 192.0.2.1
            }
        }
        session {
            asn {
                local 65000
                remote 65001
            }
            capability {
                graceful-restart
                add-path
            }
        }
    }
}
`
	schema := extendedSchemaWithGR(t) // Use schema with GR plugin YANG
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	bgpContainer := tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer)
	neighbors := bgpContainer.GetList("peer")
	n := neighbors["peer1"]

	sessionC := n.GetContainer("session")
	require.NotNil(t, sessionC)
	cap := sessionC.GetContainer("capability")
	require.NotNil(t, cap)

	// Flags should be set to "true"
	val, ok := cap.Get("graceful-restart")
	require.True(t, ok, "graceful-restart should exist")
	require.Equal(t, "true", val)

	val, ok = cap.Get("add-path")
	require.True(t, ok, "add-path should exist")
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
    peer peer1 {
        connection {
            remote {
                ip 192.0.2.1
            }
        }
        session {
            asn {
                local 65000
                remote 65001
            }
            capability {
                graceful-restart 120
            }
        }
    }
}
`
	schema := extendedSchemaWithGR(t) // Use schema with GR plugin YANG
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	bgpContainer := tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer)
	neighbors := bgpContainer.GetList("peer")
	n := neighbors["peer1"]

	cap := n.GetContainer("session").GetContainer("capability")
	val, ok := cap.GetFlex("graceful-restart")
	require.True(t, ok)
	require.Equal(t, "120", val)
}

// TestFlagWithBlock verifies flag with block still works.
//
// VALIDATES: "graceful-restart { short-lived { restart-time 120; } }" parses.
//
// PREVENTS: Breaking block syntax.
func TestFlagWithBlock(t *testing.T) {
	input := `
bgp {
    peer peer1 {
        connection {
            remote {
                ip 192.0.2.1
            }
        }
        session {
            asn {
                local 65000
                remote 65001
            }
            capability {
                graceful-restart {
                    short-lived {
                        restart-time 120
                    }
                }
            }
        }
    }
}
`
	schema := extendedSchemaWithGR(t) // Use schema with GR plugin YANG
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	bgpContainer := tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer)
	neighbors := bgpContainer.GetList("peer")
	n := neighbors["peer1"]

	cap := n.GetContainer("session").GetContainer("capability")
	gr := cap.GetContainer("graceful-restart")
	require.NotNil(t, gr)

	shortLived := gr.GetContainer("short-lived")
	require.NotNil(t, shortLived)

	val, ok := shortLived.Get("restart-time")
	require.True(t, ok)
	require.Equal(t, "120", val)
}

// TestEnableDisable verifies enable/disable as bool.
//
// VALIDATES: "connect enable;" on a boolean leaf parses as true.
//
// PREVENTS: Only accepting true/false.
func TestEnableDisable(t *testing.T) {
	input := `
bgp {
    peer peer1 {
        connection {
            remote {
                ip 192.0.2.1
                connect enable
            }
        }
        session {
            asn {
                local 65000
                remote 65001
            }
        }
    }
}
`
	schema, schemaErr := YANGSchema()
	require.NoError(t, schemaErr)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	bgpContainer := tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer)
	neighbors := bgpContainer.GetList("peer")
	n := neighbors["peer1"]

	remote := n.GetContainer("connection").GetContainer("remote")
	val, ok := remote.Get("connect")
	require.True(t, ok)
	require.Equal(t, "true", val)
}

// TestArraySyntax verifies array parsing of a leaf-list in a bus endpoint block.
//
// VALIDATES: "allow [ name1 ];" parses as array.
//
// PREVENTS: Breaking on bracket syntax.
func TestArraySyntax(t *testing.T) {
	input := `
bus {
    endpoint watcher {
        external {
            run "/usr/bin/watcher"
        }
        encoding json
        receive {
            allow [ bgp:update ]
        }
    }
}
`
	schema, schemaErr := YANGSchema()
	require.NoError(t, schemaErr)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	busContainer := tree.GetContainer("bus")
	require.NotNil(t, busContainer)
	endpoints := busContainer.GetList("endpoint")
	ep := endpoints["watcher"]
	require.NotNil(t, ep, "endpoint watcher should exist")

	receiveBlock := ep.GetContainer("receive")
	require.NotNil(t, receiveBlock, "receive block should exist")

	items := receiveBlock.GetSlice("allow")
	require.NotEmpty(t, items)
	require.Equal(t, []string{"bgp:update"}, items)
}

// TestArrayMultipleValues verifies multiple array values.
//
// VALIDATES: "allow [ a b c ];" parses all values.
//
// PREVENTS: Lost array elements.
func TestArrayMultipleValues(t *testing.T) {
	input := `
bus {
    endpoint watcher {
        external {
            run "/usr/bin/watcher"
        }
        receive {
            allow [ bgp:update bgp:state bgp:refresh ]
        }
    }
}
`
	schema, schemaErr := YANGSchema()
	require.NoError(t, schemaErr)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	busContainer := tree.GetContainer("bus")
	require.NotNil(t, busContainer)
	endpoints := busContainer.GetList("endpoint")
	ep := endpoints["watcher"]
	require.NotNil(t, ep, "endpoint watcher should exist")

	receiveBlock := ep.GetContainer("receive")
	require.NotNil(t, receiveBlock, "receive block should exist")

	items := receiveBlock.GetSlice("allow")
	require.NotEmpty(t, items)
	require.Equal(t, []string{"bgp:update", "bgp:state", "bgp:refresh"}, items)
}

// TestArrayRoundtrip verifies array serialization.
//
// VALIDATES: Arrays survive roundtrip.
//
// PREVENTS: Lost array syntax.
func TestArrayRoundtrip(t *testing.T) {
	input := `
bus {
    endpoint watcher {
        external {
            run "/usr/bin/watcher"
        }
        receive {
            allow [ bgp:update ]
        }
    }
}
`
	schema, schemaErr := YANGSchema()
	require.NoError(t, schemaErr)
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	output := Serialize(tree, schema)

	tree2, err := p.Parse(output)
	require.NoError(t, err)

	require.True(t, TreeEqual(tree, tree2))
}
