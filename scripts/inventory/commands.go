// Design: docs/architecture/api/commands.md — command verb taxonomy.
//
// command_inventory generates a markdown or JSON inventory of all registered
// commands, classified by verb (show, set, del, update, monitor, other).
//
// It imports the real plugin/RPC registrations and YANG command tree, so the
// output is always accurate.
//
// Usage: go run scripts/inventory/commands.go [--json]
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
	// Uses plugin/all to match the runtime import set exactly.
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// CommandInfo holds metadata for a single command.
type CommandInfo struct {
	Verb       string `json:"verb"`
	Path       string `json:"path"`
	WireMethod string `json:"wire-method"`
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
				Verb:   classifyVerb(prefix),
				Path:   prefix,
				Source: "streaming",
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
			Verb:   "monitor",
			Path:   "monitor bgp",
			Source: "cli",
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
	fmt.Printf("| Verb | CLI Path | Wire Method | Source |\n")
	fmt.Printf("|------|----------|-------------|--------|\n")
	for _, c := range commands {
		fmt.Printf("| %s | %s | %s | %s |\n",
			c.Verb, c.Path, c.WireMethod, c.Source)
	}
	fmt.Printf("\nTotal: %d commands\n", len(commands))
}
