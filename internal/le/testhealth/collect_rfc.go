// Design: docs/architecture/testing/test-health.md -- RFC proof density
//
// collect_rfc.go answers Q2 for the RFC ledger: how many gated MUST
// requirements are proven by a test PAIR, and what the rest are.
//
// The ledger's own summary reports "0 outstanding", which is true and reads as
// 100%. It merges four different states. This splits them back apart.
package testhealth

import (
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ledgerRow is one coverage row of the RFC ledger's rollup table.
type ledgerRow struct {
	rfc   string
	gated int
	both  int
	state string
}

// annotationPattern answers the pattern that finds one annotation kind on a
// requirement line. The trailing class is what tells `{gap}` from `{gap: why}`
// and from a word that merely starts the same way.
func annotationPattern(kind string) *regexp.Regexp {
	var tb textbuf.Buffer
	return regexp.MustCompile(tb.Str(`\{`).Str(regexp.QuoteMeta(kind)).Str(`[:}]`).String())
}

// annotationPatterns is the three kinds, compiled once, in the order the first
// match wins in.
var annotationPatterns = func() []*regexp.Regexp {
	out := make([]*regexp.Regexp, 0, len(annotationKinds))
	for _, kind := range annotationKinds {
		out = append(out, annotationPattern(kind))
	}
	return out
}()

// collectRFC answers the proof-density metric and the unproven-RFC metric.
//
// Population: the ENROLLED ledger rows only, which is the set `le rfc check`
// actually gates. The un-enrolled remainder is not hidden -- the ledger states
// it in its own preamble and lists every one of those rows -- it simply is not
// part of the partition asserted below, because nothing obliges it to be proven
// or annotated yet.
func collectRFC(t *tree, floors qualityFloors) (Metric, Metric, error) {
	text, err := t.readText(rfcLedger)
	if err != nil {
		return Metric{}, Metric{}, err
	}
	if !strings.Contains(text, rfcTableHeader) {
		return Metric{}, Metric{}, collectErrorf(
			"%s has no recognizable coverage table header. The ledger format changed; "+
				"update the pinned header rather than letting this report a zero it did not measure",
			rfcLedger)
	}

	rows, err := ledgerRows(text)
	if err != nil {
		return Metric{}, Metric{}, err
	}

	// ENROLLED ROWS ONLY, on every side of the partition below. "Every gated
	// requirement is proven in both polarities or annotated" is what the RFC
	// gate enforces, and it enforces it for the ENROLLED set alone. An
	// un-enrolled summary's gated MUSTs are legitimately neither proven nor
	// annotated, because extract-then-enroll is the mandated order, so "gated
	// rows, not yet enrolled" is a REQUIRED intermediate state rather than an
	// anomaly. Summing the whole table made that state raise.
	//
	// Enrolment is read from the ledger ROW, never from rfc/enrolled.txt: the
	// row IS the population this assertion is about, and a second source is how
	// the two diverge again.
	enrolled := make([]ledgerRow, 0, len(rows))
	for _, row := range rows {
		if strings.HasPrefix(row.state, rfcStateEnrolled) {
			enrolled = append(enrolled, row)
		}
	}
	if len(enrolled) == 0 {
		return Metric{}, Metric{}, collectErrorf(
			"%s parsed %d coverage row(s), none marked %q in its State column. Either the "+
				"ledger's state marker changed or the row parse broke; an empty enrolled "+
				"population would satisfy the annotation partition vacuously, so refuse it",
			rfcLedger, len(rows), rfcStateEnrolled)
	}
	rows = enrolled

	gated, both := 0, 0
	for _, row := range rows {
		gated += row.gated
		both += row.both
	}
	if gated == 0 {
		return Metric{}, Metric{}, collectErrorf(
			"%s reports zero gated requirements across its %d gate-carrying RFC(s)",
			rfcLedger, len(rows))
	}

	kinds, err := annotationSplit(t, rows)
	if err != nil {
		return Metric{}, Metric{}, err
	}

	annotated := gated - both
	splitTotal := kinds.total()
	if splitTotal != annotated {
		return Metric{}, Metric{}, collectErrorf(
			"annotation split %s sums to %d, but the ledger's remainder is %d (%d gated - %d "+
				"proven). The page must not present a non-partition as one; the two sources have diverged",
			pythonDict(kinds), splitTotal, annotated, gated, both)
	}

	unproven := unprovenRows(rows)
	density := ratio(both, gated)

	return densityMetric(rows, unproven, kinds, density, floors, both, gated, annotated),
		unprovenMetric(rows, unproven), nil
}

// ledgerRows parses the rollup table, and refuses a table that yielded nothing.
func ledgerRows(text string) ([]ledgerRow, error) {
	var rows []ledgerRow
	for line := range strings.SplitSeq(text, "\n") {
		match := rfcRow.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil {
			continue
		}
		gated, gatedErr := strconv.Atoi(match[2])
		both, bothErr := strconv.Atoi(match[3])
		if gatedErr != nil || bothErr != nil {
			continue
		}
		rows = append(rows, ledgerRow{
			rfc: match[1], gated: gated, both: both, state: strings.TrimSpace(match[9]),
		})
	}
	if len(rows) == 0 {
		return nil, collectErrorf("%s coverage table parsed to zero rows", rfcLedger)
	}
	return rows, nil
}

// annotationSplit counts the coverage annotations of every enrolled summary.
//
// The other half of "same population": two filters, both required. A summary
// counts only when it owns an ENROLLED ledger row, and a line counts only when
// its requirement level is gated. Drop either filter and the split stops being
// a partition of `gated - both`.
func annotationSplit(t *tree, rows []ledgerRow) (annotationCounts, error) {
	known := make(map[string]bool, len(rows))
	for _, row := range rows {
		known[row.rfc] = true
	}

	listed, err := t.trackedMatching(rfcSummariesTree, ".md")
	if err != nil {
		return annotationCounts{}, err
	}

	var summaries []string
	for _, rel := range listed {
		if path.Dir(rel) != rfcSummaries {
			continue
		}
		if known[strings.TrimSuffix(path.Base(rel), ".md")] {
			summaries = append(summaries, rel)
		}
	}
	if len(summaries) == 0 {
		return annotationCounts{}, collectErrorf(
			"no tracked summaries under %s match an enrolled ledger row: refusing to report "+
				"the annotation split as all-zero when nothing was measured", rfcSummaries)
	}

	kinds := annotationCounts{counts: map[string]int{}}
	for _, rel := range summaries {
		body, readErr := t.readBody(rel)
		if readErr != nil {
			return annotationCounts{}, readErr
		}
		for line := range strings.SplitSeq(body, "\n") {
			level := rfcLevel.FindStringSubmatch(line)
			if len(level) == 0 || !gatedLevels[level[1]] {
				continue
			}
			if kind, annotated := annotationOf(line); annotated {
				kinds.add(kind)
			}
		}
	}
	return kinds, nil
}

// annotationCounts is the split, keeping the order its kinds were FIRST
// counted in.
//
// The order is not decoration: it is what the divergence message prints, and
// the two halves must diagnose one divergence with one set of words while they
// run side by side. The script's counter is a defaultdict, whose keys arrive in
// that same order.
type annotationCounts struct {
	order  []string
	counts map[string]int
}

// add counts one line under its kind.
func (a *annotationCounts) add(kind string) {
	if _, seen := a.counts[kind]; !seen {
		a.order = append(a.order, kind)
	}
	a.counts[kind]++
}

// get answers one kind's count, and zero for a kind nothing carried.
func (a annotationCounts) get(kind string) int { return a.counts[kind] }

// total answers the split's sum, which the partition check compares against the
// ledger's remainder.
func (a annotationCounts) total() int {
	sum := 0
	for _, count := range a.counts {
		sum += count
	}
	return sum
}

// annotationOf answers the coverage annotation a requirement line carries.
//
// A line carries at most ONE, and the first kind in table order wins. The
// RFC gate refuses a second coverage annotation on one line, so the rule is
// rarely exercised -- and it is the rule that keeps the split a PARTITION of
// the ledger's remainder rather than a count that can exceed it.
func annotationOf(line string) (string, bool) {
	for index, pattern := range annotationPatterns {
		if pattern.MatchString(line) {
			return annotationKinds[index], true
		}
	}
	return "", false
}

// unprovenRows answers the enrolled RFCs that gate something and prove none of
// it, worst first. The sort is stable, so ties keep the ledger's own order.
func unprovenRows(rows []ledgerRow) []ledgerRow {
	var unproven []ledgerRow
	for _, row := range rows {
		if row.gated > 0 && row.both == 0 {
			unproven = append(unproven, row)
		}
	}
	sort.SliceStable(unproven, func(i, j int) bool { return unproven[i].gated > unproven[j].gated })
	return unproven
}

// densityMetric renders the headline proof-density row.
func densityMetric(rows, unproven []ledgerRow, kinds annotationCounts, density object,
	floors qualityFloors, both, gated, annotated int,
) Metric {
	var tb textbuf.Buffer
	detail := tb.Str(valueText(percentOf(density))).
		Str("% carry both polarities. Of the remaining ").Int(int64(annotated)).Str(": ").
		Int(int64(kinds.get("not-applicable"))).
		Str(" not-applicable (ze deliberately does not do it, so no test is owed), ").
		Int(int64(kinds.get("gap"))).
		Str(" known gap (unimplemented, genuinely untested), and ").
		Int(int64(kinds.get("single-polarity"))).
		Str(" single-polarity -- those DO have a passing tagged test, just one side of the " +
			"pair, and the RFC gate fails if that test is missing. Only the gap column is " +
			"untested work.").String()

	tb.Reset()
	value := tb.Int(int64(both)).Str(" / ").Int(int64(gated)).String()

	worst := make([]any, 0, 10)
	for index, row := range unproven {
		if index >= 10 {
			break
		}
		entry := object{}
		entry.set("rfc", row.rfc)
		entry.set("gated", row.gated)
		worst = append(worst, entry)
	}

	annotations := object{}
	for _, kind := range annotationKinds {
		annotations.set(kind, kinds.get(kind))
	}

	data := object{}
	data.set("proof_density", density)
	data.set("annotations", annotations)
	data.set("rfcs_total", len(rows))
	data.set("rfcs_without_any_proof", len(unproven))
	data.set("worst", worst)

	return Metric{
		Key:      keyProofDensity,
		Question: "Q2",
		Label:    "RFC MUST requirements proven by a positive+negative test pair",
		Status:   floors.status(keyProofDensity, percentOf(density)),
		Value:    value,
		Detail:   detail,
		Action: "Convert a {gap} or {single-polarity} annotation into a test pair. " +
			"Not-applicable needs no test.",
		Data: data,
	}
}

// unprovenMetric renders the row that NAMES every enrolled RFC with no pair.
func unprovenMetric(rows, unproven []ledgerRow) Metric {
	status := statusOK
	if len(unproven) > 0 {
		status = statusWarn
	}

	// The WHOLE set, named, and sorted by name rather than by rank.
	// StructuralFacts gates this list, so two properties are load-bearing.
	// Complete: the density metric's `worst` is a display slice of ten, so an
	// eleventh RFC earning its first pair would have been an undetectable
	// event. Rank-free: ordering by gated count would rewrite this list
	// whenever extraction moves a count, turning pure churn into a diff.
	names := make([]any, 0, len(unproven))
	sorted := make([]string, 0, len(unproven))
	for _, row := range unproven {
		sorted = append(sorted, row.rfc)
	}
	sort.Strings(sorted)
	for _, name := range sorted {
		names = append(names, name)
	}

	data := object{}
	data.set("unproven", ratio(len(unproven), len(rows)))
	data.set("unproven_rfcs", names)

	var tb textbuf.Buffer
	return Metric{
		Key:      keyUnproven,
		Question: "Q2",
		Label:    "Enrolled RFCs with zero test-proven requirements",
		Status:   status,
		Value:    tb.Int(int64(len(unproven))).Str(" / ").Int(int64(len(rows))).String(),
		Detail: "Enrolled and gate-green, but no requirement is proven by BOTH polarities. " +
			"Some of these do carry positive-only tests; none carries a pair.",
		Action: "Pick the largest and complete a pair, or accept it is a single-polarity claim.",
		Data:   data,
	}
}

// pythonDict renders a counter the way Python renders a dict in a message, so
// the two halves diagnose a divergence with the same words.
func pythonDict(kinds annotationCounts) string {
	var tb textbuf.Buffer
	tb.Byte('{')
	for index, kind := range kinds.order {
		if index > 0 {
			tb.Str(", ")
		}
		tb.Byte('\'').Str(kind).Str("': ").Int(int64(kinds.get(kind)))
	}
	return tb.Byte('}').String()
}
