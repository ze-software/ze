// Design: docs/architecture/testing/test-health.md -- the generated page
//
// render.go writes docs/features/test-health.md.
//
// Exceptions come FIRST. Green is the absence of information: if the reader has
// to scroll past healthy rows to find the problems, the page is a trophy case.
// Everything after the attention table is the full listing, grouped by the
// three questions and ordered so `unknown` outranks `warn` outranks `ok` -- a
// number nobody is computing is worse than a number that looks bad.
package testhealth

import (
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Question is one of the three headings a metric can belong to. Key is the
// value a metric carries; Title heads the section; Prompt is the sentence under
// it.
type Question struct {
	Key    string `json:"key"`
	Title  string `json:"title"`
	Prompt string `json:"prompt"`
}

// questions are the three questions, in the order the page asks them.
var questions = [...]Question{
	{"Q1", "Sensitivity", "If the code were wrong, would something go red?"},
	{"Q2", "Intent coverage", "Are the things that matter checked, or only the happy path?"},
	{"Q3", "Integrity", "When something goes red, does it stop the line?"},
}

// Questions answers the three questions, in the order the page asks them.
//
// A second renderer of this record, the website's quality/health/ page, groups
// its metrics under the same headings. It reads them here rather than spelling
// them again, because two statements of one heading drift apart and the reader
// then meets a metric under a question it does not answer.
func Questions() []Question { return questions[:] }

// statusMark is the word the page prints beside a value.
var statusMark = map[string]string{
	statusOK: statusOK, statusWarn: "attention", statusUnknown: statusUnknown,
}

// statusOrder sorts unknown first. A dead sensor outranks a known problem,
// because a number nobody is computing is worse than a number that looks bad.
var statusOrder = map[string]int{statusUnknown: 0, statusWarn: 1, statusOK: 2}

// Statuses answers the statuses a collector can produce, worst first: unknown,
// then warn, then ok.
//
// The ORDER is the fact. A renderer that lists metrics in it puts the dead
// sensors above the known problems and the known problems above the calm rows,
// and a status this does not name sorts with the first entry rather than the
// last, so a typo cannot present as calm.
func Statuses() []string { return []string{statusUnknown, statusWarn, statusOK} }

// Series is one line of the trend table: the key a history sample stores it
// under, and the label the table prints.
type Series struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// trendSeries are the series the trend table draws, in page order.
var trendSeries = [...]Series{
	{"rfc_proof_percent", "RFC proof density %"},
	{"assert_nothing", "Assert-nothing tests"},
	{"tag_orphan", "Tag-orphaned files"},
}

// TrendSeries answers the series the trend table draws, in page order.
//
// The website draws the same series from the same history, so it reads the
// labels here: a page and its Markdown mirror that named one series two ways
// would be two names for one measurement (ai/rules/writing.md).
func TrendSeries() []Series { return trendSeries[:] }

// The sparkline's box, in the units the SVG viewBox states.
const (
	sparklineWidth  = 240
	sparklineHeight = 40
)

// sparkline renders an inline SVG polyline. No chart library, no JavaScript.
//
// It renders as a chart in any Markdown viewer that passes block-level HTML
// through, and degrades to an inert tag elsewhere. It is NOT what the website
// draws: the site build reads Render and draws its own sparkline, sized for a
// page rather than for a table cell.
func sparkline(values []pyNum) string {
	const width, height = sparklineWidth, sparklineHeight
	if len(values) < 2 {
		return ""
	}

	low, high := values[0].Float(), values[0].Float()
	for _, value := range values {
		if value.Float() < low {
			low = value.Float()
		}
		if value.Float() > high {
			high = value.Float()
		}
	}
	span := high - low
	if span == 0 {
		span = 1
	}
	step := float64(width) / float64(len(values)-1)

	var tb textbuf.Buffer
	points := textbuf.Buffer{}
	for index, value := range values {
		if index > 0 {
			points.Byte(' ')
		}
		points.Float(float64(index)*step, 1).Byte(',').
			Float(float64(height)-((value.Float()-low)/span)*float64(height-4)-2, 1)
	}

	lowest, highest := values[0], values[0]
	for _, value := range values {
		if value.Float() < lowest.Float() {
			lowest = value
		}
		if value.Float() > highest.Float() {
			highest = value
		}
	}

	return tb.Str(`<svg viewBox="0 0 `).Int(width).Byte(' ').Int(height).
		Str(`" width="`).Int(width).Str(`" height="`).Int(height).
		Str(`" role="img" aria-label="trend, `).Int(int64(len(values))).
		Str(` samples, min `).Str(lowest.String()).Str(`, max `).Str(highest.String()).
		Str(`"><polyline points="`).Str(points.String()).
		Str(`" fill="none" stroke="currentColor" stroke-width="2"/></svg>`).String()
}

// renderMarkdown answers the whole page, ending in a newline.
func renderMarkdown(metrics []Metric, history []object) string {
	var tb textbuf.Buffer
	add := func(line string) { tb.Str(line).Byte('\n') }

	add("# Testing Health")
	add("")
	add("GENERATED by `./le test-health update` -- do not edit. Source: " +
		"`internal/le/testhealth.Answer`.")
	add("")
	add("**How current is this?** The structural facts -- which test files nothing " +
		"runs, which RFCs have no test pair, and every metric's status -- are checked " +
		"by `./le verify current mode full` and cannot lag the tree. The volume counters are as of " +
		"the last `./le test-health update` and may lag by a few tests; they deliberately " +
		"do not fail the check, because a check that fired on most commits that add a " +
		"test would be routed around rather than read. Ratchets are enforced from the " +
		"tree itself, not from this page.")
	add("")
	//nolint:misspell // This generated prose deliberately uses British spelling.
	add("This page answers **is our testing correct**, not *is our testing large*. " +
		"Those are different questions. A suite can grow forever while the share of " +
		"behaviour it would actually catch a regression in falls, and no count of " +
		"tests can show that. Every metric below belongs to one of three questions; " +
		"anything belonging to none is volume and is deliberately absent.")
	add("")

	renderAttention(add, metrics)
	renderGroups(add, metrics)
	renderTrends(add, history)

	add("## How to read this")
	add("")
	add("- Every ratio shows its numerator and denominator. A percentage alone hides " +
		"the case where a score improves because the denominator shrank.")
	add("- `unknown` is not `ok`. A metric whose input is missing sorts above every " +
		"other row, because a number nobody is computing is worse than a bad number.")
	add("- Counts marked with a floor are ratchets: they may fall, never rise. " +
		"`./le verify current mode full` enforces them.")
	add("- Volume figures cover in-repo trees only. `vendor/` and `gokrazy/modcache/` " +
		"are third-party module trees; including them inflates the test count roughly sixfold.")
	add("")
	return tb.String()
}

// renderAttention writes the exceptions table, or the one line that says there
// are none.
func renderAttention(add func(string), metrics []Metric) {
	var problems, healthy []Metric
	for _, metric := range metrics {
		if metric.Status == statusOK {
			healthy = append(healthy, metric)
			continue
		}
		problems = append(problems, metric)
	}

	add("## Needs attention")
	add("")
	if len(problems) == 0 {
		add("Nothing outstanding. Every metric below is within its threshold.")
	} else {
		add("| Metric | Question | Value | What to do |")
		add("|---|---|---|---|")
		var tb textbuf.Buffer
		for _, metric := range byStatus(problems) {
			tb.Reset()
			add(tb.Str("| ").Str(metric.Label).Str(" | ").Str(metric.Question).
				Str(" | **").Str(metric.Value).Str("** (").Str(statusMark[metric.Status]).
				Str(") | ").Str(metric.Action).Str(" |").String())
		}
	}
	add("")

	if len(healthy) > 0 {
		var tb textbuf.Buffer
		add(tb.Int(int64(len(healthy))).
			Str(" further metric(s) are within threshold and are listed in full below.").String())
		add("")
	}
}

// renderGroups writes the three question sections and every metric under them.
func renderGroups(add func(string), metrics []Metric) {
	var tb textbuf.Buffer
	for _, asked := range questions {
		var group []Metric
		for _, metric := range metrics {
			if metric.Question == asked.Key {
				group = append(group, metric)
			}
		}
		if len(group) == 0 {
			continue
		}

		tb.Reset()
		add(tb.Str("## ").Str(asked.Title).String())
		add("")
		tb.Reset()
		add(tb.Byte('*').Str(asked.Prompt).Byte('*').String())
		add("")

		for _, metric := range byStatus(group) {
			tb.Reset()
			add(tb.Str("### ").Str(metric.Label).String())
			add("")
			tb.Reset()
			add(tb.Str("**").Str(metric.Value).Str("** (").Str(statusMark[metric.Status]).
				Byte(')').String())
			add("")
			if metric.Detail != "" {
				add(metric.Detail)
				add("")
			}
			if metric.Action != "" {
				tb.Reset()
				add(tb.Str("*Action if this degrades:* ").Str(metric.Action).String())
				add("")
			}
			for _, table := range detailTables(metric) {
				add(table)
				add("")
			}
		}
	}
}

// renderTrends writes the trend table, or the line that says why there is none.
func renderTrends(add func(string), history []object) {
	var tb textbuf.Buffer
	add("## Trends")
	add("")
	if len(history) < minSamples {
		add(tb.Str("Insufficient data: ").Int(int64(len(history))).Str(" recorded sample(s), ").
			Int(minSamples).Str(" needed before a trend is drawn. A line through three points " +
			"is noise with a direction. Append a sample with `./le test-health record`.").String())
		add("")
		return
	}

	add("| Series | Trend | Latest | Samples |")
	add("|---|---|---|---|")
	for _, series := range trendSeries {
		var values []pyNum
		for _, sample := range history {
			number, ok := sample.get(series.Key).(pyNum)
			if !ok {
				continue
			}
			values = append(values, number)
		}
		tb.Reset()
		if len(values) < minSamples {
			add(tb.Str("| ").Str(series.Label).Str(" | insufficient data | - | ").
				Int(int64(len(values))).Str(" |").String())
			continue
		}
		add(tb.Str("| ").Str(series.Label).Str(" | ").Str(sparkline(values)).Str(" | ").
			Str(values[len(values)-1].String()).Str(" | ").Int(int64(len(values))).
			Str(" |").String())
	}
	add("")
}

// byStatus orders a group so `unknown` comes first, keeping the collectors'
// order within one status.
func byStatus(metrics []Metric) []Metric {
	out := make([]Metric, len(metrics))
	copy(out, metrics)
	sort.SliceStable(out, func(i, j int) bool {
		return statusOrder[out[i].Status] < statusOrder[out[j].Status]
	})
	return out
}

// cell renders one Markdown table cell: stringified, with the separator
// escaped. An unescaped "|" in a file name silently broke the table.
func cell(value any) string {
	return strings.ReplaceAll(valueText(value), "|", "\\|")
}

// detailTables answers a metric's supporting tables, ordered deterministically.
func detailTables(metric Metric) []string {
	var out []string
	if table, ok := worstTable(metric); ok {
		out = append(out, table)
	}
	if table, ok := orphanTable(metric); ok {
		out = append(out, table)
	}
	if table, ok := bucketTable(metric); ok {
		out = append(out, table)
	}
	return out
}

// worstTable renders the `worst` list, whatever keys its rows carry.
//
// The header is the UNION of the rows' keys in the order they were set. Taking
// the keys from the first row alone failed on a heterogeneous row.
func worstTable(metric Metric) (string, bool) {
	rows, ok := metric.Data.get("worst").([]any)
	if !ok || len(rows) == 0 {
		return "", false
	}

	var keys []string
	var entries []object
	for _, raw := range rows {
		entry, isObject := raw.(object)
		if !isObject {
			continue
		}
		entries = append(entries, entry)
		for _, key := range entry.keys {
			if !slices.Contains(keys, key) {
				keys = append(keys, key)
			}
		}
	}
	if len(entries) == 0 {
		return "", false
	}

	var tb textbuf.Buffer
	tb.Str("| ")
	for index, key := range keys {
		if index > 0 {
			tb.Str(" | ")
		}
		tb.Str(strings.ReplaceAll(key, "_", " "))
	}
	tb.Str(" |\n|").Repeat("---|", len(keys))

	for _, entry := range entries {
		tb.Str("\n| ")
		for index, key := range keys {
			if index > 0 {
				tb.Str(" | ")
			}
			if entry.has(key) {
				tb.Str(cell(entry.get(key)))
			}
		}
		tb.Str(" |")
	}
	return tb.String(), true
}

// orphanTable renders the stranded test files and the tags they wanted.
func orphanTable(metric Metric) (string, bool) {
	rows, ok := metric.Data.get("orphans").([]any)
	if !ok || len(rows) == 0 {
		return "", false
	}

	var tb textbuf.Buffer
	tb.Str("| file | requires |\n|---|---|")
	for _, raw := range rows {
		entry, isObject := raw.(object)
		if !isObject {
			continue
		}
		tb.Str("\n| `").Str(cell(missingAsQuestion(entry, "file"))).Str("` | `").
			Str(cell(missingAsQuestion(entry, "requires"))).Str("` |")
	}
	return tb.String(), true
}

// missingAsQuestion answers a row's field, and the question mark the script
// printed when the row does not carry it.
func missingAsQuestion(entry object, key string) any {
	if !entry.has(key) {
		return "?"
	}
	return entry.get(key)
}

// bucketTable renders the package-age table of the adoption metric.
func bucketTable(metric Metric) (string, bool) {
	buckets, ok := metric.Data.get("buckets").(object)
	if !ok || len(buckets.keys) == 0 {
		return "", false
	}

	years := make([]string, len(buckets.keys))
	copy(years, buckets.keys)
	sort.Strings(years)

	var tb textbuf.Buffer
	tb.Str("| package first commit | packages with tests | with a fuzz target " +
		"| with an RFC-tagged test | with a .ci scenario |\n|---|---|---|---|---|")
	for _, year := range years {
		slot, isObject := buckets.get(year).(object)
		if !isObject {
			continue
		}
		withCI := any(0)
		if slot.has("with_ci") {
			withCI = slot.get("with_ci")
		}
		tb.Str("\n| ").Str(year).Str(" | ").Str(valueText(slot.get("packages"))).
			Str(" | ").Str(valueText(slot.get("with_fuzz"))).
			Str(" | ").Str(valueText(slot.get("with_rfc_tag"))).
			Str(" | ").Str(valueText(withCI)).Str(" |")
	}
	return tb.String(), true
}
