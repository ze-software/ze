// Design: website/AI.md -- one snapshot carries every number the site publishes
package site

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
	"github.com/ze-software/ze/internal/le/testhealth"
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
	writeFixtureFile(t, filepath.Join(source, "data", "features.json"), `{"sections":[
	  {"id":"core","cards":[{"category":"routing"},{"category":"operate"},{"category":"secure"}]},
	  {"id":"experimental","cards":[{"category":"observe"}]},
	  {"id":"roadmap","cards":[{"category":"platform"},{"category":"automate"}]}
	]}`)
	writeFixtureFile(t, filepath.Join(source, "blog", "posts", "one.md"), "# One\n")
	writeFixtureFile(t, filepath.Join(source, "changes", "posts", "2026-08-17.md"), "# Week\n")
	writeFixtureFile(t, filepath.Join(source, "changes", "posts", "2026-08-10.md"), "# Week\n")
	writeFixtureFile(t, filepath.Join(root, "go.mod"), "module example.test\n\ngo 1.25\n\nrequire (\n\tone.test/a v1.0.0\n\ttwo.test/b v2.0.0\n\tthree.test/c v3.0.0 // indirect\n)\n")

	writeArtifactFile(t, output, catalogFile, `[{"path":"show test","description":"Show rows","mode":"read-only"},
		{"path":"show other","description":"Show other rows","mode":"read-only"}]`)
	writeArtifactFile(t, output, configTreeFile, `{"bgp":{"kind":"container","description":"BGP."},
		"static":{"kind":"container","description":"Static routes."}}`)

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
func fixtureRFCLedger() rfc.Collected {
	return rfc.Collected{
		Enrolled: map[string]bool{"rfc4271": true, "rfc7606": true},
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
// The contract is the file the site published at gh-pages 2fa8fa2ad, not the
// retired script: that script never wrote the _sources entry naming
// website/data/repo-facts.json, so the last publish ran a version of it nobody
// committed. The VALUES are re-derived from the tree and are expected to differ
// from the published ones; the SHAPE is what binds.
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
}

// publishedFactKeys are the dotted keys of every number the published
// data/site-facts.json carries, taken from gh-pages 2fa8fa2ad.
func publishedFactKeys() []string {
	return []string{
		"blog_articles", "changes", "cli_commands", "config_sections", "dependencies",
		"features.core_experimental", "features.planned",
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
		{"roadmap features", facts.Features.Planned, 2},
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
				`{"sections":[{"id":"roadmap","cards":[{"category":"platform"}]}]}`)
		}, "no shipped or experimental feature"},
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
	root := repositoryRoot(t)
	output := t.TempDir()
	writeArtifactFile(t, output, catalogFile, `[{"path":"show test","description":"Show rows","mode":"read-only"}]`)
	writeArtifactFile(t, output, configTreeFile, `{"bgp":{"kind":"container","description":"BGP."}}`)
	stubGitHubStars(t, 50, nil)

	facts, err := deriveSiteFacts(Paths{Repository: root, Source: filepath.Join(root, "website"), Output: output})
	if err != nil {
		t.Fatalf("this checkout cannot answer its own published facts: %v", err)
	}
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
	t.Logf("other: features=%d/%d changes=%d blog=%d dependencies=%d interop=%d/%d",
		facts.Features.CoreExperimental, facts.Features.Planned, facts.Changes,
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
// A {{ze:name}} token names a dotted path into this file. A key renamed here
// and not there leaves the token unresolved, and substitute leaves an
// unresolved token alone, so the braces reach the reader with nothing to notice.
func TestEveryProseTokenResolvesAgainstTheSnapshotThisBuildWrites(t *testing.T) {
	paths := factsFixture(t)
	if err := publishSiteFacts(paths); err != nil {
		t.Fatalf("publish the facts snapshot: %v", err)
	}
	tokens, err := loadNumberTokens(paths.Output)
	if err != nil {
		t.Fatal(err)
	}
	for name := range numberTokenSpecs {
		if _, resolved := tokens[name]; !resolved {
			t.Errorf("{{ze:%s}} resolves to nothing in the snapshot this build wrote", name)
		}
	}
	marked, err := tokens.substitute("Ze runs {{ze:unit-tests}} unit tests.", true)
	if err != nil {
		t.Fatal(err)
	}
	if marked != `Ze runs <span data-ze-stat="tests.unit_display">23,700+</span> unit tests.` {
		t.Errorf("the HTML form reads %q", marked)
	}
	plain, err := tokens.substitute("Ze runs {{ze:unit-tests}} unit tests.", false)
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
	stubLiveInputs(t, `[{"path":"show test","description":"Show rows","mode":"read-only"}]`)
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
		Enrolled:     map[string]bool{"rfc4271": true},
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
	root := repositoryRoot(t)
	output := t.TempDir()
	writeArtifactFile(t, output, catalogFile, `[{"path":"show test","description":"Show rows","mode":"read-only"}]`)
	writeArtifactFile(t, output, configTreeFile, `{"bgp":{"kind":"container","description":"BGP."}}`)
	stubGitHubStars(t, 50, nil)

	facts, err := deriveSiteFacts(Paths{
		Repository: root, Source: filepath.Join(root, "website"), Output: output})
	if err != nil {
		t.Fatalf("this checkout cannot answer its own published facts: %v", err)
	}
	if len(facts.Sources) == 0 {
		t.Fatal("the facts carry no _sources at all, so this proves nothing")
	}
	for key, source := range facts.Sources {
		for _, generated := range []string{
			"rfc/enrolled.txt", "rfc/not-enrolled.txt", "docs/features/rfc-status.md",
		} {
			if strings.Contains(source, generated) {
				t.Errorf("_sources[%q] names %s as an input, and that file is GENERATED "+
					"from rfc/short/*.md by ./le rfc index-update", key, generated)
			}
		}
	}
	t.Logf("%d published provenance strings, none naming a generated ledger file",
		len(facts.Sources))
}
