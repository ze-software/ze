// Design: website/AI.md -- the testing-health page is internal/le/test/health's own record
package site

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	testhealth "github.com/ze-software/ze/internal/le/test/health"
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
	paths := Paths{Repository: root, Source: source, Output: t.TempDir()}
	publishHealthFacts(t, paths.Output, record)
	return paths
}

// publishHealthFacts writes the facts snapshot a build publishes before the
// page producers run, holding the health sections derived from the stated
// record, because the page reads every figure back from it.
//
// A record no page can be made from publishes no snapshot: renderHealth then
// refuses the record itself, by the reason each refusal case names.
func publishHealthFacts(t *testing.T, output, record string) {
	t.Helper()
	parsed, err := parseHealthRecord(record)
	if err != nil {
		return
	}
	health, trends, err := healthFacts(parsed)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(map[string]any{healthSection: health, healthTrendsSection: trends})
	if err != nil {
		t.Fatal(err)
	}
	writeArtifactFile(t, output, factsFile, string(encoded))
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
// drew four series where internal/le/test/health now states three, because the
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

// VALIDATES: the trend table draws the series internal/le/test/health states,
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
			series := func(field, value string) string {
				return markedFact(healthFigureKey(healthTrendsSection, "rfc_proof_percent", field), value)
			}
			sentence := `<span class="sr-only">trend over ` + series("drawn", "4") + ` samples, low ` +
				series("low", "30") + `, high ` + series("high", "33") + `</span>`
			if drawn && !strings.Contains(page, sentence) {
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

	drawn := markedFact(healthFigureKey(healthTrendsSection, "rfc_proof_percent", "drawn"), strconv.Itoa(healthSamplesDrawn))
	if !strings.Contains(page, `<span class="sr-only">trend over `+drawn+` samples`) {
		t.Errorf("the line is drawn over more than the %d samples it is bounded to", healthSamplesDrawn)
	}
	samples := markedFact(healthFigureKey(healthTrendsSection, "rfc_proof_percent", "samples"), strconv.Itoa(len(history)))
	if !strings.Contains(page, "<td>"+samples+"</td>") {
		t.Errorf("the sample column does not state the whole history of %d", len(history))
	}
}

// VALIDATES: AC-18. Every figure the testing-health page prints is a marked
// element under health.<metric key> or health_trends.<series>, whose text is
// the snapshot's value for that key, and the snapshot holds every such key.
//
// Method: render the published record with a history long enough to draw every
// series. Forward, every key of the two health sections must appear on the
// page as its marked element, and every metric whose value states a figure
// must carry that figure's key. Reverse, with every marked element, code
// element, style and drawing cut away, no number may be left in what a reader
// sees: a figure the page prints from the record directly is red. The only
// number the reverse half allows is a package-age bucket's name, which the
// record states as a key, not as a figure.
func TestEveryHealthFigureIsAMarkedSnapshotValue(t *testing.T) {
	var document map[string]any
	if err := json.Unmarshal([]byte(readFixture(t, "published-health-record.json")), &document); err != nil {
		t.Fatal(err)
	}
	var history []any
	for sample := range healthMinSamples + 1 {
		history = append(history, map[string]any{
			"assert_nothing": 140 - sample, "rfc_proof_percent": 35.6 + float64(sample), "tag_orphan": 12,
		})
	}
	document["history"] = history
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	paths := healthPathsFrom(t, string(encoded), "# Testing Health\n")
	if _, err := renderHealth(paths); err != nil {
		t.Fatal(err)
	}
	page := mainContent(t, readArtifact(t, paths.Output, healthDest))
	snapshot, err := loadSiteFactSnapshot(paths.Output)
	if err != nil {
		t.Fatal(err)
	}
	record, err := parseHealthRecord(string(encoded))
	if err != nil {
		t.Fatal(err)
	}

	stated := map[string]bool{}
	for _, key := range snapshot.keys() {
		if !strings.HasPrefix(key, "health.") && !strings.HasPrefix(key, "health_trends.") {
			continue
		}
		stated[key] = true
		value, _ := snapshot.value(key)
		if !strings.Contains(page, markedFact(key, value)) {
			t.Errorf("the page does not show %s as its marked value %q", key, value)
		}
	}
	figure := regexp.MustCompile(`\d+(?:\.\d+)?`)
	bucketNames := map[string]bool{}
	for _, metric := range record.Metrics {
		if figure.MatchString(metric.text("value")) && !stated["health."+metric.text("key")+".value.1"] {
			t.Errorf("the snapshot carries no figure of %s's value %q", metric.text("key"), metric.text("value"))
		}
		buckets, _ := metric["buckets"].(map[string]any)
		for name := range buckets {
			bucketNames[name] = true
		}
	}
	for _, series := range testhealth.TrendSeries() {
		for _, field := range []string{"latest", "samples"} {
			if !stated["health_trends."+series.Key+"."+field] {
				t.Errorf("the snapshot carries no %s of the %s series", field, series.Key)
			}
		}
	}

	unmarked := page
	for _, cut := range []*regexp.Regexp{
		regexp.MustCompile(`<span data-ze-stat="[^"]*">[^<]*</span>`),
		regexp.MustCompile(`(?s)<code>.*?</code>`),
		regexp.MustCompile(`(?s)<style>.*?</style>`),
		regexp.MustCompile(`(?s)<svg.*?</svg>`),
	} {
		unmarked = cut.ReplaceAllString(unmarked, "")
	}
	standalone := regexp.MustCompile(`(^|[^\w.])(\d+(?:\.\d+)?)`)
	for _, match := range standalone.FindAllStringSubmatch(visibleText(unmarked), -1) {
		if bucketNames[match[2]] {
			continue
		}
		t.Errorf("the page prints %q outside a marked element", match[2])
	}
	// An accessible label is read without the script that fills a marker, so
	// a figure in one is a copy site.js never refreshes.
	for _, label := range regexp.MustCompile(`aria-label="([^"]*)"`).FindAllStringSubmatch(page, -1) {
		if figure.MatchString(label[1]) {
			t.Errorf("the page states a figure in the accessible label %q", label[1])
		}
	}
}

// VALIDATES: a meter's bar is drawn from the percentage the snapshot holds,
// and names that percentage's key, so site.js redraws the bar from the same
// file it fills the figure beside it from.
//
// Method: render the published record and read every meter fill. Each one
// names a key under health.<metric>.<ratio>.percent that the snapshot holds,
// its style width is that value, and the note beside the bar shows the same
// key marked. A fill naming no key, or a key the snapshot lacks, is a bar
// whose width no JSON update can move.
func TestTheMeterWidthIsTheMarkedPercentage(t *testing.T) {
	paths := healthPaths(t)
	if _, err := renderHealth(paths); err != nil {
		t.Fatal(err)
	}
	page := mainContent(t, readArtifact(t, paths.Output, healthDest))
	snapshot, err := loadSiteFactSnapshot(paths.Output)
	if err != nil {
		t.Fatal(err)
	}

	fills := regexp.MustCompile(`<span class="th-meter-fill"([^>]*)>`).FindAllStringSubmatch(page, -1)
	if len(fills) == 0 {
		t.Fatal("the published record draws no meter, so this test proves nothing")
	}
	named := regexp.MustCompile(`^ data-ze-stat-width="(health\.[^"]+\.percent)" style="width:([^"%]*)%"$`)
	for _, fill := range fills {
		parts := named.FindStringSubmatch(fill[1])
		if parts == nil {
			t.Errorf("the meter fill %q names no percentage key the browser can redraw it from", fill[0])
			continue
		}
		value, held := snapshot.value(parts[1])
		if !held {
			t.Errorf("the meter fill names %s, which the snapshot does not hold", parts[1])
			continue
		}
		if parts[2] != value {
			t.Errorf("the meter fill for %s is %s%% wide, but the snapshot states %s", parts[1], parts[2], value)
		}
		if !strings.Contains(page, markedFact(parts[1], value)+"%)") {
			t.Errorf("the note beside the %s bar does not show the same key marked", parts[1])
		}
	}
}
