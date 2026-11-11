// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: session.go -- bashAgentIdentity and commandIdentityPrefix
// Related: bash.go -- bashSystemTmp and bashScratch, whose advice these pin

package hookruntime

import (
	"strings"
	"testing"
)

// subagentLine is the identity line runSubagentBash's payload owns.
const subagentLine = "export ZE_AGENT_ID=agent-1; export CLAUDE_CODE_SESSION_ID=parent-17; "

// TestScratchAdviceCarriesTheSubagentIdentityLine is review N3.
//
// VALIDATES: for a subagent, the /tmp refusal and the tmp/-root scratch
// refusal advise `dir=$(./le session scratch ensure)` behind the agent's own
// identity line, so following the advice is not refused again; for the main
// thread the advice carries no line.
// PREVENTS: advice whose literal command the identity check refuses.
func TestScratchAdviceCarriesTheSubagentIdentityLine(t *testing.T) {
	for _, command := range []string{
		subagentLine + "cat /tmp/x",
		subagentLine + "echo hi > tmp/out.log",
	} {
		code, _, message := runSubagentBash(t, command)
		if code != 2 {
			t.Fatalf("%q: code %d, want 2: %s", command, code, message)
		}
		if !strings.Contains(message, subagentLine+"dir=$(./le session scratch ensure)") {
			t.Errorf("%q: the scratch advice lacks the identity line: %s", command, message)
		}
	}
	code, _, message := runHook(t, t.TempDir(), "pretool-bash", map[string]any{
		"tool_name": "Bash", "tool_input": map[string]any{"command": "cat /tmp/x"},
	})
	if code != 2 {
		t.Fatalf("main thread: code %d, want 2: %s", code, message)
	}
	if strings.Contains(message, "export ZE_AGENT_ID") {
		t.Errorf("main thread: the advice carries an identity line: %s", message)
	}
}

// TestIdentityRefusalEndsWithTheFixingLine is review N4.
//
// VALIDATES: the refusal of a command with no identity line lists the
// readers before the fix and ends with the exact line to type; the agent's
// own exact line after leading whitespace passes, as it does without it.
// PREVENTS: the fix buried above a long list, and a correct line refused as
// another agent's.
func TestIdentityRefusalEndsWithTheFixingLine(t *testing.T) {
	code, _, message := runSubagentBash(t, "./le spec current")
	if code != 2 {
		t.Fatalf("code %d, want 2: %s", code, message)
	}
	// The line ends in "; " and the space is part of it: bashAgentIdentity
	// cuts the exact line, space included, from the command's front. Only
	// newlines after it are trimmed, so the suffix is the whole line.
	trimmed := strings.TrimRight(message, "\n")
	if !strings.HasSuffix(trimmed, subagentLine) {
		t.Errorf("the refusal does not end with the identity line: %s", message)
	}
	if strings.Index(message, "readers") > strings.LastIndex(message, "ZE_AGENT_ID=agent-1") {
		t.Errorf("the reader list follows the fixing line: %s", message)
	}
	for _, command := range []string{subagentLine + "true ./le", "  " + subagentLine + "true ./le", "\n" + subagentLine + "true ./le"} {
		code, _, message := runSubagentBash(t, command)
		if strings.Contains(message, "takes another identity") || strings.Contains(message, "needs the agent's identity line") {
			t.Errorf("%q: the agent's own line was refused (code %d): %s", command, code, message)
		}
	}
}
