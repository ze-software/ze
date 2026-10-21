package mutation

import (
	"fmt"
	"go/ast"
	"go/token"
)

const (
	invertNegativesMutatorName = "invert_negatives"
	invertNegativesType        = "invert_negatives"
)

// InvertNegativesMutator mutates unary minus operators to unary plus.
type InvertNegativesMutator struct {
}

// Name returns the name of the mutator.
func (m *InvertNegativesMutator) Name() string {
	return invertNegativesMutatorName
}

// Description returns a human-readable summary of the mutator.
func (m *InvertNegativesMutator) Description() string {
	return "Replace unary minus with unary plus"
}

// CanMutate returns true if the node can be mutated by this mutator.
func (m *InvertNegativesMutator) CanMutate(node ast.Node) bool {
	if n, ok := node.(*ast.UnaryExpr); ok {
		return n.Op == token.SUB
	}

	return false
}

// Mutate generates mutants for the given node.
func (m *InvertNegativesMutator) Mutate(node ast.Node, fset *token.FileSet) []Mutant {
	expr, ok := node.(*ast.UnaryExpr)
	if !ok || expr.Op != token.SUB {
		return nil
	}

	pos := fset.Position(node.Pos())

	return []Mutant{
		{
			Line:        pos.Line,
			Column:      pos.Column,
			Type:        invertNegativesType,
			Original:    token.SUB.String(),
			Mutated:     token.ADD.String(),
			Description: fmt.Sprintf("Replace unary %s with unary %s", token.SUB.String(), token.ADD.String()),
		},
	}
}

// Apply applies the mutation to the given AST node.
func (m *InvertNegativesMutator) Apply(node ast.Node, mutant Mutant) bool {
	if mutant.Type != invertNegativesType {
		return false
	}

	expr, ok := node.(*ast.UnaryExpr)
	if !ok || expr.Op != token.SUB {
		return false
	}

	if expr.Op.String() != mutant.Original {
		return false
	}

	newOp := stringToToken(mutant.Mutated)
	if newOp == token.ILLEGAL {
		return false
	}

	expr.Op = newOp

	return true
}
