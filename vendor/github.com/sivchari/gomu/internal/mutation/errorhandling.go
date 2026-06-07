package mutation

import (
	"fmt"
	"go/ast"
	"go/token"
)

const (
	errorHandlingMutatorName = "error_handling"
	errorNilifyType          = "error_nilify"
	errIdentName             = "err"
	nilIdentName             = "nil"
)

// ErrorHandlingMutator mutates error return values by replacing err with nil.
type ErrorHandlingMutator struct {
}

// Name returns the name of the mutator.
func (m *ErrorHandlingMutator) Name() string {
	return errorHandlingMutatorName
}

// CanMutate returns true if the node is a return statement containing an err identifier.
func (m *ErrorHandlingMutator) CanMutate(node ast.Node) bool {
	stmt, ok := node.(*ast.ReturnStmt)
	if !ok {
		return false
	}

	for _, expr := range stmt.Results {
		if isErrIdent(expr) {
			return true
		}
	}

	return false
}

// Mutate generates mutants for the given node.
func (m *ErrorHandlingMutator) Mutate(node ast.Node, fset *token.FileSet) []Mutant {
	stmt, ok := node.(*ast.ReturnStmt)
	if !ok {
		return nil
	}

	pos := fset.Position(stmt.Pos())
	mutants := make([]Mutant, 0, len(stmt.Results))

	for _, expr := range stmt.Results {
		ident, ok := expr.(*ast.Ident)
		if !ok || ident.Name != errIdentName {
			continue
		}

		mutants = append(mutants, Mutant{
			Line:        pos.Line,
			Column:      pos.Column,
			Type:        errorNilifyType,
			Original:    ident.Name,
			Mutated:     nilIdentName,
			Description: fmt.Sprintf("Replace return %s with return nil", ident.Name),
		})
	}

	return mutants
}

// Apply applies the mutation to the given AST node.
func (m *ErrorHandlingMutator) Apply(node ast.Node, mutant Mutant) bool {
	stmt, ok := node.(*ast.ReturnStmt)
	if !ok {
		return false
	}

	if mutant.Type != errorNilifyType {
		return false
	}

	for i, expr := range stmt.Results {
		ident, ok := expr.(*ast.Ident)
		if !ok {
			continue
		}

		if ident.Name != mutant.Original {
			continue
		}

		stmt.Results[i] = &ast.Ident{Name: nilIdentName}

		return true
	}

	return false
}

// isErrIdent reports whether expr is an identifier named "err".
func isErrIdent(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)

	return ok && ident.Name == errIdentName
}
