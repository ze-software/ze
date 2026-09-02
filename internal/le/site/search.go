// Design: website/AI.md -- the site searches itself in the reader's browser
// Detail: the index is one record per published Markdown mirror, plus one per config section.
// Related: config.go publishes the configuration tree this file indexes section by section.
package site

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The search page and its index are written together, after every page
// producer, because the index is a walk over the Markdown mirror of every
// published page.
func init() {
	registerDerivedProducer(Producer{Name: "search", Render: renderSearch})
}

// Where the search page and its index are published, and how far the page sits
// from the site root.
const (
	searchDirectory = "search"
	searchDest      = searchDirectory + "/" + pageIndexFile
	searchRoot      = "../"
	searchIndexFile = "data/search-index.json"
	searchTitle     = "Search - Ze"
	searchLead      = "Search across all of Ze's documentation and pages. Everything runs in " +
		"your browser: nothing you type is sent anywhere."
	searchDescription = "Search across all of Ze's documentation and pages, in your browser."
)

// searchBodyCap bounds one page's indexed body, and searchConfigBodyCap bounds
// one configuration section's.
//
// A cap is needed because the index is downloaded whole by every reader who
// opens the search page. The page cap is large enough that a multi-section
// guide stays searchable past its first screen. The section cap is larger
// because a configuration section's body is written names first, so the cap
// only ever trims trailing prose and never a setting name.
const (
	searchBodyCap       = 8000
	searchConfigBodyCap = 12000
)

// configurationReferenceDirectory is the one page indexed section by section
// rather than as one record.
//
// Its Markdown mirror is a single dump of the whole YANG tree, well past the
// page cap, so one capped record buried every configuration term in it: a
// search for a section name found nothing. Each section is indexed instead,
// deep-linked to the hash the in-page explorer routes on.
const configurationReferenceDirectory = "reference/configuration"

// searchRecord is one entry of the browser's index.
//
// The field order is published: the file is one record per line so a content
// change reads as a changed line rather than as a replaced file, and a struct
// states an order where a map states none. Title and DisplayTitle carry one
// value each because the page's script reads the display pair and the matcher
// reads the other.
type searchRecord struct {
	Title          string `json:"title"`
	DisplayTitle   string `json:"displayTitle"`
	URL            string `json:"url"`
	Section        string `json:"section"`
	DisplaySection string `json:"displaySection"`
	Text           string `json:"text"`
}

// newSearchRecord answers one record with its display fields stripped of
// Markdown, which is what the page prints into the results list.
func newSearchRecord(title, url, section, text string) searchRecord {
	title = stripMarkdown(title)
	section = stripMarkdown(section)
	return searchRecord{
		Title: title, DisplayTitle: title,
		URL:     url,
		Section: section, DisplaySection: section,
		Text: text,
	}
}

// renderSearch publishes the search page, its mirror and the index the page
// reads, and answers the one route it writes.
//
// The page is written BEFORE the walk, so the search page is in its own index.
// The retired renderer wrote it after, and the record for it existed only
// because the previous artifact seeded the mirror back in.
func renderSearch(paths Paths) ([]string, error) {
	links, err := loadPageLinks(paths.Source)
	if err != nil {
		return nil, err
	}
	if err := writeSearchPage(paths.Output, links); err != nil {
		return nil, err
	}
	records, err := searchRecords(paths.Output)
	if err != nil {
		return nil, err
	}
	if err := writeNamedArtifact(paths.Output, searchIndexFile, searchIndexJSON(records)); err != nil {
		return nil, err
	}
	return []string{"/" + searchDirectory + "/"}, nil
}

// writeSearchPage publishes the search page and the Markdown mirror beside it.
func writeSearchPage(output string, links pageLinks) error {
	var body textbuf.Buffer
	body.Reset().Str(`            <section class="md-content reveal" aria-labelledby="search-title">`).Byte('\n')
	body.Str(pageHero("Search", searchLead, "Site search", ` id="search-title"`, heroClasses)).Byte('\n')
	body.Str(`                <div class="cli-search-wrap">`).Byte('\n')
	body.Str(`                    <input id="site-search" type="search" autocomplete="off"`).Byte('\n')
	body.Str(`                        autofocus aria-label="Search the site"`).Byte('\n')
	body.Str(`                        placeholder="Search the site `).
		Str(`(e.g. flowspec, quickstart, RPKI, exabgp)..." />`).Byte('\n')
	body.Str("                </div>\n")
	body.Str(`                <p id="search-status" class="search-status" aria-live="polite"></p>`).Byte('\n')
	body.Str(`                <ol id="search-results" class="search-results"></ol>`).Byte('\n')
	body.Str(`                <noscript><p>JavaScript is disabled. Browse from the `).
		Str(`<a href="`).Str(searchRoot).Str(`docs/">documentation hub</a> instead.</p></noscript>`).Byte('\n')
	body.Str("            </section>\n")

	shell := pageShell{
		Title:       searchTitle,
		Description: searchDescription,
		Root:        searchRoot,
		Path:        searchDest,
		Sidebar:     pageSidebar(searchRoot, searchDirectory+"/", links),
	}
	page := filepath.Join(output, filepath.FromSlash(searchDest))
	if err := os.MkdirAll(filepath.Dir(page), 0o755); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return err
	}
	if err := os.WriteFile(page, []byte(shell.render(body.String())), 0o644); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return fmt.Errorf("write %s: %w", page, err)
	}
	return writeMarkdownMirror(page, searchMirror())
}

// searchMirror is the search page as a reader without JavaScript reads it: what
// the page does, what it reads, and where to go instead.
func searchMirror() string {
	return "# Search\n\nSearch across all of Ze's documentation and pages. This page runs the " +
		"search in your browser against [" + searchIndexFile + "](" + searchRoot + searchIndexFile + "), " +
		"a JSON index of every page's Markdown mirror. It needs JavaScript; with it " +
		"off, browse from the [documentation hub](" + searchRoot + "docs/) instead.\n"
}

// searchRecords answers every record of the index, ordered by URL.
//
// The walk order is the artifact's own path order and the sort is STABLE, so
// two records sharing a URL keep the order they were discovered in. The sort
// alone does not decide the file, which is why the walk is sorted first.
func searchRecords(output string) ([]searchRecord, error) {
	records, err := pageSearchRecords(output)
	if err != nil {
		return nil, err
	}
	configRecords, err := configSearchRecords(output)
	if err != nil {
		return nil, err
	}
	records = append(records, configRecords...)
	sortSearchRecords(records)
	return records, nil
}

// sortSearchRecords orders one index by URL and keeps the order two records
// sharing one URL were discovered in.
//
// The sort is STABLE because the discovery it orders is already sorted: the
// page mirrors arrive in path order and the configuration sections in name
// order. A tie is therefore an order the walk STATES, and sort.Slice would
// discard it for whatever partition its pivots chose.
func sortSearchRecords(records []searchRecord) {
	sort.SliceStable(records, func(left, right int) bool { return records[left].URL < records[right].URL })
}

// pageSearchRecords answers one record for each published Markdown mirror.
func pageSearchRecords(output string) ([]searchRecord, error) {
	var mirrors []string
	err := filepath.WalkDir(output, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			relative, relErr := filepath.Rel(output, path)
			if relErr != nil {
				return relErr
			}
			if unpublishedDirectories[filepath.ToSlash(relative)] {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() != pageMirrorFile {
			return nil
		}
		relative, relErr := filepath.Rel(output, path)
		if relErr != nil {
			return relErr
		}
		mirrors = append(mirrors, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(mirrors)

	records := make([]searchRecord, 0, len(mirrors))
	for _, mirror := range mirrors {
		directory := strings.TrimSuffix(strings.TrimSuffix(mirror, pageMirrorFile), "/")
		if directory == configurationReferenceDirectory {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(output, filepath.FromSlash(mirror))) //nolint:gosec // the artifact this build just wrote
		if readErr != nil {
			return nil, readErr
		}
		record := newSearchRecord(searchTitleOf(string(content), directory), searchURLOf(directory),
			searchSectionOf(directory), capRunes(stripMarkdown(string(content)), searchBodyCap))
		records = append(records, record)
	}
	return records, nil
}

// configSearchRecords answers one record for the configuration reference and
// one for each of its top-level sections.
func configSearchRecords(output string) ([]searchRecord, error) {
	_, tree, err := readConfigTree(output)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(tree))
	for name := range tree {
		names = append(names, name)
	}
	sort.Strings(names)

	records := make([]searchRecord, 0, len(names)+1)
	landing := "The complete Ze configuration as one searchable tree of sections, generated " +
		"from the YANG schema. Sections: " + strings.Join(names, ", ") + "."
	records = append(records, newSearchRecord("Configuration Reference",
		configurationReferenceDirectory+"/", configurationReferenceDirectory,
		capRunes(landing, searchBodyCap)))
	for _, name := range names {
		node := tree[name]
		var heads, descriptions []string
		flattenConfigSection(&node, &heads, &descriptions)
		body := strings.Join(slices.Concat(heads, descriptions), " ")
		records = append(records, newSearchRecord(name,
			configurationReferenceDirectory+"/#"+name, configurationReferenceDirectory,
			capRunes(body, searchConfigBodyCap)))
	}
	return records, nil
}

// flattenConfigSection collects one configuration subtree into two buckets.
//
// heads takes every node's own line and its type, which are the compact terms
// an operator searches for. descriptions takes the prose. The caller joins
// heads before descriptions, so the section cap only ever trims prose.
//
// The recursion is bounded by the depth of the YANG tree, which this build
// generated from the schema in the same checkout.
func flattenConfigSection(node *configNode, heads, descriptions *[]string) {
	*heads = append(*heads, configNodeHead(node))
	if node.Type != "" {
		*heads = append(*heads, node.Type)
	}
	if node.Description != "" {
		*descriptions = append(*descriptions, strings.Join(strings.Fields(node.Description), " "))
	}
	for index := range node.Children {
		flattenConfigSection(&node.Children[index], heads, descriptions)
	}
}

// searchIndexJSON answers the published index: valid JSON, one record per line.
//
// One record per line keeps a content change to the lines that changed. A
// minified single line would make every edit read as a full-file replacement.
func searchIndexJSON(records []searchRecord) string {
	if len(records) == 0 {
		return "[]\n"
	}
	var index textbuf.Buffer
	index.Reset().Str("[\n")
	for position := range records {
		if position != 0 {
			index.Str(",\n")
		}
		index.Str(searchRecordJSON(records[position]))
	}
	index.Str("\n]\n")
	return index.String()
}

// searchRecordJSON answers one record as one compact JSON object.
//
// HTML escaping is turned off because this is a data file rather than a page:
// the encoder writes "<" for "<" by default, which would spell every angle
// bracket a configuration node's head carries as an escape a reader must decode.
func searchRecordJSON(record searchRecord) string {
	var line bytes.Buffer
	encoder := json.NewEncoder(&line)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(record); err != nil {
		panic("BUG: site.searchRecordJSON: a record has no JSON encoding: " + err.Error())
	}
	return strings.TrimRight(line.String(), "\n")
}

// searchURLOf answers the site-relative URL of one page directory, empty for
// the site root.
func searchURLOf(directory string) string {
	if directory == "" || directory == "." {
		return ""
	}
	return directory + "/"
}

// searchSectionOf answers the section a page belongs to, which is its first
// path segment, or Home for the site root.
func searchSectionOf(directory string) string {
	if directory == "" {
		return "Home"
	}
	first, _, _ := strings.Cut(directory, "/")
	return first
}

// searchTitleOf answers a page's title: its first heading, or the last segment
// of its directory when it carries none.
func searchTitleOf(mirror, directory string) string {
	if match := firstHeading.FindStringSubmatch(mirror); match != nil {
		return match[1]
	}
	if directory == "" {
		return "Ze"
	}
	return directory[strings.LastIndex(directory, "/")+1:]
}

// The passes that turn one Markdown mirror into searchable words. Each one
// removes a construct a reader never types into a search box.
//
// They are named for this file rather than for Markdown, because two other
// files strip Markdown for a different reader and answer differently:
// llmsdata.go keeps a sentence readable, and this one keeps only the words a
// match runs over.
var (
	searchComment     = regexp.MustCompile(`(?s)<!--.*?-->`)
	searchFence       = regexp.MustCompile("(?m)^```.*$")
	searchLinkText    = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	searchHeadingMark = regexp.MustCompile(`(?m)^\s{0,3}#{1,6}\s*`)
	searchMarkup      = regexp.MustCompile("[*`_>]+")
	searchSpaceRuns   = regexp.MustCompile(`\s+`)
)

// stripMarkdown answers one Markdown text as the words a reader would search
// for: no comments, no fences, link text without its target, no heading marks,
// no emphasis, and one space between words.
func stripMarkdown(text string) string {
	text = searchComment.ReplaceAllString(text, " ")
	text = searchFence.ReplaceAllString(text, " ")
	text = searchLinkText.ReplaceAllString(text, "$1")
	text = searchHeadingMark.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "|", " ")
	text = searchMarkup.ReplaceAllString(text, " ")
	text = searchSpaceRuns.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// capRunes answers the first cap characters of a text.
//
// The unit is the character rather than the byte, so a cut never lands inside
// one: a body ending in a replacement character is what a byte cut publishes.
func capRunes(text string, cap int) string {
	runes := []rune(text)
	if len(runes) <= cap {
		return text
	}
	return string(runes[:cap])
}
