// Design: docs/architecture/core-design.md -- the usage gate
// Overview: contract.go -- the other gate that walks the YANG command tree
// Related: report.go -- the answers this command renders
//
// usage.go answers one question about every operational command: does the model
// state its argument grammar, or does a description spell it out in prose?
//
// A description states what a command MEANS. It must not prescribe a CLI
// spelling (ai/rules/cli.md), so a "Usage:" sentence inside one is a violation
// this gate names, command by command, until none is left.
//
// The gate prints the generated line beside the authored one, so the difference
// count is the work still owed by the model rather than a judgement anyone
// records by hand.

package docvalid

import (
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/lepath"
)

// usageMarkers are the words a description opens a CLI grammar with.
//
// Three rather than one, because the word in front of the grammar is the
// cheapest thing to change: `show system sockets` writes "Filters: [tcp|udp]
// [state <STATE>] [port <N>]" and the `bgp rib` RPCs write "Syntax: show bgp
// rib [scope] [filters...]". Each states an invocation form, which is what
// ai/rules/cli.md refuses a description, whatever it is called.
//
// "Example:" is deliberately absent. `ze-fib-p4-conf.yang` writes "Example:
// 127.0.0.1:9559" to say what a listener ADDRESS looks like, which prescribes
// no CLI spelling.
var usageMarkers = [...]string{"Usage:", "Syntax:", "Filters:"}

// usageRow is one command node's usage line: the CLI path it belongs to, the
// line the model renders, the sentence a description spells by hand, and the
// word that opened it.
//
// The three usage types stay package-private while 80 descriptions still carry
// authored prose. Exporting them is what wires this gate into `./le verify`
// (internal/le/doc/wiring/docverify.go, beside Drift and Validate), and a gate
// wired before the prose is gone turns every session's verify red.
type usageRow struct {
	Path      string `json:"path"`
	Generated string `json:"generated"`
	Authored  string `json:"authored"`
	Marker    string `json:"marker"`
}

// usageReport is the whole answer of one `le docvalid usage-contract` run.
//
// Prose names every description that prescribes a CLI spelling. Differ names
// the subset whose authored sentence and generated line disagree, which is the
// count the model has to close.
type usageReport struct {
	Commands int        `json:"commands"`
	Prose    []usageRow `json:"prose"`
	Differ   []usageRow `json:"differ"`
	Valid    bool       `json:"valid"`
}

// usageContract walks the command tree the loader holds and reports every
// description that prescribes a CLI spelling, with the count of command nodes
// the tree carries.
//
// The tree is the parameter rather than the checkout, so a test names a fixture
// module by building a loader over it (contract.go, Validate, takes the same
// shape for the same reason).
func usageContract(loader *yang.Loader) usageReport {
	report := usageReport{Prose: []usageRow{}, Differ: []usageRow{}}
	collectUsage(yang.BuildCommandTree(loader), nil, &report)
	sort.Slice(report.Prose, func(i, j int) bool { return report.Prose[i].Path < report.Prose[j].Path })
	sort.Slice(report.Differ, func(i, j int) bool { return report.Differ[i].Path < report.Differ[j].Path })
	report.Valid = len(report.Prose) == 0
	return report
}

// collectUsage walks one node's children in name order, counting the command
// nodes and collecting the authored sentences.
//
// The recursion is over the command tree, which this process built from its own
// embedded modules. No peer chooses its depth (docs/contributing/ze-go-style.md).
func collectUsage(node *command.Node, path []string, report *usageReport) {
	if node == nil {
		return
	}
	names := make([]string, 0, len(node.Children))
	for name := range node.Children {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		child := node.Children[name]
		childPath := append(path, name) //nolint:gocritic // childPath is consumed before the next iteration reuses the array
		if child.WireMethod != "" {
			report.Commands++
		}
		if marker, authored := authoredUsage(child.Description); authored != "" {
			var tb textbuf.Buffer
			row := usageRow{
				Path:      tb.Join(childPath, " ").String(),
				Generated: command.UsageLine(command.Usage(childPath, child)),
				Authored:  authored,
				Marker:    marker,
			}
			report.Prose = append(report.Prose, row)
			if row.Generated != row.Authored {
				report.Differ = append(report.Differ, row)
			}
		}
		collectUsage(child, childPath, report)
	}
}

// authoredUsage returns the marker that opened a hand-spelled CLI grammar and
// the sentence that follows it, with its line wrapping folded to single spaces.
// Both are "" when the description states meaning only.
//
// A description carrying two markers is reported under the one it writes
// FIRST. Reporting it under each would say there are two violations where
// there is one description to fix.
//
// The sentence runs to the first period that CLOSES it: one at the end of the
// description, or one followed by whitespace. A period inside an address or a
// version number is followed by a digit and does not end the sentence.
func authoredUsage(description string) (marker, authored string) {
	rest := ""
	opensAt := len(description)
	for _, candidate := range usageMarkers {
		at := strings.Index(description, candidate)
		if at < 0 || at >= opensAt {
			continue
		}
		opensAt = at
		marker = candidate
		rest = description[at+len(candidate):]
	}
	if marker == "" {
		return "", ""
	}

	for i := range len(rest) {
		if rest[i] != '.' {
			continue
		}
		if i+1 == len(rest) || rest[i+1] == ' ' || rest[i+1] == '\n' || rest[i+1] == '\t' {
			rest = rest[:i]
			break
		}
	}

	var tb textbuf.Buffer
	return marker, tb.Join(strings.Fields(rest), " ").String()
}

// Text renders the usage report: the command count, one line per authored
// sentence, and the verdict. It ends in a newline.
func (r usageReport) Text() string {
	var tb textbuf.Buffer

	tb.Str("# Command Usage\n\n")
	tb.Str("Command nodes: ").Int(int64(r.Commands)).Byte('\n')
	tb.Str("Authored usage sentences: ").Int(int64(len(r.Prose))).Byte('\n')
	tb.Str("Authored and generated disagree: ").Int(int64(len(r.Differ))).Str("\n\n")

	if len(r.Prose) > 0 {
		tb.Str("## Descriptions that prescribe a CLI spelling (").Int(int64(len(r.Prose))).Str(")\n\n")
		for _, row := range r.Prose {
			tb.Str("  ").Str(row.Path).Str("\n    generated: ").Str(row.Generated).
				Str("\n    authored:  ").Str(row.Authored).Byte('\n')
		}
		tb.Byte('\n')
	}

	if r.Valid {
		tb.Str("Every command states its grammar in the model.\n")
	} else {
		tb.Str("FAILED: ").Int(int64(len(r.Prose))).Str(" description(s) prescribe a CLI spelling\n")
	}

	return tb.String()
}

// runUsage runs the usage gate over the modules this binary carries.
func runUsage() (any, int) {
	if _, err := lepath.Root(); err != nil {
		reportError(err)
		return nil, 1
	}
	loader, err := yang.DefaultLoader()
	if err != nil {
		reportError(err)
		return nil, 1
	}
	report := usageContract(loader)
	if !report.Valid {
		return report, 1
	}
	return report, 0
}
