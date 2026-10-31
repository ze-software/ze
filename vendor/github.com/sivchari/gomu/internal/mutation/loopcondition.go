package mutation

import (
	"fmt"
	"go/ast"
	"go/token"
)

const (
	loopConditionMutatorName = "loop_condition"
	loopConditionType        = "loop_condition"
)

// LoopConditionMutator mutates the condition of for statements to the boolean
// literals true and false, testing whether loop termination is properly
// covered by tests.
type LoopConditionMutator struct {
}

// Name returns the name of the mutator.
func (m *LoopConditionMutator) Name() string {
	return loopConditionMutatorName
}

// Description returns a human-readable summary of the mutator.
func (m *LoopConditionMutator) Description() string {
	return "Force for-loop conditions to true or false"
}

// CanMutate returns true if the node can be mutated by this mutator.
func (m *LoopConditionMutator) CanMutate(node ast.Node) bool {
	stmt, ok := node.(*ast.ForStmt)
	if !ok || stmt.Cond == nil {
		return false
	}

	return !isBoolLiteral(stmt.Cond)
}

// Mutate generates mutants for the given node.
func (m *LoopConditionMutator) Mutate(node ast.Node, fset *token.FileSet) []Mutant {
	stmt, ok := node.(*ast.ForStmt)
	if !ok || stmt.Cond == nil || isBoolLiteral(stmt.Cond) {
		return nil
	}

	pos := fset.Position(stmt.Pos())
	original := exprToString(stmt.Cond)

	return []Mutant{
		{
			Line:        pos.Line,
			Column:      pos.Column,
			Type:        loopConditionType,
			Original:    original,
			Mutated:     boolFalse,
			Description: fmt.Sprintf("Replace loop condition %q with false", original),
		},
		{
			Line:        pos.Line,
			Column:      pos.Column,
			Type:        loopConditionType,
			Original:    original,
			Mutated:     boolTrue,
			Description: fmt.Sprintf("Replace loop condition %q with true", original),
		},
	}
}

// Apply applies the mutation to the given AST node.
func (m *LoopConditionMutator) Apply(node ast.Node, mutant Mutant) bool {
	if mutant.Type != loopConditionType {
		return false
	}

	stmt, ok := node.(*ast.ForStmt)
	if !ok || stmt.Cond == nil {
		return false
	}

	if exprToString(stmt.Cond) != mutant.Original {
		return false
	}

	stmt.Cond = &ast.Ident{Name: mutant.Mutated}

	return true
}
