package repo

import (
	"strings"
	"testing"

	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// TestEveryGenerationStepNamesAnActionItsAreaAccepts asks, of every row of
// both generation tables, whether the area it names is registered and whether
// that area's own action table takes the row's words. Method: the action
// table each area publishes (leroot.ActionsOf) judges the line with
// List.AcceptsLine, the check dispatch makes, so a row naming a verb the
// area never declared ("web vendor sync", when the action is update) is
// refused here rather than by an exit 1 inside `./le repo update`.
func TestEveryGenerationStepNamesAnActionItsAreaAccepts(t *testing.T) {
	for _, table := range [][]generationAction{generationActions, generationChecks} {
		for _, step := range table {
			actions, declared := leroot.ActionsOf(step.area)
			if !declared {
				t.Errorf("generation step %q %q: no le area registers under %q", step.area, step.verb, step.area)
				continue
			}
			if err := actions.AcceptsLine(strings.Fields(step.verb)); err != nil {
				t.Errorf("generation step %q %q: the area refuses the line: %v", step.area, step.verb, err)
			}
		}
	}
}
