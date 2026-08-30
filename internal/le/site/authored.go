// Design: website/AI.md -- a hand-authored page is published through the shell every other page uses
// Detail: shell.go wraps the body, nav.go states the sidebar, mirror.go writes the sibling.
package site

import (
	"fmt"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The authored pages are registered from here, so a build discovers them
// through the registry rather than through a call the build states by name.
func init() {
	registerProducer(Producer{Name: "authored", Render: renderAuthoredPages})
}

// authoredTitle and authoredDescription read the two facts the shell cannot
// derive from an authored page: what it is called and what it is about. Each
// one appears once in a head, so the first match is the answer.
//
// Both patterns cross a line ending, because the authored pages are formatted
// by prettier: website/performance/index.html states its description over three
// lines, and a single-line pattern reads that page as carrying none.
var (
	authoredTitle       = regexp.MustCompile(`(?s)<title>(.*?)</title>`)
	authoredDescription = regexp.MustCompile(`(?s)<meta\s+name="description"\s+content="([^"]*)"`)
)

// authoredSidebarBlock matches the page sidebar an authored page carries,
// including the indentation before it and the newline after it. A sidebar never
// nests a second one, so the first closing tag ends the first opening one.
var authoredSidebarBlock = regexp.MustCompile(`(?s)[ \t]*<aside class="page-sidebar"[^>]*>.*?</aside>\n?`)

// renderAuthoredPages publishes every hand-authored page of the website source
// tree and answers the route of each one.
func renderAuthoredPages(paths Paths) ([]string, error) {
	sources, err := authoredSources(paths.Source)
	if err != nil {
		return nil, err
	}
	links, err := loadPageLinks(paths.Source)
	if err != nil {
		return nil, err
	}
	routes := make([]string, 0, len(sources))
	for _, name := range sources {
		if err := publishAuthoredPage(paths, name, links); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		routes = append(routes, "/"+strings.TrimSuffix(name, pageIndexFile))
	}
	return routes, nil
}

// authoredSources answers every hand-authored page of one website source tree,
// as source-relative POSIX paths.
//
// The set is DISCOVERED rather than declared, for the reason a producer answers
// the routes it wrote rather than declaring them: a page added under website/
// would reach the artifact through staging and be claimed by nobody, which is
// the coverage hole this producer exists to close. fs.WalkDir walks in lexical
// order, so one checkout answers one order.
func authoredSources(source string) ([]string, error) {
	var pages []string
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relative)
		if entry.IsDir() {
			if isSourceOnly(name) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() != pageIndexFile || isSourceOnly(name) {
			return nil
		}
		pages = append(pages, name)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return pages, nil
}

// publishAuthoredPage writes one authored page into the artifact, with the
// Markdown mirror every published route carries.
func publishAuthoredPage(paths Paths, name string, links pageLinks) error {
	source, err := os.ReadFile(filepath.Join(paths.Source, filepath.FromSlash(name))) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return err
	}
	if isFrozenTalkPath(name) {
		return publishFrozenDeck(paths.Output, name, source)
	}
	page, err := authoredShellPage(string(source), name, links)
	if err != nil {
		return err
	}
	body, err := extractMain(page)
	if err != nil {
		return err
	}

	// The mirror is converted back from the page this build just wrote, because
	// an authored page has no Markdown source to copy: the body IS the markup,
	// and a reader of index.md wants the page rather than the markup.
	mirror, err := htmlToMarkdown(body, pageCanonicalURL(name))
	if err != nil {
		return err
	}
	return writePublishedPage(paths.Output, name, page, mirror)
}

// publishFrozenDeck writes one talk deck into the artifact exactly as its
// author wrote it, and writes no mirror.
//
// The deck reaches the artifact through staging as well. It is written here so
// that one named producer answers for it: a published page nobody claims is
// what `./le site check` refuses, and "staging copied it" names no producer.
func publishFrozenDeck(output, name string, source []byte) error {
	path := filepath.Join(output, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return err
	}
	if err := os.WriteFile(path, source, 0o644); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// authoredShellPage answers one authored page rendered through the shared
// shell: the authored head states the title and the description, the authored
// <main> states the body, and every other part of the page comes from the
// shell.
//
// Rendering rather than patching is what makes an authored page carry the same
// chrome as a generated one. The retired build patched four fragments into a
// page that already held its own head, its own header mount and its own footer,
// so an authored page kept whatever chrome it was written with wherever a patch
// did not reach.
func authoredShellPage(source, name string, links pageLinks) (string, error) {
	title, err := authoredHeadValue(authoredTitle, source, name, "<title>")
	if err != nil {
		return "", err
	}
	description, err := authoredHeadValue(authoredDescription, source, name, `<meta name="description">`)
	if err != nil {
		return "", err
	}
	body, err := extractMain(source)
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	root := pageRoot(name)
	shell := pageShell{
		Title:       title,
		Description: description,
		Root:        root,
		Path:        name,
		Sidebar:     pageSidebar(root, name, links),
	}
	return shell.render(authoredBody(body)), nil
}

// authoredHeadValue answers one value of an authored head, unescaped.
//
// The value is unescaped because the shell escapes what it writes: a title
// holding an apostrophe would otherwise publish the character reference itself.
// A page that states neither value is refused by name, because the shell would
// publish an empty title and a search result with no description.
func authoredHeadValue(pattern *regexp.Regexp, source, name, element string) (string, error) {
	match := pattern.FindStringSubmatch(source)
	if match == nil {
		return "", fmt.Errorf("%s: the page states no %s", name, element)
	}
	value := strings.TrimSpace(html.UnescapeString(match[1]))
	if value == "" {
		return "", fmt.Errorf("%s: the page states an empty %s", name, element)
	}
	return value, nil
}

// authoredBody answers the body the shell splices in: the authored <main>
// content with its own page sidebar removed and the line ends around it
// trimmed.
//
// The sidebar goes because the shell writes the one
// website/data/page-links.json declares, and an authored copy would publish the
// sidebar the page was written with instead. The opening newline and the
// closing indentation go because the shell writes the <main> and </main> lines
// itself, and the authored body carries the line ends of both.
func authoredBody(body string) string {
	body = authoredSidebarBlock.ReplaceAllString(body, "")
	return strings.TrimRight(strings.TrimLeft(body, "\n"), " \t")
}
