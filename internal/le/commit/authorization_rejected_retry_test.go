// Design: docs/contributing/committing.md -- a retry must not turn rejected input into an authorization.
package commit

import (
	"errors"
	"maps"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/le/human/humantest"
)

// TestRejectedAttestationRetryPreservesIncompatibleOperands reaches the real
// registered entry, captures its nonterminal refusal and replays the printed
// command under a PTY. Every incompatible input must remain incompatible, never
// become an empty attestation after the renderer silently drops that operand.
func TestRejectedAttestationRetryPreservesIncompatibleOperands(t *testing.T) {
	for _, operand := range [][]string{
		{keywordFile, "a.txt"}, {keywordRemove, "a.txt"}, {keywordHunks, "a.txt", "unread.patch"},
		{keywordNoTest, "fixture reason"}, {keywordReviewOverride, "fixture reason"},
		{keywordRFCChangeOK, "fixture reason"}, {keywordLedgerCarried, "fixture row"},
	} {
		t.Run(operand[0], func(t *testing.T) {
			humantest.Streams(t, "piped-both")
			root := newOwedRepository(t)
			t.Setenv("ZE_REPO_ROOT", root)
			t.Setenv("CLAUDE_CODE_SESSION_ID", "rejected-attest-retry-origin")
			t.Setenv("ZE_AGENT_ID", "origin-agent")
			env.ResetCache()
			t.Cleanup(env.ResetCache)
			owed := commitOwed(t, root, "the change", map[string]string{"a.txt": "a\n"}, nil, "Review-owed: demo: no reviewer")
			arguments := append([]string{"create", "subject", "owner attests", "attest", owed, "Thomas reviewed the change"}, operand...)
			t.Run("original-terminal", func(t *testing.T) {
				humantest.Streams(t, "terminal")
				output, code := captureAttestationAnswer(t, arguments)
				if code != 2 || !strings.Contains(output, "attest refused with") {
					t.Fatalf("original incompatible operation: exit %d, %s", code, output)
				}
			})
			before := humantest.Snapshot(t, root)
			output, code := captureAttestationAnswer(t, arguments)
			if code != 2 {
				t.Fatalf("nonterminal original exit = %d, want 2", code)
			}
			if !maps.Equal(before, humantest.Snapshot(t, root)) {
				t.Fatal("refused nonterminal original changed the checkout")
			}
			retry := humantest.RetryArguments(t, errors.New(strings.TrimSpace(output)))
			t.Run("printed-terminal-retry", func(t *testing.T) {
				humantest.Streams(t, "terminal")
				output, code := captureAttestationAnswer(t, retry[2:])
				if code != 2 || !strings.Contains(output, "attest refused with") {
					t.Errorf("retry changed a rejected operation into an attestation: exit %d, %s", code, output)
				}
				if !maps.Equal(before, humantest.Snapshot(t, root)) {
					t.Error("retry of incompatible input wrote authorization artifacts")
				}
			})
		})
	}
}

func captureAttestationAnswer(t *testing.T, arguments []string) (string, int) {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	stderr := os.Stderr
	os.Stderr = file
	defer func() {
		os.Stderr = stderr
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	}()
	_, code := Answer(arguments)
	output, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(output), code
}
