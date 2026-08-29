package commit

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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
	writeCommitFixture(t, root, "internal/a.go", "package internal\n")
	session := "12345678"
	stem := "native-port"
	artifact := filepath.Join(root, "tmp", "review", stem+"-"+session+".md")
	if err := os.MkdirAll(filepath.Dir(artifact), 0o750); err != nil {
		t.Fatal(err)
	}
	content := "<!-- ze-review verdict=clean -->\n  " +
		reviewHash(filepath.Join(root, "internal", "a.go")) + "  internal/a.go\n"
	if err := os.WriteFile(artifact, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	clean := CheckReview(root, session, stem, []string{"internal/a.go", "docs/note.md"})
	if !clean.Clean || len(clean.CodeFiles) != 1 {
		t.Fatalf("clean review = %#v", clean)
	}
	writeCommitFixture(t, root, "internal/a.go", "package changed\n")
	stale := CheckReview(root, session, stem, []string{"internal/a.go"})
	if stale.Clean || !slices.Equal(stale.Stale, []string{"internal/a.go"}) {
		t.Fatalf("stale review = %#v", stale)
	}
}

func TestClosureStemUsesOnlyNewJournalEvidence(t *testing.T) {
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
	if stem, err := closureStem(root, []string{journalPath}, nil); err != nil || stem != "new-spec" {
		t.Fatalf("new journal closure = %q, %v", stem, err)
	}
	if stem, err := closureStem(root, nil, []string{"plan/spec-removed.md"}); err != nil || stem != "removed" {
		t.Fatalf("removed spec closure = %q, %v", stem, err)
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
		StaleIndexOK: "fixture repository has no generated index",
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
