// Design: docs/architecture/api/commands.md -- YANG-typed argument validation
// Related: argvalidate.go -- the validators that build a value fault
// Related: argfaults.go -- ArgumentFaults, which restates a value fault with the whole command
// Related: ../../core/suggest/valuelist.go -- the value list a config refusal writes too
//
// argvalue.go holds the refusal of a value its argument's YANG type refuses:
// a word outside an enumeration or a union, a number outside its range, a
// string outside its length or its pattern. It reads like the config value
// refusal (internal/component/config/yang, refusal.go), because an operator
// meets both: the argument named, the value quoted, the accepted values or
// bounds as the reason, and the value that replaces it in the fix.

package command

import (
	"strconv"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/suggest"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ValueArgumentError is the fault of one typed value its argument refuses. It
// is a fault, not a refusal: it holds the problem and the why, and the
// *ArgumentFaults that holds it writes the one fix line, the whole command with
// Correction in place of the value when a route restates it
// (ArgumentFaults.RestatedForLine). A line whose one fault is a value fault is
// an *ArgumentFaults of one, so every route restates it.
//
// Built by the validators of argvalidate.go; the zero value names nothing.
// Immutable after construction, safe for concurrent use.
type ValueArgumentError struct {
	// Leaf is the argument whose type refused the value.
	Leaf string
	// subject is the words before the problem's colon: Leaf, or the command
	// path and Leaf once a route restated the fault (withSubject).
	subject string
	// value is the refused value as the problem quotes it (echoValue).
	value string
	// marker is what follows the quoted value: the kind, and a close accepted
	// value when there is one.
	marker string
	// why is the reason, one or more lines: the accepted values, bounds or
	// format, or the error-message the restriction declares.
	why string
	// correction replaces the value in a fix line.
	correction string
	// literalCorrection distinguishes an accepted enum value from correction
	// syntax. Even an accepted value that looks like <a|b> is literal data.
	literalCorrection bool
	// at is the index in the refused line of the first token the value was
	// typed as, and span how many tokens it took (a rest-of-line value takes
	// several). at is -1 for a value the line does not hold, one a route bound
	// out of band.
	at   int
	span int
}

// Error answers the fault as it reads alone: the problem, then the why.
func (e *ValueArgumentError) Error() string {
	var tb textbuf.Buffer
	return tb.Str(e.Problem()).Byte('\n').Str(e.why).String()
}

// Problem answers the fault's first line: the argument, the value and the kind
// (`encoding: "gzip" is not an accepted value`).
func (e *ValueArgumentError) Problem() string {
	var tb textbuf.Buffer
	return tb.Str(e.subject).Str(": ").Str(strconv.Quote(e.value)).Str(e.marker).String()
}

// Why answers the reason the value is refused, one or more lines.
func (e *ValueArgumentError) Why() string {
	return e.why
}

// Correction answers what replaces the value in a fix line: the close accepted
// value, the only accepted value, or the kind's placeholder ("<hex|b64>",
// "<1..4094>", "<value>").
func (e *ValueArgumentError) Correction() string {
	return e.correction
}

// writeCorrection quotes concrete accepted values in the operational grammar,
// while preserving placeholders as syntax the operator must replace.
func (e *ValueArgumentError) writeCorrection(tb *textbuf.Buffer) {
	if e.literalCorrection {
		WriteInvocationValue(tb, e.correction)
		return
	}
	tb.Str(e.correction)
}

// Value answers the refused value as the problem quotes it.
func (e *ValueArgumentError) Value() string {
	return e.value
}

// listEntry answers the fault as one entry of a list of faults: the problem,
// then each line of the why indented under it. It holds no fix line, because
// the list's one fix is the last line of the refusal.
func (e *ValueArgumentError) listEntry() string {
	var tb textbuf.Buffer
	tb.Str(e.Problem())
	start := 0
	for i := 0; i <= len(e.why); i++ {
		if i < len(e.why) && e.why[i] != '\n' {
			continue
		}
		tb.Str("\n  ").Str(e.why[start:i])
		start = i + 1
	}
	return tb.String()
}

// withSubject answers a copy whose problem names path before the argument, the
// command as the operator types it with no value in it ("send bgp raw").
func (e *ValueArgumentError) withSubject(path string) *ValueArgumentError {
	restated := *e
	var tb textbuf.Buffer
	restated.subject = tb.Str(path).Byte(' ').Str(e.Leaf).String()
	return &restated
}

// typedAt answers a copy that records the value as typed at index at of the
// line, over span tokens.
func (e *ValueArgumentError) typedAt(at, span int) *ValueArgumentError {
	typed := *e
	typed.at, typed.span = at, span
	return &typed
}

// newValueFault builds the fault of value refused by def, with marker after the
// quoted value, why as the reason and correction for the fix line. The value
// is not yet placed in a line (at -1).
func newValueFault(def *ArgDef, value, marker, why, correction string) *ValueArgumentError {
	return &ValueArgumentError{
		Leaf:       def.name,
		subject:    def.name,
		value:      echoValue(value),
		marker:     marker,
		why:        why,
		correction: correction,
		at:         -1,
	}
}

// enumFault is the refusal of a value outside def's enumeration: a close
// accepted value is named in the problem and replaces the value in the fix;
// the why lists every accepted value with its help, in the model's order.
func enumFault(arg string, def *ArgDef) *ValueArgumentError {
	values := acceptedValues(def)
	suggestion := suggest.Correction(arg, def.enumValues)
	var why textbuf.Buffer
	why.Str(def.name).Str(" accepts only these values:\n").Str(suggest.ValueLines(values, "  "))
	correction, literal := enumCorrection(suggestion, values, nil)
	fault := newValueFault(def, arg, enumMarker(suggestion), why.String(), correction)
	fault.literalCorrection = literal
	return fault
}

// unionFault is the refusal of a value no member of def's union accepts: the
// why lists the values of its enumeration members, then names the other member
// types in words.
func unionFault(arg string, def *ArgDef) *ValueArgumentError {
	values := acceptedValues(def)
	others := make([]string, 0, len(def.unionDefs))
	for i := range def.unionDefs {
		if def.unionDefs[i].kind != ArgEnum {
			others = append(others, memberTypeName(&def.unionDefs[i]))
		}
	}
	suggestion := suggest.Correction(arg, def.enumValues)
	var why textbuf.Buffer
	why.Str(def.name)
	switch {
	case len(others) == 0:
		why.Str(" accepts only these values:\n").Str(suggest.ValueLines(values, "  "))
	case len(values) == 0:
		why.Str(" accepts a value of type ").Join(others, " or ")
	default:
		why.Str(" accepts one of these values:\n").Str(suggest.ValueLines(values, "  ")).
			Str("\nor a value of type ").Join(others, " or ")
	}
	correction, literal := enumCorrection(suggestion, values, others)
	fault := newValueFault(def, arg, enumMarker(suggestion), why.String(), correction)
	fault.literalCorrection = literal
	return fault
}

// enumMarker is the problem's kind for a refused word, with the close accepted
// value when there is one.
func enumMarker(suggestion string) string {
	if suggestion == "" {
		return " is not an accepted value"
	}
	var tb textbuf.Buffer
	return tb.Str(" is not an accepted value (did you mean ").Str(suggestion).Str("?)").String()
}

// enumCorrection is what replaces a refused word in a fix line: the close
// value, the one accepted value when the argument takes one, else every value
// and every other member type as the alternation.
func enumCorrection(suggestion string, values []suggest.Value, others []string) (text string, literal bool) {
	if suggestion != "" {
		return suggestion, true
	}
	if len(values) == 1 && len(others) == 0 {
		return values[0].Name, true
	}
	choices := values
	for _, other := range others {
		choices = append(choices, suggest.Value{Name: other})
	}
	return suggest.Alternation(choices), false
}

// memberTypeName names a union member type in words: its YANG base type, or
// the typedef its pattern comes from.
func memberTypeName(def *ArgDef) string {
	switch def.kind {
	case ArgUint:
		var tb textbuf.Buffer
		return tb.Str("uint").Int(int64(def.uintBits)).String()
	case ArgString:
		if def.formatType != "" {
			return def.formatType
		}
		return "string"
	case ArgUnion:
		return "union"
	case ArgFlag:
		return "empty"
	case ArgEnum:
		return "enumeration"
	default:
		panic("BUG: invalid command argument kind")
	}
}

// rangeFault is the refusal of a number outside def's declared ranges.
func rangeFault(arg string, def *ArgDef) *ValueArgumentError {
	bounds := joinRanges(def.ranges)
	why := def.rangeMessage
	if why == "" {
		var tb textbuf.Buffer
		why = tb.Str(def.name).Str(" accepts a value in ").Str(bounds).String()
	}
	return newValueFault(def, arg, " is outside the accepted range", why, "<"+bounds+">")
}

// lengthFault is the refusal of a string outside def's declared length, which
// counts characters (RFC 7950 Section 9.4.4).
func lengthFault(arg string, def *ArgDef) *ValueArgumentError {
	bounds := joinRanges(def.lengths)
	why := def.lengthMessage
	if why == "" {
		var tb textbuf.Buffer
		why = tb.Str(def.name).Str(" accepts ").Str(bounds).Str(" characters").String()
	}
	var marker textbuf.Buffer
	marker.Str(" is outside the accepted length (").Int(int64(utf8.RuneCountInString(arg))).Str(" characters)")
	return newValueFault(def, arg, marker.String(), why, "<"+bounds+" characters>")
}

// patternFault is the refusal of a string the pattern at index i of def does
// not match. The why is the error-message the pattern declares, else the
// format in words, else the typedef's name; never the regular expression.
func patternFault(arg string, def *ArgDef, i int) *ValueArgumentError {
	why := ""
	if i < len(def.patternMessages) {
		why = def.patternMessages[i]
	}
	if why == "" {
		var tb textbuf.Buffer
		tb.Str(def.name)
		switch {
		case def.formatText != "":
			tb.Str(" accepts ").Str(def.formatText)
		case def.formatType != "":
			tb.Str(" accepts a value of type ").Str(def.formatType)
		default:
			tb.Str(" does not accept this value")
		}
		why = tb.String()
	}
	placeholder := "<value>"
	if def.formatType != "" {
		placeholder = "<" + def.formatType + ">"
	}
	return newValueFault(def, arg, " is not in the accepted format", why, placeholder)
}
