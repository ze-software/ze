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

// FlagRegisterHit is one flag-register violation (F1 to F4) and the source that
// carries it. A root registration has no site, because the root feeder reads
// names rather than call sites.
//
// The finding's fields are repeated rather than embedded, so every key this
// payload publishes is kebab-case (ai/rules/cli.md). An embedded
// grammar.FlagFinding carries no tags, and encoding/json would flatten it into
// Command, Rule, Flag and Message.
type FlagRegisterHit struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Command string `json:"command"`
	Rule    string `json:"rule"`
	Flag    string `json:"flag"`
	Message string `json:"message"`
}

// flagHit pairs one finding with the source that carries it.
func flagHit(finding grammar.FlagFinding, file string, line int) FlagRegisterHit {
	return FlagRegisterHit{
		File: file, Line: line,
		Command: finding.Command, Rule: finding.Rule,
		Flag: finding.Flag, Message: finding.Message,
	}
}

// FlagDebt is one entry of the tracked flag-register debt: what it forgives,
// why it is still here, and whether the violation is still in the tree.
//
// Present false says the fix landed and the entry can go. It is reported rather
// than failed, so a fix landing in this shared checkout never turns the gate
// red for the session that did not write it.
type FlagDebt struct {
	Entry string `json:"entry"`
	// Reason is why the violation is still here, and what removing it needs.
	Reason string `json:"reason"`
	// Tracked is how many violations the entry forgives.
	Tracked int `json:"tracked"`
	// Present is how many of them the tree still carries. Zero says the fix
	// landed and the entry can go.
	Present int `json:"present"`
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
	LeExempt     int `json:"le-namespace-exempt"`
	TreeExempt   int `json:"tree-namespace-exempt"`
	PendingSplit int `json:"pending-namespace-split"`
	// FlagFindings are feeder 7's open violations, the debt list apart.
	FlagFindings []FlagRegisterHit `json:"flag-findings"`
	// FlagDebt is the tracked debt, printed so nothing is silently forgiven.
	FlagDebt []FlagDebt `json:"flag-debt"`
	// GoFilesRead is how many Go sources feeder 7 parsed.
	GoFilesRead int `json:"go-files-read"`
	// FlagSetsRead is how many flag.NewFlagSet call sites it resolved.
	FlagSetsRead int `json:"flag-sets-read"`
	// FlagSetsInScope is how many of them are `ze` offline commands, which are
	// the ones that owe a registry declaration.
	FlagSetsInScope int `json:"flag-sets-in-scope"`
	// FlagSetsOutOfScope is how many belong to another binary (ze-test,
	// ze-perf, ze-chaos, the mock servers), which has no completion surface.
	FlagSetsOutOfScope int `json:"flag-sets-out-of-scope"`
	// FlagNamesUnresolved counts flag declarations whose name is not a literal,
	// so no feeder judged them.
	FlagNamesUnresolved int `json:"flag-names-unresolved"`
	// FlagSetNamesUnresolved counts flag sets whose own name is not a literal.
	FlagSetNamesUnresolved int `json:"flag-set-names-unresolved"`
	// FlagsUnattributed counts flag declarations on a set the enclosing
	// function did not build, so no command owns them here.
	FlagsUnattributed int `json:"flag-declarations-unattributed"`
	// ClientLiteralsServedLocally counts command strings carrying a flag whose
	// path command.ServeLocal answers in-process, which the daemon's dispatcher
	// therefore never sees.
	ClientLiteralsServedLocally int  `json:"client-literals-served-locally"`
	Valid                       bool `json:"valid"`
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

	tb.Str("Go sources parsed: ").Int(int64(r.GoFilesRead)).Byte('\n')
	tb.Str("Flag sets checked: ").Int(int64(r.FlagSetsInScope)).
		Str(" of ").Int(int64(r.FlagSetsRead)).
		Str(" (").Int(int64(r.FlagSetsOutOfScope)).Str(" outside the ze command surface)\n")
	if r.ClientLiteralsServedLocally > 0 {
		tb.Str("Command strings with a flag whose path is served in-process: ").
			Int(int64(r.ClientLiteralsServedLocally)).Byte('\n')
	}
	if r.FlagSetNamesUnresolved > 0 || r.FlagNamesUnresolved > 0 || r.FlagsUnattributed > 0 {
		tb.Str("Flag declarations no static scan can place: ").
			Int(int64(r.FlagSetNamesUnresolved)).Str(" set name(s), ").
			Int(int64(r.FlagNamesUnresolved)).Str(" flag name(s), ").
			Int(int64(r.FlagsUnattributed)).Str(" on a set built elsewhere\n")
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
	if len(r.FlagFindings) > 0 {
		tb.Str("## Flag in the wrong register (").Int(int64(len(r.FlagFindings))).Str(")\n\n")
		for _, hit := range r.FlagFindings {
			tb.Str("  [").Str(hit.Rule).Str("] ").Str(hit.Command)
			if hit.File != "" {
				tb.Str("  (").Str(hit.File).Byte(':').Int(int64(hit.Line)).Byte(')')
			}
			tb.Byte('\n')
			tb.Str("        ").Str(hit.Message).Byte('\n')
		}
		tb.Byte('\n')
	}
	if len(r.FlagDebt) > 0 {
		tb.Str("## Tracked flag-register debt (").Int(int64(len(r.FlagDebt))).Str(")\n\n").
			Str("The reason each entry is still here is in flagRegisterDebt and\n").
			Str("flagDeclarationDebt (internal/le/cligrammar/flags.go), beside the fix.\n\n")
		for _, entry := range r.FlagDebt {
			tb.Str("  ").Str(entry.Entry)
			switch {
			case entry.Present == 0:
				tb.Str("  -- FIXED, delete this entry")
			case entry.Present < entry.Tracked:
				tb.Str("  -- ").Int(int64(entry.Tracked - entry.Present)).
					Str(" of ").Int(int64(entry.Tracked)).Str(" fixed, trim this entry")
			case entry.Tracked > 1:
				tb.Str("  -- ").Int(int64(entry.Tracked)).Str(" flags")
			}
			tb.Byte('\n')
			// The reason is printed only for an entry that needs action. It is
			// a sentence per entry, and fifty of them would bury the two rows a
			// reader has to act on.
			if entry.Present < entry.Tracked {
				tb.Str("        ").Str(entry.Reason).Byte('\n')
			}
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
		Int(int64(len(r.DemoLaunch))).Str(" demo-launch, ").
		Int(int64(len(r.FlagFindings))).Str(" flag-register)\n")
	return tb.String()
}
