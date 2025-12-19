package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSchemaLeaf verifies leaf node creation and type validation.
//
// VALIDATES: Leaf nodes store typed values correctly.
//
// PREVENTS: Type mismatches in configuration values.
func TestSchemaLeaf(t *testing.T) {
	schema := NewSchema()

	// Define a simple leaf
	schema.Define("router-id", Leaf(TypeIPv4))
	schema.Define("local-as", Leaf(TypeUint32))
	schema.Define("passive", Leaf(TypeBool))
	schema.Define("description", Leaf(TypeString))

	require.True(t, schema.Has("router-id"))
	require.True(t, schema.Has("local-as"))
	require.True(t, schema.Has("passive"))
	require.False(t, schema.Has("unknown"))

	node := schema.Get("router-id")
	require.NotNil(t, node)
	require.Equal(t, NodeLeaf, node.Kind())
	require.Equal(t, TypeIPv4, node.(*LeafNode).Type)
}

// TestSchemaContainer verifies container node with children.
//
// VALIDATES: Containers hold child nodes correctly.
//
// PREVENTS: Lost nested configuration structure.
func TestSchemaContainer(t *testing.T) {
	schema := NewSchema()

	// Define a container with children
	schema.Define("process", Container(
		Field("run", Leaf(TypeString)),
		Field("encoder", Leaf(TypeString)),
	))

	require.True(t, schema.Has("process"))

	node := schema.Get("process")
	require.NotNil(t, node)
	require.Equal(t, NodeContainer, node.Kind())

	container := node.(*ContainerNode)
	require.True(t, container.Has("run"))
	require.True(t, container.Has("encoder"))
}

// TestSchemaList verifies list node with key.
//
// VALIDATES: Lists are keyed collections of containers.
//
// PREVENTS: Duplicate list entries or missing keys.
func TestSchemaList(t *testing.T) {
	schema := NewSchema()

	// Define a list keyed by IP address
	schema.Define("neighbor", List(TypeIPv4,
		Field("local-as", Leaf(TypeUint32)),
		Field("peer-as", Leaf(TypeUint32)),
		Field("router-id", Leaf(TypeIPv4)),
		Field("hold-time", Leaf(TypeUint16)),
		Field("passive", Leaf(TypeBool)),
	))

	require.True(t, schema.Has("neighbor"))

	node := schema.Get("neighbor")
	require.NotNil(t, node)
	require.Equal(t, NodeList, node.Kind())

	list := node.(*ListNode)
	require.Equal(t, TypeIPv4, list.KeyType)
	require.True(t, list.Has("local-as"))
	require.True(t, list.Has("peer-as"))
}

// TestSchemaNestedContainers verifies deeply nested structures.
//
// VALIDATES: Containers can be nested arbitrarily deep.
//
// PREVENTS: Flattening of hierarchical config.
func TestSchemaNestedContainers(t *testing.T) {
	schema := NewSchema()

	schema.Define("neighbor", List(TypeIPv4,
		Field("family", Container(
			Field("ipv4", Container(
				Field("unicast", Leaf(TypeBool)),
				Field("multicast", Leaf(TypeBool)),
			)),
			Field("ipv6", Container(
				Field("unicast", Leaf(TypeBool)),
			)),
		)),
	))

	node := schema.Get("neighbor")
	list := node.(*ListNode)

	family := list.Get("family").(*ContainerNode)
	require.NotNil(t, family)

	ipv4 := family.Get("ipv4").(*ContainerNode)
	require.NotNil(t, ipv4)
	require.True(t, ipv4.Has("unicast"))
	require.True(t, ipv4.Has("multicast"))
}

// TestSchemaPath verifies path-based node lookup.
//
// VALIDATES: Nodes can be found by dot-separated paths.
//
// PREVENTS: Manual tree traversal errors.
func TestSchemaPath(t *testing.T) {
	schema := NewSchema()

	schema.Define("neighbor", List(TypeIPv4,
		Field("family", Container(
			Field("ipv4", Container(
				Field("unicast", Leaf(TypeBool)),
			)),
		)),
	))

	// Lookup by path
	node, err := schema.Lookup("neighbor.family.ipv4.unicast")
	require.NoError(t, err)
	require.NotNil(t, node)
	require.Equal(t, NodeLeaf, node.Kind())
}

// TestSchemaValidateValue verifies type validation.
//
// VALIDATES: Values are validated against their declared type.
//
// PREVENTS: Invalid configuration values.
func TestSchemaValidateValue(t *testing.T) {
	tests := []struct {
		typ   ValueType
		value string
		valid bool
	}{
		{TypeUint32, "65000", true},
		{TypeUint32, "4294967295", true},
		{TypeUint32, "-1", false},
		{TypeUint32, "abc", false},
		{TypeUint16, "179", true},
		{TypeUint16, "65536", false},
		{TypeBool, "true", true},
		{TypeBool, "false", true},
		{TypeBool, "yes", false},
		{TypeIPv4, "192.0.2.1", true},
		{TypeIPv4, "192.0.2.256", false},
		{TypeIPv4, "::1", false},
		{TypeIPv6, "2001:db8::1", true},
		{TypeIPv6, "192.0.2.1", false},
		{TypeIP, "192.0.2.1", true},
		{TypeIP, "2001:db8::1", true},
		{TypeString, "anything", true},
		{TypePrefix, "10.0.0.0/8", true},
		{TypePrefix, "10.0.0.0", true}, // plain IP allowed as /32
	}

	for _, tc := range tests {
		err := ValidateValue(tc.typ, tc.value)
		if tc.valid {
			require.NoError(t, err, "expected %q valid for %v", tc.value, tc.typ)
		} else {
			require.Error(t, err, "expected %q invalid for %v", tc.value, tc.typ)
		}
	}
}

// TestSchemaDefault verifies default value handling.
//
// VALIDATES: Leaves can have default values.
//
// PREVENTS: Missing required defaults in config.
func TestSchemaDefault(t *testing.T) {
	schema := NewSchema()

	schema.Define("hold-time", LeafWithDefault(TypeUint16, "90"))
	schema.Define("passive", LeafWithDefault(TypeBool, "false"))

	node := schema.Get("hold-time").(*LeafNode)
	require.Equal(t, "90", node.Default)

	node = schema.Get("passive").(*LeafNode)
	require.Equal(t, "false", node.Default)
}
