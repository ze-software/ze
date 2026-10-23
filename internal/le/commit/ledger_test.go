// Design: docs/architecture/testing/test-health.md -- one ledger shard per commit session
package commit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	testweakened "github.com/ze-software/ze/internal/le/test/weakened"
)

// The two authors these tests drive. They are ordinary commit sessions: the
// same eight hex characters that name each one's script and message.
const (
	sessionOne = "aaaaaaaa"
	sessionTwo = "bbbbbbbb"
)

const ledgerHeader = "# Test weakenings this commit accepts\n\n| Test | Reason |\n|------|--------|\n"

// TestTwoSessionsKeepTheirOwnLedgerRowsThroughCreate is the whole defect, run
// as it happened: two authors preparing a commit from one checkout at the same
// time, each writing the rows their own weakening owes.
//
// It is driven through Create rather than through a path helper on purpose.
// The failure was that two writers resolved to ONE file, so a test over either
// writer alone cannot see it: only the second author's write, landing while
// the first still needs its rows, shows whether the rows survived.
//
// The last assertion is the one with no other tell. A commit that names
// another session's shard publishes that session's justification under its own
// subject, and both the file and the row count look right afterwards, so
// nothing downstream can find it.
func TestTwoSessionsKeepTheirOwnLedgerRowsThroughCreate(t *testing.T) {
	root := newLedgerRepository(t)
	shardOne := testweakened.ShardPath(testweakened.WeakenedDir, sessionOne)
	shardTwo := testweakened.ShardPath(testweakened.WeakenedDir, sessionTwo)

	weakenLedgerTest(t, root, "pkg/one_test.go", "TestOne")
	writeCommitFixture(t, root, shardOne,
		ledgerHeader+"| TestOne | session one accepts the skip |\n")
	if _, err := ledgerCreate(t, root, sessionOne, "pkg/one_test.go", shardOne); err != nil {
		t.Fatalf("session one refused: %v", err)
	}

	// Session two arrives while session one still holds its rows, and writes
	// its own rows exactly as its own gate tells it to.
	weakenLedgerTest(t, root, "pkg/two_test.go", "TestTwo")
	writeCommitFixture(t, root, shardTwo,
		ledgerHeader+"| TestTwo | session two accepts the skip |\n")
	if _, err := ledgerCreate(t, root, sessionTwo, "pkg/two_test.go", shardTwo); err != nil {
		t.Fatalf("session two refused: %v", err)
	}

	if got := readLedgerFile(t, root, shardOne); !strings.Contains(got, "| TestOne |") ||
		strings.Contains(got, "| TestTwo |") {
		t.Fatalf("session one's shard after session two wrote = %q", got)
	}
	if got := readLedgerFile(t, root, shardTwo); !strings.Contains(got, "| TestTwo |") ||
		strings.Contains(got, "| TestOne |") {
		t.Fatalf("session two's shard = %q", got)
	}

	// Session one prepared again, now naming session two's shard. Nothing
	// about the rows is wrong; the writer is.
	_, err := ledgerCreate(t, root, sessionOne, "pkg/one_test.go", shardOne, shardTwo)
	if err == nil || !strings.Contains(err.Error(), "is session bbbbbbbb's ledger shard") {
		t.Fatalf("Create carrying another session's shard = %v", err)
	}
}

// TestCreateStillRefusesAWeakeningItsOwnShardDoesNotName is the other
// polarity. The namespace must not have bought isolation by letting a
// weakening through: the gate that refuses an unjustified weakening is
// unchanged, and a row in ANOTHER session's shard buys nothing at all.
func TestCreateStillRefusesAWeakeningItsOwnShardDoesNotName(t *testing.T) {
	root := newLedgerRepository(t)
	shardOne := testweakened.ShardPath(testweakened.WeakenedDir, sessionOne)
	shardTwo := testweakened.ShardPath(testweakened.WeakenedDir, sessionTwo)

	weakenLedgerTest(t, root, "pkg/one_test.go", "TestOne")

	// No shard at all: refused, and the refusal names the shard to write.
	_, err := ledgerCreate(t, root, sessionOne, "pkg/one_test.go")
	if err == nil || !strings.Contains(err.Error(), shardOne) {
		t.Fatalf("Create with no shard = %v, want the refusal naming %s", err, shardOne)
	}

	// A peer holding the row that would justify it: still refused, because a
	// shard records what its own author justified.
	writeCommitFixture(t, root, shardTwo,
		ledgerHeader+"| TestOne | a peer's justification for my weakening |\n")
	_, err = ledgerCreate(t, root, sessionOne, "pkg/one_test.go")
	if err == nil || !strings.Contains(err.Error(), "has no row for it") {
		t.Fatalf("Create reading a peer's row = %v, want the weakening still refused", err)
	}

	// The author's own row, in the author's own shard, still opens it.
	writeCommitFixture(t, root, shardOne,
		ledgerHeader+"| TestOne | session one accepts the skip |\n")
	if _, err := ledgerCreate(t, root, sessionOne, "pkg/one_test.go", shardOne); err != nil {
		t.Fatalf("Create with the author's own row = %v", err)
	}
}

// TestALandedRowIsDroppedRatherThanBlockingTheNextCommit pins the rule that
// replaces "delete the rows of the last commit". A row whose commit is at HEAD
// explains a diff git already holds: it accepts nothing further, so the gate
// drops it from the shard instead of refusing the next commit for holding it.
//
// Reaching into a ledger to clear a landed row cost three sessions a hand
// repair each (plan/journal/concurrent-session-corruption.md), and each repair
// verified the same thing this function verifies from the git objects.
func TestALandedRowIsDroppedRatherThanBlockingTheNextCommit(t *testing.T) {
	root := newLedgerRepository(t)
	shard := testweakened.ShardPath(testweakened.WeakenedDir, sessionOne)

	weakenLedgerTest(t, root, "pkg/one_test.go", "TestOne")
	writeCommitFixture(t, root, shard, ledgerHeader+"| TestOne | the landed reason |\n")
	runCommitGit(t, root, "add", "--", "pkg/one_test.go", shard)
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t",
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", "session one's first commit")

	// The next commit of the SAME session, weakening a different test. Its
	// shard still holds the landed row, which is exactly the state that
	// refused a commit three times.
	weakenLedgerTest(t, root, "pkg/two_test.go", "TestTwo")
	writeCommitFixture(t, root, shard, ledgerHeader+
		"| TestOne | the landed reason |\n"+
		"| TestTwo | the reason for this commit |\n")
	if _, err := ledgerCreate(t, root, sessionOne, "pkg/two_test.go", shard); err != nil {
		t.Fatalf("Create with a landed row present = %v", err)
	}
	after := readLedgerFile(t, root, shard)
	if strings.Contains(after, "| TestOne |") {
		t.Fatalf("shard after Create = %q, want the landed row dropped by the gate", after)
	}
	if !strings.Contains(after, "| TestTwo |") || !strings.Contains(after, "| Test | Reason |") {
		t.Fatalf("shard after Create = %q, want this commit's row under the header", after)
	}

	// The discrimination: an UNLANDED row that explains nothing is still a
	// refusal. Only the proof that git holds it makes a row harmless.
	writeCommitFixture(t, root, shard, ledgerHeader+
		"| TestGone | a row for a test this commit does not weaken |\n"+
		"| TestTwo | the reason for this commit |\n")
	_, err := ledgerCreate(t, root, sessionOne, "pkg/two_test.go", shard)
	if err == nil || !strings.Contains(err.Error(), "TestGone") {
		t.Fatalf("Create with an unlanded stale row = %v, want it refused", err)
	}
}

// TestANoFindingsCommitKeepsASiblingsUnlandedRow is evidence commit
// 8bd1742581, run through Create. Two subagents of one parent share one commit
// session, so they share one shard. One of them commits a change that weakens
// nothing and names the shard only to drop its landed row. The other's row is
// in the working tree, not at HEAD, and it MUST survive: the gate proves a row
// landed before it removes it, and an unlanded row has no such proof.
func TestANoFindingsCommitKeepsASiblingsUnlandedRow(t *testing.T) {
	root := newLedgerRepository(t)
	shard := testweakened.ShardPath(testweakened.WeakenedDir, sessionOne)

	weakenLedgerTest(t, root, "pkg/one_test.go", "TestOne")
	writeCommitFixture(t, root, shard, ledgerHeader+"| TestOne | the landed reason |\n")
	runCommitGit(t, root, "add", "--", "pkg/one_test.go", shard)
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t",
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", "session one's first commit")

	writeCommitFixture(t, root, shard, ledgerHeader+
		"| TestOne | the landed reason |\n"+
		"| TestTwo | the sibling's reason, not yet committed |\n")
	writeCommitFixture(t, root, "pkg/three_test.go", ledgerTestBody("TestThree"))
	if _, err := ledgerCreate(t, root, sessionOne, "pkg/three_test.go", shard); err != nil {
		t.Fatalf("Create of a commit that weakens nothing = %v", err)
	}
	after := readLedgerFile(t, root, shard)
	if strings.Contains(after, "| TestOne |") {
		t.Fatalf("shard after Create = %q, want the landed row dropped", after)
	}
	if !strings.Contains(after, "| TestTwo |") {
		t.Fatalf("shard after Create = %q, want the sibling's unlanded row kept", after)
	}
}

// TestTheParentCommitsItsSubagentsShardAndNoOneElses drives the main thread
// committing work a subagent did. Each subagent owns its commit namespace, so
// its justification row is in a shard named for it rather than for the
// parent. The parent's commit carries that shard and is accepted, the
// weakening is judged against the child's row, and the child's row is kept in
// the commit. A shard of an unrelated session's subagent stays foreign.
func TestTheParentCommitsItsSubagentsShardAndNoOneElses(t *testing.T) {
	root := newLedgerRepository(t)
	namespaceAs := func(session, agent string) string {
		t.Helper()
		t.Setenv("CLAUDE_CODE_SESSION_ID", session)
		t.Setenv("ZE_AGENT_ID", agent)
		env.ResetCache()
		namespace, err := lepath.CommitSession(root, "")
		if err != nil {
			t.Fatalf("CommitSession(%s, %s) = %v", session, agent, err)
		}
		return namespace
	}
	t.Cleanup(env.ResetCache)
	child := namespaceAs("parent-sid", "agent-one")
	sibling := namespaceAs("parent-sid", "agent-two")
	foreign := namespaceAs("other-sid", "agent-one")
	parent := namespaceAs("parent-sid", "")
	if child == sibling {
		t.Fatalf("two sibling agents share commit namespace %s", child)
	}
	childShard := testweakened.ShardPath(testweakened.WeakenedDir, child)
	foreignShard := testweakened.ShardPath(testweakened.WeakenedDir, foreign)

	weakenLedgerTest(t, root, "pkg/one_test.go", "TestOne")
	writeCommitFixture(t, root, childShard, ledgerHeader+"| TestOne | the subagent accepts the skip |\n")
	writeCommitFixture(t, root, foreignShard, ledgerHeader+"| TestOne | an unrelated agent's reason |\n")

	_, err := ledgerCreate(t, root, parent, "pkg/one_test.go", foreignShard)
	if err == nil || !strings.Contains(err.Error(), "is session "+foreign+"'s ledger shard") {
		t.Fatalf("parent carrying an unrelated session's agent shard = %v", err)
	}
	if _, err := ledgerCreate(t, root, parent, "pkg/one_test.go", childShard); err != nil {
		t.Fatalf("parent carrying its own subagent's shard = %v", err)
	}
	if got := readLedgerFile(t, root, childShard); !strings.Contains(got, "| TestOne |") {
		t.Fatalf("child shard after the parent's Create = %q, want its used row kept", got)
	}

	namespaceAs("parent-sid", "agent-two")
	_, err = ledgerCreate(t, root, sibling, "pkg/one_test.go", childShard)
	if err == nil || !strings.Contains(err.Error(), "is session "+child+"'s ledger shard") {
		t.Fatalf("a sibling carrying another sibling's shard = %v", err)
	}
}

// newLedgerRepository is a Ze checkout holding two committed tests, which is
// the baseline every weakening below is judged against.
func newLedgerRepository(t *testing.T) string {
	t.Helper()
	root := newCommitRepository(t)
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

// ledgerCreate prepares one commit for one session over the named paths. The
// overrides name the gates a throwaway repository cannot satisfy and that this
// file does not judge.
func ledgerCreate(t *testing.T, root, session string, paths ...string) (Prepared, error) {
	t.Helper()
	return Create(root, &Options{
		Session: session,
		Subject: "ledger fixture for session " + session,
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
