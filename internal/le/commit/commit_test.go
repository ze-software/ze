package commit

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/spec"
	specjournal "github.com/ze-software/ze/internal/le/spec/journal"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

func TestNormalizePathRefusesNonFilePopulations(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	cases := []string{"", ".", "..", "../outside.txt", outside, ".git/config", "a\tb", "a\nb"}
	for _, raw := range cases {
		if path, err := normalizePath(root, raw); err == nil {
			t.Errorf("NormalizePath(%q) = %q, want refusal", raw, path)
		}
	}
	inside := filepath.Join(root, "dir", "file name.txt")
	if got, err := normalizePath(root, inside); err != nil || got != "dir/file name.txt" {
		t.Fatalf("NormalizePath(inside) = %q, %v", got, err)
	}
}

func TestAddAndRemoveValidationProtectExplicitStaging(t *testing.T) {
	root := newCommitRepository(t)
	writeCommitFixture(t, root, ".gitignore", "ignored.txt\n")
	writeCommitFixture(t, root, "ignored.txt", "ignored\n")
	writeCommitFixture(t, root, "plain.txt", "plain\n")
	if err := os.Mkdir(filepath.Join(root, "directory"), 0o750); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"AGENTS.md", "CLAUDE.md", "ignored.txt", "missing.txt", "directory"} {
		if err := validateAddPath(root, path); err == nil {
			t.Errorf("validateAddPath(%q) accepted a protected population", path)
		}
	}
	// A tracked file that matches an ignore pattern is committable. Git already
	// carries it, so the pattern governs new files under that path and not this
	// one. The published site tracks assets/demos/ and ignores additions to it,
	// and every republish of a tracked cast there was refused.
	writeCommitFixture(t, root, "tracked-ignored.txt", "was added before the rule\n")
	runCommitGit(t, root, "add", "-f", "--", "tracked-ignored.txt")
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "tracked before the ignore rule")
	writeCommitFixture(t, root, ".gitignore", "ignored.txt\ntracked-ignored.txt\n")
	if err := validateAddPath(root, "tracked-ignored.txt"); err != nil {
		t.Fatalf("validateAddPath refused a tracked file for an ignore rule: %v", err)
	}
	if err := validateAddPath(root, "plain.txt"); err != nil {
		t.Fatalf("validateAddPath(plain.txt): %v", err)
	}
	if err := validateRemovePath(root, "tracked.txt"); err != nil {
		t.Fatalf("validateRemovePath(tracked.txt): %v", err)
	}
	if err := validateRemovePath(root, "plain.txt"); err == nil {
		t.Fatal("validateRemovePath accepted an untracked file")
	}
}

// TestCreateAcceptsARemovalWhoseFileIsStillOnDisk is AC-11 of the closed
// spec-remove-takes-the-working-tree-copy (contract: docs/contributing/committing.md,
// "What the generated script contains", step 8): preparing a
// removal puts no condition on the working tree, and leaves the file where it
// is, because the generated script deletes it only after its commit succeeds.
// PREVENTS: a `create` that refuses, or deletes early, unless the caller ran
// `rm` first, the step the old contract demanded and closures forgot.
func TestCreateAcceptsARemovalWhoseFileIsStillOnDisk(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "removal-present-fixture")
	prepared, err := Create(root, &Options{Subject: "remove tracked.txt", Remove: []string{"tracked.txt"}})
	if err != nil {
		t.Fatalf("Create refused a removal whose file is on disk: %v", err)
	}
	if len(prepared.Removed) != 1 || prepared.Removed[0] != "tracked.txt" {
		t.Fatalf("Create did not record the removal: %v", prepared.Removed)
	}
	if _, err := os.Lstat(filepath.Join(root, "tracked.txt")); err != nil {
		t.Fatalf("Create deleted the file before any commit ran: %v", err)
	}
}

func TestMessageAndKeywordGrammarAreClosed(t *testing.T) {
	t.Parallel()
	if _, err := Message("", nil, nil); err == nil {
		t.Fatal("Message accepted an empty subject")
	}
	if _, err := Message(strings.Repeat("x", 73), nil, nil); err == nil {
		t.Fatal("Message accepted a 73-character subject")
	}
	message, err := Message("safe subject", []string{"one two three", "", "four"}, nil)
	if err != nil || message != "safe subject\n\none two three\n\nfour\n" {
		t.Fatalf("Message = %q, %v", message, err)
	}
	if _, err := parseCreate([]string{"subject", "x", "file", "a", "mystery", "b"}); err == nil {
		t.Fatal("parseCreate accepted an open keyword")
	}
	if _, err := parseCreate([]string{"subject", "x", "append", "append"}); err == nil {
		t.Fatal("parseCreate accepted a duplicate switch")
	}
}

func TestGoSourceRequiresTestUnlessExplicitlyExempt(t *testing.T) {
	root := t.TempDir()
	writeCommitFixture(t, root, "internal/a.go", "// Design: docs/a.md -- a\npackage a\n")
	if problems := testCoverageProblems(root, []string{"internal/a.go"}); len(problems) != 1 {
		t.Fatalf("source-only coverage problems = %q", problems)
	}
	writeCommitFixture(t, root, "internal/a_test.go", "package a\n")
	if problems := testCoverageProblems(root, []string{"internal/a.go", "internal/a_test.go"}); len(problems) != 0 {
		t.Fatalf("source plus test coverage problems = %q", problems)
	}
	for _, path := range []string{"internal/register.go", "cmd/le/main.go", "vendor/example/a.go"} {
		writeCommitFixture(t, root, path, "package p\n")
		if problems := testCoverageProblems(root, []string{path}); len(problems) != 0 {
			t.Errorf("exempt %s problems = %q", path, problems)
		}
	}
}

func TestStructuralRedsAttributeOnlyPathBearingGroups(t *testing.T) {
	root := t.TempDir()
	writeCommitFixture(t, root, "mine/a.go", "package mine\n")
	writeCommitFixture(t, root, "theirs/b.go", "package theirs\n")
	writeCommitFixture(t, root, "tmp/ze-verify-failures.json", `{"stages":[
		{"stage":"go lint/run","exit-code":1,"groups":[
			{"group-id":"lint:theirs","kind":"lint","related":["theirs/b.go"]}]},
		{"stage":"arch tier/check","exit-code":1,"groups":[
			{"group-id":"tier:unknown","kind":"subcheck","related":["theirs/b.go"]}]},
		{"stage":"doc wiring","exit-code":1,"groups":[
			{"group-id":"files:mine","kind":"files","related":["mine/a.go"]}]}
	]}`)
	reds := structuralGateReds(root, []string{"mine/a.go"})
	if !slices.Equal(reds.Charged, []string{"arch tier/check", "doc wiring"}) ||
		!slices.Equal(reds.Foreign, []string{"go lint/run"}) ||
		!slices.Equal(reds.Unattributed, []string{"arch tier/check (tier:unknown)"}) {
		t.Fatalf("structuralGateReds = %#v", reds)
	}
}

// TestStructuralRedsSeeNothingUntilARunPublishesItsIndex points the commit
// gate's red reader at a run that has written 3 of its 49 stage logs and no
// published index.
//
// It answers NOTHING: no charge, no foreign red, no unattributed red, while a
// stage log on disk already says a red names the path. The gate is not wrong to
// do that, because the index is the artifact it reads, but the empty answer is
// an absence rather than a verdict, and the freshness certificate is the guard
// that keeps it from reading as a pass. This test pins both halves, because a
// change that made a missing certificate fresh would turn a partial run into a
// silent green (ai/rules/principles.md).
//
// `le verify reds file <path>` (internal/le/verify/reds.go) is what reads those
// three logs, and it answers undetermined rather than nothing.
func TestStructuralRedsSeeNothingUntilARunPublishesItsIndex(t *testing.T) {
	root := t.TempDir()
	writeCommitFixture(t, root, "mine/a.go", "package mine\n")
	writeCommitFixture(t, root, "tmp/verify/full-fixture/01-go-lint-run.log",
		"### Stage: go lint/run\n"+
			`VERIFY FAILURE GROUP: {"group-id":"lint:mine","kind":"lint","related":["mine/a.go"],`+
			`"summary":"findings","rerun":"le go lint run"}`+"\n"+
			"VERIFY FAILURE GROUPS COMPLETE: 1\n"+
			"### Stage result: go lint/run exit=1\n")
	writeCommitFixture(t, root, "tmp/verify/full-fixture/02-tier-check.log",
		"### Stage result: tier/check exit=0\n")
	writeCommitFixture(t, root, "tmp/verify/full-fixture/03-rfc-check.log",
		"### Stage result: rfc/check exit=2\n")

	reds := structuralGateReds(root, []string{"mine/a.go"})

	if len(reds.Charged) != 0 || len(reds.Foreign) != 0 || len(reds.Unattributed) != 0 {
		t.Fatalf("structuralGateReds read an unpublished run = %#v", reds)
	}
	state := verificationState(root, []string{"mine/a.go"})
	if state.State == verifyFresh {
		t.Fatalf("a checkout with no certificate is %q, so an empty red set reads as a pass", state.State)
	}
}

func TestCreateRefusesChargedStructuralRedWithoutRecordedOverride(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "structural-create-fixture")
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	writeCommitFixture(t, root, "tmp/ze-verify-failures.json", `{"stages":[{
		"stage":"doc wiring","exit-code":1,
		"groups":[{"group-id":"files:mine","kind":"files","related":["mine.txt"]}]
	}]}`)
	options := Options{
		Subject: "charged red", Files: []string{"mine.txt"},
		DryRun: true,
	}
	if _, err := Create(root, &options); err == nil || !strings.Contains(err.Error(), "deterministic structural gate") {
		t.Fatalf("Create structural refusal = %v", err)
	}
	options.StructuralRedOK = "other session owns the red and this text cannot affect it"
	if _, err := Create(root, &options); err != nil {
		t.Fatalf("Create refused a charged red the override admits: %v", err)
	}
}

// TestAnUnreachedStructuralStageIsCharged proves AC-11: a structural stage a
// first-red run never reached proved nothing, so the commit gate charges it as
// not reached and refuses the commit until structural-red-ok names a reason. A
// non-structural unreached stage is not charged, and an index written before
// first-red existed (no reached key) still reads every stage as run.
func TestAnUnreachedStructuralStageIsCharged(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "unreached-structural-fixture")
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	writeCommitFixture(t, root, "tmp/ze-verify-failures.json", `{"stopped-at":"rfc check","stages":[
		{"stage":"rfc check","reached":true,"exit-code":1,"groups":[]},
		{"stage":"doc wiring","reached":false,"exit-code":0},
		{"stage":"go lint/run","exit-code":0},
		{"stage":"test unit/cached","reached":false,"exit-code":0}
	]}`)
	reds := structuralGateReds(root, []string{"mine.txt"})
	if !slices.Contains(reds.Charged, "doc wiring") || !slices.Equal(reds.Unreached, []string{"doc wiring"}) {
		t.Fatalf("structuralGateReds = %#v, want doc wiring charged as not reached", reds)
	}
	if slices.Contains(reds.Charged, "go lint/run") || slices.Contains(reds.Charged, "test unit/cached") {
		t.Fatalf("a stage with no reached key or a non-structural stage was charged: %#v", reds)
	}
	options := Options{Subject: "after a stopped run", Files: []string{"mine.txt"}, DryRun: true}
	_, err := Create(root, &options)
	if err == nil || !strings.Contains(err.Error(), "doc wiring: not reached, because the run stopped at its first red (rfc check)") {
		t.Fatalf("Create after a stopped run = %v", err)
	}
	options.StructuralRedOK = "the stopped run's remaining stages run in the parent's full gate"
	if _, err := Create(root, &options); err != nil {
		t.Fatalf("structural-red-ok did not pass the unreached stage: %v", err)
	}
}

// publishRun runs one verification through the engine in a throwaway worktree
// and publishes its failure index into root, the way a worktree run does.
func publishRun(t *testing.T, root, commit string, part verifyengine.Part, red string) {
	t.Helper()
	worktree := t.TempDir()
	runner := func(_ context.Context, _ string, identity verifyengine.Identity) verifyengine.ActionResult {
		result := verifyengine.ActionResult{Identity: identity, Registered: true, Completed: true}
		if identity.Name == red {
			result.Code = 1
		}
		return result
	}
	verifyengine.RunPart(context.Background(), worktree, commit, verifyengine.Mode, part,
		verifyengine.Plan{}, runner, verifyengine.Slot{})
	if err := verifyengine.PublishFailureIndex(worktree, root, ""); err != nil {
		t.Fatal(err)
	}
}

// TestAPieceKeepsAnEarlierStructuralRedCharged proves a piece of a cut run
// cannot erase a structural red an earlier run recorded for a stage the piece
// never ran, and that a piece which reaches the stage green clears it. The
// piece's index is the checkout's latest, so before carry-forward it replaced
// the full run's index and a lint red disappeared with lint never rerun.
//
// Method: publish a full run red at `go lint/run` as commit aaa, then part 2
// of 3 green (lint is dealt to part 1), and require lint charged, not as
// unreached, with the refusal naming it red at aaa. Then publish part 1 green
// and require nothing charged.
func TestAPieceKeepsAnEarlierStructuralRedCharged(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "piece-index-fixture")
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	publishRun(t, root, "aaa", verifyengine.Uncut(), "go lint/run")
	publishRun(t, root, "bbb", verifyengine.Part{Index: 2, Of: 3}, "")
	reds := structuralGateReds(root, []string{"mine.txt"})
	if !slices.Contains(reds.Charged, "go lint/run") || slices.Contains(reds.Unreached, "go lint/run") {
		t.Fatalf("structuralGateReds after a piece = %#v, want go lint/run charged as the carried red", reds)
	}
	options := Options{Subject: "after a piece", Files: []string{"mine.txt"}, DryRun: true}
	_, err := Create(root, &options)
	if err == nil || !strings.Contains(err.Error(), "go lint/run: red at aaa in ") {
		t.Fatalf("Create after a piece = %v, want a refusal naming go lint/run red at aaa", err)
	}
	publishRun(t, root, "ccc", verifyengine.Part{Index: 1, Of: 3}, "")
	if reds := structuralGateReds(root, []string{"mine.txt"}); len(reds.Charged) != 0 {
		t.Fatalf("structuralGateReds after the piece holding lint ran green = %#v, want nothing charged", reds)
	}
}

// TestACarriedRedNamesTheRunThatClearsIt proves the refusal for a carried
// structural red names a command that clears the charge, not only one that
// diagnoses it. The group's rerun runs the stage directly and publishes no
// failure index, so a fixed stage rerun green that way left the carried red
// refusing every commit.
//
// Method: publish a full run red at `go lint/run` (in the fast population)
// and at `verify deps/evidence-vet` (full population only), then a piece that
// holds neither, and require each refusal line to name the diagnosing rerun
// and the verify runs that clear it, with `le verify current fast` named for
// lint only. The diagnosing rerun is pinned to the group's own rerun string,
// so a refusal that fell back to the full run would go red.
func TestACarriedRedNamesTheRunThatClearsIt(t *testing.T) {
	for _, tc := range []struct {
		stage string
		rerun string
		fast  bool
	}{
		{stage: "go lint/run", rerun: "le go lint run", fast: true},
		{stage: "verify deps/evidence-vet", rerun: "le verify deps evidence-vet", fast: false},
	} {
		t.Run(tc.stage, func(t *testing.T) {
			root := newCommitRepository(t)
			t.Setenv("CLAUDE_CODE_SESSION_ID", "carried-clear-fixture")
			writeCommitFixture(t, root, "mine.txt", "mine\n")
			publishRun(t, root, "aaa", verifyengine.Uncut(), tc.stage)
			publishRun(t, root, "bbb", verifyengine.Part{Index: 3, Of: 3}, "")
			reds := structuralGateReds(root, []string{"mine.txt"})
			cause := ""
			for index, charged := range reds.Charged {
				if charged == tc.stage {
					cause = reds.Causes[index]
				}
			}
			if cause == "" {
				t.Fatalf("structuralGateReds = %#v, want %s charged", reds, tc.stage)
			}
			for _, want := range []string{
				"; diagnose with: " + tc.rerun + "; ",
				"clears when a verify run reaches it: le verify worktree, or the le verify worktree part <i> of <n> piece that holds it",
			} {
				if !strings.Contains(cause, want) {
					t.Fatalf("cause = %q, want it to contain %q", cause, want)
				}
			}
			if strings.Contains(cause, "rerun it with") {
				t.Fatalf("cause = %q still offers a rerun as the clearing command", cause)
			}
			if got := strings.Contains(cause, "or le verify current fast"); got != tc.fast {
				t.Fatalf("cause = %q, names le verify current fast = %v, want %v", cause, got, tc.fast)
			}
		})
	}
}

// TestACarriedRedWithNoRecordedCommitSaysSo proves the refusal for a carried
// red whose earlier run recorded no commit names it as such, rather than the
// empty "red at  in ".
func TestACarriedRedWithNoRecordedCommitSaysSo(t *testing.T) {
	root := newCommitRepository(t)
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	writeCommitFixture(t, root, "tmp/ze-verify-failures.json", `{"stages":[
		{"stage":"go lint/run","reached":true,"exit-code":1,"groups":[],"outside-run":true,"carried-from":{"commit":"","run-dir":""}}
	]}`)
	reds := structuralGateReds(root, []string{"mine.txt"})
	if len(reds.Causes) != 1 {
		t.Fatalf("structuralGateReds = %#v, want go lint/run charged", reds)
	}
	cause := reds.Causes[0]
	if !strings.Contains(cause, "go lint/run: red in an earlier run (commit not recorded), carried") ||
		strings.Contains(cause, "red at  in") {
		t.Fatalf("cause = %q, want it to name an earlier run whose commit was not recorded", cause)
	}
}

// TestAGreenFastRunChargesNoStructuralStage proves a green `verify current
// fast` leaves no structural charge for the stages the fast population drops
// (verify deps/evidence-vet): a stage no run judged is not red, and the push
// gate already owes the full verify that covers "not run".
func TestAGreenFastRunChargesNoStructuralStage(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "fast-index-fixture")
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	passing := func(_ context.Context, _ string, identity verifyengine.Identity) verifyengine.ActionResult {
		return verifyengine.ActionResult{Identity: identity, Registered: true, Completed: true}
	}
	fast := verifyengine.RunMode(context.Background(), root, "aaa", verifyengine.FastMode,
		verifyengine.Plan{}, passing, verifyengine.Slot{})
	if fast.Code != 0 {
		t.Fatalf("fast run = %#v", fast.Failure)
	}
	if reds := structuralGateReds(root, []string{"mine.txt"}); len(reds.Charged) != 0 {
		t.Fatalf("structuralGateReds after a green fast run = %#v, want nothing charged", reds)
	}
}

// TestAGreenSweepOfEveryPieceChargesNoStructuralStage proves a cut
// sweep that runs every piece of a cut green leaves no structural charge: the
// last piece's index carries the other pieces' green verdicts.
func TestAGreenSweepOfEveryPieceChargesNoStructuralStage(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "sweep-index-fixture")
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	for index := 1; index <= 3; index++ {
		publishRun(t, root, "aaa", verifyengine.Part{Index: index, Of: 3}, "")
	}
	if reds := structuralGateReds(root, []string{"mine.txt"}); len(reds.Charged) != 0 {
		t.Fatalf("structuralGateReds after a green sweep = %#v, want nothing charged", reds)
	}
}

func TestGeneratedBlockQuotesPathsAndCommitsFromItsOwnIndex(t *testing.T) {
	t.Parallel()
	block := commitBlock{
		PreparedHead: preparedHeadFixture, Tag: "a", Subject: "quote fixture", Paths: []string{"plain.txt", "space name.txt", "quote's.txt"},
		Removed: []string{"old name.txt"}, MessagePath: "tmp/message name.txt",
		IndexEntries: []string{"100644 " + strings.Repeat("0", 40) + " 0\tplain.txt"},
	}
	script := renderBlock(block, "tmp/commit-owner.sh")
	for _, want := range []string{
		`_ze_index="$PWD"/'tmp/commit-owner.index'`,
		`_ze_parent=$(git rev-parse --verify 'HEAD^{commit}')`,
		`GIT_INDEX_FILE="$_ze_index" git read-tree "$_ze_parent"`,
		"git update-index --index-info <<'ZE_INDEX_INFO'",
		"100644 " + strings.Repeat("0", 40) + " 0\tplain.txt",
		"'space name.txt'", `'quote'"'"'s.txt'`,
		`git update-index --force-remove -- 'old name.txt'`,
		`_ze_message='tmp/message name.txt'`,
		`_ze_tree=$(GIT_INDEX_FILE="$_ze_index" git write-tree)`,
		`_ze_text=$(git stripspace < "$_ze_message")`,
		`_ze_commit=$(printf '%s\n' "$_ze_text" | git commit-tree ${_ze_sign:+-S} -p "$_ze_parent" "$_ze_tree")`,
		"_ze_parent='" + preparedHeadFixture + "'",
		`_ze_moved=$(git --literal-pathspecs rev-list "$_ze_seen..$_ze_parent" -- 'plain.txt'`,
		`git update-ref -m 'commit: quote fixture' HEAD "$_ze_commit" "$_ze_parent"`,
		"git ls-tree HEAD -- 'plain.txt' 'space name.txt' 'quote'\"'\"'s.txt' | git update-index --index-info",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("generated block lacks %q:\n%s", want, script)
		}
	}
	if got := parseShellWords(quotePaths(block.Paths)); !slices.Equal(got, block.Paths) {
		t.Fatalf("parseShellWords(quotePaths) = %q, want %q", got, block.Paths)
	}
}

// The two tests that stood here judged the staged-file guard: that it refused a
// concurrent session's staged path, and that a refused block left the shared
// index as it found it. The guard is deleted, because a block now commits from
// an index of its own, where a foreign staged path cannot appear at all
// (renderPrivateIndex, script.go). Both properties are asserted over the
// mechanism that replaced it, in snapshot_test.go:
// TestTheCommitCarriesThePreparedContentAndNotAConcurrentSessionsEdit and
// TestABlockLeavesTheSharedIndexAloneWhenItsCommitFails.

func TestCreateDryRunBuildsExactScriptWithoutTouchingSharedIndex(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "commit-create-fixture")
	writeCommitFixture(t, root, "mine name.txt", "mine\n")
	writeCommitFixture(t, root, "tracked.txt", "foreign edit\n")
	runCommitGit(t, root, "add", "--", "tracked.txt")

	prepared, err := Create(root, &Options{
		Subject: "prepare exact commit", Files: []string{"mine name.txt"},
		DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(prepared.Added, []string{"mine name.txt"}) || len(prepared.Removed) != 0 {
		t.Fatalf("prepared population = added %q removed %q", prepared.Added, prepared.Removed)
	}
	if !strings.Contains(prepared.ScriptText, "\t"+"mine name.txt\n") ||
		strings.Contains(prepared.ScriptText, "tracked.txt") {
		t.Fatalf("prepared script stages the wrong population:\n%s", prepared.ScriptText)
	}
	if staged := strings.TrimSpace(runCommitGitOutput(t, root, "diff", "--cached", "--name-only")); staged != "tracked.txt" {
		t.Fatalf("Create changed shared staging: %q", staged)
	}
	messages, err := filepath.Glob(filepath.Join(root, "tmp", "commit-msg-*"))
	if err != nil || len(messages) != 0 {
		t.Fatalf("dry run retained message reservations %q, %v", messages, err)
	}
}

// TestAppendRefusesAScriptCarryingAPushSection pins the one push route. A
// script prepared with the removed `create ... push` keyword ends in its push,
// so a block appended to it would run after the push and never be published.
// The append is refused naming the push verb, and a script with no push section
// still takes an appended block.
func TestAppendRefusesAScriptCarryingAPushSection(t *testing.T) {
	t.Parallel()
	first := commitBlock{PreparedHead: preparedHeadFixture, Tag: "a", Subject: "first", Paths: []string{"one.txt"}, MessagePath: "tmp/a.txt"}
	initial, err := composeScript("/repo", "", "tmp/commit-owner.sh", "12345678", first, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(initial, "git push") {
		t.Fatalf("a commit script carries a push:\n%s", initial)
	}
	second := commitBlock{PreparedHead: preparedHeadFixture, Tag: "b", Subject: "second", Paths: []string{"two.txt"}, MessagePath: "tmp/b.txt"}
	appended, err := composeScript("/repo", initial, "tmp/commit-owner.sh", "12345678", second, true)
	if err != nil {
		t.Fatalf("append onto a commit script was refused: %v", err)
	}
	if !strings.Contains(appended, "Commit b:") {
		t.Fatalf("the appended block is missing:\n%s", appended)
	}
	legacy := initial + "\n" + pushMarker + " owner approved this push\ngit push\n"
	_, err = composeScript("/repo", legacy, "tmp/commit-owner.sh", "12345678", second, true)
	if err == nil {
		t.Fatal("append onto a script carrying a push section was accepted")
	}
	if !strings.Contains(err.Error(), "./le commit push") {
		t.Fatalf("the refusal %q does not name the push verb", err)
	}
}

func TestReplaceRefusesAnotherPreparedPopulation(t *testing.T) {
	t.Parallel()
	foreign := renderBlock(commitBlock{PreparedHead: preparedHeadFixture, Tag: "a", Subject: "foreign", Paths: []string{"their.txt"}, MessagePath: "tmp/a.txt"}, "tmp/foreign.sh")
	if err := refuseForeignReplace(foreign, []string{"mine.txt"}); err == nil {
		t.Fatal("refuseForeignReplace accepted a disjoint script")
	}
	if err := refuseForeignReplace(foreign, []string{"their.txt", "mine.txt"}); err != nil {
		t.Fatalf("refuseForeignReplace rejected an overlapping population: %v", err)
	}
}

func TestReviewArtifactIsHashPinnedToEveryCodeFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "harness-session-id")
	writeCommitFixture(t, root, "internal/a.go", "package internal\n")
	stem := "native-port"
	writeReviewArtifact(t, root, stem, "internal/a.go")

	clean := CheckReview(root, stem, []string{"internal/a.go", "docs/note.md"})
	if !clean.Clean || len(clean.CodeFiles) != 1 {
		t.Fatalf("clean review = %#v", clean)
	}
	writeCommitFixture(t, root, "internal/a.go", "package changed\n")
	stale := CheckReview(root, stem, []string{"internal/a.go"})
	if stale.Clean || !slices.Equal(stale.Stale, []string{"internal/a.go"}) {
		t.Fatalf("stale review = %#v", stale)
	}
}

// TestTheReviewGateReadsTheArtifactTheRecorderWrites pins the ONE name a review
// artifact has.
//
// VALIDATES: CheckReview looks the artifact up where internal/le/spec
// writes it, which is under the HARNESS session id.
// PREVENTS: the regression the native port shipped. This gate built the name
// itself from the eight-hex commit namespace that SessionID mints, so
// `le spec review record` wrote tmp/review/<stem>-<harness>.md, the gate
// asked for tmp/review/<stem>-<namespace>.md, and every spec closure was refused
// with "no independent-review artifact at ...". Neither package's tests could see
// it: each chose its own session id and wrote the fixture under it, so both
// passed while the two names disagreed on a real checkout.
func TestTheReviewGateReadsTheArtifactTheRecorderWrites(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "harness-session-id")
	writeCommitFixture(t, root, "internal/a.go", "package internal\n")
	stem := "native-port"

	namespace, err := lepath.CommitSession(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if namespace == "harness-session-id" {
		t.Fatal("the commit namespace equals the harness session id, so this test cannot tell the two names apart")
	}
	misnamed := filepath.Join(root, "tmp", "review", stem+"-"+namespace+".md")
	writeArtifactAt(t, root, misnamed, "internal/a.go")
	if wrong := CheckReview(root, stem, []string{"internal/a.go"}); wrong.Clean {
		t.Fatalf("the gate accepted an artifact named with the commit namespace: %#v", wrong)
	}

	writeReviewArtifact(t, root, stem, "internal/a.go")
	if clean := CheckReview(root, stem, []string{"internal/a.go"}); !clean.Clean {
		t.Fatalf("the gate did not read the artifact the recorder writes: %#v", clean)
	}
}

// TestTheGeneratedReviewRecheckNamesTheLauncherOnDisk pins the spelling of the
// one command a commit script executes.
//
// VALIDATES: the re-check line a closure script carries names ./le, the launcher
// file that sits at the checkout root.
// PREVENTS: the bare `le`. The script runs that line under `set -e`, and `le` is
// not a command on PATH here, so a closure aborted with "le: command not found"
// before its first `git add`. No other line in a commit script executes, so
// nothing else could have caught it.
func TestTheGeneratedReviewRecheckNamesTheLauncherOnDisk(t *testing.T) {
	t.Parallel()
	line := reviewCheckCommand("demo-spec", []string{"internal/a.go", "docs/note.md"})
	launcher, _, found := strings.Cut(line, " ")
	if !found || launcher != shellQuote("./le") {
		t.Fatalf("review re-check runs %q, want the ./le launcher first: %q", launcher, line)
	}
	if !strings.Contains(line, shellQuote("internal/a.go")) ||
		strings.Contains(line, shellQuote("docs/note.md")) {
		t.Fatalf("review re-check names the wrong files: %q", line)
	}
}

// writeReviewArtifact plants a clean artifact at the path the recording package
// owns, so a fixture cannot invent a name of its own.
func writeReviewArtifact(t *testing.T, root, stem string, files ...string) {
	t.Helper()
	relative, err := spec.ReviewArtifactPath(root, stem)
	if err != nil {
		t.Fatal(err)
	}
	writeArtifactAt(t, root, filepath.Join(root, filepath.FromSlash(relative)), files...)
}

func writeArtifactAt(t *testing.T, root, path string, files ...string) {
	t.Helper()
	content := "<!-- ze-review spec=native-port verdict=clean rounds=1 -->\nfiles:\n"
	for _, file := range files {
		content += "  " + reviewHash(filepath.Join(root, filepath.FromSlash(file))) + "  " + file + "\n"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestClosureStemReadsRemovalsAndNotJournalRows pins the one signal. A removed
// spec file is the closure. A journal row names the spec a defect was found
// under and says nothing about whether that spec is closing, so a row NEVER
// contributes a stem -- not a fresh one, not one naming the session's own
// claimed spec. The middle case is the one that used to fire: CLAUDE.md
// requires a row for every defect walked into, so the ordinary in-progress
// commit carries one.
func TestClosureStemReadsRemovalsAndNotJournalRows(t *testing.T) {
	root := newCommitRepository(t)
	journalPath := "plan/journal/native.md"
	header := "| Date | Spec | Surface | Symptom | Fix |\n|------|------|---------|---------|-----|\n"
	writeCommitFixture(t, root, journalPath, header+"| 2026-08-01 | old-spec | cli | old | fixed: done |\n")
	runCommitGit(t, root, "add", "--", journalPath)
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "journal baseline")
	if stem, err := closureStemOnHead(root, []string{journalPath}, nil); err != nil || stem != "" {
		t.Fatalf("unchanged journal closure = %q, %v", stem, err)
	}
	writeCommitFixture(t, root, journalPath, header+
		"| 2026-08-01 | old-spec | cli | old | fixed: done |\n"+
		"| 2026-08-27 | new-spec | cli | new | fixed: done |\n")
	if stem, err := closureStemOnHead(root, []string{journalPath}, nil); err != nil || stem != "" {
		t.Fatalf("added journal row closure = %q, %v: a row is evidence about a spec, never its closure", stem, err)
	}
	if stem, err := closureStemOnHead(root, nil, []string{"plan/spec-removed.md"}); err != nil || stem != "removed" {
		t.Fatalf("removed spec closure = %q, %v", stem, err)
	}
	// The removal still wins when both are present, and it names the removed
	// spec rather than the one the row happens to mention.
	if stem, err := closureStemOnHead(root, []string{journalPath}, []string{"plan/spec-removed.md"}); err != nil || stem != "removed" {
		t.Fatalf("removal beside a journal row = %q, %v", stem, err)
	}
}

// TestClosureStemStillRefusesAMalformedJournalRow pins what survives the change.
// Journal paths are read for their SHAPE, so a row that is not five cells
// refuses the commit while the file is in hand, whether or not any spec closes.
func TestClosureStemStillRefusesAMalformedJournalRow(t *testing.T) {
	root := newCommitRepository(t)
	journalPath := "plan/journal/native.md"
	writeCommitFixture(t, root, journalPath,
		"| Date | Spec | Surface | Symptom | Fix |\n|------|------|---------|---------|-----|\n"+
			"| 2026-08-27 | some-spec | cli | missing two cells |\n")
	if _, err := closureStemOnHead(root, []string{journalPath}, nil); err == nil {
		t.Fatal("closureStem accepted a malformed journal row; the shape check must outlive the closure inference")
	}
}

// TestCreateRefusesAnAddedJournalRowOverTheCap drives the row cap through the
// commit entry point. A long row HEAD already holds rides along with a new short
// row, and a new row over specjournal.RowCharsMax is refused with its line and
// length. The post-write hook sees only Write and Edit, so a row appended from a
// shell lands unless this gate reads it.
func TestCreateRefusesAnAddedJournalRowOverTheCap(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "commit-journal-row-cap-fixture")
	root := newCommitRepository(t)
	writeCommitFixture(t, root, "ai/.keep", "")
	journalPath := "plan/journal/native.md"
	header := "| Date | Spec | Surface | Symptom | Fix |\n|------|------|---------|---------|-----|\n"
	row := func(date string, chars int) string {
		prefix := "| " + date + " | - | cli | "
		return prefix + strings.Repeat("s", chars-len(prefix)-len(" | fixed: fix |")) + " | fixed: fix |\n"
	}
	long := row("2026-08-01", specjournal.RowCharsMax+400)
	writeCommitFixture(t, root, journalPath, header+long)
	runCommitGit(t, root, "add", "--", journalPath)
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "journal baseline")

	writeCommitFixture(t, root, journalPath, header+long+row("2026-08-02", 80))
	if _, err := Create(root, &Options{Subject: "journal: a short row", Files: []string{journalPath}}); err != nil {
		t.Fatalf("Create refused a short row beside an unchanged long HEAD row: %v", err)
	}

	writeCommitFixture(t, root, journalPath, header+long+row("2026-08-02", specjournal.RowCharsMax+1))
	_, err := Create(root, &Options{Subject: "journal: a long row", Files: []string{journalPath}, Replace: true})
	if err == nil {
		t.Fatal("Create accepted an added row over the cap")
	}
	want := fmt.Sprintf("row-too-long at line 4: added or rewritten row is %d characters", specjournal.RowCharsMax+1)
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Create refusal = %q, want it to contain %q", err, want)
	}
}

// TestCreateRefusesAnUnmarkedJournalRow drives the Fix marker through the
// commit entry point. A row whose Fix cell starts with neither `fixed: ` nor
// `open: ` refuses the commit with its line, and the same row marked lands.
func TestCreateRefusesAnUnmarkedJournalRow(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "commit-journal-unmarked-fixture")
	root := newCommitRepository(t)
	writeCommitFixture(t, root, "ai/.keep", "")
	journalPath := "plan/journal/native.md"
	header := "| Date | Spec | Surface | Symptom | Fix |\n|------|------|---------|---------|-----|\n"
	writeCommitFixture(t, root, journalPath, header+"| 2026-08-02 | - | cli | symptom | not fixed |\n")
	_, err := Create(root, &Options{Subject: "journal: an unmarked row", Files: []string{journalPath}})
	if err == nil {
		t.Fatal("Create accepted a row whose Fix cell carries no marker")
	}
	if !strings.Contains(err.Error(), "unmarked-fix at line 3") {
		t.Fatalf("Create refusal = %q, want unmarked-fix at line 3", err)
	}
	writeCommitFixture(t, root, journalPath, header+"| 2026-08-02 | - | cli | symptom | open: not fixed |\n")
	// An open row needs its owning spec in the same commit (TestCreateRefusesAnOpenJournalRowNoSpecOwns).
	specPath := "plan/release/fixes/spec-native.md"
	writeCommitFixture(t, root, specPath, ownedRowsSpec("2026-08-02"))
	if _, err := Create(root, &Options{Subject: "journal: a marked row", Files: []string{journalPath, specPath}, Replace: true}); err != nil {
		t.Fatalf("Create refused a marked row: %v", err)
	}
}

// closureStemOnHead judges a commit of its own, which lands on HEAD: the zero
// specjournal.Base. An appended block's base is TestCreateJudgesAnAppendedClosure.
func closureStemOnHead(root string, paths, removed []string) (string, error) {
	return closureStem(root, paths, removed, specjournal.Base{})
}

// ownedRowsSpec renders a spec whose owned journal rows table lists one date of
// the native.md class.
func ownedRowsSpec(date string) string {
	return "# Spec: native\n\n## Owned journal rows\n\n| Class file | Date | Surface (short) |\n" +
		"|------------|------|-----------------|\n| `native.md` | " + date + " | cli |\n"
}

// TestCreateRefusesAnOpenJournalRowNoSpecOwns drives row ownership through the
// commit entry point. VALIDATES: a commit adding an open row that no spec's
// owned journal rows table lists is refused with the row's line, the reason and
// the table line to add; the same row lands once its owning spec rides in the
// commit; a fixed row lands with no owner. PREVENTS: an open defect recorded
// with no spec that will fix it.
func TestCreateRefusesAnOpenJournalRowNoSpecOwns(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "commit-journal-owner-fixture")
	root := newCommitRepository(t)
	writeCommitFixture(t, root, "ai/.keep", "")
	journalPath := "plan/journal/native.md"
	header := "| Date | Spec | Surface | Symptom | Fix |\n|------|------|---------|---------|-----|\n"

	writeCommitFixture(t, root, journalPath, header+"| 2026-08-03 | - | cli | symptom | fixed: repaired |\n")
	if _, err := Create(root, &Options{Subject: "journal: a fixed row", Files: []string{journalPath}}); err != nil {
		t.Fatalf("Create refused a fixed row, which needs no owner: %v", err)
	}

	writeCommitFixture(t, root, journalPath, header+"| 2026-08-03 | - | cli | symptom | open: owed |\n")
	_, err := Create(root, &Options{Subject: "journal: an unowned row", Files: []string{journalPath}, Replace: true})
	if err == nil {
		t.Fatal("Create accepted an open row no spec owns")
	}
	for _, want := range []string{
		"plan/journal/native.md line 3",
		"no spec owns it",
		"| `native.md` | 2026-08-03 | cli |",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Create refusal = %q, want it to contain %q", err, want)
		}
	}

	specPath := "plan/release/fixes/spec-native.md"
	writeCommitFixture(t, root, specPath, ownedRowsSpec("2026-08-03"))
	if _, err := Create(root, &Options{Subject: "journal: an owned row", Files: []string{journalPath, specPath}, Replace: true}); err != nil {
		t.Fatalf("Create refused an open row its spec owns: %v", err)
	}
}

// TestCreateJudgesAnAppendedClosureAgainstItsEarlierBlocks drives the two-commit
// spec closure through the commit entry point. Commit A is prepared, then
// commit B is appended to A's script before A lands, so HEAD is still the
// commit before A. VALIDATES: B's journal ownership check reads the class files
// and specs A's block carries, so a closure whose A fixes the row, or moves its
// entry to another spec, is accepted; a closure whose A leaves the row open and
// unowned is still refused, naming the row. PREVENTS: every standard spec
// closure refused because B was judged against bare HEAD.
func TestCreateJudgesAnAppendedClosureAgainstItsEarlierBlocks(t *testing.T) {
	journalPath := "plan/journal/native.md"
	specPath := "plan/release/fixes/spec-native.md"
	otherPath := "plan/release/fixes/spec-other.md"
	header := "| Date | Spec | Surface | Symptom | Fix |\n|------|------|---------|---------|-----|\n"
	openRow := header + "| 2026-08-04 | - | cli | symptom | open: owed |\n"
	fixedRow := header + "| 2026-08-04 | - | cli | symptom | fixed: repaired |\n"
	noTable := "# Spec: native\n\nStatus: in-progress\n"
	tests := []struct {
		name    string
		blockA  map[string]string
		refused bool
	}{
		{name: "commit A fixes the row", blockA: map[string]string{journalPath: fixedRow, specPath: noTable}},
		// The spec stays as HEAD holds it, so only the class-file overlay can
		// clear B: without it the row reads open and B removes its only owner.
		{name: "commit A fixes the row and leaves the spec", blockA: map[string]string{journalPath: fixedRow}},
		{name: "commit A moves the entry to another spec", blockA: map[string]string{
			specPath: noTable, otherPath: ownedRowsSpec("2026-08-04"),
		}},
		// Commit A keeps the entry, so A itself is owned; B then removes the
		// only spec that lists the row.
		{name: "commit A leaves the row open and not re-homed", blockA: map[string]string{
			specPath: "Status: done\n\n" + ownedRowsSpec("2026-08-04"),
		}, refused: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("CLAUDE_CODE_SESSION_ID", "commit-journal-closure-fixture")
			root := newCommitRepository(t)
			writeCommitFixture(t, root, "ai/.keep", "")
			writeCommitFixture(t, root, journalPath, openRow)
			writeCommitFixture(t, root, specPath, ownedRowsSpec("2026-08-04"))
			runCommitGit(t, root, "add", "--", journalPath, specPath)
			runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
				"commit", "-q", "-m", "spec owns an open row")

			var files []string
			for path, content := range test.blockA {
				writeCommitFixture(t, root, path, content)
				files = append(files, path)
			}
			slices.Sort(files)
			if _, err := Create(root, &Options{Subject: "plan: close native, commit A", Files: files}); err != nil {
				t.Fatalf("Create refused commit A: %v", err)
			}
			_, err := Create(root, &Options{
				Subject: "plan: close native, commit B", Remove: []string{specPath}, Append: true,
				ReviewOverride: "fixture repository carries no review",
			})
			if !test.refused {
				if err != nil {
					t.Fatalf("Create refused commit B, appended after commit A: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Create accepted commit B although commit A leaves the row open with no owner")
			}
			for _, want := range []string{"plan/journal/native.md line 3", "no spec owns it"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("Create refusal = %q, want it to contain %q", err, want)
				}
			}
		})
	}
}

// TestCommitSessionIsStableAndExplicitlyReplaceable drives the namespace every
// artifact of a prepared commit is named after: the script, the message and
// the two test-ledger shards. The producer is
// lepath.CommitSession, which both this package and the pre-write hook read,
// and the assertions are unchanged from when it lived here.
func TestCommitSessionIsStableAndExplicitlyReplaceable(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "harness-one")
	first, err := lepath.CommitSession(root, "")
	if err != nil || len(first) != 8 {
		t.Fatalf("CommitSession(first) = %q, %v", first, err)
	}
	again, err := lepath.CommitSession(root, "")
	if err != nil || again != first {
		t.Fatalf("CommitSession(again) = %q, %v; want %q", again, err, first)
	}
	replaced, err := lepath.CommitSession(root, "ABCDEF12")
	if err != nil || replaced != "abcdef12" {
		t.Fatalf("CommitSession(replace) = %q, %v", replaced, err)
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", "harness-two")
	other, err := lepath.CommitSession(root, "87654321")
	if err != nil || other != "87654321" {
		t.Fatalf("CommitSession(other) = %q, %v", other, err)
	}
	if kept, err := lepath.CommitSession(root, ""); err != nil || kept != "87654321" {
		t.Fatalf("CommitSession(harness-two, again) = %q, %v; want 87654321", kept, err)
	}
}

func newCommitRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	// The markers make the fixture a Ze CHECKOUT, which is what the gates below
	// judge. lepath.IsCheckout reads the pair, and Create runs the source,
	// verification and review gates only where it answers true.
	writeCommitFixture(t, root, "go.mod", "module fixture\n")
	writeCommitFixture(t, root, "feature-gates.txt", "")
	writeCommitFixture(t, root, "tracked.txt", "tracked\n")
	runCommitGit(t, root, "init", "-q")
	runCommitGit(t, root, "add", "--", "tracked.txt")
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "baseline")
	return root
}

func writeCommitFixture(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func runCommitGit(t *testing.T, root string, args ...string) {
	t.Helper()
	_ = runCommitGitOutput(t, root, args...)
}

func runCommitGitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", args...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

// TestCreateRefusesBadTagBeforeTheGates asserts a tag that cannot name a
// message file is refused before any gate runs, with no message reservation
// left behind.
//
// VALIDATES: Create calls validateTag beside Message, before its gates.
// PREVENTS: the shape of
// plan/journal/record-written-before-the-operation-succeeds.md. nextTag is the
// only other place the tag is checked and it runs after every gate, so a bad
// tag found there costs a full gate pass and anything a gate wrote stays
// behind for a commit that was never made.
func TestCreateRefusesBadTagBeforeTheGates(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "bad-tag-fixture")
	writeCommitFixture(t, root, "mine.txt", "mine\n")

	options := Options{
		Tag: "fix(bgp)", Subject: "parenthesised tag", Files: []string{"mine.txt"},
		StructuralRedOK: "fixture repository carries no gate record",
	}
	_, err := Create(root, &options)
	if err == nil || !strings.Contains(err.Error(), "tag must start with an alphanumeric") {
		t.Fatalf("Create with tag %q = %v, want a tag-format refusal", options.Tag, err)
	}

	messages, globErr := filepath.Glob(filepath.Join(root, "tmp", "commit-msg-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(messages) != 0 {
		t.Fatalf("refused create left message reservation(s) behind: %q", messages)
	}
}

// TestValidateTagReportsLengthSeparatelyFromCharacters asserts the two halves of
// tagPattern are reported apart. An over-long tag made of legal characters used
// to be refused with "must start with an alphanumeric character and contain only
// alnum, dot, underscore, or dash", which sends the author hunting a bad
// character that is not there.
//
// The boundary is asserted from tagMaxLength rather than from a literal, so the
// case follows the constant the pattern is built from.
func TestValidateTagReportsLengthSeparatelyFromCharacters(t *testing.T) {
	atLimit := strings.Repeat("a", tagMaxLength)
	if err := validateTag(atLimit); err != nil {
		t.Fatalf("validateTag(%d chars) = %v, want nil: the limit itself is accepted", tagMaxLength, err)
	}

	overLimit := strings.Repeat("a", tagMaxLength+1)
	err := validateTag(overLimit)
	if err == nil {
		t.Fatalf("validateTag(%d chars) = nil, want a refusal", tagMaxLength+1)
	}
	if !strings.Contains(err.Error(), "over the") {
		t.Fatalf("validateTag(%d chars) = %v, want the length named; a character-class message hides the real cause", tagMaxLength+1, err)
	}
	if strings.Contains(err.Error(), "alphanumeric") {
		t.Fatalf("validateTag(%d chars) = %v: every character is legal, so the class must not be blamed", tagMaxLength+1, err)
	}

	if err := validateTag("fix(bgp)"); err == nil || !strings.Contains(err.Error(), "alphanumeric") {
		t.Fatalf("validateTag(%q) = %v, want the character-class refusal", "fix(bgp)", err)
	}
}

// TestCreateOutsideTheZeCheckoutSkipsTheGatesThatReadZeSources is the case that
// made the published site uncommittable.
//
// VALIDATES: Create runs the source, verification and review gates only where
// lepath.IsCheckout answers true, and prepares a plain explicit commit in the
// sibling checkouts ZE_REPO_ROOT names.
// PREVENTS: the two failures a site republish met. The discovery-index gate
// refused every commit with "the tree holds no ai/ directory", and the stale
// certificate's verdict was judged against a tree no native gate builds.
func TestCreateOutsideTheZeCheckoutSkipsTheGatesThatReadZeSources(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "commit-site-fixture")
	site := newSiteRepository(t)
	writeCommitFixture(t, site, "index.html", "<p>published</p>\n")

	prepared, err := Create(site, &Options{
		Subject: "site: publish the generated tree", Files: []string{"index.html"},
	})
	if err != nil {
		t.Fatalf("Create refused a site commit: %v", err)
	}
	if prepared.Verify.State != verifyNotApplicable {
		t.Fatalf("verify state = %q, want %q", prepared.Verify.State, verifyNotApplicable)
	}
	script, err := os.ReadFile(filepath.Join(site, filepath.FromSlash(prepared.Script)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), "cd "+shellQuote(site)+"\n") {
		t.Fatalf("script does not cd to the checkout it was prepared for:\n%s", script)
	}

	// The discrimination: the same call in a tree that IS the Ze checkout still
	// runs the verification gates, so the skip is keyed to the tree rather than
	// to what the fixture happens to hold. The gate that answers here is the
	// verification certificate: no fixture carries a FRESH-green one, so the
	// state judged is the certificate's, never not-applicable.
	//
	// It was the discovery-index gate until 2026-09-11, which refused outright
	// with "the tree holds no ai/ directory". That gate is deleted, and the map
	// it compared is derived rather than committed.
	checkout := newCommitRepository(t)
	writeCommitFixture(t, checkout, "index.html", "<p>published</p>\n")
	inside, err := Create(checkout, &Options{
		Subject: "site: publish the generated tree", Files: []string{"index.html"},
	})
	if err != nil {
		t.Fatalf("Create refused a commit inside the Ze checkout: %v", err)
	}
	if inside.Verify.State == verifyNotApplicable {
		t.Fatal("Create skipped the verification gates inside the Ze checkout")
	}
}

// TestACommitChangingAPackageCommentNeedsNoIndex is user story 2.
//
// A commit that edits the `// Package` comment the map quotes used to be
// refused with "commit changes an index-feeding source and omits
// ai/PACKAGE-MAP.md", and the author then either regenerated a map from a
// SHARED working tree, carrying other sessions' packages into it, or spent
// stale-index-ok on it. The map is derived now, so the commit owes it nothing.
func TestACommitChangingAPackageCommentNeedsNoIndex(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "commit-derived-map-fixture")
	root := newCommitRepository(t)
	writeCommitFixture(t, root, "ai/.keep", "")
	writeCommitFixture(t, root, "internal/core/x/x.go", "// Package x does x.\npackage x\n")
	// The test-coverage gate is a DIFFERENT live gate over the same commit, and
	// it refuses Go with no test. Satisfying it keeps this test about the index.
	writeCommitFixture(t, root, "internal/core/x/x_test.go", "package x\n")

	prepared, err := Create(root, &Options{
		Subject: "core: x says what it does",
		Files:   []string{"internal/core/x/x.go", "internal/core/x/x_test.go"},
	})
	if err != nil {
		t.Fatalf("Create refused a commit that changes a package comment: %v", err)
	}
	if slices.Contains(prepared.Added, "ai/PACKAGE-MAP.md") {
		t.Error("the prepared commit carries the derived map")
	}
}

// TestATaggedTestNeedsNoLedgerRegeneration is user story 1, and AC-8.
//
// VALIDATES: a commit that adds an RFC-tagged test and edits the summary that
// declares the requirement is prepared with no refusal, carries none of the five
// RFC outputs, and owes no verification debt for them.
// PREVENTS: the state every row of plan/journal/concurrent-rfc-gate-stale.md
// records. The ledger and its 194 shards were tracked, the freshness gate
// compared a re-render against the committed copy, and an author who added one
// tagged test had to regenerate the family from a SHARED working tree. That
// carried other sessions' uncommitted summaries into a tracked page, and the
// gate then reddened for whoever ran it next. The five are derived now, so the
// commit owes them nothing.
func TestATaggedTestNeedsNoLedgerRegeneration(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "commit-derived-ledger-fixture")
	root := newCommitRepository(t)
	writeCommitFixture(t, root, "ai/.keep", "")
	// The tag is ASSEMBLED from rfcTagRequirement, for the reason that constant
	// states: a literal tag in this file would make the file its own carrier,
	// and the owner-approval gate would refuse every later edit to it. The
	// checklist line carries the requirement ID alone, because the polarity
	// word belongs to the TAG and never to the summary that declares the MUST.
	requirement := strings.TrimSuffix(rfcTagRequirement, " positive")
	writeCommitFixture(t, root, "rfc/short/rfc9999.md",
		"# RFC 9999\n\n- [ ] {must} "+requirement+" -- the thing the tagged test proves\n")
	writeCommitFixture(t, root, "internal/component/bgp/message/rfc9999_test.go",
		taggedUnitText("check(1)"))

	prepared, err := Create(root, &Options{
		Subject: "bgp: the thing the tagged test proves",
		Files: []string{
			"rfc/short/rfc9999.md",
			"internal/component/bgp/message/rfc9999_test.go",
		},
	})
	if err != nil {
		t.Fatalf("Create refused a commit that adds a tagged test and regenerates nothing: %v", err)
	}

	// Absence is the criterion below, and an empty population agrees with every
	// absence, so what the commit DOES carry is checked first.
	for _, want := range []string{"rfc/short/rfc9999.md", "internal/component/bgp/message/rfc9999_test.go"} {
		if !slices.Contains(prepared.Added, want) {
			t.Fatalf("the prepared commit does not carry %s, so nothing below was judged: %v",
				want, prepared.Added)
		}
	}

	// The five outputs, named because their ABSENCE is the criterion. A
	// prepared population that carried one would be this session publishing a
	// render of a working tree it shares with about twenty others.
	derivedOutputs := []string{
		"ai/RFC-REQUIREMENTS.md", "rfc/requirements", "rfc/enrolled.txt",
		"rfc/not-enrolled.txt", "docs/features/rfc-status.md",
	}
	for _, path := range derivedOutputs {
		for _, added := range prepared.Added {
			if added == path || strings.HasPrefix(added, path+"/") {
				t.Errorf("the prepared commit carries the derived output %s", added)
			}
		}
	}
}

// TestStaleIndexOKIsNotAKeyword pins that the override went with its gate.
//
// A keyword whose gate is deleted is worse than no keyword: it parses, and it
// says a check was overridden when no check ran. parseCreate refuses an undeclared keyword, so the absence is testable.
func TestStaleIndexOKIsNotAKeyword(t *testing.T) {
	t.Parallel()
	if _, err := parseCreate([]string{"subject", "s", "file", "a", "stale-index-ok", "why"}); err == nil {
		t.Fatal("parseCreate accepted stale-index-ok, whose gate is deleted")
	}
	// The discrimination: a keyword that IS declared still parses, so the
	// refusal above is about this one name and not about the shape of the call.
	if _, err := parseCreate([]string{"subject", "s", "file", "a", "structural-red-ok", "why"}); err != nil {
		t.Fatalf("parseCreate refused a declared override keyword: %v", err)
	}
}

// TestListKeywordsExpandToTheSameExplicitPopulation pins what file-list buys and
// what it must not buy. A site republish names about 1400 paths, which nobody
// writes as 1400 keywords; the list is read into the same explicit population,
// so every path is still validated and still spelled in the script.
func TestListKeywordsExpandToTheSameExplicitPopulation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	list := filepath.Join(dir, "paths.txt")
	if err := os.WriteFile(list, []byte("# generated by the site build\nfirst.html\n\nsecond.html\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(dir, "gone.txt")
	if err := os.WriteFile(gone, []byte("old.html\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	options, err := parseCreate([]string{
		"subject", "site: republish", "file", "named.html",
		"file-list", list, "remove-list", gone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(options.Files, []string{"named.html", "first.html", "second.html"}) {
		t.Fatalf("expanded files = %q", options.Files)
	}
	if !slices.Equal(options.Remove, []string{"old.html"}) {
		t.Fatalf("expanded removals = %q", options.Remove)
	}
	if _, err := parseCreate([]string{"subject", "s", "file-list", filepath.Join(dir, "missing.txt")}); err == nil {
		t.Fatal("parseCreate accepted a list file that does not exist")
	}
	empty := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(empty, []byte("# nothing but a header\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := parseCreate([]string{"subject", "s", "file-list", empty}); err == nil {
		t.Fatal("parseCreate accepted a list that names no file")
	}
}

// newSiteRepository is a git checkout that is NOT a Ze checkout: it carries
// neither marker, exactly like the published site and the wiki.
func newSiteRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeCommitFixture(t, root, "tracked.txt", "tracked\n")
	runCommitGit(t, root, "init", "-q")
	runCommitGit(t, root, "add", "--", "tracked.txt")
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "baseline")
	return root
}

// TestARemovalRendersTheWorkingTreeDeletionAfterTheCommit asserts the order of
// the rendered block for a removal: the captured entries before the private
// `force-remove`, the commit, the working-tree deletion, and last the
// shared-index repair, with a path holding a space and a quote quoted the way
// every other path is. The repair is last because it is the one step that
// waits on another process's `.git/index.lock`, and when it gives up the
// deletion has already run.
//
// VALIDATES: AC-9 and the ordering AC-7 relies on in the closed
// spec-remove-takes-the-working-tree-copy; the contract is
// docs/contributing/committing.md, "What the generated script contains", step 8.
// PREVENTS: a deletion rendered before `git commit`, which would delete a file
// whose content no commit holds yet.
func TestARemovalRendersTheWorkingTreeDeletionAfterTheCommit(t *testing.T) {
	t.Parallel()
	block := commitBlock{PreparedHead: preparedHeadFixture, Tag: "a", Subject: "remove", Removed: []string{"it's old.txt"}, MessagePath: "tmp/m.txt"}
	script := renderBlock(block, "tmp/commit-owner.sh")
	quoted := shellQuote("it's old.txt")
	order := []string{
		`_ze_removed=$(GIT_INDEX_FILE="$_ze_index" git --literal-pathspecs -c core.quotePath=false ls-files -s -- ` + quoted + `)`,
		`GIT_INDEX_FILE="$_ze_index" git update-index --force-remove -- ` + quoted,
		`git update-ref -m `,
		`_ze_gone=(` + quoted + `)`,
		`rm -f -- "$_ze_path"`,
		`until _ze_said=$({ git update-index --force-remove -- ` + quoted,
	}
	at := 0
	for _, want := range order {
		next := strings.Index(script[at:], want)
		if next < 0 {
			t.Fatalf("generated block lacks %q after offset %d:\n%s", want, at, script)
		}
		at += next + len(want)
	}
}

// TestABlockWithoutRemovalsRendersNoWorkingTreeDeletion asserts AC-8: a block
// that only adds renders neither the capture nor the deletion, so `set -u`
// never meets an unset `_ze_removed`.
func TestABlockWithoutRemovalsRendersNoWorkingTreeDeletion(t *testing.T) {
	t.Parallel()
	block := commitBlock{PreparedHead: preparedHeadFixture, Tag: "a", Subject: "add", Paths: []string{"new.txt"}, MessagePath: "tmp/m.txt",
		IndexEntries: []string{"100644 0000000000000000000000000000000000000000 0\tnew.txt"}}
	script := renderBlock(block, "tmp/commit-owner.sh")
	for _, absent := range []string{"_ze_removed", "_ze_gone", "rm -f -- "} {
		if strings.Contains(script, absent) {
			t.Fatalf("a block with no removal renders %q:\n%s", absent, script)
		}
	}
}
