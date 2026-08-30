// Design: website/AI.md -- a public URL that moved keeps answering at its old address
package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: AC-12 -- the 177 legacy replacements run in the recorded ORDER,
// each over the output of the one before it, so a route that moved twice
// reaches its current address in one pass.
//
// The method states the same two moves in both orders. Recorded first-to-last,
// a→b then b→c takes a URL at a all the way to c. Recorded the other way round,
// b→c runs before anything has produced a b, and the URL stops at b. The two
// answers differ, which is what makes the slice a slice and not a map.
func TestRedirectsApplyInTheRecordedOrder(t *testing.T) {
	chained := []legacyRoute{{From: "a", To: "b"}, {From: "b", To: "c"}}
	reversed := []legacyRoute{{From: "b", To: "c"}, {From: "a", To: "b"}}
	text := "see " + siteBase + "a/ for more"

	if got, want := rewriteLegacyPublicURLs(text, chained), "see "+siteBase+"c/ for more"; got != want {
		t.Errorf("the recorded order answered %q, want %q", got, want)
	}
	if got, want := rewriteLegacyPublicURLs(text, reversed), "see "+siteBase+"b/ for more"; got != want {
		t.Errorf("the reversed order answered %q, want %q", got, want)
	}
}

// VALIDATES: AC-12 -- the recovered table names a target for every retired
// address a source still links, and the table this build derives carries them.
func TestTheLegacyTableCarriesTheAddressesTheSourcesStillLink(t *testing.T) {
	root := repositoryRoot(t)
	routes, err := legacyRoutes(filepath.Join(root, "website"))
	if err != nil {
		t.Fatal(err)
	}
	targets := make(map[string]string, len(routes))
	for _, route := range routes {
		targets[route.From] = route.To
	}
	// The five absolute URLs docs/history.md carries today. Each one is a
	// retired address: a reader following it reaches a redirect stub, and a
	// machine reading the Markdown mirror reaches nothing at all.
	for from, want := range map[string]string{
		"changes":                changesDirectory,
		"milestones":             "project/milestones",
		"roadmap":                "project/roadmap",
		"usage/exabgp-migration": "use-cases/exabgp-migration",
		"docs/guide/bgp-peering": "guides/bgp-peering",
	} {
		if got := targets[from]; got != want {
			t.Errorf("the table moves %q to %q, want %q", from, got, want)
		}
	}
}

// VALIDATES: AC-12 -- the rewrite reaches the ARTIFACT.
//
// rewriteLegacyPublicURLs was written and never called: the pass the retired
// build ran over every page and every mirror (website/tools/build.py,
// step_links) had no Go caller, so five absolute URLs in docs/history.md were
// published pointing at addresses that only a redirect stub answers.
//
// The method writes a page and a mirror carrying one retired address, plus a
// frozen talk deck carrying the same one, and requires the first two to be
// rewritten and the deck to be left exactly as its author wrote it.
func TestTheLegacyRewriteReachesEveryPageAndMirror(t *testing.T) {
	root := repositoryRoot(t)
	output := t.TempDir()
	link := siteBase + "changes/"
	moved := siteBase + changesDirectory + "/"

	writeArtifactFile(t, output, "project/history/index.html",
		`<!doctype html><html><body><a href="`+link+`">changes</a></body></html>`)
	writeArtifactFile(t, output, "project/history/index.md",
		"# History\n\nThe [weekly changes]("+link+") hold the record.\n")
	writeArtifactFile(t, output, "talks/demo/index.html",
		`<!doctype html><html><body><a href="`+link+`">changes</a></body></html>`)

	paths := Paths{Repository: root, Source: filepath.Join(root, "website"), Output: output}
	rewritten, err := rewriteArtifactLegacyURLs(paths)
	if err != nil {
		t.Fatal(err)
	}
	if rewritten != 2 {
		t.Errorf("the pass rewrote %d files; the page and its mirror are two", rewritten)
	}
	for _, name := range []string{"project/history/index.html", "project/history/index.md"} {
		content := readArtifact(t, output, name)
		if strings.Contains(content, link) {
			t.Errorf("%s still carries the retired address %s", name, link)
		}
		if !strings.Contains(content, moved) {
			t.Errorf("%s does not carry the address it moved to, %s", name, moved)
		}
	}
	deck := readArtifact(t, output, "talks/demo/index.html")
	if !strings.Contains(deck, link) {
		t.Error("a frozen talk deck was rewritten; a deck is published exactly as its author wrote it")
	}
}

// VALIDATES: AC-12 -- the rewrite runs BETWEEN the page producers and the
// producers that read what they wrote.
//
// A mirror the search index and llms-full.txt read after the rewrite carries
// the address a reader reaches. Ordered the other way, both would inline the
// retired one and the site would publish two answers for one link.
func TestTheLegacyRewriteRunsBeforeTheDerivedProducers(t *testing.T) {
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, changesSourceDirectory), 0o755); err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	link := siteBase + "roadmap/"
	moved := siteBase + "project/roadmap/"

	page := Producer{Name: "test-page", Render: func(paths Paths) ([]string, error) {
		return nil, writeNamedArtifact(paths.Output, "docs/index.md",
			"# Docs\n\nSee the [roadmap]("+link+").\n")
	}}
	seen := ""
	reader := Producer{Name: "test-reader", Render: func(paths Paths) ([]string, error) {
		content, err := os.ReadFile(filepath.Join(paths.Output, "docs", "index.md"))
		seen = string(content)
		return nil, err
	}}

	registeredProducers = []Producer{page}
	derivedProducers = []Producer{reader}
	t.Cleanup(func() { registeredProducers, derivedProducers = nil, nil })

	if _, err := renderProducers(Paths{Repository: t.TempDir(), Source: source, Output: output}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(seen, link) {
		t.Errorf("the derived producer read the retired address; the rewrite must run before it:\n%s", seen)
	}
	if !strings.Contains(seen, moved) {
		t.Errorf("the derived producer read %q, which carries neither address", seen)
	}
}
