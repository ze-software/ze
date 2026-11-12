// Design: docs/architecture/config/yang-config-design.md -- Value refusals
// Related: validator_error.go -- the ValidationError that holds the refusal
// Related: format.go -- the format text a pattern refusal shows
// Related: ../../../core/suggest/valuelist.go -- the value lines and the alternation

package yang

import (
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/suggest"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// RefusedValue names what a refusal refused: where, which leaf, the value as
// typed, and whether the leaf holds a secret.
type RefusedValue struct {
	// Path is the setting path as the operator types it, one word per
	// element, list keys included ("bgp", "peer", "peer1", "session", ...).
	Path []string
	// Leaf is the name of the leaf whose restriction refused the value, the
	// word the why line opens with. For a list key it is the key leaf.
	Leaf string
	// Value is the value as typed.
	Value string
	// Secret marks a ze:sensitive or ze:bcrypt leaf: the value slot shows
	// SecretValuePlaceholder, and no suggestion and no length is computed.
	Secret bool
}

// refusalFacts is what an enum, range, length or pattern refusal is built
// from, and the operator error built from it. It is never changed after
// newRefusal or attachRefusal sets it on a ValidationError.
type refusalFacts struct {
	kind       ErrorType
	subject    RefusedValue
	missing    bool            // an enum leaf written with no value
	values     []suggest.Value // enum: the accepted values, in declared order
	suggestion string          // enum: the close accepted value, "" for none
	bounds     string          // range, length: the accepted bounds
	format     ValueFormat     // pattern: the format in words
	message    string          // the error-message the constraint declares
	fix        string          // the fix a frame set, "" for the set form
	// others names the union member types that are not enumerations, in
	// member order (UnionRefusal); nil for a plain enumeration.
	others  []string
	refusal *opererr.Error
}

// UnionRefusal builds the refusal of a value no member type of a union
// accepts (RFC 7950 Section 9.12). values are the accepted values of the
// enumeration members, in member order, each with its ze:help; others names
// every other member type ("ip-address", "uint16"). The refusal is of the
// enum kind: the why lists the values, then names the other types in words,
// and the fix offers a value or a type ("<auto|ip-address>").
func UnionRefusal(subject RefusedValue, values []suggest.Value, others []string) *ValidationError {
	facts := &refusalFacts{kind: ErrTypeEnum, subject: subject, values: values, others: slices.Clone(others)}
	return newRefusal(facts)
}

// alternation is the placeholder of an enum refusal: the accepted values,
// then the other member types of a union, "<a|b|ip-address>".
func (f *refusalFacts) alternation() string {
	if len(f.others) == 0 {
		return suggest.Alternation(f.values)
	}
	choices := slices.Clone(f.values)
	for _, other := range f.others {
		choices = append(choices, suggest.Value{Name: other})
	}
	return suggest.Alternation(choices)
}

// EnumRefusal builds the refusal of a value outside an enumeration. values
// are the accepted values in the order the module declares them, each with
// its ze:help (EnumValuesOf). The problem names the path and the value, and,
// when one accepted value is close and the leaf holds no secret, that value;
// the why lists every accepted value; the fix is the set form.
func EnumRefusal(subject RefusedValue, values []suggest.Value) *ValidationError {
	facts := &refusalFacts{kind: ErrTypeEnum, subject: subject, values: values}
	return newRefusal(facts)
}

// MissingEnumValue builds the refusal of an enum leaf written with no value:
// the problem says it needs one, and the why and the fix are the enum
// refusal's.
func MissingEnumValue(path []string, leaf string, values []suggest.Value) *ValidationError {
	facts := &refusalFacts{kind: ErrTypeEnum, subject: RefusedValue{Path: path, Leaf: leaf}, missing: true, values: values}
	return newRefusal(facts)
}

// RangeRefusal builds the refusal of a number outside its range. bounds is
// the declared range ("1..65535"), message the error-message the range
// declares ("" for none), which then speaks in place of the bounds.
func RangeRefusal(subject RefusedValue, bounds, message string) *ValidationError {
	return newRefusal(&refusalFacts{kind: ErrTypeRange, subject: subject, bounds: bounds, message: message})
}

// LengthRefusal builds the refusal of a string whose length its type refuses.
// bounds is the declared length in characters ("1..4096"), message the
// error-message the length declares ("" for none).
func LengthRefusal(subject RefusedValue, bounds, message string) *ValidationError {
	return newRefusal(&refusalFacts{kind: ErrTypeLength, subject: subject, bounds: bounds, message: message})
}

// PatternRefusal builds the refusal of a string no pattern of its type
// accepts. pattern is the refusing regular expression, kept as Expected for
// the readers that look its message up; it is never shown. format is the
// format in words (LeafValueFormat), message the error-message the pattern
// declares ("" for none).
func PatternRefusal(subject RefusedValue, pattern string, format ValueFormat, message string) *ValidationError {
	refusal := newRefusal(&refusalFacts{kind: ErrTypePattern, subject: subject, format: format, message: message})
	refusal.Expected = pattern
	return refusal
}

// newRefusal builds the ValidationError of facts: the typed fields the
// diagnostic readers take, and the operator error.
func newRefusal(facts *refusalFacts) *ValidationError {
	facts.subject.Path = slices.Clone(facts.subject.Path)
	facts.values = slices.Clone(facts.values)
	refusal := &ValidationError{Path: textbuf.Join(facts.subject.Path, "/"), Type: facts.kind, Got: facts.subject.Value}
	switch facts.kind {
	case ErrTypeEnum:
		refusal.Expected = textbuf.Join(suggest.Names(facts.values), ", ")
	case ErrTypeRange, ErrTypeLength:
		refusal.Expected = facts.bounds
	case ErrTypePattern:
	case ErrTypeUnknown, ErrTypeMissing, ErrTypeType, ErrTypeCardinality, ErrTypeReference:
		panic("BUG: a refusal built for a kind that holds no operator error")
	}
	if facts.kind == ErrTypeLength {
		refusal.Got = strconv.Itoa(utf8Count(facts.subject.Value))
	}
	facts.build()
	refusal.Message = facts.refusal.Problem()
	refusal.facts = facts
	return refusal
}

// build computes the suggestion and the operator error from the other facts.
// The suggestion is suggest.Correction, the bound for a value that goes into
// a fix line, not the looser one command suggestions use.
func (f *refusalFacts) build() {
	f.suggestion = ""
	if f.kind == ErrTypeEnum && !f.missing && !f.subject.Secret {
		f.suggestion = suggest.Correction(f.subject.Value, suggest.Names(f.values))
	}
	fix := f.fix
	if fix == "" {
		fix = "set " + strings.Join(f.subject.Path, " ") + " " + f.correction()
	}
	f.refusal = opererr.New(f.problem(), f.why(), fix)
}

// problem is the refusal's first line: the path, the value and the kind.
func (f *refusalFacts) problem() string {
	var b textbuf.Buffer
	b.Join(f.subject.Path, " ").Str(": ")
	if f.missing {
		return b.Str("needs a value").String()
	}
	if f.subject.Secret {
		b.Str(SecretValuePlaceholder)
	} else {
		b.Quoted(f.subject.Value)
	}
	switch f.kind {
	case ErrTypeEnum:
		b.Str(" is not an accepted value")
		if f.suggestion != "" {
			b.Str(" (did you mean ").Str(f.suggestion).Str("?)")
		}
	case ErrTypeRange:
		b.Str(" is outside the accepted range")
	case ErrTypeLength:
		b.Str(" is outside the accepted length")
		if !f.subject.Secret {
			b.Str(" (").Int(int64(utf8Count(f.subject.Value))).Str(" characters)")
		}
	case ErrTypePattern:
		b.Str(" is not in the accepted format")
	case ErrTypeUnknown, ErrTypeMissing, ErrTypeType, ErrTypeCardinality, ErrTypeReference:
		panic("BUG: a refusal built for a kind that holds no operator error")
	}
	return b.String()
}

// why is the refusal's explanation: the accepted values, bounds or format,
// or the error-message the constraint declares (RFC 7950 Section 7.5.4.1).
func (f *refusalFacts) why() string {
	if f.message != "" {
		return f.message
	}
	var b textbuf.Buffer
	b.Str(f.subject.Leaf)
	switch f.kind {
	case ErrTypeEnum:
		if len(f.others) == 0 {
			return b.Str(" accepts only these values:\n").Str(suggest.ValueLines(f.values, "  ")).String()
		}
		if len(f.values) == 0 {
			return b.Str(" accepts a value of type ").Join(f.others, " or ").String()
		}
		return b.Str(" accepts one of these values:\n").Str(suggest.ValueLines(f.values, "  ")).
			Str("\nor a value of type ").Join(f.others, " or ").String()
	case ErrTypeRange:
		return b.Str(" accepts a value in ").Str(f.bounds).String()
	case ErrTypeLength:
		return b.Str(" accepts ").Str(f.bounds).Str(" characters").String()
	case ErrTypePattern:
		if f.format.Text() != "" {
			return b.Str(" accepts ").Str(f.format.Text()).String()
		}
		if f.format.TypeName() != "" {
			return b.Str(" accepts a value of type ").Str(f.format.TypeName()).String()
		}
		return b.Str(" does not accept this value").String()
	case ErrTypeUnknown, ErrTypeMissing, ErrTypeType, ErrTypeCardinality, ErrTypeReference:
		panic("BUG: a refusal built for a kind that holds no operator error")
	}
	panic("BUG: unreachable refusal kind")
}

// correction is what replaces the refused value in a fix line: the suggestion,
// else the kind's placeholder.
func (f *refusalFacts) correction() string {
	switch f.kind {
	case ErrTypeEnum:
		if f.suggestion != "" {
			return f.suggestion
		}
		// A leaf that accepts one value has one correction, the value itself:
		// the fix line is then the exact text to type, not a choice of one.
		if len(f.values) == 1 && len(f.others) == 0 {
			return f.values[0].Name
		}
		return f.alternation()
	case ErrTypeRange:
		return "<" + f.bounds + ">"
	case ErrTypeLength:
		return "<" + f.bounds + " characters>"
	case ErrTypePattern:
		return f.format.Placeholder()
	case ErrTypeUnknown, ErrTypeMissing, ErrTypeType, ErrTypeCardinality, ErrTypeReference:
		panic("BUG: a refusal built for a kind that holds no operator error")
	}
	panic("BUG: unreachable refusal kind")
}

// HoldsOperatorError reports whether e is an enum, range, length or pattern
// refusal holding an operator error, the only kind WithFix and WithPath take.
func (e *ValidationError) HoldsOperatorError() bool {
	return e.facts != nil
}

// WithFix answers a copy of the refusal whose fix line is fix, the exact
// command the operator types on the surface that frames it. The problem, the
// why and every typed fact are the original's. MUST be called only on a
// refusal that HoldsOperatorError, with a non-empty fix.
func (e *ValidationError) WithFix(fix string) *ValidationError {
	if fix == "" {
		panic("BUG: ValidationError.WithFix called with an empty fix")
	}
	facts := e.copyFacts("WithFix")
	facts.fix = fix
	return e.rebuilt(facts)
}

// WithPath answers a copy of the refusal whose problem names path, the
// setting path as the operator types it, and whose set-form fix names it too.
// A fix a frame set is kept. Every other fact is the original's. MUST be
// called only on a refusal that HoldsOperatorError.
func (e *ValidationError) WithPath(path []string) *ValidationError {
	facts := e.copyFacts("WithPath")
	facts.subject.Path = slices.Clone(path)
	return e.rebuilt(facts)
}

// Segments answers the setting path the refusal names, one word per element.
func (e *ValidationError) Segments() []string {
	if e.facts == nil {
		return nil
	}
	return slices.Clone(e.facts.subject.Path)
}

// Leaf answers the name of the leaf whose restriction refused the value, ""
// for a kind that holds no operator error.
func (e *ValidationError) Leaf() string {
	if e.facts == nil {
		return ""
	}
	return e.facts.subject.Leaf
}

// Correction answers what replaces the refused value in a fix line: the close
// accepted value, else the alternation of the accepted values or the kind's
// placeholder ("<1..65535>", "<rate>"). A surface that frames its own fix
// (the file form "route-refresh require;") writes it with this. "" for a kind
// that holds no operator error.
func (e *ValidationError) Correction() string {
	if e.facts == nil {
		return ""
	}
	return e.facts.correction()
}

// Values answers the accepted values of an enum refusal, in declared order.
func (e *ValidationError) Values() []suggest.Value {
	if e.facts == nil {
		return nil
	}
	return slices.Clone(e.facts.values)
}

// Format answers the format in words a pattern refusal shows, the zero value
// for any other kind.
func (e *ValidationError) Format() ValueFormat {
	if e.facts == nil {
		return ValueFormat{}
	}
	return e.facts.format
}

// Secret reports whether the refused value is a secret the refusal masks.
func (e *ValidationError) Secret() bool {
	return e.facts != nil && e.facts.subject.Secret
}

// copyFacts answers a copy of e's facts for a frame to change, and panics
// with BUG when e holds none: only a Ze caller that skipped
// HoldsOperatorError reaches that.
func (e *ValidationError) copyFacts(frame string) *refusalFacts {
	if e.facts == nil {
		panic("BUG: ValidationError." + frame + " called on a " + e.Type.String() + " error, which holds no operator error")
	}
	facts := *e.facts
	facts.subject.Path = slices.Clone(facts.subject.Path)
	return &facts
}

// rebuilt answers a copy of e holding facts, its operator error built again.
func (e *ValidationError) rebuilt(facts *refusalFacts) *ValidationError {
	facts.build()
	copied := *e
	copied.facts = facts
	return &copied
}

// attachRefusal gives a refusal the tree walk built its operator error, from
// the leaf that holds the value: the setting path, the leaf's name, the
// accepted values with their help in declared order, the bounds, the format
// and the error-message its constraint declares. A kind that holds no
// operator error, and a pattern refusal that names no pattern (the pattern
// did not compile, a defect of the module), are left as built.
func attachRefusal(refusal *ValidationError, leaf *gyang.Entry, yt *gyang.YangType, value any, messages ConstraintMessages) {
	facts := &refusalFacts{kind: refusal.Type, subject: RefusedValue{
		Path:   pathWords(refusal.Path),
		Leaf:   leaf.Name,
		Value:  refusal.Got,
		Secret: EntryHoldsSecret(leaf),
	}}
	switch refusal.Type {
	case ErrTypeEnum:
		facts.values = EnumValuesOf(leaf, yt)
		refusal.Expected = textbuf.Join(suggest.Names(facts.values), ", ")
	case ErrTypeRange:
		facts.bounds = refusal.Expected
		facts.message = messages.Range()
	case ErrTypeLength:
		facts.bounds = refusal.Expected
		facts.message = messages.Length()
		// The length arm writes the character count to Got; the refusal
		// quotes the value itself, which validateString took as a string.
		if typed, isString := value.(string); isString {
			facts.subject.Value = typed
		}
	case ErrTypePattern:
		if refusal.Expected == "" {
			return
		}
		facts.format = LeafValueFormat(leaf, yt)
		facts.message = messages.Pattern(refusal.Expected)
	case ErrTypeUnknown, ErrTypeMissing, ErrTypeType, ErrTypeCardinality, ErrTypeReference:
		return
	}
	facts.build()
	refusal.facts = facts
}

// pathWords splits a walk path ("bgp/peer[peer1]/session") into the words an
// operator types ("bgp", "peer", "peer1", "session"). A list key sits in
// brackets and may hold a slash (a prefix key), so a slash inside brackets
// does not split.
func pathWords(path string) []string {
	var words []string
	depth := 0
	start := 0
	for i := range len(path) {
		switch path[i] {
		case '[':
			if depth == 0 {
				words = appendWord(words, path[start:i])
				start = i + 1
			}
			depth++
		case ']':
			depth--
			if depth == 0 {
				words = appendWord(words, path[start:i])
				start = i + 1
			}
		case '/':
			if depth == 0 {
				words = appendWord(words, path[start:i])
				start = i + 1
			}
		}
	}
	return appendWord(words, path[start:])
}

// appendWord appends word to words unless it is empty.
func appendWord(words []string, word string) []string {
	if word == "" {
		return words
	}
	return append(words, word)
}

// utf8Count counts the characters of value, as RFC 7950 Section 9.4.4 counts
// a length.
func utf8Count(value string) int {
	return utf8.RuneCountInString(value)
}
