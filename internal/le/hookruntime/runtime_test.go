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
	"os/exec"
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
	// An agent whose session id is unsafe blocks rather than run unprefixed as
	// the main thread; TestBashRuntimeBlocksAnAgentWithNoSessionID covers the
	// message.
	if code != 2 || output != "" {
		t.Fatalf("unsafe identity: code=%d output=%q, want exit 2 and no updated input", code, output)
	}
}

// VALIDATES: a subagent's Bash call carries its own agent id beside the
// parent's session id, a fork with no agent id carries none, and an agent id
// that cannot name a file blocks the call (exit 2, no updated input) with a
// message naming the cause and the next step.
// PREVENTS: sibling subagents resolving one identity, which made them share a
// commit namespace and a spec claim
// (plan/journal/concurrent-session-corruption.md), and an unsafe agent id
// running the call silently as the main thread.
func TestBashRuntimeExportsTheAgentIdentity(t *testing.T) {
	root := t.TempDir()
	code, output, _ := runHook(t, root, "pretool-bash", map[string]any{
		"session_id": "parent-17", "agent_id": "agent-1", "tool_name": "Bash",
		"tool_input": map[string]any{"command": "echo ok"},
	})
	if code != 0 || !strings.Contains(output, "export ZE_AGENT_ID=agent-1; export CLAUDE_CODE_SESSION_ID=parent-17; echo ok") {
		t.Fatalf("agent prefix: code=%d output=%q", code, output)
	}
	t.Setenv("CLAUDE_CODE_FORK_SUBAGENT", "1")
	code, output, _ = runHook(t, root, "pretool-bash", map[string]any{
		"session_id": "parent-17", "tool_name": "Bash",
		"tool_input": map[string]any{"command": "echo ok"},
	})
	if code != 0 || strings.Contains(output, "ZE_AGENT_ID") || !strings.Contains(output, "export CLAUDE_CODE_SESSION_ID=parent-17; echo ok") {
		t.Fatalf("fork prefix: code=%d output=%q", code, output)
	}
	for _, agent := range []string{"../agent", "agent+x"} {
		code, output, errOutput := runHook(t, root, "pretool-bash", map[string]any{
			"session_id": "parent-17", "agent_id": agent, "tool_name": "Bash",
			"tool_input": map[string]any{"command": "echo ok"},
		})
		if code != 2 || output != "" {
			t.Fatalf("unsafe agent %q: code=%d output=%q, want exit 2 and no updated input", agent, code, output)
		}
		if !strings.Contains(errOutput, "BLOCKED") || !strings.Contains(errOutput, "main thread") || !strings.Contains(errOutput, "Next:") {
			t.Fatalf("unsafe agent %q: message %q lacks what, why or next step", agent, errOutput)
		}
	}
}

// VALIDATES: a subagent's Bash call whose payload names an agent but carries
// an absent or unsafe session_id blocks (exit 2, no updated input) with a
// what/why/next-step message, while a fork subagent (no agent id) with the
// same payload still runs unprefixed.
// PREVENTS: a subagent with no usable session id running its call silently
// as the main thread.
func TestBashRuntimeBlocksAnAgentWithNoSessionID(t *testing.T) {
	root := t.TempDir()
	for _, payload := range []map[string]any{
		{"agent_id": "agent-1", "tool_name": "Bash", "tool_input": map[string]any{"command": "echo ok"}},
		{"session_id": "../parent", "agent_id": "agent-1", "tool_name": "Bash", "tool_input": map[string]any{"command": "echo ok"}},
		{"session_id": 17, "agent_id": "agent-1", "tool_name": "Bash", "tool_input": map[string]any{"command": "echo ok"}},
	} {
		code, output, errOutput := runHook(t, root, "pretool-bash", payload)
		if code != 2 || output != "" {
			t.Fatalf("session_id %v: code=%d output=%q, want exit 2 and no updated input", payload["session_id"], code, output)
		}
		if !strings.Contains(errOutput, "BLOCKED") || !strings.Contains(errOutput, "main thread") || !strings.Contains(errOutput, "Next:") {
			t.Fatalf("session_id %v: message %q lacks what, why or next step", payload["session_id"], errOutput)
		}
	}
	t.Setenv("CLAUDE_CODE_FORK_SUBAGENT", "1")
	for _, payload := range []map[string]any{
		{"tool_name": "Bash", "tool_input": map[string]any{"command": "echo ok"}},
		{"session_id": "../parent", "tool_name": "Bash", "tool_input": map[string]any{"command": "echo ok"}},
	} {
		code, output, errOutput := runHook(t, root, "pretool-bash", payload)
		if code != 0 || output != "" {
			t.Fatalf("fork session_id %v: code=%d output=%q message=%q, want exit 0 unprefixed", payload["session_id"], code, output, errOutput)
		}
	}
}

// VALIDATES: an Edit from a subagent whose payload names an agent but carries
// an absent or unsafe session_id is refused by the weakening check (exit 2)
// with a message naming the cause and the next step.
// PREVENTS: the weakening check judging a subagent's edit against another
// main thread's owner approvals.
func TestWriteRuntimeWeakeningRefusesAnAgentWithNoSessionID(t *testing.T) {
	root := t.TempDir()
	for _, payload := range []map[string]any{
		{"agent_id": "agent-1", "tool_name": "Edit", "tool_input": map[string]any{"file_path": filepath.Join(root, "notes.md"), "old_string": "a", "new_string": "b"}},
		{"session_id": "../parent", "agent_id": "agent-1", "tool_name": "Edit", "tool_input": map[string]any{"file_path": filepath.Join(root, "notes.md"), "old_string": "a", "new_string": "b"}},
	} {
		code, _, errOutput := runHook(t, root, "pretool-writeedit", payload)
		if code != 2 {
			t.Fatalf("session_id %v: code=%d message=%q, want exit 2", payload["session_id"], code, errOutput)
		}
		if !strings.Contains(errOutput, "weakening check cannot name the owner approval namespace of this subagent's edit") || !strings.Contains(errOutput, "another main thread's approvals") || !strings.Contains(errOutput, "Next:") {
			t.Fatalf("session_id %v: message %q lacks what, why or next step", payload["session_id"], errOutput)
		}
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

// TestValidateSpecNeedsATestCommandNotTheFullGate drives validateSpecText with
// the smallest accepted spec, its checklist line swapped for each spelling.
//
// VALIDATES: a spec naming only a targeted command (`./le verify changed`,
// `./le test unit <group>`) is accepted, the legacy `./le verify worktree` and
// `./le verify current fast` lines still are, and a spec naming no `./le test`
// or `./le verify` command is refused with a message that names the scoped run.
// PREVENTS: the validator keeping the full gate as a mandatory spec item, or
// its refusal teaching an agent a full run. Agents run targeted tests and the
// scoped verify, and the full run is the release check Thomas starts.
func TestValidateSpecNeedsATestCommandNotTheFullGate(t *testing.T) {
	const legacy = "- [ ] `./le verify worktree` passes\n"
	base := specFixture("internal/probe/probe.go")
	if !strings.Contains(base, legacy) {
		t.Fatalf("specFixture no longer carries the line this test swaps: %q", legacy)
	}
	refused := func(body string) string {
		errors, _ := validateSpecText(t.TempDir(), body)
		for _, got := range errors {
			if strings.Contains(got, "verification checklist item") {
				return got
			}
		}
		return ""
	}
	accepted := []struct {
		name string
		line string
	}{
		{"legacy-full-gate", legacy},
		{"verify-current-fast", "- [ ] `./le verify current fast` passes\n"},
		{"verify-changed", "- [ ] `./le verify changed` passes\n"},
		{"unit-group", "- [ ] `./le test unit hookruntime` passes\n"},
		{"functional-suite", "- [ ] `./le test functional runner` passes\n"},
	}
	for _, tc := range accepted {
		body := strings.Replace(base, legacy, tc.line, 1)
		if got := refused(body); got != "" {
			t.Errorf("%s: a spec naming %q was refused: %s", tc.name, strings.TrimSpace(tc.line), got)
		}
	}
	got := refused(strings.Replace(base, legacy, "- [ ] Checked by hand\n", 1))
	if got == "" {
		t.Fatalf("a spec naming no ./le test or ./le verify command was accepted")
	}
	for _, want := range []string{"./le test unit", "./le verify changed"} {
		if !strings.Contains(got, want) {
			t.Errorf("the refusal does not tell the author what to write (%q): %s", want, got)
		}
	}
	for _, full := range []string{"verify current", "verify worktree"} {
		if strings.Contains(got, full) {
			t.Errorf("the refusal tells the author an agent's full run (%q): %s", full, got)
		}
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
	if err := os.MkdirAll(filepath.Join(root, "plan", "post-release", "engineering"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "tmp", "session"), 0o750); err != nil {
		t.Fatal(err)
	}
	spec := "# Spec\n\n| Field | Value |\n|-------|-------|\n| Status | in-progress |\n"
	if err := os.WriteFile(filepath.Join(root, "plan", "post-release", "engineering", "spec-open.md"), []byte(spec), 0o600); err != nil {
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
	if err := os.MkdirAll(filepath.Join(root, "plan", "post-release", "engineering"), 0o750); err != nil {
		t.Fatal(err)
	}

	absent := filepath.Join(root, "plan", "post-release", "engineering", "spec-this-does-not-exist.md")
	code, _, message := runHook(t, root, "validate-spec", map[string]any{
		"tool_name":  "Edit",
		"tool_input": map[string]any{"file_path": absent, "new_string": "x"},
	})
	if code != 2 {
		t.Fatalf("an unreadable spec passed with code = %d, want 2 (message %q)", code, message)
	}
	if !strings.Contains(message, "NOTHING WAS CHECKED") || !strings.Contains(message, "plan/post-release/engineering/spec-this-does-not-exist.md") {
		t.Fatalf("the refusal named neither the failure nor the path: %q", message)
	}

	present := filepath.Join(root, "plan", "post-release", "engineering", "spec-kind.md")
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

// TestStopHookJudgesStateNotWording drives block-premature-stop through its
// entry point with a permission-seeking last message.
//
// VALIDATES: with no claimed spec the stop passes whatever the message says,
// and a claimed spec that is implemented but not closed refuses the stop
// whatever the message says.
// PREVENTS: phrase matching coming back into the Stop hook, and the closure
// block going with it.
func TestStopHookJudgesStateNotWording(t *testing.T) {
	const asking = "Would you like me to continue?"
	const finished = "The commit landed."

	root := t.TempDir()
	code, _, message := runHook(t, root, "block-premature-stop", map[string]any{
		"session_id": "sess-free", "last_assistant_message": asking,
	})
	if code != 0 || message != "" {
		t.Fatalf("no claimed spec: code=%d message=%q, want 0 and silence", code, message)
	}

	closed := t.TempDir()
	writeHookFixture(t, closed, "plan/post-release/engineering/spec-widget.md",
		"| Field | Value |\n|-------|-------|\n| Status | in-progress |\n\n## Goal\n")
	writeHookFixture(t, closed, "plan/learned/900-widget.md", "# Widget\n")
	writeHookFixture(t, closed, "tmp/session/.session-sess-owed", "spec-widget.md\n")
	// The closure inventory reads the journal at HEAD, so the fixture owes one
	// commit. The learned summary is tracked, which is what makes it evidence.
	for _, arguments := range [][]string{
		{"init", "--quiet"},
		{"add", "plan/learned/900-widget.md"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "baseline"},
	} {
		command := exec.CommandContext(t.Context(), "git", arguments...)
		command.Dir = closed
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v in the fixture: %v\n%s", arguments, err, output)
		}
	}
	for _, last := range []string{asking, finished} {
		code, _, message := runHook(t, closed, "block-premature-stop", map[string]any{
			"session_id": "sess-owed", "last_assistant_message": last,
		})
		if code != 2 || !strings.Contains(message, "implemented but not closed") {
			t.Fatalf("unclosed spec, message %q: code=%d message=%q, want the closure block", last, code, message)
		}
	}
}

// TestValidateSpecRefusesUnknownRFCGap holds the refusal of AC-16 at the
// place a spec edit is refused: an `RFC gaps` item that is neither a
// requirement id an rfc/short/ summary lists nor a summary stem is an error
// naming it, while listed ids and stems pass. Method: validateSpecText over a
// tree holding one summary, with the gap cell varied.
func TestValidateSpecRefusesUnknownRFCGap(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "rfc", "short"), 0o750); err != nil {
		t.Fatal(err)
	}
	summary := "- [ ] [SFLOW-V5-3-1] [MUST] An agent MUST send the sample (§3)\n"
	if err := os.WriteFile(filepath.Join(root, "rfc", "short", "sflow-v5.md"), []byte(summary), 0o600); err != nil {
		t.Fatal(err)
	}
	gapErrors := func(cell string) []string {
		body := "# Spec: fixture\n\n| Field | Value |\n|-------|-------|\n| Status | design |\n| Updated | 2026-10-26 |\n| RFC gaps | " + cell + " |\n"
		errors, _ := validateSpecText(root, body)
		var out []string
		for _, problem := range errors {
			if strings.Contains(problem, "RFC gaps") {
				out = append(out, problem)
			}
		}
		return out
	}
	if got := gapErrors("SFLOW-V5-3-1, sflow-v5"); len(got) != 0 {
		t.Errorf("a listed id and a summary stem were refused: %v", got)
	}
	if got := gapErrors("-"); len(got) != 0 {
		t.Errorf("a spec declaring no gap was refused: %v", got)
	}
	got := gapErrors("SFLOW-V5-3-1, RFC2661-5.4-1, rfc7947")
	if len(got) != 2 {
		t.Fatalf("want one refusal per unknown item, got %v", got)
	}
	for index, item := range []string{"RFC2661-5.4-1", "rfc7947"} {
		if !strings.Contains(got[index], "names "+item+", which is neither") || !strings.Contains(got[index], "ls rfc/short/") {
			t.Errorf("refusal %d does not name %s and its fix: %q", index, item, got[index])
		}
	}
}
