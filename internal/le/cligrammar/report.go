// Design: docs/architecture/cli/command-namespacing.md -- the grammar gate's answer
//
// report.go holds what `le cli-grammar` ANSWERS, apart from what produced it.
//
// The answer is ONE document holding three row sets -- the grammar findings,
// the flag spellings in YANG, and the dead launch forms -- beside the counts
// that say how much of the tree was read. The counts are not decoration: a run
// that checked 187 commands and one that checked none both report zero
// findings, and only the counts tell them apart.

package cligrammar

import (
	"sort"

	"github.com/ze-software/ze/internal/component/command/grammar"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// FlagHit is a `--flag` spelling in a YANG file, which the command model must
// never carry (R3).
type FlagHit struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// DemoLaunchHit is a `ze ...` invocation in a checked-in demo script whose
// position-1 token is not a command the dispatcher accepts.
type DemoLaunchHit struct {
	File  string `json:"file"`
	Line  int    `json:"line"`
	Token string `json:"token"`
}

// Result is the whole answer of one grammar run. The keys are the script's,
// unchanged.
type Result struct {
	Findings     []grammar.Finding `json:"findings"`
	FlagInYANG   []FlagHit         `json:"flag-in-yang"`
	DemoLaunch   []DemoLaunchHit   `json:"demo-launch"`
	Exempt       map[string]int    `json:"exempt-by-category"`
	Checked      int               `json:"commands-checked"`
	DemoScripts  int               `json:"demo-scripts-checked"`
	RootsChecked int               `json:"roots-checked"`
	RootExempt   int               `json:"root-namespace-exempt"`
	// LeRootsChecked is how many le commands feeder 6 read.
	LeRootsChecked int `json:"le-roots-checked"`
	// LeExempt counts le commands an exemption spared.
	LeExempt     int  `json:"le-namespace-exempt"`
	TreeExempt   int  `json:"tree-namespace-exempt"`
	PendingSplit int  `json:"pending-namespace-split"`
	Valid        bool `json:"valid"`
}

// Text renders the gate for a person: the counts, then a section per row set
// that has rows, then the verdict. It ends in a newline.
func (r Result) Text() string {
	var tb textbuf.Buffer
	tb.Str("# CLI Grammar Gate\n\n")
	tb.Str("Commands checked: ").Int(int64(r.Checked)).Byte('\n')
	tb.Str("Roots checked: ").Int(int64(r.RootsChecked)).Byte('\n')
	tb.Str("le commands checked: ").Int(int64(r.LeRootsChecked)).Byte('\n')
	if r.LeExempt > 0 {
		tb.Str("le namespace exemptions: ").Int(int64(r.LeExempt)).Byte('\n')
	}
	if r.RootExempt > 0 {
		tb.Str("Root namespace-exempt (indivisible compounds): ").Int(int64(r.RootExempt)).Byte('\n')
	}
	if r.TreeExempt > 0 {
		tb.Str("Tree namespace-exempt (indivisible compounds): ").Int(int64(r.TreeExempt)).Byte('\n')
	}

	categories := make([]string, 0, len(r.Exempt))
	for category := range r.Exempt {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	for _, category := range categories {
		tb.Str("Exempt (").Str(category).Str("): ").Int(int64(r.Exempt[category])).Byte('\n')
	}

	if r.PendingSplit > 0 {
		tb.Str("Pending namespace-split (R9 debt, tracked for rename migration): ").
			Int(int64(r.PendingSplit)).Byte('\n')
	}
	tb.Byte('\n')

	if len(r.Findings) > 0 {
		tb.Str("## Grammar violations (").Int(int64(len(r.Findings))).Str(")\n\n")
		for _, finding := range r.Findings {
			tb.Str("  [").Str(finding.Rule).Str("] ").Str(finding.Command).Byte('\n')
			tb.Str("        ").Str(finding.Message).Byte('\n')
		}
		tb.Byte('\n')
	}
	if len(r.FlagInYANG) > 0 {
		tb.Str("## --flag in YANG (").Int(int64(len(r.FlagInYANG))).Str(")\n\n")
		for _, hit := range r.FlagInYANG {
			tb.Str("  ").Str(hit.File).Byte(':').Int(int64(hit.Line)).Str("  ").Str(hit.Text).Byte('\n')
		}
		tb.Byte('\n')
	}
	if len(r.DemoLaunch) > 0 {
		tb.Str("## Dead launch form in demo scripts (").Int(int64(len(r.DemoLaunch))).Str(")\n\n")
		for _, hit := range r.DemoLaunch {
			tb.Str("  ").Str(hit.File).Byte(':').Int(int64(hit.Line)).
				Str("  `ze ").Str(hit.Token).Str("` -- ").Quoted(hit.Token).
				Str(" is not a verb or a registered root\n")
		}
		tb.Byte('\n')
	}

	if r.Valid {
		tb.Str("cli-grammar: OK\n")
		return tb.String()
	}
	tb.Str("cli-grammar: FAILED (").Int(int64(len(r.Findings))).Str(" grammar, ").
		Int(int64(len(r.FlagInYANG))).Str(" flag-in-yang, ").
		Int(int64(len(r.DemoLaunch))).Str(" demo-launch)\n")
	return tb.String()
}
