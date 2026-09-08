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

// VALIDATES: the Data Flow Entry Point placeholder is refused from `design`
// onward and accepted at `skeleton`.
// PREVENTS: a hook that refuses the state the repository is full of. A skeleton
// is a spec with no design yet, so its unwritten Entry Point is the honest
// state; plan/README.md says so, and 39 committed specs carry that exact
// placeholder. Refusing it there blocked every new skeleton from being written
// at all, which is how this test came to exist.
func TestValidateSpecPlaceholderScopedToDesignOnward(t *testing.T) {
	const entryPoint = "\n## Data Flow\n\n### Entry Point\n[Where data enters]\n" +
		"\n### Transformation Path\n\n### Boundaries Crossed\n\n### Integration Points\n"

	body := func(status string) string {
		return "# Spec: fixture\n\n| Field | Value |\n|-------|-------|\n| Status | " +
			status + " |\n| Updated | 2026-09-06 |\n" + entryPoint
	}

	skeleton, _ := validateSpecText(t.TempDir(), body("skeleton"))
	for _, got := range skeleton {
		if strings.Contains(got, "Entry Point contains placeholder") {
			t.Fatalf("a skeleton was refused for the placeholder it is allowed to carry: %q", got)
		}
	}

	design, _ := validateSpecText(t.TempDir(), body("design"))
	found := false
	for _, got := range design {
		if strings.Contains(got, "Entry Point contains placeholder") {
			found = true
		}
	}
	if !found {
		t.Fatalf("a design spec kept the placeholder and was not refused: %v", design)
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

// TestPostFormatGoLeavesGeneratedSourceAlone drives the post-edit hook over two
// files that gofmt would reshape and that differ only by the generated marker.
// The generated one must come back byte for byte: its generator decides its
// bytes, and a formatted copy is a file no generator run reproduces, which the
// templ output check then reports as out of date.
func TestPostFormatGoLeavesGeneratedSourceAlone(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module sample\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	const body = "package sample\n\nimport \"errors\"\nimport \"fmt\"\n\nvar  errSample = errors.New(fmt.Sprint(1))\n"
	const marker = "// Code generated by sample. DO NOT EDIT.\n\n"

	write := func(name, text string) string {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	generated := write("generated.go", marker+body)
	authored := write("authored.go", body)

	edit := func(path string) {
		runHook(t, root, "posttool-writeedit", map[string]any{
			"tool_name":  "Edit",
			"tool_input": map[string]any{"file_path": path},
		})
	}
	read := func(path string) string {
		text, err := os.ReadFile(path) //nolint:gosec // the path is this test's own temporary file
		if err != nil {
			t.Fatal(err)
		}
		return string(text)
	}

	edit(generated)
	if got := read(generated); got != marker+body {
		t.Errorf("the hook rewrote generated source:\n%q", got)
	}

	// The same bytes without the marker are formatted, which is what proves the
	// guard reads the marker rather than skipping every Go file.
	edit(authored)
	if got := read(authored); got == body {
		t.Errorf("authored source was left unformatted, so the marker is not what decided it:\n%q", got)
	}
}

// VALIDATES: a Write or an Edit naming a spec the hook cannot read is refused
// and says so, while a spec that reads is still validated and passes.
// PREVENTS: the silent version of no validation at all, one line below the
// repair the function already carries a comment about. The read failure
// returned 0, which the hook protocol reads as "checked, allowed", so a payload
// naming an unreadable spec reported success having validated nothing
// (ai/rules/evidence.md).
func TestValidateSpecRefusesASpecItCannotRead(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plan"), 0o750); err != nil {
		t.Fatal(err)
	}

	absent := filepath.Join(root, "plan", "spec-this-does-not-exist.md")
	code, _, message := runHook(t, root, "validate-spec", map[string]any{
		"tool_name":  "Edit",
		"tool_input": map[string]any{"file_path": absent, "new_string": "x"},
	})
	if code != 2 {
		t.Fatalf("an unreadable spec passed with code = %d, want 2 (message %q)", code, message)
	}
	if !strings.Contains(message, "NOTHING WAS CHECKED") || !strings.Contains(message, "plan/spec-this-does-not-exist.md") {
		t.Fatalf("the refusal named neither the failure nor the path: %q", message)
	}

	present := filepath.Join(root, "plan", "spec-kind.md")
	// A skeleton carries no design document, so the anchor audit stays out of
	// this test: what it pins is that a spec the hook CAN read is validated and
	// passes, which is the polarity the refusal above must not have taken away.
	readable := strings.Replace(specFixture("internal/le/hookruntime/lifecycle.go"), "| Status | design |", "| Status | skeleton |", 1)
	if err := os.WriteFile(present, []byte(readable), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, message = runHook(t, root, "validate-spec", map[string]any{
		"tool_name":  "Edit",
		"tool_input": map[string]any{"file_path": present, "new_string": "x"},
	})
	if code != 0 {
		t.Fatalf("a readable spec was refused with code = %d: %q", code, message)
	}
}
