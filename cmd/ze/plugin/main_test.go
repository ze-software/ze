package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// VALIDATES: Plugin subcommand dispatch (Run), help flags, unknown plugin handling.
// PREVENTS: Broken dispatch routing, panic on empty args.

// TestRun_NoArgs verifies exit 1 when no arguments provided.
func TestRun_NoArgs(t *testing.T) {
	code := Run(nil)
	assert.Equal(t, 1, code)
}

// TestRun_EmptyArgs verifies exit 1 for empty slice.
func TestRun_EmptyArgs(t *testing.T) {
	code := Run([]string{})
	assert.Equal(t, 1, code)
}

// TestRun_HelpFlag verifies help returns exit 0.
func TestRun_HelpFlag(t *testing.T) {
	tests := []struct {
		name string
		arg  string
	}{
		{"help", "help"},
		{"dash_h", "-h"},
		{"double_dash_help", "--help"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := Run([]string{tt.arg})
			assert.Equal(t, 0, code)
		})
	}
}

// TestRun_UnknownPlugin verifies exit 1 for unregistered plugin name.
func TestRun_UnknownPlugin(t *testing.T) {
	code := Run([]string{"nonexistent-plugin-xyz"})
	assert.Equal(t, 1, code)
}

// TestMapKeys verifies key extraction from map.
func TestMapKeys(t *testing.T) {
	m := map[string]any{
		"alpha": 1,
		"beta":  2,
	}
	keys := mapKeys(m)
	assert.Len(t, keys, 2)
	assert.Contains(t, keys, "alpha")
	assert.Contains(t, keys, "beta")
}

// TestMapKeys_Empty verifies empty map returns empty slice.
func TestMapKeys_Empty(t *testing.T) {
	keys := mapKeys(map[string]any{})
	assert.Empty(t, keys)
}
