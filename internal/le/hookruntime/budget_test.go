// Related: budget.go -- the editing agent's tool-call budget
//
// VALIDATES: an editing agent's 101st hooked call is refused through the
// dispatcher, the handoff calls pass after the budget, and no other context
// is counted.
// PREVENTS: a budget that binds the main thread or a read-only agent, and a
// refusal that also refuses the handoff it asks for.
package hookruntime

import (
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

func agentPayload(agentType, tool string, input map[string]any) map[string]any {
	return map[string]any{
		"session_id": "parent-17", "agent_id": "a1b2c3", "agent_type": agentType,
		"tool_name": tool, "tool_input": input,
	}
}

func TestAgentCallBudgetRefusesTheCallPastTheBudget(t *testing.T) {
	root := t.TempDir()
	bash := agentPayload(budgetedAgentType, "Bash", map[string]any{"command": "echo ok"})
	for call := 1; call <= agentCallBudget; call++ {
		code, _, message := runHook(t, root, "pretool-bash", bash)
		if code != 0 {
			t.Fatalf("call %d: code=%d message=%q", call, code, message)
		}
	}
	code, _, message := runHook(t, root, "pretool-bash", bash)
	if code != 2 || !strings.Contains(message, "100 tool calls; its budget is 100") {
		t.Fatalf("call 101: code=%d message=%q", code, message)
	}
	edit := agentPayload(budgetedAgentType, "Edit", map[string]any{"file_path": "internal/core/x.go", "old_string": "a", "new_string": "b"})
	code, _, message = runHook(t, root, "pretool-writeedit", edit)
	if code != 2 || !strings.Contains(message, "its budget") {
		t.Fatalf("edit past budget: code=%d message=%q", code, message)
	}
}

func TestAgentCallBudgetLetsTheHandoffThrough(t *testing.T) {
	root := t.TempDir()
	bash := agentPayload(budgetedAgentType, "Bash", map[string]any{"command": "echo ok"})
	for call := 1; call <= agentCallBudget+1; call++ {
		runHook(t, root, "pretool-bash", bash)
	}
	handoffs := []struct {
		kind    string
		payload map[string]any
	}{
		{"pretool-bash", agentPayload(budgetedAgentType, "Bash", map[string]any{"command": "./le spec state current"})},
		{"pretool-bash", agentPayload(budgetedAgentType, "Bash", map[string]any{"command": "./le spec state current"})},
		{"pretool-bash", agentPayload(budgetedAgentType, "Bash", map[string]any{"command": "cat >> tmp/session/2026-09-15-parent-17/state/session-state-spec-x-parent-17.md <<'EOF'\n- done\nEOF"})},
		{"pretool-writeedit", agentPayload(budgetedAgentType, "Write", map[string]any{"file_path": "tmp/session/2026-09-15-parent-17/state/session-state-spec-x-parent-17.md", "content": "- done"})},
	}
	for _, handoff := range handoffs {
		code, _, message := runHook(t, root, handoff.kind, handoff.payload)
		if code != 0 {
			t.Fatalf("%s handoff past budget: code=%d message=%q", handoff.kind, code, message)
		}
	}
	code, _, message := runHook(t, root, "pretool-bash", bash)
	if code != 2 {
		t.Fatalf("ordinary call after handoff: code=%d message=%q", code, message)
	}
}

func TestAgentCallBudgetBindsOnlyTheEditingAgent(t *testing.T) {
	root := t.TempDir()
	reader := agentPayload("ze-read", "Bash", map[string]any{"command": "echo ok"})
	main := map[string]any{"session_id": "parent-17", "tool_name": "Bash", "tool_input": map[string]any{"command": "echo ok"}}
	for call := 1; call <= agentCallBudget+1; call++ {
		if code, _, message := runHook(t, root, "pretool-bash", reader); code != 0 {
			t.Fatalf("ze-read call %d: code=%d message=%q", call, code, message)
		}
		if code, _, message := runHook(t, root, "pretool-bash", main); code != 0 {
			t.Fatalf("main call %d: code=%d message=%q", call, code, message)
		}
	}
}

// VALIDATES: every agent id the session identity accepts (lepath.ValidAgentID,
// which admits '.') is counted by the budget and refused past it, and an id
// the identity refuses is reported as uncounted.
// PREVENTS: a second agent-id character set in the budget that lets an agent
// the identity names run uncounted.
func TestAgentCallBudgetCountsEveryIdTheIdentityAccepts(t *testing.T) {
	agent := "a1.b2_c-3"
	if !lepath.ValidAgentID(agent) {
		t.Fatalf("lepath.ValidAgentID(%q) = false, want the identity to accept it", agent)
	}
	root := t.TempDir()
	bash := agentPayload(budgetedAgentType, "Bash", map[string]any{"command": "echo ok"})
	bash["agent_id"] = agent
	for call := 1; call <= agentCallBudget; call++ {
		if code, _, message := runHook(t, root, "pretool-bash", bash); code != 0 || message != "" {
			t.Fatalf("call %d: code=%d message=%q", call, code, message)
		}
	}
	if code, _, message := runHook(t, root, "pretool-bash", bash); code != 2 || !strings.Contains(message, "its budget is 100") {
		t.Fatalf("call 101 for %q: code=%d message=%q, want the budget refusal", agent, code, message)
	}
	edit := agentPayload(budgetedAgentType, "Edit", map[string]any{"file_path": "notes.md", "old_string": "a", "new_string": "b"})
	edit["agent_id"] = "a1+b2"
	if lepath.ValidAgentID("a1+b2") {
		t.Fatal(`lepath.ValidAgentID("a1+b2") = true, want the separator refused`)
	}
	if _, _, message := runHook(t, t.TempDir(), "pretool-writeedit", edit); !strings.Contains(message, "is not a file name, so this agent is not counted") {
		t.Fatalf("unsafe agent id: message=%q, want the uncounted report", message)
	}
}

func TestAgentCallBudgetCountsEachAgentAlone(t *testing.T) {
	root := t.TempDir()
	first := agentPayload(budgetedAgentType, "Bash", map[string]any{"command": "echo ok"})
	for call := 1; call <= agentCallBudget+1; call++ {
		runHook(t, root, "pretool-bash", first)
	}
	second := agentPayload(budgetedAgentType, "Bash", map[string]any{"command": "echo ok"})
	second["agent_id"] = "d4e5f6"
	code, _, message := runHook(t, root, "pretool-bash", second)
	if code != 0 {
		t.Fatalf("second agent's first call: code=%d message=%q", code, message)
	}
}
