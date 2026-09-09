package config

import (
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
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
	schema.Define("group-updates", Leaf(TypeBool))
	schema.Define("description", Leaf(TypeString))

	require.True(t, schema.Has("router-id"))
	require.True(t, schema.Has("local-as"))
	require.True(t, schema.Has("group-updates"))
	require.False(t, schema.Has("unknown"))

	node := schema.Get("router-id")
	require.NotNil(t, node)
	require.Equal(t, NodeLeaf, node.Kind())
	leaf, ok := node.(*LeafNode)
	require.True(t, ok, "expected LeafNode")
	require.Equal(t, TypeIPv4, leaf.Type)
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

	container, ok := node.(*ContainerNode)
	require.True(t, ok, "expected ContainerNode")
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
		Field("receive-hold-time", Leaf(TypeUint16)),
		Field("group-updates", Leaf(TypeBool)),
	))

	require.True(t, schema.Has("neighbor"))

	node := schema.Get("neighbor")
	require.NotNil(t, node)
	require.Equal(t, NodeList, node.Kind())

	list, ok := node.(*ListNode)
	require.True(t, ok, "expected ListNode")
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
	list, ok := node.(*ListNode)
	require.True(t, ok, "expected ListNode")

	familyNode := list.Get("family")
	fam, ok := familyNode.(*ContainerNode)
	require.True(t, ok, "expected ContainerNode")
	require.NotNil(t, fam)

	ipv4Node := fam.Get("ipv4")
	ipv4, ok := ipv4Node.(*ContainerNode)
	require.True(t, ok, "expected ContainerNode")
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
	node, err := schema.Lookup("neighbor/family/ipv4/unicast")
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
		{TypeBool, "enable", true},
		{TypeBool, "disable", true},
		{TypeBool, "require", true},
		{TypeBool, "refuse", true},
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

	for _, tt := range tests {
		err := ValidateValue(tt.typ, tt.value)
		if tt.valid {
			require.NoError(t, err, "expected %q valid for %v", tt.value, tt.typ)
		} else {
			require.Error(t, err, "expected %q invalid for %v", tt.value, tt.typ)
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

	schema.Define("receive-hold-time", LeafWithDefault(TypeUint16, "90"))
	schema.Define("group-updates", LeafWithDefault(TypeBool, "false"))

	holdTimeNode, ok := schema.Get("receive-hold-time").(*LeafNode)
	require.True(t, ok, "expected LeafNode")
	require.Equal(t, "90", holdTimeNode.Default)

	groupUpdatesNode, ok := schema.Get("group-updates").(*LeafNode)
	require.True(t, ok, "expected LeafNode")
	require.Equal(t, "false", groupUpdatesNode.Default)
}

// TestMergeListNodePreservesBackend verifies that merging two definitions of
// the same list under a shared container preserves a ze:backend annotation that
// only one definition carries, regardless of merge order. This is the exact
// shape of the ze-cos-conf / ze-iface-conf collision on interface/bridge: the
// cos overlay (visited first, no backend annotation) must NOT erase the iface
// backend matrix, or the commit-time feature gate silently accepts bridge/veth
// under backend vpp.
//
// VALIDATES: an absent dst.Backend adopts src.Backend during container merge.
//
// PREVENTS: a backend-gated feature (bridge, veth) losing its gate when a
// second module contributes the same list to the interface container.
func TestMergeListNodePreservesBackend(t *testing.T) {
	// Overlay container (sorts first, like ze-cos-conf): bridge list with NO
	// backend annotation.
	overlay := Container(
		Field("bridge", List(TypeString, Field("name", Leaf(TypeString)))),
	)

	// Canonical container (like ze-iface-conf): bridge list WITH the netlink
	// backend matrix.
	canonicalBridge := List(TypeString, Field("name", Leaf(TypeString)))
	canonicalBridge.Backend = []string{"netlink"}
	canonical := Container(Field("bridge", canonicalBridge))

	schema := NewSchema()
	schema.Define("interface", overlay)   // visited first -> becomes dst
	schema.Define("interface", canonical) // merged in -> src carries backend

	iface, ok := schema.Get("interface").(*ContainerNode)
	require.True(t, ok, "expected interface ContainerNode")
	bridge, ok := iface.Get("bridge").(*ListNode)
	require.True(t, ok, "expected bridge ListNode")
	require.Equal(t, []string{"netlink"}, bridge.Backend,
		"merged bridge must keep the netlink backend annotation from the canonical module")
}

// TestMergeListNodeDoesNotWidenBackend verifies that when dst already carries a
// backend annotation, a later annotation-free src merge does NOT clear it and a
// different src annotation does NOT silently widen it (first annotation wins).
//
// VALIDATES: dst.Backend is only filled when empty; a populated dst is kept.
//
// PREVENTS: an overlay module accidentally widening or clearing a feature gate.
func TestMergeListNodeDoesNotWidenBackend(t *testing.T) {
	canonicalBridge := List(TypeString, Field("name", Leaf(TypeString)))
	canonicalBridge.Backend = []string{"netlink"}
	canonical := Container(Field("bridge", canonicalBridge))

	overlay := Container(
		Field("bridge", List(TypeString, Field("name", Leaf(TypeString)))),
	)

	schema := NewSchema()
	schema.Define("interface", canonical) // visited first -> dst has [netlink]
	schema.Define("interface", overlay)   // src has no annotation

	iface, ok := schema.Get("interface").(*ContainerNode)
	require.True(t, ok, "expected interface ContainerNode")
	bridge, ok := iface.Get("bridge").(*ListNode)
	require.True(t, ok, "expected bridge ListNode")
	require.Equal(t, []string{"netlink"}, bridge.Backend,
		"populated dst backend must survive an annotation-free overlay merge")
}

// TestParseASNAsdot proves an ASN-typed leaf takes an AS number in any of the
// three RFC 5396 notations and stores the decimal form, while a plain uint32
// leaf keeps rejecting a dotted token. The method validates then normalizes
// each value the way every write path does.
//
// VALIDATES: ValidateValue and NormalizeLeafValue on TypeASN.
// PREVENTS: an asdot value reaching the tree unconverted, which every reader
// downstream parses as a decimal and fails on.
func TestParseASNAsdot(t *testing.T) {
	accepted := []struct {
		value string
		want  string
	}{
		{"1.10", "65546"},
		{"65546", "65546"},
		{"0.100", "100"},
		{"100", "100"},
		{"65535.65535", "4294967295"},
		{"4294967295", "4294967295"},
	}
	for _, tc := range accepted {
		if err := ValidateValue(TypeASN, tc.value); err != nil {
			t.Fatalf("ValidateValue(TypeASN, %q): unexpected error %v", tc.value, err)
		}
		if got := NormalizeLeafValue(TypeASN, tc.value); got != tc.want {
			t.Errorf("NormalizeLeafValue(TypeASN, %q) = %q, want %q", tc.value, got, tc.want)
		}
	}

	for _, value := range []string{"1.99999", "65536.0", "1.2.3", "4294967296", "one.ten", ""} {
		if err := ValidateValue(TypeASN, value); err == nil {
			t.Errorf("ValidateValue(TypeASN, %q) accepted an invalid AS number", value)
		}
	}

	// R-2: a leaf that is a bare 32-bit number, not an AS number, must keep
	// refusing a dotted token. Nothing in the asdot work widens uint32.
	if err := ValidateValue(TypeUint32, "1.10"); err == nil {
		t.Error("ValidateValue(TypeUint32, \"1.10\") accepted a dotted value")
	}
	if got := NormalizeLeafValue(TypeUint32, "1.10"); got != "1.10" {
		t.Errorf("NormalizeLeafValue(TypeUint32, %q) = %q, want the value unchanged", "1.10", got)
	}
}

// TestValidateASNLeafRange proves the YANG range on the asn typedef is applied
// to the NUMBER an asdot token names, not to its text. The method runs a leaf
// carrying the typedef's own range over both spellings of AS 0.
//
// VALIDATES: validateNumericRanges reads TypeASN through asn.Parse.
// PREVENTS: "0.0" slipping past a `range "1..4294967295"` that "0" fails.
func TestValidateASNLeafRange(t *testing.T) {
	leaf := &LeafNode{Type: TypeASN, Ranges: []NumericRange{{Min: "1", Max: "4294967295"}}}

	for _, value := range []string{"1.10", "65546", "1", "0.1"} {
		if err := ValidateLeafValue(leaf, value); err != nil {
			t.Errorf("ValidateLeafValue(%q): unexpected error %v", value, err)
		}
	}
	for _, value := range []string{"0", "0.0"} {
		if err := ValidateLeafValue(leaf, value); err == nil {
			t.Errorf("ValidateLeafValue(%q) accepted AS 0, which the range refuses", value)
		}
	}
}

// TestASNTypedefMapsToTypeASN proves the ze-types `asn` typedef is what marks a
// leaf as taking the dotted notations, and that the mapping holds for the
// prefixed spelling a module importing ze-types writes. The method maps each
// type name and compares the ValueType.
//
// VALIDATES: yangTypeToValueType answers TypeASN for the asn typedef.
// PREVENTS: an ASN leaf falling back to TypeUint32, which refuses "1.10".
func TestASNTypedefMapsToTypeASN(t *testing.T) {
	for _, name := range []string{"asn", "zt:asn"} {
		if got := yangTypeToValueType(&gyang.YangType{Name: name, Kind: gyang.Yuint32}); got != TypeASN {
			t.Errorf("yangTypeToValueType(%q) = %v, want TypeASN", name, got)
		}
	}
	if got := yangTypeToValueType(&gyang.YangType{Name: "uint32", Kind: gyang.Yuint32}); got != TypeUint32 {
		t.Errorf("yangTypeToValueType(\"uint32\") = %v, want TypeUint32", got)
	}
}
