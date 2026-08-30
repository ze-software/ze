// Design: website/AI.md -- the wiki is referenced by the site, never republished by it
//
// The Ze wiki is its own source of truth. plan/spec-website-wiki-content-migration.md
// settled that on 2026-07-22: content the wiki owns is linked, not copied, so a
// reader is never shown two answers with two dates on them.
//
// What the site publishes is therefore an INDEX: one title, one public URL and
// one summary for each wiki page. This package derives that index from a wiki
// checkout and writes it to website/data/wiki.json, which the site build reads.
// The build never opens the wiki checkout, so a machine that has only this
// repository produces the same artifact.
//
// Related: internal/le/site/llmsfull.go -- the reader of the committed index.
package sitewiki

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The committed index and the file the order comes from.
//
// The sidebar is the wiki's own curation. This repository cannot derive it: an
// alphabetical list of 171 pages says nothing about which one a reader opens
// first, and the sidebar groups already run About and First Steps before
// Configuration and Operation, which is evaluation before usage.
const (
	// IndexFile is the committed index, relative to the checkout root.
	IndexFile = "website/data/wiki.json"
	// sidebarFile is the wiki page that states the reading order.
	sidebarFile = "_Sidebar.md"
	// markdownExtension selects a wiki page in the checkout.
	markdownExtension = ".md"
)

// DefaultBaseURL is where a reader opens a wiki page.
//
// The base belongs in the committed index rather than in this code, because a
// host change must not need a Go edit: `le site wiki update base-url <url>`
// changes it in one place and every link follows. This value is the default
// the site publishes today.
const DefaultBaseURL = "https://github.com/ze-software/ze/wiki/"

// summaryLimit bounds one page summary, in characters.
//
// The index is a reference, not a second copy of the wiki: a reader scans it to
// choose a page. One capped sentence for each of 167 pages keeps the section a
// list rather than a document.
const summaryLimit = 200

// Index is the committed answer: where the wiki is, and what it holds.
//
// Groups carry the sidebar's own order, and each group carries its pages in the
// sidebar's own order, because that order is the only curation there is.
type Index struct {
	BaseURL string  `json:"base-url"`
	Groups  []Group `json:"groups"`
	// Unlisted names the wiki pages the sidebar does not carry, with the reason
	// each one is out. It is published rather than dropped, so a page the index
	// omits is a stated omission and never a silent one.
	Unlisted []Unlisted `json:"unlisted,omitempty"`
}

// Group is one sidebar heading and the pages under it.
type Group struct {
	Title string `json:"title"`
	Pages []Page `json:"pages"`
}

// Page is one wiki page as the site references it.
type Page struct {
	Title   string `json:"title"`
	Slug    string `json:"slug"`
	Summary string `json:"summary"`
}

// Unlisted is one wiki page the sidebar does not list, and why.
type Unlisted struct {
	Slug string `json:"slug"`
	Why  string `json:"why"`
}

// URL answers where a reader opens one page of an index.
func (index Index) URL(slug string) string {
	return strings.TrimSuffix(index.BaseURL, "/") + "/" + slug
}

// PageCount answers how many pages the index references.
func (index Index) PageCount() int {
	count := 0
	for _, group := range index.Groups {
		count += len(group.Pages)
	}
	return count
}

// accountedUnlisted names every wiki page the sidebar does not list and this
// repository has already judged, with the reason it stays out of the index.
//
// The table exists so the refusal below fires on a page NOBODY has judged.
// Without it the refusal fires on every build and gets switched off; with it,
// a new unlisted page is refused by name until somebody decides what it is.
//
// The last two rows are DEBT, not a decision to omit reader content: both pages
// carry material a reader wants, and the fix is one line each in the wiki's own
// _Sidebar.md. They are named here so the omission is published rather than
// invisible, and the row goes the day the sidebar carries the page.
var accountedUnlisted = map[string]string{
	"CLAUDE":            "agent instructions, not reader content",
	"command-catalog":   "a generated command dump; the site publishes its own command reference",
	"community-filters": "reader content the wiki sidebar omits; add it to _Sidebar.md and this row goes",
	"telemetry":         "reader content the wiki sidebar omits; add it to _Sidebar.md and this row goes",
}

// sidebarLink matches one Markdown link of the sidebar.
var sidebarLink = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// Derive answers the index one wiki checkout states.
//
// It refuses three things, each by name, because each one publishes a wrong
// index in silence: a sidebar link that resolves to no page, a page the sidebar
// lists under no group, and a page the sidebar does not list at all.
func Derive(wikiRoot, baseURL string) (Index, error) {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	groups, listed, err := readSidebar(wikiRoot)
	if err != nil {
		return Index{}, err
	}
	present, err := readPages(wikiRoot)
	if err != nil {
		return Index{}, err
	}
	if err := refuseDanglingLinks(groups, present); err != nil {
		return Index{}, err
	}
	unlisted, err := refuseUnjudgedPages(present, listed)
	if err != nil {
		return Index{}, err
	}
	for groupIndex := range groups {
		for pageIndex := range groups[groupIndex].Pages {
			page := &groups[groupIndex].Pages[pageIndex]
			summary, readErr := pageSummary(wikiRoot, page.Slug)
			if readErr != nil {
				return Index{}, readErr
			}
			page.Summary = summary
		}
	}
	return Index{BaseURL: baseURL, Groups: groups, Unlisted: unlisted}, nil
}

// readSidebar answers the sidebar's groups and the set of slugs it lists.
//
// A bold line opens a group. When that line carries a link, the link is both
// the group's title and its first page, which is how the wiki writes a section
// that has a landing page of its own.
//
// A page the sidebar lists TWICE is kept at its first position and dropped at
// every later one. The wiki menu lists twelve pages under two groups each --
// bfd and firewall sit under Configuration and under Operation, vpp under four
// -- because a menu offers a reader two ways to the same page. The index is not
// a menu: a second entry states one page's title, URL and summary again, and
// makes the page count say the wiki is larger than it is.
func readSidebar(wikiRoot string) ([]Group, map[string]bool, error) {
	path := filepath.Join(wikiRoot, sidebarFile)
	content, err := os.Open(path) //nolint:gosec // the wiki checkout this action was pointed at
	if err != nil {
		return nil, nil, fmt.Errorf("read the wiki sidebar: %w", err)
	}
	defer content.Close() //nolint:errcheck // a read-only file

	var groups []Group
	listed := make(map[string]bool, 192)
	scanner := bufio.NewScanner(content)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		match := sidebarLink.FindStringSubmatch(line)
		if strings.HasPrefix(line, "**") {
			title := strings.TrimSpace(strings.Trim(line, "* "))
			if match != nil {
				title = match[1]
			}
			if title == "" {
				continue
			}
			groups = append(groups, Group{Title: title})
		}
		if match == nil {
			continue
		}
		if len(groups) == 0 {
			return nil, nil, fmt.Errorf("%s: the link [%s](%s) sits above every group heading, so the index has nowhere to put it", sidebarFile, match[1], match[2])
		}
		if listed[match[2]] {
			continue
		}
		group := &groups[len(groups)-1]
		group.Pages = append(group.Pages, Page{Title: match[1], Slug: match[2]})
		listed[match[2]] = true
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("read the wiki sidebar: %w", err)
	}
	return groups, listed, nil
}

// readPages answers the slug of every reader-facing page of a wiki checkout.
//
// A name opening with an underscore is wiki furniture -- the sidebar, the
// footer -- rather than a page, so it is not a candidate for the index and not
// a candidate for the refusal below either.
func readPages(wikiRoot string) (map[string]bool, error) {
	entries, err := os.ReadDir(wikiRoot)
	if err != nil {
		return nil, fmt.Errorf("read the wiki checkout: %w", err)
	}
	present := make(map[string]bool, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != markdownExtension || strings.HasPrefix(name, "_") {
			continue
		}
		present[strings.TrimSuffix(name, markdownExtension)] = true
	}
	return present, nil
}

// refuseDanglingLinks refuses a sidebar entry that resolves to no page.
//
// The index would publish a link a reader clicks into a 404, and the wiki's own
// llms.txt generator only prints a warning for the same case.
func refuseDanglingLinks(groups []Group, present map[string]bool) error {
	var dangling []string
	for _, group := range groups {
		for _, page := range group.Pages {
			if !present[page.Slug] {
				dangling = append(dangling, page.Slug)
			}
		}
	}
	if len(dangling) == 0 {
		return nil
	}
	sort.Strings(dangling)
	return fmt.Errorf("the wiki sidebar links %d page(s) the checkout does not hold: %s",
		len(dangling), strings.Join(dangling, ", "))
}

// refuseUnjudgedPages refuses a wiki page the sidebar does not list and the
// accounting table does not name, and answers the accounted omissions.
//
// An unlisted page is the failure this whole index exists to make visible: it
// carries reader content, the site references every other page, and nothing in
// a committed artifact would say it is missing.
func refuseUnjudgedPages(present, listed map[string]bool) ([]Unlisted, error) {
	var unjudged []string
	var accounted []Unlisted
	for slug := range present {
		if listed[slug] {
			continue
		}
		why, judged := accountedUnlisted[slug]
		if !judged {
			unjudged = append(unjudged, slug)
			continue
		}
		accounted = append(accounted, Unlisted{Slug: slug, Why: why})
	}
	if len(unjudged) != 0 {
		sort.Strings(unjudged)
		return nil, fmt.Errorf("the wiki sidebar does not list %d page(s) and nothing says why: %s"+
			" -- list each one in %s, or state its reason in accountedUnlisted (internal/le/site/wiki/index.go)",
			len(unjudged), strings.Join(unjudged, ", "), sidebarFile)
	}
	// The map above answers in a random order, so the published file would
	// differ between two runs over one checkout without this sort.
	sort.Slice(accounted, func(left, right int) bool { return accounted[left].Slug < accounted[right].Slug })
	return accounted, nil
}

// prosePrefixes open a sidebar line that carries no prose for a reader: a
// heading, a quote, an image, a table row.
var prosePrefixes = []string{"#", ">", "![", "|"}

// pageSummary answers one page's first sentence of prose.
func pageSummary(wikiRoot, slug string) (string, error) {
	path := filepath.Join(wikiRoot, slug+markdownExtension)
	content, err := os.ReadFile(path) //nolint:gosec // the wiki checkout this action was pointed at
	if err != nil {
		return "", fmt.Errorf("read the wiki page %s: %w", slug, err)
	}
	for line := range strings.SplitSeq(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || hasProsePrefix(trimmed) {
			continue
		}
		return capText(stripMarkdown(firstSentence(trimmed)), summaryLimit), nil
	}
	return "", nil
}

// hasProsePrefix reports whether one line opens with a marker that is not prose.
func hasProsePrefix(line string) bool {
	for _, prefix := range prosePrefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

// sentenceFloor is the shortest text a full stop can end.
//
// A summary is one sentence, and a full stop inside "e.g." or "v1.2" would end
// it after three characters. The floor is the wiki generator's own, so the two
// answer the same summary for the same page.
const sentenceFloor = 20

// firstSentence answers the text up to the first full stop that ends a sentence.
func firstSentence(text string) string {
	for index, character := range text {
		if character != '.' || index <= sentenceFloor {
			continue
		}
		if index+1 == len(text) || text[index+1] == ' ' {
			return text[:index+1]
		}
	}
	return text
}

// The inline markup a summary carries no font for.
var (
	inlineBold = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	inlineLink = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	inlineCode = regexp.MustCompile("`([^`]+)`")
)

// stripMarkdown folds one line of Markdown into the words a reader sees.
func stripMarkdown(text string) string {
	text = inlineBold.ReplaceAllString(text, "$1")
	text = inlineLink.ReplaceAllString(text, "$1")
	return inlineCode.ReplaceAllString(text, "$1")
}

// capText cuts one summary at the last word boundary inside limit characters.
func capText(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	cut := string(runes[:limit])
	if space := strings.LastIndex(cut, " "); space > 0 {
		cut = cut[:space]
	}
	return strings.TrimRight(cut, " .,;:") + "..."
}

// Read answers the committed index of one checkout.
func Read(root string) (Index, error) {
	path := filepath.Join(root, filepath.FromSlash(IndexFile))
	content, err := os.ReadFile(path) //nolint:gosec // the checkout this build was pointed at
	if err != nil {
		return Index{}, fmt.Errorf("read %s: %w", IndexFile, err)
	}
	var index Index
	if err := json.Unmarshal(content, &index); err != nil {
		return Index{}, fmt.Errorf("read %s: %w", IndexFile, err)
	}
	return index, nil
}

// Marshal answers the committed form of one index.
//
// It is written indented and newline-terminated because it is a committed file
// a person reads in a diff, and a one-line JSON blob makes every edit look like
// a rewrite of the whole file.
func Marshal(index Index) ([]byte, error) {
	content, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

// Write puts one index where the site build reads it, and answers the path.
func Write(root string, index Index) (string, error) {
	content, err := Marshal(index)
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, filepath.FromSlash(IndexFile))
	if err := os.WriteFile(path, content, 0o644); err != nil { //nolint:gosec // a committed source file the site build reads
		return "", fmt.Errorf("write %s: %w", IndexFile, err)
	}
	return path, nil
}
