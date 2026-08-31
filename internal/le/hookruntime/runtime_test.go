// Related: runtime.go -- native hook protocol dispatcher
//
// VALIDATES: hook JSON, exit severity, stderr messages, updated-input output,
// session isolation, and marker paths all execute in Go.
// PREVENTS: a configuration-only migration that leaves hook behavior inert.
package hookruntime

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runHook(t *testing.T, root, kind string, payload any) (int, string, string) {
	t.Helper()
	t.Setenv("CLAUDE_PROJECT_DIR", root)
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(kind, bytes.NewReader(body), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestBashRuntimePreservesSeverityAndAllMessages(t *testing.T) {
	root := t.TempDir()
	code, _, message := runHook(t, root, "pretool-bash", map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "git reset --hard; go build ./cmd/ze"},
	})
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	for _, want := range []string{"git reset", "go build without -o bin/"} {
		if !strings.Contains(message, want) {
			t.Errorf("message missing %q: %s", want, message)
		}
	}
}

func TestBashRuntimePrefixesOnlySafeForkIdentity(t *testing.T) {
	root := t.TempDir()
	code, output, _ := runHook(t, root, "pretool-bash", map[string]any{
		"session_id": "parent-17", "agent_id": "agent-1", "tool_name": "Bash",
		"tool_input": map[string]any{"command": "echo ok"},
	})
	if code != 0 || !strings.Contains(output, "export CLAUDE_CODE_SESSION_ID=parent-17; echo ok") {
		t.Fatalf("safe prefix: code=%d output=%q", code, output)
	}
	code, output, _ = runHook(t, root, "pretool-bash", map[string]any{
		"session_id": "../shared", "agent_id": "agent-1", "tool_name": "Bash",
		"tool_input": map[string]any{"command": "echo ok"},
	})
	if code != 0 || output != "" {
		t.Fatalf("unsafe identity changed input: code=%d output=%q", code, output)
	}
}

func TestWriteRuntimeUsesOneScratchIdentityPolicy(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o755); err != nil {
		t.Fatal(err)
	}
	payload := func(path string) map[string]any {
		return map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": path, "content": "x"}}
	}
	code, _, message := runHook(t, root, "pretool-writeedit", payload("tmp/out.log"))
	if code != 2 || !strings.Contains(message, "ad-hoc scratch") {
		t.Fatalf("root scratch: code=%d message=%q", code, message)
	}
	code, _, _ = runHook(t, root, "pretool-writeedit", payload("tmp/session/x/out.log"))
	if code != 0 {
		t.Fatalf("nested scratch code = %d, want 0", code)
	}
}

func TestAgentRuntimeBlocksCoveredRawTask(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "ai", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ai", "skills", "ze-review.md"), []byte("skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, message := runHook(t, root, "pretool-agent-skill", map[string]any{
		"tool_name": "Agent", "tool_input": map[string]any{"prompt": "Review this implementation for bugs"},
	})
	if code != 2 || !strings.Contains(message, "Use /ze-review") {
		t.Fatalf("code=%d message=%q", code, message)
	}
}

func TestLSPRuntimeWritesOnlyCurrentSessionMarker(t *testing.T) {
	root := t.TempDir()
	payload := map[string]any{
		"session_id": "session-a", "tool_name": "ToolSearch",
		"tool_input": map[string]any{"query": "select:LSP"},
	}
	code, _, _ := runHook(t, root, "block-until-lsp", payload)
	if code != 0 {
		t.Fatalf("ToolSearch code = %d", code)
	}
	marker := filepath.Join(root, "tmp", "session", ".lsp-loaded-session-a")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("marker: %v", err)
	}
	code, _, message := runHook(t, root, "block-until-lsp", map[string]any{
		"session_id": "session-b", "tool_name": "Read", "tool_input": map[string]any{},
	})
	if code != 2 || !strings.Contains(message, "LSP tool must be loaded") {
		t.Fatalf("isolated gate: code=%d message=%q", code, message)
	}
}

func TestValidateSpecFailsSpeakWithoutToolName(t *testing.T) {
	code, _, message := runHook(t, t.TempDir(), "validate-spec", map[string]any{"tool_input": map[string]any{}})
	if code != 2 || !strings.Contains(message, "NOTHING WAS CHECKED") {
		t.Fatalf("code=%d message=%q", code, message)
	}
}

func TestSessionIDHookStatusContract(t *testing.T) {
	root := t.TempDir()
	code, output, _ := runHook(t, root, "session-id", map[string]any{"session_id": "safe-id"})
	if code != 0 || output != "safe-id\n" {
		t.Fatalf("safe: code=%d output=%q", code, output)
	}
	code, _, _ = runHook(t, root, "session-id", map[string]any{})
	if code != 1 {
		t.Fatalf("absent code = %d, want 1", code)
	}
	code, _, _ = runHook(t, root, "session-id", map[string]any{"session_id": ".."})
	if code != 2 {
		t.Fatalf("invalid code = %d, want 2", code)
	}
	var out bytes.Buffer
	var errOut bytes.Buffer
	if code = Run("session-id", strings.NewReader("{"), &out, &errOut); code != 2 {
		t.Fatalf("malformed code = %d, want 2", code)
	}
}

func TestStopWarningIsOneLineNamingTheOpenSpec(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plan"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "tmp", "session"), 0o750); err != nil {
		t.Fatal(err)
	}
	spec := "# Spec\n\n| Field | Value |\n|-------|-------|\n| Status | in-progress |\n"
	if err := os.WriteFile(filepath.Join(root, "plan", "spec-open.md"), []byte(spec), 0o600); err != nil {
		t.Fatal(err)
	}
	claim := filepath.Join(root, "tmp", "session", ".session-sess-stop")
	if err := os.WriteFile(claim, []byte("spec-open.md\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, message := runHook(t, root, "block-premature-stop", map[string]any{
		"session_id": "sess-stop", "last_assistant_message": "The commit landed.",
	})
	if code != 1 {
		t.Fatalf("code = %d, want 1: %q", code, message)
	}
	if lines := strings.Count(strings.TrimSpace(message), "\n"); lines != 0 {
		t.Errorf("warning spans %d extra line(s): %q", lines, message)
	}
	for _, want := range []string{"spec-open.md", "in-progress", "Delegation:"} {
		if !strings.Contains(message, want) {
			t.Errorf("warning missing %q: %q", want, message)
		}
	}
}

func TestRuleCoverageNeverExitsNonZeroWithoutSaying(t *testing.T) {
	root := t.TempDir()
	rulesDir := filepath.Join(root, "ai", "rules")
	if err := os.MkdirAll(rulesDir, 0o750); err != nil {
		t.Fatal(err)
	}
	rule := "# performance.md\n**When:** writing any wire-encoding path\n**Severity:** blocking\n\n## Directives\n- do it\n"
	if err := os.WriteFile(filepath.Join(rulesDir, "performance.md"), []byte(rule), 0o600); err != nil {
		t.Fatal(err)
	}
	core := "# Ze Rules -- Always-On Core\n\n## principles.md\n`ai/rules/principles.md`\n**When:** always\n"
	if err := os.WriteFile(filepath.Join(rulesDir, "CORE.md"), []byte(core), 0o600); err != nil {
		t.Fatal(err)
	}
	row := map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"role": "assistant",
			"content": []any{map[string]any{
				"type": "tool_use", "name": "Write",
				"input": map[string]any{"file_path": filepath.Join(root, "internal", "wire.go")},
			}},
		},
	}
	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(root, "session.jsonl")
	if err := os.WriteFile(transcript, append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"session_id": "sess-cov", "transcript_path": transcript}
	code, _, message := runHook(t, root, "rule-coverage-report", payload)
	if code != 1 || !strings.Contains(message, "1 of 1 matched blocking rule(s) unread") {
		t.Fatalf("first run: code=%d message=%q", code, message)
	}
	code, _, message = runHook(t, root, "rule-coverage-report", payload)
	if code != 0 || strings.TrimSpace(message) != "" {
		t.Fatalf("repeated run: code=%d message=%q, want a silent 0", code, message)
	}
}
