package speccitation

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

func TestAnchorAuditSeparatesDeclaredOwnersFromMentions(t *testing.T) {
	// VALIDATES: a source header creates a blocking owner while the generated
	// reverse index creates advisory mentions.
	// PREVENTS: treating every document edge as equally blocking or omitting the
	// source file's stronger declaration.
	root := citationTree(t, map[string]string{
		"ai/CODE-TO-DOCS.md":    "## `pkg/plugin/rpc/`\n\n| File | Docs |\n|------|------|\n| `mux.go` | `docs/architecture/api/ipc.md`, `docs/why-ze.md` |\n",
		"pkg/plugin/rpc/mux.go": "// Design: docs/architecture/api/ipc.md -- wire lifetime\npackage rpc\n",
		"plan/spec-a.md":        "## Files to Modify\n- `pkg/plugin/rpc/mux.go` - lifetime\n",
	})
	report, err := AuditAnchors(root, "plan/spec-a.md")
	if err != nil {
		t.Fatalf("AuditAnchors: %v", err)
	}
	wantOwners := []AnchorFinding{{Document: "docs/architecture/api/ipc.md", Sources: []string{"pkg/plugin/rpc/mux.go"}}}
	wantMentions := []AnchorFinding{{Document: "docs/why-ze.md", Sources: []string{"pkg/plugin/rpc/mux.go"}}}
	if !reflect.DeepEqual(report.Owners, wantOwners) {
		t.Errorf("Owners = %#v, want %#v", report.Owners, wantOwners)
	}
	if !reflect.DeepEqual(report.Mentions, wantMentions) {
		t.Errorf("Mentions = %#v, want %#v", report.Mentions, wantMentions)
	}
}

func TestAnchorAuditAcceptsOwnerNamedAnywhereInSpec(t *testing.T) {
	// VALIDATES: naming a declared document in a checklist explanation satisfies
	// the audit without requiring the document in Files to Modify.
	// PREVENTS: turning a look-and-explain obligation into a forced document edit.
	root := citationTree(t, map[string]string{
		"ai/CODE-TO-DOCS.md": "## `internal/x/`\n\n| File | Docs |\n|------|------|\n| `x.go` | `docs/x.md` |\n",
		"internal/x/x.go":    "// Design: docs/x.md -- x\npackage x\n",
		"plan/spec-a.md":     "## Files to Modify\n- `internal/x/x.go` - x\n\n| 1 | Docs | No | `docs/x.md` is unaffected |\n",
	})
	report, err := AuditAnchors(root, "plan/spec-a.md")
	if err != nil {
		t.Fatalf("AuditAnchors: %v", err)
	}
	if len(report.Owners) != 0 {
		t.Errorf("Owners = %#v, want none", report.Owners)
	}
}

func TestAnchorAuditReadsOnlyTheSourceHeader(t *testing.T) {
	// VALIDATES: Design declarations below the 25-line header block do not own a
	// document for this audit.
	// PREVENTS: a body comment being mistaken for the file header contract.
	root := citationTree(t, map[string]string{
		"ai/CODE-TO-DOCS.md": "## `internal/x/`\n\n| File | Docs |\n|------|------|\n| `x.go` | `docs/indexed.md` |\n",
		"plan/spec-a.md":     "## Files to Modify\n- `internal/x/x.go` - x\n",
	})
	body := ""
	for range 26 {
		body += "\n"
	}
	body += "// Design: docs/late.md -- late\npackage x\n"
	path := filepath.Join(root, "internal", "x", "x.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := AuditAnchors(root, "plan/spec-a.md")
	if err != nil {
		t.Fatalf("AuditAnchors: %v", err)
	}
	if len(report.Owners) != 0 {
		t.Errorf("Owners = %#v, want none", report.Owners)
	}
}

func TestSpecCitationCommandMapsTheAnchorAudit(t *testing.T) {
	// VALIDATES: the grouped native citation command maps the former
	// spec_doc_anchors.py interface to anchors spec <path>.
	// PREVENTS: hook cutover retaining an audit API with no command route.
	root := citationTree(t, map[string]string{
		"ai/CODE-TO-DOCS.md":   "## `internal/x/`\n\n| File | Docs |\n|------|------|\n| `x.go` | `docs/x.md` |\n",
		"internal/x/x.go":      "// Design: docs/x.md -- x\npackage x\n",
		"plan/spec-command.md": "## Files to Modify\n- `internal/x/x.go` - x\n",
	})
	setCitationAnswerRoot(t, root)
	payload, code := Answer([]string{"anchors", "spec", "plan/spec-command.md"})
	report, ok := payload.(AnchorReport)
	if code != 1 {
		t.Fatalf("anchor command = (%#v, %d)", payload, code)
	}
	if !ok {
		t.Fatalf("anchor command payload = %T", payload)
	}
	if len(report.Owners) != 1 {
		t.Fatalf("anchor command = %#v", report)
	}
	if _, code := Answer([]string{"anchors", "plan/spec-command.md"}); code != 2 {
		t.Fatalf("anchor command accepted a path without the spec selector, code %d", code)
	}
}

func setCitationAnswerRoot(t *testing.T, root string) {
	t.Helper()
	old, present := os.LookupEnv("ZE_REPO_ROOT")
	if err := os.Setenv("ZE_REPO_ROOT", root); err != nil {
		t.Fatal(err)
	}
	env.ResetCache()
	t.Cleanup(func() {
		var err error
		if present {
			err = os.Setenv("ZE_REPO_ROOT", old)
		} else {
			err = os.Unsetenv("ZE_REPO_ROOT")
		}
		if err != nil {
			t.Errorf("restore ZE_REPO_ROOT: %v", err)
		}
		env.ResetCache()
	})
}
