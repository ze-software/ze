// Design: website/AI.md -- every public page has one route and one Markdown mirror
package sitebuild

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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
			if name == "presentations" || name == "tmp" || name == ".git" {
				return filepath.SkipDir
			}
		}
		if entry.IsDir() || entry.Name() != "index.html" {
			return nil
		}
		content, err := os.ReadFile(path)
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
			Markdown: filepath.ToSlash(filepath.Join(filepath.Dir(relative), "index.md")),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(pages, func(left, right int) bool { return pages[left].Route < pages[right].Route })
	return pages, nil
}

func isRedirectPage(content []byte) bool {
	text := string(content)
	return strings.Contains(text, `<meta name="robots" content="noindex">`) &&
		strings.Contains(text, `<meta http-equiv="refresh"`)
}

// checkPageMirrors reports every public route whose Markdown sibling is absent.
func checkPageMirrors(root string) ([]string, error) {
	pages, err := pageRegistry(root)
	if err != nil {
		return nil, err
	}
	var missing []string
	for _, page := range pages {
		if info, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(page.Markdown))); statErr != nil || !info.Mode().IsRegular() {
			missing = append(missing, fmt.Sprintf("%s is missing %s", page.Route, page.Markdown))
		}
	}
	return missing, nil
}
