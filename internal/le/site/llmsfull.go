// Design: website/AI.md -- llms-full.txt is the whole site as one document
// Detail: derived.go writes llms.txt, the index; this file writes the bodies.
// Related: nav.go models website/data/nav.json; internal/le/site/wiki holds the wiki index.
package site

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	sitewiki "github.com/ze-software/ze/internal/le/site/wiki"
)

// llms-full.txt reads the Markdown mirror of every published page, so it is
// registered as a DERIVED producer and runs after every page producer.
//
// It answers NO route: it is a published file rather than a page, so the
// coverage arithmetic must not count it. namedArtifacts names it instead.
func init() {
	registerDerivedProducer(Producer{Name: "llms-full", Render: renderLLMSFull})
}

// llmsFullFile is the published path of the full-text answer for a machine
// reader, beside llms.txt.
const llmsFullFile = "llms-full.txt"

// The two prefixes that tell this file's own structure apart from the page
// bodies it carries. A mirror holds headings of its own, so "## Start" alone
// would read the same as a heading inside the page above it.
const (
	llmsFullSectionPrefix = "## Section: "
	llmsFullPagePrefix    = "### Page: "
)

// readingKind says whether a section tells a reader what Ze is, or how to use
// it. Zero is not a valid state, so a section that names no kind is refused
// rather than sorted into one.
type readingKind int

const (
	readingUnspecified readingKind = iota
	// readingEvaluation is what the software is and why it is worth evaluating.
	readingEvaluation
	// readingUsage is how to use it, and the material a reader reaches after
	// deciding to.
	readingUsage
)

// String answers the kind as a reader of a refusal reads it.
func (kind readingKind) String() string {
	switch kind {
	case readingEvaluation:
		return "evaluation"
	case readingUsage:
		return "usage"
	case readingUnspecified:
		return "unspecified"
	default:
		return "unspecified"
	}
}

// readingSection is one section of llms-full.txt: what it is for, and where its
// pages come from.
//
// Exactly one of Nav and Routes is set. A Nav section takes its membership and
// its internal order from a website/data/nav.json dropdown. A Routes section
// names its pages here, because the menu does not carry them and the document
// still owes them a home, and it carries a Title of its own for the same reason.
type readingSection struct {
	Title  string
	Kind   readingKind
	Nav    string
	Routes []string
}

// title answers the heading this section publishes. A nav section is titled by
// its dropdown, because the two are one curation under one name.
func (section readingSection) title() string {
	if section.Nav != "" {
		return section.Nav
	}
	return section.Title
}

// llmsFullReadingOrder is the ORDER llms-full.txt argues in, and it is a
// contract stated here rather than read from a file.
//
// The requirement is the owner's: what the software is and why it is worth
// evaluating comes first, how to use it comes second. nav.json supplies each
// section's MEMBERSHIP and the order WITHIN a section, and it supplies neither
// the section order nor what a section is for. A menu is ordered for a menu, so
// slaving this document's argument to it would let a menu reshuffle rewrite
// what the file argues with nothing to notice. checkNavReadingOrder is the
// thing that notices.
//
// The order follows llms.txt's own curation, which opens with the product
// snapshot, the verification model, the comparison and the feature inventory
// before it reaches the configuration model, the plugin registry and the CLI.
// nav.json's six dropdowns already run Start and Evaluate before Docs,
// Examples, Reference and Project, which is the same argument.
var llmsFullReadingOrder = []readingSection{
	// The site root is the one page that states what Ze is in a paragraph, so
	// it opens the document. It is named here because no menu entry points at
	// the site root: the logo does.
	{Title: "Overview", Kind: readingEvaluation, Routes: []string{"/"}},
	{Kind: readingEvaluation, Nav: "Start"},
	{Kind: readingEvaluation, Nav: "Evaluate"},
	{Kind: readingUsage, Nav: "Docs"},
	{Kind: readingUsage, Nav: "Examples"},
	{Kind: readingUsage, Nav: "Reference"},
	{Kind: readingUsage, Nav: "Project"},
	// The pages the menu does not carry, named one by one. A catch-all would
	// be the thing AC-15b forbids: a page landing at the end because nothing
	// claimed it. A page added to the site and to no section is refused instead.
	{Title: "About this site", Kind: readingUsage, Routes: []string{
		"/code-of-conduct/", "/demos/", "/license/", "/search/", "/style-guide/", "/zeledon/",
	}},
}

// sectionClaim is one section's claim on one published route.
//
// Length is how much of the route the claim matched, and it is what decides
// between two claims: a nav entry for /reference/cli/ and one for /reference/
// both match /reference/cli/bgp/, and the more specific entry is the one a
// reader would have used to reach the page. Order is the position the claiming
// entry holds inside its section, which is the order nav.json states.
//
// Exact separates the two kinds of claim. A nav entry claims the page it points
// at AND every page under it, because that is how a reader reaches a subtree
// from a menu. A route a section names here claims that page and nothing else:
// the site root would otherwise be a prefix of every route on the site, and
// "Overview" would swallow each page no menu carries.
type sectionClaim struct {
	section int
	order   int
	length  int
	exact   bool
}

// assignment is one published page and where the reading order puts it.
type assignment struct {
	route  string
	mirror string
	order  int
}

// renderLLMSFull publishes llms-full.txt: every published page's Markdown
// mirror, in the reading order, each preceded by its title and canonical URL.
//
// The frozen talk decks are excluded, as they are from every other mirror pass:
// a deck is its own document, carries no mirror, and nothing this file says
// about it would be true.
func renderLLMSFull(paths Paths) ([]string, error) {
	var nav siteNav
	if err := readSourceJSON(paths.Source, navDataFile, &nav); err != nil {
		return nil, err
	}
	if err := checkNavReadingOrder(nav); err != nil {
		return nil, err
	}
	pages, err := pageRegistry(paths.Output)
	if err != nil {
		return nil, err
	}
	sections, err := assignPages(pages, nav)
	if err != nil {
		return nil, err
	}
	index, err := sitewiki.Read(paths.Repository)
	if err != nil {
		return nil, err
	}

	var out textbuf.Buffer
	out.Reset()
	writeLLMSFullIntro(&out)
	for position, section := range llmsFullReadingOrder {
		if err := writeLLMSFullSection(&out, paths.Output, section, sections[position]); err != nil {
			return nil, err
		}
	}
	writeLLMSFullWiki(&out, index)

	content := strings.TrimRight(out.String(), "\n") + "\n"
	if err := writeNamedArtifact(paths.Output, llmsFullFile, content); err != nil {
		return nil, err
	}
	return nil, nil
}

// checkNavReadingOrder refuses a nav.json whose dropdown order disagrees with
// what this document argues.
//
// The reading order above owns the document, so a reshuffled menu changes no
// byte of llms-full.txt. That is exactly why the disagreement must be reported:
// the menu and the document would then argue different orders, and nothing in
// either file would say so. It also refuses a dropdown the reading order does
// not declare, because a dropdown with no kind is one this check cannot place.
func checkNavReadingOrder(nav siteNav) error {
	kinds := make(map[string]readingKind, len(llmsFullReadingOrder))
	for _, section := range llmsFullReadingOrder {
		if section.Nav != "" {
			kinds[section.Nav] = section.Kind
		}
	}
	usage := ""
	for index := range nav.Dropdowns {
		label := nav.Dropdowns[index].Label
		kind, declared := kinds[label]
		if !declared {
			return fmt.Errorf("%s states the dropdown %q and llmsFullReadingOrder declares no section for it,"+
				" so llms-full.txt cannot say whether it comes before or after the usage sections",
				navFile, label)
		}
		if kind == readingUsage {
			if usage == "" {
				usage = label
			}
			continue
		}
		if usage != "" {
			return fmt.Errorf("%s puts the usage section %q before the %s section %q;"+
				" llms-full.txt states what Ze is before how to use it, so the two orders disagree",
				navFile, usage, kind, label)
		}
	}
	return nil
}

// assignPages answers the pages of each declared section, in the order the
// section states them.
//
// It refuses three things by name. A page no section claims would otherwise be
// appended at the end because nothing wanted it. A page two sections claim
// would be published twice. A section no page fills would publish a heading
// over nothing, which reads as a gap rather than as the red it is.
func assignPages(pages []Page, nav siteNav) ([][]assignment, error) {
	claimants := sectionClaimants(nav)
	sections := make([][]assignment, len(llmsFullReadingOrder))
	var unsectioned []string
	for _, page := range pages {
		if isFrozenTalkPath(page.HTML) {
			continue
		}
		claim, doubled := claimFor(page.Route, claimants)
		if len(doubled) != 0 {
			return nil, fmt.Errorf("the page %s belongs to %d sections of llms-full.txt (%s);"+
				" one page is published once, so one section claims it",
				page.Route, len(doubled), strings.Join(doubled, ", "))
		}
		if claim.length == 0 {
			unsectioned = append(unsectioned, page.Route)
			continue
		}
		sections[claim.section] = append(sections[claim.section],
			assignment{route: page.Route, mirror: page.Markdown, order: claim.order})
	}
	if len(unsectioned) != 0 {
		sort.Strings(unsectioned)
		return nil, fmt.Errorf("%d published page(s) belong to no section of llms-full.txt: %s"+
			" -- give each one a section in llmsFullReadingOrder or a nav.json entry",
			len(unsectioned), strings.Join(unsectioned, ", "))
	}
	var empty []string
	for position, section := range llmsFullReadingOrder {
		if len(sections[position]) == 0 {
			empty = append(empty, section.title())
			continue
		}
		// pageRegistry answers in route order, so a stable sort on the claiming
		// entry keeps the pages of one entry in route order under it.
		assignments := sections[position]
		sort.SliceStable(assignments, func(left, right int) bool {
			return assignments[left].order < assignments[right].order
		})
	}
	if len(empty) != 0 {
		return nil, fmt.Errorf("%d section(s) of llms-full.txt are declared and no page fills them: %s"+
			" -- a section that silently empties reads as a gap, so it is refused here",
			len(empty), strings.Join(empty, ", "))
	}
	return sections, nil
}

// sectionClaimants answers every claim the reading order can make, keyed by the
// route prefix that carries it.
//
// A code-owned section claims a route exactly, so its prefix is the whole
// route. A nav section claims through each of its entries, so an entry claims
// the page it points at and every page under it.
func sectionClaimants(nav siteNav) map[string][]sectionClaim {
	byNav := make(map[string]int, len(llmsFullReadingOrder))
	claimants := make(map[string][]sectionClaim, 128)
	for position, section := range llmsFullReadingOrder {
		if section.Nav != "" {
			byNav[section.Nav] = position
			continue
		}
		for order, route := range section.Routes {
			claimants[route] = append(claimants[route],
				sectionClaim{section: position, order: order, length: len(route), exact: true})
		}
	}
	for index := range nav.Dropdowns {
		dropdown := &nav.Dropdowns[index]
		position, declared := byNav[dropdown.Label]
		if !declared {
			// checkNavReadingOrder refuses this before assignPages runs. The
			// guard is here so a future caller cannot silently drop a dropdown.
			continue
		}
		order := 0
		for _, column := range dropdown.Columns {
			for _, entry := range column {
				if entry.LabelOnly != "" {
					continue
				}
				prefix := rootedRoute(entry.Href)
				claimants[prefix] = append(claimants[prefix],
					sectionClaim{section: position, order: order, length: len(prefix)})
				order++
			}
		}
	}
	return claimants
}

// rootedRoute answers one nav href as pageRegistry spells a route: a leading
// slash, a trailing slash, and "/" for the site root.
func rootedRoute(href string) string {
	trimmed := strings.Trim(href, "/")
	if trimmed == "" {
		return "/"
	}
	return "/" + trimmed + "/"
}

// claimFor answers the most specific claim on one route, and the titles of the
// sections that tie for it when more than one does.
//
// Specificity settles the ordinary case: /reference/ and /reference/cli/ both
// claim /reference/cli/bgp/, and the second is the entry a reader would have
// followed. A tie is the case AC-15b calls a page belonging to two sections,
// and it is refused rather than resolved, because nothing in either file says
// which section should win.
func claimFor(route string, claimants map[string][]sectionClaim) (sectionClaim, []string) {
	best := sectionClaim{}
	titles := make(map[string]bool, 2)
	for prefix, claims := range claimants {
		if !strings.HasPrefix(route, prefix) {
			continue
		}
		for _, claim := range claims {
			if claim.exact && prefix != route {
				continue
			}
			if claim.length < best.length {
				continue
			}
			if claim.length > best.length {
				best = claim
				titles = map[string]bool{llmsFullReadingOrder[claim.section].title(): true}
				continue
			}
			titles[llmsFullReadingOrder[claim.section].title()] = true
		}
	}
	if len(titles) < 2 {
		return best, nil
	}
	tied := make([]string, 0, len(titles))
	for title := range titles {
		tied = append(tied, title)
	}
	sort.Strings(tied)
	return best, tied
}

// writeLLMSFullIntro states what this file is and how it is ordered.
func writeLLMSFullIntro(out *textbuf.Buffer) {
	out.Str("# Ze: every published page\n\n")
	out.Str("> The full text of the Ze website, as one file. Each page is the Markdown ").
		Str("mirror the site publishes beside its HTML, preceded by the page title and the canonical ").
		Str("URL a person opens. llms.txt beside this file is the same curation as links alone.\n\n")
	out.Str("The order is a reading order, not the site's route order: what Ze is and ").
		Str("why it is worth evaluating comes first, how to use it comes second. Talk decks are left ").
		Str("out, because a deck is its own document and publishes no Markdown mirror.\n\n")
	// A page body carries headings of its own, so a bare "## Start" would read
	// the same as a heading inside the page above it. The two prefixes are what
	// tells this file's own structure apart from the pages it carries.
	out.Str("Every section opens with `").Str(llmsFullSectionPrefix).Str("<name>` and every page with `").
		Str(llmsFullPagePrefix).Str("<title>` followed by its URL on the next line. The headings inside a ").
		Str("page body carry neither prefix.\n\n")
}

// writeLLMSFullSection writes one section: its heading, then each page's title,
// canonical URL and body.
func writeLLMSFullSection(out *textbuf.Buffer, output string, section readingSection, pages []assignment) error {
	out.Str(llmsFullSectionPrefix).Str(section.title()).Str("\n\n")
	for _, page := range pages {
		body, err := os.ReadFile(filepath.Join(output, filepath.FromSlash(page.mirror))) //nolint:gosec // the artifact this build just wrote
		if err != nil {
			return fmt.Errorf("llms-full.txt: the published page %s has no Markdown mirror: %w", page.route, err)
		}
		text := strings.TrimSpace(string(body))
		out.Str("---\n\n")
		out.Str(llmsFullPagePrefix).Str(mirrorTitle(text, page.route)).Byte('\n')
		out.Str(siteBase).Str(strings.TrimPrefix(page.route, "/")).Str("\n\n")
		if text != "" {
			out.Str(text).Str("\n\n")
		}
	}
	return nil
}

// mirrorTitle answers one page's title, taken from its mirror's own first
// heading, and its route when the mirror states none.
func mirrorTitle(text, route string) string {
	if match := markdownHeading.FindStringSubmatch(text); match != nil {
		if title := cleanInline(match[1]); title != "" {
			return title
		}
	}
	return route
}

// writeLLMSFullWiki writes the wiki reference section.
//
// It REFERENCES the wiki and does not republish it: one title, one public URL
// and one summary for each page. spec-website-wiki-content-migration
// settled that on 2026-07-22, and a reader shown two copies of one answer has
// to work out which one is current.
//
// The index is the committed website/data/wiki.json, so this build never opens
// a wiki checkout and a machine that has only this repository writes the same
// file. `le site wiki update` is what refreshes it.
func writeLLMSFullWiki(out *textbuf.Buffer, index sitewiki.Index) {
	out.Str(llmsFullSectionPrefix).Str("Wiki\n\n")
	out.Str("The Ze wiki is a separate source of truth, with its own pages and its own ").
		Str("edit history. It is referenced here rather than copied, so nothing below states an ").
		Str("answer twice with two dates on it. The order and the grouping are the wiki's own.\n\n")
	for _, group := range index.Groups {
		out.Str("### ").Str(group.Title).Str("\n\n")
		for _, page := range group.Pages {
			out.Str("- [").Str(page.Title).Str("](").Str(index.URL(page.Slug)).Byte(')')
			if page.Summary != "" {
				out.Str(": ").Str(page.Summary)
			}
			out.Byte('\n')
		}
		out.Byte('\n')
	}
	if len(index.Unlisted) == 0 {
		return
	}
	out.Str("### Not referenced\n\n")
	out.Str("The wiki holds these pages and its own sidebar does not list them, so this ").
		Str("index has no place to put them. Each one is named rather than dropped.\n\n")
	for _, omission := range index.Unlisted {
		out.Str("- ").Str(omission.Slug).Str(": ").Str(omission.Why).Byte('\n')
	}
	out.Byte('\n')
}
