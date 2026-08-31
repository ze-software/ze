// Design: docs/architecture/config/yang-config-design.md — schema CLI
//
// Package schema provides the ze schema subcommand.
package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/suggest"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var errNoPluginCommandSpecified = errors.New("no plugin command specified")

// Plugin ID prefix for internal plugins (e.g., "ze.bgp", "ze.gr").
const internalPluginPrefix = "ze."

// Module name prefix for Ze modules (e.g., "ze-bgp", "ze-types").
const moduleNamePrefix = "ze-"

// YANG module name for the core BGP configuration schema.
const bgpConfModule = "ze-bgp-conf"

// Timeout for external plugin YANG queries.
const externalPluginTimeout = 10 * time.Second

// Core YANG modules that are always available (not shown as imports).
var coreModules = map[string]bool{
	"ze-types":       true,
	"ze-extensions":  true,
	"ze-hub-conf":    true,
	"ze-plugin-conf": true,
}

// Run executes the schema subcommand with the given arguments.
// plugins is an optional list of external plugin commands to query for YANG.
// Returns exit code.
func Run(args, plugins []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}

	// `-h` and `--help` are the one flag spelling every command keeps
	// (ai/rules/cli.md). They are answered wherever they appear: no subcommand
	// here parses a flag of its own.
	if slices.Contains(args, "-h") || slices.Contains(args, "--help") {
		usage()
		return 0
	}

	// The cases stay written out. TestDispatchParity reads them from this
	// file's AST and compares them with schemaCommands, and it can only see a
	// string literal.
	switch args[0] {
	case "list":
		return cmdList(args[1:], plugins)
	case "show":
		return cmdShow(args[1:], plugins)
	case "handlers":
		return cmdHandlers(args[1:], plugins)
	case "methods":
		return cmdMethods(args[1:], plugins)
	case "events":
		return cmdEvents(args[1:], plugins)
	case "protocol":
		return cmdProtocol()
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown schema command: %s\n", args[0])
		if suggestion := suggest.Command(args[0], append(schemaCommands, "show", "help")); suggestion != "" {
			fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", suggestion)
		}
		usage()
		return 1
	}
}

func usage() {
	p := helpfmt.Page{
		Command: "ze schema",
		Summary: "Schema discovery commands",
		Usage:   []string{"ze schema <command> [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Commands", Entries: []helpfmt.HelpEntry{
				{Name: subList, Desc: "List all registered schemas"},
				{Name: "show <module>", Desc: "Show YANG content for a module"},
				{Name: subHandlers, Desc: "List handler -> module mapping"},
				{Name: "methods [module]", Desc: "List RPCs from YANG (all or specific module)"},
				{Name: "events [module]", Desc: "List notifications from YANG"},
				{Name: subProtocol, Desc: "Show protocol version and format info"},
				{Name: "help", Desc: "Show this help"},
			}},
		},
		Examples: []string{
			"ze schema list",
			`ze cli -c "show schema list | json"`,
			"ze schema show ze-bgp",
			"ze schema handlers",
			"ze schema methods",
			"ze schema methods ze-bgp-api",
			"ze schema events",
		},
	}
	p.WriteErr()
}

// commandArguments answers the positional arguments a schema subcommand takes,
// and refuses every other token by name.
//
// No subcommand here parses a flag. Rendering belongs to the pipe layer, so
// `show schema list | json` answers what an option once answered. A token this
// dispatch dropped in silence would report success for a question nobody
// answered.
func commandArguments(subcommand, takes string, args []string, most int) ([]string, error) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return nil, fmt.Errorf(
				"ze schema %s: unknown option %q; render the answer with a pipe: ze cli -c %q",
				subcommand, arg, "show schema "+subcommand+" | json")
		}
	}
	if len(args) > most {
		return nil, fmt.Errorf("ze schema %s: unexpected argument %q; it takes %s",
			subcommand, args[most], takes)
	}
	return args, nil
}

// writeArgumentError reports a refused argument and answers the exit code.
func writeArgumentError(err error) int {
	fmt.Fprintln(os.Stderr, "error:", err)
	return 1
}

// cmdList lists all registered schemas.
func cmdList(args, plugins []string) int {
	if _, err := commandArguments(subList, "no argument", args, 0); err != nil {
		return writeArgumentError(err)
	}

	registry, err := buildSchemaRegistry(plugins)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	modules := registry.ListModules()
	if len(modules) == 0 {
		fmt.Println("No schemas registered")
		return 0
	}

	sort.Strings(modules)

	// Print header
	fmt.Printf("%-24s %-20s %-16s %s\n", "Module", "Namespace", "Wants Config", "Imports")

	for _, name := range modules {
		s, _ := registry.GetByModule(name)
		if s != nil {
			imports := "-"
			if len(s.Imports) > 0 {
				formattedImports := make([]string, len(s.Imports))
				for i, imp := range s.Imports {
					formattedImports[i] = formatModuleAsNamespace(imp)
				}
				imports = textbuf.Join(formattedImports, ", ")
			}
			wantsConfig := "-"
			if len(s.WantsConfig) > 0 {
				wantsConfig = textbuf.Join(s.WantsConfig, ", ")
			}
			fmt.Printf("%-24s %-20s %-16s %s\n", name, s.Namespace, wantsConfig, imports)
		}
	}

	return 0
}

// cmdShow shows YANG content for a specific module.
func cmdShow(args, plugins []string) int {
	positional, err := commandArguments("show", "one module name", args, 1)
	if err != nil {
		return writeArgumentError(err)
	}
	if len(positional) < 1 {
		fmt.Fprintln(os.Stderr,
			"error: ze schema show: no module named; ze schema list names every module")
		return 1
	}

	moduleName := positional[0]
	registry, err := buildSchemaRegistry(plugins)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	s, err := registry.GetByModule(moduleName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if s.Yang == "" {
		fmt.Printf("# Module: %s (no YANG content available)\n", moduleName)
		fmt.Printf("# Namespace: %s\n", s.Namespace)
		fmt.Printf("# Plugin: %s\n", s.Plugin)
		fmt.Fprintf(os.Stdout, "# Handlers: %s\n", textbuf.Join(s.Handlers, ", ")) //nolint:errcheck // terminal output
		return 0
	}

	fmt.Println(s.Yang)
	return 0
}

// cmdHandlers lists handler → module mapping.
func cmdHandlers(args, plugins []string) int {
	if _, err := commandArguments(subHandlers, "no argument", args, 0); err != nil {
		return writeArgumentError(err)
	}

	registry, err := buildSchemaRegistry(plugins)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	handlers := registry.ListHandlers()
	if len(handlers) == 0 {
		fmt.Println("No handlers registered")
		return 0
	}

	paths := make([]string, 0, len(handlers))
	for path := range handlers {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	fmt.Println("Handler → Module mapping:")
	for _, path := range paths {
		fmt.Printf("  %-30s → %s\n", path, handlers[path])
	}

	return 0
}

// cmdProtocol shows protocol version and format info.
func cmdProtocol() int {
	fmt.Println("Hub Architecture Protocol")
	fmt.Println("=========================")
	fmt.Println()
	fmt.Println("Version: 1.0")
	fmt.Println()
	fmt.Println("Schema Declaration Format:")
	fmt.Println("  declare schema module <name>")
	fmt.Println("  declare schema namespace <uri>")
	fmt.Println("  declare schema handler <path>")
	fmt.Println("  declare schema yang <<EOF")
	fmt.Println("  ... YANG content ...")
	fmt.Println("  EOF")
	fmt.Println()
	fmt.Println("Handler Routing:")
	fmt.Println("  Longest prefix match on handler path")
	fmt.Println("  Example: bgp/peer/timers → matches bgp/peer handler")
	fmt.Println()
	fmt.Println("Config Reader Protocol:")
	fmt.Println("  config schema <module> <yang-content>")
	fmt.Println("  config verify <handler-path> <data>")
	fmt.Println("  config apply <handler-path> <data>")
	fmt.Println("  config complete")
	return 0
}

// schemaEntry holds display data for an RPC or notification row.
type schemaEntry struct {
	wire   string
	module string
	desc   string
}

// printSchemaTable prints a sorted table of schema entries with the given header.
func printSchemaTable(header, kind string, entries []schemaEntry) {
	if len(entries) == 0 {
		fmt.Printf("No %ss registered\n", kind)
		return
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].wire < entries[j].wire
	})

	fmt.Printf("%-36s %-16s %s\n", header, "Module", "Description")
	for _, e := range entries {
		// The description is written whole. It is the declared one-line
		// summary, which the help-shape gate holds to one sentence of at
		// most 25 words, so this surface has nothing left to shorten. The
		// fixed 50-character cut that stood here sliced bytes rather than
		// runes, so a multi-byte character on the boundary left invalid
		// UTF-8 behind.
		desc := e.desc
		fmt.Printf("%-36s %-16s %s\n", e.wire, e.module, desc)
	}
}

// cmdMethods lists RPCs from YANG API modules.
func cmdMethods(args, plugins []string) int {
	return cmdListSchema(args, plugins, "methods", "RPC", "Method", func(reg *pluginserver.SchemaRegistry, module string) []schemaEntry {
		rpcs := reg.ListRPCs(module)
		entries := make([]schemaEntry, len(rpcs))
		for i, rpc := range rpcs {
			entries[i] = schemaEntry{wire: rpc.WireMethod, module: rpc.Module, desc: rpc.Description}
		}
		return entries
	})
}

// cmdEvents lists notifications from YANG API modules.
func cmdEvents(args, plugins []string) int {
	return cmdListSchema(args, plugins, "events", "notification", "Event", func(reg *pluginserver.SchemaRegistry, module string) []schemaEntry {
		notifs := reg.ListNotifications(module)
		entries := make([]schemaEntry, len(notifs))
		for i, notif := range notifs {
			entries[i] = schemaEntry{wire: notif.WireMethod, module: notif.Module, desc: notif.Description}
		}
		return entries
	})
}

// cmdListSchema is the shared logic for listing RPCs or notifications.
func cmdListSchema(
	args []string,
	plugins []string,
	subcommand string,
	kind string,
	header string,
	listFn func(*pluginserver.SchemaRegistry, string) []schemaEntry,
) int {
	positional, err := commandArguments(subcommand, "one module name", args, 1)
	if err != nil {
		return writeArgumentError(err)
	}

	var module string
	if len(positional) > 0 {
		module = positional[0]
	}

	registry, err := buildSchemaRegistry(plugins)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	entries := listFn(registry, module)
	if len(entries) == 0 && module != "" {
		fmt.Fprintf(os.Stderr, "no %ss found for module %s\n", kind, module)
		return 1
	}

	printSchemaTable(header, kind, entries)
	return 0
}

// loadAPIRPCs loads API YANG modules and registers their RPCs and notifications.
func loadAPIRPCs(registry *pluginserver.SchemaRegistry) error {
	loader := yang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		return fmt.Errorf("load core modules: %w", err)
	}
	// LoadRegistered loads all conf + API modules via init() registration.
	if err := loader.LoadRegistered(); err != nil {
		return fmt.Errorf("load registered modules: %w", err)
	}
	if err := loader.Resolve(); err != nil {
		return fmt.Errorf("resolve YANG: %w", err)
	}

	// Extract and register RPCs and notifications from each API module
	for _, moduleName := range loader.APIModuleNames() {
		rpcs := yang.ExtractRPCs(loader, moduleName)
		if len(rpcs) > 0 {
			if err := registry.RegisterRPCs(moduleName, rpcs); err != nil {
				return fmt.Errorf("register RPCs from %s: %w", moduleName, err)
			}
		}

		notifs := yang.ExtractNotifications(loader, moduleName)
		if len(notifs) > 0 {
			if err := registry.RegisterNotifications(moduleName, notifs); err != nil {
				return fmt.Errorf("register notifications from %s: %w", moduleName, err)
			}
		}
	}

	return nil
}

// buildSchemaRegistry builds a registry with all available YANG schemas.
// Loads ze-bgp, internal plugins, and optionally external plugins.
// Also loads API YANG modules for RPC/notification indexing.
func buildSchemaRegistry(extPlugins []string) (*pluginserver.SchemaRegistry, error) {
	registry := pluginserver.NewSchemaRegistry()
	loaded := make(map[string]bool)

	// Register ze-bgp schema first (base module) - it provides config, doesn't want it.
	// The module is read from the plugin registry, where the BGP plugin publishes
	// it, rather than imported from internal/component/bgp/yang: this file is
	// always-on and that import would pin the BGP subtree into every binary
	// (//go:build ze_bgp). An empty result means BGP is compiled out, and the
	// loop below registers whatever plugins this build does have.
	if bgpYANG := plugin.GetInternalPluginYANG("bgp"); bgpYANG != "" {
		if err := registerYANG(registry, bgpYANG, internalPluginPrefix+"bgp", []string{"bgp", "bgp/peer"}, nil, loaded); err != nil {
			return nil, fmt.Errorf("register ze-bgp: %w", err)
		}
	}

	// Register internal plugin schemas
	for name, yangContent := range plugin.GetAllInternalPluginYANG() {
		pluginName := strings.TrimSuffix(strings.TrimPrefix(name, moduleNamePrefix), ".yang")
		wantsConfig := plugin.GetInternalPluginWantsConfig(pluginName)
		if err := registerYANG(registry, yangContent, internalPluginPrefix+pluginName, nil, wantsConfig, loaded); err != nil {
			return nil, fmt.Errorf("register %s: %w", name, err)
		}
	}

	// Register external plugin schemas
	// Note: External plugins don't have static WantsConfig metadata - they declare it at runtime
	for _, pluginSpec := range extPlugins {
		yangContent, pluginName, err := getPluginYANG(pluginSpec)
		if err != nil {
			return nil, fmt.Errorf("get YANG from %s: %w", pluginSpec, err)
		}
		if yangContent == "" {
			continue // Plugin doesn't provide YANG
		}

		if err := registerYANG(registry, yangContent, pluginName, nil, nil, loaded); err != nil {
			return nil, fmt.Errorf("register %s: %w", pluginSpec, err)
		}
	}

	// Load API YANG modules and extract RPCs/notifications
	if err := loadAPIRPCs(registry); err != nil {
		return nil, fmt.Errorf("load API RPCs: %w", err)
	}

	return registry, nil
}

// registerYANG registers YANG content and verifies dependencies are available.
func registerYANG(registry *pluginserver.SchemaRegistry, yangContent, pluginName string, handlers, wantsConfig []string, loaded map[string]bool) error {
	meta, err := yang.ParseYANGMetadata(yangContent)
	if err != nil {
		return fmt.Errorf("parse YANG: %w", err)
	}

	// Skip if already loaded (deduplication by module name)
	if loaded[meta.Module] {
		return nil
	}

	// Check dependencies
	for _, imp := range meta.Imports {
		if loaded[imp] || coreModules[imp] {
			continue
		}

		// Try to auto-load internal dependency
		if autoLoaded := tryAutoLoadInternal(registry, imp, loaded); autoLoaded {
			continue
		}

		return fmt.Errorf("module %s imports %s but it is not available", meta.Module, imp)
	}

	if handlers == nil {
		handlers = []string{}
	}

	loaded[meta.Module] = true

	// Filter core modules from displayed imports
	var displayImports []string
	for _, imp := range meta.Imports {
		if !coreModules[imp] {
			displayImports = append(displayImports, imp)
		}
	}

	return registry.Register(&pluginserver.Schema{
		Module:      meta.Module,
		Namespace:   yang.FormatNamespace(meta.Namespace),
		Yang:        yangContent,
		Imports:     displayImports,
		Handlers:    handlers,
		Plugin:      pluginName,
		WantsConfig: wantsConfig,
	})
}

// tryAutoLoadInternal attempts to auto-load an internal plugin by module name.
func tryAutoLoadInternal(registry *pluginserver.SchemaRegistry, moduleName string, loaded map[string]bool) bool {
	if _, err := registry.GetByModule(moduleName); err == nil {
		loaded[moduleName] = true
		return true
	}

	// Module names are "ze-bgp", plugin names are "bgp"
	pluginName := strings.TrimPrefix(moduleName, moduleNamePrefix)

	// Try to get YANG content for this module
	yangContent, handlers, pluginID := getInternalYANG(moduleName, pluginName)

	// If plugin name lookup fails, search the YANG module registry.
	// Module names don't always match plugin names (e.g., ze-fib-conf is
	// registered by the fibkernel plugin, not a plugin named "fib-conf").
	if yangContent == "" {
		var tb textbuf.Buffer
		yangFileName := tb.Str(moduleName).Str(".yang").String()
		for _, m := range yang.Modules() {
			if m.Name == yangFileName {
				yangContent = m.Content
				pluginID = internalPluginPrefix + pluginName
				break
			}
		}
	}

	if yangContent == "" {
		return false
	}

	// Register using common helper
	if err := registerInternalYANG(registry, yangContent, handlers, pluginID, loaded); err != nil {
		return false
	}
	return true
}

// getInternalYANG returns YANG content, handlers, and plugin ID for an internal module.
// Returns empty yangContent if module is not found.
func getInternalYANG(moduleName, pluginName string) (yangContent string, handlers []string, pluginID string) {
	// Core BGP module, read from the plugin registry (see buildSchemaRegistry).
	// Falls through to the generic lookup below when BGP is compiled out.
	if moduleName == bgpConfModule {
		if bgpYANG := plugin.GetInternalPluginYANG("bgp"); bgpYANG != "" {
			var tb textbuf.Buffer
			return bgpYANG, []string{"bgp", "bgp/peer"}, tb.Str(internalPluginPrefix).Str("bgp").String()
		}
	}

	// Internal plugins
	yangContent = plugin.GetInternalPluginYANG(pluginName)
	if yangContent != "" {
		var tb2 textbuf.Buffer
		return yangContent, nil, tb2.Str("ze.").Str(pluginName).String()
	}

	return "", nil, ""
}

// registerInternalYANG registers YANG content from an internal source.
// Extracts metadata from YANG and registers with the given handlers and plugin ID.
// Automatically populates WantsConfig from static metadata.
func registerInternalYANG(registry *pluginserver.SchemaRegistry, yangContent string, handlers []string, pluginID string, loaded map[string]bool) error {
	meta, err := yang.ParseYANGMetadata(yangContent)
	if err != nil {
		return err
	}

	if handlers == nil {
		handlers = []string{}
	}

	// Filter core modules from displayed imports (same as registerYANG)
	var displayImports []string
	for _, imp := range meta.Imports {
		if !coreModules[imp] {
			displayImports = append(displayImports, imp)
		}
	}

	// Get WantsConfig from static metadata (pluginID is "ze.name", extract "name")
	var wantsConfig []string
	if strings.HasPrefix(pluginID, internalPluginPrefix) {
		pluginName := pluginID[3:]
		wantsConfig = plugin.GetInternalPluginWantsConfig(pluginName)
	}

	if err := registry.Register(&pluginserver.Schema{
		Module:      meta.Module,
		Namespace:   yang.FormatNamespace(meta.Namespace),
		Yang:        yangContent,
		Imports:     displayImports,
		Handlers:    handlers,
		Plugin:      pluginID,
		WantsConfig: wantsConfig,
	}); err != nil {
		return err
	}
	loaded[meta.Module] = true
	return nil
}

// formatModuleAsNamespace converts a module name to namespace display format.
func formatModuleAsNamespace(module string) string {
	if strings.HasPrefix(module, moduleNamePrefix) {
		return internalPluginPrefix + module[len(moduleNamePrefix):]
	}
	return module
}

// getPluginYANG gets YANG from a plugin based on its specification.
// Handles: "ze.name" (internal), "ze-name" (internal), or path (external).
//
// Note: ze.X (goroutine mode) and ze-X (direct call mode) are treated identically
// for schema discovery because both just need the YANG content. The execution mode
// distinction only matters at runtime when actually running plugin commands.
func getPluginYANG(pluginSpec string) (string, string, error) {
	// Internal plugin "ze.name" or "ze-name" - both have 3-char prefix
	// Both modes use the same YANG content; mode only affects runtime execution.
	if strings.HasPrefix(pluginSpec, internalPluginPrefix) || strings.HasPrefix(pluginSpec, moduleNamePrefix) {
		name := pluginSpec[len(internalPluginPrefix):] // Strip prefix (both are same length)

		yangContent := plugin.GetInternalPluginYANG(name)
		if yangContent == "" {
			return "", "", fmt.Errorf("internal plugin %q has no YANG", name)
		}

		var tb3 textbuf.Buffer
		return yangContent, tb3.Str("ze.").Str(name).String(), nil
	}

	// External plugin - execute with --yang flag
	yangContent, err := getExternalPluginYANG(pluginSpec)
	if err != nil {
		return "", "", err
	}
	if yangContent == "" {
		return "", "", nil
	}

	return yangContent, pluginSpec, nil
}

// getExternalPluginYANG executes an external plugin with --yang and returns its YANG.
func getExternalPluginYANG(pluginSpec string) (string, error) {
	args := strings.Fields(pluginSpec)
	if len(args) == 0 {
		return "", errNoPluginCommandSpecified
	}

	// Append --yang to get YANG output
	args = append(args, "--yang")

	ctx, cancel := context.WithTimeout(context.Background(), externalPluginTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, args[0], args[1:]...) //nolint:gosec // Plugin command from user
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("execute %v: %w (stderr: %s)", args, err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}
