// Detail: dispatch.go -- helpAsked, the help question a file-reading
// subcommand asks before it opens its words as MRT files.

package analyze

import (
	"os"
	"strings"
	"testing"
)

// TestEverySubcommandAnswersAHelpWordWithItsUsage drives `<subcommand> --help`
// through the dispatcher for every registered subcommand, so a new one joins
// the check by registering.
//
// VALIDATES: each subcommand prints its own usage for a help word, and a
// subcommand that reads its words as file names answers 0 for it.
// PREVENTS: `le mrt statistics --help` opening a file named `--help`, and
// `density` or `count-attrs` printing an empty analysis in place of help.
func TestEverySubcommandAnswersAHelpWordWithItsUsage(t *testing.T) {
	readsFiles := map[string]bool{
		"aspath": true, "attributes": true, "count-attrs": true, "density": true,
		"mrt-dump": true, "routes": true, "show": true, "statistics": true,
	}
	targets := Targets()
	if len(targets) == 0 {
		t.Fatal("analyze registered no subcommand")
	}
	for _, target := range targets {
		code := -1
		var stdout string
		stderr := captureStd(t, &os.Stderr, func() {
			stdout = captureStd(t, &os.Stdout, func() { code = Dispatch([]string{target.Name, "--help"}) })
		})
		text := stdout + stderr
		if strings.Contains(text, "open --help") {
			t.Errorf("%s --help opened a file named --help: %q", target.Name, text)
		}
		if !strings.Contains(text, target.Name) {
			t.Errorf("%s --help printed no usage naming it: %q", target.Name, text)
		}
		if readsFiles[target.Name] && code != 0 {
			t.Errorf("%s --help answered %d, want 0", target.Name, code)
		}
	}
	for name := range readsFiles {
		if !strings.Contains(Subcommands(), name) {
			t.Errorf("readsFiles names %q, which analyze does not register", name)
		}
	}
}
