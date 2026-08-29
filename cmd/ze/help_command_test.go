//go:build ze_core

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectCommands(t *testing.T) {
	entries := collectCommands()
	require.NotEmpty(t, entries, "collectCommands must return commands")

	for _, e := range entries {
		assert.NotEmpty(t, e.Path, "every entry must have a path")
		assert.NotEmpty(t, e.Mode, "every entry must have a mode")
		assert.NotEmpty(t, e.Description, "command %q must have a description", e.Path)
	}

	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		assert.False(t, seen[e.Path], "duplicate path %q", e.Path)
		seen[e.Path] = true
	}

	for i := 1; i < len(entries); i++ {
		assert.True(t, entries[i-1].Path <= entries[i].Path,
			"entries must be sorted: %q > %q", entries[i-1].Path, entries[i].Path)
	}
}

func TestFilterCommands(t *testing.T) {
	entries := []commandEntry{
		{Path: "show bgp peers", Description: "List all BGP peer sessions", Mode: "read-only"},
		{Path: "set bgp peer", Description: "Configure a BGP peer", Mode: "daemon"},
		{Path: "show interface", Description: "List OS network interfaces", Mode: "read-only"},
	}

	filtered := filterCommands(entries, "bgp")
	assert.Len(t, filtered, 2)
	for _, e := range filtered {
		assert.Contains(t, e.Path+e.Description, "bgp")
	}

	filtered = filterCommands(entries, "BGP")
	assert.Len(t, filtered, 2, "filter must be case-insensitive")

	filtered = filterCommands(entries, "interface")
	assert.Len(t, filtered, 1)
	assert.Equal(t, "show interface", filtered[0].Path)

	filtered = filterCommands(entries, "nonexistent")
	assert.Empty(t, filtered)
}

func TestExtractCommandFilter(t *testing.T) {
	assert.Equal(t, "bgp", extractCommandFilter([]string{"bgp", "--json"}))
	assert.Equal(t, "", extractCommandFilter([]string{"--json"}))
	assert.Equal(t, "show", extractCommandFilter([]string{"show"}))
	assert.Equal(t, "", extractCommandFilter(nil))
}

// VALIDATES: the published command catalog spells an argument placeholder as
// <id>, not as a unicode escape.
// PREVENTS: the state the gh-pages catalog reached on 2026-08-29, where every
// angle bracket in a usage line was published as a six-character escape,
// because encoding/json escapes HTML by default and this encoder had never
// said otherwise. The bytes decode to the same string, so no parser complains
// and nothing goes red. A person reading the reference is the one who pays.
func TestCommandJSONDoesNotEscapeAngleBrackets(t *testing.T) {
	entries := []commandEntry{{
		Path:        "request cache expire",
		Description: "Remove a cached message immediately.\nUsage: request cache expire <id>.",
		Mode:        "daemon",
	}}

	var out bytes.Buffer
	require.Equal(t, 0, printCommandJSON(&out, entries))

	assert.Contains(t, out.String(), "<id>",
		"the catalog must publish the placeholder as written")
	assert.NotContains(t, strings.ToLower(out.String()), "u003c",
		"an angle bracket must not reach the catalog as a unicode escape")

	var round []commandEntry
	require.NoError(t, json.Unmarshal(out.Bytes(), &round))
	require.Len(t, round, 1)
	assert.Equal(t, entries[0].Description, round[0].Description,
		"the answer must still parse back to the description it was given")
}
