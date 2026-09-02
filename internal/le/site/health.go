// Design: website/AI.md -- the testing-health page is internal/le/testhealth's own record
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
	"github.com/ze-software/ze/internal/le/testhealth"
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
// The numbers are read from the tree being built rather than from the committed
// snapshot, because the volume counters move with every test added and the
// committed snapshot is refreshed by hand. A page sourced from it would state
// yesterday's tree while claiming to describe today's.
func renderHealth(paths Paths) ([]string, error) {
	rendered, err := liveTestHealth(paths.Repository)
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
		shell.render(healthBody(record)), rendered.Page); err != nil {
		return nil, err
	}
	return []string{healthRoute}, nil
}

// healthRecord is one whole reading of the tree, as internal/le/testhealth
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
	sort.Strings(keys)
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

// healthBody renders the page under <main>.
func healthBody(record healthRecord) string {
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
	body.Str(healthAttention(record.Metrics))
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
			body.Str(healthCardHTML(metric))
		}
		body.Str("</section>\n")
	}
	body.Str(healthTrends(record.History))
	body.Str("            </section>\n")
	return body.String()
}

// healthAttention renders the exceptions table, or the one line that says there
// are none. Green is the absence of information, so the exceptions come first.
func healthAttention(metrics []healthMetric) string {
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
			Str(html.EscapeString(metric.text("value"))).Str("</strong></td><td>").
			Str(html.EscapeString(metric.text("action"))).Str("</td></tr>")
	}
	out.Str("</tbody></table></section>\n")
	return out.String()
}

// healthCardHTML renders one metric.
func healthCardHTML(metric healthMetric) string {
	var out textbuf.Buffer
	out.Str(`<article class="th-card th-`).Str(healthStatusName(metric.text("status"))).Str("\">\n")
	out.Str("  <h3>").Str(html.EscapeString(metric.text("label"))).Str("</h3>\n")
	out.Str(`  <p class="th-value">`).Str(html.EscapeString(metric.text("value"))).Str(` <span class="th-status">`).
		Str(healthStatusLabel(metric.text("status"))).Str("</span></p>\n")
	out.Str("  ").Str(healthMeter(metric)).Byte('\n')
	out.Str("  <p>").Str(html.EscapeString(metric.text("detail"))).Str("</p>\n")
	out.Str(`  <p class="th-action"><strong>If this degrades:</strong> `).
		Str(html.EscapeString(metric.text("action"))).Str("</p>\n")
	out.Str("  ").Str(healthDetailTable(metric)).Byte('\n')
	out.Str("</article>\n")
	return out.String()
}

// healthMeter renders the proportion bar of a metric that carries a ratio, and
// nothing for one that does not.
//
// Both parts are printed beside the bar. A bar alone hides the case where a
// ratio improves only because its denominator shrank.
func healthMeter(metric healthMetric) string {
	for _, key := range metric.payloadKeys() {
		part, isObject := metric[key].(map[string]any)
		if !isObject {
			continue
		}
		spelled, isNumber := part["percent"].(json.Number)
		if !isNumber {
			continue
		}
		percent, err := spelled.Float64()
		if err != nil {
			continue
		}
		numerator := html.EscapeString(healthValueText(part["numerator"]))
		denominator := html.EscapeString(healthValueText(part["denominator"]))
		width := strconv.FormatFloat(percent, 'f', 1, 64)
		return `<div class="th-meter" role="img" aria-label="` + numerator + " of " + denominator +
			`"><span class="th-meter-fill" style="width:` + width + `%"></span></div>` +
			`<p class="th-meter-note">` + numerator + " of " + denominator + " (" + width + "%)</p>"
	}
	return ""
}

// healthDetailTable renders a metric's supporting table: its worst rows, then
// its stranded files, then its package-age buckets.
func healthDetailTable(metric healthMetric) string {
	if table, ok := healthWorstTable(metric); ok {
		return table
	}
	if table, ok := healthOrphanTable(metric); ok {
		return table
	}
	if table, ok := healthBucketTable(metric); ok {
		return table
	}
	return ""
}

// healthWorstTable renders the `worst` list, whatever keys its rows carry.
//
// The header is the UNION of the rows' keys, sorted. Taking them from the first
// row alone raised an error the moment a later row carried a different shape,
// and a malformed collector must not take the whole site build down.
func healthWorstTable(metric healthMetric) (string, bool) {
	rows, ok := metric["worst"].([]any)
	if !ok || len(rows) == 0 {
		return "", false
	}
	var entries []map[string]any
	keySet := map[string]bool{}
	for _, raw := range rows {
		entry, isObject := raw.(map[string]any)
		if !isObject {
			continue
		}
		entries = append(entries, entry)
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
	sort.Strings(keys)

	var out textbuf.Buffer
	out.Str("<table><thead><tr>")
	for _, key := range keys {
		out.Str("<th>").Str(html.EscapeString(strings.ReplaceAll(key, "_", " "))).Str("</th>")
	}
	out.Str("</tr></thead><tbody>")
	for _, entry := range entries {
		out.Str("<tr>")
		for _, key := range keys {
			out.Str("<td><code>").Str(html.EscapeString(healthValueText(entry[key]))).Str("</code></td>")
		}
		out.Str("</tr>")
	}
	out.Str("</tbody></table>")
	return out.String(), true
}

// healthOrphanTable renders the stranded test files and the tags they wanted.
func healthOrphanTable(metric healthMetric) (string, bool) {
	rows, ok := metric["orphans"].([]any)
	if !ok || len(rows) == 0 {
		return "", false
	}
	var out textbuf.Buffer
	out.Str("<table><thead><tr><th>File</th><th>Requires</th></tr></thead><tbody>")
	for _, raw := range rows {
		entry, isObject := raw.(map[string]any)
		if !isObject {
			continue
		}
		out.Str("<tr><td><code>").Str(html.EscapeString(healthField(entry, "file"))).Str("</code></td><td><code>").
			Str(html.EscapeString(healthField(entry, "requires"))).Str("</code></td></tr>")
	}
	out.Str("</tbody></table>")
	return out.String(), true
}

// healthField answers one row's field, and the question mark the retired
// renderer printed for a row that does not carry it.
func healthField(entry map[string]any, key string) string {
	value, held := entry[key]
	if !held {
		return "?"
	}
	return healthValueText(value)
}

// healthBucketTable renders the package-age table of the adoption metric.
func healthBucketTable(metric healthMetric) (string, bool) {
	buckets, ok := metric["buckets"].(map[string]any)
	if !ok || len(buckets) == 0 {
		return "", false
	}
	years := make([]string, 0, len(buckets))
	for year := range buckets {
		years = append(years, year)
	}
	sort.Strings(years)

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
			out.Str("<td>").Str(html.EscapeString(healthField(slot, key))).Str("</td>")
		}
		out.Str("</tr>")
	}
	out.Str("</tbody></table>")
	return out.String(), true
}

// healthTrends renders the trend table, one row for each series the generator
// states.
func healthTrends(history []map[string]any) string {
	var out textbuf.Buffer
	out.Str(`<section class="th-trends"><h2>Evolution over time</h2>`).
		Str("<p>Each row shows its sample count. A statistic without its <em>n</em> is ").
		Str("an assertion, not a measurement, and a trend drawn through three points is ").
		Str("noise with a direction.</p>").Str("<table><thead><tr><th>Series</th><th>Trend</th><th>Latest</th>").
		Str("<th>Samples</th></tr></thead><tbody>")
	for _, series := range testhealth.TrendSeries() {
		var values []json.Number
		for _, sample := range history {
			number, isNumber := sample[series.Key].(json.Number)
			if !isNumber {
				continue
			}
			values = append(values, number)
		}
		label := html.EscapeString(series.Label)
		if len(values) < healthMinSamples {
			out.Str("<tr><td>").Str(label).Str("</td><td><em>insufficient data</em></td><td>-</td><td>").
				Int(int64(len(values))).Str("</td></tr>")
			continue
		}
		drawn := values
		if len(drawn) > healthSamplesDrawn {
			drawn = drawn[len(drawn)-healthSamplesDrawn:]
		}
		out.Str("<tr><td>").Str(label).Str("</td><td>").Str(healthSparkline(drawn)).Str("</td><td><code>").
			Str(values[len(values)-1].String()).Str("</code></td><td>").Int(int64(len(values))).Str("</td></tr>")
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

// healthSparkline renders an inline SVG polyline.
//
// No chart library: the site publishes content that is meaningful without
// JavaScript, and its content security policy blocks an external script.
//
// The geometry is computed from the numbers; the label states the low and the
// high as the record spells them, so a sample of 0.0 is not reported as 0.
func healthSparkline(values []json.Number) string {
	if len(values) < 2 {
		return ""
	}
	low, high := healthFloat(values[0]), healthFloat(values[0])
	lowest, highest := values[0], values[0]
	for _, value := range values {
		number := healthFloat(value)
		if number < low {
			low, lowest = number, value
		}
		if number > high {
			high, highest = number, value
		}
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
		`" role="img" aria-label="trend over ` + strconv.Itoa(len(values)) +
		` samples, low ` + lowest.String() + `, high ` + highest.String() +
		`"><polyline points="` + points.String() +
		`" fill="none" stroke="currentColor" stroke-width="2" ` +
		`stroke-linejoin="round" stroke-linecap="round"/></svg>`
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
.th-card { border-radius: 14px; padding: 1.1rem 1.25rem; margin: 0 0 1rem; background: var(--surface, #fff); box-shadow: 0 2px 0 rgba(0,0,0,.06); border: 2px solid rgba(255,255,255,.8); }
.th-card h3 { margin: 0 0 .35rem; font-size: 1.05rem; }
.th-value { font-size: 1.5rem; font-weight: 700; margin: .2rem 0 .5rem; }
.th-status { font-size: .8rem; font-weight: 600; opacity: .7; margin-left: .5rem; }
.th-warn { border-left: 6px solid #e2a33c; }
.th-unknown { border-left: 6px solid #8b8b8b; }
.th-ok { border-left: 6px solid #57a773; }
.th-meter { display: block; height: 10px; border-radius: 6px; background: rgba(0,0,0,.08); overflow: hidden; }
.th-meter-fill { display: block; height: 100%; background: currentColor; opacity: .55; }
.th-meter-note { font-size: .85rem; opacity: .75; margin: .3rem 0 .6rem; }
.th-action { font-size: .9rem; }
.th-spark { vertical-align: middle; }
.th-attention table, .th-trends table, .th-card table { width: 100%; border-collapse: collapse; }
.th-card table { font-size: .85rem; margin-top: .6rem; }
.th-card td, .th-card th, .th-attention td, .th-attention th, .th-trends td, .th-trends th { text-align: left; padding: .35rem .5rem; border-bottom: 1px solid rgba(0,0,0,.08); }
</style>
`
