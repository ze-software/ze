// VALIDATES: `le build installer` declares one writing action per installer
// architecture, with the reason each one publishes, and registers its command.
// PREVENTS: an architecture dropped from the table, or a build that no longer
// says it writes, when the old build-artifacts verbs were split.

package buildinstaller

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/le/root"
)

func TestInstallerDeclaresTwoWritingArchitectures(t *testing.T) {
	list := Actions()
	wantVerbs := []string{"amd64", "arm64"}
	wantWhys := []string{
		"the installer initrd PID 1 for amd64, at bin/ze-installer-amd64",
		"the installer initrd PID 1 for arm64, at bin/ze-installer-arm64",
	}
	if len(list.Actions) != len(wantVerbs) {
		t.Fatalf("actions = %d, want %d", len(list.Actions), len(wantVerbs))
	}
	for index, row := range list.Actions {
		if row.Verb != wantVerbs[index] {
			t.Errorf("action %d verb = %q, want %q", index, row.Verb, wantVerbs[index])
		}
		if !row.Writes {
			t.Errorf("action %s is not marked as writing", row.Verb)
		}
		if row.Why != wantWhys[index] {
			t.Errorf("action %s reason = %q, want %q", row.Verb, row.Why, wantWhys[index])
		}
	}
	if !registry.HasLocal(leroot.CommandPath(area)) {
		t.Fatalf("importing buildinstaller did not register %q", area)
	}
}
