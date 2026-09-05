// Design: docs/architecture/core-design.md -- the Go-version gate's answer
// Overview: goversion.go -- what produces these rows
//
// report.go holds what `le go-version check` ANSWERS, apart from what produced
// it.
//
// The answer is the finding list, the excluded list and the count of carriers
// compared, which is structured data: `| json` feeds a script and
// `| match go-minor-mismatch` keeps one reason. The report also renders ITSELF
// (Text), because a person reading a failing gate wants the page rather than a
// table (internal/le/leroot, Prose).

package goversion

import "github.com/ze-software/ze/internal/core/textbuf"

// Finding reason codes. They are stable and machine-readable, and they are what
// `| match` selects on.
const (
	// ReasonMismatch: the carrier names a Go minor other than the declared one.
	ReasonMismatch = "go-minor-mismatch"
	// ReasonUnreadableTag: a golang image whose tag carries no <major>.<minor>,
	// such as latest, alpine, or an unexpanded build argument.
	ReasonUnreadableTag = "unreadable-image-tag"
	// ReasonUnreadableBase: a stage that copies this module in and does not
	// build on a golang image, so its Go version is somewhere this gate cannot
	// read.
	ReasonUnreadableBase = "unreadable-build-base"
)

// ExcludedNoModuleCopy is the one derived reason a golang stage is not judged:
// it copies no module in, so it builds other sources and carries a Go minimum
// of its own.
const ExcludedNoModuleCopy = "builds-other-sources"

// Finding is one carrier that does not agree with go.mod, and it is one ROW of
// the answer.
type Finding struct {
	Carrier  string `json:"carrier"`
	Line     int    `json:"line"`
	Names    string `json:"names"`
	Declared string `json:"declared"`
	Reason   string `json:"reason"`
}

// Excluded is one golang stage the gate read and did not judge, with the reason
// it derived. It is reported rather than dropped, so a reader can check the
// derivation against the file instead of taking the silence on trust.
type Excluded struct {
	Carrier string `json:"carrier"`
	Line    int    `json:"line"`
	Names   string `json:"names"`
	Reason  string `json:"reason"`
}

// Result is the whole answer of one run.
//
// Carriers is what tells a clean tree from a walk that read nothing, which is
// why Check refuses to answer a Result whose count is zero.
type Result struct {
	Declared string     `json:"declared-minor"`
	Findings []Finding  `json:"findings,omitempty"`
	Excluded []Excluded `json:"excluded,omitempty"`
	Carriers int        `json:"carriers-checked"`
	Valid    bool       `json:"valid"`
}

// Text renders the result for a person: the heading, what go.mod declares, the
// count, then the findings or the verdict. It ends in a newline.
func (r Result) Text() string {
	var tb textbuf.Buffer
	tb.Str("# Go Version Gate\n\n")
	tb.Str("go.mod declares: ").Str(r.Declared).Str("\n")
	tb.Str("Carriers checked: ").Int(int64(r.Carriers)).Str("\n\n")

	// The excluded rows print on a green run too. A gate that walks part of the
	// set it claims prints what a clean tree prints, so the reader who wants to
	// check the derivation needs it where the gate passed.
	if len(r.Excluded) != 0 {
		tb.Str("## Not judged (").Int(int64(len(r.Excluded))).Str(")\n\n")
		for _, excluded := range r.Excluded {
			tb.Str("  [").Str(excluded.Reason).Str("] ").Str(excluded.Carrier).
				Byte(':').Int(int64(excluded.Line)).
				Str(" names ").Str(excluded.Names).Byte('\n')
		}
		tb.Byte('\n')
	}

	if len(r.Findings) == 0 {
		tb.Str("go-version: OK\n")
		return tb.String()
	}

	tb.Str("## Findings (").Int(int64(len(r.Findings))).Str(")\n\n")
	for _, finding := range r.Findings {
		tb.Str("  [").Str(finding.Reason).Str("] ").Str(finding.Carrier).
			Byte(':').Int(int64(finding.Line)).
			Str(" names ").Str(finding.Names).
			Str(", go.mod declares ").Str(finding.Declared).Byte('\n')
	}
	tb.Str("\nFix: name the ").Str(r.Declared).
		Str(" image in each carrier above, or move the go directive in go.mod.\n")
	tb.Str("\ngo-version: FAILED\n")
	return tb.String()
}
