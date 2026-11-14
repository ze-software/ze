// Related: tool.go -- HoldTool, the slot `le exec` holds for a heavy tool
// Related: central_test.go -- the fixtures: centralOver, entriesOf, loglessEntry
//
// VALIDATES: spec-native-action-job-admission decision 3 and C-1: a heavy tool
// `le exec` carries runs as le's child inside one slot labeled by `exec` and
// the tool's name, names that slot as the tool's parent, and answers the
// tool's code; inside a live parent's slot, or with no checkout, it holds
// nothing and the caller replaces its process.
// PREVENTS: a shimmed linter that waits for a second slot behind the `le go
// lint run` that started it, and a heavy tool typed through `le exec` that
// runs with no slot.
//
// No case starts a process: the child is a function the case passes, and a
// live holder is a fabricated entry for this test's own PID.

package job

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// toolLine is the le line the cases hold a slot for, and toolCommand the
// command name exec derives from the tool it carries.
var toolLine = []string{"./le", "exec", "go", "test", "./x/"}

const toolCommand = "exec go test"

// TestHoldToolRunsTheToolInsideOneSlot asks whether a heavy tool with no
// parent runs inside one slot of its own, sees that slot named as its parent,
// and answers its code. Its method: the point over a fixture tree, a child
// function that reads the registry and the environment it was handed, and the
// registry read again once the hold has answered.
func TestHoldToolRunsTheToolInsideOneSlot(t *testing.T) {
	detach(t)
	root := t.TempDir()
	var errOut bytes.Buffer
	point, _ := centralOver(t, root, &errOut)

	tool := Tool{Path: "/usr/bin/go", Argv: []string{"go", "test", "./x/"}, Environ: []string{"HOME=/h", "ze.run.job=/stale"}}
	var held []string
	var handed Tool
	code, answered := point.holdTool(toolCommand, toolLine, tool, func(child Tool) int {
		held = entriesOf(t, root)
		handed = child
		return 7
	})
	if !answered || code != 7 {
		t.Fatalf("the hold answered %d, %v; want the tool's 7 with the line answered", code, answered)
	}
	if len(held) != 1 || !strings.Contains(held[0], "exec-go-test") {
		t.Fatalf("the tool ran with the entries %v, want one entry labeled exec-go-test", held)
	}
	want := ParentVariable + "=" + held[0]
	if !slices.Contains(handed.Environ, want) {
		t.Errorf("the tool's environment is %v, want it to name its slot as %s", handed.Environ, want)
	}
	if slices.Contains(handed.Environ, "ze.run.job=/stale") {
		t.Errorf("the tool's environment kept a stale parent spelling: %v", handed.Environ)
	}
	if handed.Path != tool.Path || !slices.Equal(handed.Argv, tool.Argv) {
		t.Errorf("the tool ran as %q %v, want %q %v", handed.Path, handed.Argv, tool.Path, tool.Argv)
	}
	if left := entriesOf(t, root); len(left) != 0 {
		t.Errorf("the hold left the entries %v, want its slot released", left)
	}
}

// TestHoldToolHoldsNothingInsideAParent asks whether a heavy tool started
// inside a live parent's slot holds nothing and leaves the process to the
// caller, which replaces it. Its method: a fabricated entry for this
// process's own PID named as the parent, then a hold whose child records that
// it ran.
func TestHoldToolHoldsNothingInsideAParent(t *testing.T) {
	detach(t)
	root := t.TempDir()
	var errOut bytes.Buffer
	point, _ := centralOver(t, root, &errOut)

	parent := loglessEntry(t, root, "go-lint", os.Getpid())
	if err := env.Set(ParentKey, parent); err != nil {
		t.Fatalf("name the parent: %v", err)
	}
	t.Cleanup(func() {
		if err := env.Set(ParentKey, ""); err != nil {
			t.Errorf("clear %s: %v", ParentKey, err)
		}
	})

	ran := false
	_, answered := point.holdTool(toolCommand, toolLine, Tool{Path: "/x"}, func(Tool) int {
		ran = true
		return 0
	})
	if answered || ran {
		t.Errorf("inside a live parent the hold answered %v and ran the tool %v, want neither: the caller replaces its process", answered, ran)
	}
	if entries := entriesOf(t, root); len(entries) != 1 {
		t.Errorf("inside a live parent the registry holds %v, want the parent's entry alone", entries)
	}
}

// TestHoldToolHoldsNothingWithNoCheckout asks whether a heavy tool run where
// no checkout exists reads no policy and leaves the process to the caller.
func TestHoldToolHoldsNothingWithNoCheckout(t *testing.T) {
	var errOut bytes.Buffer
	point := centralPoint{
		root: func() (string, error) { return "", lepath.ErrNoCheckout },
		policy: func(string) (*Admission, error) {
			t.Error("the admission policy was read with no checkout to hold a registry")
			return nil, lepath.ErrNoCheckout
		},
		errOut: &errOut,
	}
	ran := false
	_, answered := point.holdTool(toolCommand, toolLine, Tool{Path: "/x"}, func(Tool) int {
		ran = true
		return 0
	})
	if answered || ran {
		t.Errorf("with no checkout the hold answered %v and ran the tool %v, want neither", answered, ran)
	}
	if errOut.Len() != 0 {
		t.Errorf("with no checkout the hold wrote %q, want nothing", errOut.String())
	}
}
