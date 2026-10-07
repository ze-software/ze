// Related: render.go -- the page and the card rule these cases drive
//
// VALIDATES: docs/features.md is rendered from the declarations alone: every
// declaration has its row, the status is the vocabulary's label for its pair,
// the description is verbatim, and a new declaration appears with no Go edit.
// The site card rule (D-9) refuses a card no declaration backs.
// PREVENTS: a hand-typed status, a feature missing from the public page, and a
// solid card for a feature that is not complete and supported.

package feature

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/derived"
)

// declarationText writes one declaration from its varying cells.
func declarationText(name, page, kind, scope, level, extra, description string) string {
	var cells strings.Builder
	cells.WriteString("# " + name + "\n\n## Meta\n\n| Field | Value |\n|-------|-------|\n")
	cells.WriteString("| Name | " + name + " |\n")
	if page != "" {
		cells.WriteString("| Page | " + page + " |\n")
	}
	cells.WriteString("| Kind | " + kind + " |\n| Scope | " + scope + " |\n")
	if level != "" {
		cells.WriteString("| Level | " + level + " |\n")
	}
	cells.WriteString(extra)
	cells.WriteString("\n## Description\n\n" + description + "\n")
	return cells.String()
}

// renderFixture is one declaration of each shape the page renders differently.
var renderFixture = map[string]string{
	"zebra": declarationText("Zebra routing", "docs/guide/zebra.md#use", "protocol", "complete", "supported",
		"| Components | internal/zebra |\n", "Zebra routes. <!-- source: internal/zebra/z.go -- Route -->"),
	"aardvark": declarationText("aardvark lookups", "", "protocol", "partial", "experimental",
		"| Components | internal/aardvark |\n| Scope gaps | reverse lookups, bulk export |\n", "Aardvark answers."),
	"tool": declarationText("Perf tool", "website/perf.md", "dev-tool", "complete", "stub-backed",
		"| Components | internal/perf |\n", "Measures throughput."),
	"later": declarationText("Later thing", "", "daemon", "future", "", "", "Planned."),
	"suite": declarationText("Routing suite", "", "umbrella", "partial", "experimental",
		"| Scope gaps | the aardvark gaps |\n| Parts | zebra, aardvark |\n", "Every routing protocol."),
}

func parsedFixture(t *testing.T, texts map[string]string) []Declaration {
	t.Helper()
	declarations := make([]Declaration, 0, len(texts))
	for id, text := range texts {
		declaration, err := Parse(text, id, "features/"+id+".md")
		if err != nil {
			t.Fatalf("fixture %s: %v", id, err)
		}
		declarations = append(declarations, declaration)
	}
	return declarations
}

func TestFeaturesPageRendersEveryDeclaration(t *testing.T) {
	page := RenderPage(parsedFixture(t, renderFixture))

	if !strings.HasPrefix(page, "<!-- GENERATED from features/*.md") {
		t.Errorf("the page carries no generated banner:\n%s", page)
	}
	for _, label := range StatusLabels() {
		bare := strings.TrimSuffix(label, " (partial)")
		if !strings.Contains(page, "`"+bare+"` means ") {
			t.Errorf("the vocabulary sentence does not define %q", bare)
		}
	}
	if !strings.Contains(page, "`(partial)` after a level means "+scopeMeanings[ScopePartial]) {
		t.Error("the vocabulary sentence does not define (partial)")
	}

	for _, row := range []string{
		"| [Zebra routing](guide/zebra.md#use) | Supported | Zebra routes. <!-- source: internal/zebra/z.go -- Route --> |\n",
		"| aardvark lookups | Experimental (partial) | Aardvark answers. **Scope gaps:** reverse lookups; bulk export. |\n",
		"| [Perf tool](../website/perf.md) | Stub-backed | Measures throughput. |\n",
		"| Later thing | Future | Planned. |\n",
		"| Routing suite | Experimental (partial) | Every routing protocol. **Scope gaps:** the aardvark gaps. " +
			"**Parts:** Zebra routing (Supported), aardvark lookups (Experimental (partial)). |\n",
	} {
		if !strings.Contains(page, row) {
			t.Errorf("the page carries no row\n  %q\nin\n%s", row, page)
		}
	}

	// D-11: one table per kind in the vocabulary's kind order, rows by name.
	previous := -1
	for _, marker := range []string{
		"## Protocols", "| aardvark lookups", "| [Zebra routing]",
		"## Daemon services", "| Later thing",
		"## Development tools", "| [Perf tool]",
		"## Umbrellas", "| Routing suite",
	} {
		at := strings.Index(page, marker)
		if at < 0 {
			t.Fatalf("the page carries no %q", marker)
		}
		if at < previous {
			t.Errorf("%q sits above the line declared before it", marker)
		}
		previous = at
	}
	if strings.Contains(page, "## Libraries") {
		t.Error("a kind with no declaration rendered an empty table")
	}
}

// VALIDATES: the registered derived artifact renders the page from a tree, a
// declaration added as one file appears on the next render (AC-23), and only a
// declaration feeds it.
func TestTheDerivedFeaturePageFollowsTheDeclarations(t *testing.T) {
	var artifact *derived.Artifact
	for _, candidate := range derived.All() {
		if candidate.Path == pageRel {
			artifact = &candidate
		}
	}
	if artifact == nil {
		t.Fatalf("no derived artifact is registered for %s", pageRel)
	}

	tree := t.TempDir()
	writeFile(t, tree, "features/zebra.md", renderFixture["zebra"])
	if err := artifact.Rebuild(tree); err != nil {
		t.Fatal(err)
	}
	writeFile(t, tree, "features/later.md", renderFixture["later"])
	if err := artifact.Rebuild(tree); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(pageRel)))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []string{"| [Zebra routing](guide/zebra.md#use) | Supported |", "| Later thing | Future |"} {
		if !strings.Contains(string(page), row) {
			t.Errorf("the rendered page carries no %q", row)
		}
	}

	writeFile(t, tree, "features/broken.md", "# Broken\n")
	if err := artifact.Rebuild(tree); err == nil {
		t.Error("a malformed declaration rendered a page without it")
	}

	for path, feeds := range map[string]bool{
		"features/zebra.md":        true,
		"features/runs/zebra.json": false,
		"features/runs/zebra.md":   false,
		"docs/features.md":         false,
		"internal/le/feature/x.md": false,
	} {
		if got := artifact.Feeds(tree, path); got != feeds {
			t.Errorf("Feeds(%s) = %v, want %v", path, got, feeds)
		}
	}

	empty := t.TempDir()
	if err := artifact.Rebuild(empty); err != nil {
		t.Errorf("a tree with no features/ refused: %v", err)
	}
	if _, err := os.Stat(filepath.Join(empty, filepath.FromSlash(pageRel))); !os.IsNotExist(err) {
		t.Errorf("a tree with no features/ was given a page: %v", err)
	}
}

// VALIDATES: D-9, solid exactly when every named feature is complete and
// supported; a card no declaration backs is refused by name (AC-18, AC-19).
func TestASiteCardIsShippedOnlyWhenEveryFeatureItNamesIs(t *testing.T) {
	byID := ByID(parsedFixture(t, renderFixture))
	for _, tc := range []struct {
		ids     []string
		shipped bool
	}{
		{[]string{"zebra"}, true},
		{[]string{"zebra", "aardvark"}, false},
		{[]string{"aardvark"}, false},
		{[]string{"tool"}, false},
		{[]string{"zebra", "later"}, false},
	} {
		shipped, err := CardShipped(byID, tc.ids)
		if err != nil {
			t.Fatalf("%v: %v", tc.ids, err)
		}
		if shipped != tc.shipped {
			t.Errorf("%v shipped = %v, want %v", tc.ids, shipped, tc.shipped)
		}
	}
	for _, tc := range []struct {
		ids  []string
		says string
	}{
		{nil, "names no feature"},
		{[]string{"zebra", "unicorn"}, "unicorn"},
		{[]string{"later"}, "only future or rejected"},
	} {
		if _, err := CardShipped(byID, tc.ids); err == nil || !strings.Contains(err.Error(), tc.says) {
			t.Errorf("%v: refusal %v, want one naming %q", tc.ids, err, tc.says)
		}
	}
	if CardLabel(true) != "" {
		t.Error("a shipped card carries a badge")
	}
	if CardLabel(false) != StatusLabel(ScopeComplete, LevelExperimental) {
		t.Errorf("a dashed card's badge is %q", CardLabel(false))
	}
}
