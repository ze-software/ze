// Design: docs/contributing/ze-go-style.md -- the gated core and its sibling pages
// Related: links.go -- the link check that resolves the core's sibling links.

package doccheck

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// styleCorePath is the page the Go style gate requires a whole read of.
const styleCorePath = "docs/contributing/ze-go-style.md"

// styleSiblingDir holds the pages the core's tables, long examples and
// reasoning moved to.
const styleSiblingDir = "docs/contributing/go-style"

// styleCoreBytesMax is the size the core MUST stay within, so that the whole
// read the gate requires stays affordable for every editing agent.
const styleCoreBytesMax = 16 << 10

// splitPageHeadings is every heading of the style page as it stood before the
// split, level marks included. It is a snapshot on purpose: the page it
// describes no longer exists in the tree, so nothing else can declare it.
var splitPageHeadings = []string{
	"# Ze Go Style",
	"## The rules a reader applies",
	"### Assertions, in a language that has none",
	"### A zero value is never an answer",
	"### Types that cannot lie",
	"#### One state, one variant",
	"#### Every enum switch has a coverage policy",
	"#### Validated at construction",
	"#### One type per lifecycle state",
	"### A limit on everything",
	"### Control flow a reader can simulate",
	"### Every error is handled",
	"### Goroutines",
	"### Comments say why and state the contract",
	"### Names",
	"### State that goes stale",
	"### The shape of a function",
	"### Performance",
	"### Simplicity and debt",
	"## The rules a tool enforces",
	"### By the numbers",
	"## Where Ze differs from standard Go",
	"### Encoding and wire",
	"### Architecture and registration",
	"### Config and schema",
	"### Communication and IPC",
	"### Testing",
	"### CLI and commands",
	"### Native tooling",
}

// styleSection is one heading of the core and the lines under it, up to the
// next heading of any level.
type styleSection struct {
	heading string
	level   int
	body    []string
}

// styleSections splits a markdown page at its headings. A line inside a fenced
// code block is body, never a heading.
func styleSections(page string) []styleSection {
	var sections []styleSection
	fenced := false
	for line := range strings.SplitSeq(page, "\n") {
		if strings.HasPrefix(line, "```") {
			fenced = !fenced
		}
		level := len(line) - len(strings.TrimLeft(line, "#"))
		if !fenced && level > 0 && strings.HasPrefix(line[level:], " ") {
			sections = append(sections, styleSection{heading: line, level: level})
			continue
		}
		if len(sections) > 0 {
			sections[len(sections)-1].body = append(sections[len(sections)-1].body, line)
		}
	}
	return sections
}

// markdownLink matches an inline link, its text and its target.
var markdownLink = regexp.MustCompile(`\[[^\]]*\]\([^)]*\)`)

// navigationTargets mark a paragraph that points at another page of the split
// rather than stating anything: a core link to a sibling, or a sibling's link
// back to the core.
var navigationTargets = []string{"](go-style/", "](../ze-go-style.md)"}

// statesItsRule answers whether a section body holds a paragraph of its own: a
// run of non-blank lines that links no page of the split, and that still holds
// text once its HTML comments, links and punctuation are taken out.
func statesItsRule(body []string) bool {
	var paragraph []string
	for _, line := range append(body, "") {
		if strings.TrimSpace(line) != "" {
			paragraph = append(paragraph, line)
			continue
		}
		if statesSomething(paragraph) {
			return true
		}
		paragraph = paragraph[:0]
	}
	return false
}

// statesSomething answers whether one paragraph is prose rather than
// navigation, a comment or a bare link.
func statesSomething(paragraph []string) bool {
	text := strings.Join(paragraph, "\n")
	for _, target := range navigationTargets {
		if strings.Contains(text, target) {
			return false
		}
	}
	for {
		start := strings.Index(text, "<!--")
		if start < 0 {
			break
		}
		end := strings.Index(text[start:], "-->")
		if end < 0 {
			text = text[:start]
			break
		}
		text = text[:start] + text[start+end+len("-->"):]
	}
	text = markdownLink.ReplaceAllString(text, "")
	return strings.TrimFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) != ""
}

func readStyleCore(t *testing.T) (root, page string) {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the checkout root: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(styleCorePath)))
	if err != nil {
		t.Fatalf("read the style core: %v", err)
	}
	return root, string(data)
}

// TestTheStyleCoreKeepsEveryHeadingOfTheSplitPage checks the split of the Go
// style page into a gated core and sibling pages (AC-10 of
// spec-agent-economy-3-preamble).
// VALIDATES: every heading of the page before the split is a heading of the
// core, and each one states something of its own under it unless a deeper
// heading follows at once; every page under docs/contributing/go-style/ is
// linked from the core.
// PREVENTS: a rule leaving the page the gate makes every writer read, with
// its heading, and a sibling page no reader of the core can reach.
func TestTheStyleCoreKeepsEveryHeadingOfTheSplitPage(t *testing.T) {
	root, page := readStyleCore(t)
	sections := styleSections(page)
	index := make(map[string]int, len(sections))
	for position, section := range sections {
		index[section.heading] = position
	}

	for _, heading := range splitPageHeadings {
		position, found := index[heading]
		if !found {
			t.Errorf("%s lost the heading %q of the page it was split from", styleCorePath, heading)
			continue
		}
		section := sections[position]
		if statesItsRule(section.body) {
			continue
		}
		parent := position+1 < len(sections) && sections[position+1].level > section.level
		if parent {
			continue
		}
		t.Errorf("%s keeps the heading %q with no rule under it: only a sibling link", styleCorePath, heading)
	}

	siblings, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(styleSiblingDir), "*.md"))
	if err != nil {
		t.Fatalf("list the sibling pages: %v", err)
	}
	if len(siblings) == 0 {
		t.Fatalf("no sibling page under %s: the detail of the style core has nowhere to live", styleSiblingDir)
	}
	for _, sibling := range siblings {
		name := filepath.Base(sibling)
		link := "](go-style/" + name + ")"
		if !strings.Contains(page, link) {
			t.Errorf("%s does not link the sibling page %s/%s", styleCorePath, styleSiblingDir, name)
		}
		data, err := os.ReadFile(sibling)
		if err != nil {
			t.Errorf("read the sibling page %s/%s: %v", styleSiblingDir, name, err)
			continue
		}
		holds := false
		for _, section := range styleSections(string(data)) {
			if statesItsRule(section.body) {
				holds = true
				break
			}
		}
		if !holds {
			t.Errorf("%s/%s holds no detail of its own: only its title and its link back to the core", styleSiblingDir, name)
		}
	}
}

// TestTheStyleCoreFitsItsSizeBound checks the size half of AC-10 of
// spec-agent-economy-3-preamble.
// VALIDATES: the gated style core is at most 16KB.
// PREVENTS: the core growing back to a page whose whole read costs every
// editing agent more than the rules it needs while writing.
func TestTheStyleCoreFitsItsSizeBound(t *testing.T) {
	_, page := readStyleCore(t)
	if len(page) > styleCoreBytesMax {
		t.Errorf("%s is %d bytes, above the %d it must stay within: move a table, a long example or reasoning to a page under %s",
			styleCorePath, len(page), styleCoreBytesMax, styleSiblingDir)
	}
}
