// Design: docs/architecture/config/syntax.md — config dump command
// Overview: main.go — dispatch and exit codes

package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/config/infra"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/secret"
	"codeberg.org/thomas-mangin/ze/internal/core/cliio"
	"codeberg.org/thomas-mangin/ze/internal/core/helpfmt"
)

func cmdDump(args []string) int {
	fs := flag.NewFlagSet("config dump", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "output as JSON")
	stripPrivate := fs.Bool("strip-private", false, "replace sensitive values with /* SECRET-DATA */")
	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze config dump",
			Summary: "Dump parsed configuration in human-readable or JSON format",
			Usage:   []string{"ze config dump [options] <config>"},
			Sections: []helpfmt.HelpSection{
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "--json", Desc: "Output as JSON"},
					{Name: "--strip-private", Desc: "Replace sensitive values with /* SECRET-DATA */"},
				}},
			},
		}
		p.WriteErr()
		fmt.Fprintf(os.Stderr, "\nSensitive values (passwords, keys) are displayed as $9$-encoded by default.\n")
		fmt.Fprintf(os.Stderr, "Use --strip-private to replace them with /* SECRET-DATA */ for sharing.\n")
		fmt.Fprintf(os.Stderr, "Use - to read from stdin.\n")
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

	data, err := cliio.ReadFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
		return 1
	}

	schema, err := config.YANGSchema()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
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

	// Resolve the bgp{} section through the always-on seam. On a binary built
	// without the BGP engine (//go:build ze_bgp) a config with no bgp{} block
	// resolves to an empty tree and the dump proceeds from the parsed tree
	// alone; one WITH a bgp{} block is an error rather than a silently
	// BGP-less dump (the seam fails closed).
	bgpTree, err := infra.ResolveBGPTree(tree)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving config: %v\n", err)
		return 1
	}

	mode := config.DisplayEncode
	if *stripPrivate {
		mode = config.DisplayStrip
	}
	sensitiveKeys := config.SensitiveKeys(schema)

	// Mask ze:bcrypt leaves with the placeholder BEFORE dumping. Bcrypt is
	// one-way, so it is always the strip placeholder, never $9$ (reversible; the
	// parser also refuses $9$ on a bcrypt leaf). MaskBcrypt clones, so the on-disk
	// config is untouched. The sensitive/$9$ masking below skips a value that is
	// already the placeholder, so a bcrypt leaf whose name also happens to be
	// ze:sensitive in another module is not re-encoded as $9$.
	tree = config.MaskBcrypt(tree, schema)

	if *jsonOutput {
		// Build full dump with resolved BGP section.
		dumpMap := tree.ToMap()
		dumpMap["bgp"] = bgpTree
		maskMapValues(dumpMap, sensitiveKeys, mode)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(dumpMap); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	printConfig(bgpTree, tree, sensitiveKeys, mode)
	return 0
}

// maskMapValues recursively replaces sensitive values in a map. A value already
// equal to SecretDataPlaceholder (e.g. a ze:bcrypt leaf masked by MaskBcrypt) is
// left as-is, never re-encoded as $9$.
func maskMapValues(m map[string]any, sensitiveKeys map[string]bool, mode config.DisplayMode) {
	if mode == config.DisplayPlain {
		return
	}
	for k, v := range m {
		switch val := v.(type) {
		case map[string]any:
			maskMapValues(val, sensitiveKeys, mode)
		case string:
			if sensitiveKeys[k] && val != config.SecretDataPlaceholder {
				m[k] = maskValue(val, mode)
			}
		}
	}
}

// maskValue applies the display mode to a sensitive value.
func maskValue(v string, mode config.DisplayMode) string {
	switch mode {
	case config.DisplayStrip:
		return config.SecretDataPlaceholder
	case config.DisplayEncode:
		encoded, err := secret.Encode(v)
		if err != nil {
			slog.Debug("secret encode failed", "error", err)
			return config.SecretDataPlaceholder
		}
		return encoded
	default:
		return v
	}
}

func printConfig(bgpTree map[string]any, tree *config.Tree, sensitiveKeys map[string]bool, mode config.DisplayMode) {
	// Global BGP settings.
	if v, ok := bgpTree["router-id"]; ok {
		fmt.Printf("router-id: %s\n", v)
	}
	if localMap, ok := bgpTree["local"].(map[string]any); ok {
		if v, ok := localMap["as"]; ok {
			fmt.Printf("local-as: %s\n", v)
		}
	}
	if v, ok := bgpTree["listen"]; ok {
		fmt.Printf("listen: %s\n", v)
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
		printTreeMap(peer, "  ", sensitiveKeys, mode)
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
func printTreeMap(m map[string]any, indent string, sensitiveKeys map[string]bool, mode config.DisplayMode) {
	for k, v := range m {
		switch val := v.(type) {
		case map[string]any:
			fmt.Printf("%s%s:\n", indent, k)
			printTreeMap(val, indent+"  ", sensitiveKeys, mode)
		default:
			display := fmt.Sprintf("%v", val)
			if sensitiveKeys[k] {
				display = maskValue(display, mode)
			}
			fmt.Printf("%s%s: %s\n", indent, k, display)
		}
	}
}
