// Design: website/AI.md -- compact labels distinguish software from evidence.
package site

import (
	"html"
	"regexp"
	"strings"
	"testing"
)

// VALIDATES: both data-driven chip producers classify complete compact labels,
// without coloring protocols, mode claims, mixed labels or surrounding prose.
func TestCompactChipsCarryTheirMeaning(t *testing.T) {
	for _, test := range []struct {
		label string
		class string
	}{
		{"HTMX", "chip concept-software"},
		{"Birdwatcher", "chip concept-software"},
		{"nftables", "chip concept-software"},
		{"DPDK", "chip concept-software"},
		{"GoVPP", "chip concept-software"},
		{"ExaBGP", "chip concept-software"},
		{"Docker", "chip concept-software"},
		{"Netdata", "chip concept-software"},
		{"Prometheus", "chip concept-software"},
		{"Systemd", "chip concept-software"},
		{"Compose", "chip concept-software"},
		{"netlab", "chip concept-software"},
		{"containerlab", "chip concept-software"},
		{"FRR", "chip concept-software"},
		{"BIRD", "chip concept-software"},
		{"GoBGP", "chip concept-software"},
		{"gokrazy", "chip concept-software"},
		{"gokrazy image", "chip concept-software"},
		{"Fuzz", "chip concept-evidence"},
		{"Interop", "chip concept-evidence"},
		{"BGP", "chip"},
		{"YANG", "chip"},
		{"SSE", "chip"},
		{"RFC 5880", "chip"},
		{"ze doctor", "chip"},
		{"systemd-ready", "chip"},
		{"HTMX / SSE", "chip"},
		{"YANG / ExaBGP", "chip"},
		{"Unknown <tool>", "chip"},
	} {
		t.Run(test.label, func(t *testing.T) {
			want := `<span class="` + test.class + `">` + html.EscapeString(test.label) + `</span>`
			feature := featureCardHTML(&featureCard{Title: "Docker and BGP", Category: "operate",
				Chips: []featureChip{{Text: test.label}}, Bullets: []string{"Use Docker with BGP."}})
			audience, err := audienceCardHTML(&audienceCard{Title: "Docker and BGP", Category: "operate",
				Body: "Use Docker with BGP.", Chips: []string{test.label}})
			if err != nil {
				t.Fatal(err)
			}
			for _, body := range []string{feature, audience} {
				if !strings.Contains(body, want) {
					t.Errorf("compact label has no %s", want)
				}
				if !strings.Contains(body, `class="cat">Operate</span>`) {
					t.Error("concept treatment changed the category badge")
				}
				for _, plain := range []string{`<h3 class="concept-`, `<p class="concept-`} {
					if strings.Contains(body, plain) {
						t.Error("concept treatment reached a heading or body")
					}
				}
			}
		})
	}
	mode := featureCardHTML(&featureCard{Category: "operate", Chips: []featureChip{{Text: "Docker", Mode: true}}})
	if !strings.Contains(mode, `<span class="chip mode">Docker</span>`) {
		t.Error("a mode chip acquired a software role")
	}
}

// VALIDATES: homepage proof labels and every interop software name carry roles,
// while the protocol headings and the proof metrics remain plain.
func TestHomeCompactLabelsCarryTheirMeaning(t *testing.T) {
	page, _ := renderHomeFixture(t)
	if !strings.Contains(page, `class="interop-strip-label concept-evidence">Tested against</span>`) {
		t.Error("the interop strip has no evidence label")
	}
	if got := strings.Count(page, `class="proof-check concept-evidence"`); got != 5 {
		t.Errorf("the homepage marks %d proof labels, want 5", got)
	}
	strip := sliceBetween(t, page, `<div class="interop-strip"`, "</section>")
	for _, name := range []string{"FRR", "BIRD", "GoBGP", "OpenBGPd", "FreeRtr", "RustyBGP", "rustbgpd",
		"ExaBGP", "StayRTR", "pmacct", "strongSwan", "accel-ppp", "xl2tpd", "FreeRADIUS", "keepalived"} {
		link := regexp.MustCompile(`<a class="concept-software" href="[^"]+">` + regexp.QuoteMeta(name) + `</a>`)
		if !link.MatchString(strip) {
			t.Errorf("the interop strip lost the software link %q", name)
		}
	}
	if got := strings.Count(strip, `class="concept-software"`); got != 15 {
		t.Errorf("the interop strip marks %d software labels, want all 15", got)
	}
	if !strings.Contains(strip, `<b>BGP</b>`) {
		t.Error("the software role reached a protocol heading")
	}
	if strings.Contains(page, `<strong class="concept-`) {
		t.Error("the evidence role reached a proof metric")
	}
}

// VALIDATES: sidebar labels get their role without changing descriptions,
// destinations, external-link isolation, category headings or fallback text.
func TestSidebarCompactLabelsCarryTheirMeaning(t *testing.T) {
	links := pageLinks{Pages: map[string]pageLinkSpec{"test/": {
		Eyebrow: "Evidence", Groups: []pageLinkGroup{{Title: "Evidence nearby", Links: []pageLink{
			{Href: "quality/", Label: "Quality evidence", Desc: "BGP and Docker results."},
			{External: "frr"}, {Href: "docs/", Label: "FRR docs"},
		}}},
	}}, External: map[string]pageLinkTarget{"frr": {Label: "FRRouting", URL: "https://frrouting.org/"}}}
	body := pageSidebar("../", "test/", links)
	for _, want := range []string{
		`class="page-sidebar-eyebrow concept-evidence">Evidence</p>`,
		`class="page-sidebar-link-label concept-evidence">Quality evidence</span>`,
		`class="page-sidebar-link-label concept-software">FRRouting</span>`,
		`class="page-sidebar-link-label">FRR docs</span>`,
		`href="../quality/"`, `href="https://frrouting.org/" target="_blank" rel="noopener"`,
		`<small>BGP and Docker results.</small>`, `<h2>Evidence nearby</h2>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("sidebar lost %s", want)
		}
	}
}

// VALIDATES: dependency identifiers and vendor table labels have explicit
// software meaning; versions, commands and descriptions keep their own markup.
func TestReferenceSoftwareNamesCarryTheirMeaning(t *testing.T) {
	body := dependencyGroupHTML(dependencyCategory{Name: "Libraries", Modules: []dependencyModule{
		{Module: "example.org/a&b", Why: "A **parser**."},
	}}, map[string]string{"example.org/a&b": "v1.2.3"})
	for _, want := range []string{`<code class="concept-software">example.org/a&amp;b</code>`,
		`<td><code>v1.2.3</code></td>`, `<td>A <strong>parser</strong>.</td>`} {
		if !strings.Contains(body, want) {
			t.Errorf("dependency table lost %s", want)
		}
	}
	mapping := &equivalentMapping{Vendors: map[string]equivalentVendor{"router": {Label: "Router & OS"}}}
	entry := &equivalentEntry{Intent: "Inspect BGP", Vendors: map[string][]equivalentCommand{
		"router": {{Command: "show bgp"}},
	}}
	for _, table := range []string{equivalentTable(mapping, nil, []string{"router"}, false),
		equivalentVendorOnly(mapping, []*equivalentEntry{entry}, []string{"router"})} {
		if !strings.Contains(table, `<th><span class="concept-software">Router &amp; OS</span></th>`) {
			t.Error("vendor table has no escaped software label")
		}
	}
}
