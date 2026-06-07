package mutation

import (
	"fmt"
	"go/ast"
	"go/token"
)

const (
	logicalMutatorName    = "logical"
	logicalBinaryType     = "logical_binary"
	logicalNotRemovalType = "logical_not_removal"
)

// LogicalMutator mutates logical operators.
type LogicalMutator struct {
}

// Name returns the name of the mutator.
func (m *LogicalMutator) Name() string {
	return logicalMutatorName
}

// CanMutate returns true if the node can be mutated by this mutator.
func (m *LogicalMutator) CanMutate(node ast.Node) bool {
	if n, ok := node.(*ast.BinaryExpr); ok {
		return m.isLogicalOp(n.Op)
	}

	if n, ok := node.(*ast.UnaryExpr); ok {
		return n.Op == token.NOT
	}

	return false
}

// Mutate generates mutants for the given node.
func (m *LogicalMutator) Mutate(node ast.Node, fset *token.FileSet) []Mutant {
	pos := fset.Position(node.Pos())

	if n, ok := node.(*ast.BinaryExpr); ok {
		return m.mutateBinaryExpr(n, pos)
	}

	if n, ok := node.(*ast.UnaryExpr); ok {
		return m.mutateUnaryExpr(n, pos)
	}

	return nil
}

func (m *LogicalMutator) mutateUnaryExpr(expr *ast.UnaryExpr, pos token.Position) []Mutant {
	return []Mutant{
		{
			Line:        pos.Line,
			Column:      pos.Column,
			Type:        logicalNotRemovalType,
			Original:    expr.Op.String(),
			Mutated:     "",
			Description: fmt.Sprintf("Remove %s operator", expr.Op.String()),
		},
	}
}

func (m *LogicalMutator) mutateBinaryExpr(expr *ast.BinaryExpr, pos token.Position) []Mutant {
	mutations := m.getLogicalMutations(expr.Op)

	mutants := make([]Mutant, 0, len(mutations))

	for _, newOp := range mutations {
		mutants = append(mutants, Mutant{
			Line:        pos.Line,
			Column:      pos.Column,
			Type:        logicalBinaryType,
			Original:    expr.Op.String(),
			Mutated:     newOp.String(),
			Description: fmt.Sprintf("Replace %s with %s", expr.Op.String(), newOp.String()),
		})
	}

	return mutants
}

func (m *LogicalMutator) isLogicalOp(op token.Token) bool {
	switch op {
	case token.LAND, token.LOR:
		return true
	default:
		return false
	}
}

func (m *LogicalMutator) getLogicalMutations(op token.Token) []token.Token {
	switch op {
	case token.LAND: // &&
		return []token.Token{token.LOR}
	case token.LOR: // ||
		return []token.Token{token.LAND}
	default:
		return nil
	}
}

// Apply applies the mutation to the given AST node.
func (m *LogicalMutator) Apply(node ast.Node, mutant Mutant) bool {
	if mutant.Type == logicalBinaryType {
		return m.applyBinary(node, mutant)
	}

	if mutant.Type == logicalNotRemovalType {
		// NOT removal requires node replacement via CursorApplier; not supported here.
		return false
	}

	return false
}

// ApplyWithCursor applies the NOT removal mutation using a replace function for node replacement.
func (m *LogicalMutator) ApplyWithCursor(node ast.Node, replaceFunc func(ast.Node), mutant Mutant) bool {
	if mutant.Type != logicalNotRemovalType {
		return false
	}

	expr, ok := node.(*ast.UnaryExpr)
	if !ok || expr.Op != token.NOT {
		return false
	}

	replaceFunc(expr.X)

	return true
}

// applyBinary applies binary operator mutation.
func (m *LogicalMutator) applyBinary(node ast.Node, mutant Mutant) bool {
	if expr, ok := node.(*ast.BinaryExpr); ok {
		if expr.Op.String() != mutant.Original {
			return false
		}

		newOp := stringToToken(mutant.Mutated)
		if newOp != token.ILLEGAL {
			expr.Op = newOp

			return true
		}
	}

	return false
}
