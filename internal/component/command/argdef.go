// Design: docs/architecture/api/commands.md — command argument definitions
// Related: node.go — the command tree that carries the definitions
// Related: argvalidate.go — validation of a token against a definition

package command

import (
	"errors"
	"fmt"
	"iter"
	"regexp"
	"slices"
)

// ErrArgDef is wrapped by every refusal of an argument definition
// constructor.
var ErrArgDef = errors.New("invalid command argument definition")

// ArgDef declares a typed argument for an operational command, extracted from
// YANG leaves inside ze:command containers. Drives completion, validation,
// and documentation from a single source.
//
// Only the constructors (NewStringArg, NewUintArg, NewEnumArg, NewUnionArg,
// NewFlagArg) build one, and every field is private, so a definition whose
// kind and payload disagree, or whose ranges overlap or descend, cannot be
// written outside this package. The constructors copy their input slices and
// the accessors expose no slice, so a validated definition cannot be changed
// through an alias.
//
// The zero value, ArgDef{}, compiles anywhere and is NOT a definition: it
// would otherwise read as an unrestricted string. ValidateArgs and
// ValidateArgString refuse it with an error (the constructed marker).
//
// A value is immutable after construction, so it is safe for concurrent use.
type ArgDef struct {
	name       string
	kind       ArgKind
	enumValues []string         // ArgEnum names; for ArgUnion the flattened names of its enum members
	uintBits   int              // 8, 16, 32 or 64, for ArgUint
	ranges     []UintRange      // ArgUint value ranges, ascending and disjoint (nil = the whole width)
	lengths    []UintRange      // ArgString character-count ranges, ascending and disjoint (nil = any length)
	patterns   []*regexp.Regexp // ArgString patterns, all must match (nil = any string)
	unionDefs  []ArgDef         // ArgUnion members, tried in order
	mandatory  bool

	// shortHelp is the leaf's one-line summary, from its ze:help statement, and
	// description is the long explanation, from its description statement.
	// Neither is derived from the other: a leaf that declares one text leaves
	// the other empty, and every reader prints the one it has.
	shortHelp   string
	description string

	// anchor names the path keyword this value follows, and it is set when the
	// leaf is declared by a container ABOVE the command rather than by the
	// command itself: `request interface <name> down` declares `name` on
	// `interface`, so the anchor is `interface`.
	//
	// It is empty for a leaf the command declares. Such a leaf follows the
	// container whose name it repeats, and trails the last keyword when it
	// repeats none, which is the rule the renderer already applied.
	//
	// The anchor decides where a value is printed (usageAnchor) and where the
	// dispatcher reads it: matchCommandTokens binds the bare token after the
	// anchor keyword to the leaf anchored there (anchoredDef,
	// internal/component/plugin/server), and the web admin form prints a
	// posted value at that same place (commandArguments,
	// internal/component/web).
	anchor string

	// constructed is true for every value a constructor returned. It is the
	// guard that tells a definition from the zero value (D-4).
	constructed bool
}

// ArgOptions carries the parts of a definition every kind shares. None of them
// bears an invariant, so the struct is plain data.
type ArgOptions struct {
	Mandatory   bool   // the YANG leaf has mandatory true
	ShortHelp   string // the ze:help summary
	Description string // the YANG description
	Anchor      string // the keyword of the container above the command that declares the leaf
}

// NewStringArg builds an ArgString definition. lengths are character-count
// ranges and patterns the compiled patterns that must all match; either may be
// nil. It refuses an empty name, a nil pattern, and lengths that are not
// ascending and disjoint. The definition takes ownership of the compiled
// patterns: the caller MUST NOT call Longest on one afterwards, because that
// would change what the definition accepts.
func NewStringArg(name string, lengths []UintRange, patterns []*regexp.Regexp, opts ArgOptions) (ArgDef, error) {
	if err := checkArgName(name); err != nil {
		return ArgDef{}, err
	}
	// RFC 7950 Section 9.4.4: "If multiple values or ranges are given, they
	// all MUST be disjoint and MUST be in ascending order."
	if err := checkParts(name, "length", lengths, maxUint(64)); err != nil {
		return ArgDef{}, err
	}
	for i, pattern := range patterns {
		if pattern == nil {
			return ArgDef{}, fmt.Errorf("%w: %s: pattern %d is nil", ErrArgDef, name, i)
		}
	}
	def := newArgDef(name, ArgString, opts)
	def.lengths = slices.Clone(lengths)
	def.patterns = slices.Clone(patterns)
	return def, nil
}

// NewUintArg builds an ArgUint definition of the given width. ranges may be
// nil, meaning the whole width. It refuses an empty name, a width outside
// {8, 16, 32, 64}, a range above the width, and ranges that are not ascending
// and disjoint.
func NewUintArg(name string, bits int, ranges []UintRange, opts ArgOptions) (ArgDef, error) {
	if err := checkArgName(name); err != nil {
		return ArgDef{}, err
	}
	switch bits {
	case 8, 16, 32, 64:
	default:
		return ArgDef{}, fmt.Errorf("%w: %s: width %d is not 8, 16, 32 or 64", ErrArgDef, name, bits)
	}
	// RFC 7950 Section 9.2.4: "If multiple values or ranges are given, they
	// all MUST be disjoint and MUST be in ascending order."
	if err := checkParts(name, "range", ranges, maxUint(bits)); err != nil {
		return ArgDef{}, err
	}
	def := newArgDef(name, ArgUint, opts)
	def.uintBits = bits
	def.ranges = slices.Clone(ranges)
	return def, nil
}

// NewEnumArg builds an ArgEnum definition over values, in the order given. It
// refuses an empty name, an empty value list and an empty value.
func NewEnumArg(name string, values []string, opts ArgOptions) (ArgDef, error) {
	if err := checkArgName(name); err != nil {
		return ArgDef{}, err
	}
	if len(values) == 0 {
		return ArgDef{}, fmt.Errorf("%w: %s: an enumeration needs at least one value", ErrArgDef, name)
	}
	for i, value := range values {
		if value == "" {
			return ArgDef{}, fmt.Errorf("%w: %s: enum value %d is empty", ErrArgDef, name, i)
		}
	}
	def := newArgDef(name, ArgEnum, opts)
	def.enumValues = slices.Clone(values)
	return def, nil
}

// NewUnionArg builds an ArgUnion definition over members, tried in order. Its
// enum values are the names of its enum members, flattened in member order. It
// refuses an empty name and a member no constructor built. A union of no
// members is a definition: it is what a union of types no command argument
// can carry lowers to, and it accepts no value.
func NewUnionArg(name string, members []ArgDef, opts ArgOptions) (ArgDef, error) {
	if err := checkArgName(name); err != nil {
		return ArgDef{}, err
	}
	var values []string
	for i := range members {
		if !members[i].constructed {
			return ArgDef{}, fmt.Errorf("%w: %s: union member %d was not built by a constructor", ErrArgDef, name, i)
		}
		if members[i].kind == ArgEnum {
			values = append(values, members[i].enumValues...)
		}
	}
	def := newArgDef(name, ArgUnion, opts)
	def.unionDefs = slices.Clone(members)
	def.enumValues = values
	return def, nil
}

// NewFlagArg builds an ArgFlag definition: the keyword alone is the argument.
// It refuses an empty name.
func NewFlagArg(name string, opts ArgOptions) (ArgDef, error) {
	if err := checkArgName(name); err != nil {
		return ArgDef{}, err
	}
	return newArgDef(name, ArgFlag, opts), nil
}

// newArgDef answers a constructed definition of kind carrying opts, with no
// payload. Every constructor starts from it, after its checks passed.
func newArgDef(name string, kind ArgKind, opts ArgOptions) ArgDef {
	return ArgDef{
		name:        name,
		kind:        kind,
		mandatory:   opts.Mandatory,
		shortHelp:   opts.ShortHelp,
		description: opts.Description,
		anchor:      opts.Anchor,
		constructed: true,
	}
}

func checkArgName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: the name is empty", ErrArgDef)
	}
	return nil
}

// checkParts refuses parts whose bounds descend, that overlap or come out of
// order, or that exceed limit. The loop is bounded by the parts one YANG
// restriction declares.
func checkParts(name, what string, parts []UintRange, limit uint64) error {
	for i, part := range parts {
		if part.Min > part.Max {
			return fmt.Errorf("%w: %s: %s part %d..%d descends", ErrArgDef, name, what, part.Min, part.Max)
		}
		if part.Max > limit {
			return fmt.Errorf("%w: %s: %s part %d..%d exceeds %d", ErrArgDef, name, what, part.Min, part.Max, limit)
		}
		if i == 0 {
			continue
		}
		if part.Min <= parts[i-1].Max {
			return fmt.Errorf("%w: %s: %s parts %d..%d and %d..%d overlap or are not ascending",
				ErrArgDef, name, what, parts[i-1].Min, parts[i-1].Max, part.Min, part.Max)
		}
	}
	return nil
}

// maxUint answers the largest value an unsigned integer of bits holds.
func maxUint(bits int) uint64 {
	return ^uint64(0) >> (64 - bits)
}

// WithAnchor answers a copy of d anchored to the keyword of the container
// above the command that declares it. d itself is unchanged.
func (d ArgDef) WithAnchor(anchor string) ArgDef {
	d.anchor = anchor
	return d
}

// Name answers the YANG leaf name, the keyword that introduces the value.
func (d ArgDef) Name() string { return d.name }

// Kind answers the argument type category.
func (d ArgDef) Kind() ArgKind { return d.kind }

// Mandatory reports whether the YANG leaf has mandatory true.
func (d ArgDef) Mandatory() bool { return d.mandatory }

// UintBits answers the width of an ArgUint, and 0 for every other kind.
func (d ArgDef) UintBits() int { return d.uintBits }

// ShortHelp answers the ze:help summary, empty when the leaf declares none.
func (d ArgDef) ShortHelp() string { return d.shortHelp }

// Description answers the YANG description, empty when the leaf declares none.
func (d ArgDef) Description() string { return d.description }

// Anchor answers the keyword this value follows, empty for a leaf the command
// itself declares (see the anchor field).
func (d ArgDef) Anchor() string { return d.anchor }

// EnumValues yields the enum names, in declared order: an ArgEnum's own, or
// the flattened names of an ArgUnion's enum members.
func (d ArgDef) EnumValues() iter.Seq[string] { return slices.Values(d.enumValues) }

// Ranges yields an ArgUint's value ranges, ascending; none means the whole
// width.
func (d ArgDef) Ranges() iter.Seq[UintRange] { return slices.Values(d.ranges) }

// Lengths yields an ArgString's character-count ranges, ascending; none means
// any length.
func (d ArgDef) Lengths() iter.Seq[UintRange] { return slices.Values(d.lengths) }

// Patterns yields the source text of an ArgString's patterns. The compiled
// expressions stay private: a *regexp.Regexp has a mutating method, Longest.
func (d ArgDef) Patterns() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, pattern := range d.patterns {
			if !yield(pattern.String()) {
				return
			}
		}
	}
}

// UnionDefs yields an ArgUnion's members, in the order they are tried.
func (d ArgDef) UnionDefs() iter.Seq[ArgDef] { return slices.Values(d.unionDefs) }

// InvocationArg is what WriteInvocation reads of an argument: its name, the
// keyword it is anchored to, and whether it is a flag, whose keyword is the
// whole argument. The MCP server builds it from a lister that knows no types.
type InvocationArg struct {
	Name   string
	Anchor string
	Flag   bool
}

// InvocationArgs projects defs onto what WriteInvocation reads.
func InvocationArgs(defs []ArgDef) []InvocationArg {
	args := make([]InvocationArg, len(defs))
	for i := range defs {
		args[i] = InvocationArg{Name: defs[i].name, Anchor: defs[i].anchor, Flag: defs[i].kind == ArgFlag}
	}
	return args
}
