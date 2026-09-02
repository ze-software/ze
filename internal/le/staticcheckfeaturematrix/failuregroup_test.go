// Related: report.go — Verdict.Text, Verdict.failingPaths
//
// VALIDATES: a failing matrix run says what broke and which files it was about,
// in the answer the verify engine reads its groups from.
// PREVENTS: an unattributable structural red. A group carrying no usable paths
// is charged to EVERY commit in the checkout (internal/le/commit/verification.go,
// structuralGateReds), and a run that failed before producing a diagnostic used
// to render nothing at all, so its stage log named neither the cause nor a file.
package staticcheckfeaturematrix

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// TestAFailingRunNamesTheFilesItsDiagnosticsCite is the case that unblocks other
// sessions, and the one that fails when Text goes back to rendering nothing.
func TestAFailingRunNamesTheFilesItsDiagnosticsCite(t *testing.T) {
	v := Verdict{
		Part: 3, Parts: 6, Rows: 2, Names: []string{"all_features", "without_ze_ssh"},
		Diagnostics: []string{
			"internal/component/bgp/reactor/peer.go:120:2: this value of err is never used (SA4006)",
			"internal/core/family/family.go:12:1: unreachable code (SA4006)",
			"internal/component/bgp/reactor/peer.go:340:9: another finding in the same file",
		},
	}

	text := v.Text()
	group := soleGroup(t, text)

	want := []string{"internal/component/bgp/reactor/peer.go", "internal/core/family/family.go"}
	if !slices.Equal(group.Related, want) {
		t.Fatalf("related = %v, want %v: each file once, sorted", group.Related, want)
	}
	if group.Kind != "files" {
		t.Errorf("kind = %q, want \"files\": the commit side reads paths only from "+
			"a kind it recognizes", group.Kind)
	}
	for _, line := range v.Diagnostics {
		if !strings.Contains(text, line) {
			t.Errorf("the rendered answer dropped a diagnostic: %s", line)
		}
	}
}

// TestARunThatDiedBeforeDiagnosingStillSaysWhy is the case that was silent. A
// broken vendor tree fails every row before staticcheck emits one finding, and
// the stage log held nothing at all.
func TestARunThatDiedBeforeDiagnosingStillSaysWhy(t *testing.T) {
	v := Verdict{Part: 1, Parts: 6, Rows: 2, Names: []string{"all_features", "without_ze_ssh"}, Tool: "go: inconsistent vendoring in /repo:\n\tgithub.com/x/y@v1: is explicitly required in go.mod, but not marked as explicit in vendor/modules.txt\n"}

	text := v.Text()

	if !strings.Contains(text, "inconsistent vendoring") {
		t.Fatalf("a run that produced no diagnostic said nothing about why it failed:\n%q", text)
	}
	group := soleGroup(t, text)
	if len(group.Related) != 0 {
		t.Fatalf("a failure naming no source file answered %v", group.Related)
	}
	if group.Kind == "files" {
		t.Errorf("kind = %q: a group with no paths must not claim a path-bearing kind", group.Kind)
	}
}

// TestAPassingRunDeclaresNoGroup keeps the emitter honest. A group announces a
// failure, so a green run that declared one would charge a red to the commits
// whose files it named.
func TestAPassingRunDeclaresNoGroup(t *testing.T) {
	text := Verdict{Part: 1, Parts: 1, Rows: 2, Names: []string{"all_features", "core_only"}, Passed: true}.Text()

	if strings.Contains(text, "VERIFY FAILURE GROUP:") {
		t.Fatalf("a green run declared a failure group:\n%s", text)
	}
	if !strings.Contains(text, "checked 2 row(s): all_features, core_only") {
		t.Fatalf("a green run stopped reporting the rows it judged:\n%s", text)
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
