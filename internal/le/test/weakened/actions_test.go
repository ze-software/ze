package testweakened

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// TestCheckThroughTheActionTable drives `le test weakened check` through the
// action table, the route the command line takes, rather than calling Audit.
//
// The method is a fixture repository of a base commit and one commit after it
// that adds an assertion, named to lepath.Root through ZE_REPO_ROOT. The table
// must refuse the `base` keyword with no value before the audit runs, must
// answer 0 for the one-commit range from that base, and must answer 1 once the
// worktree weakens the test.
//
// VALIDATES: `check base <revision>` reaches checkAnswer with the revision
// and answers the audit's exit code; `check base` alone exits 2 with no
// payload.
// PREVENTS: a renamed verb or keyword, or a checkAnswer that drops the base or
// the report, passing while the direct Audit tests stay green.
func TestCheckThroughTheActionTable(t *testing.T) {
	root := t.TempDir()
	path := "pkg/a_test.go"
	baseline := "package p\nfunc TestA(t *testing.T) {\n\trequire.NoError(t, err)\n\trequire.Equal(t, 1, got)\n}\n"
	writeProspectiveFile(t, root, path, baseline)
	runProspectiveGit(t, root, "init", "-q")
	runProspectiveGit(t, root, "add", "-A")
	auditCommit(t, root, "baseline")
	base := strings.TrimSpace(runProspectiveGitOutput(t, root, "rev-parse", "HEAD"))
	strengthened := strings.Replace(baseline, "\trequire.Equal(t, 1, got)\n", "\trequire.Equal(t, 1, got)\n\trequire.True(t, ok)\n", 1)
	writeProspectiveFile(t, root, path, strengthened)
	runProspectiveGit(t, root, "add", "-A")
	auditCommit(t, root, "one more assertion, which weakens nothing")
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	payload, code := Answer([]string{"check", "base"})
	if code != 2 {
		t.Fatalf("check base with no value answered %d, want 2", code)
	}
	if payload != nil {
		t.Fatalf("check base with no value returned a payload: %#v", payload)
	}

	payload, code = Answer([]string{"check", "base", base})
	if code != 0 {
		t.Fatalf("check base %s over a clean range answered %d, want 0: %#v", base, code, payload)
	}
	report, ok := payload.(AuditReport)
	if !ok {
		t.Fatalf("check returned %T, want AuditReport", payload)
	}
	if report.Base != base {
		t.Fatalf("check audited from base %q, want %q", report.Base, base)
	}
	if report.Examined != 1 {
		t.Fatalf("check examined %d changed test files, want the 1 the range changed", report.Examined)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("check over a clean range found %#v", report.Findings)
	}

	writeProspectiveFile(t, root, path, strings.ReplaceAll(strengthened, "\trequire.NoError(t, err)\n", ""))
	payload, code = Answer([]string{"check", "base", base})
	if code != 1 {
		t.Fatalf("check base %s over a weakened worktree answered %d, want 1: %#v", base, code, payload)
	}
}
