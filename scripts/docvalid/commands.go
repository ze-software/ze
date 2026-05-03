// Design: (none -- build tool)
//
// validate-commands cross-checks YANG command tree declarations against
// registered command handlers. Reports mismatches in both directions:
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
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all"

	// BGP cmd plugin schema packages (not in all.go -- triggered via reactor.go at runtime).
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/monitor/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/peer/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/raw/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/rib/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/update/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/route_refresh/schema"

	// BGP cmd handler packages (register RPCs via init()).
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/cache"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/commit"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/monitor"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/peer"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/raw"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/rib"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/update"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/route_refresh/handler"

	// General cmd handler packages (register RPCs via init()).
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/cache"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/commit"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/del"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/log"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/meta"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/metrics"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/set"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/show"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/subscribe"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/update"

	// Interface RPC handler package (register RPCs via init()).
	_ "codeberg.org/thomas-mangin/ze/internal/component/iface/cmd"

	// Resolve RPC handler package (DNS, IRR, PeeringDB, Cymru lookups).
	_ "codeberg.org/thomas-mangin/ze/internal/component/resolve/cmd"

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
	YANGCommands         []CommandEntry `json:"yang-commands"`
	Handlers             []string       `json:"handlers"`
	LocalHandlers        []string       `json:"local-handlers"`
	OrphanYANG           []CommandEntry `json:"orphan-yang"`
	OrphanHandlers       []string       `json:"orphan-handlers"`
	OrphanLocalHandlers  []string       `json:"orphan-local-handlers"`
	SkippedHandlers      []string       `json:"skipped-handlers"`
	Total                int            `json:"total-yang"`
	TotalHandlers        int            `json:"total-handlers"`
	TotalLocal           int            `json:"total-local-handlers"`
	Valid                bool           `json:"valid"`
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

	localHandlers := collectLocalHandlers()
	localSet := make(map[string]bool, len(localHandlers))
	for _, path := range localHandlers {
		localSet[path] = true
	}
	sort.Strings(localHandlers)

	// Build YANG command set.
	yangSet := make(map[string]bool, len(commands))
	for _, cmd := range commands {
		yangSet[cmd.WireMethod] = true
	}

	// Cross-check.
	var orphanYANG []CommandEntry
	for _, cmd := range commands {
		if !handlerSet[cmd.WireMethod] && !localSet[yangPathToCLIPath(cmd.YANGPath)] {
			orphanYANG = append(orphanYANG, cmd)
		}
	}

	var orphanHandlers []string
	for _, wm := range handlers {
		if !yangSet[wm] {
			orphanHandlers = append(orphanHandlers, wm)
		}
	}

	yangCLIPathSet := make(map[string]bool, len(commands))
	for _, cmd := range commands {
		yangCLIPathSet[yangPathToCLIPath(cmd.YANGPath)] = true
	}
	var orphanLocalHandlers []string
	for _, path := range localHandlers {
		if !yangCLIPathSet[path] {
			orphanLocalHandlers = append(orphanLocalHandlers, path)
		}
	}

	return ValidationResult{
		YANGCommands:         commands,
		Handlers:             handlers,
		LocalHandlers:        localHandlers,
		OrphanYANG:           orphanYANG,
		OrphanHandlers:       orphanHandlers,
		OrphanLocalHandlers:  orphanLocalHandlers,
		SkippedHandlers:      skipped,
		Total:                len(commands),
		TotalHandlers:        len(handlers),
		TotalLocal:           len(localHandlers),
		Valid:                len(orphanYANG) == 0 && len(orphanHandlers) == 0,
	}
}

func yangPathToCLIPath(path string) string {
	return strings.ReplaceAll(path, " > ", " ")
}

func collectLocalHandlers() []string {
	paths := map[string]bool{}
	for _, path := range localCommandRegistryFiles() {
		collectLocalHandlersFromFile(path, paths)
	}
	out := make([]string, 0, len(paths))
	for path := range paths {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func localCommandRegistryFiles() []string {
	files, err := filepath.Glob("cmd/ze/*/register.go")
	if err != nil {
		fatal("local command registry glob: " + err.Error())
	}
	files = append(files, "cmd/ze/main.go")
	sort.Strings(files)
	return files
}

func collectLocalHandlersFromFile(path string, paths map[string]bool) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		fatal("parse local command registry " + path + ": " + err.Error())
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		if !ok || pkg.Name != "cmdregistry" {
			return true
		}
		switch selector.Sel.Name {
		case "MustRegisterLocal", "MustRegisterLocalMeta", "RegisterLocal", "RegisterLocalMeta":
		default:
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		cmdPath, err := strconv.Unquote(literal.Value)
		if err != nil {
			fatal("unquote local command registry path in " + path + ": " + err.Error())
		}
		paths[cmdPath] = true
		return true
	})
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
	fmt.Printf("Registered local handlers: %d\n", r.TotalLocal)
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

	if len(r.OrphanLocalHandlers) > 0 {
		fmt.Printf("## Local handlers with no YANG command (%d)\n\n", len(r.OrphanLocalHandlers))
		for _, path := range r.OrphanLocalHandlers {
			fmt.Printf("  %s\n", path)
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
