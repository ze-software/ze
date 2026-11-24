// Design: docs/architecture/config/syntax.md -- set and set-meta comment preservation.
// Related: serialize_set_comments.go, setparser_comments.go.

package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/version"
)

func setCommentSchema() *Schema {
	schema := NewSchema()
	schema.Define("router-id", Leaf(TypeIPv4))
	schema.Define("local-as", Leaf(TypeUint32))
	schema.Define("filters", bracketLeafList(TypeString))
	schema.Define("name-server", valueOrArray(TypeString))
	presence := Container(
		Field("limit", Leaf(TypeUint32)),
	)
	presence.Presence = true
	schema.Define("accounting", presence)
	multiKey := List(TypeString,
		Field("interface", Leaf(TypeString)),
		Field("profile", Leaf(TypeString)),
	)
	multiKey.KeyName = "peer interface"
	schema.Define("session", multiKey)
	schema.Define("bgp", Container(
		Field("description", Leaf(TypeString)),
		Field("peer", List(TypeString,
			Field("description", Leaf(TypeString)),
			Field("peer-as", Leaf(TypeUint32)),
		)),
	))
	return schema
}

// TestSerializeSetCommentsRoundTrip verifies that SerializeSet and
// SerializeSetWithMeta preserve every Tree comment slot and that SetParser
// round-trips them back without colliding with metadata prefixes.
//
// VALIDATES: Root head/closing, block leading/opening/head/closing/trailing,
// leaf leading/trailing (including merged statements), inactive member
// comments, empty list entry comments, multi-key list entry comments,
// presence flag vs block comments, and special comment text (#nospace, empty,
// metadata-like tokens) survive set and set-meta serialization and parsing.
// PREVENTS: SerializeSetWithMeta writers (editor commit, evolution, recovery,
// migration) dropping operator comments on save.
func TestSerializeSetCommentsRoundTrip(t *testing.T) {
	schema := setCommentSchema()
	input := `# header one
#nospace-header
#

# lead router-id
router-id 192.0.2.1 # first router-id
router-id 192.0.2.2 # second router-id
# lead local-as
local-as 65000 # @ssh %2026-01-01T00:00:00Z ^prev "quoted"
# lead filters
filters [ a b ] # trail filters
# lead inactive filter member
inactive: filters b # trail inactive filter member
# lead name-server
name-server [ 1.1.1.1 8.8.8.8 ] # trail name-server
# lead inactive ns member
inactive: name-server 8.8.8.8 # trail inactive ns member
# lead presence flag
accounting # trail presence flag
# lead presence block
accounting { # open presence
	# head presence

	# lead limit
	limit 42 # trail limit
	# closing presence
} # trail presence block
# lead session
session fe80::1 { # open session
	interface eth0
	profile fast # trail profile
} # trail session
# lead bgp
bgp { # open bgp
	# head bgp

	# lead description
	description "edge"
	# lead peer p1
	peer p1 {
		peer-as 65001 # trail peer-as
		# closing peer p1
	} # trail peer p1
	# lead empty peer
	peer p-empty { # open empty peer
	} # trail empty peer
	# lead inactive peer
	inactive: peer p2 {
		peer-as 65002
	}
	# closing bgp
} # trail bgp
# end of file
`
	orig, err := NewParser(schema).Parse(input)
	require.NoError(t, err)
	wantHierarchical := Serialize(orig, schema)

	t.Run("plain set round-trip", func(t *testing.T) {
		setText := SerializeSet(orig, schema)
		require.Equal(t, FormatSet, DetectFormat(setText), "set output with comments must detect as FormatSet:\n%s", setText)

		reparsed, err := NewSetParser(schema).Parse(setText)
		require.NoError(t, err, "set output did not parse:\n%s", setText)
		require.Equal(t, wantHierarchical, Serialize(reparsed, schema))
		require.Equal(t, setText, SerializeSet(reparsed, schema), "second set serialize must be idempotent")
		require.Equal(t, orig.ToMap(), reparsed.ToMap())
	})

	t.Run("set-meta round-trip alongside metadata prefixes", func(t *testing.T) {
		meta := NewMetaTree()
		ts := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
		meta.SetEntry("router-id", MetaEntry{
			User:     "thomas",
			Source:   "ssh",
			Time:     ts,
			Previous: "192.0.2.1",
			Value:    "192.0.2.2",
		})
		meta.SetEntry("local-as", MetaEntry{
			User:   "alice",
			Source: "web",
			Time:   ts,
			Value:  "65000",
		})
		bgpMeta := meta.GetOrCreateContainer("bgp")
		p1Meta := bgpMeta.GetOrCreateContainer("peer").GetOrCreateListEntry("p1")
		p1Meta.SetEntry("peer-as", MetaEntry{
			User:   "bob",
			Source: "local",
			Time:   ts,
			Value:  "65001",
		})

		metaText := SerializeSetWithMeta(orig, meta, schema)
		require.Equal(t, FormatSetMeta, DetectFormat(metaText), "set-meta output with comments must detect as FormatSetMeta:\n%s", metaText)

		reparsed, reparsedMeta, err := NewSetParser(schema).ParseWithMeta(metaText)
		require.NoError(t, err, "set-meta output did not parse:\n%s", metaText)
		require.Equal(t, wantHierarchical, Serialize(reparsed, schema))
		require.Equal(t, metaText, SerializeSetWithMeta(reparsed, reparsedMeta, schema), "second set-meta serialize must be idempotent")
		require.Equal(t, orig.ToMap(), reparsed.ToMap())

		ridMeta, ok := reparsedMeta.GetEntry("router-id")
		require.True(t, ok)
		assert.Equal(t, "thomas", ridMeta.User)
		assert.Equal(t, "ssh", ridMeta.Source)
		assert.Equal(t, "192.0.2.1", ridMeta.Previous)

		peerMeta, ok := reparsedMeta.GetContainer("bgp").GetContainer("peer").GetListEntry("p1").GetEntry("peer-as")
		require.True(t, ok)
		assert.Equal(t, "bob", peerMeta.User)
		assert.Equal(t, "local", peerMeta.Source)
	})
}

// TestSerializeSetCommentsDoNotDuplicateSchemaStamp verifies that a stamped
// hierarchical tree written with FormatSchemaStamp() + SerializeSetWithMeta
// replaces the schema stamp on line 1 while keeping operator head comments.
//
// VALIDATES: Operator head comments after `# ze-schema:` survive set-meta save
// without carrying a stale stamp inside `comment head`.
// PREVENTS: Duplicate or stale schema stamps accumulating across commits.
func TestSerializeSetCommentsDoNotDuplicateSchemaStamp(t *testing.T) {
	schema := commentSchema()
	input := "# ze-schema: 26.10.07\n# site A\n\nrouter-id 192.0.2.1 # primary\n"

	tree, err := NewParser(schema).Parse(input)
	require.NoError(t, err)

	stamped := FormatSchemaStamp() + SerializeSetWithMeta(tree, NewMetaTree(), schema)
	require.Equal(t, version.Release(), ScanStampRelease([]byte(stamped)))
	assert.NotContains(t, stamped, "26.10.07")
	assert.Contains(t, stamped, "site A")

	reparsed, _, err := NewSetParser(schema).ParseWithMeta(stamped)
	require.NoError(t, err)
	wantHierarchical := "# site A\n\nrouter-id 192.0.2.1 # primary\n"
	require.Equal(t, wantHierarchical, Serialize(reparsed, schema))
}

// TestDetectFormatWithSetComments verifies DetectFormat classifies files
// beginning with set-format `comment` lines accurately.
//
// VALIDATES: Files with leading `comment` statements detect as FormatSet when
// bare and FormatSetMeta when metadata prefixes follow.
// PREVENTS: DetectFormat misclassifying commented set/set-meta files as
// FormatHierarchical.
func TestDetectFormatWithSetComments(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  ConfigFormat
	}{
		{
			name:  "comment before set",
			input: "comment head \" header\"\nset router-id 1.2.3.4\n",
			want:  FormatSet,
		},
		{
			name:  "comment only",
			input: "comment head \" header\"\n",
			want:  FormatSet,
		},
		{
			name:  "comment before set-meta",
			input: "comment head \" header\"\n#thomas @local set router-id 1.2.3.4\n",
			want:  FormatSetMeta,
		},
		{
			name:  "comment with hash-identifier text stays FormatSet",
			input: "comment head \"#thomas\"\nset router-id 1.2.3.4\n",
			want:  FormatSet,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, DetectFormat(tc.input))
		})
	}
}

// TestSetParserCommentRefusals verifies malformed `comment` lines fail closed
// in both Parse and ParseWithMeta.
//
// VALIDATES: Unknown comment slots, missing paths, type mismatches (leaf vs
// block), and unknown schema fields return parse errors.
// PREVENTS: Malformed comment directives silently dropping or corrupting state.
func TestSetParserCommentRefusals(t *testing.T) {
	schema := setCommentSchema()
	badLines := []struct {
		name string
		line string
	}{
		{"missing text", "comment head"},
		{"unknown slot", "comment bogus router-id \" text\""},
		{"block-leading without path", "comment block-leading \" text\""},
		{"leading without path", "comment leading \" text\""},
		{"block-leading on leaf", "comment block-leading router-id \" text\""},
		{"leading on non-presence container", "comment leading bgp \" text\""},
		{"member-leading missing member", "comment member-leading filters \" text\""},
		{"member-leading on non-leaflist", "comment member-leading router-id val \" text\""},
		{"unknown field", "comment leading nonexistent \" text\""},
	}
	for _, tc := range badLines {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewSetParser(schema).Parse(tc.line + "\n")
			require.Error(t, err, "Parse should refuse %q", tc.line)
			_, _, err = NewSetParser(schema).ParseWithMeta(tc.line + "\n")
			require.Error(t, err, "ParseWithMeta should refuse %q", tc.line)
		})
	}
}

// TestSetParserCommentSkipUnknownRoots verifies SkipUnknownRoots (FirstPass)
// skips comment statements targeting roots absent from the partial schema.
//
// VALIDATES: FirstPass parses set/set-meta files carrying comments on
// external-plugin or non-plugin roots without failing.
// PREVENTS: FirstPass failing on commented configs before external schemas load.
func TestSetParserCommentSkipUnknownRoots(t *testing.T) {
	schema := NewSchema()
	schema.Define("router-id", Leaf(TypeIPv4))

	input := "comment head \" file\"\n" +
		"comment block-leading external-root \" ext\"\n" +
		"comment leading external-root leaf \" ext leaf\"\n" +
		"set external-root leaf val\n" +
		"comment leading router-id \" rid\"\n" +
		"set router-id 192.0.2.1\n"

	p := NewSetParser(schema)
	p.SkipUnknownRoots()
	tree, err := p.Parse(input)
	require.NoError(t, err)
	rid, ok := tree.Get("router-id")
	require.True(t, ok)
	assert.Equal(t, "192.0.2.1", rid)
	assert.Equal(t, []string{" rid"}, tree.commentsOfLeaf("router-id").Leading)
}

// TestFilterSetByPathComments verifies FilterSetByPath retains comment lines
// matching the selected path and drops unrelated comments.
//
// VALIDATES: Subtree set display keeps comments belonging to the selected path.
// PREVENTS: Unrelated section comments leaking into filtered set output.
func TestFilterSetByPathComments(t *testing.T) {
	schema := setCommentSchema()
	input := "comment head \" root\"\n" +
		"comment leading router-id \" rid\"\n" +
		"set router-id 1.2.3.4\n" +
		"comment block-leading bgp peer p1 \" p1\"\n" +
		"comment trailing bgp peer p1 peer-as \" as\"\n" +
		"set bgp peer p1 peer-as 65001\n" +
		"comment block-leading bgp peer p2 \" p2\"\n" +
		"set bgp peer p2 peer-as 65002\n"

	filtered := FilterSetByPath(input, schema, []string{"bgp", "peer", "p1"})
	assert.Contains(t, filtered, "comment block-leading bgp peer p1 \" p1\"\n")
	assert.Contains(t, filtered, "comment trailing bgp peer p1 peer-as \" as\"\n")
	assert.Contains(t, filtered, "set bgp peer p1 peer-as 65001\n")
	assert.NotContains(t, filtered, "router-id")
	assert.NotContains(t, filtered, "root")
	assert.NotContains(t, filtered, "p2")
}
