// Design: docs/features/cli-commands.md -- transport-independent recall eligibility.
package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestHistoryCommandModeFollowsEnter pins useful operational and configuration
// commands beside the controls Enter consumes without adding a recall entry.
func TestHistoryCommandModeFollowsEnter(t *testing.T) {
	for _, tc := range []struct {
		input  string
		start  EditorMode
		want   EditorMode
		record bool
	}{
		{"set bgp router-id 192.0.2.1", ModeOperational, ModeConfig, true},
		{"show bgp", ModeOperational, ModeOperational, true},
		{"configure", ModeOperational, ModeOperational, false},
		{"exit", ModeConfig, ModeConfig, false},
		{"top quit", ModeConfig, ModeConfig, false},
		{"top stop", ModeConfig, ModeConfig, false},
		{"restart", ModeOperational, ModeOperational, false},
		{"run", ModeConfig, ModeConfig, false},
		{"top run show bgp", ModeConfig, ModeConfig, true},
		{"set cli format json", ModeOperational, ModeOperational, true},
	} {
		t.Run(tc.input, func(t *testing.T) {
			mode, record := HistoryCommandMode(tc.start, tc.input, nil)
			require.Equal(t, tc.want, mode)
			require.Equal(t, tc.record, record)
		})
	}
}
