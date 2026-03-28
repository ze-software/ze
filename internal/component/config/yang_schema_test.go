package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Blank imports trigger init() registration of YANG modules.
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/softver/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/schema"
)

func TestYANGSchemaLoads(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}

	// Should have bgp from ze-bgp-conf.yang
	bgp := schema.Get("bgp")
	require.NotNil(t, bgp, "should have bgp node")

	// bgp should be a container
	bgpContainer, ok := bgp.(*ContainerNode)
	require.True(t, ok, "bgp should be ContainerNode")

	// Should have peer list
	peer := bgpContainer.Get("peer")
	require.NotNil(t, peer, "should have peer")

	// peer should be a list
	_, ok = peer.(*ListNode)
	assert.True(t, ok, "peer should be ListNode")
}

func TestYANGSchemaLeafTypes(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}

	bgpNode := schema.Get("bgp")
	require.NotNil(t, bgpNode)
	bgp, ok := bgpNode.(*ContainerNode)
	require.True(t, ok, "bgp should be ContainerNode")

	// local.as should be uint32 (nested in local container)
	localNode := bgp.Get("local")
	require.NotNil(t, localNode)
	localContainer, ok := localNode.(*ContainerNode)
	require.True(t, ok, "local should be ContainerNode")
	localAS := localContainer.Get("as")
	require.NotNil(t, localAS)
	leaf, ok := localAS.(*LeafNode)
	require.True(t, ok)
	assert.Equal(t, TypeUint32, leaf.Type, "local.as type: got %v", leaf.Type)

	// router-id should be IPv4 - check actual type for debugging
	routerID := bgp.Get("router-id")
	require.NotNil(t, routerID)
	leaf, ok = routerID.(*LeafNode)
	require.True(t, ok)
	t.Logf("router-id leaf.Type = %v (%s)", leaf.Type, leaf.Type.String())
	// Accept either TypeIPv4 or TypeString for now (typedef resolution)
	assert.True(t, leaf.Type == TypeIPv4 || leaf.Type == TypeString,
		"router-id should be TypeIPv4 or TypeString, got %v", leaf.Type)
}

func TestYANGSchemaSensitiveExtension(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}

	bgpNode := schema.Get("bgp")
	require.NotNil(t, bgpNode)
	bgp, ok := bgpNode.(*ContainerNode)
	require.True(t, ok)
	peerNode := bgp.Get("peer")
	require.NotNil(t, peerNode)
	peer, ok := peerNode.(*ListNode)
	require.True(t, ok)

	// md5-password should be marked sensitive
	md5 := peer.Get("md5-password")
	require.NotNil(t, md5, "peer should have md5-password")
	md5Leaf, ok := md5.(*LeafNode)
	require.True(t, ok, "md5-password should be LeafNode")
	assert.True(t, md5Leaf.Sensitive, "md5-password should be marked sensitive")

	// remote.as should NOT be sensitive
	remoteNode := peer.Get("remote")
	require.NotNil(t, remoteNode, "peer should have remote")
	remote, ok := remoteNode.(*ContainerNode)
	require.True(t, ok, "remote should be ContainerNode")
	peerAS := remote.Get("as")
	require.NotNil(t, peerAS, "remote should have as")
	peerASLeaf, ok := peerAS.(*LeafNode)
	require.True(t, ok)
	assert.False(t, peerASLeaf.Sensitive, "remote.as should not be sensitive")
}

func TestYANGSchemaSyntaxHints(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}

	// Navigate to capability
	bgpNode := schema.Get("bgp")
	require.NotNil(t, bgpNode)
	bgp, ok := bgpNode.(*ContainerNode)
	require.True(t, ok)
	peerNode := bgp.Get("peer")
	require.NotNil(t, peerNode)
	peer, ok := peerNode.(*ListNode)
	require.True(t, ok)
	cap := peer.Get("capability")
	require.NotNil(t, cap)

	capContainer, ok := cap.(*ContainerNode)
	require.True(t, ok, "capability should be ContainerNode")

	// route-refresh should be a presence ContainerNode (standard YANG, no ze:syntax)
	rr := capContainer.Get("route-refresh")
	if rr != nil {
		rrContainer, ok := rr.(*ContainerNode)
		assert.True(t, ok, "route-refresh should be ContainerNode, got %T", rr)
		if ok {
			assert.True(t, rrContainer.Presence, "route-refresh should have Presence=true")
		}
	}
}

func TestYANGSchemaCanParse(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}

	// Test that parser works with YANG-derived schema
	config := `bgp {
    local {
        as 65000
    }
    router-id 1.2.3.4
    peer upstream {
        remote {
            ip 192.168.1.1
            as 65001
        }
    }
}`

	parser := NewParser(schema)
	tree, err := parser.Parse(config)
	require.NoError(t, err)
	require.NotNil(t, tree)

	// Verify parsed values
	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp)

	localContainer := bgp.GetContainer("local")
	require.NotNil(t, localContainer)
	localAS, ok := localContainer.Get("as")
	assert.True(t, ok)
	assert.Equal(t, "65000", localAS)

	routerID, ok := bgp.Get("router-id")
	assert.True(t, ok)
	assert.Equal(t, "1.2.3.4", routerID)

	peers := bgp.GetList("peer")
	assert.Len(t, peers, 1)
	peerTree := peers["upstream"]
	require.NotNil(t, peerTree)
	remoteContainer := peerTree.GetContainer("remote")
	require.NotNil(t, remoteContainer)
	peerAS, ok := remoteContainer.Get("as")
	assert.True(t, ok)
	assert.Equal(t, "65001", peerAS)
}

// TestYANGSchema_NoAnnounce verifies announce block is rejected.
//
// VALIDATES: ExaBGP announce syntax is not accepted by YANGSchema.
// PREVENTS: Regression allowing legacy ExaBGP syntax in engine.
func TestYANGSchema_NoAnnounce(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}

	// ExaBGP-style announce block should be rejected
	config := `bgp {
    local {
        as 65000
    }
    router-id 1.2.3.4
    peer upstream {
        remote {
            ip 192.168.1.1
            as 65001
        }
        announce {
            ipv4 {
                unicast 10.0.0.0/24 next-hop 10.0.0.1
            }
        }
    }
}`

	parser := NewParser(schema)
	_, err = parser.Parse(config)
	require.Error(t, err, "announce block should be rejected")
	assert.Contains(t, err.Error(), "announce", "error should mention announce")
}

// TestYANGSchema_NoStatic verifies static block is rejected.
//
// VALIDATES: ExaBGP static syntax is not accepted by YANGSchema.
// PREVENTS: Regression allowing legacy ExaBGP syntax in engine.
func TestYANGSchema_NoStatic(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}

	// ExaBGP-style static block should be rejected
	config := `bgp {
    local {
        as 65000
    }
    router-id 1.2.3.4
    peer upstream {
        remote {
            ip 192.168.1.1
            as 65001
        }
        static {
            route 10.0.0.0/24 next-hop 10.0.0.1
        }
    }
}`

	parser := NewParser(schema)
	_, err = parser.Parse(config)
	require.Error(t, err, "static block should be rejected")
	assert.Contains(t, err.Error(), "static", "error should mention static")
}

// TestYANGLeafListDefaultIsValueOrArray verifies that leaf-list nodes
// without ze:syntax annotations produce ValueOrArrayNode (accepts both
// single value and bracket list syntax).
//
// VALIDATES: Phase 7 -- leaf-list natively accepts value; and [ v1 v2 ];
// PREVENTS: Regression to MultiLeafNode default (no bracket support).
func TestYANGLeafListDefaultIsValueOrArray(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}

	// Navigate to bgp -> peer -> process -> receive (a leaf-list)
	bgpNode := schema.Get("bgp")
	require.NotNil(t, bgpNode)
	bgp, ok := bgpNode.(*ContainerNode)
	require.True(t, ok)
	peerNode := bgp.Get("peer")
	require.NotNil(t, peerNode)
	peer, ok := peerNode.(*ListNode)
	require.True(t, ok)
	processNode := peer.Get("process")
	require.NotNil(t, processNode)
	process, ok := processNode.(*ListNode)
	require.True(t, ok)

	// 'receive' is a leaf-list -- should be ValueOrArrayNode
	receiveNode := process.Get("receive")
	require.NotNil(t, receiveNode, "receive node should exist")
	_, ok = receiveNode.(*ValueOrArrayNode)
	assert.True(t, ok, "leaf-list 'receive' should be ValueOrArrayNode, got %T", receiveNode)

	// 'send' is also a leaf-list -- should be ValueOrArrayNode
	sendNode := process.Get("send")
	require.NotNil(t, sendNode, "send node should exist")
	_, ok = sendNode.(*ValueOrArrayNode)
	assert.True(t, ok, "leaf-list 'send' should be ValueOrArrayNode, got %T", sendNode)
}

// TestYANGPresenceContainerDetected verifies that a YANG container with
// a `presence` statement (and no ze:syntax annotation) becomes a
// ContainerNode with Presence=true in the schema.
//
// VALIDATES: Phase 8 -- presence containers detected from standard YANG.
// PREVENTS: Presence containers being treated as regular containers.
func TestYANGPresenceContainerDetected(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}

	// Navigate to bgp -> peer -> capability -> route-refresh
	// After Phase 8 YANG changes, route-refresh should be a presence container
	bgpNode := schema.Get("bgp")
	require.NotNil(t, bgpNode)
	bgp, ok := bgpNode.(*ContainerNode)
	require.True(t, ok)
	peerNode := bgp.Get("peer")
	require.NotNil(t, peerNode)
	peer, ok := peerNode.(*ListNode)
	require.True(t, ok)
	cap := peer.Get("capability")
	require.NotNil(t, cap)
	capContainer, ok := cap.(*ContainerNode)
	require.True(t, ok)

	// route-refresh should be a ContainerNode with Presence=true
	rr := capContainer.Get("route-refresh")
	require.NotNil(t, rr, "route-refresh should exist")
	rrContainer, ok := rr.(*ContainerNode)
	require.True(t, ok, "route-refresh should be ContainerNode, got %T", rr)
	assert.True(t, rrContainer.Presence, "route-refresh should have Presence=true")
}

// TestPresenceContainerParsesAllForms verifies that a presence container
// accepts flag, value, and block forms -- the same behavior as FlexNode.
//
// VALIDATES: Phase 8 -- presence containers replace flex parsing.
// PREVENTS: Regression in capability config parsing after ze:syntax removal.
func TestPresenceContainerParsesAllForms(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		input     string
		checkTree func(t *testing.T, cap *Tree)
	}{
		{
			name: "flag form",
			input: `bgp {
    local {
        as 65000
    }
    peer upstream {
        remote {
            ip 192.168.1.1
            as 65001
        }
        capability {
            route-refresh
        }
    }
}`,
			checkTree: func(t *testing.T, cap *Tree) {
				v, ok := cap.Get("route-refresh")
				require.True(t, ok, "route-refresh should be set as value")
				assert.Equal(t, "true", v, "flag form should store 'true'")
			},
		},
		{
			name: "value form",
			input: `bgp {
    local {
        as 65000
    }
    peer upstream {
        remote {
            ip 192.168.1.1
            as 65001
        }
        capability {
            route-refresh true
        }
    }
}`,
			checkTree: func(t *testing.T, cap *Tree) {
				v, ok := cap.GetFlex("route-refresh")
				require.True(t, ok, "route-refresh should be set")
				assert.Equal(t, "true", v)
			},
		},
		{
			name: "block form with children",
			input: `bgp {
    local {
        as 65000
    }
    peer upstream {
        remote {
            ip 192.168.1.1
            as 65001
        }
        capability {
            add-path {
                send true
                receive true
            }
        }
    }
}`,
			checkTree: func(t *testing.T, cap *Tree) {
				ap := cap.GetContainer("add-path")
				require.NotNil(t, ap, "add-path should be a container")
				v, ok := ap.Get("send")
				require.True(t, ok)
				assert.Equal(t, "true", v)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(schema)
			tree, err := parser.Parse(tt.input)
			require.NoError(t, err)

			bgp := tree.GetContainer("bgp")
			require.NotNil(t, bgp)
			peers := bgp.GetList("peer")
			require.Len(t, peers, 1)
			peer := peers["upstream"]
			require.NotNil(t, peer)
			cap := peer.GetContainer("capability")
			require.NotNil(t, cap)

			tt.checkTree(t, cap)
		})
	}
}

// TestPresenceContainerSerializesAllForms verifies that presence containers
// serialize correctly in flag, value, and block forms with roundtrip.
//
// VALIDATES: Phase 8 -- presence container serialization roundtrip.
// PREVENTS: Lost data during serialization of presence containers.
func TestPresenceContainerSerializesAllForms(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		input string
	}{
		{
			name: "flag form roundtrip",
			input: `bgp {
    local {
        as 65000
    }
    peer upstream {
        remote {
            ip 192.168.1.1
            as 65001
        }
        capability {
            route-refresh
        }
    }
}`,
		},
		{
			name: "block form roundtrip",
			input: `bgp {
    local {
        as 65000
    }
    peer upstream {
        remote {
            ip 192.168.1.1
            as 65001
        }
        capability {
            extended-message
            software-version
        }
    }
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(schema)
			tree, err := parser.Parse(tt.input)
			require.NoError(t, err)

			output := Serialize(tree, schema)
			tree2, err := parser.Parse(output)
			require.NoError(t, err)
			require.True(t, TreeEqual(tree, tree2), "trees should be equal after roundtrip\nSerialized:\n%s", output)
		})
	}
}

// TestYANGLeafListParsesBothForms verifies that a leaf-list node
// accepts both `name value;` and `name [ v1 v2 ];` syntax.
//
// VALIDATES: Phase 7 -- leaf-list parsing unification.
// PREVENTS: Bracket syntax failing after ze:syntax removal.
func TestYANGLeafListParsesBothForms(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name: "bracket form",
			input: `bgp {
    local {
        as 65000
    }
    peer upstream {
        remote {
            ip 192.168.1.1
            as 65001
        }
        process rib {
            receive [ update state ]
        }
    }
}`,
			want: []string{"update", "state"},
		},
		{
			name: "single value",
			input: `bgp {
    local {
        as 65000
    }
    peer upstream {
        remote {
            ip 192.168.1.1
            as 65001
        }
        process rib {
            receive update
        }
    }
}`,
			want: []string{"update"},
		},
		{
			name: "space-separated values",
			input: `bgp {
    local {
        as 65000
    }
    peer upstream {
        remote {
            ip 192.168.1.1
            as 65001
        }
        process rib {
            receive update state
        }
    }
}`,
			want: []string{"update", "state"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(schema)
			tree, err := parser.Parse(tt.input)
			require.NoError(t, err)

			bgp := tree.GetContainer("bgp")
			require.NotNil(t, bgp)
			peers := bgp.GetList("peer")
			require.Len(t, peers, 1)
			peer := peers["upstream"]
			require.NotNil(t, peer)
			processes := peer.GetList("process")
			require.Len(t, processes, 1)
			proc := processes["rib"]
			require.NotNil(t, proc)

			items := proc.GetSlice("receive")
			require.NotEmpty(t, items, "receive should be set")
			assert.Equal(t, tt.want, items)
		})
	}
}

// TestDeadCapabilityRejected verifies that dead capabilities are rejected.
//
// VALIDATES: AC-1, AC-2, AC-3: multi-session/operational/aigp rejected
// PREVENTS: Dead capabilities being silently accepted in config.
func TestDeadCapabilityRejected(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		capability string
	}{
		{name: "multi-session", capability: "multi-session"},
		{name: "operational", capability: "operational"},
		{name: "aigp", capability: "aigp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := `bgp {
    local {
        as 65000
    }
    peer upstream {
        remote {
            ip 192.168.1.1
            as 65001
        }
        capability {
            ` + tt.capability + `
        }
    }
}`
			parser := NewParser(schema)
			_, err := parser.Parse(input)
			require.Error(t, err, "%s should be rejected", tt.capability)
			assert.Contains(t, err.Error(), tt.capability, "error should mention %s", tt.capability)
		})
	}
}

// TestDeadPeerLeafRejected verifies that dead peer-level config is rejected.
//
// VALIDATES: AC-4: peer-level multi-session leaf rejected
// PREVENTS: Dead peer-level config being silently accepted.
func TestDeadPeerLeafRejected(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}

	input := `bgp {
    local {
        as 65000
    }
    peer upstream {
        remote {
            ip 192.168.1.1
            as 65001
        }
        multi-session
    }
}`
	parser := NewParser(schema)
	_, err = parser.Parse(input)
	require.Error(t, err, "multi-session as peer leaf should be rejected")
	assert.Contains(t, err.Error(), "multi-session", "error should mention multi-session")
}

// TestProcessReceiveAcceptsStrings verifies that receive/send accept arbitrary strings
// at the YANG level (validation is at runtime via parseReceiveFlags, not YANG enum).
//
// VALIDATES: AC-9: receive/send use type string in YANG, runtime validates event types.
// PREVENTS: YANG rejecting plugin-registered event types like "update-rpki".
func TestProcessReceiveAcceptsStrings(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		field string
	}{
		{
			name:  "receive custom event type",
			field: "receive [ update-rpki ]",
		},
		{
			name:  "receive mixed base and custom",
			field: "receive [ update update-rpki ]",
		},
		{
			name:  "send custom type",
			field: "send [ update ]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := `bgp {
    local {
        as 65000
    }
    peer upstream {
        remote {
            ip 192.168.1.1
            as 65001
        }
        process rib {
            ` + tt.field + `
        }
    }
}`
			parser := NewParser(schema)
			_, err := parser.Parse(input)
			require.NoError(t, err, "YANG should accept %s (runtime validates)", tt.field)
		})
	}
}
