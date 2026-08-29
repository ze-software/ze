// Related: failuregroup.go — pathCollector, declareLintFailureGroup
// Related: verifylint.go — streamCommand, the pass loop that tees into the collector
//
// VALIDATES: a lint red names the files whose findings caused it, in a failure
// group the verify engine can read back out of the stage log.
// PREVENTS: an unattributable lint red. internal/le/commit/verification.go's
// structuralGateReds charges a failure group carrying no usable paths to EVERY
// commit in the checkout, so one session's lint finding refuses every other
// session's commit however unrelated the change.
package verifylint

import (
	"context"
	"encoding/json"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/leaction"
)

// findingLines is one golangci-lint run's output, two findings in one file and
// one in another, mixed with the lines a run prints around them.
const findingLines = "" +
	"internal/le/verify/lint/verifylint.go:41:2: field is unused (unused)\n" +
	"level=info golangci-lint has version 2.1.0\n" +
	"internal/le/verify/lint/verifylint.go:88:6: comment should end in a period (godot)\n" +
	"internal/core/family/family.go:12:1: exported type is missing a comment (revive)\n"

// TestALintRedNamesTheFilesItFailedAbout is the case that unblocks other
// sessions, and the one that fails when the pass loop goes back to passing no
// watcher into the child.
func TestALintRedNamesTheFilesItFailedAbout(t *testing.T) {
	root, tracked := lintFixture(t)
	ops, _ := fixtureOps(root, tracked)
	ops.stream = func(_ context.Context, _ []string, _ string, _ []string, watch io.Writer) (int, error) {
		if watch == nil {
			t.Error("the pass loop gave the child no watcher, so no finding can be attributed")
			return 1, nil
		}
		if _, err := io.WriteString(watch, findingLines); err != nil {
			t.Errorf("write findings: %v", err)
		}
		return 1, nil
	}
	runner, err := newRunner(t.Context(), root, fixtureChain(root), ops)
	if err != nil {
		t.Fatalf("newRunner: %v", err)
	}

	var answer any
	var code int
	captureLintOutput(t, func() { answer, code = runRunner(runner, leaction.Arguments{}) })
	if code == 0 {
		t.Fatal("every child returned 1, so the stage must be red")
	}
	report, ok := answer.(Report)
	if !ok {
		t.Fatalf("the action answered %T, want a Report", answer)
	}

	// Text, not the captured stream. The engine reads a stage's groups out of the
	// log, and the log holds what the action RETURNED.
	group := soleFailureGroup(t, report.Text())
	want := []string{"internal/core/family/family.go", "internal/le/verify/lint/verifylint.go"}
	if !slices.Equal(group.Related, want) {
		t.Fatalf("group related = %v, want %v: each file once, sorted, whatever order "+
			"the findings arrived in", group.Related, want)
	}
	if group.Kind != "lint" {
		t.Errorf("group kind = %q, want \"lint\": the commit side only reads paths "+
			"from a group whose kind it recognizes", group.Kind)
	}
}

// TestAGreenLintDeclaresNoGroup keeps the emitter honest. A group announces a
// failure, so a passing stage that declared one would charge a red to the
// commits whose files it named.
func TestAGreenLintDeclaresNoGroup(t *testing.T) {
	root, tracked := lintFixture(t)
	ops, _ := fixtureOps(root, tracked)
	ops.stream = func(_ context.Context, _ []string, _ string, _ []string, watch io.Writer) (int, error) {
		_, _ = io.WriteString(watch, findingLines)
		return 0, nil
	}
	runner, err := newRunner(t.Context(), root, fixtureChain(root), ops)
	if err != nil {
		t.Fatalf("newRunner: %v", err)
	}

	var answer any
	stdout, stderr := captureLintOutput(t, func() { answer, _ = runRunner(runner, leaction.Arguments{}) })

	report, ok := answer.(Report)
	if !ok {
		t.Fatalf("the action answered %T, want a Report", answer)
	}
	for name, text := range map[string]string{
		"the returned text": report.Text(), "stdout": stdout, "stderr": stderr,
	} {
		if strings.Contains(text, "VERIFY FAILURE GROUP:") {
			t.Fatalf("a green lint declared a failure group on %s:\n%s", name, text)
		}
	}
}

// TestAFindingSplitAcrossTwoWritesIsStillSeen covers the pipe boundary. A child
// writes when its buffer fills, not when a line ends, so a path that straddles
// two chunks must not be lost.
func TestAFindingSplitAcrossTwoWritesIsStillSeen(t *testing.T) {
	collector := newPathCollector()

	cut := len("internal/le/verify/lint/verify")
	if _, err := collector.Write([]byte(findingLines[:cut])); err != nil {
		t.Fatalf("write head: %v", err)
	}
	if _, err := collector.Write([]byte(findingLines[cut:])); err != nil {
		t.Fatalf("write tail: %v", err)
	}

	want := []string{"internal/core/family/family.go", "internal/le/verify/lint/verifylint.go"}
	if got := collector.paths(); !slices.Equal(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
}

// TestOutputWithNoFindingsNamesNothing stops the scanner inventing an
// attribution from a run that names no file, such as a configuration error.
func TestOutputWithNoFindingsNamesNothing(t *testing.T) {
	collector := newPathCollector()
	if _, err := collector.Write([]byte("level=error cannot load config: no such file\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := collector.paths(); len(got) != 0 {
		t.Fatalf("paths = %v, want none", got)
	}
}

// TestAnUnattributableRedStaysUnattributable is the honest fallback. A red the
// scanner cannot place must not be given paths it did not read, so it keeps
// being charged to everybody rather than to a wrong commit.
func TestAnUnattributableRedStaysUnattributable(t *testing.T) {
	var out strings.Builder
	if err := declareLintFailureGroup(&out, nil); err != nil {
		t.Fatalf("declare: %v", err)
	}

	group := soleFailureGroup(t, out.String())
	if len(group.Related) != 0 {
		t.Fatalf("a red naming no file answered %v", group.Related)
	}
	if group.Kind == "lint" {
		t.Errorf("kind = %q: a group with no paths must not claim a kind whose paths "+
			"the commit side would then read as an empty attribution", group.Kind)
	}
}

// failureGroup is the part of the declared line these tests assert on.
type failureGroup struct {
	Kind    string   `json:"kind"`
	Related []string `json:"related"`
}

// soleFailureGroup reads back the one group the stage declared, and fails when
// the run declared none or more than one.
func soleFailureGroup(t *testing.T, text string) failureGroup {
	t.Helper()

	var found []failureGroup
	for line := range strings.SplitSeq(text, "\n") {
		payload, ok := strings.CutPrefix(line, "VERIFY FAILURE GROUP: ")
		if !ok {
			continue
		}
		var group failureGroup
		if err := json.Unmarshal([]byte(payload), &group); err != nil {
			t.Fatalf("group line is not JSON: %v\n%s", err, line)
		}
		found = append(found, group)
	}
	if len(found) != 1 {
		t.Fatalf("declared %d failure groups, want 1:\n%s", len(found), text)
	}
	if !strings.Contains(text, "VERIFY FAILURE GROUPS COMPLETE: 1") {
		t.Fatalf("the group list was never closed, so the engine reads none of it:\n%s", text)
	}

	return found[0]
}
