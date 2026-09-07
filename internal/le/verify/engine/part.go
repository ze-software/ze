// Design: docs/architecture/testing/verify-freshness-scope.md -- one piece of a stage population
// Related: stages.go -- the populations a part is dealt from
package verifyengine

import (
	"fmt"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Part is one piece of a stage population. Both numbers count from one, so an
// uncut run is part 1 of 1 and runs every stage its mode declares.
//
// The pieces are DEALT round robin rather than cut into contiguous blocks,
// which is the shape `staticcheckfeaturematrix.Matrix.Part` already uses and
// for the same reason: the expensive stages sit together in the ordered
// population, so a contiguous first piece would carry the whole Staticcheck
// matrix and a contiguous last piece both functional suites. Dealing also keeps
// the partition derived from the population, so a stage added to stages.go
// lands in one piece with nothing to assign by hand.
//
// The zero value names no piece and is REFUSED. A run that cannot say which
// piece it judges MUST NOT judge one, and "the caller said nothing" is not a
// piece (ai/rules/principles.md).
type Part struct {
	Index int `json:"index"`
	Of    int `json:"of"`
}

// Uncut is the whole population as one piece.
func Uncut() Part { return Part{Index: 1, Of: 1} }

// Cut reports whether this part is one piece of several.
func (p Part) Cut() bool { return p.Of != 1 }

// Valid answers why these two numbers name no piece of a population, or nil.
func (p Part) Valid() error {
	if p.Of < 1 {
		return fmt.Errorf("a run cut into %d pieces judges nothing, want at least 1 piece", p.Of)
	}
	if p.Index < 1 || p.Index > p.Of {
		return fmt.Errorf("part %d is outside the 1 to %d the run was cut into", p.Index, p.Of)
	}
	return nil
}

// deal answers the stages this part runs, or the error that says why the
// numbers name no piece.
//
// Every stage lands in exactly one piece, so the pieces together run the whole
// population exactly once. A piece is EMPTY when a population is cut into more
// pieces than it holds stages, and an empty piece proves nothing on its own:
// what discharges a caller's obligation is every piece passing over one tree,
// never one of them.
func (p Part) deal(all []Stage) ([]Stage, error) {
	if err := p.Valid(); err != nil {
		return nil, err
	}
	if !p.Cut() {
		return all, nil
	}
	dealt := make([]Stage, 0, len(all)/p.Of+1)
	for position, current := range all {
		if position%p.Of == p.Index-1 {
			dealt = append(dealt, current)
		}
	}
	return dealt, nil
}

// Name answers the certificate mode of a population cut this way.
//
// A cut run judged a SUBSET, so the name it writes is deliberately one
// StagesForMode does not know: CheckCertificate refuses a certificate whose
// mode names no population, and one piece of a run therefore can never read as
// a pass over the tree.
func (p Part) Name(mode string) string {
	if !p.Cut() {
		return mode
	}
	var text textbuf.Buffer
	return text.Str(mode).Str("-part-").Int(int64(p.Index)).
		Str("-of-").Int(int64(p.Of)).String()
}
