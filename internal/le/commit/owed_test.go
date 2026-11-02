package commit

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
	"github.com/ze-software/ze/internal/le/verify"
)

// The derivation's tests run over a real git checkout with a real bare
// upstream, because every answer the derivation gives is a git question
// (`@{u}`, `HEAD^{tree}`, ancestry, `rev-list`, `log`, `patch-id`). A stubbed
// git would test the stub. The trailers are written into commit messages by
// hand, in the spellings the spec's Person-act trailers table declares, so the
// fixture states the contract the reader is held to:
//
//	Review-owed: <spec stem>: <reason>
//	RFC-approval-owed: <package>.<Test>: <reason>
//	RFC-approved: <package>.<Test>: <reason>
//	Owner-attested: <stable patch-id>: <owner words>
//	Ledger-carried: <full sha> <stable patch-id> <gate key>: <reason>

const (
	owedSpecPath   = "plan/immediate/spec-owed-thing.md"
	owedSpecStem   = "owed-thing"
	owedTaggedPath = "pkg/thing_test.go"
	owedTaggedUnit = "pkg.TestThing"
)

// TestPushTargetIsHeadWithNoVerifyRecord proves the push owes no local verify:
// one local commit, no record of any `./le verify` run, and P is HEAD, nothing
// is owed, and the push is admitted. GitHub CI verifies the pushed commit.
// VALIDATES: the owner rule that no agent runs `./le verify` and a push does
// not depend on a local verify record (ai/META-DECISIONS.md).
// PREVENTS: a push refused until an agent runs `./le verify changed`.
func TestPushTargetIsHeadWithNoVerifyRecord(t *testing.T) {
	root := newOwedRepository(t)
	head := commitOwed(t, root, "a change", map[string]string{"a.go": "package a\n"}, nil)

	derived := deriveOwed(t, root, "")
	if derived.Target != pushTargetHead || derived.TargetSHA != head {
		t.Fatalf("target %d %q, want HEAD %q", derived.Target, derived.TargetSHA, head)
	}
	if len(derived.Items) != 0 || len(derived.Refusals) != 0 {
		t.Fatalf("a HEAD with no verify record owes something: %+v", derived)
	}
	if shas := owedSHAs(derived.Published); !slices.Equal(shas, []string{head}) {
		t.Fatalf("published %v, want exactly the local commit %s", shas, head)
	}
	if err := derived.pushRefusal(); err != nil {
		t.Fatalf("a HEAD with nothing owed refused its push: %v", err)
	}
	if derived.owed() {
		t.Fatal("a HEAD with nothing owed reads as owed")
	}
}

// TestPushTargetIgnoresAScopedRecord proves a scoped record left by an earlier
// `./le verify changed` run moves nothing: P is still HEAD, every local commit
// is published, and nothing is owed for the commits after the record.
// VALIDATES: the push gate does not read the scoped record.
func TestPushTargetIgnoresAScopedRecord(t *testing.T) {
	root := newOwedRepository(t)
	first := commitOwed(t, root, "first", map[string]string{"a.txt": "a\n"}, nil)
	writeOwedGreen(t, root, first)
	second := commitOwed(t, root, "second", map[string]string{"b.go": "package b\n"}, nil)

	derived := deriveOwed(t, root, "")
	if derived.Target != pushTargetHead || derived.TargetSHA != second {
		t.Fatalf("target %d %q, want HEAD %q", derived.Target, derived.TargetSHA, second)
	}
	shas := owedSHAs(derived.Published)
	slices.Sort(shas)
	want := []string{first, second}
	slices.Sort(want)
	if !slices.Equal(shas, want) {
		t.Fatalf("published %v, want both local commits %v", shas, want)
	}
	if derived.owed() || derived.pushRefusal() != nil {
		t.Fatalf("a commit after a scoped record is owed: %+v", derived)
	}
}

// TestSpecWorkTrailerOwesNothingAtPush pins that the attribution is a record
// with no person act behind it.
// VALIDATES: AC-6; a commit carrying `Spec-work: <stem>` derives no
// malformed-trailer refusal and no owed item, and its push is admitted.
// PREVENTS: R-2, the push parser shape-checking every declared key as
// `<subject>: <reason>` and holding every push made under a claim.
func TestSpecWorkTrailerOwesNothingAtPush(t *testing.T) {
	root := newOwedRepository(t)
	commitOwed(t, root, "a change made under a claim", map[string]string{"a.txt": "a\n"}, nil,
		trailerSpecWork+"x")

	derived := deriveOwed(t, root, "")
	if len(derived.Items) != 0 || len(derived.Refusals) != 0 {
		t.Fatalf("the attribution owes something: items %+v, refusals %q", derived.Items, derived.Refusals)
	}
	if err := derived.pushRefusal(); err != nil {
		t.Fatalf("a commit carrying only the attribution refused its push: %v", err)
	}
}

// TestDerivedDebtRefusesWithoutUpstream proves "no upstream" is an error that
// names its fix, never an empty range read as nothing owed.
// VALIDATES: AC-11, R-2.
func TestDerivedDebtRefusesWithoutUpstream(t *testing.T) {
	root := newCommitRepository(t)

	_, err := deriveDebt(root, "")
	if err == nil {
		t.Fatal("a branch with no upstream was derived")
	}
	if !strings.Contains(err.Error(), "git branch --set-upstream-to") {
		t.Fatalf("the refusal %q does not name git branch --set-upstream-to", err)
	}
}

// TestDerivedDebtNotApplicableOutsideCheckout proves a tree that is not the Ze
// checkout is answered NOT-APPLICABLE with no derivation, so the site and wiki
// still push.
// VALIDATES: AC-23.
func TestDerivedDebtNotApplicableOutsideCheckout(t *testing.T) {
	root := t.TempDir()
	runCommitGit(t, root, "init", "-q")

	derived := deriveOwed(t, root, "")
	if derived.Target != pushTargetNotApplicable {
		t.Fatalf("target %d, want not-applicable outside the Ze checkout", derived.Target)
	}
}

// TestRebasedBranchPushesHead proves a rebase over a moved upstream leaves
// nothing owed: P is the rebased HEAD and both rebased local commits publish.
// VALIDATES: AC-17, with no verify record to lose.
func TestRebasedBranchPushesHead(t *testing.T) {
	root := newOwedRepository(t)
	commitOwed(t, root, "local one", map[string]string{"a.txt": "a\n"}, nil)
	commitOwed(t, root, "local two", map[string]string{"b.txt": "b\n"}, nil)

	pushFromOtherMachine(t, root, "the other machine", "other.txt")
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
		"pull", "-q", "--rebase", "--no-autostash")

	derived := deriveOwed(t, root, "")
	if derived.Target != pushTargetHead || derived.TargetSHA != revParse(t, root, "HEAD") {
		t.Fatalf("target %d %q after a rebase, want HEAD", derived.Target, derived.TargetSHA)
	}
	subjects := owedSubjects(derived.Published)
	slices.Sort(subjects)
	if !slices.Equal(subjects, []string{"local one", "local two"}) {
		t.Fatalf("published %v, want both rebased local commits", subjects)
	}
	if err := derived.pushRefusal(); err != nil {
		t.Fatalf("a rebased branch refused its push: %v", err)
	}
}

// TestDerivedDebtSkipsPulledCommits proves a commit the other machine pushed,
// pulled through origin, is never published again from here.
// VALIDATES: AC-18, D-2.
func TestDerivedDebtSkipsPulledCommits(t *testing.T) {
	root := newOwedRepository(t)
	pushFromOtherMachine(t, root, "the other machine", "other.txt")
	runCommitGit(t, root, "pull", "-q", "--ff-only")
	commitOwed(t, root, "local after pull", map[string]string{"a.txt": "a\n"}, nil)

	derived := deriveOwed(t, root, "")
	if subjects := owedSubjects(derived.Published); !slices.Equal(subjects, []string{"local after pull"}) {
		t.Fatalf("published %v, want only the local commit", subjects)
	}
}

// TestDerivedDebtRefusesMalformedTrailer proves a trailer the derivation cannot
// read holds the push instead of being skipped, which would drop the
// obligation it declares.
// VALIDATES: no silent zero over the Person-act trailers.
func TestDerivedDebtRefusesMalformedTrailer(t *testing.T) {
	root := newOwedRepository(t)
	commitOwed(t, root, "a change", map[string]string{"a.txt": "a\n"}, nil,
		"Review-owed: no stem and no reason")

	derived := deriveOwed(t, root, "")
	if !owedSaid(derived.Refusals, "malformed Review-owed: trailer") {
		t.Fatalf("refusals %q do not name the malformed trailer", derived.Refusals)
	}
	if derived.pushRefusal() == nil {
		t.Fatal("a push was admitted over a trailer nobody could read")
	}
}

// TestDerivedDebtReadsPersonActTrailers proves each owed trailer holds the push
// until one of its answers holds, and that each answer is derived at read
// time: not-applicable, closed, reviewed, approved, owner.
// VALIDATES: AC-13, AC-14, the Person-act answers table.
func TestDerivedDebtReadsPersonActTrailers(t *testing.T) {
	t.Run("review owed until attested", func(t *testing.T) {
		root := newOwedRepository(t)
		commitOwed(t, root, "the spec lands", map[string]string{owedSpecPath: specText("in-progress", "")}, nil)
		closure := commitOwed(t, root, "the spec closes", nil, []string{owedSpecPath},
			"Review-owed: "+owedSpecStem+": no reviewer free")

		derived := deriveOwed(t, root, "")
		assertOneItem(t, derived, closure, owedItemReview, owedSpecStem)
		if derived.pushRefusal() == nil {
			t.Fatal("a push with an open Review-owed item was admitted")
		}

		commitOwed(t, root, "owner attests the closure", nil, nil,
			"Owner-attested: "+stablePatchID(t, root, closure)+": Thomas reviewed it in person")
		if derived = deriveOwed(t, root, ""); len(derived.Items) != 0 {
			t.Fatalf("an Owner-attested closure is still owed: %+v", derived.Items)
		}
	})
	t.Run("review not applicable when nothing closes", func(t *testing.T) {
		root := newOwedRepository(t)
		commitOwed(t, root, "a journal row", map[string]string{"plan/journal/a.md": "# a\n"}, nil,
			"Review-owed: "+owedSpecStem+": nothing to review")
		if derived := deriveOwed(t, root, ""); len(derived.Items) != 0 {
			t.Fatalf("a commit closing no spec owes a review: %+v", derived.Items)
		}
	})
	t.Run("review closed by a recorded Review Gate", func(t *testing.T) {
		root := newOwedRepository(t)
		commitOwed(t, root, "the spec lands", map[string]string{
			owedSpecPath: specText("in-progress", reviewGateSection("Verdict", "clean")),
		}, nil)
		commitOwed(t, root, "the spec closes", nil, []string{owedSpecPath},
			"Review-owed: "+owedSpecStem+": gate recorded in the spec")
		if derived := deriveOwed(t, root, ""); len(derived.Items) != 0 {
			t.Fatalf("a closure whose spec records its Review Gate is owed: %+v", derived.Items)
		}
	})
	t.Run("review answered by a clean artifact over the commit's blobs", func(t *testing.T) {
		const code = "pkg/thing.go"
		root := newOwedRepository(t)
		commitOwed(t, root, "the spec lands", map[string]string{owedSpecPath: specText("in-progress", "")}, nil)
		commitOwed(t, root, "the spec closes with code",
			map[string]string{code: "package pkg\n\nfunc Thing() {}\n"}, []string{owedSpecPath},
			"Review-owed: "+owedSpecStem+": reviewed later")
		if derived := deriveOwed(t, root, ""); len(derived.Items) != 1 {
			t.Fatalf("items %+v before any artifact, want the review owed", derived.Items)
		}
		writeCommitFixture(t, root, "tmp/review/"+owedSpecStem+"-fixture.md", reviewArtifact("clean",
			map[string]string{code: reviewHash(filepath.Join(root, filepath.FromSlash(code)))}))
		if derived := deriveOwed(t, root, ""); len(derived.Items) != 0 {
			t.Fatalf("a clean artifact over the commit's own blobs left the review owed: %+v", derived.Items)
		}
	})
	t.Run("rfc approval owed until approved", func(t *testing.T) {
		root := newOwedRepository(t)
		commitOwed(t, root, "the tagged test lands", map[string]string{owedTaggedPath: taggedThingText("check(1)")}, nil)
		changed := commitOwed(t, root, "the tagged test changes",
			map[string]string{owedTaggedPath: taggedThingText("check(2)")}, nil,
			"RFC-approval-owed: "+owedTaggedUnit+": owner away")

		derived := deriveOwed(t, root, "")
		assertOneItem(t, derived, changed, owedItemRFCApproval, owedTaggedUnit)

		commitOwed(t, root, "owner approves", nil, nil,
			"RFC-approved: "+owedTaggedUnit+": Thomas approved the new assertion")
		if derived = deriveOwed(t, root, ""); len(derived.Items) != 0 {
			t.Fatalf("an RFC-approved unit is still owed: %+v", derived.Items)
		}
	})
	t.Run("rfc approval not applicable when no tagged unit moves", func(t *testing.T) {
		root := newOwedRepository(t)
		commitOwed(t, root, "a prose edit", map[string]string{"docs/note.md": "# note\n"}, nil,
			"RFC-approval-owed: "+owedTaggedUnit+": nothing tagged moved")
		if derived := deriveOwed(t, root, ""); len(derived.Items) != 0 {
			t.Fatalf("a commit moving no tagged unit owes an approval: %+v", derived.Items)
		}
	})
}

// TestOwnerAttestationSurvivesRebase proves the attestation is keyed by the
// stable patch-id: a clean rebase keeps it, a rebase whose conflict resolution
// changes the diff owes again.
// VALIDATES: AC-15, AC-16, A-6.
func TestOwnerAttestationSurvivesRebase(t *testing.T) {
	root := newOwedRepository(t)
	commitOwed(t, root, "the spec lands", map[string]string{owedSpecPath: specText("in-progress", "")}, nil)
	closure := commitOwed(t, root, "the spec closes", map[string]string{"shared.txt": "mine\n"}, []string{owedSpecPath},
		"Review-owed: "+owedSpecStem+": no reviewer")
	commitOwed(t, root, "owner attests", nil, nil,
		"Owner-attested: "+stablePatchID(t, root, closure)+": Thomas read it")

	pushFromOtherMachine(t, root, "an unrelated upstream change", "elsewhere.txt")
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
		"pull", "-q", "--rebase", "--no-autostash")
	if derived := deriveOwed(t, root, ""); len(derived.Items) != 0 {
		t.Fatalf("a clean rebase lost the attestation: %+v", derived.Items)
	}

	// The diff changes: the closure is rewritten with other content, as a
	// conflict resolution would, and the attestation no longer names it.
	other := newOwedRepository(t)
	commitOwed(t, other, "the spec lands", map[string]string{owedSpecPath: specText("in-progress", "")}, nil)
	before := commitOwed(t, other, "the spec closes", map[string]string{"shared.txt": "mine\n"}, []string{owedSpecPath},
		"Review-owed: "+owedSpecStem+": no reviewer")
	attestedID := stablePatchID(t, other, before)
	runCommitGit(t, other, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
		"reset", "-q", "--soft", "HEAD^")
	resolved := commitOwed(t, other, "the spec closes", map[string]string{"shared.txt": "resolved differently\n"}, nil,
		"Review-owed: "+owedSpecStem+": no reviewer")
	commitOwed(t, other, "owner attests", nil, nil, "Owner-attested: "+attestedID+": Thomas read it")
	derived := deriveOwed(t, other, "")
	assertOneItem(t, derived, resolved, owedItemReview, owedSpecStem)
}

// TestDebtStatusCommitRefusesCommitOffBranch proves a named candidate that is
// not on HEAD's branch is refused, so a commit that left the branch is never
// published, and that a named candidate on the branch is admissible whatever
// follows it.
// VALIDATES: AC-6, AC-25.
func TestDebtStatusCommitRefusesCommitOffBranch(t *testing.T) {
	root := newOwedRepository(t)
	runCommitGit(t, root, "checkout", "-q", "-b", "side")
	side := commitOwed(t, root, "a side commit", map[string]string{"side.txt": "s\n"}, nil)
	runCommitGit(t, root, "checkout", "-q", "main")
	commitOwed(t, root, "on main", map[string]string{"main.txt": "m\n"}, nil)

	derived := deriveOwed(t, root, side)
	if derived.Target != pushTargetNone {
		t.Fatalf("target %d %q, want none for a commit off HEAD's branch", derived.Target, derived.TargetSHA)
	}
	if !owedSaid(derived.Refusals, "is not on HEAD's branch") {
		t.Fatalf("refusals %q do not say the commit is off the branch", derived.Refusals)
	}
	if derived.pushRefusal() == nil {
		t.Fatal("a commit off the branch was admitted as a push target")
	}

	onBranch := commitOwed(t, root, "named", map[string]string{"v.txt": "v\n"}, nil)
	commitOwed(t, root, "after", map[string]string{"after.txt": "a\n"}, nil)
	derived = deriveOwed(t, root, onBranch)
	if derived.Target != pushTargetNamed || derived.TargetSHA != onBranch {
		t.Fatalf("target %d %q, want the named commit %q", derived.Target, derived.TargetSHA, onBranch)
	}
	if err := derived.pushRefusal(); err != nil {
		t.Fatalf("an admissible named commit was refused: %v", err)
	}

	derived = deriveOwed(t, root, "no-such-revision")
	if derived.Target != pushTargetNone || !owedSaid(derived.Refusals, "does not resolve") {
		t.Fatalf("an unresolvable candidate: target %d refusals %q", derived.Target, derived.Refusals)
	}
}

// TestPushRefusedWhenUpstreamMovedOrNothingAhead proves the two refusals that
// are not person acts: an upstream P does not fast-forward, and nothing ahead
// of the upstream. Neither names a verify.
// VALIDATES: AC-33.
func TestPushRefusedWhenUpstreamMovedOrNothingAhead(t *testing.T) {
	root := newOwedRepository(t)
	commitOwed(t, root, "local", map[string]string{"a.txt": "a\n"}, nil)
	pushFromOtherMachine(t, root, "upstream moved", "moved.txt")
	runCommitGit(t, root, "fetch", "-q")

	derived := deriveOwed(t, root, "")
	if !owedSaid(derived.Refusals, "git pull --rebase --no-autostash") ||
		!owedSaid(derived.Refusals, "./le commit push") {
		t.Fatalf("refusals %q do not name the pull and the push", derived.Refusals)
	}
	if owedSaid(derived.Refusals, "le verify") {
		t.Fatalf("refusals %q name a verify", derived.Refusals)
	}
	if derived.pushRefusal() == nil {
		t.Fatal("a push that does not fast-forward the upstream was admitted")
	}

	level := newOwedRepository(t)
	derived = deriveOwed(t, level, "")
	if derived.Target != pushTargetHead || len(derived.Published) != 0 {
		t.Fatalf("target %d published %+v, want HEAD with nothing ahead", derived.Target, derived.Published)
	}
	err := derived.pushRefusal()
	if err == nil {
		t.Fatal("a push with nothing ahead of the upstream was admitted")
	}
	if !strings.Contains(err.Error(), "nothing is ahead of the upstream") || strings.Contains(err.Error(), "le verify") {
		t.Fatalf("the refusal %q does not say nothing is ahead, or names a verify", err)
	}
}

// TestCarriedRowOnUnpushedCommitGatesPush proves the D-8 rule: a carried row on
// an unpublished commit, keyed by its patch-id, holds the push; a carried row
// with no patch-id is audit only.
// VALIDATES: AC-26, R-11, D-8.
func TestCarriedRowOnUnpushedCommitGatesPush(t *testing.T) {
	root := newOwedRepository(t)
	commitOwed(t, root, "the spec lands", map[string]string{owedSpecPath: specText("in-progress", "")}, nil)
	closure := commitOwed(t, root, "the spec closes", nil, []string{owedSpecPath})
	commitOwed(t, root, "the ledger is deleted", nil, nil,
		"Ledger-carried: "+closure+" "+stablePatchID(t, root, closure)+" review-override: no reviewer",
		"Ledger-carried: 0123456789abcdef0123456789abcdef01234567 review-override: published long ago")

	derived := deriveOwed(t, root, "")
	assertOneItem(t, derived, closure, owedItemReview, owedSpecStem)
	if derived.pushRefusal() == nil {
		t.Fatal("a carried row on an unpushed commit did not hold the push")
	}

	commitOwed(t, root, "owner attests the carried closure", nil, nil,
		"Owner-attested: "+stablePatchID(t, root, closure)+": Thomas reviewed it")
	if derived = deriveOwed(t, root, ""); len(derived.Items) != 0 {
		t.Fatalf("an attested carried row is still owed: %+v", derived.Items)
	}
}

// newOwedRepository is a Ze checkout on branch main whose upstream is a bare
// repository holding the baseline commit.
func newOwedRepository(t *testing.T) string {
	t.Helper()
	root := newCommitRepository(t)
	// A generated commit script runs plain `git commit`, so the fixture
	// carries its own author: the test must not depend on the runner having a
	// git identity (CI has none).
	configureCommitAuthor(t, root)
	remote := filepath.Join(t.TempDir(), "origin.git")
	runCommitGit(t, root, "init", "-q", "--bare", "--initial-branch=main", remote)
	runCommitGit(t, root, "branch", "-M", "main")
	runCommitGit(t, root, "remote", "add", "origin", remote)
	runCommitGit(t, root, "push", "-q", "-u", "origin", "main")
	return root
}

// commitOwed writes and removes paths and commits them, with each trailer as a
// line of the message's last paragraph, and answers the SHA. A commit naming
// no path is empty, which is the shape of an attestation commit.
func commitOwed(t *testing.T, root, subject string, write map[string]string, remove []string, trailers ...string) string {
	t.Helper()
	for path, content := range write {
		writeCommitFixture(t, root, path, content)
		runCommitGit(t, root, "add", "--", path)
	}
	for _, path := range remove {
		runCommitGit(t, root, "rm", "-q", "--", path)
	}
	message := subject
	if len(trailers) > 0 {
		message += "\n\n" + strings.Join(trailers, "\n")
	}
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
		"commit", "-q", "--allow-empty", "-m", message)
	return revParse(t, root, "HEAD")
}

// pushFromOtherMachine clones root's upstream, commits one file there and
// pushes it, which is how a commit from the other machine reaches origin.
func pushFromOtherMachine(t *testing.T, root, subject, path string) {
	t.Helper()
	remote := strings.TrimSpace(runCommitGitOutput(t, root, "remote", "get-url", "origin"))
	clone := filepath.Join(t.TempDir(), "other")
	runCommitGit(t, root, "clone", "-q", "--branch", "main", remote, clone)
	commitOwed(t, clone, subject, map[string]string{path: subject + "\n"}, nil)
	runCommitGit(t, clone, "push", "-q", "origin", "main")
}

// scopedRecordFor is a green scoped record over commit, whose tree is tree,
// anchored where a run at root would anchor it: merge-base(commit, @{u}). A
// commit the checkout cannot reach is anchored at @{u} itself.
func scopedRecordFor(t *testing.T, root, commit, tree string) (verify.ScopedRecord, error) {
	t.Helper()
	upstream := strings.TrimSpace(runCommitGitOutputOr(root, "rev-parse", "@{u}"))
	if upstream == "" {
		// A checkout with no upstream: the record still needs an anchor, and
		// the derivation refuses on the missing upstream before reading it.
		upstream = commit
	}
	anchor := upstream
	command := exec.CommandContext(t.Context(), "git", "merge-base", commit, upstream) //nolint:gosec // test fixture revisions
	command.Dir = root
	if out, err := command.Output(); err == nil {
		anchor = strings.TrimSpace(string(out))
	}
	return verify.NewScopedRecord(commit, tree,
		repochanged.Anchor{Commit: anchor, Upstream: "@{u}", UpstreamCommit: upstream}, time.Now())
}

// writeOwedGreen leaves the record a green `./le verify changed` run over
// commit writes at root, which the push gate must not read.
func writeOwedGreen(t *testing.T, root, commit string) {
	t.Helper()
	record, err := scopedRecordFor(t, root, commit, revParse(t, root, commit+"^{tree}"))
	if err != nil {
		t.Fatal(err)
	}
	if err := verify.WriteScopedRecord(root, record); err != nil {
		t.Fatal(err)
	}
}

// deriveOwed derives at root and fails the test on "cannot derive".
func deriveOwed(t *testing.T, root, candidate string) derivedDebt {
	t.Helper()
	derived, err := deriveDebt(root, candidate)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	return derived
}

func assertOneItem(t *testing.T, derived derivedDebt, commit string, kind owedItemKind, subject string) {
	t.Helper()
	if len(derived.Items) != 1 {
		t.Fatalf("items %+v, want exactly one on %s", derived.Items, commit)
	}
	item := derived.Items[0]
	if item.Commit != commit || item.Kind != kind || item.Subject != subject {
		t.Fatalf("item %+v, want commit %s kind %d subject %q", item, commit, kind, subject)
	}
}

func revParse(t *testing.T, root, revision string) string {
	t.Helper()
	return strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", revision))
}

// stablePatchID answers `git patch-id --stable` of one commit's diff.
func stablePatchID(t *testing.T, root, commit string) string {
	t.Helper()
	show := runCommitGitOutput(t, root, "show", commit)
	command := exec.CommandContext(t.Context(), "git", "patch-id", "--stable")
	command.Dir = root
	command.Stdin = bytes.NewReader([]byte(show))
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git patch-id: %v", err)
	}
	fields := strings.Fields(string(output))
	if len(fields) == 0 {
		t.Fatalf("git patch-id answered nothing for %s", commit)
	}
	return fields[0]
}

// taggedThingText is a test file whose one function carries an RFC
// requirement tag inside its body, so editing the call moves a tagged unit.
func taggedThingText(call string) string {
	return "package pkg\n\nfunc TestThing(t *testing.T) {\n" +
		"\t// RFC requirement: rfc9999-1 positive\n\t" + call + "\n}\n"
}

func owedSHAs(commits []owedCommit) []string {
	shas := make([]string, 0, len(commits))
	for _, commit := range commits {
		shas = append(shas, commit.SHA)
	}
	return shas
}

func owedSubjects(commits []owedCommit) []string {
	subjects := make([]string, 0, len(commits))
	for _, commit := range commits {
		subjects = append(subjects, commit.Subject)
	}
	return subjects
}

func owedSaid(lines []string, want string) bool {
	return slices.ContainsFunc(lines, func(line string) bool { return strings.Contains(line, want) })
}

// TestNoPushSurfaceNamesVerify proves no answer of the push gate tells an agent
// to run a verify: the derivation's refusals, the push refusal, the
// session-start warning, the create report and debt-status, over a HEAD
// with nothing owed, an upstream that moved, nothing ahead, and an open
// person act.
// VALIDATES: the owner rule that no agent runs any `./le verify` command.
// PREVENTS: a tool message that sends an agent to `./le verify changed`.
func TestNoPushSurfaceNamesVerify(t *testing.T) {
	cases := map[string]func(t *testing.T) string{
		"nothing owed": func(t *testing.T) string {
			t.Helper()
			root := newOwedRepository(t)
			commitOwed(t, root, "a change", map[string]string{"a.go": "package a\n"}, nil)
			return root
		},
		"upstream moved": func(t *testing.T) string {
			t.Helper()
			root := newOwedRepository(t)
			commitOwed(t, root, "a change", map[string]string{"a.go": "package a\n"}, nil)
			pushFromOtherMachine(t, root, "upstream moved", "moved.txt")
			runCommitGit(t, root, "fetch", "-q")
			return root
		},
		"nothing ahead": newOwedRepository,
		"person act owed": func(t *testing.T) string {
			t.Helper()
			root := newOwedRepository(t)
			owedClosure(t, root)
			return root
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			root := build(t)
			derived := deriveOwed(t, root, "")
			texts := slices.Clone(derived.Refusals)
			if err := derived.pushRefusal(); err != nil {
				texts = append(texts, err.Error())
			}
			warning, err := DebtWarning(root)
			if err != nil {
				t.Fatal(err)
			}
			status, _, err := answerDebtStatus(root, "")
			if err != nil {
				t.Fatal(err)
			}
			texts = append(texts, warning, debtReport(root), status.Text())
			for _, text := range texts {
				if strings.Contains(text, "le verify") || strings.Contains(text, "scoped") {
					t.Errorf("a push surface tells an agent to verify: %q", text)
				}
			}
		})
	}
}

// runCommitGitOutputOr answers git's output at root, or empty when git fails.
func runCommitGitOutputOr(root string, args ...string) string {
	command := exec.Command("git", args...) //nolint:gosec,noctx // test fixture revisions
	command.Dir = root
	out, err := command.Output()
	if err != nil {
		return ""
	}
	return string(out)
}
