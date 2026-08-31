package commit

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	specsession "github.com/ze-software/ze/internal/le/spec/session"
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

func TestMessageAndKeywordGrammarAreClosed(t *testing.T) {
	t.Parallel()
	if _, err := Message("", nil); err == nil {
		t.Fatal("Message accepted an empty subject")
	}
	if _, err := Message(strings.Repeat("x", 73), nil); err == nil {
		t.Fatal("Message accepted a 73-character subject")
	}
	message, err := Message("safe subject", []string{"one two three", "", "four"})
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
		{"stage":"verify lint/run","exit-code":1,"groups":[
			{"group-id":"lint:theirs","kind":"lint","related":["theirs/b.go"]}]},
		{"stage":"tier/check","exit-code":1,"groups":[
			{"group-id":"tier:unknown","kind":"subcheck","related":["theirs/b.go"]}]},
		{"stage":"doc wiring","exit-code":1,"groups":[
			{"group-id":"files:mine","kind":"files","related":["mine/a.go"]}]}
	]}`)
	reds := structuralGateReds(root, []string{"mine/a.go"})
	if !slices.Equal(reds.Charged, []string{"tier/check", "doc wiring"}) ||
		!slices.Equal(reds.Foreign, []string{"verify lint/run"}) ||
		!slices.Equal(reds.Unattributed, []string{"tier/check (tier:unknown)"}) {
		t.Fatalf("structuralGateReds = %#v", reds)
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
		StaleIndexOK: "fixture repository has no generated index", DryRun: true,
	}
	if _, err := Create(root, &options); err == nil || !strings.Contains(err.Error(), "deterministic structural gate") {
		t.Fatalf("Create structural refusal = %v", err)
	}
	options.StructuralRedOK = "other session owns the red and this text cannot affect it"
	prepared, err := Create(root, &options)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, debt := range prepared.Debt {
		if debt.Gate == "native structural checks (red)" {
			found = true
		}
	}
	if !found {
		t.Fatalf("override did not record structural debt: %#v", prepared.Debt)
	}
}

func TestGeneratedBlockQuotesPathsAndGuardsForeignStaging(t *testing.T) {
	t.Parallel()
	block := commitBlock{
		Tag: "a", Subject: "quote fixture", Paths: []string{"plain.txt", "space name.txt", "quote's.txt"},
		Removed: []string{"old name.txt"}, MessagePath: "tmp/message name.txt",
	}
	script := renderBlock(block, "tmp/commit-owner.sh")
	for _, want := range []string{
		"git add -- \\", "'space name.txt'", `'quote'"'"'s.txt'`, "git rm -- 'old name.txt'",
		"git -c core.quotePath=false diff --cached --name-only", "this script: tmp/commit-owner.sh",
		"git commit -F 'tmp/message name.txt'",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("generated block lacks %q:\n%s", want, script)
		}
	}
	if got := parseShellWords(quotePaths(block.Paths)); !slices.Equal(got, block.Paths) {
		t.Fatalf("parseShellWords(quotePaths) = %q, want %q", got, block.Paths)
	}
}

func TestStagingGuardRefusesForeignIndexAndAcceptsItsOwn(t *testing.T) {
	root := newCommitRepository(t)
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	writeCommitFixture(t, root, "tracked.txt", "foreign edit\n")
	runCommitGit(t, root, "add", "--", "tracked.txt")

	guard := renderStagingGuard([]string{"mine.txt"}, "tmp/commit-owner.sh")
	command := exec.CommandContext(t.Context(), "bash", "-c", guard)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "ABORT: index has staged files") ||
		!strings.Contains(string(output), "this script: tmp/commit-owner.sh") ||
		!strings.Contains(string(output), "tracked.txt") {
		t.Fatalf("foreign staging verdict = %v, %q", err, output)
	}

	runCommitGit(t, root, "restore", "--staged", "--", "tracked.txt")
	runCommitGit(t, root, "add", "--", "mine.txt")
	command = exec.CommandContext(t.Context(), "bash", "-c", guard)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("own staging was refused: %v: %s", err, output)
	}
}

// TestRefusedBlockLeavesTheIndexAsItFoundIt runs a whole generated block
// against a repository another session has already staged into, and asserts
// the refusal stages nothing of its own.
//
// VALIDATES: the concurrency guard is emitted BEFORE the add and the rm, so a
// refused script leaves the index exactly as it found it.
// PREVENTS: the deadlock of 2026-08-30. The guard used to run after staging,
// so a refused script added its own paths on the way to reporting somebody
// else's. Two sessions then held one index between them, each script refusing
// the other's paths, and neither could proceed. Clearing that needs
// `git restore --staged`, which no agent may run, so it reached the owner.
func TestRefusedBlockLeavesTheIndexAsItFoundIt(t *testing.T) {
	root := newCommitRepository(t)
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	writeCommitFixture(t, root, "gone.txt", "removed by this block\n")
	runCommitGit(t, root, "add", "--", "gone.txt")
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t",
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", "add gone.txt")

	// The other session got there first.
	writeCommitFixture(t, root, "tracked.txt", "foreign edit\n")
	runCommitGit(t, root, "add", "--", "tracked.txt")

	block := commitBlock{
		Tag: "a", Subject: "refused", Paths: []string{"mine.txt"},
		Removed: []string{"gone.txt"}, MessagePath: "tmp/message.txt",
	}
	command := exec.CommandContext(t.Context(), "bash", "-c", renderBlock(block, "tmp/commit-mine.sh"))
	command.Dir = root
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("the block committed over a foreign index: %s", output)
	}
	if !strings.Contains(string(output), "ABORT: index has staged files") {
		t.Fatalf("refusal did not name the guard: %s", output)
	}

	staged := strings.Fields(runCommitGitOutput(t, root, "diff", "--cached", "--name-only"))
	if !slices.Equal(staged, []string{"tracked.txt"}) {
		t.Fatalf("the refusal changed the index: staged = %q, want only the foreign path", staged)
	}
}

func TestCreateDryRunBuildsExactScriptWithoutTouchingSharedIndex(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "commit-create-fixture")
	writeCommitFixture(t, root, "mine name.txt", "mine\n")
	writeCommitFixture(t, root, "tracked.txt", "foreign edit\n")
	runCommitGit(t, root, "add", "--", "tracked.txt")

	prepared, err := Create(root, &Options{
		Subject: "prepare exact commit", Files: []string{"mine name.txt"},
		StaleIndexOK: "fixture repository has no generated index", DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(prepared.Added, []string{"mine name.txt"}) || len(prepared.Removed) != 0 {
		t.Fatalf("prepared population = added %q removed %q", prepared.Added, prepared.Removed)
	}
	if !strings.Contains(prepared.ScriptText, "git add -- \\\n  'mine name.txt'") ||
		strings.Contains(prepared.ScriptText, "git add -- \\\n  'tracked.txt'") {
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

func TestAppendKeepsOneAuthorisedPushAtTheEnd(t *testing.T) {
	t.Parallel()
	first := commitBlock{Tag: "a", Subject: "first", Paths: []string{"one.txt"}, MessagePath: "tmp/a.txt"}
	initial, err := composeScript("", "tmp/commit-owner.sh", "12345678", first, false, "owner approved this push")
	if err != nil {
		t.Fatal(err)
	}
	second := commitBlock{Tag: "b", Subject: "second", Paths: []string{"two.txt"}, MessagePath: "tmp/b.txt"}
	appended, err := composeScript(initial, "tmp/commit-owner.sh", "12345678", second, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(appended, "git push") != 1 || !strings.HasSuffix(appended, "git push\n") {
		t.Fatalf("append did not preserve one final push:\n%s", appended)
	}
	if strings.Index(appended, "Commit b:") > strings.Index(appended, pushMarker) {
		t.Fatal("second commit landed after the push")
	}
	tampered := initial + "echo after-push\n"
	if _, _, err := splitPush(tampered); err == nil {
		t.Fatal("splitPush accepted commands after the push")
	}
}

func TestReplaceRefusesAnotherPreparedPopulation(t *testing.T) {
	t.Parallel()
	foreign := renderBlock(commitBlock{Tag: "a", Subject: "foreign", Paths: []string{"their.txt"}, MessagePath: "tmp/a.txt"}, "tmp/foreign.sh")
	if err := refuseForeignReplace(foreign, []string{"mine.txt"}); err == nil {
		t.Fatal("refuseForeignReplace accepted a disjoint script")
	}
	if err := refuseForeignReplace(foreign, []string{"their.txt", "mine.txt"}); err != nil {
		t.Fatalf("refuseForeignReplace rejected an overlapping population: %v", err)
	}
}

func TestDebtRowsDeduplicateOnlyWhileOpen(t *testing.T) {
	root := t.TempDir()
	owed := []Debt{{Gate: debtGates[0].Name, Reason: "run after commit"}}
	path, err := recordDebt(root, "12345678", "subject", owed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recordDebt(root, "12345678", "subject", owed); err != nil {
		t.Fatal(err)
	}
	rows, err := ListDebt(root)
	if err != nil || len(rows) != 1 || rows[0].Status != "open" {
		t.Fatalf("ListDebt after duplicate = %#v, %v", rows, err)
	}
	if cleared, err := clearDebtRows(root, map[string]bool{debtGates[0].Name: true}); err != nil || cleared != 1 {
		t.Fatalf("clearDebtRows = %d, %v", cleared, err)
	}
	if _, err := recordDebt(root, "12345678", "subject", owed); err != nil {
		t.Fatal(err)
	}
	rows, err = ListDebt(root)
	if err != nil || len(rows) != 2 || rows[0].Status != "cleared" || rows[1].Status != "open" {
		t.Fatalf("ListDebt after re-owing %s = %#v, %v", path, rows, err)
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
// VALIDATES: CheckReview looks the artifact up where internal/le/spec/session
// writes it, which is under the HARNESS session id.
// PREVENTS: the regression the native port shipped. This gate built the name
// itself from the eight-hex commit namespace that SessionID mints, so
// `le spec session review record` wrote tmp/review/<stem>-<harness>.md, the gate
// asked for tmp/review/<stem>-<namespace>.md, and every spec closure was refused
// with "no independent-review artifact at ...". Neither package's tests could see
// it: each chose its own session id and wrote the fixture under it, so both
// passed while the two names disagreed on a real checkout.
func TestTheReviewGateReadsTheArtifactTheRecorderWrites(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "harness-session-id")
	writeCommitFixture(t, root, "internal/a.go", "package internal\n")
	stem := "native-port"

	namespace, err := SessionID(root, "")
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
	relative, err := specsession.ReviewArtifactPath(root, stem)
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
	writeCommitFixture(t, root, journalPath, header+"| 2026-08-01 | old-spec | cli | old | fixed |\n")
	runCommitGit(t, root, "add", "--", journalPath)
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "journal baseline")
	if stem, err := closureStem(root, []string{journalPath}, nil); err != nil || stem != "" {
		t.Fatalf("unchanged journal closure = %q, %v", stem, err)
	}
	writeCommitFixture(t, root, journalPath, header+
		"| 2026-08-01 | old-spec | cli | old | fixed |\n"+
		"| 2026-08-27 | new-spec | cli | new | fixed |\n")
	if stem, err := closureStem(root, []string{journalPath}, nil); err != nil || stem != "" {
		t.Fatalf("added journal row closure = %q, %v: a row is evidence about a spec, never its closure", stem, err)
	}
	if stem, err := closureStem(root, nil, []string{"plan/spec-removed.md"}); err != nil || stem != "removed" {
		t.Fatalf("removed spec closure = %q, %v", stem, err)
	}
	// The removal still wins when both are present, and it names the removed
	// spec rather than the one the row happens to mention.
	if stem, err := closureStem(root, []string{journalPath}, []string{"plan/spec-removed.md"}); err != nil || stem != "removed" {
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
	if _, err := closureStem(root, []string{journalPath}, nil); err == nil {
		t.Fatal("closureStem accepted a malformed journal row; the shape check must outlive the closure inference")
	}
}

func TestCommitSessionIsStableAndExplicitlyReplaceable(t *testing.T) {
	root := t.TempDir()
	first, err := sessionIDFor(root, "", "harness-one")
	if err != nil || len(first) != 8 {
		t.Fatalf("sessionIDFor(first) = %q, %v", first, err)
	}
	again, err := sessionIDFor(root, "", "harness-one")
	if err != nil || again != first {
		t.Fatalf("sessionIDFor(again) = %q, %v; want %q", again, err, first)
	}
	replaced, err := sessionIDFor(root, "ABCDEF12", "harness-one")
	if err != nil || replaced != "abcdef12" {
		t.Fatalf("sessionIDFor(replace) = %q, %v", replaced, err)
	}
	other, err := sessionIDFor(root, "87654321", "harness-two")
	if err != nil || other != "87654321" {
		t.Fatalf("sessionIDFor(other) = %q, %v", other, err)
	}
}

func newCommitRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
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

// TestCreateRefusesBadTagBeforeRecordingDebt asserts a tag that cannot name a
// message file is refused with NO verification-debt shard left behind.
//
// VALIDATES: Create calls validateTag beside Message, before recordDebt.
// PREVENTS: the second occurrence of
// plan/journal/record-written-before-the-operation-succeeds.md. nextTag is the
// only other place the tag is checked and it runs after recordDebt has already
// appended its rows, so `tag "fix(bgp)"` left three rows naming a commit that
// git log finds zero of. Orphan rows are indistinguishable from real debt by
// reading the ledger, they make debt-clear re-run gates for a commit that does
// not exist, and they refuse a push until somebody deletes them by hand.
func TestCreateRefusesBadTagBeforeRecordingDebt(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "bad-tag-fixture")
	writeCommitFixture(t, root, "mine.txt", "mine\n")

	options := Options{
		Tag: "fix(bgp)", Subject: "parenthesised tag", Files: []string{"mine.txt"},
		StaleIndexOK:    "fixture repository has no generated index",
		StructuralRedOK: "fixture repository carries no gate record",
	}
	_, err := Create(root, &options)
	if err == nil || !strings.Contains(err.Error(), "tag must start with an alphanumeric") {
		t.Fatalf("Create with tag %q = %v, want a tag-format refusal", options.Tag, err)
	}

	shards, globErr := filepath.Glob(filepath.Join(root, "plan", "verification-debt", "*.md"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(shards) != 0 {
		body, _ := os.ReadFile(shards[0])
		t.Fatalf("refused create left %d debt shard(s) behind, first is %s:\n%s", len(shards), shards[0], body)
	}
}
