// Related: failuregroup.go — Paths, Merge, Declare
//
// VALIDATES: a verify stage's output yields the distinct source files its
// diagnostics named, and a declaration the engine can read back.
// PREVENTS: an unattributable structural red. A group carrying no usable paths
// is charged to EVERY commit in the checkout (../../commit/verification.go,
// structuralGateReds), so one session's red refuses every other session's
// commit however unrelated the change.
package failuregroup

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// TestBothToolchainDiagnosticShapesAreRead is the case that matters. The Go
// compiler and golangci-lint print different position prefixes, and the two
// stages that declare groups produce one each.
func TestBothToolchainDiagnosticShapesAreRead(t *testing.T) {
	text := "" +
		"# github.com/ze-software/ze/internal/test/fixture\n" +
		"    internal/test/fixture/misc_fixture_runner.go:62:35: undefined: argInit\n" +
		"    internal/test/fixture/misc_fixture_runner.go:99:3: undefined: fileGoMod\n" +
		"internal/le/tier/tier.go:40:1: File is not properly formatted (goimports)\n" +
		"level=info golangci-lint has version 2.1.0\n" +
		"internal/core/family/family.go:12: a position with no column\n"

	want := []string{
		"internal/core/family/family.go",
		"internal/le/tier/tier.go",
		"internal/test/fixture/misc_fixture_runner.go",
	}
	if got := Paths(text); !slices.Equal(got, want) {
		t.Fatalf("Paths = %v, want %v: each file once, sorted, whichever tool named it", got, want)
	}
}

// TestProseThatNamesNoFileYieldsNothing stops the scanner inventing an
// attribution from a failure it cannot place, which would charge a red to a
// commit that did not cause it.
func TestProseThatNamesNoFileYieldsNothing(t *testing.T) {
	for name, text := range map[string]string{
		"a config error": "level=error cannot load config: no such file\n",
		"a passing run":  "tracked-build: OK (every flavor of the committed tree compiles)\n",
		"a bare symbol":  "undefined: argInit\n",
		"nothing at all": "",
	} {
		if got := Paths(text); len(got) != 0 {
			t.Errorf("%s named %v", name, got)
		}
	}
}

// TestMergeKeepsOneEntryPerFile covers the multi-flavor case: six build flavors
// fail on the same file, and the group must name it once.
func TestMergeKeepsOneEntryPerFile(t *testing.T) {
	got := Merge([]string{"b.go"}, []string{"a.go", "b.go", "c.go"})

	if want := []string{"a.go", "b.go", "c.go"}; !slices.Equal(got, want) {
		t.Fatalf("Merge = %v, want %v", got, want)
	}
}

// TestADeclarationCarriesItsPathsAndCloses is what the engine actually parses.
// It accepts a declaration only when the closing count matches the group total.
func TestADeclarationCarriesItsPathsAndCloses(t *testing.T) {
	var out strings.Builder

	if err := Declare(&out, "files:stage", "files", "summary", "./le rerun",
		[]string{"a.go", "b.go"}); err != nil {
		t.Fatalf("Declare: %v", err)
	}

	group := soleGroup(t, out.String())
	if !slices.Equal(group.Related, []string{"a.go", "b.go"}) {
		t.Fatalf("related = %v", group.Related)
	}
	if group.Kind != "files" {
		t.Fatalf("kind = %q: the commit side reads paths only from files, lint, or package", group.Kind)
	}
}

// TestADeclarationWithNoPathsStaysUnattributable is the honest fallback, and the
// discrimination case for the test above. A failure the scanner could not place
// must not be given a path-bearing kind, or an empty list would read as an
// attribution rather than as its absence.
func TestADeclarationWithNoPathsStaysUnattributable(t *testing.T) {
	var out strings.Builder

	if err := Declare(&out, "files:stage", "files", "summary", "./le rerun", nil); err != nil {
		t.Fatalf("Declare: %v", err)
	}

	group := soleGroup(t, out.String())
	if len(group.Related) != 0 {
		t.Fatalf("related = %v, want none", group.Related)
	}
	if group.Kind == "files" {
		t.Fatalf("kind = %q: a group naming no file must not claim a path-bearing kind", group.Kind)
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
