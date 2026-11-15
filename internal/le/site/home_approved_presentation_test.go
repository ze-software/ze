// Design: website/AI.md -- approved hero ticks and capability-directory presentation
package site

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Every hero key point receives the approved tick without repeating the glyph
// in authored copy or splitting a tick from its label when the row wraps.
func TestHeroKeyPointsUseApprovedTicks(t *testing.T) {
	row := sliceBetween(t, homeTemplate, `<p class="hero-start-points"`, "</p>")
	points := regexp.MustCompile(`<span>([^<]*)</span>`).FindAllStringSubmatch(row, -1)
	want := []string{"Self-contained", "Extend with plugins", "Appliance option", "ExaBGP compatible"}
	var got []string
	for _, point := range points {
		got = append(got, point[1])
	}
	if !slices.Equal(got, want) {
		t.Fatalf("hero key points = %v, want %v", got, want)
	}
	var tick, nowrap bool
	for _, rule := range websiteStyleRules(t) {
		if slices.Contains(rule.selectors, ".site-home .hero-start-points span::before") &&
			strings.Contains(rule.body, `content: "✓"`) && strings.Contains(rule.body, "color: #87d5fa") {
			tick = true
		}
		if slices.Contains(rule.selectors, ".site-home .hero-start-points span") &&
			strings.Contains(rule.body, "white-space: nowrap") {
			nowrap = true
		}
	}
	if !tick || !nowrap {
		t.Errorf("hero key points need a shared blue tick and an unbroken label: tick=%t nowrap=%t", tick, nowrap)
	}
}

// The approved capability treatment keeps the generated inventory and useful
// reference destinations while making maturity and lookup paths explicit.
func TestHomepageCapabilitiesRetainGeneratedDestinations(t *testing.T) {
	paths := homeFixture(t)
	if _, err := renderHome(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, homeDest)
	section := sliceBetween(t, page, `<section id="features-summary"`, "</section>")
	for _, text := range []string{
		"Capabilities and current limitations.",
		`data-ze-stat="cli_commands">499</span> CLI commands`,
		`data-ze-stat="plugin_registrations">96</span> registered plugins`,
		"The YANG tree", "next website build updates the reference",
	} {
		if !strings.Contains(section, text) {
			t.Errorf("capability section omits %q", text)
		}
	}
	targets := linkTargets(section)
	for _, target := range []string{"reference/configuration/", "reference/cli/", "reference/plugins/", "reference/feature-status/", "docs/", "features/"} {
		if !slices.Contains(targets, target) {
			t.Errorf("capability section omits destination %s", target)
		}
	}
	features, err := loadFeatureData(paths)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(section, homeFeatureOverview(features)) {
		t.Error("capability section does not preserve the complete generated feature directory")
	}
	var fourColumns bool
	for _, rule := range websiteStyleRules(t) {
		if slices.Contains(rule.selectors, ".site-home .home-section-panel-map .capability-grid") &&
			strings.Contains(rule.body, "repeat(4, minmax(0, 1fr))") {
			fourColumns = true
		}
	}
	if !fourColumns {
		t.Error("capability directory has no approved four-column desktop layout")
	}
}

// The Why Ze reason cards carry the four arguments Thomas approved, in order,
// each with its destination. Method: read the card row from the homepage
// template and compare every card's label, title and link.
func TestWhyZeCardsCarryApprovedArguments(t *testing.T) {
	row := sliceBetween(t, homeTemplate, `<div class="cards usp-grid reveal"`, "</section>")
	cards := regexp.MustCompile(`(?s)<span class="cat">([^<]*)</span>\s*<h3><a href="([^"]*)">([^<]*)</a></h3>`).FindAllStringSubmatch(row, -1)
	want := [][3]string{
		{"ExaBGP", "features/exabgp-compatibility/", "ExaBGP on steroids"},
		{"Peering", "guides/irr-filtering/", "Made for operators"},
		{"Deployment", "guides/ze-install/", "Run it the way you want"},
		{"AI", "guides/ai-agent/", "Made for AI assistants"},
	}
	var got [][3]string
	for _, card := range cards {
		got = append(got, [3]string{card[1], card[2], card[3]})
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Why Ze cards = %v, want %v", got, want)
	}
	words := strings.Join(strings.Fields(row), " ")
	for _, text := range []string{
		"Used as a controller, Ze behaves like ExaBGP",
		"Its native API offers much more",
		"finds the AS-SET in PeeringDB",
		"a Docker container, or a node in a containerlab topology",
		"describe the symptom",
	} {
		if !strings.Contains(words, text) {
			t.Errorf("Why Ze cards omit approved argument %q", text)
		}
	}
}
