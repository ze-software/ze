// Design: website/AI.md -- one snapshot carries every number the site publishes
// Detail: docpage.go resolves a {{ze:...}} prose token against it, home.go
// renders the proof strip from it, derived.go states it in llms.txt.
package site

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/rfc"
)

// repositoryFactsFile is the committed file `./le site facts update` writes.
//
// The six numbers it states are counted over the tree a person is about to
// commit rather than over whatever a build found on disk, so the site build
// reads them here instead of walking the tree for them. That is the whole
// reason internal/le/site/facts exists.
const repositoryFactsFile = "repo-facts.json"

// repositoryFactKeys are the six facts the committed file owes this snapshot,
// named by the path each one takes in the published file.
//
// The names are the same on both sides, so nothing is renamed on the way
// through and a reader can trace a published number to the committed one by
// its key alone.
var repositoryFactKeys = []string{
	"interop.scenario_dirs_raw",
	"interop.scenarios",
	"interop.targets",
	"repo.design_comments",
	"repo.detail_comments",
	"repo.go_packages",
}

// The one network reach a site build makes, and its bounds.
//
// The count is published on the site header, and it exists nowhere in this
// repository, so it cannot be derived from a tree. The reach is bounded, its
// failure is not fatal, and a build that cannot make it says in _sources that
// the number it published is a carried one.
const (
	githubStarsAPI     = "https://api.github.com/repos/ze-software/ze"
	githubStarsTimeout = 5 * time.Second
)

// The three things a build can say about the star count it published.
const (
	starsSourceLive    = githubStarsAPI
	starsSourceCarried = "carried from the previous artifact: " + githubStarsAPI + " could not be reached"
	starsSourceUnknown = "unknown: " + githubStarsAPI + " could not be reached and no previous artifact stated a count"
)

// inventoryMetric is the test-health metric carrying the volume counters, and
// the four counts this snapshot publishes out of it.
const (
	inventoryMetric   = "inventory"
	inventoryCountsIn = "counts"
	countUnitTests    = "test_funcs"
	countFuzzTargets  = "fuzz_funcs"
	countCIScenarios  = "ci_files"
	countEditorTests  = "et_files"
)

// siteFacts is data/site-facts.json: every number the site publishes about this
// repository, and where each one came from.
//
// ONE model, written by publishSiteFacts and read by llmsdata.go, docpage.go
// and home.go. Two models of one file drift apart, which is what phase 6 of
// plan/spec-site-renderers-in-go.md removed for features.json and
// dependencies.json.
//
// The fields are declared in the alphabetical order of their JSON keys,
// because Go serializes a struct in declaration order and this file is
// published key-sorted. A person diffing two builds then reads a content
// change rather than a reordering.
type siteFacts struct {
	// Sources says where each published number came from, one entry for each
	// fact. Nothing reads it: it is here so a person meeting a number on a page
	// can find the producer behind it (ai/rules/evidence.md).
	Sources        map[string]string `json:"_sources"`
	BlogArticles   int               `json:"blog_articles"`
	Changes        int               `json:"changes"`
	CLICommands    int               `json:"cli_commands"`
	ConfigSections int               `json:"config_sections"`
	Dependencies   int               `json:"dependencies"`
	Features       factsFeatures     `json:"features"`
	GeneratedAt    string            `json:"generated_at"`
	GitHubStars    int               `json:"github_stars"`
	Interop        factsInterop      `json:"interop"`
	PublishedAt    string            `json:"published_at"`
	Repo           factsRepo         `json:"repo"`
	RFC            factsRFC          `json:"rfc"`
	Tests          factsTests        `json:"tests"`
}

// factsFeatures counts the feature cards website/data/features.json states: the
// shipped and experimental ones, and the roadmap ones that are neither.
type factsFeatures struct {
	CoreExperimental int `json:"core_experimental"`
	Planned          int `json:"planned"`
}

// factsInterop counts the peers ze is tested against and the scenarios it is
// tested through. ScenarioDirsRaw counts every directory, Scenarios counts the
// ones a reader sees.
type factsInterop struct {
	ScenarioDirsRaw  int    `json:"scenario_dirs_raw"`
	Scenarios        int    `json:"scenarios"`
	ScenariosDisplay string `json:"scenarios_display"`
	TargetDisplay    string `json:"target_display"`
	Targets          int    `json:"targets"`
}

// factsRepo counts what the Go tree holds: the packages, and the file headers
// that explain themselves.
type factsRepo struct {
	DesignComments        int    `json:"design_comments"`
	DesignCommentsDisplay string `json:"design_comments_display"`
	DetailComments        int    `json:"detail_comments"`
	DetailCommentsDisplay string `json:"detail_comments_display"`
	GoPackages            int    `json:"go_packages"`
	GoPackagesDisplay     string `json:"go_packages_display"`
}

// factsRFC counts the RFC requirement ledger: every requirement, the summaries
// they were extracted from, the MUST-level ones, and the MUST-level ones an
// enrolled RFC makes `./le rfc check` gate.
//
// Every figure here is printed EXACTLY, with no rounding, because `./le rfc
// check` compares the same numbers. A rounded figure would put the site out of
// agreement with its own gate.
type factsRFC struct {
	Enrolled            int    `json:"enrolled"`
	EnrolledDisplay     string `json:"enrolled_display"`
	GatedMust           int    `json:"gated_must"`
	GatedMustDisplay    string `json:"gated_must_display"`
	Must                int    `json:"must"`
	MustDisplay         string `json:"must_display"`
	Requirements        int    `json:"requirements"`
	RequirementsDisplay string `json:"requirements_display"`
	Summaries           int    `json:"summaries"`
	SummariesDisplay    string `json:"summaries_display"`
}

// factsTests counts the four test populations the homepage proof strip shows.
type factsTests struct {
	E2E           int    `json:"e2e"`
	E2EDisplay    string `json:"e2e_display"`
	Editor        int    `json:"editor"`
	EditorDisplay string `json:"editor_display"`
	Fuzz          int    `json:"fuzz"`
	FuzzDisplay   string `json:"fuzz_display"`
	Unit          int    `json:"unit"`
	UnitDisplay   string `json:"unit_display"`
}

// liveSiteFacts answers the numbers a build publishes about the checkout it
// read.
//
// It is a variable for the reason liveCommandCatalog is: a build over a
// synthetic tree has no RFC ledger, no test-health record and no committed
// repository facts, so a test that is not about the snapshot states one rather
// than laying down four inputs it does not care about.
var liveSiteFacts = deriveSiteFacts

// liveRFCLedger answers the RFC requirement ledger this checkout carries.
//
// It is a variable so a test can state a ledger, the way liveTestHealth lets a
// test state a health record. A synthetic checkout carries no rfc/short tree,
// and a test held against the real one would move with every requirement
// somebody extracts.
var liveRFCLedger = rfc.Collect

// liveGitHubStars answers the star count the project's forge holds.
//
// It is a variable so a test can state an answer or a failure, the way
// liveCommandCatalog lets a test state a catalog. A test that reached the
// network would publish a different number on every run.
var liveGitHubStars = fetchGitHubStars

// publishSiteFacts writes data/site-facts.json into the artifact.
//
// It runs in refreshNativeSurfaces rather than as a producer, for the reason
// publishPluginRegistry does: three surfaces read this file, and a producer
// runs in registration order, so an input a producer writes cannot be an input
// another producer reads.
//
// It runs AFTER the command catalog and the configuration tree, because it
// counts both.
func publishSiteFacts(paths Paths) error {
	facts, err := liveSiteFacts(paths)
	if err != nil {
		return err
	}
	content, err := json.MarshalIndent(facts, "", "  ")
	if err != nil {
		return fmt.Errorf("render %s: %w", factsFile, err)
	}
	return writeNamedArtifact(paths.Output, factsFile, string(content)+"\n")
}

// deriveSiteFacts answers every published number from this checkout.
//
// A number the tree cannot answer stops the build by name. The retired renderer
// warned and published a zero, and a zero on a page reads as a measurement
// rather than as an absent one: "0 unit tests" is a claim, and it is false.
// The star count is the one exception and it says so in _sources, because it is
// the one figure this repository does not hold.
func deriveSiteFacts(paths Paths) (siteFacts, error) {
	published := buildClock().UTC()
	facts := siteFacts{
		GeneratedAt: published.Format(time.DateOnly),
		PublishedAt: published.Format(time.RFC3339),
		Sources:     map[string]string{},
	}

	if err := factsFromRepositoryFile(paths.Source, &facts); err != nil {
		return siteFacts{}, err
	}
	if err := factsFromTestHealth(paths.Repository, &facts); err != nil {
		return siteFacts{}, err
	}
	if err := factsFromRFCLedger(paths.Repository, &facts); err != nil {
		return siteFacts{}, err
	}
	if err := factsFromSiteData(paths, &facts); err != nil {
		return siteFacts{}, err
	}

	stars, source := githubStars(paths.Output)
	facts.GitHubStars = stars
	facts.Sources["github_stars"] = source
	return facts, nil
}

// factsFromRepositoryFile reads the six numbers the committed repository facts
// state, and refuses one the file does not carry.
//
// The refusal names `./le site facts update` because that action is the fix: a
// key missing here means the committed file predates the fact, not that the
// tree has none.
func factsFromRepositoryFile(source string, facts *siteFacts) error {
	var committed struct {
		Facts map[string]struct {
			Value  int    `json:"value"`
			Source string `json:"source"`
		} `json:"facts"`
	}
	if err := readSourceJSON(source, repositoryFactsFile, &committed); err != nil {
		return err
	}
	for _, key := range repositoryFactKeys {
		if _, stated := committed.Facts[key]; !stated {
			return fmt.Errorf("website/data/%s states no %s; run `./le site facts update`", repositoryFactsFile, key)
		}
		facts.Sources[key] = "website/data/" + repositoryFactsFile + ": " + committed.Facts[key].Source
	}

	facts.Interop.ScenarioDirsRaw = committed.Facts["interop.scenario_dirs_raw"].Value
	facts.Interop.Scenarios = committed.Facts["interop.scenarios"].Value
	facts.Interop.ScenariosDisplay = displayCount(facts.Interop.Scenarios)
	facts.Interop.Targets = committed.Facts["interop.targets"].Value
	facts.Interop.TargetDisplay = displayCount(facts.Interop.Targets)

	facts.Repo.DesignComments = committed.Facts["repo.design_comments"].Value
	facts.Repo.DesignCommentsDisplay = displayCount(facts.Repo.DesignComments)
	facts.Repo.DetailComments = committed.Facts["repo.detail_comments"].Value
	facts.Repo.DetailCommentsDisplay = displayCount(facts.Repo.DetailComments)
	facts.Repo.GoPackages = committed.Facts["repo.go_packages"].Value
	facts.Repo.GoPackagesDisplay = displayCount(facts.Repo.GoPackages)
	return nil
}

// factsFromTestHealth reads the four test counts from the SAME producer the
// testing-health page reads.
//
// One counter, so the two pages cannot disagree about how many tests ze has.
// The retired build had two counters over one tree and they differed by 30 the
// moment both existed; reading test/health/latest.json here instead would bring
// that back, because health.go reads the tree while the committed snapshot is
// refreshed by hand.
func factsFromTestHealth(repository string, facts *siteFacts) error {
	rendered, err := liveTestHealth(repository)
	if err != nil {
		return err
	}
	record, err := parseHealthRecord(rendered.Record)
	if err != nil {
		return err
	}
	counts, err := inventoryCounts(record)
	if err != nil {
		return err
	}
	facts.Tests.Unit = counts[countUnitTests]
	facts.Tests.UnitDisplay = displayCount(facts.Tests.Unit)
	facts.Tests.Fuzz = counts[countFuzzTargets]
	facts.Tests.FuzzDisplay = displayCount(facts.Tests.Fuzz)
	facts.Tests.E2E = counts[countCIScenarios]
	facts.Tests.E2EDisplay = displayCount(facts.Tests.E2E)
	facts.Tests.Editor = counts[countEditorTests]
	facts.Tests.EditorDisplay = displayCount(facts.Tests.Editor)
	facts.Sources["tests"] = "internal/le/testhealth.Render, over the tree this build read"
	return nil
}

// inventoryCounts answers the four volume counters the test-health record's
// inventory metric carries.
//
// A count the metric does not state is refused by name rather than read as
// zero: the metric is generated, so an absent key means the generator changed
// and this snapshot has to change with it.
func inventoryCounts(record healthRecord) (map[string]int, error) {
	wanted := []string{countUnitTests, countFuzzTargets, countCIScenarios, countEditorTests}
	for _, metric := range record.Metrics {
		if metric.text("key") != inventoryMetric {
			continue
		}
		stated, isObject := metric[inventoryCountsIn].(map[string]any)
		if !isObject {
			return nil, fmt.Errorf("the test-health %s metric states no %s object", inventoryMetric, inventoryCountsIn)
		}
		counts := make(map[string]int, len(wanted))
		for _, name := range wanted {
			value, ok := jsonInt(stated[name])
			if !ok {
				return nil, fmt.Errorf("the test-health %s metric states no %s count", inventoryMetric, name)
			}
			counts[name] = value
		}
		return counts, nil
	}
	return nil, fmt.Errorf("the test-health record has no %s metric, so the published test counts would be zero", inventoryMetric)
}

// jsonInt answers a whole number a JSON decoder read, whichever of its two
// number shapes it used. parseHealthRecord decodes with UseNumber, so the
// values arrive as json.Number; a record built by a test may hold float64.
func jsonInt(value any) (int, bool) {
	switch typed := value.(type) {
	case json.Number:
		whole, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return int(whole), true
	case float64:
		return int(typed), true
	}
	return 0, false
}

// factsFromRFCLedger counts the requirement ledger from the gate's own parse.
//
// The four figures are the ones ai/RFC-REQUIREMENTS.md prints in its summary
// line, and they are derived here from the same rfc.Collect that generates that
// line rather than parsed back out of the generated text
// (ai/rules/evidence.md: read the producer).
func factsFromRFCLedger(repository string, facts *siteFacts) error {
	collected, err := liveRFCLedger(repository)
	if err != nil {
		return err
	}
	if len(collected.Requirements) == 0 {
		return fmt.Errorf("the RFC ledger states no requirement, so the published RFC counts would be zero")
	}
	summaries := map[string]bool{}
	for _, requirement := range collected.Requirements {
		summaries[requirement.RFC] = true
		if !requirement.Gated() {
			continue
		}
		facts.RFC.Must++
		if collected.Enrolled[requirement.RFC] {
			facts.RFC.GatedMust++
		}
	}
	facts.RFC.Requirements = len(collected.Requirements)
	facts.RFC.Summaries = len(summaries)
	facts.RFC.Enrolled = len(collected.Enrolled)

	facts.RFC.EnrolledDisplay = groupThousands(facts.RFC.Enrolled)
	facts.RFC.GatedMustDisplay = groupThousands(facts.RFC.GatedMust)
	facts.RFC.MustDisplay = groupThousands(facts.RFC.Must)
	facts.RFC.RequirementsDisplay = groupThousands(facts.RFC.Requirements)
	facts.RFC.SummariesDisplay = groupThousands(facts.RFC.Summaries)
	facts.Sources["rfc"] = "internal/le/rfc.Collect, over rfc/short/*.md and rfc/enrolled.txt"
	return nil
}

// factsFromSiteData counts what the site's own sources and this build's own
// artifact hold: the feature cards, the articles, the weeks, the dependencies,
// the commands and the configuration sections.
func factsFromSiteData(paths Paths, facts *siteFacts) error {
	var features featureData
	if err := readSourceJSON(paths.Source, featuresDataFile, &features); err != nil {
		return err
	}
	for _, section := range features.Sections {
		if section.ID == featureSectionCore || section.ID == featureSectionExperimental {
			facts.Features.CoreExperimental += len(section.Cards)
			continue
		}
		facts.Features.Planned += len(section.Cards)
	}
	if facts.Features.CoreExperimental == 0 {
		return fmt.Errorf("data/%s states no shipped or experimental feature, so the published feature count would be zero", featuresDataFile)
	}
	facts.Sources["features"] = "website/" + featuresFile

	articles, err := countMarkdownSources(paths.Source, blogSourceDirectory)
	if err != nil {
		return err
	}
	facts.BlogArticles = articles
	facts.Sources["blog_articles"] = "website/" + blogSourceDirectory

	weeks, err := countMarkdownSources(paths.Source, changesSourceDirectory)
	if err != nil {
		return err
	}
	facts.Changes = weeks
	facts.Sources["changes"] = "website/" + changesSourceDirectory

	versions, err := directModuleVersions(filepath.Join(paths.Repository, goModuleFile))
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		return fmt.Errorf("%s requires no module directly, so the published dependency count would be zero", goModuleFile)
	}
	facts.Dependencies = len(versions)
	facts.Sources["dependencies"] = goModuleFile + ", the direct requirements only"

	commands, err := loadCommandCatalog(paths.Output)
	if err != nil {
		return err
	}
	facts.CLICommands = len(commands)
	facts.Sources["cli_commands"] = catalogFile + ", published by this build from the live binary"

	_, tree, err := readConfigTree(paths.Output)
	if err != nil {
		return err
	}
	facts.ConfigSections = len(tree)
	facts.Sources["config_sections"] = configTreeFile + ", published by this build from the live schema"
	return nil
}

// countMarkdownSources counts the Markdown files one source directory holds.
//
// A directory with none is refused: the site publishes an index over each of
// these two, so an empty one is a checkout the build cannot describe rather
// than a count of zero.
func countMarkdownSources(source, directory string) (int, error) {
	entries, err := os.ReadDir(filepath.Join(source, filepath.FromSlash(directory)))
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), markdownExtension) {
			count++
		}
	}
	if count == 0 {
		return 0, fmt.Errorf("no Markdown source in website/%s, so its published count would be zero", directory)
	}
	return count, nil
}

// githubStars answers the star count this build publishes and the sentence
// _sources states about it.
//
// The fetch is bounded and its failure is not fatal, so a build with no network
// still finishes. It publishes the number the previous artifact published, or
// zero when there is no previous artifact to carry one from, and the sentence
// says which: a carried number is not a measurement of today, and a reader of
// the file has to be able to tell the two apart.
func githubStars(output string) (int, string) {
	stars, err := liveGitHubStars()
	if err == nil {
		return stars, starsSourceLive
	}
	carried, published := previousStarCount(output)
	if !published {
		return 0, starsSourceUnknown
	}
	return carried, starsSourceCarried
}

// fetchGitHubStars reads the star count from the project's forge.
func fetchGitHubStars() (int, error) {
	// The context is the ONE bound on this call, and it covers the body read as
	// well as the connection. A Timeout on the client beside it would be a
	// second statement of one limit, and the two would disagree the day
	// somebody changed one.
	ctx, cancel := context.WithTimeout(context.Background(), githubStarsTimeout)
	defer cancel()

	client := http.Client{}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, githubStarsAPI, http.NoBody)
	if err != nil {
		return 0, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "ze-site-build")
	response, err := client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close() //nolint:errcheck // the body is drained below and a close error cannot change the answer
	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("%s answered %s", githubStarsAPI, response.Status)
	}
	// The repository record is a few kilobytes. The bound is stated because
	// nothing on the other side of a socket is obliged to stop
	// (docs/contributing/ze-go-style.md, "A limit on everything").
	const bodyMax = 1 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, bodyMax))
	if err != nil {
		return 0, err
	}
	var record struct {
		Stars *int `json:"stargazers_count"`
	}
	if err := json.Unmarshal(body, &record); err != nil {
		return 0, fmt.Errorf("read what %s answered: %w", githubStarsAPI, err)
	}
	if record.Stars == nil {
		return 0, fmt.Errorf("%s stated no stargazers_count", githubStarsAPI)
	}
	return *record.Stars, nil
}

// previousStarCount answers the star count the previous artifact published, and
// whether it published one at all.
//
// An artifact with no snapshot, or one whose snapshot cannot be read, has
// published no count. The absence is answered rather than defaulted, so the
// caller can say "unknown" instead of inventing a number no tree holds.
func previousStarCount(output string) (int, bool) {
	content, err := os.ReadFile(filepath.Join(output, filepath.FromSlash(factsFile))) //nolint:gosec // a site build reads the artifact it was pointed at
	if err != nil {
		return 0, false
	}
	var previous struct {
		Stars *int `json:"github_stars"`
	}
	if err := json.Unmarshal(content, &previous); err != nil || previous.Stars == nil {
		return 0, false
	}
	return *previous.Stars, true
}

// displayCount rounds a count down to one tenth of its own magnitude and marks
// a rounded answer with a plus.
//
// A magnitude's last digits answer nothing and they rewrite every page carrying
// the number on every build, which is why the published figure is rounded and
// the exact one sits beside it under its own key. 23,718 unit tests publishes
// as "23,700+", 166 editor tests as "160+", and 78 fuzz targets exactly,
// because a count below a hundred is already at its own precision.
func displayCount(count int) string {
	step := 1
	switch {
	case count < 100:
		step = 1
	case count < 1000:
		step = 10
	default:
		// The step is one tenth of the count's own thousands group: a
		// five-digit count rounds to hundreds, an eight-digit one to hundreds
		// of thousands. The loop is bounded by the digit count of an int.
		for range 3*((len(strconv.Itoa(count))-1)/3) - 1 {
			step *= 10
		}
	}
	rounded := (count / step) * step
	if rounded == count {
		return groupThousands(rounded)
	}
	return groupThousands(rounded) + "+"
}
