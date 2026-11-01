// Design: docs/architecture/api/commands.md — where a command is served
// Related: docs/architecture/api/commands.md — the operator model this serves
//
// schema_data.go answers the five `show schema *` commands with structured
// data, so they reach the pipe layer.
//
// They printed a table and returned an exit code. YANG declares a wire method
// for each and no daemon handler implements one, so `ze cli -c "show schema
// list | json"` answered `unknown command` while `ze help command --json`
// published `global-pipes: true` for it.
//
// The payloads are the ones the deleted rendering option produced, lifted out
// of the printers rather than invented, so the answer has one definition. The
// root `ze schema` command keeps its own printing, and prints only.

package cli

import (
	"slices"
	"sort"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	"github.com/ze-software/ze/internal/component/command"
)

// Row and field keys of the answers below. register.go declares the column
// order with the same names, so a rename that misses one drops a column.
const (
	keyModule      = "module"
	keyNamespace   = "namespace"
	keyMethod      = "method"
	keyDescription = "description"
)

// schemaRefusal is the answer when the schema registry could not be built. It
// carries the line the printers write for the same failure, so both spellings
// of the failure agree. The handler writes nothing itself
// (command.LocalDataHandler).
func schemaRefusal(err error) command.LocalAnswer {
	return command.LocalRefusal(1, command.LocalErrorLine(err))
}

// dataList answers `show schema list`: every registered module.
func dataList(command.ValidatedArgs) command.LocalAnswer {
	registry, err := buildSchemaRegistry(nil)
	if err != nil {
		return schemaRefusal(err)
	}
	modules := registry.ListModules()
	slices.Sort(modules)

	rows := make([]map[string]any, 0, len(modules))
	for _, name := range modules {
		s, _ := registry.GetByModule(name)
		if s == nil {
			continue
		}
		row := map[string]any{keyModule: name, keyNamespace: s.Namespace}
		if len(s.WantsConfig) > 0 {
			row["wants-config"] = s.WantsConfig
		}
		if len(s.Imports) > 0 {
			row["imports"] = s.Imports
		}
		rows = append(rows, row)
	}
	return command.LocalAnswer{Payload: map[string]any{"schemas": rows}}
}

// dataHandlers answers `show schema handlers` as ROWS rather than as the
// path-to-module map the deleted rendering option emitted. A map keyed by path
// carries the same facts, and rows are what a row operator can act on.
func dataHandlers(command.ValidatedArgs) command.LocalAnswer {
	registry, err := buildSchemaRegistry(nil)
	if err != nil {
		return schemaRefusal(err)
	}
	handlers := registry.ListHandlers()
	paths := make([]string, 0, len(handlers))
	for path := range handlers {
		paths = append(paths, path)
	}
	slices.Sort(paths)

	rows := make([]map[string]any, 0, len(paths))
	for _, path := range paths {
		rows = append(rows, map[string]any{"handler": path, keyModule: handlers[path]})
	}
	return command.LocalAnswer{Payload: map[string]any{subHandlers: rows}}
}

// dataMethods answers `show schema methods [module]`.
func dataMethods(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	return schemaEntryRows(args, "methods", func(reg *pluginserver.SchemaRegistry, module string) []schemaEntry {
		rpcs := reg.ListRPCs(module)
		entries := make([]schemaEntry, len(rpcs))
		for i, rpc := range rpcs {
			entries[i] = schemaEntry{wire: rpc.WireMethod, module: rpc.Module, desc: rpc.ShortHelp}
		}
		return entries
	})
}

// dataEvents answers `show schema events [module]`.
func dataEvents(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	return schemaEntryRows(args, "events", func(reg *pluginserver.SchemaRegistry, module string) []schemaEntry {
		notifs := reg.ListNotifications(module)
		entries := make([]schemaEntry, len(notifs))
		for i, notif := range notifs {
			entries[i] = schemaEntry{wire: notif.WireMethod, module: notif.Module, desc: notif.ShortHelp}
		}
		return entries
	})
}

// schemaEntryRows is the shared half of methods and events: both list wire
// methods, optionally narrowed to one module.
func schemaEntryRows(args []string, key string,
	listFn func(*pluginserver.SchemaRegistry, string) []schemaEntry,
) command.LocalAnswer {
	var module string
	if len(args) > 0 {
		module = args[0]
	}
	registry, err := buildSchemaRegistry(nil)
	if err != nil {
		return schemaRefusal(err)
	}

	entries := listFn(registry, module)
	sort.Slice(entries, func(i, j int) bool { return entries[i].wire < entries[j].wire })

	rows := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, map[string]any{
			keyMethod: e.wire, keyModule: e.module, keyDescription: e.desc,
		})
	}
	// An empty answer is not an error here: `| match` over it answers nothing
	// and exits 0, which is what lets a caller tell "no methods" from "the
	// command did not run". The printer exits 1 for a named module with none,
	// and that stays its own behavior.
	return command.LocalAnswer{Payload: map[string]any{key: rows}}
}

// dataProtocol answers `show schema protocol`. It is ONE document rather than
// rows, and it declares that shape, so the row operators are refused by name
// over it instead of answering something plausible.
func dataProtocol(command.ValidatedArgs) command.LocalAnswer {
	return command.LocalAnswer{Payload: map[string]any{
		subProtocol: "Hub Architecture",
		"version":   "1.0",
	}}
}
