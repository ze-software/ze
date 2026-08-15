package golden

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// portFixtures is one side of a comparison: a fixture path under the root, and
// the bytes it holds.
type portFixtures map[string]string

// newPortRepo builds a throwaway git repository whose commit holds before and
// whose working tree holds after, and returns its directory and that commit.
//
// It writes a tree with plumbing and a temporary index of its own. Nothing here
// reads the index of the repository under development, and nothing writes it.
func newPortRepo(t *testing.T, root string, before, after portFixtures) (string, string) {
	t.Helper()

	dir := t.TempDir()

	run := func(stdinText string, args ...string) string {
		t.Helper()

		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = dir
		cmd.Stdin = strings.NewReader(stdinText)
		cmd.Env = append(os.Environ(),
			"GIT_INDEX_FILE="+filepath.Join(dir, "port-index"),
			"GIT_AUTHOR_NAME=port", "GIT_AUTHOR_EMAIL=port@example.invalid",
			"GIT_COMMITTER_NAME=port", "GIT_COMMITTER_EMAIL=port@example.invalid",
			"GIT_AUTHOR_DATE=2026-08-15T00:00:00Z", "GIT_COMMITTER_DATE=2026-08-15T00:00:00Z",
		)

		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}

		return strings.TrimSpace(string(out))
	}

	run("", "init", "--quiet")
	writePortFixtures(t, dir, root, before)
	run("", "add", "--all")

	tree := run("", "write-tree")
	commit := run("commit for the port comparison", "commit-tree", tree)

	if err := os.RemoveAll(filepath.Join(dir, root)); err != nil {
		t.Fatalf("clear %s: %v", root, err)
	}

	writePortFixtures(t, dir, root, after)

	return dir, commit
}

// writePortFixtures writes one side of a comparison into dir.
func writePortFixtures(t *testing.T, dir, root string, files portFixtures) {
	t.Helper()

	for name, content := range files {
		path := filepath.Join(dir, root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(path), err)
		}

		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

// findingsFor runs the comparison and fails the test on an error it cannot
// recover from, which is what AssertPortFidelity does with the same result.
func findingsFor(t *testing.T, dir, ref, root string, kind PortKind, restructured map[string]string) []string {
	t.Helper()

	findings, _, err := portFindings(t.Context(), dir, ref, root, kind, restructured)
	if err != nil {
		t.Fatalf("compare %s against %s: %v", root, ref, err)
	}

	return findings
}

// TestPortFidelityNamesTheUnitThatChanged proves the comparison discriminates.
//
// VALIDATES: a unit whose markup changed is reported by name, and a unit that
// only changed its encoding is not.
// PREVENTS: the AC-2 comparison passing over a port that moved what an operator
// reads. It is the whole evidence for AC-2, so a comparison that reports
// everything faithful makes the acceptance criterion vacuous.
func TestPortFidelityNamesTheUnitThatChanged(t *testing.T) {
	root := filepath.Join("testdata", "golden")

	dir, ref := newPortRepo(t, root,
		portFixtures{
			"page/one.html": "<div>\n  <span>established</span>\n</div>",
			"page/two.html": `<a hx-vals='{"leaf":"x"}' title="a &#43; b">P</a>`,
		},
		portFixtures{
			"page/one.html": "<div><span>idle</span></div>",
			"page/two.html": `<a hx-vals="{&#34;leaf&#34;:&#34;x&#34;}" title="a + b">P</a>`,
		})

	findings := findingsFor(t, dir, ref, root, PortMarkup, nil)

	if len(findings) != 1 {
		t.Fatalf("want one finding, got %d: %v", len(findings), findings)
	}

	if !strings.Contains(findings[0], "page/one.html") {
		t.Errorf("the finding does not name the unit that changed: %s", findings[0])
	}
}

// TestPortFidelityRefusesAUnitThatStoppedBeingCaptured pins the cheapest route
// from red to green.
//
// VALIDATES: a fixture that existed at the ref and exists no longer is a
// finding, and naming it as restructured with a reason clears it.
// PREVENTS: a failing unit deleted from the capture instead of repaired. The
// comparison would then pass over a page nobody renders any more.
func TestPortFidelityRefusesAUnitThatStoppedBeingCaptured(t *testing.T) {
	root := filepath.Join("testdata", "golden")

	dir, ref := newPortRepo(t, root,
		portFixtures{"page/one.html": "<div>a</div>", "page/gone.html": "<div>b</div>"},
		portFixtures{"page/one.html": "<div>a</div>"})

	findings := findingsFor(t, dir, ref, root, PortMarkup, nil)

	if len(findings) != 1 || !strings.Contains(findings[0], "page/gone.html") {
		t.Fatalf("want one finding naming page/gone.html, got %v", findings)
	}

	cleared := findingsFor(t, dir, ref, root, PortMarkup,
		map[string]string{"page/gone.html": "the two halves became one component"})

	if len(cleared) != 0 {
		t.Errorf("a restructured unit with a reason is still a finding: %v", cleared)
	}
}

// TestPortFidelityRefusesAStaleExplanation pins the other direction.
//
// VALIDATES: an entry that names a unit which does not differ, or a path
// neither side holds, is a finding of its own.
// PREVENTS: the table of declared differences outliving what it declared. An
// exemption nobody removes is how a comparison stops comparing, one unit at a
// time.
func TestPortFidelityRefusesAStaleExplanation(t *testing.T) {
	root := filepath.Join("testdata", "golden")

	dir, ref := newPortRepo(t, root,
		portFixtures{"page/one.html": "<div>a</div>"},
		portFixtures{"page/one.html": "<div>a</div>"})

	stale := map[string]string{
		"page/one.html":   "AC-5: the value was escaped twice before",
		"page/never.html": "a path neither side holds",
	}

	findings := findingsFor(t, dir, ref, root, PortMarkup, stale)

	if len(findings) != 2 {
		t.Fatalf("want two findings, got %d: %v", len(findings), findings)
	}

	joined := strings.Join(findings, "\n")
	for _, want := range []string{"page/one.html", "page/never.html"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the stale entry for %s is not reported: %s", want, joined)
		}
	}
}

// TestPortFidelityAcceptsADeclaredDifference proves the exemption works.
//
// VALIDATES: a unit whose difference is declared with a reason passes, and the
// same unit passes for no other reason.
// PREVENTS: a deliberate change having nowhere to go but a weakened comparison.
func TestPortFidelityAcceptsADeclaredDifference(t *testing.T) {
	root := filepath.Join("testdata", "golden")

	dir, ref := newPortRepo(t, root,
		portFixtures{"page/one.html": "<div>&amp;#34;x&amp;#34;</div>"},
		portFixtures{"page/one.html": "<div>&#34;x&#34;</div>"})

	if findings := findingsFor(t, dir, ref, root, PortMarkup, nil); len(findings) != 1 {
		t.Fatalf("want the undeclared difference reported, got %v", findings)
	}

	declared := map[string]string{"page/one.html": "AC-5: the value was escaped twice before the port"}
	if findings := findingsFor(t, dir, ref, root, PortMarkup, declared); len(findings) != 0 {
		t.Errorf("a declared difference is still a finding: %v", findings)
	}
}

// TestPortFidelityComparesAResponseHeadExactly pins the split.
//
// VALIDATES: a status line and a header are compared byte for byte, a body that
// declares HTML is compared through normalizeHTML, and a body that declares
// anything else is compared byte for byte.
// PREVENTS: an encoding fold reaching JSON or an event stream, where whitespace
// is content and no engine rewrote it.
func TestPortFidelityComparesAResponseHeadExactly(t *testing.T) {
	root := filepath.Join("testdata", "handler")

	const htmlHead = "status: 200\nheader: Content-Type: text/html; charset=utf-8\n\n"

	const jsonHead = "status: 200\nheader: Content-Type: application/json\n\n"

	dir, ref := newPortRepo(t, root,
		portFixtures{
			"head.txt": htmlHead + "<div>a</div>",
			"html.txt": htmlHead + "<div>\n  <span>a</span>\n</div>",
			"json.txt": jsonHead + "{\n  \"peers\": 1\n}",
		},
		portFixtures{
			"head.txt": "status: 404\nheader: Content-Type: text/html; charset=utf-8\n\n<div>a</div>",
			"html.txt": htmlHead + "<div><span>a</span></div>",
			"json.txt": jsonHead + `{"peers": 1}`,
		})

	findings := findingsFor(t, dir, ref, root, PortResponse, nil)

	if len(findings) != 2 {
		t.Fatalf("want two findings, got %d: %v", len(findings), findings)
	}

	joined := strings.Join(findings, "\n")
	if !strings.Contains(joined, "head.txt status and headers") {
		t.Errorf("the changed status is not reported: %s", joined)
	}

	if !strings.Contains(joined, "json.txt body") {
		t.Errorf("the reformatted JSON body is not reported: %s", joined)
	}
}

// TestPortFidelityRefusesAnEmptyPrePortSide pins the fail-closed direction.
//
// VALIDATES: a ref that holds no fixture under the root is an error, not a
// silent pass over zero units.
// PREVENTS: a wrong REF, or a root renamed since the capture, reading as a
// faithful port. Every check here is over the intersection, so an empty
// pre-port side compares nothing and finds nothing.
func TestPortFidelityRefusesAnEmptyPrePortSide(t *testing.T) {
	root := filepath.Join("testdata", "golden")

	dir, ref := newPortRepo(t, root,
		portFixtures{"page/one.html": "<div>a</div>"},
		portFixtures{"page/one.html": "<div>a</div>"})

	_, _, err := portFindings(t.Context(), dir, ref, filepath.Join("testdata", "absent"), PortMarkup, nil)
	if err == nil {
		t.Fatal("a root with no fixture at the ref reported no error")
	}
}
