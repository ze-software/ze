// Design: website/AI.md -- every public page has one route and one Markdown mirror
package site

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// pageIndexFile is the file that makes a directory a public route, and
// pageMirrorFile is the Markdown sibling beside it. A directory without the
// first is not a page, whatever else it holds.
const (
	pageIndexFile  = "index.html"
	pageMirrorFile = "index.md"
	// markdownExtension names a Markdown source. A page producer that reads a
	// directory of sources selects on it.
	markdownExtension = ".md"
)

// unpublishedDirectories are the top-level directories of the artifact that
// hold no published page. Each one can carry a Markdown file that is not a
// mirror, and each one holds files a crawler has no route to.
//
// The search index and the sitemap read this one declaration. They walked two
// identical copies until 2026-09-02, which is two answers to one question with
// nothing to arbitrate them.
var unpublishedDirectories = map[string]bool{
	"assets": true, "data": true, "tmp": true, gitMetadataDir: true,
}

// Page is one public route and its human and machine-readable representations.
type Page struct {
	Route    string `json:"route"`
	HTML     string `json:"html"`
	Markdown string `json:"markdown"`
}

// pageRegistry discovers public routes from the artifact. A directory enters
// the registry only through index.html, so helper files cannot become pages by
// accident.
func pageRegistry(root string) ([]Page, error) {
	var pages []Page
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			name := filepath.ToSlash(relative)
			if name == "presentations" || name == "tmp" || name == gitMetadataDir {
				return filepath.SkipDir
			}
		}
		if entry.IsDir() || entry.Name() != pageIndexFile {
			return nil
		}
		content, err := os.ReadFile(path) //nolint:gosec // a site build reads the checkout it was pointed at
		if err != nil {
			return err
		}
		if isRedirectPage(content) {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		directory := filepath.Dir(filepath.ToSlash(relative))
		route := "/"
		if directory != "." {
			route += strings.Trim(directory, "/") + "/"
		}
		pages = append(pages, Page{
			Route: route, HTML: filepath.ToSlash(relative),
			Markdown: filepath.ToSlash(filepath.Join(filepath.Dir(relative), pageMirrorFile)),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(pages, func(left, right int) bool { return pages[left].Route < pages[right].Route })
	return pages, nil
}

// isFrozenTalkPath reports whether one artifact-relative path belongs to a talk
// deck, which every pass of the build leaves exactly as its author wrote it.
//
// The rule is the retired build's own: a path under talks/ whose second segment
// is not index.html is frozen. So talks/index.html is the listing and is
// rendered like any other page, while talks/<slug>/ is a self-contained deck --
// its own document, its own stylesheet, its 3.7MB inlined bundle and its
// slides -- and neither the shared shell nor a Markdown mirror says anything
// true about it.
func isFrozenTalkPath(name string) bool {
	segments := strings.Split(strings.Trim(filepath.ToSlash(name), "/"), "/")
	return len(segments) > 1 && segments[0] == talksDirectory && segments[1] != pageIndexFile
}

func isRedirectPage(content []byte) bool {
	text := string(content)
	return strings.Contains(text, `<meta name="robots" content="noindex">`) &&
		strings.Contains(text, `<meta http-equiv="refresh"`)
}

// checkPageMirrors reports every public route whose Markdown sibling is absent,
// the frozen talk decks aside: a deck owes no mirror.
func checkPageMirrors(root string) ([]string, error) {
	pages, err := pageRegistry(root)
	if err != nil {
		return nil, err
	}
	var missing []string
	for _, page := range pages {
		// A frozen talk deck carries no mirror, because nothing renders it:
		// the retired build's own mirror check skipped the same paths.
		if isFrozenTalkPath(page.HTML) {
			continue
		}
		if info, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(page.Markdown))); statErr != nil || !info.Mode().IsRegular() {
			missing = append(missing, fmt.Sprintf("%s is missing %s", page.Route, page.Markdown))
		}
	}
	return missing, nil
}
