// Design: docs/architecture/testing/test-health.md -- exact Go unit boundaries.
// Related: scope.go -- grouping, structural attribution and semantic comparison.
package testweakened

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/rfc"
)

// goUnits reads content through rfc.DeclaredUnits, the one Go unit inventory
// the RFC gate reads too, so the two gates name the same units. An inexact
// declaration carries no text. Content that did not parse ends with an unnamed
// incomplete entry, which records the uncertainty outside every declaration:
// a declaration the parser skipped is absent, not removed. Neither can be
// mistaken for an exact span.
func goUnits(content string) []namedUnit {
	declared, parsed := rfc.DeclaredUnits(content)
	units := make([]namedUnit, 0, len(declared)+1)
	for _, unit := range declared {
		if !unit.Exact {
			units = append(units, namedUnit{name: unit.Name, incomplete: true})
			continue
		}
		units = append(units, namedUnit{name: unit.Name, text: unit.Text, start: unit.Start, end: unit.End})
	}
	if !parsed {
		units = append(units, namedUnit{incomplete: true})
	}
	return units
}

// goUnresolvedText retains spans that cannot be compared as named units on both
// sides. In particular, a complete baseline stays when its replacement is
// incomplete, so assertion loss remains visible. Inventory offsets prevent an
// identical comment or literal from being removed instead of the declaration.
func goUnresolvedText(content string, other []namedUnit) string {
	var remaining textbuf.Buffer
	start := 0
	for _, unit := range goUnits(content) {
		if unit.incomplete {
			continue
		}
		if !unitBoundaryKnown(other, unit.name) {
			continue
		}
		// A doc comment that reached back over the previous unit's end would
		// overlap it; keeping the text residual judges it rather than panic.
		if unit.start < start {
			continue
		}
		remaining.Str(content[start:unit.start]).Byte('\n')
		start = unit.end
	}
	return remaining.Str(content[start:]).String()
}
