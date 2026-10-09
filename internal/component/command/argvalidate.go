// Design: docs/architecture/api/commands.md -- YANG-typed argument validation
// Related: node.go -- ArgDef type definitions

package command

import (
	"fmt"
	"slices"
	"strconv"
	"unicode/utf8"
)

const maxArgLength = 1024

// ValidateArgString validates a raw string argument against an ArgDef. It
// refuses every token for a definition no constructor built (ErrArgDef): the
// zero ArgDef would otherwise accept any string.
func ValidateArgString(arg string, def *ArgDef) error {
	if !def.constructed {
		return fmt.Errorf("%w: argument %q was not built by a constructor", ErrArgDef, def.name)
	}
	if len(arg) > maxArgLength {
		return fmt.Errorf("argument too long (max %d bytes)", maxArgLength)
	}

	switch def.kind {
	case ArgEnum:
		return validateEnum(arg, def)
	case ArgUint:
		return validateUint(arg, def)
	case ArgString:
		return validateString(arg, def)
	case ArgUnion:
		return validateUnion(arg, def)
	case ArgFlag:
		// A flag is its own keyword. No token is ever its value.
		return fmt.Errorf("%s takes no value", def.name)
	default:
		panic("BUG: invalid command argument kind")
	}
}

func validateEnum(arg string, def *ArgDef) error {
	if slices.Contains(def.enumValues, arg) {
		return nil
	}
	return fmt.Errorf("invalid value %q, expected one of: %s", arg, joinEnum(def.enumValues))
}

func validateUint(arg string, def *ArgDef) error {
	v, err := strconv.ParseUint(arg, 10, def.uintBits)
	if err != nil {
		return fmt.Errorf("invalid value %q, expected unsigned integer", arg)
	}
	if len(def.ranges) > 0 {
		for _, r := range def.ranges {
			if v >= r.Min && v <= r.Max {
				return nil
			}
		}
		if len(def.ranges) == 1 {
			return fmt.Errorf("value %d out of range %d..%d", v, def.ranges[0].Min, def.ranges[0].Max)
		}
		return fmt.Errorf("value %d out of allowed ranges", v)
	}
	return nil
}

// validateString judges a string argument against its YANG length and every
// pattern its type chain declares.
func validateString(arg string, def *ArgDef) error {
	if len(def.lengths) > 0 {
		// RFC 7950 Section 9.4.4: "A "length" statement restricts the number
		// of Unicode characters in the string."
		length := uint64(utf8.RuneCountInString(arg))
		if !inRanges(length, def.lengths) {
			return fmt.Errorf("invalid value %q, length %d out of range %s", arg, length, joinRanges(def.lengths))
		}
	}
	// RFC 7950 Section 9.4.5: "If the type has multiple "pattern" statements,
	// the expressions are ANDed together, i.e., all such expressions have to
	// match."
	for _, pattern := range def.patterns {
		if !pattern.MatchString(arg) {
			return fmt.Errorf("invalid value %q, does not match expected pattern", arg)
		}
	}
	return nil
}

// inRanges reports whether v falls inside any of ranges.
func inRanges(v uint64, ranges []UintRange) bool {
	for _, r := range ranges {
		if v >= r.Min && v <= r.Max {
			return true
		}
	}
	return false
}

// joinRanges renders ranges the way a YANG module writes them, "1..8 | 16..32",
// with a single value written once.
func joinRanges(ranges []UintRange) string {
	buf := make([]byte, 0, 24*len(ranges))
	for i, r := range ranges {
		if i > 0 {
			buf = append(buf, " | "...)
		}
		buf = strconv.AppendUint(buf, r.Min, 10)
		if r.Max != r.Min {
			buf = append(buf, ".."...)
			buf = strconv.AppendUint(buf, r.Max, 10)
		}
	}
	return string(buf)
}

func validateUnion(arg string, def *ArgDef) error {
	for i := range def.unionDefs {
		if ValidateArgString(arg, &def.unionDefs[i]) == nil {
			return nil
		}
	}
	var hint string
	for i := range def.unionDefs {
		m := &def.unionDefs[i]
		if m.kind == ArgEnum {
			hint = joinEnum(m.enumValues)
			break
		}
	}
	if hint != "" {
		return fmt.Errorf("invalid value %q, expected unsigned integer or one of: %s", arg, hint)
	}
	return fmt.Errorf("invalid value %q, does not match any accepted type", arg)
}

func joinEnum(values []string) string {
	if len(values) == 0 {
		return ""
	}
	n := 2 * (len(values) - 1)
	for _, v := range values {
		n += len(v)
	}
	buf := make([]byte, 0, n)
	for i, v := range values {
		if i > 0 {
			buf = append(buf, ',', ' ')
		}
		buf = append(buf, v...)
	}
	return string(buf)
}

// ArgConstraint ranks how much an argument definition constrains its value. A
// LOWER rank admits fewer values.
//
// Ranking exists so a positional token goes to the definition that names it
// most exactly, and never to whichever definition a slice happens to hold
// first. `show system sockets 8080` must reach the port leaf even though the
// state leaf is a pattern-less string that accepts the same token.
type ArgConstraint uint8

const (
	// ConstraintUnspecified is the zero value and ranks nothing, so a
	// definition built by mistake never reads as a strong constraint.
	ConstraintUnspecified ArgConstraint = iota
	// ConstraintEnum admits a closed set of words.
	ConstraintEnum
	// ConstraintRangedUint admits the integers of a declared range.
	ConstraintRangedUint
	// ConstraintUint admits every integer of its width.
	ConstraintUint
	// ConstraintPattern admits the strings one regular expression matches.
	ConstraintPattern
	// ConstraintAny admits every string, so it is the last resort.
	ConstraintAny
)

// Constraint ranks def by how much its type constrains the value it accepts.
//
// A union takes the rank of its MOST PERMISSIVE member, because a union accepts
// every value any of its members accepts. A union of no members constrains
// nothing and ranks last.
func Constraint(def *ArgDef) ArgConstraint {
	switch def.kind {
	case ArgEnum:
		return ConstraintEnum
	case ArgUint:
		if len(def.ranges) > 0 {
			return ConstraintRangedUint
		}
		return ConstraintUint
	case ArgString:
		if len(def.patterns) > 0 {
			return ConstraintPattern
		}
		return ConstraintAny
	case ArgUnion:
		weakest := ConstraintUnspecified
		for i := range def.unionDefs {
			if member := Constraint(&def.unionDefs[i]); member > weakest {
				weakest = member
			}
		}
		if weakest == ConstraintUnspecified {
			return ConstraintAny
		}
		return weakest
	case ArgFlag:
		return ConstraintAny
	default:
		panic("BUG: invalid command argument kind")
	}
}
