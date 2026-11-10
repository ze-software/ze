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
// and so does each pre-bound value, plus one for each missing mandatory
// argument, so the list is bounded by the line and the definitions. The cap
// bounds the refusal text for a line no operator would type: a fault past it
// is counted, and the refusal says how many it did not list.
const argumentFaultsMax = 16

// ArgumentFaults is ValidateArgs's refusal of a call whose line holds two or
// more faults: a keyword with no value, a value its leaf refuses, a keyword
// typed twice, a token no definition takes, a missing mandatory argument. A
// line with one fault is refused with that fault alone, so its text is the
// same as before the line was read to its end.
//
// Unwrap answers each fault, so errors.Is and errors.As reach every one of
// them: a *MissingArgumentError among them is found the same way as when it
// is the only fault, and errors.As answers the first. It holds one
// *MissingArgumentError per missing leaf, so a list can hold missing
// arguments only, all carrying the same positional binding. A route that must
// tell "one leaf alone is missing" from "the line has several faults" asks
// errors.As for an *ArgumentFaults first, and reads Faults to learn whether
// any of them is not a missing argument.
//
// Each fault reads in the list as its own text, except a missing argument,
// which also names the words that supply it (`required argument missing: asn,
// as in 'asn <asn>'`); alone, a missing argument keeps its own text. A route
// that knows more than the definitions do restates the list (Restated): the
// daemon dispatcher names the line that supplies a missing selector, and the
// whole command when every fault is a missing argument.
//
// Build it with newArgumentFaults; the zero value holds no fault and renders
// the opererr defect text. Immutable after construction, safe for concurrent
// use.
type ArgumentFaults struct {
	faults  []error
	lines   []string
	omitted int
	refusal *opererr.Error
}

// argumentFaultsFix is the fix line of a list no route restated with a fix of
// its own: the definitions alone cannot write the whole command, because they
// do not hold its path.
const argumentFaultsFix = "correct each fault listed above, then type the command again"

// newArgumentFaults builds the refusal of faults, in the order the line was
// read, each named by the line at the same index of lines. omitted counts the
// faults past argumentFaultsMax that were not kept. fix is the fix line, the
// general correction when empty. faults MUST hold two or more errors, one fault
// being refused as itself, and lines MUST hold one line per fault.
func newArgumentFaults(faults []error, lines []string, omitted int, fix string) *ArgumentFaults {
	if len(faults) < 2 {
		panic("BUG: newArgumentFaults called with fewer than two faults")
	}
	if len(lines) != len(faults) {
		panic("BUG: newArgumentFaults called with a line count that differs from the fault count")
	}
	if fix == "" {
		fix = argumentFaultsFix
	}
	total := len(faults) + omitted
	var problem textbuf.Buffer
	problem.Str("this command has ").Int(int64(total)).Str(" faults, so Ze did not run it")
	var why textbuf.Buffer
	why.Str("Ze runs a command only when every word of it is valid, and each line below is one fault:")
	for _, line := range lines {
		why.Str("\n- ").Str(line)
	}
	if omitted > 0 {
		why.Str("\n- and ").Int(int64(omitted)).Str(" more faults not listed here")
	}
	return &ArgumentFaults{
		faults:  faults,
		lines:   lines,
		omitted: omitted,
		refusal: opererr.New(problem.String(), why.String(), fix),
	}
}

// Restated answers the refusal with each fault for which restate answers an
// error replaced by that error, named in the list by its own text, and with
// fix as its fix line, or the general correction when fix is empty. A fault for
// which restate answers nil keeps its place and its line. The receiver is not
// changed. restate MUST NOT be nil.
func (e *ArgumentFaults) Restated(restate func(fault error) error, fix string) *ArgumentFaults {
	faults := make([]error, len(e.faults))
	lines := make([]string, len(e.lines))
	for i, fault := range e.faults {
		faults[i], lines[i] = fault, e.lines[i]
		if replacement := restate(fault); replacement != nil {
			faults[i], lines[i] = replacement, replacement.Error()
		}
	}
	return newArgumentFaults(faults, lines, e.omitted, fix)
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

// Omitted answers how many faults the refusal counted past argumentFaultsMax
// and did not keep.
func (e *ArgumentFaults) Omitted() int {
	return e.omitted
}

// Faults answers a copy of the faults the refusal names, in the order the
// line was read.
func (e *ArgumentFaults) Faults() []error {
	out := make([]error, len(e.faults))
	copy(out, e.faults)
	return out
}

// faultList collects the faults ValidateArgs finds in one line, up to
// argumentFaultsMax, with the line each reads as in a list. Not safe for
// concurrent use: one call owns it.
type faultList struct {
	faults  []error
	lines   []string
	omitted int
}

// add keeps fault, named by its own text, or counts it once the list is full.
func (l *faultList) add(fault error) {
	l.keep(fault, fault.Error())
}

// addMissing keeps a missing argument, named with the words that supply it.
func (l *faultList) addMissing(missing *MissingArgumentError) {
	var line textbuf.Buffer
	line.Str(missing.Error()).Str(", as in '").Str(missing.Usage()).Byte('\'')
	l.keep(missing, line.String())
}

// keep keeps fault with line, or counts it once the list is full.
func (l *faultList) keep(fault error, line string) {
	if len(l.faults) < argumentFaultsMax {
		l.faults = append(l.faults, fault)
		l.lines = append(l.lines, line)
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
	return newArgumentFaults(l.faults, l.lines, l.omitted, "")
}
