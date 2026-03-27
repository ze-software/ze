// Design: docs/architecture/api/commands.md — command verb taxonomy.
//
// command_inventory generates a markdown or JSON inventory of all registered
// commands, classified by verb (show, set, del, update, monitor, other).
//
// It imports the real plugin/RPC registrations and YANG command tree, so the
// output is always accurate.
//
// Usage: go run scripts/command_inventory.go [--json]
// Called by: make ze-command-list
//
//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	// Blank imports trigger init() registrations for all RPCs.
	// Uses schema/ subpackages where available (YANG only, no code deps).
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/monitor"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/peer"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/raw"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/rib"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/update"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/cache/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/commit/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/del/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/log/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/meta"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/metrics/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/set/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/show"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/subscribe/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/update/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/core/ipc/schema"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// CommandInfo holds metadata for a single command.
type CommandInfo struct {
	Verb       string `json:"verb"`
	Path       string `json:"path"`
	WireMethod string `json:"wire-method"`
	Help       string `json:"help"`
	ReadOnly   bool   `json:"read-only"`
	Source     string `json:"source"`
}

func main() {
	jsonMode := len(os.Args) > 1 && os.Args[1] == "--json"

	commands := collect()
	sort.Slice(commands, func(i, j int) bool {
		if commands[i].Verb != commands[j].Verb {
			return commands[i].Verb < commands[j].Verb
		}
		return commands[i].Path < commands[j].Path
	})

	if jsonMode {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(commands) // stdout write; error is not actionable
		return
	}

	printMarkdown(commands)
}

func collect() []CommandInfo {
	// Load YANG for wire-to-path mapping.
	loader, _ := yang.DefaultLoader()
	wireToPath := yang.WireMethodToPath(loader)

	var commands []CommandInfo

	// Builtin RPCs.
	for _, rpc := range pluginserver.AllBuiltinRPCs() {
		path := wireToPath[rpc.WireMethod]
		if path == "" {
			path = rpc.WireMethod // No YANG mapping, show wire method.
		}
		commands = append(commands, CommandInfo{
			Verb:       classifyVerb(path),
			Path:       path,
			WireMethod: rpc.WireMethod,
			Help:       rpc.Help,
			ReadOnly:   rpc.ReadOnly,
			Source:     "builtin",
		})
	}

	// Streaming handlers.
	for _, prefix := range pluginserver.StreamingPrefixes() {
		// Skip if already covered by a builtin RPC with the same path.
		found := false
		for _, c := range commands {
			if strings.EqualFold(c.Path, prefix) {
				found = true
				break
			}
		}
		if !found {
			commands = append(commands, CommandInfo{
				Verb:     classifyVerb(prefix),
				Path:     prefix,
				Help:     "(streaming)",
				ReadOnly: true,
				Source:   "streaming",
			})
		}
	}

	// TUI-only commands (not registered as RPCs or streaming handlers).
	// Only add if not already present from RPC registration.
	hasDashboard := false
	for _, c := range commands {
		if c.Path == "monitor bgp" {
			hasDashboard = true
			break
		}
	}
	if !hasDashboard {
		commands = append(commands, CommandInfo{
			Verb:     "monitor",
			Path:     "monitor bgp",
			Help:     "Live peer status dashboard (TUI)",
			ReadOnly: true,
			Source:   "cli",
		})
	}

	return commands
}

// classifyVerb extracts the verb from the first word of the CLI path.
func classifyVerb(path string) string {
	first, _, _ := strings.Cut(path, " ")
	first = strings.ToLower(first)
	switch first {
	case "show", "set", "del", "update", "monitor":
		return first
	}
	return "-"
}

func printMarkdown(commands []CommandInfo) {
	fmt.Printf("# Command Inventory\n\n")
	fmt.Printf("| Verb | CLI Path | Wire Method | Help | RO | Source |\n")
	fmt.Printf("|------|----------|-------------|------|----|--------|\n")
	for _, c := range commands {
		ro := "no"
		if c.ReadOnly {
			ro = "yes"
		}
		fmt.Printf("| %s | %s | %s | %s | %s | %s |\n",
			c.Verb, c.Path, c.WireMethod, c.Help, ro, c.Source)
	}
	fmt.Printf("\nTotal: %d commands\n", len(commands))
}
