package verify

import (
	"strings"
	"testing"

	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// TestVerifyAreaDescribesTheNamespace checks AC-13 under D-P7-short-long: the
// short help `le verify` registered names the change set beside the whole
// tree, and the long description names every job the namespace holds, the
// scoped change-set check beside the release checks and the reds, rather than
// only `verify worktree`'s job. Method: read both texts back from the registry
// le renders its pages from.
func TestVerifyAreaDescribesTheNamespace(t *testing.T) {
	var short, long string
	for _, command := range leroot.Commands() {
		if command.Name == area {
			short = command.Meta.ShortHelp
			long = command.Meta.Description
		}
	}
	if short == "" {
		t.Fatal("le verify registered no short help")
	}
	if long == "" {
		t.Fatal("le verify registered no description")
	}
	if strings.HasPrefix(long, "the full native verification population") {
		t.Errorf("the description still describes verify worktree alone: %q", long)
	}
	if !strings.Contains(short, "change set") {
		t.Errorf("the short help %q does not name the change set", short)
	}
	for _, job := range []string{"change set", "release checks", "reds"} {
		if !strings.Contains(long, job) {
			t.Errorf("the description %q does not name the namespace's %s", long, job)
		}
	}
}
