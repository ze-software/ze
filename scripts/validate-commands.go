// Design: (none -- build tool)
//
// validate-commands cross-checks YANG command tree declarations against
// registered RPC handlers. Reports mismatches in both directions:
//   - YANG ze:command referencing a handler that doesn't exist
//   - Registered handler with no YANG ze:command entry
//
// Usage: go run scripts/validate-commands.go [--json]
// Called by: make ze-validate-commands
//
//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all"

	// BGP cmd plugin schema packages (not in all.go -- triggered via reactor.go at runtime).
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/peer/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/raw/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/rib/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/update/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/route_refresh/schema"

	// BGP cmd handler packages (register RPCs via init()).
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/peer"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/raw"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/rib"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/update"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/route_refresh/handler"

	// General cmd handler packages (register RPCs via init()).
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/cache"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/commit"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/log"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/meta"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/metrics"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/subscribe"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/update"

	// Editor mode RPCs.
	_ "codeberg.org/thomas-mangin/ze/internal/component/cli"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"

	gyang "github.com/openconfig/goyang/pkg/yang"
)

// cmdModuleSuffix identifies YANG command tree modules by name convention.
// Canonical definition: internal/component/config/yang/command.go
const cmdModuleSuffix = "-cmd"

// CommandEntry represents a ze:command found in the YANG tree.
type CommandEntry struct {
	WireMethod string `json:"wire-method"`
	YANGPath   string `json:"yang-path"`
	Module     string `json:"module"`
}

// ValidationResult holds the cross-check output.
type ValidationResult struct {
	YANGCommands    []CommandEntry `json:"yang-commands"`
	Handlers        []string       `json:"handlers"`
	OrphanYANG      []CommandEntry `json:"orphan-yang"`
	OrphanHandlers  []string       `json:"orphan-handlers"`
	SkippedHandlers []string       `json:"skipped-handlers"`
	Total           int            `json:"total-yang"`
	TotalHandlers   int            `json:"total-handlers"`
	Valid           bool           `json:"valid"`
}

func main() {
	jsonMode := len(os.Args) > 1 && os.Args[1] == "--json"

	result := validate()

	if jsonMode {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fatal("json encode: " + err.Error())
		}
		if !result.Valid {
			os.Exit(1)
		}
		return
	}

	printResult(result)
	if !result.Valid {
		os.Exit(1)
	}
}

func fatal(msg string) {
	fmt.Printf("validate-commands: %s\n", msg)
	os.Exit(1)
}

// skippedWireMethods are handlers that don't need YANG command tree entries.
var skippedWireMethods = map[string]bool{
	"ze-editor:mode-command": true,
	"ze-editor:mode-edit":    true,
}

func validate() ValidationResult {
	loader, err := yang.DefaultLoader()
	if err != nil {
		fatal(err.Error())
	}

	// Discover -cmd modules dynamically from loaded YANG modules.
	var cmdModules []string
	for _, name := range loader.ModuleNames() {
		if strings.HasSuffix(name, cmdModuleSuffix) {
			cmdModules = append(cmdModules, name)
		}
	}
	sort.Strings(cmdModules)

	// Collect ze:command entries from YANG tree.
	var commands []CommandEntry
	for _, mod := range cmdModules {
		entry := loader.GetEntry(mod)
		if entry == nil {
			fmt.Printf("warning: module %s not found\n", mod)
			continue
		}
		walkEntry(entry, "", mod, &commands)
	}
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].WireMethod < commands[j].WireMethod
	})

	// Collect registered handlers.
	rpcs := pluginserver.AllBuiltinRPCs()
	handlerSet := make(map[string]bool, len(rpcs))
	var handlers []string
	var skipped []string
	for _, rpc := range rpcs {
		if skippedWireMethods[rpc.WireMethod] {
			skipped = append(skipped, rpc.WireMethod)
			continue
		}
		handlerSet[rpc.WireMethod] = true
		handlers = append(handlers, rpc.WireMethod)
	}
	sort.Strings(handlers)
	sort.Strings(skipped)

	// Build YANG command set.
	yangSet := make(map[string]bool, len(commands))
	for _, cmd := range commands {
		yangSet[cmd.WireMethod] = true
	}

	// Cross-check.
	var orphanYANG []CommandEntry
	for _, cmd := range commands {
		if !handlerSet[cmd.WireMethod] {
			orphanYANG = append(orphanYANG, cmd)
		}
	}

	var orphanHandlers []string
	for _, wm := range handlers {
		if !yangSet[wm] {
			orphanHandlers = append(orphanHandlers, wm)
		}
	}

	return ValidationResult{
		YANGCommands:    commands,
		Handlers:        handlers,
		OrphanYANG:      orphanYANG,
		OrphanHandlers:  orphanHandlers,
		SkippedHandlers: skipped,
		Total:           len(commands),
		TotalHandlers:   len(handlers),
		Valid:           len(orphanYANG) == 0 && len(orphanHandlers) == 0,
	}
}

func walkEntry(entry *gyang.Entry, path, module string, commands *[]CommandEntry) {
	if entry == nil || entry.Dir == nil {
		return
	}
	for name, child := range entry.Dir {
		// Match BuildCommandTree: only walk config false containers.
		if child.Config != gyang.TSFalse {
			continue
		}
		childPath := path + name
		wm := yang.GetCommandExtension(child)
		if wm != "" {
			*commands = append(*commands, CommandEntry{
				WireMethod: wm,
				YANGPath:   childPath,
				Module:     module,
			})
		}
		if child.Dir != nil {
			walkEntry(child, childPath+" > ", module, commands)
		}
	}
}

func printResult(r ValidationResult) {
	fmt.Printf("# Command Validation\n\n")
	fmt.Printf("YANG commands: %d\n", r.Total)
	fmt.Printf("Registered handlers: %d\n", r.TotalHandlers)
	fmt.Printf("Skipped (editor-internal): %d\n\n", len(r.SkippedHandlers))

	if len(r.OrphanYANG) > 0 {
		fmt.Printf("## YANG commands with no handler (%d)\n\n", len(r.OrphanYANG))
		for _, cmd := range r.OrphanYANG {
			fmt.Printf("  %s  (%s in %s)\n", cmd.WireMethod, cmd.YANGPath, cmd.Module)
		}
		fmt.Printf("\n")
	}

	if len(r.OrphanHandlers) > 0 {
		fmt.Printf("## Handlers with no YANG command (%d)\n\n", len(r.OrphanHandlers))
		for _, wm := range r.OrphanHandlers {
			fmt.Printf("  %s\n", wm)
		}
		fmt.Printf("\n")
	}

	if r.Valid {
		fmt.Printf("All commands validated.\n")
	} else {
		problems := len(r.OrphanYANG) + len(r.OrphanHandlers)
		fmt.Printf("FAILED: %d problem(s)\n", problems)
	}

	fmt.Printf("\n## All YANG commands (%d)\n\n", r.Total)
	fmt.Printf("| WireMethod | YANG Path | Module |\n")
	fmt.Printf("|------------|-----------|--------|\n")
	for _, cmd := range r.YANGCommands {
		fmt.Printf("| %s | %s | %s |\n", cmd.WireMethod, cmd.YANGPath, cmd.Module)
	}

	if len(r.SkippedHandlers) > 0 {
		fmt.Printf("\n## Skipped handlers (editor-internal)\n\n")
		fmt.Printf("| WireMethod | Reason |\n")
		fmt.Printf("|------------|--------|\n")
		for _, wm := range r.SkippedHandlers {
			reason := "editor mode command"
			if strings.Contains(wm, "mode-command") {
				reason = "run -- editor mode switch"
			} else if strings.Contains(wm, "mode-edit") {
				reason = "edit -- editor mode switch"
			}
			fmt.Printf("| %s | %s |\n", wm, reason)
		}
	}
}
