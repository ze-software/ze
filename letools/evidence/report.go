// Design: ai/rules/cli.md -- one payload, and the operator picks the rendering
// Overview: actions.go -- the area table that answers with this
// Overview: evidence.go -- the run that fills this in
//
// report.go is what a release-candidate run answers. It is data, so `| json`,
// `| yaml` and `| table` each render it with no code here, and Text is the
// second reading of the same data for a person who typed no operator.

package evidence

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Report is one release-candidate run.
//
// Dirty is the reason the run refused, when it refused: the paths git named,
// with git's own status prefix kept. Code is the container's exit status, which
// reaches the process exit status unchanged.
type Report struct {
	Image    string   `json:"image"`
	Platform string   `json:"platform"`
	Tree     string   `json:"tree"`
	Dirty    []string `json:"dirty"`
	Passed   bool     `json:"passed"`
	Code     int      `json:"code"`
}

// Text renders the run for a person, in the shape the shell script printed: the
// refusal and its paths, or one line saying what the gate did.
func (r Report) Text() string {
	var tb textbuf.Buffer

	if len(r.Dirty) > 0 {
		tb.Str("error: ").Str(ErrDirtyTree.Error()).Byte('\n')
		tb.Str("commit, remove, or intentionally exclude these paths before running this target:\n")
		for _, line := range r.Dirty {
			tb.Str(line).Byte('\n')
		}
		return tb.String()
	}

	if r.Passed {
		tb.Str("OK: the verify gate passed over a clean clone in ").Str(r.Image).Byte('\n')
		return tb.String()
	}

	tb.Str("FAIL: the verify gate exited ").Int(int64(r.Code)).
		Str(" over a clean clone in ").Str(r.Image).Byte('\n')
	return tb.String()
}
