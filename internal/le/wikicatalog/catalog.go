// Design: docs/architecture/api/commands.md -- the live command catalog
//
// Package wikicatalog generates the wiki's command catalog directly from the
// product registries. It never starts ze: the YANG command tree, local command
// registry, pipe catalog, and answer-shape declarations are already Go data in
// this process.
package wikicatalog

import (
	"slices"
	"strings"

	// The catalog describes the product, so it loads the product composition
	// root. This direction is deliberate: le may introspect ze, while ze never
	// links an internal/le package.
	_ "github.com/ze-software/ze/internal/component/plugin/all"

	cli "github.com/ze-software/ze/internal/component/cli/client"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
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
type Argument struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Values    []string `json:"values,omitempty"`
	Mandatory bool     `json:"mandatory,omitempty"`
}

// Pipe describes one command-specific pipe filter.
type Pipe struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	TakesArg    bool   `json:"takes-arg,omitempty"`
}

// Operator describes one shared pipe operator as it applies to one command.
type Operator struct {
	Name        string `json:"name"`
	Class       string `json:"class"`
	Available   string `json:"available"`
	LocalOnly   bool   `json:"local-only,omitempty"`
	Description string `json:"description"`
}

// Alias describes one named pipe chain.
type Alias struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Expansion   string `json:"expansion"`
}

// Entry is one command in the product catalog.
type Entry struct {
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
	// LongHelp is the command's own explanation, from ze:help. Description is
	// the one-line summary beside it: two declarations, never one authored
	// string cut in two, so each surface reads the half it renders.
	LongHelp      string     `json:"long-help,omitempty"`
	Mode          string     `json:"mode"`
	WireMethod    string     `json:"wire-method,omitempty"`
	Backend       []string   `json:"backend,omitempty"`
	TaskSupport   string     `json:"task-support,omitempty"`
	Args          []Argument `json:"args,omitempty"`
	Pipes         []Pipe     `json:"pipes,omitempty"`
	Operators     []Operator `json:"operators,omitempty"`
	AnswerShape   string     `json:"answer-shape,omitempty"`
	AddressFields []string   `json:"address-fields,omitempty"`
	// ColumnOrders are the JSON keys of the answer's records, in the order a
	// person reads them, one list per record shape the command renders. A
	// command that renders an outer record and a list of rows declares two
	// orders, so a flat list could state neither.
	ColumnOrders [][]string `json:"column-orders,omitempty"`
	Aliases      []Alias    `json:"pipe-aliases,omitempty"`
	// Usage is the invocation form generated from the command model, and
	// Grammar is the same form as an ordered token list. Both come from one
	// producer (internal/component/command, Usage), so the two cannot disagree.
	Usage       string               `json:"usage,omitempty"`
	Grammar     []command.UsageToken `json:"grammar,omitempty"`
	Subcommands []string             `json:"subcommands,omitempty"`
}

// Collect answers the same sorted product inventory as ze help command. It
// joins exported registry and catalog APIs in-process rather than parsing a
// child process's JSON output.
func Collect() []Entry {
	var entries []Entry
	seen := make(map[string]bool)

	tree := cli.YANGCommandTree()
	wireToPaths := cli.WireToPaths()
	for wireMethod, cliPaths := range wireToPaths {
		for _, cliPath := range cliPaths {
			if seen[cliPath] {

				continue
			}
			mode := "daemon"
			if pluginserver.IsReadOnlyPath(cliPath) {
				mode = "read-only"
			}
			node := findNode(tree, cliPath)
			entry := Entry{Path: cliPath, Mode: mode, WireMethod: wireMethod}
			if node != nil {
				entry.Description = node.Description
				entry.LongHelp = node.LongHelp
				entry.Args = extractArgs(node)
				entry.Grammar = command.Usage(strings.Fields(cliPath), node)
				entry.Usage = command.UsageLine(entry.Grammar)
				entry.Subcommands = extractSubcommands(node)
				entry.Backend = node.Backend
				entry.TaskSupport = node.TaskSupport
			}
			declared := command.DeclaredForCommand(cliPath)
			entry.Operators, entry.AnswerShape = operatorsFor(cliPath, declared)
			entry.AddressFields = declared.AddressFields
			entry.ColumnOrders = command.ColumnNames(declared.Columns)
			entry.Aliases = aliasesFor(declared)
			entry.Pipes = pipesFor(cliPath)
			entries = append(entries, entry)
			seen[cliPath] = true
		}
	}

	// These four local handlers live in cmd/ze's main package, which an
	// internal package cannot import. The doc-drift producer comparison checks
	// every field against the live registry and rejects any drift here.
	builtins := []Entry{
		{
			Path:        "help ai",
			Description: "Print the agent reference this binary builds from its own registries.",
			LongHelp: "The sections are cli, api, mcp, dispatch and all, and the answer renders as JSON " +
				"for a program to read.",
			Mode: modeOffline,
		},
		{
			Path:        "help command",
			Description: "List every command this binary carries with its summary.",
			LongHelp: "A filter word keeps the commands whose path holds it, and the answer renders as " +
				"JSON for a program to read.",
			Mode: modeOffline,
		},
		{Path: "show version", Description: "Show the running Ze version and build date", Mode: modeOffline},
		{
			Path:        "update serve",
			Description: "Serve this binary and its version manifest for update checks.",
			LongHelp: "The server answers a version manifest, the running binary and its SHA-256 digest. " +
				"It is meant for build infrastructure rather than for a router in production.",
			Mode: modeOffline,
		},
	}
	for index := range builtins {
		if seen[builtins[index].Path] {
			continue
		}
		entries = append(entries, builtins[index])
		seen[builtins[index].Path] = true
	}
	for _, local := range registry.ListLocal() {
		// leroot registers every development command under "le ". The wiki is
		// the ze product catalog, so those process-local registrations are not
		// part of the old ze help command inventory being replaced.
		if strings.HasPrefix(local.Path, "le ") || seen[local.Path] {
			continue
		}
		mode := local.Meta.Mode
		if mode == "" {
			mode = modeOffline
		}
		entries = append(entries, Entry{
			Path:        local.Path,
			Description: local.Meta.Description,
			LongHelp:    local.Meta.LongHelp,
			Mode:        mode,
		})
		seen[local.Path] = true
	}

	entries = appendPluginCommands(entries, seen, tree)

	slices.SortFunc(entries, func(left, right Entry) int {
		return strings.Compare(left.Path, right.Path)
	})
	return entries
}

// appendPluginCommands adds every command a plugin declares on its
// registration that nothing above already named.
//
// It is the twin of appendPluginCommands in cmd/ze/help_command.go and MUST
// stay its twin: the two catalogs are compared field for field
// (internal/le/docvalid, compareWikiCatalogProducer). A plugin's command is
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
func appendPluginCommands(entries []Entry, seen map[string]bool, tree *command.Node) []Entry {
	for _, registration := range pluginregistry.All() {
		for index := range registration.Commands {
			decl := &registration.Commands[index]
			if decl.Name == "" || decl.Hidden || seen[decl.Name] {
				continue
			}
			mode := "daemon"
			if pluginserver.IsReadOnlyPath(decl.Name) {
				mode = "read-only"
			}
			entry := Entry{
				Path:        decl.Name,
				Description: decl.Description,
				LongHelp:    decl.LongHelp,
				Mode:        mode,
				Usage:       pluginUsage(decl.Name, decl.Args),
			}
			if node := findNode(tree, decl.Name); node != nil {
				entry.Description = node.Description
				entry.LongHelp = node.LongHelp
				entry.Args = extractArgs(node)
				entry.Subcommands = extractSubcommands(node)
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
			entry.Operators, entry.AnswerShape = operatorsFor(decl.Name, declared)
			entry.AddressFields = declared.AddressFields
			entry.ColumnOrders = command.ColumnNames(declared.Columns)
			entry.Aliases = aliasesFor(declared)
			entry.Pipes = pipesFor(decl.Name)
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
	for _, definition := range node.ArgDefs {
		arg := Argument{
			Name:      definition.Name,
			Type:      argumentKind(definition.Kind),
			Mandatory: definition.Mandatory,
		}
		if len(definition.EnumValues) > 0 {
			arg.Values = definition.EnumValues
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
	default:
		return "string"
	}
}

func extractSubcommands(node *command.Node) []string {
	if len(node.Children) == 0 {
		return nil
	}
	names := make([]string, 0, len(node.Children))
	for name := range node.Children {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// operatorsFor answers what one command supports, derived from the operator
// catalog and the declaration the command carries.
//
// The declaration is passed in rather than looked up, so one lookup answers the
// operators, the answer shape, the column orders and the address fields. That
// lookup reads a plugin's registration for a command no process here ever ran
// (internal/component/command, DeclaredForCommand), and `ze help command
// --json` reads the same function, so the two catalogs agree by derivation
// rather than by comparison.
func operatorsFor(path string, declared command.Declared) ([]Operator, string) {
	if plainLocalOnly(path) {
		return nil, ""
	}
	shape := declared.Shape
	hasAddress := len(declared.AddressFields) > 0
	operators := make([]Operator, 0, 16)
	for _, catalog := range command.PipeOperatorCatalog() {
		if catalog.NeedsAddressField && !hasAddress {
			continue
		}
		operator := Operator{
			Name:        catalog.Name,
			Class:       catalog.Class.String(),
			LocalOnly:   catalog.LocalOnly,
			Description: catalog.Description,
		}
		switch {
		case catalog.Class == command.ClassStream:
			operator.Available = "when-streaming"
		case catalog.Class == command.ClassGlobal:
			operator.Available = modeAlways
		case declared.ShapeDeclared && catalog.Applies(shape):
			operator.Available = modeAlways
		case declared.ShapeDeclared:
			continue
		default:
			operator.Available = "with-rows"
		}
		operators = append(operators, operator)
	}
	if !declared.ShapeDeclared {
		return operators, ""
	}
	return operators, shape.String()
}

func plainLocalOnly(path string) bool {
	if !registry.HasLocal(path) || command.HasLocalData(path) {
		return false
	}
	wireToPaths := cli.WireToPaths()
	for _, registration := range pluginserver.AllBuiltinRPCs() {
		if registration.Handler != nil && slices.Contains(wireToPaths[registration.WireMethod], path) {
			return false
		}
	}
	return true
}

// aliasesFor answers the chains a command names, from the declaration the
// catalog already resolved for it. The declaration carries a registered
// plugin's aliases beside the alias registry's, because this process starts no
// plugin and so receives no Stage 1 message.
func aliasesFor(declared command.Declared) []Alias {
	if len(declared.Aliases) == 0 {
		return nil
	}
	aliases := make([]Alias, 0, len(declared.Aliases))
	for _, alias := range declared.Aliases {
		aliases = append(aliases, Alias{
			Name: alias.Name, Description: alias.Description, Expansion: alias.Expansion,
		})
	}
	return aliases
}

func pipesFor(path string) []Pipe {
	filters := command.PipeFiltersForCommand(path)
	if len(filters) == 0 {
		return nil
	}
	pipes := make([]Pipe, 0, len(filters))
	for _, filter := range filters {
		pipes = append(pipes, Pipe{
			Name: filter.Name, Description: filter.Description, TakesArg: filter.TakesArg,
		})
	}
	return pipes
}

// The command availability modes this catalog publishes.
const (
	modeOffline = "offline"
	modeAlways  = "always"
)
