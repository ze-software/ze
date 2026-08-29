// Design: docs/architecture/api/commands.md -- command surface ownership gate
//
// Package commandownership enforces the command-surface-ownership invariants:
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
// Every read the gate makes is CHECKED. A Go file it cannot parse, a directory
// it cannot list and a walk that cannot finish are each an error rather than an
// empty result, because the gate's whole answer is what it found, and a read
// that returned nothing looks exactly like a clean tree.

package commandownership

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
)

// name is the word this command is typed as. The retired Make target used the
// equivalent ze-command-ownership-check spelling.
const name = "command ownership"

// parseFloor is the smallest number of Go files under cmd/ze and internal/ that
// counts as having read a Ze checkout. A gate that walked a tree holding
// neither directory answered OK and exited 0, which is the failure this gate
// exists to prevent applied to itself, so the population is checked before the
// verdict is.
//
// It is a floor rather than a count: 5,700 files on 2026-08-26.
const parseFloor = 500

// noOwnerAllowlist is the set of root commands that may stay in cmd/ze because
// no internal component, plugin, or backend owns the behavior. Each entry
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

// scan is one run's state: the tree it reads, the findings it draws, and how
// many Go files it actually parsed. The count is what the floor reads, and it
// is kept here rather than returned from each check because every check
// contributes to one population.
type scan struct {
	tree   string
	parsed int
}

// Check reads tree and answers every ownership violation in it.
//
// floor is the smallest population that counts as having read the tree, and it
// is a parameter because a fixture is not a checkout: le passes parseFloor and
// a test naming a three-file tree passes 0.
//
// The error is about the READ rather than about the tree: a Go file that will
// not parse, a directory that will not list, a walk that cannot finish, or a
// population too small to be the tree the caller meant. Each one means the gate
// did not judge what it was asked to judge.
func Check(tree string, floor int) (Findings, error) {
	run := &scan{tree: tree}

	var findings Findings
	for _, check := range []func(*scan) (Findings, error){
		checkOwnersAreCmdZeFree,
		checkRootHandlersAreInternal,
		checkNoOwnerAllowlist,
		checkNoOwnerAllowlistHasNoOwnerHandlers,
	} {
		drawn, err := check(run)
		if err != nil {
			return nil, err
		}
		findings = append(findings, drawn...)
	}

	if run.parsed < floor {
		var tb textbuf.Buffer
		return nil, fmt.Errorf("%s", tb.Str("only ").Int(int64(run.parsed)).
			Str(" Go files parsed under cmd/ze and internal (floor ").Int(int64(floor)).
			Str("): this is not the tree the gate was asked to judge, so it judged almost nothing").String())
	}
	return findings, nil
}

// rel answers a path relative to the tree, with forward slashes, which is what
// a finding names and what the script printed when it ran from the root.
func (s *scan) rel(path string) (string, error) {
	relative, err := filepath.Rel(s.tree, path)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(relative), nil
}

// goFilesUnder walks one directory of the tree and calls visit for every
// non-test .go file in it, with the path and its tree-relative spelling.
//
// A directory the tree does not hold contributes no code, which is a fact about
// the tree rather than a read that failed; the floor is what keeps that from
// becoming a gate over nothing. Every other error stops the walk.
func (s *scan) goFilesUnder(dir string, visit func(path, rel string) error) error {
	root := filepath.Join(s.tree, dir)
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return nil //nolint:nilerr // a root the tree does not hold contributes no code; the floor is what refuses a tree that holds none of them
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == "testdata" {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relative, relErr := s.rel(path)
		if relErr != nil {
			return relErr
		}
		return visit(path, relative)
	})
}

// ownerCommandDirs returns every internal command-owner package directory
// (a `cli` or `client` directory under internal/ that contains a register.go).
func ownerCommandDirs(s *scan) ([]string, error) {
	var dirs []string
	root := filepath.Join(s.tree, "internal")
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return nil, nil //nolint:nilerr // a tree with no internal/ owns no command package; the floor refuses a tree holding neither root
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == "testdata" {
			return filepath.SkipDir
		}
		if entry.IsDir() || filepath.Base(path) != "register.go" {
			return nil
		}
		switch filepath.Base(filepath.Dir(path)) {
		case "cli", "client":
			dirs = append(dirs, filepath.Dir(path))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(dirs)
	return dirs, nil
}

// checkOwnersAreCmdZeFree verifies no owner command package imports cmd/ze.
func checkOwnersAreCmdZeFree(s *scan) (Findings, error) {
	dirs, err := ownerCommandDirs(s)
	if err != nil {
		return nil, err
	}

	var out Findings
	for _, dir := range dirs {
		entries, readErr := os.ReadDir(dir)
		if readErr != nil {
			return nil, readErr
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			relative, relErr := s.rel(path)
			if relErr != nil {
				return nil, relErr
			}
			imports, impErr := s.fileImports(path)
			if impErr != nil {
				return nil, impErr
			}
			for _, imported := range imports {
				if !strings.Contains(imported, "/cmd/ze/") && !strings.HasSuffix(imported, "/cmd/ze") {
					continue
				}
				var tb textbuf.Buffer
				out = append(out, Finding{
					Kind: "owner-imports-cmd-ze",
					File: relative,
					Msg: tb.Str("owner command package must not import ").Str(imported).
						Str(" (move the dependency to internal/core or a leaf package)").String(),
				})
			}
		}
	}
	return out, nil
}

// checkRootHandlersAreInternal verifies RegisterRootHandler is only called from
// internal/ packages, never from cmd/ze -- except for commands in the no-owner
// allowlist, which legitimately register handlers centrally.
func checkRootHandlersAreInternal(s *scan) (Findings, error) {
	var out Findings
	err := s.goFilesUnder("cmd/ze", func(path, relative string) error {
		variant, tagErr := hasVariantBuildTag(path)
		if tagErr != nil {
			return tagErr
		}
		if variant {
			return nil
		}
		return s.forEachRegistryCall(path, func(method string, call *ast.CallExpr) {
			if method != "RegisterRootHandler" && method != "MustRegisterRootHandler" {
				return
			}
			if len(call.Args) > 0 {
				if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if root, unquoteErr := strconv.Unquote(lit.Value); unquoteErr == nil {
						if _, allowed := noOwnerAllowlist[root]; allowed {
							return
						}
					}
				}
			}
			out = append(out, Finding{
				Kind: "root-handler-in-cmd-ze",
				File: relative,
				Msg:  "owner-backed RegisterRootHandler must live in the owner's internal package, not cmd/ze",
			})
		})
	})
	return out, err
}

// checkNoOwnerAllowlist verifies every metadata-only root registered from cmd/ze
// (registry.RegisterRoot) is in the no-owner allowlist.
func checkNoOwnerAllowlist(s *scan) (Findings, error) {
	var out Findings
	err := s.goFilesUnder("cmd/ze", func(path, relative string) error {
		roots, rootsErr := s.registerRootNames(path)
		if rootsErr != nil {
			return rootsErr
		}
		for _, root := range roots {
			if _, ok := noOwnerAllowlist[root]; ok {
				continue
			}
			var tb textbuf.Buffer
			out = append(out, Finding{
				Kind: "root-not-allowlisted",
				File: relative,
				Msg: tb.Str("central root ").Str(strconv.Quote(root)).
					Str(" has no owner and is not in noOwnerAllowlist; migrate it to its owner or add it with a reason").String(),
			})
		}
		return nil
	})
	return out, err
}

// checkNoOwnerAllowlistHasNoOwnerHandlers verifies the central no-owner
// allowlist does not name roots that already have an internal owner handler.
func checkNoOwnerAllowlistHasNoOwnerHandlers(s *scan) (Findings, error) {
	handlers, err := internalRootHandlerNames(s)
	if err != nil {
		return nil, err
	}

	var names []string
	for root := range noOwnerAllowlist {
		if _, ok := handlers[root]; ok {
			names = append(names, root)
		}
	}
	sort.Strings(names)

	var out Findings
	for _, root := range names {
		var tb textbuf.Buffer
		out = append(out, Finding{
			Kind: "allowlisted-owned-root",
			File: handlers[root],
			Msg: tb.Str("root ").Str(strconv.Quote(root)).
				Str(" is owner-registered but still listed in noOwnerAllowlist; remove the no-owner allowlist entry").String(),
		})
	}
	return out, nil
}

// internalRootHandlerNames maps every root an internal package registers a
// handler for to the file that registers it, first registration winning.
func internalRootHandlerNames(s *scan) (map[string]string, error) {
	handlers := make(map[string]string)
	err := s.goFilesUnder("internal", func(path, relative string) error {
		return s.forEachRegistryCall(path, func(method string, call *ast.CallExpr) {
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
			root, unquoteErr := strconv.Unquote(lit.Value)
			if unquoteErr != nil {
				return
			}
			if _, exists := handlers[root]; !exists {
				handlers[root] = relative
			}
		})
	})
	if err != nil {
		return nil, err
	}
	return handlers, nil
}

// fileImports answers every import path of one Go file. A file that will not
// parse is an error: the script returned an empty list there, so a syntactically
// broken owner package imported nothing and passed.
func (s *scan) fileImports(path string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	s.parsed++

	var imports []string
	for _, spec := range file.Imports {
		unquoted, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr != nil {
			return nil, unquoteErr
		}
		imports = append(imports, unquoted)
	}
	return imports, nil
}

// registerRootNames returns the string literal first arg of every
// registry.RegisterRoot(name, ...) call (metadata-only no-owner roots).
func (s *scan) registerRootNames(path string) ([]string, error) {
	var names []string
	err := s.forEachRegistryCall(path, func(method string, call *ast.CallExpr) {
		if method != "RegisterRoot" || len(call.Args) == 0 {
			return
		}
		if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if root, unquoteErr := strconv.Unquote(lit.Value); unquoteErr == nil {
				names = append(names, root)
			}
		}
	})
	return names, err
}

// hasVariantBuildTag reports whether a Go file starts with a build tag
// for a binary variant (ze_test, ze_chaos) that is exempt from the
// ownership check because its handlers are test/tool infrastructure.
func hasVariantBuildTag(path string) (bool, error) {
	file, err := os.Open(path) //nolint:gosec // the path comes from this tool's own walk
	if err != nil {
		return false, err
	}
	defer file.Close() //nolint:errcheck // read-only

	buf := make([]byte, 512)
	read, err := file.Read(buf)
	if err != nil && read == 0 {
		// A file too short to hold a build tag carries none. Any other read
		// failure is a file the gate could not judge.
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		return false, err
	}
	header := string(buf[:read])
	return strings.Contains(header, "//go:build ze_test") ||
		strings.Contains(header, "//go:build ze_chaos"), nil
}

// forEachRegistryCall invokes fn for every call whose selector package is
// `registry` (or `cmdregistry` as an import alias).
//
// A file that will not parse is an error. The script returned silently there,
// so a cmd/ze file with a syntax error registered no root at all and the gate
// reported OK over it.
func (s *scan) forEachRegistryCall(path string, fn func(method string, call *ast.CallExpr)) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return err
	}
	s.parsed++

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
	return nil
}

// Answer is the `le command ownership` command. The tree is the checkout, so
// the command takes no argument.
func Answer(args []string) (any, int) {
	if len(args) > 0 {
		return nil, leroot.RefuseArgument(name, args[0])
	}

	tree, err := lepath.Root()
	if err != nil {
		fmt.Fprintf(os.Stderr, "command-ownership: %v\n", err) //nolint:errcheck // CLI output
		return nil, 2
	}

	findings, err := Check(tree, parseFloor)
	if err != nil {
		// 2 rather than 1: a read that did not complete is a different fact
		// from a tree holding a violation.
		fmt.Fprintf(os.Stderr, "command-ownership: %v\n", err) //nolint:errcheck // CLI output
		return nil, 2
	}

	if len(findings) > 0 {
		return findings, 1
	}
	return findings, 0
}
