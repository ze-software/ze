// VALIDATES: Dispatch holds no alias layer. A word sequence spelled like a
// retired name answers `unknown command` and exit 1, writes no rename line,
// and never reaches the command whose words it resembles (AC-17 of
// spec-le-subject-first-command-tree).
// PREVENTS: a rewrite surviving the Phase 3 removal, which would run an old
// name as a new command and keep callers of the old spelling green.
//
// The real rename map lives in internal/le/doc/check, which imports this
// package, so this internal test cannot read it. retired_test.go drives every
// row of it through Dispatch from an external test package.
package leroot

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

func TestDispatchRewritesNoRetiredName(t *testing.T) {
	reached := false
	Register("probe-renamed new", GroupGate, func(args []string) (any, int) {
		reached = true
		return map[string]any{"args": args}, 0
	}, probeMeta("a rename probe"))
	RegisterShape("probe-renamed new", command.ShapeMap)

	code := 0
	stderr := captureStderr(t, func() {
		captureStdout(t, func() { code = Dispatch("le", []string{"probe-retired-split", "go", "x"}) })
	})

	if code != 1 {
		t.Errorf("a word sequence le does not register answered %d, want 1", code)
	}
	if !strings.Contains(stderr, "unknown command: probe-retired-split") {
		t.Errorf("the refusal does not name the unknown command:\n%s", stderr)
	}
	if strings.Contains(stderr, "is renamed") {
		t.Errorf("Dispatch wrote a rename line, so an alias layer survives:\n%s", stderr)
	}
	if reached {
		t.Error("the unregistered words reached a registered command")
	}
}
