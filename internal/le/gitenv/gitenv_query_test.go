package gitenv

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// TestReadOnlyQueryTurnsOffAutomaticMaintenance proves the environment
// ReadOnlyQuery answers is what git reads: maintenance.auto and gc.auto come
// back off from `git config`, the optional lock is off, and a caller's own
// config pairs keep their numbers.
// Method: a real `git config --get` in an isolated fixture repository under
// the answered environment, plus the pair numbering over a base that already
// declares one pair.
// VALIDATES: ReadOnlyQuery's two settings reach git, numbered after the
// caller's pairs, with the new count last.
// PREVENTS: a hook query that starts a detached maintenance run writing under
// .git, or that drops a caller's GIT_CONFIG_KEY_0.
func TestReadOnlyQueryTurnsOffAutomaticMaintenance(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	tree := statDirtyTree(t)
	env := ReadOnlyQuery(isolatedEnvironment(t))
	for key, want := range map[string]string{"maintenance.auto": "false", "gc.auto": "0"} {
		command := exec.CommandContext(t.Context(), "git", "config", "--get", key)
		command.Dir = tree
		command.Env = env
		output, err := command.Output()
		if err != nil {
			t.Fatalf("git config --get %s: %v", key, err)
		}
		if got := strings.TrimSpace(string(output)); got != want {
			t.Errorf("git config --get %s = %q, want %q", key, got, want)
		}
	}
	if !slices.Contains(env, optionalLocksOff) {
		t.Errorf("ReadOnlyQuery lacks %q: %v", optionalLocksOff, env)
	}

	numbered := ReadOnlyQuery([]string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=a.b", "GIT_CONFIG_VALUE_0=1"})
	for _, want := range []string{"GIT_CONFIG_KEY_0=a.b", "GIT_CONFIG_KEY_1=maintenance.auto", "GIT_CONFIG_VALUE_1=false", "GIT_CONFIG_KEY_2=gc.auto", "GIT_CONFIG_VALUE_2=0"} {
		if !slices.Contains(numbered, want) {
			t.Errorf("ReadOnlyQuery lacks %q: %v", want, numbered)
		}
	}
	if last := numbered[len(numbered)-1]; last != "GIT_CONFIG_COUNT=3" {
		t.Errorf("the new count is not the last GIT_CONFIG_COUNT, so exec keeps the old one: %v", numbered)
	}
}
