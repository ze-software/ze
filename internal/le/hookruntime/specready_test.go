// VALIDATES: writeSpecReady refuses a subagent's edit that moves a spec to
// ready, and leaves a subagent's edit that keeps Status as it is alone
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-20).
// PREVENTS: a design no one put to Thomas reaching ready through a subagent,
// as the 2026-11-12 pipe-chain spec did in the working tree.
package hookruntime

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/spec/design/designtest"
)

// readyFixtureSpec is a spec in Status design, the edit's starting text.
const readyFixtureSpec = "# Spec: widget\n\n| Field | Value |\n|-------|-------|\n| Status | design |\n| Updated | 2026-11-19 |\n\n## Task\n\nWidget.\n"

// specReadyContext is the check's context for one Edit of the fixture spec,
// replacing old with new, from the subagent agent (empty: the main session).
func specReadyContext(t *testing.T, root, agent, old, replacement string) context {
	t.Helper()
	ctx, ok := newContext(Payload{
		SessionID: "sess-a", ToolName: "Edit", AgentID: agent,
		ToolInput: map[string]any{"file_path": specFixturePath, "old_string": old, "new_string": replacement},
	}, root)
	if !ok {
		t.Fatal("newContext refused the fixture root")
	}
	return ctx
}

// TestSpecReadyRefusedFromSubagent is the wiring row "pre-write hook, subagent
// payload, Status to ready". Method: call writeSpecReady over a subagent's
// Edit that sets Status to ready, then over its Edit of another line.
func TestSpecReadyRefusedFromSubagent(t *testing.T) {
	root := t.TempDir()
	writeFixtureFile(t, root, specFixturePath, readyFixtureSpec)

	result := writeSpecReady(specReadyContext(t, root, "agent-1", "| Status | design |", "| Status | ready |"))
	if result == nil {
		t.Fatal("a subagent's edit moving a spec to ready was admitted")
	}
	if result.code != 2 {
		t.Fatalf("the refusal exited %d, want 2: %q", result.code, result.message)
	}
	for _, want := range []string{specFixturePath, "a subagent cannot move a spec to ready", "fix: hand the design back to the main session and stop"} {
		if !strings.Contains(result.message, want) {
			t.Fatalf("the refusal does not say %q: %q", want, result.message)
		}
	}

	if result := writeSpecReady(specReadyContext(t, root, "agent-1", "Widget.", "Widget, revised.")); result != nil {
		t.Fatalf("a subagent's edit that keeps Status was refused: %q", result.message)
	}
}

// TestSpecReadyNeedsTheApproval is AC-21 and its survivors. Method: in a
// checkout with an empty history, the main session's Edit that sets Status to
// ready is refused naming the check and approve commands; once Thomas's
// approval of the proposed text is recorded the same Edit passes; and an
// Edit of a spec already ready passes with no approval at all.
func TestSpecReadyNeedsTheApproval(t *testing.T) {
	root := t.TempDir()
	if err := designtest.InitRepository(root); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, root, specFixturePath, readyFixtureSpec)
	toReady := func() *verdict {
		return writeSpecReady(specReadyContext(t, root, "", "| Status | design |", "| Status | ready |"))
	}

	result := toReady()
	if result == nil {
		t.Fatal("the main session's move to ready with no owner approval was admitted")
	}
	if result.code != 2 {
		t.Fatalf("the refusal exited %d, want 2: %q", result.code, result.message)
	}
	for _, want := range []string{
		specFixturePath + " cannot be marked ready: its design has no owner approval matching this text",
		"./le spec design check spec " + specFixturePath,
		"./le spec design approve spec " + specFixturePath + ` words "<what he said>"`,
	} {
		if !strings.Contains(result.message, want) {
			t.Errorf("the refusal does not say %q:\n%s", want, result.message)
		}
	}

	proposed := strings.Replace(readyFixtureSpec, "| Status | design |", "| Status | ready |", 1)
	if err := designtest.SeedApproval(root, specFixturePath, proposed, "2026-11-20", "go ahead"); err != nil {
		t.Fatal(err)
	}
	if result := toReady(); result != nil {
		t.Fatalf("the move to ready Thomas approved was refused: %q", result.message)
	}

	already := t.TempDir()
	if err := designtest.InitRepository(already); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, already, specFixturePath, proposed)
	if result := writeSpecReady(specReadyContext(t, already, "", "Widget.", "Widget, revised.")); result != nil {
		t.Fatalf("an edit of a spec already ready was refused: %q", result.message)
	}
}
