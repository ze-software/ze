// Design: docs/architecture/cli/command-namespacing.md -- the command surface a call site is checked against
//
// resolver.go builds the thing every emitter is judged by: a Dispatcher
// carrying every registered command path with its YANG-declared ArgDefs, so an
// inline-selector form matches exactly as it does at runtime.
//
// Resolution is delegated to the REAL matcher. There is deliberately no second
// copy of matchCommandTokens here -- a checker that reimplemented
// inline-selector matching would drift from the dispatcher and start lying.

package cidispatch

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	// Blank import triggers every init() registration, so the command surface
	// matches the runtime one exactly.
	_ "github.com/ze-software/ze/internal/component/plugin/all"

	"github.com/ze-software/ze/internal/component/command"
	yangloader "github.com/ze-software/ze/internal/component/config/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// declRoots are the trees searched for plugin CommandDecl literals.
var declRoots = []string{"internal", "pkg"}

// Surface is the command set an emitter is checked against: the dispatcher that
// answers whether a string routes, and the sorted keys a prefix is matched
// against.
type Surface struct {
	dispatcher *pluginserver.Dispatcher
	keys       []string
}

// Keys answers every registered command path, lowercased and sorted.
func (s Surface) Keys() []string { return s.keys }

// newSurface builds the command surface from the linked registry plus the
// plugin command declarations found under tree.
func newSurface(tree string) (Surface, error) {
	loader, err := commandLoader(tree)
	if err != nil {
		return Surface{}, err
	}
	wireToPaths := yangloader.WireMethodToPaths(loader)
	pathToArgDefs := yangloader.PathToArgDefs(loader)

	dispatcher := pluginserver.NewDispatcher()
	seen := make(map[string]bool)
	register := func(path string, defs []command.ArgDef) {
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		dispatcher.RegisterWithOptions(path, nil, "", pluginserver.RegisterOptions{ArgDefs: defs})
	}

	for _, paths := range wireToPaths {
		for _, path := range paths {
			register(path, pathToArgDefs[path])
		}
	}
	// Streaming handlers ("monitor event") and the TUI-only dashboard
	// ("monitor bgp") are dispatchable but are not builtin RPCs.
	for _, prefix := range pluginserver.StreamingPrefixes() {
		register(prefix, pathToArgDefs[prefix])
	}
	register("monitor bgp", pathToArgDefs["monitor bgp"])

	// The SSH exec middleware intercepts these THREE before the dispatcher ever
	// sees them and routes them to shutdownFunc/restartFunc/rebootFunc
	// (internal/component/ssh/ssh.go, execMiddleware). Its own comment records
	// that the dispatcher deliberately registers no key for them, so a gate
	// that only knows the dispatcher would call `ze signal stop` dead and push
	// someone into "fixing" a command that works. They are a real surface, not
	// an exemption.
	for _, lifecycle := range []string{"stop", "restart", "reboot"} {
		register(lifecycle, nil)
	}

	// Plugin commands are declared in sdk.CommandDecl literals and only reach a
	// registry when a plugin completes stage 1, so they cannot be read from a
	// registry at check time. Parsing the literals is the only static source; a
	// name that is not a literal is a hard error rather than a skip, because an
	// unread declaration would turn every legitimate use of that command into a
	// false "dead" finding.
	decls, unreadable, err := pluginCommandDecls(tree)
	if err != nil {
		return Surface{}, err
	}
	for _, name := range decls {
		register(name, nil)
	}
	if len(unreadable) > 0 {
		return Surface{}, fmt.Errorf("plugin CommandDecl names not statically readable: %s",
			textbuf.Join(unreadable, "; "))
	}

	if len(seen) == 0 {
		return Surface{}, fmt.Errorf("no commands registered -- refusing to pass every emitter vacuously")
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, strings.ToLower(key))
	}
	sort.Strings(keys)
	return Surface{dispatcher: dispatcher, keys: keys}, nil
}

func commandLoader(tree string) (*yangloader.Loader, error) {
	loader := yangloader.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		return nil, fmt.Errorf("load embedded YANG: %w", err)
	}
	root := filepath.Join(tree, "internal")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".yang") ||
			entry.Name() == "ze-extensions.yang" || entry.Name() == "ze-types.yang" {
			return nil
		}
		if err := loader.AddModuleFromFile(path); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load command YANG from %s: %w", root, err)
	}
	if err := loader.Resolve(); err != nil {
		return nil, fmt.Errorf("resolve command YANG: %w", err)
	}
	return loader, nil
}

// Resolves reports whether the dispatcher could route cmd.
//
// It asks the REAL dispatcher through already-exported API only: every command
// is registered with a nil handler, so a matched command returns StatusDone and
// an unmatched one returns ErrUnknownCommand. Any OTHER error -- "requires a
// selector", an argument-validation complaint -- means the command EXISTS and
// this checker simply did not supply realistic arguments, which is not what the
// gate is asking about.
func (s Surface) Resolves(cmd string) bool {
	_, err := s.dispatcher.Dispatch(nil, cmd)
	return !errors.Is(err, pluginserver.ErrUnknownCommand)
}

// prefixKnown reports whether prefix is the start of some registered command,
// or is itself a resolvable command with trailing arguments.
func (s Surface) prefixKnown(prefix string) bool {
	if prefix == "" {
		return false
	}
	if s.Resolves(prefix) {
		return true
	}
	lower := strings.ToLower(prefix)
	for _, key := range s.keys {
		if strings.HasPrefix(key, lower) {
			return true
		}
	}
	return false
}

// pluginCommandDecls parses `sdk.CommandDecl{Name: "..."}` literals out of
// tree. It answers the names, plus the sites whose Name is not a string
// literal.
//
// Every read and parse failure is an ERROR. The script this replaces returned
// silently from each of them, and an unread declaration turns every legitimate
// use of that command into a false "dead" finding.
func pluginCommandDecls(tree string) (names, unreadable []string, err error) {
	fset := token.NewFileSet()

	// Several plugins name their commands with package constants
	// (`{Name: cmdShowVRRP}`), so a literal-only reader would report them as
	// unreadable and fail the gate on a perfectly good pattern. Collect every
	// package-level string constant first, keyed by package directory, and
	// resolve identifier values against it.
	constsByDir, err := stringConstsByDir(tree, fset)
	if err != nil {
		return nil, nil, err
	}

	for _, root := range declRoots {
		walkErr := filepath.WalkDir(filepath.Join(tree, root), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, readErr := os.ReadFile(path) //nolint:gosec // repository path
			if readErr != nil {
				return readErr
			}
			if !strings.Contains(string(src), "CommandDecl") {
				return nil
			}
			file, parseErr := parser.ParseFile(fset, path, src, 0)
			if parseErr != nil {
				return fmt.Errorf("parse %s: %w", path, parseErr)
			}

			found, unread := declNamesIn(fset, file, constsByDir[filepath.Dir(path)])
			names = append(names, found...)
			unreadable = append(unreadable, unread...)
			return nil
		})
		if walkErr != nil {
			return nil, nil, walkErr
		}
	}
	return names, unreadable, nil
}

// declNamesIn answers the command names one file declares, plus the sites whose
// Name is neither a literal nor a known package constant.
func declNamesIn(fset *token.FileSet, file *ast.File, consts map[string]string) (names, unreadable []string) {
	ast.Inspect(file, func(node ast.Node) bool {
		lit, ok := node.(*ast.CompositeLit)
		if !ok || !isCommandDeclType(lit.Type) {
			return true
		}
		// In `[]sdk.CommandDecl{{Name: "..."}, ...}` the ELEMENT literals carry
		// no type of their own (Go elides it), so the KeyValue pairs live one
		// level down. Flatten both shapes: the slice form and a bare
		// `sdk.CommandDecl{Name: "..."}`.
		fields := make([]ast.Expr, 0, len(lit.Elts))
		for _, element := range lit.Elts {
			if inner, isLit := element.(*ast.CompositeLit); isLit {
				fields = append(fields, inner.Elts...)
				continue
			}
			fields = append(fields, element)
		}

		for _, element := range fields {
			pair, isPair := element.(*ast.KeyValueExpr)
			if !isPair {
				continue
			}
			if key, isIdent := pair.Key.(*ast.Ident); !isIdent || key.Name != "Name" {
				continue
			}
			if name, isLit := stringLit(pair.Value); isLit {
				names = append(names, name)
				continue
			}
			if ident, isIdent := pair.Value.(*ast.Ident); isIdent {
				if value, known := consts[ident.Name]; known {
					names = append(names, value)
					continue
				}
			}
			position := fset.Position(pair.Value.Pos())
			var tb textbuf.Buffer
			unreadable = append(unreadable, tb.Str(position.Filename).Byte(':').Int(int64(position.Line)).String())
		}
		return true
	})
	return names, unreadable
}

// stringConstsByDir collects package-level const and var string literals keyed
// by package directory, so a CommandDecl naming its command with a constant
// resolves instead of failing the gate.
func stringConstsByDir(tree string, fset *token.FileSet) (map[string]map[string]string, error) {
	out := make(map[string]map[string]string)
	for _, root := range declRoots {
		err := filepath.WalkDir(filepath.Join(tree, root), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			src, readErr := os.ReadFile(path) //nolint:gosec // repository path
			if readErr != nil {
				return readErr
			}
			file, parseErr := parser.ParseFile(fset, path, src, 0)
			if parseErr != nil {
				return fmt.Errorf("parse %s: %w", path, parseErr)
			}

			dir := filepath.Dir(path)
			for _, decl := range file.Decls {
				general, isGeneral := decl.(*ast.GenDecl)
				if !isGeneral || (general.Tok != token.CONST && general.Tok != token.VAR) {
					continue
				}
				for _, spec := range general.Specs {
					values, isValues := spec.(*ast.ValueSpec)
					if !isValues {
						continue
					}
					for i, ident := range values.Names {
						if i >= len(values.Values) {
							continue
						}
						lit, isLit := stringLit(values.Values[i])
						if !isLit {
							continue
						}
						if out[dir] == nil {
							out[dir] = make(map[string]string)
						}
						out[dir][ident.Name] = lit
					}
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// isCommandDeclType reports whether a composite literal's type is
// sdk.CommandDecl, CommandDecl, or a slice of either.
func isCommandDeclType(expr ast.Expr) bool {
	switch typed := expr.(type) {
	case *ast.SelectorExpr:
		return typed.Sel.Name == "CommandDecl"
	case *ast.Ident:
		return typed.Name == "CommandDecl"
	case *ast.ArrayType:
		return isCommandDeclType(typed.Elt)
	}
	return false
}

// stringLit answers the value of a string-literal expression.
func stringLit(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}
