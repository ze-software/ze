// Design: website/AI.md -- approved deployment choices and homepage reading order.
package site

import (
	"path/filepath"
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// The trial choices immediately follow Getting started. Method: inspect the
// rendered page between those sections and reject any intervening section.
func TestHomepageTrialChoicesFollowGettingStarted(t *testing.T) {
	page, _ := renderHomeFixture(t)
	between := sliceBetween(t, page, `<section id="try"`, `<section class="home-section-panel home-section-panel-run`)
	if strings.Count(between, "<section") != 1 {
		t.Fatal("another section separates Getting started from the trial choices")
	}
}

// The published choices use the approved deployment names and destinations.
// Method: render the real authored data instead of the historical fixture.
func TestHomepageDeploymentChoicesUseApprovedPaths(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	paths := homeFixture(t)
	copyFixture(t, filepath.Join(root, "website", "data", audienceDataFile),
		filepath.Join(paths.Source, "data", audienceDataFile))
	if _, err := renderHome(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, homeDest)
	section := sliceBetween(t, page, `<section class="home-section-panel home-section-panel-run`, `</section>`)
	cards := strings.Split(section, `<article class="index4-choice`)[1:]
	want := []struct {
		label string
		href  string
		role  string
	}{
		{"Linux", "guides/ubuntu-build-install/", "use your host as a router"},
		{"Docker", "guides/docker/", "routing daemon in a container"},
		{"Appliance", "guides/appliance/", "dedicated router on hardware or a VM"},
		{"Containerlab", "guides/netlab/", "test network"},
	}
	if len(cards) != len(want) {
		t.Fatalf("homepage has %d deployment choices, want %d", len(cards), len(want))
	}
	for index, choice := range want {
		card := strings.SplitN(cards[index], "</article>", 2)[0]
		for _, text := range []string{`class="index4-choice-badge">` + choice.label + `</span>`, `href="` + choice.href + `"`, choice.role} {
			if !strings.Contains(card, text) {
				t.Errorf("%s choice omits %q", choice.label, text)
			}
		}
		if index < 2 && !strings.Contains(card, `href="use-cases/exabgp-migration/"`) {
			t.Errorf("%s choice omits the ExaBGP migration path", choice.label)
		}
	}
	if !strings.Contains(section, `href="guides/quickstart/"`) {
		t.Error("deployment section lost the two-peer quickstart")
	}
}
