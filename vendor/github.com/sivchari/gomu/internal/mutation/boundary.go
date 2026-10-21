package mutation

import (
	"fmt"
	"go/ast"
	"go/token"
	"math"
	"strconv"
	"strings"
)

const (
	boundaryValueMutatorName = "boundary_value"
	boundaryValueType        = "boundary_value"
)

// BoundaryValueMutator mutates integer literals to their boundary neighbours
// (N -> N+1 and N -> N-1), surfacing weak off-by-one / boundary tests.
//
// Relational operator boundary shifts (e.g. < -> <=) are already covered by
// the conditional mutator, so this mutator focuses on the literal side of the
// boundary to avoid generating duplicate mutants.
//
// Mutants keep the literal's base prefix and case (0x10 -> 0x11, 0X10 -> 0X11)
// and digit separators on decimal literals (1_000 -> 1_001); literals up to the
// uint64 maximum are mutated.
type BoundaryValueMutator struct {
}

// Name returns the name of the mutator.
func (m *BoundaryValueMutator) Name() string {
	return boundaryValueMutatorName
}

// Description returns a human-readable summary of the mutator.
func (m *BoundaryValueMutator) Description() string {
	return "Shift integer literals to boundary values (N-1, N+1)"
}

// CanMutate returns true if the node can be mutated by this mutator.
func (m *BoundaryValueMutator) CanMutate(node ast.Node) bool {
	lit, ok := node.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return false
	}

	_, ok = parseIntLit(lit.Value)

	return ok
}

// Mutate generates mutants for the given node.
func (m *BoundaryValueMutator) Mutate(node ast.Node, fset *token.FileSet) []Mutant {
	lit, ok := node.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return nil
	}

	value, ok := parseIntLit(lit.Value)
	if !ok {
		return nil
	}

	pos := fset.Position(node.Pos())

	var mutants []Mutant

	// N -> N+1. MaxInt64+1 is skipped as well: without type information the
	// literal is assumed to be an int, where it would not compile.
	if value != math.MaxUint64 && value != math.MaxInt64 {
		mutants = append(mutants, m.newMutant(lit.Value, formatIntLit(lit.Value, value+1), pos))
	}

	// N -> N-1 (skip when the result would become a negative literal, which is
	// not representable as a single integer literal in the AST).
	if value >= 1 {
		mutants = append(mutants, m.newMutant(lit.Value, formatIntLit(lit.Value, value-1), pos))
	}

	return mutants
}

func (m *BoundaryValueMutator) newMutant(original, mutated string, pos token.Position) Mutant {
	return Mutant{
		Line:        pos.Line,
		Column:      pos.Column,
		Type:        boundaryValueType,
		Original:    original,
		Mutated:     mutated,
		Description: fmt.Sprintf("Replace integer literal %s with %s", original, mutated),
	}
}

// Apply applies the mutation to the given AST node.
func (m *BoundaryValueMutator) Apply(node ast.Node, mutant Mutant) bool {
	if mutant.Type != boundaryValueType {
		return false
	}

	lit, ok := node.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return false
	}

	if lit.Value != mutant.Original {
		return false
	}

	lit.Value = mutant.Mutated

	return true
}

// parseIntLit parses a Go integer literal (supporting base prefixes and digit
// separators) into a uint64. It reports false when the literal exceeds uint64.
func parseIntLit(s string) (uint64, bool) {
	value, err := strconv.ParseUint(s, 0, 64)
	if err != nil {
		return 0, false
	}

	return value, true
}

// formatIntLit formats n using the base prefix, prefix case and (for decimal
// literals) digit separators of the original literal.
func formatIntLit(original string, n uint64) string {
	if len(original) > 2 && original[0] == '0' {
		prefix := original[:2]

		switch prefix {
		case "0x":
			return prefix + strconv.FormatUint(n, 16)
		case "0X":
			return prefix + strings.ToUpper(strconv.FormatUint(n, 16))
		case "0o", "0O":
			return prefix + strconv.FormatUint(n, 8)
		case "0b", "0B":
			return prefix + strconv.FormatUint(n, 2)
		}
	}

	digits := strconv.FormatUint(n, 10)
	if !strings.Contains(original, "_") {
		return digits
	}

	var b strings.Builder

	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte('_')
		}

		b.WriteRune(d)
	}

	return b.String()
}
