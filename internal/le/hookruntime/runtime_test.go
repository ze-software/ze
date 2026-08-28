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
