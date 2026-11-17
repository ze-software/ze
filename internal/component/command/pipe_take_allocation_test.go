// Design: docs/architecture/api/commands.md -- row limits preserve the answer shape.

package command

import (
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestApplyTakeBoundsRequestedCountByRows asks for the largest valid count over
// tiny answers. An allocation sized from that count overflows before allocating,
// so the regression fails safely without exhausting the test host's memory.
func TestApplyTakeBoundsRequestedCountByRows(t *testing.T) {
	operators := []struct {
		name string
		take func(string, string) (string, string)
	}{
		{name: "first", take: applyFirst},
		{name: "last", take: applyLast},
	}
	answers := []struct {
		name string
		body string
	}{
		{name: "array", body: `[{"n":1},{"n":2}]`},
		{name: "envelope", body: `{"peers":[{"n":1},{"n":2}],"total":2}`},
		{name: "identity", body: `{"peers":{"192.0.2.1":{"n":1},"192.0.2.2":{"n":2}}}`},
		{name: "empty", body: `[]`},
		{name: "empty-envelope", body: `{"peers":[],"total":0}`},
	}
	count := strconv.Itoa(math.MaxInt)
	for _, operator := range operators {
		for _, answer := range answers {
			t.Run(operator.name+"/"+answer.name, func(t *testing.T) {
				require.NotPanics(t, func() {
					got, msg := operator.take(answer.body, count)
					require.Empty(t, msg)
					require.JSONEq(t, answer.body, got)
				})
			})
		}
	}
}
