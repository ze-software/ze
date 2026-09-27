// VALIDATES: writeStyleGuideRead refuses a Go write until the session, or the
// subagent, read the whole Go style guide through Read or a Bash print.
// PREVENTS: a Go edit from a context that never read the guide, or read part of it.
package hookruntime

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// styleGateRefusal is the text only writeStyleGuideRead writes, so a test can
// tell its refusal from any other check's.
const styleGateRefusal = "before this session's first Go edit"

// goEditPayload proposes one Edit of a Go file, from a session and, when agent
// is not empty, from a subagent of that session.
func goEditPayload(session, agent string) map[string]any {
	payload := map[string]any{
		"session_id": session,
		"tool_name":  "Edit",
		"tool_input": map[string]any{"file_path": "internal/core/sample.go", "old_string": "a := 1", "new_string": "a := 2"},
	}
	if agent != "" {
		payload["agent_id"] = agent
	}
	return payload
}

// readStyleGuide runs the Read hook over the style guide, as the harness does
// after a Read tool call.
func readStyleGuide(t *testing.T, root, session, agent string) {
	t.Helper()
	readStyleGuideWindow(t, root, session, agent, nil)
}

// guideLines is how many lines the test's copy of the style guide holds.
const guideLines = 10

// readStyleGuideWindow runs the Read hook over a guideLines-line copy of the
// style guide, with window adding the Read call's offset and limit.
func readStyleGuideWindow(t *testing.T, root, session, agent string, window map[string]any) {
	t.Helper()
	guide := filepath.Join(root, filepath.FromSlash(styleGuidePath))
	if err := os.MkdirAll(filepath.Dir(guide), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(guide, []byte(strings.Repeat("A line of the guide.\n", guideLines)), 0o600); err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"file_path": guide}
	maps.Copy(input, window)
	payload := map[string]any{
		"session_id": session,
		"tool_name":  "Read",
		"tool_input": input,
	}
	if agent != "" {
		payload["agent_id"] = agent
	}
	if code, _, message := runHook(t, root, "mark-source-read", payload); code != 0 {
		t.Fatalf("mark-source-read: code=%d message=%q", code, message)
	}
}

// requireStyleGate asserts whether the pre-write hook refuses the Go edit for
// want the style guide read.
func requireStyleGate(t *testing.T, root string, payload map[string]any, refused bool) {
	t.Helper()
	code, _, message := runHook(t, root, "pretool-writeedit", payload)
	got := strings.Contains(message, styleGateRefusal)
	if got != refused {
		t.Fatalf("style gate refused = %v (code %d), want %v: %q", got, code, refused, message)
	}
	if refused && code != 2 {
		t.Fatalf("a refusal must exit 2, got %d: %q", code, message)
	}
	if refused && !strings.Contains(message, styleGuidePath) {
		t.Fatalf("the refusal does not name the file to read: %q", message)
	}
}

// TestStyleGateRefusesGoEditUntilTheGuideIsRead drives the whole hook chain: a
// Go edit with no record of the style guide is refused, and the same edit after
// a Read of the guide in the same session passes the gate.
func TestStyleGateRefusesGoEditUntilTheGuideIsRead(t *testing.T) {
	root := t.TempDir()
	requireStyleGate(t, root, goEditPayload("sess-a", ""), true)
	readStyleGuide(t, root, "sess-a", "")
	requireStyleGate(t, root, goEditPayload("sess-a", ""), false)
}

// TestStyleGateCoversWriteAndMultiEdit holds the tools that write a Go file
// whole or in several hunks to the same gate as Edit.
func TestStyleGateCoversWriteAndMultiEdit(t *testing.T) {
	root := t.TempDir()
	for _, tool := range []string{"Write", "MultiEdit"} {
		payload := goEditPayload("sess-a", "")
		payload["tool_name"] = tool
		requireStyleGate(t, root, payload, true)
	}
}

// TestStyleGateLeavesNonGoWritesAlone pins the negative half: a write to a
// file that is not Go never needs the guide.
func TestStyleGateLeavesNonGoWritesAlone(t *testing.T) {
	root := t.TempDir()
	payload := map[string]any{
		"session_id": "sess-a",
		"tool_name":  "Write",
		"tool_input": map[string]any{"file_path": "docs/sample.md", "content": "Sample.\n"},
	}
	requireStyleGate(t, root, payload, false)
}

// TestStyleGateIgnoresAnotherSessionsRead proves the record is per session: a
// read by one session does not open the gate for another.
func TestStyleGateIgnoresAnotherSessionsRead(t *testing.T) {
	root := t.TempDir()
	readStyleGuide(t, root, "sess-b", "")
	requireStyleGate(t, root, goEditPayload("sess-a", ""), true)
}

// TestStyleGateHoldsEachSubagentToItsOwnRead covers the payload a subagent's
// hooks receive: the PARENT's session_id with the subagent's own agent_id. The
// parent's read does not open the gate for the subagent, the subagent's read
// does, and the subagent's read does not open it for the parent.
func TestStyleGateHoldsEachSubagentToItsOwnRead(t *testing.T) {
	root := t.TempDir()
	readStyleGuide(t, root, "sess-a", "")
	requireStyleGate(t, root, goEditPayload("sess-a", "agent1"), true)
	readStyleGuide(t, root, "sess-a", "agent1")
	requireStyleGate(t, root, goEditPayload("sess-a", "agent1"), false)
	requireStyleGate(t, root, goEditPayload("sess-a", "agent2"), true)

	readStyleGuide(t, root, "sess-c", "agent1")
	requireStyleGate(t, root, goEditPayload("sess-c", ""), true)
}

// TestStyleGateFailsClosedWithoutAnId refuses the edit when the payload names a
// session id that cannot name a marker, rather than reading that as a pass.
func TestStyleGateFailsClosedWithoutAnId(t *testing.T) {
	root := t.TempDir()
	payload := goEditPayload("../escape", "")
	code, _, message := runHook(t, root, "pretool-writeedit", payload)
	if code != 2 || !strings.Contains(message, "no usable id") {
		t.Fatalf("code=%d message=%q, want a fail-closed refusal", code, message)
	}
	payload = goEditPayload("sess-a", "../agent")
	code, _, message = runHook(t, root, "pretool-writeedit", payload)
	if code != 2 || !strings.Contains(message, "no usable id") {
		t.Fatalf("code=%d message=%q, want a fail-closed refusal for a bad agent id", code, message)
	}
}

// TestStyleGateCountsOnlyAWholeRead holds the Read hook to "in full": a Read
// with an offset past the first line, or a limit short of the last line, records
// nothing, and a limit that reaches the last line records the read.
func TestStyleGateCountsOnlyAWholeRead(t *testing.T) {
	partial := []map[string]any{
		{"offset": float64(3)},
		{"limit": float64(guideLines - 1)},
		{"offset": float64(1), "limit": float64(5)},
		{"limit": "all"},
	}
	for _, window := range partial {
		root := t.TempDir()
		readStyleGuideWindow(t, root, "sess-a", "", window)
		requireStyleGate(t, root, goEditPayload("sess-a", ""), true)
	}
	whole := []map[string]any{
		{"limit": float64(guideLines)},
		{"offset": float64(1), "limit": float64(guideLines + 5)},
	}
	for _, window := range whole {
		root := t.TempDir()
		readStyleGuideWindow(t, root, "sess-a", "", window)
		requireStyleGate(t, root, goEditPayload("sess-a", ""), false)
	}
}

// TestStyleGateMeasuresTheFileOnDisk covers the two edges of counting lines: a
// Read of a guide that is not on disk records nothing, because nobody can say
// the read was whole, and a guide whose last line has no newline still counts
// that line, so a limit equal to the line count is a whole read.
func TestStyleGateMeasuresTheFileOnDisk(t *testing.T) {
	read := func(t *testing.T, root string, window map[string]any) {
		t.Helper()
		input := map[string]any{"file_path": filepath.Join(root, filepath.FromSlash(styleGuidePath))}
		maps.Copy(input, window)
		runHook(t, root, "mark-source-read", map[string]any{"session_id": "sess-a", "tool_name": "Read", "tool_input": input})
	}

	missing := t.TempDir()
	read(t, missing, nil)
	requireStyleGate(t, missing, goEditPayload("sess-a", ""), true)

	unterminated := t.TempDir()
	guide := filepath.Join(unterminated, filepath.FromSlash(styleGuidePath))
	if err := os.MkdirAll(filepath.Dir(guide), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(guide, []byte("one\ntwo\nthree"), 0o600); err != nil {
		t.Fatal(err)
	}
	read(t, unterminated, map[string]any{"limit": float64(2)})
	requireStyleGate(t, unterminated, goEditPayload("sess-a", ""), true)
	read(t, unterminated, map[string]any{"limit": float64(3)})
	requireStyleGate(t, unterminated, goEditPayload("sess-a", ""), false)
}

// TestStyleGateCountsABashRead drives the Bash pre-hook: a command that prints
// the whole guide records the read. A command that prints a range, pipes the
// guide through another program, only names it, or rewrites it records nothing.
func TestStyleGateCountsABashRead(t *testing.T) {
	bash := func(t *testing.T, root, session, agent, command string) {
		t.Helper()
		payload := map[string]any{
			"session_id": session,
			"tool_name":  "Bash",
			"tool_input": map[string]any{"command": command},
		}
		if agent != "" {
			payload["agent_id"] = agent
		}
		runHook(t, root, "pretool-bash", payload)
	}
	records := []string{
		"cat " + styleGuidePath,
		"cd /repo && cat -n /repo/" + styleGuidePath,
		"nl ./" + styleGuidePath,
	}
	for _, command := range records {
		root := t.TempDir()
		bash(t, root, "sess-a", "", command)
		requireStyleGate(t, root, goEditPayload("sess-a", ""), false)
	}
	ignored := []string{
		"echo " + styleGuidePath,
		"grep -n panic " + styleGuidePath,
		"sed -i 's/a/b/' " + styleGuidePath,
		"sed -n '1,400p' " + styleGuidePath,
		"head -n 800 " + styleGuidePath,
		"tail " + styleGuidePath,
		"bat -r 1:40 " + styleGuidePath,
		"cat " + styleGuidePath + " | head",
		"cat " + styleGuidePath + " > /dev/null",
		"cat " + styleGuidePath + " >tmp/x",
		"diff <(cat " + styleGuidePath + ") other.md",
		"echo $(cat " + styleGuidePath + ")",
		"less +200 " + styleGuidePath,
		"more +40 " + styleGuidePath,
		"cat docs/contributing/not-" + filepath.Base(styleGuidePath),
	}
	for _, command := range ignored {
		root := t.TempDir()
		bash(t, root, "sess-a", "", command)
		requireStyleGate(t, root, goEditPayload("sess-a", ""), true)
	}

	root := t.TempDir()
	bash(t, root, "sess-a", "agent1", "cat "+styleGuidePath)
	requireStyleGate(t, root, goEditPayload("sess-a", "agent1"), false)
	requireStyleGate(t, root, goEditPayload("sess-a", ""), true)
	requireStyleGate(t, root, goEditPayload("sess-a", "agent2"), true)
}
