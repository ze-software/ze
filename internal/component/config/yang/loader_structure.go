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
// that is not a YANG statement, that the block holding it does not admit, or
// whose argument, substatement counts or block break its Section 14 rule. goyang keeps an
// extension statement as raw text (ast.go, build: "it might be an
// extension"), so it never reads these substatements.
var ErrExtensionSubstatement = errors.New("invalid YANG statement under an extension")

// checkStructure joins the structural checks goyang does not perform, over
// every loaded module and submodule.
func (l *Loader) checkStructure() error {
	mods := l.sourceModules()
	errs := make([]error, 0, len(mods))
	for _, mod := range mods {
		errs = append(errs, moduleLengthErrors(mod)...)
		errs = append(errs, moduleEnumErrors(mod)...)
		errs = append(errs, moduleExtensionSubstatementErrors(l.modules.source(mod))...)
	}
	return errors.Join(errs...)
}

// moduleLengthErrors returns one error for each `length` statement in mod
// whose parts, in the order written, are not disjoint and ascending over the
// bounds of the type the statement restricts.
func moduleLengthErrors(mod *yang.Module) []error {
	var errs []error
	for _, typ := range moduleTypeNodes(mod) {
		if typ.Length == nil {
			continue
		}
		err := lengthOrderError(typ)
		if err == nil {
			continue
		}
		errs = append(errs, fmt.Errorf("%w: module %s: %s: length %q: %w",
			ErrLengthOrder, mod.Name, yang.Source(typ.Length), typ.Length.Name, err))
	}
	return errs
}

// lengthOrderError answers why the length statement of typ is out of order,
// or nil when its parts are disjoint and ascending.
func lengthOrderError(typ *yang.Type) error {
	restricted, err := restrictedLengthSpan(typ)
	if err != nil {
		if namesTypeBound(typ.Length.Name) {
			return err
		}
		// No part names min or max, so no bound of the restricted type is
		// read and any span serves.
		restricted = unrestrictedLength
	}
	return lengthPartsAscending(typ.Length.Name, restricted)
}

// namesTypeBound reports whether a length expression names "min" or "max",
// the bounds of the type it restricts.
func namesTypeBound(expression string) bool {
	for part := range strings.SplitSeq(expression, "|") {
		lower, upper, _ := strings.Cut(part, "..")
		for _, bound := range []string{lower, upper} {
			bound = strings.TrimSpace(bound)
			if bound == boundaryMin || bound == boundaryMax {
				return true
			}
		}
	}
	return false
}

// boundaryMin and boundaryMax are the Section 14 "min-keyword" and
// "max-keyword", the special bounds of a length or range expression.
const (
	boundaryMin = "min"
	boundaryMax = "max"
)

// lengthSpan is a closed span of lengths, lower to upper. newLengthSpan is its
// only constructor and refuses a lower bound above the upper one, so a
// lengthSpan in hand never descends. The zero value is the span of length 0.
type lengthSpan struct {
	lower uint64
	upper uint64
}

// newLengthSpan answers the span lower..upper, or an error when it descends.
func newLengthSpan(lower, upper uint64) (lengthSpan, error) {
	if lower > upper {
		return lengthSpan{}, fmt.Errorf("lower bound %d is above upper bound %d", lower, upper)
	}
	return lengthSpan{lower: lower, upper: upper}, nil
}

// unrestrictedLength is the span of a string or binary that no length
// restricts. RFC 7950 Section 9.4.4: "An implementation is not required to
// support a length value larger than 18446744073709551615." goyang applies
// the same span (types.go, Type.resolve: "parentRange := Uint64Range").
var unrestrictedLength = lengthSpan{lower: 0, upper: ^uint64(0)}

// restrictedLengthSpan answers the lengths accepted by the type typ's length
// statement restricts: the effective length of the typedef typ derives from,
// which goyang resolved through the whole derivation chain into the base
// type node's YangType, or unrestrictedLength when no type in the chain
// carries a length.
//
// A type goyang never resolved has no YangType: it sits in a grouping no
// schema node uses. Its derivation is then unknown unless it names a built-in
// type directly, so the answer is an error rather than a guessed span.
func restrictedLengthSpan(typ *yang.Type) (lengthSpan, error) {
	if typ.YangType == nil {
		kind := yang.TypeKindFromName[typ.Name]
		if kind == yang.Ystring || kind == yang.Ybinary {
			return unrestrictedLength, nil
		}
		return lengthSpan{}, fmt.Errorf("type %s is unresolved, so min and max name no bounds", typ.Name)
	}
	base := typ.YangType.Base
	if base == nil || base.YangType == nil {
		return unrestrictedLength, nil
	}
	parts := base.YangType.Length
	if len(parts) == 0 {
		return unrestrictedLength, nil
	}
	first, last := parts[0].Min, parts[len(parts)-1].Max
	if first.Negative || last.Negative {
		return lengthSpan{}, fmt.Errorf("type %s has a negative length", base.Name)
	}
	return newLengthSpan(first.Value, last.Value)
}

// lengthPartsAscending checks a length expression in the order it is written,
// over restricted, the lengths accepted by the type it restricts.
//
// RFC 7950 Section 9.4.4: "Length-restricting values MUST NOT be negative.
// If multiple values or ranges are given, they all MUST be disjoint and MUST
// be in ascending order." The parts are read in the order written.
func lengthPartsAscending(expression string, restricted lengthSpan) error {
	var previous lengthSpan
	for i, part := range strings.Split(expression, "|") {
		span, err := lengthPartSpan(part, restricted)
		if err != nil {
			return err
		}
		if i > 0 && span.lower <= previous.upper {
			return fmt.Errorf("part %q does not start after the part before it", strings.TrimSpace(part))
		}
		previous = span
	}
	return nil
}

// lengthPartSpan answers the span of one length part, an explicit value or
// "lower..upper", with min and max read over restricted.
func lengthPartSpan(part string, restricted lengthSpan) (lengthSpan, error) {
	lowerText, upperText, isRange := strings.Cut(part, "..")
	lower, err := lengthBound(lowerText, restricted)
	if err != nil {
		return lengthSpan{}, err
	}
	if !isRange {
		return newLengthSpan(lower, lower)
	}
	upper, err := lengthBound(upperText, restricted)
	if err != nil {
		return lengthSpan{}, err
	}
	return newLengthSpan(lower, upper)
}

// lengthBound answers one length boundary: "min", "max", or a non-negative
// integer (RFC 7950 Section 14, length-boundary).
//
// RFC 7950 Section 9.4.4: "A length value is a non-negative integer or one of
// the special values "min" or "max". "min" and "max" mean the minimum and
// maximum lengths accepted for the type being restricted, respectively." So
// min and max read restricted's bounds.
func lengthBound(text string, restricted lengthSpan) (uint64, error) {
	bound := strings.TrimSpace(text)
	if bound == boundaryMin {
		return restricted.lower, nil
	}
	if bound == boundaryMax {
		return restricted.upper, nil
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

// moduleExtensionSubstatementErrors returns one error for each statement
// under an extension statement of source's module that breaks the Section
// 14 grammar. source carries the text the module was parsed from, for the
// block a statement's rule requires and goyang does not record.
//
// RFC 7950 Section 7.19: "Syntactically, the substatements MUST be YANG
// statements, including extensions defined using "extension" statements.
// YANG statements in extensions MUST follow the syntactical rules in
// Section 14." Each statement under an extension is resolved to the Section
// 14 rule its parent's block names, and then checked against that rule
// (extensionSubstatementError).
func moduleExtensionSubstatementErrors(source moduleSource) []error {
	mod := source.module
	if mod.Source == nil {
		return nil
	}
	grammar := rfc7950Grammar()
	// production is the Section 14 rule statement was resolved to, nil when
	// no extension holds statement or statement is an extension usage.
	type visit struct {
		statement  *yang.Statement
		production *statementProduction
	}
	var errs []error
	pending := []visit{{statement: mod.Source}}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		statement := current.statement
		var block *statementBlock
		if current.production != nil {
			block = current.production.block
		}
		if isExtensionUsage(statement) {
			block = grammar.extension
		}
		if block == nil {
			for _, sub := range statement.SubStatements() {
				pending = append(pending, visit{statement: sub})
			}
			continue
		}
		children, childErrs := resolveSubstatements(grammar, block, statement)
		for _, err := range childErrs {
			errs = append(errs, fmt.Errorf("%w: module %s: %w", ErrExtensionSubstatement, mod.Name, err))
		}
		for i, sub := range statement.SubStatements() {
			pending = append(pending, visit{statement: sub, production: children[i]})
		}
		if current.production == nil {
			continue
		}
		// RFC 7950 Section 7.19
		if err := extensionSubstatementError(grammar, current.production, statement, children, source); err != nil {
			errs = append(errs, fmt.Errorf("%w: module %s: %s: %w",
				ErrExtensionSubstatement, mod.Name, statement.Location(), err))
		}
	}
	return errs
}

// isExtensionUsage reports whether statement is an extension usage, whose
// keyword is "prefix:identifier" (RFC 7950 Section 14, unknown-statement).
func isExtensionUsage(statement *yang.Statement) bool {
	return strings.Contains(statement.Keyword, ":")
}

// resolveSubstatements answers, for each substatement of statement, the
// production of block it resolves to, nil for an extension usage or a
// substatement that resolves to none, and one error for each of those.
func resolveSubstatements(grammar *yangGrammar, block *statementBlock, statement *yang.Statement) ([]*statementProduction, []error) {
	subs := statement.SubStatements()
	children := make([]*statementProduction, len(subs))
	var errs []error
	for i, sub := range subs {
		if isExtensionUsage(sub) {
			continue
		}
		production, err := resolveProduction(grammar, block, sub)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", sub.Location(), err))
			continue
		}
		children[i] = production
	}
	return children, errs
}

// resolveProduction answers the production of block statement follows: the
// one, among the productions block admits for its keyword, whose argument
// rule its argument matches. The context is block, the parent's rule, so
// "augment" resolves to augment-stmt in body-stmts and to uses-augment-stmt
// under uses, and "deviate" to the one of its four rules its argument names.
func resolveProduction(grammar *yangGrammar, block *statementBlock, statement *yang.Statement) (*statementProduction, error) {
	candidates := block.admitted[statement.Keyword]
	if len(candidates) == 0 {
		if _, known := grammar.keywords[statement.Keyword]; !known {
			return nil, fmt.Errorf("%q is not a YANG statement", statement.Keyword)
		}
		return nil, fmt.Errorf("%s is not a substatement of %s", statement.Keyword, block.owner)
	}
	var matching []*statementProduction
	errs := make([]error, 0, len(candidates))
	for _, candidate := range candidates {
		err := argumentError(candidate, statement)
		if err == nil {
			matching = append(matching, candidate)
			continue
		}
		errs = append(errs, err)
	}
	if len(matching) == 0 {
		return nil, errors.Join(errs...)
	}
	if len(matching) > 1 {
		// TestSameKeywordProductionsHaveDisjointArguments keeps this a Ze defect.
		return nil, fmt.Errorf("BUG: %s %q matches %d rules of %s", statement.Keyword, statement.Argument, len(matching), block.owner)
	}
	return matching[0], nil
}

// argumentError answers why statement's argument breaks production's
// argument rule, or nil when it matches: absent when the rule takes none,
// present and matching the rule otherwise.
func argumentError(production *statementProduction, statement *yang.Statement) error {
	argument, hasArgument := statement.Arg()
	if production.argument == "" {
		if hasArgument {
			return fmt.Errorf("%s takes no argument", statement.Keyword)
		}
		return nil
	}
	if !hasArgument {
		return fmt.Errorf("%s requires an argument", statement.Keyword)
	}
	if err := checkArgument(production.argument, argument); err != nil {
		return fmt.Errorf("%s argument: %w", statement.Keyword, err)
	}
	return nil
}

// extensionSubstatementError answers why statement, resolved to production
// under an extension, breaks production's Section 14 rule, or nil when it
// does not. children holds the production each substatement resolved to, nil
// for an extension usage or one that resolved to none, whose error
// resolveSubstatements already gave.
//
// Two rules are checked here, the keyword and argument having chosen
// production. The block: a rule that opens it with a bare "{" requires it,
// and goyang records no block, so where an empty block would match, the
// module text is read (statementHasBlock). The counts: each repetition of the
// rule's block bounds how many substatements of a rule it takes, "[x]" at
// most one, "*x" any, "1*x" at least one, a bare "x" exactly one, and the
// counts together must match the block (yangGrammar.matches), which is where
// "deviate-not-supported-stmt / 1*(deviate-add-stmt / ...)" and the
// alternatives of type-body-stmts are decided.
func extensionSubstatementError(grammar *yangGrammar, production *statementProduction, statement *yang.Statement,
	children []*statementProduction, source moduleSource) error {
	block := production.block
	counts := make(childCounts, len(block.slots))
	for _, child := range children {
		if child == nil {
			continue
		}
		counts[block.slots[child]]++
	}
	if production.form == blockRequired && block.emptyAccepted && len(statement.SubStatements()) == 0 {
		hasBlock, err := statementHasBlock(source, statement)
		if err != nil {
			return fmt.Errorf("%s: %w", production.rule, err)
		}
		if !hasBlock {
			return fmt.Errorf("%s requires a block", production.rule)
		}
	}
	for keyword, productions := range block.admitted {
		for _, admitted := range productions {
			low, high := grammar.occurrences(block.content, admitted)
			found := counts[block.slots[admitted]]
			if high != repeatUnbounded && found > high {
				return fmt.Errorf("%s appears %d times in %s, %s admits at most %d", keyword, found, statement.Keyword, production.rule, high)
			}
			if found < low {
				return fmt.Errorf("%s requires at least %d %s, found %d", statement.Keyword, low, keyword, found)
			}
		}
	}
	if !grammar.matches(block, counts) {
		return fmt.Errorf("the substatements of %s do not match %s", statement.Keyword, production.rule)
	}
	return nil
}
