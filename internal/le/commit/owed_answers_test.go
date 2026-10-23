// Design: docs/architecture/testing/verify-freshness-scope.md -- an owed item's answer is re-derived from git, never trusted
// Related: owed_answers.go -- the producers these tests drive
//
// VALIDATES: each answer that closes a person-act obligation (not-applicable,
// closed, reviewed) re-derives its verdict from the commit's own blobs, in both
// polarities, and the commit loader refuses a revision it cannot resolve.
// PREVENTS: an obligation closed on the operator's word, or by bytes the
// working tree holds rather than the bytes the commit carried.
package commit

import (
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// reviewGateArtifact is the artifact row of a recorded Review Gate.
const reviewGateArtifact = "| Artifact | `tmp/review/fixture.md` (3 files pinned by SHA-256, verdict clean) |"

// rfcTagRequirement is the requirement id the fixture's tag names.
//
// The tag text is ASSEMBLED from this constant at run time, so no line of THIS
// file matches rfcTagPattern. A literal tag here would make the file its own
// tag carrier, and the owner-approval gate would then refuse every later edit
// to the tests that prove that gate.
const rfcTagRequirement = "rfc9999-1 positive"

// taggedUnitText renders a test file whose one function carries an RFC
// requirement tag INSIDE its body.
//
// The call in the body is the parameter because the gate reads a unit whose
// TEXT moved. A comment-only edit changes no tagged unit, so it would prove
// nothing here.
func taggedUnitText(call string) string {
	return "package pkg\n\nfunc TestThing(t *testing.T) {\n" +
		"\t// RFC requirement: " + rfcTagRequirement + "\n\t" + call + "\n}\n"
}

// commitFixture writes and removes paths, commits them under the named subject,
// and answers the SHA of the new commit.
func commitFixture(t *testing.T, root, subject string, write map[string]string, remove []string) string {
	t.Helper()
	for path, content := range write {
		writeCommitFixture(t, root, path, content)
		runCommitGit(t, root, "add", "--", path)
	}
	for _, path := range remove {
		runCommitGit(t, root, "rm", "-q", "--", path)
	}
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t",
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", subject)
	return strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
}

// factsAt reads one commit through the same loader the derivation uses.
func factsAt(t *testing.T, root, revision string) commitFacts {
	t.Helper()
	facts, err := newCommitCache().read(root, revision)
	if err != nil {
		t.Fatalf("read the facts of %s: %v", revision, err)
	}
	return facts
}

// specText is a spec file carrying a metadata Status and, when one is given, a
// Review Gate section.
func specText(status, gate string) string {
	text := "# Spec: a fixture\n\n| Field | Value |\n|-------|-------|\n| Status | " + status +
		" |\n| Scope | tooling |\n\n## Task\n\nA fixture.\n\n"
	if gate == "" {
		return text
	}
	return text + gate
}

// reviewGateSection renders a recorded Review Gate under one spelling of its
// verdict row. The predicate reads the rows, so the label is a parameter here.
func reviewGateSection(label, verdict string) string {
	return "## Review Gate\n\n| Field | Value |\n|-------|-------|\n" +
		reviewGateArtifact + "\n| " + label + " | " + verdict + " |\n| Rounds | 2 |\n" +
		"| Reviewer lenses used | logic + wiring |\n"
}

// reviewArtifact renders a ze-review artifact pinning each path to a hash.
func reviewArtifact(verdict string, hashes map[string]string) string {
	var text strings.Builder
	text.WriteString("<!-- ze-review spec=fixture verdict=" + verdict + " -->\n\n")
	for path, hash := range hashes {
		text.WriteString("  " + hash + "  " + path + "\n")
	}
	return text.String()
}

// TestNotApplicableAnswerDerivesTheClosureStem runs the closure reading over a
// commit in both polarities: a commit closing no spec owes no review, and one
// that removes a spec does, named by the stem the derivation found.
func TestNotApplicableAnswerDerivesTheClosureStem(t *testing.T) {
	root := newCommitRepository(t)
	journal := commitFixture(t, root, "a journal row and no spec",
		map[string]string{"plan/journal/a-class.md": "# a class\n"}, nil)
	if err := answerNotApplicable(root, owedItemReview, factsAt(t, root, journal)); err != nil {
		t.Fatalf("a commit closing no spec was refused: %v", err)
	}

	spec := "plan/immediate/spec-a-thing.md"
	commitFixture(t, root, "the spec lands", map[string]string{spec: specText("in-progress", "")}, nil)
	closure := commitFixture(t, root, "the spec closes", nil, []string{spec})
	err := answerNotApplicable(root, owedItemReview, factsAt(t, root, closure))
	if err == nil {
		t.Fatal("a commit that closes a spec answered not-applicable")
	}
	if !strings.Contains(err.Error(), "a-thing") {
		t.Fatalf("the refusal %q does not name the stem the derivation found", err)
	}
}

// TestNotApplicableAnswerReadsTaggedUnitsAtTheCommitParent runs the
// owner-approval gate's own reading over a commit, in both polarities.
//
// The refusing polarity is the one with no other tell: a commit that DOES move
// a tagged unit owes the owner approval, so answering it not-applicable would
// close a real obligation on nobody's word.
func TestNotApplicableAnswerReadsTaggedUnitsAtTheCommitParent(t *testing.T) {
	const tagged = "pkg/thing_test.go"

	root := newCommitRepository(t)
	commitFixture(t, root, "the tagged test lands",
		map[string]string{tagged: taggedUnitText("check(1)")}, nil)
	plain := commitFixture(t, root, "a prose edit only",
		map[string]string{"docs/note.md": "# note\n"}, nil)
	if err := answerNotApplicable(root, owedItemRFCApproval, factsAt(t, root, plain)); err != nil {
		t.Fatalf("a commit changing no tagged unit was refused: %v", err)
	}

	changed := commitFixture(t, root, "the tagged test changes",
		map[string]string{tagged: taggedUnitText("check(2)")}, nil)
	err := answerNotApplicable(root, owedItemRFCApproval, factsAt(t, root, changed))
	if err == nil {
		t.Fatal("a commit changing a tagged unit answered not-applicable")
	}
	if !strings.Contains(err.Error(), "TestThing") {
		t.Fatalf("the refusal %q does not name the unit it found", err)
	}
}

// TestNotApplicableAnswerRefusesAnUnclassifiedItem pins that an item nobody
// classified is never read as answered.
func TestNotApplicableAnswerRefusesAnUnclassifiedItem(t *testing.T) {
	root := newCommitRepository(t)
	plain := commitFixture(t, root, "a prose edit only",
		map[string]string{"docs/note.md": "# note\n"}, nil)
	if err := answerNotApplicable(root, owedItemUnspecified, factsAt(t, root, plain)); err == nil {
		t.Fatal("an owed item with no kind answered not-applicable")
	}
}

// TestReviewedAnswerJudgesTheArtifactAgainstTheCommitBytes pins that the
// hashes are compared against the COMMIT, not the working tree.
//
// The middle case is the one a working-tree comparison passes and this one must
// not: the artifact pins the bytes the file holds NOW, which is not what the
// reviewer read at the commit.
func TestReviewedAnswerJudgesTheArtifactAgainstTheCommitBytes(t *testing.T) {
	const code = "pkg/thing.go"
	root := newCommitRepository(t)
	reviewed := factsAt(t, root, commitFixture(t, root, "the reviewed change",
		map[string]string{code: "package pkg\n\nfunc Thing() {}\n"}, nil))

	artifact := "tmp/review/fixture-clean.md"
	writeCommitFixture(t, root, artifact, reviewArtifact("clean",
		map[string]string{code: reviewHash(filepath.Join(root, filepath.FromSlash(code)))}))
	if err := answerReviewed(root, artifact, reviewed); err != nil {
		t.Fatalf("a clean artifact over the commit's own bytes was refused: %v", err)
	}

	// The working tree moves on. An artifact pinned to the NEW bytes covers the
	// path and still says nothing about what the commit carried.
	writeCommitFixture(t, root, code, "package pkg\n\nfunc Thing() { println(1) }\n")
	stale := "tmp/review/fixture-stale.md"
	writeCommitFixture(t, root, stale, reviewArtifact("clean",
		map[string]string{code: reviewHash(filepath.Join(root, filepath.FromSlash(code)))}))
	err := answerReviewed(root, stale, reviewed)
	if err == nil {
		t.Fatal("an artifact pinned to the working tree answered the commit")
	}
	if !strings.Contains(err.Error(), code) {
		t.Fatalf("the refusal %q does not name the path whose bytes disagree", err)
	}

	// A verdict that is not clean, and a code path the artifact never covered.
	for _, bad := range []struct {
		why      string
		verdict  string
		coverage map[string]string
		names    string
	}{
		{"a blocker verdict", "blocker",
			map[string]string{code: reviewHash(filepath.Join(root, filepath.FromSlash(code)))}, "not clean"},
		{"an uncovered path", "clean", map[string]string{}, "not covered"},
	} {
		path := "tmp/review/fixture-" + bad.verdict + strconv.Itoa(len(bad.coverage)) + ".md"
		writeCommitFixture(t, root, path, reviewArtifact(bad.verdict, bad.coverage))
		err := answerReviewed(root, path, reviewed)
		if err == nil {
			t.Errorf("%s answered the commit", bad.why)
			continue
		}
		if !strings.Contains(err.Error(), bad.names) {
			t.Errorf("%s answered %q, want the refusal to name %q", bad.why, err, bad.names)
		}
	}
}

// TestReviewedAnswerFallsBackToTheCommittedReviewGate pins the durable half:
// tmp/review/ is untracked and is emptied, so a review recorded months ago has
// no artifact left, and the Review Gate the closure committed is what remains.
func TestReviewedAnswerFallsBackToTheCommittedReviewGate(t *testing.T) {
	root := newCommitRepository(t)
	spec := "plan/immediate/spec-reviewed-thing.md"
	commitFixture(t, root, "the spec lands",
		map[string]string{spec: specText("in-progress", reviewGateSection("`review check`", "clean"))}, nil)
	closure := factsAt(t, root, commitFixture(t, root, "the reviewed spec closes", nil, []string{spec}))

	for _, route := range []struct {
		why      string
		artifact string
	}{
		{"no artifact at all", ""},
		{"an artifact path that no longer exists", "tmp/review/gone.md"},
	} {
		if err := answerReviewed(root, route.artifact, closure); err != nil {
			t.Errorf("%s was refused, want the committed Review Gate read instead: %v", route.why, err)
		}
	}

	// A commit that removes no spec has no gate to fall back to, and must say so
	// rather than answer on the operator's word.
	plain := factsAt(t, root, commitFixture(t, root, "an ordinary commit",
		map[string]string{"docs/note.md": "# note\n"}, nil))
	err := answerReviewed(root, "", plain)
	if err == nil {
		t.Fatal("a commit removing no spec was answered through the fallback")
	}
	if !strings.Contains(err.Error(), "removes no spec") {
		t.Fatalf("the refusal %q does not say there is nothing to read", err)
	}
}

// TestClosedAnswerRequiresARecordedReviewGate reads the gate out of the
// closure commit's parent, over both era spellings of the verdict row and over
// every shape that records no review.
//
// The two spellings are why the predicate reads the section's ROWS: the label
// is authored prose and tracks neither the recorder nor the date, so a verifier
// matching one spelling reads the other as absent.
func TestClosedAnswerRequiresARecordedReviewGate(t *testing.T) {
	for _, gate := range []struct {
		why     string
		section string
	}{
		{"the older spelling", reviewGateSection("`review_gate.py check`", "clean")},
		{"the newer spelling", reviewGateSection("`review check`", "OK -- review_gate: OK")},
	} {
		root := newCommitRepository(t)
		spec := "plan/immediate/spec-closed-thing.md"
		commitFixture(t, root, "the spec lands",
			map[string]string{spec: specText("in-progress", gate.section)}, nil)
		closure := commitFixture(t, root, "the spec closes", nil, []string{spec})
		if err := answerClosed(root, factsAt(t, root, closure)); err != nil {
			t.Errorf("%s was refused: %v", gate.why, err)
		}
	}

	for _, refused := range []struct {
		why     string
		section string
		names   string
	}{
		{"an absent gate section", "", "no ## Review Gate"},
		{"the unfilled template", "## Review Gate\n\n| Field | Value |\n|-------|-------|\n" +
			"| Artifact | [path printed by `review_gate.py record`] |\n" +
			"| `review_gate.py check` | [clean / not run] |\n| Rounds | [N] |\n", "not run"},
		{"a gate recording no review", "## Review Gate\n\n| Field | Value |\n|-------|-------|\n" +
			"| Artifact | not recorded |\n| `review check` | not run |\n| Rounds | 0 |\n", "not recorded"},
		{"a gate with no rounds count", "## Review Gate\n\n| Field | Value |\n|-------|-------|\n" +
			reviewGateArtifact + "\n| `review check` | clean |\n", "rounds count"},
	} {
		root := newCommitRepository(t)
		spec := "plan/immediate/spec-closed-thing.md"
		commitFixture(t, root, "the spec lands",
			map[string]string{spec: specText("in-progress", refused.section)}, nil)
		closure := commitFixture(t, root, "the spec closes", nil, []string{spec})
		err := answerClosed(root, factsAt(t, root, closure))
		if err == nil {
			t.Errorf("%s answered the closure", refused.why)
			continue
		}
		if !strings.Contains(err.Error(), refused.names) {
			t.Errorf("%s answered %q, want the refusal to quote %q", refused.why, err, refused.names)
		}
	}
}

// TestSkeletonClosureAnswersAndAnImplementedOneDoesNot pins the branch that
// must never be entered from the absence of a gate.
//
// Every case removes a spec carrying NO Review Gate. Only the Status separates
// them, which is the whole point: an absent gate on a skeleton means there was
// nothing to review, and on an implemented spec it means the review is missing.
func TestSkeletonClosureAnswersAndAnImplementedOneDoesNot(t *testing.T) {
	for _, closure := range []struct {
		status  string
		answers bool
	}{
		{"skeleton", true},
		{"design", true},
		{"in-progress", false},
		{"verification", false},
		{"ready", false},
	} {
		root := newCommitRepository(t)
		spec := "plan/immediate/spec-status-thing.md"
		commitFixture(t, root, "the spec lands",
			map[string]string{spec: specText(closure.status, "")}, nil)
		sha := commitFixture(t, root, "the spec closes", nil, []string{spec})
		err := answerClosed(root, factsAt(t, root, sha))
		if closure.answers && err != nil {
			t.Errorf("a %s spec with no gate was refused, want it answered: %v", closure.status, err)
		}
		if !closure.answers && err == nil {
			t.Errorf("a %s spec with no gate was answered", closure.status)
		}
	}
}

// TestAReviewGateArtifactMustNameAFile pins what "a filled artifact reference"
// is, for the answers that read a committed Review Gate.
//
// A cell test that only refuses the empty string reads `n/a` and `-` as
// evidence, and those are exactly what an author writes where no review
// produced anything.
func TestAReviewGateArtifactMustNameAFile(t *testing.T) {
	for _, gate := range []struct {
		why     string
		cell    string
		answers bool
	}{
		{"a review artifact under tmp/review", "`tmp/review/fixture.md` (3 files, verdict clean)", true},
		{"a dash", "-", false},
		{"an n/a", "n/a", false},
		{"a sentence naming no file", "the reviewer read every hunk", false},
	} {
		root := newCommitRepository(t)
		spec := "plan/immediate/spec-artifact-thing.md"
		section := "## Review Gate\n\n| Field | Value |\n|-------|-------|\n" +
			"| Artifact | " + gate.cell + " |\n| `review check` | clean |\n| Rounds | 2 |\n"
		commitFixture(t, root, "the spec lands",
			map[string]string{spec: specText("in-progress", section)}, nil)
		closure := commitFixture(t, root, "the spec closes", nil, []string{spec})

		err := answerClosed(root, factsAt(t, root, closure))
		if gate.answers && err != nil {
			t.Errorf("%s was refused, want the gate read as recorded: %v", gate.why, err)
		}
		if !gate.answers {
			if err == nil {
				t.Errorf("%s answered the closure", gate.why)
				continue
			}
			if !strings.Contains(err.Error(), "names no file") {
				t.Errorf("%s answered %q, want the refusal to say the artifact names no file", gate.why, err)
			}
		}
	}
}

// TestCommitCacheLoadsEveryCommitInOneBatch pins the batched loader: several
// revisions, HEAD among them, read in one load whatever the user's diff
// configuration, and a revision that names no commit refused rather than
// answered as zero facts.
func TestCommitCacheLoadsEveryCommitInOneBatch(t *testing.T) {
	root := newCommitRepository(t)
	runCommitGit(t, root, "config", "diff.noprefix", "true")
	runCommitGit(t, root, "config", "diff.mnemonicPrefix", "true")
	first := commitFixture(t, root, "first", map[string]string{
		"plan/a.md": "a\n",
		"plan/b.md": "b\n",
	}, nil)
	second := commitFixture(t, root, "second", map[string]string{"notes.txt": "x\n"}, []string{"plan/a.md"})

	commits := newCommitCache()
	commits.load(root, []string{first, "HEAD", second, "no-such-revision", ""})

	one, err := commits.read(root, first)
	if err != nil {
		t.Fatalf("read(first): %v", err)
	}
	if one.SHA != first || one.Subject != "first" || one.Parent == "" {
		t.Fatalf("read(first) = %+v, want SHA %s, subject first and a parent", one, first)
	}
	if !slices.Contains(one.Paths, "plan/b.md") {
		t.Fatalf("read(first) paths = %v, want plan/b.md", one.Paths)
	}
	head, err := commits.read(root, "HEAD")
	if err != nil || head.SHA != second || head.Parent != first {
		t.Fatalf("read(HEAD) = %+v, %v; want SHA %s with parent %s", head, err, second, first)
	}
	if !slices.Contains(head.Removed, "plan/a.md") || slices.Contains(head.Paths, "plan/a.md") {
		t.Fatalf("read(HEAD) = %+v, want plan/a.md removed and not among the paths", head)
	}
	if bySHA, err := commits.read(root, second); err != nil || !reflect.DeepEqual(bySHA, head) {
		t.Fatalf("read(%s) = %+v, %v; want the facts read(HEAD) answered, %+v", second, bySHA, err, head)
	}
	for _, revision := range []string{"no-such-revision", ""} {
		if _, err := commits.read(root, revision); err == nil {
			t.Fatalf("read(%q) answered no error, want the revision refused", revision)
		}
	}
}
