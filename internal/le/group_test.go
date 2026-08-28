// VALIDATES: le's own shape -- every command declares a group help renders it
// under, each group's promise holds, and a command's package sits at the path
// its name predicts.
// PREVENTS: a tool that registers into no section of the help page. A gate
// stage filed where a reader would not look for it. A generator that cannot
// regenerate. A report that writes. A package a reader cannot find from the
// command name.
package le

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/verifyengine"
)

// writesMarker is what an area's Subs line prints beside an action that changes
// the tree. leaction.Area.Subs writes it, and an area with a private action
// table renders the same word, so reading the registry covers both.
const writesMarker = "(writes)"

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
	stages := verifyengine.StagesForMode(verifyengine.Mode)
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

// TestGenerateAreasCanRewriteWhatTheyCheck holds the promise the section title
// makes. "Generated artifacts (check one, or rewrite it)" is a lie for an area
// that offers no rewrite, and a reader who reached that section for the repair
// verb would find none.
func TestGenerateAreasCanRewriteWhatTheyCheck(t *testing.T) {
	for _, tool := range commandsAtStart {
		if group, _ := leroot.GroupOf(tool.Name); group != leroot.GroupGenerate {
			continue
		}
		if !strings.Contains(tool.Meta.ResolveSubs(), writesMarker) {
			t.Errorf("%q is filed under generated artifacts and declares no writing action", tool.Name)
		}
	}
}

// TestReportAreasWriteNothing holds the other title's promise. "Reports (read
// the tree, gate nothing)" must not hide a verb that changes the tree.
func TestReportAreasWriteNothing(t *testing.T) {
	for _, tool := range commandsAtStart {
		if group, _ := leroot.GroupOf(tool.Name); group != leroot.GroupReport {
			continue
		}
		if strings.Contains(tool.Meta.ResolveSubs(), writesMarker) {
			t.Errorf("%q is filed under reports and declares a writing action", tool.Name)
		}
	}
}

// TestEveryCommandIsFoundAtThePathItsNamePredicts is the structural rule. It
// holds in both directions. `le spec-session` lives at
// internal/le/specsession, and every directory that registers a command is
// reached by some command name.
//
// The rule is mechanical on purpose. A reader who knows the command knows the
// directory. There is no table to consult. Its cost is that a long command
// name makes a long directory name. The answer to that is to rename the
// command, which is the honest coupling.
func TestEveryCommandIsFoundAtThePathItsNamePredicts(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find checkout: %v", err)
	}
	tools := filepath.Join(root, "internal", "le")

	claimed := make(map[string]string, len(commandsAtStart))
	for _, tool := range commandsAtStart {
		directory := strings.ReplaceAll(tool.Name, "-", "")
		claimed[directory] = tool.Name
		if _, statErr := os.Stat(filepath.Join(tools, directory, "register.go")); statErr != nil {
			t.Errorf("command %q predicts internal/le/%s, which registers nothing: %v",
				tool.Name, directory, statErr)
		}
	}

	entries, err := os.ReadDir(tools)
	if err != nil {
		t.Fatalf("read internal/le: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(tools, entry.Name(), "register.go")); statErr != nil {
			continue // A library package registers nothing and names nothing.
		}
		if _, ok := claimed[entry.Name()]; !ok {
			t.Errorf("internal/le/%s registers a command that no command name predicts", entry.Name())
		}
	}
}
