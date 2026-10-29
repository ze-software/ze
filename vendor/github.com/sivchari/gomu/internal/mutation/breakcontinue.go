package mutation

import (
	"fmt"
	"go/ast"
	"go/token"
)

const (
	breakContinueMutatorName = "break_continue"
	breakContinueType        = "break_continue"
)

// BreakContinueMutator swaps break and continue branch statements.
type BreakContinueMutator struct {
}

// Name returns the name of the mutator.
func (m *BreakContinueMutator) Name() string {
	return breakContinueMutatorName
}

// Description returns a human-readable summary of the mutator.
func (m *BreakContinueMutator) Description() string {
	return "Swap break and continue statements"
}

// CanMutate returns true if the node can be mutated by this mutator.
func (m *BreakContinueMutator) CanMutate(node ast.Node) bool {
	stmt, ok := node.(*ast.BranchStmt)
	if !ok {
		return false
	}

	// Only swap label-less break/continue. Labeled statements reference a
	// specific loop/switch and swapping them is far more likely to be invalid.
	if stmt.Label != nil {
		return false
	}

	return stmt.Tok == token.BREAK || stmt.Tok == token.CONTINUE
}

// Mutate generates mutants for the given node.
func (m *BreakContinueMutator) Mutate(node ast.Node, fset *token.FileSet) []Mutant {
	if !m.CanMutate(node) {
		return nil
	}

	stmt := node.(*ast.BranchStmt) //nolint:forcetypeassert // guarded by CanMutate
	newOp := m.swap(stmt.Tok)
	pos := fset.Position(stmt.Pos())

	return []Mutant{
		{
			Line:        pos.Line,
			Column:      pos.Column,
			Type:        breakContinueType,
			Original:    stmt.Tok.String(),
			Mutated:     newOp.String(),
			Description: fmt.Sprintf("Replace %s with %s", stmt.Tok.String(), newOp.String()),
		},
	}
}

// Apply applies the mutation to the given AST node.
func (m *BreakContinueMutator) Apply(node ast.Node, mutant Mutant) bool {
	if mutant.Type != breakContinueType {
		return false
	}

	stmt, ok := node.(*ast.BranchStmt)
	if !ok || stmt.Label != nil {
		return false
	}

	if stmt.Tok != token.BREAK && stmt.Tok != token.CONTINUE {
		return false
	}

	if stmt.Tok.String() != mutant.Original {
		return false
	}

	stmt.Tok = m.swap(stmt.Tok)

	return true
}

func (m *BreakContinueMutator) swap(op token.Token) token.Token {
	if op == token.BREAK {
		return token.CONTINUE
	}

	return token.BREAK
}
