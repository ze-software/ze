package mutation

import (
	"go/ast"
	"go/token"
)

const (
	assignmentRemovalMutatorName = "assignment_removal"
	assignmentRemovalType        = "assignment_removal"

	assignmentRemovalMutated = "<removed>"
)

// AssignmentRemovalMutator removes assignment statements, testing whether the
// effect of the assignment is properly covered by tests.
//
// Short variable declarations (`:=`) are not targeted because removing them
// leaves later references undefined, which never compiles.
type AssignmentRemovalMutator struct {
}

// Name returns the name of the mutator.
func (m *AssignmentRemovalMutator) Name() string {
	return assignmentRemovalMutatorName
}

// Description returns a human-readable summary of the mutator.
func (m *AssignmentRemovalMutator) Description() string {
	return "Remove assignment statements"
}

// CanMutate returns true if the node can be mutated by this mutator.
func (m *AssignmentRemovalMutator) CanMutate(node ast.Node) bool {
	stmt, ok := node.(*ast.AssignStmt)

	return ok && stmt.Tok != token.DEFINE
}

// Mutate generates mutants for the given node.
func (m *AssignmentRemovalMutator) Mutate(node ast.Node, fset *token.FileSet) []Mutant {
	stmt, ok := node.(*ast.AssignStmt)
	if !ok || stmt.Tok == token.DEFINE {
		return nil
	}

	pos := fset.Position(stmt.Pos())

	return []Mutant{
		{
			Line:        pos.Line,
			Column:      pos.Column,
			Type:        assignmentRemovalType,
			Original:    exprToString(stmt.Lhs[0]),
			Mutated:     assignmentRemovalMutated,
			Description: "Remove assignment statement",
		},
	}
}

// Apply applies the mutation to the given AST node.
// Assignment removal requires node replacement via the cursor, so the plain
// Apply path always reports failure.
func (m *AssignmentRemovalMutator) Apply(_ ast.Node, _ Mutant) bool {
	return false
}

// ApplyWithCursor removes the assignment statement by replacing it with an
// empty statement.
func (m *AssignmentRemovalMutator) ApplyWithCursor(node ast.Node, replaceFunc func(ast.Node), mutant Mutant) bool {
	if mutant.Type != assignmentRemovalType {
		return false
	}

	stmt, ok := node.(*ast.AssignStmt)
	if !ok || stmt.Tok == token.DEFINE {
		return false
	}

	replaceFunc(&ast.EmptyStmt{})

	return true
}
