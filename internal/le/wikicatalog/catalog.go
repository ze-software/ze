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
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

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
	Path          string     `json:"path"`
	Description   string     `json:"description,omitempty"`
	Mode          string     `json:"mode"`
	WireMethod    string     `json:"wire-method,omitempty"`
	Backend       []string   `json:"backend,omitempty"`
	TaskSupport   string     `json:"task-support,omitempty"`
	Args          []Argument `json:"args,omitempty"`
	Pipes         []Pipe     `json:"pipes,omitempty"`
	Operators     []Operator `json:"operators,omitempty"`
	AnswerShape   string     `json:"answer-shape,omitempty"`
	AddressFields []string   `json:"address-fields,omitempty"`
	Aliases       []Alias    `json:"pipe-aliases,omitempty"`
	Subcommands   []string   `json:"subcommands,omitempty"`
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
				entry.Args = extractArgs(node)
				entry.Subcommands = extractSubcommands(node)
				entry.Backend = node.Backend
				entry.TaskSupport = node.TaskSupport
			}
			entry.Operators, entry.AnswerShape = operatorsFor(cliPath)
			entry.AddressFields = command.AddressFieldsForCommand(cliPath)
			entry.Aliases = aliasesFor(cliPath)
			entry.Pipes = pipesFor(cliPath)
			entries = append(entries, entry)
			seen[cliPath] = true
		}
	}

	// These four local handlers live in cmd/ze's main package, which an
	// internal package cannot import. The doc-drift producer comparison checks
	// every field against the live registry and rejects any drift here.
	builtins := []Entry{
		{Path: "help ai", Description: "AI reference generated from the binary. Sections: cli, api, mcp, dispatch, all (add --json).", Mode: modeOffline},
		{Path: "help command", Description: "List every command with its description. Use a filter to narrow the list.", Mode: modeOffline},
		{Path: "show version", Description: "Show the running Ze version and build date", Mode: modeOffline},
		{Path: "update serve", Description: "Run a local update server for firmware checks", Mode: modeOffline},
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
			Mode:        mode,
		})
		seen[local.Path] = true
	}

	slices.SortFunc(entries, func(left, right Entry) int {
		return strings.Compare(left.Path, right.Path)
	})
	return entries
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

func operatorsFor(path string) ([]Operator, string) {
	if plainLocalOnly(path) {
		return nil, ""
	}
	shape, declared := command.ShapeForCommand(path)
	hasAddress := len(command.AddressFieldsForCommand(path)) > 0
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
		case declared && catalog.Applies(shape):
			operator.Available = modeAlways
		case declared:
			continue
		default:
			operator.Available = "with-rows"
		}
		operators = append(operators, operator)
	}
	if !declared {
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

func aliasesFor(path string) []Alias {
	declared := command.AliasesForCommand(path)
	if len(declared) == 0 {
		return nil
	}
	aliases := make([]Alias, 0, len(declared))
	for _, alias := range declared {
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
