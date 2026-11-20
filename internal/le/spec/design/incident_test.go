// VALIDATES: the 2026-11-12 incident, replayed: the pipe-chain spec text of
// commit 2616478e64, set to ready, is stopped at the derivation, the check and
// the claim (plan/release/validation/spec-design-needs-owner-approval.md,
// AC-36). The code-edit refusal and the page-commit refusal of the same AC
// are proven at their own entry points, by TestDesignPageGateOwesTheDecisionsSection
// (internal/le/hookruntime) and TestCommitRefusesChangedDecisionRow
// (internal/le/doc/consistency), which this package must not import.
// PREVENTS: a design that contradicts an owner decision reaching
// implementation without Thomas, which is how the daemon came to refuse a
// pipe chain.
package specdesign_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/spec"
	specdesign "github.com/ze-software/ze/internal/le/spec/design"
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
	writeTree(t, root, map[string]string{
		incidentSpec:     string(text),
		incidentPipe:     "// Design: " + incidentCommands + " -- pipe operators\n\npackage command\n",
		incidentDispatch: "// Design: " + incidentProtocol + " -- dispatch\n\npackage server\n",
		incidentCommands: "# Commands\n\n## Design decisions\n\n| Decision | It forbids | Source | Date |\n|---|---|---|---|\n" +
			"| " + incidentDecision + " | Refusing a pipe a command declares | " +
			"plan/records/audit-command-pipe-vs-subcommand.md, plan/records/audit-presentation-pipes.md | 2026-08-21 |\n",
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

	if _, code := spec.Claim([]string{incidentSpec}); code != 2 {
		t.Fatalf("the claim of the unapproved incident spec answered %d, want 2", code)
	}
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(incidentSpec)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "| Status | ready |") {
		t.Fatal("the refused claim moved the incident spec out of ready")
	}
}
