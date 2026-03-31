package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testTime returns a fixed time for deterministic test output.
func testAnnotatedTime() time.Time {
	return time.Date(2026, 3, 18, 23, 52, 0, 0, time.UTC)
}

// buildSimpleTreeWithMeta creates a minimal tree+meta for annotated serialization tests.
// Tree: bgp { router-id 1.2.3.4 }
// Meta: router-id has user=thomas, source=local, time=testTime.
func buildSimpleTreeWithMeta(t *testing.T) (*Tree, *MetaTree, *Schema) {
	t.Helper()
	schema, schemaErr := YANGSchema()
	require.NoError(t, schemaErr)
	parser := NewParser(schema)
	tree, err := parser.Parse("bgp { router-id 1.2.3.4; }")
	require.NoError(t, err)

	meta := NewMetaTree()
	bgpMeta := meta.GetOrCreateContainer("bgp")
	bgpMeta.SetEntry("router-id", MetaEntry{
		User:   "thomas",
		Source: "local",
		Time:   testAnnotatedTime(),
	})

	return tree, meta, schema
}

// buildMultiLeafTreeWithMeta creates a tree with multiple leaves for alignment testing.
func buildMultiLeafTreeWithMeta(t *testing.T) (*Tree, *MetaTree, *Schema) {
	t.Helper()
	schema, schemaErr := YANGSchema()
	require.NoError(t, schemaErr)
	parser := NewParser(schema)
	tree, err := parser.Parse(`bgp {
  router-id 1.2.3.4;
  session { asn { local 65000; } }
}`)
	require.NoError(t, err)

	meta := NewMetaTree()
	bgpMeta := meta.GetOrCreateContainer("bgp")
	bgpMeta.SetEntry("router-id", MetaEntry{
		User:   "thomas",
		Source: "local",
		Time:   testAnnotatedTime(),
	})
	localMeta := bgpMeta.GetOrCreateContainer("local")
	localMeta.SetEntry("as", MetaEntry{
		User:   "alice",
		Source: "192.168.1.5",
		Time:   time.Date(2026, 3, 18, 23, 53, 0, 0, time.UTC),
	})

	return tree, meta, schema
}

// TestSerializeAnnotatedTree verifies tree format with various column combinations.
//
// VALIDATES: SerializeAnnotatedTree produces hierarchical tree with correct gutter columns.
// PREVENTS: Missing or misplaced metadata columns in annotated tree output.
func TestSerializeAnnotatedTree(t *testing.T) {
	tree, meta, schema := buildSimpleTreeWithMeta(t)

	tests := []struct {
		name    string
		columns ShowColumns
		want    string // substring that must appear in output
		notWant string // substring that must NOT appear
	}{
		{
			name:    "author only",
			columns: ShowColumns{Author: true},
			want:    "thomas",
		},
		{
			name:    "date only",
			columns: ShowColumns{Date: true},
			want:    "03-18 23:52",
		},
		{
			name:    "source only",
			columns: ShowColumns{Source: true},
			want:    "local",
		},
		{
			name:    "changes only",
			columns: ShowColumns{Changes: true},
			want:    "+",
		},
		{
			name:    "all columns",
			columns: ShowColumns{Author: true, Date: true, Source: true, Changes: true},
			want:    "thomas",
		},
		{
			name:    "author+changes",
			columns: ShowColumns{Author: true, Changes: true},
			want:    "thomas",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SerializeAnnotatedTree(tree, meta, schema, tt.columns)
			assert.Contains(t, result, tt.want)
			// All outputs should contain the tree structure
			assert.Contains(t, result, "bgp {")
			assert.Contains(t, result, "router-id 1.2.3.4")
			if tt.notWant != "" {
				assert.NotContains(t, result, tt.notWant)
			}
		})
	}
}

// TestSerializeAnnotatedSet verifies set format with column gutter.
//
// VALIDATES: SerializeAnnotatedSet produces flat set commands with correct gutter columns.
// PREVENTS: Annotated set serializer produces tree format instead of set commands.
func TestSerializeAnnotatedSet(t *testing.T) {
	tree, meta, schema := buildSimpleTreeWithMeta(t)

	tests := []struct {
		name    string
		columns ShowColumns
		want    string
	}{
		{
			name:    "author only",
			columns: ShowColumns{Author: true},
			want:    "thomas",
		},
		{
			name:    "all columns",
			columns: ShowColumns{Author: true, Date: true, Source: true, Changes: true},
			want:    "thomas",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SerializeAnnotatedSet(tree, meta, schema, tt.columns)
			assert.Contains(t, result, tt.want)
			// Set format: flat commands with "set" prefix
			assert.Contains(t, result, "set bgp")
		})
	}
}

// TestSerializeAnnotatedTreeNoMeta verifies blank padding when columns are enabled but no metadata exists.
//
// VALIDATES: Lines without metadata get blank padding to maintain column alignment.
// PREVENTS: Misaligned output when some lines have metadata and others don't.
func TestSerializeAnnotatedTreeNoMeta(t *testing.T) {
	schema, schemaErr := YANGSchema()
	require.NoError(t, schemaErr)
	parser := NewParser(schema)
	tree, err := parser.Parse("bgp { router-id 1.2.3.4; }")
	require.NoError(t, err)

	// No metadata at all
	meta := NewMetaTree()

	result := SerializeAnnotatedTree(tree, meta, schema, ShowColumns{Author: true})
	// Should have blank padding where author would go, then the tree content
	assert.Contains(t, result, "bgp {")
	assert.Contains(t, result, "router-id 1.2.3.4")

	// All lines should have the same gutter width (14 chars for author + 2 spaces padding)
	for line := range strings.SplitSeq(strings.TrimRight(result, "\n"), "\n") {
		if line == "" {
			continue
		}
		// Each line should start with at least 16 chars of padding (14 author + 2 space)
		assert.True(t, len(line) >= 16, "line too short for gutter: %q", line)
	}
}

// TestAnnotatedGutterWidth verifies fixed-width columns maintain alignment across lines.
//
// VALIDATES: Author column is always 14 chars padded, date is always 11 chars.
// PREVENTS: Jagged gutter when different usernames have different lengths.
func TestAnnotatedGutterWidth(t *testing.T) {
	tree, meta, schema := buildMultiLeafTreeWithMeta(t)

	// Author + Date columns enabled
	result := SerializeAnnotatedTree(tree, meta, schema, ShowColumns{Author: true, Date: true})
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")

	// All non-empty lines should have a consistent gutter width
	// Author: 14 + 2 = 16. Date: 11 + 2 = 13. Total gutter = 29 chars.
	gutterWidth := annotatedAuthorWidth + annotatedColumnSpacing + annotatedDateWidth + annotatedColumnSpacing
	for _, line := range lines {
		if line == "" {
			continue
		}
		assert.True(t, len(line) >= gutterWidth,
			"line shorter than gutter width %d: %q", gutterWidth, line)
	}
}

// TestAnnotatedContainerInheritance verifies opening brace gets first child metadata,
// closing brace gets last child metadata.
//
// VALIDATES: Container opening brace inherits first child entry, closing inherits last.
// PREVENTS: Empty gutter on brace lines when children have metadata.
func TestAnnotatedContainerInheritance(t *testing.T) {
	tree, meta, schema := buildMultiLeafTreeWithMeta(t)

	result := SerializeAnnotatedTree(tree, meta, schema, ShowColumns{Author: true})
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")

	// Find the "bgp {" line -- should have thomas (first child's metadata)
	var bgpOpenLine string
	for _, line := range lines {
		if strings.Contains(line, "bgp {") {
			bgpOpenLine = line
			break
		}
	}
	require.NotEmpty(t, bgpOpenLine, "should find bgp { line")
	assert.Contains(t, bgpOpenLine, "thomas", "opening brace should inherit first child metadata")

	// Find the closing "}" line for bgp -- last line containing "}" but NOT "{"
	var bgpCloseLine string
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], "}") && !strings.Contains(lines[i], "{") {
			bgpCloseLine = lines[i]
			break
		}
	}
	require.NotEmpty(t, bgpCloseLine, "should find closing } line")
	// Last child under bgp is "local" container whose child is "as" by alice
	assert.Contains(t, bgpCloseLine, "alice", "closing brace should inherit last child metadata")
}

// TestSerializeAnnotatedTreeNoColumns verifies that with no columns enabled,
// SerializeAnnotatedTree returns the same output as Serialize.
//
// VALIDATES: AC-1 -- show with no columns displays bare hierarchical tree.
// PREVENTS: Spurious padding or metadata when all columns disabled.
func TestSerializeAnnotatedTreeNoColumns(t *testing.T) {
	tree, _, schema := buildSimpleTreeWithMeta(t)

	annotated := SerializeAnnotatedTree(tree, nil, schema, ShowColumns{})
	plain := Serialize(tree, schema)

	assert.Equal(t, plain, annotated, "no-column annotated should equal bare Serialize")
}

// TestAnnotatedAuthorTruncation verifies long usernames are truncated to 14 chars.
//
// VALIDATES: Username longer than 14 chars is truncated to fit fixed-width column.
// PREVENTS: Jagged gutter with long usernames.
func TestAnnotatedAuthorTruncation(t *testing.T) {
	schema, schemaErr := YANGSchema()
	require.NoError(t, schemaErr)
	parser := NewParser(schema)
	tree, err := parser.Parse("bgp { router-id 1.2.3.4; }")
	require.NoError(t, err)

	meta := NewMetaTree()
	bgpMeta := meta.GetOrCreateContainer("bgp")
	bgpMeta.SetEntry("router-id", MetaEntry{
		User:   "averylongusernamehere",
		Source: "local",
		Time:   testAnnotatedTime(),
	})

	result := SerializeAnnotatedTree(tree, meta, schema, ShowColumns{Author: true})
	// Username should be truncated: "averylongusern" (14 chars)
	assert.Contains(t, result, "averylongusern")
	assert.NotContains(t, result, "averylongusernamehere")
}

// TestSerializeAnnotatedNilInputs verifies nil tree/schema/meta don't panic.
//
// VALIDATES: Nil safety for all public annotated serializer functions.
// PREVENTS: Nil pointer dereference when called with missing data.
func TestSerializeAnnotatedNilInputs(t *testing.T) {
	schema, schemaErr := YANGSchema()
	require.NoError(t, schemaErr)
	tree := NewTree()

	assert.Equal(t, "", SerializeAnnotatedTree(nil, nil, schema, ShowColumns{Author: true}))
	assert.Equal(t, "", SerializeAnnotatedTree(tree, nil, nil, ShowColumns{Author: true}))
	assert.Equal(t, "", SerializeAnnotatedSet(nil, nil, schema, ShowColumns{Author: true}))
	assert.Equal(t, "", SerializeAnnotatedSet(tree, nil, nil, ShowColumns{Author: true}))
}

// TestSanitizePrintable verifies non-printable characters are stripped.
//
// VALIDATES: ANSI escape sequences and control characters removed from metadata display.
// PREVENTS: Terminal escape code injection via malicious change files.
func TestSanitizePrintable(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clean", "thomas", "thomas"},
		{"escape sequence", "\x1b[31mred\x1b[0m", "[31mred[0m"},
		{"null byte", "user\x00name", "username"},
		{"tab", "user\tname", "username"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sanitizePrintable(tt.input))
		})
	}
}

// TestTruncateRunes verifies rune-aware truncation for multi-byte characters.
//
// VALIDATES: truncateRunes handles ASCII, multi-byte UTF-8, and edge cases.
// PREVENTS: Invalid UTF-8 from splitting multi-byte characters at byte boundary.
func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxRunes int
		want     string
	}{
		{"ascii short", "hello", 10, "hello"},
		{"ascii exact", "hello", 5, "hello"},
		{"ascii truncate", "hello world", 5, "hello"},
		{"empty", "", 5, ""},
		{"zero max", "hello", 0, ""},
		{"cjk truncate", "\u4e16\u754c\u4f60\u597d", 2, "\u4e16\u754c"},        // 4 CJK chars, keep 2
		{"accented", "\u00e9\u00e8\u00ea\u00eb", 3, "\u00e9\u00e8\u00ea"},      // 4 accented, keep 3
		{"emoji", "\U0001f600\U0001f601\U0001f602", 2, "\U0001f600\U0001f601"}, // 3 emoji, keep 2
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, truncateRunes(tt.input, tt.maxRunes))
		})
	}
}
