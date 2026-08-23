// Design: docs/architecture/api/commands.md — where a command is served
// Related: plan/spec-cli-pipe-operator-coverage.md — AC-10
//
// schema_data.go answers the five `show schema *` commands with structured
// data, so they reach the pipe layer.
//
// They printed a table and returned an exit code. YANG declares a wire method
// for each and no daemon handler implements one, so `ze cli -c "show schema
// list | json"` answered `unknown command` while `ze help command --json`
// published `global-pipes: true` for it.
//
// The payloads are the ones the `--json` flag already produced, lifted out of
// the printers rather than invented, so the two spellings of each answer cannot
// disagree. The root `ze schema` command keeps its own printing and its flag.

package cli

import (
	"fmt"
	"os"
	"sort"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// writeSchemaError reports why the schema registry could not be built. The
// printers report it the same way, so both spellings of the failure agree.
func writeSchemaError(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
}

// dataList answers `show schema list`: every registered module.
func dataList(_ []string) (any, int) {
	registry, err := buildSchemaRegistry(nil)
	if err != nil {
		writeSchemaError(err)
		return nil, 1
	}
	modules := registry.ListModules()
	sort.Strings(modules)

	rows := make([]map[string]any, 0, len(modules))
	for _, name := range modules {
		s, _ := registry.GetByModule(name)
		if s == nil {
			continue
		}
		row := map[string]any{"module": name, "namespace": s.Namespace}
		if len(s.WantsConfig) > 0 {
			row["wants-config"] = s.WantsConfig
		}
		if len(s.Imports) > 0 {
			row["imports"] = s.Imports
		}
		rows = append(rows, row)
	}
	return map[string]any{"schemas": rows}, 0
}

// dataHandlers answers `show schema handlers` as ROWS rather than as the
// path-to-module map the --json flag emits. A map keyed by path carries the
// same facts, and rows are what a row operator can act on.
func dataHandlers(_ []string) (any, int) {
	registry, err := buildSchemaRegistry(nil)
	if err != nil {
		writeSchemaError(err)
		return nil, 1
	}
	handlers := registry.ListHandlers()
	paths := make([]string, 0, len(handlers))
	for path := range handlers {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	rows := make([]map[string]any, 0, len(paths))
	for _, path := range paths {
		rows = append(rows, map[string]any{"handler": path, "module": handlers[path]})
	}
	return map[string]any{"handlers": rows}, 0
}

// dataMethods answers `show schema methods [module]`.
func dataMethods(args []string) (any, int) {
	return schemaEntryRows(args, "methods", func(reg *pluginserver.SchemaRegistry, module string) []schemaEntry {
		rpcs := reg.ListRPCs(module)
		entries := make([]schemaEntry, len(rpcs))
		for i, rpc := range rpcs {
			entries[i] = schemaEntry{wire: rpc.WireMethod, module: rpc.Module, desc: rpc.Description}
		}
		return entries
	})
}

// dataEvents answers `show schema events [module]`.
func dataEvents(args []string) (any, int) {
	return schemaEntryRows(args, "events", func(reg *pluginserver.SchemaRegistry, module string) []schemaEntry {
		notifs := reg.ListNotifications(module)
		entries := make([]schemaEntry, len(notifs))
		for i, notif := range notifs {
			entries[i] = schemaEntry{wire: notif.WireMethod, module: notif.Module, desc: notif.Description}
		}
		return entries
	})
}

// schemaEntryRows is the shared half of methods and events: both list wire
// methods, optionally narrowed to one module.
func schemaEntryRows(args []string, key string,
	listFn func(*pluginserver.SchemaRegistry, string) []schemaEntry,
) (any, int) {
	var module string
	if len(args) > 0 {
		module = args[0]
	}
	registry, err := buildSchemaRegistry(nil)
	if err != nil {
		writeSchemaError(err)
		return nil, 1
	}

	entries := listFn(registry, module)
	sort.Slice(entries, func(i, j int) bool { return entries[i].wire < entries[j].wire })

	rows := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, map[string]any{
			"method": e.wire, "module": e.module, "description": e.desc,
		})
	}
	// An empty answer is not an error here: `| match` over it answers nothing
	// and exits 0, which is what lets a caller tell "no methods" from "the
	// command did not run". The printer exits 1 for a named module with none,
	// and that stays its own behavior.
	return map[string]any{key: rows}, 0
}

// dataProtocol answers `show schema protocol`. It is ONE document rather than
// rows, and it declares that shape, so the row operators are refused by name
// over it instead of answering something plausible.
func dataProtocol(_ []string) (any, int) {
	return map[string]any{
		"protocol": "Hub Architecture",
		"version":  "1.0",
	}, 0
}
