// Related: report.go — Report.Text, Report.failingPaths
//
// VALIDATES: a tracked-build red names the source files the compiler named, in
// the answer the verify engine reads its groups from.
// PREVENTS: an unattributable structural red. A group carrying no usable paths
// is charged to EVERY commit in the checkout (internal/le/commit/verification.go,
// structuralGateReds), so a consumer committed without its producer refuses
// every other session's commit rather than the one that broke the tree.
package repositorytrackedbuild

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// compilerOutput is what one failing flavor answers: the package header, then
// the positions the toolchain named.
const compilerOutput = "" +
	"# github.com/ze-software/ze/internal/test/fixture\n" +
	"internal/test/fixture/misc_fixture_runner.go:62:35: undefined: argInit\n" +
	"internal/test/fixture/misc_fixture_runner.go:99:3: undefined: fileGoMod\n" +
	"internal/test/fixture/install_fixture.go:12:2: undefined: argConfig\n"

// TestAFailingRunNamesTheFilesTheCompilerNamed is the case that unblocks other
// sessions, and the one that fails when Text goes back to declaring nothing.
func TestAFailingRunNamesTheFilesTheCompilerNamed(t *testing.T) {
	report := Report{
		Rev: "HEAD", Commit: "2937d0d1c155",
		Results: []Result{
			{Name: "distro", OK: false, Output: compilerOutput},
			{Name: "setup", OK: false, Output: compilerOutput},
			{Name: "installer", OK: true},
		},
	}

	group := soleGroup(t, report.Text())

	want := []string{
		"internal/test/fixture/install_fixture.go",
		"internal/test/fixture/misc_fixture_runner.go",
	}
	if !slices.Equal(group.Related, want) {
		t.Fatalf("related = %v, want %v: each file once across every failing flavor, "+
			"sorted. With no paths the group is unattributable, and the commit gate "+
			"charges it to every commit in the checkout", group.Related, want)
	}
	if group.Kind != "files" {
		t.Errorf("kind = %q, want \"files\": the commit side reads paths only from "+
			"a kind it recognizes", group.Kind)
	}
}

// TestAPassingRunDeclaresNoGroup keeps the emitter honest. A group announces a
// failure, so a green run that declared one would charge a red to the commits
// whose files it named.
func TestAPassingRunDeclaresNoGroup(t *testing.T) {
	report := Report{
		Rev: "HEAD", Commit: "2937d0d1c155", OK: true,
		Results: []Result{{Name: "distro", OK: true, Packages: 900}},
	}

	if text := report.Text(); strings.Contains(text, "VERIFY FAILURE GROUP:") {
		t.Fatalf("a green run declared a failure group:\n%s", text)
	}
}

// TestAFailureThatNamesNoFileStaysUnattributable is the honest fallback: a
// flavor that died before compiling anything must not be given an attribution.
func TestAFailureThatNamesNoFileStaysUnattributable(t *testing.T) {
	report := Report{
		Rev: "HEAD", Commit: "2937d0d1c155",
		Results: []Result{{Name: "distro", OK: false, Output: "go: cannot find module\n"}},
	}

	group := soleGroup(t, report.Text())
	if len(group.Related) != 0 {
		t.Fatalf("a failure naming no file answered %v", group.Related)
	}
	if group.Kind == "files" {
		t.Errorf("kind = %q: a group with no paths must not claim a path-bearing kind", group.Kind)
	}
}

type parsedGroup struct {
	Kind    string   `json:"kind"`
	Related []string `json:"related"`
}

func soleGroup(t *testing.T, text string) parsedGroup {
	t.Helper()

	var found []parsedGroup
	for line := range strings.SplitSeq(text, "\n") {
		payload, ok := strings.CutPrefix(line, "VERIFY FAILURE GROUP: ")
		if !ok {
			continue
		}
		var group parsedGroup
		if err := json.Unmarshal([]byte(payload), &group); err != nil {
			t.Fatalf("group line is not JSON: %v\n%s", err, line)
		}
		found = append(found, group)
	}
	if len(found) != 1 {
		t.Fatalf("declared %d groups, want 1:\n%s", len(found), text)
	}
	if !strings.Contains(text, "VERIFY FAILURE GROUPS COMPLETE: 1") {
		t.Fatalf("the list was never closed, so the engine reads none of it:\n%s", text)
	}

	return found[0]
}
