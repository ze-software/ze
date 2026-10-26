// Design: docs/contributing/spec-workflow.md -- the roadmap Profiles column.
// Tests drive roadmap.Collect, with this package's reader registered, over
// isolated Git object stores, without touching the developer checkout.
package rfc

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/spec/roadmap"
)

func fixtureGit(t *testing.T, root, input string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
	command.Stdin = strings.NewReader(input)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

// fixtureCommit writes files as the one commit of a new repository through
// fast-import, and answers the repository root and the commit.
func fixtureCommit(t *testing.T, files map[string]string) (string, string) {
	t.Helper()
	root := t.TempDir()
	fixtureGit(t, root, "", "init", "-q", "--initial-branch=main")
	var stream strings.Builder
	stream.WriteString("commit refs/heads/main\ncommitter Fixture <fixture@example.test> 1700000000 +0000\ndata 7\nfixture\n")
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		body := files[path]
		fmt.Fprintf(&stream, "M 100644 inline %s\ndata %d\n%s\n", strconv.Quote(path), len(body), body)
	}
	stream.WriteString("\ndone\n")
	fixtureGit(t, root, stream.String(), "fast-import", "--quiet")
	return root, fixtureGit(t, root, "", "rev-parse", "HEAD")
}

func fixtureCurrentSpec(title string) string {
	return "# Spec: " + title + "\n\n| Field | Value |\n|---|---|\n| Status | ready |\n| Depends | - |\n| Phase | 1/2 |\n| Updated | 2026-09-18 |\n" +
		"| Priority | P1 |\n| Release reason | Release boundary fixture |\n"
}

// fixtureProfile is a use-case profile file ParseProfiles accepts, its
// Release cell release, with extra rows appended.
func fixtureProfile(name, release, extra string) string {
	return "# Profile: " + name + "\n\n| Field | Value |\n|-------|-------|\n| Kind | use-case |\n| Title | " + name +
		" |\n| Release | " + release + " |\n" + extra
}

// fixtureRoadmapSummary is a summary whose Meta table declares required and
// optional use cases and whose checklist holds line.
func fixtureRoadmapSummary(required, optional, line string) string {
	return "# RFC\n\n" + strings.Replace(selftestMeta, selftestUseCaseRows,
		"| Use cases | "+required+" |\n| Optional use cases | "+optional+" |\n", 1) +
		"\n## Compliance Checklist\n\n" + line
}

// TestRoadmapProfilesColumnFromRFCGaps holds AC-12: a spec whose `RFC gaps`
// names a requirement id and a stem carries exactly the profiles the named
// documents' summaries require, read from the same revision's rfc/short/ and
// rfc/profiles/ alone; an optional listing places nothing; an unknown id and
// a malformed cell reach the item's diagnostics; a spec with no gaps carries
// no profile. Method: one fixture commit holding specs, summaries and
// profiles and no RFC text, where rfc2 requires edge and lists lab as optional.
func TestRoadmapProfilesColumnFromRFCGaps(t *testing.T) {
	gapped := func(title, gaps string) string {
		return fixtureCurrentSpec(title) + "| RFC gaps | " + gaps + " |\n"
	}
	files := map[string]string{
		"plan/release/fixes/spec-gapped.md":  gapped("Gapped", "RFC3-1-1, rfc4"),
		"plan/release/fixes/spec-reached.md": gapped("Reached", "rfc2"),
		"plan/release/fixes/spec-unknown.md": gapped("Unknown", "RFC9-9-9, rfc4,, rfc4"),
		"plan/release/fixes/spec-plain.md":   fixtureCurrentSpec("Plain"),
		"rfc/short/rfc2.md": fixtureRoadmapSummary("edge", "lab",
			"- [ ] [RFC2-1-1] [MUST] A speaker MUST send the widget (§1)\n"),
		"rfc/short/rfc3.md": fixtureRoadmapSummary("edge", "-",
			"- [ ] [RFC3-1-1] [MUST] A speaker MUST send the gadget (§1)\n"),
		"rfc/short/rfc4.md": fixtureRoadmapSummary("lab", "-",
			"- [ ] [RFC4-1-1] [MUST] A speaker MUST send the gizmo (§1)\n"),
		"rfc/profiles/edge.md": fixtureProfile("edge", "yes", ""),
		"rfc/profiles/lab.md":  fixtureProfile("lab", "yes", ""),
	}
	root, revision := fixtureCommit(t, files)
	snapshot, err := roadmap.Collect(t.Context(), root, revision)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]roadmap.Item{}
	for _, item := range snapshot.Items {
		byName[item.Name] = item
	}
	want := map[string][]string{"gapped": {"edge", "lab"}, "reached": {"edge"}, "unknown": {"lab"}, "plain": {}}
	for name, profiles := range want {
		if got := byName[name].Profiles; !slices.Equal(got, profiles) {
			t.Errorf("%s: Profiles %v, want %v (diagnostics %v)", name, got, profiles, byName[name].Diagnostics)
		}
	}
	for _, name := range []string{"gapped", "reached", "plain"} {
		if diagnostics := byName[name].Diagnostics; len(diagnostics) != 0 {
			t.Errorf("%s: a placeable spec carries diagnostics %v", name, diagnostics)
		}
	}
	diagnostics := strings.Join(byName["unknown"].Diagnostics, "\n")
	for _, fragment := range []string{"names RFC9-9-9, which is neither a requirement id", "holds an empty item between commas", "names rfc4 twice"} {
		if !strings.Contains(diagnostics, fragment) {
			t.Errorf("unknown: diagnostics lack %q: %v", fragment, byName["unknown"].Diagnostics)
		}
	}
	markdown := roadmap.Markdown(&snapshot, false)
	if !strings.Contains(markdown, "| Work item | Priority | Release reason | Profiles | Status |") {
		t.Error("the roadmap table has no Profiles column")
	}
	if !strings.Contains(markdown, "| P1 | Release boundary fixture | edge, lab | ready |") {
		t.Errorf("the gapped row does not name both profiles:\n%s", markdown)
	}
}

// TestRoadmapProfilesUnjudgedCorpus holds the refusal side of AC-16 for a
// corpus that does not load: a profile that does not parse leaves every
// gapped item unplaced with a diagnostic, never an empty Profiles cell that
// reads as "no profile". Method: a profile carrying the retired Roots row.
func TestRoadmapProfilesUnjudgedCorpus(t *testing.T) {
	files := map[string]string{
		"plan/release/fixes/spec-gapped.md": fixtureCurrentSpec("Gapped") + "| RFC gaps | rfc2 |\n",
		"rfc/short/rfc2.md": fixtureRoadmapSummary("edge", "-",
			"- [ ] [RFC2-1-1] [MUST] A speaker MUST send the widget (§1)\n"),
		"rfc/profiles/edge.md": fixtureProfile("edge", "yes", "| Roots | rfc2 |\n"),
	}
	root, revision := fixtureCommit(t, files)
	snapshot, err := roadmap.Collect(t.Context(), root, revision)
	if err != nil {
		t.Fatal(err)
	}
	item := snapshot.Items[0]
	if len(item.Profiles) != 0 {
		t.Errorf("Profiles %v from a corpus that did not load", item.Profiles)
	}
	if !strings.Contains(strings.Join(item.Diagnostics, "\n"), "RFC gaps unjudged") {
		t.Errorf("no unjudged diagnostic: %v", item.Diagnostics)
	}
}

// TestRoadmapDigestCoversTheRFCCorpus: the corpus the Profiles column read is
// an input of the snapshot, so two revisions whose specs are identical and
// whose profiles differ carry different input digests, and an RFC text is
// not one. Method: two fixture commits that differ only in one profile's
// Release cell, and two that differ only in an RFC text.
func TestRoadmapDigestCoversTheRFCCorpus(t *testing.T) {
	digestOf := func(release, text string) string {
		files := map[string]string{
			"plan/release/fixes/spec-gapped.md": fixtureCurrentSpec("Gapped") + "| RFC gaps | rfc2 |\n",
			"rfc/short/rfc2.md": fixtureRoadmapSummary("edge", "-",
				"- [ ] [RFC2-1-1] [MUST] A speaker MUST send the widget (§1)\n"),
			"rfc/profiles/edge.md": fixtureProfile("edge", release, ""),
			"rfc/full/rfc2.txt":    text,
		}
		root, revision := fixtureCommit(t, files)
		snapshot, err := roadmap.Collect(t.Context(), root, revision)
		if err != nil {
			t.Fatal(err)
		}
		return snapshot.InputDigest
	}
	if digestOf("yes", "one") == digestOf("no", "one") {
		t.Error("a profile change left the roadmap's input digest unchanged")
	}
	if digestOf("yes", "one") != digestOf("yes", "two") {
		t.Error("an RFC text changed the roadmap's input digest: the Profiles column reads no text")
	}
}
