// Design: docs/architecture/api/commands.md -- command surface ownership gate
//
// command_ownership enforces the command-surface-ownership invariants:
//
//  1. No owner command package (internal/.../cli or .../client) imports cmd/ze
//     or cmd/ze/internal -- owners must be reachable without depending on the
//     process entry point (AC-1).
//  2. Every registry.RegisterRootHandler / MustRegisterRootHandler call lives
//     in an internal/ package, never under cmd/ze -- owner-backed roots are
//     owner-owned, not registered centrally.
//  3. Every metadata-only root (registry.RegisterRoot, the no-owner form) that
//     remains under cmd/ze is in the no-owner allowlist with a stated reason
//     (AC-4, AC-8). Adding a root centrally without allowlisting it fails.
//
// Usage:   go run scripts/checks/command_ownership.go [--json]
// Called by: make ze-command-ownership-check
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
)

// noOwnerAllowlist is the set of root commands that may stay in cmd/ze because
// no internal component, plugin, or backend owns the behaviour. Each entry
// names why it is process-global. This is a fixture, not a comment: a central
// root not listed here fails the gate.
var noOwnerAllowlist = map[string]string{
	"help":      "Describes the whole process command surface.",
	"version":   "Uses binary stamp and process build metadata.",
	"start":     "Starts the daemon and wires global process dependencies.",
	"install":   "Host installation for the ze binary; no runtime component owns host package installation.",
	"uninstall": "Host removal of the ze binary, unit, and config.",
	"service":   "Host service (systemd) management for the ze binary.",
	"support":   "Cross-system support bundle aggregator; archive orchestration is process-global.",
	"skills":    "Agent skill inventory tied to the current binary and generated support files.",
	"generate":  "Offline crypto artifact generation; no narrower PKI command owner exists yet.",
	"remote":    "Remote device management over SSH; no narrower runtime owner.",
	"crashes":   "Reads crash files written by the process panic handler.",
	"doctor":    "Process readiness aggregator; owner-specific checks register with the doctor registry.",
	"explain":   "Diagnostic-code lookup tied to the process binary.",
	"host":      "Offline hardware inventory for the box.",
	"pipe":      "Offline carrier for the pipe-operator language over stdin (format/filter/display); applies generic text transforms without a runtime component owner.",
	"--plugins": "Process-global flag that dumps the linked plugin inventory.",
}

type finding struct {
	Kind string `json:"kind"`
	File string `json:"file"`
	Msg  string `json:"message"`
}

func main() {
	jsonOut := false
	for _, a := range os.Args[1:] {
		if a == "--json" {
			jsonOut = true
		}
	}

	var findings []finding
	findings = append(findings, checkOwnersAreCmdZeFree()...)
	findings = append(findings, checkRootHandlersAreInternal()...)
	findings = append(findings, checkNoOwnerAllowlist()...)
	findings = append(findings, checkNoOwnerAllowlistHasNoOwnerHandlers()...)
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(findings)
	} else {
		for _, f := range findings {
			fmt.Fprintf(os.Stdout, "  [%s] %s: %s\n", f.Kind, f.File, f.Msg) //nolint:errcheck // output
		}
	}
	if len(findings) > 0 {
		fmt.Fprintf(os.Stdout, "\ncommand-ownership: FAILED, %d problem(s)\n", len(findings)) //nolint:errcheck // output
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, "command-ownership: OK (owners cmd/ze-free, root handlers internal, no-owner roots allowlisted)") //nolint:errcheck // output
}

// ownerCommandDirs returns every internal command-owner package directory
// (a `cli` or `client` directory under internal/ that contains a register.go).
func ownerCommandDirs() []string {
	var dirs []string
	_ = filepath.Walk("internal", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Base(path) != "register.go" {
			return nil
		}
		switch filepath.Base(filepath.Dir(path)) {
		case "cli", "client":
			dirs = append(dirs, filepath.Dir(path))
		}
		return nil
	})
	sort.Strings(dirs)
	return dirs
}

// checkOwnersAreCmdZeFree verifies no owner command package imports cmd/ze.
func checkOwnersAreCmdZeFree() []finding {
	var out []finding
	for _, dir := range ownerCommandDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			for _, imp := range fileImports(path) {
				if strings.Contains(imp, "/cmd/ze/") || strings.HasSuffix(imp, "/cmd/ze") {
					out = append(out, finding{
						Kind: "owner-imports-cmd-ze",
						File: path,
						Msg:  "owner command package must not import " + imp + " (move the dependency to internal/core or a leaf package)",
					})
				}
			}
		}
	}
	return out
}

// checkRootHandlersAreInternal verifies RegisterRootHandler is only called from
// internal/ packages, never from cmd/ze -- except for commands in the no-owner
// allowlist, which legitimately register handlers centrally.
func checkRootHandlersAreInternal() []finding {
	var out []finding
	_ = filepath.Walk("cmd/ze", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if hasVariantBuildTag(path) {
			return nil
		}
		forEachRegistryCall(path, func(method string, call *ast.CallExpr) {
			if method != "RegisterRootHandler" && method != "MustRegisterRootHandler" {
				return
			}
			if len(call.Args) > 0 {
				if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if name, err := strconv.Unquote(lit.Value); err == nil {
						if _, allowed := noOwnerAllowlist[name]; allowed {
							return
						}
					}
				}
			}
			out = append(out, finding{
				Kind: "root-handler-in-cmd-ze",
				File: path,
				Msg:  "owner-backed RegisterRootHandler must live in the owner's internal package, not cmd/ze",
			})
		})
		return nil
	})
	return out
}

// checkNoOwnerAllowlist verifies every metadata-only root registered from cmd/ze
// (registry.RegisterRoot) is in the no-owner allowlist.
func checkNoOwnerAllowlist() []finding {
	var out []finding
	_ = filepath.Walk("cmd/ze", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		for _, name := range registerRootNames(path) {
			if _, ok := noOwnerAllowlist[name]; !ok {
				out = append(out, finding{
					Kind: "root-not-allowlisted",
					File: path,
					Msg:  "central root " + strconv.Quote(name) + " has no owner and is not in noOwnerAllowlist; migrate it to its owner or add it with a reason",
				})
			}
		}
		return nil
	})
	return out
}

// checkNoOwnerAllowlistHasNoOwnerHandlers verifies the central no-owner
// allowlist does not name roots that already have an internal owner handler.
func checkNoOwnerAllowlistHasNoOwnerHandlers() []finding {
	handlers := internalRootHandlerNames()
	var names []string
	for name := range noOwnerAllowlist {
		if _, ok := handlers[name]; ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	var out []finding
	for _, name := range names {
		out = append(out, finding{
			Kind: "allowlisted-owned-root",
			File: handlers[name],
			Msg:  "root " + strconv.Quote(name) + " is owner-registered but still listed in noOwnerAllowlist; remove the no-owner allowlist entry",
		})
	}
	return out
}

func internalRootHandlerNames() map[string]string {
	handlers := make(map[string]string)
	_ = filepath.Walk("internal", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		forEachRegistryCall(path, func(method string, call *ast.CallExpr) {
			if method != "RegisterRootHandler" && method != "MustRegisterRootHandler" {
				return
			}
			if len(call.Args) == 0 {
				return
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return
			}
			name, err := strconv.Unquote(lit.Value)
			if err != nil {
				return
			}
			if _, exists := handlers[name]; !exists {
				handlers[name] = path
			}
		})
		return nil
	})
	return handlers
}

func fileImports(path string) []string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil
	}
	var imps []string
	for _, i := range file.Imports {
		if v, err := strconv.Unquote(i.Path.Value); err == nil {
			imps = append(imps, v)
		}
	}
	return imps
}

// registerRootNames returns the string literal first arg of every
// registry.RegisterRoot(name, ...) call (metadata-only no-owner roots).
func registerRootNames(path string) []string {
	var names []string
	forEachRegistryCall(path, func(method string, call *ast.CallExpr) {
		if method != "RegisterRoot" || len(call.Args) == 0 {
			return
		}
		if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if v, err := strconv.Unquote(lit.Value); err == nil {
				names = append(names, v)
			}
		}
	})
	return names
}

// hasVariantBuildTag reports whether a Go file starts with a build tag
// for a binary variant (ze_test, ze_chaos) that is exempt from the
// ownership check because its handlers are test/tool infrastructure.
func hasVariantBuildTag(path string) bool {
	f, err := os.Open(path) //nolint:gosec // path from filepath.Walk under cmd/ze
	if err != nil {
		return false
	}
	defer f.Close() //nolint:errcheck // read-only
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	header := string(buf[:n])
	return strings.Contains(header, "//go:build ze_test") ||
		strings.Contains(header, "//go:build ze_chaos")
}

// forEachRegistryCall invokes fn for every call whose selector package is
// `registry` (or `cmdregistry` as an import alias).
func forEachRegistryCall(path string, fn func(method string, call *ast.CallExpr)) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || (pkg.Name != "registry" && pkg.Name != "cmdregistry") {
			return true
		}
		fn(sel.Sel.Name, call)
		return true
	})
}
