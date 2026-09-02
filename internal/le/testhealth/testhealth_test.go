// VALIDATES: spec-le-is-a-ze-binary AC-5, AC-7, AC-8 -- every case here calls
// the tool as a function, the payloads are structured data with kebab-case
// keys, and each action answers the exit code its gate answered.
// PREVENTS: a port that renders the same page from different numbers. The
// artifacts this tool writes are byte-compared by a staleness gate, so a
// difference of one digit or one key order reddens `le test-health check` for
// every session until somebody regenerates.
//
// The whole-tool comparison against the script lives beside the script, in
// internal/le/testhealth/testhealth_test.go, so the commit that deletes the
// script deletes its proof with it. What is here is the arithmetic, the
// orderings and the guards, each reachable without a tree.

package testhealth

import (
	"strings"
	"testing"
)

func TestAPythonFloatKeepsItsTrailingZero(t *testing.T) {
	cases := []struct {
		value float64
		want  string
	}{
		{35.4, "35.4"},
		{100, "100.0"},
		{0, "0.0"},
		{60.4, "60.4"},
		{-0.5, "-0.5"},
		// The two ends of the fixed range Python renders without an exponent.
		{1e16, "1e+16"},
		{1e15, "1000000000000000.0"},
		{0.0001, "0.0001"},
		{0.00001, "1e-05"},
	}
	for _, tc := range cases {
		if got := pyFloatRepr(tc.value); got != tc.want {
			t.Errorf("pyFloatRepr(%v) = %q, want %q", tc.value, got, tc.want)
		}
	}
}

func TestARatioRoundsToOneDecimalAndCarriesItsParts(t *testing.T) {
	part := ratio(1239, 2966)
	if got := part.get("numerator"); got != 1239 {
		t.Errorf("the numerator is %v, want 1239", got)
	}
	if got := part.get("denominator"); got != 2966 {
		t.Errorf("the denominator is %v, want 2966", got)
	}
	if got := valueText(percentOf(part)); got != "41.8" {
		t.Errorf("the percent renders as %q, want \"41.8\"", got)
	}
	// A zero denominator answers a null percent rather than a zero: nothing was
	// measured, and a zero would read as a measured floor.
	if got := percentOf(ratio(0, 0)); got != nil {
		t.Errorf("a zero denominator answered a percent of %v, want none", got)
	}
}

func TestARoundedHalfGoesToTheEvenDigit(t *testing.T) {
	// Python's round and Go's formatter both break a tie to even over the
	// double's exact value. Scaling by ten instead would round 0.25 to 0.3 here
	// and leave the two halves disagreeing on a committed number.
	cases := []struct {
		value float64
		want  string
	}{
		{0.25, "0.2"},
		{0.35, "0.3"},
		{2.5, "2.5"},
		{66.75, "66.8"},
	}
	for _, tc := range cases {
		if got := pyFloatRepr(roundTo1(tc.value)); got != tc.want {
			t.Errorf("roundTo1(%v) renders as %q, want %q", tc.value, got, tc.want)
		}
	}
}

func TestAnIndentedDocumentSortsItsKeysAndSpellsEveryNonASCIIRune(t *testing.T) {
	inner := object{}
	inner.set("zebra", 1)
	inner.set("alpha", nil)
	outer := object{}
	outer.set("second", inner)
	outer.set("first", "a §5 heading & <tag>")
	outer.set("empty", []any{})

	got, err := dumpIndented(outer)
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}
	const want = "{\n" +
		"  \"empty\": [],\n" +
		"  \"first\": \"a \\u00a75 heading & <tag>\",\n" +
		"  \"second\": {\n" +
		"    \"alpha\": null,\n" +
		"    \"zebra\": 1\n" +
		"  }\n" +
		"}"
	if got != want {
		t.Errorf("the document is\n%s\nwant\n%s", got, want)
	}
}

func TestACompactDocumentKeepsTheOrderItsKeysWereSetIn(t *testing.T) {
	row := object{}
	row.set("ts", "2026-01-01T00:00:00Z")
	row.set("sha", "abc")
	row.set("assert_nothing", 133)
	row.set("rfc_proof_percent", nil)

	got, err := dumpCompact(row)
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}
	const want = `{"ts":"2026-01-01T00:00:00Z","sha":"abc","assert_nothing":133,"rfc_proof_percent":null}`
	if got != want {
		t.Errorf("the row is\n%s\nwant\n%s", got, want)
	}
}

func TestAPathSortsComponentByComponent(t *testing.T) {
	// `-` is 0x2d and `/` is 0x2f, so a byte comparison puts cmd/ze-gok first
	// and Python puts cmd/ze first. The order reaches the page through the
	// negative-test ranking, which is a stable sort over a ratio and breaks its
	// many ties on the order the files arrived in.
	if !lessByPathParts("cmd/ze/a_test.go", "cmd/ze-gok/a_test.go") {
		t.Errorf("cmd/ze sorted after cmd/ze-gok, which is the byte order rather than Python's")
	}
	if lessByPathParts("cmd/ze-gok/a_test.go", "cmd/ze/a_test.go") {
		t.Errorf("the comparison is not antisymmetric over the pair it exists for")
	}
	if !lessByPathParts("internal/a", "internal/a/b") {
		t.Errorf("a prefix path sorted after the path it prefixes")
	}
}

func TestATestFileBelongsToTheFirstThreePartsOfItsDirectory(t *testing.T) {
	// Taking the parts from the PATH made 117 of 318 areas single files, and
	// the five-file filter then dropped every one of them.
	cases := map[string]string{
		"internal/component/bgp/reactor/peer_test.go": "internal/component/bgp",
		"internal/core/textbuf/textbuf_test.go":       "internal/core/textbuf",
		"cmd/ze/main_test.go":                         "cmd/ze",
		"a_test.go":                                   ".",
	}
	for path, want := range cases {
		if got := areaOf(path); got != want {
			t.Errorf("areaOf(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestARatchetedCountWithNoFloorWarnsOnlyWhenItCountedSomething(t *testing.T) {
	if got := ratchetStatus(0, floor{}); got != statusOK {
		t.Errorf("a clean count with no floor read %q, want %q", got, statusOK)
	}
	if got := ratchetStatus(1, floor{}); got != statusWarn {
		t.Errorf("a count with no floor read %q, want %q", got, statusWarn)
	}
	if got := ratchetStatus(8, floor{set: true, value: 8}); got != statusOK {
		t.Errorf("a count sitting on its floor read %q, want %q", got, statusOK)
	}
	if got := ratchetStatus(9, floor{set: true, value: 8}); got != statusWarn {
		t.Errorf("a count above its floor read %q, want %q", got, statusWarn)
	}
	if got := floorSuffix(floor{}); got != "" {
		t.Errorf("a value with no floor carries the suffix %q", got)
	}
	if got := floorSuffix(floor{set: true, value: 8}); got != " (floor 8)" {
		t.Errorf("the floor suffix is %q, want \" (floor 8)\"", got)
	}
}

func TestAQualityFloorWarnsOnlyOnARealRegression(t *testing.T) {
	floors := qualityFloors{values: object{}}
	floors.values.set(keyProofDensity, pyNum{f: 60.4})

	if got := floors.status(keyProofDensity, 60.4); got != statusOK {
		t.Errorf("a metric sitting on its floor read %q, want %q", got, statusOK)
	}
	if got := floors.status(keyProofDensity, 60.35); got != statusOK {
		t.Errorf("a 0.05 wobble read %q, want %q", got, statusOK)
	}
	if got := floors.status(keyProofDensity, 60.0); got != statusWarn {
		t.Errorf("a 0.4 regression read %q, want %q", got, statusWarn)
	}
	if got := floors.status("no-such-metric", 1.0); got != statusOK {
		t.Errorf("a metric with no recorded floor read %q, want %q", got, statusOK)
	}
	if got := floors.status(keyProofDensity, nil); got != statusUnknown {
		t.Errorf("a metric that measured nothing read %q, want %q", got, statusUnknown)
	}
}

func TestARequirementLineCountsOnceUnderTheFirstKindItCarries(t *testing.T) {
	// A line carries at most one coverage annotation and the first kind in
	// table order wins. Counting a second would make the split EXCEED the
	// ledger's remainder, and the split is presented as a partition of it.
	cases := []struct {
		line string
		want string
		ok   bool
	}{
		{"- [ ] [RFC1-1-1] [MUST] a thing {gap: unimplemented}", "gap", true},
		{"- [ ] [RFC1-1-2] [MUST] a thing {not-applicable: no producer}", "not-applicable", true},
		{"- [ ] [RFC1-1-3] [MUST] a thing {single-polarity: negative; why}", "single-polarity", true},
		{"- [ ] [RFC1-1-4] [MUST] a thing {gap}", "gap", true},
		// Two kinds on one line: the first in table order wins, and the line
		// still contributes exactly one to the split.
		{"- [ ] [RFC1-1-5] [MUST] a thing {gap: why} {single-polarity: negative; why}", "gap", true},
		{"- [ ] [RFC1-1-6] [MUST] a thing {superseded: restated RFC2-1-1; why}", "", false},
		{"- [ ] [RFC1-1-7] [MUST] a thing with no annotation", "", false},
		{"- [ ] [RFC1-1-8] [MUST] a thing mentioning a gap in prose", "", false},
	}
	for _, tc := range cases {
		got, annotated := annotationOf(tc.line)
		if annotated != tc.ok || got != tc.want {
			t.Errorf("annotationOf(%q) = (%q, %v), want (%q, %v)",
				tc.line, got, annotated, tc.want, tc.ok)
		}
	}
}

func TestTheUnprovenListIsNamedInNameOrderRatherThanByRank(t *testing.T) {
	// The list is GATED, so its order is part of the fact. Ordering by gated
	// count would rewrite it whenever extraction moves a count, turning pure
	// churn into a diff that reads as an event.
	rows := []ledgerRow{
		{rfc: "rfc9001", gated: 9},
		{rfc: "rfc1003", gated: 1},
		{rfc: "rfc4271", gated: 5},
	}
	metric := unprovenMetric(rows, rows)
	listed, ok := metric.Data.get("unproven_rfcs").([]any)
	if !ok {
		t.Fatalf("the metric carries no unproven list")
	}
	want := []string{"rfc1003", "rfc4271", "rfc9001"}
	if len(listed) != len(want) {
		t.Fatalf("the list names %d RFC(s), want %d", len(listed), len(want))
	}
	for index, name := range want {
		if listed[index] != name {
			t.Errorf("entry %d is %v, want %s -- the list is in rank order rather than name order",
				index, listed[index], name)
		}
	}
	// The display slice beside it IS ranked, worst first, and the two orders
	// must not be the same list.
	unproven, ok := metric.Data.get("unproven").(object)
	if !ok {
		t.Fatalf("the metric carries no unproven ratio")
	}
	if got := unproven.get("numerator"); got != 3 {
		t.Errorf("the count is %v, want 3", got)
	}
}

func TestASensitivityFloorFallsToTheMeasuredCountAndNeverRises(t *testing.T) {
	// The floor may only FALL. Raising it here would let a regression be
	// laundered into the baseline by running the generator, which is the
	// opposite of what a ratchet is for.
	root := t.TempDir()
	writeFixtureBaseline(t, root, 5, 3)

	row := object{}
	row.set("assert_nothing", 2)
	row.set("tag_orphan", 4)

	moved, err := tightenSensitivity(root, row, false)
	if err != nil {
		t.Fatalf("tightening a present baseline: %v", err)
	}
	if !moved {
		t.Errorf("a floor that fell from 5 to 2 reported no movement")
	}
	const want = "{\n  \"assert-nothing\": 2,\n  \"tag-orphan\": 3\n}\n"
	if got := readFixtureBaseline(t, root); got != want {
		t.Errorf("the baseline is\n%s\nwant assert-nothing lowered to 2 and tag-orphan held at 3", got)
	}

	// A second run at the same counts moves nothing.
	moved, err = tightenSensitivity(root, row, false)
	if err != nil {
		t.Fatalf("tightening an unchanged baseline: %v", err)
	}
	if moved {
		t.Errorf("an unchanged baseline reported movement")
	}
}

func TestAQualityFloorRisesToABetterNumberAndNeverFallsToAWorseOne(t *testing.T) {
	root := t.TempDir()
	writeFixtureFile(t, root, QualityBaseline, "{\n  \"rfc-proof-density\": 90.0\n}\n")

	regressed := []Metric{{Key: keyProofDensity, Data: ratioData("proof_density", 15, 20)}}
	moved, err := tightenQuality(root, regressed)
	if err != nil {
		t.Fatalf("tightening against a regression: %v", err)
	}
	if moved {
		t.Errorf("a 75%% proof rate under a floor of 90 moved the floor")
	}
	if got := readFixtureFile(t, root, QualityBaseline); got != "{\n  \"rfc-proof-density\": 90.0\n}\n" {
		t.Errorf("the floor is\n%s\nwant it held at 90.0", got)
	}

	improved := []Metric{{Key: keyProofDensity, Data: ratioData("proof_density", 19, 20)}}
	moved, err = tightenQuality(root, improved)
	if err != nil {
		t.Fatalf("tightening against an improvement: %v", err)
	}
	if !moved {
		t.Errorf("a 95%% proof rate under a floor of 90 reported no movement")
	}
	if got := readFixtureFile(t, root, QualityBaseline); got != "{\n  \"rfc-proof-density\": 95.0\n}\n" {
		t.Errorf("the floor is\n%s\nwant it raised to 95.0", got)
	}
}

func TestASensitivityBaselineMissingAKeyIsRefused(t *testing.T) {
	// Defaulting a missing floor to the current count is exactly how a raise
	// sneaks through.
	root := t.TempDir()
	writeFixtureFile(t, root, Baseline, "{\n  \"assert-nothing\": 5\n}\n")

	row := object{}
	row.set("assert_nothing", 2)
	row.set("tag_orphan", 4)

	if _, err := tightenSensitivity(root, row, false); err == nil {
		t.Errorf("a baseline with no tag-orphan floor was accepted")
	}
}

func TestAGatedListIsRefusedWhenItsOwnCounterDisagrees(t *testing.T) {
	// An empty list is the GOAL STATE for every fact here, so it must never be
	// reachable by a reader bug: both sides of a snapshot comparison degenerate
	// to the same empty list and the gate then denies nothing.
	orphans := object{}
	orphans.set("key", keyTagOrphan)
	orphans.set("status", statusOK)
	entry := object{}
	entry.set("file", "internal/a/x_test.go")
	entry.set("requires", "ze_missing")
	orphans.set("orphans", []any{entry})
	orphans.set("orphan_count", 0)

	byKey := map[string]object{keyTagOrphan: orphans}
	if _, err := readTagOrphans(byKey); err == nil {
		t.Errorf("a list of one entry passed a counter of zero")
	}

	orphans.set("orphan_count", 1)
	got, err := readTagOrphans(byKey)
	if err != nil {
		t.Fatalf("a list that agrees with its counter was refused: %v", err)
	}
	if len(got) != 1 || got[0][0] != "internal/a/x_test.go" {
		t.Errorf("the fact reads %v, want the one stranded file", got)
	}

	// A metric that does not carry the field at all is a snapshot written
	// before the field existed, which is not the same fact as zero.
	orphans.set("orphans", nil)
	if _, err := readTagOrphans(byKey); err == nil {
		t.Errorf("a missing list read as the goal state")
	}
	delete(byKey, keyTagOrphan)
	if _, err := readTagOrphans(byKey); err == nil {
		t.Errorf("a missing metric read as the goal state")
	}
}

func TestAStatusFactIsRefusedWhenAKeyCollapsesOrAStatusIsNotOne(t *testing.T) {
	build := func(key, status string) object {
		metric := object{}
		metric.set("key", key)
		metric.set("status", status)
		return metric
	}

	good := []object{build("alpha", statusOK), build("beta", statusWarn)}
	statuses, err := readStatuses(good)
	if err != nil {
		t.Fatalf("two well-formed metrics were refused: %v", err)
	}
	if len(statuses) != 2 || statuses["beta"] != statusWarn {
		t.Errorf("the fact reads %v, want two statuses", statuses)
	}

	if _, err := readStatuses(nil); err == nil {
		t.Errorf("an empty record made every structural fact vacuously healthy")
	}
	// A reader on the wrong KEY field collapses every entry under one key, and
	// the map silently keeps only the last.
	if _, err := readStatuses([]object{build("alpha", statusOK), build("alpha", statusWarn)}); err == nil {
		t.Errorf("two metrics sharing a key passed, so one status is no longer gated")
	}
	// A reader on the wrong STATUS field yields a value no collector produces,
	// and it compares equal on both sides of the gate for any change at all.
	if _, err := readStatuses([]object{build("alpha", "green")}); err == nil {
		t.Errorf("a status no collector produces passed, so the fact gates nothing")
	}
	if _, err := readStatuses([]object{build("", statusOK)}); err == nil {
		t.Errorf("a metric with no usable key passed")
	}
}

func TestADescribedChangeNamesWhatMovedRatherThanBothLists(t *testing.T) {
	committed := Facts{
		Statuses:   map[string]string{"alpha": statusOK},
		TagOrphans: [][2]string{{"a_test.go", "ze_one"}},
		Unproven:   []string{"rfc1000", "rfc1001"},
	}
	generated := Facts{
		Statuses:   map[string]string{"alpha": statusWarn},
		TagOrphans: [][2]string{{"a_test.go", "ze_one"}},
		Unproven:   []string{"rfc1001", "rfc1002"},
	}
	if committed.Equal(generated) {
		t.Fatalf("two records that differ compared equal")
	}

	changes := Describe(committed, generated)
	if len(changes) != 2 {
		t.Fatalf("%d fact(s) moved, want the status and the unproven list: %v", len(changes), changes)
	}
	for _, change := range changes {
		switch change.Fact {
		case "statuses":
			if len(change.Committed) != 1 || len(change.Generated) != 1 {
				t.Errorf("the status fact reports %v / %v, want both small sides",
					change.Committed, change.Generated)
			}
		case "rfc-unproven":
			if len(change.Gone) != 1 || change.Gone[0] != "rfc1000" {
				t.Errorf("the entries that left are %v, want rfc1000", change.Gone)
			}
			if len(change.New) != 1 || change.New[0] != "rfc1002" {
				t.Errorf("the entries that arrived are %v, want rfc1002", change.New)
			}
		default:
			t.Errorf("the unchanged fact %q was reported as moved", change.Fact)
		}
	}
}

func TestAnUnchangedRecordReportsNoChange(t *testing.T) {
	facts := Facts{
		Statuses:   map[string]string{"alpha": statusOK},
		TagOrphans: [][2]string{{"a_test.go", "ze_one"}},
		Unproven:   []string{"rfc1000"},
	}
	// The second side is built separately rather than aliased: comparing a
	// value with itself would pass for an Equal that answered true always.
	same := Facts{
		Statuses:   map[string]string{"alpha": statusOK},
		TagOrphans: [][2]string{{"a_test.go", "ze_one"}},
		Unproven:   []string{"rfc1000"},
	}
	if !facts.Equal(same) {
		t.Errorf("two records stating the same facts did not compare equal")
	}
	if changes := Describe(facts, same); len(changes) != 0 {
		t.Errorf("an unchanged record reported %v", changes)
	}
}

func TestASparklineDrawsOnlyWhenThereIsALineToDraw(t *testing.T) {
	if got := sparkline([]pyNum{{isInt: true, i: 1}}); got != "" {
		t.Errorf("one sample drew %q", got)
	}
	got := sparkline([]pyNum{{isInt: true, i: 10}, {isInt: true, i: 20}, {isInt: true, i: 15}})
	for _, want := range [...]string{
		`viewBox="0 0 240 40"`, `aria-label="trend, 3 samples, min 10, max 20"`,
		`points="0.0,38.0 120.0,2.0 240.0,20.0"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the sparkline does not carry %s:\n%s", want, got)
		}
	}
	// A flat series has no span to divide by, and dividing by zero would put
	// every point at the same NaN.
	flat := sparkline([]pyNum{{isInt: true, i: 5}, {isInt: true, i: 5}})
	if strings.Contains(flat, "NaN") {
		t.Errorf("a flat series drew a NaN: %s", flat)
	}
}

func TestATableCellEscapesTheSeparatorItWouldOtherwiseBreak(t *testing.T) {
	if got := cell("a|b"); got != "a\\|b" {
		t.Errorf("cell(%q) = %q, want the separator escaped", "a|b", got)
	}
	if got := cell(nil); got != "None" {
		t.Errorf("an absent value renders as %q, want the script's None", got)
	}
	if got := cell(66.5); got != "66.5" {
		t.Errorf("a float renders as %q, want \"66.5\"", got)
	}
}

func TestAWorstTableTakesItsColumnsFromEveryRow(t *testing.T) {
	// Taking the keys from the first row alone failed on a heterogeneous row,
	// and the union is what a metric with an optional column needs.
	first := object{}
	first.set("area", "internal/a")
	first.set("files", 5)
	second := object{}
	second.set("area", "internal/b")
	second.set("percent", 12.5)

	metric := Metric{Data: object{}}
	metric.Data.set("worst", []any{first, second})

	got, ok := worstTable(metric)
	if !ok {
		t.Fatalf("a metric with a worst list rendered no table")
	}
	const want = "| area | files | percent |\n" +
		"|---|---|---|\n" +
		"| internal/a | 5 |  |\n" +
		"| internal/b |  | 12.5 |"
	if got != want {
		t.Errorf("the table is\n%s\nwant\n%s", got, want)
	}
}

func TestASampleIdenticalApartFromItsTimestampIsTheSameSample(t *testing.T) {
	last := object{}
	last.set("ts", "2026-01-01T00:00:00Z")
	last.set("sha", "abc")
	last.set("assert_nothing", pyNum{isInt: true, i: 133})
	last.set("rfc_proof_percent", pyNum{f: 60.4})

	same := object{}
	same.set("ts", "2026-02-02T00:00:00Z")
	same.set("sha", "abc")
	same.set("assert_nothing", 133)
	same.set("rfc_proof_percent", 60.4)

	if !sameSample(last, same) {
		t.Errorf("two samples differing only in their timestamp were counted as different")
	}

	moved := object{}
	for _, key := range same.keys {
		moved.set(key, same.get(key))
	}
	moved.set("assert_nothing", 132)
	if sameSample(last, moved) {
		t.Errorf("a sample whose count moved was counted as a duplicate")
	}

	// A different commit at the same numbers is still a new sample, because the
	// sha is what a reader traces the point back through.
	elsewhere := object{}
	for _, key := range same.keys {
		elsewhere.set(key, same.get(key))
	}
	elsewhere.set("sha", "def")
	if sameSample(last, elsewhere) {
		t.Errorf("a sample at another commit was counted as a duplicate")
	}
}

func TestAStruckShardIsNotALiveFailure(t *testing.T) {
	if !shardIsStruck("# notes\n\n### ~~a failure that was cleared~~\n\nfixed\n") {
		t.Errorf("a struck heading read as live debt")
	}
	if shardIsStruck("# notes\n\n### a live failure\n\nstill red\n") {
		t.Errorf("a live heading read as resolved")
	}
	// Only the FIRST heading decides, so a struck sibling further down cannot
	// clear a shard whose own failure is still red.
	if shardIsStruck("### a live failure\n\n### ~~a cleared one~~\n") {
		t.Errorf("a later struck heading cleared a live shard")
	}
	if shardIsStruck("no headings here\n") {
		t.Errorf("a shard with no heading read as resolved")
	}
}

func TestACommitHeaderIsToldApartFromAFileName(t *testing.T) {
	stamp, header := commitHeader("0123456789012345678901234567890123456789 1700000000")
	if !header || stamp != 1700000000 {
		t.Errorf("a header line read as (%d, %v)", stamp, header)
	}
	for _, line := range [...]string{
		"internal/component/bgp/reactor.go",
		"a file with two words.go",
		"0123456789012345678901234567890123456789 notanumber",
		"short 1700000000",
	} {
		if _, header := commitHeader(line); header {
			t.Errorf("%q read as a commit header", line)
		}
	}
}

func TestTheActionTableDeclaresThreeNativeActionsAndTwoWrites(t *testing.T) {
	want := []string{"check", "update", "record"}
	rows := Actions().Actions
	if len(rows) != len(want) {
		t.Fatalf("the area declares %v, want %v", rows, want)
	}
	writes := 0
	for index, row := range rows {
		if row.Verb != want[index] {
			t.Errorf("action %d is %q, want %q", index, row.Verb, want[index])
		}
		if row.Writes {
			writes++
		}
	}
	if writes != 2 {
		t.Errorf("%d action(s) write, want update and record", writes)
	}
	if subs := Subs(); subs != "check | update (writes) | record (writes)" {
		t.Errorf("the help hint is %q", subs)
	}
}

func TestAnActionThisAreaDoesNotHoldAnswersTwo(t *testing.T) {
	// 2 rather than 1: a name the area does not hold is a different fact from a
	// gate that ran and failed, and callers read the two apart.
	if _, code := Answer([]string{"regenerate"}); code != 2 {
		t.Errorf("an unknown action answered %d, want 2", code)
	}
	// A value after a verb that takes none is a usage error: the tree is the
	// checkout and the rendering is a pipe operator (ai/rules/cli.md).
	if _, code := Answer([]string{"check", "docs/features/test-health.md"}); code != 2 {
		t.Errorf("a value after check answered %d, want 2", code)
	}
	listing, code := Answer(nil)
	if code != 0 {
		t.Errorf("the bare command answered %d, want 0", code)
	}
	if _, ok := listing.(interface{ Text() string }); !ok {
		t.Errorf("the listing has no rendering for a person")
	}
}

func TestEveryPayloadKeyIsKebabCase(t *testing.T) {
	// `| json` renders the payload, and the project's JSON keys are kebab-case
	// (CLAUDE.md). The record's own metric keys are a different document: they
	// are the script's, and both halves must print them unchanged.
	payloads := map[string][]string{
		"WriteReport":  fieldTags(WriteReport{}),
		"CheckReport":  fieldTags(CheckReport{}),
		"RecordReport": fieldTags(RecordReport{}),
		"Change":       fieldTags(Change{}),
		"Facts":        fieldTags(Facts{}),
	}
	for name, tags := range payloads {
		if len(tags) == 0 {
			t.Errorf("%s declares no JSON keys, so nothing was checked", name)
		}
		for _, tag := range tags {
			if strings.ContainsAny(tag, "_ ") || strings.ToLower(tag) != tag {
				t.Errorf("%s carries the JSON key %q, which is not kebab-case", name, tag)
			}
		}
	}
}

func TestTheCheckReportRendersEachVerdictOnce(t *testing.T) {
	fresh := CheckReport{Latest: Latest, Match: true}
	if fresh.Code() != 0 {
		t.Errorf("a matching snapshot answered %d, want 0", fresh.Code())
	}
	if !strings.Contains(fresh.Text(), "match the tree") {
		t.Errorf("the passing verdict reads %q", fresh.Text())
	}

	missing := CheckReport{Latest: Latest, Missing: Page}
	if missing.Code() != 1 || !strings.Contains(missing.Text(), "does not exist") {
		t.Errorf("an absent page answered %d and %q", missing.Code(), missing.Text())
	}

	stale := CheckReport{Latest: Latest, Changes: []Change{{
		Fact: "rfc-unproven", Gone: []string{"rfc1000"}, New: []string{"rfc1002"},
	}}}
	text := stale.Text()
	if stale.Code() != 1 {
		t.Errorf("a stale snapshot answered %d, want 1", stale.Code())
	}
	for _, want := range [...]string{
		"a STRUCTURAL fact changed", "rfc-unproven",
		"left the committed list: rfc1000", "new in the generated list: rfc1002",
		"Run `./le test-health update`",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the stale verdict does not carry %q:\n%s", want, text)
		}
	}
}

// VALIDATES: the rollup's Annotated and No test counts are READ from their own
// columns, so a gated MUST with no test and no annotation lands in the No test
// column and in no other.
// PREVENTS: the defect this replaced. The annotated count was derived as
// `gated - both`, which is the whole remainder, so every untested and
// unannotated requirement was counted as an annotation that does not exist.
// The split cross-check then refused and took the entire page down whenever
// `./le rfc check` was red -- which is exactly when the page is worth reading.
func TestTheLedgerRollupIsReadFromItsAnnotatedAndNoTestColumns(t *testing.T) {
	// Columns, in the order ai/RFC-REQUIREMENTS.md declares them: RFC, Gated,
	// Both, One polarity, Annotated, No test, Outstanding, Nightly-only, State.
	table := "| RFC | Gated | Both | One polarity | Annotated | No test | Outstanding | Nightly-only | State |\n" +
		"|---|---|---|---|---|---|---|---|---|\n" +
		"| `rfc9001` | 8 | 2 | 0 | 4 | 2 | 0 | 0 | **enrolled** |\n" +
		"| `rfc9002` | 3 | 3 | 0 | 0 | 0 | 0 | 0 | **enrolled** |\n" +
		"| `rfc9003` | 5 | 0 | 0 | 5 | 0 | 0 | 0 | **backlog** |\n" +
		"| not a row | 1 | 1 |\n"

	rows, err := ledgerRows(table)
	if err != nil {
		t.Fatalf("the rollup was refused: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("the rollup parsed to %d row(s), want 3", len(rows))
	}

	first := rows[0]
	if first.rfc != "rfc9001" {
		t.Fatalf("the first row is %q, want rfc9001", first.rfc)
	}
	// The row that tells the two readings apart: `gated - both` is 6 and the
	// Annotated column says 4, because 2 requirements carry no test at all.
	if first.annotated != 4 {
		t.Errorf("the annotated count is %d, want 4 -- it is derived from gated minus both, not read",
			first.annotated)
	}
	if first.noTest != 2 {
		t.Errorf("the no-test count is %d, want 2", first.noTest)
	}
	if first.both+first.annotated+first.noTest != first.gated {
		t.Errorf("%d both + %d annotated + %d with no test is not %d gated, so the columns do not partition the row",
			first.both, first.annotated, first.noTest, first.gated)
	}
	// A fully proven row leaves both columns at zero, and zero here is a real
	// count rather than an unread cell.
	if rows[1].annotated != 0 || rows[1].noTest != 0 {
		t.Errorf("a fully proven row reads annotated=%d no-test=%d, want 0 and 0",
			rows[1].annotated, rows[1].noTest)
	}
	// State is read too: collectRFC keeps only the enrolled rows, and a backlog
	// row counted into the population would be measured against a gate that
	// does not run over it.
	if rows[2].state != "**backlog**" {
		t.Errorf("the third row's state is %q, want **backlog**", rows[2].state)
	}
}
