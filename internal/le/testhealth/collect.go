// Design: docs/architecture/testing/test-health.md -- the metrics on the page
// Overview: testhealth.go -- the package doc these collectors serve
//
// collect.go holds the eight collectors that are not the RFC ledger's. Each
// one answers a metric, and each one REFUSES rather than returning a zero it
// did not measure: a guard that reports a permissive value on a miss is worse
// than no guard (ai/rules/evidence.md).
//
// Two of them answer `unknown` instead, and the distinction is deliberate. An
// input that is absent because nobody has run the thing yet -- no mutation
// sample, no known-failures directory -- is unmeasured, and unmeasured sorts
// ABOVE a bad number on the page. An input that is present and unreadable is a
// broken measurement, and that is an error.
package testhealth

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/testsensitivity"
)

// collectInert answers the assert-nothing and tag-orphan metrics.
//
// The scan is a FUNCTION CALL rather than a `go run` of the same detector.
// That is what closes the fail-open the script carries here: it read the
// detector's JSON by string key with an `or []` fallback, so a renamed or
// missing key answered an empty list -- zero inert tests and zero stranded
// files, which is the goal state for both, on a fact `check` gates.
//
// Status comes from the committed ratchet floor, not from an invented absolute
// threshold. A count sitting exactly at its agreed floor is recorded debt under
// an enforced contract, not a new problem, and coloring it red would make the
// page cry wolf on every run until the debt is fully paid.
func collectInert(t *tree) (Metric, Metric, error) {
	scan, err := testsensitivity.Scan(t.root, testsensitivity.Tracked)
	if err != nil {
		return Metric{}, Metric{}, collectErrorf("the sensitivity scan failed: %w", err)
	}
	if scan.TestsScanned == 0 {
		return Metric{}, Metric{}, collectErrorf("the sensitivity scan scanned zero tests")
	}

	// Read through the same validation the tighten path uses. Reading it raw
	// here meant a corrupt baseline crashed long before those guards could
	// report it.
	floors, err := readSensitivityFloors(t.root)
	if err != nil {
		return Metric{}, Metric{}, err
	}

	inert := inertMetric(scan, floors)
	orphan := orphanMetric(scan, floors)
	return inert, orphan, nil
}

// inertMetric renders the assert-nothing row.
func inertMetric(scan testsensitivity.Result, floors sensitivityFloors) Metric {
	found := len(scan.AssertNothing)

	worst := make([]any, 0, 10)
	for index, finding := range scan.AssertNothing {
		if index >= 10 {
			break
		}
		entry := object{}
		entry.set("file", finding.File)
		entry.set("test", finding.Test)
		worst = append(worst, entry)
	}

	data := object{}
	data.set("inert", ratio(found, scan.TestsScanned))
	data.set("worst", worst)

	var tb textbuf.Buffer
	value := tb.Int(int64(found)).Str(" / ").Int(int64(scan.TestsScanned)).
		Str(floorSuffix(floors.assertNothing)).String()

	return Metric{
		Key:      keyAssertNothing,
		Question: "Q1",
		Label:    "Tests with no reachable failure call",
		Status:   ratchetStatus(found, floors.assertNothing),
		Value:    value,
		Detail: "These execute code and pass unconditionally. Breaking the code under test " +
			"would not turn them red.",
		Action: "Add a real assertion, or annotate with `// test-asserts-nothing: <why>` " +
			"when the oracle is genuinely implicit (a must-not-panic smoke test).",
		Data: data,
	}
}

// orphanMetric renders the tag-orphan row, and NAMES every stranded file: the
// list is what `check` gates, so a display slice would hide the eleventh.
func orphanMetric(scan testsensitivity.Result, floors sensitivityFloors) Metric {
	orphans := make([]any, 0, len(scan.TagOrphan))
	for _, finding := range scan.TagOrphan {
		entry := object{}
		entry.set("file", finding.File)
		entry.set("requires", finding.Detail)
		orphans = append(orphans, entry)
	}

	data := object{}
	data.set("orphan_count", len(scan.TagOrphan))
	data.set("orphans", orphans)

	var tb textbuf.Buffer
	value := tb.Int(int64(len(scan.TagOrphan))).Str(floorSuffix(floors.tagOrphan)).String()

	return Metric{
		Key:      keyTagOrphan,
		Question: "Q3",
		Label:    "Test files no `go test` target can build",
		Status:   ratchetStatus(len(scan.TagOrphan), floors.tagOrphan),
		Value:    value,
		Detail: "Their build tags are supplied by no go test invocation in Makefile or mk/*.mk, " +
			"so these tests exist but never run.",
		Action: "Add the tag to a go test invocation, or delete the file. Either way the " +
			"false inventory shrinks.",
		Data: data,
	}
}

// collectInventory answers the honest in-repo test counts, with the counting
// boundary stated.
func collectInventory(t *tree) (Metric, error) {
	counts := object{}
	testFuncs, fuzzFuncs, benchFuncs, testFiles := 0, 0, 0, 0

	for _, subtree := range testRoots {
		listed, err := t.trackedMatching(subtree, "_test.go")
		if err != nil {
			return Metric{}, err
		}
		for _, rel := range listed {
			body, readErr := t.readBody(rel)
			if readErr != nil {
				return Metric{}, readErr
			}
			testFiles++
			testFuncs += len(testFunc.FindAllStringIndex(body, -1))
			fuzzFuncs += len(fuzzFunc.FindAllStringIndex(body, -1))
			benchFuncs += len(benchFunc.FindAllStringIndex(body, -1))
		}
	}
	if testFuncs == 0 {
		return Metric{}, collectErrorf("counted zero test functions in the repository")
	}

	ciFiles, err := t.trackedMatching("test", ".ci")
	if err != nil {
		return Metric{}, err
	}
	etFiles, err := t.trackedMatching("test", ".et")
	if err != nil {
		return Metric{}, err
	}

	// The insertion order is the script's, which is what the page's table
	// columns would read if this metric ever grew one. The JSON sorts anyway.
	counts.set("test_funcs", testFuncs)
	counts.set("fuzz_funcs", fuzzFuncs)
	counts.set("bench_funcs", benchFuncs)
	counts.set("test_files", testFiles)
	counts.set("ci_files", len(ciFiles))
	counts.set("et_files", len(etFiles))

	data := object{}
	data.set("counts", counts)

	var tb textbuf.Buffer
	value := tb.Int(int64(testFuncs)).Str(" test functions").String()
	tb.Reset()
	detail := tb.Int(int64(testFiles)).Str(" Go test files, ").Int(int64(fuzzFuncs)).
		Str(" fuzz targets, ").Int(int64(benchFuncs)).Str(" benchmarks, ").
		Int(int64(len(ciFiles))).Str(" .ci scenarios, ").Int(int64(len(etFiles))).
		Str(" .et editor tests. Counts cover ").Join(testRoots[:], ", ").
		Str(" only: vendor/ and gokrazy/modcache/ are third-party module trees and are excluded.").
		String()

	return Metric{
		Key:      "inventory",
		Question: "Q2",
		Label:    "In-repo test inventory",
		Status:   statusOK,
		Value:    value,
		Detail:   detail,
		Action: "This is volume, not health. It is here to state the counting boundary, " +
			"because a count that silently includes vendored tests inflates by ~6x.",
		Data: data,
	}, nil
}

// mutationAction is the one remedy every unmeasured mutation branch names.
const mutationAction = "Run `make ze-mutation-test-changed`, then `make ze-test-health-record`."

// unknownMutation answers the row for a mutation history that measured nothing.
func unknownMutation(detail string) Metric {
	return Metric{
		Key:      keyMutation,
		Question: "Q1",
		Label:    "Mutation kill rate",
		Status:   statusUnknown,
		// The value word and the status word are the same word here on
		// purpose: the page prints "**unknown** (unknown)" for a sensor that
		// measured nothing, which is what makes it unmistakable.
		Value:  statusUnknown,
		Detail: detail,
		Action: mutationAction,
	}
}

// collectMutation answers the mutation kill rate from the committed history.
//
// The recorder is advisory and records nothing when the mutation report is
// missing, so an absent series means "not measured", never "score zero".
func collectMutation(t *tree, floors qualityFloors) (Metric, error) {
	var tb textbuf.Buffer
	if !exists(filepath.Join(t.root, filepath.FromSlash(mutationHistory))) {
		return unknownMutation(tb.Str(mutationHistory).
			Str(" does not exist; no mutation run has been recorded.").String()), nil
	}

	body, err := t.readBody(mutationHistory)
	if err != nil {
		return Metric{}, err
	}
	rows, err := parseNDJSON(body, mutationHistory)
	if err != nil {
		return Metric{}, err
	}
	if len(rows) == 0 {
		return unknownMutation(tb.Str(mutationHistory).Str(" is empty.").String()), nil
	}

	// Latest sample per package, so a package measured twice is not double
	// counted. The order is the order each package was FIRST seen, which is
	// what a Python dict keeps when a key is re-assigned.
	order := make([]string, 0, len(rows))
	latest := make(map[string]object, len(rows))
	for index, row := range rows {
		name, ok := row.get("package").(string)
		if !ok || name == "" {
			return Metric{}, collectErrorf(
				"%s line %d has no 'package'; rows without one would all collapse into a "+
					"single bucket and silently overwrite each other", mutationHistory, index+1)
		}
		if _, seen := latest[name]; !seen {
			order = append(order, name)
		}
		latest[name] = row
	}

	mutants, mutantsInt := sumField(order, latest, "mutants")
	killed, killedInt := sumField(order, latest, "killed")
	if mutants == 0 {
		return unknownMutation(tb.Str(mutationHistory).Str(" records ").Int(int64(len(latest))).
			Str(" package(s) but zero mutants; nothing was actually measured.").String()), nil
	}

	kill := ratioOf(killed, killedInt, mutants, mutantsInt)
	return mutationMetric(order, latest, rows, kill, floors), nil
}

// mutationMetric renders the measured mutation row.
func mutationMetric(order []string, latest map[string]object, rows []object,
	kill object, floors qualityFloors,
) Metric {
	// The ten weakest packages, by the score each row states. A row with no
	// numeric score ranks as a perfect one, which is the script's own reading
	// and is preserved: the list is a display slice, and changing the ranking
	// would change the page while the two halves run side by side.
	ranked := make([]string, len(order))
	copy(ranked, order)
	sort.SliceStable(ranked, func(i, j int) bool {
		return scoreOf(latest[ranked[i]]) < scoreOf(latest[ranked[j]])
	})

	worst := make([]any, 0, 10)
	for index, name := range ranked {
		if index >= 10 {
			break
		}
		entry := object{}
		entry.set("package", latest[name].get("package"))
		entry.set("score", latest[name].get("score"))
		worst = append(worst, entry)
	}

	data := object{}
	data.set("kill_rate", kill)
	data.set("packages_measured", len(latest))
	data.set("samples", len(rows))
	data.set("worst", worst)

	var tb textbuf.Buffer
	value := tb.Str(valueText(kill.get("numerator"))).Str(" / ").
		Str(valueText(kill.get("denominator"))).String()
	tb.Reset()
	detail := tb.Str(valueText(percentOf(kill))).Str("% across ").Int(int64(len(latest))).
		Str(" of the repository's packages. Mutation operators are biased toward arithmetic, " +
			"conditionals and returns, and are nearly blind to concurrency and wire-format semantics.").
		String()

	return Metric{
		Key:      keyMutation,
		Question: "Q1",
		Label:    "Mutants killed, latest sample per package",
		Status:   floors.status(keyMutation, percentOf(kill)),
		Value:    value,
		Detail:   detail,
		Action:   "Take the lowest-scoring package and add tests until its survivors die.",
		Data:     data,
	}
}

// collectSleepRatchet answers the .ci sleep ratchet headroom. Sleeps hide the
// races they paper over.
func collectSleepRatchet(t *tree) (Metric, error) {
	var tb textbuf.Buffer
	if !exists(filepath.Join(t.root, filepath.FromSlash(sleepBaseline))) {
		return Metric{
			Key:      keySleeps,
			Question: "Q1",
			Label:    "time.sleep() in .ci tests",
			Status:   statusUnknown,
			Value:    statusUnknown,
			Detail:   tb.Str(sleepBaseline).Str(" does not exist.").String(),
			Action:   "Restore the baseline file; the ratchet is unenforced without it.",
		}, nil
	}

	raw, err := t.readBody(sleepBaseline)
	if err != nil {
		return Metric{}, err
	}
	// The baseline is the composable delta form: full-line `#` comments plus
	// signed-integer lines that sum to the ceiling. A file with no parseable
	// integer line is malformed and must fail closed -- a garbage baseline may
	// not silently disable the ratchet (ai/rules/evidence.md).
	ceiling, active := parseSleepBaseline(raw)
	if !active {
		tb.Reset()
		return Metric{}, collectErrorf("%s has no parseable ceiling (delta form: `#` comments "+
			"+ signed-int lines): %s", sleepBaseline, tb.Quoted(strings.TrimSpace(raw)).String())
	}

	listed, err := t.trackedMatching("test", ".ci")
	if err != nil {
		return Metric{}, err
	}
	actual := 0
	for _, rel := range listed {
		body, readErr := t.readBody(rel)
		if readErr != nil {
			return Metric{}, readErr
		}
		actual += strings.Count(body, "time.sleep(")
	}

	status := statusWarn
	if actual <= ceiling {
		status = statusOK
	}

	data := object{}
	data.set("actual", actual)
	data.set("baseline", ceiling)
	data.set("headroom", ceiling-actual)

	tb.Reset()
	return Metric{
		Key:      keySleeps,
		Question: "Q1",
		Label:    "time.sleep() calls in .ci tests",
		Status:   status,
		Value:    tb.Int(int64(actual)).Str(" (floor ").Int(int64(ceiling)).Byte(')').String(),
		Detail: "A sleep is a guess about timing that hides the race it was added to mask. " +
			"The ratchet allows the count to fall, never rise.",
		Action: "Replace a sleep with a payload-predicate wait (wait_until, dispatch_until), " +
			"then lower the floor in the same change.",
		Data: data,
	}, nil
}

// areaCount is one subsystem's share of the negative-test ratio.
type areaCount struct {
	name     string
	files    int
	negative int
}

// collectNegativeTests answers the share of test files that assert an error
// path, per subsystem.
func collectNegativeTests(t *tree, floors qualityFloors) (Metric, error) {
	order := make([]string, 0)
	perArea := make(map[string]*areaCount)

	for _, subtree := range testRoots {
		listed, err := t.trackedMatching(subtree, "_test.go")
		if err != nil {
			return Metric{}, err
		}
		for _, rel := range listed {
			body, readErr := t.readBody(rel)
			if readErr != nil {
				return Metric{}, readErr
			}
			name := areaOf(rel)
			area, seen := perArea[name]
			if !seen {
				area = &areaCount{name: name}
				perArea[name] = area
				order = append(order, name)
			}
			area.files++
			code := goBlockComment.ReplaceAllString(goLineComment.ReplaceAllString(body, ""), "")
			if negativeAssert.MatchString(code) {
				area.negative++
			}
		}
	}

	totalFiles, totalNegative := 0, 0
	for _, name := range order {
		totalFiles += perArea[name].files
		totalNegative += perArea[name].negative
	}
	if totalFiles == 0 {
		return Metric{}, collectErrorf("no test files found while measuring negative-test ratio")
	}

	// Only areas with enough files for the ratio to mean anything. The sort is
	// STABLE over that ratio, so the many ties break on the order the files
	// arrived in -- which is why tracked_matching orders paths the way Python
	// orders them (tracked.go, lessByPathParts).
	ranked := make([]string, 0, len(order))
	for _, name := range order {
		if perArea[name].files >= 5 {
			ranked = append(ranked, name)
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		left, right := perArea[ranked[i]], perArea[ranked[j]]
		return float64(left.negative)/float64(left.files) < float64(right.negative)/float64(right.files)
	})

	worst := make([]any, 0, 10)
	for index, name := range ranked {
		if index >= 10 {
			break
		}
		area := perArea[name]
		entry := object{}
		entry.set("area", area.name)
		entry.set("negative", area.negative)
		entry.set("files", area.files)
		entry.set("percent", roundTo1(100.0*float64(area.negative)/float64(area.files)))
		worst = append(worst, entry)
	}

	overall := ratio(totalNegative, totalFiles)
	data := object{}
	data.set("overall", overall)
	data.set("worst", worst)

	var tb textbuf.Buffer
	return Metric{
		Key:      keyNegative,
		Question: "Q2",
		Label:    "Test files that expect a specific error",
		Status:   floors.status(keyNegative, percentOf(overall)),
		Value:    tb.Int(int64(totalNegative)).Str(" / ").Int(int64(totalFiles)).String(),
		Detail: "Counts files using an error-expectation token (wantErr, ErrorIs, " +
			"assert.Error, ...), with comments stripped. Setup guards of the form " +
			"`if err != nil { t.Fatal(err) }` are deliberately NOT counted: those assert the " +
			"happy path. Blind spot: expecting *an* error is weaker than pinning the right one.",
		Action: "Take the lowest-ranked subsystem and add malformed-input or fault-injection cases.",
		Data:   data,
	}, nil
}

// areaOf answers the subsystem a test file belongs to.
//
// The first three path components of the DIRECTORY, never of the file: taking
// them from the path made 117 of 318 "areas" single files, and the five-file
// filter then dropped every one of them, so whole trees could never appear in
// the table this metric's own action tells you to act on.
func areaOf(rel string) string {
	parent := path.Dir(rel)
	if parent == "." {
		return parent
	}
	parts := strings.Split(parent, "/")
	if len(parts) > 3 {
		parts = parts[:3]
	}
	return strings.Join(parts, "/")
}

// bucket is one package-age bucket of the adoption metric.
type bucket struct {
	packages    int
	withFuzz    int
	withRFCTag  int
	withCI      int
	fuzzSeen    map[string]bool
	rfcSeen     map[string]bool
	packageSeen map[string]bool
}

// newBucket declares an empty bucket with its per-directory memories.
func newBucket() *bucket {
	return &bucket{
		fuzzSeen:    map[string]bool{},
		rfcSeen:     map[string]bool{},
		packageSeen: map[string]bool{},
	}
}

// collectAdoption answers age-bucketed technique adoption: the
// forward-only-adoption detector.
//
// A technique introduced in year N and never back-filled shows as a step: new
// packages carry it, older ones never do. Global counts cannot see this, which
// is why back-filling is a BLOCKING rule.
//
// Package age uses each directory's first-commit date. Coarse year buckets keep
// the generated page stable: a new commit to an existing package does not move
// its first-commit year, so the page does not churn.
func collectAdoption(t *tree) (Metric, error) {
	shallow, err := t.gitOutput("rev-parse", "--is-shallow-repository")
	if err == nil && strings.TrimSpace(shallow) == "true" {
		// In a shallow clone git attributes every file to the graft commit, so
		// every package lands in one bucket. Rendering that would make the page
		// differ from the committed one and red the staleness gate with no
		// escape: the diagnostic would say "regenerate and commit", which then
		// breaks every full clone. Unmeasured is the honest answer.
		return Metric{
			Key:      "adoption",
			Question: "Q2",
			Label:    "Technique adoption by package age",
			Status:   statusUnknown,
			Value:    statusUnknown,
			Detail: "This is a shallow clone, so git attributes every file to the graft " +
				"commit and package age cannot be derived. Re-run in a full clone " +
				"(`git fetch --unshallow`).",
			Action: "Nothing to do here; the metric needs full history.",
		}, nil
	}

	firstSeen, err := firstCommitByDirectory(t)
	if err != nil {
		return Metric{}, err
	}

	buckets := map[string]*bucket{}
	var undated []string
	undatedSeen := map[string]bool{}

	for _, subtree := range testRoots {
		listed, listErr := t.trackedMatching(subtree, "_test.go")
		if listErr != nil {
			return Metric{}, listErr
		}
		for _, rel := range listed {
			directory := path.Dir(rel)
			stamp, dated := firstSeen[directory]
			if !dated {
				if !undatedSeen[directory] {
					undatedSeen[directory] = true
					undated = append(undated, directory)
				}
				continue
			}
			year := yearOf(stamp)
			slot, ok := buckets[year]
			if !ok {
				slot = newBucket()
				buckets[year] = slot
			}
			// Track seen directories as a SET per bucket. A single-slot
			// sentinel over-counted plenty: sorting by full path puts a
			// subdirectory between two files of its parent, the parent is
			// re-entered, and its package is counted again -- 70 re-entries on
			// this repository, publishing 490 packages where 481 exist.
			if !slot.packageSeen[directory] {
				slot.packages++
				slot.packageSeen[directory] = true
			}
			body, readErr := t.readBody(rel)
			if readErr != nil {
				return Metric{}, readErr
			}
			if !slot.fuzzSeen[directory] && fuzzFunc.MatchString(body) {
				slot.withFuzz++
				slot.fuzzSeen[directory] = true
			}
			if !slot.rfcSeen[directory] && strings.Contains(body, "RFC requirement:") {
				slot.withRFCTag++
				slot.rfcSeen[directory] = true
			}
		}
	}

	// .ci functional tests live under test/, not beside the Go test files, so
	// they carry their own adoption story: counting a directory as having .ci
	// coverage lets the step-detector show whether functional testing was
	// back-filled to older subsystems too.
	listed, err := t.trackedMatching("test", ".ci")
	if err != nil {
		return Metric{}, err
	}
	ciSeen := map[string]bool{}
	for _, rel := range listed {
		directory := path.Dir(rel)
		stamp, dated := firstSeen[directory]
		if !dated {
			if !undatedSeen[directory] {
				undatedSeen[directory] = true
				undated = append(undated, directory)
			}
			continue
		}
		year := yearOf(stamp)
		var tb textbuf.Buffer
		key := tb.Str(year).Byte('\x00').Str(directory).String()
		if ciSeen[key] {
			continue
		}
		ciSeen[key] = true
		slot, ok := buckets[year]
		if !ok {
			slot = newBucket()
			buckets[year] = slot
		}
		slot.withCI++
	}

	if len(buckets) == 0 {
		return Metric{}, collectErrorf("could not bucket any package by first-commit date")
	}

	return adoptionMetric(buckets, undated), nil
}

// adoptionMetric renders the bucket table.
func adoptionMetric(buckets map[string]*bucket, undated []string) Metric {
	years := make([]string, 0, len(buckets))
	for year := range buckets {
		years = append(years, year)
	}
	sort.Strings(years)

	clean := object{}
	for _, year := range years {
		slot := buckets[year]
		entry := object{}
		entry.set("packages", slot.packages)
		entry.set("with_fuzz", slot.withFuzz)
		entry.set("with_rfc_tag", slot.withRFCTag)
		entry.set("with_ci", slot.withCI)
		clean.set(year, entry)
	}

	detail := "A technique adopted only forward from its introduction shows here as a step: " +
		"recent buckets carry it, older ones never do."
	status := statusOK
	if len(undated) > 0 {
		// Silently dropping these would shrink the denominator with no signal,
		// which is the fail-open this page exists to avoid.
		status = statusWarn
		var tb textbuf.Buffer
		detail = tb.Str(detail).Byte(' ').Int(int64(len(undated))).
			Str(" directory(ies) have no add-commit in this history and are excluded; " +
				"a shallow clone is the usual cause.").String()
	}

	data := object{}
	data.set("buckets", clean)
	data.set("undated_directories", len(undated))

	var tb textbuf.Buffer
	return Metric{
		Key:      "adoption",
		Question: "Q2",
		Label:    "Technique adoption by package age",
		Status:   status,
		Value:    tb.Int(int64(len(years))).Str(" age buckets").String(),
		Detail:   detail,
		Action: "Back-fill the oldest bucket, or record the uncovered remainder as tracked " +
			"backlog (ai/rules/testing.md, Back-Fill New Test Types).",
		Data: data,
	}
}

// firstCommitByDirectory answers each directory's first-commit timestamp.
//
// Both diff knobs are PINNED. Rename detection is controlled by the user's
// diff.renames config, and with it on a renamed file's add is not reported: 515
// directories got a different first-commit stamp and 258 moved year bucket, so
// one commit rendered two different pages on two machines.
func firstCommitByDirectory(t *tree) (map[string]int64, error) {
	out, err := t.gitOutput(
		"-c", "diff.renames=false", "-c", "core.quotePath=false",
		"log", "--reverse", "--diff-filter=A", "--format=%H %at", "--name-only", "--no-renames")
	if err != nil {
		return nil, collectErrorf("git log failed: %w", err)
	}

	firstSeen := map[string]int64{}
	var stamp int64
	for line := range strings.SplitSeq(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if when, header := commitHeader(line); header {
			stamp = when
			continue
		}
		directory := path.Dir(line)
		if directory == "." || directory == "" {
			continue
		}
		if _, seen := firstSeen[directory]; !seen {
			firstSeen[directory] = stamp
		}
	}
	return firstSeen, nil
}

// commitHeader reads a `<sha> <unix-time>` line, and reports false for a file
// name. The shape is the discriminator the script used: two whitespace-split
// fields, the first forty characters long and the second all digits.
func commitHeader(line string) (int64, bool) {
	fields := strings.Fields(line)
	if len(fields) != 2 || len(fields[0]) != 40 || !allDigits(fields[1]) {
		return 0, false
	}
	stamp := int64(0)
	for index := range len(fields[1]) {
		stamp = stamp*10 + int64(fields[1][index]-'0')
	}
	return stamp, true
}

// allDigits reports whether every byte is an ASCII digit, and refuses an empty
// string the way Python's str.isdigit does.
func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for index := range len(value) {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

// yearOf answers the UTC year of a unix timestamp.
func yearOf(stamp int64) string {
	var tb textbuf.Buffer
	return tb.Int(int64(time.Unix(stamp, 0).UTC().Year())).String()
}

// collectKnownFailures answers the tests logged as known-red. Debt that is
// tracked but still debt.
//
// One file per LIVE failure; RESOLVED.md archives the history verbatim;
// README.md holds the logging instructions. Neither of those two is a live
// failure. The aggregate is folded on read here, never stored.
func collectKnownFailures(t *tree) (Metric, error) {
	directory := filepath.Join(t.root, filepath.FromSlash(knownFailures))
	info, err := os.Stat(directory)
	//nolint:nilerr // an absent directory is UNMEASURED, which is a metric value
	// rather than a failure of this run: reporting "0 known failures" for a
	// directory nobody could read is the sensor rot the page exists to expose.
	if err != nil || !info.IsDir() {
		// An absent input is unmeasured, never healthy. Reporting "0 known
		// failures" for a directory nobody could read is the sensor-rot failure
		// the page exists to expose.
		return Metric{
			Key:      "known-failures",
			Question: "Q3",
			Label:    "Logged known-failing tests",
			Status:   statusUnknown,
			Value:    statusUnknown,
			Detail:   "plan/known-failures/ does not exist, so nothing was measured.",
			Action:   "Restore the directory, or drop this metric if the log was retired.",
		}, nil
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		return Metric{}, collectErrorf("%s cannot be listed: %w", knownFailures, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".md") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	// LIVE = one shard file per live failure, excluding the two bookkeeping
	// files. Counting the RESOLVED archive would report the debt this project
	// has already paid off.
	live, struck := 0, 0
	for _, name := range names {
		if name == "README.md" || name == "RESOLVED.md" {
			continue
		}
		body, readErr := t.readBody(path.Join(knownFailures, name))
		if readErr != nil {
			return Metric{}, readErr
		}
		if shardIsStruck(body) {
			struck++
			continue
		}
		live++
	}

	resolved := struck
	if exists(filepath.Join(directory, "RESOLVED.md")) {
		body, readErr := t.readBody(path.Join(knownFailures, "RESOLVED.md"))
		if readErr != nil {
			return Metric{}, readErr
		}
		for line := range strings.SplitSeq(body, "\n") {
			if strings.HasPrefix(line, "### ") {
				resolved++
			}
		}
	}

	status := statusWarn
	if live == 0 {
		status = statusOK
	}

	data := object{}
	data.set("live", live)
	data.set("resolved", resolved)

	var tb textbuf.Buffer
	detail := tb.Str("Reds logged rather than fixed, one shard file per live failure (").
		Int(int64(resolved)).
		Str(" entries archived in plan/known-failures/RESOLVED.md are not counted). Structural " +
			"gates may never be logged here, but a live entry is not necessarily flaky: some " +
			"are deterministic product bugs awaiting a fix.").String()
	tb.Reset()

	return Metric{
		Key:      "known-failures",
		Question: "Q3",
		Label:    "Logged known-failing tests",
		Status:   status,
		Value:    tb.Int(int64(live)).String(),
		Detail:   detail,
		Action: "Fix or delete the oldest entry; a permanently logged failure is a deleted " +
			"test with extra steps.",
		Data: data,
	}, nil
}

// shardIsStruck reports whether a live shard's first `### ` heading is struck
// through.
//
// The sharded model never writes a struck live shard -- a cleared red is moved
// to RESOLVED.md and its shard deleted -- but a stray strike must not be able
// to inflate the live-debt figure, so it is treated as resolved.
func shardIsStruck(body string) bool {
	for line := range strings.SplitSeq(body, "\n") {
		if strings.HasPrefix(line, "### ") {
			return strings.HasPrefix(strings.TrimSpace(line[4:]), "~~")
		}
	}
	return false
}
