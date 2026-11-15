// Design: website/AI.md -- the public homepage preserves the approved reading order.
package site

import (
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"
)

// TestHomepagePutsManagementBeforeEvidence follows the complete rendered page
// to ensure getting started precedes management, which precedes evidence.
func TestHomepagePutsManagementBeforeEvidence(t *testing.T) {
	page, _ := renderHomeFixture(t)
	previous := -1
	for _, marker := range []string{`class="hero"`, `id="try"`, `id="why-ze"`, `id="proof"`, `id="features-summary"`} {
		at := strings.Index(page, marker)
		if at < 0 {
			t.Fatalf("homepage lacks %s", marker)
		}
		if at <= previous {
			t.Errorf("homepage puts %s before the preceding section", marker)
		}
		previous = at
	}
	if strings.Contains(page, `class="whats-new reveal"`) {
		t.Error("the homepage restored the Latest news band the owner removed")
	}
}

// TestHomepageRunChoicesKeepFiveRows checks direct element children rather
// than CSS output: every authored choice keeps its badge, title, description,
// success check and action in the same row positions.
func TestHomepageRunChoicesKeepFiveRows(t *testing.T) {
	page, _ := renderHomeFixture(t)
	root, err := xhtml.Parse(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	grid := metricNodeWithClass(root, "index4-choice-grid")
	if grid == nil {
		t.Fatal("homepage has no trial-choice grid")
	}
	cards := 0
	for card := grid.FirstChild; card != nil; card = card.NextSibling {
		if card.Type != xhtml.ElementNode {
			continue
		}
		cards++
		var rows []*xhtml.Node
		for row := card.FirstChild; row != nil; row = row.NextSibling {
			if row.Type == xhtml.ElementNode {
				rows = append(rows, row)
			}
		}
		if len(rows) != 5 {
			t.Fatalf("trial choice %d has %d rows, want 5", cards, len(rows))
		}
		for index, tag := range []string{"div", "h3", "p", "p"} {
			if rows[index].Data != tag {
				t.Errorf("trial choice %d row %d = %s, want %s", cards, index+1, rows[index].Data, tag)
			}
		}
		if metricNodeWithClass(rows[0], "index4-choice-badge") == nil {
			t.Errorf("trial choice %d has no badge wrapper", cards)
		}
		if metricNodeWithClass(rows[3], "index4-success") == nil {
			t.Errorf("trial choice %d has no success check", cards)
		}
		if rows[4].Data != "a" && rows[4].Data != "div" {
			t.Errorf("trial choice %d has no final action row", cards)
		}
	}
	if cards != 4 {
		t.Errorf("homepage has %d trial choices, want 4", cards)
	}
	for _, href := range []string{"guides/quickstart/", "use-cases/exabgp-migration/", "guides/ubuntu-build-install/", "guides/docker/", "guides/appliance/", "guides/netlab/", "labs/netlab/", "labs/bgp-interop/", "guides/ai-agent/", "reference/cli/", "guides/public-looking-glass/", "guides/ze-install/#choose-a-deployment-path"} {
		section := sliceBetween(t, page, `<section class="home-section-panel home-section-panel-run`, `</section>`)
		if !strings.Contains(section, `href="`+href+`"`) {
			t.Errorf("trial choices lost destination %s", href)
		}
	}
}

// TestHomepageOtherProtocolsStayVisible exercises the snapshot boundary with
// an unknown protocol bucket: nonzero inventory reaches readers with its marker.
func TestHomepageOtherProtocolsStayVisible(t *testing.T) {
	for _, scenario := range []struct {
		facts   string
		visible bool
	}{
		{`{"interop":{"protocols":{"other":4}}}`, true},
		{`{"interop":{"protocols":{"other":0}}}`, false},
		{`{"interop":{"protocols":{}}}`, false},
	} {
		snapshot, err := parseSiteFactSnapshot([]byte(scenario.facts))
		if err != nil {
			t.Fatal(err)
		}
		row, err := homeInteropOtherRow(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(row, markedFact("interop.protocols.other", "4")); got != scenario.visible {
			t.Errorf("Other protocols row = %q for %s", row, scenario.facts)
		}
	}
}

// TestHomepageRunFieldsEscapeAuthoredText renders a choice with markup-like
// content so new success and secondary-link fields cannot create HTML.
func TestHomepageRunFieldsEscapeAuthoredText(t *testing.T) {
	card := audienceCard{Title: "<title>", Label: "<badge>", Body: "<body>",
		SuccessLabel: "<success>", Success: "<check>", Recommendation: "<recommended>",
		Link:          &audienceLink{Href: "guide/?a=1&b=2", Label: "<primary>"},
		SecondaryLink: &audienceLink{Href: "other/?a=1&b=2", Label: "<secondary>"}}
	fragment, err := homeRunCards([]audienceCard{card})
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"&lt;title&gt;", "&lt;badge&gt;", "&lt;body&gt;", "&lt;success&gt;", "&lt;check&gt;", "&lt;recommended&gt;", "&lt;primary&gt;", "&lt;secondary&gt;", "?a=1&amp;b=2"} {
		if !strings.Contains(fragment, text) {
			t.Errorf("trial choice did not escape %s", text)
		}
	}
}

// TestHomepageOmitsRetiredAudienceSection checks the public page no longer
// renders the removed audience grid while retaining the authored Run choices.
func TestHomepageOmitsRetiredAudienceSection(t *testing.T) {
	page, _ := renderHomeFixture(t)
	for _, retired := range []string{"home-section-panel-usecases", `id="audience-title"`, "Safe ways to try Ze before the first release."} {
		if strings.Contains(page, retired) {
			t.Errorf("homepage retains removed audience section %q", retired)
		}
	}
	if !strings.Contains(page, `id="run-title">Choose how to try Ze.</h2>`) {
		t.Error("homepage lost the current trial choices")
	}
}

// TestHomepageGuideKeepsItsBox checks the guide has an inner card while the
// outer category element remains available for the shared top separator.
func TestHomepageGuideKeepsItsBox(t *testing.T) {
	page, _ := renderHomeFixture(t)
	root, err := xhtml.Parse(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	guide := metricNodeWithClass(root, "capability-guide")
	if guide == nil {
		t.Fatal("homepage has no guide category")
	}
	card := metricNodeWithClass(guide, "capability-guide-card")
	if card == nil {
		t.Fatal("homepage guide has no inner card")
	}
	if card.Parent != guide {
		t.Error("homepage guide card is not directly inside the category")
	}
	for _, target := range []string{"docs/", "features/"} {
		if !strings.Contains(sliceBetween(t, page, `<div class="capability-guide-card">`, "</aside>"), `href="`+target+`"`) {
			t.Errorf("homepage guide card lost %s", target)
		}
	}
}

// TestHomepageOmitsRetiredSafetySection checks the bottom trial reminder is
// removed while the hero still carries its release state and planned date.
func TestHomepageOmitsRetiredSafetySection(t *testing.T) {
	page, _ := renderHomeFixture(t)
	for _, retired := range []string{`id="try-safely"`, "home-section-panel-safe", "Try Ze before the first release."} {
		if strings.Contains(page, retired) {
			t.Errorf("homepage retains removed safety section %q", retired)
		}
	}
	for _, retained := range []string{"No stable release yet.", "Planned release", "Q4 2026"} {
		if !strings.Contains(page, retained) {
			t.Errorf("homepage lost hero release information %q", retained)
		}
	}
}
