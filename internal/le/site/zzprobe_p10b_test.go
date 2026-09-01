package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProbeUnclaimedRoutes runs every registered producer over the real
// checkout and reports the published routes none of them wrote. It is a
// measurement probe and is deleted before the phase ends.
func TestProbeUnclaimedRoutes(t *testing.T) {
	root := repositoryRoot(t)
	output := t.TempDir()
	published := filepath.Join(root, "tmp", "session",
		"2026-08-29-36ab4fc7-fd4f-4e52-892c-be2eaf79ddcd", "scratch", "p10", "pub")
	copyScratchTree(t, filepath.Join(published, "data"), filepath.Join(output, "data"))
	paths := Paths{Repository: root, Source: filepath.Join(root, "website"), Output: output}
	written := map[string]int{}
	var report strings.Builder
	for _, producer := range allProducers() {
		routes, err := producer.Render(paths)
		if err != nil {
			report.WriteString("FAILED " + producer.Name + ": " + err.Error() + "\n")
			continue
		}
		for _, route := range routes {
			written[route]++
		}
	}
	// The docs producer and the homepage refuse in this checkout, because the
	// artifact tree's assets/demos is absent (R-8). Their destinations are counted
	// from the page list instead, which is how the phase 10 census counted
	// them, so the two failures do not read as 150 unclaimed routes.
	docsPages, err := docsProducerPages()
	if err != nil {
		t.Fatalf("docs pages: %v", err)
	}
	for _, page := range docsPages {
		written["/"+strings.TrimSuffix(page.Dest, pageIndexFile)]++
	}
	written["/"]++

	listing, err := os.ReadFile(filepath.Join(root, "internal", "le", "site", "testdata", "published-routes.txt"))
	if err != nil {
		t.Fatalf("published routes: %v", err)
	}
	unclaimed := 0
	for route := range strings.FieldsSeq(string(listing)) {
		if written[route] == 0 {
			unclaimed++
			report.WriteString("UNCLAIMED " + route + "\n")
		}
	}
	for route, count := range written {
		if count > 1 {
			report.WriteString("DOUBLED " + route + "\n")
		}
	}
	t.Logf("PROBE unclaimed=%d written=%d\n%s", unclaimed, len(written), report.String())
}

// TestProbeSharedHeader writes the rendered header fragment to scratch, for a
// byte comparison against the published one. It is deleted before the phase
// ends.
func TestProbeSharedHeader(t *testing.T) {
	root := repositoryRoot(t)
	var data siteNav
	if err := readSourceJSON(filepath.Join(root, "website"), navDataFile, &data); err != nil {
		t.Fatal(err)
	}
	facts := siteFacts{
		CLICommands: 402, ConfigSections: 36, Dependencies: 42, GitHubStars: 50,
		Features: factsFeatures{CoreExperimental: 52},
	}
	header, err := sharedHeaderHTML(data, &facts)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "tmp", "session",
		"2026-08-29-36ab4fc7-fd4f-4e52-892c-be2eaf79ddcd", "scratch", "p10c", "header-go.html")
	if err := os.WriteFile(target, []byte(header), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestProbeAuthoredPages renders the authored pages of the real checkout into
// scratch, for comparison against the published ones. It is deleted before the
// phase ends.
func TestProbeAuthoredPages(t *testing.T) {
	root := repositoryRoot(t)
	source := filepath.Join(root, "website")
	output := filepath.Join(root, "tmp", "session",
		"2026-08-29-36ab4fc7-fd4f-4e52-892c-be2eaf79ddcd", "scratch", "p10c", "authored")
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := renderAuthoredPages(Paths{Repository: root, Source: source, Output: output}); err != nil {
		t.Fatal(err)
	}
}
