package mutation

import (
	"fmt"
	"go/ast"
	"go/token"
)

const (
	removeSelfAssignmentsMutatorName = "remove_self_assignments"
	removeSelfAssignmentsType        = "remove_self_assignments"
)

// RemoveSelfAssignmentsMutator mutates compound assignment operators
// (e.g. +=, -=, &^=) to a simple assignment (=).
type RemoveSelfAssignmentsMutator struct {
}

// Name returns the name of the mutator.
func (m *RemoveSelfAssignmentsMutator) Name() string {
	return removeSelfAssignmentsMutatorName
}

// Description returns a human-readable summary of the mutator.
func (m *RemoveSelfAssignmentsMutator) Description() string {
	return "Replace compound assignment (e.g. +=) with plain ="
}

// CanMutate returns true if the node can be mutated by this mutator.
func (m *RemoveSelfAssignmentsMutator) CanMutate(node ast.Node) bool {
	if n, ok := node.(*ast.AssignStmt); ok {
		return m.isCompoundAssignOp(n.Tok)
	}

	return false
}

// Mutate generates mutants for the given node.
func (m *RemoveSelfAssignmentsMutator) Mutate(node ast.Node, fset *token.FileSet) []Mutant {
	stmt, ok := node.(*ast.AssignStmt)
	if !ok || !m.isCompoundAssignOp(stmt.Tok) {
		return nil
	}

	pos := fset.Position(node.Pos())

	return []Mutant{
		{
			Line:        pos.Line,
			Column:      pos.Column,
			Type:        removeSelfAssignmentsType,
			Original:    stmt.Tok.String(),
			Mutated:     token.ASSIGN.String(),
			Description: fmt.Sprintf("Replace %s with %s", stmt.Tok.String(), token.ASSIGN.String()),
		},
	}
}

func (m *RemoveSelfAssignmentsMutator) isCompoundAssignOp(op token.Token) bool {
	switch op {
	case token.ADD_ASSIGN, token.SUB_ASSIGN, token.MUL_ASSIGN, token.QUO_ASSIGN, token.REM_ASSIGN,
		token.AND_ASSIGN, token.OR_ASSIGN, token.XOR_ASSIGN,
		token.SHL_ASSIGN, token.SHR_ASSIGN, token.AND_NOT_ASSIGN:
		return true
	default:
		return false
	}
}

// Apply applies the mutation to the given AST node.
func (m *RemoveSelfAssignmentsMutator) Apply(node ast.Node, mutant Mutant) bool {
	if mutant.Type != removeSelfAssignmentsType {
		return false
	}

	stmt, ok := node.(*ast.AssignStmt)
	if !ok || !m.isCompoundAssignOp(stmt.Tok) {
		return false
	}

	if stmt.Tok.String() != mutant.Original {
		return false
	}

	newOp := stringToToken(mutant.Mutated)
	if newOp == token.ILLEGAL {
		return false
	}

	stmt.Tok = newOp

	return true
}
