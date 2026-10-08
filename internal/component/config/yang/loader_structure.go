// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: loader.go — Resolve and DefaultLoader join these checks
// RFC: rfc/short/rfc7950.md -- Sections 7.19, 9.4.4 and 9.6.4, structures goyang accepts
package yang

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/openconfig/goyang/pkg/yang"
)

// ErrLengthOrder marks a `length` statement whose parts overlap or descend.
// goyang sorts and coalesces the parts before it validates them (types_builtin.go,
// parseChildRanges: "The output range is sorted and coalesced"), so the fault
// never reaches its own check.
var ErrLengthOrder = errors.New("YANG length parts not disjoint and ascending")

// ErrEnumRestriction marks an `enum` statement in a restricted enumeration
// that names no assigned name of the base type, or changes its value. goyang
// builds the restricted type's values afresh from the restricting statements
// and never compares them with the base type.
var ErrEnumRestriction = errors.New("YANG enum restriction departs from the base type")

// ErrExtensionSubstatement marks a substatement under an extension statement
// that is not a YANG statement, or that breaks the Section 14 grammar for its
// argument. goyang keeps an extension statement as raw text (ast.go, build:
// "it might be an extension"), so it never reads these substatements.
var ErrExtensionSubstatement = errors.New("invalid YANG statement under an extension")

// checkStructure joins the structural checks goyang does not perform, over
// every loaded module and submodule.
func (l *Loader) checkStructure() error {
	mods := l.sourceModules()
	errs := make([]error, 0, len(mods))
	for _, mod := range mods {
		errs = append(errs, moduleLengthErrors(mod)...)
		errs = append(errs, moduleEnumErrors(mod)...)
		errs = append(errs, moduleExtensionSubstatementErrors(mod)...)
	}
	return errors.Join(errs...)
}

// moduleLengthErrors returns one error for each `length` statement in mod
// whose parts, in the order written, are not disjoint and ascending. The walk
// is an explicit stack, as in moduleExtensionErrors.
func moduleLengthErrors(mod *yang.Module) []error {
	if mod.Source == nil {
		return nil
	}
	var errs []error
	pending := slices.Clone(mod.Source.SubStatements())
	for len(pending) > 0 {
		statement := pending[len(pending)-1]
		pending = append(pending[:len(pending)-1], statement.SubStatements()...)
		if statement.Keyword != "length" {
			continue
		}
		if err := lengthPartsAscending(statement.Argument); err != nil {
			errs = append(errs, fmt.Errorf("%w: module %s: %s: length %q: %w",
				ErrLengthOrder, mod.Name, statement.Location(), statement.Argument, err))
		}
	}
	return errs
}

// lengthPartsAscending checks a length expression in the order it is written.
//
// RFC 7950 Section 9.4.4: "Length-restricting values MUST NOT be negative.
// If multiple values or ranges are given, they all MUST be disjoint and MUST
// be in ascending order."
//
// "min" reads as 0 and "max" as the largest uint64. Both stand for the type's
// own bounds, which lie inside that span, so a part that must follow another
// can only pass with them when it follows it with the type's real bounds too.
func lengthPartsAscending(expression string) error {
	var previous uint64
	for i, part := range strings.Split(expression, "|") {
		lower, upper, err := lengthPartBounds(part)
		if err != nil {
			return err
		}
		if i > 0 && lower <= previous {
			return fmt.Errorf("part %q does not start after the part before it", strings.TrimSpace(part))
		}
		previous = upper
	}
	return nil
}

// lengthPartBounds answers the lower and upper bound of one length part, an
// explicit value or "lower..upper".
func lengthPartBounds(part string) (lower, upper uint64, err error) {
	lowerText, upperText, isRange := strings.Cut(part, "..")
	if lower, err = lengthBound(lowerText); err != nil {
		return 0, 0, err
	}
	if !isRange {
		return lower, lower, nil
	}
	if upper, err = lengthBound(upperText); err != nil {
		return 0, 0, err
	}
	return lower, upper, nil
}

// lengthBound answers one length boundary: "min", "max", or a non-negative
// integer (RFC 7950 Section 14, length-boundary).
func lengthBound(text string) (uint64, error) {
	bound := strings.TrimSpace(text)
	if bound == "min" {
		return 0, nil
	}
	if bound == "max" {
		return ^uint64(0), nil
	}
	value, err := strconv.ParseUint(bound, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("boundary %q is neither min, max nor a non-negative integer", bound)
	}
	return value, nil
}

// moduleEnumErrors returns one error for each `enum` statement in a restricted
// enumeration of mod that departs from its base type.
//
// A type goyang never resolved has no YangType: it sits in a grouping no
// schema node uses, so it restricts nothing and is skipped.
func moduleEnumErrors(mod *yang.Module) []error {
	var errs []error
	for _, typ := range moduleTypeNodes(mod) {
		if len(typ.Enum) == 0 {
			continue
		}
		if typ.YangType == nil {
			continue
		}
		if typ.YangType.Kind != yang.Yenum {
			continue
		}
		if typ.Name == "enumeration" {
			continue
		}
		errs = append(errs, enumRestrictionErrors(mod, typ)...)
	}
	return errs
}

// enumRestrictionErrors checks each enum statement of typ, a restriction of a
// derived enumeration, against the type it restricts.
//
// RFC 7950 Section 9.6.4: "When an existing enumeration type is restricted,
// the set of assigned names in the new type MUST be a subset of the base
// type's set of assigned names. The value of such an assigned name MUST NOT
// be changed."
//
// RFC 7950 Section 9.6.4.2: "When an existing enumeration type is restricted,
// the "value" statement MUST either have the same value as in the base type
// or not be present, in which case the value is the same as in the base type."
// The base value is the root enumeration's, which every restriction keeps.
func enumRestrictionErrors(mod *yang.Module, typ *yang.Type) []error {
	base, root := enumBases(typ)
	if base == nil {
		return []error{fmt.Errorf("%w: module %s: %s: type %s restricts no enumeration",
			ErrEnumRestriction, mod.Name, yang.Source(typ), typ.Name)}
	}
	var errs []error
	for _, enum := range typ.Enum {
		if !slices.ContainsFunc(base.Enum, func(e *yang.Enum) bool { return e.Name == enum.Name }) {
			errs = append(errs, fmt.Errorf("%w: module %s: %s: enum %q is not an assigned name of type %s",
				ErrEnumRestriction, mod.Name, yang.Source(enum), enum.Name, typ.Name))
			continue
		}
		if enum.Value == nil {
			continue
		}
		baseValue := root.YangType.Enum.Value(enum.Name)
		value, err := strconv.ParseInt(strings.TrimSpace(enum.Value.Name), 10, 64)
		if err != nil {
			errs = append(errs, fmt.Errorf("%w: module %s: %s: enum %q: value %q is not an integer",
				ErrEnumRestriction, mod.Name, yang.Source(enum), enum.Name, enum.Value.Name))
			continue
		}
		if value != baseValue {
			errs = append(errs, fmt.Errorf("%w: module %s: %s: enum %q: value %d, the base type assigns %d",
				ErrEnumRestriction, mod.Name, yang.Source(enum), enum.Name, value, baseValue))
		}
	}
	return errs
}

// enumBases answers, for a restricted enumeration typ, the nearest type it
// derives from that lists enum statements, whose names bound typ's, and the
// root `enumeration` type whose values every restriction keeps. Each answer is
// nil when the derivation reaches neither. The loop follows the typedef chain
// goyang resolved, so its length is the module's derivation depth.
func enumBases(typ *yang.Type) (base, root *yang.Type) {
	for node := typ.YangType.Base; node != nil; node = resolvedBase(node) {
		if base == nil && len(node.Enum) > 0 {
			base = node
		}
		if node.Name == "enumeration" {
			return base, node
		}
	}
	return base, nil
}

// resolvedBase answers the type node goyang resolved typ against, or nil when
// goyang resolved none.
func resolvedBase(typ *yang.Type) *yang.Type {
	if typ.YangType == nil {
		return nil
	}
	return typ.YangType.Base
}

// moduleTypeNodes answers every `type` node of mod's goyang AST. The walk is
// an explicit stack over the substatement fields goyang's own parser fills
// (isSubstatementField), so it reaches each node the module text declares.
func moduleTypeNodes(mod *yang.Module) []*yang.Type {
	var types []*yang.Type
	pending := []reflect.Value{reflect.ValueOf(mod)}
	for len(pending) > 0 {
		node := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if node.IsNil() {
			continue
		}
		if typ, isType := reflect.TypeAssert[*yang.Type](node); isType {
			types = append(types, typ)
		}
		pending = appendSubstatementNodes(pending, node.Elem())
	}
	return types
}

// appendSubstatementNodes appends the AST nodes held in node's substatement
// fields, each a pointer to a struct or a slice of them.
func appendSubstatementNodes(pending []reflect.Value, node reflect.Value) []reflect.Value {
	fields := node.Type()
	for i := range fields.NumField() {
		if !isSubstatementField(fields.Field(i)) {
			continue
		}
		value := node.Field(i)
		//exhaustive:ignore // goyang's substatement fields are pointers or slices of pointers; nothing else holds a node
		switch value.Kind() {
		case reflect.Pointer:
			pending = append(pending, value)
		case reflect.Slice:
			for j := range value.Len() {
				pending = append(pending, value.Index(j))
			}
		}
	}
	return pending
}

// isSubstatementField reports whether f is a field goyang fills from a
// substatement: its `yang` tag names a keyword, which is lower case, where
// the bookkeeping tags (Name, Statement, Parent, Ext) are capitalized.
func isSubstatementField(f reflect.StructField) bool {
	keyword := substatementKeyword(f)
	if keyword == "" {
		return false
	}
	return f.Type.Kind() == reflect.Pointer || f.Type.Kind() == reflect.Slice
}

// substatementKeyword answers the YANG keyword f's `yang` tag names, or ""
// when the tag is absent or names a bookkeeping field.
func substatementKeyword(f reflect.StructField) string {
	keyword, _, _ := strings.Cut(f.Tag.Get("yang"), ",")
	if keyword == "" {
		return ""
	}
	if !unicode.IsLower(rune(keyword[0])) {
		return ""
	}
	return keyword
}

// yangKeywords answers every YANG statement keyword, derived from the struct
// tags of goyang's AST, the grammar its parser applies to every statement
// outside an extension. The walk covers each AST struct type once.
var yangKeywords = sync.OnceValue(func() map[string]bool {
	keywords := map[string]bool{"module": true, "submodule": true}
	seen := map[reflect.Type]bool{}
	pending := []reflect.Type{reflect.TypeFor[yang.Module]()}
	for len(pending) > 0 {
		node := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[node] {
			continue
		}
		seen[node] = true
		for field := range node.Fields() {
			keyword := substatementKeyword(field)
			if keyword == "" {
				continue
			}
			keywords[keyword] = true
			element := field.Type
			if element.Kind() == reflect.Slice {
				element = element.Elem()
			}
			if element.Kind() == reflect.Pointer && element.Elem().Kind() == reflect.Struct {
				pending = append(pending, element.Elem())
			}
		}
	}
	return keywords
})

// argumentlessKeywords are the statements the Section 14 grammar gives no
// argument: "input-stmt = input-keyword optsep" and "output-stmt =
// output-keyword optsep". Every other statement rule reads
// "<keyword> sep <argument>".
var argumentlessKeywords = map[string]bool{"input": true, "output": true}

// moduleExtensionSubstatementErrors returns one error for each statement
// under an extension statement of mod that is not a YANG statement or breaks
// its argument grammar. A nested extension statement is itself checked by
// checkExtensions, and its own substatements here.
//
// RFC 7950 Section 7.19: "Syntactically, the substatements MUST be YANG
// statements, including extensions defined using "extension" statements.
// YANG statements in extensions MUST follow the syntactical rules in
// Section 14." The grammar part checked here is the keyword and whether
// the statement carries an argument.
func moduleExtensionSubstatementErrors(mod *yang.Module) []error {
	if mod.Source == nil {
		return nil
	}
	type visit struct {
		statement      *yang.Statement
		underExtension bool
	}
	var errs []error
	var pending []visit
	for _, statement := range mod.Source.SubStatements() {
		pending = append(pending, visit{statement, false})
	}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		statement := current.statement
		isExtension := strings.Contains(statement.Keyword, ":")
		if current.underExtension && !isExtension {
			if err := extensionSubstatementError(statement); err != nil {
				errs = append(errs, fmt.Errorf("%w: module %s: %s: %w",
					ErrExtensionSubstatement, mod.Name, statement.Location(), err))
			}
		}
		for _, sub := range statement.SubStatements() {
			pending = append(pending, visit{sub, current.underExtension || isExtension})
		}
	}
	return errs
}

// extensionSubstatementError answers why an unprefixed statement under an
// extension breaks the YANG grammar, or nil when it does not.
func extensionSubstatementError(statement *yang.Statement) error {
	if !yangKeywords()[statement.Keyword] {
		return fmt.Errorf("%q is not a YANG statement", statement.Keyword)
	}
	_, hasArgument := statement.Arg()
	if argumentlessKeywords[statement.Keyword] {
		if hasArgument {
			return fmt.Errorf("%s takes no argument", statement.Keyword)
		}
		return nil
	}
	if !hasArgument {
		return fmt.Errorf("%s requires an argument", statement.Keyword)
	}
	return nil
}
