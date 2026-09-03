// Design: docs/architecture/testing/test-health.md -- the testing-health page
// Detail: collect.go -- the collectors behind the metrics
//
// Package testhealth renders the project's testing state as one generated
// Markdown page, and gates the facts on it whose change is an EVENT.
//
// The page answers "is our testing correct?", not "is our testing large?".
// Those are different questions, and volume metrics answer only the second:
// 20k test functions tell you nothing about whether a regression would be
// caught. Every metric belongs to one of three questions; a candidate
// belonging to none is excluded as volume:
//
//	Q1 sensitivity   if the code were wrong, would something go red?
//	Q2 intent        are the things that matter checked, or only the happy path?
//	Q3 integrity     when something goes red, does it stop the line?
//
// What is gated, and what is merely published (facts.go):
//
//   - STRUCTURAL facts -- which test files no `go test` target builds, which
//     enrolled RFCs have no test pair, and every metric's status -- are gated
//     by `check`. Each one changing is an event.
//   - VOLUME counters are published, not gated. A byte-exact gate over the
//     whole report charged a regenerate-and-commit to ~60% of commits, since
//     every added test moves a denominator. A check firing that often for
//     cosmetic reasons is routed around rather than read, which is the
//     "advisory gate permanently red" failure this page exists to expose.
//
// The ratchets do not rest on any of this: `le test-sensitivity check`
// enforces them from the tree, reading only the baseline, and is unaffected by
// report staleness.
//
// Which files count is decided by GIT'S INDEX (tracked.go), never by a
// working-tree listing. Two honest limits on "committed": file CONTENTS are
// read from the working tree, so an uncommitted edit to a tracked test moves
// the counts; and the index is read, so staging a new test moves them before
// any commit.
//
// Metrics that need a live test run never go straight onto the page: `record`
// appends them to test/health/history.ndjson (committed) and the page renders
// trends from there.
package testhealth

import (
	"errors"
	"fmt"
	"regexp"
)

// The artifacts this tool reads and writes, repository-relative.
const (
	// Page is the generated Markdown a reader opens.
	Page = "docs/features/test-health.md"
	// Latest is the structured sibling of the page, and the file `check`
	// compares its structural facts against.
	Latest = "test/health/latest.json"
	// History is the append-only KPI log the trends are rendered from.
	History = "test/health/history.ndjson"
	// Baseline is the sensitivity ratchet floor. It ratchets DOWN.
	Baseline = "test/health/sensitivity-baseline.json"
	// QualityBaseline is the committed "best so far" for the higher-is-better
	// ratio metrics. A metric warns only when it drops BELOW its locked-in
	// best, so the attention table lists regressions rather than a permanent
	// gap to an arbitrary target. Contrast Baseline, which ratchets DOWN.
	QualityBaseline = "test/health/quality-baseline.json"
)

// The metric keys that more than one file names. A key is the identity a
// checked fact, ratchet floor, and KPI column share.
const (
	keyProofDensity  = "rfc-proof-density"
	keyUnproven      = "rfc-unproven"
	keyAssertNothing = "assert-nothing"
	keyTagOrphan     = "tag-orphan"
	keyNegative      = "negative-tests"
	keySleeps        = "ci-sleeps"
)

// qualityMetrics are the metrics whose status is a regression signal against
// QualityBaseline. The number each compares is read from its own data at
// tighten time.
var qualityMetrics = [...]string{keyProofDensity, keyNegative}

// The other inputs the collectors read.
const (
	rfcLedger        = "ai/RFC-REQUIREMENTS.md"
	rfcSummaries     = "rfc/short"
	rfcSummariesTree = "rfc"
	sleepBaseline    = "test/.ci-sleep-baseline"
	knownFailures    = "plan/known-failures"
)

// testRoots are the in-repo test trees. vendor/ and gokrazy/modcache/ are
// third-party module trees and are excluded.
var testRoots = [...]string{"internal", "cmd", "pkg", "test"}

// rfcTableHeader is the RFC ledger's coverage table header, pinned exactly: the
// ledger is generated, and a column change must fail loudly rather than
// silently yield zero.
//
// `Nightly-only` is a SUBSET marker rather than a partition member, so it is
// parsed but never summed with the others.
const rfcTableHeader = "| RFC | Gated | Both | One polarity | Annotated | No test | Outstanding | " +
	"Nightly-only | State |"

// rfcStateEnrolled is the State cell an enrolled RFC renders. Matched as a
// PREFIX, because the cell also carries a suffix when the RFC has been
// obsoleted: `**enrolled**, superseded by RFC9568`. Exact equality dropped four
// rows from the enrolled population and took 71 gated requirements off this
// page with them, silently, because the remainder and the annotation split were
// both narrowed by the same filter and still balanced.
const rfcStateEnrolled = "**enrolled**"

// The patterns the collectors read their inputs with.
var (
	// rfcRow is one coverage row of the ledger's rollup table.
	rfcRow = regexp.MustCompile(
		"^\\| `([^`]+)` \\| (\\d+) \\| (\\d+) \\| (\\d+) \\| (\\d+) \\| (\\d+) \\| (\\d+) \\| (\\d+) \\| (.*?) \\|$")
	// rfcLevel is a requirement line in rfc/short/*.md:
	// "- [ ] [RFC1234-1-1] [MUST] text {gap: ...}".
	rfcLevel = regexp.MustCompile(`^- \[[ x]\] \[[^\]]+\] \[([A-Z ]+)\]`)

	testFunc  = regexp.MustCompile(`(?m)^func (Test[A-Z_][A-Za-z0-9_]*)\(`)
	fuzzFunc  = regexp.MustCompile(`(?m)^func (Fuzz[A-Z_][A-Za-z0-9_]*)\(`)
	benchFunc = regexp.MustCompile(`(?m)^func (Benchmark[A-Z_][A-Za-z0-9_]*)\(`)

	// negativeAssert are the tokens that EXPECT an error, i.e. the test states
	// that a specific failure must occur. Deliberately narrow.
	//
	// An earlier version also matched `err != nil.*t\.(Fatal|Error)`, which
	// measured nearly the opposite of what it claimed: with no DOTALL it could
	// not match the idiomatic multi-line form at all, while it did match
	// one-line `if err != nil { t.Fatalf(...) }` -- which is overwhelmingly a
	// SETUP GUARD, the happy path, not an error-path assertion. Comments are
	// stripped before matching so prose mentioning wantErr does not count as
	// coverage.
	//
	// The whitespace class is spelled out rather than written `\s`: Python's
	// `\s` over a str counts the vertical tab and U+001C to U+001F, and Go's
	// does not. The corpus is gofmt'd Go, so only the space and the tab occur,
	// but the two halves must agree by construction rather than by luck.
	negativeAssert = regexp.MustCompile(
		`\b(wantErr|wantError|expectErr|expectError|ErrorIs|ErrorAs|ErrorContains|` +
			`EqualError|ErrorAssertionFunc)\b|\b(assert|require)\.Error\b` +
			`|err[ \t\n\r\f\v]*==[ \t\n\r\f\v]*nil[ \t\n\r\f\v]*\{|!errors\.Is\(|!errors\.As\(`)

	goLineComment  = regexp.MustCompile(`//[^\n]*`)
	goBlockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
)

// minSamples is how many samples a series needs before it is drawn as a trend.
// Three points make a convincing line out of noise; saying "insufficient data"
// is the honest answer.
const minSamples = 4

// The three statuses a collector can produce.
const (
	statusOK      = "ok"
	statusWarn    = "warn"
	statusUnknown = "unknown"
)

// statusValues is the only vocabulary a status may come from. The status fact
// is checked against it because that fact has no counter of its own: a reader
// on the wrong field yields a null for every metric, which compares EQUAL on
// both sides of the gate for any status change at all. The vocabulary is an
// independent source, so it catches what a snapshot comparison cannot.
var statusValues = map[string]bool{statusOK: true, statusWarn: true, statusUnknown: true}

// ErrCollect says a collector could not produce a trustworthy number.
//
// Every refusal wraps it rather than answering a zero: a guard that reports a
// permissive value on a miss is worse than no guard (ai/rules/evidence.md).
var ErrCollect = errors.New("test-health")

// collectErrorf builds a refusal a caller can test for with errors.Is.
func collectErrorf(format string, args ...any) error {
	return errors.Join(ErrCollect, fmt.Errorf(format, args...))
}

// Metric is one row on the page.
//
// Action is mandatory: a metric whose degradation implies no action is
// decoration, and decoration is what turns a dashboard into a green wall.
type Metric struct {
	Key      string
	Question string
	Label    string
	Status   string
	Value    string
	Detail   string
	Action   string
	// Data is the metric's own payload, merged into the record beside the
	// seven fields above. It keeps the order the keys were set in, so the
	// page's tables read their columns from it; the JSON sorts regardless.
	Data object
}

// asObject renders one metric the way the record carries it: the seven fields,
// then the metric's own data merged in.
func (m Metric) asObject() object {
	out := object{}
	out.set("key", m.Key)
	out.set("question", m.Question)
	out.set("label", m.Label)
	out.set("status", m.Status)
	out.set("value", m.Value)
	out.set("detail", m.Detail)
	out.set("action", m.Action)
	for _, key := range m.Data.keys {
		out.set(key, m.Data.get(key))
	}
	return out
}

// ratio answers a fraction that carries its parts.
//
// A percentage alone hides the improvement-by-shrinking-denominator failure: a
// score that rises because the hard packages stopped being sampled looks
// identical to real progress.
func ratio(num, den int) object {
	out := object{}
	out.set("numerator", num)
	out.set("denominator", den)
	if den == 0 {
		out.set("percent", nil)
		return out
	}
	out.set("percent", roundTo1(100.0*float64(num)/float64(den)))
	return out
}

// percentOf reads the percent out of a ratio, and answers nil when the
// denominator was zero.
func percentOf(part object) any { return part.get("percent") }
