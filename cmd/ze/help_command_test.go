//go:build !ze_test

package main

import (
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
