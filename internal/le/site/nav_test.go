// Design: website/AI.md -- one shared header fragment serves every published page
package site

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// publishedHeaderFacts are the numbers the published header fragment was
// rendered with, read from the site-facts snapshot published beside it at
// gh-pages 2fa8fa2ad. They are stated here so a golden comparison measures the
// renderer rather than this checkout's own counts.
var publishedHeaderFacts = siteFacts{
	CLICommands:    402,
	ConfigSections: 36,
	Dependencies:   42,
	GitHubStars:    50,
	Features:       factsFeatures{CoreExperimental: 52},
}

// siteNavigation reads website/data/nav.json from the real checkout, which is
// the menu the published fragment was rendered from.
func siteNavigation(t *testing.T) siteNav {
	t.Helper()
	var data siteNav
	if err := readSourceJSON(filepath.Join(repositoryRoot(t), "website"), navDataFile, &data); err != nil {
		t.Fatalf("read the navigation data: %v", err)
	}
	return data
}

// renderSiteHeader renders the fragment from the real navigation data and the
// numbers the published fragment carries.
func renderSiteHeader(t *testing.T) string {
	t.Helper()
	header, err := sharedHeaderHTML(siteNavigation(t), &publishedHeaderFacts)
	if err != nil {
		t.Fatalf("render the shared header: %v", err)
	}
	return header
}

// VALIDATES: AC-16, for assets/header.html. The fragment carries every entry
// website/data/nav.json declares.
//
// The header is the one surface no page test can see: it is loaded at run time,
// so a page renders correctly while the menu it loads is empty. The method is
// to walk the data and ask the fragment for each entry.
func TestTheSharedHeaderCarriesEveryNavigationEntry(t *testing.T) {
	data := siteNavigation(t)
	header := renderSiteHeader(t)

	entries := 0
	for _, dropdown := range data.Dropdowns {
		if !strings.Contains(header, ">"+dropdown.Label+"\n") {
			t.Errorf("the header carries no %s dropdown", dropdown.Label)
		}
		for _, column := range dropdown.Columns {
			for _, entry := range column {
				if entry.LabelOnly != "" {
					if !strings.Contains(header, `<span class="nav-dropdown-label">`+entry.LabelOnly+"</span>") {
						t.Errorf("the header carries no %q column heading", entry.LabelOnly)
					}
					continue
				}
				entries++
				if !strings.Contains(header, `href="`+sharedHeaderRootPlaceholder+entry.Href+`"`) {
					t.Errorf("the header links no %s", entry.Href)
				}
				if !strings.Contains(header, "<strong>"+entry.Title+"</strong>") {
					t.Errorf("the header names no %q", entry.Title)
				}
			}
		}
	}
	if count := strings.Count(header, `class="nav-dropdown-item`); count != entries {
		t.Errorf("the header carries %d menu entries, and nav.json declares %d", count, entries)
	}
}

// VALIDATES: AC-16, the property that lets ONE fragment serve every page.
//
// The fragment spells the site root as a placeholder that assets/site.js substitutes
// for the mounting page's own root. A fragment rendered with a resolved root
// would link correctly from one depth and break from every other, which no page
// test would see because the page carries the mount rather than the menu.
func TestTheSharedHeaderSpellsTheSiteRootAsAPlaceholder(t *testing.T) {
	header := renderSiteHeader(t)

	if !strings.Contains(header, sharedHeaderRootPlaceholder+"assets/ze.svg") {
		t.Errorf("the header resolves the site root rather than leaving the placeholder for site.js")
	}
	if strings.Contains(header, `href="../`) || strings.Contains(header, `href="./`) {
		t.Errorf("the header links a relative path, which is right for one depth alone")
	}
	if strings.Contains(header, `href="`+siteBase) {
		t.Errorf("the header links an absolute site URL, which leaves the site to come back to it")
	}
}

// VALIDATES: AC-11's numbers reaching the one surface that shows them on every
// page. The menu states the live counts and the star number.
//
// nav.json states each count as a placeholder, so a renderer that never
// substituted them would publish "%(features)s features" in the menu of all 712
// pages.
func TestTheSharedHeaderStatesTheCountsTheSnapshotPublishes(t *testing.T) {
	header := renderSiteHeader(t)

	for _, phrase := range []string{
		"<small>52 features, grouped by category</small>",
		"<small>42 direct packages, generated from go.mod</small>",
		"<small>402 commands, generated from Ze&#39;s live binary</small>",
		"<small>36 sections, indexed from live YANG</small>",
		`aria-label="Ze on GitHub, 50 stars"`,
		`<span class="nav-badge-count">50</span>`,
	} {
		if !strings.Contains(header, phrase) {
			t.Errorf("the header states no %s", phrase)
		}
	}
	if strings.Contains(header, "%(") {
		t.Errorf("the header publishes a placeholder rather than a count")
	}
}

// VALIDATES: a menu the reader can open and find nothing in is refused.
//
// The retired renderer had one dropdown whose entries were generated rather
// than declared. A generator that goes away leaves a labeled panel with no
// entry, and nothing reads a menu, so the refusal is the only thing that would
// say so.
func TestTheSharedHeaderRefusesADropdownWithNoEntry(t *testing.T) {
	data := siteNav{Dropdowns: []navDropdown{{Label: "Blog"}}}

	_, err := sharedHeaderHTML(data, &publishedHeaderFacts)
	if err == nil {
		t.Fatalf("an empty dropdown was published rather than refused")
	}
	if !strings.Contains(err.Error(), "Blog") {
		t.Errorf("the refusal is %q, and it must name the dropdown", err)
	}
}

// VALIDATES: a count this build cannot answer is refused by name, rather than
// published as the placeholder itself.
func TestTheSharedHeaderRefusesACountItCannotAnswer(t *testing.T) {
	data := siteNav{Dropdowns: []navDropdown{{
		Label:   "Reference",
		Columns: [][]navEntry{{{Href: "reference/peers/", Title: "Peers", Desc: "%(peers)s peers"}}},
	}}}

	_, err := sharedHeaderHTML(data, &publishedHeaderFacts)
	if err == nil {
		t.Fatalf("an unanswerable count was published rather than refused")
	}
	for _, phrase := range []string{"Peers", "%(peers)s"} {
		if !strings.Contains(err.Error(), phrase) {
			t.Errorf("the refusal is %q, and it must name %s", err, phrase)
		}
	}
}

// VALIDATES: AC-4's parity target, for the header fragment.
//
// The fragment published at gh-pages 2fa8fa2ad is the golden. The one
// difference this comparison allows is the spelling of an apostrophe: Python
// wrote &#x27; where Go writes &#39;, and the owner ruled on 2026-08-29 that a
// character reference a reader cannot tell apart is not a difference.
func TestTheSharedHeaderReadsAsThePublishedHeader(t *testing.T) {
	header := renderSiteHeader(t)
	published := strings.ReplaceAll(readTestdata(t, "published-header.html"), "&#x27;", "&#39;")

	if header != published {
		t.Errorf("the rendered header differs from the published one:\n%s",
			firstDifference(header, published))
	}
}

// VALIDATES: AC-16. The header producer writes the named artifact and claims no
// route, so `./le site check` stops reporting assets/header.html as absent.
//
// The fragment was ownerless from phase 2 to here: every page mounted it, no
// producer wrote it, and only the seed carried it from one build to the next.
func TestTheHeaderProducerWritesTheNamedArtifact(t *testing.T) {
	root := repositoryRoot(t)
	output := t.TempDir()
	facts, err := os.ReadFile(filepath.Join("testdata", "published-header-facts.json"))
	if err != nil {
		t.Fatalf("read the facts fixture: %v", err)
	}
	if err := writeNamedArtifact(output, factsFile, string(facts)); err != nil {
		t.Fatalf("publish the facts snapshot: %v", err)
	}

	routes, err := renderSharedHeader(Paths{Repository: root, Source: filepath.Join(root, "website"), Output: output})
	if err != nil {
		t.Fatalf("render the shared header: %v", err)
	}

	if len(routes) != 0 {
		t.Errorf("the header producer claims %v, and a fragment is not a route", routes)
	}
	fragment := readArtifact(t, output, sharedHeaderFile)
	if !strings.Contains(fragment, `<header class="site-header">`) {
		t.Errorf("the published fragment is not a header: %.80q", fragment)
	}
	if !strings.Contains(fragment, `<span class="nav-badge-count">`+strconv.Itoa(publishedHeaderFacts.GitHubStars)+"</span>") {
		t.Errorf("the published fragment states a star count the snapshot does not")
	}
	for _, name := range checkNamedArtifacts(output) {
		if name == sharedHeaderFile {
			t.Errorf("%s is still reported absent after its producer ran", name)
		}
	}
}
