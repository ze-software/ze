//go:build unix

// Design: docs/contributing/rfc-implementation-guide.md -- judge approvals
// Related: ledger.go -- the shared approval loader exercised with real Git
package testweakened

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// TestLoadApprovalsSharesOneRequest proves a multi-row load retains live,
// stale and spent verdicts while reading the source inventory and history
// once. Method: issue real approvals in a committed fixture, then observe
// the existing loader through a recording Git that delegates to real Git.
// Operation counts expose repeated work without a wall-clock threshold or
// a reader that fabricates approval results.
func TestLoadApprovalsSharesOneRequest(t *testing.T) {
	root, source, issued := approvalRequestFixture(t)
	logPath := fakeGit(t, "")
	rows, problems := LoadApprovals(root, source)
	if len(problems) != 0 || len(rows) != len(issued) {
		t.Fatalf("LoadApprovals = %+v, %v; want all four issued rows", rows, problems)
	}
	for index, row := range rows {
		if row.Row != issued[index] || row.Trailer != source.TrailerLine(row.Name, row.Reason) {
			t.Errorf("row %d = %+v; want unchanged issued row %+v and its exact trailer", index, row, issued[index])
		}
	}
	if rows[0].Stale != "" || rows[0].Carrier != "" || rows[1].Stale != "" || rows[1].Carrier != "" {
		t.Errorf("live rows = %+v; want both usable and unspent", rows[:2])
	}
	if rows[2].Stale != "the unit changed after the verdict, so the verdict judged another change" || rows[2].Carrier != "" {
		t.Errorf("changed row = %+v; want its Content mismatch and no carrier", rows[2])
	}
	if rows[3].Stale != "" || !strings.HasSuffix(rows[3].Carrier, " spent approval") {
		t.Errorf("spent row = %+v; want the actual landed carrier", rows[3])
	}

	counts := make(map[string]int)
	for _, call := range readGitCalls(t, logPath) {
		switch call.verb() {
		case "ls-tree", "ls-files", "show", "cat-file":
			counts[call.verb()]++
		case "log":
			for _, argument := range call.args {
				if argument == "--format=%H" {
					counts["history inventory"]++
				}
			}
		}
		if !call.readOnlyQuery() {
			t.Errorf("git %q lacks the existing read-only query environment", call.args)
		}
	}
	for operation, want := range map[string]int{
		"ls-tree": 1, "ls-files": 1, "show": 2, "history inventory": 1, "cat-file": 1,
	} {
		if got := counts[operation]; got != want {
			t.Errorf("%s calls = %d, want %d for one load of four rows over two carriers", operation, got, want)
		}
	}
}

// TestLoadApprovalsStartsFresh proves a later request sees both changed
// source bytes and a new untracked duplicate. Method: load real issued rows,
// edit one live declaration, add a competing carrier for the other, and load
// again without reissuing either approval. A request cache must not turn the
// first load's useful verdicts into authority for those later inputs.
func TestLoadApprovalsStartsFresh(t *testing.T) {
	root, source, _ := approvalRequestFixture(t)
	rows, problems := LoadApprovals(root, source)
	if len(problems) != 0 || len(rows) != 4 || rows[0].Stale != "" || rows[1].Stale != "" {
		t.Fatalf("first load = %+v, %v; want two live controls", rows, problems)
	}
	writeApproveCarrier(t, root, "pkg/first_test.go",
		"package pkg\n\nfunc TestA(t *testing.T) { t.Skip(\"changed again\") }\n\nfunc TestB(t *testing.T) { t.Skip(\"judged B\") }\n")
	writeApproveCarrier(t, root, "pkg/duplicate_test.go",
		"package pkg\n\nfunc TestB(t *testing.T) { t.Skip(\"competing B\") }\n")
	rows, problems = LoadApprovals(root, source)
	if len(problems) != 0 || len(rows) != 4 {
		t.Fatalf("fresh load = %+v, %v; want four rows with individual stale reasons", rows, problems)
	}
	if rows[0].Stale != "the unit changed after the verdict, so the verdict judged another change" {
		t.Errorf("changed A = %+v; want its new Content refused", rows[0])
	}
	if !strings.HasPrefix(rows[1].Stale, "its unit no longer resolves: ") ||
		!strings.Contains(rows[1].Stale, "first_test.go") || !strings.Contains(rows[1].Stale, "duplicate_test.go") {
		t.Errorf("duplicated B = %+v; want ambiguity naming both current carriers", rows[1])
	}
	if rows[2].Stale == "" || rows[3].Stale != "" || !strings.HasSuffix(rows[3].Carrier, " spent approval") {
		t.Errorf("independent stale/spent controls = %+v; want their original verdicts", rows[2:])
	}
}

// TestLoadApprovalsRefusesFailedHistoryAfterMatches proves successfully read
// commit frames cannot authorize rows when Git then exits unsuccessfully.
// Method: the existing recording seam delegates the actual batch to real Git
// and changes only its exit status, after the real spent row has matched.
func TestLoadApprovalsRefusesFailedHistoryAfterMatches(t *testing.T) {
	root, source, _ := approvalRequestFixture(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	quotedGit := "'" + strings.ReplaceAll(realGit, "'", "'\\''") + "'"
	fakeGit(t, "if [ \"$1\" = cat-file ]; then "+quotedGit+" \"$@\"; exit 19; fi\n")
	rows, problems := LoadApprovals(root, source)
	if rows != nil || len(problems) != 1 || !strings.Contains(problems[0], "cannot read Git approval history") ||
		!strings.Contains(problems[0], "exit status 19") {
		t.Fatalf("failed batch after real matches = %+v, %v; want a closed load", rows, problems)
	}
}

// TestLoadApprovalsEmptyLedgerDoesNotReadGit proves an empty valid ledger
// does not acquire source or history authority. Method: record every attempted
// Git query and refuse it; the existing empty-table outcome must still hold.
func TestLoadApprovalsEmptyLedgerDoesNotReadGit(t *testing.T) {
	root := t.TempDir()
	source := WeakeningApprovals(approveSession)
	writeApproveCarrier(t, root, source.Path, rfc.ApprovalTableHeader)
	logPath := fakeGit(t, "exit 23\n")
	rows, problems := LoadApprovals(root, source)
	if len(rows) != 0 || len(problems) != 0 {
		t.Fatalf("empty load = %+v, %v; want no rows or problems", rows, problems)
	}
	if calls := readGitCalls(t, logPath); len(calls) != 0 {
		t.Fatalf("empty load queried Git: %+v", calls)
	}
}

// approvalRequestFixture issues four genuine rows across two source carriers:
// two live, one changed after judgment, and one spent in real commit history.
func approvalRequestFixture(t *testing.T) (string, ApprovalSource, []Row) {
	t.Helper()
	root := t.TempDir()
	writeApproveCarrier(t, root, "pkg/first_test.go",
		"package pkg\n\nfunc TestA(t *testing.T) {}\n\nfunc TestB(t *testing.T) {}\n")
	writeApproveCarrier(t, root, "pkg/second_test.go",
		"package pkg\n\nfunc TestC(t *testing.T) {}\n\nfunc TestD(t *testing.T) {}\n")
	commitProposedFixture(t, root)
	writeApproveCarrier(t, root, "pkg/first_test.go",
		"package pkg\n\nfunc TestA(t *testing.T) { t.Skip(\"judged A\") }\n\nfunc TestB(t *testing.T) { t.Skip(\"judged B\") }\n")
	writeApproveCarrier(t, root, "pkg/second_test.go",
		"package pkg\n\nfunc TestC(t *testing.T) { t.Skip(\"judged C\") }\n\nfunc TestD(t *testing.T) { t.Skip(\"judged D\") }\n")
	for _, name := range []string{"TestA", "TestB", "TestC", "TestD"} {
		if _, err := Approve(root, approveSession, "pkg."+name, "judge: "+name); err != nil {
			t.Fatalf("issue %s: %v", name, err)
		}
	}
	source := WeakeningApprovals(approveSession)
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(source.Path)))
	if err != nil {
		t.Fatal(err)
	}
	issued, problems := ParseLedger(string(content), source.Path)
	if len(problems) != 0 || len(issued) != 4 {
		t.Fatalf("issued ledger = %+v, %v", issued, problems)
	}
	runProspectiveGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
		"commit", "--allow-empty", "-q", "-m", "spent approval\n\n"+source.TrailerLine("pkg.TestD", "judge: TestD"))
	writeApproveCarrier(t, root, "pkg/second_test.go",
		"package pkg\n\nfunc TestC(t *testing.T) { t.Skip(\"changed C\") }\n\nfunc TestD(t *testing.T) { t.Skip(\"judged D\") }\n")
	return root, source, issued
}
