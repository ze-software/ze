package configvalue

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLeafListCoercesEveryProducerShape feeds LeafList each shape the config
// pipeline can hand a reader for one YANG leaf-list, and asserts it reads the
// same members out of all of them.
//
// VALIDATES: a leaf-list value arriving as a bare string, a []string, or a
// []any yields the same []string, and an absent, empty or wrongly typed value
// yields nil.
// PREVENTS: the defect class this package exists for. Tree.ToMap collapses a
// one-member leaf-list to a bare string, so a reader that asserts []any drops
// the operator's value at exactly one member and keeps it at two. The single
// member case is the one that discriminates: a multi-member case passes with
// the bug in place.
func TestLeafListCoercesEveryProducerShape(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   any
		want []string
	}{
		{"one member, the shape ToMap emits at count one", "10.0.0.0/8", []string{"10.0.0.0/8"}},
		{"several members in process", []string{"10.0.0.0/8", "192.168.0.0/16"}, []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{"several members over JSON", []any{"10.0.0.0/8", "192.168.0.0/16"}, []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{"one member over JSON, an array of one", []any{"10.0.0.0/8"}, []string{"10.0.0.0/8"}},
		{"absent leaf", nil, nil},
		{"empty string", "", nil},
		{"empty in-process slice", []string{}, nil},
		{"empty JSON array", []any{}, nil},
		{"JSON array of non-strings", []any{1.0, true}, nil},
		{"an empty member survives both slice arms, in process", []string{"eth0", ""}, []string{"eth0", ""}},
		{"an empty member survives both slice arms, over JSON", []any{"eth0", ""}, []string{"eth0", ""}},
		{"a slice of only empty members is still members", []any{""}, []string{""}},
		{"a map, which is a list rather than a leaf-list", map[string]any{"a": "b"}, nil},
		{"a number", 42, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, LeafList(tc.in))
		})
	}
}

// TestLeafListDoesNotAliasTheCallersSlice writes through the returned slice and
// asserts the config map is unchanged.
//
// VALIDATES: LeafList returns a slice the caller may keep and mutate.
// PREVENTS: a reader that sorts or truncates its result reordering the config
// map every other reader still reads, on the in-process delivery path where the
// []string comes straight out of Tree.ToMap.
func TestLeafListDoesNotAliasTheCallersSlice(t *testing.T) {
	delivered := []string{"eth0", "eth1"}
	members := LeafList(delivered)
	require.Len(t, members, 2)

	members[0] = "mutated"

	assert.Equal(t, []string{"eth0", "eth1"}, delivered)
}

// TestListEntriesReadsKeyedMap feeds ListEntries the map of key to entry that
// Tree.ToMap emits for a YANG list, and asserts the key reaches the caller.
//
// VALIDATES: a list value yields one entry per key, carrying that key and the
// entry body, ordered by key; a wrongly typed or empty value yields nil.
// PREVENTS: two defects at once. A reader that asserts []any on a list finds
// nothing at EVERY count, because ToMap never emits a slice for a list. A
// reader that then looks for the key leaf inside the entry finds nothing
// either, because ToMap uses the key as the map key and does not repeat it.
func TestListEntriesReadsKeyedMap(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   any
		want []ListEntry
	}{
		{
			name: "one entry, the shape ToMap emits at count one",
			in:   map[string]any{"gold": map[string]any{"gateway": "10.0.0.1"}},
			want: []ListEntry{{Key: "gold", Fields: map[string]any{"gateway": "10.0.0.1"}}},
		},
		{
			name: "several entries are ordered by key",
			in: map[string]any{
				"silver": map[string]any{"gateway": "10.0.1.1"},
				"gold":   map[string]any{"gateway": "10.0.0.1"},
			},
			want: []ListEntry{
				{Key: "gold", Fields: map[string]any{"gateway": "10.0.0.1"}},
				{Key: "silver", Fields: map[string]any{"gateway": "10.0.1.1"}},
			},
		},
		{
			name: "an entry with an empty body still exists",
			in:   map[string]any{"veth-bng": map[string]any{}},
			want: []ListEntry{{Key: "veth-bng", Fields: map[string]any{}}},
		},
		{"absent list", nil, nil},
		{"empty map", map[string]any{}, nil},
		{"a slice, which no producer emits for a list", []any{map[string]any{"name": "gold"}}, nil},
		{"a bare string", "gold", nil},
		{"entries whose bodies are not maps", map[string]any{"gold": "10.0.0.1"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ListEntries(tc.in))
		})
	}
}
