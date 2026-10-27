// Design: docs/architecture/config/change-file-structural-ops.md -- canonical creation paths.

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEditPathQuotedRoundTrip uses the configuration key codec in both
// directions; whitespace and literal quote characters remain within one key.
func TestEditPathQuotedRoundTrip(t *testing.T) {
	for _, key := range []string{"edge west", `edge"west`, `edge\west`, "edge#west"} {
		t.Run(key, func(t *testing.T) {
			path := []string{"bgp", "peer", key}
			decoded, err := ParseEditPath(FormatEditPath(path))
			require.NoError(t, err)
			assert.Equal(t, path, decoded)
		})
	}
	_, err := ParseEditPath(`bgp peer "unterminated`)
	require.Error(t, err)
}

// TestEditPathCanonicalIdentity compares complete entry addresses, not textual
// prefixes, while preserving the actual list and container ancestor boundaries.
func TestEditPathCanonicalIdentity(t *testing.T) {
	schema := editPathSchema()
	input := []string{"bfd", "session", "fe80::b", "interface", "zbl0", "vrf", "default"}
	canonical, err := CanonicalEditPath(schema, input)
	require.NoError(t, err)
	assert.Equal(t, []string{"bfd", "session", "fe80::b", "interface", "zbl0"}, canonical)
	path := FormatEditPath(canonical)
	assertEditPathPrefix(t, schema, path, "bfd", true)
	assertEditPathPrefix(t, schema, path, "bfd session", true)
	assertEditPathPrefix(t, schema, path+" description", path, true)
	assertEditPathPrefix(t, schema, path, "bfd session fe80::b", false)
	assertEditPathPrefix(t, schema, "bfd session fe80::b", path, false)
	assertEditPathPrefix(t, schema, path, "bfd session fe80::b vrf default interface zbl0", true)

	rebased, err := RebaseEditPath(schema, path+" description", "bfd session fe80::b interface zbl0", "bfd session fe80::c interface zbl0")
	require.NoError(t, err)
	assert.Equal(t, "bfd session fe80::c interface zbl0 description", rebased)
	unchanged, err := RebaseEditPath(schema, path, "bfd session fe80::b", "bfd session fe80::c")
	require.NoError(t, err)
	assert.Equal(t, path, unchanged, "renaming a sibling must not rebase this entry")
}

// TestEditPathRebaseQuotedAncestor preserves string-key token boundaries when
// a created subtree is moved rather than splitting its ancestor's name.
func TestEditPathRebaseQuotedAncestor(t *testing.T) {
	schema := testChangeFileSchema()
	path, err := RebaseEditPath(schema, `bgp peer "edge west" description`, `bgp peer "edge west"`, `bgp peer "edge east"`)
	require.NoError(t, err)
	assert.Equal(t, `bgp peer "edge east" description`, path)
	assertEditPathPrefix(t, schema, path, "bgp peer edge", false)
}

func editPathSchema() *Schema {
	vrf := Leaf(TypeString)
	vrf.Default = "default"
	sessions := List(TypeString,
		Field("peer", Leaf(TypeString)),
		Field("vrf", vrf),
		Field("interface", Leaf(TypeString)),
		Field("description", Leaf(TypeString)),
	)
	sessions.KeyName = "peer vrf interface"
	schema := NewSchema()
	schema.Define("bfd", Container(Field("session", sessions)))
	return schema
}

// TestEditPathResolutionErrors distinguishes unresolved addresses from valid
// non-overlap before ownership or rebase decisions can be made.
func TestEditPathResolutionErrors(t *testing.T) {
	schema := testChangeFileSchema()
	for _, tc := range []struct {
		name        string
		path        string
		source      string
		destination string
	}{
		{"unclosed path", `bgp peer "edge`, "bgp peer edge", "bgp peer east"},
		{"unclosed source", "bgp peer edge", `bgp peer "edge`, "bgp peer east"},
		{"unknown path", "obsolete entry", "bgp peer edge", "bgp peer east"},
		{"unknown source", "bgp peer edge", "obsolete entry", "bgp peer east"},
		{"unknown destination", "bgp peer edge", "bgp peer edge", "obsolete entry"},
		{"unknown descendant", "bgp peer edge description unknown", "bgp peer edge", "bgp peer east"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RebaseEditPath(schema, tc.path, tc.source, tc.destination)
			require.Error(t, err, "unresolvable path must not look like an unrelated or valid path")
		})
	}
	_, err := ResolveEditPath(nil, "bgp peer edge")
	require.Error(t, err, "a missing schema cannot establish path ownership")
}

func assertEditPathPrefix(t *testing.T, schema *Schema, path, prefix string, want bool) {
	t.Helper()
	got, err := EditPathHasPrefix(schema, path, prefix)
	require.NoError(t, err)
	assert.Equal(t, want, got, "%s under %s", path, prefix)
}
