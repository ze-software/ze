// Design: docs/architecture/fleet-interface.md -- the filter language of the fleet tables
// Related: table.go -- QueryDevices, which applies a filter before it sorts
// Related: register_devices.go -- the fields of the devices table
//
// One text box and one small grammar serve every fleet table. A filter is
// terms separated by spaces, and a row matches when every term holds:
//
//	lon                  a bare word: the row holds it, ignoring case
//	state:offline        the field equals the value
//	state:degraded,down  the field equals any of the values
//	version:!1.4.2       the field equals none of the values
//	cpu:>80  cpu:<10     the number field is above, or below, the number
//	uptime:>30d          the duration field, with a unit: s, m, h, d or w
//	label:customer=acme  the row carries that label
//	site:lon             the same, for a label key in use
//	has:alarm            the row has what the flag names
//
// The fields and the flags of a table are a registered set, so the alarms,
// tasks and audit tables register their own and share the grammar. A term
// the grammar does not accept is refused with an operator error that names
// the term and lists the fields.

package fleetui

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The bounds of a filter.
const (
	// FilterLengthMax is the longest filter, in characters.
	FilterLengthMax = 512
	// FilterTermsMax is the most terms one filter holds.
	FilterTermsMax = 32
)

// The fixed words of the grammar.
const (
	filterFieldSeparator = ":"
	filterListSeparator  = ","
	filterNegation       = "!"
	filterLabelKey       = "label"
	filterFlagKey        = "has"
	filterAbove          = '>'
	filterBelow          = '<'
)

// ErrFilterRefused means the filter text is not one the grammar accepts. The
// table then keeps its previous rows and shows the refusal.
var ErrFilterRefused = errors.New("the filter was refused")

// filterWhy is the consequence every filter refusal states.
const filterWhy = "The filter was not applied, so the table still shows its previous rows."

// wordClearFilter labels the action that drops a table's filter, offered
// where the filter matched nothing or was refused.
const wordClearFilter = "Clear the filter"

// rowTest reports whether one row holds what a term asks.
type rowTest[T any] func(row *T) bool

// FilterField is one field a filter term may name, over rows of type T. Build
// one with TextField, NumberField or DurationField.
type FilterField[T any] struct {
	name string
	// values is the closed set of values a text field takes, or nil for a
	// field that takes any.
	values []string
	// compile turns the value of a term that names the field into the test
	// of that term.
	compile func(schema *FilterSchema[T], term, value string) (rowTest[T], error)
}

// Name answers the field's name in a term.
func (field FilterField[T]) Name() string {
	return field.name
}

// TextField builds a field compared as text, ignoring case. values is the
// closed set of values the field takes, which a term outside it is refused
// against and an assisted menu lists; pass nil for a field that takes any.
func TextField[T any](name string, values []string, text func(row *T) string) FilterField[T] {
	// The field keeps its own copy, so the caller's slice can change after.
	allowed := slices.Clone(values)

	return FilterField[T]{
		name:   name,
		values: allowed,
		compile: func(schema *FilterSchema[T], term, value string) (rowTest[T], error) {
			return schema.compileText(term, value, allowed, text)
		},
	}
}

// NumberField builds a field compared as a number. number answers false for
// a row that has no value, which then matches no term on the field.
func NumberField[T any](name string, number func(row *T) (float64, bool)) FilterField[T] {
	return FilterField[T]{
		name: name,
		compile: func(schema *FilterSchema[T], term, value string) (rowTest[T], error) {
			return schema.compileNumber(term, value, parsePlainNumber, number)
		},
	}
}

// DurationField builds a field compared as a duration, written as a number
// and a unit: s, m, h, d or w. seconds answers the row's value in seconds, or
// false for a row that has none.
func DurationField[T any](name string, seconds func(row *T) (float64, bool)) FilterField[T] {
	return FilterField[T]{
		name: name,
		compile: func(schema *FilterSchema[T], term, value string) (rowTest[T], error) {
			return schema.compileNumber(term, value, parseDurationSeconds, seconds)
		},
	}
}

// filterFlag is one thing a has: term may ask for.
type filterFlag[T any] struct {
	name string
	test rowTest[T]
}

// FilterSchema is the filter grammar of one table: its fields, its flags and
// how a bare word and a label are read from a row. The fields and the flags
// are a registered set. Build one with NewFilterSchema. Safe for concurrent
// use.
type FilterSchema[T any] struct {
	subject string
	bare    func(row *T, needle string) bool
	// label answers a label of a row, or nil for a table whose rows carry no
	// labels: the label forms are then not part of its grammar.
	label func(row *T, key string) (string, bool)

	mu     sync.RWMutex
	fields []FilterField[T]
	flags  []filterFlag[T]
}

// NewFilterSchema builds the grammar of one table with no field and no flag.
// subject names the table's rows in a refusal, such as "devices". bare
// reports whether a row holds a bare word, which arrives in lower case. label
// answers one label of a row: pass nil for rows that carry no labels.
func NewFilterSchema[T any](
	subject string,
	bare func(row *T, needle string) bool,
	label func(row *T, key string) (string, bool),
) *FilterSchema[T] {
	if bare == nil {
		panic("BUG: fleetui.NewFilterSchema: " + subject + " has no test for a bare word")
	}

	return &FilterSchema[T]{subject: subject, bare: bare, label: label}
}

// MustRegisterField adds a field to the grammar. Call it from init() in a
// register file. It panics on a field built without a constructor, on a name
// the grammar reserves, and on a name already registered: each is a defect of
// the build's composition.
func (schema *FilterSchema[T]) MustRegisterField(field FilterField[T]) {
	if field.compile == nil {
		registrationBug(bugFilterField, field.name, schema.subject, "was built without a constructor")
	}
	if field.name == "" {
		registrationBug(bugFilterField, field.name, schema.subject, bugUnnamed)
	}
	if field.name == filterLabelKey {
		registrationBug(bugFilterField, field.name, schema.subject, bugReserved)
	}
	if field.name == filterFlagKey {
		registrationBug(bugFilterField, field.name, schema.subject, bugReserved)
	}

	schema.mu.Lock()
	defer schema.mu.Unlock()

	if slices.ContainsFunc(schema.fields, func(held FilterField[T]) bool { return held.name == field.name }) {
		registrationBug(bugFilterField, field.name, schema.subject, bugTwice)
	}
	schema.fields = append(schema.fields, field)
}

// What a refused registration of this file names.
const (
	bugFilterField = "the filter field"
	bugFilterFlag  = "the filter flag"
	bugReserved    = "has a name the filter grammar reserves"
)

// MustRegisterFlag adds a flag, which a has: term names. Call it from init()
// in a register file. It panics on an unnamed flag, a flag with no test and a
// duplicate.
func (schema *FilterSchema[T]) MustRegisterFlag(name string, test func(row *T) bool) {
	if name == "" {
		registrationBug(bugFilterFlag, name, schema.subject, bugUnnamed)
	}
	if test == nil {
		registrationBug(bugFilterFlag, name, schema.subject, "has no test")
	}

	schema.mu.Lock()
	defer schema.mu.Unlock()

	if slices.ContainsFunc(schema.flags, func(held filterFlag[T]) bool { return held.name == name }) {
		registrationBug(bugFilterFlag, name, schema.subject, bugTwice)
	}
	schema.flags = append(schema.flags, filterFlag[T]{name: name, test: test})
}

// Fields answers the names of the registered fields, in registration order.
// An assisted filter menu and a refusal list them.
func (schema *FilterSchema[T]) Fields() []string {
	schema.mu.RLock()
	defer schema.mu.RUnlock()

	names := make([]string, 0, len(schema.fields))
	for _, field := range schema.fields {
		names = append(names, field.name)
	}

	return names
}

// FieldValues answers the closed set of values the named field takes, for an
// assisted filter menu. It answers false for an unknown field and for a field
// that takes any value.
func (schema *FilterSchema[T]) FieldValues(name string) ([]string, bool) {
	field, known := schema.field(name)
	if !known {
		return nil, false
	}
	if field.values == nil {
		return nil, false
	}

	return slices.Clone(field.values), true
}

// Flags answers the names of the registered flags, in registration order.
func (schema *FilterSchema[T]) Flags() []string {
	schema.mu.RLock()
	defer schema.mu.RUnlock()

	names := make([]string, 0, len(schema.flags))
	for _, flag := range schema.flags {
		names = append(names, flag.name)
	}

	return names
}

// field answers the registered field named name, or false.
func (schema *FilterSchema[T]) field(name string) (FilterField[T], bool) {
	schema.mu.RLock()
	defer schema.mu.RUnlock()

	for _, field := range schema.fields {
		if field.name == name {
			return field, true
		}
	}

	return FilterField[T]{}, false
}

// flag answers the test of the registered flag named name, or false.
func (schema *FilterSchema[T]) flag(name string) (rowTest[T], bool) {
	schema.mu.RLock()
	defer schema.mu.RUnlock()

	for _, flag := range schema.flags {
		if flag.name == name {
			return flag.test, true
		}
	}

	return nil, false
}

// Filter is a parsed filter over rows of type T. The zero value is the empty
// filter, which every row matches. Only FilterSchema.Parse builds any other.
type Filter[T any] struct {
	text  string
	tests []rowTest[T]
}

// Text answers the filter as an address carries it: its terms separated by
// one space.
func (filter Filter[T]) Text() string {
	return filter.text
}

// Match reports whether row holds every term of the filter.
func (filter Filter[T]) Match(row *T) bool {
	for _, test := range filter.tests {
		if !test(row) {
			return false
		}
	}

	return true
}

// Apply answers a new slice holding the rows that match, in their order.
func (filter Filter[T]) Apply(rows []T) []T {
	kept := make([]T, 0, len(rows))
	for index := range rows {
		if filter.Match(&rows[index]) {
			kept = append(kept, rows[index])
		}
	}

	return kept
}

// Parse reads a filter. labelKeys holds the label keys in use among the rows
// the filter will be applied to: a term whose field is one of them is read as
// a label term.
//
// It refuses, with an operator error wrapping ErrFilterRefused, a filter
// longer than FilterLengthMax characters or FilterTermsMax terms, a term that
// names an unknown field, and a term the grammar does not accept.
func (schema *FilterSchema[T]) Parse(text string, labelKeys []string) (Filter[T], error) {
	if utf8.RuneCountInString(text) > FilterLengthMax {
		return Filter[T]{}, schema.refuse(
			"The filter is longer than "+strconv.Itoa(FilterLengthMax)+" characters.",
			"shorten the filter to "+strconv.Itoa(FilterLengthMax)+" characters or fewer")
	}

	terms := strings.Fields(text)
	if len(terms) > FilterTermsMax {
		return Filter[T]{}, schema.refuse(
			"The filter holds more than "+strconv.Itoa(FilterTermsMax)+" terms.",
			"write the filter with "+strconv.Itoa(FilterTermsMax)+" terms or fewer")
	}

	filter := Filter[T]{
		text:  textbuf.New().Join(terms, " ").Slice(),
		tests: make([]rowTest[T], 0, len(terms)),
	}
	for _, term := range terms {
		test, err := schema.compileTerm(term, labelKeys)
		if err != nil {
			return Filter[T]{}, err
		}
		filter.tests = append(filter.tests, test)
	}

	return filter, nil
}

// compileTerm turns one term into its test.
func (schema *FilterSchema[T]) compileTerm(term string, labelKeys []string) (rowTest[T], error) {
	name, value, fielded := strings.Cut(term, filterFieldSeparator)
	if !fielded {
		needle := strings.ToLower(term)
		return func(row *T) bool { return schema.bare(row, needle) }, nil
	}
	if name == "" {
		return nil, schema.malformed(term, "names no field before the colon")
	}
	if value == "" {
		return nil, schema.malformed(term, "gives no value after the colon")
	}

	if field, known := schema.field(name); known {
		return field.compile(schema, term, value)
	}
	if name == filterFlagKey {
		return schema.compileFlag(term, value)
	}
	if schema.label == nil {
		return nil, schema.unknownField(term, name)
	}
	if name == filterLabelKey {
		return schema.compileLabel(term, value)
	}
	if slices.Contains(labelKeys, name) {
		return schema.compileText(term, value, nil, schema.labelText(name))
	}

	return nil, schema.unknownField(term, name)
}

// labelText answers the reader of one label as the text of a row: the empty
// text for a row that does not carry the key.
func (schema *FilterSchema[T]) labelText(key string) func(row *T) string {
	return func(row *T) string {
		value, _ := schema.label(row, key)
		return value
	}
}

// compileFlag turns the value of a has: term into the test of its flag.
func (schema *FilterSchema[T]) compileFlag(term, name string) (rowTest[T], error) {
	test, known := schema.flag(name)
	if known {
		return test, nil
	}

	flags := schema.Flags()
	if len(flags) == 0 {
		return nil, schema.unknownField(term, filterFlagKey)
	}

	return nil, schema.refuse(
		termProblem(term).Str(" asks for ").Quoted(name).
			Str(", which is not something the table of ").Str(schema.subject).Str(" can test for.").Slice(),
		textbuf.New().Str("write ").Str(filterFlagKey).Str(filterFieldSeparator).
			Str(" with one of: ").Join(flags, ", ").Slice())
}

// compileLabel turns the value of a label: term, key=value, into the test
// that a row carries that label.
func (schema *FilterSchema[T]) compileLabel(term, pair string) (rowTest[T], error) {
	key, wanted, valued := strings.Cut(pair, groupSeparator)
	if !valued {
		return nil, schema.malformed(term, "does not give the label as key=value")
	}
	if key == "" {
		return nil, schema.malformed(term, "gives no label key before the equals sign")
	}
	if wanted == "" {
		return nil, schema.malformed(term, "gives no label value after the equals sign")
	}

	return func(row *T) bool {
		value, carried := schema.label(row, key)
		return carried && strings.EqualFold(value, wanted)
	}, nil
}

// compileText turns the value of a text term into its test: one value, a
// list of values, or either after a "!" for none of them. allowed is the
// closed set of values the field takes, or nil.
func (schema *FilterSchema[T]) compileText(
	term, value string, allowed []string, text func(row *T) string,
) (rowTest[T], error) {
	if isComparison(value) {
		return nil, schema.malformed(term, "compares with > or <, which only a number field takes")
	}

	list, negated := strings.CutPrefix(value, filterNegation)
	if list == "" {
		return nil, schema.malformed(term, "gives no value after the exclamation mark")
	}

	wanted := strings.Split(list, filterListSeparator)
	for _, option := range wanted {
		if option == "" {
			return nil, schema.malformed(term, "holds an empty value in its list")
		}
		if allowed == nil {
			continue
		}
		if !containsFold(allowed, option) {
			return nil, schema.refuse(
				termProblem(term).Str(" gives the value ").Quoted(option).
					Str(", which its field does not take.").Slice(),
				textbuf.New().Str("write the term with one of: ").Join(allowed, ", ").Slice())
		}
	}

	return func(row *T) bool { return containsFold(wanted, text(row)) != negated }, nil
}

// compileNumber turns the value of a number or duration term into its test:
// above a bound, below a bound, or equal to it. A row with no value matches
// none of the three.
func (schema *FilterSchema[T]) compileNumber(
	term, value string, parse func(text string) (float64, error), number func(row *T) (float64, bool),
) (rowTest[T], error) {
	operand := value
	if isComparison(value) {
		operand = value[1:]
	}

	bound, err := parse(operand)
	if err != nil {
		return nil, schema.malformed(term, err.Error())
	}

	switch value[0] {
	case filterAbove:
		return func(row *T) bool {
			got, reported := number(row)
			return reported && got > bound
		}, nil
	case filterBelow:
		return func(row *T) bool {
			got, reported := number(row)
			return reported && got < bound
		}, nil
	}

	return func(row *T) bool {
		got, reported := number(row)
		return reported && got == bound
	}, nil
}

// isComparison reports whether a term's value opens with > or <. value MUST
// NOT be empty.
func isComparison(value string) bool {
	if value[0] == filterAbove {
		return true
	}

	return value[0] == filterBelow
}

// containsFold reports whether options holds text, ignoring case.
func containsFold(options []string, text string) bool {
	return slices.ContainsFunc(options, func(option string) bool { return strings.EqualFold(option, text) })
}

// The reasons a number or a duration does not parse. Each completes the
// sentence "The filter term X ...".
var (
	errNotNumber   = errors.New("does not give a number to compare with")
	errNotDuration = errors.New("does not give a duration as a number and a unit: s, m, h, d or w")
)

// parsePlainNumber reads the operand of a number term.
func parsePlainNumber(text string) (float64, error) {
	number, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, errNotNumber
	}

	return number, nil
}

// durationUnitSeconds holds the seconds of each unit a duration term takes.
var durationUnitSeconds = map[byte]float64{
	's': 1,
	'm': 60,
	'h': 3600,
	'd': 86400,
	'w': 604800,
}

// parseDurationSeconds reads the operand of a duration term, a number and a
// unit, into seconds.
func parseDurationSeconds(text string) (float64, error) {
	if len(text) < 2 {
		return 0, errNotDuration
	}

	unit, known := durationUnitSeconds[text[len(text)-1]]
	if !known {
		return 0, errNotDuration
	}

	count, err := strconv.ParseFloat(text[:len(text)-1], 64)
	if err != nil {
		return 0, errNotDuration
	}

	return count * unit, nil
}

// malformed is the refusal for a term the grammar does not accept. reason
// completes the sentence that names the term.
func (schema *FilterSchema[T]) malformed(term, reason string) error {
	problem := termProblem(term).Byte(' ').Str(reason).Str(".").Slice()

	return schema.refuse(problem, schema.grammarFix())
}

// termProblem opens the problem line of a refused term: it names the term,
// and the caller completes the sentence.
func termProblem(term string) *textbuf.Buffer {
	return textbuf.New().Str("The filter term ").Quoted(term)
}

// unknownField is the refusal for a term that names a field the table does
// not have.
func (schema *FilterSchema[T]) unknownField(term, name string) error {
	problem := termProblem(term).Str(" names ").Quoted(name).
		Str(", which is not a field of the table of ").Str(schema.subject)
	if schema.label != nil {
		problem.Str(" and not a label key in use")
	}
	problem.Str(".")

	return schema.refuse(problem.Slice(), schema.grammarFix())
}

// grammarFix is the fix line of a refused term: the forms of a term and the
// fields of the table.
func (schema *FilterSchema[T]) grammarFix() string {
	fix := textbuf.New().Str("write each term as a word, field:value, field:a,b, field:!value, field:>n or field:<n")
	if schema.label != nil {
		fix.Str(", or label:key=value")
	}
	if flags := schema.Flags(); len(flags) > 0 {
		fix.Str(", or ").Str(filterFlagKey).Str(filterFieldSeparator).Join(flags, "|")
	}
	fix.Str(", with one of the fields: ").Join(schema.Fields(), ", ")

	return fix.Slice()
}

// refuse builds the operator error of a refused filter.
func (schema *FilterSchema[T]) refuse(problem, fix string) error {
	return opererr.Wrap(ErrFilterRefused, problem, filterWhy, fix)
}

// SetFilterTerm answers filter with the term of one field set to any of
// values, and true: the first term that names the field is replaced, any
// later one is dropped, and a filter with none gains the term at its end. An
// assisted filter menu writes its choice into the filter box with it, so the
// box always shows the whole filter. For the field label, values is the one
// pair key=value, and the terms replaced are those of that key alone.
//
// The grammar has no quoting, so it answers false, and no filter, when it
// cannot carry the term: a field that is empty or holds a colon or a space, a
// value that is empty, holds a space or a comma, or opens with !, > or <, and
// a label pair with no key or no value. Any filter written for such a value
// would read back as other terms than the caller asked for, so a caller
// offers no link for it.
func SetFilterTerm(filter, field string, values ...string) (string, bool) {
	return setFilterTerm(filter, field, false, values)
}

// setFilterTerm is SetFilterTerm, with the term written for none of values
// when negated is true.
func setFilterTerm(filter, field string, negated bool, values []string) (string, bool) {
	if !filterFieldCarried(field) || len(values) == 0 {
		return "", false
	}

	prefix := field + filterFieldSeparator
	if field == filterLabelKey {
		key, carried := filterLabelPairKey(values)
		if !carried || negated {
			return "", false
		}
		prefix += key + groupSeparator
	} else {
		for _, value := range values {
			if !filterValueCarried(value) {
				return "", false
			}
		}
	}

	term := textbuf.New().Str(field).Str(filterFieldSeparator)
	if negated {
		term.Str(filterNegation)
	}
	term.Join(values, filterListSeparator)

	return placeFilterTerm(filter, prefix, term.Slice()), true
}

// filterFieldCarried reports whether the grammar reads field back as the
// field of a term.
func filterFieldCarried(field string) bool {
	return field != "" && !strings.ContainsFunc(field, unicode.IsSpace) && !strings.Contains(field, filterFieldSeparator)
}

// filterValueCarried reports whether the grammar reads value back as one
// literal value of a term, neither a list, a negation nor a comparison.
func filterValueCarried(value string) bool {
	if value == "" || strings.ContainsFunc(value, unicode.IsSpace) || strings.Contains(value, filterListSeparator) {
		return false
	}

	return !strings.HasPrefix(value, filterNegation) && !isComparison(value)
}

// filterLabelPairKey answers the key of the one label pair key=value that
// values holds, and whether the grammar reads the pair back: one pair, with
// no space, a key and a value. The pair is cut at its first equals sign, as
// compileLabel cuts it.
func filterLabelPairKey(values []string) (string, bool) {
	if len(values) != 1 || strings.ContainsFunc(values[0], unicode.IsSpace) {
		return "", false
	}

	key, value, valued := strings.Cut(values[0], groupSeparator)

	return key, valued && key != "" && value != ""
}

// setFixedFilterTerm is SetFilterTerm for a value of the package's own fixed
// vocabulary: a state key, or a time it formats itself. The grammar carries
// every such value, so one it cannot carry is a bug of this package.
func setFixedFilterTerm(filter, field, value string) string {
	set, carried := SetFilterTerm(filter, field, value)
	if !carried {
		panic("BUG: fleetui: the filter grammar cannot carry the fixed value " + field + filterFieldSeparator + value)
	}

	return set
}

// placeFilterTerm answers filter with term in place of the first term that
// opens with prefix, the later ones dropped, or at its end when none does.
func placeFilterTerm(filter, prefix, term string) string {
	terms := strings.Fields(filter)
	kept := make([]string, 0, len(terms)+1)
	placed := false
	for _, held := range terms {
		if !strings.HasPrefix(held, prefix) {
			kept = append(kept, held)
			continue
		}
		if placed {
			continue
		}
		kept = append(kept, term)
		placed = true
	}
	if !placed {
		kept = append(kept, term)
	}

	return textbuf.New().Join(kept, " ").Slice()
}

// RemoveFilterTerm answers filter without the terms that name field.
func RemoveFilterTerm(filter, field string) string {
	prefix := field + filterFieldSeparator

	terms := strings.Fields(filter)
	kept := make([]string, 0, len(terms))
	for _, held := range terms {
		if !strings.HasPrefix(held, prefix) {
			kept = append(kept, held)
		}
	}

	return textbuf.New().Join(kept, " ").Slice()
}
