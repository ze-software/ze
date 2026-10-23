package mutation

import (
	"go/ast"
	"go/token"
)

const (
	expressionRemovalMutatorName = "expression_removal"
	expressionRemovalType        = "expression_removal"

	expressionRemovalMutated = "<removed>"
)

// ExpressionRemovalMutator removes expression statements (e.g. function calls
// evaluated for their side effects), testing whether those side effects are
// properly covered by tests.
type ExpressionRemovalMutator struct {
}

// Name returns the name of the mutator.
func (m *ExpressionRemovalMutator) Name() string {
	return expressionRemovalMutatorName
}

// Description returns a human-readable summary of the mutator.
func (m *ExpressionRemovalMutator) Description() string {
	return "Remove expression statements"
}

// CanMutate returns true if the node can be mutated by this mutator.
func (m *ExpressionRemovalMutator) CanMutate(node ast.Node) bool {
	_, ok := node.(*ast.ExprStmt)

	return ok
}

// Mutate generates mutants for the given node.
func (m *ExpressionRemovalMutator) Mutate(node ast.Node, fset *token.FileSet) []Mutant {
	stmt, ok := node.(*ast.ExprStmt)
	if !ok {
		return nil
	}

	pos := fset.Position(stmt.Pos())

	return []Mutant{
		{
			Line:        pos.Line,
			Column:      pos.Column,
			Type:        expressionRemovalType,
			Original:    exprToString(stmt.X),
			Mutated:     expressionRemovalMutated,
			Description: "Remove expression statement",
		},
	}
}

// Apply applies the mutation to the given AST node.
// Expression removal requires node replacement via the cursor, so the plain
// Apply path always reports failure.
func (m *ExpressionRemovalMutator) Apply(_ ast.Node, _ Mutant) bool {
	return false
}

// ApplyWithCursor removes the expression statement by replacing it with an
// empty statement.
func (m *ExpressionRemovalMutator) ApplyWithCursor(node ast.Node, replaceFunc func(ast.Node), mutant Mutant) bool {
	if mutant.Type != expressionRemovalType {
		return false
	}

	if _, ok := node.(*ast.ExprStmt); !ok {
		return false
	}

	replaceFunc(&ast.EmptyStmt{})

	return true
}
