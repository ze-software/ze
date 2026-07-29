// Design: ai/rules/feature-gate-registration.md -- always-on command metadata
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
	"slices"
	"strings"
	"sync"

	yangloader "github.com/ze-software/ze/internal/component/config/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// commandMeta is the neutral, always-on description of one registered command.
// It carries every field BOTH the API and MCP command listers need so neither
// surface has to traverse the dispatcher or load YANG itself. Surface-specific
// adapters convert it to api.CommandMeta / zemcp.CommandInfo.
type commandMeta struct {
	Name        string             // dispatch path, e.g. "show bgp rib status"
	Help        string             // description from YANG
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
	Description string
	Required    bool
}

// commandUIResource is the neutral counterpart of zemcp.UIResourceInfo.
type commandUIResource struct {
	Path        string
	Permissions string
	CSP         string
}

// commandMetaSource returns a closure that builds the current command metadata
// from the plugin server's dispatcher. YANG-derived metadata (params,
// task-support, ui-resource) is loaded lazily once and cached; the dispatcher
// command list is re-read on every call so the result always reflects current
// registrations.
func commandMetaSource(s *pluginserver.Server) func() []commandMeta {
	var (
		metaOnce          sync.Once
		paramsByPath      map[string][]commandParam
		taskSupportByPath map[string]string
		uiResourceByPath  map[string]yangloader.UIResourceEntry
	)

	initMeta := func() {
		metaOnce.Do(func() {
			loader, err := yangloader.DefaultLoader()
			if err != nil {
				return
			}
			paramsByPath = buildParamMeta(loader)
			taskSupportByPath = buildTaskSupportMap(loader)
			uiResourceByPath = yangloader.PathToUIResource(loader)
		})
	}

	return func() []commandMeta {
		d := s.Dispatcher()
		if d == nil {
			return nil
		}

		initMeta()

		return buildCommandMeta(d.Commands(), d.Registry().All(),
			paramsByPath, taskSupportByPath, uiResourceByPath)
	}
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
			Help:          cmd.Help,
			ReadOnly:      cmd.ReadOnly,
			Params:        paramsByPath[cmd.Name],
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

	// Plugin-registered commands carry only name + description. The dispatcher
	// entry wins on every field it has, because it is a strict superset: YANG
	// help, read-only, params, task-support, ui-resource and selector handling.
	// The one thing the plugin can supply that the dispatcher may lack is help
	// text, when the YANG node carries no description (dispatcher Help comes
	// from pathToDesc in LoadBuiltins), so fill that gap rather than drop it.
	for _, cmd := range pluginCmds {
		if i, dup := byName[strings.ToLower(cmd.Name)]; dup {
			if infos[i].Help == "" {
				infos[i].Help = cmd.Description
			}
			continue
		}
		byName[strings.ToLower(cmd.Name)] = len(infos)
		infos = append(infos, commandMeta{
			Name: cmd.Name,
			Help: cmd.Description,
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

// buildParamMeta extracts all RPC metadata from the YANG loader and builds a
// map from CLI command path to neutral input parameters.
func buildParamMeta(loader *yangloader.Loader) map[string][]commandParam {
	if loader == nil {
		return nil
	}

	// Build reverse map: CLI path -> wire method.
	wireToPath := yangloader.WireMethodToPath(loader)
	pathToWire := make(map[string]string, len(wireToPath))
	for wire, path := range wireToPath {
		pathToWire[path] = wire
	}

	// Extract RPC input params for each command path.
	result := make(map[string][]commandParam)
	var tb textbuf.Buffer
	for path, wire := range pathToWire {
		// Wire method format: "module:rpc-name". Extract module, add "-api" suffix.
		module := wireModule(wire)
		rpcName := wireRPC(wire)
		if module == "" || rpcName == "" {
			continue
		}

		tb.Reset()
		rpcs := yangloader.ExtractRPCs(loader, tb.Str(module).Str("-api").String())
		if rpcs == nil {
			// Try without -api suffix (some modules use -cmd).
			tb.Reset()
			rpcs = yangloader.ExtractRPCs(loader, tb.Str(module).Str("-cmd").String())
		}
		for _, rpc := range rpcs {
			if rpc.Name != rpcName {
				continue
			}
			if len(rpc.Input) == 0 {
				break
			}
			params := make([]commandParam, len(rpc.Input))
			for i, leaf := range rpc.Input {
				params[i] = commandParam{
					Name:        leaf.Name,
					Type:        leaf.Type,
					Description: leaf.Description,
					Required:    leaf.Mandatory,
				}
			}
			result[path] = params
			break
		}
	}

	return result
}

// buildTaskSupportMap extracts ze:task-support values from the YANG loader.
func buildTaskSupportMap(loader *yangloader.Loader) map[string]string {
	if loader == nil {
		return nil
	}
	return yangloader.PathToTaskSupport(loader)
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

// wireModule extracts the module prefix from a wire method (e.g. "ze-bgp:peer-list" -> "ze-bgp").
func wireModule(wire string) string {
	mod, _, ok := strings.Cut(wire, ":")
	if !ok {
		return ""
	}
	return mod
}

// wireRPC extracts the RPC name from a wire method (e.g. "ze-bgp:peer-list" -> "peer-list").
func wireRPC(wire string) string {
	_, rpc, ok := strings.Cut(wire, ":")
	if !ok {
		return ""
	}
	return rpc
}
