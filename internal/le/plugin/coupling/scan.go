// Design: docs/architecture/plugin/feature-gates.md -- the coupling check
// Related: coupling.go -- the judgement over what this file finds
//
// scan.go reads the Go source of the tree and records every use of something
// one package produces and another consumes: a typed event-bus handle emitted
// or subscribed, a Register* call into a package a plugin owns, and a
// Subscribe* call into one. It parses each file, so an import alias is resolved
// per file and never guessed from the identifier a line happens to use.

package coupling

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// module is the import path prefix of every package this check reads.
const module = "github.com/ze-software/ze"

// eventsImport is the package whose Register and RegisterSignal declare a
// typed event-bus handle.
const eventsImport = module + "/internal/core/events"

// scanRoots are the trees a shipped binary is built from. internal/le and
// internal/test are skipped below: le is tooling, and the test plugins under
// internal/test register only in test binaries.
var scanRoots = []string{"cmd", "internal", "pkg"}

// skippedDirs are directories whose code no shipped binary carries.
var skippedDirs = []string{"internal/le", "internal/test", "vendor"}

// Kinds of coupling, as the report names them.
const (
	// KindEvent is a typed event-bus handle: the consumer subscribes to what
	// only the producer emits.
	KindEvent = "event"
	// KindRegistration is a Register* call: the producer fills a registry the
	// consumer's package owns, under one key.
	KindRegistration = "registration"
	// KindSubscription is a Subscribe* call: the consumer asks the producer's
	// package for a feed.
	KindSubscription = "subscription"
	// KindSeam is a Set*/Get* pair in shared code: the consumer reads what
	// only the producer installed.
	KindSeam = "seam"
)

// use is one place a consumer takes an item. dir is the consumer's package
// directory, which for a registration is the registry's package rather than
// the caller's, because the registry's owner is the one that consumes the key.
type use struct {
	key   string
	label string
	kind  string
	dir   string
	site  string
}

// production is one place an item is produced, by the package in dir.
type production struct {
	dir  string
	site string
}

// sourceFile is one parsed Go file and the repository-relative directory it
// sits in.
type sourceFile struct {
	dir  string
	path string
	file *ast.File
}

// scanned is every use and production the tree holds.
type scanned struct {
	uses        []use
	productions map[string][]production
	files       int
}

// parseTree parses every shipped, non-test Go file under the scan roots.
func parseTree(root string) ([]sourceFile, *token.FileSet, error) {
	fset := token.NewFileSet()
	var files []sourceFile
	for _, top := range scanRoots {
		// A scan root the tree does not hold has no source to read; rootExists
		// already refused a tree with no internal/ at all.
		if _, statErr := os.Stat(filepath.Join(root, top)); errors.Is(statErr, fs.ErrNotExist) {
			continue
		}
		walkErr := filepath.WalkDir(filepath.Join(root, top), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)
			if entry.IsDir() {
				if skipped(rel) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(rel, ".go") {
				return nil
			}
			if strings.HasSuffix(rel, "_test.go") {
				return nil
			}
			file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if parseErr != nil {
				return parseErr
			}
			files = append(files, sourceFile{dir: filepath.ToSlash(filepath.Dir(rel)), path: rel, file: file})
			return nil
		})
		if walkErr != nil {
			return nil, nil, walkErr
		}
	}
	return files, fset, nil
}

// skipped reports whether a directory holds no shipped code.
func skipped(rel string) bool {
	if filepath.Base(rel) == "testdata" {
		return true
	}
	for _, dir := range skippedDirs {
		if rel == dir {
			return true
		}
		if strings.HasPrefix(rel, dir+"/") {
			return true
		}
	}
	return false
}

// scanTree records every use and production over the parsed files. owned
// answers whether a package directory belongs to a plugin: a Register* or
// Subscribe* call couples two units only when the called package is one.
func scanTree(files []sourceFile, fset *token.FileSet, owned func(dir string) bool) scanned {
	names := packageNames(files)
	handles := declaredHandles(files, names)
	result := scanned{productions: map[string][]production{}, files: len(files)}

	for i := range files {
		source := &files[i]
		aliases := importAliases(source.file, names)
		ast.Inspect(source.file, func(node ast.Node) bool {
			call, isCall := node.(*ast.CallExpr)
			if !isCall {
				return true
			}
			method, isSelector := call.Fun.(*ast.SelectorExpr)
			if !isSelector {
				return true
			}
			site := source.path + ":" + strconv.Itoa(fset.Position(call.Pos()).Line)
			recordHandleCall(&result, handles, aliases, source.dir, method, site)
			recordPackageCall(&result, aliases, owned, source.dir, call, method, site)
			return true
		})
	}
	return result
}

// recordHandleCall records `alias.Handle.Emit(...)`, `alias.Handle.Subscribe(...)`
// and the unqualified `Handle.Emit(...)` inside the handle's own package.
func recordHandleCall(result *scanned, handles map[string]bool, aliases map[string]string, dir string, method *ast.SelectorExpr, site string) {
	verb := method.Sel.Name
	if verb != "Emit" && verb != "Subscribe" {
		return
	}
	handleDir, handle, label, found := handleOf(method.X, aliases, dir)
	if !found {
		return
	}
	key := handleDir + "." + handle
	if !handles[key] {
		return
	}
	if verb == "Emit" {
		result.productions[key] = append(result.productions[key], production{dir: dir, site: site})
		return
	}
	result.uses = append(result.uses, use{key: key, label: label, kind: KindEvent, dir: dir, site: site})
}

// handleOf answers the package directory and name of the handle an expression
// names, and the label the consumer's source spells it with.
func handleOf(expr ast.Expr, aliases map[string]string, dir string) (handleDir, handle, label string, found bool) {
	switch x := expr.(type) {
	case *ast.Ident:
		return dir, x.Name, x.Name, true
	case *ast.SelectorExpr:
		pkg, isIdent := x.X.(*ast.Ident)
		if !isIdent {
			return "", "", "", false
		}
		target, imported := aliases[pkg.Name]
		if !imported {
			return "", "", "", false
		}
		return target, x.Sel.Name, pkg.Name + "." + x.Sel.Name, true
	}
	// Any other receiver (a local variable, a field, a call result) is not
	// resolved statically; documented as a limit of the check.
	return "", "", "", false
}

// recordPackageCall records `alias.Register*(key, ...)` and `alias.Subscribe*(...)`
// into a package a plugin owns.
func recordPackageCall(result *scanned, aliases map[string]string, owned func(string) bool, dir string, call *ast.CallExpr, method *ast.SelectorExpr, site string) {
	pkg, isIdent := method.X.(*ast.Ident)
	if !isIdent {
		return
	}
	target, imported := aliases[pkg.Name]
	if !imported {
		return
	}
	if target == dir {
		return
	}
	function := method.Sel.Name
	label := pkg.Name + "." + function
	if !owned(target) {
		recordSeamCall(result, target, function, label, dir, site)
		return
	}
	switch {
	case strings.HasPrefix(function, "Register"), strings.HasPrefix(function, "MustRegister"):
		key := target + "." + function + "(" + registrationKey(call) + ")"
		result.productions[key] = append(result.productions[key], production{dir: dir, site: site})
		result.uses = append(result.uses, use{key: key, label: label + "(" + registrationKey(call) + ")", kind: KindRegistration, dir: target, site: site})
	case strings.HasPrefix(function, "Subscribe"):
		key := target + "." + function
		result.productions[key] = append(result.productions[key], production{dir: target, site: site})
		result.uses = append(result.uses, use{key: key, label: label, kind: KindSubscription, dir: dir, site: site})
	}
}

// recordSeamCall records `alias.Set<X>(...)` and `alias.Get<X>(...)` into a
// package no plugin owns: a seam that shared code declares, one plugin fills
// and another reads, such as registry.SetRIBDumpCallback beside
// registry.GetRIBDumpCallback. The setter's caller produces X, the getter's
// caller consumes it.
func recordSeamCall(result *scanned, target, function, label, dir, site string) {
	seam, isSetter := strings.CutPrefix(function, "Set")
	if isSetter && seam != "" {
		key := target + "." + seam
		result.productions[key] = append(result.productions[key], production{dir: dir, site: site})
		return
	}
	seam, isGetter := strings.CutPrefix(function, "Get")
	if isGetter && seam != "" {
		result.uses = append(result.uses, use{key: target + "." + seam, label: label, kind: KindSeam, dir: dir, site: site})
	}
}

// registrationKey answers the first argument of a Register* call as the
// source spells it: a string literal's value, or the expression text. Two
// registrations under one key fill the same slot.
func registrationKey(call *ast.CallExpr) string {
	if len(call.Args) == 0 {
		return ""
	}
	switch arg := call.Args[0].(type) {
	case *ast.BasicLit:
		value, err := strconv.Unquote(arg.Value)
		if err != nil {
			return arg.Value
		}
		return value
	case *ast.Ident:
		return arg.Name
	case *ast.SelectorExpr:
		if pkg, isIdent := arg.X.(*ast.Ident); isIdent {
			return pkg.Name + "." + arg.Sel.Name
		}
	}
	return "..."
}

// packageNames answers each scanned directory's import path and its package
// name, so an import without an alias binds the name the package declares.
func packageNames(files []sourceFile) map[string]string {
	names := map[string]string{}
	for i := range files {
		names[module+"/"+files[i].dir] = files[i].file.Name.Name
	}
	return names
}

// importAliases answers, for one file, each local package identifier and the
// repository-relative directory it names. A blank or dot import binds no
// identifier, and an import outside the module couples nothing this check
// judges.
func importAliases(file *ast.File, names map[string]string) map[string]string {
	aliases := map[string]string{}
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(path, module+"/") {
			continue
		}
		local := names[path]
		if local == "" {
			local = path[strings.LastIndex(path, "/")+1:]
		}
		if imported.Name != nil {
			local = imported.Name.Name
		}
		if local == "_" {
			continue
		}
		if local == "." {
			continue
		}
		aliases[local] = strings.TrimPrefix(path, module+"/")
	}
	return aliases
}

// declaredHandles answers every package-level variable bound to
// events.Register[...] or events.RegisterSignal, keyed "<dir>.<name>".
func declaredHandles(files []sourceFile, names map[string]string) map[string]bool {
	handles := map[string]bool{}
	for i := range files {
		aliases := importAliases(files[i].file, names)
		eventsAlias := ""
		for local, dir := range aliases {
			if module+"/"+dir == eventsImport {
				eventsAlias = local
			}
		}
		if eventsAlias == "" {
			continue
		}
		for _, decl := range files[i].file.Decls {
			general, isGeneral := decl.(*ast.GenDecl)
			if !isGeneral {
				continue
			}
			if general.Tok != token.VAR {
				continue
			}
			for _, spec := range general.Specs {
				value, isValue := spec.(*ast.ValueSpec)
				if !isValue {
					continue
				}
				for n, name := range value.Names {
					if n >= len(value.Values) {
						break
					}
					if registersHandle(value.Values[n], eventsAlias) {
						handles[files[i].dir+"."+name.Name] = true
					}
				}
			}
		}
	}
	return handles
}

// registersHandle reports whether an expression is a call to
// events.Register[T](...) or events.RegisterSignal(...).
func registersHandle(expr ast.Expr, eventsAlias string) bool {
	call, isCall := expr.(*ast.CallExpr)
	if !isCall {
		return false
	}
	fun := call.Fun
	switch indexed := fun.(type) {
	case *ast.IndexExpr:
		fun = indexed.X
	case *ast.IndexListExpr:
		fun = indexed.X
	}
	selector, isSelector := fun.(*ast.SelectorExpr)
	if !isSelector {
		return false
	}
	pkg, isIdent := selector.X.(*ast.Ident)
	if !isIdent {
		return false
	}
	if pkg.Name != eventsAlias {
		return false
	}
	return slices.Contains([]string{"Register", "RegisterSignal"}, selector.Sel.Name)
}

// rootExists reports whether root holds the scan roots, so a wrong root is an
// error rather than a tree with no coupling in it.
func rootExists(root string) error {
	_, err := os.Stat(filepath.Join(root, "internal"))
	return err
}
