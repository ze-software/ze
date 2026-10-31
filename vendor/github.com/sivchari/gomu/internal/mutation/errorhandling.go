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

// ErrorHandlingMutator mutates error return values by replacing them with nil.
//
// Eligibility for a given identifier is decided by TypeChecker using static type
// information: any identifier whose type is the universe error interface qualifies,
// regardless of its name. Without type information, it falls back to matching the
// identifier name "err".
type ErrorHandlingMutator struct {
}

// Name returns the name of the mutator.
func (m *ErrorHandlingMutator) Name() string {
	return errorHandlingMutatorName
}

// Description returns a human-readable summary of the mutator.
func (m *ErrorHandlingMutator) Description() string {
	return "Replace a returned err with nil"
}

// CanMutate returns true if the node is a return statement containing a candidate identifier.
func (m *ErrorHandlingMutator) CanMutate(node ast.Node) bool {
	stmt, ok := node.(*ast.ReturnStmt)
	if !ok {
		return false
	}

	for _, expr := range stmt.Results {
		if isNilifyCandidate(expr) {
			return true
		}
	}

	return false
}

// Mutate generates mutants for the given node.
//
// It emits a candidate for every identifier result; TypeChecker.IsValidMutation
// performs the actual error-type check (or the name-based fallback).
func (m *ErrorHandlingMutator) Mutate(node ast.Node, fset *token.FileSet) []Mutant {
	stmt, ok := node.(*ast.ReturnStmt)
	if !ok {
		return nil
	}

	pos := fset.Position(stmt.Pos())
	mutants := make([]Mutant, 0, len(stmt.Results))

	for _, expr := range stmt.Results {
		ident, ok := expr.(*ast.Ident)
		if !ok || !isNilifyCandidate(ident) {
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

// isNilifyCandidate reports whether expr is an identifier that could plausibly hold an
// error value, excluding the predeclared identifiers that never do.
func isNilifyCandidate(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}

	switch ident.Name {
	case nilIdentName, "_", boolTrue, boolFalse, "iota":
		return false
	default:
		return true
	}
}
