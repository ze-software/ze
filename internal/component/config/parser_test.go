package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSchema returns a schema for testing.
func testSchema() *Schema {
	schema := NewSchema()

	schema.Define("router-id", Leaf(TypeIPv4))
	schema.Define("local-as", Leaf(TypeUint32))

	schema.Define("neighbor", List(TypeIP,
		Field("description", Leaf(TypeString)),
		Field("router-id", Leaf(TypeIPv4)),
		Field("local-address", Leaf(TypeIP)),
		Field("local-as", Leaf(TypeUint32)),
		Field("peer-as", Leaf(TypeUint32)),
		Field("receive-hold-time", LeafWithDefault(TypeUint16, "90")),
		Field("family", Container(
			Field("ipv4", Container(
				Field("unicast", Leaf(TypeBool)),
				Field("multicast", Leaf(TypeBool)),
			)),
			Field("ipv6", Container(
				Field("unicast", Leaf(TypeBool)),
			)),
		)),
		Field("static", Container(
			Field("route", List(TypePrefix,
				Field("next-hop", Leaf(TypeIP)),
				Field("community", Leaf(TypeString)),
			)),
		)),
	))

	schema.Define("process", List(TypeString,
		Field("run", Leaf(TypeString)),
		Field("encoder", Leaf(TypeString)),
	))

	return schema
}

// TestParserSimpleLeaf verifies parsing a simple leaf value.
//
// VALIDATES: Top-level leaves are parsed correctly.
//
// PREVENTS: Lost simple configuration values.
func TestParserSimpleLeaf(t *testing.T) {
	input := `router-id 1.2.3.4`

	p := NewParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)
	require.NotNil(t, tree)

	val, ok := tree.Get("router-id")
	require.True(t, ok)
	require.Equal(t, "1.2.3.4", val)
}

// TestParserNeighborBlock verifies parsing a neighbor block.
//
// VALIDATES: List entries with children are parsed.
//
// PREVENTS: Lost neighbor configuration.
func TestParserNeighborBlock(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000
    peer-as 65001
    router-id 1.2.3.4
}
`

	p := NewParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	// Access neighbor
	neighbors := tree.GetList("neighbor")
	require.Len(t, neighbors, 1)

	n := neighbors["192.0.2.1"]
	require.NotNil(t, n)

	val, _ := n.Get("local-as")
	require.Equal(t, "65000", val)

	val, _ = n.Get("peer-as")
	require.Equal(t, "65001", val)
}

// TestParserMultipleNeighbors verifies multiple list entries.
//
// VALIDATES: Multiple neighbors are parsed independently.
//
// PREVENTS: Overwritten neighbor configs.
func TestParserMultipleNeighbors(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000
    peer-as 65001
}

neighbor 192.0.2.2 {
    local-as 65000
    peer-as 65002
}
`

	p := NewParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	require.Len(t, neighbors, 2)

	n1 := neighbors["192.0.2.1"]
	val, _ := n1.Get("peer-as")
	require.Equal(t, "65001", val)

	n2 := neighbors["192.0.2.2"]
	val, _ = n2.Get("peer-as")
	require.Equal(t, "65002", val)
}

// TestParserNestedContainer verifies nested containers.
//
// VALIDATES: Nested containers are parsed correctly.
//
// PREVENTS: Flattened nested config.
func TestParserNestedContainer(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000
    peer-as 65001
    family {
        ipv4 {
            unicast true
        }
        ipv6 {
            unicast true
        }
    }
}
`

	p := NewParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	n := neighbors["192.0.2.1"]

	fam := n.GetContainer("family")
	require.NotNil(t, fam)

	ipv4 := fam.GetContainer("ipv4")
	require.NotNil(t, ipv4)

	val, _ := ipv4.Get("unicast")
	require.Equal(t, "true", val)
}

// TestParserNestedList verifies list inside container.
//
// VALIDATES: Lists can be nested inside containers.
//
// PREVENTS: Lost nested list entries.
func TestParserNestedList(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000
    peer-as 65001
    static {
        route 10.0.0.0/8 {
            next-hop 192.0.2.1
        }
        route 172.16.0.0/12 {
            next-hop 192.0.2.1
        }
    }
}
`

	p := NewParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	n := neighbors["192.0.2.1"]

	static := n.GetContainer("static")
	require.NotNil(t, static)

	routes := static.GetList("route")
	require.Len(t, routes, 2)

	r1 := routes["10.0.0.0/8"]
	val, _ := r1.Get("next-hop")
	require.Equal(t, "192.0.2.1", val)
}

// TestParserProcess verifies process block (string-keyed list).
//
// VALIDATES: String-keyed lists work.
//
// PREVENTS: Only IP-keyed lists working.
func TestParserProcess(t *testing.T) {
	input := `
process announce-routes {
    run "/usr/bin/exabgp-announce"
    encoder json
}
`

	p := NewParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	procs := tree.GetList("process")
	require.Len(t, procs, 1)

	proc := procs["announce-routes"]
	require.NotNil(t, proc)

	val, _ := proc.Get("run")
	require.Equal(t, "/usr/bin/exabgp-announce", val)

	val, _ = proc.Get("encoder")
	require.Equal(t, "json", val)
}

// TestParserValidationError verifies type validation.
//
// VALIDATES: Invalid values are rejected.
//
// PREVENTS: Invalid config being accepted.
func TestParserValidationError(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as not-a-number
}
`

	p := NewParser(testSchema())
	_, err := p.Parse(input)

	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid")
}

// TestParserUnknownField verifies unknown field rejection.
//
// VALIDATES: Unknown fields are rejected.
//
// PREVENTS: Silent config typos.
func TestParserUnknownField(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    unknown-field value
}
`

	p := NewParser(testSchema())
	_, err := p.Parse(input)

	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown")
}

// TestParserUnknownTopLevel verifies unknown top-level rejection.
//
// VALIDATES: Unknown top-level blocks are rejected.
//
// PREVENTS: Ignored config sections.
func TestParserUnknownTopLevel(t *testing.T) {
	input := `
unknown-block {
    something value
}
`

	p := NewParser(testSchema())
	_, err := p.Parse(input)

	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown")
}

// TestParserQuotedValues verifies quoted string handling.
//
// VALIDATES: Quoted strings preserve spaces.
//
// PREVENTS: Broken paths or descriptions.
func TestParserQuotedValues(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000
    peer-as 65001
    description "My BGP Peer"
}
`

	p := NewParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	n := neighbors["192.0.2.1"]

	val, _ := n.Get("description")
	require.Equal(t, "My BGP Peer", val)
}

// TestParserLineNumbers verifies error line reporting.
//
// VALIDATES: Errors include line numbers.
//
// PREVENTS: Hard-to-find config errors.
func TestParserLineNumbers(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000
    unknown-field value
}
`

	p := NewParser(testSchema())
	_, err := p.Parse(input)

	require.Error(t, err)
	require.Contains(t, err.Error(), "line 4")
}

// TestParserArray verifies array syntax parsing.
//
// VALIDATES: [ item1 item2 ] arrays are parsed.
//
// PREVENTS: Broken API process lists.
func TestParserArray(t *testing.T) {
	schema := NewSchema()
	schema.Define("items", BracketLeafList(TypeString))

	input := `items [ foo bar baz ]`

	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	val, ok := tree.Get("items")
	require.True(t, ok)
	require.Equal(t, "foo bar baz", val) // stored space-separated
}

// TestParserArraySingle verifies single-item array.
//
// VALIDATES: Single item arrays work.
//
// PREVENTS: Edge case failures.
func TestParserArraySingle(t *testing.T) {
	schema := NewSchema()
	schema.Define("items", BracketLeafList(TypeString))

	input := `items [ single ]`

	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	val, ok := tree.Get("items")
	require.True(t, ok)
	require.Equal(t, "single", val)
}

// TestParserInlineContainer verifies the parser accepts inline container form.
//
// VALIDATES: AC-4 -- parser accepts "local ip 1.2.3.4" as inline container.
//
// PREVENTS: Parse errors on inline serializer output.
func TestParserInlineContainer(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)

	// Inline form: "remote ip 192.0.2.1" without braces
	input := `bgp {
	peer peer1 {
		connection {
			remote ip 192.0.2.1
		}
		session {
			asn local 65000
		}
	}
}
`
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	// Verify the tree structure is correct
	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp)

	peers := bgp.GetList("peer")
	require.Contains(t, peers, "peer1")

	peer := peers["peer1"]
	conn := peer.GetContainer("connection")
	require.NotNil(t, conn)

	remote := conn.GetContainer("remote")
	require.NotNil(t, remote)

	ip, ok := remote.Get("ip")
	require.True(t, ok)
	require.Equal(t, "192.0.2.1", ip)

	session := peer.GetContainer("session")
	require.NotNil(t, session)

	asn := session.GetContainer("asn")
	require.NotNil(t, asn)

	local, ok := asn.Get("local")
	require.True(t, ok)
	require.Equal(t, "65000", local)
}

// TestParserInlineBlockEquivalent verifies inline and block forms produce the same tree.
//
// VALIDATES: AC-5 -- inline and block produce identical Tree.
//
// PREVENTS: Semantic differences between forms.
func TestParserInlineBlockEquivalent(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err)
	p := NewParser(schema)

	block := `bgp {
	peer peer1 {
		connection {
			remote {
				ip 192.0.2.1
			}
		}
		session {
			asn {
				local 65000
			}
		}
	}
}
`
	inline := `bgp {
	peer peer1 {
		connection {
			remote ip 192.0.2.1
		}
		session {
			asn local 65000
		}
	}
}
`
	treeBlock, err := p.Parse(block)
	require.NoError(t, err)

	treeInline, err := p.Parse(inline)
	require.NoError(t, err)

	require.True(t, TreeEqual(treeBlock, treeInline), "block and inline forms should produce identical trees")
}

// TestTreeClone verifies deep cloning of Tree.
//
// VALIDATES: Clone creates independent copy with all data.
//
// PREVENTS: Mutations affecting original during migration.
func TestTreeClone(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000
    peer-as 65001
}
`
	p := NewParser(testSchema())
	original, err := p.Parse(input)
	require.NoError(t, err)

	// Clone the tree
	cloned := original.Clone()
	require.NotNil(t, cloned)

	// Verify data is preserved
	neighbors := cloned.GetList("neighbor")
	require.Len(t, neighbors, 1)
	n := neighbors["192.0.2.1"]
	require.NotNil(t, n)
	val, _ := n.Get("local-as")
	require.Equal(t, "65000", val)

	// Verify independence: modify clone, original unchanged
	cloned.Set("router-id", "9.9.9.9")
	_, ok := original.Get("router-id")
	require.False(t, ok, "original should not have router-id after clone modification")

	// Verify independence: modify cloned neighbor
	n.Set("receive-hold-time", "30")
	origNeighbors := original.GetList("neighbor")
	origN := origNeighbors["192.0.2.1"]
	_, ok = origN.Get("receive-hold-time")
	require.False(t, ok, "original neighbor should not have receive-hold-time after clone modification")
}

// TestTreeDeleteValue verifies Tree.Delete removes a leaf value.
//
// VALIDATES: Delete removes an existing leaf value and its valuesOrder entry.
// PREVENTS: Stale values remaining in tree after deletion.
func TestTreeDeleteValue(t *testing.T) {
	tree := NewTree()
	tree.Set("router-id", "1.2.3.4")
	tree.Set("local-as", "65000")

	// Delete existing key
	tree.Delete("router-id")

	_, ok := tree.Get("router-id")
	require.False(t, ok, "deleted value should not be present")

	// Other value should still exist
	val, ok := tree.Get("local-as")
	require.True(t, ok)
	require.Equal(t, "65000", val)
}

// TestTreeDeleteValueOrder verifies Tree.Delete also removes from valuesOrder.
//
// VALIDATES: After Delete, Values() no longer includes the deleted key.
// PREVENTS: Orphaned keys in valuesOrder causing stale iteration.
func TestTreeDeleteValueOrder(t *testing.T) {
	tree := NewTree()
	tree.Set("a", "1")
	tree.Set("b", "2")
	tree.Set("c", "3")

	tree.Delete("b")

	values := tree.Values()
	require.Equal(t, []string{"a", "c"}, values)
}

// TestTreeDeleteNonexistent verifies Tree.Delete on a missing key is a no-op.
//
// VALIDATES: Delete on nonexistent key does not panic or corrupt state.
// PREVENTS: Panic on deleting keys that don't exist.
func TestTreeDeleteNonexistent(t *testing.T) {
	tree := NewTree()
	tree.Set("a", "1")

	// Should not panic
	tree.Delete("nonexistent")

	// Original value still intact
	val, ok := tree.Get("a")
	require.True(t, ok)
	require.Equal(t, "1", val)
}

// VALIDATES: InsertMultiValue places values at the correct position.
// PREVENTS: Wrong insertion order in leaf-list manipulation.
func TestInsertMultiValue(t *testing.T) {
	tests := []struct {
		name     string
		initial  []string
		value    string
		position string
		ref      string
		expected []string
		wantErr  bool
	}{
		{
			name:     "first into empty",
			initial:  nil,
			value:    "a",
			position: InsertFirst,
			expected: []string{"a"},
		},
		{
			name:     "last into empty",
			initial:  nil,
			value:    "a",
			position: InsertLast,
			expected: []string{"a"},
		},
		{
			name:     "first into existing",
			initial:  []string{"b", "c"},
			value:    "a",
			position: InsertFirst,
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "last into existing",
			initial:  []string{"a", "b"},
			value:    "c",
			position: InsertLast,
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "before middle",
			initial:  []string{"a", "c"},
			value:    "b",
			position: InsertBefore,
			ref:      "c",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "after middle",
			initial:  []string{"a", "c"},
			value:    "b",
			position: InsertAfter,
			ref:      "a",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "before first",
			initial:  []string{"b", "c"},
			value:    "a",
			position: InsertBefore,
			ref:      "b",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "after last",
			initial:  []string{"a", "b"},
			value:    "c",
			position: InsertAfter,
			ref:      "b",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "before nonexistent ref",
			initial:  []string{"a", "b"},
			value:    "c",
			position: InsertBefore,
			ref:      "missing",
			wantErr:  true,
		},
		{
			name:     "invalid position",
			initial:  []string{"a"},
			value:    "b",
			position: "middle",
			wantErr:  true,
		},
		{
			name:     "duplicate value rejected",
			initial:  []string{"a", "b"},
			value:    "a",
			position: InsertLast,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewTree()
			if tt.initial != nil {
				tree.SetSlice("items", tt.initial)
			}

			err := tree.InsertMultiValue("items", tt.value, tt.position, tt.ref)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, tree.GetSlice("items"))

			// Verify values map stays in sync.
			v, ok := tree.Get("items")
			require.True(t, ok)
			require.Equal(t, strings.Join(tt.expected, " "), v)
		})
	}
}

// VALIDATES: DeactivateMultiValue adds inactive: prefix to leaf-list value.
// PREVENTS: Deactivation silently ignored for missing values.
func TestDeactivateMultiValue(t *testing.T) {
	tree := NewTree()
	tree.SetSlice("import", []string{"no-self-as", "reject-bogons"})

	err := tree.DeactivateMultiValue("import", "no-self-as")
	require.NoError(t, err)
	require.Equal(t, []string{"inactive:no-self-as", "reject-bogons"}, tree.GetSlice("import"))

	// Verify values map in sync.
	v, _ := tree.Get("import")
	require.Equal(t, "inactive:no-self-as reject-bogons", v)

	// Deactivating nonexistent value fails.
	err = tree.DeactivateMultiValue("import", "missing")
	require.Error(t, err)

	// Double-deactivation fails.
	err = tree.DeactivateMultiValue("import", "no-self-as")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already deactivated")
}

// VALIDATES: ActivateMultiValue removes inactive: prefix from leaf-list value.
// PREVENTS: Activation silently ignored for values without prefix.
func TestActivateMultiValue(t *testing.T) {
	tree := NewTree()
	tree.SetSlice("import", []string{"inactive:no-self-as", "reject-bogons"})

	err := tree.ActivateMultiValue("import", "no-self-as")
	require.NoError(t, err)
	require.Equal(t, []string{"no-self-as", "reject-bogons"}, tree.GetSlice("import"))

	// Activating value without inactive: prefix fails.
	err = tree.ActivateMultiValue("import", "reject-bogons")
	require.Error(t, err)
}
