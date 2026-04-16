package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// TestDropPlaintextPasswordEntries: the helper drops only entries whose
// final path segment starts with "plaintext-"; preserves order; returns
// a fresh slice (input is not mutated).
//
// VALIDATES: orphan-metadata cleanup after ApplyPasswordHashing.
// PREVENTS: commit metadata referencing a leaf the hash hook removed.
func TestDropPlaintextPasswordEntries(t *testing.T) {
	in := []config.SessionEntry{
		{Path: "system authentication user alice plaintext-password"},
		{Path: "bgp local-as"},
		{Path: "system authentication user bob plaintext-password"},
		{Path: "system authentication user alice profile"},
	}
	original := make([]config.SessionEntry, len(in))
	copy(original, in)

	out := dropPlaintextPasswordEntries(in)

	assert.Len(t, out, 2)
	assert.Equal(t, "bgp local-as", out[0].Path)
	assert.Equal(t, "system authentication user alice profile", out[1].Path)

	// Input slice must not have been mutated (we returned a fresh allocation).
	for i, se := range in {
		assert.Equal(t, original[i].Path, se.Path,
			"input slice modified at index %d", i)
	}
}

// TestDropPlaintextPasswordEntriesEmpty: empty input returns empty output.
func TestDropPlaintextPasswordEntriesEmpty(t *testing.T) {
	out := dropPlaintextPasswordEntries(nil)
	assert.Empty(t, out)
}

// TestDropPlaintextPasswordEntriesNoMatches: no plaintext-* entries returns
// the same content (different backing array).
func TestDropPlaintextPasswordEntriesNoMatches(t *testing.T) {
	in := []config.SessionEntry{
		{Path: "bgp local-as"},
		{Path: "system authentication user alice profile"},
	}
	out := dropPlaintextPasswordEntries(in)
	assert.Len(t, out, 2)
	assert.Equal(t, in[0].Path, out[0].Path)
	assert.Equal(t, in[1].Path, out[1].Path)
}

// TestDropPlaintextPasswordEntriesAllMatch: all-plaintext input returns empty.
func TestDropPlaintextPasswordEntriesAllMatch(t *testing.T) {
	in := []config.SessionEntry{
		{Path: "system authentication user alice plaintext-password"},
		{Path: "system authentication user bob plaintext-password"},
	}
	out := dropPlaintextPasswordEntries(in)
	assert.Empty(t, out)
}
