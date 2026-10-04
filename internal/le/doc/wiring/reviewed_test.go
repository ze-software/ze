package docwiring

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// reviewedShard is the fixture's ledger shard, named like a commit session.
const reviewedShard = ReviewedDir + "/abcd1234.md"

// reviewedTree writes the doc-drift fixture, rewrites Documented in one
// unpushed commit, and answers the root and that commit.
func reviewedTree(t *testing.T) (string, string) {
	t.Helper()
	root := docDriftTree(t)
	editDocumented(t, root)
	return root, commitFixture(t, root, "rewrite Documented")
}

// commitFixture commits the whole working tree and answers the new commit.
func commitFixture(t *testing.T, root, message string) string {
	t.Helper()
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "-c", "user.email=t@ze", "-c", "user.name=t", "commit", "-qm", message)
	return headOf(t, root)
}

// headOf answers the fixture's HEAD commit.
func headOf(t *testing.T, root string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output() //nolint:gosec,noctx // this test's own fixture
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// writeFixture writes one file of the fixture checkout.
func writeFixture(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// writeReviewed writes the fixture's shard holding rows under the header.
func writeReviewed(t *testing.T, root string, rows ...string) {
	t.Helper()
	body := "# Reviewed claims\n\n| Doc | Source | Symbol | Commit | Reason |\n|-----|--------|--------|--------|--------|\n" +
		strings.Join(rows, "\n") + "\n"
	writeFixture(t, root, reviewedShard, body)
}

// reviewedRowFor answers one row for the fixture's claim on Documented.
func reviewedRowFor(commit, reason string) string {
	return "| docs/one.md | `internal/x/x.go` | Documented | " + commit + " | " + reason + " |" // <!-- doc-links: ignore (source exists only in the disposable reviewed-claim fixture) -->
}

// driftResult answers the doc-drift result of one Run through the gate entry.
func driftResult(t *testing.T, root string) CheckResult {
	t.Helper()
	report, _ := Run(root, Options{})
	for _, check := range report.Checks {
		if check.Name == checkDocDriftName {
			return check
		}
	}
	t.Fatalf("the run held no doc-drift result: %+v", report.Checks)
	return CheckResult{}
}

// VALIDATES: a reviewed row naming the unpushed commit that changed the
// claimed symbol accepts the claim, and the verdict line counts it. A later
// edit to ANOTHER symbol of the same file leaves the row covering.
// PREVENTS: a behavior-preserving change owing a page edit made for the gate.
func TestAReviewedClaimIsAccepted(t *testing.T) {
	root, commit := reviewedTree(t)
	writeReviewed(t, root, reviewedRowFor(commit, "literal changed, claim unaffected"))

	result := driftResult(t, root)
	if result.Failed {
		t.Fatalf("a reviewed claim was refused: %+v", result.Violations)
	}
	if !strings.Contains(result.Output, "accepted by review (1 claim(s)") {
		t.Errorf("the verdict line does not count the reviewed claim: %q", result.Output)
	}

	// A later edit to Other reopens the claim on Other and leaves the row on
	// Documented covering, because the row is judged at the symbol.
	writeFixture(t, root, "internal/x/x.go", "package x\n\nfunc Documented() int { return 42 }\n\nfunc Other() int { return 3 }\n")
	result = driftResult(t, root)
	found := strings.Join(result.Violations, "\n")
	if !strings.Contains(found, "docs/two.md:4") {
		t.Errorf("the edit to Other was not reported: %q", found)
	}
	if strings.Contains(found, "docs/one.md") {
		t.Errorf("an edit to Other reopened the reviewed claim on Documented: %q", found)
	}
}

// shortOf answers the ten-digit spelling a finding uses for a commit.
func shortOf(commit string) string {
	return commit[:10]
}

// driftText answers every violation of one doc-drift result as one string.
func driftText(result CheckResult) string {
	return strings.Join(result.Violations, "\n")
}

// VALIDATES: a claim whose symbol two unpushed commits changed is covered only
// when both commits have a row, and the finding names the commit that has none.
// PREVENTS: a row for the last commit accepting an earlier change nobody read.
func TestEveryCommitThatChangedTheSymbolNeedsARow(t *testing.T) {
	root, first := reviewedTree(t)
	writeFixture(t, root, "internal/x/x.go", "package x\n\nfunc Documented() int { return 43 }\n\nfunc Other() int { return 2 }\n")
	second := commitFixture(t, root, "rewrite Documented again")

	writeReviewed(t, root, reviewedRowFor(second, "literal changed"))
	result := driftResult(t, root)
	found := driftText(result)
	if !result.Failed || !strings.Contains(found, shortOf(first)+" changed Documented and has no reviewed row") {
		t.Fatalf("an unreviewed commit was covered by the other commit's row: %+v", result)
	}
	if strings.Contains(found, shortOf(second)+" changed Documented") {
		t.Errorf("the reviewed commit was named as unreviewed: %q", found)
	}

	writeReviewed(t, root, reviewedRowFor(first, "literal changed"), reviewedRowFor(second, "literal changed"))
	if result := driftResult(t, root); result.Failed {
		t.Fatalf("both commits reviewed and the claim was refused: %+v", result.Violations)
	}
}

// VALIDATES: a working-tree edit to a reviewed symbol reopens the claim.
// PREVENTS: a row accepting an edit made after the review.
func TestAWorkingTreeEditReopensAReviewedClaim(t *testing.T) {
	root, commit := reviewedTree(t)
	writeReviewed(t, root, reviewedRowFor(commit, "literal changed"))
	writeFixture(t, root, "internal/x/x.go", "package x\n\nfunc Documented() int { return 7 }\n\nfunc Other() int { return 2 }\n")

	result := driftResult(t, root)
	if !result.Failed || !strings.Contains(driftText(result), "the working tree changed Documented") {
		t.Fatalf("a working-tree edit stayed covered: %+v", result)
	}
}

// VALIDATES: a row naming a pushed commit covers nothing, and the finding names
// the unpushed commit that still needs a row.
// PREVENTS: an old review accepting a new change.
func TestAPushedCommitCoversNothing(t *testing.T) {
	root, commit := reviewedTree(t)
	out, err := exec.Command("git", "-C", root, "rev-parse", "refs/remotes/origin/main").Output() //nolint:gosec,noctx // this test's own fixture
	if err != nil {
		t.Fatal(err)
	}
	writeReviewed(t, root, reviewedRowFor(strings.TrimSpace(string(out)), "reviewed"))

	result := driftResult(t, root)
	if !result.Failed || !strings.Contains(driftText(result), shortOf(commit)+" changed Documented and has no reviewed row") {
		t.Fatalf("a pushed commit's row covered an unpushed change: %+v", result)
	}
}

// VALIDATES: each row that is wrong on its own fails the gate with a finding
// naming the shard and line: an empty reason, a wrong cell count, a commit the
// repository does not hold, a ref in place of an id, a key no anchor claims, a
// row after a break in the table, an unpushed commit that did not change the
// symbol, and a shard with no table. The claim stays a finding.
// PREVENTS: a malformed row read as an empty ledger, which hides the review a
// writer believes they recorded, and a moving ref covering later changes.
func TestAMalformedReviewedRowIsReported(t *testing.T) {
	root, commit := reviewedTree(t)
	writeFixture(t, root, "notes.txt", "unrelated\n")
	unrelated := commitFixture(t, root, "unrelated")
	writeReviewed(t, root,
		reviewedRowFor(commit, ""),
		"| docs/one.md | internal/x/x.go | Documented | "+commit+" |",
		reviewedRowFor("deadbeef", "reviewed"),
		reviewedRowFor("HEAD", "reviewed"),
		"| docs/one.md | internal/x/x.go | Missing | "+commit+" | reviewed |",
		reviewedRowFor(unrelated, "reviewed"),
		"",
		"| after | a | break |",
	)
	writeFixture(t, root, ReviewedDir+"/ffff0000.md", "no table here\n")

	result := driftResult(t, root)
	if !result.Failed {
		t.Fatalf("a ledger of malformed rows passed: %+v", result)
	}
	found := driftText(result)
	for _, want := range []string{
		"docs/one.md:4",
		reviewedShard + ":5 names no Reason",
		reviewedShard + ":6 has 4 cells",
		reviewedShard + ":7 names commit deadbeef, which this repository does not hold",
		reviewedShard + ":8 names commit HEAD, which is not a 7 to 40 digit lowercase hex id",
		reviewedShard + ":9 names docs/one.md internal/x/x.go Missing, which matches no claim anchor",
		reviewedShard + ":10 " + unrelated + " did not change Documented",
		reviewedShard + ":12 has 3 cells",
		ReviewedDir + "/ffff0000.md has no `| Doc | Source | Symbol | Commit | Reason |` table",
	} {
		if !strings.Contains(found, want) {
			t.Errorf("no finding carries %q:\n%s", want, found)
		}
	}
}

// VALIDATES: a malformed shard fails the gate when no Go source changed.
// PREVENTS: a broken ledger waiting silently for the next Go edit.
func TestTheLedgerIsJudgedWhenNoGoChanged(t *testing.T) {
	root := docDriftTree(t)
	writeReviewed(t, root, reviewedRowFor("HEAD", "reviewed"))

	result := driftResult(t, root)
	if !result.Failed || !strings.Contains(driftText(result), reviewedShard+":5 names commit HEAD") {
		t.Fatalf("a malformed shard passed with no Go change: %+v", result)
	}
}

// VALIDATES: a page edited with its symbol still passes with no ledger row.
// PREVENTS: the ledger becoming the only way to pass.
func TestAPageChangeStillPassesWithoutAReviewedRow(t *testing.T) {
	root, _ := reviewedTree(t)
	writeFixture(t, root, "docs/one.md", "# One\n\nDocumented answers forty-two.\n<!-- source: internal/x/x.go -- Documented -->\n")

	if result := driftResult(t, root); result.Failed {
		t.Fatalf("a page changed with its symbol was refused: %+v", result.Violations)
	}
}

// VALIDATES: a symbol edited in one commit and carried to a new path by a
// rename in the next is covered by no row, and the finding names the rename:
// not a row naming the editing commit, and not a row naming a pushed commit.
// PREVENTS: a row vouching for a change the commit walk cannot see.
func TestARenamedSymbolIsNotCoveredByAnyRow(t *testing.T) {
	body := "package x\n\nfunc Documented() int { return 1 }\n"
	root := pushedTree(t, map[string]string{
		"internal/x/old.go": body,
		"docs/one.md":       "# One\n\nDocumented answers one.\n<!-- source: internal/x/new.go -- Documented -->\n",
	})
	pushed := headOf(t, root)
	writeFixture(t, root, "internal/x/old.go", strings.Replace(body, "return 1", "return 42", 1))
	edited := commitFixture(t, root, "edit Documented")
	gitIn(t, root, "mv", "internal/x/old.go", "internal/x/new.go")
	renamed := commitFixture(t, root, "rename")

	for name, commit := range map[string]string{"editing commit": edited, "pushed commit": pushed} {
		t.Run(name, func(t *testing.T) {
			writeFixture(t, root, reviewedShard, "| Doc | Source | Symbol | Commit | Reason |\n|---|---|---|---|---|\n"+
				"| docs/one.md | internal/x/new.go | Documented | "+commit+" | reviewed |\n")
			result := driftResult(t, root)
			want := "internal/x/new.go was renamed in the unpushed range (" + shortOf(renamed) + "): edit the page"
			if !result.Failed || !strings.Contains(driftText(result), want) {
				t.Fatalf("a renamed symbol was covered by a row, want %q: %+v", want, result)
			}
		})
	}
}

// VALIDATES: an edit to the symbol, then a rename into the claim's source,
// then a second edit is refused when the only row names the last commit. The
// test drives Run, so the refusal comes from the gate, not from a helper.
// PREVENTS: the commit walk, which reads each commit at the new path, skipping
// the pre-rename edit and letting a row for a later commit vouch for it.
func TestAnEditRenameEditIsNotCoveredByTheLastRow(t *testing.T) {
	body := "package x\n\nfunc Documented() int { return 1 }\n"
	root := pushedTree(t, map[string]string{
		"internal/x/old.go": body,
		"docs/one.md":       "# One\n\nDocumented answers one.\n<!-- source: internal/x/new.go -- Documented -->\n",
	})
	writeFixture(t, root, "internal/x/old.go", strings.Replace(body, "return 1", "return 42", 1))
	commitFixture(t, root, "edit Documented")
	gitIn(t, root, "mv", "internal/x/old.go", "internal/x/new.go")
	renamed := commitFixture(t, root, "rename")
	writeFixture(t, root, "internal/x/new.go", strings.Replace(body, "return 1", "return 42 + 0", 1))
	last := commitFixture(t, root, "edit Documented again")
	writeFixture(t, root, reviewedShard, "| Doc | Source | Symbol | Commit | Reason |\n|---|---|---|---|---|\n"+
		"| docs/one.md | internal/x/new.go | Documented | "+last+" | same value |\n")

	result := driftResult(t, root)
	want := "internal/x/new.go was renamed in the unpushed range (" + shortOf(renamed) + "): edit the page"
	if !result.Failed || !strings.Contains(driftText(result), want) {
		t.Fatalf("a row for the post-rename commit covered the pre-rename edit, want %q: %+v", want, result)
	}
}

// VALIDATES: a merge commit's own first-parent change to a symbol needs its
// own row, beside the row for the side commit that made the edit.
// PREVENTS: a merge that changed the symbol passing with no row, because a
// merge lists no paths unless first-parent diffs are asked for.
func TestAMergeThatChangedTheSymbolNeedsARow(t *testing.T) {
	root := docDriftTree(t)
	gitIn(t, root, "checkout", "-q", "-b", "side")
	editDocumented(t, root)
	side := commitFixture(t, root, "edit Documented on a side branch")
	gitIn(t, root, "checkout", "-q", "-")
	writeFixture(t, root, "notes.txt", "unrelated\n")
	commitFixture(t, root, "unrelated")
	gitIn(t, root, "-c", "user.email=t@ze", "-c", "user.name=t", "merge", "-q", "--no-ff", "-m", "merge side", "side")
	merge := headOf(t, root)

	writeReviewed(t, root, reviewedRowFor(side, "literal changed"))
	result := driftResult(t, root)
	if !result.Failed || !strings.Contains(driftText(result), shortOf(merge)+" changed Documented and has no reviewed row") {
		t.Fatalf("the merge's change to Documented needed no row: %+v", result)
	}

	writeReviewed(t, root, reviewedRowFor(side, "literal changed"), reviewedRowFor(merge, "merge carries the same literal"))
	if result := driftResult(t, root); result.Failed {
		t.Fatalf("both commits reviewed and the claim was refused: %+v", result.Violations)
	}
}
