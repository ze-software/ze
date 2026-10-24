// Design: docs/architecture/plugin/feature-gates.md -- what the gated-table commands answer
//
// report.go holds what the two actions ANSWER, apart from what produced it.
// Each answer carries one row set, the gated plugins, so `| json` feeds a
// script and `| count` says how many the run found.

package gatedtable

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The two reasons the table has to be rewritten.
const (
	// reasonStale is a table whose bytes disagree with the manifest and the
	// registrations.
	reasonStale = "is stale"
	// reasonMissing is a table that was never generated.
	reasonMissing = "is missing"
)

// CheckReport is what `le plugin gated-table check` answers.
type CheckReport struct {
	// File is the generated table, relative to the tree.
	File string `json:"file"`
	// Rows is every gated plugin the table owes, sorted by name.
	Rows []Row `json:"rows"`
	// Reason says why the table has to be rewritten, empty when it is current.
	Reason string `json:"reason,omitempty"`
}

// Text renders the native check verdict: what is wrong, why it matters, and
// the command that fixes it. It ends in a newline.
func (r CheckReport) Text() string {
	var tb textbuf.Buffer

	if r.Reason != "" {
		return tb.Str("./le plugin gated-table check: ").Str(r.File).Byte(' ').Str(r.Reason).
			Str(": a build without a plugin would name the wrong build tag for it, or none.\n").
			Str("Fix: ./le plugin gated-table write (or ./le repo generate)\n").String()
	}

	return tb.Str(r.File).Str(" is current (").Int(int64(len(r.Rows))).Str(" gated plugins)\n").String()
}

// WriteReport is what `le plugin gated-table write` answers.
type WriteReport struct {
	// File is the generated table, relative to the tree.
	File string `json:"file"`
	// Rows is every gated plugin the table now holds, sorted by name.
	Rows []Row `json:"rows"`
	// Written says whether this run changed the file's bytes.
	Written bool `json:"written"`
}

// Text renders the native write verdict. It ends in a newline.
func (r WriteReport) Text() string {
	var tb textbuf.Buffer

	if r.Written {
		tb.Str("Generated ")
	} else {
		tb.Str("Unchanged ")
	}

	return tb.Str(r.File).Str(" with ").Int(int64(len(r.Rows))).Str(" gated plugins\n").String()
}
