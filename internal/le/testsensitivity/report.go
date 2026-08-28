// Design: docs/architecture/testing/test-health.md -- the sensitivity gate's answers
//
// report.go holds what the actions of `le test-sensitivity` ANSWER, apart from
// what produced them.
//
// One payload, two renderings. A SCAN renders the whole page: the counts, the
// tag universe, and every finding. A CHECK renders the one verdict line, and
// its breach detail goes to stderr, which is where the script put it. Both
// encode as the same JSON document, so `| json` answers one shape whichever
// action produced it.

package testsensitivity

import "github.com/ze-software/ze/internal/core/textbuf"

// The two reasons a test is inert. They are the script's own spellings, and
// they are what `| match tag-orphan` selects on.
const (
	// ReasonAssertNothing is a Test with no reachable failure call.
	ReasonAssertNothing = "assert-nothing"
	// ReasonTagOrphan is a test file no native test tag set can build.
	ReasonTagOrphan = "tag-orphan"
)

// Finding is one inert test, and it is one ROW of either list.
type Finding struct {
	File   string `json:"file"`
	Test   string `json:"test,omitempty"`
	Line   int    `json:"line"`
	Reason string `json:"reason"`
	Detail string `json:"detail,omitempty"`
}

// Result is the whole answer of one scan. The keys are the script's,
// unchanged.
type Result struct {
	AssertNothing []Finding `json:"assert-nothing"`
	TagOrphan     []Finding `json:"tag-orphan"`
	FilesScanned  int       `json:"files-scanned"`
	TestsScanned  int       `json:"tests-scanned"`
	TagUniverse   []string  `json:"test-tag-universe"`
	Valid         bool      `json:"valid"`
}

// Text renders the scan for a person: the counts, the tag universe, and both
// lists. It ends in a newline.
func (r Result) Text() string {
	var tb textbuf.Buffer
	tb.Str("# Test Sensitivity\n\n")
	tb.Str("Test files scanned: ").Int(int64(r.FilesScanned)).Byte('\n')
	tb.Str("Test functions scanned: ").Int(int64(r.TestsScanned)).Byte('\n')
	tb.Str("Test tag universe: ").Join(r.TagUniverse, " ").Str("\n\n")

	tb.Str("## Assert-nothing (").Int(int64(len(r.AssertNothing))).Str(")\n\n")
	for _, finding := range r.AssertNothing {
		tb.Str("  ").Str(finding.File).Byte(':').Int(int64(finding.Line)).Byte(' ').Str(finding.Test).Byte('\n')
	}

	tb.Str("\n## Tag-orphan (").Int(int64(len(r.TagOrphan))).Str(")\n\n")
	for _, finding := range r.TagOrphan {
		tb.Str("  ").Str(finding.File).Byte(':').Int(int64(finding.Line)).
			Str(" requires ").Str(finding.Detail).Byte('\n')
	}
	return tb.String()
}

// Verdict is one ratchet run: the scan, and the floors it was judged against.
//
// It marshals as the SCAN, so `le test-sensitivity check | json` answers the
// same document as the report. The floors are a fact about the judgement rather
// than about the tree, and they are rendered instead.
type Verdict struct {
	Result   Result
	Baseline Baseline
}

// MarshalJSON answers the scan, so one payload shape reaches every caller of
// `| json` whichever action produced it.
func (v Verdict) MarshalJSON() ([]byte, error) { return marshalResult(v.Result) }

// Text renders the one line a passing ratchet prints, and nothing for a failing
// one: a breach is rendered by breach, on stderr, because it is advice rather
// than an answer. It ends in a newline.
func (v Verdict) Text() string {
	if !v.Result.Valid {
		return ""
	}
	var tb textbuf.Buffer
	return tb.Str("test-sensitivity: OK (assert-nothing ").
		Int(int64(len(v.Result.AssertNothing))).Byte('/').Int(int64(v.Baseline.AssertNothing)).
		Str(", tag-orphan ").
		Int(int64(len(v.Result.TagOrphan))).Byte('/').Int(int64(v.Baseline.TagOrphan)).
		Str(", ").Int(int64(v.Result.FilesScanned)).Str(" test files)\n").String()
}

// breach renders what a reader does next about a ratchet that fired, or about a
// floor that has gone slack. It is empty when the counts sit exactly on their
// floors, and it ends in a newline.
func (v Verdict) breach() string {
	var tb textbuf.Buffer
	if len(v.Result.AssertNothing) > v.Baseline.AssertNothing {
		tb.Str("\ntest-sensitivity: assert-nothing count ").Int(int64(len(v.Result.AssertNothing))).
			Str(" exceeds baseline ").Int(int64(v.Baseline.AssertNothing)).Byte('\n')
		tb.Str("  These tests contain no reachable Error/Fatal/Fail call and cannot go red:\n")
		for _, finding := range v.Result.AssertNothing {
			tb.Str("    ").Str(finding.File).Byte(':').Int(int64(finding.Line)).Byte(' ').Str(finding.Test).Byte('\n')
		}
		tb.Str("  Add a real assertion, or annotate the test with a reason:\n")
		tb.Str("    // ").Str(escapeComment).Str(" <why this test cannot assert>\n")
	}
	if len(v.Result.TagOrphan) > v.Baseline.TagOrphan {
		tb.Str("\ntest-sensitivity: tag-orphan count ").Int(int64(len(v.Result.TagOrphan))).
			Str(" exceeds baseline ").Int(int64(v.Baseline.TagOrphan)).Byte('\n')
		tb.Str("  No native test action can build these files with their required tags:\n")
		for _, finding := range v.Result.TagOrphan {
			tb.Str("    ").Str(finding.File).Byte(':').Int(int64(finding.Line)).
				Str(" requires ").Str(finding.Detail).Byte('\n')
		}
		tb.Str("  Add the tag to a native test action, or delete the file.\n")
	}
	if !v.Result.Valid {
		tb.Str("\n  Refresh the floors only when the count went DOWN: ./le test-health update\n")
		return tb.String()
	}

	// A floor that is now slack is reported so it gets lowered in the change
	// that earned it, which is what keeps a ratchet tight.
	if len(v.Result.AssertNothing) < v.Baseline.AssertNothing || len(v.Result.TagOrphan) < v.Baseline.TagOrphan {
		tb.Str("test-sensitivity: baseline is slack (assert-nothing ").
			Int(int64(len(v.Result.AssertNothing))).Byte('<').Int(int64(v.Baseline.AssertNothing)).
			Str(", tag-orphan ").
			Int(int64(len(v.Result.TagOrphan))).Byte('<').Int(int64(v.Baseline.TagOrphan)).
			Str("). Run `./le test-health update` to tighten it.\n")
	}
	return tb.String()
}
