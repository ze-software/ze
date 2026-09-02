// Design: docs/architecture/api/commands.md -- a command's two help texts
// Overview: usage.go -- collectUsage, the one walk both command gates share
// Related: report.go -- the answers this command renders
//
// helpshape.go answers one question about every node of the command tree: is
// the summary it declares the one short sentence that every one-line surface
// can render?
//
// A command declares its summary as the YANG description statement and its long
// explanation as the ze:help beside it. No renderer derives one from the other
// any more, so nothing except this gate keeps a summary short enough for a list
// row, a completion candidate or a table cell
// (plan/spec-yang-short-and-long-command-help.md, AC-3).
//
// The gate reports COVERAGE beside its refusals: how many nodes carry a summary
// and how many carry a long help. A node that declares nothing has no text to
// measure, so every shape rule would pass over it in silence, and an
// unconverted tree would read as a finished one (ai/rules/evidence.md).
//
// Three surfaces declare a command's help, and all three are judged by the same
// seven rules. A `-cmd.yang` node declares the CLI path an operator types. An
// `-api.yang` rpc declares the wire method that path reaches, and its own
// description is what `ze help` and the schema registry answer for it. An
// offline local command declares its summary as a registry.Meta beside its
// handler, and it reaches no YANG module at all. Gating one and not the others
// would leave part of the corpus with no shape to satisfy (AC-16).
//
// The third surface is the one this gate was blind to until 2026-08-31. The
// published catalog `ze help command --json` merges the command tree with the
// offline local registry (cmd/ze/help_command.go, collectCommands), so a gate
// that walked the tree alone reported GREEN over 601 nodes while
// `generate wireguard keypair` published a two-sentence, 41-word description
// nothing refused (plan/journal/gate-excludes-part-of-its-population.md).

package docvalid

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/ste"
)

// The rules a summary is held to. Each names one clause of AC-3, and each is
// reported on its own so the author is told what to change rather than that
// something is wrong.
const (
	ruleMissingSummary = "missing-summary"
	ruleOneSentence    = "one-sentence"
	ruleWordCap        = "word-cap"
	ruleNewline        = "no-newline"
	ruleSemicolon      = "no-semicolon"
	ruleFullStop       = "full-stop"
	ruleUsageMarker    = "no-usage-marker"
	ruleUnreadable     = "unreadable-node"
)

// The three surfaces that declare a command's help. A refusal names one,
// because the author fixes a command node in a `-cmd.yang` module, an rpc in an
// `-api.yang` one, and an offline local command in the Go file that registers
// it, and the three are different files.
const (
	surfaceCommand = "command"
	surfaceRPC     = "rpc"
	surfaceLocal   = "local"
)

// HelpShapeRow is one refusal: the surface it belongs to, the path that names
// it, the rule it broke, what the gate measured, and the summary it measured.
//
// Path is the CLI path for a command node and for an offline local command, and
// `<module>:<rpc-name>` for an rpc, which is the identity that names the file
// the author has to open.
type HelpShapeRow struct {
	Surface string `json:"surface"`
	Path    string `json:"path"`
	Rule    string `json:"rule"`
	Detail  string `json:"detail"`
	Summary string `json:"summary"`
}

// HelpShapeReport is the whole answer of one `le docvalid help-shape` run: the
// coverage of the command tree, and every summary that breaks the shape.
//
// WithSummary counts a DECLARED summary, whether or not it satisfies the shape.
// The two numbers answer different questions: coverage says how much of the
// tree has been written, and Broken says how much of what was written is wrong.
type HelpShapeReport struct {
	Nodes             int            `json:"nodes"`
	Commands          int            `json:"commands"`
	WithSummary       int            `json:"with-summary"`
	WithHelp          int            `json:"with-help"`
	RPCs              int            `json:"rpcs"`
	RPCsWithSummary   int            `json:"rpcs-with-summary"`
	RPCsWithHelp      int            `json:"rpcs-with-help"`
	Locals            int            `json:"locals"`
	LocalsWithSummary int            `json:"locals-with-summary"`
	LocalsWithHelp    int            `json:"locals-with-help"`
	Broken            []HelpShapeRow `json:"broken"`
	Valid             bool           `json:"valid"`
}

// node judges one command node's summary and counts it.
func (r *HelpShapeReport) node(cliPath string, node *command.Node) {
	r.Nodes++
	if node.WireMethod != "" {
		r.Commands++
	}
	if strings.TrimSpace(node.Help) != "" {
		r.WithHelp++
	}
	if r.judgeSummary(surfaceCommand, cliPath, node.Description) {
		r.WithSummary++
	}
}

// rpc judges one RPC's summary and counts it.
//
// The label is `<module>:<rpc-name>` rather than the CLI path the operator
// types: several command paths can reach one wire method, and the author edits
// the module (ai/rules/evidence.md -- name the producer, not a caller of it).
func (r *HelpShapeReport) rpc(label string, meta yang.RPCMeta) {
	r.RPCs++
	if strings.TrimSpace(meta.Help) != "" {
		r.RPCsWithHelp++
	}
	if r.judgeSummary(surfaceRPC, label, meta.Description) {
		r.RPCsWithSummary++
	}
}

// local judges one offline local command's summary and counts it.
//
// The label is the CLI path an operator types, which is also the first argument
// of the registration the author has to open.
func (r *HelpShapeReport) local(entry registry.LocalCommandEntry) {
	r.Locals++
	if strings.TrimSpace(entry.Meta.LongHelp) != "" {
		r.LocalsWithHelp++
	}
	if r.judgeSummary(surfaceLocal, entry.Path, entry.Meta.Description) {
		r.LocalsWithSummary++
	}
}

// judgeSummary judges one authored summary against the seven rules of AC-3 and
// answers whether a summary was declared at all. False means the author has
// written nothing here, which the caller counts as coverage owed rather than as
// a summary that passed.
//
// One judge for both surfaces, because AC-3 states one shape. A second copy of
// these rules would let a command node and an rpc drift into two shapes, and
// the reader of a refusal could not tell which one applied to them.
func (r *HelpShapeReport) judgeSummary(surface, label, description string) bool {
	summary := strings.TrimSpace(description)
	if summary == "" {
		var detail textbuf.Buffer
		r.refuse(surface, label, ruleMissingSummary,
			detail.Str("the ").Str(surface).Str(" declares no description").String(), "")
		return false
	}

	if strings.ContainsAny(summary, "\n\r") {
		r.refuse(surface, label, ruleNewline, "the summary is written over more than one line", summary)
	}
	if strings.Contains(summary, ";") {
		r.refuse(surface, label, ruleSemicolon, "the summary joins two statements with a semicolon", summary)
	}
	if !strings.HasSuffix(summary, ".") {
		r.refuse(surface, label, ruleFullStop, "the summary does not end in a full stop", summary)
	}
	if count := len(ste.Sentences(summary)); count > 1 {
		var detail textbuf.Buffer
		r.refuse(surface, label, ruleOneSentence,
			detail.Str("the summary is ").Int(int64(count)).Str(" sentences").String(), summary)
	}
	if count := ste.WordCount(summary); count > ste.MaxDescriptiveWords {
		var detail textbuf.Buffer
		r.refuse(surface, label, ruleWordCap, detail.Str("the summary is ").Int(int64(count)).
			Str(" words (STE Rule 6.3 allows ").Int(int64(ste.MaxDescriptiveWords)).Byte(')').String(), summary)
	}
	// The same three markers the usage gate refuses, read from the same table: a
	// summary that prescribes an invocation form breaks ai/rules/cli.md whatever
	// word it opens with.
	if marker, _ := authoredUsage(summary); marker != "" {
		var detail textbuf.Buffer
		r.refuse(surface, label, ruleUsageMarker,
			detail.Str("the summary prescribes a CLI spelling under ").Quoted(marker).String(), summary)
	}
	return true
}

// unreadable names a child the tree holds under a name with no node behind it.
func (r *HelpShapeReport) unreadable(cliPath string) {
	r.Nodes++
	r.refuse(surfaceCommand, cliPath, ruleUnreadable, "the tree holds this name with no node behind it", "")
}

// refuse records one broken rule.
func (r *HelpShapeReport) refuse(surface, label, rule, detail, summary string) {
	r.Broken = append(r.Broken, HelpShapeRow{
		Surface: surface, Path: label, Rule: rule, Detail: detail, Summary: summary,
	})
}

// helpShapeContract walks the command tree the loader holds, the RPCs beside it
// and the offline local commands, and answers the coverage of all three with
// every summary that breaks the shape.
//
// The tree and the local registrations are parameters rather than the checkout,
// so a test names a fixture module by building a loader over it and a fixture
// registration by passing one (usage.go, usageContract, takes the same shape
// for the same reason).
//
// A tree with no command node is an error rather than a valid report. Every
// count would be zero and no rule could be broken, so a load failure would read
// as a converted tree and the cheapest route from red to green would be to stop
// loading the modules (ai/rules/evidence.md).
func helpShapeContract(loader *yang.Loader, locals []registry.LocalCommandEntry) (HelpShapeReport, error) {
	tree := yang.BuildCommandTree(loader)
	walk := newUsageWalk()
	collectUsage(tree, nil, &walk)
	collectRPCs(loader, walk.shape)
	collectLocals(locals, tree, walk.shape)

	report := *walk.shape
	if report.Commands == 0 {
		return report, errors.New("the command tree holds no command node: the modules did not load")
	}
	// The RPC half is refused on the same argument as the command half: an
	// empty set of API modules breaks no rule, so a load that returned nothing
	// would read as a corpus with nothing left to write.
	if report.RPCs == 0 {
		return report, errors.New("no module declares an RPC: the modules did not load")
	}
	// And the local half on the same argument again. The offline registry is a
	// third corpus, so a run that read none of it would report the catalog as
	// covered without having read the half the command tree does not carry.
	if report.Locals == 0 {
		return report, errors.New("no offline local command was read: the registry did not load")
	}

	sort.Slice(report.Broken, func(i, j int) bool {
		left, right := report.Broken[i], report.Broken[j]
		if left.Surface != right.Surface {
			return left.Surface < right.Surface
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		return left.Rule < right.Rule
	})
	report.Valid = len(report.Broken) == 0
	return report, nil
}

// collectRPCs judges the description of every RPC the loader holds.
//
// The RPCs are read from the modules rather than from the command tree, because
// the two are separate declarations: a command node names a wire method, and
// the RPC that method reaches carries its own description in its own module
// (internal/component/config/yang/rpc.go, ExtractRPCs).
//
// EVERY module is walked, not the `-api` ones alone. 196 of the 218 RPCs are
// declared in an `-api` module and the other 22 are the plugin IPC protocol in
// `internal/core/ipc/yang/` (`ze-plugin-engine`, `ze-plugin-callback`). A
// suffix filter would leave those 22 with no shape to satisfy and report the
// corpus as covered, which is the silent answer this gate exists to remove
// (ai/rules/evidence.md).
func collectRPCs(loader *yang.Loader, report *HelpShapeReport) {
	if loader == nil {
		return
	}
	modules := loader.ModuleNames()
	sort.Strings(modules)

	var tb textbuf.Buffer
	for _, module := range modules {
		for _, meta := range yang.ExtractRPCs(loader, module) {
			tb.Reset()
			report.rpc(tb.Str(module).Byte(':').Str(meta.Name).String(), meta)
		}
	}
}

// collectLocals judges the summary of every offline local command the catalog
// publishes from the registry rather than from the command tree.
//
// A path the command tree HOLDS is skipped, because `collectCommands`
// (cmd/ze/help_command.go) publishes the node's description for such a path and
// never the registration's. Judging it would ask an author to declare the same
// summary twice, and the tree half of this gate has already judged the one the
// catalog prints.
func collectLocals(locals []registry.LocalCommandEntry, tree *command.Node, report *HelpShapeReport) {
	for _, entry := range locals {
		if command.FindNode(tree, strings.Fields(entry.Path)) != nil {
			continue
		}
		report.local(entry)
	}
}

// offlineLocalCommands answers every offline local command whose summary the
// published catalog can print: the registrations this binary links, and the
// ones cmd/ze declares in its main package.
//
// A `le ` path is left out, and the reason is the one wikicatalog.Collect
// states for the same set: `le` is development tooling that no shipped ze
// carries, so the published catalog holds none of it. It is also the one part
// of this population whose size depends on the linker rather than on the
// checkout, because each le tool registers from its own package.
func offlineLocalCommands(root string) ([]registry.LocalCommandEntry, error) {
	main, err := mainPackageLocalCommands(root)
	if err != nil {
		return nil, err
	}

	locals := append(registry.ListLocal(), main...)
	kept := locals[:0]
	for _, entry := range locals {
		if strings.HasPrefix(entry.Path, lePathPrefix) {
			continue
		}
		kept = append(kept, entry)
	}
	return kept, nil
}

// lePathPrefix names the development-tooling namespace the published catalog
// does not carry.
const lePathPrefix = "le "

// mainPackageLocalCommands reads the local commands cmd/ze registers in its
// main package, sorted by path.
//
// It is the only reader of those registrations in the repository:
// wikicatalog.Collect mirrors the same four entries, which no package can link,
// and mainpackage_test.go compares the mirror against this answer so the two
// cannot drift (ai/rules/principles.md -- one declaration, everything else
// derived from it).
//
// They are read from SOURCE because Go forbids importing a main package, so no
// gate can link them. Four of them exist today and three reach the catalog, and
// a gate that skipped what it cannot link would certify the catalog while
// leaving part of it unread -- the defect this surface was added to close.
//
// A registration whose path is not a literal STOPS the gate and names the file
// and line. The gate cannot read what that call registers, so every count it
// went on to print would be about a population it does not know
// (ai/rules/evidence.md).
func mainPackageLocalCommands(root string) ([]registry.LocalCommandEntry, error) {
	dir := filepath.Join(root, "cmd", "ze")
	names, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var out []registry.LocalCommandEntry
	for _, name := range names {
		if name.IsDir() || !strings.HasSuffix(name.Name(), ".go") || strings.HasSuffix(name.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, name.Name())
		found, err := localCommandsInFile(path)
		if err != nil {
			if vanished(path) {
				continue
			}
			return nil, err
		}
		out = append(out, found...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// localRegistrars names the registry functions that publish a local command.
// contract.go reads the same set for the same reason, and a name missing here
// is a registration the gate does not see.
var localRegistrars = map[string]bool{
	"RegisterLocal":         true,
	"MustRegisterLocal":     true,
	"RegisterLocalMeta":     true,
	"MustRegisterLocalMeta": true,
	"RegisterLocalData":     true,
	"MustRegisterLocalData": true,
}

// localCommandsInFile answers every local command the file at path registers.
func localCommandsInFile(path string) ([]registry.LocalCommandEntry, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}

	var out []registry.LocalCommandEntry
	var unreadable error
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !localRegistrars[selector.Sel.Name] {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		if !ok || (pkg.Name != pkgRegistry && pkg.Name != pkgCmdRegistry) {
			return true
		}

		cliPath, ok := stringLiteral(call.Args[0])
		if !ok {
			var tb textbuf.Buffer
			unreadable = errors.New(tb.Str(path).Byte(':').
				Int(int64(fset.Position(call.Pos()).Line)).
				Str(": registers a local command under a path this gate cannot read").String())
			return false
		}
		out = append(out, registry.LocalCommandEntry{Path: cliPath, Meta: metaLiteral(call.Args)})
		return true
	})
	if unreadable != nil {
		return nil, unreadable
	}
	return out, nil
}

// metaLiteral reads the registry.Meta a registration declares. A call that
// passes none, or passes one through a variable, declares no summary this
// reader can see, and the zero Meta it answers is what the gate refuses.
//
// The composite is matched on its TYPE rather than on its field names, so
// another literal in the same call cannot be read as the Meta because it
// happens to hold a Description field.
func metaLiteral(args []ast.Expr) registry.Meta {
	var meta registry.Meta
	for _, arg := range args {
		composite, ok := arg.(*ast.CompositeLit)
		if !ok || !isMetaType(composite.Type) {
			continue
		}
		for _, element := range composite.Elts {
			field, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := field.Key.(*ast.Ident)
			if !ok {
				continue
			}
			value, ok := stringLiteral(field.Value)
			if !ok {
				continue
			}
			switch key.Name {
			case "Description":
				meta.Description = value
			case "LongHelp":
				meta.LongHelp = value
			}
		}
	}
	return meta
}

// isMetaType answers whether a composite literal's type is the command
// registry's Meta, under either name the repository imports it by.
func isMetaType(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Meta" {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	return pkg.Name == pkgRegistry || pkg.Name == pkgCmdRegistry
}

// stringLiteral answers the value of a quoted string expression.
//
// A concatenation of quoted strings is one such expression: a summary or a long
// help wider than the line budget is written as `"..." + "..."`, and a reader
// that saw only a single literal would answer "nothing declared here" for text
// the compiler puts in the registry (ai/rules/evidence.md).
//
// The recursion walks the concatenation the Go parser built for one argument of
// one call, so its depth is the number of `+` operators an author wrote on that
// line. Nothing outside this checkout reaches it.
func stringLiteral(expr ast.Expr) (string, bool) {
	switch node := expr.(type) {
	case *ast.BasicLit:
		if node.Kind != token.STRING {
			return "", false
		}
		value, err := strconv.Unquote(node.Value)
		if err != nil {
			return "", false
		}
		return value, true
	case *ast.BinaryExpr:
		if node.Op != token.ADD {
			return "", false
		}
		left, ok := stringLiteral(node.X)
		if !ok {
			return "", false
		}
		right, ok := stringLiteral(node.Y)
		if !ok {
			return "", false
		}
		return left + right, true
	default:
		return "", false
	}
}

// Text renders the shape report: the coverage counts, the count of each broken
// rule, one line per refusal, and the verdict. It ends in a newline.
func (r HelpShapeReport) Text() string {
	var tb textbuf.Buffer

	tb.Str("# Command Help Shape\n\n")
	tb.Str("Command tree nodes: ").Int(int64(r.Nodes)).Byte('\n')
	tb.Str("Nodes that run a command: ").Int(int64(r.Commands)).Byte('\n')
	tb.Str("Nodes with a summary: ").Int(int64(r.WithSummary)).Byte('\n')
	tb.Str("Nodes with a long help: ").Int(int64(r.WithHelp)).Byte('\n')
	tb.Str("RPCs: ").Int(int64(r.RPCs)).Byte('\n')
	tb.Str("RPCs with a summary: ").Int(int64(r.RPCsWithSummary)).Byte('\n')
	tb.Str("RPCs with a long help: ").Int(int64(r.RPCsWithHelp)).Byte('\n')
	tb.Str("Offline local commands: ").Int(int64(r.Locals)).Byte('\n')
	tb.Str("Offline local commands with a summary: ").Int(int64(r.LocalsWithSummary)).Byte('\n')
	tb.Str("Offline local commands with a long help: ").Int(int64(r.LocalsWithHelp)).Str("\n\n")

	if len(r.Broken) == 0 {
		tb.Str("Every command node, every RPC and every offline local command declares a summary " +
			"of one short sentence.\n")
		return tb.String()
	}

	tb.Str("Nodes with a broken summary: ").Int(int64(r.brokenPaths(surfaceCommand))).Byte('\n')
	tb.Str("RPCs with a broken summary: ").Int(int64(r.brokenPaths(surfaceRPC))).Byte('\n')
	tb.Str("Offline local commands with a broken summary: ").
		Int(int64(r.brokenPaths(surfaceLocal))).Str("\n\n")

	counts := r.ruleCounts()
	width := 0
	for _, count := range counts {
		if len(count.rule) > width {
			width = len(count.rule)
		}
	}

	tb.Str("## Broken rules (").Int(int64(len(r.Broken))).Str(")\n\n")
	for _, count := range counts {
		tb.Str("  ").PadRight(count.rule, width).Byte(' ').Int(int64(count.nodes)).Byte('\n')
	}
	tb.Byte('\n')

	for _, row := range r.Broken {
		tb.Str("  ").Str(row.Surface).Byte(' ').Str(row.Path).Str("\n    rule:    ").Str(row.Rule).
			Str("\n    problem: ").Str(row.Detail).Byte('\n')
		if row.Summary != "" {
			tb.Str("    summary: ").Str(row.Summary).Byte('\n')
		}
	}
	tb.Byte('\n')

	tb.Str("FAILED: ").Int(int64(len(r.Broken))).Str(" summary rule(s) broken over ").
		Int(int64(r.Nodes)).Str(" command tree node(s), ").Int(int64(r.RPCs)).Str(" RPC(s) and ").
		Int(int64(r.Locals)).Str(" offline local command(s)\n")
	return tb.String()
}

// brokenPaths answers how many DISTINCT paths of one surface hold a refusal.
// One summary can break several rules, so the refusal count is not the number
// of nodes left to write, and the number of nodes is what the work is measured
// in.
func (r HelpShapeReport) brokenPaths(surface string) int {
	held := map[string]struct{}{}
	for _, row := range r.Broken {
		if row.Surface == surface {
			held[row.Path] = struct{}{}
		}
	}
	return len(held)
}

// ruleCount is one row of the tally the report opens its failures with.
type ruleCount struct {
	rule  string
	nodes int
}

// ruleCounts answers how many refusals each rule holds, most first, so the
// worklist opens with the rule that costs the most to close.
//
// The tally is DERIVED from Broken rather than counted beside it. Two counters
// of one fact disagree the first time a refusal is added on one path only.
func (r HelpShapeReport) ruleCounts() []ruleCount {
	held := map[string]int{}
	for _, row := range r.Broken {
		held[row.Rule]++
	}
	counts := make([]ruleCount, 0, len(held))
	for rule, nodes := range held {
		counts = append(counts, ruleCount{rule: rule, nodes: nodes})
	}
	sort.Slice(counts, func(i, j int) bool {
		if counts[i].nodes != counts[j].nodes {
			return counts[i].nodes > counts[j].nodes
		}
		return counts[i].rule < counts[j].rule
	})
	return counts
}

// HelpShape answers the shape gate over the command modules this binary
// carries.
//
// Every failure is an error rather than an empty report, for the reason
// helpShapeContract states: a report nobody could produce must not read as a
// tree with nothing to fix.
func HelpShape() (HelpShapeReport, error) {
	root, err := lepath.Root()
	if err != nil {
		return HelpShapeReport{}, err
	}
	locals, err := offlineLocalCommands(root)
	if err != nil {
		return HelpShapeReport{}, err
	}
	loader, err := yang.DefaultLoader()
	if err != nil {
		return HelpShapeReport{}, err
	}
	return helpShapeContract(loader, locals)
}

// runHelpShape runs the shape gate for the action table.
func runHelpShape() (any, int) {
	report, err := HelpShape()
	if err != nil {
		reportError(err)
		return nil, 1
	}
	if !report.Valid {
		return report, 1
	}
	return report, 0
}
