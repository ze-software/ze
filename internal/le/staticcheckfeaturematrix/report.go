// Design: docs/architecture/testing/tracked-build-gate.md -- the matrix gate's answers
//
// report.go holds what the two actions of `le staticcheck-feature-matrix`
// ANSWER, apart from what produced them.
//
// The matrix answer IS its rows, so it is a slice: `| json` renders the array
// and `| count` says how many combinations this run judges. It renders itself
// in the one form Staticcheck reads on stdin, which is also the form the
// script's --print-matrix printed, so the two can be compared byte for byte.

package staticcheckfeaturematrix

import (
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/verify/failuregroup"
)

// Render answers the matrix in the form Staticcheck reads on stdin: one
// `<name>: -tags=<a>,<b>` line per row, ending in a newline.
//
// The rendering is checked against the rows it came from before it is handed
// back, because Staticcheck is asked about whatever this produces and an
// empty or short document would judge less than the caller believes.
func (m Matrix) Render() ([]byte, error) {
	if err := validateRows(m); err != nil {
		return nil, err
	}

	var rendered strings.Builder
	for _, row := range m {
		rendered.WriteString(row.Name)
		rendered.WriteString(": -tags=")
		rendered.WriteString(strings.Join(row.Tags, ","))
		rendered.WriteByte('\n')
	}

	out := []byte(rendered.String())
	switch {
	case len(out) == 0:
		return nil, fmt.Errorf("rendered matrix is empty")
	case out[len(out)-1] != '\n':
		return nil, fmt.Errorf("rendered matrix is missing its final newline")
	case strings.Count(string(out), "\n") != len(m):
		return nil, fmt.Errorf("rendered matrix has %d lines, want %d rows", strings.Count(string(out), "\n"), len(m))
	}
	return out, nil
}

// Text renders the matrix for a person, which is the same document Staticcheck
// reads. A matrix that will not render answers the reason instead, because a
// caller who typed no pipe operator has nowhere else to see it.
func (m Matrix) Text() string {
	rendered, err := m.Render()
	if err != nil {
		var tb textbuf.Buffer
		return tb.Str("staticcheck feature matrix: ").Err(err).Byte('\n').String()
	}
	return string(rendered)
}

// Verdict is what one Staticcheck run over the matrix said.
type Verdict struct {
	// Rows is how many feature-tag combinations were judged.
	Rows int `json:"rows"`
	// Diagnostics are Staticcheck's own stdout lines, one per entry. A
	// type-check failure names its file, line and column here.
	Diagnostics []string `json:"diagnostics,omitempty"`
	// Tool is whatever Staticcheck wrote to stderr, kept verbatim because it
	// carries the progress and the compile errors a reader needs.
	Tool string `json:"tool,omitempty"`
	// Passed is the verdict: every row type-checked.
	Passed bool `json:"passed"`
}

// Text renders the verdict for a person: Staticcheck's own diagnostics, then
// the row count when every row type-checked. It ends in a newline.
//
// A FAILING run also renders Tool and declares a failure group. Both are here
// rather than on stderr because the verify engine reads a stage's detail log,
// and that log holds what the action RETURNED (internal/le/verify/dispatch,
// dispatch, which hands leroot.Run a capturing writer). A run that failed
// before producing a diagnostic used to render nothing at all, so its stage log
// was empty: the red said neither what broke nor which files it was about, and
// internal/le/commit/verification.go charges a red with no paths to every commit
// in the checkout.
func (v Verdict) Text() string {
	var tb textbuf.Buffer
	for _, line := range v.Diagnostics {
		tb.Str(line).Byte('\n')
	}
	if v.Passed {
		tb.Str("staticcheck feature matrix: checked ").Int(int64(v.Rows)).Str(" rows\n")

		return tb.String()
	}
	if trimmed := strings.TrimSpace(v.Tool); trimmed != "" {
		tb.Str(trimmed).Byte('\n')
	}

	var group strings.Builder
	if err := failuregroup.Declare(&group, "files:staticcheck-feature-matrix/check", "files",
		"staticcheck could not type-check the feature matrix; these files hold the diagnostics",
		"./le staticcheck-feature-matrix check", v.failingPaths()); err == nil {
		tb.Str(group.String())
	}

	return tb.String()
}

// failingPaths answers the source files the run's own output named, so a red
// here is charged to the commits that touch them. A failure that names no file,
// such as a broken vendor tree, answers nothing and stays charged to everyone,
// which is the honest answer rather than a wrong attribution.
func (v Verdict) failingPaths() []string {
	paths := failuregroup.Paths(strings.Join(v.Diagnostics, "\n"))

	return failuregroup.Merge(paths, failuregroup.Paths(v.Tool))
}
