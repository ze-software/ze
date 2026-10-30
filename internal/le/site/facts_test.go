// Design: website/AI.md -- one snapshot carries every number the site publishes
package site

import (
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
	siterelease "github.com/ze-software/ze/internal/le/site/release"
	testhealth "github.com/ze-software/ze/internal/le/test/health"
)

// factsFixture lays out one checkout the facts snapshot can be derived from:
// the committed repository facts, the feature cards, one article, one week, a
// go.mod with two direct requirements, and the two artifact files the snapshot
// counts. It stubs the three live readers so nothing here reaches the network,
// the RFC tree or a compiler.
//
// The numbers are chosen so each one is distinguishable in an assertion: no two
// counts are equal, and each sits in a different rounding band.
func factsFixture(t *testing.T) Paths {
	t.Helper()
	parent := t.TempDir()
	root := filepath.Join(parent, "main")
	source := filepath.Join(root, "website")
	output := filepath.Join(parent, "gh-pages")

	writeFixtureFile(t, filepath.Join(source, "data", "repo-facts.json"), `{
	  "facts": {
	    "interop.scenario_dirs_raw": {"category":"committed-data","value":138,"source":"git ls-files test/interop/scenarios/"},
	    "interop.scenarios": {"category":"committed-data","value":137,"source":"git ls-files test/interop/scenarios/"},
	    "interop.targets": {"category":"committed-data","value":9,"source":"git ls-files test/interop/Dockerfile.*"},
	    "repo.design_comments": {"category":"committed-data","value":4299,"source":"git ls-files '*.go'"},
	    "repo.detail_comments": {"category":"committed-data","value":3281,"source":"git ls-files '*.go'"},
	    "repo.go_packages": {"category":"committed-data","value":755,"source":"go list ./..."}
	  },
	  "live": {}
	}`)
	// Four cards, three on one supported declaration and one on an
	// experimental one, so the count is the cards kept, whatever their state.
	writeFixtureFile(t, filepath.Join(root, "features", "solid.md"), siteDeclaration("Solid", "complete", "supported"))
	writeFixtureFile(t, filepath.Join(root, "features", "dashed.md"), siteDeclaration("Dashed", "complete", "experimental"))
	writeFixtureFile(t, filepath.Join(source, "data", "features.json"), `{"sections":[{"id":"core"},{"id":"experimental"}],"cards":[
	  {"category":"routing","features":["solid"],"title":"A","href":"features/"},
	  {"category":"operate","features":["solid"],"title":"B","href":"features/"},
	  {"category":"secure","features":["solid"],"title":"C","href":"features/"},
	  {"category":"observe","features":["dashed"],"title":"D","href":"features/"}
	]}`)
	writeFixtureFile(t, filepath.Join(source, "blog", "posts", "one.md"), "# One\n")
	writeFixtureFile(t, filepath.Join(source, "changes", "posts", "2026-08-17.md"), "# Week\n")
	writeFixtureFile(t, filepath.Join(source, "changes", "posts", "2026-08-10.md"), "# Week\n")
	writeFixtureFile(t, filepath.Join(root, "go.mod"), "module example.test\n\ngo 1.25\n\nrequire (\n\tone.test/a v1.0.0\n\ttwo.test/b v2.0.0\n\tthree.test/c v3.0.0 // indirect\n)\n")

	writeArtifactFile(t, output, catalogFile, `[{"path":"show test","short-help":"Show rows","mode":"read-only"},
		{"path":"show other","short-help":"Show other rows","mode":"read-only"}]`)
	writeArtifactFile(t, output, configTreeFile, `{"bgp":{"kind":"container","description":"BGP."},
		"static":{"kind":"container","description":"Static routes."}}`)
	writeArtifactFile(t, output, equivalentsFile, `{"vendors":{"juniper":{"label":"Junos"}},
		"entries":[{"intent":"Show rows","ze":["show test"],"vendors":{"juniper":[{"command":"show rows"}]}},
		{"intent":"Vendor only","ze":[],"vendors":{"juniper":[{"command":"show other"}]}}]}`)
	writeArtifactFile(t, output, pluginFile, `[{"name":"bfd"},{"name":"bgp-rib"},{"name":"ntp"}]`)
	// A release carrying an ISO, so every key of the release section holds a
	// value (TestATokenResolvesAnyKeyTheSnapshotNames).
	writeFixtureFile(t, filepath.Join(source, "data", siterelease.DataFile), fixtureReleasesFile)

	stubFactsInputs(t)
	if err := os.MkdirAll(filepath.Join(source, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	return Paths{Repository: root, Source: source, Output: output}
}

// writeFixtureFile writes one file of a synthetic checkout, creating its
// directories.
func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// stubFactsInputs states the three inputs the facts snapshot cannot read from a
// synthetic checkout: the test-health record, the RFC ledger and the forge.
func stubFactsInputs(t *testing.T) {
	t.Helper()
	stubTestHealth(t, fixtureHealthRecord)
	stubRFCLedger(t, fixtureRFCLedger())
	stubGitHubStars(t, 50, nil)
}

// fixtureHealthRecord is a test-health record carrying the inventory metric the
// snapshot reads, with the four counts spread across the rounding bands.
const fixtureHealthRecord = `{"metrics":[
  {"key":"inventory","question":"Q2","label":"In-repo test inventory","status":"ok",
   "value":"23718 test functions","detail":"counts","action":"counting boundary",
   "counts":{"test_funcs":23718,"fuzz_funcs":78,"ci_files":1728,"et_files":166,"test_files":3541}}
]}`

// stubTestHealth states the record a build reads instead of walking the tree.
func stubTestHealth(t *testing.T, record string) {
	t.Helper()
	previous := liveTestHealth
	t.Cleanup(func() { liveTestHealth = previous })
	liveTestHealth = func(string) (testhealth.Rendered, error) {
		return testhealth.Rendered{Record: record, Page: "# Testing health\n"}, nil
	}
}

// stubRFCLedger states the ledger a build reads instead of parsing rfc/short.
func stubRFCLedger(t *testing.T, collected rfc.Collected) {
	t.Helper()
	previous := liveRFCLedger
	t.Cleanup(func() { liveRFCLedger = previous })
	liveRFCLedger = func(string) (rfc.Collected, error) { return collected, nil }
}

// stubGitHubStars states what the forge answers, or the failure it answers with.
func stubGitHubStars(t *testing.T, stars int, failure error) {
	t.Helper()
	previous := liveGitHubStars
	t.Cleanup(func() { liveGitHubStars = previous })
	liveGitHubStars = func() (int, error) { return stars, failure }
}

// fixtureRFCLedger is a ledger of four requirements over three summaries: two
// MUST-level ones under an enrolled RFC, one MUST-level one under an RFC that
// is not enrolled, and one advisory row that is neither.
//
// The Meta table is what rfc.ProvenShareOf reads, so the fixture declares one:
// enrolment alone does not say whether Ze implements a document, and a ledger
// with no public row anywhere is refused rather than published as a share over
// an empty population.
func fixtureRFCLedger() rfc.Collected {
	return rfc.Collected{
		Enrolled: map[string]bool{"rfc4271": true, "rfc7606": true},
		// Implementation is set on every row, so what keeps two of them out of
		// the enrolled set is the Enrolment cell alone. Meta.Enrolled() reads
		// both, and a fixture that left the kind absent would answer "not
		// enrolled" for the reason the case is not about.
		Metas: map[string]rfc.Meta{
			"rfc4271": {Enrolment: "enrolled", Implementation: "ze", Aggregate: "bgp-core", Row: true, Status: "Supported"},
			"rfc7606": {Enrolment: "enrolled", Implementation: "ze", Aggregate: "bgp-core", Row: true, Status: "Supported"},
			"rfc9999": {Enrolment: "backlog", Implementation: "ze"},
			"rfc8654": {Enrolment: "backlog", Implementation: "ze"},
		},
		Requirements: []rfc.Requirement{
			{RFC: "rfc4271", RID: "R-1", Level: "MUST"},
			{RFC: "rfc4271", RID: "R-2", Level: "SHALL NOT"},
			{RFC: "rfc9999", RID: "R-3", Level: "MUST"},
			{RFC: "rfc8654", RID: "R-4", Level: "SHOULD"},
		},
	}
}

// deriveFixtureFacts answers the snapshot the fixture checkout produces.
func deriveFixtureFacts(t *testing.T) siteFacts {
	t.Helper()
	facts, err := deriveSiteFacts(factsFixture(t))
	if err != nil {
		t.Fatalf("derive the facts snapshot: %v", err)
	}
	return facts
}

// VALIDATES: the snapshot states every key the published contract names, so a
// page reading one of them meets a number rather than an absent field.
//
// The shape follows the published snapshot, with pending-card counts retired
// in favor of the committed release inventory. Values come from their producers.
func TestTheFactsSnapshotStatesEveryKeyTheContractNames(t *testing.T) {
	facts, err := deriveSiteFacts(factsFixture(t))
	if err != nil {
		t.Fatalf("derive the facts snapshot: %v", err)
	}
	raw, err := json.Marshal(facts)
	if err != nil {
		t.Fatal(err)
	}
	var written map[string]any
	if err := json.Unmarshal(raw, &written); err != nil {
		t.Fatal(err)
	}
	for _, key := range publishedFactKeys() {
		if _, found := factValue(written, key); !found {
			t.Errorf("the snapshot states no %s, which the published file carries", key)
		}
	}
	if _, found := written["_sources"]; !found {
		t.Error("the snapshot states no _sources, so no published number names its producer")
	}
	if _, found := factValue(written, "features.planned"); found {
		t.Error("the snapshot still publishes a separate pending-card count")
	}
}

// publishedFactKeys names the current published fact contract.
func publishedFactKeys() []string {
	return []string{
		"blog_articles", "changes", "cli_commands", "config_sections", "dependencies",
		"features.core_experimental",
		"generated_at", "github_stars", "published_at",
		"interop.scenario_dirs_raw", "interop.scenarios", "interop.scenarios_display",
		"interop.target_display", "interop.targets",
		"repo.design_comments", "repo.design_comments_display",
		"repo.detail_comments", "repo.detail_comments_display",
		"repo.go_packages", "repo.go_packages_display",
		"rfc.enrolled", "rfc.enrolled_display", "rfc.gated_must", "rfc.gated_must_display",
		"rfc.must", "rfc.must_display", "rfc.requirements", "rfc.requirements_display",
		"rfc.summaries", "rfc.summaries_display",
		"tests.e2e", "tests.e2e_display", "tests.editor", "tests.editor_display",
		"tests.fuzz", "tests.fuzz_display", "tests.unit", "tests.unit_display",
	}
}

// VALIDATES: each number comes from the input that owns it, and the counts a
// build takes from the tree are the tree's own.
func TestEveryNumberIsTheOneItsInputStates(t *testing.T) {
	facts := deriveFixtureFacts(t)
	for _, check := range []struct {
		what string
		got  int
		want int
	}{
		{"blog articles", facts.BlogArticles, 1},
		{"weekly updates", facts.Changes, 2},
		{"CLI commands", facts.CLICommands, 2},
		{"config sections", facts.ConfigSections, 2},
		{"direct dependencies", facts.Dependencies, 2},
		{"shipped and experimental features", facts.Features.CoreExperimental, 4},
		{"interop scenarios", facts.Interop.Scenarios, 137},
		{"interop scenario directories", facts.Interop.ScenarioDirsRaw, 138},
		{"interop targets", facts.Interop.Targets, 9},
		{"design comments", facts.Repo.DesignComments, 4299},
		{"detail comments", facts.Repo.DetailComments, 3281},
		{"Go packages", facts.Repo.GoPackages, 755},
		{"RFC requirements", facts.RFC.Requirements, 4},
		{"RFC summaries", facts.RFC.Summaries, 3},
		{"MUST-level requirements", facts.RFC.Must, 3},
		{"gated MUST-level requirements", facts.RFC.GatedMust, 2},
		{"enrolled RFCs", facts.RFC.Enrolled, 2},
		{"unit tests", facts.Tests.Unit, 23718},
		{"fuzz targets", facts.Tests.Fuzz, 78},
		{"end-to-end tests", facts.Tests.E2E, 1728},
		{"editor tests", facts.Tests.Editor, 166},
		{"stars", facts.GitHubStars, 50},
	} {
		if check.got != check.want {
			t.Errorf("%s: got %d, want %d", check.what, check.got, check.want)
		}
	}
}

// VALIDATES: a number the tree cannot answer stops the build by name, rather
// than publishing a zero that reads as a measurement.
//
// The retired renderer warned and published zero for each of these, and its
// build then exited non-zero on any warning, so such a snapshot never reached a
// reader. Refusing says the same thing where the mistake is.
func TestAFactTheTreeCannotAnswerStopsTheBuild(t *testing.T) {
	for _, refusal := range []struct {
		what   string
		break_ func(t *testing.T, paths Paths)
		says   string
	}{
		{"a repository fact the committed file lost", func(t *testing.T, paths Paths) {
			// Every key but one, so the refusal names the one that is gone
			// rather than whichever the walk reached first.
			writeFixtureFile(t, filepath.Join(paths.Source, "data", "repo-facts.json"), `{"facts":{
				"interop.scenario_dirs_raw":{"value":131,"source":"x"},
				"interop.scenarios":{"value":130,"source":"x"},
				"interop.targets":{"value":9,"source":"x"},
				"repo.detail_comments":{"value":3281,"source":"x"},
				"repo.go_packages":{"value":755,"source":"x"}},"live":{}}`)
		}, "repo.design_comments"},
		{"a test-health record with no inventory metric", func(t *testing.T, _ Paths) {
			stubTestHealth(t, `{"metrics":[{"key":"proof","question":"Q1","label":"Proof","status":"ok","value":"1"}]}`)
		}, "no inventory metric"},
		{"an inventory metric that lost a count", func(t *testing.T, _ Paths) {
			stubTestHealth(t, `{"metrics":[{"key":"inventory","question":"Q2","label":"In-repo test inventory",
				"status":"ok","value":"1","counts":{"test_funcs":1,"fuzz_funcs":1,"ci_files":1}}]}`)
		}, "et_files"},
		{"an RFC ledger with no requirement", func(t *testing.T, _ Paths) {
			stubRFCLedger(t, rfc.Collected{Enrolled: map[string]bool{}})
		}, "states no requirement"},
		{"a features file with no shipped section", func(t *testing.T, paths Paths) {
			writeFixtureFile(t, filepath.Join(paths.Source, "data", "features.json"),
				`{"sections":[{"id":"core"},{"id":"experimental"}],"cards":[]}`)
		}, "no shipped or experimental feature"},
		{"a feature card no declaration backs", func(t *testing.T, paths Paths) {
			writeFixtureFile(t, filepath.Join(paths.Source, "data", "features.json"),
				`{"sections":[{"id":"core"},{"id":"experimental"}],"cards":[
				{"category":"routing","features":["solid","unicorn"],"title":"A","href":"features/"}]}`)
		}, "unicorn"},
		{"a blog with no article", func(t *testing.T, paths Paths) {
			if err := os.Remove(filepath.Join(paths.Source, "blog", "posts", "one.md")); err != nil {
				t.Fatal(err)
			}
		}, "blog/posts"},
		{"a go.mod requiring nothing directly", func(t *testing.T, paths Paths) {
			writeFixtureFile(t, filepath.Join(paths.Repository, "go.mod"),
				"module example.test\n\ngo 1.25\n\nrequire (\n\tone.test/a v1.0.0 // indirect\n)\n")
		}, "no direct dependency"},
		{"an artifact with no command catalog", func(t *testing.T, paths Paths) {
			if err := os.Remove(filepath.Join(paths.Output, filepath.FromSlash(catalogFile))); err != nil {
				t.Fatal(err)
			}
		}, catalogFile},
		{"an artifact with no configuration tree", func(t *testing.T, paths Paths) {
			if err := os.Remove(filepath.Join(paths.Output, filepath.FromSlash(configTreeFile))); err != nil {
				t.Fatal(err)
			}
		}, configTreeFile},
	} {
		t.Run(refusal.what, func(t *testing.T) {
			paths := factsFixture(t)
			refusal.break_(t, paths)
			facts, err := deriveSiteFacts(paths)
			if err == nil {
				t.Fatalf("%s published a snapshot instead of stopping the build: %+v", refusal.what, facts)
			}
			if !strings.Contains(err.Error(), refusal.says) {
				t.Errorf("%s said %q, which does not name %q", refusal.what, err, refusal.says)
			}
		})
	}
}

// VALIDATES: this checkout can answer every published fact from its own tree.
//
// The fixture above states the inputs; this reads the real ones, so a committed
// repository-facts file that lost a key, a test-health record that stopped
// carrying the inventory metric, or an RFC ledger that stopped parsing is a red
// here rather than a build failure on the day somebody publishes.
//
// The artifact half is a fixture, because the command catalog and the
// configuration tree are written by the build from a compiled ze rather than
// read from the tree.
func TestThisCheckoutCanAnswerEveryPublishedFact(t *testing.T) {
	facts := publishedFactsOfThisCheckout(t)
	// The numbers this checkout answers are logged rather than asserted: each
	// one moves with the tree, so pinning it would make every added test a red
	// here. What is asserted is that each one was ANSWERED.
	t.Logf("rfc: requirements=%d summaries=%d must=%d gated=%d enrolled=%d",
		facts.RFC.Requirements, facts.RFC.Summaries, facts.RFC.Must, facts.RFC.GatedMust, facts.RFC.Enrolled)
	t.Logf("repo: design=%d (%s) detail=%d (%s) packages=%d (%s)",
		facts.Repo.DesignComments, facts.Repo.DesignCommentsDisplay,
		facts.Repo.DetailComments, facts.Repo.DetailCommentsDisplay,
		facts.Repo.GoPackages, facts.Repo.GoPackagesDisplay)
	t.Logf("tests: unit=%d (%s) e2e=%d (%s) editor=%d (%s) fuzz=%d (%s)",
		facts.Tests.Unit, facts.Tests.UnitDisplay, facts.Tests.E2E, facts.Tests.E2EDisplay,
		facts.Tests.Editor, facts.Tests.EditorDisplay, facts.Tests.Fuzz, facts.Tests.FuzzDisplay)
	t.Logf("other: features=%d changes=%d blog=%d dependencies=%d interop=%d/%d",
		facts.Features.CoreExperimental, facts.Changes,
		facts.BlogArticles, facts.Dependencies, facts.Interop.Scenarios, facts.Interop.Targets)

	for _, check := range []struct {
		what  string
		count int
	}{
		{"design comments", facts.Repo.DesignComments},
		{"Go packages", facts.Repo.GoPackages},
		{"interop scenarios", facts.Interop.Scenarios},
		{"interop targets", facts.Interop.Targets},
		{"unit tests", facts.Tests.Unit},
		{"end-to-end tests", facts.Tests.E2E},
		{"editor tests", facts.Tests.Editor},
		{"fuzz targets", facts.Tests.Fuzz},
		{"RFC requirements", facts.RFC.Requirements},
		{"gated MUST-level requirements", facts.RFC.GatedMust},
		{"enrolled RFCs", facts.RFC.Enrolled},
		{"shipped and experimental features", facts.Features.CoreExperimental},
		{"weekly updates", facts.Changes},
		{"blog articles", facts.BlogArticles},
		{"direct dependencies", facts.Dependencies},
	} {
		if check.count <= 0 {
			t.Errorf("this checkout answers %d %s", check.count, check.what)
		}
	}
}

// VALIDATES: a build with no network keeps the star count the previous artifact
// published and says in _sources that the number is a carried one.
//
// The count exists nowhere in this repository, so a build's answer depends on
// the network and on its own last output. AC-11 asks for the offline path to
// keep the previous value and SAY SO, which is what makes a reader able to tell
// a carried number from a measured one.
func TestFactsSnapshotKeepsTheStarCountOffline(t *testing.T) {
	paths := factsFixture(t)
	writeArtifactFile(t, paths.Output, factsFile, `{"github_stars":50}`)
	stubGitHubStars(t, 0, errors.New("dial tcp: no route to host"))

	facts, err := deriveSiteFacts(paths)
	if err != nil {
		t.Fatalf("an unreachable forge stopped the build: %v", err)
	}
	if facts.GitHubStars != 50 {
		t.Errorf("offline star count: got %d, want the 50 the previous artifact published", facts.GitHubStars)
	}
	if !strings.Contains(facts.Sources["github_stars"], "carried from the previous artifact") {
		t.Errorf("_sources says %q, which does not say the number is carried", facts.Sources["github_stars"])
	}
}

// VALIDATES: a build with no network and no previous artifact publishes no
// count and says so, rather than inventing one.
//
// The retired renderer fell back to a literal 46 that traced to nothing
// (ai/rules/evidence.md). An honest absence is a zero whose _sources says
// "unknown", and it still does not stop the build.
func TestAStarCountNoBuildCanAnswerSaysUnknown(t *testing.T) {
	paths := factsFixture(t)
	stubGitHubStars(t, 0, errors.New("dial tcp: no route to host"))

	facts, err := deriveSiteFacts(paths)
	if err != nil {
		t.Fatalf("an unreachable forge with no previous artifact stopped the build: %v", err)
	}
	if facts.GitHubStars != 0 {
		t.Errorf("star count with nothing to carry: got %d, want 0", facts.GitHubStars)
	}
	if !strings.HasPrefix(facts.Sources["github_stars"], "unknown:") {
		t.Errorf("_sources says %q, which does not say the count is unknown", facts.Sources["github_stars"])
	}
}

// VALIDATES: every prose token the site states resolves against the snapshot
// this build writes.
//
// A {{ze:<dotted.key>}} token names a path into this file. A key renamed here
// and not in the source that names it would stop the build, so the method walks
// every website source for its tokens and asks this build's snapshot for each.
func TestEveryProseTokenResolvesAgainstTheSnapshotThisBuildWrites(t *testing.T) {
	paths := factsFixture(t)
	if err := publishSiteFacts(paths); err != nil {
		t.Fatalf("publish the facts snapshot: %v", err)
	}
	tokens, err := loadSiteFactSnapshot(paths.Output)
	if err != nil {
		t.Fatal(err)
	}
	stated := 0
	website := filepath.Join(repositoryRoot(t), "website")
	err = filepath.WalkDir(website, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		// AI.md is the agent guide, which spells the token syntax itself, and
		// no producer publishes it.
		if path == filepath.Join(website, "AI.md") {
			return nil
		}
		if entry.IsDir() || (!strings.HasSuffix(path, ".md") && !strings.HasSuffix(path, ".html") && !strings.HasSuffix(path, ".json")) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range valueTokenPattern.FindAllStringSubmatch(string(content), -1) {
			stated++
			if _, resolved := tokens.value(match[1]); !resolved {
				t.Errorf("%s states {{ze:%s}}, which the snapshot this build wrote does not name", path, match[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if stated == 0 {
		t.Fatal("no website source states a token, so this test proves nothing")
	}
	marked, err := tokens.substitute("Ze runs {{ze:tests.unit_display}} unit tests.", true)
	if err != nil {
		t.Fatal(err)
	}
	if marked != `Ze runs <span data-ze-stat="tests.unit_display">23,700+</span> unit tests.` {
		t.Errorf("the HTML form reads %q", marked)
	}
	plain, err := tokens.substitute("Ze runs {{ze:tests.unit_display}} unit tests.", false)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "Ze runs 23,700+ unit tests." {
		t.Errorf("the mirror form reads %q", plain)
	}
}

// VALIDATES: a build writes the facts snapshot before any producer runs, so a
// producer reading it meets this build's numbers rather than the seed's.
//
// The snapshot is an input to three surfaces -- the homepage proof strip, every
// page carrying a {{ze:...}} token, and llms.txt -- and a producer runs in
// registration order, so an input a producer writes cannot be an input another
// producer reads. It is written in refreshNativeSurfaces for that reason, beside
// the command catalog and the plugin registry.
func TestTheFactsSnapshotIsWrittenBeforeAnyProducerReadsIt(t *testing.T) {
	root, output := siteFixture(t)
	stubLiveInputs(t, `[{"path":"show test","short-help":"Show rows","mode":"read-only"}]`)
	// The whole point is the ORDER, so this producer states what it read and
	// the assertion below reads it back rather than trusting the file's
	// presence after the build.
	read := ""
	stubProducers(t, Producer{Name: "reader", Render: func(paths Paths) ([]string, error) {
		content, err := os.ReadFile(filepath.Join(paths.Output, filepath.FromSlash(factsFile)))
		if err != nil {
			return nil, err
		}
		read = string(content)
		return nil, nil
	}})
	if _, err := Build(BuildOptions{Repository: root, Output: output}); err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.Contains(read, `"cli_commands": 1`) {
		t.Errorf("a producer read %q, which is not the snapshot this build wrote", read)
	}
}

// VALIDATES: the published file is key-sorted and its numbers are stable, so a
// second build over an unchanged tree writes the same bytes.
//
// Go serializes a struct in declaration order, so the sort is a property of how
// the fields are declared rather than of the encoder. A field added out of
// order rewrites the whole file on the next build and hides the change that
// actually moved a number.
func TestTheSnapshotIsWrittenKeySorted(t *testing.T) {
	paths := factsFixture(t)
	if err := publishSiteFacts(paths); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(paths.Output, filepath.FromSlash(factsFile)))
	if err != nil {
		t.Fatal(err)
	}
	if !sortedJSONKeys(t, string(content)) {
		t.Errorf("the snapshot is not key-sorted:\n%s", content)
	}
}

// sortedJSONKeys reports whether every object of one indented JSON document
// states its keys in ascending order.
//
// The comparison is per OBJECT rather than per indentation depth: two sibling
// objects nest their fields at one depth, and the second one's first key is
// under no obligation to follow the first one's last.
func sortedJSONKeys(t *testing.T, document string) bool {
	t.Helper()
	sorted := true
	previous := map[int]string{}
	for line := range strings.SplitSeq(document, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		depth := len(line) - len(trimmed)
		if strings.HasPrefix(trimmed, "}") || strings.HasPrefix(trimmed, "{") {
			delete(previous, depth+2)
			continue
		}
		key, _, isField := strings.Cut(trimmed, `": `)
		if !isField || !strings.HasPrefix(key, `"`) {
			continue
		}
		if last, seen := previous[depth]; seen && last > key {
			t.Logf("at depth %d, %s follows %s", depth, key, last)
			sorted = false
		}
		previous[depth] = key
	}
	return sorted
}

// VALIDATES: a count rounds down to one tenth of its own magnitude, and a
// rounded answer says it was rounded.
//
// The exact number sits beside the rounded one under its own key, so the
// rounding costs a reader nothing and saves every page carrying the figure from
// being rewritten whenever somebody adds a test.
func TestADisplayCountRoundsDownToItsOwnMagnitude(t *testing.T) {
	for _, item := range []struct {
		count int
		want  string
	}{
		{0, "0"},
		{9, "9"},
		{78, "78"},
		{99, "99"},
		{100, "100"},
		{130, "130"},
		{166, "160+"},
		{687, "680+"},
		{999, "990+"},
		{1000, "1,000"},
		{1728, "1,700+"},
		{3852, "3,800+"},
		{23718, "23,700+"},
		{123456, "123,400+"},
		{1234567, "1,200,000+"},
	} {
		if got := displayCount(item.count); got != item.want {
			t.Errorf("displayCount(%d): got %q, want %q", item.count, got, item.want)
		}
	}
}

// VALIDATES: each rounded figure is the rounding of the exact number beside it,
// and each is wired to that number rather than to a neighbor.
//
// A display string and its number are two statements of one fact, so the pair
// is where they can disagree: a field wired to the wrong count, or to the exact
// formatter the RFC figures use, publishes a number no page can be held to.
func TestEveryDisplayFigureIsTheRoundingOfTheNumberBesideIt(t *testing.T) {
	facts := deriveFixtureFacts(t)
	for _, pair := range []struct {
		key     string
		count   int
		display string
	}{
		{"interop.scenarios", facts.Interop.Scenarios, facts.Interop.ScenariosDisplay},
		{"interop.targets", facts.Interop.Targets, facts.Interop.TargetDisplay},
		{"repo.design_comments", facts.Repo.DesignComments, facts.Repo.DesignCommentsDisplay},
		{"repo.detail_comments", facts.Repo.DetailComments, facts.Repo.DetailCommentsDisplay},
		{"repo.go_packages", facts.Repo.GoPackages, facts.Repo.GoPackagesDisplay},
		{"tests.unit", facts.Tests.Unit, facts.Tests.UnitDisplay},
		{"tests.e2e", facts.Tests.E2E, facts.Tests.E2EDisplay},
		{"tests.editor", facts.Tests.Editor, facts.Tests.EditorDisplay},
		{"tests.fuzz", facts.Tests.Fuzz, facts.Tests.FuzzDisplay},
	} {
		if want := displayCount(pair.count); pair.display != want {
			t.Errorf("%s_display: got %q for %d, want %q", pair.key, pair.display, pair.count, want)
		}
	}
}

// VALIDATES: the RFC figures are printed exactly, with no rounding, because
// `./le rfc check` compares the same numbers and a rounded site figure would
// put the two out of agreement.
func TestTheRFCFiguresArePrintedExactly(t *testing.T) {
	paths := factsFixture(t)
	stubRFCLedger(t, manyRFCRequirements(4747, 2975))
	facts, err := deriveSiteFacts(paths)
	if err != nil {
		t.Fatal(err)
	}
	if facts.RFC.Requirements != 4747 || facts.RFC.RequirementsDisplay != "4,747" {
		t.Errorf("requirements: %d shown as %q, want 4747 shown as \"4,747\"",
			facts.RFC.Requirements, facts.RFC.RequirementsDisplay)
	}
	if facts.RFC.GatedMust != 2975 || facts.RFC.GatedMustDisplay != "2,975" {
		t.Errorf("gated MUSTs: %d shown as %q, want 2975 shown as \"2,975\"",
			facts.RFC.GatedMust, facts.RFC.GatedMustDisplay)
	}
}

// manyRFCRequirements answers a ledger of the given size, of which gated rows
// are MUST-level under an enrolled RFC and the rest are advisory.
func manyRFCRequirements(total, gated int) rfc.Collected {
	collected := rfc.Collected{
		Enrolled: map[string]bool{"rfc4271": true},
		Metas: map[string]rfc.Meta{
			"rfc4271": {Enrolment: "enrolled", Implementation: "ze", Aggregate: "bgp-core", Row: true, Status: "Supported"},
		},
		Requirements: make([]rfc.Requirement, 0, total),
	}
	for index := range total {
		level := "SHOULD"
		if index < gated {
			level = "MUST"
		}
		collected.Requirements = append(collected.Requirements,
			rfc.Requirement{RFC: "rfc4271", RID: "R", Level: level})
	}
	return collected
}

// VALIDATES: AC-70 -- no published provenance string names a GENERATED file as
// an input.
//
// `_sources["rfc"]` said `internal/le/rfc.Collect, over rfc/short/*.md and
// rfc/enrolled.txt` after Collect had stopped reading that file, which is now
// generated from the same summaries (independent review, 2026-09-01). A false
// provenance claim in machine-readable data is worse than a stale comment,
// because a consumer acts on it. Every other entry in that map is built from
// the constant its producer just read, so it cannot drift; these three paths
// are the ones a hand-written sentence can name by mistake.
func TestNoPublishedSourceNamesAGeneratedLedgerFile(t *testing.T) {
	facts := publishedFactsOfThisCheckout(t)
	if len(facts.Sources) == 0 {
		t.Fatal("the facts carry no _sources at all, so this proves nothing")
	}
	for key, source := range facts.Sources {
		for _, generated := range []string{
			"rfc/enrolled.txt", "rfc/not-enrolled.txt", "docs/features/rfc-status.md",
		} {
			if strings.Contains(source, generated) {
				t.Errorf("_sources[%q] names %s as an input, and that file is GENERATED "+
					"from rfc/short/*.md by ./le rfc update", key, generated)
			}
		}
	}
	t.Logf("%d published provenance strings, none naming a generated ledger file",
		len(facts.Sources))
}

// checkoutFactsSnapshot is bounded to one immutable corpus in this test
// process. The two corpus assertions read it; mutation tests use factsFixture.
var checkoutFactsSnapshot struct {
	once  sync.Once
	root  string
	facts siteFacts
	err   error
}

// publishedFactsOfThisCheckout drives the real facts producer once, with only
// its network answer and compiled command/configuration artifacts controlled.
// Safe for concurrent reads; callers MUST NOT mutate the returned source map.
func publishedFactsOfThisCheckout(t *testing.T) siteFacts {
	t.Helper()
	root := repositoryRoot(t)
	checkoutFactsSnapshot.once.Do(func() {
		checkoutFactsSnapshot.root = root
		output := t.TempDir()
		writeArtifactFile(t, output, catalogFile, `[{"path":"show test","short-help":"Show rows","mode":"read-only"}]`)
		writeArtifactFile(t, output, configTreeFile, `{"bgp":{"kind":"container","description":"BGP."}}`)
		writeArtifactFile(t, output, pluginFile, `[{"name":"bgp-rib"}]`)
		// The vendor command map is a tracked source, so the build has staged
		// this checkout's own copy into the output (stageSources) before the
		// facts snapshot counts it. The catalog, the configuration tree and the
		// plugin registry above are what refreshNativeSurfaces publishes from a
		// live daemon, which this test does not compile.
		if err := stageSources(filepath.Join(root, "website"), output, []string{equivalentsFile}); err != nil {
			t.Fatalf("stage the vendor command map: %v", err)
		}
		stubGitHubStars(t, 50, nil)
		stubRFCLedger(t, collectedRFCsOfThisCheckout(t))
		checkoutFactsSnapshot.facts, checkoutFactsSnapshot.err = deriveSiteFacts(Paths{
			Repository: root, Source: filepath.Join(root, "website"), Output: output,
			testHealth: func() (testhealth.Rendered, error) { return sharedTestHealth(root) }})
	})
	if checkoutFactsSnapshot.root != root {
		t.Fatalf("site facts belong to %s, not %s", checkoutFactsSnapshot.root, root)
	}
	if checkoutFactsSnapshot.err != nil {
		t.Fatalf("this checkout cannot answer its own published facts: %v", checkoutFactsSnapshot.err)
	}
	return checkoutFactsSnapshot.facts
}

// VALIDATES: the published feature count is the number of cards the derivation
// kept, each backed by the declarations it names, and it moves when a card is
// added, not when a declaration's level changes (AC-19).
func TestFactsFeatureCountFromDeclarations(t *testing.T) {
	paths := factsFixture(t)
	facts, err := deriveSiteFacts(paths)
	if err != nil {
		t.Fatal(err)
	}
	if facts.Features.CoreExperimental != 4 {
		t.Fatalf("the count is %d, want the 4 cards the fixture keeps", facts.Features.CoreExperimental)
	}
	writeFixtureFile(t, filepath.Join(paths.Repository, "features", "solid.md"),
		siteDeclaration("Solid", "complete", "experimental"))
	facts, err = deriveSiteFacts(paths)
	if err != nil {
		t.Fatal(err)
	}
	if facts.Features.CoreExperimental != 4 {
		t.Errorf("a level change moved the count to %d", facts.Features.CoreExperimental)
	}
}

// VALIDATES: AC-1 and AC-2's source. The snapshot counts the command catalog,
// its groups, the vendor map's populations and the plugin registrations once,
// with the functions the pages group and count with.
//
// The fixture holds two commands under one verb, a vendor map of one mapped
// intent and one vendor-only intent, and three plugin registrations, so each
// published value is stated independently of the code that counts it. The
// per-group and per-vendor sections are keyed by the names the data holds: the
// verb "show" and the vendor "juniper".
func TestTheSnapshotCountsWhatThePagesShow(t *testing.T) {
	paths := factsFixture(t)
	if err := publishSiteFacts(paths); err != nil {
		t.Fatalf("publish the facts snapshot: %v", err)
	}
	snapshot, err := loadSiteFactSnapshot(paths.Output)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"cli_commands":                         "2",
		"cli_groups":                           "1",
		"cli_group_commands.show":              "2",
		"equivalents.groups.show.equivalent":   "1",
		"equivalents.groups.show.reviewed":     "1",
		"equivalents.groups.show.total":        "2",
		"equivalents.vendors.juniper.commands": "1",
		"equivalents.vendors.juniper.lines":    "1",
		"equivalents.vendor_lines":             "1",
		"equivalents.mapping_entries":          "2",
		"equivalents.reviewed":                 "1",
		"equivalents.vendor_cli":               "1",
		"equivalents.vendor_only":              "1",
		"plugin_registrations":                 "3",
	} {
		if got, _ := snapshot.value(key); got != want {
			t.Errorf("the snapshot states %s as %q, want %q", key, got, want)
		}
	}
}

// VALIDATES: AC-18, derived not enumerated. A metric the record gains, under a
// key and with payload names no code here spells, reaches the snapshot with
// every figure the page prints, and the page marks each one.
//
// Method: append an invented metric to the published record, derive the
// snapshot through factsFromTestHealth, the path a build takes, and read each
// figure back by the dotted key the page names it with. A metric key no
// snapshot key can carry is refused by name rather than published under a key
// that names something else.
func TestANewHealthMetricReachesTheSnapshotUnnamed(t *testing.T) {
	unnamed := map[string]any{
		"key": "unnamed-metric", "question": testhealth.Questions()[0].Key, "label": "A metric no code names",
		"status": "warn", "value": "7 / 9 (floor 3)", "detail": "Detail.", "action": "Action.",
		"share": map[string]any{"numerator": 7, "denominator": 9, "percent": 77.8},
		"worst": []any{map[string]any{"name": "first", "hits": 5}},
	}
	recordWith := func(metric map[string]any) string {
		var document map[string]any
		if err := json.Unmarshal([]byte(readFixture(t, "published-health-record.json")), &document); err != nil {
			t.Fatal(err)
		}
		metrics, isList := document["metrics"].([]any)
		if !isList {
			t.Fatal("the fixture record states no metric list")
		}
		document["metrics"] = append(metrics, metric)
		encoded, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		return string(encoded)
	}

	record := recordWith(unnamed)
	paths := healthPathsFrom(t, record, "# Testing Health\n")
	facts := siteFacts{Sources: map[string]string{}}
	if err := factsFromTestHealth(paths, &facts); err != nil {
		t.Fatal(err)
	}
	content, err := renderSiteFacts(&facts)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := parseSiteFactSnapshot([]byte(content))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"health.unnamed-metric.value.1":           "7",
		"health.unnamed-metric.value.2":           "9",
		"health.unnamed-metric.value.3":           "3",
		"health.unnamed-metric.share.numerator":   "7",
		"health.unnamed-metric.share.denominator": "9",
		"health.unnamed-metric.share.percent":     "77.8",
		"health.unnamed-metric.worst.1.hits":      "5",
	}
	for key, value := range want {
		if got, found := snapshot.value(key); got != value {
			t.Errorf("the snapshot states %s as %q (found %v), want %q", key, got, found, value)
		}
	}
	if _, found := snapshot.value("health.unnamed-metric.worst.1.name"); found {
		t.Error("the snapshot carries a row's name, which the page prints as a word, not a figure")
	}

	writeArtifactFile(t, paths.Output, factsFile, content)
	if _, err := renderHealth(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, healthDest)
	for key, value := range want {
		if !strings.Contains(page, markedFact(key, value)) {
			t.Errorf("the page does not show %s as its marked value %q", key, value)
		}
	}

	dotted := maps.Clone(unnamed)
	dotted["key"] = "dotted.metric"
	dottedPaths := healthPathsFrom(t, recordWith(map[string]any{
		"key": "plain", "question": testhealth.Questions()[0].Key, "label": "L", "status": "ok", "value": "1",
	}), "# Testing Health\n")
	liveTestHealth = func(string) (testhealth.Rendered, error) {
		return testhealth.Rendered{Record: recordWith(dotted), Page: "# Testing Health\n"}, nil
	}
	err = factsFromTestHealth(dottedPaths, &siteFacts{Sources: map[string]string{}})
	if err == nil || !strings.Contains(err.Error(), "dotted.metric") {
		t.Errorf("a metric keyed dotted.metric was not refused by name: %v", err)
	}
}

// fixtureReleasesFile is a committed release file stating one prerelease with
// an ISO and a checksum file, in the form `./le site release update` writes.
const fixtureReleasesFile = `{"endpoint":"https://api.github.com/repos/ze-software/ze/releases","releases":[
  {"assets":[
    {"browser_download_url":"https://github.com/ze-software/ze/releases/download/v26.11.0/SHA256SUMS","digest":null,"name":"SHA256SUMS","size":0},
    {"browser_download_url":"https://github.com/ze-software/ze/releases/download/v26.11.0/ze-20261102-100000.iso",
     "digest":"sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08","name":"ze-20261102-100000.iso","size":128933888}],
   "html_url":"https://github.com/ze-software/ze/releases/tag/v26.11.0","name":"Ze 26.11","prerelease":true,
   "published_at":"2026-11-02T10:00:00Z","tag_name":"v26.11.0"}]}`

// noReleasesFile is the committed release file while the forge states none.
const noReleasesFile = `{"endpoint":"https://api.github.com/repos/ze-software/ze/releases","releases":[]}`

// refusingTransport fails every request a build makes, and the test with it.
type refusingTransport struct{ t *testing.T }

func (transport refusingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.t.Errorf("the build reached the network: %s %s", request.Method, request.URL)
	return nil, errors.New("this test refuses every request")
}

// VALIDATES: AC-13, and the snapshot half of AC-10 and AC-11. A build reads
// release data from the committed file only.
//
// Every HTTP request of the process goes through a transport that fails the
// test (the star count is stubbed, so the build has no reason to make one),
// and the snapshot follows what the committed file states: the release, then
// none, then a file nobody committed, which is refused with the command that
// writes it.
func TestTheBuildReadsReleaseDataOnlyFromTheCommittedFile(t *testing.T) {
	original := http.DefaultTransport
	http.DefaultTransport = refusingTransport{t: t}
	t.Cleanup(func() { http.DefaultTransport = original })

	paths := factsFixture(t)
	if err := publishSiteFacts(paths); err != nil {
		t.Fatal(err)
	}
	snapshot, err := loadSiteFactSnapshot(paths.Output)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"release.version":          "v26.11.0",
		"release.published":        "2026-11-02",
		"release.url":              "https://github.com/ze-software/ze/releases/tag/v26.11.0",
		"release.iso.name":         "ze-20261102-100000.iso",
		"release.iso.bytes":        "128933888",
		"release.iso.size_display": "128.9 MB",
		"release.iso.sha256":       "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		"release.iso.download_url": "https://github.com/ze-software/ze/releases/download/v26.11.0/ze-20261102-100000.iso",
		"_sources.release":         "website/data/github-releases.json: release v26.11.0, published 2026-11-02T10:00:00Z (read from https://api.github.com/repos/ze-software/ze/releases)",
	} {
		if got, found := snapshot.value(key); !found || got != want {
			t.Errorf("%s reads %q (%v), want %q", key, got, found, want)
		}
	}

	writeFixtureFile(t, filepath.Join(paths.Source, "data", siterelease.DataFile), noReleasesFile)
	if err := publishSiteFacts(paths); err != nil {
		t.Fatal(err)
	}
	published := readArtifact(t, paths.Output, factsFile)
	if !strings.Contains(published, `"release": null`) {
		t.Errorf("with no release the snapshot does not state a null release:\n%s", published)
	}
	if !strings.Contains(published, `"release": "none yet: website/data/github-releases.json states no release`) {
		t.Errorf("with no release _sources does not say none yet:\n%s", published)
	}

	if err := os.Remove(filepath.Join(paths.Source, "data", siterelease.DataFile)); err != nil {
		t.Fatal(err)
	}
	err = publishSiteFacts(paths)
	if err == nil || !strings.Contains(err.Error(), "./le site release update") {
		t.Errorf("a checkout with no release file published (%v), want a refusal naming ./le site release update", err)
	}
}

// labISOMarker is the lab page's ISO size, marked with its fallback text.
const labISOMarker = `<span data-ze-stat="release.iso.size_display">under 150 MB</span>`

// VALIDATES: AC-14, A-6. The appliance lab's ISO size reads its fallback until
// the newest release carries an .iso asset, then the snapshot's size.
//
// The test reads the real lab page source and passes it through the pass every
// hand-written page takes (substituteHTML) against snapshots derived from three
// committed release files: none, a release with no ISO, and a release with an
// ISO beside another asset. It also proves the pass refuses a misspelled key
// while the section is null, a marker holding markup, and a release with two
// ISOs, and that the dated measurement sentence stays as written.
func TestTheLabISOSizeFallsBackUntilAReleaseCarriesOne(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "website", "labs", "appliance-install", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	lab := string(content)
	for _, phrase := range []string{"The current amd64 installation ISO is " + labISOMarker + ".", "128,933,888 bytes"} {
		if !strings.Contains(lab, phrase) {
			t.Fatalf("the lab page source does not carry %q", phrase)
		}
	}

	snapshotOf := func(t *testing.T, releases string) (siteFactSnapshot, error) {
		t.Helper()
		source := t.TempDir()
		writeFixtureFile(t, filepath.Join(source, "data", siterelease.DataFile), releases)
		facts := siteFacts{Sources: map[string]string{}}
		if err := factsFromReleases(source, &facts); err != nil {
			return siteFactSnapshot{}, err
		}
		rendered, err := renderSiteFacts(&facts)
		if err != nil {
			t.Fatal(err)
		}
		return parseSiteFactSnapshot([]byte(rendered))
	}
	withoutISO := strings.Replace(noReleasesFile, `"releases":[]`, `"releases":[{"assets":[
		{"browser_download_url":"https://github.com/ze-software/ze/releases/download/v1/notes.txt","digest":null,"name":"notes.txt","size":3}],
		"html_url":"https://github.com/ze-software/ze/releases/tag/v1","name":null,"prerelease":false,
		"published_at":"2026-11-01T09:00:00Z","tag_name":"v1"}]`, 1)
	twoISOs := strings.Replace(fixtureReleasesFile, `"name":"SHA256SUMS"`, `"name":"ze-arm64.iso"`, 1)

	for _, state := range []struct {
		what     string
		releases string
		shows    string
	}{
		{"no release", noReleasesFile, labISOMarker},
		{"a release with no ISO", withoutISO, labISOMarker},
		{"a release with an ISO", fixtureReleasesFile, markedFact("release.iso.size_display", "128.9 MB")},
	} {
		t.Run(state.what, func(t *testing.T) {
			snapshot, err := snapshotOf(t, state.releases)
			if err != nil {
				t.Fatal(err)
			}
			page, err := snapshot.substituteHTML(lab)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(page, "The current amd64 installation ISO is "+state.shows+".") {
				t.Errorf("the lab page does not show %s", state.shows)
			}
			if !strings.Contains(page, "128,933,888 bytes") {
				t.Error("the dated measurement sentence changed")
			}
		})
	}

	empty, err := snapshotOf(t, noReleasesFile)
	if err != nil {
		t.Fatal(err)
	}
	full, err := snapshotOf(t, fixtureReleasesFile)
	if err != nil {
		t.Fatal(err)
	}
	misspelled := `<p><span data-ze-stat="release.iso.size_dispaly">under 150 MB</span></p>`
	for name, snapshot := range map[string]siteFactSnapshot{"no release": empty, "a release": full} {
		if _, err := snapshot.substituteHTML(misspelled); err == nil || !strings.Contains(err.Error(), "release.iso.size_dispaly") {
			t.Errorf("with %s a misspelled marker key was not refused by name (%v)", name, err)
		}
	}
	markup := `<p><span data-ze-stat="release.iso.size_display">under <b>150</b> MB</span></p>`
	if _, err := full.substituteHTML(markup); err == nil || !strings.Contains(err.Error(), "holds markup") {
		t.Errorf("a marker holding markup was not refused (%v)", err)
	}

	if _, err := snapshotOf(t, twoISOs); err == nil || !strings.Contains(err.Error(), "two .iso assets") {
		t.Errorf("a release with two ISOs was not refused (%v)", err)
	}
}
