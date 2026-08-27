// Design: docs/architecture/core-design.md -- what the perf nudge answers
// Overview: perfbench.go -- what produces this
//
// report.go holds what `le perf-bench` ANSWERS, apart from what produced it.
//
// The payload exposes four facts that a reader must not infer.
// It gives the comparison commit, its source, the uncovered files, and any checkout read error.
// One key contains rows, so row operators act on the files.

package perfbench

import "github.com/ze-software/ze/internal/core/textbuf"

// Report is the whole answer of one run, for both verbs of this area.
type Report struct {
	// Baseline is the commit committed changes are measured against, empty
	// when no trusted point exists.
	Baseline string `json:"baseline,omitempty"`
	// Origin says where Baseline came from. A reader needs it: the same SHA
	// means "perf ran here" from the marker and "never perf-tested" from a
	// merge-base, and the nudge's own wording differs on it.
	Origin Origin `json:"origin,omitempty"`
	// Uncovered names the hot-path files changed since Baseline.
	Uncovered []string `json:"uncovered,omitempty"`
	// Recorded is the SHA the record verb wrote, and is empty for a nudge.
	Recorded string `json:"recorded,omitempty"`
	// Error explains why the checkout was unreadable.
	// The advisory still exits 0 because it must not block a build.
	// This field prevents the command from hiding that failure.
	Error string `json:"error,omitempty"`
}

// Text renders the report in the script's wording and ends with a newline.
// It returns an empty string when the nudge has nothing to report.
//
// This is the Prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it.
func (r Report) Text() string {
	var tb textbuf.Buffer

	switch {
	case r.Error != "":
		return tb.Str("perf-suggest: ").Str(r.Error).Byte('\n').String()
	case r.Recorded != "":
		return tb.Str("perf-suggest: recorded ").Str(r.Recorded).Byte('\n').String()
	case len(r.Uncovered) == 0:
		return ""
	}

	tb.Byte('\n').Str("perf-suggest: BGP data-plane code changed vs ").Str(r.originPhrase()).
		Str(". Consider a perf run before relying on these:\n")

	named := r.Uncovered
	unnamed := 0
	if len(named) > namedFiles {
		unnamed = len(named) - namedFiles
		named = named[:namedFiles]
	}
	for _, path := range named {
		tb.Str("  ").Str(path).Byte('\n')
	}
	if unnamed > 0 {
		tb.Str("  ... and ").Int(int64(unnamed)).Str(" more\n")
	}

	tb.Str("  Run:  make ze-evidence-perf-record   (Docker; records the baseline so this clears)\n")
	tb.Str("  This is advisory -- it never blocks a build.\n")
	return tb.String()
}

// originPhrase names where the baseline came from, in the words the message
// needs.
func (r Report) originPhrase() string {
	var tb textbuf.Buffer
	short := r.Baseline
	if len(short) > shortSHA {
		short = short[:shortSHA]
	}
	switch r.Origin {
	case OriginLastRun:
		return tb.Str("last perf run ").Str(short).String()
	case OriginMergeBase:
		return tb.Str("branch merge-base ").Str(short).Str(" (perf never recorded here)").String()
	case OriginWorkingTree:
		return "working tree (perf never recorded here)"
	}
	return string(r.Origin)
}
