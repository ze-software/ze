// Design: docs/architecture/api/commands.md — command catalog
//
// help_command.go implements `ze help command [<filter>] [--json] [--verbose]`.
// It walks the full command tree (YANG verbs + offline local commands)
// and emits a flat, greppable catalog with descriptions.

// JSON output is consumable by external tooling (e.g., wiki generators).

package help

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"slices"
	"sort"
	"strings"

	cli "github.com/ze-software/ze/internal/component/cli/client"
	"github.com/ze-software/ze/internal/component/command"
	pluginregistry "github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// commandArg describes a typed argument for a command. ShortHelp is the
// leaf's ze:help summary and Description its long explanation; each key is
// written when the leaf declares that text, and neither is derived from the
// other.
type commandArg struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Values      []string `json:"values,omitempty"`
	Mandatory   bool     `json:"mandatory,omitempty"`
	ShortHelp   string   `json:"short-help,omitempty"`
	Description string   `json:"description,omitempty"`
}

// pipeOperationsFor answers the `pipe-operations` list and the declared answer
// shape a catalog row publishes, from the declaration the catalog already
// resolved for the command (command.DeclaredForCommand), so a compiled-in
// plugin's operations and aliases reach this offline catalog as its commands
// do. The join is command.PipeOperations, the one `show command help` reads.
//
// A command no surface pipes publishes neither (command.PipeLayerReached).
// wireToPaths is cli.WireToPaths, fetched once by the caller, which reports its
// error: it is what tells a daemon handler from a plain local shortcut.
func pipeOperationsFor(cliPath string, declared command.Declared, wireToPaths map[string][]string) ([]command.PipeOperationEntry, string) {
	daemonServes := func(path string) bool { return daemonHandlesPath(path, wireToPaths) }
	if !command.PipeLayerReached(cliPath, daemonServes) {
		return nil, ""
	}
	operations := command.PipeOperations(declared)
	if !declared.ShapeDeclared {
		return operations, ""
	}
	return operations, declared.Shape.String()
}

// daemonHandlesPath answers whether the daemon registers a non-nil handler for
// this exact YANG path. The local and daemon surfaces are independent, so a
// plain local shortcut must not hide a reachable daemon handler. wireToPaths
// is cli.WireToPaths.
func daemonHandlesPath(cliPath string, wireToPaths map[string][]string) bool {
	for _, registration := range pluginserver.AllBuiltinRPCs() {
		if registration.Handler == nil {
			continue
		}
		if slices.Contains(wireToPaths[registration.WireMethod], cliPath) {
			return true
		}
	}
	return false
}

// operationNames answers the names of the generics and overrides available as
// available says, or of the local-only ones when localOnly is set. Answer,
// stream and surface qualifiers stay separate dimensions: flattening any one of
// them tells a caller an operator is unconditional when it is not.
func operationNames(operations []command.PipeOperationEntry, available string, localOnly bool) []string {
	var names []string
	for _, operation := range operations {
		switch operation.Source {
		case command.PipeSourceGeneric, command.PipeSourceOverride:
		default:
			// A command operation and an alias carry no catalog qualifier;
			// they are printed one per line beside these.
			continue
		}
		if localOnly {
			if operation.LocalOnly {
				names = append(names, operation.Name)
			}
			continue
		}
		if operation.Available == available {
			names = append(names, operation.Name)
		}
	}
	return names
}

// commandEntry is a single command in the catalog.
type commandEntry struct {
	Path string `json:"path"`
	// ShortHelp is the command's one-line summary. Description is its long
	// explanation. They are two declarations, not one string cut in two, so a
	// reader takes whichever its surface renders.
	ShortHelp   string       `json:"short-help,omitempty"`
	Description string       `json:"description,omitempty"`
	Mode        string       `json:"mode"`
	WireMethod  string       `json:"wire-method,omitempty"`
	Backend     []string     `json:"backend,omitempty"`
	TaskSupport string       `json:"task-support,omitempty"`
	Args        []commandArg `json:"args,omitempty"`
	// Usage is the invocation form, generated from the command model. It is
	// what an operator types.
	Usage string `json:"usage,omitempty"`
	// Grammar is that same form as an ordered token list, so a machine reader
	// never parses angle and square brackets out of a string. Both come from
	// one producer (internal/component/command, Usage), so the two projections
	// cannot disagree. Args stays beside them: it is the type dictionary, and
	// it carries no position.
	Grammar []command.UsageToken `json:"grammar,omitempty"`
	// PipeOperations is everything the command answers to after a pipe: the
	// generics the chain admits for it, its overrides, its own operations and
	// its aliases, each with its source, argument and availability. It is the
	// list `show command help` answers (command.PipeOperations).
	PipeOperations []command.PipeOperationEntry `json:"pipe-operations,omitempty"`
	// AnswerShape is the shape the command DECLARED, absent when it declared
	// none. It decides which operators are always available.
	AnswerShape string `json:"answer-shape,omitempty"`
	// AddressFields names the fields the command declares as IP addresses.
	// Their presence gates resolve and origin and is part of the published
	// contract checked against the wiki and website.
	AddressFields []string `json:"address-fields,omitempty"`
	// ColumnOrders are the JSON keys of the answer's records, in the order a
	// person reads them, one list per record shape the command renders. A
	// command that renders an outer record and a list of rows declares two
	// orders, so a flat list could state neither.
	//
	// `| json`, `| ndjson` and `| yaml` keep their alphabetical keys whatever
	// this says: a program reads them and column order carries no meaning for a
	// program (internal/component/command/column_order.go, RegisterColumns).
	ColumnOrders [][]string `json:"column-orders,omitempty"`
	Subcommands  []string   `json:"subcommands,omitempty"`
}

// catalogAnswer is the answer of `help command`: the catalog rows the filter
// kept. asJSON and verbose record the local `--json`, `--verbose` and `-v`
// rendering flags; they are not data, so JSON never carries them, and the
// daemon dispatcher refuses a flag before a handler runs.
type catalogAnswer struct {
	Commands []commandEntry `json:"command-catalog"`
	asJSON   bool
	verbose  bool
}

// answerHelpCommand answers `help command [<filter>] [--json] [--verbose]`
// with the catalog rows as DATA, so `| json` reaches them on every route.
// The local route prints them through render, the daemon's text route
// through drawCatalog. The handler writes nothing.
func answerHelpCommand(validated command.ValidatedArgs) command.LocalAnswer {
	return helpCommandAnswer(validated.Tokens())
}

// helpCommandAnswer builds the `help command` answer from the tokens.
func helpCommandAnswer(args []string) command.LocalAnswer {
	entries, err := collectCommands()
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("error: ").Err(err).Byte('\n')
		return command.LocalAnswer{Code: 1, Diagnostic: tb.String()}
	}
	if filter := extractCommandFilter(args); filter != "" {
		entries = filterCommands(entries, filter)
	}
	return command.LocalAnswer{Payload: catalogAnswer{
		Commands: entries,
		asJSON:   slices.Contains(args, flagJSON),
		verbose:  slices.Contains(args, "--verbose") || slices.Contains(args, "-v"),
	}}
}

// printCatalog prints a catalog answer to w, its diagnostics to diag, and
// returns the exit code: 1 when a write to w fails, so `ze help command |
// head` closing the pipe does not report a truncated page as whole.
func printCatalog(w, diag io.Writer, answer catalogAnswer) int {
	if answer.asJSON {
		return printCommandJSON(w, diag, answer.Commands)
	}
	rw := helpfmt.NewRenderWriter(w)
	if answer.verbose {
		printCommandVerbose(rw, diag, answer.Commands)
	} else {
		printCommandTable(rw, diag, answer.Commands)
	}
	return rw.ExitCode()
}

// drawCatalog draws the catalog table for the daemon's text route, where no
// local flag reaches the handler. "no commands found" is part of the text.
func drawCatalog(raw json.RawMessage) (string, error) {
	var entries []commandEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return "", err
	}
	var out bytes.Buffer
	printCommandTable(helpfmt.NewRenderWriter(&out), &out, entries)
	return out.String(), nil
}

// extractCommandFilter returns the filter: the value of the model's keyed
// `filter <text>`, else the first word that is neither a flag nor one of the
// model's option keywords.
func extractCommandFilter(args []string) string {
	for i, a := range args {
		if a == keywordFilter && i+1 < len(args) {
			return args[i+1]
		}
	}
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a
	}
	return ""
}

// collectCommands gathers every command from both the YANG dispatch
// registry and the offline local command registry into a flat sorted list.
// The error is the YANG loader's refusal of the schema: a catalog built from
// no schema would publish the offline commands alone as the whole surface.
func collectCommands() ([]commandEntry, error) {
	var entries []commandEntry
	seen := make(map[string]bool)

	tree, err := cli.YANGCommandTree()
	if err != nil {
		return nil, err
	}
	wireToPaths, err := cli.WireToPaths()
	if err != nil {
		return nil, err
	}

	for wireMethod, cliPaths := range wireToPaths {
		for _, cliPath := range cliPaths {
			if command.HiddenAtPath(tree, strings.Fields(cliPath)) {
				continue
			}
			if seen[cliPath] {
				continue
			}
			mode := "daemon"
			if pluginserver.IsReadOnlyPath(cliPath) {
				mode = "read-only"
			}
			// A local handler is a second route even when the daemon serves
			// the same path; the later local pass skips this entry.
			if command.HasLocal(cliPath) {
				mode += ", " + modeOffline
			}
			node := findNode(tree, cliPath)
			desc, explanation := "", ""
			if node != nil {
				desc, explanation = node.ShortHelp, node.Description
			}
			e := commandEntry{
				Path:        cliPath,
				ShortHelp:   desc,
				Description: explanation,
				Mode:        mode,
				WireMethod:  wireMethod,
			}
			declared := command.DeclaredForCommand(cliPath)
			e.PipeOperations, e.AnswerShape = pipeOperationsFor(cliPath, declared, wireToPaths)
			e.AddressFields = declared.AddressFields
			e.ColumnOrders = command.ColumnNames(declared.Columns)
			if node != nil {
				e.Args = extractArgs(node)
				e.Grammar = command.Usage(strings.Fields(cliPath), node)
				e.Usage = command.UsageLine(e.Grammar)
				e.Subcommands = command.SubcommandNames(node)
				e.Backend = node.Backend
				if node.TaskSupport != "" {
					e.TaskSupport = node.TaskSupport
				}
			}
			entries = append(entries, e)
			seen[cliPath] = true
		}
	}

	for _, lc := range command.ListLocal() {
		if command.HiddenAtPath(tree, strings.Fields(lc.Path)) {
			continue
		}
		if seen[lc.Path] {
			continue
		}
		mode := lc.Meta.Mode
		if mode == "" {
			mode = modeOffline
		}
		entries = append(entries, commandEntry{
			Path:        lc.Path,
			ShortHelp:   lc.Meta.ShortHelp,
			Description: lc.Meta.Description,
			Mode:        mode,
		})
		seen[lc.Path] = true
	}

	entries = appendPluginCommands(entries, seen, tree, wireToPaths)

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	return entries, nil
}

// appendPluginCommands adds every command a plugin declares on its
// registration that nothing above already named.
//
// A plugin's command is dispatched through the plugin, so it reaches neither
// the YANG command tree nor the local command registry, and the catalog named
// none of them however much each declared. `show bgp rpki roa` states a shape,
// a column order and an address field, and this catalog answered that the
// command does not exist.
//
// A HIDDEN declaration is skipped. This catalog is the operator's list, and the
// daemon already keeps a hidden command out of VisibleCommandEntries and out of
// completion (internal/component/plugin/server/command_registry.go), so
// publishing one here would be the only surface offering it.
//
// The YANG node wins wherever one exists. A plugin command can be modeled and
// still carry no wire method, `show vrrp interface` among them, so it reaches
// this function with an authored summary, long help and grammar already
// written. Taking the declaration's texts over those would publish the
// plugin's one-line summary in place of the model's, and drop the grammar.
//
// For a command no node models, Usage carries the invocation form the plugin
// declares and Grammar stays EMPTY. A plugin declares its arguments as text, so
// a token list built from that text would state kinds nobody declared; the four
// main-package builtins and every offline local command publish neither field
// for the same reason.
func appendPluginCommands(entries []commandEntry, seen map[string]bool, tree *command.Node, wireToPaths map[string][]string) []commandEntry {
	for _, registration := range pluginregistry.All() {
		for index := range registration.Commands {
			decl := &registration.Commands[index]
			if command.HiddenAtPath(tree, strings.Fields(decl.Name)) {
				continue
			}
			if decl.Name == "" || decl.Hidden || seen[decl.Name] {
				continue
			}
			mode := "daemon"
			if pluginserver.IsReadOnlyPath(decl.Name) {
				mode = "read-only"
			}
			e := commandEntry{
				Path:        decl.Name,
				ShortHelp:   decl.ShortHelp,
				Description: decl.Description,
				Mode:        mode,
				Usage:       pluginUsage(decl.Name, decl.Args),
			}
			if node := findNode(tree, decl.Name); node != nil {
				e.ShortHelp = node.ShortHelp
				e.Description = node.Description
				e.Args = extractArgs(node)
				e.Subcommands = command.SubcommandNames(node)
				e.Backend = node.Backend
				e.TaskSupport = node.TaskSupport
				// Usage answers nil for a node carrying no wire method, which
				// every plugin-dispatched node does, so the declaration's own
				// form is kept rather than overwritten with an empty line.
				if grammar := command.Usage(strings.Fields(decl.Name), node); len(grammar) > 0 {
					e.Grammar = grammar
					e.Usage = command.UsageLine(grammar)
				}
			}
			declared := command.DeclaredForCommand(decl.Name)
			e.PipeOperations, e.AnswerShape = pipeOperationsFor(decl.Name, declared, wireToPaths)
			e.AddressFields = declared.AddressFields
			e.ColumnOrders = command.ColumnNames(declared.Columns)
			entries = append(entries, e)
			seen[decl.Name] = true
		}
	}
	return entries
}

// pluginUsage answers the invocation form a plugin declares: the command path,
// then the argument tokens the plugin spells for a reader.
//
// The tokens are published as the plugin wrote them. They are a usage STRING in
// the declaration, which is also how the daemon carries them to `show command
// help` (internal/component/plugin/server, CommandRegistry), so rewriting them
// here would give one command two spellings.
func pluginUsage(path string, args []string) string {
	if len(args) == 0 {
		return path
	}
	var tb textbuf.Buffer
	tb.Str(path)
	for _, arg := range args {
		tb.Byte(' ').Str(arg)
	}
	return tb.String()
}

// findNode looks up the node for a CLI path in the command tree.
func findNode(tree *command.Node, cliPath string) *command.Node {
	if tree == nil {
		return nil
	}
	return command.FindNode(tree, strings.Fields(cliPath))
}

// extractArgs converts a node's ArgDefs into the JSON-friendly commandArg slice.
func extractArgs(node *command.Node) []commandArg {
	if len(node.ArgDefs) == 0 {
		return nil
	}
	args := make([]commandArg, 0, len(node.ArgDefs))
	for i := range node.ArgDefs {
		ad := &node.ArgDefs[i]
		a := commandArg{
			Name:        ad.Name(),
			Type:        argKindString(ad.Kind()),
			Mandatory:   ad.Mandatory(),
			ShortHelp:   ad.ShortHelp(),
			Description: ad.Description(),
		}
		if values := slices.Collect(ad.EnumValues()); len(values) > 0 {
			a.Values = values
		}
		args = append(args, a)
	}
	return args
}

func argKindString(k command.ArgKind) string {
	switch k {
	case command.ArgEnum:
		return "enum"
	case command.ArgUint:
		return "uint"
	case command.ArgUnion:
		return "union"
	case command.ArgFlag:
		return "flag"
	case command.ArgString:
		return "string"
	default:
		panic("BUG: unknown command argument kind")
	}
}

// filterCommands returns entries whose path or description contains
// the filter string (case-insensitive).
func filterCommands(entries []commandEntry, filter string) []commandEntry {
	lower := strings.ToLower(filter)
	var filtered []commandEntry
	for i := range entries {
		if strings.Contains(strings.ToLower(entries[i].Path), lower) ||
			strings.Contains(strings.ToLower(entries[i].ShortHelp), lower) {
			filtered = append(filtered, entries[i])
		}
	}
	return filtered
}

// printCommandJSON writes entries as a JSON array to w and returns the exit code.
func printCommandJSON(w, diag io.Writer, entries []commandEntry) int {
	rw := helpfmt.NewRenderWriter(w)
	enc := json.NewEncoder(rw)
	// A usage line spells an argument as <id>, and encoding/json escapes the
	// angle brackets to their six-character unicode escape form unless it is
	// told not to. This answer is read by an operator, by a tool, and by the
	// published command catalog, and none of them embeds it in HTML, so the escape buys
	// nothing and costs legibility. Every other encoder in the tree passes
	// false already.
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(entries); err != nil {
		var tb textbuf.Buffer
		tb.Str("error: ").Err(err).Byte('\n')
		diag.Write(tb.Bytes()) //nolint:errcheck // a best-effort diagnostic
		return 1
	}
	return rw.ExitCode()
}

// printCommandVerbose writes a detailed entry for each command through rw.
func printCommandVerbose(rw *helpfmt.RenderWriter, diag io.Writer, entries []commandEntry) {
	if len(entries) == 0 {
		io.WriteString(diag, "no commands found\n") //nolint:errcheck // a best-effort diagnostic
		return
	}

	var tb textbuf.Buffer
	tb.SetColor(slogutil.UseColor(os.Stdout))
	c := textbuf.C

	for i := range entries {
		e := &entries[i]
		if i > 0 {
			rw.Line("")
		}
		// Command path
		tb.Reset().Colored(c.BoldCyan).Str(e.Path).Colored(c.Reset)
		rw.Line(tb.Slice())

		// The summary on one line, then the long explanation under it.
		desc := e.ShortHelp
		if desc == "" {
			desc = "-"
		}
		tb.Reset().Str("  ").Str(desc)
		rw.Line(tb.Slice())
		if e.Description != "" {
			for line := range strings.SplitSeq(e.Description, "\n") {
				tb.Reset().Str("  ").Str(line)
				rw.Line(tb.Slice())
			}
		}

		// Mode, wire method, backend, task support
		tb.Reset().Str("  ").Colored(c.Dim).Str("mode: ").Str(e.Mode).Colored(c.Reset)
		if e.WireMethod != "" {
			tb.Str("  ").Colored(c.Dim).Str("wire: ").Str(e.WireMethod).Colored(c.Reset)
		}
		rw.Line(tb.Slice())

		if len(e.Backend) > 0 {
			tb.Reset().Str("  ").Colored(c.BrightYellow).Str("backend: ").Colored(c.Reset).Str(textbuf.Join(e.Backend, ", "))
			rw.Line(tb.Slice())
		}

		if e.TaskSupport != "" {
			tb.Reset().Str("  ").Colored(c.Dim).Str("task-support: ").Str(e.TaskSupport).Colored(c.Reset)
			rw.Line(tb.Slice())
		}

		// Arguments
		if len(e.Args) > 0 {
			tb.Reset().Str("  ").Colored(c.BrightYellow).Str("arguments:").Colored(c.Reset)
			rw.Line(tb.Slice())
			for _, a := range e.Args {
				tb.Reset().Str("    ").Str(a.Name).Str(" (").Str(a.Type).Str(")")
				if a.Mandatory {
					tb.Str(" REQUIRED")
				}
				rw.Line(tb.Slice())
				if len(a.Values) > 0 {
					tb.Reset().Str("      values: ").Str(textbuf.Join(a.Values, ", "))
					rw.Line(tb.Slice())
				}
			}
		}

		// Pipes
		if len(e.PipeOperations) > 0 {
			tb.Reset().Str("  ").Colored(c.BrightYellow).Str("pipes:").Colored(c.Reset)
			rw.Line(tb.Slice())
			always := operationNames(e.PipeOperations, command.PipeAvailableAlways, false)
			withRows := operationNames(e.PipeOperations, command.PipeAvailableWithRows, false)
			if len(always) > 0 {
				tb.Reset().Str("    always: ").Colored(c.Dim).Join(always, ", ").Colored(c.Reset)
				rw.Line(tb.Slice())
			}
			if len(withRows) > 0 {
				label := "    when the answer has rows: "
				if e.AnswerShape != "" {
					label = "    on its rows: "
				}
				tb.Reset().Str(label).Colored(c.Dim).Join(withRows, ", ").Colored(c.Reset)
				rw.Line(tb.Slice())
			}
			if streaming := operationNames(e.PipeOperations, command.PipeAvailableWhenStreaming, false); len(streaming) > 0 {
				tb.Reset().Str("    while the command keeps answering: ").Colored(c.Dim).
					Join(streaming, ", ").Colored(c.Reset)
				rw.Line(tb.Slice())
			}
			if local := operationNames(e.PipeOperations, "", true); len(local) > 0 {
				tb.Reset().Str("    local process only: ").Colored(c.Dim).
					Join(local, ", ").Colored(c.Reset)
				rw.Line(tb.Slice())
			}
			printOwnOperations(rw, &tb, e.PipeOperations)
		}

		// Subcommands
		if len(e.Subcommands) > 0 {
			tb.Reset().Str("  ").Colored(c.BrightYellow).Str("subcommands: ").Colored(c.Reset).Str(textbuf.Join(e.Subcommands, ", "))
			rw.Line(tb.Slice())
		}
	}
	rw.Line("")
	tb.Reset().Int(int64(len(entries))).Str(" commands")
	rw.Line(tb.Slice())
}

// printOwnOperations writes one line per alias, override and own operation of
// a command: each has a description of its own, which the grouped generic
// lines do not repeat.
func printOwnOperations(rw *helpfmt.RenderWriter, tb *textbuf.Buffer, operations []command.PipeOperationEntry) {
	c := textbuf.C
	for _, operation := range operations {
		switch operation.Source {
		case command.PipeSourceAlias:
			tb.Reset().Str("    ").Str(operation.Name).Str("  ").Colored(c.Dim).
				Str(operation.Description).Str(" (= ").Str(operation.Expansion).Str(")").Colored(c.Reset)
		case command.PipeSourceOverride, command.PipeSourceCommand:
			tb.Reset().Str("    ").Str(operation.Name)
			if operation.Argument != command.ArgNone.String() {
				tb.Str(" <value>")
			}
			tb.Str("  ").Colored(c.Dim).Str(operation.Description).Colored(c.Reset)
		default:
			// A generic is printed in the grouped lines.
			continue
		}
		rw.Line(tb.Slice())
	}
}

// printCommandTable writes entries as a human-readable table through rw.
func printCommandTable(rw *helpfmt.RenderWriter, diag io.Writer, entries []commandEntry) {
	if len(entries) == 0 {
		io.WriteString(diag, "no commands found\n") //nolint:errcheck // a best-effort diagnostic
		return
	}

	width := 0
	for i := range entries {
		if n := len(entries[i].Path); n > width {
			width = n
		}
	}
	if width < 16 {
		width = 16
	}

	var tb textbuf.Buffer
	tb.SetColor(slogutil.UseColor(os.Stdout))
	c := textbuf.C
	for i := range entries {
		e := &entries[i]
		// The summary is declared one line long, so the row prints it whole.
		desc := e.ShortHelp
		if desc == "" {
			desc = "-"
		}
		pad := strings.Repeat(" ", width-len(e.Path))
		tb.Reset().Str("  ").Colored(c.BoldCyan).Str(e.Path).Str(pad).Colored(c.Reset).Str("  ").Str(desc)
		rw.Line(tb.Slice())
	}
	rw.Line("")
	rw.Line(tb.Reset().Int(int64(len(entries))).Str(" commands").Slice())
}
