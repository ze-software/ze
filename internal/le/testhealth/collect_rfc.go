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
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/rfc"
)

// coverageRow is one RFC's gated population, in the four states the model
// partitions it into.
//
// annotated and noTest are READ from the model's own counts rather than derived
// by subtracting both from gated. The two are not the same number whenever the
// gate is red: a gated MUST with no test and no annotation is in the remainder
// and in neither count, so `gated - both` counts it as an annotation that does
// not exist (ai/rules/principles.md, declare once).
type coverageRow struct {
	rfc         string
	gated       int
	both        int
	onePolarity int
	annotated   int
	noTest      int
	// enrolled says this RFC is one `./le rfc check` gates. It is a BOOLEAN
	// rather than a rendered State cell, because that cell is a rendering:
	// the same enrolment prints as `**enrolled**` on its own and as
	// `**enrolled**, superseded by RFC9568` when the IETF has replaced the
	// document, and only the model knows those are one state.
	enrolled bool
}

// annotationPattern answers the pattern that finds one annotation kind on a
// requirement line. The trailing class is what tells `{gap}` from `{gap: why}`
// and from a word that merely starts the same way.
func annotationPattern(kind string) *regexp.Regexp {
	var tb textbuf.Buffer
	return regexp.MustCompile(tb.Str(`\{`).Str(regexp.QuoteMeta(kind)).Str(`[:}]`).String())
}

// annotationKinds are the coverage annotations the split partitions the
// ledger's remainder into, READ from the vocabulary rather than restated here.
//
// It was a literal array of three until 2026-09-03, and the fourth kind
// (`lower-layer`) would have gone uncounted: the split would no longer have
// summed to the ledger's Annotated column, and collectRFC refuses that
// divergence, so the whole page would have gone down rather than published a
// non-partition. A list beside a registry is a future disagreement with nothing
// to arbitrate it (ai/rules/principles.md).
var annotationKinds = rfc.AnnotationKinds()

// annotationPatterns is those kinds, compiled once, in the order the first
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
// split below is taken over the ENROLLED coverage rows, which is the set `le
// rfc check` actually gates. The un-enrolled remainder is not hidden -- the
// generated ledger states it and lists every one of those rows -- it simply
// is not part of the partition asserted below, because nothing obliges it to be
// proven or annotated yet.
//
// The share was `both / gated`, summed from the rendered rollup's own columns,
// until 2026-09-02, and that one answered 43.2% where /quality/rfc-compliance/
// answered 58.1% for the same question. The rest of this file kept parsing the
// render until the model replaced it here: a number read back out of a
// generated artifact is a second declaration of a fact, and the two
// declarations diverge in silence (ai/rules/principles.md).
//
// One walk of the tree feeds both halves. rfc.Collect is what the share is
// taken over and what the rollup rows are derived from, so the two populations
// on this page cannot come from two readings of the corpus.
func collectRFC(t *tree, floors qualityFloors) (Metric, Metric, error) {
	collected, err := rfc.Collect(t.root)
	if err != nil {
		return Metric{}, Metric{}, err
	}
	return rfcMetrics(t, collected, floors)
}

// rfcMetrics judges one collected corpus, and is where every refusal below
// lives.
//
// Separated from the walk above so a case can hand it a corpus. Each refusal is
// a statement about the SHAPE of a corpus, and a guard whose only reachable
// input is this checkout's own tree is a guard nothing can drive
// (ai/rules/evidence.md). The tree argument is still taken, because
// annotationSplit reads rfc/short; every refusal fires before that read.
func rfcMetrics(t *tree, collected rfc.Collected, floors qualityFloors) (Metric, Metric, error) {
	// A corpus that yields no coverage row at all is refused rather than
	// reported. Every count below would be a zero this collector did not
	// measure, and a zero that reads as data is the failure this page exists to
	// avoid publishing (ai/rules/principles.md). The pinned ledger header this
	// replaced refused the same thing for the same reason: it caught a changed
	// column shape, where this catches a corpus that answered nothing.
	rows := modelRows(collected)
	if len(rows) == 0 {
		return Metric{}, Metric{}, collectErrorf(
			"the requirement model answers no gate-carrying RFC across %d requirement(s) "+
				"and %d tag(s), so every count on this page would be a zero it did not measure",
			len(collected.Requirements), len(collected.Tags))
	}

	// ENROLLED ROWS ONLY, on every side of the partition below. "Every gated
	// requirement is proven in both polarities or annotated" is what the RFC
	// gate enforces, and it enforces it for the ENROLLED set alone. An
	// un-enrolled summary's gated MUSTs are legitimately neither proven nor
	// annotated, because extract-then-enroll is the mandated order, so "gated
	// rows, not yet enrolled" is a REQUIRED intermediate state rather than an
	// anomaly. Summing the whole table made that state raise.
	//
	// Enrolment is read from the corpus rfc.Collect answered, never from
	// rfc/enrolled.txt: that map IS the population this assertion is about, and
	// a second source is how the two diverge again.
	enrolled := make([]coverageRow, 0, len(rows))
	for _, row := range rows {
		if row.enrolled {
			enrolled = append(enrolled, row)
		}
	}
	if len(enrolled) == 0 {
		return Metric{}, Metric{}, collectErrorf(
			"the requirement model answers %d gate-carrying RFC(s) and none of them enrolled. "+
				"An empty enrolled population satisfies the annotation partition below "+
				"vacuously, so it is refused rather than published as a measurement",
			len(rows))
	}
	rows = enrolled

	gated, both, onePolarity, annotated, noTest := 0, 0, 0, 0, 0
	for _, row := range rows {
		gated += row.gated
		both += row.both
		onePolarity += row.onePolarity
		annotated += row.annotated
		noTest += row.noTest
	}
	// A backstop, and named as one. rfc.CoverageRows drops an RFC whose gated
	// count is zero rather than answering a row for it, so every row here gates
	// at least one requirement and this sum cannot reach zero today. It stays
	// because the alternative to a guard that never fires is a page that
	// divides by zero the day that skip changes.
	if gated == 0 {
		return Metric{}, Metric{}, collectErrorf(
			"the requirement model reports zero gated requirements across %d enrolled "+
				"gate-carrying RFC(s)", len(rows))
	}

	kinds, err := annotationSplit(t, rows)
	if err != nil {
		return Metric{}, Metric{}, err
	}

	// The cross-check is the model's annotated COUNT against the live count
	// over rfc/short. Two derivations of one population, one from the parsed
	// requirement lines and one from a second read of the same summaries, which
	// is a real disagreement to arbitrate. Comparing against `gated - both`
	// instead compared the split against a different population -- the
	// remainder holds every gated MUST with no test as well -- so a red gate
	// made this refuse and took the whole page down with it.
	splitTotal := kinds.total()
	if splitTotal != annotated {
		return Metric{}, Metric{}, collectErrorf(
			"annotation split %s sums to %d, but the model's annotated count sums to %d "+
				"across %d gated requirement(s). The page must not present a non-partition as "+
				"one; the two sources have diverged",
			pythonDict(kinds), splitTotal, annotated, gated)
	}
	// The gated population has FOUR buckets, and every one of them must be
	// summed. One polarity is a real bucket: a requirement whose positive test
	// exists and whose negative one does not is neither proven both ways nor
	// annotated nor untested. Omitting it asserted a three-way partition over a
	// four-way population, which held only while that count summed to zero.
	if both+onePolarity+annotated+noTest != gated {
		return Metric{}, Metric{}, collectErrorf(
			"the model's own counts do not partition its gated population: %d both + %d "+
				"one polarity + %d annotated + %d with no test is not %d gated, so the page "+
				"would publish a remainder it cannot account for",
			both, onePolarity, annotated, noTest, gated)
	}

	// The published share is derived LAST, after the two guards above have
	// judged the corpus it is taken over. Taken first, it refused an empty
	// enrolled population on its own terms -- "no enrolled RFC declares a
	// public row claiming support" -- and that answer names a missing `|
	// Support status |` cell for a corpus whose real fault is that it carries
	// no enrolled RFC at all. A guard sitting behind another guard's message
	// cannot be reached, and cannot be driven by a case either
	// (ai/rules/evidence.md).
	share, err := provenShare(collected)
	if err != nil {
		return Metric{}, Metric{}, err
	}

	unproven := unprovenRows(rows)
	density := ratio(share.Proven, share.Gated)

	return densityMetric(rows, unproven, kinds, density, floors, share,
			rfcTotals{both: both, gated: gated, annotated: annotated, noTest: noTest}),
		unprovenMetric(rows, unproven), nil
}

// provenShare answers the published proof share over one collected corpus.
//
// The carriers argument is nil because no field of a ProvenShare reads one:
// rfc.CoverageRows takes them to decide NightlyOnly, which the share does not
// count. The site's home page and its RFC compliance report pass nil for the
// same reason, so the three surfaces cannot answer different numbers.
func provenShare(collected rfc.Collected) (rfc.ProvenShare, error) {
	return rfc.ProvenShareOf(collected.Metas, collected.Requirements, collected.Tags, nil)
}

// modelRows answers one coverage row per RFC from the requirement model.
//
// The SAME call renderRollup makes (internal/le/rfc/sections.go) to render the
// ledger's rollup table, so the page and this collector state one derivation of
// one population rather than one of them parsing the other's output
// (ai/rules/principles.md, declare once).
//
// The carriers argument is nil because no field read here depends on one:
// rfc.CoverageRows takes carriers only to decide NightlyOnly, which the ledger
// prints as an overlapping subset marker and this collector never sums.
func modelRows(collected rfc.Collected) []coverageRow {
	coverage := rfc.CoverageRows(collected.Requirements, collected.Tags, nil)
	rows := make([]coverageRow, 0, len(coverage))
	for _, row := range coverage {
		rows = append(rows, coverageRow{
			rfc: row.RFC, gated: row.Gated, both: row.Both, onePolarity: row.One,
			annotated: row.Annotated, noTest: row.Missing,
			enrolled: collected.Enrolled[row.RFC],
		})
	}
	return rows
}

// annotationSplit counts the coverage annotations of every enrolled summary.
//
// The other half of "same population": two filters, both required. A summary
// counts only when it owns an ENROLLED coverage row, and a line counts only when
// its requirement level is gated. Drop either filter and the split stops being
// a partition of `gated - both`.
func annotationSplit(t *tree, rows []coverageRow) (annotationCounts, error) {
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
			"no tracked summaries under %s match an enrolled coverage row: refusing to report "+
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
// it, worst first and then by name.
//
// The name breaks the tie because the order is PUBLISHED: densityMetric prints
// the first ten of this list. A stable sort left a tie in the order the rows
// arrived in, which made a display slice of ten a property of whoever produced
// the rows rather than of the rows themselves. Three RFCs gate 13 requirements
// each today, so moving this collector from the rendered ledger to the
// requirement model reordered the page while every count on it stayed put.
func unprovenRows(rows []coverageRow) []coverageRow {
	var unproven []coverageRow
	for _, row := range rows {
		if row.gated > 0 && row.both == 0 {
			unproven = append(unproven, row)
		}
	}
	slices.SortFunc(unproven, func(a, b coverageRow) int {
		if a.gated != b.gated {
			return b.gated - a.gated
		}
		return strings.Compare(a.rfc, b.rfc)
	})
	return unproven
}

// rfcTotals is the enrolled population, summed from the model's own counts.
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
func densityMetric(rows, unproven []coverageRow, kinds annotationCounts, density object,
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
		Int(int64(kinds.get(rfc.AnnotationNotApplicable))).
		Str(" not-applicable (recorded as not binding ze; the owner ruling of 2026-08-31 " +
			"presumes most of these need re-homing, so they stay inside the denominator " +
			"above rather than being subtracted from it), ").
		Int(int64(kinds.get(rfc.AnnotationGap))).
		Str(" known gap (unimplemented, genuinely untested), ").
		Int(int64(kinds.get(rfc.AnnotationSinglePolarity))).
		Str(" single-polarity -- those DO have a passing tagged test, just one side of the " +
			"pair, and the RFC gate fails if that test is missing -- ").
		Int(int64(kinds.get(rfc.AnnotationLowerLayer))).
		Str(" met by a layer under ze on state ze installs, which the annotation names with " +
			"the producer that installs it: those are MET and are not proven by ze, so they " +
			"count in the denominator above and never in the share, ").
		Int(int64(kinds.get(rfc.AnnotationFeatureDeclined))).
		Str(" conditional on an optional feature ze does not offer, each quoting the RFC " +
			"sentence that makes it optional: the condition is false, so nothing is owed, and ").
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
func unprovenMetric(rows, unproven []coverageRow) Metric {
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
	slices.Sort(sorted)
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
