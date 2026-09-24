// VALIDATES: every row of the rename map names a command le really composes,
// and that command's package sits at the directory its new name predicts
// (AC-1, AC-2 and AC-6 of plan/spec-le-subject-first-command-tree.md).
// PREVENTS: a family that moved its directory but kept its old registered
// name, or registered the new name from a directory no reader would open.
//
// Rows fail BY NAME until Phase 1b moves their family, so the red list is the
// list of families still to move.
package le

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
)

// TestEveryNewNameResolvesToItsArea reads the rename map in leroot, the one
// declaration of old and new names, and resolves each new command three
// ways: through the dispatcher's lookup, in the manifest `./le '|' json`
// prints, and at the package directory directoryFor predicts.
func TestEveryNewNameResolvesToItsArea(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find checkout: %v", err)
	}
	listed := make(map[string]bool, len(commandsAtStart))
	for _, command := range commandsAtStart {
		listed[command.Name] = true
	}

	for _, row := range leroot.Renames() {
		t.Run(strings.Join(row.Old(), " "), func(t *testing.T) {
			if leroot.LookupCommand(row.Command) == nil {
				t.Fatalf("new command %q is not registered", row.Command)
			}
			if !listed[row.Command] {
				t.Errorf("new command %q resolves but is not in the manifest", row.Command)
			}
			directory := directoryFor(row.Command)
			if _, statErr := os.Stat(filepath.Join(root, "internal", "le", directory, "register.go")); statErr != nil {
				t.Errorf("new command %q predicts internal/le/%s, which registers nothing: %v",
					row.Command, directory, statErr)
			}
		})
	}
}
