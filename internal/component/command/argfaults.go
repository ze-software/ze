// Design: docs/architecture/api/commands.md -- YANG-typed argument validation
// Related: argbind.go -- ValidateArgs, which collects the faults of one line
// Related: ../../core/opererr/opererr.go -- the three parts the refusal renders
//
// argfaults.go holds the refusal of a command line that breaks its definitions
// in more than one place. The validator reads the whole line and names every
// fault it holds, so an operator who fixes the first one is not refused again
// for the next.

package command

import (
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// argumentFaultsMax bounds how many faults one refusal keeps and names. Each
// token of a line yields at most one fault (a keyword and its value are one),
// and so does each pre-bound value, plus one for the first missing mandatory
// argument, so the list is bounded by the line. The cap bounds the refusal
// text for a line no operator would type: a fault past it is counted, and the
// refusal says how many it did not list.
const argumentFaultsMax = 16

// ArgumentFaults is ValidateArgs's refusal of a call whose line holds two or
// more faults: a keyword with no value, a value its leaf refuses, a keyword
// typed twice, a token no definition takes, a missing mandatory argument. A
// line with one fault is refused with that fault alone, so its text is the
// same as before the line was read to its end.
//
// Unwrap answers each fault, so errors.Is and errors.As reach every one of
// them: a *MissingArgumentError among them is found the same way as when it
// is the only fault. It holds at most one *MissingArgumentError (ValidateArgs
// names the first missing leaf only), so a list always holds at least one
// fault that is not a missing argument. A route that must tell "the selector
// alone is missing" from "the line has other faults" asks errors.As for an
// *ArgumentFaults first.
//
// Build it with newArgumentFaults; the zero value holds no fault and renders
// the opererr defect text. Immutable after construction, safe for concurrent
// use.
type ArgumentFaults struct {
	faults  []error
	refusal *opererr.Error
}

// newArgumentFaults builds the refusal of faults, in the order the line was
// read. omitted counts the faults past argumentFaultsMax that were not kept.
// faults MUST hold two or more errors: one fault is refused as itself.
func newArgumentFaults(faults []error, omitted int) *ArgumentFaults {
	if len(faults) < 2 {
		panic("BUG: newArgumentFaults called with fewer than two faults")
	}
	total := len(faults) + omitted
	var problem textbuf.Buffer
	problem.Str("this command has ").Int(int64(total)).Str(" faults, so Ze did not run it")
	var why textbuf.Buffer
	why.Str("Ze runs a command only when every word of it is valid, and each line below is one fault:")
	for _, fault := range faults {
		why.Str("\n- ").Err(fault)
	}
	if omitted > 0 {
		why.Str("\n- and ").Int(int64(omitted)).Str(" more faults not listed here")
	}
	return &ArgumentFaults{
		faults:  faults,
		refusal: opererr.New(problem.String(), why.String(), "correct each fault listed above, then type the command again"),
	}
}

// Error answers the refusal text every route prints: the problem, then each
// fault on a line of its own, then the fix.
func (e *ArgumentFaults) Error() string {
	if e.refusal == nil {
		return (&opererr.Error{}).Error()
	}
	return e.refusal.Error()
}

// Unwrap answers the operator error that renders the refusal, then each
// fault, so errors.As finds the three parts for the web notice and every
// fault kind for the routes that branch on one.
func (e *ArgumentFaults) Unwrap() []error {
	out := make([]error, 0, 1+len(e.faults))
	if e.refusal != nil {
		out = append(out, e.refusal)
	}
	return append(out, e.faults...)
}

// Faults answers a copy of the faults the refusal names, in the order the
// line was read.
func (e *ArgumentFaults) Faults() []error {
	out := make([]error, len(e.faults))
	copy(out, e.faults)
	return out
}

// faultList collects the faults ValidateArgs finds in one line, up to
// argumentFaultsMax. Not safe for concurrent use: one call owns it.
type faultList struct {
	faults  []error
	omitted int
}

// add keeps fault, or counts it once the list is full.
func (l *faultList) add(fault error) {
	if len(l.faults) < argumentFaultsMax {
		l.faults = append(l.faults, fault)
		return
	}
	l.omitted++
}

// verdict answers nil for a line with no fault, the fault itself for a line
// with one, and an *ArgumentFaults for a line with more.
func (l *faultList) verdict() error {
	if len(l.faults) == 0 {
		return nil
	}
	if len(l.faults) == 1 {
		return l.faults[0]
	}
	return newArgumentFaults(l.faults, l.omitted)
}
