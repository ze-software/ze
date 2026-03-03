// Design: docs/architecture/config/syntax.md — config dump command
// Overview: main.go — dispatch and exit codes

package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	bgpconfig "codeberg.org/thomas-mangin/ze/internal/component/bgp/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

func cmdDump(args []string) int {
	fs := flag.NewFlagSet("config dump", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "output as JSON")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze config dump [options] <config>

Dump parsed configuration in human-readable or JSON format.
Useful for debugging config parsing issues.
Use - to read from stdin.

Options:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing config file (use - for stdin)\n")
		fs.Usage()
		return 1
	}

	configPath := fs.Arg(0)

	var data []byte
	var err error
	if configPath == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(configPath) //nolint:gosec // Path from CLI
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
		return 1
	}

	schema := config.YANGSchema()
	if schema == nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load YANG schema\n")
		return 1
	}
	p := config.NewParser(schema)
	tree, err := p.Parse(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing config: %v\n", err)
		return 1
	}

	if warnings := p.Warnings(); len(warnings) > 0 {
		fmt.Fprintf(os.Stderr, "Warnings:\n")
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "  %s\n", w)
		}
		fmt.Fprintln(os.Stderr)
	}

	bgpTree, err := bgpconfig.ResolveBGPTree(tree)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving config: %v\n", err)
		return 1
	}

	if *jsonOutput {
		// Build full dump with resolved BGP section.
		dumpMap := tree.ToMap()
		dumpMap["bgp"] = bgpTree
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(dumpMap); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	printConfig(bgpTree, tree)
	return 0
}

func printConfig(bgpTree map[string]any, tree *config.Tree) {
	// Global BGP settings.
	for _, key := range []string{"router-id", "local-as", "listen"} {
		if v, ok := bgpTree[key]; ok {
			fmt.Printf("%s: %s\n", key, v)
		}
	}
	fmt.Println()

	// Peers from resolved tree.
	peers, _ := bgpTree["peer"].(map[string]any)
	for addr, v := range peers {
		peer, ok := v.(map[string]any)
		if !ok {
			continue
		}
		fmt.Printf("peer %s:\n", addr)
		printTreeMap(peer, "  ")
		fmt.Println()
	}

	// Plugins from original tree.
	if pluginContainer := tree.GetContainer("plugin"); pluginContainer != nil {
		plugins := pluginContainer.GetListOrdered("external")
		if len(plugins) > 0 {
			fmt.Printf("plugins:\n")
			for _, entry := range plugins {
				fmt.Printf("  - name: %s\n", entry.Key)
				if run, ok := entry.Value.Get("run"); ok {
					fmt.Printf("    run: %s\n", run)
				}
				if enc, ok := entry.Value.Get("encoder"); ok {
					fmt.Printf("    encoder: %s\n", enc)
				}
			}
		}
	}
}

// printTreeMap prints a map[string]any tree in a readable key-value format.
func printTreeMap(m map[string]any, indent string) {
	for k, v := range m {
		switch val := v.(type) {
		case map[string]any:
			fmt.Printf("%s%s:\n", indent, k)
			printTreeMap(val, indent+"  ")
		default:
			fmt.Printf("%s%s: %v\n", indent, k, val)
		}
	}
}
