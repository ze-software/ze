// Design: docs/architecture/config/syntax.md — config dump command
// Overview: main.go — dispatch and exit codes

package cli

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/infra"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/secret"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func cmdDump(args []string) int {
	fs := flag.NewFlagSet("config dump", flag.ExitOnError)
	stripPrivate := fs.Bool("strip-private", false, "replace sensitive values with /* SECRET-DATA */")
	user := fs.String("user", "", "account that asks the running ze about a config that declares an external plugin")
	fs.Usage = func() {
		p := helpfmt.Page{
			Command:   "ze config dump",
			ShortHelp: "Dump parsed configuration in human-readable form",
			Usage:     []string{"ze config dump [options] <config>"},
			Sections: []helpfmt.HelpSection{
				{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
					{Name: "--strip-private", Desc: "Replace sensitive values with /* SECRET-DATA */"},
					accountOptionHelp(),
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

	var diag textbuf.Buffer
	res, code := dumpAsking(configPath, configPath, *stripPrivate, command.RouteLocal, *user, routeAsk, &diag)
	diag.StdErr() //nolint:errcheck // CLI diagnostic
	if code != 0 {
		return code
	}

	printConfig(res.bgpTree, res.declaredPlugins(), res.sensitiveKeys, res.mode)
	return 0
}

// dumpPlugin is one external plugin declaration as printConfig prints it. A
// leaf the declaration does not hold is not printed.
type dumpPlugin struct {
	name       string
	run        string
	hasRun     bool
	encoder    string
	hasEncoder bool
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
	case config.DisplayPlain:
		return v
	default:
		panic("BUG: unknown config dump display mode")
	}
}

func printConfig(bgpTree map[string]any, plugins []dumpPlugin, sensitiveKeys map[string]bool, mode config.DisplayMode) {
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

	// The external plugin declarations (dumpResult.declaredPlugins).
	if len(plugins) == 0 {
		return
	}
	var list textbuf.Buffer
	list.Str("plugins:\n")
	for _, plugin := range plugins {
		list.Str("  - name: ").Str(plugin.name).Byte('\n')
		if plugin.hasRun {
			list.Str("    run: ").Str(plugin.run).Byte('\n')
		}
		if plugin.hasEncoder {
			list.Str("    encoder: ").Str(plugin.encoder).Byte('\n')
		}
	}
	list.StdOut() //nolint:errcheck // CLI output, nothing to report a stdout failure to
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

// dumpResult is everything the two callers of dumpAsking need from one dump:
// the resolved map `show config dump` answers with, and the pieces printConfig
// renders for a reader.
//
// tree is the parsed config of a dump made in this process. A dump the running
// ze made holds none, and holds the plugin declarations of its document in
// plugins instead (dumpResult.declaredPlugins reads whichever is there).
type dumpResult struct {
	dumpMap       map[string]any
	bgpTree       map[string]any
	tree          *config.Tree
	plugins       []dumpPlugin
	sensitiveKeys map[string]bool
	mode          config.DisplayMode
}

// declaredPlugins answers the external plugin declarations of a dump: from
// the parsed tree of a dump made in this process, in the order the config
// declares them, and the ones dumpAsking read from the daemon's document
// otherwise.
func (r dumpResult) declaredPlugins() []dumpPlugin {
	if r.tree == nil {
		return r.plugins
	}
	container := r.tree.GetContainer(keyPluginRoot)
	if container == nil {
		return nil
	}
	entries := container.GetListOrdered(keyPluginExternal)
	plugins := make([]dumpPlugin, 0, len(entries))
	for _, entry := range entries {
		plugin := dumpPlugin{name: entry.Key}
		plugin.run, plugin.hasRun = entry.Value.Get(keyPluginRun)
		plugin.encoder, plugin.hasEncoder = entry.Value.Get(keyPluginEncoder)
		plugins = append(plugins, plugin)
	}
	return plugins
}

// resolveDumpBytes is the file and supplied-content routes' shared producer.
// It parses and masks bytes without reading or publishing configuration.
//
// It is the payload half of cmdDump, so `show config dump` answers with DATA
// and the two spellings cannot drift (dumpAsking reads the file and calls it).
// It writes nothing itself. The parse warnings of a config it still resolved
// go into diag, whole lines, and each caller prints diag where its route puts
// diagnostics: a data handler runs on routes that are not the operator's
// terminal.
func resolveDumpBytes(data []byte, stripPrivate bool, diag *textbuf.Buffer) (dumpResult, error) {
	schema, declared, err := config.SchemaWithDeclaredPlugins(config.ConfigSource{Text: string(data)})
	if err != nil {
		return dumpResult{}, opererr.Wrap(err, "The configuration schema could not be loaded.",
			"The configuration could not be interpreted; no dump was produced.",
			"ze show doctor")
	}
	p := config.NewParser(schema)
	tree, err := p.Parse(string(data))
	if err != nil {
		if refusal := declaredParseExplanation(declared, err); refusal != nil {
			return dumpResult{}, refusal
		}
		return dumpResult{}, opererr.Wrap(err, "Error parsing config",
			"The supplied configuration could not be parsed; no configuration dump was produced.",
			"ze config validate -")
	}

	if warnings := p.Warnings(); len(warnings) > 0 {
		diag.Str("Warnings:\n")
		for _, w := range warnings {
			diag.Str("  ").Str(w).Byte('\n')
		}
		diag.Byte('\n')
	}

	// Resolve the bgp{} section through the always-on seam. On a binary built
	// without the BGP engine (//go:build ze_bgp) a config with no bgp{} block
	// resolves to an empty tree and the dump proceeds from the parsed tree
	// alone; one WITH a bgp{} block is an error rather than a silently
	// BGP-less dump (the seam fails closed).
	bgpTree, err := infra.ResolveBGPTree(tree)
	if err != nil {
		return dumpResult{}, opererr.Wrap(err, "Error resolving config",
			"The supplied configuration could not be resolved; no configuration dump was produced.",
			"ze config validate -")
	}

	mode := config.DisplayEncode
	if stripPrivate {
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

	dumpMap := tree.ToMap()
	dumpMap["bgp"] = bgpTree
	maskMapValues(dumpMap, sensitiveKeys, mode)
	return dumpResult{
		dumpMap:       dumpMap,
		bgpTree:       bgpTree,
		tree:          tree,
		sensitiveKeys: sensitiveKeys,
		mode:          mode,
	}, nil
}
