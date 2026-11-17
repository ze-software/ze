// Design: docs/architecture/api/process-protocol.md — plugin CLI dispatch

package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// pluginFlags collects multiple --plugin flag values.
type pluginFlags []string

func (p *pluginFlags) String() string {
	return textbuf.Join(*p, ",")
}

func (p *pluginFlags) Set(value string) error {
	*p = append(*p, value)
	return nil
}

// rootFlags supports repeatable --root flag.
type rootFlags []string

func (r *rootFlags) String() string { return textbuf.Join(*r, ",") }
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
		p := helpfmt.Page{
			Command:   "ze plugin test",
			ShortHelp: "Test plugin configuration and protocol behavior",
			Usage:     []string{"ze plugin test [options] <config-file>"},
			Sections: []helpfmt.HelpSection{
				{Title: "Description", Entries: []helpfmt.HelpEntry{
					{Name: "", Desc: "Useful for debugging plugin YANG schema loading and config delivery."},
				}},
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "--plugin <name>", Desc: "Plugin to test (repeatable: ze.hostname, ze.rib, ...)"},
					{Name: "--root <path>", Desc: "Config root to show (repeatable, default: bgp)"},
					{Name: "--schema", Desc: "Show schema fields for capability block"},
					{Name: "--tree", Desc: "Show raw config tree that would be sent to plugins"},
					{Name: "--json", Desc: "Show JSON that would be sent to each plugin"},
				}},
			},
			Examples: []string{
				"ze plugin test --plugin ze.hostname --schema config.conf",
				"ze plugin test --plugin ze.hostname --tree config.conf",
				"ze plugin test --plugin ze.hostname --json config.conf",
				"ze plugin test --plugin ze.hostname --json --root bgp --root rib config.conf",
				"ze plugin test --json --root bgp/peer config.conf",
			},
		}
		p.WriteErr()
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

	// Read config (or stdin when "-").
	data, err := cliio.ReadFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading config: %v\n", err)
		return 1
	}

	// The modules of the named plugins that the schema below holds, and the
	// named plugins whose module it does not: this command parses with the
	// modules init() registered, and joins no module from a registration.
	moduleFiles, unregistered := pluginModuleFiles(plugins)
	var modules textbuf.Buffer
	modules.Str("📦 Plugin YANG modules loaded: ").Int(int64(len(moduleFiles))).Byte('\n')
	for _, file := range moduleFiles {
		modules.Str("   - ").Str(file).Byte('\n')
	}
	for _, name := range unregistered {
		modules.Str("   ! ").Str(name).
			Str(": its YANG module is not loaded, because no init() registers it, so this command refuses its config section.\n").
			Str("     Put the module in the plugin's yang/ directory, then run:\n").
			Str("     ./le yang glue update\n")
	}
	modules.Byte('\n')
	if writeErr := modules.StdOut(); writeErr != nil {
		return 1
	}

	// Show schema if requested
	if *showSchema {
		fmt.Println("📋 Schema capability fields:")
		schema, schemaErr := config.YANGSchema()
		if schemaErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", schemaErr)
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

	// Parse config with the process-wide schema, which holds every module
	// init() registered and none of the plugins reported unregistered above. No
	// external plugin is started to read its module: a section of one is
	// refused below, with the command that reaches a running ze.
	schema, declared, schemaErr := config.SchemaWithDeclaredPlugins(config.ConfigSource{Text: string(data), Path: configPath})
	if schemaErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", schemaErr)
		return 1
	}

	p := config.NewParser(schema)
	tree, err := p.Parse(string(data))
	if err != nil {
		// One operator error: the refusal, a declared external plugin ze holds
		// no schema of when that explains it, and the command that fixes it.
		var refusal textbuf.Buffer
		refusal.Str("error: ").
			Err(declared.ParseRefusal(configPath, err, "ze plugin test starts no plugin from a config it cannot parse")).
			Byte('\n')
		refusal.StdErr() //nolint:errcheck // the exit code carries the failure; a stderr write error has no other channel
		return 1
	}

	fmt.Println("✅ Config parsed successfully")

	// Default to "bgp" if no roots specified
	if len(roots) == 0 {
		roots = []string{"bgp"}
	}

	// ToPluginMap, not ToMap: both blocks below print what a plugin RECEIVES,
	// and a command that shows a different payload from the one the daemon
	// sends is worse than one that shows nothing.
	treeMap := tree.ToPluginMap()

	// Show tree if requested
	if *showTree {
		for _, root := range roots {
			fmt.Printf("\n🌳 Config tree (root=%s):\n", root)
			subtree := pluginserver.ExtractConfigSubtree(treeMap, root)
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
			subtree := pluginserver.ExtractConfigSubtree(treeMap, root)
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

// pluginModuleFiles answers, in command-line order and once each, the file
// name the YANG module of each named internal plugin is registered under, and
// the named plugins whose YANG text no registered module carries. A
// registration's text is normally the one its yang package's init()
// registered (configyang.RegisterModule), so the process's schema holds it. A
// text no init() registered is joined by ze start --plugin
// (config.LoadConfig), but not by this command, whose schema is the
// registered set alone (config.SchemaWithDeclaredPlugins): such a plugin is
// answered in unregistered, never dropped and never counted as loaded. A name
// with the "ze." prefix is the plugin without it. A name no internal plugin
// registers, or a plugin with no YANG, adds nothing.
func pluginModuleFiles(plugins []string) (files, unregistered []string) {
	for _, name := range plugins {
		pluginName := strings.TrimPrefix(name, "ze.")
		text := plugin.GetInternalPluginYANG(pluginName)
		if text == "" {
			continue
		}
		file, registered := registeredModuleFile(text)
		if !registered {
			if !slices.Contains(unregistered, pluginName) {
				unregistered = append(unregistered, pluginName)
			}
			continue
		}
		if !slices.Contains(files, file) {
			files = append(files, file)
		}
	}
	return files, unregistered
}

// registeredModuleFile answers the file name of the registered module whose
// content is text, and false when no registered module carries it.
func registeredModuleFile(text string) (string, bool) {
	for _, module := range configyang.Modules() {
		if module.Content == text {
			return module.Name, true
		}
	}
	return "", false
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
