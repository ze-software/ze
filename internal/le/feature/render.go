// Design: docs/contributing/feature-maturity.md -- the declaration model and the page it renders
// Related: vocabulary.go -- the labels, meanings and kind headings the page states
// Related: declaration.go -- Load, the one parser of features/*.md
//
// docs/features.md and the site's feature cards both read their maturity from
// here. The page is DERIVED: register.go registers it with internal/le/derived,
// so it is rendered from the declarations and never authored or tracked.

package feature

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/derived"
)

// pageRel is the rendered feature table, relative to the checkout root.
const pageRel = "docs/features.md"

// pageDocsDir is the directory pageRel sits in. A Page field is a repository
// path, and the row links it relative to the page.
const pageDocsDir = "docs/"

// pageIntro is the product sentence the page opens with. It states what Ze is,
// not how mature any part of it is, so it is the page's own prose.
const pageIntro = "Ze is an open-source configuration and protocol engine written in Go. " +
	"The network operating system built on it runs on any Linux or as a gokrazy appliance."

// Declared loads every declaration, and refuses a tree holding a malformed one.
//
// A reader that publishes (the page, the site) cannot skip a declaration the
// parser refused: the feature would vanish from the public surface with nothing
// saying so. The check collects the refusals instead; a publisher stops on them.
func Declared(tree string) ([]Declaration, error) {
	declarations, problems, err := Load(tree)
	if err != nil {
		return nil, err
	}
	if len(problems) != 0 {
		return nil, errors.Join(problems...)
	}
	return declarations, nil
}

// ByID indexes declarations by id. The pointers address the slice it was given.
func ByID(declarations []Declaration) map[string]*Declaration {
	out := make(map[string]*Declaration, len(declarations))
	for index := range declarations {
		out[declarations[index].ID] = &declarations[index]
	}
	return out
}

// CardShipped answers the D-9 rule for one site card naming ids: the card is
// solid (shipped) exactly when every feature it names is complete in scope and
// supported in level, and dashed (experimental) otherwise.
//
// A card naming no feature, an id no declaration holds, or only future and
// rejected features is refused: each would publish a card no declaration backs.
func CardShipped(byID map[string]*Declaration, ids []string) (bool, error) {
	if len(ids) == 0 {
		return false, errors.New("names no feature declaration")
	}
	shipped := true
	implemented := false
	for _, id := range ids {
		declaration, known := byID[id]
		if !known {
			return false, fmt.Errorf("names feature %q, which no %s/%s.md declares", id, declarationDir, id)
		}
		if declaration.Scope.Implemented() {
			implemented = true
		}
		if declaration.Scope != ScopeComplete {
			shipped = false
		}
		if declaration.Level != LevelSupported {
			shipped = false
		}
	}
	if !implemented {
		return false, errors.New("names only future or rejected features, so it describes nothing Ze ships")
	}
	return shipped, nil
}

// CardLabel is the badge a site card carries: none when shipped, the
// experimental level's label otherwise. The site has two states (D-9), so a
// card that is not shipped reads as experimental whatever holds it back.
func CardLabel(shipped bool) string {
	if shipped {
		return ""
	}
	return levelLabels[LevelExperimental]
}

// CardClass is the style class of a card that is not shipped: the
// experimental level's declared name.
func CardClass() string { return LevelExperimental.String() }

// RenderPage renders docs/features.md from the declarations.
//
// One table per kind, in the order the vocabulary declares the kinds, and the
// rows of a table ordered by name (D-11). Every declaration has a row: a part
// of an umbrella keeps its own row in its kind's table, and the umbrella's row
// names its parts with their status, so no description is dropped.
func RenderPage(declarations []Declaration) string {
	byID := ByID(declarations)
	var page textbuf.Buffer
	page.Str("<!-- GENERATED from features/*.md by internal/le/feature (RenderPage). ").
		Str("Do not edit: change the feature's declaration. -->\n\n")
	page.Str("# Ze Features\n\n").Str(pageIntro).Str("\n\n")
	page.Str(vocabularySentence()).Byte('\n')
	for _, kind := range kindNames {
		rows := ofKind(declarations, kind.value)
		if len(rows) == 0 {
			continue
		}
		page.Str("\n## ").Str(kindHeadings[kind.value]).Str("\n\n")
		page.Str("| Feature | Status | Description |\n")
		page.Str("|---------|--------|-------------|\n")
		for _, declaration := range rows {
			page.Str(pageRow(declaration, byID))
		}
	}
	return page.String()
}

// vocabularySentence states what each status label asserts, from the one
// declaration of the vocabulary.
func vocabularySentence() string {
	var tb textbuf.Buffer
	tb.Str("Status values: ")
	for _, entry := range slices.Backward(levelNames) {
		tb.Byte('`').Str(levelLabels[entry.value]).Str("` means ").Str(levelMeanings[entry.value]).Str("; ")
	}
	tb.Str("`(partial)` after a level means ").Str(scopeMeanings[ScopePartial]).Str("; ")
	tb.Byte('`').Str(scopeLabels[ScopeFuture]).Str("` means ").Str(scopeMeanings[ScopeFuture]).Str("; ")
	tb.Byte('`').Str(scopeLabels[ScopeRejected]).Str("` means ").Str(scopeMeanings[ScopeRejected]).Str(".")
	return tb.String()
}

// ofKind answers the declarations of one kind, ordered by name, then by id so
// two equal names still sort the same way every time.
func ofKind(declarations []Declaration, kind Kind) []*Declaration {
	var out []*Declaration
	for index := range declarations {
		if declarations[index].Kind == kind {
			out = append(out, &declarations[index])
		}
	}
	slices.SortFunc(out, func(a, b *Declaration) int {
		if order := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); order != 0 {
			return order
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

// pageRow renders one table row: the linked name, the status label, and the
// description verbatim, followed by what a partial scope lacks and what an
// umbrella summarizes.
func pageRow(declaration *Declaration, byID map[string]*Declaration) string {
	var row textbuf.Buffer
	row.Str("| ")
	if declaration.Page == "" {
		row.Str(declaration.Name)
	} else {
		row.Byte('[').Str(declaration.Name).Str("](").Str(pageLink(declaration.Page)).Byte(')')
	}
	row.Str(" | ").Str(StatusLabel(declaration.Scope, declaration.Level)).Str(" | ").Str(declaration.Description)
	if len(declaration.ScopeGaps) != 0 {
		row.Str(" **Scope gaps:** ").Str(strings.Join(declaration.ScopeGaps, "; ")).Byte('.')
	}
	if len(declaration.Parts) != 0 {
		row.Str(" **Parts:** ")
		for index, id := range declaration.Parts {
			if index != 0 {
				row.Str(", ")
			}
			row.Str(partLabel(id, byID))
		}
		row.Byte('.')
	}
	row.Str(" |\n")
	return row.String()
}

// partLabel names one part of an umbrella with its status. A part no
// declaration holds is named by its id alone; `./le feature check` refuses it.
func partLabel(id string, byID map[string]*Declaration) string {
	part, known := byID[id]
	if !known {
		return id
	}
	var tb textbuf.Buffer
	return tb.Str(part.Name).Str(" (").Str(StatusLabel(part.Scope, part.Level)).Byte(')').String()
}

// pageLink answers a repository path relative to docs/features.md.
func pageLink(repoPath string) string {
	if rest, under := strings.CutPrefix(repoPath, pageDocsDir); under {
		return rest
	}
	return "../" + repoPath
}

// feedsPage reports whether writing path can change the rendered page: a
// declaration, directly under features/. Run records under features/runs/ do
// not feed it, because the page states the declared level.
func feedsPage(_, path string) bool {
	rest, under := strings.CutPrefix(path, declarationDir+"/")
	if !under {
		return false
	}
	if strings.Contains(rest, "/") {
		return false
	}
	return strings.HasSuffix(rest, ".md")
}

// rebuildPage writes docs/features.md from the declarations under root.
//
// A tree with no features/ directory owns no feature page, so it answers
// nothing, as rebuildLedger does for a tree with no rfc/short/. A malformed
// declaration fails the render rather than publishing a page without it.
func rebuildPage(root string) error {
	if _, err := os.Stat(filepath.Join(root, declarationDir)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	declarations, err := Declared(root)
	if err != nil {
		return err
	}
	target := filepath.Join(root, filepath.FromSlash(pageRel))
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return err
	}
	return derived.WriteAtomic(target, []byte(RenderPage(declarations)))
}
