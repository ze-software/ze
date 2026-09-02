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

// Names answers the row names in the order Staticcheck is asked about them, so
// a reader of ONE piece's log can tell which combinations that piece covered.
func (m Matrix) Names() []string {
	names := make([]string, 0, len(m))
	for _, row := range m {
		names = append(names, row.Name)
	}
	return names
}

// Verdict is what one Staticcheck run over the matrix said.
type Verdict struct {
	// Part and Parts name the piece of the matrix this run judged. A run that
	// was not cut is part 1 of 1, so neither field is ever a zero standing for
	// "nobody set this".
	Part  int `json:"part"`
	Parts int `json:"parts"`
	// Rows is how many feature-tag combinations were judged.
	Rows int `json:"rows"`
	// Names are the combinations this run judged, one per judged row.
	Names []string `json:"names,omitempty"`
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
		tb.Str("staticcheck feature matrix: part ").Int(int64(v.Part)).Str(" of ").Int(int64(v.Parts))
		if v.Rows == 0 {
			// A piece with no row is not a piece that judged nothing by
			// accident: the matrix in hand holds fewer rows than the run was
			// cut into, and every row it does hold is judged by another piece.
			return tb.Str(" was dealt no row; the matrix is smaller than the number of parts\n").String()
		}
		tb.Str(" checked ").Int(int64(v.Rows)).Str(" row(s): ").Join(v.Names, ", ").Byte('\n')

		return tb.String()
	}
	// A red is read one piece at a time, in that shard's log alone, so the scope
	// of the run that produced it is stated beside the diagnostics.
	tb.Str("staticcheck feature matrix: part ").Int(int64(v.Part)).Str(" of ").Int(int64(v.Parts)).
		Str(" judged ").Int(int64(v.Rows)).Str(" row(s): ").Join(v.Names, ", ").Byte('\n')
	if trimmed := strings.TrimSpace(v.Tool); trimmed != "" {
		tb.Str(trimmed).Byte('\n')
	}

	var group strings.Builder
	if err := failuregroup.Declare(&group, "files:"+v.stage(), "files",
		"staticcheck could not type-check the feature matrix; these files hold the diagnostics",
		v.rerun(), v.failingPaths()); err == nil {
		tb.Str(group.String())
	}

	return tb.String()
}

// stage answers the identity of the run that produced this verdict, which is the
// stage name the verify population gave the piece. Each piece declares its own
// group, so a red is charged to the piece that found it and never to a sibling.
func (v Verdict) stage() string {
	var tb textbuf.Buffer
	return tb.Str("staticcheck-feature-matrix/check/part/").Int(int64(v.Part)).
		Str("/of/").Int(int64(v.Parts)).String()
}

// rerun answers the command that judges this piece again, and nothing else. A
// reader of one shard's log has that piece's rows and no other.
func (v Verdict) rerun() string {
	var tb textbuf.Buffer
	return tb.Str("./le staticcheck-feature-matrix check part ").Int(int64(v.Part)).
		Str(" of ").Int(int64(v.Parts)).String()
}

// failingPaths answers the source files the run's own output named, so a red
// here is charged to the commits that touch them. A failure that names no file,
// such as a broken vendor tree, answers nothing and stays charged to everyone,
// which is the honest answer rather than a wrong attribution.
func (v Verdict) failingPaths() []string {
	paths := failuregroup.Paths(strings.Join(v.Diagnostics, "\n"))

	return failuregroup.Merge(paths, failuregroup.Paths(v.Tool))
}
