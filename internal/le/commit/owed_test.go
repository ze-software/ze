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

// TestDerivedDebtEmptyWhenTreesMatch proves the clear answer: one local commit,
// a green record over it, and HEAD's tree is the green tree.
// VALIDATES: AC-3; P is HEAD, nothing left behind, nothing owed, the push admitted.
func TestDerivedDebtEmptyWhenTreesMatch(t *testing.T) {
	root := newOwedRepository(t)
	head := commitOwed(t, root, "a verified change", map[string]string{"a.txt": "a\n"}, nil)
	writeOwedGreen(t, root, head)

	derived := deriveOwed(t, root, "")
	if derived.Target != pushTargetHead || derived.TargetSHA != head {
		t.Fatalf("target %d %q, want HEAD %q", derived.Target, derived.TargetSHA, head)
	}
	if len(derived.LeftBehind) != 0 || len(derived.Items) != 0 || len(derived.Refusals) != 0 {
		t.Fatalf("a verified HEAD owes something: %+v", derived)
	}
	if shas := owedSHAs(derived.Published); !slices.Equal(shas, []string{head}) {
		t.Fatalf("published %v, want exactly the verified commit %s", shas, head)
	}
	if err := derived.pushRefusal(); err != nil {
		t.Fatalf("a verified HEAD with nothing owed refused its push: %v", err)
	}
}

// TestDerivedDebtListsCommitsAfterGreen proves a commit after the green record
// is named, by SHA and subject, as not in this push.
// VALIDATES: AC-4; P is G, and the later commit is listed as left behind.
func TestDerivedDebtListsCommitsAfterGreen(t *testing.T) {
	root := newOwedRepository(t)
	green := commitOwed(t, root, "the verified change", map[string]string{"a.txt": "a\n"}, nil)
	writeOwedGreen(t, root, green)
	later := commitOwed(t, root, "a change after the verify", map[string]string{"b.txt": "b\n"}, nil)

	derived := deriveOwed(t, root, "")
	if derived.Target != pushTargetRecord || derived.TargetSHA != green {
		t.Fatalf("target %d %q, want the green commit %q", derived.Target, derived.TargetSHA, green)
	}
	if len(derived.LeftBehind) != 1 || derived.LeftBehind[0].SHA != later ||
		derived.LeftBehind[0].Subject != "a change after the verify" {
		t.Fatalf("left behind %+v, want exactly %s with its subject", derived.LeftBehind, later)
	}
}

// TestPushTargetIsGreenCommitWhenHeadMovedOn proves D-6: HEAD moving on after
// the verify does not void it, the push publishes G and only G's range.
// VALIDATES: AC-4, D-6.
func TestPushTargetIsGreenCommitWhenHeadMovedOn(t *testing.T) {
	root := newOwedRepository(t)
	first := commitOwed(t, root, "first", map[string]string{"a.txt": "a\n"}, nil)
	green := commitOwed(t, root, "second", map[string]string{"b.txt": "b\n"}, nil)
	writeOwedGreen(t, root, green)
	commitOwed(t, root, "third", map[string]string{"c.txt": "c\n"}, nil)
	commitOwed(t, root, "fourth", map[string]string{"d.txt": "d\n"}, nil)

	derived := deriveOwed(t, root, "")
	if derived.Target != pushTargetRecord || derived.TargetSHA != green {
		t.Fatalf("target %d %q, want G %q", derived.Target, derived.TargetSHA, green)
	}
	if shas := owedSHAs(derived.Published); !slices.Equal(shas, []string{green, first}) &&
		!slices.Equal(shas, []string{first, green}) {
		t.Fatalf("published %v, want exactly %s and %s", shas, first, green)
	}
	if len(derived.LeftBehind) != 2 {
		t.Fatalf("left behind %+v, want the two commits after G", derived.LeftBehind)
	}
	if err := derived.pushRefusal(); err != nil {
		t.Fatalf("a push of G with nothing owed was refused: %v", err)
	}
}

// TestDerivedDebtReportsTreeMismatchWithEmptyList proves the derivation half of
// Data Flow step 6: HEAD is the upstream (nothing local to list) and the green
// record names another tree, so the derivation finds no target, lists nothing,
// records a refusal naming ./le verify worktree, and refuses the push.
// VALIDATES: Data Flow step 6, the derivation's fields and pushRefusal only.
// The surfaces that print the answer are TestDebtConsumersNeverSayZeroWithoutTarget.
func TestDerivedDebtReportsTreeMismatchWithEmptyList(t *testing.T) {
	root := newOwedRepository(t)
	other := newOwedRepository(t)
	foreign := commitOwed(t, other, "a commit this checkout never saw", map[string]string{"x.txt": "x\n"}, nil)
	record, err := scopedRecordFor(t, root, foreign, revParse(t, other, foreign+"^{tree}"))
	if err != nil {
		t.Fatal(err)
	}
	if err := verify.WriteScopedRecord(root, record); err != nil {
		t.Fatal(err)
	}

	derived := deriveOwed(t, root, "")
	if derived.Target != pushTargetNone {
		t.Fatalf("target %d, want none: HEAD's tree is not the green tree", derived.Target)
	}
	if len(derived.LeftBehind) != 0 {
		t.Fatalf("left behind %+v, want none: HEAD is the upstream", derived.LeftBehind)
	}
	if !owedSaid(derived.Refusals, "./le verify changed") {
		t.Fatalf("refusals %q do not name ./le verify changed", derived.Refusals)
	}
	if derived.pushRefusal() == nil {
		t.Fatal("a push was admitted with no verified target")
	}
}

// TestDerivedDebtRefusesWithoutUpstream proves "no upstream" is an error that
// names its fix, never an empty range read as nothing owed.
// VALIDATES: AC-11, R-2.
func TestDerivedDebtRefusesWithoutUpstream(t *testing.T) {
	root := newCommitRepository(t)
	head := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
	writeOwedGreen(t, root, head)

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

// TestDerivedDebtRebaseReowesEveryLocalCommit proves a rebase over a moved
// upstream re-owes every local commit: G leaves the branch, so no target.
// VALIDATES: AC-17.
func TestDerivedDebtRebaseReowesEveryLocalCommit(t *testing.T) {
	root := newOwedRepository(t)
	commitOwed(t, root, "local one", map[string]string{"a.txt": "a\n"}, nil)
	green := commitOwed(t, root, "local two", map[string]string{"b.txt": "b\n"}, nil)
	writeOwedGreen(t, root, green)

	pushFromOtherMachine(t, root, "the other machine", "other.txt")
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
		"pull", "-q", "--rebase", "--no-autostash")

	derived := deriveOwed(t, root, "")
	if derived.Target != pushTargetNone {
		t.Fatalf("target %d %q after a rebase, want none", derived.Target, derived.TargetSHA)
	}
	subjects := owedSubjects(derived.LeftBehind)
	slices.Sort(subjects)
	if !slices.Equal(subjects, []string{"local one", "local two"}) {
		t.Fatalf("listed %v, want both rebased local commits", subjects)
	}
}

// TestDerivedDebtSkipsPulledCommits proves a commit the other machine pushed,
// pulled through origin, is never owed here.
// VALIDATES: AC-18, D-2.
func TestDerivedDebtSkipsPulledCommits(t *testing.T) {
	root := newOwedRepository(t)
	pushFromOtherMachine(t, root, "the other machine", "other.txt")
	runCommitGit(t, root, "pull", "-q", "--ff-only")
	commitOwed(t, root, "local after pull", map[string]string{"a.txt": "a\n"}, nil)

	derived := deriveOwed(t, root, "")
	if subjects := owedSubjects(derived.LeftBehind); !slices.Equal(subjects, []string{"local after pull"}) {
		t.Fatalf("listed %v, want only the local commit", subjects)
	}
}

// TestDerivedDebtProseOnlyCommitIsCovered proves the coverage rule (D-9): a
// commit changing only Markdown under docs/, ai/ or plan/ after the green
// record owes nothing, so P is HEAD with no new verify. The same commit also
// touching a root-level page, a .ci body, the RFC corpus or Go is owed: P
// stops before it, a prose commit before it is still published with G, and a
// prose commit after it is not listed as owing a verify.
// VALIDATES: AC-19, D-9.
func TestDerivedDebtProseOnlyCommitIsCovered(t *testing.T) {
	t.Run("prose only", func(t *testing.T) {
		root := newOwedRepository(t)
		green := commitOwed(t, root, "code", map[string]string{"a.go": "package a\n"}, nil)
		writeOwedGreen(t, root, green)
		commitOwed(t, root, "prose only", map[string]string{
			"docs/page.md": "# page\n", "ai/rules/r.md": "# r\n", "plan/s.md": "# s\n",
		}, nil)

		derived := deriveOwed(t, root, "")
		if derived.Target != pushTargetHead || derived.TargetSHA != revParse(t, root, "HEAD") {
			t.Fatalf("target %d %q, want HEAD: a prose-only commit owes no verify", derived.Target, derived.TargetSHA)
		}
		if len(derived.LeftBehind) != 0 {
			t.Fatalf("left behind %+v, want none", derived.LeftBehind)
		}
		if err := derived.pushRefusal(); err != nil {
			t.Fatalf("a prose-only commit after the green record refused the push: %v", err)
		}
	})
	for _, owed := range []string{"AGENTS.md", "test/runner/a.ci", "rfc/short/rfc4271.md", "pkg/a.go", "docs/x.png"} {
		t.Run(owed, func(t *testing.T) {
			root := newOwedRepository(t)
			green := commitOwed(t, root, "code", map[string]string{"a.go": "package a\n"}, nil)
			writeOwedGreen(t, root, green)
			prose := commitOwed(t, root, "prose before", map[string]string{"docs/before.md": "# b\n"}, nil)
			mixed := commitOwed(t, root, "prose and "+owed, map[string]string{"docs/page.md": "# p\n", owed: "x\n"}, nil)
			commitOwed(t, root, "prose after", map[string]string{"plan/after.md": "# a\n"}, nil)

			derived := deriveOwed(t, root, "")
			if derived.Target != pushTargetRecord || derived.TargetSHA != prose {
				t.Fatalf("target %d %q, want the prose commit %q before the owed one", derived.Target, derived.TargetSHA, prose)
			}
			if shas := owedSHAs(derived.LeftBehind); !slices.Equal(shas, []string{mixed}) {
				t.Fatalf("left behind %v, want exactly %s (the prose commit after it owes nothing)", shas, mixed)
			}
		})
	}
}

// TestDerivedDebtDiffFailureIsNotCovered proves a comparison git cannot make
// is never read as an empty diff: the green record names a real commit whose
// recorded tree is not in the repository, so every `git diff` against it
// fails. Read as empty, HEAD would be covered and pushed unverified.
// VALIDATES: AC-35.
func TestDerivedDebtDiffFailureIsNotCovered(t *testing.T) {
	root := newOwedRepository(t)
	head := commitOwed(t, root, "a change", map[string]string{"a.go": "package a\n"}, nil)
	record, err := scopedRecordFor(t, root, head, "0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatal(err)
	}
	if err := verify.WriteScopedRecord(root, record); err != nil {
		t.Fatal(err)
	}

	derived := deriveOwed(t, root, "")
	if derived.Target != pushTargetNone {
		t.Fatalf("target %d %q, want none: no comparison with the green tree succeeded", derived.Target, derived.TargetSHA)
	}
	if !owedSaid(derived.Refusals, "cannot compare") || !owedSaid(derived.Refusals, "git -c core.quotePath=false diff") {
		t.Fatalf("refusals %q do not name the failed git diff", derived.Refusals)
	}
	if derived.pushRefusal() == nil {
		t.Fatal("a push was admitted over a diff git could not compute")
	}
}

// TestDerivedDebtRefusesMalformedTrailer proves a trailer the derivation cannot
// read holds the push instead of being skipped, which would drop the
// obligation it declares.
// VALIDATES: no silent zero over the Person-act trailers.
func TestDerivedDebtRefusesMalformedTrailer(t *testing.T) {
	root := newOwedRepository(t)
	head := commitOwed(t, root, "a change", map[string]string{"a.txt": "a\n"}, nil,
		"Review-owed: no stem and no reason")
	writeOwedGreen(t, root, head)

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
		writeOwedGreen(t, root, closure)

		derived := deriveOwed(t, root, "")
		assertOneItem(t, derived, closure, owedItemReview, owedSpecStem)
		if derived.pushRefusal() == nil {
			t.Fatal("a push with an open Review-owed item was admitted")
		}

		attest := commitOwed(t, root, "owner attests the closure", nil, nil,
			"Owner-attested: "+stablePatchID(t, root, closure)+": Thomas reviewed it in person")
		writeOwedGreen(t, root, attest)
		if derived = deriveOwed(t, root, ""); len(derived.Items) != 0 {
			t.Fatalf("an Owner-attested closure is still owed: %+v", derived.Items)
		}
	})
	t.Run("review not applicable when nothing closes", func(t *testing.T) {
		root := newOwedRepository(t)
		plain := commitOwed(t, root, "a journal row", map[string]string{"plan/journal/a.md": "# a\n"}, nil,
			"Review-owed: "+owedSpecStem+": nothing to review")
		writeOwedGreen(t, root, plain)
		if derived := deriveOwed(t, root, ""); len(derived.Items) != 0 {
			t.Fatalf("a commit closing no spec owes a review: %+v", derived.Items)
		}
	})
	t.Run("review closed by a recorded Review Gate", func(t *testing.T) {
		root := newOwedRepository(t)
		commitOwed(t, root, "the spec lands", map[string]string{
			owedSpecPath: specText("in-progress", reviewGateSection("Verdict", "clean")),
		}, nil)
		closure := commitOwed(t, root, "the spec closes", nil, []string{owedSpecPath},
			"Review-owed: "+owedSpecStem+": gate recorded in the spec")
		writeOwedGreen(t, root, closure)
		if derived := deriveOwed(t, root, ""); len(derived.Items) != 0 {
			t.Fatalf("a closure whose spec records its Review Gate is owed: %+v", derived.Items)
		}
	})
	t.Run("review answered by a clean artifact over the commit's blobs", func(t *testing.T) {
		const code = "pkg/thing.go"
		root := newOwedRepository(t)
		commitOwed(t, root, "the spec lands", map[string]string{owedSpecPath: specText("in-progress", "")}, nil)
		closure := commitOwed(t, root, "the spec closes with code",
			map[string]string{code: "package pkg\n\nfunc Thing() {}\n"}, []string{owedSpecPath},
			"Review-owed: "+owedSpecStem+": reviewed later")
		writeOwedGreen(t, root, closure)
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
		writeOwedGreen(t, root, changed)

		derived := deriveOwed(t, root, "")
		assertOneItem(t, derived, changed, owedItemRFCApproval, owedTaggedUnit)

		approved := commitOwed(t, root, "owner approves", nil, nil,
			"RFC-approved: "+owedTaggedUnit+": Thomas approved the new assertion")
		writeOwedGreen(t, root, approved)
		if derived = deriveOwed(t, root, ""); len(derived.Items) != 0 {
			t.Fatalf("an RFC-approved unit is still owed: %+v", derived.Items)
		}
	})
	t.Run("rfc approval not applicable when no tagged unit moves", func(t *testing.T) {
		root := newOwedRepository(t)
		plain := commitOwed(t, root, "a prose edit", map[string]string{"docs/note.md": "# note\n"}, nil,
			"RFC-approval-owed: "+owedTaggedUnit+": nothing tagged moved")
		writeOwedGreen(t, root, plain)
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
	writeOwedGreen(t, root, revParse(t, root, "HEAD"))
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
	writeOwedGreen(t, other, revParse(t, other, "HEAD"))
	derived := deriveOwed(t, other, "")
	assertOneItem(t, derived, resolved, owedItemReview, owedSpecStem)
}

// TestDebtStatusCommitRefusesCommitOffBranch proves a named candidate whose
// tree is the green tree is still refused when it is not on HEAD's branch, so
// a commit that left the branch is never published.
// VALIDATES: AC-6, AC-25.
func TestDebtStatusCommitRefusesCommitOffBranch(t *testing.T) {
	root := newOwedRepository(t)
	runCommitGit(t, root, "checkout", "-q", "-b", "side")
	side := commitOwed(t, root, "a side commit", map[string]string{"side.txt": "s\n"}, nil)
	runCommitGit(t, root, "checkout", "-q", "main")
	commitOwed(t, root, "on main", map[string]string{"main.txt": "m\n"}, nil)
	writeOwedGreen(t, root, side)

	derived := deriveOwed(t, root, side)
	if derived.Target != pushTargetNone {
		t.Fatalf("target %d %q, want none for a commit off HEAD's branch", derived.Target, derived.TargetSHA)
	}
	if derived.pushRefusal() == nil {
		t.Fatal("a commit off the branch was admitted as a push target")
	}

	// The same rule on the branch: a candidate on HEAD's branch whose tree is
	// the green tree is admissible, whatever follows it.
	onBranch := commitOwed(t, root, "verified", map[string]string{"v.txt": "v\n"}, nil)
	writeOwedGreen(t, root, onBranch)
	commitOwed(t, root, "after", map[string]string{"after.txt": "a\n"}, nil)
	derived = deriveOwed(t, root, onBranch)
	if derived.Target != pushTargetNamed || derived.TargetSHA != onBranch {
		t.Fatalf("target %d %q, want the named commit %q", derived.Target, derived.TargetSHA, onBranch)
	}
	if err := derived.pushRefusal(); err != nil {
		t.Fatalf("an admissible named commit was refused: %v", err)
	}
}

// TestPushRefusedWhenUpstreamMovedOrNothingAhead proves the two refusals that
// are not owed commits: an upstream P does not fast-forward, and nothing
// verified ahead of the upstream.
// VALIDATES: AC-33.
func TestPushRefusedWhenUpstreamMovedOrNothingAhead(t *testing.T) {
	root := newOwedRepository(t)
	green := commitOwed(t, root, "verified", map[string]string{"a.txt": "a\n"}, nil)
	writeOwedGreen(t, root, green)
	pushFromOtherMachine(t, root, "upstream moved", "moved.txt")
	runCommitGit(t, root, "fetch", "-q")

	derived := deriveOwed(t, root, "")
	if !owedSaid(derived.Refusals, "git pull --rebase --no-autostash") ||
		!owedSaid(derived.Refusals, "./le verify changed") {
		t.Fatalf("refusals %q do not name the pull and the verify", derived.Refusals)
	}
	if derived.pushRefusal() == nil {
		t.Fatal("a push that does not fast-forward the upstream was admitted")
	}

	level := newOwedRepository(t)
	writeOwedGreen(t, level, revParse(t, level, "HEAD"))
	derived = deriveOwed(t, level, "")
	if derived.Target != pushTargetHead || len(derived.Published) != 0 {
		t.Fatalf("target %d published %+v, want HEAD with nothing ahead", derived.Target, derived.Published)
	}
	err := derived.pushRefusal()
	if err == nil {
		t.Fatal("a push with nothing ahead of the upstream was admitted")
	}
	if !strings.Contains(err.Error(), "nothing verified is ahead of the upstream") {
		t.Fatalf("the refusal %q does not say nothing verified is ahead", err)
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
	deletion := commitOwed(t, root, "the ledger is deleted", nil, nil,
		"Ledger-carried: "+closure+" "+stablePatchID(t, root, closure)+" review-override: no reviewer",
		"Ledger-carried: 0123456789abcdef0123456789abcdef01234567 review-override: published long ago")
	writeOwedGreen(t, root, deletion)

	derived := deriveOwed(t, root, "")
	assertOneItem(t, derived, closure, owedItemReview, owedSpecStem)
	if derived.pushRefusal() == nil {
		t.Fatal("a carried row on an unpushed commit did not hold the push")
	}

	commitOwed(t, root, "owner attests the carried closure", nil, nil,
		"Owner-attested: "+stablePatchID(t, root, closure)+": Thomas reviewed it")
	writeOwedGreen(t, root, revParse(t, root, "HEAD"))
	if derived = deriveOwed(t, root, ""); len(derived.Items) != 0 {
		t.Fatalf("an attested carried row is still owed: %+v", derived.Items)
	}
}

// newOwedRepository is a Ze checkout on branch main whose upstream is a bare
// repository holding the baseline commit.
func newOwedRepository(t *testing.T) string {
	t.Helper()
	root := newCommitRepository(t)
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

// writeOwedGreen records a green scoped verification over commit at root.
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

// TestPushAdmitsScopedRecordCoveringHead proves a scoped record over HEAD,
// anchored at the upstream, admits the push with no full verify asked (AC-15).
func TestPushAdmitsScopedRecordCoveringHead(t *testing.T) {
	root := newOwedRepository(t)
	head := commitOwed(t, root, "a scoped change", map[string]string{"a.go": "package a\n"}, nil)
	writeOwedGreen(t, root, head)
	derived := deriveOwed(t, root, "")
	if derived.Target != pushTargetHead || derived.TargetSHA != head || len(derived.Refusals) != 0 {
		t.Fatalf("a scoped record over HEAD: target %d %q refusals %q", derived.Target, derived.TargetSHA, derived.Refusals)
	}
	if err := derived.pushRefusal(); err != nil {
		t.Fatalf("a scoped record over HEAD refused its push: %v", err)
	}
}

// TestPushCarriesProseAfterScopedRecord proves prose-only commits after the
// recorded commit ride the push, as the walk always carried them (AC-16).
func TestPushCarriesProseAfterScopedRecord(t *testing.T) {
	root := newOwedRepository(t)
	verified := commitOwed(t, root, "a scoped change", map[string]string{"a.go": "package a\n"}, nil)
	writeOwedGreen(t, root, verified)
	prose := commitOwed(t, root, "a page", map[string]string{"docs/a.md": "# a\n"}, nil)
	derived := deriveOwed(t, root, "")
	if derived.TargetSHA != prose || len(derived.Refusals) != 0 {
		t.Fatalf("prose after the record: target %q refusals %q, want %s", derived.TargetSHA, derived.Refusals, prose)
	}
}

// TestPushRefusesWhenUpstreamMovedPastAnchor proves a record whose anchor is
// not an ancestor of the current @{u} proves no push (AC-17). The upstream is
// rewritten, not advanced: an advanced @{u} still holds the anchor, so only a
// rewrite reaches the anchor check, with HEAD rebuilt on the new tip so the
// fast-forward check passes and the anchor refusal stands alone.
func TestPushRefusesWhenUpstreamMovedPastAnchor(t *testing.T) {
	root := newOwedRepository(t)
	anchor := revParse(t, root, "@{u}")
	// The upstream is rewritten under the run: its new tip shares no history
	// with the anchor, and the branch is rebuilt on that tip, so the push still
	// fast-forwards @{u} and only the anchor fails.
	rewritten := strings.TrimSpace(runCommitGitOutputOr(root, "-c", "user.email=t@t", "-c", "user.name=t",
		"commit-tree", anchor+"^{tree}", "-m", "the rewritten upstream"))
	if rewritten == "" {
		t.Fatal("git commit-tree made no commit")
	}
	runCommitGit(t, root, "update-ref", "refs/remotes/origin/main", rewritten)
	runCommitGit(t, root, "checkout", "-q", "-B", "main", rewritten)
	head := commitOwed(t, root, "a scoped change", map[string]string{"a.go": "package a\n"}, nil)
	upstream := revParse(t, root, "@{u}")
	if upstream != rewritten || upstream == anchor {
		t.Fatalf("@{u} is %s, want the rewritten tip %s, not the anchor %s", upstream, rewritten, anchor)
	}
	if exec.CommandContext(t.Context(), "git", "-C", root, "merge-base", "--is-ancestor", anchor, upstream).Run() == nil { //nolint:gosec // test fixture revisions
		t.Fatalf("the anchor %s is an ancestor of @{u} %s: the fixture tests an advanced upstream", anchor, upstream)
	}
	record, err := verify.NewScopedRecord(head, revParse(t, root, head+"^{tree}"),
		repochanged.Anchor{Commit: anchor, Upstream: "@{u}", UpstreamCommit: anchor}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := verify.WriteScopedRecord(root, record); err != nil {
		t.Fatal(err)
	}
	derived := deriveOwed(t, root, "")
	refusals := strings.Join(derived.Refusals, "\n")
	for _, want := range []string{"moved past the scoped run's anchor " + shortSHA(anchor) + " without holding it",
		"is not an ancestor of " + shortSHA(upstream), pullCommand, "./le verify changed"} {
		if !strings.Contains(refusals, want) {
			t.Errorf("refusals %q do not name %q", refusals, want)
		}
	}
	if len(derived.Refusals) != 1 {
		t.Errorf("refusals %q, want the anchor refusal alone: the push fast-forwards @{u}", refusals)
	}
	if derived.pushRefusal() == nil {
		t.Error("a record whose anchor the upstream does not hold admitted the push")
	}
}

// TestPushRefusalNamesVerifyChanged proves a checkout with no scoped record is
// refused naming the scoped run, never a full verify (AC-18).
func TestPushRefusalNamesVerifyChanged(t *testing.T) {
	root := newOwedRepository(t)
	commitOwed(t, root, "an unverified change", map[string]string{"a.go": "package a\n"}, nil)
	derived := deriveOwed(t, root, "")
	err := derived.pushRefusal()
	if err == nil {
		t.Fatal("no scoped record admitted the push")
	}
	text := err.Error() + strings.Join(derived.Refusals, "\n")
	if !strings.Contains(text, "./le verify changed") {
		t.Errorf("the refusal %q does not name ./le verify changed", text)
	}
	if strings.Contains(text, "verify worktree") || strings.Contains(text, "verify current") {
		t.Errorf("the refusal %q names a full verify", text)
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
