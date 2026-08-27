// Design: docs/architecture/testing/test-health.md -- what the check gates
//
// facts.go holds the claims worth gating: the ones whose change is an EVENT,
// not churn.
//
// A byte-exact gate over the whole page charged a regeneration-and-commit to
// ~60% of commits, because every added test moves a denominator. That is the
// "advisory gate permanently red" failure the page itself is built to expose: a
// check that fires constantly for cosmetic reasons trains people to regenerate
// without reading.
//
// What is NOT here, deliberately: every volume counter. A stale test count is
// cosmetic. What IS here changes only when something happened.
//
// EVERY fact here FAILS CLOSED, and that is deliberate rather than incidental.
// A fact read with a `.get(...) or []` shape can be permanently empty on BOTH
// sides of the comparison and gate nothing while reading as the goal state,
// which is precisely what one of these facts did before it was fixed. A
// snapshot comparison cannot catch a reader bug, because both sides degenerate
// identically. So each fact is checked against a source independent of itself
// before it is compared at all.
package testhealth

import (
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Facts is what one side of the comparison holds.
type Facts struct {
	// Statuses is every metric's status, keyed by metric. A flip to `warn`, and
	// above all to `unknown`, means a collector stopped measuring. Sensor rot
	// is the failure mode the page exists to make visible, so it must not be
	// able to land silently.
	Statuses map[string]string `json:"statuses"`
	// TagOrphans names the test files no `go test` target can build. A new one
	// means a build tag or a make target just stranded a file.
	TagOrphans [][2]string `json:"tag-orphans"`
	// Unproven names the enrolled RFCs with no test pair. One leaving the list
	// is a requirement newly proven.
	Unproven []string `json:"rfc-unproven"`
}

// Equal reports whether two sides hold the same facts.
func (f Facts) Equal(other Facts) bool {
	if len(f.Statuses) != len(other.Statuses) {
		return false
	}
	for key, status := range f.Statuses {
		if other.Statuses[key] != status {
			return false
		}
	}
	if len(f.TagOrphans) != len(other.TagOrphans) || len(f.Unproven) != len(other.Unproven) {
		return false
	}
	for index := range f.TagOrphans {
		if f.TagOrphans[index] != other.TagOrphans[index] {
			return false
		}
	}
	for index := range f.Unproven {
		if f.Unproven[index] != other.Unproven[index] {
			return false
		}
	}
	return true
}

// StructuralFacts reads the three gated facts off one record.
//
// The order is fixed so the shared precondition -- a non-empty metric list --
// is checked before any per-fact guard, and so each fact's failure is
// attributable to its own guard rather than to whichever sibling ran first.
func StructuralFacts(metrics []object) (Facts, error) {
	statuses, err := readStatuses(metrics)
	if err != nil {
		return Facts{}, err
	}
	byKey := make(map[string]object, len(metrics))
	for _, metric := range metrics {
		if key, ok := metric.get("key").(string); ok {
			byKey[key] = metric
		}
	}
	orphans, err := readTagOrphans(byKey)
	if err != nil {
		return Facts{}, err
	}
	unproven, err := readUnprovenRFCs(byKey)
	if err != nil {
		return Facts{}, err
	}
	return Facts{Statuses: statuses, TagOrphans: orphans, Unproven: unproven}, nil
}

// readStatuses answers every metric's status, keyed by metric.
//
// This fact has NO counter to cross-check against, and counting the map against
// itself would be a tautology. Two sources independent of its content are
// checked instead: the status VOCABULARY, because a reader on the wrong field
// yields a null for every metric and a null-for-everything compares equal on
// both sides of the gate for any status change whatsoever; and the CARDINALITY
// of the metrics list, because a reader on the wrong KEY field collapses every
// entry under one key and the map silently keeps only the last.
func readStatuses(metrics []object) (map[string]string, error) {
	if len(metrics) == 0 {
		return nil, collectErrorf(
			"the record has no metrics, so there are no statuses to gate. An empty record " +
				"makes every structural fact vacuously healthy, which is the one reading this " +
				"gate must never produce")
	}

	order := make([]string, 0, len(metrics))
	keyed := make(map[string]any, len(metrics))
	statuses := make(map[string]any, len(metrics))
	for _, metric := range metrics {
		canonical := canonicalKey(metric.get("key"))
		if _, seen := keyed[canonical]; !seen {
			order = append(order, canonical)
		}
		keyed[canonical] = metric.get("key")
		statuses[canonical] = metric.get("status")
	}
	if len(statuses) != len(metrics) {
		return nil, collectErrorf(
			"`statuses` has %d entry(ies) for %d metric(s): two metrics share a key, so one "+
				"was silently dropped and its status is no longer gated. Keys seen: %s",
			len(statuses), len(metrics), keyList(order, keyed))
	}

	out := make(map[string]string, len(statuses))
	for _, canonical := range sortedByText(order, keyed) {
		key, isString := keyed[canonical].(string)
		if !isString || key == "" {
			return nil, collectErrorf(
				"`statuses` has a metric with no usable `key` (%s), so its status is gated "+
					"under a name nothing can be traced to", valueText(keyed[canonical]))
		}
		status, isStatus := statuses[canonical].(string)
		if !isStatus || !statusValues[status] {
			return nil, collectErrorf(
				"`statuses` gives metric %s the status %s, which no collector produces "+
					"(expected one of [ok, unknown, warn]). The field is missing or renamed; "+
					"a constant status gates nothing", quote(key), valueText(statuses[canonical]))
		}
		out[key] = status
	}
	return out, nil
}

// readTagOrphans answers the test files no `go test` target can build,
// cross-checked against the same metric's own count.
//
// The collector writes the count as the length of the list, so the two can only
// disagree if one of them is not what it claims to be. Without the check, a
// misspelled metric key, a renamed field, or a wrong type turns into "nothing
// is stranded" -- the goal state -- identically on both sides of the gate.
func readTagOrphans(byKey map[string]object) ([][2]string, error) {
	listed, err := gatedList(byKey, keyTagOrphan, "orphans",
		func(metric object) any { return metric.get("orphan_count") },
		"orphan_count", "the test files no `go test` target can build")
	if err != nil {
		return nil, err
	}

	out := make([][2]string, 0, len(listed))
	for _, raw := range listed {
		entry, isObject := raw.(object)
		if !isObject {
			continue
		}
		out = append(out, [2]string{
			valueText(orDefault(entry, "file", "")),
			valueText(orDefault(entry, "requires", "")),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] != out[j][0] {
			return out[i][0] < out[j][0]
		}
		return out[i][1] < out[j][1]
	})
	return out, nil
}

// readUnprovenRFCs answers the enrolled RFCs with no proven pair, read from the
// metric that IS that claim and cross-checked against that metric's own count.
func readUnprovenRFCs(byKey map[string]object) ([]string, error) {
	listed, err := gatedList(byKey, keyUnproven, "unproven_rfcs",
		func(metric object) any {
			part, ok := metric.get("unproven").(object)
			if !ok {
				return nil
			}
			return part.get("numerator")
		},
		"unproven.numerator", "the enrolled RFCs with no test pair")
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(listed))
	for _, raw := range listed {
		out = append(out, valueText(raw))
	}
	sort.Strings(out)
	return out, nil
}

// gatedList reads one gated list off its metric and cross-checks it against
// that metric's own counter. Every branch refuses rather than answering an
// empty list.
//
// An empty list is the one answer a gated fact must never be able to INVENT,
// because for every fact here it is the goal state -- no unproven RFC, no
// stranded test file -- so it is indistinguishable from success, and it is what
// the shipped defect produced.
//
// The length cross-check is what closes it for good. Each list and its counter
// are computed from the SAME data by the same collector, so a list disagreeing
// with its count cannot be measurement: it is a stale snapshot, a truncation,
// or a reader on the wrong field. An empty list stays legal, but ONLY when the
// counter agrees, which makes the vacuous state unreachable rather than merely
// unlikely.
//
// Note what this deliberately does NOT do: compare the list's length to itself.
// That tautology would read as protection while providing none.
func gatedList(byKey map[string]object, metricKey, listField string,
	countOf func(object) any, countPath, means string,
) ([]any, error) {
	metric, held := byKey[metricKey]
	if !held {
		return nil, collectErrorf(
			"no `%s` metric in the record, so the gated fact naming %s has no source. "+
				"Refusing to report it as empty: that reads as the goal state", metricKey, means)
	}
	if !metric.has(listField) || metric.get(listField) == nil {
		return nil, collectErrorf(
			"the `%s` metric carries no `%s` field. A snapshot written before the field "+
				"existed has no answer here, which is not the same as zero. Run "+
				"`make ze-test-health-update` and commit %s", metricKey, listField, Latest)
	}
	listed, isList := metric.get(listField).([]any)
	if !isList {
		return nil, collectErrorf(
			"`%s.%s` is not a list: got %s", metricKey, listField, valueText(metric.get(listField)))
	}

	counted := countOf(metric)
	number, isInteger := asInteger(counted)
	if !isInteger {
		return nil, collectErrorf(
			"`%s.%s` is not an integer, so the gated list cannot be checked against its own "+
				"count: got %s", metricKey, countPath, valueText(counted))
	}
	if int64(len(listed)) != number {
		return nil, collectErrorf(
			"`%s` lists %d entry(ies) in `%s` but counts %d in `%s`. Both come from the same "+
				"data, so they have diverged: the snapshot is stale, or the list is truncated. "+
				"Run `make ze-test-health-update` and commit %s",
			metricKey, len(listed), listField, number, countPath, Latest)
	}
	return listed, nil
}

// asInteger answers a counter's whole-number value, and refuses everything
// else.
//
// Two spellings reach here and both are integers: the freshly measured record
// holds a Go int, and the committed snapshot holds a number decoded from JSON.
// A BOOLEAN is refused explicitly, because a boolean is an integer in Python
// and a one-element list would then satisfy the comparison against `true`
// while proving nothing. A float is refused too: a count that arrived with a
// decimal point was not written by the collector that owns it.
func asInteger(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case pyNum:
		return typed.i, typed.isInt
	default:
		return 0, false
	}
}

// orDefault answers a row's field, or the fallback the script used when the row
// does not carry it.
func orDefault(entry object, key string, fallback any) any {
	if !entry.has(key) {
		return fallback
	}
	return entry.get(key)
}

// canonicalKey renders a metric key so two metrics collide here exactly when
// they would collide in the record's own object.
//
// The type is carried, so a metric with no key at all and a metric keyed by the
// literal string "None" stay two entries rather than one.
func canonicalKey(value any) string {
	var tb textbuf.Buffer
	switch typed := value.(type) {
	case nil:
		return "none"
	case string:
		return tb.Str("str\x00").Str(typed).String()
	default:
		return tb.Str("other\x00").Str(valueText(value)).String()
	}
}

// keyList renders the keys a duplicate-key failure saw, sorted the way the
// script sorted them: by their rendered text.
func keyList(order []string, keyed map[string]any) string {
	var tb textbuf.Buffer
	tb.Byte('[')
	for index, canonical := range sortedByText(order, keyed) {
		if index > 0 {
			tb.Str(", ")
		}
		tb.Str(quote(valueText(keyed[canonical])))
	}
	return tb.Byte(']').String()
}

// sortedByText orders canonical keys by the text their original key renders as,
// which is the order the script's own guard walked them in.
func sortedByText(order []string, keyed map[string]any) []string {
	out := make([]string, len(order))
	copy(out, order)
	sort.SliceStable(out, func(i, j int) bool {
		return valueText(keyed[out[i]]) < valueText(keyed[out[j]])
	})
	return out
}

// Change is one gated fact that moved, named so the failure is diagnosable
// without a diff tool.
type Change struct {
	Fact string `json:"fact"`
	// Gone and New are the membership difference of a list-valued fact. The
	// unproven-RFC fact names dozens of RFCs, so printing both sides leaves the
	// reader to find the one that moved by eye -- which is the diff tool this
	// exists to replace.
	Gone []string `json:"gone,omitempty"`
	New  []string `json:"new,omitempty"`
	// Committed and Generated are both sides of a fact that is not a list. The
	// status map is small and keyed, so both sides stay readable.
	Committed []string `json:"committed,omitempty"`
	Generated []string `json:"generated,omitempty"`
}

// Describe names what moved between the committed record and the tree.
func Describe(committed, generated Facts) []Change {
	var out []Change
	if change, moved := describeStatuses(committed.Statuses, generated.Statuses); moved {
		out = append(out, change)
	}
	if change, moved := describeList("tag-orphans",
		pairText(committed.TagOrphans), pairText(generated.TagOrphans)); moved {
		out = append(out, change)
	}
	if change, moved := describeList("rfc-unproven", committed.Unproven, generated.Unproven); moved {
		out = append(out, change)
	}
	return out
}

// describeStatuses reports both sides of the status map, which is small enough
// to read whole.
func describeStatuses(committed, generated map[string]string) (Change, bool) {
	left, right := statusText(committed), statusText(generated)
	if strings.Join(left, "\n") == strings.Join(right, "\n") {
		return Change{}, false
	}
	return Change{Fact: "statuses", Committed: left, Generated: right}, true
}

// describeList reports the membership difference of a list-valued fact.
func describeList(name string, committed, generated []string) (Change, bool) {
	gone := missingFrom(committed, generated)
	fresh := missingFrom(generated, committed)
	if len(gone) == 0 && len(fresh) == 0 {
		return Change{}, false
	}
	return Change{Fact: name, Gone: gone, New: fresh}, true
}

// missingFrom answers the entries of left that right does not hold.
func missingFrom(left, right []string) []string {
	held := make(map[string]bool, len(right))
	for _, value := range right {
		held[value] = true
	}
	var out []string
	for _, value := range left {
		if !held[value] {
			out = append(out, value)
		}
	}
	return out
}

// pairText renders the orphan pairs so the difference reads as one line each.
func pairText(pairs [][2]string) []string {
	out := make([]string, 0, len(pairs))
	var tb textbuf.Buffer
	for _, pair := range pairs {
		tb.Reset()
		out = append(out, tb.Str(pair[0]).Str(" requires ").Str(pair[1]).String())
	}
	return out
}

// statusText renders the status map as sorted `key: status` lines.
func statusText(statuses map[string]string) []string {
	keys := make([]string, 0, len(statuses))
	for key := range statuses {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]string, 0, len(keys))
	var tb textbuf.Buffer
	for _, key := range keys {
		tb.Reset()
		out = append(out, tb.Str(key).Str(": ").Str(statuses[key]).String())
	}
	return out
}
