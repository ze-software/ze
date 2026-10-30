// Design: docs/architecture/site-facts.md -- the plain-text guard over a built artifact
// Related: factmark.go -- markedFact, the one element a published value may stand in
// Related: actions.go -- runCheck reports what this file finds
// Related: rfccompliance.go -- rfcComplianceDirectory, the RFC section the ratchet may list

package site

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	xhtml "golang.org/x/net/html"
)

// plainFigure is one snapshot value a page shows outside its marker: the page,
// the figure as the reader sees it, and the snapshot key it matched.
type plainFigure struct {
	Page   string `json:"page"`
	Figure string `json:"figure"`
	Key    string `json:"key"`
}

// publishedFigure is one string the guard looks for in a page's text.
//
// A figure with no Unit is one of the snapshot's display strings, carrying a
// thousands separator or a "+", which reads as nothing but that figure wherever
// it stands. A figure with a Unit is a whole number of 100 or more, which is a
// published value only beside its unit word: smaller numbers and bare numbers
// are too common in prose (years, section numbers, counts of other things) to
// match without refusing pages that are right.
type publishedFigure struct {
	Key  string
	Text string
	Unit string
}

// rawFigureMin is the smallest whole number the guard matches beside its unit
// word. A value under it is matched only as a display string.
const rawFigureMin = 100

// displayFigurePattern matches a display string: a grouped number, a number
// with a "+", or both.
var displayFigurePattern = regexp.MustCompile(`^(?:\d{1,3}(?:,\d{3})+\+?|\d+\+)$`)

// publishedFigures answers every figure the snapshot publishes, sorted by key,
// so a value two keys share is reported under the first.
//
// The figures derive from the snapshot's own keys, so a value a later section
// adds is guarded with no edit here. A key's unit word is the last word of its
// own name, with the _display suffix dropped: cli_commands is "commands",
// tests.unit_display is "unit".
func publishedFigures(snapshot siteFactSnapshot) []publishedFigure {
	var figures []publishedFigure
	for _, key := range snapshot.keys() {
		if strings.HasPrefix(key, "_") {
			continue
		}
		value, _ := snapshot.value(key)
		if displayFigurePattern.MatchString(value) {
			figures = append(figures, publishedFigure{Key: key, Text: value})
			continue
		}
		number, err := strconv.Atoi(value)
		if err != nil {
			continue
		}
		if number < rawFigureMin {
			continue
		}
		unit := factUnit(key)
		figures = append(figures, publishedFigure{Key: key, Text: value, Unit: unit})
		if grouped := groupThousands(number); grouped != value {
			figures = append(figures, publishedFigure{Key: key, Text: grouped, Unit: unit})
		}
	}
	return figures
}

// factUnit answers the unit word of one snapshot key.
func factUnit(key string) string {
	name := key[strings.LastIndex(key, ".")+1:]
	name = strings.TrimSuffix(name, "_display")
	return name[strings.LastIndex(name, "_")+1:]
}

// plainFigureRatchetFile is the committed list, relative to the website
// sources, of the RFC pages that may still show server-written figures. The
// RFC section's own spec (spec-rfc-figures-sectioned-presentation) empties it
// as each page moves to data-ze-rfc markers; the check only ever shrinks it.
const plainFigureRatchetFile = "data/plain-figure-ratchet.json"

// plainFigureCheck is what the guard answers for one artifact: the figures
// pages show outside a marker, and the ratchet entries owed removal because
// their page shows none.
type plainFigureCheck struct {
	Figures      []plainFigure
	StaleRatchet []string
}

// plainFigureRatchet is the committed ratchet file.
type plainFigureRatchet struct {
	Pages []plainFigureRatchetPage `json:"pages"`
}

// plainFigureRatchetPage is one listed page and why it still shows plain
// figures.
type plainFigureRatchetPage struct {
	Page   string `json:"page"`
	Reason string `json:"reason"`
}

// checkPlainFigures walks every HTML page of one artifact and answers each
// snapshot value it shows outside a marker, less the pages the ratchet lists,
// and each listed page that shows none.
//
// The snapshot is the artifact's own data/site-facts.json, which is the file a
// reader's browser fills the marked values from. An artifact without one is
// refused, because no value could be told from plain text. The ratchet is read
// from the website sources, never from the artifact, and never written.
func checkPlainFigures(source, output string) (plainFigureCheck, error) {
	ratcheted, err := loadPlainFigureRatchet(source)
	if err != nil {
		return plainFigureCheck{}, err
	}
	findings, err := plainFiguresOfArtifact(output)
	if err != nil {
		return plainFigureCheck{}, err
	}
	var check plainFigureCheck
	carried := map[string]bool{}
	for _, finding := range findings {
		if ratcheted[finding.Page] {
			carried[finding.Page] = true
			continue
		}
		check.Figures = append(check.Figures, finding)
	}
	for page := range ratcheted {
		if !carried[page] {
			check.StaleRatchet = append(check.StaleRatchet, page)
		}
	}
	slices.Sort(check.StaleRatchet)
	return check, nil
}

// loadPlainFigureRatchet reads the committed ratchet file and answers the set
// of pages it lists.
//
// A missing file, a field it does not hold, a page outside the RFC section, a
// path that is not clean, a page listed twice and a page with no reason are
// each refused by name: the list exists only for the RFC section's
// server-written figures, so an entry anywhere else would be an exemption.
func loadPlainFigureRatchet(source string) (map[string]bool, error) {
	path := filepath.Join(source, filepath.FromSlash(plainFigureRatchetFile))
	content, err := os.ReadFile(path) //nolint:gosec // the check reads the committed list of the checkout it serves
	if err != nil {
		return nil, fmt.Errorf("read the plain-figure ratchet %s: %w; the file is committed with the website sources, "+
			"so restore it from git: git show HEAD:website/%s", path, err, plainFigureRatchetFile)
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var ratchet plainFigureRatchet
	if err := decoder.Decode(&ratchet); err != nil {
		return nil, fmt.Errorf("parse the plain-figure ratchet %s: %w; it holds only {\"pages\": [{\"page\", \"reason\"}]}", path, err)
	}
	if ratchet.Pages == nil {
		return nil, fmt.Errorf("the plain-figure ratchet %s states no \"pages\" list; an empty list is written []", path)
	}
	section := rfcComplianceDirectory + "/"
	listed := make(map[string]bool, len(ratchet.Pages))
	for _, entry := range ratchet.Pages {
		if !strings.HasPrefix(entry.Page, section) {
			return nil, fmt.Errorf("the plain-figure ratchet %s lists %q, which is outside the RFC section %s; "+
				"only an RFC page may be listed, so mark the page's figures with data-ze-stat and remove the entry",
				path, entry.Page, section)
		}
		if pathpkg.Clean(entry.Page) != entry.Page {
			return nil, fmt.Errorf("the plain-figure ratchet %s lists %q, which is not a clean artifact path; write it as %q",
				path, entry.Page, pathpkg.Clean(entry.Page))
		}
		if listed[entry.Page] {
			return nil, fmt.Errorf("the plain-figure ratchet %s lists %q twice; remove the second entry", path, entry.Page)
		}
		if strings.TrimSpace(entry.Reason) == "" {
			return nil, fmt.Errorf("the plain-figure ratchet %s lists %q with no reason; state why its figures are still server-written",
				path, entry.Page)
		}
		listed[entry.Page] = true
	}
	return listed, nil
}

// plainFiguresOfArtifact walks every HTML page of one artifact and answers each
// snapshot value it shows outside a marker.
func plainFiguresOfArtifact(output string) ([]plainFigure, error) {
	snapshot, err := loadSiteFactSnapshot(output)
	if err != nil {
		return nil, err
	}
	if snapshot.facts == nil {
		return nil, fmt.Errorf("%s is absent, so the site check cannot tell a published value from plain text; "+
			"build the artifact first: ./le site build output %s", snapshot.path, output)
	}
	figures := publishedFigures(snapshot)
	var findings []plainFigure
	err = filepath.WalkDir(output, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".html") {
			return nil
		}
		relative, err := filepath.Rel(output, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path) //nolint:gosec // the check reads the artifact it was pointed at
		if err != nil {
			return err
		}
		found, err := plainFigures(filepath.ToSlash(relative), string(content), figures)
		if err != nil {
			return err
		}
		findings = append(findings, found...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return findings, nil
}

// plainFigures answers each figure one page shows outside a marker.
//
// It walks the parsed page, the head included, because a title or a link
// preview description is read without the script that fills a marker. Text
// inside an element carrying data-ze-stat (a value of data/site-facts.json) or
// data-ze-rfc (a value of data/rfc/<stem>.json) is marked. Text inside code,
// pre, script and style is not prose a reader takes as a figure. The text
// attributes a reader or a crawler reads (meta content, title, alt, aria-label)
// are judged with the text.
func plainFigures(page, content string, figures []publishedFigure) ([]plainFigure, error) {
	document, err := xhtml.Parse(strings.NewReader(content))
	if err != nil {
		return nil, err
	}
	var findings []plainFigure
	// The walk is iterative, so a deeply nested page costs memory, never stack.
	stack := []*xhtml.Node{document}
	for len(stack) != 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node.Type == xhtml.TextNode {
			findings = append(findings, figuresIn(page, node.Data, figures)...)
			continue
		}
		if node.Type == xhtml.ElementNode {
			if figureMarked(node) {
				continue
			}
			if slices.Contains([]string{"code", "pre", "script", "style"}, node.Data) {
				continue
			}
			for _, attribute := range node.Attr {
				if figureAttribute(node.Data, attribute.Key) {
					findings = append(findings, figuresIn(page, attribute.Val, figures)...)
				}
			}
		}
		for child := node.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}
	return findings, nil
}

// figureMarked answers whether an element carries one of the two markers.
func figureMarked(node *xhtml.Node) bool {
	for _, attribute := range node.Attr {
		if attribute.Key == "data-ze-stat" {
			return true
		}
		if attribute.Key == "data-ze-rfc" {
			return true
		}
	}
	return false
}

// figureAttribute answers whether an attribute holds text a reader or a
// crawler reads as prose.
func figureAttribute(element, attribute string) bool {
	switch attribute {
	case "title", "alt", "aria-label":
		return true
	case "content":
		return element == "meta"
	}
	return false
}

// figuresIn answers each figure one run of text shows. A position two keys
// share is reported once, under the first key.
func figuresIn(page, text string, figures []publishedFigure) []plainFigure {
	var findings []plainFigure
	reported := map[int]bool{}
	for _, figure := range figures {
		for offset := 0; offset < len(text); {
			index := strings.Index(text[offset:], figure.Text)
			if index < 0 {
				break
			}
			start := offset + index
			end := start + len(figure.Text)
			offset = end
			if !figureStandsAlone(text, start, end) {
				continue
			}
			if figure.Unit != "" && !unitFollows(text[end:], figure.Unit) {
				continue
			}
			if reported[start] {
				continue
			}
			reported[start] = true
			findings = append(findings, plainFigure{Page: page, Figure: figure.Text, Key: figure.Key})
		}
	}
	return findings
}

// figureStandsAlone answers whether text[start:end] is a whole number rather
// than a piece of a longer one.
func figureStandsAlone(text string, start, end int) bool {
	if start > 0 {
		previous := text[start-1]
		if isFigureDigit(previous) {
			return false
		}
		if previous == ',' {
			return false
		}
		if previous == '.' {
			return false
		}
	}
	if end >= len(text) {
		return true
	}
	if isFigureDigit(text[end]) {
		return false
	}
	if end+1 < len(text) && isFigureDigit(text[end+1]) {
		return text[end] != ',' && text[end] != '.'
	}
	return true
}

// isFigureDigit answers whether one byte is an ASCII digit.
func isFigureDigit(character byte) bool {
	return character >= '0' && character <= '9'
}

// unitFollows answers whether the next word after a number is its unit word,
// in the singular or the plural.
func unitFollows(rest, unit string) bool {
	trimmed := strings.TrimLeftFunc(rest, unicode.IsSpace)
	if len(trimmed) == len(rest) {
		return false
	}
	end := strings.IndexFunc(trimmed, func(character rune) bool {
		return !unicode.IsLetter(character) && !unicode.IsDigit(character)
	})
	if end < 0 {
		end = len(trimmed)
	}
	word := strings.ToLower(trimmed[:end])
	if word == "" {
		return false
	}
	return word == unit || word == unit+"s" || word+"s" == unit
}
