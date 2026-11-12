// Design: docs/architecture/api/commands.md -- YANG-typed argument validation
// Related: argbind.go -- ValidateArgs, which collects the faults of one line
// Related: ../../core/opererr/opererr.go -- the three parts the refusal renders
//
// Related: argvalue.go -- ValueArgumentError, the fault of a value its leaf refuses
//
// argfaults.go holds the refusal of a command line that breaks its definitions
// in more than one place, or whose one fault is a value its leaf refuses. The
// validator reads the whole line and names every fault it holds, so an
// operator who fixes the first one is not refused again for the next.

package command

import (
	"errors"
	"strconv"

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
// more faults (a keyword with no value, a value its leaf refuses, a keyword
// typed twice, a token no definition takes, a missing mandatory argument), or
// whose one fault is a value its leaf refuses (*ValueArgumentError). A line
// with one fault of another kind is refused with that fault alone.
//
// A refusal of one value fault is that fault: its problem, then its why, with
// no count of faults. A refusal of several counts them, and its why holds one
// `- ` entry per fault.
//
// Unwrap answers each fault, so errors.Is and errors.As reach every one of
// them: a *MissingArgumentError among them is found the same way as when it
// is the only fault, and errors.As answers the first. It holds one
// *MissingArgumentError per missing leaf, so a list can hold missing
// arguments only, all carrying the same positional binding. A route that must
// tell "one leaf alone is missing" from "the line has a fault of another
// kind" asks errors.As for an *ArgumentFaults first, and reads Faults to learn
// whether any of them is not a missing argument.
//
// Each fault reads in the list as its own text, except a missing argument,
// which also names the words that supply it (`required argument missing: asn,
// as in 'asn <asn>'`), and a value fault, which reads as its problem with its
// why indented under it and never carries a fix of its own; alone, a missing
// argument keeps its own text. Every route that knows the command path
// restates the refusal (RestatedForLine), so it reads the same on all of
// them, with the whole command as its fix when every fault is a missing
// argument or a value typed in the line: the daemon dispatcher, which also
// names the line that supplies a missing selector, and every route that
// judges through the model (ValidateModelArgs).
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

// oneValueFix is the fix line of a lone value fault no route restated with the
// whole command: the value to type in place of the refused one.
func oneValueFix(fault *ValueArgumentError) string {
	var tb textbuf.Buffer
	tb.Str("type the command again with ").Str(fault.Correction()).Str(" in place of ").Str(strconv.Quote(fault.Value()))
	return tb.String()
}

// newArgumentFaults builds the refusal of faults, in the order the line was
// read, each named by the line at the same index of lines. omitted counts the
// faults past argumentFaultsMax that were not kept, and rest where the words
// of a rest-of-line leaf begin. fix is the fix line, the general correction
// when empty. faults MUST hold two or more errors, or one *ValueArgumentError
// with none omitted (a lone fault of another kind is refused as itself), and
// lines MUST hold one line per fault.
func newArgumentFaults(faults []error, lines []string, omitted int, rest restPosition, fix string) *ArgumentFaults {
	if len(lines) != len(faults) {
		panic("BUG: newArgumentFaults called with a line count that differs from the fault count")
	}
	if len(faults) == 1 && omitted == 0 {
		lone, ok := errors.AsType[*ValueArgumentError](faults[0])
		if !ok {
			panic("BUG: newArgumentFaults called with one fault that is not a value fault")
		}
		if fix == "" {
			fix = oneValueFix(lone)
		}
		return &ArgumentFaults{
			faults:  faults,
			lines:   lines,
			rest:    rest,
			refusal: opererr.New(lone.Problem(), lone.Why(), fix),
		}
	}
	if len(faults) < 2 {
		panic("BUG: newArgumentFaults called with fewer than two faults")
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
// error replaced by that error, named in the list as faultLine names it, and with
// fix as its fix line, or the general correction when fix is empty. A fault for
// which restate answers nil keeps its place and its line. The receiver is not
// changed. restate MUST NOT be nil.
func (e *ArgumentFaults) Restated(restate func(fault error) error, fix string) *ArgumentFaults {
	faults := make([]error, len(e.faults))
	lines := make([]string, len(e.lines))
	for i, fault := range e.faults {
		faults[i], lines[i] = fault, e.lines[i]
		if replacement := restate(fault); replacement != nil {
			faults[i], lines[i] = replacement, faultLine(replacement)
		}
	}
	return newArgumentFaults(faults, lines, e.omitted, e.rest, fix)
}

// RestatedForLine answers the refusal as a route that knows the command line
// restates it, and is the one place any route does: each fault for which
// restate answers an error is replaced as Restated replaces it (nil restate
// keeps every fault's line), each value fault names path before its argument
// (`send bgp raw encoding: ...`), and the fix line is chosen here.
//
// When every fault is a missing argument or a value fault typed in args, and
// the list omitted none, the fix is the whole command to type: head, then each
// token of args but the first one equal to placedToken, each refused value
// replaced by its fault's Correction, then each missing argument by its
// keyword (MissingArgumentError.Usage) but placedLeaf's, which head already
// writes. Every other token typed was valid, so the line is the operator's own
// line corrected and completed. A missing argument goes before the words of a
// rest-of-line leaf the line typed, which would read it as text. Otherwise the
// fix stays the general correction, because a fault in a typed word of
// another kind, or a value the route bound out of band, has no form a route
// can write for the operator.
//
// path is the command as the operator types it with no value in it, head the
// same with any value the route bound out of band in place. args MUST be the
// tokens ValidateArgs refused, in the order it read them. The receiver is not
// changed.
func (e *ArgumentFaults) RestatedForLine(restate func(fault error) error, path, head string, args []string, placedToken, placedLeaf string) *ArgumentFaults {
	if restate == nil {
		restate = keepFault
	}
	subjected := func(fault error) error {
		if replacement := restate(fault); replacement != nil {
			return replacement
		}
		if value, ok := errors.AsType[*ValueArgumentError](fault); ok {
			return value.withSubject(path)
		}
		return nil
	}
	fix := ""
	if e.wholeCommandKnown() {
		fix = e.wholeCommand(head, args, placedToken, placedLeaf)
	}
	return e.Restated(subjected, fix)
}

// keepFault is the restate function that keeps every fault as it is.
func keepFault(error) error {
	return nil
}

// wholeCommandKnown reports whether every fault the refusal holds is a missing
// argument or a value fault typed in the line, with none omitted past the cap:
// only then is the whole command known, because an omitted fault could be a
// typed word at fault.
func (e *ArgumentFaults) wholeCommandKnown() bool {
	if e.omitted > 0 {
		return false
	}
	for _, fault := range e.faults {
		if _, missing := errors.AsType[*MissingArgumentError](fault); missing {
			continue
		}
		value, ok := errors.AsType[*ValueArgumentError](fault)
		if !ok {
			return false
		}
		if value.at < 0 {
			return false
		}
	}
	return true
}

// correctionAt answers the value fault typed at index i of the line, or nil
// when no fault's value starts there.
func (e *ArgumentFaults) correctionAt(i int) *ValueArgumentError {
	for _, fault := range e.faults {
		value, ok := errors.AsType[*ValueArgumentError](fault)
		if !ok {
			continue
		}
		if value.at == i {
			return value
		}
	}
	return nil
}

// wholeCommand writes the fix line RestatedForLine describes.
func (e *ArgumentFaults) wholeCommand(head string, args []string, placedToken, placedLeaf string) string {
	var tb textbuf.Buffer
	tb.Str(head)
	written := false
	skipped := placedToken == ""
	covered := 0
	for i, token := range args {
		if i < covered {
			continue
		}
		if !skipped && token == placedToken {
			skipped = true
			continue
		}
		if e.rest.typed && i == e.rest.at {
			e.writeMissing(&tb, placedLeaf)
			written = true
		}
		tb.Byte(' ')
		if value := e.correctionAt(i); value != nil {
			tb.Str(value.Correction())
			covered = i + max(value.span, 1)
			continue
		}
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

// add keeps fault, named as faultLine names it, or counts it once the list is
// full.
func (l *faultList) add(fault error) {
	l.keep(fault, faultLine(fault))
}

// faultLine answers how fault reads as one entry of a list of faults: a value
// fault as its problem with its why indented under it (never a fix line, which
// only the refusal's last line holds), any other fault as its own text.
func faultLine(fault error) string {
	if value, ok := errors.AsType[*ValueArgumentError](fault); ok {
		return value.listEntry()
	}
	return fault.Error()
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

// verdict answers nil for a line with no fault, an *ArgumentFaults of one for
// a line whose one fault is a value its leaf refuses, so every route restates
// it with the whole command as its fix, the fault itself for a line with one
// fault of another kind, and an *ArgumentFaults for a line with more.
func (l *faultList) verdict() error {
	if len(l.faults) == 0 {
		return nil
	}
	if len(l.faults) == 1 {
		if _, value := errors.AsType[*ValueArgumentError](l.faults[0]); !value {
			return l.faults[0]
		}
	}
	return newArgumentFaults(l.faults, l.lines, l.omitted, l.rest, "")
}
