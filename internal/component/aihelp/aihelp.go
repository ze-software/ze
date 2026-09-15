// Design: docs/features/ai-first.md -- AI reference assembly (single source of truth)
//
// Package aihelp assembles the machine-readable "AI reference" for the running
// binary: CLI subcommands, daemon API RPCs (with dispatch keys), loaded plugins,
// address families, and config services. Every field is derived from the live
// registries and YANG schemas, so the reference always matches this binary.
//
// It is the single source of truth shared by two surfaces:
//   - the CLI: `ze help ai` / `ze help ai --json` (cmd/ze/help_ai.go)
//   - the MCP server: the `ze_reference` tool (internal/component/mcp)
//
// The CLI renders these structures as text; Build returns the JSON shape.
package aihelp

import (
	"slices"
	"sort"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/component/command"
	cmdregistry "github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// CLICommand describes a top-level `ze <command>` subcommand. The json tags make
// it serve both the text renderer and Reference.Commands.
type CLICommand struct {
	Name      string `json:"name"`
	Mode      string `json:"mode"` // "offline"/"read-only", "daemon", or "setup"
	ShortHelp string `json:"short-help"`
	Subs      string `json:"subs,omitempty"`
}

// ServiceLeaf is one config leaf of a YANG environment service (text rendering).
type ServiceLeaf struct {
	Name      string
	Type      string
	Default   string
	ShortHelp string
}

// Service is a YANG "environment" container, e.g. web, mcp, looking-glass.
type Service struct {
	Name      string
	ShortHelp string
	Leaves    []ServiceLeaf
	EnvVars   []string // registered ze.* env vars for this service
}

// Reference is the machine-readable AI reference (the `ze help ai --json` shape).
type Reference struct {
	Commands      []CLICommand      `json:"commands"`
	RPCs          []RPC             `json:"rpcs"`
	Notifications []Notification    `json:"notifications,omitempty"`
	DispatchKeys  map[string]string `json:"dispatch-keys"`
	Plugins       []Plugin          `json:"plugins"`
	Families      []string          `json:"families"`
	Services      []ServiceRef      `json:"services"`
}

// RPC is one daemon API endpoint (wire method) with its two declared help
// texts: the one-line summary, and the long explanation where one was written.
// The keys match the pair `ze help command --json` carries for a command.
// Input and Output list the rpc's leaves, each with the same pair.
type RPC struct {
	WireMethod  string `json:"wire-method"`
	ShortHelp   string `json:"short-help,omitempty"`
	Description string `json:"description,omitempty"`
	Input       []Leaf `json:"input,omitempty"`
	Output      []Leaf `json:"output,omitempty"`
}

// Leaf is one rpc input, rpc output or notification leaf with its two
// declared texts. Neither is derived from the other, and an undeclared text
// writes no key.
type Leaf struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Mandatory   bool   `json:"mandatory,omitempty"`
	ShortHelp   string `json:"short-help,omitempty"`
	Description string `json:"description,omitempty"`
}

// Notification is one daemon event (wire method) with its summary and the
// leaves its body carries.
type Notification struct {
	WireMethod string `json:"wire-method"`
	ShortHelp  string `json:"short-help,omitempty"`
	Leaves     []Leaf `json:"leaves,omitempty"`
}

// leavesOf converts the schema registry's leaf metadata to the published
// shape, keeping both texts.
func leavesOf(metas []yang.LeafMeta) []Leaf {
	if len(metas) == 0 {
		return nil
	}
	leaves := make([]Leaf, 0, len(metas))
	for _, meta := range metas {
		leaves = append(leaves, Leaf{
			Name:        meta.Name,
			Type:        meta.Type,
			Mandatory:   meta.Mandatory,
			ShortHelp:   meta.ShortHelp,
			Description: meta.Description,
		})
	}
	return leaves
}

// Plugin is one loaded plugin with the address families it handles.
type Plugin struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Families    []string `json:"families,omitempty"`
}

// ServiceRef is the flattened service shape used in the JSON reference.
type ServiceRef struct {
	Name   string   `json:"name"`
	Leaves []string `json:"leaves,omitempty"`
}

// CLISubcommands returns the CLI subcommand tree.
//
// Two dynamic sources, no static list:
//  1. YANG verb tree (show, set, del, update, route-refresh, ...).
//     These are the daemon-side dispatch verbs.
//  2. cmdregistry.ListRoot(): top-level `ze <name>` subcommands registered at
//     startup from cmd/ze/main.go.
//
// Verb commands that also appear as root commands (duplicates) are
// de-duplicated: the root-command metadata wins because it carries a richer
// description and sub-path hint.
func CLISubcommands() []CLICommand {
	seen := map[string]bool{}
	var cmds []CLICommand

	loader, err := yang.DefaultLoader()
	var yangTree *command.Node
	if err == nil {
		yangTree = yang.BuildCommandTree(loader)
	}
	if yangTree != nil {
		for _, name := range sortedChildren(yangTree) {
			child := yangTree.Children[name]
			desc := child.ShortHelp
			if desc == "" {
				var tb textbuf.Buffer
				desc = tb.Str(name).Str(" commands").String()
			}
			// The same predicate authorization uses, so the mode this
			// reference publishes and the section a command lands in cannot
			// disagree. Reading the verb registry alone answered "daemon" for
			// the resolve, help, system, plugin and rib roots, which an
			// operator holding read-only access can run.
			mode := "daemon"
			if pluginserver.IsReadOnlyPath(name) {
				mode = "read-only"
			}
			var tb textbuf.Buffer
			cmds = append(cmds, CLICommand{
				Name:      name,
				Mode:      mode,
				ShortHelp: desc,
				Subs:      tb.Str("ze ").Str(name).Str(" help").String(),
			})
			seen[name] = true
		}
	}

	for _, rc := range cmdregistry.ListRoot() {
		if seen[rc.Name] {
			continue // YANG verb already covered the slot
		}
		cmds = append(cmds, CLICommand{
			Name:      rc.Name,
			Mode:      rc.Meta.Mode,
			ShortHelp: rc.Meta.ShortHelp,
			Subs:      rc.Meta.ResolveSubs(),
		})
	}

	return cmds
}

// sortedChildren returns sorted child names of a command node.
func sortedChildren(node *command.Node) []string {
	names := make([]string, 0, len(node.Children))
	for name := range node.Children {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// SchemaRegistry builds a schema registry with YANG RPC metadata.
func SchemaRegistry() *pluginserver.SchemaRegistry {
	schemaReg := pluginserver.NewSchemaRegistry()

	loader := yang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		return schemaReg
	}
	if err := loader.LoadRegistered(); err != nil {
		return schemaReg
	}
	if err := loader.Resolve(); err != nil {
		return schemaReg
	}

	for _, name := range loader.APIModuleNames() {
		rpcs := yang.ExtractRPCs(loader, name)
		_ = schemaReg.RegisterRPCs(name, rpcs)
	}

	return schemaReg
}

// Services walks registered YANG conf modules for environment containers.
func Services() []Service {
	loader := yang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		return nil
	}
	if err := loader.LoadRegistered(); err != nil {
		return nil
	}
	if err := loader.Resolve(); err != nil {
		return nil
	}

	// Build env var groups keyed by second segment: "ze.web.listen" -> "web".
	envByPrefix := make(map[string][]string)
	for _, e := range env.Entries() {
		parts := strings.SplitN(e.Key, ".", 3) //nolint:mnd // ze.<group>.<leaf>
		if len(parts) >= 2 {
			envByPrefix[parts[1]] = append(envByPrefix[parts[1]], e.Key)
		}
	}

	// Map YANG service container names to env prefixes. Most match directly
	// (web->web, mcp->mcp, ssh->ssh); abbreviations are found by checking leaf
	// names against env var leaf names.
	matchEnvVars := func(svcName string, leafNames []string) []string {
		if vars, ok := envByPrefix[svcName]; ok {
			return vars
		}
		leafSet := make(map[string]bool, len(leafNames))
		for _, n := range leafNames {
			leafSet[n] = true
		}
		var bestPrefix string
		var bestCount int
		for prefix, vars := range envByPrefix {
			count := 0
			for _, v := range vars {
				parts := strings.SplitN(v, ".", 3)        //nolint:mnd // ze.<group>.<leaf>
				if len(parts) == 3 && leafSet[parts[2]] { //nolint:mnd // ze.<group>.<leaf>
					count++
				}
			}
			if count > bestCount {
				bestCount = count
				bestPrefix = prefix
			}
		}
		if bestCount > 0 {
			return envByPrefix[bestPrefix]
		}
		return nil
	}

	var services []Service

	for _, mod := range yang.Modules() {
		if !strings.HasSuffix(mod.Name, "-conf.yang") {
			continue
		}
		modName := strings.TrimSuffix(mod.Name, ".yang")
		entry := loader.GetEntry(modName)
		if entry == nil || entry.Dir == nil {
			continue
		}

		envEntry, ok := entry.Dir["environment"]
		if !ok || envEntry.Dir == nil {
			continue
		}

		for svcName, svcEntry := range envEntry.Dir {
			if svcEntry.Kind != gyang.DirectoryEntry {
				continue
			}

			svc := Service{
				Name:      svcName,
				ShortHelp: yang.GetHelpExtension(svcEntry.Exts), // the ze:help summary
			}

			leafNames := make([]string, 0, len(svcEntry.Dir))
			for name := range svcEntry.Dir {
				leafNames = append(leafNames, name)
			}
			slices.Sort(leafNames)

			for _, leafName := range leafNames {
				child := svcEntry.Dir[leafName]
				leaf := ServiceLeaf{
					Name:      leafName,
					ShortHelp: yang.GetHelpExtension(child.Exts), // the ze:help summary
				}
				if child.Type != nil {
					leaf.Type = child.Type.Name
				}
				if len(child.Default) > 0 {
					leaf.Default = child.Default[0]
				}
				svc.Leaves = append(svc.Leaves, leaf)
			}

			svc.EnvVars = matchEnvVars(svcName, leafNames)
			slices.Sort(svc.EnvVars)

			services = append(services, svc)
		}
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})

	return services
}

// Build assembles the full machine-readable reference from the live registries
// and YANG schemas. The result is identical to `ze help ai --json`.
func Build() Reference {
	ref := Reference{Commands: CLISubcommands()}

	schemaReg := SchemaRegistry()
	for _, rpc := range schemaReg.ListRPCs("") {
		ref.RPCs = append(ref.RPCs, RPC{
			WireMethod:  rpc.WireMethod,
			ShortHelp:   rpc.ShortHelp,
			Description: rpc.Description,
			Input:       leavesOf(rpc.Input),
			Output:      leavesOf(rpc.Output),
		})
	}
	for _, notif := range schemaReg.ListNotifications("") {
		ref.Notifications = append(ref.Notifications, Notification{
			WireMethod: notif.WireMethod,
			ShortHelp:  notif.ShortHelp,
			Leaves:     leavesOf(notif.Leaves),
		})
	}
	for _, brpc := range pluginserver.AllBuiltinRPCs() {
		ref.RPCs = append(ref.RPCs, RPC{WireMethod: brpc.WireMethod})
	}

	if loader, err := yang.DefaultLoader(); err == nil {
		ref.DispatchKeys = yang.WireMethodToPath(loader)
	}
	if ref.DispatchKeys == nil {
		ref.DispatchKeys = map[string]string{}
	}

	seen := make(map[string]bool)
	for _, r := range registry.All() {
		ref.Plugins = append(ref.Plugins, Plugin{Name: r.Name, Description: r.Description, Families: r.Families})
		for _, f := range r.Families {
			if !seen[f] {
				seen[f] = true
				ref.Families = append(ref.Families, f)
			}
		}
	}
	slices.Sort(ref.Families)

	for _, svc := range Services() {
		leafNames := make([]string, len(svc.Leaves))
		for i, l := range svc.Leaves {
			leafNames[i] = l.Name
		}
		ref.Services = append(ref.Services, ServiceRef{Name: svc.Name, Leaves: leafNames})
	}

	return ref
}
