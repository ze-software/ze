// Design: docs/architecture/command-ownership.md -- what the plugin-imports commands answer
//
// report.go holds what the two actions ANSWER, apart from what produced it.
//
// Each answer carries one row set -- the generated files -- so `| json` feeds a
// script and `| count` says how many the run compared. Each also renders ITSELF
// in the words the script printed, because the script printed a verdict rather
// than a table (letools/leroot, Prose).
//
// One deliberate change: a file is named RELATIVE to the tree, where the script
// printed the absolute path it had joined. An absolute path names this machine's
// checkout as much as it names the file, and it would leave `| json` and the
// default rendering disagreeing about what the value is (ai/rules/cli.md).
// scripts/codegen/parity_test.go normalizes it.

package pluginimports

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The three reasons a generated file has to be rewritten. Each is the phrase
// the script printed, so the verdict a developer reads is unchanged.
const (
	// reasonStale is a file whose bytes disagree with what the tree registers.
	reasonStale = "is stale"
	// reasonMissing is a gated group whose file was never generated.
	reasonMissing = "missing"
	// reasonUngated is a generated file left behind by a gate that is gone.
	reasonUngated = "is stale (no longer gated)"
)

// Counts is what a run read out of the tree, and it is the number the verdict
// carries. A run that found none of something has proven nothing about it,
// which is why the count is published rather than implied.
type Counts struct {
	Plugins     int `json:"plugins"`
	Schemas     int `json:"schemas"`
	RPCs        int `json:"rpcs"`
	Namespaces  int `json:"namespaces"`
	GatedGroups int `json:"gated-groups"`
}

// CheckReport is what `le plugin-imports check` answers.
type CheckReport struct {
	Counts
	// Files names every generated file the run compared, relative to the tree,
	// in the order it read them. It is the answer's only row set.
	Files []string `json:"files"`
	// Stale is the FIRST file that has to be rewritten, empty when they all
	// agree with the tree.
	Stale string `json:"stale,omitempty"`
	// Reason says why that file has to be rewritten, in the script's words.
	Reason string `json:"reason,omitempty"`
}

// Text renders the verdict in the words the script printed. It ends in a
// newline.
func (r CheckReport) Text() string {
	var tb textbuf.Buffer

	if r.Stale != "" {
		return tb.Str("plugin_imports: ").Str(r.Stale).Byte(' ').Str(r.Reason).
			Str("; run make generate\n").String()
	}

	return tb.Str(allFile).Str(" is current (").Str(r.Counts.text()).Str(")\n").String()
}

// WriteReport is what `le plugin-imports write` answers.
type WriteReport struct {
	Counts
	// Written names every generated file whose bytes this run changed, relative
	// to the tree. A file that already agreed is absent, because it was not
	// written.
	Written []string `json:"written"`
	// Removed names every generated tag file this run deleted, because no gate
	// needs it any more.
	Removed []string `json:"removed"`
}

// Text renders the verdict in the words the script printed. It ends in a
// newline.
func (r WriteReport) Text() string {
	var tb textbuf.Buffer

	return tb.Str("Generated ").Str(allFile).Str(" with ").Str(r.Counts.text()).Byte('\n').String()
}

// text renders the counts in the order and wording both verdicts carry.
func (c Counts) text() string {
	var tb textbuf.Buffer

	return tb.Int(int64(c.Plugins)).Str(" plugins, ").
		Int(int64(c.Schemas)).Str(" schemas, ").
		Int(int64(c.RPCs)).Str(" rpcs, ").
		Int(int64(c.Namespaces)).Str(" namespaces, ").
		Int(int64(c.GatedGroups)).Str(" gated groups").String()
}
