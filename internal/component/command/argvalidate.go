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
