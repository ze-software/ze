// Package schema provides the ze schema subcommand.
package schema

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
)

// Run executes the schema subcommand with the given arguments.
// Returns exit code.
func Run(args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}

	switch args[0] {
	case "list":
		return cmdList(args[1:])
	case "show":
		return cmdShow(args[1:])
	case "handlers":
		return cmdHandlers(args[1:])
	case "protocol":
		return cmdProtocol()
	case "help", "-h", "--help":
		usage()
		return 0
	default: // unknown command - report error
		fmt.Fprintf(os.Stderr, "unknown schema command: %s\n", args[0])
		usage()
		return 1
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `ze schema - Schema discovery commands

Usage:
  ze schema <command> [options]

Commands:
  list              List all registered schemas
  show <module>     Show YANG content for a module
  handlers          List handler → module mapping
  protocol          Show protocol version and format info
  help              Show this help

Examples:
  ze schema list
  ze schema show ze-bgp
  ze schema handlers
`)
}

// cmdList lists all registered schemas.
func cmdList(args []string) int {
	fs := flag.NewFlagSet("schema list", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return 1
	}

	registry := getSchemaRegistry()
	modules := registry.ListModules()

	if len(modules) == 0 {
		fmt.Println("No schemas registered")
		return 0
	}

	sort.Strings(modules)
	fmt.Println("Registered schemas:")
	for _, name := range modules {
		s, _ := registry.GetByModule(name)
		if s != nil {
			fmt.Printf("  %-20s %s (%s)\n", name, s.Namespace, s.Plugin)
		} else {
			fmt.Printf("  %s\n", name)
		}
	}

	return 0
}

// cmdShow shows YANG content for a specific module.
func cmdShow(args []string) int {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: ze schema show <module>\n")
		return 1
	}

	moduleName := args[0]
	registry := getSchemaRegistry()

	s, err := registry.GetByModule(moduleName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if s.Yang == "" {
		fmt.Printf("# Module: %s (no YANG content available)\n", moduleName)
		fmt.Printf("# Namespace: %s\n", s.Namespace)
		fmt.Printf("# Plugin: %s\n", s.Plugin)
		fmt.Printf("# Handlers: %s\n", strings.Join(s.Handlers, ", "))
		return 0
	}

	fmt.Println(s.Yang)
	return 0
}

// cmdHandlers lists handler → module mapping.
func cmdHandlers(args []string) int {
	_ = args // unused

	registry := getSchemaRegistry()
	handlers := registry.ListHandlers()

	if len(handlers) == 0 {
		fmt.Println("No handlers registered")
		return 0
	}

	// Sort handlers for consistent output
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
	fmt.Println("  Example: bgp.peer.timers → matches bgp.peer handler")
	fmt.Println()
	fmt.Println("Config Reader Protocol:")
	fmt.Println("  config schema <module> <yang-content>")
	fmt.Println("  config verify <handler-path> <data>")
	fmt.Println("  config apply <handler-path> <data>")
	fmt.Println("  config complete")
	return 0
}

// getSchemaRegistry returns the global schema registry.
// Currently returns a demo registry.
// In a running server, this would access the actual registry.
func getSchemaRegistry() *plugin.SchemaRegistry {
	// Create a demo registry with embedded YANG modules
	registry := plugin.NewSchemaRegistry()

	// Register ze-bgp schema (demo)
	if err := registry.Register(&plugin.Schema{
		Module:    "ze-bgp",
		Namespace: "urn:ze:bgp",
		Handlers:  []string{"bgp", "bgp.peer"},
		Plugin:    "core",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to register ze-bgp schema: %v\n", err)
	}

	// Register ze-plugin schema (demo)
	if err := registry.Register(&plugin.Schema{
		Module:    "ze-plugin",
		Namespace: "urn:ze:plugin",
		Handlers:  []string{"plugin", "plugin.external"},
		Plugin:    "core",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to register ze-plugin schema: %v\n", err)
	}

	// Register ze-types schema (demo)
	if err := registry.Register(&plugin.Schema{
		Module:    "ze-types",
		Namespace: "urn:ze:types",
		Handlers:  []string{},
		Plugin:    "core",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to register ze-types schema: %v\n", err)
	}

	return registry
}
