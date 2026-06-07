package mutation

import (
	"fmt"
	"go/ast"
	"go/token"
)

const (
	returnMutatorName     = "return"
	returnBoolLiteralType = "return_bool_literal"
	returnZeroValueType   = "return_zero_value"
	boolTrue              = "true"
	boolFalse             = "false"
)

// ReturnMutator mutates return statement values.
type ReturnMutator struct {
}

// Name returns the name of the mutator.
func (m *ReturnMutator) Name() string {
	return returnMutatorName
}

// CanMutate returns true if the node can be mutated by this mutator.
func (m *ReturnMutator) CanMutate(node ast.Node) bool {
	stmt, ok := node.(*ast.ReturnStmt)
	if !ok {
		return false
	}

	for _, expr := range stmt.Results {
		if m.isMutableExpr(expr) {
			return true
		}
	}

	return false
}

func (m *ReturnMutator) isMutableExpr(expr ast.Expr) bool {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name == boolTrue || ident.Name == boolFalse
	}

	if lit, ok := expr.(*ast.BasicLit); ok {
		return lit.Kind == token.INT || lit.Kind == token.FLOAT || lit.Kind == token.STRING
	}

	return false
}

// Mutate generates mutants for the given node.
func (m *ReturnMutator) Mutate(node ast.Node, fset *token.FileSet) []Mutant {
	stmt, ok := node.(*ast.ReturnStmt)
	if !ok {
		return nil
	}

	pos := fset.Position(stmt.Pos())

	var mutants []Mutant

	for _, expr := range stmt.Results {
		mutants = append(mutants, m.mutateExpr(expr, pos)...)
	}

	return mutants
}

func (m *ReturnMutator) mutateExpr(expr ast.Expr, pos token.Position) []Mutant {
	if ident, ok := expr.(*ast.Ident); ok {
		return m.mutateBoolIdent(ident, pos)
	}

	if lit, ok := expr.(*ast.BasicLit); ok {
		return m.mutateBasicLit(lit, pos)
	}

	return nil
}

func (m *ReturnMutator) mutateBoolIdent(ident *ast.Ident, pos token.Position) []Mutant {
	var mutated string

	if ident.Name == boolTrue {
		mutated = boolFalse
	} else {
		mutated = boolTrue
	}

	return []Mutant{{
		Line:        pos.Line,
		Column:      pos.Column,
		Type:        returnBoolLiteralType,
		Original:    ident.Name,
		Mutated:     mutated,
		Description: fmt.Sprintf("Replace return %s with return %s", ident.Name, mutated),
	}}
}

func (m *ReturnMutator) mutateBasicLit(lit *ast.BasicLit, pos token.Position) []Mutant {
	var mutated string

	switch lit.Kind {
	case token.INT, token.FLOAT:
		mutated = "0"
	case token.STRING:
		mutated = `""`
	default:
		return nil
	}

	return []Mutant{{
		Line:        pos.Line,
		Column:      pos.Column,
		Type:        returnZeroValueType,
		Original:    lit.Value,
		Mutated:     mutated,
		Description: fmt.Sprintf("Replace return %s with return %s", lit.Value, mutated),
	}}
}

// Apply applies the mutation to the given AST node.
func (m *ReturnMutator) Apply(node ast.Node, mutant Mutant) bool {
	stmt, ok := node.(*ast.ReturnStmt)
	if !ok {
		return false
	}

	for _, expr := range stmt.Results {
		if m.applyToExpr(expr, mutant) {
			return true
		}
	}

	return false
}

func (m *ReturnMutator) applyToExpr(expr ast.Expr, mutant Mutant) bool {
	switch mutant.Type {
	case returnBoolLiteralType:
		return m.applyBoolIdent(expr, mutant)
	case returnZeroValueType:
		return m.applyZeroValue(expr, mutant)
	}

	return false
}

func (m *ReturnMutator) applyBoolIdent(expr ast.Expr, mutant Mutant) bool {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}

	if ident.Name != mutant.Original {
		return false
	}

	ident.Name = mutant.Mutated

	return true
}

func (m *ReturnMutator) applyZeroValue(expr ast.Expr, mutant Mutant) bool {
	lit, ok := expr.(*ast.BasicLit)
	if !ok {
		return false
	}

	if lit.Value != mutant.Original {
		return false
	}

	lit.Value = mutant.Mutated

	return true
}
