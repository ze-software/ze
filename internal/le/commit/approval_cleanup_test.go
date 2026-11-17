// Design: docs/contributing/committing.md -- exact approval cleanup follows publication
// Related: review_publication_runtime_test.go -- actual registered Answer subprocess
package commit

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
	testweakened "github.com/ze-software/ze/internal/le/test/weakened"
)

// Goal: the real native cleanup consumes only its captured kind and namespace,
// even when its runtime session belongs to another author. Method: both ledger
// kinds hold actual rows in two namespaces; the existing native subprocess
// bridge dispatches the advertised command, then every surviving row is read.
func TestApprovalCleanupNativeRouteBindsCapturedNamespace(t *testing.T) {
	for _, kind := range []struct {
		name    string
		path    func(string) string
		approve func(string, string, string, string) error
	}{
		{name: approvalRFCName, path: rfc.ApprovalPath, approve: func(root, session, unit, reason string) error {
			_, err := rfc.Approve(root, session, unit, reason)
			return err
		}},
		{name: approvalWeakeningName, path: testweakened.ApprovalPath, approve: func(root, session, unit, reason string) error {
			_, err := testweakened.Approve(root, session, unit, reason)
			return err
		}},
	} {
		t.Run(kind.name, func(t *testing.T) {
			root := newLedgerRepository(t)
			if err := kind.approve(root, sessionOne, "pkg.TestOne", "captured consumed verdict"); err != nil {
				t.Fatal(err)
			}
			if err := kind.approve(root, sessionOne, "pkg.TestTwo", "unrelated original verdict"); err != nil {
				t.Fatal(err)
			}
			if err := kind.approve(root, sessionTwo, "pkg.TestOne", "runtime author's untouched verdict"); err != nil {
				t.Fatal(err)
			}
			pending := readLedgerFile(t, root, kind.path(sessionOne))
			captured := approvalCleanupFixtureRow(t, pending, "pkg.TestOne")
			runtimeBefore := readLedgerFile(t, root, kind.path(sessionTwo))
			head := headOf(t, root)
			t.Setenv("CLAUDE_CODE_SESSION_ID", "different-runtime-approval-cleanup-author")
			command := exec.CommandContext(t.Context(), filepath.Join(root, "le"),
				"commit", "approval", "clean", "kind", kind.name,
				"session", sessionOne, "row", captured)
			command.Dir = root
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("real native cleanup failed: %v\n%s", err, output)
			}
			after := readLedgerFile(t, root, kind.path(sessionOne))
			if strings.Contains(after, "| pkg.TestOne |") {
				t.Fatalf("captured consumed row survived native cleanup:\n%s", after)
			}
			if !strings.Contains(after, "| pkg.TestTwo | unrelated original verdict |") {
				t.Fatalf("native cleanup lost the unrelated original row:\n%s", after)
			}
			if runtimeAfter := readLedgerFile(t, root, kind.path(sessionTwo)); runtimeAfter != runtimeBefore {
				t.Fatalf("native cleanup changed the runtime author's ledger:\nbefore:\n%s\nafter:\n%s", runtimeBefore, runtimeAfter)
			}
			if afterHead := headOf(t, root); afterHead != head {
				t.Fatalf("native row cleanup moved HEAD: %s -> %s", head, afterHead)
			}
		})
	}
}

// Goal: actual generated publication carries its captured verdict while cleanup
// preserves a newer verdict for that same unit. Method: renew after Create,
// run the real script under a different runtime author, then inspect Git and
// both namespaces. Existing used-row tests remain the exact-consumption control.
func TestGeneratedApprovalCleanupKeepsRenewedSameUnitVerdict(t *testing.T) {
	root := newLedgerRepository(t)
	weakenLedgerTest(t, root, "pkg/one_test.go", "TestOne")
	const capturedReason = "judge approved the captured original change"
	const freshReason = "judge independently renewed the same unit"
	if _, err := testweakened.Approve(root, sessionOne, "pkg.TestOne", capturedReason); err != nil {
		t.Fatal(err)
	}
	if _, err := testweakened.Approve(root, sessionOne, "pkg.TestTwo", "unrelated original verdict"); err != nil {
		t.Fatal(err)
	}
	prepared, err := ledgerCreate(t, root, "pkg/one_test.go")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testweakened.Approve(root, sessionOne, "pkg.TestOne", freshReason); err != nil {
		t.Fatal(err)
	}
	if _, err := testweakened.Approve(root, sessionTwo, "pkg.TestOne", "runtime author's untouched verdict"); err != nil {
		t.Fatal(err)
	}
	runtimeBefore := readLedgerFile(t, root, testweakened.ApprovalPath(sessionTwo))
	t.Setenv("CLAUDE_CODE_SESSION_ID", "different-runtime-generated-cleanup-author")
	runCommitScript(t, root, prepared.Script)
	message := runCommitGitOutput(t, root, "log", "-1", "--format=%B")
	if !strings.Contains(message, "\n"+testweakened.ApprovalTrailerLine("pkg.TestOne", capturedReason)+"\n") {
		t.Fatalf("actual publication lost its captured verdict trailer:\n%s", message)
	}
	if strings.Contains(message, testweakened.ApprovalTrailerLine("pkg.TestOne", freshReason)) {
		t.Fatalf("actual publication substituted the newer pending verdict:\n%s", message)
	}
	after := readLedgerFile(t, root, testweakened.ApprovalPath(sessionOne))
	if !strings.Contains(after, "| pkg.TestOne | "+freshReason+" |") {
		t.Fatalf("post-publication cleanup erased the renewed same-unit verdict:\n%s", after)
	}
	if !strings.Contains(after, "| pkg.TestTwo | unrelated original verdict |") {
		t.Fatalf("post-publication cleanup erased the unrelated verdict:\n%s", after)
	}
	if runtimeAfter := readLedgerFile(t, root, testweakened.ApprovalPath(sessionTwo)); runtimeAfter != runtimeBefore {
		t.Fatalf("generated cleanup changed the runtime author's ledger:\nbefore:\n%s\nafter:\n%s", runtimeBefore, runtimeAfter)
	}
}

// Goal: the first publication introducing native cleanup can run that cleanup
// without waiting for a shared HEAD launcher to rebuild. Method: record the
// actual rendered command's argv; a stale shared-helper control refuses ONLY
// unnamed cleanup, while the selected named route dispatches the real Answer.
// Removing the explicit named selector makes this unit fail after publication
// at the stale-helper sentinel, leaving the exact used row unconsumed.
func TestGeneratedApprovalCleanupUsesNamedLauncher(t *testing.T) {
	root := newLedgerRepository(t)
	weakenLedgerTest(t, root, "pkg/one_test.go", "TestOne")
	if _, err := testweakened.Approve(root, sessionOne, "pkg.TestOne", "actual named cleanup consumed verdict"); err != nil {
		t.Fatal(err)
	}
	if _, err := testweakened.Approve(root, sessionOne, "pkg.TestTwo", "actual named cleanup unrelated verdict"); err != nil {
		t.Fatal(err)
	}
	bridge := filepath.Join(root, "native-approval-dispatch")
	if err := os.Rename(filepath.Join(root, "le"), bridge); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(t.TempDir(), "actual-cleanup-argv")
	stale := filepath.Join(t.TempDir(), "stale-unnamed-cleanup-reached")
	guard := "#!/bin/bash\nprintf '%s\\0' \"$@\" >> " + shellQuote(record) + "\nprintf '\\n' >> " + shellQuote(record) + "\n" +
		"if [ \"${1:-}\" = commit ] && [ \"${2:-}\" = approval ] && [ \"${3:-}\" = clean ]; then\n" +
		"  : > " + shellQuote(stale) + "\n  exit 97\nfi\n" +
		"exec " + shellQuote(bridge) + " \"$@\"\n"
	writeCommitFixture(t, root, "le", guard)
	if err := os.Chmod(filepath.Join(root, "le"), 0o700); err != nil { //nolint:gosec // owned executable test bridge
		t.Fatal(err)
	}
	prepared, err := ledgerCreate(t, root, "pkg/one_test.go")
	if err != nil {
		t.Fatal(err)
	}
	beforeHead := headOf(t, root)
	output, runErr := runScript(t, root, prepared.Script)
	if _, statErr := os.Stat(stale); statErr == nil {
		if headOf(t, root) == beforeHead {
			t.Fatalf("stale-cleanup control fired before actual publication:\n%s", output)
		}
		pending := readLedgerFile(t, root, testweakened.ApprovalPath(sessionOne))
		if !strings.Contains(pending, "| pkg.TestOne | actual named cleanup consumed verdict |") {
			t.Fatalf("stale-cleanup control unexpectedly consumed the captured row:\n%s", pending)
		}
		if !strings.Contains(pending, "| pkg.TestTwo | actual named cleanup unrelated verdict |") {
			t.Fatalf("stale-cleanup control lost the unrelated row:\n%s", pending)
		}
		t.Fatalf("generated cleanup reached the stale unnamed helper after publication; used rows remain pending: %v\n%s", runErr, output)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal(statErr)
	}
	if runErr != nil {
		t.Fatalf("named generated cleanup did not finish actual publication: %v\n%s", runErr, output)
	}
	if headOf(t, root) == beforeHead {
		t.Fatal("named cleanup control never published its actual commit")
	}
	actual, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{"--name", "commit-approval-" + sessionOne,
		"commit", "approval", "clean", "kind", "weakening", "session", sessionOne}, "\x00") + "\x00"
	if count := strings.Count(string(actual), want); count != 1 {
		t.Fatalf("actual cleanup argv held the captured named selector %d times, want exactly once: %q", count, actual)
	}
	after := readLedgerFile(t, root, testweakened.ApprovalPath(sessionOne))
	if strings.Contains(after, "| pkg.TestOne |") {
		t.Fatalf("actual named native cleanup retained its consumed row:\n%s", after)
	}
	if !strings.Contains(after, "| pkg.TestTwo | actual named cleanup unrelated verdict |") {
		t.Fatalf("actual named native cleanup lost the unrelated row:\n%s", after)
	}
}

func approvalCleanupFixtureRow(t *testing.T, content, unit string) string {
	t.Helper()
	var found string
	for line := range strings.SplitSeq(content, "\n") {
		if !strings.HasPrefix(line, "| "+unit+" |") {
			continue
		}
		if found != "" {
			t.Fatalf("native fixture contains duplicate rows for %s", unit)
		}
		found = line
	}
	if found == "" {
		t.Fatalf("native fixture contains no actual row for %s", unit)
	}
	return found
}
