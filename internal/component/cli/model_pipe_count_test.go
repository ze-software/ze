// Design: docs/guide/config-editor.md -- display pipes bound storage by their input.

package cli

import (
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTailBoundsRequestedCountByLines asks for a count whose slice size cannot
// fit. The old ring allocation panics before reserving memory; a display limit
// above the available lines must instead preserve the complete input.
func TestTailBoundsRequestedCountByLines(t *testing.T) {
	for _, content := range []string{"", "one", "one\n", "one\ntwo\nthree", "one\ntwo\n"} {
		t.Run(strconv.Quote(content), func(t *testing.T) {
			require.NotPanics(t, func() {
				got, err := ApplyPipeFilter(content, PipeFilter{Type: cmdTail, Arg: strconv.Itoa(math.MaxInt)})
				require.NoError(t, err)
				require.Equal(t, content, got)
			})
		})
	}
}
