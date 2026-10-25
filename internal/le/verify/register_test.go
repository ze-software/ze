package verify

import (
	"strings"
	"testing"

	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// TestVerifyAreaDescribesTheNamespace checks AC-13: the summary `le verify`
// registered describes what the namespace is for, the scoped change-set check
// beside the release checks, rather than only `verify worktree`'s job. Method:
// read the registered summary back from the registry le renders its pages from.
func TestVerifyAreaDescribesTheNamespace(t *testing.T) {
	var registered string
	for _, command := range leroot.Commands() {
		if command.Name == area {
			registered = command.Meta.ShortHelp
		}
	}
	if registered == "" {
		t.Fatal("le verify registered no summary")
	}
	if strings.HasPrefix(registered, "the full native verification population") {
		t.Errorf("the summary still describes verify worktree alone: %q", registered)
	}
	for _, job := range []string{"change set", "release checks", "reds"} {
		if !strings.Contains(registered, job) {
			t.Errorf("the summary %q does not name the namespace's %s", registered, job)
		}
	}
}
