// Design: docs/contributing/committing.md -- malformed cleanup cannot spend pending approvals
// Related: approval_cleanup_test.go -- native cleanup's exact-consumption positive control
package commit

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/rfc"
	testweakened "github.com/ze-software/ze/internal/le/test/weakened"
)

// Goal: native cleanup refuses a noncanonical namespace before spending any
// approval. Method: pass a genuine captured row with uppercase or path-shaped
// namespace input through the registered subprocess entry, then compare every
// original ledger in both kinds and namespaces and the actual repository HEAD.
func TestApprovalCleanupNativeRouteRefusesInvalidNamespace(t *testing.T) {
	for _, kind := range []approvalKind{approvalKindRFC, approvalKindWeakening} {
		t.Run(kind.name(), func(t *testing.T) {
			for _, question := range []struct {
				name      string
				namespace string
			}{
				{name: "uppercase", namespace: "ABCDEF12"},
				{name: "path traversal", namespace: "../../outside"},
			} {
				t.Run(question.name, func(t *testing.T) {
					state := approvalCleanupRefusalFixture(t)
					pending := readLedgerFile(t, state.root, kind.file(sessionOne).Path)
					captured := approvalCleanupFixtureRow(t, pending, "pkg.TestOne")
					command := exec.CommandContext(t.Context(), filepath.Join(state.root, "le"),
						"commit", "approval", "clean", "kind", kind.name(),
						"session", question.namespace, "row", captured)
					command.Dir = state.root
					output, err := command.CombinedOutput()
					var failure *exec.ExitError
					if !errors.As(err, &failure) {
						t.Fatalf("invalid cleanup namespace did not produce a native refusal: %v\n%s", err, output)
					}
					if code := failure.ExitCode(); code != 2 {
						t.Fatalf("invalid cleanup namespace exited %d, want grammar refusal 2:\n%s", code, output)
					}
					if !strings.Contains(string(output), lepath.ErrCommitSessionSpelling.Error()) {
						t.Fatalf("native refusal did not name the canonical namespace requirement:\n%s", output)
					}
					state.assertUnchanged(t)
				})
			}
		})
	}
}

// Goal: native cleanup validates every consumed row before removing the first
// valid one. Method: send an actual captured row first, followed by an empty or
// line-breaking row, and require the specific producer refusal with every
// original ledger and HEAD unchanged. The native success control lives beside
// this test in TestApprovalCleanupNativeRouteBindsCapturedNamespace.
func TestApprovalCleanupNativeRouteRefusesMalformedConsumedRow(t *testing.T) {
	for _, kind := range []approvalKind{approvalKindRFC, approvalKindWeakening} {
		t.Run(kind.name(), func(t *testing.T) {
			for _, question := range []struct {
				name    string
				suffix  string
				problem string
			}{
				{name: "empty", problem: "approval cleanup: an exact consumed row is required"},
				{name: "carriage return", suffix: "\r", problem: "approval cleanup: a consumed row must be one whole line"},
				{name: "line feed", suffix: "\n", problem: "approval cleanup: a consumed row must be one whole line"},
			} {
				t.Run(question.name, func(t *testing.T) {
					state := approvalCleanupRefusalFixture(t)
					pending := readLedgerFile(t, state.root, kind.file(sessionOne).Path)
					captured := approvalCleanupFixtureRow(t, pending, "pkg.TestOne")
					malformed := ""
					if question.suffix != "" {
						malformed = captured + question.suffix
					}
					command := exec.CommandContext(t.Context(), filepath.Join(state.root, "le"),
						"commit", "approval", "clean", "kind", kind.name(),
						"session", sessionOne, "row", captured, "row", malformed)
					command.Dir = state.root
					output, err := command.CombinedOutput()
					var failure *exec.ExitError
					if !errors.As(err, &failure) {
						t.Fatalf("malformed consumed row did not produce a native refusal: %v\n%s", err, output)
					}
					if code := failure.ExitCode(); code != 1 {
						t.Fatalf("malformed consumed row exited %d, want mutation refusal 1:\n%s", code, output)
					}
					if !strings.Contains(string(output), "approval cleanup failed") {
						t.Fatalf("native cleanup did not report the failed mutation:\n%s", output)
					}
					if !strings.Contains(string(output), question.problem) {
						t.Fatalf("native cleanup did not name the consumed-row defect %q:\n%s", question.problem, output)
					}
					state.assertUnchanged(t)
				})
			}
		})
	}
}

type approvalCleanupRefusalState struct {
	root    string
	head    string
	ledgers []approvalCleanupRefusalLedger
}

type approvalCleanupRefusalLedger struct {
	path string
	body string
}

// approvalCleanupRefusalFixture creates real approved rows before the native
// request. Every case owns its repository and subprocess bridge; no shared
// pending approval or repository state is changed. It is not safe for parallel
// use because the existing bridge fixture sets process environment values.
func approvalCleanupRefusalFixture(t *testing.T) *approvalCleanupRefusalState {
	t.Helper()
	state := &approvalCleanupRefusalState{root: newLedgerRepository(t)}
	for _, namespace := range []string{sessionOne, sessionTwo} {
		for _, unit := range []string{"pkg.TestOne", "pkg.TestTwo"} {
			if _, err := rfc.Approve(state.root, namespace, unit, "original pending verdict"); err != nil {
				t.Fatal(err)
			}
			if _, err := testweakened.Approve(state.root, namespace, unit, "original pending verdict"); err != nil {
				t.Fatal(err)
			}
		}
		for _, kind := range []approvalKind{approvalKindRFC, approvalKindWeakening} {
			path := kind.file(namespace).Path
			state.ledgers = append(state.ledgers, approvalCleanupRefusalLedger{
				path: path,
				body: readLedgerFile(t, state.root, path),
			})
		}
	}
	state.head = headOf(t, state.root)
	return state
}

func (state *approvalCleanupRefusalState) assertUnchanged(t *testing.T) {
	t.Helper()
	for _, ledger := range state.ledgers {
		if after := readLedgerFile(t, state.root, ledger.path); after != ledger.body {
			t.Errorf("refused native cleanup changed %s:\nbefore:\n%s\nafter:\n%s", ledger.path, ledger.body, after)
		}
	}
	if after := headOf(t, state.root); after != state.head {
		t.Errorf("refused native cleanup moved HEAD: %s -> %s", state.head, after)
	}
}
