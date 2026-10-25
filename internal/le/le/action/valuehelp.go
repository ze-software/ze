// Design: docs/contributing/running-commands.md -- every word le offers says what it does
// Related: parameter.go -- the Values and ValueHelp a declaration carries
//
// The declared values of an enum or a closed list, each with the line that
// says what it does, as completion, the menu and help offer them.
package leaction

// ValueCandidates answers the parameter's declared Values in declaration
// order, each with its ValueHelp line, so a completion or a prompt that offers
// a value says what choosing it does. A value with no line answers an empty
// Summary, which le's startup check refuses before any reader asks.
func (p Parameter) ValueCandidates() []Candidate {
	out := make([]Candidate, 0, len(p.Values))
	for _, value := range p.Values {
		out = append(out, Candidate{Value: value, Summary: p.ValueHelp[value]})
	}
	return out
}
