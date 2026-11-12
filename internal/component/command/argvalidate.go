// Design: docs/architecture/api/commands.md -- YANG-typed argument validation
// Related: node.go -- ArgDef type definitions

package command

import (
	"fmt"
	"slices"
	"strconv"
	"unicode/utf8"
)

// maxArgLength is the most bytes a token may hold for a leaf whose model
// declares no longer length. A leaf that declares one, such as the hex Data
// field of request peer notify (8150 digits), is held to that length instead,
// counted in the bytes its characters can take (argLengthMax).
const maxArgLength = 1024

// echoMax is the most characters of a refused value a refusal quotes, so a
// refusal of a value thousands of characters long stays readable.
const echoMax = 64

// ValidateArgString validates a raw string argument against an ArgDef. It
// refuses every token for a definition no constructor built (ErrArgDef): the
// zero ArgDef would otherwise accept any string.
func ValidateArgString(arg string, def *ArgDef) error {
	if !def.constructed {
		return fmt.Errorf("%w: argument %q was not built by a constructor", ErrArgDef, def.name)
	}
	if limit := argLengthMax(def); len(arg) > limit {
		return fmt.Errorf("argument too long (max %d bytes): %s is %d bytes; shorten it to %d bytes or fewer", limit, def.name, len(arg), limit)
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

// validateEnum refuses a word outside def's enumeration with the value fault
// that lists every accepted value (ValueArgumentError).
//
// RFC 7950 Section 9.6: "The enumeration built-in type represents values from a set of assigned names."
func validateEnum(arg string, def *ArgDef) error {
	if slices.Contains(def.enumValues, arg) {
		return nil
	}
	return enumFault(arg, def)
}

// validateUint refuses a token that is not a number of def's width with the
// type refusal, and a number outside def's declared ranges with the value
// fault that names the bounds, or the error-message the range declares
// (RFC 7950 Section 7.5.4.1).
func validateUint(arg string, def *ArgDef) error {
	v, err := strconv.ParseUint(arg, 10, def.uintBits)
	if err != nil {
		return fmt.Errorf("invalid value %q, expected unsigned integer", arg)
	}
	if len(def.ranges) == 0 {
		return nil
	}
	// RFC 7950 Section 9.2.4: "It is used to restrict integer and decimal built-in types, or types derived from them."
	if inRanges(v, def.ranges) {
		return nil
	}
	return rangeFault(arg, def)
}

// validateString judges a string argument against its YANG length and every
// pattern its type chain declares, and refuses a miss with the value fault of
// its kind.
func validateString(arg string, def *ArgDef) error {
	if len(def.lengths) > 0 {
		// RFC 7950 Section 9.4.4: "A "length" statement restricts the number
		// of Unicode characters in the string."
		length := uint64(utf8.RuneCountInString(arg))
		if !inRanges(length, def.lengths) {
			return lengthFault(arg, def)
		}
	}
	// RFC 7950 Section 9.4.5: "If the type has multiple "pattern" statements,
	// the expressions are ANDed together, i.e., all such expressions have to
	// match."
	for i, pattern := range def.patterns {
		if pattern.MatchString(arg) {
			continue
		}
		return patternFault(arg, def, i)
	}
	return nil
}

// argLengthMax answers the most bytes a token may hold for def: maxArgLength,
// or more when the leaf declares a length whose upper bound, counted at the
// utf8.UTFMax bytes one character can take, is larger. The declared length is
// then judged in characters by validateString, so the byte cap only keeps a
// token the declared length could accept from being refused before it.
func argLengthMax(def *ArgDef) int {
	limit := maxArgLength
	for _, part := range def.lengths {
		if part.Max > uint64(maxArgLength) {
			limit = max(limit, int(min(part.Max, 1<<20))*utf8.UTFMax)
		}
	}
	return limit
}

// echoValue answers arg as a refusal quotes it: whole when short, otherwise its
// first echoMax characters followed by "...".
func echoValue(arg string) string {
	if utf8.RuneCountInString(arg) <= echoMax {
		return arg
	}
	runes := []rune(arg)
	return string(runes[:echoMax]) + "..."
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

// validateUnion accepts a value any member of def's union accepts, and refuses
// any other with the value fault that lists the enumeration members' values
// and names the other member types.
//
// RFC 7950 Section 9.12: "When interpreting an XML encoding, a value is validated consecutively against each member type, in the order they are specified in the "type" statement, until a match is found."
func validateUnion(arg string, def *ArgDef) error {
	for i := range def.unionDefs {
		if ValidateArgString(arg, &def.unionDefs[i]) == nil {
			return nil
		}
	}
	return unionFault(arg, def)
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
