package commit

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
	testweakened "github.com/ze-software/ze/internal/le/test/weakened"
	"github.com/ze-software/ze/internal/le/weekly"
)

// weeklyCommit commits the files write into root with a subject and an
// optional body, and answers the new commit ID.
func weeklyCommit(t *testing.T, root, subject, body string, write map[string]string) string {
	t.Helper()
	for path, content := range write {
		writeCommitFixture(t, root, path, content)
		runCommitGit(t, root, "add", "--", path)
	}
	args := []string{"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
		"commit", "-q", "--allow-empty", "-m", subject}
	if body != "" {
		args = append(args, "-m", body)
	}
	runCommitGit(t, root, args...)
	return strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
}

// weeklyRange is the EventRange the report hands every source for from..HEAD:
// the first-parent commits after from, oldest first.
func weeklyRange(t *testing.T, root, from string) weekly.EventRange {
	t.Helper()
	to := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
	listed := strings.Fields(runCommitGitOutput(t, root, "rev-list", "--first-parent", "--reverse", from+".."+to))
	return weekly.EventRange{Repo: root, FromCommit: from, ToCommit: to, Commits: listed}
}

// weeklyFact answers the fact named name, failing the test when the body has none.
func weeklyFact(t *testing.T, body weekly.SectionBody, name string) weekly.Fact {
	t.Helper()
	index := slices.IndexFunc(body.Facts, func(fact weekly.Fact) bool { return fact.Name == name })
	if index < 0 {
		t.Fatalf("section has no fact %q: %+v", name, body.Facts)
	}
	return body.Facts[index]
}

// TestReportCountsApprovalsInRange asserts the approvals section counts each
// approval line of the range's commits, with the commits that carry it.
//
// VALIDATES: AC-5. RFC-approved and Weakening-approved trailers and Test-judge
// body lines are counted per line, a commit before the range is not counted, a
// commit carrying two lines is listed once with both counted, a key in the
// middle of a line is not a record, and the note says only landed approvals
// are counted. An empty range answers zero counts, never no facts.
// PREVENTS: a count read over the wrong range, or a key copied as a string
// that drifts from the writer's constant.
func TestReportCountsApprovalsInRange(t *testing.T) {
	root := newCommitRepository(t)
	weeklyCommit(t, root, "before the range", rfc.ApprovalTrailer+"pkg.TestOld: an approval before the range", nil)
	from := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
	twoRFC := weeklyCommit(t, root, "two rfc approvals",
		rfc.ApprovalTrailer+"pkg.TestA: proves more\n"+rfc.ApprovalTrailer+"pkg.TestB: proves more", nil)
	weakening := weeklyCommit(t, root, "a weakening and a judge line",
		"Test-judge: test/x.ci: the old expectation was wrong\n\n"+testweakened.ApprovalTrailer+"pkg.TestC: verdict", nil)
	weeklyCommit(t, root, "mentions keys mid-line", "this body quotes "+rfc.ApprovalTrailer+"inside a sentence", nil)

	body, err := answerWeeklyApprovals(t.Context(), weeklyRange(t, root, from))
	if err != nil {
		t.Fatalf("the approvals section failed: %v", err)
	}
	cases := []struct {
		name    string
		value   string
		commits []string
	}{
		{name: "RFC-approved trailers", value: "2", commits: []string{twoRFC}},
		{name: "Weakening-approved trailers", value: "1", commits: []string{weakening}},
		{name: "Test-judge lines", value: "1", commits: []string{weakening}},
	}
	for _, want := range cases {
		fact := weeklyFact(t, body, want.name)
		if fact.Value != want.value {
			t.Errorf("%s: value %q, want %q", want.name, fact.Value, want.value)
		}
		if !slices.Equal(fact.Items, want.commits) {
			t.Errorf("%s: commits %v, want %v", want.name, fact.Items, want.commits)
		}
	}
	if !strings.Contains(body.Note, "only the approvals that landed") {
		t.Errorf("the note does not say only landed approvals are counted: %q", body.Note)
	}

	later, err := answerWeeklyApprovals(t.Context(), weeklyRange(t, root, weakening))
	if err != nil {
		t.Fatalf("the approvals section failed over a range: %v", err)
	}
	head := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
	none, err := answerWeeklyApprovals(t.Context(), weeklyRange(t, root, head))
	if err != nil {
		t.Fatalf("the approvals section failed over an empty range: %v", err)
	}
	if len(none.Facts) != len(weeklyApprovalLines) {
		t.Fatalf("an empty range answered %d facts, want %d", len(none.Facts), len(weeklyApprovalLines))
	}
	for _, fact := range none.Facts {
		if fact.Value != "0" {
			t.Errorf("an empty range answered %s = %q, want 0", fact.Name, fact.Value)
		}
	}
	if fact := weeklyFact(t, later, "RFC-approved trailers"); fact.Value != "0" {
		t.Errorf("a range after the two-approval commit counted %q RFC approvals", fact.Value)
	}
}

// TestReportGroupsFragmentsByFoundBy asserts the fragments section lists the
// news fragments the range adds, grouped by found-by, each with its owner.
//
// VALIDATES: AC-6 and AC-8. A fragment added before the range is not listed;
// each added fragment with found-by is listed under its value with its spec or
// journal row; fragments with no found-by are one total; every declared value
// is stated, a zero included; a README added beside them is not a fragment;
// and a fragment that does not parse fails the section naming its path rather
// than dropping out of the count.
// PREVENTS: a malformed fragment silently missing from the week's fixes.
func TestReportGroupsFragmentsByFoundBy(t *testing.T) {
	root := newCommitRepository(t)
	sections := newsValidSections()
	weeklyCommit(t, root, "vocabulary and an old fragment", "", map[string]string{
		newsTopicsPath: newsVocabulary,
		newsFragmentDirectory + "old.md": newsFragmentText(
			"journal: bgp-old 2026-10-01\ntopic: BGP\nfound-by: interop\n", sections),
	})
	from := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
	weeklyCommit(t, root, "an interop fix", "", map[string]string{
		newsFragmentDirectory + "interop-fix.md": newsFragmentText(
			"spec: spec-bgp-reset\ntopic: BGP\nfound-by: interop\n", sections),
	})
	weeklyCommit(t, root, "a review fix and a feature", "", map[string]string{
		newsFragmentDirectory + "review-fix.md": newsFragmentText(
			"journal: bgp-reset 2026-11-09\ntopic: BGP\nfound-by: review\n", sections),
		newsFragmentDirectory + "feature.md": newsFragmentText("spec: spec-bgp-feature\ntopic: BGP\n", sections),
		newsFragmentReadme:                   "# News fragments\n",
	})

	body, err := answerWeeklyFragments(t.Context(), weeklyRange(t, root, from))
	if err != nil {
		t.Fatalf("the fragments section failed: %v", err)
	}
	if len(body.Facts) != len(newsFoundBy)+1 {
		t.Fatalf("answered %d facts, want one per found-by value and the total with none: %+v", len(body.Facts), body.Facts)
	}
	cases := []struct {
		name   string
		value  string
		owners []string
	}{
		{name: "found by interop", value: "1", owners: []string{"spec spec-bgp-reset"}},
		{name: "found by review", value: "1", owners: []string{"journal bgp-reset 2026-11-09"}},
		{name: "found by rfc-check", value: "0", owners: nil},
		{name: "with no found-by", value: "1", owners: nil},
	}
	for _, want := range cases {
		fact := weeklyFact(t, body, want.name)
		if fact.Value != want.value {
			t.Errorf("%s: value %q, want %q", want.name, fact.Value, want.value)
		}
		if !slices.Equal(fact.Items, want.owners) {
			t.Errorf("%s: owners %v, want %v", want.name, fact.Items, want.owners)
		}
	}

	broken := newsFragmentDirectory + "broken.md"
	weeklyCommit(t, root, "a malformed fragment", "", map[string]string{broken: "no front matter at all\n"})
	_, err = answerWeeklyFragments(t.Context(), weeklyRange(t, root, from))
	if err == nil {
		t.Fatal("a range adding a fragment that does not parse answered the section")
	}
	if !strings.Contains(err.Error(), broken) {
		t.Errorf("the failure does not name %s: %v", broken, err)
	}
}
