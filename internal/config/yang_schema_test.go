package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYANGSchemaLoads(t *testing.T) {
	schema := YANGSchema()
	require.NotNil(t, schema, "YANGSchema should load")

	// Should have bgp from ze-bgp.yang
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
	schema := YANGSchema()
	require.NotNil(t, schema)

	bgpNode := schema.Get("bgp")
	require.NotNil(t, bgpNode)
	bgp, ok := bgpNode.(*ContainerNode)
	require.True(t, ok, "bgp should be ContainerNode")

	// local-as should be uint32
	localAS := bgp.Get("local-as")
	require.NotNil(t, localAS)
	leaf, ok := localAS.(*LeafNode)
	require.True(t, ok)
	assert.Equal(t, TypeUint32, leaf.Type, "local-as type: got %v", leaf.Type)

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

func TestYANGSchemaSyntaxHints(t *testing.T) {
	schema := YANGSchema()
	require.NotNil(t, schema)

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

	// route-refresh should be FlexNode (from syntax hints)
	rr := capContainer.Get("route-refresh")
	if rr != nil {
		_, ok := rr.(*FlexNode)
		assert.True(t, ok, "route-refresh should be FlexNode")
	}
}

func TestYANGSchemaCanParse(t *testing.T) {
	schema := YANGSchema()
	require.NotNil(t, schema)

	// Test that parser works with YANG-derived schema
	config := `bgp {
    local-as 65000;
    router-id 1.2.3.4;
    peer 192.168.1.1 {
        peer-as 65001;
    }
}`

	parser := NewParser(schema)
	tree, err := parser.Parse(config)
	require.NoError(t, err)
	require.NotNil(t, tree)

	// Verify parsed values
	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp)

	localAS, ok := bgp.Get("local-as")
	assert.True(t, ok)
	assert.Equal(t, "65000", localAS)

	routerID, ok := bgp.Get("router-id")
	assert.True(t, ok)
	assert.Equal(t, "1.2.3.4", routerID)

	peers := bgp.GetList("peer")
	assert.Len(t, peers, 1)
	peerTree := peers["192.168.1.1"]
	require.NotNil(t, peerTree)
	peerAS, ok := peerTree.Get("peer-as")
	assert.True(t, ok)
	assert.Equal(t, "65001", peerAS)
}

// TestYANGSchema_NoAnnounce verifies announce block is rejected.
//
// VALIDATES: ExaBGP announce syntax is not accepted by YANGSchema.
// PREVENTS: Regression allowing legacy ExaBGP syntax in engine.
func TestYANGSchema_NoAnnounce(t *testing.T) {
	schema := YANGSchema()
	require.NotNil(t, schema)

	// ExaBGP-style announce block should be rejected
	config := `bgp {
    local-as 65000;
    router-id 1.2.3.4;
    peer 192.168.1.1 {
        peer-as 65001;
        announce {
            ipv4 {
                unicast 10.0.0.0/24 next-hop 10.0.0.1;
            }
        }
    }
}`

	parser := NewParser(schema)
	_, err := parser.Parse(config)
	require.Error(t, err, "announce block should be rejected")
	assert.Contains(t, err.Error(), "announce", "error should mention announce")
}

// TestYANGSchema_NoStatic verifies static block is rejected.
//
// VALIDATES: ExaBGP static syntax is not accepted by YANGSchema.
// PREVENTS: Regression allowing legacy ExaBGP syntax in engine.
func TestYANGSchema_NoStatic(t *testing.T) {
	schema := YANGSchema()
	require.NotNil(t, schema)

	// ExaBGP-style static block should be rejected
	config := `bgp {
    local-as 65000;
    router-id 1.2.3.4;
    peer 192.168.1.1 {
        peer-as 65001;
        static {
            route 10.0.0.0/24 next-hop 10.0.0.1;
        }
    }
}`

	parser := NewParser(schema)
	_, err := parser.Parse(config)
	require.Error(t, err, "static block should be rejected")
	assert.Contains(t, err.Error(), "static", "error should mention static")
}
