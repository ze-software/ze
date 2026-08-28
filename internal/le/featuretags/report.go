// Design: ai/rules/plugins.md -- what the feature-tags commands answer
//
// report.go holds what the two actions ANSWER, apart from what produced it.
//
// Each answer carries one row set, so `| json` feeds a script, `| match
// golangci` keeps one file and `| count` says how many are stale. Each also
// renders ITSELF as a native actionable verdict (internal/le/leroot, Prose).

package featuretags

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// CheckReport is what `le feature-tags check` answers: every derived file whose
// tag list disagrees with feature-gates.txt.
type CheckReport struct {
	// Stale names every file that must be rewritten, relative to the tree, in
	// the order the table declares them. It is the answer's only row set, and
	// an empty one is the pass.
	Stale []string `json:"stale"`
}

// Text renders the native check verdict. It ends in a newline.
func (r CheckReport) Text() string {
	var tb textbuf.Buffer

	if len(r.Stale) == 0 {
		return tb.Str("feature-tag lists are current (").Str(derivedFiles()).Str(")\n").String()
	}

	for _, file := range r.Stale {
		tb.Str(file).Str(" is stale; run ./le feature-tags write\n")
	}

	return tb.String()
}

// WriteReport is what `le feature-tags write` answers: every derived file whose
// bytes this run changed.
type WriteReport struct {
	// Updated names every file this run rewrote, relative to the tree. A file
	// whose tag list already agreed is absent, because it was not written.
	Updated []string `json:"updated"`
}

// Text renders the native write verdict. It ends in a newline.
func (r WriteReport) Text() string {
	var tb textbuf.Buffer

	if len(r.Updated) == 0 {
		return tb.Str("feature-tag lists already current (").Str(derivedFiles()).Str(")\n").String()
	}

	for _, file := range r.Updated {
		tb.Str("updated ").Str(file).Byte('\n')
	}

	return tb.String()
}
