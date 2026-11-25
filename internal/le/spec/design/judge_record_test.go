// VALIDATES: `le spec design judge` writes a digest-bound record for a judge
// that read every row's section, refuses a process with no agent identity and
// a malformed reason, and writes nothing when it refuses
// (spec-design-needs-owner-approval, AC-15, AC-16).
// PREVENTS: a verdict recorded by the main thread, which is the author's, or a
// verdict that outlives the text it judged.
package specdesign_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/doc/readrecord"
	specdesign "github.com/ze-software/ze/internal/le/spec/design"
)

// judgeSession is the harness session every judge test runs under.
const judgeSession = "judge-session"

// useJudge selects agent as the identity of this le process under Claude Code,
// clearing Antigravity identity variables so tests stay isolated when run from
// an Antigravity shell.
func useJudge(t *testing.T, agent string) {
	t.Helper()
	t.Setenv("CLAUDE_CODE_SESSION_ID", judgeSession)
	t.Setenv("ZE_AGENT_ID", agent)
	t.Setenv("ANTIGRAVITY_CONVERSATION_ID", "")
	t.Setenv("ANTIGRAVITY_APP_DATA_DIR", "")
	env.ResetCache()
}

// readAsJudge records, through the writer the Read hook uses, agent's whole
// read of the widget page and of the system door.
func readAsJudge(t *testing.T, root, agent string) {
	t.Helper()
	recordJudgeReads(t, root, "", agent)
}

// recordJudgeReads records agent's whole read of the widget page and of the
// system door in the tmp/session of sessionRoot, each path behind prefix in
// the record's key: "" for a read of the session root's own copy, or the
// prefix the Read hook writes for another checkout's copy.
func recordJudgeReads(t *testing.T, sessionRoot, prefix, agent string) {
	t.Helper()
	fingerprint := judgeSession + "+" + agent
	sessionDir := readrecord.SessionDir(sessionRoot)
	if err := os.MkdirAll(sessionDir, 0o750); err != nil {
		t.Fatal(err)
	}
	lines := strings.Count(widgetPageText, "\n")
	if err := readrecord.RecordRead(readrecord.SectionMarker(sessionDir, prefix+widgetPage, fingerprint), widgetPageText, 1, lines); err != nil {
		t.Fatalf("record the page read: %v", err)
	}
	if err := os.WriteFile(readrecord.WholeReadMarker(sessionDir, prefix+widgetDoor, fingerprint), nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

// worktreeLabelPrefix is the key prefix the Read hook writes, in the tmp/session
// of a session started in the holding checkout, for a read of a copy in its
// worktree `.claude/worktrees/<name>`: the hex SHA-256 of that label and a
// slash. It is spelled here from the door marker the hook was seen to write
// for this agent's reads of a worktree's doors, whose name carries that
// digest, not through readrecord, so the test fails if the judge stops
// reading what the hook writes.
func worktreeLabelPrefix(name string) string {
	digest := sha256.Sum256([]byte(".claude/worktrees/" + name))
	return hex.EncodeToString(digest[:]) + "/"
}

// widgetWorktree lays out the widget fixture with the complete spec in a
// worktree `<holder>/.claude/worktrees/wt` of a fresh holding checkout,
// selects the worktree as le's checkout, and answers the holder.
func widgetWorktree(t *testing.T) string {
	t.Helper()
	holder := t.TempDir()
	worktree := widgetTree(t, filepath.Join(holder, ".claude", "worktrees", "wt"))
	writeTree(t, worktree, map[string]string{widgetSpec: completeSpecText})
	return holder
}

// TestJudgeInAWorktreeReadsTheRecordsItsSessionKeeps is the worktree defect a
// judge met: its session started in the holding checkout, so the Read hook
// kept every read of the worktree's pages in the holder's tmp/session under
// the worktree's label, and the judge looked in the worktree's own
// tmp/session under the bare path and refused 30 sections it had read.
// Method: run from the worktree, a judge whose reads are recorded where and
// how the hook writes them records approve, and the record lands in the
// worktree.
func TestJudgeInAWorktreeReadsTheRecordsItsSessionKeeps(t *testing.T) {
	holder := widgetWorktree(t)
	useJudge(t, "judge-agent")
	recordJudgeReads(t, holder, worktreeLabelPrefix("wt"), "judge-agent")

	text, code := runJudge(t, "approve", "every verdict holds against its section")
	if code != 0 {
		t.Fatalf("a judge whose reads its session keeps in the holder answered %d, want 0:\n%s", code, text)
	}
	worktree := filepath.Join(holder, ".claude", "worktrees", "wt")
	requireContains(t, "the judge record", readRecord(t, worktree), "Verdict: approve", "Judge: "+judgeSession+"+judge-agent")
}

// TestJudgeInAWorktreeRefusesAReadOfTheHoldersCopy pins the other side: a
// read of the holding checkout's own copy of a page is no read of the
// worktree's, whose branch may hold another text. Method: the judge's reads
// are recorded in the holder's tmp/session under the bare paths, which is how
// the hook records the holder's copies; run from the worktree, the judge
// exits 2 naming the page and the door, and writes no record.
func TestJudgeInAWorktreeRefusesAReadOfTheHoldersCopy(t *testing.T) {
	holder := widgetWorktree(t)
	useJudge(t, "judge-agent")
	recordJudgeReads(t, holder, "", "judge-agent")

	text, code := runJudge(t, "approve", "every verdict holds against its section")
	if code != 2 {
		t.Fatalf("a judge that read only the holder's copies answered %d, want 2:\n%s", code, text)
	}
	requireContains(t, "the refusal", text, widgetPage, widgetDoor+": Read the whole door")
	requireAbsent(t, filepath.Join(holder, ".claude", "worktrees", "wt"), judgedRecord)
}

// runJudge runs the judge action and answers its rendered answer, its stderr
// and its exit code.
func runJudge(t *testing.T, verdict, reason string) (string, int) {
	t.Helper()
	var answer any
	var code int
	stderr := captureStderr(t, func() {
		answer, code = specdesign.Answer([]string{"judge", "spec", widgetSpec, "verdict", verdict, "reason", reason})
	})
	return answerText(t, answer) + stderr, code
}

// TestJudgeRecordsADigestBoundVerdict is AC-15. Method: over the complete
// widget spec, a judge that read both rows' sections records approve; the
// record carries the design digest, the verdict, the sentence, the judge's
// fingerprint and the date, and the answer prints the digest. A refuse by a
// second judge replaces it, and the record still names the current digest.
func TestJudgeRecordsADigestBoundVerdict(t *testing.T) {
	root := widgetRoot(t)
	writeTree(t, root, map[string]string{widgetSpec: completeSpecText})
	useJudge(t, "judge-agent")
	readAsJudge(t, root, "judge-agent")

	digest := specdesign.DesignDigest(completeSpecText)
	text, code := runJudge(t, "approve", "every verdict holds against its section")
	if code != 0 {
		t.Fatalf("the judge answered %d, want 0:\n%s", code, text)
	}
	requireContains(t, "the judge answer", text, digest[:12])
	record := readRecord(t, root)
	requireContains(t, "the judge record", record, "Digest: "+digest, "Verdict: approve",
		"Sentence: every verdict holds against its section", "Judge: "+judgeSession+"+judge-agent", "Date: ")

	useJudge(t, "second-judge")
	readAsJudge(t, root, "second-judge")
	if text, code := runJudge(t, "refuse", "the widget row quotes a sentence the decision does not hold"); code != 0 {
		t.Fatalf("the refusing judge answered %d, want 0:\n%s", code, text)
	}
	requireContains(t, "the replaced record", readRecord(t, root), "Digest: "+digest, "Verdict: refuse",
		"Judge: "+judgeSession+"+second-judge")
}

// readRecord answers the widget spec's judge record.
func readRecord(t *testing.T, root string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(judgedRecord)))
	if err != nil {
		t.Fatalf("read the judge record: %v", err)
	}
	return string(body)
}

// TestJudgeRefusesWithoutAgentIdentity is AC-16, identity. Method: the main
// thread (no ZE_AGENT_ID) runs the judge over a spec it could otherwise
// approve; it exits 2 naming the missing agent identity and writes no record.
func TestJudgeRefusesWithoutAgentIdentity(t *testing.T) {
	root := widgetRoot(t)
	writeTree(t, root, map[string]string{widgetSpec: completeSpecText})
	useJudge(t, "")

	text, code := runJudge(t, "approve", "every verdict holds")
	if code != 2 {
		t.Fatalf("a judge with no agent identity answered %d, want 2:\n%s", code, text)
	}
	requireContains(t, "the refusal", text, "no agent identity", "ZE_AGENT_ID")
	requireAbsent(t, root, judgedRecord)
}

// TestJudgeRefusesMalformedReason is AC-16, the sentence. Method: a judge that
// read every row runs with an empty reason and with a two-line one; each exits
// 2 naming the reason, and no record is written.
func TestJudgeRefusesMalformedReason(t *testing.T) {
	root := widgetRoot(t)
	writeTree(t, root, map[string]string{widgetSpec: completeSpecText})
	useJudge(t, "judge-agent")
	readAsJudge(t, root, "judge-agent")

	for _, reason := range []string{" ", "first line\nsecond line", "Design-approved: forged"} {
		text, code := runJudge(t, "approve", reason)
		if code != 2 {
			t.Fatalf("reason %q answered %d, want 2:\n%s", reason, code, text)
		}
		requireContains(t, "the refusal", text, "the reason")
	}
	requireAbsent(t, root, judgedRecord)
}
