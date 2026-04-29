package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInlineEnumExtraction verifies that inline YANG enum definitions
// populate LeafNode.Enums correctly.
//
// VALIDATES: yangToLeaf extracts inline enum values from YANG leaf.
// PREVENTS: Inline enums silently dropped, leaving empty Enums slice.
func TestInlineEnumExtraction(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	bgpNode := schema.Get("bgp")
	require.NotNil(t, bgpNode, "bgp must exist in schema")
	bgp, ok := bgpNode.(*ContainerNode)
	require.True(t, ok)

	peerNode := bgp.Get("peer")
	require.NotNil(t, peerNode, "peer must exist under bgp")
	peer, ok := peerNode.(*ListNode)
	require.True(t, ok)

	updateNode := peer.Get("update")
	require.NotNil(t, updateNode, "update must exist under peer")

	// update is a list, get its children.
	updateList, ok := updateNode.(*ListNode)
	require.True(t, ok, "update must be a ListNode")

	attrNode := updateList.Get("attribute")
	require.NotNil(t, attrNode, "attribute must exist under update")
	attr, ok := attrNode.(*ContainerNode)
	require.True(t, ok)

	originNode := attr.Get("origin")
	require.NotNil(t, originNode, "origin must exist under attribute")
	origin, ok := originNode.(*LeafNode)
	require.True(t, ok)

	// Origin has inline enums: igp, egp, incomplete.
	assert.NotEmpty(t, origin.Enums, "origin must have enum values")
	assert.Contains(t, origin.Enums, "igp")
	assert.Contains(t, origin.Enums, "egp")
	assert.Contains(t, origin.Enums, "incomplete")
}

// TestTypedefEnumExtraction verifies that YANG typedef enum definitions
// (resolved by goyang) populate LeafNode.Enums correctly.
//
// VALIDATES: yangToLeaf extracts typedef enum values after goyang resolution.
// PREVENTS: Typedef enums not resolved, leaving empty Enums slice.
func TestTypedefEnumExtraction(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	bgpNode := schema.Get("bgp")
	require.NotNil(t, bgpNode, "bgp must exist in schema")
	bgp, ok := bgpNode.(*ContainerNode)
	require.True(t, ok)

	peerNode := bgp.Get("peer")
	require.NotNil(t, peerNode, "peer must exist under bgp")
	peer, ok := peerNode.(*ListNode)
	require.True(t, ok)

	sessionNode := peer.Get("session")
	require.NotNil(t, sessionNode, "session must exist under peer")
	session, ok := sessionNode.(*ContainerNode)
	require.True(t, ok)

	capNode := session.Get("capability")
	require.NotNil(t, capNode, "capability must exist under session")
	cap, ok := capNode.(*ContainerNode)
	require.True(t, ok)

	// nexthop is a list with leaf nhafi using typedef zt:afi.
	nhListNode := cap.Get("nexthop")
	require.NotNil(t, nhListNode, "nexthop must exist under capability")
	nhList, ok := nhListNode.(*ListNode)
	require.True(t, ok)

	nhNode := nhList.Get("nhafi")
	require.NotNil(t, nhNode, "nhafi must exist under nexthop")
	nh, ok := nhNode.(*LeafNode)
	require.True(t, ok)

	// zt:afi is a typedef enum with ipv4, ipv6.
	assert.NotEmpty(t, nh.Enums, "nhafi must have enum values from typedef zt:afi")
	assert.Contains(t, nh.Enums, "ipv4")
	assert.Contains(t, nh.Enums, "ipv6")
}

// TestYANGLeafRestrictionsValidateParse verifies YANG enum and range
// restrictions are enforced by the config parser, not merely extracted into the
// schema.
//
// VALIDATES: leaf enum/range restrictions from YANG reject invalid values.
// PREVENTS: `ze config validate` accepting values outside schema constraints.
func TestYANGLeafRestrictionsValidateParse(t *testing.T) {
	schema, err := YANGSchemaWithPlugins(map[string]string{
		"ze-restriction-test-conf.yang": `
module ze-restriction-test-conf {
  namespace "urn:ze:restriction-test";
  prefix zrt;

  container restriction-test {
    leaf mode {
      type enumeration {
        enum fast;
        enum slow;
      }
    }
    leaf poll-interval {
      type uint16 {
        range "1..10";
      }
    }
  }
}`,
	})
	require.NoError(t, err)

	_, err = NewParser(schema).Parse(`restriction-test { mode turbo; }`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mode")
	assert.Contains(t, err.Error(), "invalid enum")

	_, err = NewParser(schema).Parse(`restriction-test { poll-interval 0; }`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "poll-interval")
	assert.Contains(t, err.Error(), "outside range")

	_, err = NewParser(schema).Parse(`restriction-test { mode fast; poll-interval 10; }`)
	require.NoError(t, err)
}
