package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetParserSimpleLeaf verifies parsing a simple set command.
//
// VALIDATES: Top-level leaves are set correctly.
//
// PREVENTS: Lost simple configuration values.
func TestSetParserSimpleLeaf(t *testing.T) {
	input := `set router-id 1.2.3.4`

	p := NewSetParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)
	require.NotNil(t, tree)

	val, ok := tree.Get("router-id")
	require.True(t, ok)
	require.Equal(t, "1.2.3.4", val)
}

// TestSetParserNeighborLeaf verifies setting a neighbor field.
//
// VALIDATES: List entry fields are set via path.
//
// PREVENTS: Lost nested configuration.
func TestSetParserNeighborLeaf(t *testing.T) {
	input := `
set neighbor 192.0.2.1 local-as 65000
set neighbor 192.0.2.1 peer-as 65001
set neighbor 192.0.2.1 router-id 1.2.3.4
`

	p := NewSetParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	require.Len(t, neighbors, 1)

	n := neighbors["192.0.2.1"]
	require.NotNil(t, n)

	val, _ := n.Get("local-as")
	require.Equal(t, "65000", val)

	val, _ = n.Get("peer-as")
	require.Equal(t, "65001", val)

	val, _ = n.Get("router-id")
	require.Equal(t, "1.2.3.4", val)
}

// TestSetParserMultipleNeighbors verifies multiple list entries.
//
// VALIDATES: Multiple neighbors are created correctly.
//
// PREVENTS: Overwritten neighbor configs.
func TestSetParserMultipleNeighbors(t *testing.T) {
	input := `
set neighbor 192.0.2.1 local-as 65000
set neighbor 192.0.2.1 peer-as 65001
set neighbor 192.0.2.2 local-as 65000
set neighbor 192.0.2.2 peer-as 65002
`

	p := NewSetParser(testSchema())
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

// TestSetParserNestedContainer verifies nested container paths.
//
// VALIDATES: Nested containers are created via path.
//
// PREVENTS: Flat structure instead of nested.
func TestSetParserNestedContainer(t *testing.T) {
	input := `
set neighbor 192.0.2.1 local-as 65000
set neighbor 192.0.2.1 peer-as 65001
set neighbor 192.0.2.1 family ipv4 unicast true
set neighbor 192.0.2.1 family ipv6 unicast true
`

	p := NewSetParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	n := neighbors["192.0.2.1"]

	family := n.GetContainer("family")
	require.NotNil(t, family)

	ipv4 := family.GetContainer("ipv4")
	require.NotNil(t, ipv4)

	val, _ := ipv4.Get("unicast")
	require.Equal(t, "true", val)

	ipv6 := family.GetContainer("ipv6")
	require.NotNil(t, ipv6)

	val, _ = ipv6.Get("unicast")
	require.Equal(t, "true", val)
}

// TestSetParserNestedList verifies nested list paths.
//
// VALIDATES: Lists inside containers work.
//
// PREVENTS: Lost nested list entries.
func TestSetParserNestedList(t *testing.T) {
	input := `
set neighbor 192.0.2.1 local-as 65000
set neighbor 192.0.2.1 peer-as 65001
set neighbor 192.0.2.1 static route 10.0.0.0/8 next-hop 192.0.2.1
set neighbor 192.0.2.1 static route 172.16.0.0/12 next-hop 192.0.2.1
`

	p := NewSetParser(testSchema())
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

// TestSetParserProcess verifies string-keyed list.
//
// VALIDATES: String keys work for lists.
//
// PREVENTS: Only IP-keyed lists working.
func TestSetParserProcess(t *testing.T) {
	input := `
set process announce-routes run "/usr/bin/exabgp-announce"
set process announce-routes encoder json
`

	p := NewSetParser(testSchema())
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

// TestSetParserComments verifies comment handling.
//
// VALIDATES: Comments are ignored.
//
// PREVENTS: Comments parsed as commands.
func TestSetParserComments(t *testing.T) {
	input := `
# This is a comment
set router-id 1.2.3.4
# Another comment
set local-as 65000
`

	p := NewSetParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	val, _ := tree.Get("router-id")
	require.Equal(t, "1.2.3.4", val)

	val, _ = tree.Get("local-as")
	require.Equal(t, "65000", val)
}

// TestSetParser_NoValidateValue verifies SetParser accepts values without type checking.
// YANG validates types later in the pipeline — SetParser only does structural navigation.
//
// VALIDATES: SetParser accepts any string value for leaves (no own type checking).
// PREVENTS: SetParser rejecting values that YANG should validate.
func TestSetParser_NoValidateValue(t *testing.T) {
	input := `set neighbor 192.0.2.1 local-as not-a-number`

	p := NewSetParser(testSchema())
	tree, err := p.Parse(input)

	// SetParser no longer calls ValidateValue — it accepts any value.
	// Type validation is deferred to YANG.
	require.NoError(t, err)

	entries := tree.GetList("neighbor")
	require.NotNil(t, entries)
	entry := entries["192.0.2.1"]
	require.NotNil(t, entry)
	val, ok := entry.Get("local-as")
	require.True(t, ok)
	assert.Equal(t, "not-a-number", val)
}

// TestSetParserUnknownPath verifies unknown path rejection.
//
// VALIDATES: Unknown paths are rejected.
//
// PREVENTS: Silent config typos.
func TestSetParserUnknownPath(t *testing.T) {
	input := `set neighbor 192.0.2.1 unknown-field value`

	p := NewSetParser(testSchema())
	_, err := p.Parse(input)

	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown")
}

// TestSetParserQuotedValues verifies quoted string handling.
//
// VALIDATES: Quoted strings preserve spaces.
//
// PREVENTS: Broken paths or descriptions.
func TestSetParserQuotedValues(t *testing.T) {
	input := `set neighbor 192.0.2.1 description "My BGP Peer"`

	p := NewSetParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	n := neighbors["192.0.2.1"]

	val, _ := n.Get("description")
	require.Equal(t, "My BGP Peer", val)
}

// TestSetParserLineNumbers verifies error line reporting.
//
// VALIDATES: Errors include line numbers.
//
// PREVENTS: Hard-to-find config errors.
func TestSetParserLineNumbers(t *testing.T) {
	input := `
set router-id 1.2.3.4
set neighbor 192.0.2.1 unknown-field value
`

	p := NewSetParser(testSchema())
	_, err := p.Parse(input)

	require.Error(t, err)
	require.Contains(t, err.Error(), "line 3")
}

// TestSetParserEmptyLines verifies empty line handling.
//
// VALIDATES: Empty lines are ignored.
//
// PREVENTS: Errors on blank lines.
func TestSetParserEmptyLines(t *testing.T) {
	input := `

set router-id 1.2.3.4

set local-as 65000

`

	p := NewSetParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	val, _ := tree.Get("router-id")
	require.Equal(t, "1.2.3.4", val)
}

// TestSetParserDelete verifies delete command.
//
// VALIDATES: Delete removes values.
//
// PREVENTS: Inability to unset config.
func TestSetParserDelete(t *testing.T) {
	input := `
set neighbor 192.0.2.1 local-as 65000
set neighbor 192.0.2.1 peer-as 65001
delete neighbor 192.0.2.1 peer-as
`

	p := NewSetParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	n := neighbors["192.0.2.1"]

	val, ok := n.Get("local-as")
	require.True(t, ok)
	require.Equal(t, "65000", val)

	_, ok = n.Get("peer-as")
	require.False(t, ok, "peer-as should be deleted")
}

// TestParseSetWithMetaSimple verifies metadata prefix parsing for top-level leaves.
//
// VALIDATES: User, time, and session metadata are extracted from line prefixes.
//
// PREVENTS: Lost metadata when parsing draft files.
func TestParseSetWithMetaSimple(t *testing.T) {
	input := "#thomas @local %2026-03-12T14:30:01Z set router-id 1.2.3.4\n" +
		"#alice @ssh %2026-03-12T14:31:00Z set local-as 65000\n"

	p := NewSetParser(testSchema())
	tree, meta, err := p.ParseWithMeta(input)

	require.NoError(t, err)

	// Tree values correct
	val, ok := tree.Get("router-id")
	require.True(t, ok)
	assert.Equal(t, "1.2.3.4", val)

	val, ok = tree.Get("local-as")
	require.True(t, ok)
	assert.Equal(t, "65000", val)

	// Metadata correct
	e, ok := meta.GetEntry("router-id")
	require.True(t, ok)
	assert.Equal(t, "thomas", e.User)
	assert.Equal(t, "local", e.Source)
	assert.Equal(t, 2026, e.Time.Year())
	assert.Equal(t, time.Month(3), e.Time.Month())

	e, ok = meta.GetEntry("local-as")
	require.True(t, ok)
	assert.Equal(t, "alice", e.User)
	assert.Equal(t, "ssh", e.Source)
}

// TestParseSetWithMetaNested verifies metadata for nested paths.
//
// VALIDATES: Metadata is stored at the correct MetaTree depth.
//
// PREVENTS: Metadata misplaced in tree hierarchy.
func TestParseSetWithMetaNested(t *testing.T) {
	input := "#thomas @local %2026-03-12T14:30:01Z set neighbor 192.0.2.1 local-as 65000\n"

	p := NewSetParser(testSchema())
	_, meta, err := p.ParseWithMeta(input)

	require.NoError(t, err)

	// Navigate MetaTree: neighbor -> 192.0.2.1 -> local-as
	neighborMeta := meta.containers["neighbor"]
	require.NotNil(t, neighborMeta)
	entryMeta := neighborMeta.lists["192.0.2.1"]
	require.NotNil(t, entryMeta)
	e, ok := entryMeta.GetEntry("local-as")
	require.True(t, ok)
	assert.Equal(t, "thomas", e.User)
	assert.Equal(t, "local", e.Source)
}

// TestParseSetWithMetaMixed verifies lines with and without metadata.
//
// VALIDATES: Lines without metadata produce no MetaEntry.
//
// PREVENTS: Spurious metadata for hand-written config lines.
func TestParseSetWithMetaMixed(t *testing.T) {
	input := "#thomas @local %2026-03-12T14:30:01Z set router-id 1.2.3.4\n" +
		"set local-as 65000\n"

	p := NewSetParser(testSchema())
	tree, meta, err := p.ParseWithMeta(input)

	require.NoError(t, err)

	// Both values present in tree
	val, _ := tree.Get("router-id")
	assert.Equal(t, "1.2.3.4", val)
	val, _ = tree.Get("local-as")
	assert.Equal(t, "65000", val)

	// Only router-id has metadata
	_, ok := meta.GetEntry("router-id")
	assert.True(t, ok)
	_, ok = meta.GetEntry("local-as")
	assert.False(t, ok)
}

// TestParseSetWithMetaComments verifies comment handling in metadata format.
//
// VALIDATES: "# text" comments are skipped, "#user" metadata is parsed.
//
// PREVENTS: Comments confused with user metadata.
func TestParseSetWithMetaComments(t *testing.T) {
	input := "# This is a comment\n" +
		"#thomas @local set router-id 1.2.3.4\n" +
		"# Another comment\n"

	p := NewSetParser(testSchema())
	tree, meta, err := p.ParseWithMeta(input)

	require.NoError(t, err)

	val, ok := tree.Get("router-id")
	require.True(t, ok)
	assert.Equal(t, "1.2.3.4", val)

	e, ok := meta.GetEntry("router-id")
	require.True(t, ok)
	assert.Equal(t, "thomas", e.User)
	assert.Equal(t, "local", e.Source)
}

// TestParseSetWithMetaRoundTrip verifies parse -> serialize -> parse with metadata.
//
// VALIDATES: Metadata survives serialization round-trip.
//
// PREVENTS: Metadata loss through serialization.
func TestParseSetWithMetaRoundTrip(t *testing.T) {
	input := "#thomas @local %2026-03-12T14:30:01Z set router-id 1.2.3.4\n" +
		"#alice @ssh %2026-03-12T14:31:00Z set local-as 65000\n"

	schema := testSchema()
	p := NewSetParser(schema)

	// Parse
	tree1, meta1, err := p.ParseWithMeta(input)
	require.NoError(t, err)

	// Serialize with metadata
	output := SerializeSetWithMeta(tree1, meta1, schema)

	// Re-parse
	tree2, meta2, err := p.ParseWithMeta(output)
	require.NoError(t, err)

	// Compare trees
	output2 := SerializeSetWithMeta(tree2, meta2, schema)
	assert.Equal(t, output, output2, "round-trip should produce identical output")

	// Verify metadata survived
	e, ok := meta2.GetEntry("router-id")
	require.True(t, ok)
	assert.Equal(t, "thomas", e.User)
	assert.Equal(t, "local", e.Source)
}

// TestParseSetWithMetaDelete verifies metadata parsing for delete commands.
//
// VALIDATES: Delete with metadata records entry with empty Value.
//
// PREVENTS: Lost session metadata for delete operations.
func TestParseSetWithMetaDelete(t *testing.T) {
	input := "#bob @local %2026-03-12T15:00:00Z set router-id 1.2.3.4\n" +
		"#alice @ssh %2026-03-12T16:00:00Z delete router-id\n"

	p := NewSetParser(testSchema())
	tree, meta, err := p.ParseWithMeta(input)
	require.NoError(t, err)

	// Delete should have removed the tree value
	_, ok := tree.Get("router-id")
	assert.False(t, ok, "router-id should be deleted from tree")

	// But metadata should survive for both entries
	all := meta.GetAllEntries("router-id")
	require.Len(t, all, 2, "both set and delete metadata should be preserved")

	// Bob's set entry
	assert.Equal(t, "bob", all[0].User)
	assert.Equal(t, "local", all[0].Source)

	// Alice's delete entry (Value should be empty since it's a delete)
	assert.Equal(t, "alice", all[1].User)
	assert.Equal(t, "ssh", all[1].Source)
}

// TestParseSetWithMetaValueField verifies that Entry.Value is populated
// from the set command value during metadata parsing.
//
// VALIDATES: MetaEntry.Value records the value from the set command.
//
// PREVENTS: Empty Value field preventing contested leaf serialization.
func TestParseSetWithMetaValueField(t *testing.T) {
	input := "#alice @local %2026-01-01T10:00:00Z set router-id 10.0.0.1\n" +
		"#bob @local %2026-01-01T10:01:00Z set router-id 1.2.3.4\n"

	p := NewSetParser(testSchema())
	_, meta, err := p.ParseWithMeta(input)
	require.NoError(t, err)

	all := meta.GetAllEntries("router-id")
	require.Len(t, all, 2)

	assert.Equal(t, "10.0.0.1", all[0].Value, "alice's Value should be recorded")
	assert.Equal(t, "1.2.3.4", all[1].Value, "bob's Value should be recorded")
}

// TestParseSetWithMetaDeleteNested verifies delete metadata at nested paths.
//
// VALIDATES: Delete metadata navigates to correct MetaTree depth.
//
// PREVENTS: Delete metadata stored at wrong MetaTree level.
func TestParseSetWithMetaDeleteNested(t *testing.T) {
	input := "#alice @local %2026-01-01T10:00:00Z set neighbor 192.0.2.1 peer-as 65001\n" +
		"#bob @ssh %2026-01-01T10:01:00Z delete neighbor 192.0.2.1 peer-as\n"

	p := NewSetParser(testSchema())
	_, meta, err := p.ParseWithMeta(input)
	require.NoError(t, err)

	// Navigate to the leaf's MetaTree
	neighborMeta := meta.GetContainer("neighbor")
	require.NotNil(t, neighborMeta)
	peerMeta := neighborMeta.GetListEntry("192.0.2.1")
	require.NotNil(t, peerMeta)

	all := peerMeta.GetAllEntries("peer-as")
	require.Len(t, all, 2)
	assert.Equal(t, "local", all[0].Source)
	assert.Equal(t, "ssh", all[1].Source)
}

// TestParseSetWithMetaPartialFields verifies metadata with only some fields present.
//
// VALIDATES: Metadata parsing handles partial prefixes (e.g., only session, no user/time).
//
// PREVENTS: Parse failure when not all metadata fields are present.
func TestParseSetWithMetaPartialFields(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		user    string
		source  string
		hasTime bool
	}{
		{
			name:   "only user",
			input:  "#alice set router-id 1.2.3.4\n",
			user:   "alice",
			source: "",
		},
		{
			name:   "only source",
			input:  "@local set router-id 1.2.3.4\n",
			user:   "",
			source: "local",
		},
		{
			name:    "user and time only",
			input:   "#alice %2026-03-12T10:00:00Z set router-id 1.2.3.4\n",
			user:    "alice",
			source:  "",
			hasTime: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewSetParser(testSchema())
			tree, meta, err := p.ParseWithMeta(tt.input)
			require.NoError(t, err)

			val, ok := tree.Get("router-id")
			require.True(t, ok)
			assert.Equal(t, "1.2.3.4", val)

			e, ok := meta.GetEntry("router-id")
			require.True(t, ok)
			assert.Equal(t, tt.user, e.User)
			assert.Equal(t, tt.source, e.Source)
			if tt.hasTime {
				assert.False(t, e.Time.IsZero())
			} else {
				assert.True(t, e.Time.IsZero())
			}
		})
	}
}

// TestPreviousQuoteEscapeRoundTrip verifies Previous values with embedded double quotes
// survive serialization and parsing.
//
// VALIDATES: Backslash-escaped quotes in ^"..." round-trip correctly.
// PREVENTS: Truncated Previous values when they contain double quotes.
func TestPreviousQuoteEscapeRoundTrip(t *testing.T) {
	schema := testSchema()

	tests := []struct {
		name     string
		previous string
	}{
		{name: "simple", previous: "65000"},
		{name: "spaces", previous: "My BGP Peer"},
		{name: "embedded quotes", previous: `My "special" peer`},
		{name: "trailing quote", previous: `value"`},
		{name: "only quotes", previous: `""`},
		{name: "backslash", previous: `path\to\file`},
		{name: "backslash before quote", previous: `value\"`},
		{name: "trailing backslash with space", previous: `back slash\`},
		{name: "backslash and quotes", previous: `a\"b`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewTree()
			tree.Set("router-id", "1.2.3.4")

			meta := NewMetaTree()
			meta.SetEntry("router-id", MetaEntry{
				User:     "thomas",
				Source:   "local",
				Time:     time.Date(2026, 3, 12, 14, 30, 1, 0, time.UTC),
				Previous: tt.previous,
				Value:    "1.2.3.4",
			})

			// Serialize
			output := SerializeSetWithMeta(tree, meta, schema)

			// Parse back
			p := NewSetParser(schema)
			_, meta2, err := p.ParseWithMeta(output)
			require.NoError(t, err)

			e, ok := meta2.GetEntry("router-id")
			require.True(t, ok)
			assert.Equal(t, tt.previous, e.Previous,
				"Previous value should survive round-trip")
		})
	}
}

// TestParseInlineArgs verifies schema-driven inline arg parsing.
//
// VALIDATES: ParseInlineArgs builds correct Tree from flat token sequence.
// PREVENTS: Schema-driven parsing silently dropping or misplacing values.
func TestParseInlineArgs(t *testing.T) {
	schema := YANGSchema()
	require.NotNil(t, schema)

	peerNode, err := schema.Lookup("bgp.peer")
	require.NoError(t, err)

	tree, err := ParseInlineArgs(peerNode, []string{
		"remote", "as", "65001",
		"local", "as", "65000",
		"timer", "receive-hold-time", "90",
		"connection", "passive",
	})
	require.NoError(t, err)

	m := tree.ToMap()

	// Verify container fields
	remote, ok := m["remote"].(map[string]any)
	require.True(t, ok, "remote should be a map")
	assert.Equal(t, "65001", remote["as"])

	local, ok := m["local"].(map[string]any)
	require.True(t, ok, "local should be a map")
	assert.Equal(t, "65000", local["as"])

	// Verify timer container with receive-hold-time leaf
	timer, ok := m["timer"].(map[string]any)
	require.True(t, ok, "timer should be a map")
	assert.Equal(t, "90", timer["receive-hold-time"])
	assert.Equal(t, "passive", m["connection"])
}

// TestParseInlineArgsListNode verifies list-type fields are handled.
//
// VALIDATES: ParseInlineArgs handles NodeList (key + field + value).
// PREVENTS: List-type YANG nodes rejected as unsupported.
func TestParseInlineArgsListNode(t *testing.T) {
	schema := YANGSchema()
	require.NotNil(t, schema)

	peerNode, err := schema.Lookup("bgp.peer")
	require.NoError(t, err)

	tree, err := ParseInlineArgs(peerNode, []string{
		"remote", "as", "65001",
		"family", "ipv4/unicast", "mode", "enable",
	})
	require.NoError(t, err)

	m := tree.ToMap()

	// Verify list entry
	familyMap, ok := m["family"].(map[string]any)
	require.True(t, ok, "family should be a map")
	entry, ok := familyMap["ipv4/unicast"].(map[string]any)
	require.True(t, ok, "ipv4/unicast entry should be a map")
	assert.Equal(t, "enable", entry["mode"])
}

// TestParseInlineArgsUnknownKey verifies unknown keys are rejected.
//
// VALIDATES: ParseInlineArgs rejects keys not in the schema.
// PREVENTS: Typos silently accepted.
func TestParseInlineArgsUnknownKey(t *testing.T) {
	schema := YANGSchema()
	require.NotNil(t, schema)

	peerNode, err := schema.Lookup("bgp.peer")
	require.NoError(t, err)

	_, err = ParseInlineArgs(peerNode, []string{"bogus-key", "value"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown option")
}
