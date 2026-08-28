// Design: docs/architecture/web-interface.md -- what the web-assets commands answer
//
// report.go holds what the three actions ANSWER, apart from what produced it.
//
// The script printed nothing on a passing check and one line on a failing one,
// and it printed that line to stderr because it modeled staleness as an error.
// A verdict is DATA here, so it goes to the payload and the payload renders to
// stdout: `le web-assets check | json` then carries the verdict a script can
// read. That stream change is the one deliberate difference in this port, and
// internal/le/parity/parity_test.go compares the two streams together.

package webassets

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// PageSets is what `le web-assets pages` answers: each page, keyed by the
// source that renders its head, and the vendored files it must load.
//
// It is a map rather than a struct so `| json` answers the same object the
// script's --json printed, which is what internal/component/web reads back.
type PageSets map[string][]string

// CheckReport is what `le web-assets check` answers.
type CheckReport struct {
	// Derived names every generated file the run compared, sorted. It is the
	// answer's only row set, and it says the check covered all three surfaces
	// rather than stopping early for some other reason.
	Derived []string `json:"derived"`
	// Stale is the FIRST generated file that disagrees with the markup its
	// package renders, empty when they all agree.
	Stale string `json:"stale,omitempty"`
}

// Text renders the native verdict, and renders nothing when every file agrees.
func (r CheckReport) Text() string {
	if r.Stale == "" {
		return ""
	}

	var tb textbuf.Buffer

	return tb.Str("web_assets: ").Str(r.Stale).
		Str(" is stale: it disagrees with the markup its package renders; run: ./le web-assets write\n").String()
}

// WriteReport is what `le web-assets write` answers.
type WriteReport struct {
	// Derived names every generated file the run produced, sorted.
	Derived []string `json:"derived"`
	// Written names the ones whose bytes changed. A file that already agreed is
	// absent, because it was not written.
	Written []string `json:"written"`
}

// Text renders nothing, which is what the script printed on a successful write.
func (r WriteReport) Text() string { return "" }
