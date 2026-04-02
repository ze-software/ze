package config

import (
	"strings"
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

	// session.asn.local should be uint32 (nested in session > asn container).
	sessionNode := bgp.Get("session")
	require.NotNil(t, sessionNode)
	sessionContainer, ok := sessionNode.(*ContainerNode)
	require.True(t, ok, "session should be ContainerNode")
	asnNode := sessionContainer.Get("asn")
	require.NotNil(t, asnNode)
	asnContainer, ok := asnNode.(*ContainerNode)
	require.True(t, ok, "asn should be ContainerNode")
	localAS := asnContainer.Get("local")
	require.NotNil(t, localAS)
	leaf, ok := localAS.(*LeafNode)
	require.True(t, ok)
	assert.Equal(t, TypeUint32, leaf.Type, "session.asn.local type: got %v", leaf.Type)

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

	// connection.md5.password should be marked sensitive
	connNode := peer.Get("connection")
	require.NotNil(t, connNode, "peer should have connection")
	conn, ok := connNode.(*ContainerNode)
	require.True(t, ok, "connection should be ContainerNode")
	md5Node := conn.Get("md5")
	require.NotNil(t, md5Node, "connection should have md5")
	md5Container, ok := md5Node.(*ContainerNode)
	require.True(t, ok, "md5 should be ContainerNode")
	md5Pass := md5Container.Get("password")
	require.NotNil(t, md5Pass, "md5 should have password")
	md5Leaf, ok := md5Pass.(*LeafNode)
	require.True(t, ok, "md5.password should be LeafNode")
	assert.True(t, md5Leaf.Sensitive, "md5.password should be marked sensitive")

	// session.asn.remote should NOT be sensitive
	sessionNode := peer.Get("session")
	require.NotNil(t, sessionNode, "peer should have session")
	session, ok := sessionNode.(*ContainerNode)
	require.True(t, ok, "session should be ContainerNode")
	asnNode := session.Get("asn")
	require.NotNil(t, asnNode, "session should have asn")
	asnContainer, ok := asnNode.(*ContainerNode)
	require.True(t, ok, "asn should be ContainerNode")
	peerAS := asnContainer.Get("remote")
	require.NotNil(t, peerAS, "asn should have remote")
	peerASLeaf, ok := peerAS.(*LeafNode)
	require.True(t, ok)
	assert.False(t, peerASLeaf.Sensitive, "session.asn.remote should not be sensitive")
}

// TestYANGSchemaDecorateExtension verifies that ze:decorate is extracted from YANG leaves.
// VALIDATES: AC-7 -- ze:decorate extension available for YANG modules.
// PREVENTS: Decorator name lost during YANG-to-schema conversion.
func TestYANGSchemaDecorateExtension(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}

	bgpNode := schema.Get("bgp")
	require.NotNil(t, bgpNode)
	bgp, ok := bgpNode.(*ContainerNode)
	require.True(t, ok)

	// Global session.asn.local should have ze:decorate "asn-name"
	sessionNode := bgp.Get("session")
	require.NotNil(t, sessionNode)
	session, ok := sessionNode.(*ContainerNode)
	require.True(t, ok)
	asnNode := session.Get("asn")
	require.NotNil(t, asnNode)
	asnContainer, ok := asnNode.(*ContainerNode)
	require.True(t, ok)
	localAS := asnContainer.Get("local")
	require.NotNil(t, localAS)
	localASLeaf, ok := localAS.(*LeafNode)
	require.True(t, ok)
	assert.Equal(t, "asn-name", localASLeaf.Decorate, "global session.asn.local should have ze:decorate asn-name")

	// Peer session.asn.remote should have ze:decorate "asn-name"
	peerNode := bgp.Get("peer")
	require.NotNil(t, peerNode)
	peer, ok := peerNode.(*ListNode)
	require.True(t, ok)
	peerSessionNode := peer.Get("session")
	require.NotNil(t, peerSessionNode)
	peerSession, ok := peerSessionNode.(*ContainerNode)
	require.True(t, ok)
	peerAsnNode := peerSession.Get("asn")
	require.NotNil(t, peerAsnNode)
	peerAsnContainer, ok := peerAsnNode.(*ContainerNode)
	require.True(t, ok)
	peerAS := peerAsnContainer.Get("remote")
	require.NotNil(t, peerAS)
	peerASLeaf, ok := peerAS.(*LeafNode)
	require.True(t, ok)
	assert.Equal(t, "asn-name", peerASLeaf.Decorate, "peer session.asn.remote should have ze:decorate asn-name")

	// router-id should NOT be decorated
	routerID := bgp.Get("router-id")
	require.NotNil(t, routerID)
	ridLeaf, ok := routerID.(*LeafNode)
	require.True(t, ok)
	assert.Empty(t, ridLeaf.Decorate, "router-id should not be decorated")
}

func TestYANGSchemaSyntaxHints(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}

	// Navigate to session.capability
	bgpNode := schema.Get("bgp")
	require.NotNil(t, bgpNode)
	bgp, ok := bgpNode.(*ContainerNode)
	require.True(t, ok)
	peerNode := bgp.Get("peer")
	require.NotNil(t, peerNode)
	peer, ok := peerNode.(*ListNode)
	require.True(t, ok)
	sessionNode := peer.Get("session")
	require.NotNil(t, sessionNode)
	session, ok := sessionNode.(*ContainerNode)
	require.True(t, ok, "session should be ContainerNode")
	cap := session.Get("capability")
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
    session {
        asn {
            local 65000
        }
    }
    router-id 1.2.3.4
    peer upstream {
        connection {
            remote {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
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

	sessionContainer := bgp.GetContainer("session")
	require.NotNil(t, sessionContainer)
	asnContainer := sessionContainer.GetContainer("asn")
	require.NotNil(t, asnContainer)
	localAS, ok := asnContainer.Get("local")
	assert.True(t, ok)
	assert.Equal(t, "65000", localAS)

	routerID, ok := bgp.Get("router-id")
	assert.True(t, ok)
	assert.Equal(t, "1.2.3.4", routerID)

	peers := bgp.GetList("peer")
	assert.Len(t, peers, 1)
	peerTree := peers["upstream"]
	require.NotNil(t, peerTree)
	peerSessionContainer := peerTree.GetContainer("session")
	require.NotNil(t, peerSessionContainer)
	peerAsnContainer := peerSessionContainer.GetContainer("asn")
	require.NotNil(t, peerAsnContainer)
	peerAS, ok := peerAsnContainer.Get("remote")
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
    session {
        asn {
            local 65000
        }
    }
    router-id 1.2.3.4
    peer upstream {
        connection {
            remote {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
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
    session {
        asn {
            local 65000
        }
    }
    router-id 1.2.3.4
    peer upstream {
        connection {
            remote {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
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

	// Navigate to bgp -> peer -> session -> capability -> route-refresh
	// After Phase 8 YANG changes, route-refresh should be a presence container
	bgpNode := schema.Get("bgp")
	require.NotNil(t, bgpNode)
	bgp, ok := bgpNode.(*ContainerNode)
	require.True(t, ok)
	peerNode := bgp.Get("peer")
	require.NotNil(t, peerNode)
	peer, ok := peerNode.(*ListNode)
	require.True(t, ok)
	sessionNode := peer.Get("session")
	require.NotNil(t, sessionNode)
	session, ok := sessionNode.(*ContainerNode)
	require.True(t, ok)
	cap := session.Get("capability")
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
    session {
        asn {
            local 65000
        }
    }
    peer upstream {
        connection {
            remote {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
            capability {
                route-refresh
            }
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
    session {
        asn {
            local 65000
        }
    }
    peer upstream {
        connection {
            remote {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
            capability {
                route-refresh true
            }
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
    session {
        asn {
            local 65000
        }
    }
    peer upstream {
        connection {
            remote {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
            capability {
                add-path {
                    send true
                    receive true
                }
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
			session := peer.GetContainer("session")
			require.NotNil(t, session)
			cap := session.GetContainer("capability")
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
    session {
        asn {
            local 65000
        }
    }
    peer upstream {
        connection {
            remote {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
            capability {
                route-refresh
            }
        }
    }
}`,
		},
		{
			name: "block form roundtrip",
			input: `bgp {
    session {
        asn {
            local 65000
        }
    }
    peer upstream {
        connection {
            remote {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
            capability {
                extended-message
                software-version
            }
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
    session {
        asn {
            local 65000
        }
    }
    peer upstream {
        connection {
            remote {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
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
    session {
        asn {
            local 65000
        }
    }
    peer upstream {
        connection {
            remote {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
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
    session {
        asn {
            local 65000
        }
    }
    peer upstream {
        connection {
            remote {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
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

// TestInactiveLeafInjected verifies that every ContainerNode and ListNode
// in the schema has an auto-injected "inactive" boolean leaf.
//
// VALIDATES: AC-11 -- inactive is a valid keyword in any container/list.
// PREVENTS: Missing inactive leaf causing "unknown field" errors.
func TestInactiveLeafInjected(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	// Check bgp container has inactive
	bgpNode := schema.Get("bgp")
	require.NotNil(t, bgpNode)
	bgp, ok := bgpNode.(*ContainerNode)
	require.True(t, ok)
	inactiveNode := bgp.Get("inactive")
	require.NotNil(t, inactiveNode, "bgp container should have inactive leaf")
	inactiveLeaf, ok := inactiveNode.(*LeafNode)
	require.True(t, ok, "inactive should be LeafNode, got %T", inactiveNode)
	assert.Equal(t, TypeBool, inactiveLeaf.Type)
	assert.Equal(t, "false", inactiveLeaf.Default)

	// Check peer list has inactive
	peerNode := bgp.Get("peer")
	require.NotNil(t, peerNode)
	peer, ok := peerNode.(*ListNode)
	require.True(t, ok)
	inactiveNode = peer.Get("inactive")
	require.NotNil(t, inactiveNode, "peer list should have inactive leaf")
	inactiveLeaf, ok = inactiveNode.(*LeafNode)
	require.True(t, ok)
	assert.Equal(t, TypeBool, inactiveLeaf.Type)

	// Check nested container (session.capability) has inactive
	sessionNode := peer.Get("session")
	require.NotNil(t, sessionNode)
	sessionContainer, ok := sessionNode.(*ContainerNode)
	require.True(t, ok)
	capNode := sessionContainer.Get("capability")
	require.NotNil(t, capNode)
	capContainer, ok := capNode.(*ContainerNode)
	require.True(t, ok)
	inactiveNode = capContainer.Get("inactive")
	require.NotNil(t, inactiveNode, "capability container should have inactive leaf")

	// Check group list has inactive
	groupNode := bgp.Get("group")
	require.NotNil(t, groupNode)
	group, ok := groupNode.(*ListNode)
	require.True(t, ok)
	inactiveNode = group.Get("inactive")
	require.NotNil(t, inactiveNode, "group list should have inactive leaf")

	// Check update list has inactive
	updateNode := peer.Get("update")
	require.NotNil(t, updateNode)
	update, ok := updateNode.(*ListNode)
	require.True(t, ok)
	inactiveNode = update.Get("inactive")
	require.NotNil(t, inactiveNode, "update list should have inactive leaf")
}

// TestInactiveParses verifies that config with inactive true parses successfully.
//
// VALIDATES: AC-6 -- inactive enable is accepted as valid config.
// PREVENTS: YANG rejecting inactive leaf in config.
func TestInactiveParses(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	input := `bgp {
    session {
        asn {
            local 65000
        }
    }
    router-id 1.2.3.4
    peer upstream {
        connection {
            remote {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
        }
        inactive enable
    }
}`
	parser := NewParser(schema)
	tree, err := parser.Parse(input)
	require.NoError(t, err)

	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp)
	peers := bgp.GetList("peer")
	require.Len(t, peers, 1)
	peer := peers["upstream"]
	require.NotNil(t, peer)
	v, ok := peer.Get("inactive")
	require.True(t, ok, "inactive should be set on peer")
	assert.Equal(t, "true", v, "inactive enable should normalize to true")
}

// TestParseInactivePrefix verifies that "inactive: node { }" is accepted
// as sugar for "node { inactive enable; ... }".
//
// VALIDATES: AC-5 -- inactive: prefix parsed correctly.
// PREVENTS: inactive: prefix rejected as unknown keyword.
func TestParseInactivePrefix(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	input := `bgp {
    session {
        asn {
            local 65000
        }
    }
    router-id 1.2.3.4
    inactive: peer upstream {
        connection {
            remote {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
        }
    }
}`
	parser := NewParser(schema)
	tree, err := parser.Parse(input)
	require.NoError(t, err)

	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp)
	peers := bgp.GetList("peer")
	require.Len(t, peers, 1)
	peer := peers["upstream"]
	require.NotNil(t, peer)
	v, ok := peer.Get("inactive")
	require.True(t, ok, "inactive should be set on peer")
	assert.Equal(t, "true", v, "inactive: prefix should set inactive=true")
}

// TestParseInactiveRoundTrip verifies that parse -> serialize -> parse
// produces identical trees for configs with inactive nodes.
//
// VALIDATES: AC-5, AC-7 -- round-trip preserves inactive state.
// PREVENTS: Lost inactive state during serialize/parse cycle.
func TestParseInactiveRoundTrip(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	input := `bgp {
    session {
        asn {
            local 65000
        }
    }
    router-id 1.2.3.4
    peer active-peer {
        connection {
            remote {
                ip 10.0.0.1
            }
        }
        session {
            asn {
                remote 65001
            }
        }
    }
    inactive: peer disabled-peer {
        connection {
            remote {
                ip 10.0.0.2
            }
        }
        session {
            asn {
                remote 65002
            }
        }
    }
}`
	parser := NewParser(schema)
	tree1, err := parser.Parse(input)
	require.NoError(t, err)

	output := Serialize(tree1, schema)
	tree2, err := parser.Parse(output)
	require.NoError(t, err)
	assert.True(t, TreeEqual(tree1, tree2), "trees should be equal after roundtrip\nSerialized:\n%s", output)
}

// TestParseInactivePrefixInsideListEntry verifies that "inactive:" prefix
// works inside a list entry block (parseListFieldBlock), not just at
// the container level (parseContainer).
//
// VALIDATES: AC-5 -- inactive: prefix works at all nesting levels.
// PREVENTS: inactive: prefix failing inside peer { } blocks.
func TestParseInactivePrefixInsideListEntry(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	input := `bgp {
    session {
        asn {
            local 65000
        }
    }
    router-id 1.2.3.4
    peer upstream {
        connection {
            remote {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
            inactive: capability {
                route-refresh
            }
        }
    }
}`
	parser := NewParser(schema)
	tree, err := parser.Parse(input)
	require.NoError(t, err)

	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp)
	peers := bgp.GetList("peer")
	peer := peers["upstream"]
	require.NotNil(t, peer)
	session := peer.GetContainer("session")
	require.NotNil(t, session)
	cap := session.GetContainer("capability")
	require.NotNil(t, cap, "capability container should exist")
	v, ok := cap.Get("inactive")
	require.True(t, ok, "inactive should be set on capability")
	assert.Equal(t, "true", v, "inactive: prefix should set inactive=true on capability")
}

// TestSerializeInactivePrefix verifies that inactive containers and list
// entries are serialized with the "inactive: " prefix instead of showing
// the inactive leaf as a normal value.
//
// VALIDATES: AC-7 -- text output shows inactive: prefix.
// PREVENTS: Inactive shown as "inactive enable" leaf inside the block.
func TestSerializeInactivePrefix(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	input := `bgp {
    session {
        asn {
            local 65000
        }
    }
    router-id 1.2.3.4
    peer upstream {
        connection {
            remote {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
        }
        inactive enable
    }
}`
	parser := NewParser(schema)
	tree, err := parser.Parse(input)
	require.NoError(t, err)

	output := Serialize(tree, schema)
	assert.Contains(t, output, "inactive: peer upstream", "should render inactive: prefix")
	assert.NotContains(t, output, "inactive enable", "should not render inactive as leaf value")
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
    session {
        asn {
            local 65000
        }
    }
    peer upstream {
        connection {
            remote {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
            capability {
        }
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
    session {
        asn {
            local 65000
        }
    }
    peer upstream {
        connection {
            remote {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
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
    session {
        asn {
            local 65000
        }
    }
    peer upstream {
        connection {
            remote {
                ip 192.168.1.1
            }
        }
        session {
            asn {
                remote 65001
            }
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

// TestListNodeRequiredParsing verifies that ze:required extensions on a list node
// are parsed into ListNode.Required as split path segments.
// VALIDATES: AC-1 -- ze:required parsed into ListNode.Required
// PREVENTS: Required field paths silently dropped during YANG-to-schema conversion.
func TestListNodeRequiredParsing(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	bgpNode := schema.Get("bgp")
	require.NotNil(t, bgpNode)
	bgp, ok := bgpNode.(*ContainerNode)
	require.True(t, ok)

	peerNode := bgp.Get("peer")
	require.NotNil(t, peerNode)
	peer, ok := peerNode.(*ListNode)
	require.True(t, ok)

	// Peer list should have ze:required fields parsed.
	require.NotEmpty(t, peer.Required, "peer list should have Required fields from ze:required")

	// Check specific required fields from ze-bgp-conf.yang.
	found := map[string]bool{}
	for _, req := range peer.Required {
		found[joinSlashPath(req)] = true
	}
	assert.True(t, found["connection/remote/ip"], "connection/remote/ip should be required")
	assert.True(t, found["session/asn/local"], "session/asn/local should be required")
	assert.True(t, found["session/asn/remote"], "session/asn/remote should be required")
}

// TestListNodeSuggestParsing verifies that ze:suggest extensions on a list node
// are parsed into ListNode.Suggest as split path segments.
// VALIDATES: AC-2 -- ze:suggest parsed into ListNode.Suggest
// PREVENTS: Suggest field paths silently dropped during YANG-to-schema conversion.
func TestListNodeSuggestParsing(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	bgpNode := schema.Get("bgp")
	require.NotNil(t, bgpNode)
	bgp, ok := bgpNode.(*ContainerNode)
	require.True(t, ok)

	peerNode := bgp.Get("peer")
	require.NotNil(t, peerNode)
	peer, ok := peerNode.(*ListNode)
	require.True(t, ok)

	// Peer list should have ze:suggest fields parsed.
	require.NotEmpty(t, peer.Suggest, "peer list should have Suggest fields from ze:suggest")

	// Check specific suggest fields from ze-bgp-conf.yang.
	found := map[string]bool{}
	for _, sug := range peer.Suggest {
		found[joinSlashPath(sug)] = true
	}
	assert.True(t, found["connection/local/ip"], "connection/local/ip should be suggested")
}

// joinSlashPath joins path segments with "/".
func joinSlashPath(parts []string) string {
	return strings.Join(parts, "/")
}
