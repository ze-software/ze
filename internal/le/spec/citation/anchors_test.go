package speccitation

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/docstocode"
	"github.com/ze-software/ze/internal/le/lepath"
)

func TestAnchorAuditSeparatesDeclaredOwnersFromMentions(t *testing.T) {
	// VALIDATES: a source header creates a blocking owner while the generated
	// reverse index creates advisory mentions.
	// PREVENTS: treating every document edge as equally blocking or omitting the
	// source file's stronger declaration.
	root := citationTree(t, map[string]string{
		"docs/architecture/api/ipc.md": "<!-- source: pkg/plugin/rpc/mux.go -- Mux -->\n",
		"docs/why-ze.md":               "<!-- source: pkg/plugin/rpc/mux.go -- Mux -->\n",
		"pkg/plugin/rpc/mux.go":        "// Design: docs/architecture/api/ipc.md -- wire lifetime\npackage rpc\n",
		"plan/spec-a.md":               "## Files to Modify\n- `pkg/plugin/rpc/mux.go` - lifetime\n",
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
		"docs/x.md":       "<!-- source: internal/x/x.go -- X -->\n",
		"internal/x/x.go": "// Design: docs/x.md -- x\npackage x\n",
		"plan/spec-a.md":  "## Files to Modify\n- `internal/x/x.go` - x\n\n| 1 | Docs | No | `docs/x.md` is unaffected |\n", // <!-- doc-links: ignore (a synthetic path in a test fixture; these tests are about dead-path detection, so the paths must not resolve) -->
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
		"docs/indexed.md": "<!-- source: internal/x/x.go -- X -->\n",
		"plan/spec-a.md":  "## Files to Modify\n- `internal/x/x.go` - x\n", // <!-- doc-links: ignore (a synthetic path in a test fixture; these tests are about dead-path detection, so the paths must not resolve) -->
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
		"docs/x.md":            "<!-- source: internal/x/x.go -- X -->\n",
		"internal/x/x.go":      "// Design: docs/x.md -- x\npackage x\n",
		"plan/spec-command.md": "## Files to Modify\n- `internal/x/x.go` - x\n", // <!-- doc-links: ignore (a synthetic path in a test fixture; these tests are about dead-path detection, so the paths must not resolve) -->
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

// repoRoot answers the checkout the real-tree cases below measure.
func repoRoot(t *testing.T) string {
	t.Helper()

	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the checkout root: %v", err)
	}
	return root
}

// TestTheDocumentIndexHoldsEveryCodePath judges the map this package reads
// against the map the generator holds, over this checkout.
//
// VALIDATES: the audit sees every code path the documentation anchors, with the
// same documents against each one.
// PREVENTS: the blind spot a rendered index creates. ai/CODE-TO-DOCS.md renders
// a package of three files or fewer as bullets and a larger one as a table, and
// the parse matched the table alone, so 631 of 2,533 entries were invisible to
// a reader that had no way to know it was reading a quarter short.
func TestTheDocumentIndexHoldsEveryCodePath(t *testing.T) {
	root := repoRoot(t)

	model, err := docstocode.DocumentsByPath(root)
	if err != nil {
		t.Fatalf("build the reverse index: %v", err)
	}
	// A generator answering nothing would agree with an empty index, so it is
	// refused rather than read as agreement.
	if len(model) == 0 {
		t.Fatal("the generator answered no code path at all, so nothing was compared")
	}

	index, err := loadDocumentIndex(root)
	if err != nil {
		t.Fatalf("load the document index: %v", err)
	}
	if len(index) != len(model) {
		t.Errorf("the audit reads %d code path(s); the generator holds %d", len(index), len(model))
	}

	var missing, different []string
	for path, documents := range model {
		held, present := index[path]
		if !present {
			missing = append(missing, path)
			continue
		}
		if !reflect.DeepEqual(held, documents) {
			different = append(different, path)
		}
	}
	slices.Sort(missing)
	slices.Sort(different)
	if len(missing) != 0 {
		t.Errorf("%d code path(s) the generator holds are absent from the audit's index, first %s",
			len(missing), strings.Join(missing[:min(3, len(missing))], " "))
	}
	if len(different) != 0 {
		t.Errorf("%d code path(s) carry different documents in the two maps, first %s",
			len(different), strings.Join(different[:min(3, len(different))], " "))
	}
}

// VALIDATES: a spec naming a source file that lives in a package of three files
// or fewer is judged against the document its own header declares AND against
// every other document that anchors it.
// PREVENTS: the blind spot a rendered index created. renderCodeIndex writes a
// package of at most three files as bullets and a larger one as a table, and
// the parse this audit used matched the table alone, so no file in a small
// package ever reached the mentions half. Every other case in this file builds a
// package of one or two files and every one of them passed, because each
// hand-wrote a table the renderer would never have emitted for it.
func TestTheAnchorGuardSeesAFileInASmallPackage(t *testing.T) {
	root := citationTree(t, map[string]string{
		// Three files in one package: the bullet shape, exactly at the bound.
		"docs/small-owner.md":     "<!-- source: internal/small/one.go -- One -->\n",
		"docs/small-other.md":     "<!-- source: internal/small/one.go, two.go, three.go -- the package -->\n",
		"internal/small/one.go":   "// Design: docs/small-owner.md -- one\npackage small\n",
		"internal/small/two.go":   "package small\n",
		"internal/small/three.go": "package small\n",
		"plan/spec-small.md":      "## Files to Modify\n- `internal/small/one.go` - one\n",
	})

	report, err := AuditAnchors(root, "plan/spec-small.md")
	if err != nil {
		t.Fatalf("AuditAnchors: %v", err)
	}
	wantOwners := []AnchorFinding{{Document: "docs/small-owner.md", Sources: []string{"internal/small/one.go"}}}
	wantMentions := []AnchorFinding{{Document: "docs/small-other.md", Sources: []string{"internal/small/one.go"}}}
	if !reflect.DeepEqual(report.Owners, wantOwners) {
		t.Errorf("Owners = %#v, want %#v", report.Owners, wantOwners)
	}
	if !reflect.DeepEqual(report.Mentions, wantMentions) {
		t.Errorf("Mentions = %#v, want %#v", report.Mentions, wantMentions)
	}
}
