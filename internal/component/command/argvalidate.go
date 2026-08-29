// Design: docs/architecture/api/commands.md -- YANG-typed argument validation
// Related: node.go -- ArgDef type definitions

package command

import (
	"fmt"
	"slices"
	"strconv"
)

const maxArgLength = 1024

// ValidateArgString validates a raw string argument against an ArgDef.
func ValidateArgString(arg string, def *ArgDef) error {
	if len(arg) > maxArgLength {
		return fmt.Errorf("argument too long (max %d bytes)", maxArgLength)
	}

	switch def.Kind {
	case ArgEnum:
		return validateEnum(arg, def)
	case ArgUint:
		return validateUint(arg, def)
	case ArgString:
		return validateString(arg, def)
	case ArgUnion:
		return validateUnion(arg, def)
	default:
		return nil
	}
}

func validateEnum(arg string, def *ArgDef) error {
	if slices.Contains(def.EnumValues, arg) {
		return nil
	}
	return fmt.Errorf("invalid value %q, expected one of: %s", arg, joinEnum(def.EnumValues))
}

func validateUint(arg string, def *ArgDef) error {
	bits := def.UintBits
	if bits == 0 {
		bits = 64
	}
	v, err := strconv.ParseUint(arg, 10, bits)
	if err != nil {
		return fmt.Errorf("invalid value %q, expected unsigned integer", arg)
	}
	if len(def.Ranges) > 0 {
		for _, r := range def.Ranges {
			if v >= r.Min && v <= r.Max {
				return nil
			}
		}
		if len(def.Ranges) == 1 {
			return fmt.Errorf("value %d out of range %d..%d", v, def.Ranges[0].Min, def.Ranges[0].Max)
		}
		return fmt.Errorf("value %d out of allowed ranges", v)
	}
	return nil
}

func validateString(arg string, def *ArgDef) error {
	if def.Pattern != nil && !def.Pattern.MatchString(arg) {
		return fmt.Errorf("invalid value %q, does not match expected pattern", arg)
	}
	return nil
}

func validateUnion(arg string, def *ArgDef) error {
	for i := range def.UnionDefs {
		if ValidateArgString(arg, &def.UnionDefs[i]) == nil {
			return nil
		}
	}
	var hint string
	for _, m := range def.UnionDefs {
		if m.Kind == ArgEnum {
			hint = joinEnum(m.EnumValues)
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
	switch def.Kind {
	case ArgEnum:
		return ConstraintEnum
	case ArgUint:
		if len(def.Ranges) > 0 {
			return ConstraintRangedUint
		}
		return ConstraintUint
	case ArgString:
		if def.Pattern != nil {
			return ConstraintPattern
		}
		return ConstraintAny
	case ArgUnion:
		weakest := ConstraintUnspecified
		for i := range def.UnionDefs {
			if member := Constraint(&def.UnionDefs[i]); member > weakest {
				weakest = member
			}
		}
		if weakest == ConstraintUnspecified {
			return ConstraintAny
		}
		return weakest
	default:
		return ConstraintAny
	}
}
