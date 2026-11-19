// Design: docs/architecture/fleet-interface.md -- the task kinds the sample answers

package sample

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/web/fleetui"
)

// TestSampleTaskKindsHaveWords pins every task kind the sample answers, the
// tasks the script creates included, to words of its own: a kind added to the
// sample without words in fleetui.TaskKindWords fails here.
func TestSampleTaskKindsHaveWords(t *testing.T) {
	source := newTestEstate(t)
	ctx := t.Context()

	kinds := map[string]struct{}{}
	collect := func() {
		tasks, err := source.Tasks(ctx)
		require.NoError(t, err)
		for index := range tasks {
			kinds[tasks[index].Kind] = struct{}{}
		}
	}

	collect()
	var stepper Stepper = source
	for range len(scriptSteps) {
		_, err := stepper.Step()
		require.NoError(t, err)
		collect()
	}

	require.NotEmpty(t, kinds)
	for kind := range kinds {
		assert.NotEqual(t, kind, fleetui.TaskKindWords(kind), "task kind %q has no words", kind)
	}
}
