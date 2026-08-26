// Design: docs/architecture/core-design.md -- the YANG/handler contract gate
// Overview: report.go -- the answer this file produces
// Detail: contract_bgp.go -- the BGP command handlers, behind their own tag
//
// contract.go cross-checks the YANG command tree against the registered command
// handlers, in both directions: a ze:command node naming a handler nobody
// registered, and a registered handler no node names.
//
// It links the product on purpose. The question it answers is "what did ze
// register", and the only honest way to ask it is to load ze's registries and
// read them (spec-le-is-a-ze-binary, AC-3: the never-linked rule is
// directional). The blank imports below are the command islands that register
// through init() and are not reached by internal/component/plugin/all. The
// eight that register a BGP RPC live in contract_bgp.go, behind the ze_bgp tag
// the rest of that subtree is gated by.

package docvalid

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "github.com/ze-software/ze/internal/component/plugin/all"

	// BGP cmd plugin YANG packages (not in all.go -- triggered via reactor.go at runtime).
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/cache/yang"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/commit/yang"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/monitor/yang"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/peer/yang"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/raw/yang"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/rib/yang"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/update/yang"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/route_refresh/yang"

	// General cmd handler packages (register RPCs via init()).
	_ "github.com/ze-software/ze/internal/component/cmd/delete"
	_ "github.com/ze-software/ze/internal/component/cmd/log"
	_ "github.com/ze-software/ze/internal/component/cmd/meta"
	_ "github.com/ze-software/ze/internal/component/cmd/metrics"
	_ "github.com/ze-software/ze/internal/component/cmd/monitor"
	_ "github.com/ze-software/ze/internal/component/cmd/set"
	_ "github.com/ze-software/ze/internal/component/cmd/show"
	_ "github.com/ze-software/ze/internal/component/cmd/subscribe"
	_ "github.com/ze-software/ze/internal/component/cmd/update"

	// Interface RPC handler package (register RPCs via init()).
	_ "github.com/ze-software/ze/internal/component/iface/cmd"

	// Resolve RPC handler package (DNS, IRR, PeeringDB, Cymru lookups).
	_ "github.com/ze-software/ze/internal/component/resolve/cmd"

	// Editor mode RPCs.
	_ "github.com/ze-software/ze/internal/component/cli"

	"github.com/ze-software/ze/internal/component/config/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"

	gyang "github.com/openconfig/goyang/pkg/yang"
)

// cmdModuleSuffix identifies YANG command tree modules by name convention.
// Canonical definition: internal/component/config/yang/command.go.
const cmdModuleSuffix = "-cmd"

// skippedWireMethods are handlers that need no YANG command tree entry.
var skippedWireMethods = map[string]bool{
	"ze-editor:mode-command": true,
	"ze-editor:mode-edit":    true,
}

// skipReason says why a skipped handler is skipped, for the table Text prints.
func skipReason(wireMethod string) string {
	switch {
	case strings.Contains(wireMethod, "mode-command"):
		return "run -- editor mode switch"
	case strings.Contains(wireMethod, "mode-edit"):
		return "edit -- editor mode switch"
	default:
		return "editor mode command"
	}
}

// Validate cross-checks the YANG command tree in this process against the
// handlers registered in it, and reads root for the local command
// registrations that live in source rather than in a registry.
//
// The tree is a parameter rather than the working directory, so a test names a
// fixture by calling this function (spec-le-is-a-ze-binary, step 4: the
// scripts' --root flag became ZE_REPO_ROOT and an argument).
func Validate(root string) (ValidationResult, error) {
	loader, err := yang.DefaultLoader()
	if err != nil {
		return ValidationResult{}, err
	}

	// Discover -cmd modules dynamically from the loaded YANG modules.
	var cmdModules []string
	for _, name := range loader.ModuleNames() {
		if strings.HasSuffix(name, cmdModuleSuffix) {
			cmdModules = append(cmdModules, name)
		}
	}
	sort.Strings(cmdModules)

	// Collect the ze:command entries from the YANG tree.
	var commands []CommandEntry
	var warnings []string
	for _, mod := range cmdModules {
		entry := loader.GetEntry(mod)
		if entry == nil {
			warnings = append(warnings, mod)
			continue
		}
		walkEntry(entry, "", mod, &commands)
	}
	sortCommands(commands)

	// Collect the registered handlers.
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

	localHandlers, err := collectLocalHandlers(root)
	if err != nil {
		return ValidationResult{}, err
	}
	localSet := make(map[string]bool, len(localHandlers))
	for _, path := range localHandlers {
		localSet[path] = true
	}

	yangSet := make(map[string]bool, len(commands))
	yangCLIPathSet := make(map[string]bool, len(commands))
	for _, cmd := range commands {
		yangSet[cmd.WireMethod] = true
		yangCLIPathSet[yangPathToCLIPath(cmd.YANGPath)] = true
	}

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

	var orphanLocalHandlers []string
	for _, path := range localHandlers {
		if !yangCLIPathSet[path] {
			orphanLocalHandlers = append(orphanLocalHandlers, path)
		}
	}

	return ValidationResult{
		YANGCommands:        commands,
		Handlers:            handlers,
		LocalHandlers:       localHandlers,
		OrphanYANG:          orphanYANG,
		OrphanHandlers:      orphanHandlers,
		OrphanLocalHandlers: orphanLocalHandlers,
		SkippedHandlers:     skipped,
		Total:               len(commands),
		TotalHandlers:       len(handlers),
		TotalLocal:          len(localHandlers),
		Valid:               contractSatisfied(orphanYANG, orphanHandlers),
		Warnings:            warnings,
	}, nil
}

// sortCommands puts the command table in a TOTAL order: wire method, then YANG
// path, then module.
//
// The script sorted on the wire method alone, with sort.Slice, which is not
// stable. One wire method reached from two YANG paths is common -- an interface
// unit is created from two places in the tree -- and those rows arrived in
// whatever order a map walk produced, so two runs of the same gate over the
// same tree printed the table in different orders. Measured 2026-08-26: five
// runs of the script, four orders, six to eight lines apart. That table is
// pasted into documents and read by diff, and a report that changes when
// nothing changed cannot be diffed at all.
func sortCommands(commands []CommandEntry) {
	sort.Slice(commands, func(i, j int) bool {
		if commands[i].WireMethod != commands[j].WireMethod {
			return commands[i].WireMethod < commands[j].WireMethod
		}
		if commands[i].YANGPath != commands[j].YANGPath {
			return commands[i].YANGPath < commands[j].YANGPath
		}
		return commands[i].Module < commands[j].Module
	})
}

// contractSatisfied is the gate's verdict, and it reads BOTH directions: no
// YANG command node without a handler, and no registered handler without a
// node.
//
// It is a function of its own because it is the whole of what the gate
// ANSWERS, and the second half cannot be reached from any fixture tree: the
// handlers come from the process's own registry, so a test that varies the
// tree varies only the first half. Named here, the verdict is driven directly.
func contractSatisfied(orphanYANG []CommandEntry, orphanHandlers []string) bool {
	return len(orphanYANG) == 0 && len(orphanHandlers) == 0
}

// yangPathToCLIPath turns the tree path into the words an operator types.
func yangPathToCLIPath(path string) string {
	return strings.ReplaceAll(path, " > ", " ")
}

// collectLocalHandlers answers every command path registered from source under
// root, sorted.
func collectLocalHandlers(root string) ([]string, error) {
	files, err := localCommandRegistryFiles(root)
	if err != nil {
		return nil, err
	}
	paths := map[string]bool{}
	for _, path := range files {
		if err := collectLocalHandlersFromFile(path, paths); err != nil {
			return nil, err
		}
	}
	out := make([]string, 0, len(paths))
	for path := range paths {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}

// localCommandRegistryFiles answers the source files that register a local
// command handler.
//
// Migrated owners register their offline `show X` shortcuts from
// internal/.../cli/register.go (and internal/component/cli/client) through the
// importable registry rather than from cmd/ze, so those owner command packages
// are scanned too. Until the command-provider aggregator lands, this filesystem
// walk is what keeps the YANG/handler contract honest across the migration.
func localCommandRegistryFiles(root string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(root, "cmd", "ze", "*", "register.go"))
	if err != nil {
		return nil, err
	}
	files = append(files, filepath.Join(root, "cmd", "ze", "main.go"))

	internalRoot := filepath.Join(root, "internal")
	walkErr := filepath.Walk(internalRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// A tree with no internal/ registers no local handler, and a file
			// that VANISHED between the walk listing it and this callback was
			// never part of the tree either: another session shares this
			// checkout (letools/inventory, vanished). Anything else is a part
			// of the tree this scan cannot read, and a scan that silently
			// skips a register.go reports every command it holds as orphaned
			// in one direction and none in the other.
			if path == internalRoot || vanished(path) {
				return filepath.SkipAll
			}
			return err
		}
		if info.IsDir() || filepath.Base(path) != "register.go" {
			return nil
		}
		switch filepath.Base(filepath.Dir(path)) {
		case "cli", "client":
			files = append(files, path)
		case "crashes", "debug", "diag", "env", "explain", "host", "skills", "support":
			files = append(files, path)
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Strings(files)
	return files, nil
}

// vanished reports whether path is gone from the filesystem, which is how a
// read failure in a SHARED checkout is told from one this scan must report.
// The precedent and the reason are letools/inventory, vanished.
func vanished(path string) bool {
	_, err := os.Lstat(path)
	return errors.Is(err, fs.ErrNotExist)
}

// collectLocalHandlersFromFile records every command path registered by the
// file at path.
//
// Most callers use the bare `registry` name; help_ai.go aliases it as
// `cmdregistry` to avoid a collision with plugin/registry, so both are
// accepted. A command that answers with DATA registers a local handler too:
// RegisterLocalData builds one from the data handler so `ze <verb>` and
// `ze cli -c` render through one path. Omitting those names reported twelve
// YANG commands as having no handler on the day they were converted, when
// every one of them had gained one.
func collectLocalHandlersFromFile(path string, paths map[string]bool) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return err
	}
	var unquoteErr error
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
		if !ok || (pkg.Name != "cmdregistry" && pkg.Name != "registry") {
			return true
		}
		switch selector.Sel.Name {
		case "MustRegisterLocal", "MustRegisterLocalMeta", "RegisterLocal", "RegisterLocalMeta",
			"MustRegisterLocalData", "RegisterLocalData":
		default:
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		cmdPath, err := strconv.Unquote(literal.Value)
		if err != nil {
			unquoteErr = err
			return false
		}
		paths[cmdPath] = true
		return true
	})
	return unquoteErr
}

// walkEntry collects every ze:command node under entry.
//
// Only a `config false` container is walked, which is what BuildCommandTree
// does: the command tree is the operational half of the schema.
func walkEntry(entry *gyang.Entry, path, module string, commands *[]CommandEntry) {
	if entry == nil || entry.Dir == nil {
		return
	}
	for name, child := range entry.Dir {
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
			var tb textbuf.Buffer
			walkEntry(child, tb.Str(childPath).Str(" > ").String(), module, commands)
		}
	}
}
