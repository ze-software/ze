package bgp

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
)

// cmdSchema handles the "schema" subcommand for schema discovery.
func cmdSchema(args []string) int {
	if len(args) < 1 {
		schemaUsage()
		return 1
	}

	switch args[0] {
	case "list":
		return cmdSchemaList(args[1:])
	case "show":
		return cmdSchemaShow(args[1:])
	case "handlers":
		return cmdSchemaHandlers(args[1:])
	case "protocol":
		return cmdSchemaProtocol()
	case "help", "-h", "--help":
		schemaUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown schema command: %s\n", args[0])
		schemaUsage()
		return 1
	}
}

func schemaUsage() {
	fmt.Fprintf(os.Stderr, `ze bgp schema - Schema discovery commands

Usage:
  ze bgp schema <command> [options]

Commands:
  list              List all registered schemas
  show <module>     Show YANG content for a module
  handlers          List handler → module mapping
  protocol          Show protocol version and format info
  help              Show this help

Examples:
  ze bgp schema list
  ze bgp schema show ze-bgp
  ze bgp schema handlers
`)
}

// cmdSchemaList lists all registered schemas.
func cmdSchemaList(args []string) int {
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
		schema, _ := registry.GetByModule(name)
		if schema != nil {
			fmt.Printf("  %-20s %s (%s)\n", name, schema.Namespace, schema.Plugin)
		} else {
			fmt.Printf("  %s\n", name)
		}
	}

	return 0
}

// cmdSchemaShow shows YANG content for a specific module.
func cmdSchemaShow(args []string) int {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: ze bgp schema show <module>\n")
		return 1
	}

	moduleName := args[0]
	registry := getSchemaRegistry()

	schema, err := registry.GetByModule(moduleName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if schema.Yang == "" {
		fmt.Printf("# Module: %s (no YANG content available)\n", moduleName)
		fmt.Printf("# Namespace: %s\n", schema.Namespace)
		fmt.Printf("# Plugin: %s\n", schema.Plugin)
		fmt.Printf("# Handlers: %s\n", strings.Join(schema.Handlers, ", "))
		return 0
	}

	fmt.Println(schema.Yang)
	return 0
}

// cmdSchemaHandlers lists handler → module mapping.
func cmdSchemaHandlers(args []string) int {
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

// cmdSchemaProtocol shows protocol version and format info.
func cmdSchemaProtocol() int {
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
// Currently returns an empty registry for demonstration.
// In a running server, this would access the actual registry.
func getSchemaRegistry() *plugin.SchemaRegistry {
	// Create a demo registry with embedded YANG modules
	registry := plugin.NewSchemaRegistry()

	// Register ze-bgp schema (demo)
	_ = registry.Register(&plugin.Schema{
		Module:    "ze-bgp",
		Namespace: "urn:ze:bgp",
		Handlers:  []string{"bgp", "bgp.peer", "bgp.peer-group", "bgp.route-map", "bgp.prefix-list"},
		Plugin:    "core",
	})

	// Register ze-plugin schema (demo)
	_ = registry.Register(&plugin.Schema{
		Module:    "ze-plugin",
		Namespace: "urn:ze:plugin",
		Handlers:  []string{"plugin", "plugin.external"},
		Plugin:    "core",
	})

	// Register ze-types schema (demo)
	_ = registry.Register(&plugin.Schema{
		Module:    "ze-types",
		Namespace: "urn:ze:types",
		Handlers:  []string{},
		Plugin:    "core",
	})

	return registry
}
