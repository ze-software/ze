package mutation

import (
	"go/ast"
	"go/token"
)

const (
	emptyBlockMutatorName = "empty_block"
	emptyBlockType        = "empty_block"

	emptyBlockOriginal = "{...}"
	emptyBlockMutated  = "{}"
)

// EmptyBlockMutator empties the body of a block statement, testing whether the
// statements inside the block are properly covered by tests.
type EmptyBlockMutator struct {
}

// Name returns the name of the mutator.
func (m *EmptyBlockMutator) Name() string {
	return emptyBlockMutatorName
}

// Description returns a human-readable summary of the mutator.
func (m *EmptyBlockMutator) Description() string {
	return "Remove all statements from a block body"
}

// CanMutate returns true if the node can be mutated by this mutator.
func (m *EmptyBlockMutator) CanMutate(node ast.Node) bool {
	block, ok := node.(*ast.BlockStmt)

	return ok && len(block.List) > 0
}

// Mutate generates mutants for the given node.
func (m *EmptyBlockMutator) Mutate(node ast.Node, fset *token.FileSet) []Mutant {
	block, ok := node.(*ast.BlockStmt)
	if !ok || len(block.List) == 0 {
		return nil
	}

	pos := fset.Position(block.Pos())

	return []Mutant{
		{
			Line:        pos.Line,
			Column:      pos.Column,
			Type:        emptyBlockType,
			Original:    emptyBlockOriginal,
			Mutated:     emptyBlockMutated,
			Description: "Remove all statements in block",
		},
	}
}

// Apply applies the mutation to the given AST node.
func (m *EmptyBlockMutator) Apply(node ast.Node, mutant Mutant) bool {
	if mutant.Type != emptyBlockType {
		return false
	}

	block, ok := node.(*ast.BlockStmt)
	if !ok || len(block.List) == 0 {
		return false
	}

	block.List = nil

	return true
}
