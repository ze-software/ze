// Design: docs/architecture/api/commands.md -- the live command catalog
//
// Package clicatalog generates the wiki's command catalog directly from the
// product registries. It never starts ze: the YANG command tree, local command
// registry, pipe catalog, and answer-shape declarations are already Go data in
// this process.
package clicatalog

import (
	"slices"
	"strings"

	// The catalog describes the product, so it loads the product composition
	// root. This direction is deliberate: le may introspect ze, while ze never
	// links an internal/le package.
	_ "github.com/ze-software/ze/internal/component/plugin/all"

	cli "github.com/ze-software/ze/internal/component/cli/client"
	"github.com/ze-software/ze/internal/component/command"
	_ "github.com/ze-software/ze/internal/component/doctor"
	pluginregistry "github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"

	// Offline command packages are the direct registration half of
	// cmd/ze/ze_core_dispatch.go. The live product and this catalog must compose
	// the same local command registry.
	_ "github.com/ze-software/ze/internal/component/aaa/all"
	_ "github.com/ze-software/ze/internal/component/config/cli"
	_ "github.com/ze-software/ze/internal/component/config/schema/cli"
	_ "github.com/ze-software/ze/internal/component/config/storage/cli"
	_ "github.com/ze-software/ze/internal/component/config/yang/cli"
	_ "github.com/ze-software/ze/internal/component/plugin/cli"
	_ "github.com/ze-software/ze/internal/component/resolve/cli"
	_ "github.com/ze-software/ze/internal/component/traffic/cli"
	_ "github.com/ze-software/ze/internal/plugins/completion"
	_ "github.com/ze-software/ze/internal/plugins/crashes"
	_ "github.com/ze-software/ze/internal/plugins/debug"
	_ "github.com/ze-software/ze/internal/plugins/diag"
	_ "github.com/ze-software/ze/internal/plugins/explain"
	_ "github.com/ze-software/ze/internal/plugins/host"
	_ "github.com/ze-software/ze/internal/plugins/init"
	_ "github.com/ze-software/ze/internal/plugins/passwd"
	_ "github.com/ze-software/ze/internal/plugins/signal"
	_ "github.com/ze-software/ze/internal/plugins/skills"
	_ "github.com/ze-software/ze/internal/plugins/support"
)

// Argument describes one typed argument in the published command grammar.
// ShortHelp is the leaf's ze:help summary and Description its long
// explanation, the two keys `ze help command --json` carries for an argument.
// Neither is derived from the other, and an undeclared text is absent.
type Argument struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Values      []string `json:"values,omitempty"`
	Mandatory   bool     `json:"mandatory,omitempty"`
	ShortHelp   string   `json:"short-help,omitempty"`
	Description string   `json:"description,omitempty"`
}

// Entry is one command in the product catalog.
type Entry struct {
	Path      string `json:"path"`
	ShortHelp string `json:"short-help,omitempty"`
	// Description is the command's own explanation, from the YANG description.
	// Description is the one-line summary from the ze:help beside it: two
	// declarations, never one authored
	// string cut in two, so each surface reads the half it renders.
	Description string     `json:"description,omitempty"`
	Mode        string     `json:"mode"`
	WireMethod  string     `json:"wire-method,omitempty"`
	Backend     []string   `json:"backend,omitempty"`
	TaskSupport string     `json:"task-support,omitempty"`
	Args        []Argument `json:"args,omitempty"`
	// PipeOperations is everything the command answers to after a pipe, the
	// list `ze help command --json` publishes (command.PipeOperations).
	PipeOperations []command.PipeOperationEntry `json:"pipe-operations,omitempty"`
	AnswerShape    string                       `json:"answer-shape,omitempty"`
	AddressFields  []string                     `json:"address-fields,omitempty"`
	// ColumnOrders are the JSON keys of the answer's records, in the order a
	// person reads them, one list per record shape the command renders. A
	// command that renders an outer record and a list of rows declares two
	// orders, so a flat list could state neither.
	ColumnOrders [][]string `json:"column-orders,omitempty"`
	// Usage is the invocation form generated from the command model, and
	// Grammar is the same form as an ordered token list. Both come from one
	// producer (internal/component/command, Usage), so the two cannot disagree.
	Usage       string               `json:"usage,omitempty"`
	Grammar     []command.UsageToken `json:"grammar,omitempty"`
	Subcommands []string             `json:"subcommands,omitempty"`
}

// Collect answers the same sorted product inventory as ze help command. It
// joins exported registry and catalog APIs in-process rather than parsing a
// child process's JSON output. The error is the YANG loader's refusal of the
// schema: an inventory built from no schema would publish the offline
// commands alone as the whole product.
func Collect() ([]Entry, error) {
	var entries []Entry
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
			// A local handler at the same path (local data, or a local
			// command the daemon also serves) is a second route: the
			// command runs in the caller's process with no daemon too.
			if command.HasLocal(cliPath) {
				mode = withLocalRoute(mode)
			}
			node := findNode(tree, cliPath)
			entry := Entry{Path: cliPath, Mode: mode, WireMethod: wireMethod}
			if node != nil {
				entry.ShortHelp = node.ShortHelp
				entry.Description = node.Description
				entry.Args = extractArgs(node)
				entry.Grammar = command.Usage(strings.Fields(cliPath), node)
				entry.Usage = command.UsageLine(entry.Grammar)
				entry.Subcommands = command.SubcommandNames(node)
				entry.Backend = node.Backend
				entry.TaskSupport = node.TaskSupport
			}
			declared := command.DeclaredForCommand(cliPath)
			entry.PipeOperations, entry.AnswerShape = pipeOperationsFor(cliPath, declared, wireToPaths)
			entry.AddressFields = declared.AddressFields
			entry.ColumnOrders = command.ColumnNames(declared.Columns)
			entries = append(entries, entry)
			seen[cliPath] = true
		}
	}

	builtins := mainPackageLocals()
	for index := range builtins {
		if command.HiddenAtPath(tree, strings.Fields(builtins[index].Path)) {
			continue
		}
		if seen[builtins[index].Path] {
			// The daemon serves this path under its YANG node too, so the
			// entry above carries the daemon route alone: add the local one.
			labelLocalRoute(entries, builtins[index].Path)
			continue
		}
		entries = append(entries, builtins[index])
		seen[builtins[index].Path] = true
	}
	for _, local := range command.ListLocal() {
		// leroot registers every development command under "le ". The wiki is
		// the ze product catalog, so those process-local registrations are not
		// part of the old ze help command inventory being replaced.
		if command.HiddenAtPath(tree, strings.Fields(local.Path)) {
			continue
		}
		if strings.HasPrefix(local.Path, "le ") || seen[local.Path] {
			continue
		}
		mode := local.Meta.Mode
		if mode == "" {
			mode = modeOffline
		}
		entries = append(entries, Entry{
			Path:        local.Path,
			ShortHelp:   local.Meta.ShortHelp,
			Description: local.Meta.Description,
			Mode:        mode,
		})
		seen[local.Path] = true
	}

	entries = appendPluginCommands(entries, seen, tree, wireToPaths)

	slices.SortFunc(entries, func(left, right Entry) int {
		return strings.Compare(left.Path, right.Path)
	})
	return entries, nil
}

// mainPackageLocals answers the four local handlers that live in cmd/ze's main
// package, which an internal package cannot import. The doc-drift producer comparison checks
// every field against the live registry and rejects any drift here.
func mainPackageLocals() []Entry {
	return []Entry{
		{
			Path:      "help ai",
			ShortHelp: "Print the agent reference this binary builds from its own registries.",
			Description: "The sections are cli, api, mcp, dispatch and all, and the answer renders as JSON " +
				"for a program to read.",
			Mode: modeOffline,
		},
		{
			Path:      "help command",
			ShortHelp: "List every command this binary carries with its summary.",
			Description: "A filter word keeps the commands whose path holds it, and the answer renders as " +
				"JSON for a program to read.",
			Mode: modeOffline,
		},
		{Path: "show version", ShortHelp: "Show the running Ze version and build date", Mode: modeOffline},
		{
			Path:      "update serve",
			ShortHelp: "Serve this binary and its version manifest for update checks.",
			Description: "The server answers a version manifest, the running binary and its SHA-256 digest. " +
				"It is meant for build infrastructure rather than for a router in production.",
			Mode: modeOffline,
		},
	}
}

// withLocalRoute labels a command the daemon serves that also runs locally,
// with no daemon: both routes, never the daemon's alone.
func withLocalRoute(daemonMode string) string {
	return daemonMode + ", " + modeOffline
}

// labelLocalRoute adds the local route to the entry at path, which a YANG
// node already labeled with the daemon route. Collect calls it only for a
// path it has seen, so a miss is a defect in Collect.
func labelLocalRoute(entries []Entry, path string) {
	for index := range entries {
		if entries[index].Path != path {
			continue
		}
		if !strings.HasSuffix(entries[index].Mode, ", "+modeOffline) {
			entries[index].Mode = withLocalRoute(entries[index].Mode)
		}
		return
	}
	panic("BUG: labelLocalRoute: Collect saw " + path + " but holds no entry for it")
}

// appendPluginCommands adds every command a plugin declares on its
// registration that nothing above already named.
//
// It is the twin of appendPluginCommands in cmd/ze/help_command.go and MUST
// stay its twin: the two catalogs are compared field for field
// (internal/le/doc/yangcontract, compareWikiCatalogProducer). A plugin's command is
// dispatched through the plugin, so it reaches neither the YANG command tree
// nor the local command registry, and both catalogs named none of them however
// much each declared.
//
// A HIDDEN declaration is skipped, because the daemon already keeps one out of
// VisibleCommandEntries and out of completion, and this catalog is what the
// website and the wiki publish to an operator.
//
// The YANG node wins wherever one exists. A plugin command can be modeled and
// still carry no wire method, `show vrrp interface` among them, so it arrives
// with an authored summary, long help and grammar already written.
func appendPluginCommands(entries []Entry, seen map[string]bool, tree *command.Node, wireToPaths map[string][]string) []Entry {
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
			entry := Entry{
				Path:        decl.Name,
				ShortHelp:   decl.ShortHelp,
				Description: decl.Description,
				Mode:        mode,
				Usage:       pluginUsage(decl.Name, decl.Args),
			}
			if node := findNode(tree, decl.Name); node != nil {
				entry.ShortHelp = node.ShortHelp
				entry.Description = node.Description
				entry.Args = extractArgs(node)
				entry.Subcommands = command.SubcommandNames(node)
				entry.Backend = node.Backend
				entry.TaskSupport = node.TaskSupport
				// Usage answers nil for a node carrying no wire method, which
				// every plugin-dispatched node does, so the declaration's own
				// form is kept rather than overwritten with an empty line.
				if grammar := command.Usage(strings.Fields(decl.Name), node); len(grammar) > 0 {
					entry.Grammar = grammar
					entry.Usage = command.UsageLine(grammar)
				}
			}
			declared := command.DeclaredForCommand(decl.Name)
			entry.PipeOperations, entry.AnswerShape = pipeOperationsFor(decl.Name, declared, wireToPaths)
			entry.AddressFields = declared.AddressFields
			entry.ColumnOrders = command.ColumnNames(declared.Columns)
			entries = append(entries, entry)
			seen[decl.Name] = true
		}
	}
	return entries
}

// pluginUsage answers the invocation form a plugin declares: the command path,
// then the argument tokens the plugin spells for a reader. Grammar stays empty
// beside it, because a plugin declares its arguments as text and a token list
// built from that text would state kinds nobody declared.
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

func findNode(tree *command.Node, path string) *command.Node {
	if tree == nil {
		return nil
	}
	return command.FindNode(tree, strings.Fields(path))
}

func extractArgs(node *command.Node) []Argument {
	if len(node.ArgDefs) == 0 {
		return nil
	}
	args := make([]Argument, 0, len(node.ArgDefs))
	for i := range node.ArgDefs {
		definition := &node.ArgDefs[i]
		arg := Argument{
			Name:        definition.Name(),
			Type:        argumentKind(definition.Kind()),
			Mandatory:   definition.Mandatory(),
			ShortHelp:   definition.ShortHelp(),
			Description: definition.Description(),
		}
		if values := slices.Collect(definition.EnumValues()); len(values) > 0 {
			arg.Values = values
		}
		args = append(args, arg)
	}
	return args
}

func argumentKind(kind command.ArgKind) string {
	switch kind {
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
		panic("BUG: command catalog has an unknown argument kind")
	}
}

// pipeOperationsFor answers the `pipe-operations` list and the declared answer
// shape of one command, from the declaration the catalog already resolved for
// it. The join is command.PipeOperations, which `ze help command --json` and
// `show command help` read too, so the catalogs agree by derivation rather
// than by comparison. A command no surface pipes publishes neither
// (command.PipeLayerReached). wireToPaths is cli.WireToPaths, fetched once by
// Collect.
func pipeOperationsFor(path string, declared command.Declared, wireToPaths map[string][]string) ([]command.PipeOperationEntry, string) {
	daemonServes := func(candidate string) bool {
		for _, registration := range pluginserver.AllBuiltinRPCs() {
			if registration.Handler != nil && slices.Contains(wireToPaths[registration.WireMethod], candidate) {
				return true
			}
		}
		return false
	}
	if !command.PipeLayerReached(path, daemonServes) {
		return nil, ""
	}
	operations := command.PipeOperations(declared)
	if !declared.ShapeDeclared {
		return operations, ""
	}
	return operations, declared.Shape.String()
}

// The command availability modes this catalog publishes.
const (
	modeOffline = "offline"
)
