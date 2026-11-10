// Design: docs/functional-tests.md -- a compiled fixture that stops the daemon it runs inside
// Overview: register_stops_daemon_guard_test.go -- the guard that runs this walk over the package
// Related: fixture.go -- Register, StopsDaemon

package fixture

import (
	"go/ast"
	"go/token"
	"slices"
	"strconv"
)

// The walk below judges every registration site of the package from its
// syntax alone, so the guard needs no build of the drivers and no registry.
// It imports nothing but go/ast, so a codemod can run the same walk over the
// package to add the declarations it asks for.

// daemonStopCommand is the command a fixture dispatches to stop the daemon
// it runs inside. observeConfigured sends it after every Observe scenario.
const daemonStopCommand = "request shutdown"

// stopsDaemonProperty is the Property a registration site names to declare it.
const stopsDaemonProperty = "StopsDaemon"

// registerFunction is the registrar every site ends in.
const registerFunction = "Register"

// stopsDaemonFindingKind names what a registration site got wrong.
type stopsDaemonFindingKind uint8

const (
	stopsDaemonFindingUnspecified stopsDaemonFindingKind = iota
	// stopsDaemonFine: the site declares StopsDaemon exactly when its driver
	// reaches the command, or it is a helper that hands its caller's
	// properties on.
	stopsDaemonFine
	// stopsDaemonMissing: the driver reaches the command, the site does not
	// declare StopsDaemon.
	stopsDaemonMissing
	// stopsDaemonUndue: the site declares StopsDaemon, the driver never
	// reaches the command.
	stopsDaemonUndue
	// stopsDaemonDropped: a helper registers a driver its caller passes in, and
	// does not hand its caller's properties on, so no caller can declare one.
	stopsDaemonDropped
)

// stopsDaemonSite is one call that registers a driver: a Register call, or a
// call of a package helper that registers a driver its caller passes in.
type stopsDaemonSite struct {
	position token.Position
	// scope is the function the call sits in, and callee the one it calls.
	scope  string
	callee string
	// argsEnd is where a property argument goes: the end of the last argument.
	argsEnd token.Pos
	finding stopsDaemonFindingKind
}

// stopsDaemonWalk is the package's non-test source, indexed for the walk.
// It is not safe for concurrent use: the memo is written as the walk runs.
type stopsDaemonWalk struct {
	// functions and methods hold every declaration by name. A name two
	// platform files both declare holds both declarations.
	functions map[string][]*ast.FuncDecl
	methods   map[string][]*ast.FuncDecl
	// variables holds the initializers of every package-level variable.
	variables map[string][]ast.Expr
	// shutdownConstants names the package constants whose value is the command.
	shutdownConstants map[string]bool
	// types holds the type expression of every package type, so a parameter
	// of a named func type (Driver, ObserverScenario) or an alias of one is
	// known to carry a driver.
	types map[string]ast.Expr
	// registrars holds every package function that registers a driver one of
	// its parameters supplies, so its callers are the registration sites.
	registrars map[string]*ast.FuncDecl
	memo       map[string]bool
	visiting   map[string]bool
}

// newStopsDaemonWalk indexes the files of one package.
func newStopsDaemonWalk(files []*ast.File) *stopsDaemonWalk {
	source := &stopsDaemonWalk{
		functions:         make(map[string][]*ast.FuncDecl),
		methods:           make(map[string][]*ast.FuncDecl),
		variables:         make(map[string][]ast.Expr),
		shutdownConstants: make(map[string]bool),
		types:             make(map[string]ast.Expr),
		registrars:        make(map[string]*ast.FuncDecl),
		memo:              make(map[string]bool),
		visiting:          make(map[string]bool),
	}
	for _, file := range files {
		source.index(file)
	}
	source.findRegistrars()
	return source
}

func (source *stopsDaemonWalk) index(file *ast.File) {
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if decl.Recv == nil {
				source.functions[decl.Name.Name] = append(source.functions[decl.Name.Name], decl)
				continue
			}
			source.methods[decl.Name.Name] = append(source.methods[decl.Name.Name], decl)
		case *ast.GenDecl:
			source.indexGeneral(decl)
		}
	}
}

func (source *stopsDaemonWalk) indexGeneral(decl *ast.GenDecl) {
	for _, spec := range decl.Specs {
		switch spec := spec.(type) {
		case *ast.TypeSpec:
			source.types[spec.Name.Name] = spec.Type
		case *ast.ValueSpec:
			for i, ident := range spec.Names {
				if i >= len(spec.Values) {
					continue
				}
				if decl.Tok == token.CONST {
					source.shutdownConstants[ident.Name] = source.shutdownConstants[ident.Name] || isDaemonStopLiteral(spec.Values[i])
					continue
				}
				source.variables[ident.Name] = append(source.variables[ident.Name], spec.Values[i])
			}
		}
	}
}

// findRegistrars marks every function that passes a driver-typed parameter of
// its own to a registration site, until no more are found: a helper that calls
// a helper that calls Register is a registrar too. A call that declares
// StopsDaemon itself does not make its helper a registrar: the helper then
// declares for every caller, and the call is judged where it stands.
func (source *stopsDaemonWalk) findRegistrars() {
	for changed := true; changed; {
		changed = false
		for name, decls := range source.functions {
			if _, known := source.registrars[name]; known {
				continue
			}
			for _, decl := range decls {
				if source.passesDriverParameter(decl) {
					source.registrars[name] = decl
					changed = true
					break
				}
			}
		}
	}
}

func (source *stopsDaemonWalk) passesDriverParameter(decl *ast.FuncDecl) bool {
	parameters := source.driverParameters(decl)
	if len(parameters) == 0 {
		return false
	}
	if decl.Body == nil {
		return false
	}
	passes := false
	ast.Inspect(decl.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if passes {
			return !passes
		}
		if !ok {
			return !passes
		}
		if _, registration := source.registrationCallee(call); !registration {
			return true
		}
		if slices.ContainsFunc(call.Args, isStopsDaemonProperty) {
			return true
		}
		for _, arg := range call.Args {
			ast.Inspect(arg, func(node ast.Node) bool {
				ident, ok := node.(*ast.Ident)
				if ok && parameters[ident.Name] {
					passes = true
				}
				return !passes
			})
		}
		return !passes
	})
	return passes
}

// driverParameters names the parameters of decl that can carry a driver: a
// func, an interface (any), or a package type that is one of those.
func (source *stopsDaemonWalk) driverParameters(decl *ast.FuncDecl) map[string]bool {
	parameters := make(map[string]bool)
	for _, field := range decl.Type.Params.List {
		if !source.canCarryDriver(field.Type, 0) {
			continue
		}
		for _, name := range field.Names {
			parameters[name.Name] = true
		}
	}
	return parameters
}

// typeDepthMax bounds how many named types canCarryDriver follows, so a
// cycle of type names in broken source cannot recurse without end.
const typeDepthMax = 8

func (source *stopsDaemonWalk) canCarryDriver(expr ast.Expr, depth int) bool {
	switch expr := expr.(type) {
	case *ast.FuncType, *ast.InterfaceType:
		return true
	case *ast.Ident:
		if expr.Name == "any" {
			return true
		}
		named, known := source.types[expr.Name]
		return known && depth < typeDepthMax && source.canCarryDriver(named, depth+1)
	}
	return false
}

// propertiesParameter names the variadic Property parameter of decl, or ""
// when it has none.
func propertiesParameter(decl *ast.FuncDecl) string {
	fields := decl.Type.Params.List
	if len(fields) == 0 {
		return ""
	}
	last := fields[len(fields)-1]
	variadic, ok := last.Type.(*ast.Ellipsis)
	if !ok {
		return ""
	}
	if len(last.Names) != 1 {
		return ""
	}
	ident, ok := variadic.Elt.(*ast.Ident)
	if !ok {
		return ""
	}
	if ident.Name != "Property" {
		return ""
	}
	return last.Names[0].Name
}

// registrationCallee answers the name call calls when it is a registration
// call: Register with a name and a driver, or a registrar. Every other call,
// a selector or a function value included, answers false.
func (source *stopsDaemonWalk) registrationCallee(call *ast.CallExpr) (string, bool) {
	fun, ok := call.Fun.(*ast.Ident)
	if !ok {
		return "", false
	}
	if fun.Name == registerFunction {
		return fun.Name, len(call.Args) >= 2
	}
	_, registrar := source.registrars[fun.Name]
	return fun.Name, registrar
}

// sites judges every registration call in files, which must be the files the
// source indexed.
func (source *stopsDaemonWalk) sites(fset *token.FileSet, files []*ast.File) []stopsDaemonSite {
	var sites []stopsDaemonSite
	for _, file := range files {
		for _, decl := range file.Decls {
			scope, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if scope.Body == nil {
				continue
			}
			ast.Inspect(scope.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				if callee, registration := source.registrationCallee(call); registration {
					sites = append(sites, source.judgeSite(fset, scope, call, callee))
				}
				return true
			})
		}
	}
	return sites
}

// judgeSite judges one registration call; callee is the name
// registrationCallee answered for it.
func (source *stopsDaemonWalk) judgeSite(fset *token.FileSet, scope *ast.FuncDecl, call *ast.CallExpr, callee string) stopsDaemonSite {
	site := stopsDaemonSite{
		position: fset.Position(call.Pos()),
		scope:    scope.Name.Name,
		callee:   callee,
		argsEnd:  call.Args[len(call.Args)-1].End(),
		finding:  stopsDaemonFine,
	}
	if _, inRegistrar := source.registrars[scope.Name.Name]; inRegistrar && scope.Recv == nil {
		// The caller of this helper is the site: the helper only hands its
		// caller's properties on.
		if !forwardsProperties(scope, call) {
			site.finding = stopsDaemonDropped
		}
		return site
	}
	reaches := source.callReaches(scope, call, callee)
	declared := slices.ContainsFunc(call.Args, isStopsDaemonProperty)
	if reaches && !declared {
		site.finding = stopsDaemonMissing
	}
	if declared && !reaches {
		site.finding = stopsDaemonUndue
	}
	return site
}

func forwardsProperties(scope *ast.FuncDecl, call *ast.CallExpr) bool {
	parameter := propertiesParameter(scope)
	if parameter == "" {
		return false
	}
	if !call.Ellipsis.IsValid() {
		return false
	}
	last, ok := call.Args[len(call.Args)-1].(*ast.Ident)
	return ok && last.Name == parameter
}

// callReaches answers whether the driver one registration call registers
// reaches the command: through its arguments (the name excepted, for
// Register), or through the registrar's own body.
func (source *stopsDaemonWalk) callReaches(scope *ast.FuncDecl, call *ast.CallExpr, callee string) bool {
	args := call.Args
	if callee == registerFunction {
		args = args[1:]
	}
	for _, arg := range args {
		if source.reaches(arg, scope) {
			return true
		}
	}
	registrar, isRegistrar := source.registrars[callee]
	if !isRegistrar {
		return false
	}
	return source.memoized("registrar "+callee, func() bool {
		reaches := false
		ast.Inspect(registrar.Body, func(node ast.Node) bool {
			inner, ok := node.(*ast.CallExpr)
			if reaches {
				return !reaches
			}
			if !ok {
				return !reaches
			}
			if innerCallee, registration := source.registrationCallee(inner); registration {
				reaches = source.callReaches(registrar, inner, innerCallee)
			}
			return !reaches
		})
		return reaches
	})
}

// reaches answers whether node holds the command, or names something of the
// package that does: a function, a method, a package variable's initializer,
// or a variable local to scope. A parameter of scope is not followed: a
// registrar's caller supplies it, and that caller is the site judged.
//
// The walk over-approximates. A function named anywhere counts as reached,
// called or not, and a method is matched by name alone. A false positive asks
// for a declaration the driver may not need, and the site names the driver so
// a reader can see why.
func (source *stopsDaemonWalk) reaches(node ast.Node, scope *ast.FuncDecl) bool {
	found := false
	ast.Inspect(node, func(node ast.Node) bool {
		if found {
			return false
		}
		switch node := node.(type) {
		case *ast.BasicLit:
			found = isDaemonStopLiteral(node)
		case *ast.SelectorExpr:
			found = source.reaches(node.X, scope) || source.declarationsReach("method "+node.Sel.Name, source.methods[node.Sel.Name])
			return false
		case *ast.Ident:
			found = source.identReaches(node.Name, scope)
		}
		return !found
	})
	return found
}

func (source *stopsDaemonWalk) identReaches(name string, scope *ast.FuncDecl) bool {
	if source.shutdownConstants[name] {
		return true
	}
	if decls, known := source.functions[name]; known {
		return source.declarationsReach("func "+name, decls)
	}
	if values, known := source.variables[name]; known {
		return source.memoized("var "+name, func() bool {
			return slices.ContainsFunc(values, func(value ast.Expr) bool { return source.reaches(value, nil) })
		})
	}
	if scope == nil {
		return false
	}
	if scope.Body == nil {
		return false
	}
	key := "local " + strconv.Itoa(int(scope.Pos())) + " " + name
	return source.memoized(key, func() bool {
		return slices.ContainsFunc(localDefinitions(scope, name), func(value ast.Expr) bool {
			return source.reaches(value, scope)
		})
	})
}

func (source *stopsDaemonWalk) declarationsReach(key string, decls []*ast.FuncDecl) bool {
	if len(decls) == 0 {
		return false
	}
	return source.memoized(key, func() bool {
		return slices.ContainsFunc(decls, func(decl *ast.FuncDecl) bool {
			return decl.Body != nil && source.reaches(decl.Body, decl)
		})
	})
}

// memoized answers compute once per key. A key met again while it is still
// being computed (a recursive function) answers false for that inner visit.
func (source *stopsDaemonWalk) memoized(key string, compute func() bool) bool {
	if reaches, done := source.memo[key]; done {
		return reaches
	}
	if source.visiting[key] {
		return false
	}
	source.visiting[key] = true
	reaches := compute()
	source.visiting[key] = false
	source.memo[key] = reaches
	return reaches
}

// localDefinitions returns every expression scope assigns to a variable of
// this name: a := or = right-hand side, the range expression of a range loop
// that binds it, and a var declaration's value. It matches by name, so a
// shadowing variable adds its own value too.
func localDefinitions(scope *ast.FuncDecl, name string) []ast.Expr {
	var values []ast.Expr
	isName := func(expr ast.Expr) bool {
		ident, ok := expr.(*ast.Ident)
		return ok && ident.Name == name
	}
	ast.Inspect(scope.Body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.AssignStmt:
			for i, lhs := range node.Lhs {
				if !isName(lhs) {
					continue
				}
				if len(node.Rhs) == len(node.Lhs) {
					values = append(values, node.Rhs[i])
					continue
				}
				values = append(values, node.Rhs...)
			}
		case *ast.RangeStmt:
			if isName(node.Key) || isName(node.Value) {
				values = append(values, node.X)
			}
		case *ast.ValueSpec:
			for i, ident := range node.Names {
				if ident.Name == name && i < len(node.Values) {
					values = append(values, node.Values[i])
				}
			}
		}
		return true
	})
	return values
}

func isStopsDaemonProperty(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == stopsDaemonProperty
}

func isDaemonStopLiteral(expr ast.Expr) bool {
	literal, ok := expr.(*ast.BasicLit)
	if !ok {
		return false
	}
	if literal.Kind != token.STRING {
		return false
	}
	value, err := strconv.Unquote(literal.Value)
	return err == nil && value == daemonStopCommand
}
