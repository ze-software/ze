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
	"bytes"
	"fmt"
	"os/exec"
	"path"
	"sort"
	"strconv"
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
// count the model has to close. Hidden names the sentences a commit DELETED
// while the model still renders something else, which is the same count going
// missing rather than closing.
type usageReport struct {
	Commands int        `json:"commands"`
	Prose    []usageRow `json:"prose"`
	Differ   []usageRow `json:"differ"`
	Hidden   []usageRow `json:"hidden"`
	Valid    bool       `json:"valid"`
}

// usageWalk carries the three answers one walk of the command tree produces:
// the report itself, the line the model renders for every command path, and the
// set of paths whose description still spells a grammar by hand.
//
// The last two exist for the HEAD comparison. It asks which paths LOST their
// sentence, so it needs every path the tree carries, not only the ones that
// still prescribe.
type usageWalk struct {
	report    *usageReport
	generated map[string]string
	authored  map[string]bool
}

// usageContract walks the command tree the loader holds and reports every
// description that prescribes a CLI spelling, with the count of command nodes
// the tree carries.
//
// The tree is the parameter rather than the checkout, so a test names a fixture
// module by building a loader over it (contract.go, Validate, takes the same
// shape for the same reason).
// The head map is the same walk performed over the modules at git HEAD, keyed
// by CLI path. A nil map skips the deletion half, which is what a fixture test
// of the prose half passes.
func usageContract(loader *yang.Loader, head map[string]usageRow) usageReport {
	report := usageReport{Prose: []usageRow{}, Differ: []usageRow{}, Hidden: []usageRow{}}
	walk := usageWalk{report: &report, generated: map[string]string{}, authored: map[string]bool{}}
	collectUsage(yang.BuildCommandTree(loader), nil, &walk)

	for cliPath, was := range head {
		if walk.authored[cliPath] {
			continue
		}
		now, reached := walk.generated[cliPath]
		if !reached {
			continue
		}
		if now == was.Authored {
			continue
		}
		report.Hidden = append(report.Hidden, usageRow{
			Path: cliPath, Generated: now, Authored: was.Authored, Marker: was.Marker,
		})
	}

	sort.Slice(report.Prose, func(i, j int) bool { return report.Prose[i].Path < report.Prose[j].Path })
	sort.Slice(report.Differ, func(i, j int) bool { return report.Differ[i].Path < report.Differ[j].Path })
	sort.Slice(report.Hidden, func(i, j int) bool { return report.Hidden[i].Path < report.Hidden[j].Path })
	report.Valid = len(report.Prose) == 0 && len(report.Hidden) == 0
	return report
}

// collectUsage walks one node's children in name order, counting the command
// nodes and collecting the authored sentences.
//
// The recursion is over the command tree, which this process built from its own
// embedded modules. No peer chooses its depth (docs/contributing/ze-go-style.md).
func collectUsage(node *command.Node, path []string, walk *usageWalk) {
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
			walk.report.Commands++
		}
		var tb textbuf.Buffer
		cliPath := tb.Join(childPath, " ").String()
		walk.generated[cliPath] = command.UsageLine(command.Usage(childPath, child))
		if marker, authored := authoredUsage(child.Description); authored != "" {
			walk.authored[cliPath] = true
			row := usageRow{
				Path:      cliPath,
				Generated: walk.generated[cliPath],
				Authored:  authored,
				Marker:    marker,
			}
			walk.report.Prose = append(walk.report.Prose, row)
			if row.Generated != row.Authored {
				walk.report.Differ = append(walk.report.Differ, row)
			}
		}
		collectUsage(child, childPath, walk)
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
	tb.Str("Authored and generated disagree: ").Int(int64(len(r.Differ))).Byte('\n')
	tb.Str("Deleted while the model still differs: ").Int(int64(len(r.Hidden))).Str("\n\n")

	if len(r.Prose) > 0 {
		tb.Str("## Descriptions that prescribe a CLI spelling (").Int(int64(len(r.Prose))).Str(")\n\n")
		for _, row := range r.Prose {
			tb.Str("  ").Str(row.Path).Str("\n    generated: ").Str(row.Generated).
				Str("\n    authored:  ").Str(row.Authored).Byte('\n')
		}
		tb.Byte('\n')
	}

	if len(r.Hidden) > 0 {
		tb.Str("## Sentences deleted while the model still renders another line (").
			Int(int64(len(r.Hidden))).Str(")\n\n")
		for _, row := range r.Hidden {
			tb.Str("  ").Str(row.Path).Str("\n    generated: ").Str(row.Generated).
				Str("\n    at HEAD:   ").Str(row.Authored).Byte('\n')
		}
		tb.Byte('\n')
	}

	switch {
	case r.Valid:
		tb.Str("Every command states its grammar in the model.\n")
	case len(r.Hidden) > 0:
		tb.Str("FAILED: ").Int(int64(len(r.Prose))).Str(" description(s) prescribe a CLI spelling, and ").
			Int(int64(len(r.Hidden))).Str(" deletion(s) hide a difference the model has not closed\n")
	default:
		tb.Str("FAILED: ").Int(int64(len(r.Prose))).Str(" description(s) prescribe a CLI spelling\n")
	}

	return tb.String()
}

// runUsage runs the usage gate over the modules this binary carries.
func runUsage() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		reportError(err)
		return nil, 1
	}
	loader, err := yang.DefaultLoader()
	if err != nil {
		reportError(err)
		return nil, 1
	}
	head, err := headUsage(root)
	if err != nil {
		reportError(err)
		return nil, 1
	}
	report := usageContract(loader, head)
	if !report.Valid {
		return report, 1
	}
	return report, 0
}

// cmdModuleFile is the file name ending of a module BuildCommandTree reads.
// Only these modules carry a ze:command, so only these can change an authored
// sentence or a generated line, and only these are read from HEAD.
// cmdModuleSuffix (contract.go) is the same convention without the extension.
const cmdModuleFile = cmdModuleSuffix + ".yang"

// headUsage walks the command tree the modules at git HEAD declare and returns
// the authored sentence of every command path, keyed by that path.
//
// The gate compares against HEAD rather than against a checked-in baseline
// file. A file can be edited to lie, and editing it is then the cheapest route
// from red to green; HEAD cannot be edited without also editing history
// (plan/spec-generated-command-usage.md, Key Design Decisions).
//
// Every failure returns an error rather than an empty map. An empty baseline
// reports no deletion at all, which is the answer that lets the whole gate be
// bypassed by breaking git (ai/rules/evidence.md: a zero value must never be a
// valid-looking answer).
func headUsage(root string) (map[string]usageRow, error) {
	files, err := cmdModulePaths(root)
	if err != nil {
		return nil, err
	}
	blobs, err := gitHeadBlobs(root, files)
	if err != nil {
		return nil, err
	}

	loader := yang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		return nil, fmt.Errorf("load the embedded modules: %w", err)
	}
	for _, module := range yang.Modules() {
		content := module.Content
		if was, held := blobs[files[module.Name]]; held {
			content = was
		}
		if err := loader.AddModuleFromText(module.Name, content); err != nil {
			return nil, fmt.Errorf("load %s at HEAD: %w", module.Name, err)
		}
	}
	// Best-effort, as DefaultLoader is: the command tree needs the -cmd modules
	// and the extensions they import, not the whole conf and api set.
	_ = loader.Resolve()

	report := usageReport{Prose: []usageRow{}, Differ: []usageRow{}, Hidden: []usageRow{}}
	walk := usageWalk{report: &report, generated: map[string]string{}, authored: map[string]bool{}}
	collectUsage(yang.BuildCommandTree(loader), nil, &walk)

	was := make(map[string]usageRow, len(report.Prose))
	for _, row := range report.Prose {
		was[row.Path] = row
	}
	return was, nil
}

// cmdModulePaths maps the module name a package registers to the tracked file
// that holds it. A module registers under its base name, so the map is keyed by
// base name and a repeated one is refused: two files answering to one name
// would make the HEAD text of that module a coin toss.
//
// A path under testdata is not a module of this product. The migration tool's
// fixtures repeat three base names on purpose.
func cmdModulePaths(root string) (map[string]string, error) {
	listed, err := gitOutput(root, "ls-files", "-z", "--", "*"+cmdModuleFile)
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	for rel := range strings.SplitSeq(string(listed), "\x00") {
		if rel == "" || strings.Contains(rel, "/testdata/") {
			continue
		}
		name := path.Base(rel)
		if held, repeated := files[name]; repeated {
			return nil, fmt.Errorf("two tracked files are named %s: %s and %s", name, held, rel)
		}
		files[name] = rel
	}
	return files, nil
}

// gitHeadBlobs returns the HEAD content of each path, keyed by path. A path git
// reports as missing is absent from the answer: it is a file added since HEAD,
// which carries no sentence HEAD could have held.
func gitHeadBlobs(root string, files map[string]string) (map[string]string, error) {
	if len(files) == 0 {
		return map[string]string{}, nil
	}
	names := make([]string, 0, len(files))
	for _, rel := range files {
		names = append(names, rel)
	}
	sort.Strings(names)

	var request bytes.Buffer
	for _, rel := range names {
		request.WriteString("HEAD:")
		request.WriteString(rel)
		request.WriteByte('\n')
	}
	cmd := exec.Command("git", "cat-file", "--batch") //nolint:gosec,noctx // this developer tool reads the checkout it was given
	cmd.Dir = root
	cmd.Stdin = &request
	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("read the command modules at HEAD: %w", err)
	}

	blobs := make(map[string]string, len(names))
	at := 0
	for _, rel := range names {
		newline := bytes.IndexByte(data[at:], '\n')
		if newline < 0 {
			return nil, fmt.Errorf("git cat-file stopped before %s", rel)
		}
		header := strings.Fields(string(data[at : at+newline]))
		at += newline + 1
		if len(header) == 2 {
			continue // "missing" or "ambiguous": the file is newer than HEAD
		}
		if len(header) != 3 || header[1] != "blob" {
			return nil, fmt.Errorf("git cat-file answered %q for %s", strings.Join(header, " "), rel)
		}
		size, err := strconv.Atoi(header[2])
		if err != nil || at+size > len(data) {
			return nil, fmt.Errorf("git cat-file gave %s an unusable size %q", rel, header[2])
		}
		blobs[rel] = string(data[at : at+size])
		at += size + 1
	}
	return blobs, nil
}

// gitOutput runs one git command in the checkout and returns its standard
// output.
func gitOutput(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...) //nolint:gosec,noctx // this developer tool reads the checkout it was given
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}
