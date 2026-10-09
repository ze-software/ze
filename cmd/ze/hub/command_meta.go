// Design: ai/rules/plugins.md -- always-on command metadata
//
// Neutral, always-on command metadata shared by the API and MCP command
// listers. Both surfaces need the same dispatcher traversal + YANG-derived
// metadata (params, task-support, ui-resource); only the OUTPUT type differs
// (api.CommandMeta vs zemcp.CommandInfo). Keeping the traversal here, in a
// neutral hub type, lets MCP be compiled out (//go:build ze_mcp) without
// dropping the API command lister: API adapts commandMeta directly, while the
// gated service_mcp.go wraps the same source as a zemcp.CommandLister.
//
// Before the feature gate this lived in main_servers.go as serverCommandLister
// (returning zemcp.CommandLister), which transitively pinned internal/component/mcp
// into every binary through API's reuse of it.

package hub

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/command"
	yangloader "github.com/ze-software/ze/internal/component/config/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// commandMeta is the neutral, always-on description of one registered command.
// It carries every field BOTH the API and MCP command listers need so neither
// surface has to traverse the dispatcher or load YANG itself. Surface-specific
// adapters convert it to api.CommandMeta / zemcp.CommandInfo.
type commandMeta struct {
	Name string // dispatch path, e.g. "show bgp rib status"
	// ShortHelp is the command's one-line summary, from its YANG ze:help
	// extension or from the plugin's own declaration. Every surface that shows
	// the command on one line reads it.
	ShortHelp string
	// Description is the explanation the command declares with the YANG
	// description statement, or that a
	// plugin sends as CommandDecl.Description. Only a per-command help page reads
	// it: the OpenAPI operation description, and the MCP tool description. It
	// is NEVER read as a summary. Empty means none was declared.
	Description string
	ReadOnly    bool               // true if a read-only command
	Params      []commandParam     // input parameters from YANG RPC (nil = none)
	TaskSupport string             // raw YANG ze:task-support value ("" = optional)
	UIResource  *commandUIResource // YANG ze:ui-resource extension (nil = no UI)
	// TakesSelector is true when Dispatch consumes an inline selector token for
	// this command (`show bgp peer <selector> detail`). Surfaces that BUILD a
	// command string rather than parse one -- MCP -- need it to know whether a
	// selector argument is meaningful at all, and where its value belongs.
	// Derived from the dispatcher's own predicate, never from a name pattern.
	TakesSelector bool
}

// commandParam is one input parameter, neutral counterpart of zemcp.ParamInfo
// and api.ParamMeta.
type commandParam struct {
	Name        string
	Type        string
	ShortHelp   string // the ze:help summary
	Description string // the YANG description explanation
	Required    bool
	// Anchor is the path keyword the value follows, read from the registered
	// command's ArgDef of the same name (anchoredParams). Empty means the
	// value follows the command.
	Anchor string
}

// commandUIResource is the neutral counterpart of zemcp.UIResourceInfo.
type commandUIResource struct {
	Path        string
	Permissions string
	CSP         string
}

// commandMetaSource returns a closure that builds the current command metadata
// from the plugin server's dispatcher. YANG-derived metadata (params,
// task-support, ui-resource) is loaded lazily once and cached together with
// the loader's error; the dispatcher command list is re-read on every call so
// the result always reflects current registrations. When yang.DefaultLoader
// refuses the schema, every call returns that error: metadata built from no
// schema would publish each command with no parameters and no task support.
func commandMetaSource(s *pluginserver.Server) func() ([]commandMeta, error) {
	loadMeta := sync.OnceValues(func() (*yangCommandMeta, error) {
		schema, err := yangloader.DefaultLoader()
		if err != nil {
			return nil, fmt.Errorf("command metadata: %w", err)
		}
		return &yangCommandMeta{
			paramsByPath:      buildParamMeta(schema),
			taskSupportByPath: buildTaskSupportMap(schema),
			uiResourceByPath:  yangloader.PathToUIResource(schema),
		}, nil
	})

	return func() ([]commandMeta, error) {
		d := s.Dispatcher()
		if d == nil {
			return nil, nil
		}

		meta, err := loadMeta()
		if err != nil {
			return nil, err
		}

		return buildCommandMeta(d.Commands(), d.Registry().All(),
			meta.paramsByPath, meta.taskSupportByPath, meta.uiResourceByPath), nil
	}
}

// yangCommandMeta is the YANG-derived half of the command metadata, built once
// per source from the default loader.
type yangCommandMeta struct {
	paramsByPath      map[string][]commandParam
	taskSupportByPath map[string]string
	uiResourceByPath  map[string]yangloader.UIResourceEntry
}

// buildCommandMeta merges the dispatcher's builtin commands with the plugin
// command registry into one deduplicated, name-ordered list.
//
// A plugin-proxied command is registered in BOTH sources on purpose:
// Dispatcher.RegisterWithOptions skips AddBuiltin when opts.PluginProxy is set,
// precisely so the plugin can register the same name in the CommandRegistry for
// ForwardToPlugin routing. The two entries describe one command, so a plain
// union shows it twice to every consumer.
//
// Pure so the merge can be tested without standing up a plugin server; the
// caller supplies the YANG-derived maps, any of which may be nil.
func buildCommandMeta(
	dispatcherCmds []*pluginserver.Command,
	pluginCmds []*pluginserver.RegisteredCommand,
	paramsByPath map[string][]commandParam,
	taskSupportByPath map[string]string,
	uiResourceByPath map[string]yangloader.UIResourceEntry,
) []commandMeta {
	// byName indexes into infos by the same lowercase key both sources store
	// their commands under (Dispatcher.commands and CommandRegistry.commands).
	infos := make([]commandMeta, 0, len(dispatcherCmds)+len(pluginCmds))
	byName := make(map[string]int, len(dispatcherCmds)+len(pluginCmds))

	for _, cmd := range dispatcherCmds {
		info := commandMeta{
			Name:          cmd.Name,
			ShortHelp:     cmd.ShortHelp,
			Description:   cmd.Description,
			ReadOnly:      cmd.ReadOnly,
			Params:        anchoredParams(paramsByPath[cmd.Name], cmd.ArgDefs),
			TaskSupport:   taskSupportByPath[cmd.Name],
			TakesSelector: cmd.TakesInlineSelector(),
		}
		if ui, ok := lookupUIResource(cmd.Name, uiResourceByPath); ok {
			info.UIResource = &commandUIResource{
				Path:        ui.Path,
				Permissions: ui.Permissions,
				CSP:         ui.CSP,
			}
		}
		byName[strings.ToLower(cmd.Name)] = len(infos)
		infos = append(infos, info)
	}

	// A plugin-registered command carries a name, a summary and an explanation.
	// The dispatcher entry wins on every field it has, because it is a strict
	// superset: YANG help, read-only, params, task-support, ui-resource and
	// selector handling. What the plugin supplies that the dispatcher can lack
	// is either half of the help, when the YANG node declares neither. The
	// dispatcher reads both halves from pathToDesc and pathToHelp in
	// LoadBuiltins. Each half is filled on its own, so a command with a YANG
	// summary and a plugin explanation keeps both.
	for _, cmd := range pluginCmds {
		// A plugin sets Hidden to remove a command from the operator-facing
		// surfaces. VisibleCommandEntries and Complete already exclude a hidden
		// command from the completion tree. This list is the only source for two
		// more surfaces: the MCP tools/list result and the API command list. A
		// hidden command must reach neither.
		//
		// The skip is at the top of the loop, because the duplicate branch below
		// fills help text on a dispatcher entry. That entry keeps the command
		// visible.
		//
		// The dispatcher loop above needs no equivalent skip.
		// pluginserver.Command has no Hidden field, and a builtin command has no
		// hidden state.
		if cmd.Hidden {
			continue
		}
		if i, dup := byName[strings.ToLower(cmd.Name)]; dup {
			if infos[i].ShortHelp == "" {
				infos[i].ShortHelp = cmd.ShortHelp
			}
			if infos[i].Description == "" {
				infos[i].Description = cmd.Description
			}
			continue
		}
		byName[strings.ToLower(cmd.Name)] = len(infos)
		infos = append(infos, commandMeta{
			Name:        cmd.Name,
			ShortHelp:   cmd.ShortHelp,
			Description: cmd.Description,
		})
	}

	// Both sources range over Go maps, so without this the order differs
	// between two calls describing identical state. Consumers cache and diff
	// this list (MCP tools/list is cacheable and wants a stable tool order),
	// and names are unique after the dedupe above, so name is a total order.
	slices.SortFunc(infos, func(a, b commandMeta) int {
		return strings.Compare(a.Name, b.Name)
	})

	return infos
}

// anchoredParams copies params with the Anchor each one carries in the
// dispatcher's own definitions, matched by name. The RPC input names the
// parameter and its type; where the dispatcher reads its value is a fact the
// command tree holds (command.ArgDef.Anchor), and a surface that BUILDS a
// command string, MCP, writes the value where the dispatcher binds it. A copy
// is made because paramsByPath is shared by every call.
func anchoredParams(params []commandParam, defs []command.ArgDef) []commandParam {
	if len(params) == 0 {
		return nil
	}
	anchored := make([]commandParam, len(params))
	copy(anchored, params)
	for i := range anchored {
		for j := range defs {
			if defs[j].Name() != anchored[i].Name {
				continue
			}
			anchored[i].Anchor = defs[j].Anchor()
			break
		}
	}
	return anchored
}

// buildParamMeta extracts all RPC metadata from the YANG loader and builds a
// map from CLI command path to neutral input parameters.
func buildParamMeta(schema *yangloader.Resolved) map[string][]commandParam {
	if schema == nil {
		return nil
	}

	// Build reverse map: CLI path -> wire method.
	wireToPath := yangloader.WireMethodToPath(schema)
	pathToWire := make(map[string]string, len(wireToPath))
	for wire, path := range wireToPath {
		pathToWire[path] = wire
	}

	// Each rpc a command node points at carries that node's wire method
	// (yangloader.PublishedRPCs), so the join is by method, and no module name
	// is rebuilt from it.
	pub, err := yangloader.PublishedRPCs(schema)
	if err != nil {
		return nil
	}
	inputs := make(map[string][]yangloader.LeafMeta, len(pub.Commands))
	for _, rpc := range pub.Commands {
		inputs[rpc.WireMethod] = rpc.Input
	}

	result := make(map[string][]commandParam)
	for path, wire := range pathToWire {
		input := inputs[wire]
		if len(input) == 0 {
			continue
		}
		params := make([]commandParam, len(input))
		for i, leaf := range input {
			params[i] = commandParam{
				Name:        leaf.Name,
				Type:        leaf.Type,
				ShortHelp:   leaf.ShortHelp,
				Description: leaf.Description,
				Required:    leaf.Mandatory,
			}
		}
		result[path] = params
	}

	return result
}

// buildTaskSupportMap extracts ze:task-support values from the YANG loader.
func buildTaskSupportMap(schema *yangloader.Resolved) map[string]string {
	if schema == nil {
		return nil
	}
	return yangloader.PathToTaskSupport(schema)
}

// lookupUIResource checks if a command path or any of its parent paths has a
// ze:ui-resource annotation. Commands like "show bgp peer list" inherit the UI
// resource from the "peer" grouping container.
func lookupUIResource(cmdPath string, m map[string]yangloader.UIResourceEntry) (yangloader.UIResourceEntry, bool) {
	if m == nil {
		return yangloader.UIResourceEntry{}, false
	}
	if info, ok := m[cmdPath]; ok {
		return info, true
	}
	for {
		idx := strings.LastIndex(cmdPath, " ")
		if idx < 0 {
			break
		}
		cmdPath = cmdPath[:idx]
		if info, ok := m[cmdPath]; ok {
			return info, true
		}
	}
	return yangloader.UIResourceEntry{}, false
}
