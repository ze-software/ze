// Design: website/AI.md -- the testing-health page is internal/le/test/health's own record
// Detail: rfccompliance.go holds the other quality page, over internal/le/rfc.
package site

import (
	"encoding/json"
	"fmt"
	"html"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	testhealth "github.com/ze-software/ze/internal/le/test/health"
)

// The testing-health page registers from here.
func init() {
	registerProducer(Producer{Name: healthProducerName, Render: renderHealth})
}

// Where the testing-health page is published.
const (
	healthProducerName = "test-health"
	healthRoute        = "/quality/health/"
	healthDest         = "quality/health/" + pageIndexFile
	healthRoot         = "../../"
)

// liveTestHealth answers the two generated test-health artifacts for one
// checkout: the metric record and the page Markdown.
//
// It is a variable so a test can state a record. A live read walks every test
// file in the tree and answers today's counters, so a golden page held against
// a live read would move with every test somebody adds.
var liveTestHealth = testhealth.Render

// renderHealth publishes the testing-health page and its mirror.
//
// The record is read from the tree being built rather than from the committed
// test/health/latest.json, because the volume counters move with every test
// added and that file is refreshed by hand. A page sourced from it would state
// yesterday's tree while claiming to describe today's.
//
// Every figure the page prints is read back from the facts snapshot this build
// published (healthFacts derives it from the same producer), and printed as a
// marked element, so the page and the snapshot state one copy of each figure.
// The record supplies the words around them: labels, prose, row names.
func renderHealth(paths Paths) ([]string, error) {
	rendered, err := paths.readTestHealth()
	if err != nil {
		return nil, err
	}
	record, err := parseHealthRecord(rendered.Record)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(rendered.Page) == "" {
		return nil, fmt.Errorf("test-health answered no page, so %s would have no mirror", healthDest)
	}
	links, err := loadPageLinks(paths.Source)
	if err != nil {
		return nil, err
	}
	snapshot, err := loadSiteFactSnapshot(paths.Output)
	if err != nil {
		return nil, err
	}
	lookup := &factLookup{snapshot: snapshot}
	body := healthBody(record, lookup)
	if err := lookup.failure(); err != nil {
		return nil, fmt.Errorf("%s cannot state its figures: %w", healthDest, err)
	}

	const description = "Whether a regression would be caught: proof density, tests that cannot " +
		"fail, tests nothing runs, and how they move over time."
	shell := pageShell{
		Title:       "Testing Health - Ze",
		Description: description,
		Root:        healthRoot,
		Path:        healthDest,
		Sidebar:     pageSidebar(healthRoot, healthDest, links),
	}
	if err := writePublishedPage(paths.Output, healthDest,
		shell.render(body), rendered.Page); err != nil {
		return nil, err
	}
	return []string{healthRoute}, nil
}

// healthRecord is one whole reading of the tree, as internal/le/test/health
// states it.
//
// A metric is read by key rather than into a struct because the payload beside
// the seven named fields differs for each one: a ratio, a table of worst rows,
// a list of stranded files, a table of package ages. The renderer is generic
// over that payload, which is what lets a metric the collectors gain publish
// without a change here.
type healthRecord struct {
	Metrics []healthMetric   `json:"metrics"`
	History []map[string]any `json:"history"`
}

// healthMetric is one metric: the seven named fields and its own payload.
type healthMetric map[string]any

// parseHealthRecord reads the record, refusing one no page can be made from.
//
// The retired renderer warned and served the previous artifact, so a record
// nobody could read published as whatever the last build left behind. Each
// refusal names the metric, because a page with one wrong metric is harder to
// find than a build that stopped.
func parseHealthRecord(body string) (healthRecord, error) {
	// UseNumber keeps each number spelled as the record spells it. Decoding
	// into float64 and re-rendering turns a mutation score of 0.0 into 0, so
	// the page would state a precision the collector did not.
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.UseNumber()
	var record healthRecord
	if err := decoder.Decode(&record); err != nil {
		return healthRecord{}, fmt.Errorf("the test-health record cannot be read: %w", err)
	}
	if len(record.Metrics) == 0 {
		return healthRecord{}, fmt.Errorf("the test-health record states no metric, so %s would be empty", healthDest)
	}
	asked := map[string]bool{}
	for _, question := range testhealth.Questions() {
		asked[question.Key] = true
	}
	for position, metric := range record.Metrics {
		name := metric.text("key")
		if name == "" {
			name = "metric " + strconv.Itoa(position+1)
		}
		if metric.text("label") == "" {
			return healthRecord{}, fmt.Errorf("test-health %s names no label", name)
		}
		if metric.text("value") == "" {
			return healthRecord{}, fmt.Errorf("test-health %s states no value", name)
		}
		if !asked[metric.text("question")] {
			return healthRecord{}, fmt.Errorf("test-health %s asks %q, which is none of the three questions",
				name, metric.text("question"))
		}
	}
	return record, nil
}

// text answers one field as the string it holds, and "" for anything else.
func (metric healthMetric) text(key string) string {
	word, ok := metric[key].(string)
	if !ok {
		return ""
	}
	return word
}

// payloadKeys answers the metric's own keys, sorted, without the seven the page
// prints by name.
//
// Sorted because the record is written with its keys sorted, so this is the
// order the payload arrives in and the order the published page shows.
func (metric healthMetric) payloadKeys() []string {
	named := map[string]bool{"key": true, "question": true, "label": true,
		"status": true, "value": true, "detail": true, "action": true}
	keys := make([]string, 0, len(metric))
	for key := range metric {
		if !named[key] {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys
}

// healthStatusRank answers where a metric sorts. A status the generator does
// not name ranks with the first one, so a typo cannot present as calm.
func healthStatusRank(status string) int {
	if rank := slices.Index(testhealth.Statuses(), status); rank >= 0 {
		return rank
	}
	return 0
}

// healthCalmStatus is the status that needs no attention: the last of the
// generator's worst-first order.
func healthCalmStatus() string {
	named := testhealth.Statuses()
	return named[len(named)-1]
}

// healthStatusName answers the status a metric is styled and labeled by, which
// is the generator's own word for a status it names and the first status
// otherwise.
func healthStatusName(status string) string {
	if slices.Contains(testhealth.Statuses(), status) {
		return status
	}
	return testhealth.Statuses()[0]
}

// healthStatusLabels are the words a card prints beside a value, indexed by the
// generator's own worst-first rank. They are the page's own vocabulary: the
// Markdown mirror writes the generator's compact spelling, which reads as a
// table cell rather than as a badge.
var healthStatusLabels = []string{"Not measured", "Needs attention", "Within threshold"}

// healthStatusLabel answers the badge one status prints. A rank this has no
// word for takes the first, which is the same fail-worst rule the rank uses.
func healthStatusLabel(status string) string {
	rank := healthStatusRank(status)
	if rank >= len(healthStatusLabels) {
		return healthStatusLabels[0]
	}
	return healthStatusLabels[rank]
}

// byHealthStatus orders metrics worst first, keeping the record's own order
// within one status.
func byHealthStatus(metrics []healthMetric) []healthMetric {
	out := make([]healthMetric, len(metrics))
	copy(out, metrics)
	sort.SliceStable(out, func(left, right int) bool {
		return healthStatusRank(out[left].text("status")) < healthStatusRank(out[right].text("status"))
	})
	return out
}

// healthBody renders the page under <main>, every figure read through lookup.
func healthBody(record healthRecord, lookup *factLookup) string {
	var body textbuf.Buffer
	body.Str("            <section aria-labelledby=\"test-health-title\" class=\"md-content reveal cat-observe\">\n")
	body.Str(pageHero("Testing Health",
		"Not how many tests exist, but whether a regression would be caught. A suite can grow "+
			//nolint:misspell // Recovered published prose: the page says "behaviour".
			"forever while the share of behaviour it actually pins falls, and no count of tests can "+
			"show that. Every metric here belongs to one of three questions; anything belonging to "+
			"none is volume, and is deliberately absent.",
		"Observe", ` id="test-health-title"`, heroClasses)).Byte('\n')
	body.Str(healthStyle)
	body.Str(healthAttention(record.Metrics, lookup))
	for _, question := range testhealth.Questions() {
		var group []healthMetric
		for _, metric := range record.Metrics {
			if metric.text("question") == question.Key {
				group = append(group, metric)
			}
		}
		if len(group) == 0 {
			continue
		}
		body.Str("<section><h2>").Str(html.EscapeString(question.Title)).Str("</h2><p><em>").
			Str(html.EscapeString(question.Prompt)).Str("</em></p>\n")
		for _, metric := range byHealthStatus(group) {
			body.Str(healthCardHTML(metric, lookup))
		}
		body.Str("</section>\n")
	}
	body.Str(healthTrends(record.History, lookup))
	body.Str("            </section>\n")
	return body.String()
}

// healthAttention renders the exceptions table, or the one line that says there
// are none. Green is the absence of information, so the exceptions come first.
func healthAttention(metrics []healthMetric, lookup *factLookup) string {
	var problems []healthMetric
	calm := healthCalmStatus()
	for _, metric := range metrics {
		if metric.text("status") != calm {
			problems = append(problems, metric)
		}
	}
	if len(problems) == 0 {
		return `<section class="th-attention"><h2>Needs attention</h2>` +
			"<p>Nothing outstanding. Every metric is within its threshold.</p></section>\n"
	}

	var out textbuf.Buffer
	out.Str(`<section class="th-attention"><h2>Needs attention</h2>`).
		Str("<table><thead><tr><th>Metric</th><th>Question</th><th>Value</th>").
		Str("<th>What to do</th></tr></thead><tbody>")
	for _, metric := range byHealthStatus(problems) {
		out.Str("<tr><td>").Str(html.EscapeString(metric.text("label"))).Str("</td><td>").
			Str(html.EscapeString(metric.text("question"))).Str("</td><td><strong>").
			Str(healthTextHTML(metric, "value", lookup)).Str("</strong></td><td>").
			Str(healthTextHTML(metric, "action", lookup)).Str("</td></tr>")
	}
	out.Str("</tbody></table></section>\n")
	return out.String()
}

// healthTextHTML renders one composed field of a metric (healthComposedFields):
// the record's words, with each figure as its marked snapshot value.
func healthTextHTML(metric healthMetric, field string, lookup *factLookup) string {
	var out textbuf.Buffer
	key := metric.text("key")
	for _, part := range healthTextParts(metric.text(field)) {
		if part.figure == 0 {
			out.Str(html.EscapeString(part.text))
			continue
		}
		out.Str(lookup.marked(healthFigureKey(healthSection, key, field, strconv.Itoa(part.figure))))
	}
	return out.String()
}

// healthCellHTML renders one table cell's content: a figure as its marked
// snapshot value under segments, and a word as the record spells it.
func healthCellHTML(value any, lookup *factLookup, segments ...string) string {
	if _, isFigure := healthCellFigure(value); isFigure {
		return lookup.marked(healthFigureKey(healthSection, segments...))
	}
	return html.EscapeString(healthValueText(value))
}

// healthCardHTML renders one metric.
func healthCardHTML(metric healthMetric, lookup *factLookup) string {
	var out textbuf.Buffer
	out.Str(`<article class="th-card th-`).Str(healthStatusName(metric.text("status"))).Str("\">\n")
	out.Str("  <h3>").Str(html.EscapeString(metric.text("label"))).Str("</h3>\n")
	out.Str(`  <p class="th-value">`).Str(healthTextHTML(metric, "value", lookup)).Str(` <span class="th-status">`).
		Str(healthStatusLabel(metric.text("status"))).Str("</span></p>\n")
	out.Str("  ").Str(healthMeter(metric, lookup)).Byte('\n')
	out.Str("  <p>").Str(healthTextHTML(metric, "detail", lookup)).Str("</p>\n")
	out.Str(`  <p class="th-action"><strong>If this degrades:</strong> `).
		Str(healthTextHTML(metric, "action", lookup)).Str("</p>\n")
	out.Str("  ").Str(healthDetailTable(metric, lookup)).Byte('\n')
	out.Str("</article>\n")
	return out.String()
}

// healthMeter renders the proportion bar of a metric that carries a ratio, and
// nothing for one that does not.
//
// Both parts are printed beside the bar. A bar alone hides the case where a
// ratio improves only because its denominator shrank. The bar's label names
// the metric and states no figure: an attribute cannot hold a marked element,
// so a figure there would be a copy site.js never refreshes, and the note under
// the bar states the figures, marked. The bar's width is the percentage too, so
// the fill names the percentage's key in data-ze-stat-width and site.js sets
// the width from the snapshot, exactly as it sets the note's text: the style
// the build writes is only the last build's fallback, and a bar never shows a
// proportion the note beside it no longer states.
func healthMeter(metric healthMetric, lookup *factLookup) string {
	ratio, _, _, drawn := healthRatio(metric)
	// refuse-by-default: guards healthCardHTML -- a metric that carries no ratio draws no meter, which is the empty fragment
	if !drawn {
		return ""
	}
	key := metric.text("key")
	numeratorKey := healthFigureKey(healthSection, key, ratio, "numerator")
	denominatorKey := healthFigureKey(healthSection, key, ratio, "denominator")
	percentKey := healthFigureKey(healthSection, key, ratio, "percent")
	label := html.EscapeString(metric.text("label"))
	width := html.EscapeString(lookup.plain(percentKey))
	return `<div class="th-meter" role="img" aria-label="` + label +
		`"><span class="th-meter-fill" data-ze-stat-width="` + html.EscapeString(percentKey) +
		`" style="width:` + width + `%"></span></div>` +
		`<p class="th-meter-note">` + lookup.marked(numeratorKey) + " of " + lookup.marked(denominatorKey) +
		" (" + lookup.marked(percentKey) + "%)</p>"
}

// healthDetailTable renders a metric's supporting table: its worst rows, then
// its stranded files, then its package-age buckets.
func healthDetailTable(metric healthMetric, lookup *factLookup) string {
	if table, ok := healthWorstTable(metric, lookup); ok {
		return table
	}
	if table, ok := healthOrphanTable(metric, lookup); ok {
		return table
	}
	if table, ok := healthBucketTable(metric, lookup); ok {
		return table
	}
	return ""
}

// healthWorstTable renders the `worst` list, whatever keys its rows carry.
//
// The header is the UNION of the rows' keys, sorted. Taking them from the first
// row alone raised an error the moment a later row carried a different shape,
// and a malformed collector must not take the whole site build down.
func healthWorstTable(metric healthMetric, lookup *factLookup) (string, bool) {
	entries := healthTableRows(metric, "worst")
	keySet := map[string]bool{}
	for _, entry := range entries {
		for key := range entry {
			keySet[key] = true
		}
	}
	if len(entries) == 0 {
		return "", false
	}
	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	var out textbuf.Buffer
	out.Str("<table><thead><tr>")
	for _, key := range keys {
		out.Str("<th>").Str(html.EscapeString(strings.ReplaceAll(key, "_", " "))).Str("</th>")
	}
	out.Str("</tr></thead><tbody>")
	name := metric.text("key")
	for row, entry := range entries {
		out.Str("<tr>")
		for _, key := range keys {
			out.Str("<td><code>").Str(healthCellHTML(entry[key], lookup, name, "worst", strconv.Itoa(row+1), key)).
				Str("</code></td>")
		}
		out.Str("</tr>")
	}
	out.Str("</tbody></table>")
	return out.String(), true
}

// healthOrphanTable renders the stranded test files and the tags they wanted.
func healthOrphanTable(metric healthMetric, lookup *factLookup) (string, bool) {
	entries := healthTableRows(metric, "orphans")
	if len(entries) == 0 {
		return "", false
	}
	name := metric.text("key")
	var out textbuf.Buffer
	out.Str("<table><thead><tr><th>File</th><th>Requires</th></tr></thead><tbody>")
	for row, entry := range entries {
		position := strconv.Itoa(row + 1)
		out.Str("<tr><td><code>").Str(healthFieldHTML(entry, lookup, name, "orphans", position, "file")).
			Str("</code></td><td><code>").
			Str(healthFieldHTML(entry, lookup, name, "orphans", position, "requires")).Str("</code></td></tr>")
	}
	out.Str("</tbody></table>")
	return out.String(), true
}

// healthFieldHTML renders one row's named field (healthCellHTML), and the
// question mark the retired renderer printed for a row that does not carry it.
// The field's name is the last of segments.
func healthFieldHTML(entry map[string]any, lookup *factLookup, segments ...string) string {
	value, held := entry[segments[len(segments)-1]]
	if !held {
		return "?"
	}
	return healthCellHTML(value, lookup, segments...)
}

// healthBucketTable renders the package-age table of the adoption metric.
func healthBucketTable(metric healthMetric, lookup *factLookup) (string, bool) {
	buckets, ok := metric["buckets"].(map[string]any)
	if !ok || len(buckets) == 0 {
		return "", false
	}
	years := make([]string, 0, len(buckets))
	for year := range buckets {
		years = append(years, year)
	}
	slices.Sort(years)

	var out textbuf.Buffer
	out.Str("<table><thead><tr><th>Package first commit</th><th>Packages with tests</th>").
		Str("<th>With a fuzz target</th><th>With an RFC-tagged test</th>").
		Str("<th>With a .ci scenario</th></tr></thead><tbody>")
	for _, year := range years {
		slot, isObject := buckets[year].(map[string]any)
		if !isObject {
			continue
		}
		out.Str("<tr><td>").Str(html.EscapeString(year)).Str("</td>")
		for _, key := range []string{"packages", "with_fuzz", "with_rfc_tag", "with_ci"} {
			out.Str("<td>").Str(healthFieldHTML(slot, lookup, metric.text("key"), "buckets", year, key)).Str("</td>")
		}
		out.Str("</tr>")
	}
	out.Str("</tbody></table>")
	return out.String(), true
}

// healthTrends renders the trend table, one row for each series the generator
// states.
func healthTrends(history []map[string]any, lookup *factLookup) string {
	var out textbuf.Buffer
	out.Str(`<section class="th-trends"><h2>Evolution over time</h2>`).
		Str("<p>Each row shows its sample count. A statistic without its <em>n</em> is ").
		Str("an assertion, not a measurement, and a trend drawn through three points is ").
		Str("noise with a direction.</p>").Str("<table><thead><tr><th>Series</th><th>Trend</th><th>Latest</th>").
		Str("<th>Samples</th></tr></thead><tbody>")
	for _, series := range testhealth.TrendSeries() {
		values := healthSeriesValues(history, series.Key)
		label := html.EscapeString(series.Label)
		samples := lookup.marked(healthFigureKey(healthTrendsSection, series.Key, "samples"))
		if len(values) < healthMinSamples {
			out.Str("<tr><td>").Str(label).Str("</td><td><em>insufficient data</em></td><td>-</td><td>").
				Str(samples).Str("</td></tr>")
			continue
		}
		out.Str("<tr><td>").Str(label).Str("</td><td>").Str(healthSparkline(healthDrawnSamples(values), series.Key, lookup)).
			Str("</td><td><code>").
			Str(lookup.marked(healthFigureKey(healthTrendsSection, series.Key, "latest"))).
			Str("</code></td><td>").Str(samples).Str("</td></tr>")
	}
	out.Str("</tbody></table></section>\n")
	return out.String()
}

// The trend line's shape. healthMinSamples matches the generator's own floor: a
// line through three points is noise with a direction.
//
// healthSamplesDrawn bounds the line. test/health/history.ndjson is append-only
// and grows one row for each recorded run, so an unbounded line would put one
// SVG point in the page for every sample ever taken. The most recent samples
// are the ones a trend is read from; the sample column still states the total.
const (
	healthMinSamples      = 4
	healthSamplesDrawn    = 120
	healthSparklineWidth  = 260
	healthSparklineHeight = 44
)

// healthSparkline renders an inline SVG polyline, and the sentence a screen
// reader reads in its place.
//
// No chart library: the site publishes content that is meaningful without
// JavaScript, and its content security policy blocks an external script.
//
// The geometry is computed from the numbers. The drawing is hidden from a
// screen reader, and the sentence beside it states how many samples it draws
// and their low and high as marked snapshot values (healthSparkRange): an
// attribute cannot hold a marked element, so a figure in an aria-label would
// be a copy site.js never refreshes.
func healthSparkline(values []json.Number, series string, lookup *factLookup) string {
	if len(values) < 2 {
		return ""
	}
	low, high := healthFloat(values[0]), healthFloat(values[0])
	for _, value := range values {
		low = min(low, healthFloat(value))
		high = max(high, healthFloat(value))
	}
	span := high - low
	if span == 0 {
		span = 1
	}
	step := float64(healthSparklineWidth) / float64(len(values)-1)

	var points textbuf.Buffer
	for index, value := range values {
		if index > 0 {
			points.Byte(' ')
		}
		points.Float(float64(index)*step, 1).Byte(',').
			Float(float64(healthSparklineHeight)-
				((healthFloat(value)-low)/span)*float64(healthSparklineHeight-6)-3, 1)
	}
	size := strconv.Itoa(healthSparklineWidth) + " " + strconv.Itoa(healthSparklineHeight)
	return `<svg class="th-spark" viewBox="0 0 ` + size + `" width="` +
		strconv.Itoa(healthSparklineWidth) + `" height="` + strconv.Itoa(healthSparklineHeight) +
		`" aria-hidden="true"><polyline points="` + points.String() +
		`" fill="none" stroke="currentColor" stroke-width="2" ` +
		`stroke-linejoin="round" stroke-linecap="round"/></svg>` +
		`<span class="sr-only">trend over ` + lookup.marked(healthFigureKey(healthTrendsSection, series, "drawn")) +
		` samples, low ` + lookup.marked(healthFigureKey(healthTrendsSection, series, "low")) +
		`, high ` + lookup.marked(healthFigureKey(healthTrendsSection, series, "high")) + `</span>`
}

// healthFloat answers a recorded number as a float, and zero for one that is
// not a number. A sample the collector could not compute plots at the floor
// rather than stopping the page.
func healthFloat(value json.Number) float64 {
	number, err := value.Float64()
	if err != nil {
		return 0
	}
	return number
}

// healthValueText answers one JSON value as the page prints it.
func healthValueText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case json.Number:
		return typed.String()
	default:
		return fmt.Sprint(typed)
	}
}

// healthStyle is the page's own stylesheet, recovered from the published page.
// It is inline because these rules serve one page and nothing else links them.
const healthStyle = `<style>
.th-card { border-radius: 14px; padding: 1.1rem 1.25rem; margin: 0 0 1rem; background: var(--panel); box-shadow: 0 2px 0 var(--shadow); border: 1px solid var(--line); }
.th-card h3 { margin: 0 0 .35rem; font-size: 1.05rem; }
.th-value { font-size: 1.5rem; font-weight: 700; margin: .2rem 0 .5rem; }
.th-status { font-size: .8rem; font-weight: 600; opacity: .7; margin-left: .5rem; }
.th-warn { border-left: 6px solid var(--amber); }
.th-unknown { border-left: 6px solid var(--dim); }
.th-ok { border-left: 6px solid var(--green); }
.th-meter { display: block; height: 10px; border-radius: 6px; background: var(--line); overflow: hidden; }
.th-meter-fill { display: block; height: 100%; background: currentColor; opacity: .55; }
.th-meter-note { font-size: .85rem; opacity: .75; margin: .3rem 0 .6rem; }
.th-action { font-size: .9rem; }
.th-spark { vertical-align: middle; }
.th-attention table, .th-trends table, .th-card table { width: 100%; border-collapse: collapse; }
.th-card table { font-size: .85rem; margin-top: .6rem; }
.th-card td, .th-card th, .th-attention td, .th-attention th, .th-trends td, .th-trends th { text-align: left; padding: .35rem .5rem; border-bottom: 1px solid var(--line); }
</style>
`
