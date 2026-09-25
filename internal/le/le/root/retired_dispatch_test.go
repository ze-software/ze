// VALIDATES: a retired name runs as the command that replaced it. It writes
// one stderr line naming the new words, then answers the same payload, pipe
// rendering and exit code as the new name (AC-3). A verb split and a merge are
// one mechanism (AC-4, AC-5). A retired name whose new command is not
// registered yet still runs its own handler, silently.
// PREVENTS: an alias that owns an exit code or a rendering of its own. A
// shorter row winning over the row written for one action. A peer session
// calling an old name in Phase 1 before its family moved, and getting
// `unknown command`.
package leroot

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// withRenames swaps the rename map for rows of the test's own, and restores
// it when the test ends. The probes are registered under names no area uses.
func withRenames(t *testing.T, rows ...Rename) {
	t.Helper()
	saved := renames
	renames = append(slices.Clone(renames), rows...)
	t.Cleanup(func() { renames = saved })
}

// registerVerdict registers a command answering its arguments with a code of
// its own choice, so a test can tell whose code reached the caller.
func registerVerdict(t *testing.T, name string, code int) {
	t.Helper()
	Register(name, GroupGate, func(args []string) (any, int) {
		return map[string]any{"command": name, "args": args}, code
	}, probeMeta("a rename probe"))
	RegisterShape(name, command.ShapeMap)
}

// dispatchStreams runs Dispatch and answers what it wrote to each stream and
// the code it returned.
func dispatchStreams(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	stderr = captureStderr(t, func() {
		stdout = captureStdout(t, func() { code = Dispatch("le", args) })
	})
	return stdout, stderr, code
}

func TestRetiredNameAnswersAsItsNewCommand(t *testing.T) {
	registerVerdict(t, "probe-renamed new", 3)
	withRenames(t,
		Rename{Retired: "probe-retired-split", RetiredAction: "go", Command: "probe-renamed new", Action: "run"},
		Rename{Retired: "probe-retired-merge", Command: "probe-renamed new"},
	)

	newOut, newErr, newCode := dispatchStreams(t, "probe-renamed", "new", "run", "x", "|", "json")
	if newCode != 3 {
		t.Fatalf("the new command answered %d, want its own 3", newCode)
	}
	if !strings.Contains(newOut, `"run"`) {
		t.Fatalf("the new command did not receive its action through the json operator: %q", newOut)
	}

	cases := []struct {
		name string
		old  []string
		line string
	}{
		{
			name: "verb split",
			old:  []string{"probe-retired-split", "go", "x", "|", "json"},
			line: "warning: le probe-retired-split go is renamed: run le probe-renamed new run\n",
		},
		{
			name: "merge",
			old:  []string{"probe-retired-merge", "run", "x", "|", "json"},
			line: "warning: le probe-retired-merge is renamed: run le probe-renamed new\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			oldOut, oldErr, oldCode := dispatchStreams(t, tc.old...)
			if oldCode != newCode {
				t.Errorf("the retired name answered %d, want the new command's %d", oldCode, newCode)
			}
			if oldOut != newOut {
				t.Errorf("the retired name rendered\n%q\nwant the new command's\n%q", oldOut, newOut)
			}
			if oldErr != tc.line+newErr {
				t.Errorf("the retired name wrote to stderr\n%q\nwant one line then the new command's\n%q", oldErr, tc.line+newErr)
			}
		})
	}
}

func TestLongestRetiredRowWins(t *testing.T) {
	registerVerdict(t, "probe-longest whole", 0)
	registerVerdict(t, "probe-longest verb", 0)
	withRenames(t,
		Rename{Retired: "probe-longest-old", Command: "probe-longest whole"},
		Rename{Retired: "probe-longest-old", RetiredAction: "verb", Command: "probe-longest verb", Action: "done"},
	)

	rewritten, row, ok := retiredRewrite([]string{"probe-longest-old", "verb", "tail"})
	if !ok {
		t.Fatal("a retired name with a registered new command was not rewritten")
	}
	if row.Command != "probe-longest verb" {
		t.Errorf("the row for the whole command won over the row for its action: %+v", row)
	}
	if want := []string{"probe-longest", "verb", "done", "tail"}; !slices.Equal(rewritten, want) {
		t.Errorf("rewritten to %v, want %v", rewritten, want)
	}
}

func TestRetiredNameRunsItsOwnHandlerUntilItsFamilyMoves(t *testing.T) {
	registerVerdict(t, "probe-not-moved", 4)
	withRenames(t, Rename{Retired: "probe-not-moved", Command: "probe-moved-nowhere"})

	_, stderr, code := dispatchStreams(t, "probe-not-moved")
	if code != 4 {
		t.Errorf("the old name answered %d, want its own handler's 4", code)
	}
	if strings.Contains(stderr, "renamed") {
		t.Errorf("an old name whose new command is not registered announced a rename: %q", stderr)
	}
}
