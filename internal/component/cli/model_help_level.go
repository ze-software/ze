// Design: docs/architecture/cli/command-completion.md — what Tab reveals
// Related: model_keys.go — the keys that reveal a level and dismiss it
// Related: model_render.go — the rows each level draws
// Related: model.go — the fields revealLevel reads

package cli

// revealLevel names what the completion machinery has on the screen. Each level
// is named for what an operator sees, not for a step in a sequence.
//
// The level is DERIVED from the fields that own each row, and never stored. A
// stored level beside those fields would be a second declaration of one fact,
// free to disagree with it.
type revealLevel uint8

const (
	// revealNothing is the plain prompt: no menu and no explanation.
	revealNothing revealLevel = iota
	// revealCandidates is the completion menu, with the selected candidate's
	// summary on the second message line.
	revealCandidates
	// revealExplanation is the command's long explanation, in its own region.
	revealExplanation
)

// revealLevel answers what is on the screen now. It reads the field that owns
// each row.
//
// The explanation wins, and View draws it in that order. ? reveals the
// explanation of a candidate with the menu open, so the menu waits under the
// box until one Escape takes the box off.
func (m Model) revealLevel() revealLevel {
	if m.explanation != "" {
		return revealExplanation
	}
	if m.showDropdown {
		return revealCandidates
	}
	return revealNothing
}

// Explanation returns the long explanation now on screen, and the empty string
// when none is. A test reads the text an operator sees, and does not parse the
// box that frames it.
func (m Model) Explanation() string {
	return m.explanation
}

// dismissReveal takes off the screen everything the completion machinery put
// there: the hint on the second message line and the explanation.
//
// Every key that ends a reveal calls this one method. Thirteen sites used to
// clear the hint by hand. A level added later would have had thirteen places to
// be forgotten, and each miss leaves stale text under what the operator has
// since typed.
func (m *Model) dismissReveal() {
	m.completionHint = ""
	m.completionHintDim = false
	m.explanation = ""
	m.explanationSubject = ""
}
