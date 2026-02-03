package bgp

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/config"
	"codeberg.org/thomas-mangin/ze/internal/plugin"
)

// rootFlags supports repeatable --root flag.
type rootFlags []string

func (r *rootFlags) String() string { return strings.Join(*r, ",") }
func (r *rootFlags) Set(v string) error {
	*r = append(*r, v)
	return nil
}

func cmdPluginTest(args []string) int {
	fs := flag.NewFlagSet("plugin-test", flag.ExitOnError)
	var plugins pluginFlags
	var roots rootFlags
	fs.Var(&plugins, "plugin", "plugin to test (repeatable: ze.hostname, ze.rib, ...)")
	fs.Var(&roots, "root", "config root to show (repeatable, default: bgp)")
	showSchema := fs.Bool("schema", false, "show schema fields for capability block")
	showTree := fs.Bool("tree", false, "show raw config tree that would be sent to plugins")
	showJSON := fs.Bool("json", false, "show JSON that would be sent to each plugin")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze bgp plugin test [options] <config-file>

Test plugin configuration and protocol behavior.
Useful for debugging plugin YANG schema loading and config delivery.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  ze bgp plugin test --plugin ze.hostname --schema config.conf
  ze bgp plugin test --plugin ze.hostname --tree config.conf
  ze bgp plugin test --plugin ze.hostname --json config.conf
  ze bgp plugin test --plugin ze.hostname --json --root bgp --root rib config.conf
  ze bgp plugin test --json --root bgp/peer config.conf
`)
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing config file\n")
		fs.Usage()
		return 1
	}

	configPath := fs.Arg(0)

	// Read config
	data, err := os.ReadFile(configPath) //nolint:gosec // Path from CLI
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading config: %v\n", err)
		return 1
	}

	// Collect plugin YANG
	pluginYANG := plugin.CollectPluginYANG(plugins)
	fmt.Printf("📦 Plugin YANG modules loaded: %d\n", len(pluginYANG))
	for name := range pluginYANG {
		fmt.Printf("   - %s\n", name)
	}
	fmt.Println()

	// Show schema if requested
	if *showSchema {
		fmt.Println("📋 Schema capability fields:")
		schema := config.YANGSchemaWithPlugins(pluginYANG)
		if schema == nil {
			fmt.Fprintf(os.Stderr, "error: failed to load YANG schema\n")
			return 1
		}

		// Try to find capability in bgp.peer
		if bgpNode := schema.Get("bgp"); bgpNode != nil {
			fmt.Printf("   bgp: %T\n", bgpNode)
			// Walk the schema to find capability
			showSchemaNode(bgpNode, "   ", 3)
		} else {
			fmt.Println("   (no bgp node found)")
		}
		fmt.Println()
	}

	// Parse config with plugin YANG
	schema := config.YANGSchemaWithPlugins(pluginYANG)
	if schema == nil {
		fmt.Fprintf(os.Stderr, "error: failed to load YANG schema\n")
		return 1
	}

	p := config.NewParser(schema)
	tree, err := p.Parse(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Parse error: %v\n", err)
		return 1
	}

	fmt.Println("✅ Config parsed successfully")

	// Default to "bgp" if no roots specified
	if len(roots) == 0 {
		roots = []string{"bgp"}
	}

	treeMap := tree.ToMap()

	// Show tree if requested
	if *showTree {
		for _, root := range roots {
			fmt.Printf("\n🌳 Config tree (root=%s):\n", root)
			subtree := extractSubtree(treeMap, root)
			if subtree != nil {
				jsonBytes, _ := json.MarshalIndent(subtree, "   ", "  ")
				fmt.Printf("   %s\n", string(jsonBytes))
			} else {
				fmt.Printf("   (root %q not found in tree)\n", root)
				fmt.Printf("   Available roots: %v\n", mapKeys(treeMap))
			}
		}
	}

	// Show JSON that would be sent to plugins
	if *showJSON {
		for _, root := range roots {
			fmt.Printf("\n📤 JSON delivery for root=%s:\n", root)
			subtree := extractSubtree(treeMap, root)
			if subtree != nil {
				jsonBytes, _ := json.Marshal(subtree)
				fmt.Printf("   config json %s %s\n", root, string(jsonBytes))
			} else {
				fmt.Printf("   (root %q not found)\n", root)
			}
		}
	}

	return 0
}

// showSchemaNode recursively shows schema node structure.
func showSchemaNode(node config.Node, indent string, depth int) {
	if depth <= 0 || node == nil {
		return
	}

	switch n := node.(type) {
	case *config.ContainerNode:
		for _, name := range n.Children() {
			child := n.Get(name)
			fmt.Printf("%s%s: %T\n", indent, name, child)
			showSchemaNode(child, indent+"  ", depth-1)
		}
	case *config.ListNode:
		fmt.Printf("%skey=%v children=[\n", indent, n.KeyType)
		for _, name := range n.Children() {
			child := n.Get(name)
			fmt.Printf("%s  %s: %T\n", indent, name, child)
			showSchemaNode(child, indent+"    ", depth-1)
		}
		fmt.Printf("%s]\n", indent)
	}
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// extractSubtree extracts a subtree based on path, wrapped in full path structure.
// "bgp/peer" returns {"bgp": {"peer": ...}} - always includes path from root.
func extractSubtree(tree map[string]any, path string) any {
	if path == "*" {
		return tree
	}

	// Split path and filter empty parts
	rawParts := strings.Split(path, "/")
	var parts []string
	for _, p := range rawParts {
		if p != "" {
			parts = append(parts, p)
		}
	}

	if len(parts) == 0 {
		return tree
	}

	// Navigate to the leaf data
	var current any = tree
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[part]
		if current == nil {
			return nil
		}
	}

	// Wrap the leaf data in its path structure (from leaf to root)
	result := current
	for i := len(parts) - 1; i >= 0; i-- {
		result = map[string]any{parts[i]: result}
	}
	return result
}
