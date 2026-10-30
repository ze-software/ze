package mutation

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"

	"golang.org/x/tools/go/ast/astutil"
)

// ApplyMutantToSource returns src with mutant applied, gofmt-formatted.
//
// The target node is the first one whose position equals mutant.Line:Column;
// the registered mutators are tried in registry order, CursorApplier first.
func ApplyMutantToSource(src []byte, filename string, mutant Mutant) ([]byte, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	mutators := getAllMutators()

	mutated := false

	astutil.Apply(file, nil, func(c *astutil.Cursor) bool {
		if mutated {
			return false
		}

		node := c.Node()
		if node == nil {
			return true
		}

		pos := fset.Position(node.Pos())
		if pos.Line == mutant.Line && pos.Column == mutant.Column {
			mutated = applyMutantToNode(mutators, node, func(replacement ast.Node) {
				c.Replace(replacement)
			}, mutant)
		}

		return !mutated
	})

	if !mutated {
		return nil, fmt.Errorf("failed to find mutation target at %s:%d:%d", filename, mutant.Line, mutant.Column)
	}

	var buf bytes.Buffer

	if err := format.Node(&buf, fset, file); err != nil {
		return nil, fmt.Errorf("failed to write mutated file: %w", err)
	}

	return buf.Bytes(), nil
}

// applyMutantToNode applies mutant to node, trying each mutator's
// CursorApplier.ApplyWithCursor before falling back to its Apply.
func applyMutantToNode(mutators []Mutator, node ast.Node, replace func(ast.Node), mutant Mutant) bool {
	for _, m := range mutators {
		if ca, ok := m.(CursorApplier); ok {
			if ca.ApplyWithCursor(node, replace, mutant) {
				return true
			}
		}
	}

	for _, m := range mutators {
		if m.Apply(node, mutant) {
			return true
		}
	}

	return false
}
