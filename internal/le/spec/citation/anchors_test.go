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

// TestTheAuditReportsEveryDocumentOfABulletRenderedPath judges the CONSUMER
// over this checkout, on the shape the deleted parse could not see.
//
// VALIDATES: a spec naming a source file whose package renders as BULLETS in
// ai/CODE-TO-DOCS.md is judged against every document that anchors that file,
// over the real corpus rather than over a hand-built tree.
// PREVENTS: the blind spot a rendered index creates. renderCodeIndex writes a
// package of at most namedInline files as bullets and a larger one as a table
// (internal/le/docstocode/codetodocs_report.go), and the parse this audit used
// matched the table alone, so 674 of the tree's 2,537 code paths were invisible
// to it.
//
// It does NOT compare the audit's index against the generator's map. Those are
// ONE call now (loadDocumentIndex), so a test that compared them would compare a
// function with itself and would pass for any population, an empty one included.
// The consumer is where a claim about the population can still be checked.
func TestTheAuditReportsEveryDocumentOfABulletRenderedPath(t *testing.T) {
	root := repoRoot(t)

	model, err := docstocode.DocumentsByPath(root)
	if err != nil {
		t.Fatalf("build the reverse index: %v", err)
	}
	source, documents := bulletRenderedPath(t, root, model)

	specPath := filepath.Join(t.TempDir(), "spec-bullet-rendered-path.md")
	// The spec names the SOURCE and no document, so every document that anchors
	// it is owed as a finding: AuditAnchors drops one the spec already names.
	body := "## Files to Modify\n- `" + source + "` - the case under test\n"
	if err := os.WriteFile(specPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	report, err := AuditAnchors(root, specPath)
	if err != nil {
		t.Fatalf("AuditAnchors: %v", err)
	}

	reported := map[string]bool{}
	for _, finding := range slices.Concat(report.Owners, report.Mentions) {
		if slices.Contains(finding.Sources, source) {
			reported[finding.Document] = true
		}
	}
	for _, document := range documents {
		if !reported[document] {
			t.Errorf("%s anchors %s and the audit named neither it nor the file's own owner; "+
				"reported %v", document, source, sortedSet(reported))
		}
	}
}

// bulletRenderedPath answers one real code path the index renders as a bullet,
// with every document that anchors it.
//
// The choice is the FIRST such path in sorted order, so a failure names the same
// case on every machine. A tree that offers none fails rather than passes: this
// test's whole subject is that shape, and a silent skip would report a corpus
// fact as a green assertion (ai/rules/principles.md).
func bulletRenderedPath(t *testing.T, root string, model map[string][]string) (string, []string) {
	t.Helper()

	// namedInline, the bound renderCodeIndex switches shape at, restated here
	// because it is unexported in the package that owns it. A change there
	// widens the bullet population and can only make this case easier to find.
	const namedInline = 3

	inPackage := map[string]int{}
	for path := range model {
		inPackage[packageOf(path)]++
	}
	for _, path := range sortedKeysOf(model) {
		if inPackage[packageOf(path)] > namedInline {
			continue
		}
		if !slices.ContainsFunc(sourcePrefixes[:], func(prefix string) bool {
			return strings.HasPrefix(path, prefix)
		}) {
			continue
		}
		if !slices.ContainsFunc(sourceSuffixes[:], func(suffix string) bool {
			return strings.HasSuffix(path, suffix)
		}) {
			continue
		}
		if len(model[path]) == 0 {
			continue
		}
		// The document must come from the INDEX and not from the file's own
		// `// Design:` header. AuditAnchors builds report.Owners from
		// declaredDesignDocument, which never reads the index, so a path whose
		// only anchor IS its declared owner is reported either way and proves
		// nothing about the index. Picking one would leave this test green under
		// the table-only blind spot it exists to catch, with no edit to it.
		owner := declaredDesignDocument(root, path)
		if !slices.ContainsFunc(model[path], func(document string) bool { return document != owner }) {
			continue
		}
		return path, model[path]
	}
	t.Fatal("no anchored code path in this checkout sits in a package the index renders as bullets, " +
		"so the shape this test exists for was never exercised")
	return "", nil
}

// packageOf is the directory part of a code path, which is what the renderer
// groups by (packageDir, internal/le/docstocode/codetodocs.go).
func packageOf(path string) string {
	if index := strings.LastIndex(path, "/"); index >= 0 {
		return path[:index]
	}
	return path
}

func sortedKeysOf(model map[string][]string) []string {
	keys := make([]string, 0, len(model))
	for key := range model {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func sortedSet(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
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
