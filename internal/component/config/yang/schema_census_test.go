// Design: docs/architecture/config/yang-config-design.md -- schema resolution in test binaries.
// Related: loader_registered_test.go -- DefaultLoader refuses a module whose import nobody registered.

package yang_test

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// modulePath is the Go module this repository holds; a schema producer's
// directory is its import path with this prefix removed.
const modulePath = "github.com/ze-software/ze"

// schemaProducers are the packages that answer the resolved schema. The census
// derives which of their functions resolve it from their own source.
var schemaProducers = []string{
	modulePath + "/internal/component/config/yang",
	modulePath + "/internal/component/config",
}

// schemaResolveRoot is the function every schema answer comes from: it loads
// the embedded and every registered module and joins their errors, so a module
// a linked module imports but nobody registered is an error here.
const schemaResolveRoot = modulePath + "/internal/component/config/yang.resolveDefault"

// TestEverySchemaResolvingCallerChecksTheResolveError is the census of the
// binaries, test binaries included, that resolve the YANG schema. A binary
// links only the modules its imports register, so the resolve error is the one
// place a missing module is named; a caller that discards it reads a nil
// schema and fails somewhere else, or not at all.
//
// VALIDATES: the functions that answer the schema are derived from the
// producers' source: every package-level function of the two schema packages
// that answers a *Resolved or *Schema with an error and reaches resolveDefault
// through calls, methods or function values. Every call to one of them
// anywhere in the tree (vendor, testdata, dot directories, and the root tmp
// and bin excepted) keeps its error, unless the line carries a reviewed
// `//nolint:errcheck // <reason>`.
// PREVENTS: a schema-dependent test binary that lacks a module's registration
// passing on a nil schema, the silent shape behind the config/cli failures
// that read "no such module: ze-bgp-conf".
//
// MUTATION: (internal/component/plugin/server/all_import_test.go, `loader, err := yang.DefaultLoader()`, `loader, _ := yang.DefaultLoader(); var err error`): the census goes red and names that file and line.
// MUTATION: (internal/component/config/yang_schema.go, `resolved, err := yang.DefaultLoader()`, `resolved, err := yang.NewLoader().Resolve()`): the derivation loses config.YANGSchema and the census refuses before it walks.
func TestEverySchemaResolvingCallerChecksTheResolveError(t *testing.T) {
	root := moduleRoot(t)
	resolvers := schemaResolvers(t, root)
	// Two resolvers every reader knows, as a guard on the derivation: a
	// derivation that lost them would census nothing and pass.
	for _, want := range []string{schemaProducers[0] + ".DefaultLoader", schemaProducers[1] + ".YANGSchema"} {
		if !resolvers[want] {
			t.Fatalf("the derivation lost %s, so the census would check nothing it should: derived %v",
				want, slices.Sorted(maps.Keys(resolvers)))
		}
	}

	var findings []string
	packages := map[string]bool{}
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if skipCensusDir(root, file, entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(file, ".go") {
			return nil
		}
		found, calls, fileErr := censusFile(root, file, resolvers)
		if fileErr != nil {
			return fileErr
		}
		findings = append(findings, found...)
		if calls > 0 {
			packages[relSlash(root, filepath.Dir(file))] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	// The package whose schema-dependent cases failed on a missing module must
	// be in the population, or the walk did not reach the callers it exists for.
	if !packages["internal/component/config/cli"] {
		t.Fatalf("the census found no schema-resolving call in internal/component/config/cli: the walk or the derivation is broken (%d packages found)", len(packages))
	}
	t.Logf("%d packages resolve the schema through %d derived functions", len(packages), len(resolvers))

	if len(findings) > 0 {
		slices.Sort(findings)
		t.Fatalf("these calls discard the schema resolve error, so a binary that does not link a module its schema imports reads a nil schema instead of failing on the missing module:\n%s\n"+
			"Check the error (`if err != nil { t.Fatal(err) }`), or mark a reviewed discard `//nolint:errcheck // <reason>`.",
			strings.Join(findings, "\n"))
	}
}

// moduleRoot walks up from the package directory to the directory holding
// go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the package directory")
		}
		dir = parent
	}
}

// relSlash answers target relative to root, slash-separated. Every target
// comes from walking root, so it is always under it.
func relSlash(root, target string) string {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return filepath.ToSlash(target)
	}
	return filepath.ToSlash(rel)
}

// skipCensusDir names the trees that hold no source of this module's
// binaries: dot directories, vendored modules, fixtures, and the scratch and
// build trees at the root, which can hold other checkouts' copies.
func skipCensusDir(root, dir, name string) bool {
	if dir == root {
		return false
	}
	if strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata" || name == "node_modules" {
		return true
	}
	if filepath.Dir(dir) != root {
		return false
	}
	return name == "tmp" || name == "bin"
}

// producerNode is one package-level function or method of a schema producer:
// whether it answers the schema, and the census keys its body names.
type producerNode struct {
	function      bool
	answersSchema bool
	names         []string
}

// schemaResolvers derives the census keys (import path + "." + name) of the
// package-level functions that answer the resolved schema, from the
// producers' non-test source. A node reaches the root when its body names one
// that does, by a call, a method call or a function value; a resolver is a
// reaching function that answers a schema.
func schemaResolvers(t *testing.T, root string) map[string]bool {
	t.Helper()
	nodes := map[string]producerNode{}
	fset := token.NewFileSet()
	for _, producer := range schemaProducers {
		dir := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(producer, modulePath+"/")))
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read schema producer %s: %v", dir, err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}
			addProducerNodes(nodes, file, producer)
		}
	}

	if _, ok := nodes[schemaResolveRoot]; !ok {
		t.Fatalf("the schema producers no longer declare %s, so the census has no root to derive from", schemaResolveRoot)
	}
	reach := map[string]bool{schemaResolveRoot: true}
	// Bounded by the number of producer nodes: each pass adds one or stops.
	for changed := true; changed; {
		changed = false
		for key, node := range nodes {
			if reach[key] {
				continue
			}
			if slices.ContainsFunc(node.names, func(name string) bool { return reach[name] }) {
				reach[key] = true
				changed = true
			}
		}
	}
	resolvers := map[string]bool{}
	for key := range reach {
		if node := nodes[key]; node.function && node.answersSchema {
			resolvers[key] = true
		}
	}
	return resolvers
}

// addProducerNodes records every package-level function and method file
// declares. A method is keyed by name alone, because a call through a package
// variable carries no receiver type the parser can name; two methods of one
// name in a producer share a node, which can only widen the census.
func addProducerNodes(nodes map[string]producerNode, file *ast.File, producer string) {
	imports := fileImports(file)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		key := producer + "." + fn.Name.Name
		if fn.Recv != nil {
			key = producer + ".(method)." + fn.Name.Name
		}
		node := nodes[key]
		node.function = fn.Recv == nil
		node.answersSchema = answersSchema(fn)
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch expr := n.(type) {
			case *ast.Ident:
				node.names = append(node.names, producer+"."+expr.Name)
			case *ast.SelectorExpr:
				node.names = append(node.names, selectorKey(expr, producer, imports))
			}
			return true
		})
		nodes[key] = node
	}
}

// answersSchema reports whether fn returns a *Resolved or *Schema first and
// an error last: the shape whose error is the resolve error.
func answersSchema(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil {
		return false
	}
	var results []ast.Expr
	for _, field := range fn.Type.Results.List {
		for range max(1, len(field.Names)) {
			results = append(results, field.Type)
		}
	}
	if len(results) < 2 {
		return false
	}
	last, ok := results[len(results)-1].(*ast.Ident)
	if !ok || last.Name != "error" {
		return false
	}
	star, ok := results[0].(*ast.StarExpr)
	if !ok {
		return false
	}
	var name string
	switch target := star.X.(type) {
	case *ast.Ident:
		name = target.Name
	case *ast.SelectorExpr:
		name = target.Sel.Name
	default:
		return false
	}
	return name == "Resolved" || name == "Schema"
}

// fileImports maps each local import name to its path.
func fileImports(file *ast.File) map[string]string {
	imports := map[string]string{}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(importPath)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = importPath
	}
	return imports
}

// selectorKey answers the census key a selector names: pkg.Name through an
// import, and otherwise, inside a producer, a method of that producer.
func selectorKey(expr *ast.SelectorExpr, own string, imports map[string]string) string {
	if pkg, ok := expr.X.(*ast.Ident); ok {
		if importPath, imported := imports[pkg.Name]; imported {
			return importPath + "." + expr.Sel.Name
		}
	}
	if own == "" {
		return ""
	}
	return own + ".(method)." + expr.Sel.Name
}

// calleeKey answers the census key of the package-level function a call
// names: a bare name inside producer own (own is "" outside a producer), or
// pkg.Name through an import. Any other call answers "".
func calleeKey(call *ast.CallExpr, own string, imports map[string]string) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		if own == "" {
			return ""
		}
		return own + "." + fun.Name
	case *ast.SelectorExpr:
		pkg, ok := fun.X.(*ast.Ident)
		if !ok {
			return ""
		}
		importPath, imported := imports[pkg.Name]
		if !imported {
			return ""
		}
		return importPath + "." + fun.Sel.Name
	}
	return ""
}

// censusFile reports each call in file that discards a resolver's error, and
// how many resolver calls the file makes.
func censusFile(root, file string, resolvers map[string]bool) (findings []string, calls int, err error) {
	source, err := os.ReadFile(file) //nolint:gosec // file comes from walking the module root
	if err != nil {
		return nil, 0, err
	}
	if !mentionsResolver(source, resolvers) {
		return nil, 0, nil
	}
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, source, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, 0, err
	}

	own := ""
	dir := relSlash(root, filepath.Dir(file))
	for _, producer := range schemaProducers {
		if dir == strings.TrimPrefix(producer, modulePath+"/") && parsed.Name.Name == path.Base(producer) {
			own = producer
		}
	}
	imports := fileImports(parsed)
	reviewed := map[int]bool{}
	for _, group := range parsed.Comments {
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, "//nolint:errcheck //") {
				reviewed[fset.Position(comment.Pos()).Line] = true
			}
		}
	}
	isResolver := func(expr ast.Expr) bool {
		call, ok := expr.(*ast.CallExpr)
		return ok && resolvers[calleeKey(call, own, imports)]
	}
	report := func(node ast.Node) {
		position := fset.Position(node.Pos())
		if reviewed[position.Line] {
			return
		}
		findings = append(findings, relSlash(root, position.Filename)+":"+strconv.Itoa(position.Line))
	}
	ast.Inspect(parsed, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if isResolver(node) {
				calls++
			}
		case *ast.AssignStmt:
			if len(node.Rhs) == 1 && isResolver(node.Rhs[0]) && isBlank(node.Lhs[len(node.Lhs)-1]) {
				report(node)
			}
		case *ast.ValueSpec:
			if len(node.Values) == 1 && isResolver(node.Values[0]) && isBlank(node.Names[len(node.Names)-1]) {
				report(node)
			}
		case *ast.ExprStmt:
			if isResolver(node.X) {
				report(node)
			}
		}
		return true
	})
	return findings, calls, nil
}

func isBlank(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "_"
}

// mentionsResolver is the cheap filter before a parse: a file that never
// spells a resolver's name followed by "(" cannot call it.
func mentionsResolver(source []byte, resolvers map[string]bool) bool {
	for key := range resolvers {
		name := key[strings.LastIndexByte(key, '.')+1:]
		if bytes.Contains(source, []byte(name+"(")) {
			return true
		}
	}
	return false
}
