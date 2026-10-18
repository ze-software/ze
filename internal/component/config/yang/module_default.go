// Design: docs/architecture/config/yang-config-design.md -- YANG type validation
// Related: validator.go -- the type checks a canonical default then goes through
// RFC: rfc/short/rfc7950.md -- integer notations of a module default (Sections 9.1, 9.2.1)

package yang

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// errModuleIntegerNotation reports a module default that is in none of the three
// integer notations of RFC 7950 Section 9.2.1.
var errModuleIntegerNotation = errors.New("not a decimal, hexadecimal (0x...) or octal (0...) integer; " +
	"write the value in one of those notations")

// errModuleIntegerOctal reports a module default that starts with 0 and holds
// only decimal digits, one of them an 8 or a 9, which octal has no place for.
var errModuleIntegerOctal = errors.New("a leading 0 makes a YANG module default octal, and octal has no digit 8 or 9; " +
	"drop the leading 0 to write the value in decimal")

// errModuleIntegerRange reports a module default too large for any integer type.
var errModuleIntegerRange = errors.New("the value is larger than any YANG integer type holds (uint64 maximum " +
	"18446744073709551615); write a value the leaf's type can hold")

// errModuleUnionNoMember reports a union default that no member type accepts.
var errModuleUnionNoMember = errors.New("no member type of the union accepts it; " +
	"write a value one member type holds")

// moduleDefaultValidator checks a union default against each member type. It
// holds no loader: a type check reads the YangType alone.
var moduleDefaultValidator = NewValidator(nil)

// CanonicalModuleDefault returns the canonical form of a default statement read
// from a YANG module. An integer default may be written in hexadecimal or octal
// there, so it is read in its notation and returned in decimal, the form a config
// value takes. A default of any other type is returned unchanged, because only the
// integer types have a notation a module alone may use.
//
// A union default is read as the first member type that accepts it, so an integer
// member reads it in the module notations only when no member before it accepts
// the text as written.
//
// The caller MUST still check the returned value against the type: for an integer
// type this reads the notation and nothing else, so a range or a sign the type
// refuses passes through.
//
// It implements RFC 7950 Section 9.1 for an integer default: "Implementations
// MUST support all lexical representations specified in this document." The
// module notations it reads are those of Section 9.2.1.
func CanonicalModuleDefault(yangType *gyang.YangType, def string) (string, error) {
	if yangType == nil {
		return def, nil
	}
	if yangType.Kind == gyang.Yunion {
		return canonicalUnionDefault(yangType, def)
	}
	if !isIntegerKind(yangType.Kind) {
		return def, nil
	}
	negative, magnitude, err := parseModuleInteger(def)
	if err != nil {
		return "", err
	}
	var tb textbuf.Buffer
	if negative && magnitude != 0 {
		tb.Byte('-')
	}
	return tb.Uint(magnitude).String(), nil
}

// canonicalUnionDefault returns the canonical form of a union default: the form
// of the first member type, in declaration order, that accepts it. A member that
// is itself a union is read the same way; the recursion is bounded by the union
// nesting the module declares, which the schema loader has already resolved.
//
// RFC 7950 Section 9.12: "a value is validated consecutively against each member
// type, in the order they are specified in the "type" statement, until a match is
// found. The type that matched will be the type of the value for the node that
// was validated, and the encoding is interpreted according to the rules for that
// type."
// RFC 7950 Section 9.12.3: "The canonical form of a union value is the same as the
// canonical form of the member type of the value."
// So the matching member's canonical form is what it returns.
func canonicalUnionDefault(union *gyang.YangType, def string) (string, error) {
	var refusal error
	for _, member := range union.Type {
		canonical, err := CanonicalModuleDefault(member, def)
		if err == nil {
			err = moduleDefaultValidator.ValidateType("", member, canonical)
		}
		if err == nil {
			return canonical, nil
		}
		if refusal == nil {
			refusal = err
		}
	}
	if refusal == nil {
		return "", errModuleUnionNoMember
	}
	return "", fmt.Errorf("%w; the first member type refused it: %w", errModuleUnionNoMember, refusal)
}

// isIntegerKind reports whether kind is one of the eight built-in integer types.
func isIntegerKind(kind gyang.TypeKind) bool {
	//exhaustive:ignore // a predicate over the integer subset; every other kind answers false
	switch kind {
	case gyang.Yint8, gyang.Yint16, gyang.Yint32, gyang.Yint64,
		gyang.Yuint8, gyang.Yuint16, gyang.Yuint32, gyang.Yuint64:
		return true
	}
	return false
}

// parseModuleInteger reads an integer written in a YANG module default. Each
// digit run is parsed in an explicit base, so strconv accepts no prefix, sign or
// underscore of its own: "0b1", "0o17" and "1_0" are refused, as the RFC defines
// none of them.
//
// RFC 7950 Section 9.2.1: "An integer value is lexically represented as an
// optional sign ("+" or "-"), followed by a sequence of decimal digits."
// RFC 7950 Section 9.2.1: "The hexadecimal notation consists of an optional sign
// ("+" or "-"), followed by the characters "0x", followed by a number of
// hexadecimal digits where letters may be uppercase or lowercase. The octal
// notation consists of an optional sign ("+" or "-"), followed by the character
// "0", followed by a number of octal digits."
// RFC 7950 Section 9.2.1: "Note that if a default value in a YANG module has a
// leading zero ("0"), it is interpreted as an octal number." A lone "0" is the
// decimal zero, which is the same value.
func parseModuleInteger(def string) (negative bool, magnitude uint64, err error) {
	body := def
	if rest, ok := strings.CutPrefix(body, "-"); ok {
		negative = true
		body = rest
	} else {
		body = strings.TrimPrefix(body, "+")
	}
	digits, base := body, 10
	if hex, ok := strings.CutPrefix(body, "0x"); ok {
		digits, base = hex, 16
	} else if len(body) > 1 && body[0] == '0' {
		digits, base = body[1:], 8
	}
	magnitude, err = strconv.ParseUint(digits, base, 64)
	if err == nil {
		return negative, magnitude, nil
	}
	if errors.Is(err, strconv.ErrRange) {
		return false, 0, errModuleIntegerRange
	}
	if base == 8 && strings.Trim(digits, "0123456789") == "" {
		return false, 0, errModuleIntegerOctal
	}
	return false, 0, errModuleIntegerNotation
}
