package commit

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// specWorkSpecPath is the spec every fixture below closes. Its stem, the name
// the closure gate and the trailer both use, is "x".
const specWorkSpecPath = "plan/release/fixes/spec-x.md"

// claimAs points the ambient harness identity at session (and agent, when
// not empty) and, when specFile is not empty, writes that identity's claim
// marker the way `./le spec claim` does.
func claimAs(t *testing.T, root, session, agent, specFile string) {
	t.Helper()
	t.Setenv("CLAUDE_CODE_SESSION_ID", session)
	t.Setenv("ZE_AGENT_ID", agent)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	if specFile == "" {
		return
	}
	marker, err := lepath.SpecClaimMarker(root, session, agent)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(marker), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte(specFile+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// preparedMessage reads the message file a prepared commit wrote.
func preparedMessage(t *testing.T, root string, prepared *Prepared) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(prepared.Message)))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// TestCreateWritesSpecWorkFromTheClaim drives the action layer, where the
// claim is resolved.
//
// VALIDATES: a commit prepared under a claimed spec ends with the trailer
// `Spec-work: <stem>`, and an agent that claimed nothing writes its parent's
// claim (AC-1).
// PREVENTS: R-3, the claim read under the eight-hex commit namespace rather
// than the harness session, which finds no marker and writes no trailer.
func TestCreateWritesSpecWorkFromTheClaim(t *testing.T) {
	for _, agent := range []string{"", "agent-1"} {
		root := newCommitRepository(t)
		claimAs(t, root, "spec-work-session", "", "spec-x.md")
		claimAs(t, root, "spec-work-session", agent, "")
		writeCommitFixture(t, root, "tracked.txt", "changed "+agent+"\n")
		answer, code := answerCreate(root, leaction.Arguments{keywordSubject: {"s"}, keywordFile: {"tracked.txt"}})
		prepared, ok := answer.(*Prepared)
		if code != 0 || !ok {
			t.Fatalf("agent %q: create answered (%#v, %d)", agent, answer, code)
		}
		message := preparedMessage(t, root, prepared)
		lines := strings.Split(strings.TrimRight(message, "\n"), "\n")
		if last := lines[len(lines)-1]; last != "Spec-work: x" {
			t.Fatalf("agent %q: the message ends with %q, want the trailer Spec-work: x:\n%s", agent, last, message)
		}
	}
}

// TestCreateWithoutAClaimWritesNoSpecWork pins the two commits that carry no
// attribution (AC-2).
//
// VALIDATES: a commit with no claim carries no Spec-work: line and is not
// refused; an attestation, which changes no file, carries none even under a
// claim.
// PREVENTS: an attribution invented for a session with no spec, and an empty
// commit attributed to a spec whose review it can add nothing to.
func TestCreateWithoutAClaimWritesNoSpecWork(t *testing.T) {
	root := newOwedRepository(t)
	claimAs(t, root, "spec-work-unclaimed", "", "")
	writeCommitFixture(t, root, "tracked.txt", "changed\n")
	answer, code := answerCreate(root, leaction.Arguments{keywordSubject: {"s"}, keywordFile: {"tracked.txt"}})
	prepared, ok := answer.(*Prepared)
	if code != 0 || !ok {
		t.Fatalf("create with no claim answered (%#v, %d)", answer, code)
	}
	if message := preparedMessage(t, root, prepared); strings.Contains(message, "Spec-work:") {
		t.Fatalf("a commit with no claim carries an attribution:\n%s", message)
	}

	owed := commitOwed(t, root, "owes a review", map[string]string{"owed.txt": "owed\n"}, nil,
		trailerReviewOwed+"x: fixture")
	attested, err := Create(root, &Options{Subject: "the owner attests", Attest: owed,
		AttestWords: "the owner said yes", SpecWork: "x", DryRun: true})
	if err != nil {
		t.Fatalf("attestation refused: %v", err)
	}
	if strings.Contains(attested.MessageText, "Spec-work:") {
		t.Fatalf("an attestation carries an attribution:\n%s", attested.MessageText)
	}
}

// specWorkHistory builds the history the closure fixtures share: the spec,
// then a tagged commit carrying b.go, an untagged one carrying d.go (AC-9),
// and one tagged for another spec whose stem has x as a prefix, carrying c.go
// (AC-4). It answers the SHA of the tagged commit.
func specWorkHistory(t *testing.T, root string) string {
	t.Helper()
	commitFixture(t, root, "the spec", map[string]string{specWorkSpecPath: "# Spec: x\n"}, nil)
	tagged := commitFixture(t, root, "work on x\n\n"+trailerSpecWork+"x",
		map[string]string{"internal/b.go": "package internal\n"}, nil)
	commitFixture(t, root, "legacy work on x\n\nSpec: x",
		map[string]string{"internal/d.go": "package internal\n"}, nil)
	commitFixture(t, root, "work on xy\n\n"+trailerSpecWork+"xy",
		map[string]string{"internal/c.go": "package internal\n"}, nil)
	return tagged
}

// prepareSpecWorkClosure prepares commit A (a.go) and appends commit B (the
// spec removal) to its script, the way /ze-close does, with the review
// artifact pinning the files given.
func prepareSpecWorkClosure(t *testing.T, root string, pinned ...string) (Prepared, error) {
	t.Helper()
	writeCommitFixture(t, root, "internal/a.go", "package internal\n")
	if _, err := Create(root, &Options{Subject: "x: commit A", Files: []string{"internal/a.go"},
		NoTest: "the fixture carries no test", SpecWork: "x"}); err != nil {
		t.Fatalf("Create refused commit A: %v", err)
	}
	writeReviewArtifact(t, root, "x", pinned...)
	return Create(root, &Options{Subject: "plan: close x", Remove: []string{specWorkSpecPath},
		Append: true, SpecWork: "x"})
}

// TestClosureRefusesCodeAnEarlierSpecCommitCarried drives the closure gate
// through Create.
//
// VALIDATES: commit B is refused when the review does not cover a.go, which
// the unlanded commit A in the same script carries, and b.go, which a landed
// commit tagged Spec-work: x carries; the refusal names the record verb as a
// usage shape. d.go (untagged) and c.go (tagged for xy) are not demanded
// (AC-3, AC-9).
// PREVENTS: the vacuous gate: B carries only the removal, so a gate judging
// B's own paths judged nothing.
func TestClosureRefusesCodeAnEarlierSpecCommitCarried(t *testing.T) {
	root := newCommitRepository(t)
	claimAs(t, root, "spec-work-closure", "", "")
	specWorkHistory(t, root)
	_, err := prepareSpecWorkClosure(t, root)
	if err == nil {
		t.Fatal("the closure was accepted with a review that covers none of the spec's code")
	}
	for _, want := range []string{"internal/a.go", "internal/b.go", "./le spec review record spec " + shellQuote("x") + " ..."} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q:\n%v", want, err)
		}
	}
	for _, foreign := range []string{"internal/c.go", "internal/d.go"} {
		if strings.Contains(err.Error(), foreign) {
			t.Errorf("the refusal demands %s, which no commit attributed to x carries:\n%v", foreign, err)
		}
	}
}

// TestSpecWorkMatchIsExactNotSubstring pins R-1.
//
// VALIDATES: the tagged commits of x are those with a message line equal to
// `Spec-work: x`, never one that merely contains it.
// PREVENTS: git's --grep, a substring match, attributing spec xy's commit, a
// line with trailing words, or an indented line to x (AC-4).
func TestSpecWorkMatchIsExactNotSubstring(t *testing.T) {
	root := newCommitRepository(t)
	tagged := specWorkHistory(t, root)
	commitFixture(t, root, "quoted\n\nsee "+trailerSpecWork+"x", map[string]string{"internal/e.go": "package internal\n"}, nil)
	commitFixture(t, root, "trailing\n\n"+trailerSpecWork+"x later", map[string]string{"internal/f.go": "package internal\n"}, nil)
	population, err := specWorkPopulation(root, "x", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(population.Commits, []string{tagged}) {
		t.Fatalf("tagged commits of x = %v, want only %s", population.Commits, tagged)
	}
	if !slices.Equal(population.Paths, []string{"internal/b.go"}) {
		t.Fatalf("population of x = %v, want internal/b.go alone", population.Paths)
	}
}

// TestClosureAcceptsAReviewCoveringTheSpecsCommits is the accepting polarity
// of the refusal above.
//
// VALIDATES: a clean review pinning a.go and b.go at current content clears
// commit B; the review result lists the tagged commit it covered, and the
// script's re-check line names both files (AC-5).
// PREVENTS: R-6, a script re-check that judges only B's own paths and so
// passes or fails where the prepare gate did the opposite.
func TestClosureAcceptsAReviewCoveringTheSpecsCommits(t *testing.T) {
	root := newCommitRepository(t)
	claimAs(t, root, "spec-work-accepted", "", "")
	tagged := specWorkHistory(t, root)
	prepared, err := prepareSpecWorkClosure(t, root, "internal/a.go", "internal/b.go")
	if err != nil {
		t.Fatalf("the closure was refused with a review covering the spec's code: %v", err)
	}
	if prepared.Review == nil || !slices.Contains(prepared.Review.Commits, tagged) {
		t.Fatalf("the review result does not list the tagged commit %s: %#v", tagged, prepared.Review)
	}
	script, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(prepared.Script)))
	if err != nil {
		t.Fatal(err)
	}
	var recheck string
	for line := range strings.SplitSeq(string(script), "\n") {
		if strings.Contains(line, actionReviewCheck) {
			recheck = line
		}
	}
	for _, want := range []string{shellQuote("internal/a.go"), shellQuote("internal/b.go")} {
		if !strings.Contains(recheck, want) {
			t.Errorf("the script's review re-check %q does not name %s", recheck, want)
		}
	}
}

// TestDischargeJudgesTheSpecsEarlierCommits drives the push discharge of a
// closure committed with review-override.
//
// VALIDATES: an artifact covering only the closure commit's own code does not
// answer its Review-owed:, naming the earlier tagged commit's file; one
// covering the whole population at the closure commit's tree does (AC-7).
// PREVENTS: R-8, a discharge judging fewer files than the gate demanded.
func TestDischargeJudgesTheSpecsEarlierCommits(t *testing.T) {
	root := newCommitRepository(t)
	specWorkHistory(t, root)
	closure := factsAt(t, root, commitFixture(t, root, "plan: close x\n\n"+trailerSpecWork+"x",
		map[string]string{"internal/e.go": "package internal\n"}, []string{specWorkSpecPath}))
	pin := func(paths ...string) map[string]string {
		hashes := make(map[string]string, len(paths))
		for _, path := range paths {
			hashes[path] = reviewHash(filepath.Join(root, filepath.FromSlash(path)))
		}
		return hashes
	}
	own := "tmp/review/x-own.md"
	writeCommitFixture(t, root, own, reviewArtifact("clean", pin("internal/e.go")))
	err := answerReviewed(root, own, closure)
	if err == nil {
		t.Fatal("an artifact covering only the closure commit's own code answered the spec's review")
	}
	if !strings.Contains(err.Error(), "internal/b.go") {
		t.Fatalf("the refusal %q does not name the earlier tagged commit's file", err)
	}
	whole := "tmp/review/x-whole.md"
	writeCommitFixture(t, root, whole, reviewArtifact("clean", pin("internal/e.go", "internal/b.go")))
	// The working tree moves on; the discharge reads the closure commit's tree.
	writeCommitFixture(t, root, "internal/b.go", "package moved\n")
	if err := answerReviewed(root, whole, closure); err != nil {
		t.Fatalf("an artifact covering the spec's whole change set was refused: %v", err)
	}
}
