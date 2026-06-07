// Package mutation provides mutation testing functionality.
package mutation

import (
	"fmt"
	"go/ast"
	"go/token"
)

const (
	arithmeticMutatorName = "arithmetic"
	arithmeticBinaryType  = "arithmetic_binary"
	arithmeticAssignType  = "arithmetic_assign"
	arithmeticIncDecType  = "arithmetic_incdec"
)

// ArithmeticMutator mutates arithmetic operators.
type ArithmeticMutator struct {
}

// Name returns the name of the mutator.
func (m *ArithmeticMutator) Name() string {
	return arithmeticMutatorName
}

// CanMutate returns true if the node can be mutated by this mutator.
func (m *ArithmeticMutator) CanMutate(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.BinaryExpr:
		return m.isArithmeticOp(n.Op)
	case *ast.AssignStmt:
		return m.isArithmeticAssignOp(n.Tok)
	case *ast.IncDecStmt:
		return true
	}

	return false
}

func (m *ArithmeticMutator) isArithmeticOp(op token.Token) bool {
	switch op {
	case token.ADD, token.SUB, token.MUL, token.QUO, token.REM:
		return true
	default:
		return false
	}
}

func (m *ArithmeticMutator) isArithmeticAssignOp(op token.Token) bool {
	switch op {
	case token.ADD_ASSIGN, token.SUB_ASSIGN, token.MUL_ASSIGN, token.QUO_ASSIGN:
		return true
	default:
		return false
	}
}

// Mutate generates mutants for the given node.
func (m *ArithmeticMutator) Mutate(node ast.Node, fset *token.FileSet) []Mutant {
	var mutants []Mutant

	pos := fset.Position(node.Pos())

	switch n := node.(type) {
	case *ast.BinaryExpr:
		mutants = append(mutants, m.mutateBinaryExpr(n, pos)...)
	case *ast.AssignStmt:
		mutants = append(mutants, m.mutateAssignStmt(n, pos)...)
	case *ast.IncDecStmt:
		mutants = append(mutants, m.mutateIncDecStmt(n, pos)...)
	}

	return mutants
}

func (m *ArithmeticMutator) mutateBinaryExpr(expr *ast.BinaryExpr, pos token.Position) []Mutant {
	mutations := m.getArithmeticMutations(expr.Op)

	// Generate all mutations - validation will be done at compile time
	// No pre-filtering based on type safety

	mutants := make([]Mutant, 0, len(mutations))

	for _, newOp := range mutations {
		mutants = append(mutants, Mutant{
			Line:        pos.Line,
			Column:      pos.Column,
			Type:        arithmeticBinaryType,
			Original:    expr.Op.String(),
			Mutated:     newOp.String(),
			Description: fmt.Sprintf("Replace %s with %s", expr.Op.String(), newOp.String()),
		})
	}

	return mutants
}

func (m *ArithmeticMutator) mutateAssignStmt(stmt *ast.AssignStmt, pos token.Position) []Mutant {
	op := stmt.Tok
	mutations := m.getAssignMutations(op)

	// Generate all mutations - validation will be done at compile time
	// No pre-filtering based on type safety

	mutants := make([]Mutant, 0, len(mutations))

	for _, newOp := range mutations {
		mutants = append(mutants, Mutant{
			Line:        pos.Line,
			Column:      pos.Column,
			Type:        arithmeticAssignType,
			Original:    op.String(),
			Mutated:     newOp.String(),
			Description: fmt.Sprintf("Replace %s with %s", op.String(), newOp.String()),
		})
	}

	return mutants
}

func (m *ArithmeticMutator) mutateIncDecStmt(stmt *ast.IncDecStmt, pos token.Position) []Mutant {
	var newOp token.Token

	var desc string

	if stmt.Tok == token.INC {
		newOp = token.DEC
		desc = "Replace ++ with --"
	} else {
		newOp = token.INC
		desc = "Replace -- with ++"
	}

	return []Mutant{{
		Line:        pos.Line,
		Column:      pos.Column,
		Type:        arithmeticIncDecType,
		Original:    stmt.Tok.String(),
		Mutated:     newOp.String(),
		Description: desc,
	}}
}

func (m *ArithmeticMutator) getArithmeticMutations(op token.Token) []token.Token {
	switch op {
	case token.ADD:
		return []token.Token{token.SUB, token.MUL, token.QUO}
	case token.SUB:
		return []token.Token{token.ADD, token.MUL, token.QUO}
	case token.MUL:
		return []token.Token{token.ADD, token.SUB, token.QUO, token.REM}
	case token.QUO:
		return []token.Token{token.ADD, token.SUB, token.MUL, token.REM}
	case token.REM:
		return []token.Token{token.ADD, token.SUB, token.MUL, token.QUO}
	default:
		return nil
	}
}

func (m *ArithmeticMutator) getAssignMutations(op token.Token) []token.Token {
	switch op {
	case token.ADD_ASSIGN:
		return []token.Token{token.SUB_ASSIGN, token.MUL_ASSIGN, token.QUO_ASSIGN}
	case token.SUB_ASSIGN:
		return []token.Token{token.ADD_ASSIGN, token.MUL_ASSIGN, token.QUO_ASSIGN}
	case token.MUL_ASSIGN:
		return []token.Token{token.ADD_ASSIGN, token.SUB_ASSIGN, token.QUO_ASSIGN}
	case token.QUO_ASSIGN:
		return []token.Token{token.ADD_ASSIGN, token.SUB_ASSIGN, token.MUL_ASSIGN}
	default:
		return nil
	}
}

// Apply applies the mutation to the given AST node.
func (m *ArithmeticMutator) Apply(node ast.Node, mutant Mutant) bool {
	switch mutant.Type {
	case arithmeticBinaryType:
		return m.applyBinary(node, mutant)
	case arithmeticAssignType:
		return m.applyAssign(node, mutant)
	case arithmeticIncDecType:
		return m.applyIncDec(node, mutant)
	}

	return false
}

// applyBinary applies binary operator mutation.
func (m *ArithmeticMutator) applyBinary(node ast.Node, mutant Mutant) bool {
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
func (m *ArithmeticMutator) applyAssign(node ast.Node, mutant Mutant) bool {
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

// applyIncDec applies increment/decrement operator mutation.
func (m *ArithmeticMutator) applyIncDec(node ast.Node, mutant Mutant) bool {
	if stmt, ok := node.(*ast.IncDecStmt); ok {
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
