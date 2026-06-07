package mutation

import (
	"fmt"
	"go/ast"
	"go/token"
)

const (
	bitwiseMutatorName = "bitwise"
	bitwiseBinaryType  = "bitwise_binary"
	bitwiseAssignType  = "bitwise_assign"
)

// BitwiseMutator mutates bitwise operators.
type BitwiseMutator struct {
}

// Name returns the name of the mutator.
func (m *BitwiseMutator) Name() string {
	return bitwiseMutatorName
}

// CanMutate returns true if the node can be mutated by this mutator.
func (m *BitwiseMutator) CanMutate(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.BinaryExpr:
		return m.isBitwiseOperator(n.Op)
	case *ast.AssignStmt:
		return m.isBitwiseAssignOperator(n.Tok)
	}

	return false
}

// Mutate generates mutants for the given node.
func (m *BitwiseMutator) Mutate(node ast.Node, fset *token.FileSet) []Mutant {
	var mutants []Mutant

	pos := fset.Position(node.Pos())

	switch n := node.(type) {
	case *ast.BinaryExpr:
		mutants = append(mutants, m.mutateBinaryExpr(n, pos)...)
	case *ast.AssignStmt:
		mutants = append(mutants, m.mutateAssignStmt(n, pos)...)
	}

	return mutants
}

func (m *BitwiseMutator) mutateBinaryExpr(expr *ast.BinaryExpr, pos token.Position) []Mutant {
	mutations := m.getBitwiseMutations(expr.Op)

	mutants := make([]Mutant, 0, len(mutations))

	for _, newOp := range mutations {
		mutants = append(mutants, Mutant{
			Line:        pos.Line,
			Column:      pos.Column,
			Type:        bitwiseBinaryType,
			Original:    expr.Op.String(),
			Mutated:     newOp.String(),
			Description: fmt.Sprintf("Replace %s with %s", expr.Op.String(), newOp.String()),
		})
	}

	return mutants
}

func (m *BitwiseMutator) mutateAssignStmt(stmt *ast.AssignStmt, pos token.Position) []Mutant {
	op := stmt.Tok
	mutations := m.getBitwiseAssignMutations(op)

	mutants := make([]Mutant, 0, len(mutations))

	for _, newOp := range mutations {
		mutants = append(mutants, Mutant{
			Line:        pos.Line,
			Column:      pos.Column,
			Type:        bitwiseAssignType,
			Original:    op.String(),
			Mutated:     newOp.String(),
			Description: fmt.Sprintf("Replace %s with %s", op.String(), newOp.String()),
		})
	}

	return mutants
}

func (m *BitwiseMutator) isBitwiseOperator(op token.Token) bool {
	switch op {
	case token.AND, token.OR, token.XOR, token.AND_NOT, token.SHL, token.SHR:
		return true
	default:
		return false
	}
}

func (m *BitwiseMutator) isBitwiseAssignOperator(op token.Token) bool {
	switch op {
	case token.AND_ASSIGN, token.OR_ASSIGN, token.XOR_ASSIGN, token.SHL_ASSIGN, token.SHR_ASSIGN:
		return true
	default:
		return false
	}
}

func (m *BitwiseMutator) getBitwiseMutations(op token.Token) []token.Token {
	switch op {
	case token.AND: // &
		return []token.Token{token.OR, token.XOR, token.AND_NOT}
	case token.OR: // |
		return []token.Token{token.AND, token.XOR, token.AND_NOT}
	case token.XOR: // ^
		return []token.Token{token.AND, token.OR, token.AND_NOT}
	case token.AND_NOT: // &^
		return []token.Token{token.AND, token.OR, token.XOR}
	case token.SHL: // <<
		return []token.Token{token.SHR}
	case token.SHR: // >>
		return []token.Token{token.SHL}
	default:
		return nil
	}
}

func (m *BitwiseMutator) getBitwiseAssignMutations(op token.Token) []token.Token {
	switch op {
	case token.AND_ASSIGN: // &=
		return []token.Token{token.OR_ASSIGN, token.XOR_ASSIGN}
	case token.OR_ASSIGN: // |=
		return []token.Token{token.AND_ASSIGN, token.XOR_ASSIGN}
	case token.XOR_ASSIGN: // ^=
		return []token.Token{token.AND_ASSIGN, token.OR_ASSIGN}
	case token.SHL_ASSIGN: // <<=
		return []token.Token{token.SHR_ASSIGN}
	case token.SHR_ASSIGN: // >>=
		return []token.Token{token.SHL_ASSIGN}
	default:
		return nil
	}
}

// Apply applies the mutation to the given AST node.
func (m *BitwiseMutator) Apply(node ast.Node, mutant Mutant) bool {
	switch mutant.Type {
	case bitwiseBinaryType:
		return m.applyBinary(node, mutant)
	case bitwiseAssignType:
		return m.applyAssign(node, mutant)
	}

	return false
}

// applyBinary applies binary operator mutation.
func (m *BitwiseMutator) applyBinary(node ast.Node, mutant Mutant) bool {
	if expr, ok := node.(*ast.BinaryExpr); ok {
		// Verify the original operator matches
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

// applyAssign applies assignment operator mutation.
func (m *BitwiseMutator) applyAssign(node ast.Node, mutant Mutant) bool {
	if stmt, ok := node.(*ast.AssignStmt); ok {
		// Verify the original operator matches
		if stmt.Tok.String() != mutant.Original {
			return false
		}

		newOp := stringToToken(mutant.Mutated)
		if newOp != token.ILLEGAL {
			stmt.Tok = newOp

			return true
		}
	}

	return false
}
