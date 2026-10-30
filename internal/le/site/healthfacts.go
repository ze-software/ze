// Design: docs/architecture/site-facts.md -- the health section carries every figure the testing-health page prints
// Related: health.go -- the testing-health page, which reads every figure back from these keys
// Related: facts.go -- the snapshot this section is part of
package site

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	testhealth "github.com/ze-software/ze/internal/le/test/health"
)

// The two snapshot sections the testing-health page reads its figures from.
// healthSection holds one entry for each metric, keyed by the metric's own key;
// healthTrendsSection holds one entry for each trend series the generator
// states. They are separate sections so a metric the collectors gain can never
// be keyed like a series.
const (
	healthSection       = "health"
	healthTrendsSection = "health_trends"
)

// healthFigureKey answers the dotted snapshot key of one figure the page prints.
func healthFigureKey(section string, segments ...string) string {
	return section + "." + strings.Join(segments, ".")
}

// healthComposedFields are the metric fields the collector composes from its
// figures: the value ("111 / 39886 (floor 111)"), and the detail and action
// prose ("3284 Go test files, 78 fuzz targets"). Each figure in them is carried
// as its own key and the page composes the sentence from marked parts around
// the record's own words. The label is a name and states no figure.
var healthComposedFields = []string{"value", "detail", "action"}

// healthFigure matches one figure inside a composed field: a run of digits
// with an optional fraction. A minus sign, a percent sign or a unit stays in
// the words around it. A run that continues a word ("rfc7084", "Q2", "v1.2")
// is part of a name, not a figure, and healthTextParts leaves it in the words.
var healthFigure = regexp.MustCompile(`\d+(?:\.\d+)?`)

// healthTextPart is one run of a composed field: the record's own words, or
// the figure-th figure of the field, counted from one.
type healthTextPart struct {
	text   string
	figure int
}

// healthTextParts splits a composed field into its words and its figures, in
// the order the field states them.
func healthTextParts(text string) []healthTextPart {
	var parts []healthTextPart
	last, figures := 0, 0
	for _, span := range healthFigure.FindAllStringIndex(text, -1) {
		if span[0] > 0 && healthContinuesWord(text[span[0]-1]) {
			continue
		}
		if span[0] > last {
			parts = append(parts, healthTextPart{text: text[last:span[0]]})
		}
		figures++
		parts = append(parts, healthTextPart{text: text[span[0]:span[1]], figure: figures})
		last = span[1]
	}
	if last < len(text) {
		parts = append(parts, healthTextPart{text: text[last:]})
	}
	return parts
}

// healthContinuesWord answers whether a digit run after this byte continues a
// word or a number rather than starting a figure.
func healthContinuesWord(previous byte) bool {
	if previous == '_' {
		return true
	}
	if previous == '.' {
		return true
	}
	if previous >= '0' && previous <= '9' {
		return true
	}
	return (previous|0x20) >= 'a' && (previous|0x20) <= 'z'
}

// healthRatio answers the payload object a metric's meter is drawn from: its
// name, the object, and its percentage as the page prints it.
//
// It is the first payload object, in payloadKeys order, carrying a percent the
// page can read. The page and the snapshot both ask here, so the meter the
// page draws is the one whose figures the snapshot carries.
func healthRatio(metric healthMetric) (string, map[string]any, string, bool) {
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
		return key, part, strconv.FormatFloat(percent, 'f', 1, 64), true
	}
	return "", nil, "", false
}

// healthCellFigure answers a table cell as a figure, when it is one.
//
// A figure is a number the record states. A word, a path or an identifier
// ("rfc7084", a test file) is the row's name, and the page prints it as text.
func healthCellFigure(value any) (string, bool) {
	number, isNumber := value.(json.Number)
	if !isNumber {
		return "", false
	}
	return number.String(), true
}

// healthTableRows answers the object rows of one of a metric's row lists, the
// worst rows or the stranded files, in the record's order. A row that is not
// an object is not printed, so it is not counted either: row numbers in the
// snapshot are the page's own row positions, counted from one.
func healthTableRows(metric healthMetric, list string) []map[string]any {
	rows, isList := metric[list].([]any)
	// refuse-by-default: guards healthMetricFigures -- a metric that states no such list has no rows, so neither the page nor the snapshot has a figure from it
	if !isList {
		return nil
	}
	var entries []map[string]any
	for _, raw := range rows {
		entry, isObject := raw.(map[string]any)
		if !isObject {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// healthSeriesValues answers the samples one trend series holds, in the
// history's order. A sample that is not a number is not a sample.
func healthSeriesValues(history []map[string]any, series string) []json.Number {
	var values []json.Number
	for _, sample := range history {
		number, isNumber := sample[series].(json.Number)
		if !isNumber {
			continue
		}
		values = append(values, number)
	}
	return values
}

// healthFacts derives the two health sections of the snapshot from the record
// the testing-health page renders.
//
// Every figure the page prints is carried, as the string the page prints it:
// each figure of a metric's value, detail and action, its meter's numerator, denominator and
// percentage, each number in its worst rows, stranded files and package-age
// buckets, and each trend series' latest sample and sample count. Strings,
// because the record spells a score as 0.0 and a number would publish it as 0.
// Nothing else of the record is carried: labels, prose and the rows' names
// are the page's words, not figures (R-9).
//
// The metric is keyed by its own key, the columns by the record's own names,
// so a metric or a column the collectors gain reaches the snapshot with no edit
// here. A name no snapshot key can carry is refused by name (factKeySegment).
func healthFacts(record healthRecord) (map[string]any, map[string]any, error) {
	section := map[string]any{}
	for _, metric := range record.Metrics {
		name, err := factKeySegment(metric.text("key"), "test-health metric "+strconv.Quote(metric.text("key")))
		if err != nil {
			return nil, nil, err
		}
		if _, taken := section[name]; taken {
			return nil, nil, fmt.Errorf("two test-health metrics are keyed %q, so the snapshot cannot tell their figures apart", name)
		}
		figures, err := healthMetricFigures(name, metric)
		if err != nil {
			return nil, nil, err
		}
		section[name] = figures
	}

	trends := map[string]any{}
	for _, series := range testhealth.TrendSeries() {
		name, err := factKeySegment(series.Key, "test-health trend series "+strconv.Quote(series.Key))
		if err != nil {
			return nil, nil, err
		}
		values := healthSeriesValues(record.History, series.Key)
		entry := map[string]any{"samples": strconv.Itoa(len(values))}
		if len(values) >= healthMinSamples {
			entry["latest"] = values[len(values)-1].String()
		}
		trends[name] = entry
	}
	return section, trends, nil
}

// healthMetricFigures answers the figures of one metric, nested as the dotted
// keys under health.<name> name them.
func healthMetricFigures(name string, metric healthMetric) (map[string]any, error) {
	figures := healthFigureTree{metric: name, root: map[string]any{}}
	for _, field := range healthComposedFields {
		for _, part := range healthTextParts(metric.text(field)) {
			if part.figure == 0 {
				continue
			}
			if err := figures.set(part.text, field, strconv.Itoa(part.figure)); err != nil {
				return nil, err
			}
		}
	}
	if ratioName, ratio, percent, drawn := healthRatio(metric); drawn {
		for _, field := range []string{"numerator", "denominator"} {
			text := healthValueText(ratio[field])
			if text == "" {
				return nil, fmt.Errorf("test-health %s draws its %s meter with no %s, so the page cannot state it",
					name, ratioName, field)
			}
			if err := figures.set(text, ratioName, field); err != nil {
				return nil, err
			}
		}
		if err := figures.set(percent, ratioName, "percent"); err != nil {
			return nil, err
		}
	}
	for _, list := range []string{"worst", "orphans"} {
		for row, entry := range healthTableRows(metric, list) {
			for column, value := range entry {
				text, isFigure := healthCellFigure(value)
				if !isFigure {
					continue
				}
				if err := figures.set(text, list, strconv.Itoa(row+1), column); err != nil {
					return nil, err
				}
			}
		}
	}
	buckets, hasBuckets := metric["buckets"].(map[string]any)
	if hasBuckets {
		if err := healthBucketFigures(figures, buckets); err != nil {
			return nil, err
		}
	}
	return figures.root, nil
}

// healthBucketFigures adds the numbers of a metric's package-age buckets, keyed
// by the bucket's name and the record's own field names.
func healthBucketFigures(figures healthFigureTree, buckets map[string]any) error {
	for bucket, raw := range buckets {
		slot, isObject := raw.(map[string]any)
		if !isObject {
			continue
		}
		for field, value := range slot {
			text, isFigure := healthCellFigure(value)
			if !isFigure {
				continue
			}
			if err := figures.set(text, "buckets", bucket, field); err != nil {
				return err
			}
		}
	}
	return nil
}

// healthFigureTree builds the nested objects of one metric's figures.
//
// Not safe for concurrent use.
type healthFigureTree struct {
	metric string
	root   map[string]any
}

// set places one figure at the dotted path the segments name, refusing a
// segment no snapshot key can carry and a path another figure already holds.
func (tree healthFigureTree) set(text string, segments ...string) error {
	key := healthFigureKey(healthSection, append([]string{tree.metric}, segments...)...)
	object := tree.root
	for position, segment := range segments {
		if _, err := factKeySegment(segment, "test-health "+tree.metric+" field "+strconv.Quote(segment)); err != nil {
			return err
		}
		if position == len(segments)-1 {
			if _, taken := object[segment]; taken {
				return fmt.Errorf("test-health %s states %s twice, so the snapshot cannot carry both", tree.metric, key)
			}
			object[segment] = text
			return nil
		}
		next, present := object[segment]
		if !present {
			child := map[string]any{}
			object[segment] = child
			object = child
			continue
		}
		child, isObject := next.(map[string]any)
		if !isObject {
			return fmt.Errorf("test-health %s states %s beneath a figure, so the snapshot cannot carry both", tree.metric, key)
		}
		object = child
	}
	return fmt.Errorf("BUG: test-health %s figure with no key", tree.metric)
}
