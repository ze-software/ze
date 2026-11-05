// Design: docs/architecture/plugin/feature-gates.md -- branch-neutral namespace evidence, never predicate evaluation.
// Related: assemblysyntax.go -- all-arm census; assembly.go -- entered-marker refusal.
package repoinventory

import "fmt"

type assemblyConditionalOp uint8

const (
	assemblyConditionalNone assemblyConditionalOp = iota
	assemblyConditionalOpen
	assemblyConditionalElse
	assemblyConditionalEnd
)

// The census and entered-body consumer MUST recognize the SAME marker set.
func assemblyConditionalDirective(tokens []string) assemblyConditionalOp {
	if len(tokens) < 2 {
		return assemblyConditionalNone
	}
	if tokens[0] != "#" {
		return assemblyConditionalNone
	}
	switch tokens[1] {
	case "ifdef", "ifndef":
		return assemblyConditionalOpen
	case "else":
		return assemblyConditionalElse
	case "endif":
		return assemblyConditionalEnd
	}
	return assemblyConditionalNone
}

type assemblyConditionals struct {
	elseSeen []bool
}

// accept tracks only balanced structure. Every push consumes a retained logical
// directive, so retained source bounds the stack. It never looks up a predicate.
func (blocks *assemblyConditionals) accept(tokens []string) (bool, error) {
	switch assemblyConditionalDirective(tokens) {
	case assemblyConditionalNone:
		return false, nil
	case assemblyConditionalOpen:
		if len(tokens) != 3 {
			return true, fmt.Errorf("malformed assembly conditional: expected one predicate identifier")
		}
		if _, err := assemblySymbol(tokens[2], ""); err != nil {
			return true, fmt.Errorf("malformed assembly conditional: predicate is not an identifier: %w", err)
		}
		blocks.elseSeen = append(blocks.elseSeen, false)
	case assemblyConditionalElse:
		if len(tokens) != 2 {
			return true, fmt.Errorf("malformed assembly conditional: else has operands")
		}
		if len(blocks.elseSeen) == 0 {
			return true, fmt.Errorf("malformed assembly conditional: else without opening directive")
		}
		last := len(blocks.elseSeen) - 1
		if blocks.elseSeen[last] {
			return true, fmt.Errorf("malformed assembly conditional: repeated else")
		}
		blocks.elseSeen[last] = true
	case assemblyConditionalEnd:
		if len(tokens) != 2 {
			return true, fmt.Errorf("malformed assembly conditional: endif has operands")
		}
		if len(blocks.elseSeen) == 0 {
			return true, fmt.Errorf("malformed assembly conditional: endif without opening directive")
		}
		blocks.elseSeen = blocks.elseSeen[:len(blocks.elseSeen)-1]
	}
	return true, nil
}

func (blocks *assemblyConditionals) inside() bool {
	return len(blocks.elseSeen) != 0
}

func (blocks *assemblyConditionals) finish() error {
	if blocks.inside() {
		return fmt.Errorf("malformed assembly conditional: unterminated block")
	}
	return nil
}
