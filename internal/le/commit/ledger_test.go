// Design: docs/architecture/testing/test-health.md -- owner approval of a test weakening
package commit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
	testweakened "github.com/ze-software/ze/internal/le/test/weakened"
)

// The two authors these tests drive. They are ordinary commit sessions: the
// same eight hex characters that name each one's script and message.
const (
	sessionOne = "aaaaaaaa"
	sessionTwo = "bbbbbbbb"
)

// ledgerHeader opens a weakening approval file a fixture writes by hand, in
// the row grammar the approve writer produces.
const ledgerHeader = "# Test weakenings the owner approved\n\n| Test | Reason |\n|------|--------|\n"

// TestCreateRefusesAnUnapprovedWeakening proves the commit gate refuses a
// weakening no owner approval of this session names (AC-6).
//
// The method is one weakened test, prepared three times: with no approval
// file, with the approval recorded under another session, and with the
// approval under this session.
//
// VALIDATES: the refusal names the unit and the approve command; another
// session's approval admits nothing; this session's approval admits it.
// PREVENTS: a weakening committed with no approval Thomas gave this session.
func TestCreateRefusesAnUnapprovedWeakening(t *testing.T) {
	root := newLedgerRepository(t)
	weakenLedgerTest(t, root, "pkg/one_test.go", "TestOne")

	_, err := ledgerCreate(t, root, "pkg/one_test.go")
	if err == nil || !strings.Contains(err.Error(), "pkg.TestOne") ||
		!strings.Contains(err.Error(), testweakened.ApproveCommand("pkg.TestOne")) {
		t.Fatalf("Create with no approval = %v, want the unit and the approve command", err)
	}

	if _, err := testweakened.Approve(root, sessionTwo, "pkg.TestOne", "a peer's approval"); err != nil {
		t.Fatal(err)
	}
	_, err = ledgerCreate(t, root, "pkg/one_test.go")
	if err == nil || !strings.Contains(err.Error(), "no ze-test-judge approval names it") {
		t.Fatalf("Create reading a peer's approval = %v, want the weakening still refused", err)
	}

	if _, err := testweakened.Approve(root, sessionOne, "pkg.TestOne", "Thomas accepts the skip"); err != nil {
		t.Fatal(err)
	}
	if _, err := ledgerCreate(t, root, "pkg/one_test.go"); err != nil {
		t.Fatalf("Create with this session's approval = %v", err)
	}
}

// TestCreateCarriesWeakeningApprovedTrailer proves an approved weakening is
// recorded in the commit message and nowhere else (AC-7).
//
// VALIDATES: the message ends with the unwrapped `Weakening-approved:` line,
// the commit carries it, and the commit changes only the named test file.
// PREVENTS: a tracked ledger file riding along with the weakening.
func TestCreateCarriesWeakeningApprovedTrailer(t *testing.T) {
	root := newLedgerRepository(t)
	weakenLedgerTest(t, root, "pkg/one_test.go", "TestOne")
	reason := "Thomas accepts the skip until the fixture returns, which is a long reason that must never be wrapped"
	if _, err := testweakened.Approve(root, sessionOne, "pkg.TestOne", reason); err != nil {
		t.Fatal(err)
	}

	prepared, err := ledgerCreate(t, root, "pkg/one_test.go")
	if err != nil {
		t.Fatalf("an approved weakening was refused: %v", err)
	}
	trailer := testweakened.ApprovalTrailerLine("pkg.TestOne", reason)
	message := readLedgerFile(t, root, prepared.Message)
	if !strings.HasSuffix(message, "\n"+trailer+"\n") {
		t.Fatalf("the message does not end with the trailer line:\n%s", message)
	}

	output := runCommitScript(t, root, prepared.Script)
	body := runCommitGitOutput(t, root, "log", "-1", "--format=%B")
	if !strings.Contains(body, "\n"+trailer+"\n") {
		t.Fatalf("the commit does not carry the trailer:\n%s\n%s", body, output)
	}
	files := strings.TrimSpace(runCommitGitOutput(t, root, "show", "--name-only", "--format=", "HEAD"))
	if files != "pkg/one_test.go" {
		t.Fatalf("the commit changed %q, want only pkg/one_test.go", files)
	}
}

// TestOneRFCApprovalCarriesOneTrailerForATaggedWeakening proves a weakened
// RFC-tagged unit that one `./le rfc approve` row names is admitted by both
// gates through that one row, and the commit carries its trailer once.
//
// The method commits a tagged test, weakens it, records one RFC approval and
// no weakening approval, prepares the commit and runs its script.
//
// VALIDATES: the message holds exactly one `RFC-approved:` line and no
// `Weakening-approved:` line; the script drops the used row.
// PREVENTS: the RFC gate and the weakening gate each emitting the same
// trailer, and a used RFC row left to approve a second change.
func TestOneRFCApprovalCarriesOneTrailerForATaggedWeakening(t *testing.T) {
	root := newLedgerRepository(t)
	path := "pkg/tagged_test.go"
	writeCommitFixture(t, root, path, "package pkg\n\n// RFC requirement: RFC2119-1-1 positive\n"+
		"func TestTagged(t *testing.T) {\n\trequire.Equal(t, 1, got)\n}\n")
	runCommitGit(t, root, "add", "--", path)
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t",
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", "tagged baseline")
	writeCommitFixture(t, root, path, "package pkg\n\n// RFC requirement: RFC2119-1-1 positive\n"+
		"func TestTagged(t *testing.T) {\n\tt.Skip(\"later\")\n\trequire.Equal(t, 1, got)\n}\n")
	reason := "Thomas approved the skip of the tagged test"
	if _, err := rfc.Approve(root, sessionOne, "pkg.TestTagged", reason); err != nil {
		t.Fatal(err)
	}

	prepared, err := ledgerCreate(t, root, path)
	if err != nil {
		t.Fatalf("a tagged weakening with its RFC approval was refused: %v", err)
	}
	message := readLedgerFile(t, root, prepared.Message)
	if got := strings.Count(message, rfc.ApprovalTrailer); got != 1 ||
		!strings.Contains(message, "\n"+rfc.ApprovalTrailerLine("pkg.TestTagged", reason)+"\n") {
		t.Fatalf("the message carries %d RFC-approved lines, want exactly the one row's:\n%s", got, message)
	}
	if strings.Contains(message, testweakened.ApprovalTrailer) {
		t.Fatalf("the message carries a Weakening-approved line no weakening row produced:\n%s", message)
	}

	output := runCommitScript(t, root, prepared.Script)
	approvals := rfc.ApprovalPath(sessionOne)
	if after, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(approvals))); err == nil &&
		strings.Contains(string(after), "| pkg.TestTagged |") {
		t.Fatalf("the used RFC row survived the commit:\n%s\n%s", after, output)
	}
}

// TestCreateRefusesAnApprovalNamingTwoDirectoriesUnits proves the commit gate
// refuses a commit whose one approval would name tests in two directories that
// share a base name: `cmd.TestOne` names both x/cmd and y/cmd, so the trailer
// could not say which test Thomas approved.
//
// The method commits a same-named test in x/cmd and y/cmd, then changes both
// under one approval of `cmd.<Test>`: once weakened under a weakening
// approval, once as an RFC-tagged change under an RFC approval.
//
// VALIDATES: each commit is refused, naming both directories and telling the
// author to split the commit.
// PREVENTS: one approval silently admitting every same-named test in every
// same-named directory one commit touches.
func TestCreateRefusesAnApprovalNamingTwoDirectoriesUnits(t *testing.T) {
	for _, shape := range []struct {
		name    string
		before  string
		after   string
		approve func(root string) error
	}{
		{
			name:   "weakening",
			before: "package cmd\n\nfunc TestOne(t *testing.T) {\n\trequire.Equal(t, 1, got)\n}\n",
			after:  "package cmd\n\nfunc TestOne(t *testing.T) {\n\tt.Skip(\"later\")\n\trequire.Equal(t, 1, got)\n}\n",
			approve: func(root string) error {
				_, err := testweakened.Approve(root, sessionOne, "cmd.TestOne", "Thomas accepts the skip")
				return err
			},
		},
		{
			name: "rfc",
			before: "package cmd\n\n// RFC requirement: RFC2119-1-1 positive\n" +
				"func TestOne(t *testing.T) {\n\trequire.Equal(t, 1, got)\n}\n",
			after: "package cmd\n\n// RFC requirement: RFC2119-1-1 positive\n" +
				"func TestOne(t *testing.T) {\n\trequire.Equal(t, 2, got)\n}\n",
			approve: func(root string) error {
				_, err := rfc.Approve(root, sessionOne, "cmd.TestOne", "Thomas approved the new value")
				return err
			},
		},
	} {
		root := newLedgerRepository(t)
		paths := []string{"x/cmd/one_test.go", "y/cmd/one_test.go"}
		for _, path := range paths {
			writeCommitFixture(t, root, path, shape.before)
		}
		runCommitGit(t, root, append([]string{"add", "--"}, paths...)...)
		runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t",
			"-c", "commit.gpgsign=false", "commit", "-q", "-m", "two cmd directories")
		for _, path := range paths {
			writeCommitFixture(t, root, path, shape.after)
		}
		if err := shape.approve(root); err != nil {
			t.Fatal(err)
		}

		_, err := ledgerCreate(t, root, paths...)
		if err == nil {
			t.Fatalf("%s: Create admitted one cmd.TestOne approval over x/cmd and y/cmd", shape.name)
		}
		for _, want := range []string{"cmd.TestOne", "x/cmd", "y/cmd", "split the commit"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("%s: Create refusal lacks %q:\n%v", shape.name, want, err)
			}
		}
	}
}

// TestRFCChangeOKAdmitsNoUnitInTwoDirectories proves the owed-admission
// keyword is refused the same way an approval row is when the RFC-tagged
// unit it would record sits in two directories of one commit.
//
// The method commits one RFC-tagged cmd.TestOne in x/cmd and in y/cmd,
// changes both, records no approval, and prepares the commit with
// rfc-change-ok set.
//
// VALIDATES: Create is refused, naming the unit, both directories and the
// split.
// PREVENTS: an `RFC-approval-owed:` trailer naming a unit that cannot say
// which of the two tests Thomas still owes an answer for.
func TestRFCChangeOKAdmitsNoUnitInTwoDirectories(t *testing.T) {
	root := newLedgerRepository(t)
	paths := []string{"x/cmd/one_test.go", "y/cmd/one_test.go"}
	for _, path := range paths {
		writeCommitFixture(t, root, path, "package cmd\n\n// RFC requirement: RFC2119-1-1 positive\n"+
			"func TestOne(t *testing.T) {\n\trequire.Equal(t, 1, got)\n}\n")
	}
	runCommitGit(t, root, append([]string{"add", "--"}, paths...)...)
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t",
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", "two cmd directories")
	for _, path := range paths {
		writeCommitFixture(t, root, path, "package cmd\n\n// RFC requirement: RFC2119-1-1 positive\n"+
			"func TestOne(t *testing.T) {\n\trequire.Equal(t, 2, got)\n}\n")
	}

	_, err := Create(root, &Options{
		Session:     sessionOne,
		Subject:     "ledger fixture for session " + sessionOne,
		Files:       paths,
		NoTest:      "the fixture carries tests only",
		RFCChangeOK: "Thomas is away and answers on his return",
	})
	if err == nil {
		t.Fatal("Create admitted rfc-change-ok for one cmd.TestOne over x/cmd and y/cmd")
	}
	for _, want := range []string{"cmd.TestOne", "x/cmd", "y/cmd", "split the commit"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Create refusal lacks %q:\n%v", want, err)
		}
	}
}

// TestAMovedWeakenedTestIsOneSite proves a weakened test moved from one
// directory to another is one site, so its one approval admits it.
//
// The method commits cmd.TestOne in a/cmd, then moves it to b/cmd with a
// t.Skip added and records one approval: the new path is named as a file and
// the old one as a remove path, as `./le commit create` takes a move.
//
// VALIDATES: Create admits the move, every weakening finding sits in b/cmd,
// and the message carries the one `Weakening-approved:` trailer.
// PREVENTS: the two-directory refusal reading a rename's old and new path as
// two tests, which would make a weakened test impossible to move.
func TestAMovedWeakenedTestIsOneSite(t *testing.T) {
	root := newLedgerRepository(t)
	writeCommitFixture(t, root, "a/cmd/one_test.go",
		"package cmd\n\nfunc TestOne(t *testing.T) {\n\trequire.Equal(t, 1, got)\n}\n")
	runCommitGit(t, root, "add", "--", "a/cmd/one_test.go")
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t",
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", "one cmd directory")
	writeCommitFixture(t, root, "b/cmd/one_test.go",
		"package cmd\n\nfunc TestOne(t *testing.T) {\n\tt.Skip(\"later\")\n\trequire.Equal(t, 1, got)\n}\n")
	reason := "Thomas accepts the skip in its new home"
	if _, err := testweakened.Approve(root, sessionOne, "cmd.TestOne", reason); err != nil {
		t.Fatal(err)
	}

	prepared, err := Create(root, &Options{
		Session: sessionOne,
		Subject: "ledger fixture for session " + sessionOne,
		Files:   []string{"b/cmd/one_test.go"},
		Remove:  []string{"a/cmd/one_test.go"},
		NoTest:  "the fixture carries tests only",
	})
	if err != nil {
		t.Fatalf("a moved weakened test with one approval was refused: %v", err)
	}
	if len(prepared.Weakened) == 0 {
		t.Fatal("the move reported no weakening, so the approval was never judged")
	}
	for _, finding := range prepared.Weakened {
		if finding.Path != "b/cmd/one_test.go" {
			t.Fatalf("finding at %q, want every finding at the new path b/cmd/one_test.go", finding.Path)
		}
	}
	message := readLedgerFile(t, root, prepared.Message)
	if trailer := testweakened.ApprovalTrailerLine("cmd.TestOne", reason); !strings.Contains(message, "\n"+trailer+"\n") {
		t.Fatalf("the message does not carry the trailer:\n%s", message)
	}
}

// TestScriptPrunesUsedWeakeningApprovals proves the generated script drops the
// row the commit used, only after the commit, and keeps every other row (AC-8).
//
// VALIDATES: the row is still present after Create and gone after the script;
// a row for another unit survives.
// PREVENTS: an approval consumed by a commit that never landed, and a prune
// that drops the owner's approval of a later weakening.
func TestScriptPrunesUsedWeakeningApprovals(t *testing.T) {
	root := newLedgerRepository(t)
	weakenLedgerTest(t, root, "pkg/one_test.go", "TestOne")
	if _, err := testweakened.Approve(root, sessionOne, "pkg.TestOne", "Thomas accepts the skip"); err != nil {
		t.Fatal(err)
	}
	if _, err := testweakened.Approve(root, sessionOne, "pkg.TestLater", "an approval for a later commit"); err != nil {
		t.Fatal(err)
	}
	approvals := testweakened.ApprovalPath(sessionOne)

	prepared, err := ledgerCreate(t, root, "pkg/one_test.go")
	if err != nil {
		t.Fatal(err)
	}
	if got := readLedgerFile(t, root, approvals); !strings.Contains(got, "| pkg.TestOne |") {
		t.Fatalf("preparing the commit dropped the row before the commit was made:\n%s", got)
	}
	runCommitScript(t, root, prepared.Script)
	after := readLedgerFile(t, root, approvals)
	if strings.Contains(after, "| pkg.TestOne |") {
		t.Fatalf("the used row survived the commit:\n%s", after)
	}
	if !strings.Contains(after, "| pkg.TestLater | an approval for a later commit |") ||
		!strings.Contains(after, "| Test | Reason |") {
		t.Fatalf("the unused row or the header did not survive the commit:\n%s", after)
	}
}

// TestALandedWeakeningApprovalApprovesNothingFurther proves a row whose
// trailer a commit already carries approves no second weakening (AC-9, R-4),
// and does not block a commit that another approval admits.
//
// The method lands one approved weakening, writes the same row back as a
// failed prune would leave it, and prepares a second weakening of the same
// test, then a weakening of another test.
//
// VALIDATES: the second weakening is refused naming the carrier commit; the
// other weakening is admitted, and its script drops the landed row too.
// PREVENTS: one owner approval spent twice.
func TestALandedWeakeningApprovalApprovesNothingFurther(t *testing.T) {
	root := newLedgerRepository(t)
	approvals := testweakened.ApprovalPath(sessionOne)
	weakenLedgerTest(t, root, "pkg/one_test.go", "TestOne")
	if _, err := testweakened.Approve(root, sessionOne, "pkg.TestOne", "the landed reason"); err != nil {
		t.Fatal(err)
	}
	prepared, err := ledgerCreate(t, root, "pkg/one_test.go")
	if err != nil {
		t.Fatal(err)
	}
	runCommitScript(t, root, prepared.Script)
	landed := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "--short", "HEAD"))

	// The prune failed: the file holds the used row again.
	if _, err := testweakened.Approve(root, sessionOne, "pkg.TestOne", "the landed reason"); err != nil {
		t.Fatal(err)
	}
	writeCommitFixture(t, root, "pkg/one_test.go", "package pkg\n\nfunc TestOne(t *testing.T) {\n\tt.Skip(\"later\")\n}\n")
	_, err = ledgerCreate(t, root, "pkg/one_test.go")
	if err == nil || !strings.Contains(err.Error(), "pkg.TestOne") || !strings.Contains(err.Error(), landed) {
		t.Fatalf("Create reusing a landed approval = %v, want it refused naming commit %s", err, landed)
	}

	writeCommitFixture(t, root, "pkg/one_test.go", "package pkg\n\nfunc TestOne(t *testing.T) {\n\tt.Skip(\"later\")\n\trequire.Equal(t, 1, got)\n}\n")
	weakenLedgerTest(t, root, "pkg/two_test.go", "TestTwo")
	if _, err := testweakened.Approve(root, sessionOne, "pkg.TestTwo", "the reason for this commit"); err != nil {
		t.Fatal(err)
	}
	prepared, err = ledgerCreate(t, root, "pkg/two_test.go")
	if err != nil {
		t.Fatalf("Create with a landed row present = %v", err)
	}
	runCommitScript(t, root, prepared.Script)
	if after := readLedgerFile(t, root, approvals); strings.Contains(after, "| pkg.TestOne |") ||
		strings.Contains(after, "| pkg.TestTwo |") {
		t.Fatalf("approval file after the second commit = %q, want the landed and the used rows dropped", after)
	}
}

// TestANoFindingsCommitKeepsASiblingsUnlandedRow proves an approval row that a
// commit does not use survives that commit's script. Two subagents of one
// parent share the parent's approval namespace, so one's commit that weakens
// nothing MUST leave the other's recorded approval in place.
func TestANoFindingsCommitKeepsASiblingsUnlandedRow(t *testing.T) {
	root := newLedgerRepository(t)
	approvals := testweakened.ApprovalPath(sessionOne)
	writeCommitFixture(t, root, approvals, ledgerHeader+
		"| pkg.TestTwo | the sibling's approval, not yet used |\n")

	writeCommitFixture(t, root, "pkg/three_test.go", ledgerTestBody("TestThree"))
	prepared, err := ledgerCreate(t, root, "pkg/three_test.go")
	if err != nil {
		t.Fatalf("Create of a commit that weakens nothing = %v", err)
	}
	runCommitScript(t, root, prepared.Script)
	if after := readLedgerFile(t, root, approvals); !strings.Contains(after, "| pkg.TestTwo |") {
		t.Fatalf("approval file after the commit = %q, want the sibling's unused row kept", after)
	}
}

// newLedgerRepository is a Ze checkout holding two committed tests, which is
// the baseline every weakening below is judged against. It carries its own
// author, because the generated scripts these tests run commit with the
// repository's config, and a CI runner has no global git identity.
func newLedgerRepository(t *testing.T) string {
	t.Helper()
	root := newCommitRepository(t)
	configureCommitAuthor(t, root)
	writeCommitFixture(t, root, "pkg/one_test.go", ledgerTestBody("TestOne"))
	writeCommitFixture(t, root, "pkg/two_test.go", ledgerTestBody("TestTwo"))
	runCommitGit(t, root, "add", "--", "pkg/one_test.go", "pkg/two_test.go")
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t",
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", "tests baseline")
	return root
}

func ledgerTestBody(name string) string {
	return "package pkg\n\nfunc " + name + "(t *testing.T) {\n\trequire.Equal(t, 1, got)\n}\n"
}

// weakenLedgerTest adds a t.Skip, which the detector reports as a blocking
// weakening: the test stops running.
func weakenLedgerTest(t *testing.T, root, path, name string) {
	t.Helper()
	writeCommitFixture(t, root, path, "package pkg\n\nfunc "+name+
		"(t *testing.T) {\n\tt.Skip(\"later\")\n\trequire.Equal(t, 1, got)\n}\n")
}

// ledgerCreate prepares one commit for sessionOne, the session every approval
// these tests expect to admit is recorded under, over the named paths. The
// overrides name the gates a throwaway repository cannot satisfy and that this
// file does not judge.
func ledgerCreate(t *testing.T, root string, paths ...string) (Prepared, error) {
	t.Helper()
	return Create(root, &Options{
		Session: sessionOne,
		Subject: "ledger fixture for session " + sessionOne,
		Files:   paths,
		NoTest:  "the fixture carries tests only",
	})
}

func readLedgerFile(t *testing.T, root, path string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
