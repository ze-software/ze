// Design: docs/architecture/plugin/plugin-system.md -- registration closes when main starts
// Related: report.go -- what the check answers
// Related: actions.go -- the `le plugin registration-sites` command surface
//
// Package pluginregistrationsites judges where Ze's own code calls a
// registering function of the plugin registry. The registry seals when main
// starts (registry.Seal), so a registration is only safe while packages
// initialize. A call that sits anywhere a later goroutine can reach, or a
// registering function handed out as a value, is a registration the seal
// refuses at run time and the declared walk of the inventory cannot see.
//
// The registering functions are not listed here: they are the exported
// functions of the registry package that reach the seal guard, read from the
// same committed snapshot, so a new mutator that calls the guard is judged with
// no edit to this file.

package pluginregistrationsites

import (
	"context"
	"errors"
	"go/ast"
	"go/token"
	"maps"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	repoinventory "github.com/ze-software/ze/internal/le/repo/inventory"
)

const (
	// initName is the package initializer Go runs before main.
	initName = "init"
	// registryDirectory is the plugin registry's committed directory.
	registryDirectory = "internal/component/plugin/registry"
	// registryImport is the import path of registryDirectory.
	registryImport = "github.com/ze-software/ze/" + registryDirectory
	// sealGuard is the registry function every registering function reaches
	// before it writes (internal/component/plugin/registry/seal.go).
	sealGuard = "refuseSealedLocked"
	// ruleCommand prints the rule a refusal names.
	ruleCommand = "./le help plugin registration-sites check"
)

// Check reads the tree's committed HEAD and judges every registering site in
// it. An error means the check could not read or judge the tree, which is a
// different fact from a tree holding a wrong site.
func Check(ctx context.Context, root string) (Report, error) {
	snapshot, err := repoinventory.ReadCommitted(ctx, root)
	if err != nil {
		return Report{}, err
	}
	return Judge(snapshot)
}

// Judge judges every registering site of the Ze-module packages snapshot
// selected. vendor/ is never read: Go refuses a vendored package's import of
// an internal/ package, so vendored code holds a registering function only
// when Ze code hands it one, which is a value use refused here.
func Judge(snapshot *repoinventory.Snapshot) (Report, error) {
	files, fset := snapshot.Sources()
	tree := newSourceTree(snapshot, fset)
	for _, file := range files {
		if strings.HasPrefix(file.Path, "vendor/") {
			continue
		}
		tree.add(file)
	}
	registering, err := tree.registeringFunctions()
	if err != nil {
		return Report{}, err
	}
	tree.index(registering)

	report := Report{Revision: snapshot.Revision(), Packages: len(tree.packages), Registering: slices.Sorted(maps.Keys(registering))}
	for _, file := range tree.files {
		if path.Dir(file.Path) == registryDirectory {
			continue
		}
		report.Refused = append(report.Refused, tree.dotImports(file)...)
	}
	for _, name := range report.Registering {
		for _, reference := range tree.uses[name] {
			if reference.selector == nil || path.Dir(reference.file.Path) == registryDirectory {
				continue
			}
			site, held := tree.registrySite(reference)
			if !held {
				continue
			}
			if rule := tree.judge(reference, &site); rule != "" {
				report.Refused = append(report.Refused, Refusal{Site: site, Rule: rule, Fix: fixLine})
				continue
			}
			report.Accepted = append(report.Accepted, site)
		}
	}
	slices.SortFunc(report.Accepted, compareSites)
	slices.SortFunc(report.Refused, func(a, b Refusal) int { return compareSites(a.Site, b.Site) })
	return report, nil
}

// fixLine is the repair every refusal carries.
const fixLine = "call the registering function directly from init() of the plugin's register.go, or from a function only initialization calls; the rule: " + ruleCommand

// use is one reference to a name: a bare identifier, or the selector of a
// qualified one, with the syntax around it the rule reads.
type use struct {
	file     repoinventory.SourceFile
	ident    *ast.Ident
	selector *ast.SelectorExpr
	// called is true when the reference is the function of a call expression,
	// parentheses aside: anything else hands the function on as a value.
	called bool
	// decl is the innermost function declaration holding the reference, nil for
	// a package-level declaration.
	decl *ast.FuncDecl
	// literal is true when a function literal holds the reference.
	literal bool
	// initializer is true for a reference in a package-level var or const.
	initializer bool
}

// sourceTree is the Ze-module half of one snapshot, indexed for the rule.
// It is built and read by one Judge call and is not safe for concurrent use.
type sourceTree struct {
	snapshot  *repoinventory.Snapshot
	fset      *token.FileSet
	files     []repoinventory.SourceFile
	packages  map[string][]repoinventory.SourceFile
	functions map[string]map[string]*ast.FuncDecl
	uses      map[string][]use
	// verdicts memoizes initializationOnly; a function on the stack is held as
	// visiting, so a recursive chain answers rather than looping.
	verdicts map[*ast.FuncDecl]verdict
	imported map[*ast.SelectorExpr]importAnswer
}

type verdict struct {
	visiting bool
	reason   string
}

type importAnswer struct {
	directory string
	held      bool
}

func newSourceTree(snapshot *repoinventory.Snapshot, fset *token.FileSet) *sourceTree {
	return &sourceTree{
		snapshot: snapshot, fset: fset,
		packages:  make(map[string][]repoinventory.SourceFile),
		functions: make(map[string]map[string]*ast.FuncDecl),
		uses:      make(map[string][]use),
		verdicts:  make(map[*ast.FuncDecl]verdict),
		imported:  make(map[*ast.SelectorExpr]importAnswer),
	}
}

// add records file and its package-level functions (no receiver, not init).
func (t *sourceTree) add(file repoinventory.SourceFile) {
	directory := path.Dir(file.Path)
	t.files = append(t.files, file)
	t.packages[directory] = append(t.packages[directory], file)
	for _, declaration := range file.File.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil || function.Name.Name == initName {
			continue
		}
		if t.functions[directory] == nil {
			t.functions[directory] = make(map[string]*ast.FuncDecl)
		}
		t.functions[directory][function.Name.Name] = function
	}
}

// registeringFunctions answers the exported registry functions that reach the
// seal guard through calls inside the registry package. It refuses a snapshot
// without the registry or the guard, because an empty set would pass every
// tree.
func (t *sourceTree) registeringFunctions() (map[string]bool, error) {
	declared := t.functions[registryDirectory]
	if len(declared) == 0 {
		return nil, errors.New("committed snapshot selects no function of " + registryDirectory + "; the registration-sites check has nothing to derive its registering functions from; run it from a Ze checkout whose cmd/ze imports the plugin registry")
	}
	if declared[sealGuard] == nil {
		return nil, errors.New(registryDirectory + " declares no " + sealGuard + ", so no registry function can be told to register; restore the seal guard in internal/component/plugin/registry/seal.go")
	}
	callees := make(map[string][]string, len(declared))
	for name, function := range declared {
		ast.Inspect(function.Body, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				if callee, ok := ast.Unparen(call.Fun).(*ast.Ident); ok && declared[callee.Name] != nil {
					callees[name] = append(callees[name], callee.Name)
				}
			}
			return true
		})
	}
	reaches := map[string]bool{sealGuard: true}
	// The loop adds at least one function per pass or stops, so it runs at
	// most once per declared function.
	for changed := true; changed; {
		changed = false
		for name, called := range callees {
			if reaches[name] || !slices.ContainsFunc(called, func(callee string) bool { return reaches[callee] }) {
				continue
			}
			reaches[name] = true
			changed = true
		}
	}
	registering := make(map[string]bool)
	for name := range reaches {
		if ast.IsExported(name) {
			registering[name] = true
		}
	}
	if len(registering) == 0 {
		return nil, errors.New("no exported function of " + registryDirectory + " calls " + sealGuard + ", so the check would judge nothing; make every registering function call the seal guard")
	}
	return registering, nil
}

// index records every reference to a registering function or to a
// package-level function, with the syntax around it.
func (t *sourceTree) index(registering map[string]bool) {
	names := make(map[string]bool)
	for _, functions := range t.functions {
		for name := range functions {
			names[name] = true
		}
	}
	for name := range registering {
		names[name] = true
	}
	for _, file := range t.files {
		t.indexFile(file, names)
	}
}

// indexFile walks file once with the stack of enclosing nodes.
func (t *sourceTree) indexFile(file repoinventory.SourceFile, names map[string]bool) {
	var stack []ast.Node
	ast.Inspect(file.File, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if declaration, ok := node.(*ast.GenDecl); ok && declaration.Tok == token.IMPORT {
			return false
		}
		switch value := node.(type) {
		case *ast.SelectorExpr:
			if names[value.Sel.Name] {
				t.uses[value.Sel.Name] = append(t.uses[value.Sel.Name], t.useAt(file, stack, value, nil, value))
			}
		case *ast.Ident:
			if names[value.Name] && bareReference(stack, value, file.File) {
				t.uses[value.Name] = append(t.uses[value.Name], t.useAt(file, stack, value, value, nil))
			}
		}
		stack = append(stack, node)
		return true
	})
}

// bareReference is false for an identifier that names no package-level value:
// the selector half of a qualified name, a declaration's own name, a field, a
// composite-literal key, the package clause.
func bareReference(stack []ast.Node, ident *ast.Ident, file *ast.File) bool {
	if ident == file.Name || len(stack) == 0 {
		return false
	}
	switch parent := stack[len(stack)-1].(type) {
	case *ast.SelectorExpr:
		return parent.Sel != ident
	case *ast.FuncDecl:
		return parent.Name != ident
	case *ast.Field:
		return !slices.Contains(parent.Names, ident)
	case *ast.KeyValueExpr:
		if parent.Key != ident || len(stack) < 2 {
			return true
		}
		_, inLiteral := stack[len(stack)-2].(*ast.CompositeLit)
		return !inLiteral
	}
	return true
}

// useAt records the syntax around node, the reference whose stack is given.
func (t *sourceTree) useAt(file repoinventory.SourceFile, stack []ast.Node, node ast.Expr, ident *ast.Ident, selector *ast.SelectorExpr) use {
	found := use{file: file, ident: ident, selector: selector}
	var child ast.Node = node
	for _, parent := range slices.Backward(stack) {
		if paren, ok := parent.(*ast.ParenExpr); ok && paren.X == child {
			child = paren
			continue
		}
		call, ok := parent.(*ast.CallExpr)
		found.called = ok && call.Fun == child
		break
	}
	declared := false
	for _, parent := range slices.Backward(stack) {
		switch enclosing := parent.(type) {
		case *ast.FuncLit:
			found.literal = true
		case *ast.FuncDecl:
			found.decl = enclosing
		case *ast.GenDecl:
			declared = true
		}
	}
	// A GenDecl inside a function is a local declaration, not a package-level
	// initializer, so only a reference outside every function is one.
	found.initializer = declared && found.decl == nil && !found.literal
	return found
}

// registrySite answers the site of reference when its qualifier is the plugin
// registry import. A qualifier spelled as the file's registry alias that the
// import binding cannot prove is held too, and judged: a site the check cannot
// resolve is never passed in silence.
func (t *sourceTree) registrySite(reference use) (Site, bool) {
	directory, held := t.importedDirectory(reference.file, reference.selector)
	if !held {
		owner, ok := reference.selector.X.(*ast.Ident)
		alias, imports := registryAlias(reference.file.File)
		if !ok || !imports || owner.Name != alias || owner.Obj != nil {
			return Site{}, false
		}
	} else if directory != registryDirectory {
		return Site{}, false
	}
	return t.siteOf(reference, "registry."+reference.selector.Sel.Name), true
}

// importedDirectory memoizes Snapshot.ImportedDirectory per selector.
func (t *sourceTree) importedDirectory(file repoinventory.SourceFile, selector *ast.SelectorExpr) (string, bool) {
	if answer, known := t.imported[selector]; known {
		return answer.directory, answer.held
	}
	directory, held := t.snapshot.ImportedDirectory(file, selector)
	t.imported[selector] = importAnswer{directory: directory, held: held}
	return directory, held
}

// registryAlias answers the name file binds the plugin registry import to.
func registryAlias(file *ast.File) (string, bool) {
	for _, imported := range file.Imports {
		name, err := strconv.Unquote(imported.Path.Value)
		if err != nil || name != registryImport {
			continue
		}
		if imported.Name != nil {
			return imported.Name.Name, true
		}
		return path.Base(registryImport), true
	}
	return "", false
}

// dotImports refuses a dot import of the registry: its registering functions
// are then bare names, which no qualifier ties to the registry.
func (t *sourceTree) dotImports(file repoinventory.SourceFile) []Refusal {
	var refused []Refusal
	for _, imported := range file.File.Imports {
		name, err := strconv.Unquote(imported.Path.Value)
		if err != nil || name != registryImport || imported.Name == nil || imported.Name.Name != "." {
			continue
		}
		position := t.fset.Position(imported.Pos())
		refused = append(refused, Refusal{
			Site: Site{Package: path.Dir(file.Path), File: file.Path, Line: position.Line, Function: "registry (dot import)", Within: "import"},
			Rule: "the plugin registry is dot-imported, so its registering functions are bare names this check cannot tell from the file's own",
			Fix:  "import the plugin registry under a name, then " + fixLine,
		})
	}
	return refused
}

// judge answers the rule reference breaks, or "" for an accepted site, and
// names the declaration holding the site in site.Within.
func (t *sourceTree) judge(reference use, site *Site) string {
	site.Within = withinOf(reference)
	switch {
	case !reference.called:
		return "a registering function is used as a value, so the reference can reach code that runs after start-up, which the seal then refuses"
	case reference.literal:
		return "a registering function is called inside a function literal, which can run after start-up, when the seal refuses it"
	case !registerFile(reference.file.Path):
		return "a registering function is called outside a register*.go file, where Ze keeps every registration"
	case reference.decl == nil && reference.initializer:
		return ""
	case reference.decl == nil:
		return "a registering function is called outside any function or package initializer"
	case reference.decl.Recv != nil:
		return "a registering function is called in method " + reference.decl.Name.Name + ", which any holder of its receiver can call after start-up"
	case reference.decl.Name.Name == initName:
		return ""
	}
	return t.initializationOnly(path.Dir(reference.file.Path), reference.decl)
}

// initializationOnly answers "" when every reference to function is a direct
// call only package initialization reaches, and the first reference that is
// not otherwise. The chain is followed across Ze packages; it ends at an
// init() or a package-level initializer of a register*.go file.
func (t *sourceTree) initializationOnly(directory string, function *ast.FuncDecl) string {
	if known, held := t.verdicts[function]; held {
		if known.visiting {
			return function.Name.Name + " is reached from itself, so no initializer bounds when it runs"
		}
		return known.reason
	}
	t.verdicts[function] = verdict{visiting: true}
	reason := t.chainReason(directory, function)
	t.verdicts[function] = verdict{reason: reason}
	return reason
}

func (t *sourceTree) chainReason(directory string, function *ast.FuncDecl) string {
	name := function.Name.Name
	if strings.HasPrefix(directory, "pkg/") && ast.IsExported(name) {
		return name + " is exported from the public pkg/ tree, so code outside this module can call it after start-up"
	}
	references := t.references(directory, function)
	if len(references) == 0 {
		return name + " is never called, so nothing shows it runs during package initialization"
	}
	for _, reference := range references {
		at := t.position(reference)
		switch {
		case !reference.called:
			return name + " is used as a value at " + at + ", so it can run after start-up"
		case reference.literal:
			return name + " is called inside a function literal at " + at + ", which can run after start-up"
		case reference.decl == nil && !reference.initializer:
			return name + " is called outside any function or package initializer at " + at
		case reference.decl == nil || reference.decl.Name.Name == initName:
			if !registerFile(reference.file.Path) {
				return name + " is called from initialization outside a register*.go file at " + at
			}
			continue
		case reference.decl.Recv != nil:
			return name + " is called from method " + reference.decl.Name.Name + " at " + at + ", which any holder of its receiver can call after start-up"
		}
		if reason := t.initializationOnly(path.Dir(reference.file.Path), reference.decl); reason != "" {
			return reason
		}
	}
	return ""
}

// references answers every reference to function: bare names in its own
// package that resolve to it, and qualified names elsewhere whose import names
// its package.
func (t *sourceTree) references(directory string, function *ast.FuncDecl) []use {
	var found []use
	for _, reference := range t.uses[function.Name.Name] {
		home := path.Dir(reference.file.Path)
		if reference.ident != nil {
			if home != directory {
				continue
			}
			if reference.ident.Obj != nil && reference.ident.Obj.Decl != function {
				continue
			}
			found = append(found, reference)
			continue
		}
		if home == directory || !ast.IsExported(function.Name.Name) {
			continue
		}
		if imported, held := t.importedDirectory(reference.file, reference.selector); held && imported == directory {
			found = append(found, reference)
		}
	}
	return found
}

func (t *sourceTree) siteOf(reference use, function string) Site {
	position := t.fset.Position(reference.selector.Pos())
	return Site{Package: path.Dir(reference.file.Path), File: reference.file.Path, Line: position.Line, Function: function}
}

func (t *sourceTree) position(reference use) string {
	var node ast.Node = reference.ident
	if reference.selector != nil {
		node = reference.selector
	}
	position := t.fset.Position(node.Pos())
	var tb textbuf.Buffer
	return tb.Str(reference.file.Path).Byte(':').Int(int64(position.Line)).String()
}

// withinOf names the declaration holding reference.
func withinOf(reference use) string {
	switch {
	case reference.decl != nil && reference.decl.Recv != nil:
		return "method " + reference.decl.Name.Name
	case reference.decl != nil:
		return reference.decl.Name.Name
	case reference.initializer:
		return "package initializer"
	}
	return "file scope"
}

// registerFile is Ze's registration-file convention, the one the edit hook's
// init() rule holds too (internal/le/hookruntime/writeedit.go): register.go and
// its register_*.go split or platform variants.
func registerFile(name string) bool {
	base := path.Base(name)
	return strings.HasPrefix(base, "register") && strings.HasSuffix(base, ".go")
}

func compareSites(a, b Site) int {
	if order := strings.Compare(a.File, b.File); order != 0 {
		return order
	}
	return a.Line - b.Line
}
