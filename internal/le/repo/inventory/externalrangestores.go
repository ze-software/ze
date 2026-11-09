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

// externalRangeValue is a private source equation, not a source AST mutation:
// the value a range header stores into an existing target, which
// rangeAssignedValue evaluates.
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

// rangeAssignedValue is the value a range header with = stores into an
// existing key or value target. Go specification, "For statements with range
// clause": "If the iteration variables are not explicitly declared by the
// "range" clause, they must be preexisting. In this case, the iteration
// values are assigned to the respective variables as in an assignment
// statement." For an array, slice, string or map
// those values are indexes, keys and elements of the collection, so the
// stored value is the collection's flattened contents, as an index read
// answers, and a field read of it refuses for want of an element summary. A
// function iterator yields what its body chooses and a channel what is sent
// on it, neither of which is the collection value, so those refuse, and so
// does a collection whose type is not resolved.
func (e *externalEffects) rangeAssignedValue(source sourceExpr, ranged *ast.UnaryExpr, possible bool, depth int) externalValue {
	collection := sourceExpr{value: ranged.X, file: source.file, pkg: source.pkg}
	datatype, known := e.source.snapshot.evaluator().valueType(collection, 0)
	if !known {
		const reason = "range assignment capability ownership is unsupported"
		e.problem(source, reason)
		return externalValue{unknown: reason}
	}
	if reason := externalRangeCollectionShape(datatype); reason != "" {
		e.problem(source, reason)
		return externalValue{unknown: reason}
	}
	result := externalFlat(e.value(collection, possible, depth+1), 0)
	result.fields = nil
	result.unresolvedFields = "ranged element field ownership requires a retained element summary"
	return result
}

// externalRangeCollectionShape answers the refusal for a ranged collection
// whose iteration values are not its contents, or "" when they are.
func externalRangeCollectionShape(datatype sourceExpr) string {
	switch ast.Unparen(datatype.value).(type) {
	case *ast.FuncType:
		return "range callback capability transfer is unsupported"
	case *ast.ChanType:
		return "range channel capability transfer is unsupported"
	}
	return ""
}
