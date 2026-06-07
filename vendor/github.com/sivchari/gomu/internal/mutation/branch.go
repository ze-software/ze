package mutation

import (
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"strings"
)

const (
	branchMutatorName   = "branch"
	branchConditionType = "branch_condition"
)

// BranchMutator mutates branch conditions in if statements.
type BranchMutator struct {
}

// Name returns the name of the mutator.
func (m *BranchMutator) Name() string {
	return branchMutatorName
}

// CanMutate returns true if the node can be mutated by this mutator.
func (m *BranchMutator) CanMutate(node ast.Node) bool {
	stmt, ok := node.(*ast.IfStmt)
	if !ok {
		return false
	}

	return !isBoolLiteral(stmt.Cond)
}

// Mutate generates mutants for the given node.
func (m *BranchMutator) Mutate(node ast.Node, fset *token.FileSet) []Mutant {
	stmt, ok := node.(*ast.IfStmt)
	if !ok {
		return nil
	}

	pos := fset.Position(stmt.Pos())
	original := exprToString(stmt.Cond)

	return []Mutant{
		{
			Line:        pos.Line,
			Column:      pos.Column,
			Type:        branchConditionType,
			Original:    original,
			Mutated:     boolTrue,
			Description: fmt.Sprintf("Replace branch condition %q with true", original),
		},
		{
			Line:        pos.Line,
			Column:      pos.Column,
			Type:        branchConditionType,
			Original:    original,
			Mutated:     boolFalse,
			Description: fmt.Sprintf("Replace branch condition %q with false", original),
		},
	}
}

// Apply applies the mutation to the given AST node.
func (m *BranchMutator) Apply(node ast.Node, mutant Mutant) bool {
	stmt, ok := node.(*ast.IfStmt)
	if !ok {
		return false
	}

	if mutant.Type != branchConditionType {
		return false
	}

	stmt.Cond = &ast.Ident{Name: mutant.Mutated}

	return true
}

// isBoolLiteral reports whether expr is the identifier true or false.
func isBoolLiteral(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)

	return ok && (ident.Name == boolTrue || ident.Name == boolFalse)
}

// exprToString returns a string representation of an AST expression.
func exprToString(expr ast.Expr) string {
	var buf strings.Builder

	printer.Fprint(&buf, token.NewFileSet(), expr) //nolint:errcheck

	return buf.String()
}
