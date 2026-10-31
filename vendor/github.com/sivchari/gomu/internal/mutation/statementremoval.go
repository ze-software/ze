package mutation

import (
	"go/ast"
	"go/printer"
	"go/token"
	"strings"
)

const (
	statementRemovalMutatorName = "statement_removal"
	statementRemovalType        = "statement_removal"

	statementRemovalMutated = "<removed>"
)

// StatementRemovalMutator removes standalone statements that carry a side
// effect, testing whether that side effect is covered by tests.
//
// To avoid generating duplicate mutants, assignment statements and expression
// statements are handled by the dedicated assignment_removal and
// expression_removal mutators; this mutator covers the remaining removable
// statement kinds: increment/decrement, defer, go, and channel send.
type StatementRemovalMutator struct {
}

// Name returns the name of the mutator.
func (m *StatementRemovalMutator) Name() string {
	return statementRemovalMutatorName
}

// Description returns a human-readable summary of the mutator.
func (m *StatementRemovalMutator) Description() string {
	return "Remove statements (inc/dec, defer, go, channel send)"
}

// CanMutate returns true if the node can be mutated by this mutator.
func (m *StatementRemovalMutator) CanMutate(node ast.Node) bool {
	return isRemovableStatement(node)
}

// Mutate generates mutants for the given node.
func (m *StatementRemovalMutator) Mutate(node ast.Node, fset *token.FileSet) []Mutant {
	if !isRemovableStatement(node) {
		return nil
	}

	pos := fset.Position(node.Pos())

	return []Mutant{
		{
			Line:        pos.Line,
			Column:      pos.Column,
			Type:        statementRemovalType,
			Original:    stmtToString(node),
			Mutated:     statementRemovalMutated,
			Description: "Remove statement",
		},
	}
}

// Apply applies the mutation to the given AST node.
// Statement removal requires node replacement via the cursor, so the plain
// Apply path always reports failure.
func (m *StatementRemovalMutator) Apply(_ ast.Node, _ Mutant) bool {
	return false
}

// ApplyWithCursor removes the statement by replacing it with an empty statement.
func (m *StatementRemovalMutator) ApplyWithCursor(node ast.Node, replaceFunc func(ast.Node), mutant Mutant) bool {
	if mutant.Type != statementRemovalType {
		return false
	}

	if !isRemovableStatement(node) {
		return false
	}

	replaceFunc(&ast.EmptyStmt{})

	return true
}

// isRemovableStatement reports whether node is a statement kind handled by this
// mutator. AssignStmt and ExprStmt are intentionally excluded (handled by the
// assignment_removal and expression_removal mutators respectively).
func isRemovableStatement(node ast.Node) bool {
	switch node.(type) {
	case *ast.IncDecStmt, *ast.DeferStmt, *ast.GoStmt, *ast.SendStmt:
		return true
	default:
		return false
	}
}

// stmtToString renders a statement node to its source representation.
func stmtToString(node ast.Node) string {
	var buf strings.Builder

	printer.Fprint(&buf, token.NewFileSet(), node) //nolint:errcheck

	return buf.String()
}
