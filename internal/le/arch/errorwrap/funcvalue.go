// Design: docs/contributing/ze-go-style.md -- following a function value to the functions it can hold
// Related: resolve.go -- naming a callee from its syntax
// Related: errorwrap.go -- the trace that reads what this answers
//
// funcvalue.go answers which function bodies a called name can hold. The
// hub's reload is the shape it exists for: the error a wrap reads comes from
// a parameter, whose value is a literal built three callers away.
//
//	func diskConfigLoaders(...) (loadMap ..., loadBoth func() (...)) {
//		loadBoth = func() (...) { ... }
//		return loadMap, loadBoth
//	}
//	_, loadBoth = diskConfigLoaders(...)
//	go handleSIGHUPReload(..., loadBoth, ...) // then sighupReload, signalReload
//	newTree, parsedTree, loadErr := load()    // in runReloadContext
//
// A name is followed to the value it was last bound to: a function literal,
// another name, a function or method the syntax names, or one result of a
// call, read from that callee's return statements. A function parameter is
// followed to the argument each call of its function in the same package
// passes for it.
//
// Every step is one level of depthMax, the bound the trace already holds, so a
// value built more than depthMax steps from its use is not seen. A parameter
// already being followed higher up the same walk adds nothing (a function
// passing its own parameter to itself): its values reach the answer through
// the outer walk. What the syntax cannot follow makes the answer incomplete,
// and the trace counts the wrap untraced, while still reading every body it
// did name: a parameter of a method or a literal, whose callers no name
// finds; a parameter of an exported function, which another package may call
// with a value this package never sees; a call that passes fewer arguments
// than the parameter's position; a bare return of named results; an
// interface method; a name bound to nothing the package declares.

package archerrorwrap

import (
	"go/ast"
	"go/token"
)

// callSite is one call by a bare name, in the function that holds it.
type callSite struct {
	fn   *funcRef
	call *ast.CallExpr
}

// paramKey names one parameter of one function body by its position.
type paramKey struct {
	body     *ast.BlockStmt
	position int
}

// funcSet is the bodies a function value can hold, and whether the syntax
// named every one. Its zero value is the answer "named nothing, and could not
// look": an incomplete empty set, which the trace counts as untraced.
type funcSet struct {
	refs     []*funcRef
	complete bool
}

// add joins other's bodies to s; s stays complete only while other is.
func (s *funcSet) add(other funcSet) {
	s.refs = append(s.refs, other.refs...)
	if !other.complete {
		s.complete = false
	}
}

// funcValues answers every body expr can hold when fn evaluates it at at.
func (ix *index) funcValues(fn *funcRef, expr ast.Expr, at token.Pos, depth int) (funcSet, error) {
	if depth >= depthMax {
		return funcSet{complete: true}, nil
	}
	switch typed := ast.Unparen(expr).(type) {
	case *ast.Ident:
		return ix.nameValues(fn, typed.Name, at, depth)
	case *ast.CallExpr:
		return ix.returnedValues(fn, typed, 0, depth+1)
	}
	ref, how, err := ix.resolve(fn, expr)
	if err != nil {
		return funcSet{}, err
	}
	switch how {
	case resolutionFound:
		return funcSet{refs: []*funcRef{ref}, complete: true}, nil
	case resolutionOutside:
		return funcSet{complete: true}, nil
	case resolutionUnnamed:
		return funcSet{}, nil
	case resolutionUnspecified:
		panic("BUG: resolve answered no resolution beside a nil error")
	}
	panic("BUG: resolve answered a resolution this switch does not list")
}

// nameValues answers every body a bare name can hold: the value it was last
// bound to, the arguments passed for it when it is a parameter, or the
// function the package declares under it.
func (ix *index) nameValues(fn *funcRef, name string, at token.Pos, depth int) (funcSet, error) {
	value, result, owner := lookup(fn, name, at)
	if value != nil {
		return ix.valueAt(owner, name, value, result, value.Pos(), depth)
	}
	if scope, position, isParam := paramOf(fn, name); isParam {
		return ix.argumentValues(scope, position, depth+1)
	}
	ref, declared := fn.pkg.funcs[name]
	// refuse-by-default: guards markUntraced -- the empty set is incomplete, so the trace counts the wrap untraced
	if !declared {
		return funcSet{}, nil
	}
	return funcSet{refs: []*funcRef{ref}, complete: true}, nil
}

// valueAt answers the bodies one bound value can hold, result naming which
// result of a multi-value call it is. A literal is named name.
func (ix *index) valueAt(fn *funcRef, name string, value ast.Expr, result int, at token.Pos, depth int) (funcSet, error) {
	switch typed := ast.Unparen(value).(type) {
	case *ast.FuncLit:
		return funcSet{refs: []*funcRef{literal(fn, typed, name)}, complete: true}, nil
	case *ast.CallExpr:
		return ix.returnedValues(fn, typed, result, depth+1)
	}
	return ix.funcValues(fn, value, at, depth+1)
}

// returnedValues answers the bodies result of call can hold, read from the
// return statements of every function call reaches.
func (ix *index) returnedValues(fn *funcRef, call *ast.CallExpr, result, depth int) (funcSet, error) {
	if depth >= depthMax {
		return funcSet{complete: true}, nil
	}
	callees, err := ix.funcValues(fn, call.Fun, call.Pos(), depth)
	if err != nil {
		return funcSet{}, err
	}
	answer := funcSet{complete: callees.complete}
	for _, callee := range callees.refs {
		returned, err := ix.calleeReturns(callee, result, depth+1)
		if err != nil {
			return funcSet{}, err
		}
		answer.add(returned)
	}
	return answer, nil
}

// calleeReturns answers the bodies result of callee's returns can hold,
// outside nested function literals.
func (ix *index) calleeReturns(callee *funcRef, result, depth int) (funcSet, error) {
	answer := funcSet{complete: true}
	var err error
	ast.Inspect(callee.body, func(node ast.Node) bool {
		if err != nil {
			return false
		}
		switch stmt := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			value, valueResult, named := returnedAt(stmt.Results, result)
			if !named {
				answer.complete = false
				return false
			}
			var held funcSet
			held, err = ix.valueAt(callee, callee.name+" (func literal)", value, valueResult, stmt.Pos(), depth)
			answer.add(held)
			return false
		}
		return true
	})
	if err != nil {
		return funcSet{}, err
	}
	return answer, nil
}

// returnedAt answers the expression a return statement gives result, and
// which result of it that is: its own expression when the statement lists
// every result, or the one call it passes up whole. A bare return of named
// results names nothing.
func returnedAt(results []ast.Expr, result int) (ast.Expr, int, bool) {
	if len(results) == 1 {
		return results[0], result, true
	}
	if result < len(results) {
		return results[result], 0, true
	}
	return nil, 0, false
}

// argumentValues answers every body the parameter at position of scope can
// hold: the argument each call of scope in its package passes there.
func (ix *index) argumentValues(scope *funcRef, position, depth int) (funcSet, error) {
	if scope.callable == "" {
		return funcSet{}, nil
	}
	key := paramKey{body: scope.body, position: position}
	if held, read := ix.paramMemo[key]; read {
		return held, nil
	}
	// A parameter already followed higher up this walk: the outer walk holds
	// its values, and following it again would loop.
	if ix.paramOpen[key] {
		ix.paramCuts++
		return funcSet{complete: true}, nil
	}
	ix.paramOpen[key] = true
	cuts := ix.paramCuts
	answer, err := ix.callerArguments(scope, position, depth)
	delete(ix.paramOpen, key)
	if err != nil {
		return funcSet{}, err
	}
	// An answer that met a parameter open higher up lacks the values that
	// parameter holds, so only the outer walk's answer is whole; it is not
	// kept for a later walk to reuse.
	if ix.paramCuts == cuts {
		ix.paramMemo[key] = answer
	}
	return answer, nil
}

// callerArguments reads the argument at position of every call of scope in
// its package. An exported function stays incomplete whatever its callers
// pass here, because another package may call it too.
func (ix *index) callerArguments(scope *funcRef, position, depth int) (funcSet, error) {
	answer := funcSet{complete: !ast.IsExported(scope.callable)}
	for _, site := range scope.pkg.callSites(scope.callable) {
		if position >= len(site.call.Args) {
			answer.complete = false
			continue
		}
		held, err := ix.valueAt(site.fn, site.fn.name+" (func literal)", site.call.Args[position], 0, site.call.Pos(), depth)
		if err != nil {
			return funcSet{}, err
		}
		answer.add(held)
	}
	return answer, nil
}

// paramOf answers the function, fn or one it is nested in, that declares
// name as a parameter, and its position in that parameter list.
func paramOf(fn *funcRef, name string) (*funcRef, int, bool) {
	for scope := fn; scope != nil; scope = scope.outer {
		if position, declared := paramPosition(scope.params, name); declared {
			return scope, position, true
		}
	}
	return nil, 0, false
}

// paramPosition answers where name sits in a parameter list, counting each
// parameter, named or not.
func paramPosition(params *ast.FieldList, name string) (int, bool) {
	if params == nil {
		return 0, false
	}
	position := 0
	for _, field := range params.List {
		if len(field.Names) == 0 {
			position++
			continue
		}
		for _, ident := range field.Names {
			if ident.Name == name {
				return position, true
			}
			position++
		}
	}
	return 0, false
}

// callSites answers every call by the bare name name in the package, each in
// the function that holds it, indexing the package's calls the first time.
func (pkg *goPackage) callSites(name string) []callSite {
	// A nil map is the index not yet built; a built one is never nil.
	if pkg.calls == nil {
		pkg.calls = map[string][]callSite{}
		for _, file := range pkg.files {
			for _, decl := range file.syntax.Decls {
				fn := &funcRef{name: packageScope, pkg: pkg, file: file, body: &ast.BlockStmt{}}
				if funcDecl, isFunc := decl.(*ast.FuncDecl); isFunc {
					if funcDecl.Body == nil {
						continue
					}
					fn = declaredRef(pkg, file, funcDecl)
				}
				pkg.indexCalls(fn, decl)
			}
		}
	}
	return pkg.calls[name]
}

// indexCalls records every call by a bare name under root, held by fn, and
// those of every function literal inside it, each under its own literal.
func (pkg *goPackage) indexCalls(fn *funcRef, root ast.Node) {
	ast.Inspect(root, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.FuncLit:
			pkg.indexCalls(literal(fn, typed, fn.name+" (func literal)"), typed.Body)
			return false
		case *ast.CallExpr:
			if ident, isIdent := ast.Unparen(typed.Fun).(*ast.Ident); isIdent {
				pkg.calls[ident.Name] = append(pkg.calls[ident.Name], callSite{fn: fn, call: typed})
			}
		}
		return true
	})
}
