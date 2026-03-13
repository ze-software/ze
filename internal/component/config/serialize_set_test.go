package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSerializeSetSimpleLeaf verifies serialization of a simple leaf value.
//
// VALIDATES: Top-level leaves are serialized as set commands.
//
// PREVENTS: Lost simple configuration values in set format output.
func TestSerializeSetSimpleLeaf(t *testing.T) {
	tree := NewTree()
	tree.Set("router-id", "1.2.3.4")
	tree.Set("local-as", "65000")

	schema := testSchema()
	output := SerializeSet(tree, schema)

	assert.Contains(t, output, "set router-id 1.2.3.4\n")
	assert.Contains(t, output, "set local-as 65000\n")
}

// TestSerializeSetNeighborLeaf verifies serialization of list entry fields.
//
// VALIDATES: List entries emit set commands with key in path.
//
// PREVENTS: Lost nested configuration in set format.
func TestSerializeSetNeighborLeaf(t *testing.T) {
	tree := NewTree()
	entry := NewTree()
	entry.Set("local-as", "65000")
	entry.Set("peer-as", "65001")
	tree.AddListEntry("neighbor", "192.0.2.1", entry)

	schema := testSchema()
	output := SerializeSet(tree, schema)

	assert.Contains(t, output, "set neighbor 192.0.2.1 local-as 65000\n")
	assert.Contains(t, output, "set neighbor 192.0.2.1 peer-as 65001\n")
}

// TestSerializeSetNestedContainer verifies serialization of nested containers.
//
// VALIDATES: Container paths are flattened into set command paths.
//
// PREVENTS: Missing intermediate container path segments.
func TestSerializeSetNestedContainer(t *testing.T) {
	tree := NewTree()
	entry := NewTree()
	family := NewTree()
	ipv4 := NewTree()
	ipv4.Set("unicast", "true")
	family.SetContainer("ipv4", ipv4)
	entry.SetContainer("family", family)
	entry.Set("local-as", "65000")
	tree.AddListEntry("neighbor", "192.0.2.1", entry)

	schema := testSchema()
	output := SerializeSet(tree, schema)

	assert.Contains(t, output, "set neighbor 192.0.2.1 local-as 65000\n")
	assert.Contains(t, output, "set neighbor 192.0.2.1 family ipv4 unicast enable\n")
}

// TestSerializeSetMultipleNeighbors verifies multiple list entries.
//
// VALIDATES: Multiple list entries are serialized correctly.
//
// PREVENTS: Overwritten or missing list entries.
func TestSerializeSetMultipleNeighbors(t *testing.T) {
	tree := NewTree()
	e1 := NewTree()
	e1.Set("local-as", "65000")
	e1.Set("peer-as", "65001")
	tree.AddListEntry("neighbor", "192.0.2.1", e1)

	e2 := NewTree()
	e2.Set("local-as", "65000")
	e2.Set("peer-as", "65002")
	tree.AddListEntry("neighbor", "192.0.2.2", e2)

	schema := testSchema()
	output := SerializeSet(tree, schema)

	assert.Contains(t, output, "set neighbor 192.0.2.1 local-as 65000\n")
	assert.Contains(t, output, "set neighbor 192.0.2.1 peer-as 65001\n")
	assert.Contains(t, output, "set neighbor 192.0.2.2 local-as 65000\n")
	assert.Contains(t, output, "set neighbor 192.0.2.2 peer-as 65002\n")
}

// TestSerializeSetSchemaOrder verifies output follows YANG schema order.
//
// VALIDATES: Fields are emitted in schema definition order, not insertion order.
//
// PREVENTS: Non-deterministic output ordering.
func TestSerializeSetSchemaOrder(t *testing.T) {
	tree := NewTree()
	// Insert in reverse order
	tree.Set("local-as", "65000")
	tree.Set("router-id", "1.2.3.4")

	schema := testSchema()
	output := SerializeSet(tree, schema)

	// router-id comes before local-as in the schema
	ridIdx := strings.Index(output, "set router-id")
	lasIdx := strings.Index(output, "set local-as")
	require.NotEqual(t, -1, ridIdx, "router-id not found")
	require.NotEqual(t, -1, lasIdx, "local-as not found")
	require.Greater(t, lasIdx, ridIdx, "router-id should come before local-as (schema order)")
}

// TestSerializeSetRoundTrip verifies parse -> serialize -> parse produces same tree.
//
// VALIDATES: Set format is lossless for config data.
//
// PREVENTS: Data loss through serialization round-trip.
func TestSerializeSetRoundTrip(t *testing.T) {
	input := `
set router-id 1.2.3.4
set local-as 65000
set neighbor 192.0.2.1 local-as 65000
set neighbor 192.0.2.1 peer-as 65001
set neighbor 192.0.2.1 family ipv4 unicast enable
`

	schema := testSchema()

	// Parse
	p := NewSetParser(schema)
	tree1, err := p.Parse(input)
	require.NoError(t, err)

	// Serialize
	output := SerializeSet(tree1, schema)

	// Re-parse
	tree2, err := p.Parse(output)
	require.NoError(t, err)

	// Compare: serialize both trees and compare output (canonical form)
	output2 := SerializeSet(tree2, schema)
	assert.Equal(t, output, output2, "round-trip should produce identical output")
}

// TestSerializeSetNestedList verifies nested list serialization.
//
// VALIDATES: Lists within containers serialize correctly.
//
// PREVENTS: Lost nested list entries in set format.
func TestSerializeSetNestedList(t *testing.T) {
	tree := NewTree()
	entry := NewTree()
	static := NewTree()
	r1 := NewTree()
	r1.Set("next-hop", "192.0.2.1")
	static.AddListEntry("route", "10.0.0.0/8", r1)
	entry.SetContainer("static", static)
	entry.Set("local-as", "65000")
	tree.AddListEntry("neighbor", "192.0.2.1", entry)

	schema := testSchema()
	output := SerializeSet(tree, schema)

	assert.Contains(t, output, "set neighbor 192.0.2.1 local-as 65000\n")
	assert.Contains(t, output, "set neighbor 192.0.2.1 static route 10.0.0.0/8 next-hop 192.0.2.1\n")
}

// TestSerializeSetEmptyTree verifies empty tree produces empty output.
//
// VALIDATES: Empty tree produces no output.
//
// PREVENTS: Spurious output from empty config.
func TestSerializeSetEmptyTree(t *testing.T) {
	tree := NewTree()
	schema := testSchema()
	output := SerializeSet(tree, schema)
	assert.Equal(t, "", output)
}

// TestSerializeSetCrossFormatRoundTrip verifies hierarchical -> set -> set produces consistent output.
//
// VALIDATES: Cross-format round-trip preserves data.
//
// PREVENTS: Data loss when migrating from hierarchical to set format.
func TestSerializeSetCrossFormatRoundTrip(t *testing.T) {
	hierarchical := `router-id 1.2.3.4
local-as 65000
neighbor 192.0.2.1 {
	local-as 65000
	peer-as 65001
	family {
		ipv4 {
			unicast enable
		}
	}
}
`

	schema := testSchema()

	// Parse hierarchical
	hp := NewParser(schema)
	tree1, err := hp.Parse(hierarchical)
	require.NoError(t, err)

	// Serialize to set format
	setOutput := SerializeSet(tree1, schema)

	// Parse set format
	sp := NewSetParser(schema)
	tree2, err := sp.Parse(setOutput)
	require.NoError(t, err)

	// Serialize again to set format
	setOutput2 := SerializeSet(tree2, schema)

	// Both set serializations should be identical
	assert.Equal(t, setOutput, setOutput2, "cross-format round-trip should be stable")

	// Verify key values survived
	rid, _ := tree2.Get("router-id")
	assert.Equal(t, "1.2.3.4", rid)

	n := tree2.GetList("neighbor")["192.0.2.1"]
	require.NotNil(t, n)

	peerAs, _ := n.Get("peer-as")
	assert.Equal(t, "65001", peerAs)
}

// TestSerializeSetQuotedValues verifies values with spaces are quoted.
//
// VALIDATES: Values containing spaces are quoted in output.
//
// PREVENTS: Broken parsing of values with spaces.
func TestSerializeSetQuotedValues(t *testing.T) {
	tree := NewTree()
	entry := NewTree()
	entry.Set("description", "My BGP Peer")
	entry.Set("local-as", "65000")
	tree.AddListEntry("neighbor", "192.0.2.1", entry)

	schema := testSchema()
	output := SerializeSet(tree, schema)

	assert.Contains(t, output, `set neighbor 192.0.2.1 description "My BGP Peer"`)
}

// TestFormatAutoDetect verifies format detection from first line.
//
// VALIDATES: First non-empty, non-comment line determines format.
//
// PREVENTS: Wrong parser used for config format.
// TestSerializeSetWithMeta verifies serialization with metadata prefixes.
//
// VALIDATES: Tree + MetaTree serialized with user/time/session prefixes.
//
// PREVENTS: Lost metadata in draft file serialization.
func TestSerializeSetWithMeta(t *testing.T) {
	tree := NewTree()
	tree.Set("router-id", "1.2.3.4")
	tree.Set("local-as", "65000")

	meta := NewMetaTree()
	meta.SetEntry("router-id", MetaEntry{
		User:    "thomas@local",
		Time:    time.Date(2026, 3, 12, 14, 30, 1, 0, time.UTC),
		Session: "thomas@local:1741783801",
	})
	meta.SetEntry("local-as", MetaEntry{
		User:    "alice@ssh",
		Time:    time.Date(2026, 3, 12, 14, 31, 0, 0, time.UTC),
		Session: "alice@ssh:1741783860",
	})

	schema := testSchema()
	output := SerializeSetWithMeta(tree, meta, schema)

	assert.Contains(t, output, "#thomas@local @2026-03-12T14:30:01Z %thomas@local:1741783801 set router-id 1.2.3.4\n")
	assert.Contains(t, output, "#alice@ssh @2026-03-12T14:31:00Z %alice@ssh:1741783860 set local-as 65000\n")
}

// TestSerializeSetWithMetaNested verifies metadata serialization for nested paths.
//
// VALIDATES: Metadata for list entry children is emitted correctly.
//
// PREVENTS: Lost metadata in nested config structures.
func TestSerializeSetWithMetaNested(t *testing.T) {
	tree := NewTree()
	entry := NewTree()
	entry.Set("local-as", "65000")
	tree.AddListEntry("neighbor", "192.0.2.1", entry)

	meta := NewMetaTree()
	neighbor := meta.GetOrCreateContainer("neighbor")
	peer := neighbor.GetOrCreateListEntry("192.0.2.1")
	peer.SetEntry("local-as", MetaEntry{
		User:    "thomas@local",
		Time:    time.Date(2026, 3, 12, 14, 30, 1, 0, time.UTC),
		Session: "thomas@local:1741783801",
	})

	schema := testSchema()
	output := SerializeSetWithMeta(tree, meta, schema)

	assert.Contains(t, output, "#thomas@local @2026-03-12T14:30:01Z %thomas@local:1741783801 set neighbor 192.0.2.1 local-as 65000\n")
}

// TestSerializeSetWithMetaMixed verifies lines without metadata emit bare set commands.
//
// VALIDATES: Missing metadata produces lines without prefixes.
//
// PREVENTS: Spurious empty metadata prefixes.
func TestSerializeSetWithMetaMixed(t *testing.T) {
	tree := NewTree()
	tree.Set("router-id", "1.2.3.4")
	tree.Set("local-as", "65000")

	meta := NewMetaTree()
	// Only router-id has metadata
	meta.SetEntry("router-id", MetaEntry{
		User:    "thomas@local",
		Time:    time.Date(2026, 3, 12, 14, 30, 1, 0, time.UTC),
		Session: "thomas@local:1741783801",
	})

	schema := testSchema()
	output := SerializeSetWithMeta(tree, meta, schema)

	assert.Contains(t, output, "#thomas@local @2026-03-12T14:30:01Z %thomas@local:1741783801 set router-id 1.2.3.4\n")
	// local-as should be a bare set line without metadata prefix
	assert.Contains(t, output, "\nset local-as 65000\n")
}

// TestSerializeBlame verifies blame view with fixed-width gutter.
//
// VALIDATES: Blame view emits user, date, time, marker, and tree content.
//
// PREVENTS: Misaligned blame gutter columns.
func TestSerializeBlame(t *testing.T) {
	tree := NewTree()
	tree.Set("router-id", "1.2.3.4")
	tree.Set("local-as", "65000")

	meta := NewMetaTree()
	meta.SetEntry("router-id", MetaEntry{
		User: "thomas@local",
		Time: time.Date(2026, 3, 12, 14, 30, 1, 0, time.UTC),
	})

	schema := testSchema()
	output := SerializeBlame(tree, meta, schema)

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	require.Len(t, lines, 2, "expect one line per leaf")

	// Line with metadata: fixed-width gutter (29 chars) + content
	assert.Equal(t, blameGutterWidth, 29, "gutter width constant")
	routerLine := lines[0]
	assert.Equal(t, "thomas@local  03-12 14:30  + router-id 1.2.3.4", routerLine)
	// Gutter portion is exactly blameGutterWidth characters
	assert.Equal(t, blameGutterWidth, len(routerLine)-len("router-id 1.2.3.4"))

	// Line without metadata: empty gutter (29 spaces) + content
	localAsLine := lines[1]
	assert.Equal(t, strings.Repeat(" ", blameGutterWidth)+"local-as 65000", localAsLine)
}

// TestSerializeBlameMarkers verifies '+' for new and '*' for modified entries.
//
// VALIDATES: Blame marker distinguishes new vs modified leaves.
//
// PREVENTS: Wrong marker character for modified entries.
func TestSerializeBlameMarkers(t *testing.T) {
	tree := NewTree()
	tree.Set("router-id", "1.2.3.4")
	tree.Set("local-as", "65000")

	meta := NewMetaTree()
	// New entry (no previous)
	meta.SetEntry("router-id", MetaEntry{
		User: "alice",
		Time: time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC),
	})
	// Modified entry (has previous)
	meta.SetEntry("local-as", MetaEntry{
		User:     "bob",
		Time:     time.Date(2026, 3, 12, 11, 0, 0, 0, time.UTC),
		Previous: "64512",
	})

	output := SerializeBlame(tree, meta, testSchema())
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	require.Len(t, lines, 2)

	// New entry has '+' marker
	assert.Contains(t, lines[0], "  + router-id")
	// Modified entry has '*' marker
	assert.Contains(t, lines[1], "  * local-as")
}

// TestSerializeBlameBraceInheritance verifies brace lines inherit gutter from first/last child.
//
// VALIDATES: Opening brace inherits first child metadata, closing brace inherits last child.
//
// PREVENTS: Brace lines showing empty gutters when children have metadata.
func TestSerializeBlameBraceInheritance(t *testing.T) {
	tree := NewTree()
	neighbor := NewTree()
	neighbor.Set("description", "test peer")
	neighbor.Set("peer-as", "65001")
	tree.AddListEntry("neighbor", "192.0.2.1", neighbor)

	meta := NewMetaTree()
	neighborMeta := meta.GetOrCreateContainer("neighbor")
	entryMeta := neighborMeta.GetOrCreateListEntry("192.0.2.1")
	entryMeta.SetEntry("description", MetaEntry{
		User: "alice",
		Time: time.Date(2026, 1, 10, 8, 0, 0, 0, time.UTC),
	})
	entryMeta.SetEntry("peer-as", MetaEntry{
		User: "bob",
		Time: time.Date(2026, 2, 20, 16, 30, 0, 0, time.UTC),
	})

	output := SerializeBlame(tree, meta, testSchema())
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	// Find brace lines
	var openBrace, closeBrace string
	for _, line := range lines {
		trimmed := strings.TrimRight(line[blameGutterWidth:], " \t")
		if strings.HasSuffix(trimmed, "{") {
			openBrace = line
		}
		if trimmed == "}" {
			closeBrace = line
		}
	}
	require.NotEmpty(t, openBrace, "should have opening brace line")
	require.NotEmpty(t, closeBrace, "should have closing brace line")

	// Opening brace inherits first child (alice, sorted: description < peer-as)
	assert.Contains(t, openBrace, "alice")
	assert.Contains(t, openBrace, "01-10")
	// Closing brace inherits last child (bob, sorted: peer-as after description)
	assert.Contains(t, closeBrace, "bob")
	assert.Contains(t, closeBrace, "02-20")
}

func TestFormatAutoDetect(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect ConfigFormat
	}{
		{
			name:   "set command",
			input:  "set router-id 1.2.3.4\nset local-as 65000\n",
			expect: FormatSet,
		},
		{
			name:   "delete command",
			input:  "delete neighbor 192.0.2.1\n",
			expect: FormatSet,
		},
		{
			name:   "set with leading comments",
			input:  "# comment\n\nset router-id 1.2.3.4\n",
			expect: FormatSet,
		},
		{
			name:   "hierarchical",
			input:  "router-id 1.2.3.4\n",
			expect: FormatHierarchical,
		},
		{
			name:   "hierarchical with comment",
			input:  "# comment\nrouter-id 1.2.3.4\n",
			expect: FormatHierarchical,
		},
		{
			name:   "metadata prefix",
			input:  "#thomas@local @2026-03-12T14:30:01 set router-id 1.2.3.4\n",
			expect: FormatSetMeta,
		},
		{
			name:   "empty",
			input:  "",
			expect: FormatSet,
		},
		{
			name:   "only comments",
			input:  "# just a comment\n# another\n",
			expect: FormatSet,
		},
		{
			name:   "only blank lines",
			input:  "\n\n\n",
			expect: FormatSet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFormat(tt.input)
			assert.Equal(t, tt.expect, got, "format detection mismatch")
		})
	}
}

// TestSerializeBlameNilMeta verifies blame view handles nil MetaTree.
//
// VALIDATES: SerializeBlame with nil meta produces output with empty gutters.
//
// PREVENTS: Nil pointer dereference when no metadata exists.
func TestSerializeBlameNilMeta(t *testing.T) {
	tree := NewTree()
	tree.Set("router-id", "1.2.3.4")
	tree.Set("local-as", "65000")

	output := SerializeBlame(tree, nil, testSchema())
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	require.Len(t, lines, 2)

	// Both lines should have empty gutters (29 spaces)
	emptyGutter := strings.Repeat(" ", blameGutterWidth)
	assert.True(t, strings.HasPrefix(lines[0], emptyGutter), "line 0 should have empty gutter")
	assert.True(t, strings.HasPrefix(lines[1], emptyGutter), "line 1 should have empty gutter")

	// Content should still be present
	assert.Contains(t, lines[0], "router-id 1.2.3.4")
	assert.Contains(t, lines[1], "local-as 65000")
}

// TestSerializeBlameEmptyTree verifies blame view with empty tree.
//
// VALIDATES: Empty tree produces empty blame output.
//
// PREVENTS: Spurious output from empty config.
func TestSerializeBlameEmptyTree(t *testing.T) {
	meta := NewMetaTree()
	meta.SetEntry("router-id", MetaEntry{User: "alice"})

	output := SerializeBlame(NewTree(), meta, testSchema())
	assert.Equal(t, "", output, "empty tree should produce empty blame output")
}

// TestSerializeBlameNestedContainer verifies blame with nested containers.
//
// VALIDATES: Metadata at different nesting depths renders correctly.
//
// PREVENTS: Lost or misplaced gutters in deeply nested structures.
func TestSerializeBlameNestedContainer(t *testing.T) {
	tree := NewTree()
	entry := NewTree()
	family := NewTree()
	ipv4 := NewTree()
	ipv4.Set("unicast", "true")
	family.SetContainer("ipv4", ipv4)
	entry.SetContainer("family", family)
	entry.Set("peer-as", "65001")
	tree.AddListEntry("neighbor", "192.0.2.1", entry)

	meta := NewMetaTree()
	neighborMeta := meta.GetOrCreateContainer("neighbor")
	peerMeta := neighborMeta.GetOrCreateListEntry("192.0.2.1")
	familyMeta := peerMeta.GetOrCreateContainer("family")
	ipv4Meta := familyMeta.GetOrCreateContainer("ipv4")
	ipv4Meta.SetEntry("unicast", MetaEntry{
		User: "alice",
		Time: time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC),
	})

	output := SerializeBlame(tree, meta, testSchema())

	// The unicast leaf should have alice's gutter
	assert.Contains(t, output, "alice")
	assert.Contains(t, output, "unicast enable")

	// peer-as without metadata should have empty gutter
	emptyGutter := strings.Repeat(" ", blameGutterWidth)
	assert.Contains(t, output, emptyGutter+"\tpeer-as 65001\n",
		"peer-as without metadata should have empty gutter")
}

// TestSerializeBlameSingleChildBrace verifies brace gutter with only one child.
//
// VALIDATES: When a container has one annotated child, both open and close braces
// inherit from that same entry (first == last).
//
// PREVENTS: Brace gutter mismatch when container has single child.
func TestSerializeBlameSingleChildBrace(t *testing.T) {
	tree := NewTree()
	entry := NewTree()
	entry.Set("peer-as", "65001")
	tree.AddListEntry("neighbor", "192.0.2.1", entry)

	meta := NewMetaTree()
	neighborMeta := meta.GetOrCreateContainer("neighbor")
	peerMeta := neighborMeta.GetOrCreateListEntry("192.0.2.1")
	peerMeta.SetEntry("peer-as", MetaEntry{
		User: "alice",
		Time: time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC),
	})

	output := SerializeBlame(tree, meta, testSchema())
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	// Find open and close brace lines
	var braceLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line[blameGutterWidth:])
		if strings.HasSuffix(trimmed, "{") || trimmed == "}" {
			braceLines = append(braceLines, line)
		}
	}
	require.GreaterOrEqual(t, len(braceLines), 2, "should have open and close brace lines")

	// Both open and close braces should inherit from the single child (alice)
	for _, bl := range braceLines {
		assert.Contains(t, bl, "alice",
			"brace line should inherit from single child metadata")
	}
}

// TestSerializeSetWithMetaContested verifies serialization of contested leaf
// (multiple sessions editing same leaf with different values).
//
// VALIDATES: Contested leaf emits one line per session entry with its own value.
//
// PREVENTS: Lost session-specific values in draft serialization.
func TestSerializeSetWithMetaContested(t *testing.T) {
	tree := NewTree()
	tree.Set("router-id", "1.2.3.4") // Last-writer-wins in tree

	meta := NewMetaTree()
	// Session A set the value first
	meta.SetEntry("router-id", MetaEntry{
		User:    "alice",
		Session: "alice:100",
		Value:   "10.0.0.1",
	})
	// Session B set a different value (overwrites in tree, but meta keeps both)
	meta.SetEntry("router-id", MetaEntry{
		User:    "bob",
		Session: "bob:200",
		Value:   "1.2.3.4",
	})

	schema := testSchema()
	output := SerializeSetWithMeta(tree, meta, schema)

	// Should have two lines for router-id, one per session
	assert.Contains(t, output, "#alice %alice:100 set router-id 10.0.0.1\n")
	assert.Contains(t, output, "#bob %bob:200 set router-id 1.2.3.4\n")
}

// TestSerializeSetWithMetaContestedDelete verifies contested leaf where one
// session deletes and another sets.
//
// VALIDATES: Delete intent (Value="" with Session) emits "delete" command.
//
// PREVENTS: Delete intent serialized as "set" with empty value.
func TestSerializeSetWithMetaContestedDelete(t *testing.T) {
	tree := NewTree()
	tree.Set("router-id", "1.2.3.4")

	meta := NewMetaTree()
	// Session A sets the value
	meta.SetEntry("router-id", MetaEntry{
		User:    "alice",
		Session: "alice:100",
		Value:   "1.2.3.4",
	})
	// Session B deletes the value (Value="" with Session)
	meta.SetEntry("router-id", MetaEntry{
		User:    "bob",
		Session: "bob:200",
		Value:   "",
	})

	schema := testSchema()
	output := SerializeSetWithMeta(tree, meta, schema)

	assert.Contains(t, output, "#alice %alice:100 set router-id 1.2.3.4\n")
	assert.Contains(t, output, "#bob %bob:200 delete router-id\n")
}

// TestSerializeSetWithMetaOrphanDelete verifies orphan metadata (meta entry
// without corresponding tree value) round-trips correctly.
//
// VALIDATES: writeDeleteMetaLines emits delete lines for orphan metadata.
//
// PREVENTS: Lost metadata when session deletes a leaf.
func TestSerializeSetWithMetaOrphanDelete(t *testing.T) {
	tree := NewTree()
	tree.Set("local-as", "65000")
	// router-id is NOT in the tree (deleted), but metadata exists

	meta := NewMetaTree()
	meta.SetEntry("local-as", MetaEntry{
		User:    "alice",
		Session: "alice:100",
		Value:   "65000",
	})
	// Orphan: session deleted router-id, metadata remains
	meta.SetEntry("router-id", MetaEntry{
		User:    "bob",
		Session: "bob:200",
		Value:   "",
	})

	schema := testSchema()
	output := SerializeSetWithMeta(tree, meta, schema)

	// local-as should be a normal set line
	assert.Contains(t, output, "set local-as 65000")
	// router-id should appear as a delete line from orphan metadata
	assert.Contains(t, output, "#bob %bob:200 delete router-id\n")
}

// TestSerializeSetWithMetaOrphanRoundTrip verifies orphan delete metadata
// survives a serialize -> parse -> serialize cycle.
//
// VALIDATES: Delete metadata is preserved through round-trip.
//
// PREVENTS: Silent loss of pending delete intent on draft re-read.
func TestSerializeSetWithMetaOrphanRoundTrip(t *testing.T) {
	tree := NewTree()
	tree.Set("local-as", "65000")

	meta := NewMetaTree()
	meta.SetEntry("local-as", MetaEntry{
		User:    "alice",
		Session: "alice:100",
		Value:   "65000",
	})
	meta.SetEntry("router-id", MetaEntry{
		User:    "bob",
		Session: "bob:200",
		Value:   "",
	})

	schema := testSchema()

	// First serialize
	output1 := SerializeSetWithMeta(tree, meta, schema)

	// Parse back
	p := NewSetParser(schema)
	tree2, meta2, err := p.ParseWithMeta(output1)
	require.NoError(t, err)

	// Re-serialize
	output2 := SerializeSetWithMeta(tree2, meta2, schema)

	assert.Equal(t, output1, output2, "orphan delete metadata should survive round-trip")
}

// TestSerializeSetWithMetaContestedWithCommitted verifies contested leaf
// where one entry is sessionless (committed) and another has a session.
//
// VALIDATES: Committed entry (Session="", Value="") uses tree value.
//
// PREVENTS: Committed entry rendering as empty value or delete.
func TestSerializeSetWithMetaContestedWithCommitted(t *testing.T) {
	tree := NewTree()
	tree.Set("router-id", "1.2.3.4")

	meta := NewMetaTree()
	// Committed entry (no session, no value -- tree value is the committed value)
	meta.SetEntry("router-id", MetaEntry{
		User: "alice",
		Time: time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC),
	})
	// Session entry with different value
	meta.SetEntry("router-id", MetaEntry{
		User:    "bob",
		Session: "bob:200",
		Value:   "10.0.0.1",
	})

	schema := testSchema()
	output := SerializeSetWithMeta(tree, meta, schema)

	// Committed entry should use tree value "1.2.3.4"
	assert.Contains(t, output, "#alice @2026-03-12T10:00:00Z set router-id 1.2.3.4\n")
	// Session entry should use its own value
	assert.Contains(t, output, "#bob %bob:200 set router-id 10.0.0.1\n")
}

// TestFormatAutoDetectMetaPrefixes verifies format detection for all metadata prefix types.
//
// VALIDATES: @timestamp, %session, and ^previous prefixes all detect FormatSetMeta.
//
// PREVENTS: Wrong format detection for files starting with metadata.
func TestFormatAutoDetectMetaPrefixes(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "timestamp prefix", input: "@2026-03-12T14:30:01Z set router-id 1.2.3.4\n"},
		{name: "session prefix", input: "%thomas@local:123 set router-id 1.2.3.4\n"},
		{name: "previous prefix", input: "^oldvalue set router-id 1.2.3.4\n"},
		{name: "user hash no space", input: "#thomas set router-id 1.2.3.4\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFormat(tt.input)
			assert.Equal(t, FormatSetMeta, got, "should detect metadata format")
		})
	}
}

// TestBlameGutterWidth verifies the fixed-width blame gutter produces exactly 29 characters.
//
// VALIDATES: Blame gutter is exactly blameGutterWidth (29) chars for alignment.
// PREVENTS: Misaligned blame view when username is short, long, or exactly 14 chars.
func TestBlameGutterWidth(t *testing.T) {
	tests := []struct {
		name string
		user string
	}{
		{name: "short user", user: "tom"},
		{name: "exact 14 chars", user: "12345678901234"},
		{name: "longer than 14", user: "longusernameoverflow"},
		{name: "single char", user: "a"},
		{name: "empty user", user: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := MetaEntry{
				User:    tt.user,
				Time:    time.Date(2026, 3, 12, 14, 30, 0, 0, time.UTC),
				Session: tt.user + "@local:12345",
			}

			var b strings.Builder
			writeMetaEntryGutter(&b, entry)
			got := b.String()
			assert.Equal(t, blameGutterWidth, len(got),
				"gutter for user %q should be %d chars, got %d: %q",
				tt.user, blameGutterWidth, len(got), got)
		})
	}
}

// TestBlameEmptyGutterWidth verifies empty gutter is the same width as populated gutter.
//
// VALIDATES: Empty gutter produces blameGutterWidth (29) spaces for alignment.
// PREVENTS: Jagged alignment between lines with and without blame metadata.
func TestBlameEmptyGutterWidth(t *testing.T) {
	var b strings.Builder
	writeEmptyGutter(&b)
	got := b.String()
	assert.Equal(t, blameGutterWidth, len(got),
		"empty gutter should be %d chars, got %d", blameGutterWidth, len(got))
	assert.Equal(t, strings.Repeat(" ", blameGutterWidth), got, "empty gutter should be all spaces")
}

// TestWriteDeleteMetaLinesSessionSetTreeDeleted verifies writeDeleteMetaLines
// when a session set a value but another session deleted it from the tree.
//
// VALIDATES: Orphan meta with Value!="" emits "set" line (not "delete").
//
// PREVENTS: Session's set intent lost when another session deletes the tree value.
func TestWriteDeleteMetaLinesSessionSetTreeDeleted(t *testing.T) {
	tree := NewTree()
	// router-id is NOT in the tree (another session deleted it)

	meta := NewMetaTree()
	// alice set router-id to 10.0.0.1, but the tree value was removed by bob
	meta.SetEntry("router-id", MetaEntry{
		User:    "alice",
		Session: "alice:100",
		Value:   "10.0.0.1",
	})

	schema := testSchema()
	output := SerializeSetWithMeta(tree, meta, schema)

	// Should emit a "set" line with alice's value (not "delete")
	assert.Contains(t, output, "#alice %alice:100 set router-id 10.0.0.1\n",
		"orphan meta with Value should emit set line preserving session's intent")
	assert.NotContains(t, output, "delete router-id",
		"should not emit delete when session's intent was to set")
}

// TestWriteDeleteMetaLinesMixedOrphans verifies writeDeleteMetaLines when
// both a set-intent and delete-intent orphan exist for the same leaf.
//
// VALIDATES: Each orphan entry emits according to its own Value field.
//
// PREVENTS: Mixed orphan entries collapsing into a single line.
func TestWriteDeleteMetaLinesMixedOrphans(t *testing.T) {
	tree := NewTree()
	// router-id has no tree value

	meta := NewMetaTree()
	// alice wants to set, bob wants to delete
	meta.SetEntry("router-id", MetaEntry{
		User:    "alice",
		Session: "alice:100",
		Value:   "10.0.0.1",
	})
	meta.SetEntry("router-id", MetaEntry{
		User:    "bob",
		Session: "bob:200",
		Value:   "",
	})

	schema := testSchema()
	output := SerializeSetWithMeta(tree, meta, schema)

	assert.Contains(t, output, "#alice %alice:100 set router-id 10.0.0.1\n")
	assert.Contains(t, output, "#bob %bob:200 delete router-id\n")
}

// TestSerializeSetWithMetaPresenceContainer verifies metadata on presence
// container flag values.
//
// VALIDATES: Presence container flag (value=true) gets metadata prefix.
//
// PREVENTS: Missing metadata on presence container entries.
func TestSerializeSetWithMetaPresenceContainer(t *testing.T) {
	// Create a schema with a presence container.
	schema := NewSchema()
	schema.Define("router-id", Leaf(TypeIPv4))
	passiveNode := Container()
	passiveNode.Presence = true
	schema.Define("passive", passiveNode)

	tree := NewTree()
	tree.Set("router-id", "1.2.3.4")
	tree.Set("passive", configTrue)

	meta := NewMetaTree()
	meta.SetEntry("router-id", MetaEntry{User: "alice", Session: "alice:100", Value: "1.2.3.4"})
	meta.SetEntry("passive", MetaEntry{User: "alice", Session: "alice:100"})

	output := SerializeSetWithMeta(tree, meta, schema)
	assert.Contains(t, output, "#alice %alice:100 set router-id 1.2.3.4\n")
	assert.Contains(t, output, "#alice %alice:100 set passive\n")
}
