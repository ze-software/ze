package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/command"
)

// TestRelatedExtension_ParseDescriptor verifies that a minimal descriptor with
// only the three required fields (id, label, command) parses cleanly and that
// optional fields receive their documented defaults.
//
// VALIDATES: Wire-format Argument Wire Format table -- required fields,
// default placement=detail, default presentation=modal.
// PREVENTS: A typo in the parser silently dropping a required field, or
// optional fields losing their declared defaults.
func TestRelatedExtension_ParseDescriptor(t *testing.T) {
	got, err := ParseRelatedDescriptor(`id=peer-detail; label="Peer Detail"; command="show bgp peer ${path:connection/remote/ip|key}"`)
	require.NoError(t, err)

	assert.Equal(t, "peer-detail", got.ID)
	assert.Equal(t, "Peer Detail", got.Label)
	assert.Equal(t, "show bgp peer ${path:connection/remote/ip|key}", got.Command)
	assert.Equal(t, RelatedPlacementDetail, got.Placement, "placement default must be detail")
	assert.Equal(t, RelatedPresentationModal, got.Presentation, "presentation default must be modal")
	assert.Empty(t, got.Confirm)
	assert.Empty(t, got.Requires)
	assert.Empty(t, got.Class)
	assert.Equal(t, RelatedEmptyDisable, got.Empty, "empty default must be disable")
}

// TestRelatedExtension_AllOptionalFields verifies every optional field round
// trips with a representative non-default value.
func TestRelatedExtension_AllOptionalFields(t *testing.T) {
	got, err := ParseRelatedDescriptor(`id=peer-teardown; label=Teardown; command=teardown; placement=row; presentation=drawer; confirm="Tear down session?"; requires=peer; class=danger; empty=allow`)
	require.NoError(t, err)

	assert.Equal(t, "peer-teardown", got.ID)
	assert.Equal(t, "Teardown", got.Label)
	assert.Equal(t, "teardown", got.Command)
	assert.Equal(t, RelatedPlacementRow, got.Placement)
	assert.Equal(t, RelatedPresentationDrawer, got.Presentation)
	assert.Equal(t, "Tear down session?", got.Confirm)
	assert.Equal(t, []string{"peer"}, got.Requires)
	assert.Equal(t, RelatedClassDanger, got.Class)
	assert.Equal(t, RelatedEmptyAllow, got.Empty)
}

// TestRelatedExtension_WireFormat_QuotedValues verifies that values containing
// the field separator (`;`), the key/value separator (`=`), the placeholder
// closer (`}`), or leading/trailing whitespace round-trip when quoted, and
// that the escape sequences `\"` and `\\` decode correctly.
//
// VALIDATES: Spec Argument Wire Format -- Quoting and Escapes rows.
// PREVENTS: An operator label containing punctuation breaking the parser, or
// a quoted command being silently truncated at an unescaped delimiter.
func TestRelatedExtension_WireFormat_QuotedValues(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "semicolon in value",
			in:   `id=t; label="a;b"; command=cmd`,
			want: "a;b",
		},
		{
			name: "equals in value",
			in:   `id=t; label="a=b"; command=cmd`,
			want: "a=b",
		},
		{
			name: "brace in value",
			in:   `id=t; label="end}"; command=cmd`,
			want: "end}",
		},
		{
			name: "leading whitespace in quoted value",
			in:   `id=t; label="  padded"; command=cmd`,
			want: "  padded",
		},
		{
			name: "escaped quote",
			in:   `id=t; label="said \"hi\""; command=cmd`,
			want: `said "hi"`,
		},
		{
			name: "escaped backslash",
			in:   `id=t; label="path\\to"; command=cmd`,
			want: `path\to`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseRelatedDescriptor(tc.in)
			require.NoError(t, err, "input: %s", tc.in)
			assert.Equal(t, tc.want, got.Label)
		})
	}
}

// TestRelatedExtension_WireFormat_RejectsUnknownKey verifies that an unknown
// descriptor key fails the parser. Silent acceptance would let typos lurk
// until they cause a runtime feature regression.
//
// VALIDATES: Spec Argument Wire Format -- "Unknown keys: Rejected at parse
// time" row.
// PREVENTS: A typo like `placemnt=row` parsing as a no-op while the operator
// expects the row placement to apply.
func TestRelatedExtension_WireFormat_RejectsUnknownKey(t *testing.T) {
	_, err := ParseRelatedDescriptor(`id=t; label=L; command=c; placemnt=row`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "placemnt", "error must name the offending key")
}

// TestRelatedExtension_WireFormat_RejectsBadEscape verifies that an unknown
// escape sequence inside a quoted value (`\n`, `\x`, etc.) is a parse error.
//
// VALIDATES: Spec "Unknown escape sequences (`\x`, `\n`, etc.) are a parse error".
// PREVENTS: Silent acceptance of an unintended escape that operators expect
// to be a literal substring.
func TestRelatedExtension_WireFormat_RejectsBadEscape(t *testing.T) {
	_, err := ParseRelatedDescriptor(`id=t; label="bad\nescape"; command=cmd`)
	require.Error(t, err)
}

// TestRelatedExtension_RejectsInvalidDescriptor covers the cluster of
// rejection cases listed in the Metadata Validation Rules table.
//
// VALIDATES: required fields present; enum values valid; length bounds.
// PREVENTS: Schema build accepting a half-formed annotation that would crash
// at render time.
func TestRelatedExtension_RejectsInvalidDescriptor(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{name: "missing id", in: `label=L; command=cmd`},
		{name: "missing label", in: `id=t; command=cmd`},
		{name: "missing command", in: `id=t; label=L`},
		{name: "empty id value", in: `id=; label=L; command=cmd`},
		{name: "duplicate key", in: `id=a; id=b; label=L; command=cmd`},
		{name: "invalid placement enum", in: `id=t; label=L; command=cmd; placement=elsewhere`},
		{name: "invalid presentation enum", in: `id=t; label=L; command=cmd; presentation=fullscreen`},
		{name: "invalid empty enum", in: `id=t; label=L; command=cmd; empty=force`},
		{name: "id too long", in: `id=` + strings.Repeat("x", 65) + `; label=L; command=cmd`},
		{name: "label too long", in: `id=t; label="` + strings.Repeat("y", 49) + `"; command=cmd`},
		{name: "command too long", in: `id=t; label=L; command="` + strings.Repeat("z", 513) + `"`},
		{name: "trailing field separator with empty key", in: `id=t; label=L; command=cmd; ;`},
		{name: "value missing closing quote", in: `id=t; label="oops; command=cmd`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseRelatedDescriptor(tc.in)
			require.Error(t, err, "input: %s", tc.in)
		})
	}
}

// TestRelatedExtension_RequiresEscapedComma verifies the `\,` escape inside a
// quoted `requires` value lets a placeholder name embed a literal comma.
//
// VALIDATES: Spec "Use `\,` inside a quoted value to embed a literal comma."
// PREVENTS: Comma-bearing placeholder names splitting the requires list.
func TestRelatedExtension_RequiresEscapedComma(t *testing.T) {
	got, err := ParseRelatedDescriptor(`id=t; label=L; command=cmd; requires="path:foo\,bar,key"`)
	require.NoError(t, err)
	assert.Equal(t, []string{"path:foo,bar", "key"}, got.Requires)
}

// TestRelatedExtension_BoundaryLengths exercises the boundary table for id,
// label, and command at exactly the last valid length.
func TestRelatedExtension_BoundaryLengths(t *testing.T) {
	id := strings.Repeat("a", 64)
	label := strings.Repeat("b", 48)
	cmd := strings.Repeat("c", 512)
	got, err := ParseRelatedDescriptor(`id=` + id + `; label="` + label + `"; command="` + cmd + `"`)
	require.NoError(t, err)
	assert.Len(t, got.ID, 64)
	assert.Len(t, got.Label, 48)
	assert.Len(t, got.Command, 512)
}

// TestRelatedExtension_RejectsCommandNotInTree verifies that a descriptor
// whose static command prefix is not present in the operational command
// tree fails validation. This is the schema-load gate that catches typos
// and renamed commands before any operator click; the spec lists it under
// the Metadata Validation Rules table.
//
// Tools whose first token is a placeholder (no static prefix to anchor)
// pass the check -- there is nothing to validate without runtime data.
//
// VALIDATES: ValidateRelatedAgainstCommandTree fails on unknown prefixes,
// passes on known prefixes, and stays silent on no-static-prefix tools.
// PREVENTS: Renamed commands lurking in YANG until a user hits them.
func TestRelatedExtension_RejectsCommandNotInTree(t *testing.T) {
	// Build a stub command tree mirroring the structure that BuildCommandTree
	// produces for show / peer subtrees.
	tree := &command.Node{
		Children: map[string]*command.Node{
			"show": {
				Name: "show",
				Children: map[string]*command.Node{
					"bgp-health": {Name: "bgp-health", WireMethod: "ze-show:bgp-health"},
					"warnings":   {Name: "warnings", WireMethod: "ze-show:warnings"},
					"errors":     {Name: "errors", WireMethod: "ze-show:errors"},
				},
			},
			"peer": {
				Name: "peer",
				Children: map[string]*command.Node{
					"detail":       {Name: "detail", WireMethod: "ze-bgp:peer-detail"},
					"capabilities": {Name: "capabilities", WireMethod: "ze-bgp:peer-capabilities"},
				},
			},
		},
	}

	cases := []struct {
		name     string
		template string
		wantErr  bool
	}{
		{name: "known static prefix", template: `peer ${path:connection/remote/ip|key} detail`, wantErr: false},
		{name: "known global command", template: `show bgp-health`, wantErr: false},
		{name: "renamed command", template: `peer ${path:connection/remote/ip|key} caps`, wantErr: false}, // suffix not validated, prefix `peer` exists
		{name: "typo in static prefix", template: `peeer ${path:connection/remote/ip|key} detail`, wantErr: true},
		{name: "non-existent root", template: `nonexistent`, wantErr: true},
		{name: "stale show subtree", template: `show bgp peer`, wantErr: true},
		{name: "placeholder first token", template: `${path:foo}`, wantErr: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tool, err := ParseRelatedDescriptor(`id=t; label=L; command="` + tc.template + `"`)
			require.NoError(t, err)
			errs := ValidateRelatedAgainstCommandTree([]*RelatedTool{tool}, tree)
			if tc.wantErr {
				assert.NotEmpty(t, errs, "expected validation error for %q", tc.template)
			} else {
				assert.Empty(t, errs, "unexpected validation error for %q: %v", tc.template, errs)
			}
		})
	}
}
