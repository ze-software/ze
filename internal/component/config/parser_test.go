package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config/secret"
)

// testSchema returns a schema for testing.
func testSchema() *Schema {
	schema := NewSchema()

	schema.Define("router-id", Leaf(TypeIPv4))
	schema.Define("local-as", Leaf(TypeUint32))

	schema.Define("name-server", ValueOrArray(TypeIP))

	// as-path is an ordered SEQUENCE (ze:ordered): duplicate ASNs are meaningful
	// prepends and must NOT be deduplicated the way a set leaf-list is.
	orderedASPath := ValueOrArray(TypeString)
	orderedASPath.Ordered = true
	schema.Define("as-path", orderedASPath)

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

// TestParserLeafListRepeatedStatements verifies that a leaf-list spelled as
// repeated `name value;` statements accumulates per YANG (RFC 7950 sec 7.7),
// rather than the last statement silently winning.
//
// VALIDATES: repeated leaf-list statements append; the bracket form and a single
// statement still produce the same members; duplicates collapse (leaf-lists are sets).
//
// PREVENTS: a regression where two `instance-id 0; instance-id 5;` lines kept only
// instance 5 -- silent config data loss (the ospf-instance-demux failure).
func TestParserLeafListRepeatedStatements(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{"repeated statements accumulate", "name-server 9.9.9.9;\nname-server 8.8.8.8;\n", []string{"9.9.9.9", "8.8.8.8"}},
		{"bracket form", "name-server [ 9.9.9.9 8.8.8.8 ];\n", []string{"9.9.9.9", "8.8.8.8"}},
		{"single statement", "name-server 9.9.9.9;\n", []string{"9.9.9.9"}},
		{"duplicate collapses", "name-server 9.9.9.9;\nname-server 9.9.9.9;\n", []string{"9.9.9.9"}},
		{"bracket then repeated append", "name-server [ 9.9.9.9 ];\nname-server 8.8.8.8;\n", []string{"9.9.9.9", "8.8.8.8"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := NewParser(testSchema()).Parse(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.want, tree.GetSlice("name-server"))
			// The scalar mirror reflects the full accumulated list, not the last statement.
			v, _ := tree.Get("name-server")
			require.Equal(t, strings.Join(tc.want, " "), v)
			// ToMap delivers the leaf-list to plugins as an array (>1 member).
			if len(tc.want) > 1 {
				require.Equal(t, tc.want, tree.ToMap()["name-server"])
			}
		})
	}
}

// TestParserOrderedLeafListPreservesDuplicates locks the ze:ordered behavior: a
// leaf-list modeling an ordered SEQUENCE (AS_PATH, MPLS labels) keeps duplicate
// values instead of deduplicating them as a set. Duplicate ASNs are load-bearing
// prepends (RFC 4271 Section 5.1.2), so collapsing `as-path [ 30740 30740 ... ]`
// to a single 30740 silently drops the prepend and changes what ze advertises.
//
// VALIDATES: an Ordered ValueOrArray node preserves order and duplicates in both the
// bracket form and repeated-statement form, while a non-ordered set still dedups.
//
// PREVENTS: the l2vpn AS_PATH regression (encode 31 / exabgp conf-l2vpn) where
// `as-path [ 30740 x7 ]` was stored as a single ASN.
func TestParserOrderedLeafListPreservesDuplicates(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{"bracket keeps duplicates", "as-path [ 30740 30740 30740 30740 30740 30740 30740 ];\n", []string{"30740", "30740", "30740", "30740", "30740", "30740", "30740"}},
		{"repeated statements keep duplicates", "as-path 65001;\nas-path 65001;\n", []string{"65001", "65001"}},
		{"distinct sequence preserved in order", "as-path [ 65001 65002 65001 ];\n", []string{"65001", "65002", "65001"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := NewParser(testSchema()).Parse(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.want, tree.GetSlice("as-path"))
			// The scalar mirror joins every member, so config-route parsing
			// (GetSlice -> join -> ParseASPath) recovers the full ordered path.
			v, _ := tree.Get("as-path")
			require.Equal(t, strings.Join(tc.want, " "), v)
		})
	}

	// A non-ordered set leaf-list must still deduplicate: the ordered flag is the
	// only thing that changes the behavior.
	setTree, err := NewParser(testSchema()).Parse("name-server 9.9.9.9;\nname-server 9.9.9.9;\n")
	require.NoError(t, err)
	require.Equal(t, []string{"9.9.9.9"}, setTree.GetSlice("name-server"))
}

// TestParserOrderedLeafListRejectsAmbiguousDeactivation locks the fail-closed guard
// that keeps duplicate preservation from opening an AS_PATH-corruption hole:
// deactivation is value-keyed, so deactivating one of several identical members
// would blank every copy. An ordered leaf-list rejects that; a unique member still
// deactivates cleanly.
//
// PREVENTS: `as-path [ 65001 inactive:65001 ]` silently collapsing to an empty
// AS_PATH (all copies of 65001 marked inactive).
func TestParserOrderedLeafListRejectsAmbiguousDeactivation(t *testing.T) {
	// Deactivating a REPEATED value in an ordered leaf-list is a parse error.
	_, err := NewParser(testSchema()).Parse("as-path [ 65001 inactive:65001 ];\n")
	require.Error(t, err)
	require.Contains(t, err.Error(), "as-path")

	// Deactivating a UNIQUE value still works; the remaining member survives.
	tree, err := NewParser(testSchema()).Parse("as-path [ 65001 inactive:65002 ];\n")
	require.NoError(t, err)
	require.Equal(t, []string{"65001"}, tree.GetSlice("as-path"))
}

// TestParserLeafListRepeatedStatementsDeactivation locks the scalar-mirror/GetSlice
// consistency when a member deactivated by an EARLIER statement is followed by a
// later plain statement: the mirror must NOT leak the deactivated member.
//
// VALIDATES: AppendSlice re-syncs the scalar mirror to active members, so Get() and
// GetSlice() agree even across repeated statements with an out-of-band deactivation.
//
// PREVENTS: a regression where `import inactive:X; import Y;` leaves Get()="X Y" while
// GetSlice()=["Y"] (the mirror leaking a deactivated member into scalar consumers).
func TestParserLeafListRepeatedStatementsDeactivation(t *testing.T) {
	tree, err := NewParser(testSchema()).Parse("name-server inactive:9.9.9.9;\nname-server 8.8.8.8;\n")
	require.NoError(t, err)
	// Effective view drops the deactivated member.
	require.Equal(t, []string{"8.8.8.8"}, tree.GetSlice("name-server"))
	// The scalar mirror must match the effective view, not leak 9.9.9.9.
	v, _ := tree.Get("name-server")
	require.Equal(t, "8.8.8.8", v)
	// Structural view keeps both, with 9.9.9.9 marked inactive.
	require.Equal(t, []MemberState{{Value: "9.9.9.9", Inactive: true}, {Value: "8.8.8.8"}},
		tree.GetMultiValuesState("name-server"))
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

// TestParserDuplicateListKeyRejected verifies repeated YANG list entries with
// the same key fail instead of being silently renamed with a #N suffix.
//
// VALIDATES: Duplicate YANG list keys are rejected during hierarchical parse.
//
// PREVENTS: Duplicate operator config becoming a different hidden key.
func TestParserDuplicateListKeyRejected(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "named block",
			input: `
neighbor 192.0.2.1 {
    peer-as 65001
}
neighbor 192.0.2.1 {
    peer-as 65002
}
`,
		},
		{
			name: "inline block",
			input: `
neighbor {
    192.0.2.1 first
    192.0.2.1 second
}
`,
		},
		{
			name: "anonymous block",
			input: `
process {
    run one
}
process {
    encoder two
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(testSchema())
			_, err := p.Parse(tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "duplicate list key")
		})
	}
}

func inlineRouteSchema() *Schema {
	schema := NewSchema()
	schema.Define("route", InlineList(TypePrefix,
		Field("next-hop", Leaf(TypeIP)),
		Field("path-information", Leaf(TypeIP)),
	))
	return schema
}

// TestParserInlineListDuplicatePathInformation verifies ADD-PATH style route
// entries may repeat a prefix only when path-information disambiguates them.
//
// VALIDATES: Inline route duplicates with distinct path-information parse as
// separate ordered entries.
//
// PREVENTS: ExaBGP ADD-PATH static routes being rejected as duplicate list keys.
func TestParserInlineListDuplicatePathInformation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "distinct path information allowed",
			input: `
route 10.0.0.10 next-hop 10.10.1.1 path-information 0.0.0.1;
route 10.0.0.10 next-hop 10.10.1.2 path-information 0.0.0.2;
`,
		},
		{
			name: "same path information rejected",
			input: `
route 10.0.0.10 next-hop 10.10.1.1 path-information 0.0.0.1;
route 10.0.0.10 next-hop 10.10.1.2 path-information 0.0.0.1;
`,
			wantErr: true,
		},
		{
			name: "missing path information rejected",
			input: `
route 10.0.0.10 next-hop 10.10.1.1;
route 10.0.0.10 next-hop 10.10.1.2;
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(inlineRouteSchema())
			tree, err := p.Parse(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "duplicate list key")
				return
			}

			require.NoError(t, err)
			routes := tree.GetListOrdered("route")
			require.Len(t, routes, 2)
			assert.Equal(t, "10.0.0.10", StripListKeySuffix(routes[0].Key))
			assert.Equal(t, "10.0.0.10", StripListKeySuffix(routes[1].Key))
			assert.Equal(t, "0.0.0.1", mustTreeValue(t, routes[0].Value, "path-information"))
			assert.Equal(t, "0.0.0.2", mustTreeValue(t, routes[1].Value, "path-information"))
		})
	}
}

func mustTreeValue(t *testing.T, tree *Tree, key string) string {
	t.Helper()
	value, ok := tree.Get(key)
	require.True(t, ok, "missing %s", key)
	return value
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

// TestParserArrayStoresSlice verifies a bracket leaf-list is stored as a SLICE,
// not only as the joined scalar mirror.
//
// VALIDATES: every member of `name [ a b ]` is retrievable as its own value, so
// consumers (and the JSON delivered to plugins via ToMap) see a list.
//
// PREVENTS: the multi-member regression this test was written for --
// parseBracketLeafList used to call only tree.Set(joined), so GetSlice returned
// nil and ToMap emitted "a b" as ONE string. Every consumer then parsed the
// joined text as a single value: `interface ... ipv4 { address [ a b ]; }`
// failed with `ParseAddr("10.0.0.1/24 10.0.0.2/24")`, i.e. ze could not put two
// addresses on a unit at all. It stayed invisible because every config in the
// tree used exactly one member. The sibling path (storeValueOrArray) always did
// SetSlice + Set; this one just forgot the SetSlice.
func TestParserArrayStoresSlice(t *testing.T) {
	schema := NewSchema()
	schema.Define("items", BracketLeafList(TypeString))

	p := NewParser(schema)
	tree, err := p.Parse(`items [ foo bar baz ]`)
	require.NoError(t, err)

	require.Equal(t, []string{"foo", "bar", "baz"}, tree.GetSlice("items"))

	// The scalar mirror stays joined: existing consumers read it via Get.
	val, ok := tree.Get("items")
	require.True(t, ok)
	require.Equal(t, "foo bar baz", val)
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

// VALIDATES: DeactivateMultiValue marks a leaf-list member deactivated
// out-of-band; the effective view drops it while the structural view keeps it.
// PREVENTS: Deactivation silently ignored for missing values.
func TestDeactivateMultiValue(t *testing.T) {
	tree := NewTree()
	tree.SetSlice("import", []string{"no-self-as", "reject-bogons"})

	err := tree.DeactivateMultiValue("import", "no-self-as")
	require.NoError(t, err)
	// Effective view: deactivated member excluded. No "inactive:" string.
	require.Equal(t, []string{"reject-bogons"}, tree.GetSlice("import"))
	if _, inactive := tree.MultiValueMemberState("import", "no-self-as"); true {
		require.True(t, inactive, "no-self-as recorded deactivated out-of-band")
	}
	require.Equal(t, []MemberState{{Value: "no-self-as", Inactive: true}, {Value: "reject-bogons"}},
		tree.GetMultiValuesState("import"))

	// Verify values map in sync (active-only join).
	v, _ := tree.Get("import")
	require.Equal(t, "reject-bogons", v)

	// Deactivating nonexistent value fails.
	err = tree.DeactivateMultiValue("import", "missing")
	require.Error(t, err)

	// Double-deactivation fails.
	err = tree.DeactivateMultiValue("import", "no-self-as")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already deactivated")
}

// VALIDATES: ActivateMultiValue clears the out-of-band deactivation marker.
// PREVENTS: Activation silently ignored for members that are not deactivated.
func TestActivateMultiValue(t *testing.T) {
	tree := NewTree()
	tree.SetSlice("import", []string{"no-self-as", "reject-bogons"})
	require.NoError(t, tree.DeactivateMultiValue("import", "no-self-as"))

	err := tree.ActivateMultiValue("import", "no-self-as")
	require.NoError(t, err)
	require.Equal(t, []string{"no-self-as", "reject-bogons"}, tree.GetSlice("import"))
	if _, inactive := tree.MultiValueMemberState("import", "no-self-as"); true {
		require.False(t, inactive, "no-self-as reactivated")
	}

	// Activating a member that is not deactivated fails.
	err = tree.ActivateMultiValue("import", "reject-bogons")
	require.Error(t, err)
}

// bcryptTestSchema returns a schema with two secret-shaped leaves for testing
// ze:sensitive ($9$) vs ze:bcrypt (verbatim) parser behavior divergence.
func bcryptTestSchema() *Schema {
	schema := NewSchema()
	secretLeaf := Leaf(TypeString)
	secretLeaf.Sensitive = true
	schema.Define("api-token", secretLeaf)

	hashLeaf := Leaf(TypeString)
	hashLeaf.Bcrypt = true
	schema.Define("password-hash", hashLeaf)
	return schema
}

// TestParserBcryptLeafNoSecretDecode verifies that a $9$-encoded value on a
// ze:bcrypt leaf is preserved verbatim. The $9$ reversible obfuscation must
// NOT be applied to bcrypt leaves -- bcrypt is one-way.
//
// VALIDATES: Parser skips $9$ decode on ze:bcrypt leaves.
//
// PREVENTS: Accidental plaintext extraction from bcrypt leaves that happen
// to hold a $9$-prefixed string.
func TestParserBcryptLeafNoSecretDecode(t *testing.T) {
	// A valid $9$-encoded string. On a Sensitive leaf this would decode
	// to plaintext; on a Bcrypt leaf it must be preserved.
	sample := "$9$abcdefg"

	p := NewParser(bcryptTestSchema())
	tree, err := p.Parse("password-hash " + sample)
	require.NoError(t, err)

	val, ok := tree.Get("password-hash")
	require.True(t, ok)
	assert.Equal(t, sample, val, "Bcrypt leaf must preserve $9$-prefixed value verbatim")
}

// TestParserBcryptLeafAcceptsHash verifies a bcrypt hash stored verbatim.
//
// VALIDATES: ze:bcrypt leaf accepts $2a$10$... canonical format as-is.
//
// PREVENTS: Bcrypt hash corruption on config load.
func TestParserBcryptLeafAcceptsHash(t *testing.T) {
	hash := "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234"

	p := NewParser(bcryptTestSchema())
	tree, err := p.Parse(`password-hash "` + hash + `"`)
	require.NoError(t, err)

	val, ok := tree.Get("password-hash")
	require.True(t, ok)
	assert.Equal(t, hash, val)
}

// TestParserSensitiveLeafStillDecodesSecret confirms the existing $9$ path
// remains intact for Sensitive (non-Bcrypt) leaves.
//
// VALIDATES: ze:sensitive semantics unchanged by ze:bcrypt addition.
//
// PREVENTS: Regression on API-token leaves that rely on $9$ reversibility.
func TestParserSensitiveLeafStillDecodesSecret(t *testing.T) {
	// Encode a known plaintext to $9$ form, then verify the parser decodes it.
	plain := "hello"
	encoded, err := secret.Encode(plain)
	require.NoError(t, err)

	p := NewParser(bcryptTestSchema())
	tree, err := p.Parse(`api-token "` + encoded + `"`)
	require.NoError(t, err)

	val, ok := tree.Get("api-token")
	require.True(t, ok)
	assert.Equal(t, plain, val, "Sensitive leaf must still decode $9$ to plaintext")
}
