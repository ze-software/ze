package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseRestoreArgs verifies the restore grammar: <file> then the mode
// keyword, then the optional name keyword with its value; anything else is
// refused naming what was wrong.
// VALIDATES: AC-7, AC-20 (name keyword).
// PREVENTS: a mistyped mode or keyword reaching the store.
func TestParseRestoreArgs(t *testing.T) {
	parsed, err := parseRestoreArgs([]string{"/b.zefs", "config", "name", "a.conf"})
	require.NoError(t, err)
	assert.Equal(t, restoreArgs{file: "/b.zefs", mode: restoreModeConfig, sourceName: "a.conf"}, parsed)
	for args, want := range map[string][]string{
		"missing":         {"/b.zefs"},
		"unknown mode":    {"/b.zefs", "partial"},
		"unknown keyword": {"/b.zefs", "config", "label", "x"},
		"needs a config":  {"/b.zefs", "config", "name"},
	} {
		_, err := parseRestoreArgs(want)
		require.Error(t, err, args)
		assert.Contains(t, err.Error(), args)
	}
}
