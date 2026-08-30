// Design: website/AI.md -- the testing-health page is internal/le/testhealth's own record
package site

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/testhealth"
)

// healthPaths lays out one checkout whose test-health answer is the record the
// published page was rendered from.
//
// The record is stated rather than collected: a live read of this tree answers
// today's counters, so a golden page would move with every test added.
func healthPaths(t *testing.T) Paths {
	t.Helper()
	return healthPathsFrom(t, readFixture(t, "published-health-record.json"), "# Testing Health\n")
}

// healthPathsFrom lays out a checkout whose test-health answer is stated.
func healthPathsFrom(t *testing.T, record, page string) Paths {
	t.Helper()
	root := repositoryRoot(t)
	source := t.TempDir()
	copyFixture(t, filepath.Join(root, "website", "data", "page-links.json"),
		filepath.Join(source, "data", "page-links.json"))

	previous := liveTestHealth
	t.Cleanup(func() { liveTestHealth = previous })
	liveTestHealth = func(string) (testhealth.Rendered, error) {
		return testhealth.Rendered{Record: record, Page: page}, nil
	}
	return Paths{Repository: root, Source: source, Output: t.TempDir()}
}

// healthRecordWith answers the fixture record with its metrics replaced.
func healthRecordWith(t *testing.T, metrics, history []map[string]any) string {
	t.Helper()
	document := map[string]any{"metrics": metrics, "history": history}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// healthMetricFixture answers one metric of the stated status, under the first
// question the generator asks.
func healthMetricFixture(key, label, status string) map[string]any {
	return map[string]any{
		"key": key, "question": testhealth.Questions()[0].Key, "label": label, "status": status,
		"value": "1 / 2", "detail": "Detail for " + key, "action": "Do something about " + key,
	}
}

// VALIDATES: the page reads as the published page, from the record the
// published page was rendered from.
//
// The trend table is excluded and has cases of its own: the retired renderer
// drew four series where internal/le/testhealth now states three, because the
// mutation collector it took the fourth from no longer exists. Everything above
// it is the same page.
func TestTheTestingHealthPageReadsAsThePublishedPage(t *testing.T) {
	paths := healthPaths(t)

	routes, err := renderHealth(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || routes[0] != healthRoute {
		t.Fatalf("the producer claimed %v, want [%s]", routes, healthRoute)
	}

	page := readArtifact(t, paths.Output, healthDest)
	for _, chrome := range []string{
		"<title>Testing Health - Ze</title>",
		`<link rel="canonical" href="https://ze-software.net/quality/health/" />`,
		`<link rel="stylesheet" href="../../assets/site.css" />`,
		`<main id="top" class="has-page-sidebar" tabindex="-1">`,
		`<section aria-labelledby="test-health-title" class="md-content reveal cat-observe">`,
		`<h1 id="test-health-title">Testing Health</h1>`,
		`<aside class="page-sidebar" aria-label="Related page links">`,
		"<footer>",
	} {
		if !strings.Contains(page, chrome) {
			t.Errorf("the testing-health page is missing %q", chrome)
		}
	}

	got := healthTextBeforeTrends(visibleText(mainContent(t, page)))
	want := healthTextBeforeTrends(visibleText(readFixture(t, "published-health-body.html")))
	if got != want {
		t.Errorf("the testing-health page reads as\n  %q\nthe published page reads as\n  %q", got, want)
	}
}

// healthTextBeforeTrends cuts one page's visible text at the trend section.
func healthTextBeforeTrends(text string) string {
	before, _, cut := strings.Cut(text, "Evolution over time")
	if !cut {
		return text
	}
	return before
}

// VALIDATES: the section names the heading that labels it.
//
// visibleText cannot see an attribute, so the parity test above passes whether
// or not aria-labelledby names an element the page carries.
func TestTheHealthPageIsLabelledByItsOwnHeading(t *testing.T) {
	paths := healthPaths(t)
	if _, err := renderHealth(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, healthDest)

	if !strings.Contains(page, `aria-labelledby="test-health-title"`) {
		t.Error("the section carries no aria-labelledby")
	}
	if !strings.Contains(page, `id="test-health-title"`) {
		t.Error(`aria-labelledby names test-health-title, which the page does not carry`)
	}
}

// VALIDATES: a metric nobody is measuring sorts above one that looks bad, in
// the attention table and inside its own question.
//
// A page that ranked `unknown` below `warn` would put a dead sensor at the
// bottom of the attention table, which is the failure the ordering exists to
// stop. The method is positional over the rendered page.
func TestAnUnmeasuredMetricSortsAboveAKnownProblem(t *testing.T) {
	record := healthRecordWith(t, []map[string]any{
		healthMetricFixture("warned", "A warned metric", "warn"),
		healthMetricFixture("healthy", "A healthy metric", "ok"),
		healthMetricFixture("unmeasured", "An unmeasured metric", "unknown"),
	}, nil)
	paths := healthPathsFrom(t, record, "# Testing Health\n")
	if _, err := renderHealth(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, healthDest)

	unmeasured := strings.Index(page, "An unmeasured metric")
	warned := strings.Index(page, "A warned metric")
	healthy := strings.Index(page, "A healthy metric")
	if unmeasured < 0 || warned < 0 || healthy < 0 {
		t.Fatalf("the page is missing a metric: unknown at %d, warn at %d, ok at %d",
			unmeasured, warned, healthy)
	}
	if unmeasured >= warned || warned >= healthy {
		t.Errorf("the page orders unknown at %d, warn at %d, ok at %d, want them in that order",
			unmeasured, warned, healthy)
	}
	if !strings.Contains(page, `class="th-card th-unknown"`) {
		t.Error("the unmeasured metric takes no th-unknown styling")
	}
}

// VALIDATES: a status the generator does not name is treated as unmeasured
// rather than as calm.
//
// A typo'd or newly-introduced status that ranked below `ok` would sit at the
// bottom of the page with no colored border: sensor rot presenting as calm.
func TestAStatusNobodyDeclaredIsTreatedAsUnmeasured(t *testing.T) {
	record := healthRecordWith(t, []map[string]any{
		healthMetricFixture("healthy", "A healthy metric", "ok"),
		healthMetricFixture("mistyped", "A mistyped metric", "okk"),
	}, nil)
	paths := healthPathsFrom(t, record, "# Testing Health\n")
	if _, err := renderHealth(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, healthDest)

	if !strings.Contains(page, `class="th-card th-unknown"`) {
		t.Error("the mistyped status takes no th-unknown styling")
	}
	if strings.Index(page, "A mistyped metric") > strings.Index(page, "A healthy metric") {
		t.Error("the mistyped status sorts below the healthy one, so sensor rot reads as calm")
	}
	if !strings.Contains(page, "Not measured") {
		t.Error("the mistyped status carries no label")
	}
}

// VALIDATES: the page has a word for every status the generator can produce.
//
// The labels are indexed by rank, so a fourth status would silently take the
// first label and read as unmeasured.
func TestThePageHasALabelForEveryStatus(t *testing.T) {
	if len(healthStatusLabels) != len(testhealth.Statuses()) {
		t.Fatalf("the page states %d status labels for %d statuses",
			len(healthStatusLabels), len(testhealth.Statuses()))
	}
	for rank, status := range testhealth.Statuses() {
		if got := healthStatusLabel(status); got != healthStatusLabels[rank] {
			t.Errorf("status %q takes the label %q, want %q", status, got, healthStatusLabels[rank])
		}
	}
}

// VALIDATES: the trend table draws the series internal/le/testhealth states,
// under the labels it states, in its order.
//
// A second list of series here would drift from the mirror's, so one page would
// name a measurement two ways.
func TestTheTrendTableTakesItsSeriesFromTheGenerator(t *testing.T) {
	paths := healthPaths(t)
	if _, err := renderHealth(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, healthDest)

	previous := -1
	for _, series := range testhealth.TrendSeries() {
		at := strings.Index(page, "<td>"+series.Label+"</td>")
		if at < 0 {
			t.Fatalf("the trend table has no row for %q", series.Label)
		}
		if at < previous {
			t.Errorf("the trend row for %q is out of the generator's order", series.Label)
		}
		previous = at
	}
	if strings.Contains(page, "Mutation kill %") {
		t.Error("the trend table draws a series the generator no longer states")
	}
}

// VALIDATES: a series is drawn only when there are enough samples to draw one.
//
// A line through three points is noise with a direction, so the page says how
// many samples it has and draws nothing until it has four.
func TestASeriesIsDrawnOnlyWhenThereAreEnoughSamples(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		samples int
		drawn   bool
	}{
		{"three samples", 3, false},
		{"four samples", 4, true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var history []map[string]any
			for sample := range testCase.samples {
				history = append(history, map[string]any{"rfc_proof_percent": 30 + sample})
			}
			record := healthRecordWith(t, []map[string]any{
				healthMetricFixture("healthy", "A healthy metric", "ok"),
			}, history)
			paths := healthPathsFrom(t, record, "# Testing Health\n")
			if _, err := renderHealth(paths); err != nil {
				t.Fatal(err)
			}
			page := readArtifact(t, paths.Output, healthDest)

			drawn := strings.Contains(page, `class="th-spark"`)
			if drawn != testCase.drawn {
				t.Errorf("a %d-sample series draws %v, want %v", testCase.samples, drawn, testCase.drawn)
			}
			if !drawn && !strings.Contains(page, "insufficient data") {
				t.Error("a series with too few samples does not say so")
			}
			if drawn && !strings.Contains(page, `aria-label="trend over 4 samples, low 30, high 33"`) {
				t.Error("the drawn trend does not state its samples and its range")
			}
		})
	}
}

// VALIDATES: an input a page cannot be made from stops the build, by name.
//
// The retired renderer warned and served the previous artifact, so a record
// nobody could read published as the page the last build left behind.
func TestATestHealthRecordAPageCannotBeMadeFromIsRefused(t *testing.T) {
	for _, testCase := range []struct {
		name, record, page, want string
	}{
		{"not JSON", "{", "# Testing Health\n", "cannot be read"},
		{"no metric", `{"metrics":[]}`, "# Testing Health\n", "states no metric"},
		{"no label", `{"metrics":[{"key":"a","question":"Q1","status":"ok","value":"1"}]}`,
			"# Testing Health\n", "names no label"},
		{"no value", `{"metrics":[{"key":"a","question":"Q1","label":"A","status":"ok"}]}`,
			"# Testing Health\n", "states no value"},
		{"no question", `{"metrics":[{"key":"a","question":"Q9","label":"A","status":"ok","value":"1"}]}`,
			"# Testing Health\n", "Q9"},
		{"no mirror", `{"metrics":[{"key":"a","question":"Q1","label":"A","status":"ok","value":"1"}]}`,
			"", "no page"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			paths := healthPathsFrom(t, testCase.record, testCase.page)
			_, err := renderHealth(paths)
			if err == nil {
				t.Fatal("the build published a page it could not make")
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Errorf("the refusal is %q, which does not name %q", err, testCase.want)
			}
		})
	}
}

// VALIDATES: the mirror beside the page is the generated Markdown, unchanged.
//
// The mirror is docs/features/test-health.md's own bytes. A producer that
// reformatted them would make the site a second author of one document, which
// is the defect the generator was written to remove.
func TestTheHealthMirrorIsTheGeneratedPage(t *testing.T) {
	const generated = "# Testing Health\n\nGenerated prose, verbatim.\n"
	paths := healthPathsFrom(t, readFixture(t, "published-health-record.json"), generated)
	if _, err := renderHealth(paths); err != nil {
		t.Fatal(err)
	}

	mirror := readArtifact(t, paths.Output, "quality/health/"+pageMirrorFile)
	if mirror != generated {
		t.Errorf("the mirror is\n%q\nthe generator answered\n%q", mirror, generated)
	}
}

// VALIDATES: the producer claims the one route it publishes, and that route is
// one the site publishes.
func TestTheHealthProducerClaimsItsPublishedRoute(t *testing.T) {
	paths := healthPaths(t)
	routes, err := renderHealth(paths)
	if err != nil {
		t.Fatal(err)
	}
	published := map[string]bool{}
	for _, route := range publishedArtifactRoutes(t) {
		published[route] = true
	}
	for _, route := range routes {
		if !published[route] {
			t.Errorf("the producer claims %s, which the published site does not carry", route)
		}
	}
	if len(routes) != 1 {
		t.Errorf("the producer claims %d routes, want 1", len(routes))
	}
}

// VALIDATES: a long history draws a bounded line and still states its whole
// sample count.
//
// test/health/history.ndjson is append-only, so an unbounded line would put one
// SVG point in the page for every sample ever recorded.
func TestALongHistoryDrawsABoundedLine(t *testing.T) {
	var history []map[string]any
	for sample := range healthSamplesDrawn + 40 {
		history = append(history, map[string]any{"rfc_proof_percent": 30 + sample%7})
	}
	record := healthRecordWith(t, []map[string]any{
		healthMetricFixture("healthy", "A healthy metric", "ok"),
	}, history)
	paths := healthPathsFrom(t, record, "# Testing Health\n")
	if _, err := renderHealth(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, healthDest)

	if !strings.Contains(page, `aria-label="trend over `+strconv.Itoa(healthSamplesDrawn)+` samples`) {
		t.Errorf("the line is drawn over more than the %d samples it is bounded to", healthSamplesDrawn)
	}
	if !strings.Contains(page, "<td>"+strconv.Itoa(len(history))+"</td>") {
		t.Errorf("the sample column does not state the whole history of %d", len(history))
	}
}
