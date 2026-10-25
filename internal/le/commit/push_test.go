// Design: docs/contributing/committing.md -- the push route over the derived verification debt
//
// VALIDATES: the consumers of the derived verification debt (AC-1, AC-3..AC-5,
// AC-15, AC-21, AC-22, AC-25..AC-29): the push verb and its push-only script,
// debt-status, the create report, the empty attestation commit and the
// ledger-deletion route.
// PREVENTS: a push of an unverified commit, an attestation that changes the
// tree, and a commit appended behind a push that would not publish it.
package commit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const pushTestAuthorisation = "the owner ordered this push in the test"

// owedClosure lands a spec and closes it with an open Review-owed trailer, and
// answers the closure's SHA.
func owedClosure(t *testing.T, root string) string {
	t.Helper()
	commitOwed(t, root, "the spec lands", map[string]string{owedSpecPath: specText("in-progress", "")}, nil)
	return commitOwed(t, root, "the spec closes", nil, []string{owedSpecPath},
		"Review-owed: "+owedSpecStem+": no reviewer free")
}

func readScript(t *testing.T, root, script string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(script)))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// TestDebtStatusExitCodes validates AC-25: exit 0 when P is HEAD and nothing
// is owed, 1 when anything is owed, 2 when the derivation cannot answer; with
// a named commit, what follows it on the branch does not count.
func TestDebtStatusExitCodes(t *testing.T) {
	root := newOwedRepository(t)
	verified := commitOwed(t, root, "the verified change", map[string]string{"a.txt": "a\n"}, nil)
	if _, code, err := answerDebtStatus(root, ""); code != 1 || err != nil {
		t.Fatalf("no green record: code %d err %v, want 1", code, err)
	}
	writeOwedGreen(t, root, verified)
	if status, code, err := answerDebtStatus(root, ""); code != 0 || err != nil || status.Owed {
		t.Fatalf("HEAD verified: code %d err %v status %+v, want 0", code, err, status)
	}
	commitOwed(t, root, "a change after the verify", map[string]string{"b.txt": "b\n"}, nil)
	status, code, err := answerDebtStatus(root, "")
	if code != 1 || err != nil {
		t.Fatalf("HEAD moved on: code %d err %v, want 1", code, err)
	}
	if !strings.Contains(status.Text(), "a change after the verify") || !strings.Contains(status.Text(), shortSHA(verified)) {
		t.Fatalf("HEAD moved on: the answer does not name the owed commit and P:\n%s", status.Text())
	}
	if _, code, err := answerDebtStatus(root, verified); code != 0 || err != nil {
		t.Fatalf("commit P with a later commit after it: code %d err %v, want 0", code, err)
	}
	if _, code, err := answerDebtStatus(root, "HEAD"); code != 1 || err != nil {
		t.Fatalf("commit HEAD the green record does not cover: code %d err %v, want 1", code, err)
	}
	runCommitGit(t, root, "branch", "--unset-upstream")
	// No upstream: the debt cannot be derived, so it could not run (D-Q8): 3.
	if _, code, err := answerDebtStatus(root, ""); code != 3 || err == nil || !strings.Contains(err.Error(), "--set-upstream-to") {
		t.Fatalf("no upstream: code %d err %v, want 3 naming git branch --set-upstream-to", code, err)
	}
}

// TestPushScriptPushesPreparedSHAToUpstream validates AC-3 and A-2: the
// push-only script re-derives against P before it pushes P, by SHA, to the
// upstream's remote and branch, and makes no commit.
func TestPushScriptPushesPreparedSHAToUpstream(t *testing.T) {
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "push-script-fixture")
	head := commitOwed(t, root, "the verified change", map[string]string{"a.txt": "a\n"}, nil)
	writeOwedGreen(t, root, head)
	prepared, code, err := preparePush(root, pushTestAuthorisation)
	if err != nil || code != 0 {
		t.Fatalf("push over a verified HEAD: code %d err %v", code, err)
	}
	if prepared.Target != head {
		t.Fatalf("push target %s, want HEAD %s", prepared.Target, head)
	}
	script := readScript(t, root, prepared.Script)
	recheck := strings.Index(script, "./le commit debt-status commit "+head+";")
	push := strings.Index(script, "git push 'origin' '"+head+":refs/heads/main'")
	if recheck < 0 || push < 0 || recheck > push {
		t.Fatalf("the script does not re-check P before pushing P to the upstream branch:\n%s", script)
	}
	if strings.Contains(script, "commit-tree") || strings.Contains(script, blockMarker) {
		t.Fatalf("the push-only script makes a commit:\n%s", script)
	}
	if !strings.Contains(prepared.Text(), "script="+prepared.Script) {
		t.Fatalf("the answer does not print the script line:\n%s", prepared.Text())
	}
}

// TestPushScriptPushesPreparedCommitAfterHeadMoved validates AC-4 and R-7:
// with a commit after the green record, the script pushes the verified commit
// G, and the answer names the later commit as left behind.
func TestPushScriptPushesPreparedCommitAfterHeadMoved(t *testing.T) {
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "push-moved-fixture")
	green := commitOwed(t, root, "the verified change", map[string]string{"a.txt": "a\n"}, nil)
	writeOwedGreen(t, root, green)
	later := commitOwed(t, root, "a change after the verify", map[string]string{"b.txt": "b\n"}, nil)
	prepared, code, err := preparePush(root, pushTestAuthorisation)
	if err != nil || code != 0 {
		t.Fatalf("push with HEAD past the green commit: code %d err %v", code, err)
	}
	script := readScript(t, root, prepared.Script)
	if !strings.Contains(script, "'"+green+":refs/heads/main'") || strings.Contains(script, later) {
		t.Fatalf("the script does not push exactly the green commit %s:\n%s", green, script)
	}
	text := prepared.Text()
	if !strings.Contains(text, shortSHA(later)+" a change after the verify") || !strings.Contains(text, scopedCommand) {
		t.Fatalf("the answer does not name the commit left behind and its verify:\n%s", text)
	}
}

// TestPushOntoCommitScriptRefused validates AC-29: a push is refused while
// this session holds a commit script not yet run, and prepared once it ran.
func TestPushOntoCommitScriptRefused(t *testing.T) {
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "push-onto-commit-fixture")
	commitOwed(t, root, "a commit ahead of the upstream", map[string]string{"a.txt": "a\n"}, nil)
	writeCommitFixture(t, root, "c.txt", "c\n")
	created, err := Create(root, &Options{Subject: "pending change", Files: []string{"c.txt"}, NoTest: "fixture"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	writeOwedGreen(t, root, revParse(t, root, "HEAD"))
	_, code, err := preparePush(root, pushTestAuthorisation)
	// The push gate ran and found an unrun commit script (D-Q8): 1.
	if err == nil || code != 1 || !strings.Contains(err.Error(), "bash "+created.Script) {
		t.Fatalf("push over an unrun commit script: code %d err %v, want a refusal naming bash %s", code, err, created.Script)
	}
	runCommitScript(t, root, created.Script)
	writeOwedGreen(t, root, revParse(t, root, "HEAD"))
	if _, code, err := preparePush(root, pushTestAuthorisation); err != nil || code != 0 {
		t.Fatalf("push once the commit script ran: code %d err %v", code, err)
	}
}

// TestAppendOntoPushOnlyScriptRefused validates AC-29 and R-10: a block
// appended to a push-only script would not be published, so append is refused
// naming the push script and replace; replace takes the script over.
func TestAppendOntoPushOnlyScriptRefused(t *testing.T) {
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "append-onto-push-fixture")
	writeOwedGreen(t, root, commitOwed(t, root, "the verified change", map[string]string{"a.txt": "a\n"}, nil))
	prepared, _, err := preparePush(root, pushTestAuthorisation)
	if err != nil {
		t.Fatal(err)
	}
	writeCommitFixture(t, root, "d.txt", "d\n")
	_, err = Create(root, &Options{Subject: "appended", Files: []string{"d.txt"}, NoTest: "fixture", Append: true})
	if err == nil || !strings.Contains(err.Error(), "run the push script first, or use replace") {
		t.Fatalf("append onto a push-only script: %v, want the refusal naming the push script and replace", err)
	}
	replaced, err := Create(root, &Options{Subject: "replaced", Files: []string{"d.txt"}, NoTest: "fixture",
		Replace: true, Script: prepared.Script})
	if err != nil {
		t.Fatalf("replace onto a push-only script: %v", err)
	}
	if script := readScript(t, root, replaced.Script); strings.Contains(script, pushMarker) {
		t.Fatalf("replace kept the push:\n%s", script)
	}
}

// TestAttestPreparesEmptyCommit validates AC-15 and D-7: an attestation with
// no file lands as an empty commit carrying Owner-attested: keyed by the owed
// commit's stable patch-id, which answers the item without a new verify.
func TestAttestPreparesEmptyCommit(t *testing.T) {
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "attest-empty-fixture")
	closure := owedClosure(t, root)
	writeOwedGreen(t, root, closure)
	if derived := deriveOwed(t, root, ""); derived.pushRefusal() == nil {
		t.Fatal("a push with an open Review-owed item was admitted")
	}
	prepared, err := Create(root, &Options{Subject: "owner attests the closure", Attest: shortSHA(closure),
		AttestWords: "Thomas reviewed it in person"})
	if err != nil {
		t.Fatalf("create attest: %v", err)
	}
	runCommitScript(t, root, prepared.Script)
	if revParse(t, root, "HEAD^{tree}") != revParse(t, root, "HEAD^^{tree}") {
		t.Fatal("the attestation commit changed the tree")
	}
	message := runCommitGitOutput(t, root, "log", "-1", "--format=%B")
	want := "Owner-attested: " + stablePatchID(t, root, closure) + ": Thomas reviewed it in person"
	if !strings.Contains(message, want) {
		t.Fatalf("the attestation message is\n%s\nwant the line %q", message, want)
	}
	derived := deriveOwed(t, root, "")
	if err := derived.pushRefusal(); err != nil {
		t.Fatalf("the attested push is still refused: %v", err)
	}
	if derived.Target != pushTargetHead {
		t.Fatalf("push target %d, want HEAD with no new verify", derived.Target)
	}
}

// TestAttestWithFileRefused validates AC-27 and the attestation scope of the
// Security Review Checklist: an attestation that names a path, a commit
// already published, and a commit owing nothing are each refused.
func TestAttestWithFileRefused(t *testing.T) {
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "attest-refused-fixture")
	closure := owedClosure(t, root)
	writeCommitFixture(t, root, "e.txt", "e\n")
	_, err := Create(root, &Options{Subject: "attest and change", Attest: closure, AttestWords: "fine",
		Files: []string{"e.txt"}})
	if err == nil || !strings.Contains(err.Error(), "must not change the tree") {
		t.Fatalf("attest with a file: %v, want the refusal naming the tree", err)
	}
	plain := commitOwed(t, root, "a plain change", map[string]string{"f.txt": "f\n"}, nil)
	_, err = Create(root, &Options{Subject: "attest nothing", Attest: plain, AttestWords: "fine"})
	if err == nil || !strings.Contains(err.Error(), "owes no person act") {
		t.Fatalf("attest of a commit owing nothing: %v", err)
	}
	runCommitGit(t, root, "push", "-q", "origin", "main")
	_, err = Create(root, &Options{Subject: "attest the published", Attest: closure, AttestWords: "fine"})
	if err == nil || !strings.Contains(err.Error(), "already on the upstream") {
		t.Fatalf("attest of a published commit: %v", err)
	}
}

// TestEmptyAttestationBlockRefusesAnyPath validates AC-28: the empty block
// builds on its parent's tree, and a block whose diff names a path is refused
// with HEAD unchanged. The second half edits the generated script so the
// commit takes another tree, which only a defect could produce.
func TestEmptyAttestationBlockRefusesAnyPath(t *testing.T) {
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "attest-stray-fixture")
	closure := owedClosure(t, root)
	prepared, err := Create(root, &Options{Subject: "owner attests", Attest: closure, AttestWords: "Thomas agrees"})
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, filepath.FromSlash(prepared.Script))
	text := readScript(t, root, prepared.Script)
	if !strings.Contains(text, `"$_ze_parent^{tree}"`) {
		t.Fatalf("the empty block does not build on its parent's tree:\n%s", text)
	}
	stray := strings.Replace(text, `"$_ze_parent^{tree}"`, `"$(git rev-parse 'HEAD~1^{tree}')"`, 1)
	if err := os.WriteFile(script, []byte(stray), 0o700); err != nil { //nolint:gosec // the test runs the script it rewrote
		t.Fatal(err)
	}
	before := revParse(t, root, "HEAD")
	command := exec.CommandContext(t.Context(), "bash", script)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "must not change the tree") {
		t.Fatalf("an empty block whose commit changes paths ran: %v\n%s", err, output)
	}
	if revParse(t, root, "HEAD") != before {
		t.Fatal("a refused empty block moved HEAD")
	}
}

// TestCreateReportsDerivedDebt validates AC-1: create stages only the named
// paths, no ledger file, and reports the derived count since the green record.
func TestCreateReportsDerivedDebt(t *testing.T) {
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "create-debt-fixture")
	green := commitOwed(t, root, "the verified change", map[string]string{"a.txt": "a\n"}, nil)
	writeOwedGreen(t, root, green)
	commitOwed(t, root, "a change after the verify", map[string]string{"b.txt": "b\n"}, nil)
	writeCommitFixture(t, root, "g.txt", "g\n")
	prepared, err := Create(root, &Options{Subject: "one more", Files: []string{"g.txt"}, NoTest: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Added) != 1 || prepared.Added[0] != "g.txt" || len(prepared.Removed) != 0 {
		t.Fatalf("create staged %v and removed %v, want g.txt alone", prepared.Added, prepared.Removed)
	}
	want := "debt=1 commit(s) owe a scoped verify since " + shortSHA(green) + ": run ./le verify changed"
	if !strings.Contains(prepared.Text(), want) {
		t.Fatalf("the create report is\n%s\nwant %q", prepared.Text(), want)
	}
}

// TestLedgerCarriedTrailersKeyUnpushedRows validates AC-26 and D-8: a carried
// row on a published commit carries no patch-id, one on an unpushed commit
// carries its stable patch-id and holds the push, and the route refuses a
// commit that adds a file.
func TestLedgerCarriedTrailersKeyUnpushedRows(t *testing.T) {
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "ledger-carried-fixture")
	published := commitOwed(t, root, "a published change", map[string]string{"ledger/old.md": "row\n"}, nil)
	runCommitGit(t, root, "push", "-q", "origin", "main")
	commitOwed(t, root, "the spec lands", map[string]string{owedSpecPath: specText("in-progress", "")}, nil)
	closure := commitOwed(t, root, "the spec closes", nil, []string{owedSpecPath})
	rows := []string{published + " review-override: published long ago", shortSHA(closure) + " review-override: no reviewer"}
	_, err := Create(root, &Options{Subject: "the ledger goes", Files: []string{"tracked.txt"}, LedgerCarried: rows, NoTest: "fixture"})
	if err == nil || !strings.Contains(err.Error(), "adds no file") {
		t.Fatalf("ledger-carried on a commit adding a file: %v", err)
	}
	prepared, err := Create(root, &Options{Subject: "the ledger goes", Remove: []string{"ledger/old.md"}, LedgerCarried: rows, NoTest: "fixture"})
	if err != nil {
		t.Fatalf("create the deletion commit: %v", err)
	}
	wantAudit := "Ledger-carried: " + published + " review-override: published long ago"
	wantGate := "Ledger-carried: " + closure + " " + stablePatchID(t, root, closure) + " review-override: no reviewer"
	if len(prepared.Trailers) != 2 || prepared.Trailers[0] != wantAudit || prepared.Trailers[1] != wantGate {
		t.Fatalf("trailers %q, want %q and %q", prepared.Trailers, wantAudit, wantGate)
	}
	runCommitScript(t, root, prepared.Script)
	writeOwedGreen(t, root, revParse(t, root, "HEAD"))
	assertOneItem(t, deriveOwed(t, root, ""), closure, owedItemReview, owedSpecStem)
}

// TestDebtWarningListsAtMostFiveItems validates AC-22: one line naming the
// count and the green commit, at most five items, and the rest counted.
func TestDebtWarningListsAtMostFiveItems(t *testing.T) {
	root := newOwedRepository(t)
	green := commitOwed(t, root, "the verified change", map[string]string{"a.txt": "a\n"}, nil)
	writeOwedGreen(t, root, green)
	if warning, err := DebtWarning(root); err != nil || warning != "" {
		t.Fatalf("a verified HEAD warned %q (%v)", warning, err)
	}
	for _, name := range []string{"h1", "h2", "h3", "h4", "h5", "h6", "h7"} {
		commitOwed(t, root, "change "+name, map[string]string{name + ".txt": name + "\n"}, nil)
	}
	warning, err := DebtWarning(root)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(warning, "\n"), "\n")
	if lines[0] != "verification debt: 7 commit(s) owe a scoped verify since "+shortSHA(green)+": run ./le verify changed, push refused until cleared" {
		t.Fatalf("first line %q", lines[0])
	}
	if len(lines) != 7 || !strings.Contains(lines[6], "and 2 more") {
		t.Fatalf("the warning is\n%s\nwant five items and the remaining two counted", warning)
	}
}

// TestRemovedKeywordsNameReplacement validates AC-21: each keyword the
// derived debt removed from create is refused naming its replacement.
func TestRemovedKeywordsNameReplacement(t *testing.T) {
	for keyword, replacement := range map[string]string{
		"unverified": "./le verify changed", "missing-full-verify-ok": "./le verify changed",
		"push": `./le commit push "<owner authorisation>"`,
	} {
		_, err := parseCreate([]string{"subject", "s", keyword})
		if err == nil || !strings.Contains(err.Error(), replacement) {
			t.Fatalf("create %s: %v, want the refusal naming %s", keyword, err, replacement)
		}
	}
	// The table names the value the line left out, the attestation's second.
	if _, err := parseCreate([]string{"subject", "s", "attest", "abc"}); err == nil || !strings.Contains(err.Error(), "requires <owner words>") {
		t.Fatalf("attest with one value: %v", err)
	}
}

// TestPushRefusedInLinkedWorktreeWithoutUpstream validates the push half of
// AC-12: in a linked worktree whose branch tracks nothing, the push is
// refused with exit 2 naming the git command that sets an upstream, and no
// script is written. A missing upstream is a question the derivation cannot
// answer, never an empty range read as nothing owed.
func TestPushRefusedInLinkedWorktreeWithoutUpstream(t *testing.T) {
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "push-worktree-fixture")
	head := commitOwed(t, root, "the verified change", map[string]string{"a.txt": "a\n"}, nil)
	writeOwedGreen(t, root, head)
	linked := filepath.Join(t.TempDir(), "linked")
	runCommitGit(t, root, "worktree", "add", "-q", "-b", "feature", linked)
	// The fixture's checkout markers are untracked, so the linked worktree is
	// given its own: a real Ze worktree carries them, and without them the
	// derivation would answer NOT-APPLICABLE instead of judging the upstream.
	writeCommitFixture(t, linked, "go.mod", "module fixture\n")
	writeCommitFixture(t, linked, "feature-gates.txt", "")
	writeOwedGreen(t, linked, head)

	prepared, code, err := preparePush(linked, pushTestAuthorisation)
	// No upstream: the push could not be prepared (D-Q8): 3.
	if code != 3 || err == nil {
		t.Fatalf("push from a linked worktree with no upstream: code %d err %v, want 3", code, err)
	}
	if !strings.Contains(err.Error(), "git branch --set-upstream-to") {
		t.Fatalf("the refusal %q does not name git branch --set-upstream-to", err)
	}
	if prepared.Script != "" {
		t.Fatalf("a refused push named a script %q", prepared.Script)
	}
}

// TestPushOutsideTheZeCheckoutCarriesNoRecheck validates AC-23: in a tree
// that is not the Ze checkout nothing is derived, so the push-only script
// publishes HEAD as it stood at preparation, carries no debt-status re-check
// (the tree has no ./le to run it), and the answer says NOT-APPLICABLE.
func TestPushOutsideTheZeCheckoutCarriesNoRecheck(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "push-site-fixture")
	site := newSiteRepository(t)
	remote := filepath.Join(t.TempDir(), "origin.git")
	runCommitGit(t, site, "init", "-q", "--bare", "--initial-branch=main", remote)
	runCommitGit(t, site, "branch", "-M", "main")
	runCommitGit(t, site, "remote", "add", "origin", remote)
	runCommitGit(t, site, "push", "-q", "-u", "origin", "main")
	writeCommitFixture(t, site, "page.html", "<p>published</p>\n")
	runCommitGit(t, site, "add", "--", "page.html")
	runCommitGit(t, site, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
		"commit", "-q", "-m", "site: publish")
	head := revParse(t, site, "HEAD")

	prepared, code, err := preparePush(site, pushTestAuthorisation)
	if err != nil || code != 0 {
		t.Fatalf("push outside the Ze checkout: code %d err %v, want 0", code, err)
	}
	if prepared.Applicable || prepared.Target != head {
		t.Fatalf("applicable %v target %s, want not applicable and HEAD %s", prepared.Applicable, prepared.Target, head)
	}
	script := readScript(t, site, prepared.Script)
	if strings.Contains(script, debtStatusCommand) {
		t.Fatalf("the script outside the Ze checkout carries a re-check:\n%s", script)
	}
	if !strings.Contains(script, "git push 'origin' '"+head+":refs/heads/main'") {
		t.Fatalf("the script does not push HEAD %s to the upstream branch:\n%s", head, script)
	}
	if !strings.Contains(prepared.Text(), "verify=NOT-APPLICABLE") {
		t.Fatalf("the answer does not say NOT-APPLICABLE:\n%s", prepared.Text())
	}
}
