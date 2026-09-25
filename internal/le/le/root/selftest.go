// Design: docs/architecture/core-design.md -- what a gate says about ITSELF
//
// selftest.go is the answer shape of a gate that checks its own detection
// against fixtures, which eight of the ported gates do.
//
// The case those gates exist for is the one their own output cannot show: a
// gate over a clean tree and a gate whose detection broke print the same page.
// The selftest runs fixtures that MUST be flagged and fixtures that must not,
// so the silence over the real tree means something.
//
// The shape is stated once because eight tools carry it, and because the
// scripts each spelled it differently: some returned a bool, some a list of
// message strings, and none of them said which fixture failed in a form a
// caller could read. Here every run answers one ROW per fixture, passed or
// failed, and `| json` renders that array with no per-tool code.
package leroot

import (
	"encoding/json"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// SelftestResult is one fixture's outcome, and it is one ROW of a selftest
// answer.
type SelftestResult struct {
	// Case names the fixture, which is what a reader looks up when it fails.
	Case string `json:"case"`
	// Passed is the verdict for that fixture.
	Passed bool `json:"passed"`
	// Detail says what the failure means, and is absent when the case passed.
	Detail string `json:"detail,omitempty"`
}

// SelftestReport is the whole answer of one selftest run.
//
// The two verdict lines are unexported and are NOT part of the payload: they
// are how one tool's page reads, and each script printed its own wording. The
// data is the rows, which is why MarshalJSON answers the array alone -- a
// caller of `| json` gets the same document whichever gate produced it, and
// `| count` acts on the cases.
type SelftestReport struct {
	verdictOK     string
	verdictFailed string
	Results       []SelftestResult
}

// NewSelftestReport declares a report and the two lines its page ends or opens
// with. okLine is printed when every case passed; failedLine heads the list of
// the ones that did not.
func NewSelftestReport(okLine, failedLine string, results ...SelftestResult) SelftestReport {
	// Both panics are reachable only from a Ze defect at the call site -- a
	// report that would render a blank verdict -- never from anything an
	// operator typed.
	if okLine == "" || failedLine == "" {
		panic("BUG: leroot.NewSelftestReport: a report needs both verdict lines")
	}
	return SelftestReport{verdictOK: okLine, verdictFailed: failedLine, Results: results}
}

// Pass answers a result for a fixture that behaved as it declared.
func Pass(name string) SelftestResult { return SelftestResult{Case: name, Passed: true} }

// Fail answers a result for a fixture that did not, carrying what the failure
// means.
func Fail(name, detail string) SelftestResult {
	return SelftestResult{Case: name, Detail: detail}
}

// Failures answers the cases that did not pass.
func (r SelftestReport) Failures() []SelftestResult {
	var failed []SelftestResult
	for _, result := range r.Results {
		if !result.Passed {
			failed = append(failed, result)
		}
	}
	return failed
}

// Code answers the exit code the report earns: 0 when every fixture behaved as
// declared, and code otherwise. The failing code is the caller's because the
// scripts disagree about it, and a caller that reads 1 apart from 2 keeps
// reading them apart.
func (r SelftestReport) Code(failing int) int {
	if len(r.Failures()) == 0 {
		return 0
	}
	return failing
}

// MarshalJSON answers the RESULTS, so the payload is the array of rows rather
// than an object wrapping it. The verdict lines are a rendering, not data.
func (r SelftestReport) MarshalJSON() ([]byte, error) { return json.Marshal(r.Results) }

// Text renders the selftest for a person: the failures if there are any, and
// the one-line verdict otherwise. It ends in a newline.
func (r SelftestReport) Text() string {
	var tb textbuf.Buffer
	failed := r.Failures()
	if len(failed) == 0 {
		return tb.Str(r.verdictOK).Byte('\n').String()
	}

	tb.Str(r.verdictFailed).Byte('\n')
	for _, result := range failed {
		tb.Str("  ").Str(result.Detail).Byte('\n')
	}
	return tb.String()
}
