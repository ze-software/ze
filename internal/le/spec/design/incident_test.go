// VALIDATES: the 2026-11-12 incident, replayed: the pipe-chain spec text of
// commit 2616478e64, set to ready, is stopped at every gate it passed that day:
// the derivation, the check, the claim, a subagent's move to ready, the edit
// of pipe.go before the decision row was read, and the commit that rewrote
// the page's decision (plan/release/validation/spec-design-needs-owner-approval.md,
// AC-36). The hook and the commit flow import this package; this external
// test package imports them back, which Go allows and the package itself
// must not do.
// PREVENTS: a design that contradicts an owner decision reaching
// implementation without Thomas, which is how the daemon came to refuse a
// pipe chain.
package specdesign_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/commit"
	"github.com/ze-software/ze/internal/le/hookruntime"
	"github.com/ze-software/ze/internal/le/spec"
	specdesign "github.com/ze-software/ze/internal/le/spec/design"
	"github.com/ze-software/ze/internal/le/spec/design/designtest"
)

// The incident's files, as the tree held them, reduced to their headers.
const (
	incidentSpec     = "plan/release/fixes/spec-plugin-dispatch-pipe-chain-reaches-the-handler.md"
	incidentCommands = "docs/architecture/api/commands.md"
	incidentProtocol = "docs/architecture/api/process-protocol.md"
	incidentPipe     = "internal/component/command/pipe.go"
	incidentDispatch = "internal/component/plugin/server/command.go"
	incidentDecision = "A command path names the subject and stops; scope, presentation and format are pipe " +
		"operations after it, and the pipe wins over a subcommand"
)

// incidentRoot lays out the incident checkout: the spec from testdata, the two
// files it changed with their // Design: headers, commands.md carrying the
// 2026-08-21 decision row, process-protocol.md, the doors and the records.
func incidentRoot(t *testing.T) string {
	t.Helper()
	text, err := os.ReadFile(filepath.Join("testdata", "incident-pipe-chain.md"))
	if err != nil {
		t.Fatalf("read the incident spec: %v", err)
	}
	if !strings.Contains(string(text), "| Status | ready |") {
		t.Fatal("the incident testdata is not in Status ready")
	}
	root := t.TempDir()
	// An empty history, so the claim's approval lookup reads no trailer
	// rather than failing, and the refusal judged is the claim's.
	if err := designtest.InitRepository(root); err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, map[string]string{
		incidentSpec:     string(text),
		incidentPipe:     "// Design: " + incidentCommands + " -- pipe operators\n\npackage command\n",
		incidentDispatch: "// Design: " + incidentProtocol + " -- dispatch\n\npackage server\n",
		incidentCommands: incidentPageText,
		incidentProtocol: "# Process protocol\n",
		"plan/records/audit-command-pipe-vs-subcommand.md": "# Audit\n",
		"plan/records/audit-presentation-pipes.md":         "# Audit\n",
		"ai/doors/system.md":                               "# System door\n\n<!-- door-base: pkg cmd/ze internal -->\n",
		"ai/doors/commands-cli.md":                         "# Door\n\n<!-- door-base: internal/component/command -->\n",
		"ai/doors/plugin-infra.md":                         "# Door\n\n<!-- door-base: internal/component/plugin pkg/plugin pkg/ze -->\n",
	})
	useRoot(t, root)
	return root
}

// TestIncidentPipeChainSpecIsRefused is the wiring row "The incident,
// replayed". Method: over the incident checkout, the derivation lists both
// design pages, the doors and the decision row; the check of a spec whose
// table omits them fails naming the decision; the claim of the ready spec is
// refused and leaves it ready.
func TestIncidentPipeChainSpecIsRefused(t *testing.T) {
	root := incidentRoot(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "incident-replay")

	answer, code := specdesign.Answer([]string{"documents", "spec", incidentSpec})
	if code != 0 {
		t.Fatalf("documents over the incident spec answered (%#v, %d), want exit 0", answer, code)
	}
	requireContains(t, "the incident derivation", answerText(t, answer),
		incidentCommands, incidentProtocol, "ai/doors/system.md", "ai/doors/commands-cli.md", "ai/doors/plugin-infra.md", incidentDecision)

	var checkAnswer any
	stderr := captureStderr(t, func() {
		checkAnswer, code = specdesign.Answer([]string{"check", "spec", incidentSpec})
	})
	if code != 1 {
		t.Fatalf("check over the incident spec answered %d, want 1:\n%s", code, stderr)
	}
	requireContains(t, "the incident check", answerText(t, checkAnswer)+stderr,
		"## Design documents", incidentCommands, "2026-08-21")

	claimAnswer, code := spec.Claim([]string{incidentSpec})
	if code != 2 {
		t.Fatalf("the claim of the unapproved incident spec answered %d, want 2", code)
	}
	claim, ok := claimAnswer.(spec.ClaimReport)
	if !ok {
		t.Fatalf("the refused claim answered %#v, want the claim report carrying the refusal", claimAnswer)
	}
	requireContains(t, "the incident claim", claim.Text(),
		"refusing to start spec-plugin-dispatch-pipe-chain-reaches-the-handler.md: its design has no owner approval")
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(incidentSpec)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "| Status | ready |") {
		t.Fatal("the refused claim moved the incident spec out of ready")
	}

	// The subagent's move to ready, replayed from Status design (AC-20).
	designed := strings.Replace(string(body), "| Status | ready |", "| Status | design |", 1)
	writeTree(t, root, map[string]string{incidentSpec: designed})
	code, message, found := hookruntime.Probe("writeSpecReady", hookruntime.Payload{
		SessionID: "incident-replay", ToolName: "Edit", AgentID: "incident-designer",
		ToolInput: map[string]any{"file_path": filepath.Join(root, filepath.FromSlash(incidentSpec)),
			"old_string": "| Status | design |", "new_string": "| Status | ready |"},
	}, root)
	if !found || code != 2 || !strings.Contains(message, "a subagent cannot move a spec to ready") {
		t.Errorf("the subagent's ready edit answered (%d, %q, found %v), want the refusal", code, message, found)
	}

	// The implementing context's edit of pipe.go, before it read the decision
	// row its // Design: page carries (AC-1).
	code, message, found = hookruntime.Probe("writeDesignDoorsRead", hookruntime.Payload{
		SessionID: "incident-replay", ToolName: "Edit",
		ToolInput: map[string]any{"file_path": filepath.Join(root, filepath.FromSlash(incidentPipe)),
			"old_string": "package command", "new_string": "package command // the dispatcher refuses a pipe chain"},
	}, root)
	if !found || code != 2 {
		t.Errorf("the edit of pipe.go before the decision was read answered (%d, %q, found %v), want exit 2", code, message, found)
	}
	requireContains(t, "the pipe.go refusal", message, incidentCommands, "Design decisions")

	// The commit that rewrote the page's decision to fit the design (AC-10).
	incidentCommit(t, root)
	writeTree(t, root, map[string]string{incidentCommands: strings.Replace(incidentPageText,
		incidentDecision, "The dispatcher refuses a pipe chain; the surface splits it", 1)})
	if _, err := commit.Create(root, &commit.Options{Subject: "commands: the dispatcher refuses a pipe chain",
		Files: []string{incidentCommands}}); err == nil || !strings.Contains(err.Error(), "the decision of 2026-08-21 is changed") {
		t.Errorf("the commit rewriting the 2026-08-21 decision answered %v, want the decision-row refusal", err)
	}
}

// incidentPageText is commands.md as the incident checkout holds it: the
// 2026-08-21 decision row in its Design decisions section.
const incidentPageText = "# Commands\n\n## Design decisions\n\n| Decision | It forbids | Source | Date |\n|---|---|---|---|\n" +
	"| " + incidentDecision + " | Refusing a pipe a command declares | " +
	"plan/records/audit-command-pipe-vs-subcommand.md, plan/records/audit-presentation-pipes.md | 2026-08-21 |\n"

// incidentCommit makes the incident checkout a Ze checkout whose HEAD holds
// every file it carries, so the page commit is judged against that parent.
func incidentCommit(t *testing.T, root string) {
	t.Helper()
	writeTree(t, root, map[string]string{"go.mod": "module fixture\n", "feature-gates.txt": ""})
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "the incident checkout"}} {
		if err := designtest.Git(root, args...); err != nil {
			t.Fatal(err)
		}
	}
}
