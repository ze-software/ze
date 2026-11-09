// Design: docs/contributing/spec-workflow.md -- current hook and spec judgments
// VALIDATES: registered hooks judge each panic and direct WIP transition,
// require daemon-path proof, and retain delegated Git safety.
// PREVENTS: one allowed call exempting an edit, and claims being the only WIP gate.
package hookruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	lecommit "github.com/ze-software/ze/internal/le/commit"
	"github.com/ze-software/ze/internal/le/derived"
)

// historicalSession is the session every historical-intent probe claims.
const historicalSession = "historical-intent"

// historicalClaimTree builds a checkout whose session claims a ready spec, or
// claims none. Ready, not in-progress: the stop hook also warns about an
// in-progress claim, and that reason would hide a lost delegation reminder.
func historicalClaimTree(t *testing.T, claimed, spawned bool) string {
	t.Helper()
	t.Setenv("CLAUDE_CODE_SESSION_ID", historicalSession)
	root := t.TempDir()
	writeHookFixture(t, root, "tmp/session/.keep", "")
	if claimed {
		writeHookFixture(t, root, "tmp/session/.session-"+historicalSession, "spec-probe.md\n")
		writeHookFixture(t, root, "plan/post-release/engineering/spec-probe.md", "| Field | Value |\n|---|---|\n| Status | ready |\n")
	}
	if spawned {
		writeHookFixture(t, root, "tmp/session/.agent-spawned-"+historicalSession, "")
	}
	return root
}

// TestHistoricalHookIntentHasCurrentConsumer gives each of the four hook
// categories the Go port lost (plan/journal/refactor-removes-feature.md,
// 2026-09-03) its current disposition, and asks that consumer with one
// violating and one control input.
//
//   - commit deferral: RETIRED as a refusal, because a hook checks what a
//     command, path or file is, never how a message is worded
//     (ai/META-DECISIONS.md), and AC-8 forbids reinstating it. The permitted
//     behavior is asserted: deferral prose is admitted, and its only live
//     consumer, the postDeferral advisory, warns (exit 1) and refuses nothing.
//   - test-first (ai/rules/testing.md, "A test MUST exist"): consumed by the
//     test-coverage refusal of ./le commit create.
//   - delegation reminder (ai/META-DECISIONS.md, "Delegate to subagents"):
//     consumed by the Stop hook's open-state warning.
//   - subagent context (ai/rules/planning.md): consumed by the SubagentStart
//     hook, which names the parent's claimed spec. The retired LSP and
//     per-message wording it once asserted are not reinstated.
//
// MUTATION: removing the phrase loop from postDeferral, returning nil from
// testCoverageProblemsSelected, dropping the "Delegation: no subagent
// spawned" reason from hookStop, or dropping the claimed-spec block from
// hookSubagentContext each turns its subtest red.
func TestHistoricalHookIntentHasCurrentConsumer(t *testing.T) {
	t.Run("commit-deferral-retired-advisory-only", func(t *testing.T) {
		for _, test := range []struct {
			content string
			want    int
		}{
			{"The remainder is deferred to later.\n", 1},
			{"The remainder is implemented now.\n", 0},
		} {
			code, message, found := Probe("postDeferral", Payload{
				ToolName:  toolWrite,
				ToolInput: map[string]any{"file_path": "docs/probe.md", "content": test.content},
			}, t.TempDir())
			if !found || code != test.want {
				t.Fatalf("deferral advisory over %q = %d, found %v, want %d: %s", test.content, code, found, test.want, message)
			}
			if test.want == 1 && !strings.Contains(message, "Deferral language") {
				t.Fatalf("the advisory does not name deferral language: %s", message)
			}
		}
	})
	t.Run("test-first-commit-coverage", func(t *testing.T) {
		root := t.TempDir()
		writeHookFixture(t, root, "internal/probe/probe.go", "package probe\n")
		writeHookFixture(t, root, "internal/probe/probe_test.go", "package probe\n")
		untested, err := lecommit.CoverageProblems(root, []string{"internal/probe/probe.go"})
		if err != nil {
			t.Fatal(err)
		}
		if len(untested) != 1 || !strings.Contains(untested[0], "internal/probe/probe.go") {
			t.Fatalf("a commit of Go with no test was not refused naming its file: %q", untested)
		}
		tested, err := lecommit.CoverageProblems(root, []string{"internal/probe/probe.go", "internal/probe/probe_test.go"})
		if err != nil {
			t.Fatal(err)
		}
		if len(tested) != 0 {
			t.Fatalf("a commit carrying its test was refused: %q", tested)
		}
	})
	t.Run("delegation-reminder-stop-warning", func(t *testing.T) {
		for _, test := range []struct {
			name    string
			spawned bool
			want    int
		}{
			{"no-subagent", false, 1},
			{"subagent-spawned", true, 0},
		} {
			root := historicalClaimTree(t, true, test.spawned)
			code, output, found := ProbeLifecycle("block-premature-stop", Payload{SessionID: historicalSession}, root)
			if !found || code != test.want {
				t.Fatalf("%s: stop = %d, found %v, want %d: %s", test.name, code, found, test.want, output)
			}
			if test.want == 1 && !strings.Contains(output, "Delegation: no subagent spawned") {
				t.Fatalf("%s: the stop warning does not carry the delegation reason: %s", test.name, output)
			}
		}
	})
	t.Run("subagent-context-names-claimed-spec", func(t *testing.T) {
		const named = "Spec claimed by the session that spawned you: plan/post-release/engineering/spec-probe.md"
		for _, test := range []struct {
			name    string
			claimed bool
		}{
			{"no-claim", false},
			{"claimed", true},
		} {
			root := historicalClaimTree(t, test.claimed, false)
			code, output, found := ProbeLifecycle("subagent-context", Payload{SessionID: historicalSession}, root)
			if !found || code != 0 {
				t.Fatalf("%s: subagent context = %d, found %v: %s", test.name, code, found, output)
			}
			if strings.Contains(output, named) != test.claimed {
				t.Fatalf("%s: context names the claimed spec = %v, want %v: %s", test.name, !test.claimed, test.claimed, output)
			}
			if !strings.Contains(output, "ai/rules/planning.md") {
				t.Fatalf("%s: context lost the planning contract: %s", test.name, output)
			}
		}
	})
}

// TestHookVerdictBoundaries exercises the registered write check with allowed,
// forbidden and mixed calls, in both orders, without executing any panic.
func TestHookVerdictBoundaries(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		want int
	}{
		{"bug", `panic("BUG: invariant")`, 0},
		{"operating-error", `panic(err)`, 2},
		{"mixed-bug-first", "panic(\"BUG: invariant\")\npanic(err)", 2},
		{"mixed-bug-last", "panic(err)\npanic(\"BUG: invariant\")", 2},
		{"spaced", "panic ( err )\npanic (\"BUG: invariant\")", 2},
		{"comment", "// panic(err)\nreturn nil", 0},
		{"string", "message := `panic(err)`", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			code, message, found := Probe("writeGoPatterns", Payload{
				ToolName:  toolWrite,
				ToolInput: map[string]any{"file_path": "internal/probe/probe.go", "content": test.body},
			}, t.TempDir())
			if !found || code != test.want {
				t.Fatalf("registered write check = %d, found %v, want %d: %s", code, found, test.want, message)
			}
		})
	}
}

// TestDirectSpecEditChecksWIP drives the registered pre-write gate, without a
// claim, at the cap. An existing active spec may still be edited.
func TestDirectSpecEditChecksWIP(t *testing.T) {
	t.Setenv("ZE_SPEC_WIP_CAP", "1")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	root := t.TempDir()
	const active = "| Field | Value |\n|---|---|\n| Status | in-progress |\n"
	const ready = "| Field | Value |\n|---|---|\n| Status | ready |\n"
	writeHookFixture(t, root, "plan/post-release/engineering/spec-active.md", active)
	writeHookFixture(t, root, "plan/post-release/engineering/spec-next.md", ready)
	for _, test := range []struct {
		name string
		file string
		old  string
		text string
		want int
	}{
		{"crossing-cap", "spec-next.md", ready, active, 2},
		{"existing-active", "spec-active.md", active, active + "More detail.\n", 0},
		{"no-transition", "spec-next.md", ready, ready + "More detail.\n", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			code, message, found := Probe("writeSpecWIP", Payload{
				ToolName:  toolEdit,
				ToolInput: map[string]any{"file_path": filepath.Join(root, "plan/post-release/engineering", test.file), "old_string": test.old, "new_string": test.text},
			}, root)
			if !found || code != test.want {
				t.Fatalf("direct edit = %d, found %v, want %d: %s", code, found, test.want, message)
			}
			if code == 2 && !strings.Contains(message, "WIP") {
				t.Fatalf("refusal does not identify the WIP cap: %s", message)
			}
		})
	}
}

// TestDaemonPathDemandsFunctionalProof calls the actual validation hook over a
// daemon-path spec. Helper-only proof cannot establish daemon entry behavior.
func TestDaemonPathDemandsFunctionalProof(t *testing.T) {
	for _, test := range []struct {
		name, proof string
		want        int
	}{
		{"helper-only", "internal/component/probe/probe_test.go", 2},
		{"functional-entry", "test/ui/probe.ci", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			body := specFixture("internal/component/probe/probe.go") + "\n## Proof\n`" + test.proof + "`\n"
			writeHookFixture(t, root, "internal/component/probe/probe.go",
				"// Design: docs/probe-design.md -- the test-owned daemon subject\npackage probe\n")
			writeHookFixture(t, root, "docs/probe-design.md",
				"# Probe design\n\n<!-- source: internal/component/probe/probe.go -- the probe producer -->\n")
			body += "\n## Design documentation\n`docs/probe-design.md`\n"
			body = strings.Replace(body, "## Files to Modify\n\nOne.",
				"## Files to Modify\n\n- `internal/component/probe/probe.go`", 1)
			const file = "plan/post-release/engineering/spec-daemon.md"
			writeHookFixture(t, root, file, body)
			code, _, message := runHook(t, root, "validate-spec", map[string]any{
				"tool_name": toolEdit, "tool_input": map[string]any{"file_path": filepath.Join(root, file)},
			})
			if code != test.want {
				t.Fatalf("daemon spec = %d, want %d: %s", code, test.want, message)
			}
			if test.want == 2 && !strings.Contains(message, "functional test") {
				t.Fatalf("daemon proof was not the refusing judgment: %s", message)
			}
		})
	}
}

// TestDelegatedRenameUsesCommitRoute asks the real Bash hook, never Git, to
// judge a delegated direct index rename and the supported prepared route.
func TestDelegatedRenameUsesCommitRoute(t *testing.T) {
	for _, test := range []struct {
		command string
		want    int
	}{
		{"git mv old.go new.go", 2},
		{"bash tmp/commit-4f95a2b5-a-probe.sh", 0},
	} {
		code, _, message := runHook(t, t.TempDir(), "pretool-bash", map[string]any{
			"session_id": "rename-proof", "agent_id": "delegate", "tool_name": "Bash",
			"tool_input": map[string]any{"command": test.command},
		})
		if code != test.want {
			t.Fatalf("%s = %d, want %d: %s", test.command, code, test.want, message)
		}
	}
}

// TestSessionStartWithinHarnessBudget runs the actual session hook over a tree
// able to render artifacts. Required context survives and no artifact is built.
func TestSessionStartWithinHarnessBudget(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "startup-proof")
	root := derivedFixture(t)
	artifacts := artifactsWithPolicy(t, derived.SessionStartBuild)
	for _, artifact := range artifacts {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(artifact.Path))); err != nil {
			t.Fatal(err)
		}
	}
	started := time.Now()
	code, output, found := ProbeLifecycle("session-start", Payload{SessionID: "startup-proof"}, root)
	if !found || code != 0 {
		t.Fatalf("session start = %d, found %v: %s", code, found, output)
	}
	if elapsed := time.Since(started); elapsed >= 5*time.Second {
		t.Errorf("session start exceeded the harness budget: %s", elapsed)
	}
	for _, required := range []string{"Read spec + source", "ai/rules/TRIGGERS.md"} {
		if !strings.Contains(output, required) {
			t.Errorf("required context %q missing: %s", required, output)
		}
	}
	for _, artifact := range artifacts {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(artifact.Path))); !os.IsNotExist(err) {
			t.Errorf("session start built %s instead of leaving derivation to its consumer", artifact.Path)
		}
	}
}

// TestDesignEvidenceNamesSubjectAndFreshness records reads through the lifecycle
// entry before asking the registered design gate. Another source or a previous
// version of the right source cannot justify a claim about today's subject.
func TestDesignEvidenceNamesSubjectAndFreshness(t *testing.T) {
	for _, test := range []struct {
		name   string
		read   string
		change bool
		want   int
	}{
		{"matching", "internal/probe/subject.go", false, 0},
		{"wrong-subject", "internal/other/unrelated.go", false, 2},
		{"changed-after-read", "internal/probe/subject.go", true, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeHookFixture(t, root, "internal/probe/subject.go", "package probe\n")
			writeHookFixture(t, root, "internal/other/unrelated.go", "package other\n")
			observed, err := os.ReadFile(filepath.Join(root, test.read))
			if err != nil {
				t.Fatal(err)
			}
			code, _, message := runHook(t, root, "mark-source-read", map[string]any{
				"session_id": "design-proof", "tool_name": "Read",
				"tool_input":  map[string]any{"file_path": filepath.Join(root, test.read)},
				"tool_use_id": "design-proof-read",
				"tool_response": map[string]any{"type": "text", "file": map[string]any{
					"filePath": filepath.Join(root, test.read), "content": string(observed),
					"startLine": 1, "numLines": 2, "totalLines": 2,
				}},
			})
			if code != 0 {
				t.Fatalf("record source read: %d: %s", code, message)
			}
			if test.change {
				writeHookFixture(t, root, "internal/probe/subject.go", "package probe\nfunc Changed() {}\n")
			}
			code, message, found := Probe("writeDesignEvidence", Payload{
				SessionID: "design-proof", ToolName: toolWrite,
				ToolInput: map[string]any{
					"file_path": "plan/post-release/engineering/spec-subject.md",
					"content":   "## Files to Modify\n- `internal/probe/subject.go`\n",
				},
			}, root)
			if !found || code != test.want {
				t.Fatalf("design gate = %d, found %v, want %d: %s", code, found, test.want, message)
			}
		})
	}
}
