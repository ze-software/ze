// Design: docs/architecture/testing/test-health.md -- RFC proof density
//
// collect_rfc.go answers Q2 for the RFC ledger: how much of what Ze implements
// is proven by test, and what the rest of the gated population is.
//
// The share itself is rfc.ProvenShareOf, read rather than computed here, so
// this page, the site home page and /quality/rfc-compliance/ state one number.
// What this file derives is the SPLIT beside it: the ledger's own summary
// reports "0 outstanding", which is true and reads as 100%, and it merges four
// different states. This splits them back apart.
package testhealth

import (
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/rfc"
)

// ledgerRow is one coverage row of the RFC ledger's rollup table.
//
// annotated and noTest are READ from the ledger's own columns rather than
// derived by subtracting both from gated. The two are not the same number
// whenever the gate is red: a gated MUST with no test and no annotation is in
// the remainder and in neither column, so `gated - both` counts it as an
// annotation that does not exist (ai/rules/principles.md, declare once).
type ledgerRow struct {
	rfc       string
	gated     int
	both      int
	annotated int
	noTest    int
	state     string
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
// Two populations, and the metric names both. The published SHARE comes from
// rfc.ProvenShareOf and is taken over the RFCs Ze implements. The annotation
// split below is taken over the ENROLLED ledger rows, which is the set `le rfc
// check` actually gates. The un-enrolled remainder is not hidden -- the ledger
// states it in its own preamble and lists every one of those rows -- it simply
// is not part of the partition asserted below, because nothing obliges it to be
// proven or annotated yet.
//
// The share was `both / gated`, summed from the rendered rollup's own columns,
// until 2026-09-02. A number parsed back out of a generated artifact is a
// second declaration of a fact (ai/rules/principles.md), and that one answered
// 43.2% where /quality/rfc-compliance/ answered 58.1% for the same question.
// Only the share moved: the rollup parse still feeds everything else here,
// because the partition this page asserts is a property of the ledger's own
// columns.
func collectRFC(t *tree, floors qualityFloors) (Metric, Metric, error) {
	share, err := provenShare(t)
	if err != nil {
		return Metric{}, Metric{}, err
	}
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

	gated, both, annotated, noTest := 0, 0, 0, 0
	for _, row := range rows {
		gated += row.gated
		both += row.both
		annotated += row.annotated
		noTest += row.noTest
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

	// The cross-check is the ledger's Annotated COLUMN against the live count
	// over rfc/short. Two derivations of one population, from two files, which
	// is a real disagreement to arbitrate. Comparing against `gated - both`
	// instead compared the split against a different population -- the
	// remainder holds every gated MUST with no test as well -- so a red gate
	// made this refuse and took the whole page down with it.
	splitTotal := kinds.total()
	if splitTotal != annotated {
		return Metric{}, Metric{}, collectErrorf(
			"annotation split %s sums to %d, but the ledger's Annotated column sums to %d "+
				"across %d gated requirement(s). The page must not present a non-partition as "+
				"one; the two sources have diverged",
			pythonDict(kinds), splitTotal, annotated, gated)
	}
	if annotated+noTest+both != gated {
		return Metric{}, Metric{}, collectErrorf(
			"the ledger's own columns do not partition its gated population: %d both + %d "+
				"annotated + %d with no test is not %d gated. One polarity is counted "+
				"nowhere here, so the page would publish a remainder it cannot account for",
			both, annotated, noTest, gated)
	}

	unproven := unprovenRows(rows)
	density := ratio(share.Proven, share.Gated)

	return densityMetric(rows, unproven, kinds, density, floors, share,
			rfcTotals{both: both, gated: gated, annotated: annotated, noTest: noTest}),
		unprovenMetric(rows, unproven), nil
}

// provenShare answers the published proof share over this checkout.
//
// The carriers argument is nil because no field of a ProvenShare reads one:
// rfc.CoverageRows takes them to decide NightlyOnly, which the share does not
// count. The site's home page and its RFC compliance report pass nil for the
// same reason, so the three surfaces cannot answer different numbers.
func provenShare(t *tree) (rfc.ProvenShare, error) {
	collected, err := rfc.Collect(t.root)
	if err != nil {
		return rfc.ProvenShare{}, err
	}
	return rfc.ProvenShareOf(collected.Metas, collected.Requirements, collected.Tags, nil)
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
		annotated, annotatedErr := strconv.Atoi(match[5])
		noTest, noTestErr := strconv.Atoi(match[6])
		if annotatedErr != nil || noTestErr != nil {
			continue
		}
		rows = append(rows, ledgerRow{
			rfc: match[1], gated: gated, both: both, annotated: annotated,
			noTest: noTest, state: strings.TrimSpace(match[9]),
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
			if len(level) == 0 || !rfc.IsGatedLevel(level[1]) {
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

// rfcTotals is the enrolled population, summed from the ledger's own columns.
//
// noTest is carried rather than left out of the sentence: a gated MUST that
// nothing tests and nothing excuses is the weakest state on this page, and a
// detail line that named only the annotations would present the remainder as
// though every row in it had a reason.
type rfcTotals struct {
	both      int
	gated     int
	annotated int
	noTest    int
}

// densityMetric renders the headline proof-density row.
//
// The percentage is the producer's own rendering rather than one this file
// formats, so the number here and the number the site publishes are the same
// string. The detail names the population under every count it states: the
// share is over the RFCs Ze implements, and the annotation split beside it is
// over every enrolled RFC, which is the wider set the gate holds.
func densityMetric(rows, unproven []ledgerRow, kinds annotationCounts, density object,
	floors qualityFloors, share rfc.ProvenShare, totals rfcTotals,
) Metric {
	both, gated, noTest := totals.both, totals.gated, totals.noTest
	var tb textbuf.Buffer
	detail := tb.Str(share.Percent()).
		Str("% of the ").Int(int64(share.Gated)).
		Str(" gated MUSTs the ").Int(int64(share.RFCs)).
		Str(" RFCs ze implements carry are proven by a tagged test: both polarities, or one " +
			"polarity whose annotation records that no input drives the other side. The gate " +
			"holds a wider set -- ").Int(int64(share.GatedInspected)).
		Str(" gated MUSTs across ").Int(int64(share.Inspected)).
		Str(" enrolled RFCs -- and of the ").Int(int64(gated - both)).
		Str(" of those not proven in both polarities: ").
		Int(int64(kinds.get("not-applicable"))).
		Str(" not-applicable (recorded as not binding ze; the owner ruling of 2026-08-31 " +
			"presumes most of these need re-homing, so they stay inside the denominator " +
			"above rather than being subtracted from it), ").
		Int(int64(kinds.get("gap"))).
		Str(" known gap (unimplemented, genuinely untested), ").
		Int(int64(kinds.get("single-polarity"))).
		Str(" single-polarity -- those DO have a passing tagged test, just one side of the " +
			"pair, and the RFC gate fails if that test is missing -- and ").
		Int(int64(noTest)).
		Str(" with no test and no annotation at all, which is what `./le rfc check` is red " +
			"about. Only the gap column and that last one are untested work.").String()

	tb.Reset()
	value := tb.Int(int64(share.Proven)).Str(" / ").Int(int64(share.Gated)).String()

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
	data.set("gated_without_any_test", noTest)
	data.set("rfcs_total", len(rows))
	data.set("rfcs_without_any_proof", len(unproven))
	data.set("worst", worst)

	return Metric{
		Key:      keyProofDensity,
		Question: "Q2",
		Label:    "RFC MUST requirements proven by test, over the RFCs ze implements",
		Status:   floors.status(keyProofDensity, percentOf(density)),
		Value:    value,
		Detail:   detail,
		Action: "Write a test for a {gap} requirement, or for one carrying no test and no " +
			"annotation. A single-polarity requirement is already counted as proven, and " +
			"not-applicable needs no test.",
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
	// structuralFacts gates this list, so two properties are load-bearing.
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
