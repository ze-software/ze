// Design: docs/contributing/ze-go-style.md -- a zero value is never an answer
// Related: refusebydefault.go -- the state-type convention and finding anchors
//
// state_assignments.go follows file-local declarations to recognize literal
// field writes after a state value's construction. It does not load packages
// or infer a call's result type.
package archrefusebydefault

import (
	"go/ast"
	"go/token"
)

// stateScope holds each visible name, including names whose type is unknown
// or is not a state. Its parent is the enclosing lexical scope, or nil for
// the package. Keeping every binding makes a shadow hide an outer state.
type stateScope struct {
	parent *stateScope
	names  map[string]bool
}

// stateAssignments judges selector assignments against visible source types.
func (s *fileScan) stateAssignments(file *ast.File) {
	scope := &stateScope{names: map[string]bool{}}
	var values []*ast.ValueSpec
	for _, decl := range file.Decls {
		switch typed := decl.(type) {
		case *ast.FuncDecl:
			if typed.Recv == nil {
				scope.names[typed.Name.Name] = false
			}
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				switch named := spec.(type) {
				case *ast.ValueSpec:
					values = append(values, named)
					for _, name := range named.Names {
						scope.names[name.Name] = stateNamedType(named.Type)
					}
				case *ast.TypeSpec:
					scope.names[named.Name.Name] = false
				case *ast.ImportSpec:
					if named.Name != nil {
						scope.names[named.Name.Name] = false
					}
				}
			}
		}
	}
	scope.resolvePackageStates(values)
	ast.Walk(&stateAssignmentWalk{scan: s, scope: scope, fn: packageScope}, file)
}

// resolvePackageStates follows package initializer dependencies independently
// of declaration order. Each pass only promotes unknown bindings to known
// states, and a pass without a promotion stops. At most one pass per package
// name is needed, so an unknown initializer cycle also has a finite bound.
func (scope *stateScope) resolvePackageStates(values []*ast.ValueSpec) {
	for range len(scope.names) {
		progress := false
		for _, value := range values {
			for index, name := range value.Names {
				if scope.names[name.Name] {
					continue
				}
				if !scope.declaredState(value, index) {
					continue
				}
				scope.names[name.Name] = true
				progress = true
			}
		}
		if !progress {
			return
		}
	}
}

// stateAssignmentWalk visits a scope in source order. A child visitor owns a
// child scope, so leaving a block restores the enclosing bindings.
type stateAssignmentWalk struct {
	scan  *fileScan
	scope *stateScope
	fn    string
	// body shares the function's parameter and named-result scope.
	body *ast.BlockStmt
}

// nested creates the scope of a block or a statement's initializer.
func (w *stateAssignmentWalk) nested() *stateAssignmentWalk {
	child := *w
	child.scope = &stateScope{parent: w.scope, names: map[string]bool{}}
	return &child
}

// Visit follows lexical scopes and judges each assignment before declaring
// its new names. Function bodies share their parameters' scope in Go.
func (w *stateAssignmentWalk) Visit(node ast.Node) ast.Visitor {
	switch typed := node.(type) {
	case nil:
		return nil
	case *ast.FuncDecl:
		child := w.nested()
		child.fn = funcName(typed)
		child.body = typed.Body
		child.scope.fields(typed.Recv)
		child.scope.fields(typed.Type.Params)
		child.scope.fields(typed.Type.Results)
		return child
	case *ast.FuncLit:
		child := w.nested()
		if w.fn != packageScope {
			child.fn += ".func"
		}
		child.body = typed.Body
		child.scope.fields(typed.Type.Params)
		child.scope.fields(typed.Type.Results)
		return child
	case *ast.BlockStmt:
		if typed == w.body {
			return w
		}
		return w.nested()
	case *ast.IfStmt, *ast.ForStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt,
		*ast.CaseClause, *ast.CommClause:
		return w.nested()
	case *ast.RangeStmt:
		// Range bindings are not visible in the range expression itself.
		ast.Walk(w, typed.X)
		child := w.nested()
		for _, target := range []ast.Expr{typed.Key, typed.Value} {
			if target == nil {
				continue
			}
			if typed.Tok == token.DEFINE {
				if name, isName := target.(*ast.Ident); isName {
					child.scope.names[name.Name] = false
				}
				continue
			}
			ast.Walk(w, target)
		}
		ast.Walk(child, typed.Body)
		return nil
	case *ast.ValueSpec:
		if w.scope.parent != nil {
			for _, value := range typed.Values {
				ast.Walk(w, value)
			}
			w.scope.declare(typed)
			return nil
		}
	case *ast.TypeSpec:
		w.scope.names[typed.Name.Name] = false
	case *ast.AssignStmt:
		for _, target := range typed.Lhs {
			ast.Walk(w, target)
		}
		for _, value := range typed.Rhs {
			ast.Walk(w, value)
		}
		w.assignment(typed)
		return nil
	}
	return w
}

// assignment pairs each right-hand value with its own target. A call that
// answers several values carries no result-type information in this AST.
func (w *stateAssignmentWalk) assignment(assign *ast.AssignStmt) {
	paired := len(assign.Lhs) == len(assign.Rhs)
	if paired {
		for index, target := range assign.Lhs {
			if !isBoolLiteral(assign.Rhs[index]) {
				continue
			}
			if _, bound := w.scope.lookup(finalName(assign.Rhs[index])); bound {
				continue
			}
			field, isField := ast.Unparen(target).(*ast.SelectorExpr)
			if !isField {
				continue
			}
			if w.scope.stateValue(field.X) {
				w.scan.add(RuleLiteralState, w.fn,
					"a literal boolean is written into a state member. Read the state or answer Unknown with the reason", assign)
			}
		}
	}
	if assign.Tok != token.DEFINE {
		return
	}
	// Compute every RHS before introducing any name: a parallel declaration
	// can read an outer name that its own LHS will shadow.
	facts := make([]bool, len(assign.Lhs))
	if paired {
		for index, value := range assign.Rhs {
			facts[index] = w.scope.stateValue(value)
		}
	}
	for index, target := range assign.Lhs {
		name, isName := target.(*ast.Ident)
		if !isName {
			continue
		}
		if _, exists := w.scope.names[name.Name]; exists {
			continue
		}
		w.scope.names[name.Name] = facts[index]
	}
}

// declare records a var or const declaration, including unknown bindings.
func (scope *stateScope) declare(value *ast.ValueSpec) {
	facts := make([]bool, len(value.Names))
	for index := range value.Names {
		facts[index] = scope.declaredState(value, index)
	}
	for index, name := range value.Names {
		scope.names[name.Name] = facts[index]
	}
}

// declaredState answers one declaration's type fact from its explicit type
// or its corresponding initializer. Multi-result calls remain unknown.
func (scope *stateScope) declaredState(value *ast.ValueSpec, index int) bool {
	if value.Type != nil {
		return stateNamedType(value.Type)
	}
	if len(value.Values) != len(value.Names) {
		return false
	}
	return scope.stateValue(value.Values[index])
}

// fields records receivers, parameters and named results.
func (scope *stateScope) fields(fields *ast.FieldList) {
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		for _, name := range field.Names {
			scope.names[name.Name] = stateNamedType(field.Type)
		}
	}
}

// lookup answers a visible binding and whether the name is bound at all.
// The loop is bounded by the enclosing scopes in this parsed file.
func (scope *stateScope) lookup(name string) (bool, bool) {
	for current := scope; current != nil; current = current.parent {
		if state, exists := current.names[name]; exists {
			return state, true
		}
	}
	return false, false
}

// stateValue recognizes direct construction, builtin new, pointer operations
// and copies of known values. Each step removes one expression node, so the
// loop is bounded by this expression's syntax. Other call results and member
// types remain unknown rather than being guessed from their names.
func (scope *stateScope) stateValue(expr ast.Expr) bool {
	for expr != nil {
		switch typed := ast.Unparen(expr).(type) {
		case *ast.Ident:
			state, _ := scope.lookup(typed.Name)
			return state
		case *ast.CompositeLit:
			return isStateType(typed.Type)
		case *ast.UnaryExpr:
			if typed.Op != token.AND {
				return false
			}
			expr = typed.X
		case *ast.StarExpr:
			expr = typed.X
		case *ast.CallExpr:
			if !isIdentNamed(typed.Fun, "new") {
				return false
			}
			if _, bound := scope.lookup("new"); bound {
				return false
			}
			if len(typed.Args) != 1 {
				return false
			}
			return stateNamedType(typed.Args[0])
		default:
			return false
		}
	}
	return false
}

// stateNamedType applies the existing State, Status and Health convention to
// a declared type, through any pointer layers in that type's syntax.
func stateNamedType(expr ast.Expr) bool {
	for expr != nil {
		pointer, isPointer := ast.Unparen(expr).(*ast.StarExpr)
		if !isPointer {
			return isStateType(expr)
		}
		expr = pointer.X
	}
	return false
}
