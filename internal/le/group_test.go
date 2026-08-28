// VALIDATES: every le command declares a group help renders it under, and the
// commands the pre-commit gate runs never claim to be a mere convenience.
// PREVENTS: a tool that registers into no section of the help page, and a gate
// stage filed under Workflow or Reports where a reader would not look for it.
package le

import (
	"testing"

	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/verify"
)

func TestEveryToolDeclaresARenderedGroup(t *testing.T) {
	if len(commandsAtStart) == 0 {
		t.Fatal("le registered no local-data command")
	}
	populated := make(map[leroot.Group]int, len(leroot.Groups()))
	for _, tool := range commandsAtStart {
		group, ok := leroot.GroupOf(tool.Name)
		if !ok {
			t.Errorf("tool %q declared no group, so help prints it under Ungrouped", tool.Name)
			continue
		}
		if !leroot.KnownGroup(group) {
			t.Errorf("tool %q declared group %q, which help has no title for", tool.Name, group)
			continue
		}
		populated[group]++
	}
	for _, group := range leroot.Groups() {
		if populated[group] == 0 {
			t.Errorf("group %q holds no command, so help renders an order nothing uses", group)
		}
	}
}

// TestGateStagesAreNotWorkflowOrReport ties the help taxonomy to the pre-commit
// population. A command the gate runs judges the tree, rewrites an artifact the
// gate compares, or runs a suite. It is never something a person merely types to
// move their own work along, and it never gates nothing.
func TestGateStagesAreNotWorkflowOrReport(t *testing.T) {
	stages := verify.StagesForMode(verify.Mode)
	if len(stages) == 0 {
		t.Fatal("full verification declares no stage")
	}
	for _, stage := range stages {
		name := stage.Identity.Command
		group, ok := leroot.GroupOf(name)
		if !ok {
			t.Errorf("stage %q names a command that declared no group", name)
			continue
		}
		switch group {
		case leroot.GroupGate, leroot.GroupGenerate, leroot.GroupSuite:
		default:
			t.Errorf("stage %q is declared %q; a gate stage is a gate, a generator, or a suite", name, group)
		}
	}
}
