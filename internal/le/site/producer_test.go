// Design: website/AI.md -- every published route is written by one named producer
package site

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// stubProducers replaces the producer registry with the given producers and
// restores the compiled-in registry when the test ends.
func stubProducers(t *testing.T, chosen ...Producer) {
	t.Helper()
	previousPages, previousDerived := registeredProducers, derivedProducers
	t.Cleanup(func() { registeredProducers, derivedProducers = previousPages, previousDerived })
	registeredProducers, derivedProducers = nil, nil
	for _, producer := range chosen {
		registerProducer(producer)
	}
}

// writeProducerPage writes one published page at the given artifact-relative
// directory, as a producer would.
func writeProducerPage(t *testing.T, output, directory string) {
	t.Helper()
	path := filepath.Join(output, filepath.FromSlash(directory), "index.html")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("<!doctype html><html><body>page</body></html>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// siteFixture lays out one checkout a build can read: a website source tree
// with a single staged file, tracked by git, and a published artifact beside it
// carrying the asset bundles a build seeds rather than renders. It answers the
// repository root and the artifact output.
func siteFixture(t *testing.T) (root, output string) {
	t.Helper()
	parent := t.TempDir()
	root = filepath.Join(parent, "main")
	source := filepath.Join(root, "website")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "CNAME"), []byte("example.test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeSourceAssets(t, source)
	assets := filepath.Join(parent, "gh-pages", "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "site.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "site.js"), []byte("\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"init"}, {"add", "."}} {
		command := exec.CommandContext(t.Context(), "git", arguments...)
		command.Dir = root
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, out)
		}
	}
	return root, filepath.Join(parent, "artifact")
}

// VALIDATES: a build runs every registered producer, in registration order, and
// records the route each one wrote. Nothing in the retired build could name its
// render steps, which is why five surfaces out of thirty-eight reported success.
func TestBuildRunsEveryRegisteredProducer(t *testing.T) {
	stubLiveInputs(t, `[{"path":"show test","description":"Show rows","mode":"read-only"}]`)
	var ran []string
	stubProducers(t,
		Producer{Name: "labs", Render: func(paths Paths) ([]string, error) {
			ran = append(ran, "labs")
			writeProducerPage(t, paths.Output, "labs")
			return []string{"/labs/"}, nil
		}},
		Producer{Name: "guides", Render: func(paths Paths) ([]string, error) {
			ran = append(ran, "guides")
			writeProducerPage(t, paths.Output, "guides")
			return []string{"/guides/"}, nil
		}},
	)
	root, output := siteFixture(t)

	report, err := Build(BuildOptions{Repository: root, Output: output})
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(ran, []string{"labs", "guides"}) {
		t.Fatalf("producers ran as %v, want every producer in registration order", ran)
	}
	if report.Coverage.Producers != 2 || report.Coverage.Written != 2 {
		t.Fatalf("unexpected coverage: %#v", report.Coverage)
	}
	for _, route := range []string{"/labs/", "/guides/"} {
		if slices.Contains(report.Coverage.Unclaimed, route) {
			t.Fatalf("%s was written by a producer and reported unclaimed: %v", route, report.Coverage.Unclaimed)
		}
	}
	if _, err := os.Stat(filepath.Join(output, "data", "site-producers.json")); !os.IsNotExist(err) {
		t.Fatalf("the build wrote its bookkeeping into the published artifact: %v", err)
	}
	claims, err := readProducerRecord(Paths{Repository: root, Output: output})
	if err != nil {
		t.Fatal(err)
	}
	want := []Claim{{Route: "/guides/", Producers: []string{"guides"}}, {Route: "/labs/", Producers: []string{"labs"}}}
	if len(claims) != len(want) {
		t.Fatalf("recorded claims %v, want %v", claims, want)
	}
	for index, claim := range claims {
		if claim.Route != want[index].Route || !slices.Equal(claim.Producers, want[index].Producers) {
			t.Fatalf("recorded claims %v, want %v", claims, want)
		}
	}
}

// VALIDATES: a published route that no producer wrote fails the check by name.
// A page carried forward by the seed is exactly this route: it has fresh mtimes
// and frozen content, and every other check the artifact carries passes it.
func TestCheckRefusesAPublishedRouteWithNoProducer(t *testing.T) {
	parent := t.TempDir()
	paths := Paths{Repository: filepath.Join(parent, "main"), Output: filepath.Join(parent, "artifact")}
	writeProducerPage(t, paths.Output, "labs")
	writeProducerPage(t, paths.Output, "seeded")
	if err := writeProducerRecord(paths, []Claim{{Route: "/labs/", Producers: []string{"labs"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(paths.Output, "data", "site-producers.json")); !os.IsNotExist(err) {
		t.Fatalf("the record was written into the published artifact: %v", err)
	}

	coverage, err := checkCoverage(paths)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(coverage.Unclaimed, []string{"/seeded/"}) {
		t.Fatalf("unclaimed routes %v, want the one route no producer wrote", coverage.Unclaimed)
	}
	if !coverage.Red() {
		t.Fatal("an unclaimed route left the coverage green")
	}
	if !strings.Contains(coverage.Spec, "spec-site-renderers-in-go") {
		t.Fatalf("coverage names %q, want the spec that owns this red", coverage.Spec)
	}
	if exit := (checkReport{Coverage: coverage}).exit(); exit == 0 {
		t.Fatal("a check with an unclaimed route exited zero")
	}
}

// VALIDATES: two producers writing one route is as red as none writing it. The
// second write silently wins, so which content a reader sees would otherwise
// depend on registration order alone.
func TestCheckRefusesARouteTwoProducersWrote(t *testing.T) {
	output := t.TempDir()
	writeProducerPage(t, output, "labs")
	stubProducers(t,
		Producer{Name: "docs", Render: func(Paths) ([]string, error) { return []string{"/labs/"}, nil }},
		Producer{Name: "hubs", Render: func(Paths) ([]string, error) { return []string{"/labs/"}, nil }},
	)

	claims, err := renderProducers(Paths{Output: output})
	if err != nil {
		t.Fatal(err)
	}
	coverage, err := coverageOf(output, claims)
	if err != nil {
		t.Fatal(err)
	}

	if len(coverage.Unclaimed) != 0 {
		t.Fatalf("unclaimed routes %v, want none", coverage.Unclaimed)
	}
	if len(coverage.Doubled) != 1 || coverage.Doubled[0].Route != "/labs/" ||
		!slices.Equal(coverage.Doubled[0].Producers, []string{"docs", "hubs"}) {
		t.Fatalf("doubly claimed routes %v, want /labs/ named by both producers", coverage.Doubled)
	}
	if !coverage.Red() {
		t.Fatal("a doubly claimed route left the coverage green")
	}
	if exit := (checkReport{Coverage: coverage}).exit(); exit == 0 {
		t.Fatal("a check with a doubly claimed route exited zero")
	}
}

// VALIDATES: two artifacts built from one checkout keep separate records, so a
// build into a scratch tree cannot make the check believe the published tree
// was claimed. The record lives outside the artifact, so its name is the only
// thing keeping the two apart.
func TestProducerRecordsAreKeyedByArtifact(t *testing.T) {
	parent := t.TempDir()
	repository := filepath.Join(parent, "main")
	published := Paths{Repository: repository, Output: filepath.Join(parent, "gh-pages")}
	scratch := Paths{Repository: repository, Output: filepath.Join(parent, "artifact")}
	if err := writeProducerRecord(published, []Claim{{Route: "/labs/", Producers: []string{"labs"}}}); err != nil {
		t.Fatal(err)
	}
	if err := writeProducerRecord(scratch, nil); err != nil {
		t.Fatal(err)
	}

	claims, err := readProducerRecord(published)
	if err != nil {
		t.Fatal(err)
	}

	if producerRecordPath(published) == producerRecordPath(scratch) {
		t.Fatalf("both artifacts record to %s", producerRecordPath(published))
	}
	if len(claims) != 1 || claims[0].Route != "/labs/" {
		t.Fatalf("the published record answers %v, want the claim it was written with", claims)
	}
}

// VALIDATES: an artifact with no record is fully unclaimed rather than fully
// claimed. A fresh checkout beside a published site has built nothing, so the
// absent file must fail closed: reading it as full coverage would call every
// frozen page green.
func TestAnArtifactWithNoRecordIsFullyUnclaimed(t *testing.T) {
	parent := t.TempDir()
	paths := Paths{Repository: filepath.Join(parent, "main"), Output: filepath.Join(parent, "artifact")}
	writeProducerPage(t, paths.Output, "labs")
	writeProducerPage(t, paths.Output, "guides")

	coverage, err := checkCoverage(paths)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(coverage.Unclaimed, []string{"/guides/", "/labs/"}) {
		t.Fatalf("unclaimed routes %v, want every published route", coverage.Unclaimed)
	}
	if !coverage.Red() {
		t.Fatal("an artifact no producer ever wrote to left the coverage green")
	}
}

// VALIDATES: the command surfaces are claimed by exactly one producer each, and
// llms.txt by exactly one that claims no route.
//
// R-2 in the spec: a route two producers write is as red as a route none
// writes, because the second write wins and nothing says so. The three
// producers here all read the same command catalog, so a route drifting from
// one to another is the concrete way that could happen.
func TestExactlyOneProducerClaimsEachCommandRoute(t *testing.T) {
	paths := llmsPaths(t)
	stubProducers(t,
		Producer{Name: "cli-reference", Render: renderCLIReference},
		Producer{Name: "command-equivalents", Render: renderCommandEquivalents},
		Producer{Name: "llms", Render: renderLLMS},
	)

	claims, err := renderProducers(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(claims) != 11 {
		t.Fatalf("the producers claimed %d routes, want the CLI reference, the vendor index and nine detail pages", len(claims))
	}
	for _, claim := range claims {
		if len(claim.Producers) != 1 {
			t.Errorf("%s is claimed by %v, want exactly one producer", claim.Route, claim.Producers)
		}
	}
	if claims[0].Route != "/reference/cli/" || claims[1].Route != "/reference/command-equivalents/" {
		t.Errorf("the claimed routes open with %s and %s", claims[0].Route, claims[1].Route)
	}
	if _, err := os.Stat(filepath.Join(paths.Output, llmsFile)); err != nil {
		t.Errorf("llms.txt was not written by the producer that claims no route: %v", err)
	}
}

// VALIDATES: AC-16 -- a named non-route artifact that disappears is refused by
// name.
//
// A producer answers ROUTES, so the coverage arithmetic cannot see a published
// file that is not a page: each one could stop being written and every other
// check the artifact carries would still pass. llms.txt is why the list exists,
// having lost seventeen of its eighteen sections with nothing to say so, and
// llms-full.txt is the file this phase adds to it.
func TestCheckRefusesAMissingNamedArtifact(t *testing.T) {
	// The population AC-16 names. Each one is published, none of them is a
	// route, and the list is the whole population rather than a sample: the
	// loop below removes every entry in turn.
	for _, name := range []string{
		llmsFile, llmsFullFile, sitemapFile, robotsFile, searchIndexFile,
		blogFeedDest, changesFeedDest, changesLegacyFeedDest,
	} {
		if !slices.Contains(namedArtifacts, name) {
			t.Errorf("%s is published and no check answers for it", name)
		}
	}
	output := t.TempDir()
	for _, name := range namedArtifacts {
		if err := writeNamedArtifact(output, name, "content\n"); err != nil {
			t.Fatal(err)
		}
	}
	if missing := checkNamedArtifacts(output); len(missing) != 0 {
		t.Fatalf("a complete artifact was reported as missing %v", missing)
	}

	for _, name := range namedArtifacts {
		if err := os.Remove(filepath.Join(output, filepath.FromSlash(name))); err != nil {
			t.Fatal(err)
		}
		missing := checkNamedArtifacts(output)
		if len(missing) != 1 || missing[0] != name {
			t.Errorf("removing %s was reported as %v", name, missing)
		}
		// Naming the file in a report is not refusing the artifact. The check
		// must exit non-zero for it, which is what a caller acts on.
		if exit := (checkReport{MissingArtifacts: missing}).exit(); exit == 0 {
			t.Errorf("a check missing %s exited zero", name)
		}
		if err := writeNamedArtifact(output, name, "content\n"); err != nil {
			t.Fatal(err)
		}
	}

	// An empty file and an absent one leave a reader the same blank page, so
	// the check answers the same way for both.
	if err := writeNamedArtifact(output, llmsFullFile, ""); err != nil {
		t.Fatal(err)
	}
	missing := checkNamedArtifacts(output)
	if len(missing) != 1 || missing[0] != llmsFullFile {
		t.Errorf("an empty %s was reported as %v", llmsFullFile, missing)
	}
}
