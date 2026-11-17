// Design: docs/architecture/config/yang-config-design.md — schema CLI
//
// Package schema provides the ze schema subcommand.
package cli

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/suggest"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Plugin ID prefix for internal plugins (e.g., "ze.bgp", "ze.gr").
const internalPluginPrefix = "ze."

// Module name prefix for Ze modules (e.g., "ze-bgp", "ze-types").
const moduleNamePrefix = "ze-"

// YANG module name for the core BGP configuration schema.
const bgpConfModule = "ze-bgp-conf"

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
		Command:   "ze schema",
		ShortHelp: "Schema discovery commands",
		Usage:     []string{"ze schema <command> [options]"},
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

	registry, err := commandLineRegistry(plugins)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	modules := registry.ListModules()
	if len(modules) == 0 {
		fmt.Println("No schemas registered")
		return 0
	}

	slices.Sort(modules)

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
	registry, err := commandLineRegistry(plugins)
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

	registry, err := commandLineRegistry(plugins)
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
	slices.Sort(paths)

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
			entries[i] = schemaEntry{wire: rpc.WireMethod, module: rpc.Module, desc: rpc.ShortHelp}
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
			entries[i] = schemaEntry{wire: notif.WireMethod, module: notif.Module, desc: notif.ShortHelp}
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

	registry, err := commandLineRegistry(plugins)
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
	schema, err := loader.Resolve()
	if err != nil {
		return fmt.Errorf("resolve YANG: %w", err)
	}

	// Each rpc is published under the method of the command node that points
	// at it (yang.PublishedRPCs). A notification declares its own event name.
	pub, err := yang.PublishedRPCs(schema)
	if err != nil {
		return fmt.Errorf("publish RPCs: %w", err)
	}
	if err := registry.RegisterRPCs(pub.Commands); err != nil {
		return fmt.Errorf("register RPCs: %w", err)
	}
	for _, moduleName := range schema.ModuleNames() {
		notifs := yang.ExtractNotifications(schema, moduleName)
		if len(notifs) > 0 {
			if err := registry.RegisterNotifications(notifs); err != nil {
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
	internalYANG := plugin.GetAllInternalPluginYANG()
	// Sorted, so the registry is built the same way on every call.
	for _, name := range slices.Sorted(maps.Keys(internalYANG)) {
		yangContent := internalYANG[name]
		pluginName := strings.TrimSuffix(strings.TrimPrefix(name, moduleNamePrefix), ".yang")
		wantsConfig := plugin.GetInternalPluginWantsConfig(pluginName)
		if err := registerYANG(registry, yangContent, internalPluginPrefix+pluginName, nil, wantsConfig, loaded); err != nil {
			return nil, fmt.Errorf("register %s: %w", name, err)
		}
	}

	// Register the schemas of the plugins named on the command line that are
	// compiled into ze. A spec that names an external program registers
	// nothing: with no daemon running ze starts no external plugin's program
	// (owner ruling), and commandLineRegistry says so beside the answer (writeExternalNotShown).
	//
	// A spec that names nothing is refused first: it is neither kind, and the
	// loop would otherwise skip it as an external program with an empty name.
	for _, pluginSpec := range extPlugins {
		if strings.TrimSpace(pluginSpec) == "" {
			return nil, emptyPluginSpecRefusal()
		}
		if !namesCompiledInPlugin(pluginSpec) {
			continue
		}
		yangContent, pluginName, err := getPluginYANG(pluginSpec)
		if err != nil {
			return nil, fmt.Errorf("get YANG from %s: %w", pluginSpec, err)
		}
		if err := registerYANG(registry, yangContent, pluginName, nil, nil, loaded); err != nil {
			return nil, fmt.Errorf("register %s: %w", pluginSpec, err)
		}
	}

	// Register the module each external plugin of this process declared when
	// ze started (yang.PublishedExternalModules). Only a running daemon holds
	// any: it joined them once, at start, and this reads that set and never
	// the plugin again. A command with no daemon holds none.
	published, _ := yang.PublishedExternalModules()
	wanted := pluginserver.ExternalConfigRoots()
	for _, module := range published {
		if err := registerPublishedYANG(registry, module, wanted[module.Plugin], loaded); err != nil {
			return nil, fmt.Errorf("register the module of plugin %s: %w", module.Plugin, err)
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

	// Skip if already loaded (deduplication by module name). A module another
	// module imports can be loaded first by tryAutoLoadInternal, which does not
	// know the owning plugin and so records no config roots; the owner's own
	// registration then carries them, and they are kept. Without this the
	// answer of `show schema list` depended on which plugin the map iteration
	// in buildSchemaRegistry met first.
	if loaded[meta.Module] {
		if len(wantsConfig) == 0 {
			return nil
		}
		if s, lookupErr := registry.GetByModule(meta.Module); lookupErr == nil && len(s.WantsConfig) == 0 {
			s.WantsConfig = wantsConfig
		}
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

// namesCompiledInPlugin reports whether a plugin spec of the command line
// names a plugin compiled into ze: "ze.name" or "ze-name". Every other spec
// names an external program.
func namesCompiledInPlugin(pluginSpec string) bool {
	if strings.HasPrefix(pluginSpec, internalPluginPrefix) {
		return true
	}
	return strings.HasPrefix(pluginSpec, moduleNamePrefix)
}

// emptyPluginSpecRefusal answers the operator error for a `--plugin` whose
// value is empty or only spaces: what is wrong, why ze schema does not answer
// without it, and the command that lists the names a `--plugin` can take.
func emptyPluginSpecRefusal() *opererr.Error {
	return opererr.New(
		"ze schema: a --plugin option names no plugin: its value is empty",
		"an empty value names neither a plugin compiled into ze, written ze.<name>, nor a program, "+
			"so the answer would leave out the plugin the command line meant and say nothing of it",
		"ze show plugin list")
}

// getPluginYANG answers the YANG and the plugin ID of a plugin compiled into
// ze, from its spec "ze.name" or "ze-name". The caller MUST pass a spec
// namesCompiledInPlugin holds for.
//
// Note: ze.X (goroutine mode) and ze-X (direct call mode) are treated identically
// for schema discovery because both just need the YANG content. The execution mode
// distinction only matters at runtime when actually running plugin commands.
func getPluginYANG(pluginSpec string) (string, string, error) {
	name := pluginSpec[len(internalPluginPrefix):] // both prefixes are the same length

	yangContent := plugin.GetInternalPluginYANG(name)
	if yangContent == "" {
		return "", "", fmt.Errorf("internal plugin %q has no YANG", name)
	}

	var tb3 textbuf.Buffer
	return yangContent, tb3.Str("ze.").Str(name).String(), nil
}

// registerPublishedYANG registers the module an external plugin declared when
// ze started, with wantsConfig the config roots that plugin declared it
// wants. The join that published the module already resolved its imports
// against the whole module set (yang.Join), so none is checked again here.
func registerPublishedYANG(registry *pluginserver.SchemaRegistry, module yang.ExternalModule, wantsConfig []string, loaded map[string]bool) error {
	meta, err := yang.ParseYANGMetadata(module.Text)
	if err != nil {
		return err
	}
	var displayImports []string
	for _, imp := range meta.Imports {
		if !coreModules[imp] {
			displayImports = append(displayImports, imp)
		}
	}
	if err := registry.Register(&pluginserver.Schema{
		Module:      meta.Module,
		Namespace:   yang.FormatNamespace(meta.Namespace),
		Yang:        module.Text,
		Imports:     displayImports,
		Handlers:    []string{},
		Plugin:      module.Plugin,
		WantsConfig: wantsConfig,
	}); err != nil {
		return err
	}
	loaded[meta.Module] = true
	return nil
}

// externalPluginSpecs answers the specs of plugins that name an external
// program, in the order given.
func externalPluginSpecs(plugins []string) []string {
	var external []string
	for _, pluginSpec := range plugins {
		if !namesCompiledInPlugin(pluginSpec) {
			external = append(external, pluginSpec)
		}
	}
	return external
}

// externalNotShown answers the operator error `ze schema` adds after its
// answer when the command line names external plugins: the answer holds the
// modules compiled into ze alone, why, and the command that reaches a running
// ze, which holds the module each external plugin declared. It answers nil
// when plugins names no external program.
//
// The why names the two commands a running ze has for it. The daemon serves
// no `show schema show <module>`: the text of one module is printed by the
// local `ze schema show` alone, so the why names `show yang tree config`,
// which shows the module's nodes. test/ui/external-yang-daemon-schema.ci
// sends both to a daemon that runs an external plugin.
//
// With no daemon running ze starts no external plugin's program (owner
// ruling), so nothing here runs a spec to read its module.
func externalNotShown(plugins []string) *opererr.Error {
	external := externalPluginSpecs(plugins)
	if len(external) == 0 {
		return nil
	}
	var problem textbuf.Buffer
	problem.Str("ze schema answered from the modules compiled into ze alone: the config schema of external plugin")
	if len(external) > 1 {
		problem.Byte('s')
	}
	for i, pluginSpec := range external {
		if i > 0 {
			problem.Byte(',')
		}
		problem.Byte(' ').Quoted(pluginSpec)
	}
	problem.Str(" is not available with no running ze")
	return opererr.New(problem.String(),
		"with no daemon running ze starts no external plugin's program, and such a plugin's module exists only in what its "+
			"program declares; a running ze holds the module each external plugin declared when it started: typed in a "+
			"`ze cli` session, `show schema list` lists that module and `show yang tree config` shows its nodes",
		"ze cli")
}

// commandLineRegistry answers the schema registry a `ze schema` command reads,
// for the plugins named on the command line. It is buildSchemaRegistry, which
// starts no program, and it says on stderr which of those plugins the answer
// shows nothing of (writeExternalNotShown). Every `ze schema` command that
// answers from the registry builds it here; a command the daemon serves names
// no plugin and calls buildSchemaRegistry.
func commandLineRegistry(plugins []string) (*pluginserver.SchemaRegistry, error) {
	registry, err := buildSchemaRegistry(plugins)
	if err != nil {
		return nil, err
	}
	writeExternalNotShown(plugins)
	return registry, nil
}

// writeExternalNotShown writes externalNotShown to stderr, beside the answer
// the command writes on stdout. It writes nothing when plugins names no
// external program.
func writeExternalNotShown(plugins []string) {
	notShown := externalNotShown(plugins)
	if notShown == nil {
		return
	}
	var tb textbuf.Buffer
	tb.Str("warning: ").Err(notShown).Byte('\n')
	tb.StdErr() //nolint:errcheck // a notice on stderr; the command's answer and exit code do not depend on it
}
