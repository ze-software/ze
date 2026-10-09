// Design: docs/architecture/config/yang-config-design.md -- the model is the one
// declaration of an enumeration's value set.
// Related: loader_structure.go -- checkStructure refuses an enumeration this file cannot assign.
// Related: enum.go -- the schema-node reader that orders names by these values.
// RFC: rfc/short/rfc7950.md -- Section 9.6.4.2, the value statement.

package yang

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"
)

// enumAssignment is the value RFC 7950 Section 9.6.4.2 assigns each name of
// one enumeration type, in the order its enum statements are written. It is
// the ONE producer of an enum's value in Ze: goyang builds a restricted type's
// EnumType afresh from the restricting statements, numbering them as if they
// were the whole enumeration, so nothing reads EnumType.Value, Values or
// ToString to judge or order a value.
//
// Only parseEnumAssignment and assignEnumValues build one. Every value held is
// within int32 and unique, and every name is unique. The zero value assigns no
// name, so Value answers false for every name and namesByValue answers none.
// The slices are never handed out, so a caller cannot change what was checked.
type enumAssignment struct {
	names  []string
	values []int64
}

// Value answers the value assigned to name, and false when the enumeration
// assigns name nothing.
func (a enumAssignment) Value(name string) (int64, bool) {
	for i, held := range a.names {
		if held == name {
			return a.values[i], true
		}
	}
	return 0, false
}

// namesByValue answers the assigned names in ascending value order, which is
// the order of the enum statements for every enumeration that states no value.
func (a enumAssignment) namesByValue() []string {
	order := make([]int, len(a.names))
	for i := range order {
		order[i] = i
	}
	// Values are unique, so the order is total and no tie needs a rule.
	slices.SortFunc(order, func(left, right int) int { return cmp.Compare(a.values[left], a.values[right]) })
	names := make([]string, len(order))
	for i, index := range order {
		names[i] = a.names[index]
	}
	return names
}

// enumerationType is the built-in type every enumeration derives from.
const enumerationType = "enumeration"

// parseEnumAssignment answers the assignment of declared, the type statement a
// leaf or a union member writes: the names of the nearest type along its
// derivation that lists enum statements, each holding the value the root
// enumeration assigns it. RFC 7950 Section 9.6.4.2: "When an existing
// enumeration type is restricted, the "value" statement MUST either have the
// same value as in the base type or not be present, in which case the value
// is the same as in the base type." So a restriction's own value statements
// never decide a value; checkStructure refuses one that disagrees.
//
// The walk follows the typedef chain goyang resolved, bounded by typeDepthMax.
func parseEnumAssignment(declared *gyang.Type) (enumAssignment, error) {
	var listing, root *gyang.Type
	node := declared
	for depth := 0; node != nil; depth++ {
		if depth > typeDepthMax {
			return enumAssignment{}, fmt.Errorf("%w: %s: type %s derives deeper than %d types",
				ErrEnumValue, gyang.Source(declared), declared.Name, typeDepthMax)
		}
		if listing == nil && len(node.Enum) > 0 {
			listing = node
		}
		if node.Name == enumerationType {
			root = node
			break
		}
		node = resolvedBase(node)
	}
	if listing == nil || root == nil {
		return enumAssignment{}, fmt.Errorf("%w: %s: type %s derives from no enumeration that lists an enum",
			ErrEnumValue, gyang.Source(declared), declared.Name)
	}
	assigned, err := assignEnumValues(root)
	if err != nil {
		return enumAssignment{}, err
	}
	if listing == root {
		return assigned, nil
	}
	restricted := enumAssignment{}
	for _, enum := range listing.Enum {
		value, held := assigned.Value(enum.Name)
		if !held {
			return enumAssignment{}, fmt.Errorf("%w: %s: enum %q is not an assigned name of the base type",
				ErrEnumValue, gyang.Source(enum), enum.Name)
		}
		restricted.names = append(restricted.names, enum.Name)
		restricted.values = append(restricted.values, value)
	}
	return restricted, nil
}

// assignEnumValues answers the values root, a `type enumeration` statement,
// assigns its enum statements, and an error naming the first statement RFC
// 7950 Section 9.6.4.2 forbids.
//
// RFC 7950 Section 9.6.4.2: "If a value is not specified, then one will be
// automatically assigned. If the "enum" substatement is the first one
// defined, the assigned value is zero (0); otherwise, the assigned value is
// one greater than the current highest enum value (i.e., the highest enum
// value, implicit or explicit, prior to the current "enum" substatement in
// the parent "type" statement)."
//
// RFC 7950 Section 9.6.4.2: "Note that the presence of an "if-feature"
// statement in an "enum" statement does not affect the automatically assigned
// value." So every enum statement counts, whatever its if-feature.
func assignEnumValues(root *gyang.Type) (enumAssignment, error) {
	var assigned enumAssignment
	var highest int64
	for _, enum := range root.Enum {
		value, err := enumValue(enum, assigned, highest)
		if err != nil {
			return enumAssignment{}, err
		}
		if _, held := assigned.Value(enum.Name); held {
			// RFC 7950 Section 9.6.4: "All assigned names in an enumeration
			// MUST be unique."
			return enumAssignment{}, fmt.Errorf("%w: %s: enum %q is assigned twice",
				ErrEnumValue, gyang.Source(enum), enum.Name)
		}
		for i, held := range assigned.values {
			if held == value {
				// RFC 7950 Section 9.6.4.2: "This integer value MUST be in the
				// range -2147483648 to 2147483647, and it MUST be unique within
				// the enumeration type."
				return enumAssignment{}, fmt.Errorf("%w: %s: enum %q: value %d is already assigned to enum %q",
					ErrEnumValue, gyang.Source(enum), enum.Name, value, assigned.names[i])
			}
		}
		if len(assigned.names) == 0 || value > highest {
			highest = value
		}
		assigned.names = append(assigned.names, enum.Name)
		assigned.values = append(assigned.values, value)
	}
	return assigned, nil
}

// enumValue answers the value enum takes, given the names assigned before it
// and highest, the greatest value among them.
func enumValue(enum *gyang.Enum, assigned enumAssignment, highest int64) (int64, error) {
	if enum.Value != nil {
		// RFC 7950 Section 9.6.4.2: "This integer value MUST be in the range
		// -2147483648 to 2147483647".
		value, err := strconv.ParseInt(strings.TrimSpace(enum.Value.Name), 10, 32)
		if err != nil {
			return 0, fmt.Errorf("%w: %s: enum %q: value %q is not an integer from -2147483648 to 2147483647",
				ErrEnumValue, gyang.Source(enum), enum.Name, enum.Value.Name)
		}
		return value, nil
	}
	if len(assigned.names) == 0 {
		return 0, nil
	}
	// RFC 7950 Section 9.6.4.2: "If the current highest value is equal to
	// 2147483647, then an enum value MUST be specified for "enum"
	// substatements following the one with the current highest value."
	if highest == math.MaxInt32 {
		return 0, fmt.Errorf("%w: %s: enum %q needs a value: the highest value before it is 2147483647",
			ErrEnumValue, gyang.Source(enum), enum.Name)
	}
	return highest + 1, nil
}
