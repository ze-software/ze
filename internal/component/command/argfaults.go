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
	"errors"

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
// as in 'asn <asn>'`); alone, a missing argument keeps its own text. Every
// route that knows the command path restates the list (RestatedForLine), so a
// list whose every fault is a missing argument reads the same on all of them,
// with the whole command as its fix: the daemon dispatcher, which also names
// the line that supplies a missing selector, and every route that judges
// through the model (ValidateModelArgs).
//
// Build it with newArgumentFaults; the zero value holds no fault and renders
// the opererr defect text. Immutable after construction, safe for concurrent
// use.
type ArgumentFaults struct {
	faults  []error
	lines   []string
	omitted int
	rest    restPosition
	refusal *opererr.Error
}

// restPosition is the index in the refused line where a rest-of-line leaf's
// words begin, its keyword or its first word (restStart). typed false means
// the line holds none, so no word of it is free text. The whole-command fix
// writes each missing argument before that index: after it, the leaf would
// read them as words of its text.
type restPosition struct {
	typed bool
	at    int
}

// argumentFaultsFix is the fix line of a list no route restated with a fix of
// its own: the definitions alone cannot write the whole command, because they
// do not hold its path.
const argumentFaultsFix = "correct each fault listed above, then type the command again"

// newArgumentFaults builds the refusal of faults, in the order the line was
// read, each named by the line at the same index of lines. omitted counts the
// faults past argumentFaultsMax that were not kept, and rest where the words
// of a rest-of-line leaf begin. fix is the fix line, the general correction
// when empty. faults MUST hold two or more errors, one fault being refused as
// itself, and lines MUST hold one line per fault.
func newArgumentFaults(faults []error, lines []string, omitted int, rest restPosition, fix string) *ArgumentFaults {
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
		rest:    rest,
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
	return newArgumentFaults(faults, lines, e.omitted, e.rest, fix)
}

// RestatedForLine answers the refusal as a route that knows the command line
// restates it, and is the one place any route does: each fault for which
// restate answers an error is replaced as Restated replaces it (nil restate
// keeps every fault's line), and the fix line is chosen here.
//
// When every fault is a missing argument and the list omitted none, the fix is
// the whole command to type: head, then each token of args but the first one
// equal to placedToken, then each missing argument by its keyword
// (MissingArgumentError.Usage) but placedLeaf's, which head already writes.
// Every token typed was valid, so the line is the operator's own line
// completed. A missing argument goes before the words of a rest-of-line leaf
// the line typed, which would read it as text. Otherwise the fix stays the general
// correction, because a fault in a typed word has no form a route can write
// for the operator.
//
// args MUST be the tokens ValidateArgs refused, in the order it read them.
// head is the command as the route's operator types it, with any value the
// route bound out of band already in place. The receiver is not changed.
func (e *ArgumentFaults) RestatedForLine(restate func(fault error) error, head string, args []string, placedToken, placedLeaf string) *ArgumentFaults {
	if restate == nil {
		restate = keepFault
	}
	fix := ""
	if e.onlyMissing() {
		fix = e.wholeCommand(head, args, placedToken, placedLeaf)
	}
	return e.Restated(restate, fix)
}

// keepFault is the restate function that keeps every fault as it is.
func keepFault(error) error {
	return nil
}

// onlyMissing reports whether every fault the refusal holds is a missing
// argument, with none omitted past the cap: only then is the whole command
// known, because an omitted fault could be a typed word at fault.
func (e *ArgumentFaults) onlyMissing() bool {
	if e.omitted > 0 {
		return false
	}
	for _, fault := range e.faults {
		if _, missing := errors.AsType[*MissingArgumentError](fault); !missing {
			return false
		}
	}
	return true
}

// wholeCommand writes the fix line RestatedForLine describes.
func (e *ArgumentFaults) wholeCommand(head string, args []string, placedToken, placedLeaf string) string {
	var tb textbuf.Buffer
	tb.Str(head)
	written := false
	skipped := placedToken == ""
	for i, token := range args {
		if !skipped && token == placedToken {
			skipped = true
			continue
		}
		if e.rest.typed && i == e.rest.at {
			e.writeMissing(&tb, placedLeaf)
			written = true
		}
		tb.Byte(' ')
		WriteInvocationValue(&tb, token)
	}
	if !written {
		e.writeMissing(&tb, placedLeaf)
	}
	return tb.String()
}

// writeMissing writes each missing argument but placedLeaf's by its keyword,
// in the order the refusal names them.
func (e *ArgumentFaults) writeMissing(tb *textbuf.Buffer, placedLeaf string) {
	for _, fault := range e.faults {
		missing, ok := errors.AsType[*MissingArgumentError](fault)
		if !ok {
			continue
		}
		if missing.Leaf == placedLeaf {
			continue
		}
		tb.Byte(' ').Str(missing.Usage())
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
	rest    restPosition
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
	return newArgumentFaults(l.faults, l.lines, l.omitted, l.rest, "")
}
