package mutation

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
)

const (
	stringLiteralMutatorName = "string_literal"
	stringLiteralType        = "string_literal"

	// stringLiteralNonEmpty is the replacement used when an empty string
	// literal is mutated, so that empty-string assertions are exercised.
	stringLiteralNonEmpty = `"mutated"`
	// stringLiteralEmpty is the replacement used when a non-empty string
	// literal is mutated.
	stringLiteralEmpty = `""`
)

// StringLiteralMutator mutates string literals: non-empty strings become the
// empty string and empty strings become a non-empty placeholder.
type StringLiteralMutator struct {
}

// Name returns the name of the mutator.
func (m *StringLiteralMutator) Name() string {
	return stringLiteralMutatorName
}

// Description returns a human-readable summary of the mutator.
func (m *StringLiteralMutator) Description() string {
	return "Swap string literals between empty and non-empty"
}

// CanMutate returns true if the node can be mutated by this mutator.
func (m *StringLiteralMutator) CanMutate(node ast.Node) bool {
	lit, ok := node.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}

	_, ok = unquoteString(lit.Value)

	return ok
}

// Mutate generates mutants for the given node.
func (m *StringLiteralMutator) Mutate(node ast.Node, fset *token.FileSet) []Mutant {
	lit, ok := node.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return nil
	}

	content, ok := unquoteString(lit.Value)
	if !ok {
		return nil
	}

	mutated := stringLiteralEmpty
	if content == "" {
		mutated = stringLiteralNonEmpty
	}

	pos := fset.Position(node.Pos())

	return []Mutant{
		{
			Line:        pos.Line,
			Column:      pos.Column,
			Type:        stringLiteralType,
			Original:    lit.Value,
			Mutated:     mutated,
			Description: fmt.Sprintf("Replace string literal %s with %s", lit.Value, mutated),
		},
	}
}

// Apply applies the mutation to the given AST node.
func (m *StringLiteralMutator) Apply(node ast.Node, mutant Mutant) bool {
	if mutant.Type != stringLiteralType {
		return false
	}

	lit, ok := node.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}

	if lit.Value != mutant.Original {
		return false
	}

	lit.Value = mutant.Mutated

	return true
}

// unquoteString unquotes a Go string literal (interpreted or raw), returning
// its content and whether it could be unquoted.
func unquoteString(s string) (string, bool) {
	content, err := strconv.Unquote(s)
	if err != nil {
		return "", false
	}

	return content, true
}
