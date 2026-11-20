// VALIDATES: `le spec design judge` writes a digest-bound record for a judge
// that read every row's section, refuses a process with no agent identity and
// a malformed reason, and writes nothing when it refuses
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-15, AC-16).
// PREVENTS: a verdict recorded by the main thread, which is the author's, or a
// verdict that outlives the text it judged.
package specdesign_test

import (
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

// useJudge selects agent as the identity of this le process.
func useJudge(t *testing.T, agent string) {
	t.Helper()
	t.Setenv("CLAUDE_CODE_SESSION_ID", judgeSession)
	t.Setenv("ZE_AGENT_ID", agent)
	env.ResetCache()
}

// readAsJudge records, through the writer the Read hook uses, agent's whole
// read of the widget page and of the system door.
func readAsJudge(t *testing.T, root, agent string) {
	t.Helper()
	fingerprint := judgeSession + "+" + agent
	sessionDir := readrecord.SessionDir(root)
	if err := os.MkdirAll(sessionDir, 0o750); err != nil {
		t.Fatal(err)
	}
	lines := strings.Count(widgetPageText, "\n")
	if err := readrecord.RecordRead(readrecord.SectionMarker(sessionDir, widgetPage, fingerprint), widgetPageText, 1, lines); err != nil {
		t.Fatalf("record the page read: %v", err)
	}
	if err := os.WriteFile(readrecord.WholeReadMarker(sessionDir, widgetDoor, fingerprint), nil, 0o600); err != nil {
		t.Fatal(err)
	}
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
