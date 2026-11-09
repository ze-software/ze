// Design: docs/architecture/plugin/feature-gates.md -- range header stores remain retained assignments.
package repoinventory

import (
	"go/ast"
	"go/token"
)

// externalRangeTargets returns only actual stores. DEFINE creates fresh
// per-iteration bindings, and the blank identifier never stores a capability.
func externalRangeTargets(statement *ast.RangeStmt) []ast.Expr {
	if statement.Tok != token.ASSIGN {
		return nil
	}
	var targets []ast.Expr
	for _, target := range []ast.Expr{statement.Key, statement.Value} {
		if target == nil {
			continue
		}
		if name, identifier := target.(*ast.Ident); identifier {
			if name.Name == "_" {
				continue
			}
		}
		targets = append(targets, target)
	}
	return targets
}

// externalRangeValue is a private source equation, not a source AST mutation.
// Its RANGE token requires an explicit unsupported transfer when a retained
// target is read; a type-derived empty field cannot erase that uncertainty.
func externalRangeValue(statement *ast.RangeStmt, target ast.Expr) ast.Expr {
	return &ast.UnaryExpr{Op: token.RANGE, OpPos: target.Pos(), X: statement.X}
}

func (e *externalEffects) indexRangeAssignment(statement *ast.RangeStmt, scope sourceExpr, owner ast.Node) {
	if statement.Tok == token.DEFINE {
		// A fresh range variable has no definition to index: its source is
		// its collection's elements, which formal provenance reaches here.
		for _, target := range []ast.Expr{statement.Key, statement.Value} {
			if name, identifier := target.(*ast.Ident); identifier {
				if name.Obj != nil {
					e.rangeVariables[name.Obj] = sourceExpr{value: name, file: scope.file, pkg: scope.pkg}
				}
			}
		}
	}
	for _, target := range externalRangeTargets(statement) {
		value := externalRangeValue(statement, target)
		source := sourceExpr{value: target, file: scope.file, pkg: scope.pkg}
		object := e.object(source)
		if object == nil {
			e.stores = append(e.stores, externalStore{target: source, values: []ast.Expr{value}})
			continue
		}
		e.definitions[object] = append(e.definitions[object], externalDefinition{
			source: sourceExpr{value: value, file: scope.file, pkg: scope.pkg},
			owner:  owner,
		})
	}
}

func (e *externalEffects) rangeAssignment(statement *ast.RangeStmt, scope sourceExpr, possible bool) {
	targets := externalRangeTargets(statement)
	for _, target := range targets {
		assignment := &ast.AssignStmt{
			Lhs:    []ast.Expr{target},
			Tok:    token.ASSIGN,
			TokPos: statement.TokPos,
			Rhs:    []ast.Expr{externalRangeValue(statement, target)},
		}
		e.assignment(assignment, scope, possible)
	}
}

func externalRangeStore(store externalStore) bool {
	if len(store.values) != 1 {
		return false
	}
	value, unary := store.values[0].(*ast.UnaryExpr)
	if !unary {
		return false
	}
	return value.Op == token.RANGE
}

func (e *externalEffects) storeLocationProblem(store externalStore, detail string) {
	if externalRangeStore(store) {
		e.problem(store.target, "range assignment location ownership is unresolved"+detail)
		return
	}
	e.problem(store.target, "mutation location ownership is unresolved"+detail)
}
