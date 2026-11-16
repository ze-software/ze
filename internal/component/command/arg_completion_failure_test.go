// Design: docs/architecture/cli/command-completion.md -- safe completion failures.

package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestArgumentCompletionFailureDoesNotRepeatValues preserves the real query's
// arguments while malformed-answer recovery never repeats private input.
func TestArgumentCompletionFailureDoesNotRepeatValues(t *testing.T) {
	const completed = "private-completed-value"
	const partial = "private-partial-value"
	for _, tc := range []struct{ payload, reason string }{
		{"not JSON", "runtime values are unavailable"},
		{"{}", "the daemon's answer cannot establish the available values"},
	} {
		t.Run(tc.payload, func(t *testing.T) {
			calls := 0
			suggestions, err := ArgumentSuggestions(func(line string) (string, error) {
				calls++
				words, _, _, scanErr := ScanInvocation(line)
				require.NoError(t, scanErr)
				assert.Equal(t, []string{"system", "command", "complete", "args", "show sample", completed, partial}, words)
				return tc.payload, nil
			}, "show sample", []string{completed}, partial)
			require.Error(t, err)
			assert.Nil(t, suggestions)
			assert.Equal(t, 1, calls)
			assert.NotContains(t, err.Error(), completed)
			assert.NotContains(t, err.Error(), partial)
			assert.Contains(t, err.Error(), tc.reason)
			assert.Contains(t, err.Error(), "fix: system command list")
		})
	}
}
