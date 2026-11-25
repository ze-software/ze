// Design: docs/architecture/core-design.md -- native hook policy regressions
// Related: systemtmp.go -- namesSystemTmp blanking reader here-document bodies
// Related: subagent_heavy.go -- bashSubagentHeavy blanking reader here-document bodies
// Related: writeedit.go -- writeFilePatterns golangci-lint v2 exclusions and warning labels
package hookruntime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/ai/rules/ruleset"
)

// TestReaderHeredocBodyNotMisreadBySystemTmpAndSubagentHeavy proves that
// saving a prose report via a quoted here-document (`cat > ... <<'EOF'`) does
// not trip bashSystemTmp or bashSubagentHeavy when the body mentions `/tmp` or
// `go build`, while real `/tmp` redirects, expanding substitutions, and
// non-reader here-documents remain refused.
func TestReaderHeredocBodyNotMisreadBySystemTmpAndSubagentHeavy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	idLine := identityLine("agent-heredoc", "parent-heredoc") + "\n"
	reportWrite := idLine + "cat > tmp/session/2026-11-22-sid/scratch/report.md <<'EOF'\n" +
		"Owed commands:\n" +
		"go build ./...\n" +
		"Avoid writing under /tmp/foo.\n" +
		"EOF"
	if code, message, found := Probe("bashSystemTmp", Payload{
		ToolName:  toolBash,
		ToolInput: map[string]any{"command": reportWrite},
		AgentID:   "agent-heredoc",
		SessionID: "parent-heredoc",
	}, root); !found || code != 0 {
		t.Fatalf("bashSystemTmp on reader heredoc = (%d, %q, found=%v), want exit 0", code, message, found)
	}
	if code, message, found := Probe("bashSubagentHeavy", Payload{
		ToolName:  toolBash,
		ToolInput: map[string]any{"command": reportWrite},
		AgentID:   "agent-heredoc",
		SessionID: "parent-heredoc",
	}, root); !found || code != 0 {
		t.Fatalf("bashSubagentHeavy on reader heredoc = (%d, %q, found=%v), want exit 0", code, message, found)
	}

	// Negative cases: writing TO /tmp, running bash on a heredoc, or expanding
	// a command substitution inside a heredoc must still be refused.
	badTmp := idLine + "cat > /tmp/out.md <<'EOF'\nhello\nEOF"
	if code, _, found := Probe("bashSystemTmp", Payload{
		ToolName:  toolBash,
		ToolInput: map[string]any{"command": badTmp},
		AgentID:   "agent-heredoc",
		SessionID: "parent-heredoc",
	}, root); !found || code != 2 {
		t.Fatalf("bashSystemTmp on > /tmp/out.md = (%d, found=%v), want exit 2", code, found)
	}

	badBash := idLine + "bash <<'EOF'\ngo test ./...\nEOF"
	if code, _, found := Probe("bashSubagentHeavy", Payload{
		ToolName:  toolBash,
		ToolInput: map[string]any{"command": badBash},
		AgentID:   "agent-heredoc",
		SessionID: "parent-heredoc",
	}, root); !found || code != 2 {
		t.Fatalf("bashSubagentHeavy on bash heredoc = (%d, found=%v), want exit 2", code, found)
	}

	badSubst := idLine + "cat > tmp/session/2026-11-22-sid/scratch/report.md <<EOF\n$(go test ./...)\nEOF"
	if code, _, found := Probe("bashSubagentHeavy", Payload{
		ToolName:  toolBash,
		ToolInput: map[string]any{"command": badSubst},
		AgentID:   "agent-heredoc",
		SessionID: "parent-heredoc",
	}, root); !found || code != 2 {
		t.Fatalf("bashSubagentHeavy on expanding heredoc = (%d, found=%v), want exit 2", code, found)
	}
}

// TestSubagentMayRunGoplsCheck proves that `gopls check` is permitted for a
// subagent so ze-work can type-check edited Go files before handing back owed
// commands.
func TestSubagentMayRunGoplsCheck(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := identityLine("agent-gopls", "parent-gopls") + "\ngopls check internal/le/hookruntime/systemtmp.go"
	if code, message, found := Probe("bashSubagentHeavy", Payload{
		ToolName:  toolBash,
		ToolInput: map[string]any{"command": cmd},
		AgentID:   "agent-gopls",
		SessionID: "parent-gopls",
	}, root); !found || code != 0 {
		t.Fatalf("bashSubagentHeavy on gopls check = (%d, %q, found=%v), want exit 0", code, message, found)
	}
}

// TestWriteFilePatternsBlocksGolangciV2Exclusions proves that writeFilePatterns
// catches golangci-lint v2 `exclusions:` entries in .golangci.yml and does not
// misfire on Go ruleguard files under .golangci/ruleguard/.
func TestWriteFilePatternsBlocksGolangciV2Exclusions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	v2Edit := "linters:\n  exclusions:\n    rules:\n      - linters: [gosec]\n"
	if code, _, found := Probe("writeFilePatterns", Payload{
		ToolName: "Edit",
		ToolInput: map[string]any{
			"file_path":  filepath.Join(root, ".golangci.yml"),
			"new_string": v2Edit,
		},
	}, root); !found || code != 2 {
		t.Fatalf("writeFilePatterns on .golangci.yml v2 exclusions = (%d, found=%v), want exit 2", code, found)
	}

	ruleguardFile := "package ruleguard\n// exclusions: comment in Go file\n"
	if code, msg, found := Probe("writeFilePatterns", Payload{
		ToolName: "Edit",
		ToolInput: map[string]any{
			"file_path":  filepath.Join(root, ".golangci", "ruleguard", "modern.go"),
			"new_string": ruleguardFile,
		},
	}, root); !found || code != 0 {
		t.Fatalf("writeFilePatterns on ruleguard Go file = (%d, %q, found=%v), want exit 0", code, msg, found)
	}
}

// TestExitOneWarningsSayWarnNotBlocked proves that non-blocking exit-1 hooks
// in writeGoPatterns and writeFilePatterns print WARN rather than BLOCKED.
func TestExitOneWarningsSayWarnNotBlocked(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	code, msg, found := Probe("writeGoPatterns", Payload{
		ToolName: toolWrite,
		ToolInput: map[string]any{
			"file_path": filepath.Join(root, "internal", "x", "example.go"),
			"content":   "package x\n\nfunc ParseAndValidate() {}\n",
		},
	}, root)
	if !found || code != 1 {
		t.Fatalf("writeGoPatterns And-name = (%d, %q, found=%v), want exit 1", code, msg, found)
	}
	if strings.Contains(msg, "BLOCKED") || !strings.Contains(msg, "WARN:") {
		t.Fatalf("exit-1 warning message = %q, want WARN without BLOCKED", msg)
	}
}

// TestSubagentStartAvoidsDuplicateHardBansWhenDefinitionCarriesThem proves
// that SubagentStart does not re-inject the verbatim Hard Bans block when the
// agent definition already embeds the verified hard-bans marker, while still
// injecting Hard Bans for an omitting definition that lacks the marker.
func TestSubagentStartAvoidsDuplicateHardBansWhenDefinitionCarriesThem(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const uniqueBan = "Never run the duplicate-ban-probe verb."
	files := map[string]string{
		filepath.Join(ruleset.AgentDefinitionsDir, "ze-inline.md"): "---\nname: ze-inline\nomitClaudeMd: true\n---\n<!-- hard-bans: begin -->\n# Hard Bans\n- " + uniqueBan + "\n<!-- hard-bans: end -->\n",
		filepath.Join(ruleset.AgentDefinitionsDir, "ze-plain.md"):  "---\nname: ze-plain\nomitClaudeMd: true\n---\nBody.\n",
		ruleset.InstructionsFile:                                   "# Ze\n\n# " + ruleset.HardBansHeading + "\n\n- " + uniqueBan + "\n",
		ruleset.DecisionsFile:                                      "# Decisions\n\n# " + ruleset.DecisionsAgentPartHeading + "\n- A.\n\n# " + ruleset.DecisionsMainPartHeading + "\n- B.\n",
		ruleset.SituationalDecisionsFile:                           "# " + ruleset.DecisionsSituationalPartHeading + "\n\n## Lab\n" + ruleset.SituationPrefix + "running a lab.\n- C.\n",
	}
	for rel, body := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	readContext := func(agentType string) string {
		t.Helper()
		code, out, found := ProbeLifecycle("subagent-context", Payload{
			SessionID: "parent-dedup",
			AgentID:   "agent-dedup",
			AgentType: agentType,
		}, root)
		if !found || code != 0 {
			t.Fatalf("subagent-context(%s) = (%d, %q, found=%v), want exit 0", agentType, code, out, found)
		}
		var resp struct {
			Output struct {
				Context string `json:"additionalContext"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal([]byte(out), &resp); err != nil {
			t.Fatalf("unmarshal(%s): %v", agentType, err)
		}
		return resp.Output.Context
	}

	inlineCtx := readContext("ze-inline")
	if strings.Contains(inlineCtx, uniqueBan) {
		t.Errorf("ze-inline received duplicate verbatim Hard Bans:\n%s", inlineCtx)
	}
	if !strings.Contains(inlineCtx, "already carries the verified") {
		t.Errorf("ze-inline missing already-carries notice:\n%s", inlineCtx)
	}

	plainCtx := readContext("ze-plain")
	if !strings.Contains(plainCtx, uniqueBan) {
		t.Errorf("ze-plain missing injected Hard Bans:\n%s", plainCtx)
	}
}

// VALIDATES: plan/templates/TEMPLATE-FIX.md passes validateSpecText with zero
// errors as a skeleton and when promoted to design with real fields.
// PREVENTS: shipping a compact fix spec template that hookValidateSpec rejects.
func TestTemplateFixValidatesCleanly(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("abs root: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "plan", "templates", "TEMPLATE-FIX.md"))
	if err != nil {
		t.Fatalf("read TEMPLATE-FIX.md: %v", err)
	}
	errs, _ := validateSpecText(root, string(raw))
	if len(errs) != 0 {
		t.Fatalf("TEMPLATE-FIX.md skeleton failed validateSpecText: %v", errs)
	}
	designText := strings.Replace(string(raw), "| Status | skeleton |", "| Status | design |", 1)
	designText = strings.Replace(designText, "| Updated | YYYY-MM-DD |", "| Updated | 2026-11-22 |", 1)
	designText = strings.Replace(designText, "- `internal/...` - [producing fix]", "- `internal/le/hookruntime/systemtmp.go` - blank reader heredoc body", 1)
	errs, warns := validateSpecText(root, designText)
	if len(errs) != 0 {
		t.Fatalf("TEMPLATE-FIX.md in design status failed validateSpecText: %v", errs)
	}
	if len(warns) != 0 {
		t.Fatalf("TEMPLATE-FIX.md in design status produced warnings: %v", warns)
	}
}
