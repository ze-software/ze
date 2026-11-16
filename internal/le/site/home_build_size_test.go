// Design: website/AI.md -- approved compile-time build selection panel.
package site

import (
	"slices"
	"strings"
	"testing"
)

// The build-size message belongs with deployment choices. Method: read the
// homepage template, check its sole panel follows the introduction and precedes
// the generated cards, then inspect the shared styles for the approved treatment.
func TestHomepageBuildSizePanelStaysWithTrialChoices(t *testing.T) {
	section := sliceBetween(t, homeTemplate, `<section class="home-section-panel home-section-panel-run`, "</section>")
	panel := sliceBetween(t, section, `<aside class="build-size"`, "</aside>")
	for _, text := range []string{
		`aria-labelledby="build-size-title"`,
		`<h3 id="build-size-title">Build Ze to fit your needs.</h3>`,
		"Choose which optional subsystems to include at compile time, from a focused service to a full network OS.",
		"Subsystems you leave out are excluded from the binary.",
	} {
		if !strings.Contains(panel, text) {
			t.Errorf("build-size panel omits approved copy %q", text)
		}
	}
	if got := strings.Count(homeTemplate, `<aside class="build-size"`); got != 1 {
		t.Errorf("homepage has %d build-size panels, want one", got)
	}
	if got := strings.Count(homeTemplate, "Build Ze to fit your needs."); got != 1 {
		t.Errorf("homepage repeats the build-size heading %d times, want one", got)
	}
	introEnd := strings.Index(section, "</div>")
	panelStart := strings.Index(section, `<aside class="build-size"`)
	if panelStart < introEnd {
		t.Error("build-size panel must follow the trial-choice introduction")
	}
	cardsStart := strings.Index(section, "{run_cards}")
	if cardsStart < 0 {
		t.Fatal("trial choices omit the generated deployment cards")
	}
	if cardsStart < panelStart {
		t.Error("build-size panel must precede the generated deployment cards")
	}
	for _, want := range []struct {
		selector string
		style    string
	}{
		{".site-home .build-size", "background: var(--panel)"},
		{".site-home .build-size", "border: 1px solid var(--line)"},
		{".site-home .build-size h3", "color: var(--blue)"},
		{".site-home .build-size p", "color: var(--muted)"},
	} {
		var found bool
		for _, rule := range websiteStyleRules(t) {
			if slices.Contains(rule.selectors, want.selector) {
				if strings.Contains(rule.body, want.style) {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("build-size panel lacks approved style %s { %s }", want.selector, want.style)
		}
	}
}
