// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: validator.go — validateYangType dispatches the empty, binary and bits kinds here
// RFC: rfc/short/rfc7950.md

package yang

import (
	"encoding/base64"
	"reflect"
	"strconv"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// emptyPresence is the one value the config tree stores for a present
// type-empty leaf (the config package's configTrue): the parser writes it for
// a bare "name;" and for its round-trip form "name true;".
const emptyPresence = "true"

// validateEmpty validates a type-empty leaf value. RFC 7950 Section 9.11.2
// gives the type no lexical representation ("Not applicable"), so the only
// value a present leaf holds is the presence marker the tree stores.
func (v *Validator) validateEmpty(path string, value any) error {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Path:     path,
			Type:     ErrTypeType,
			Message:  "expected a presence flag",
			Expected: emptyPresence,
			Got:      valueTypeName(value),
		}
	}
	if str != emptyPresence {
		var tb textbuf.Buffer
		return &ValidationError{
			Path:     path,
			Type:     ErrTypeType,
			Message:  tb.Str("a flag takes no value, got ").Quoted(str).Str(": write the flag alone to set it, or delete it to clear it").String(),
			Expected: emptyPresence,
			Got:      str,
		}
	}
	return nil
}

// validateBinary validates a binary value: its base64 form, then the decoded
// octet count against the type's length.
func (v *Validator) validateBinary(path string, yangType *yang.YangType, value any) error {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Path:     path,
			Type:     ErrTypeType,
			Message:  "expected base64-encoded binary",
			Expected: yangType.Name,
			Got:      valueTypeName(value),
		}
	}
	// RFC 7950 Section 9.8.2: "Binary values are encoded with the base64
	// encoding scheme (see Section 4 in [RFC4648])." RFC 4648 Section 3.3
	// has an implementation reject a character outside the alphabet unless
	// the referring specification says otherwise, and Go's decoder skips CR
	// and LF even in strict mode, so they are refused first. Strict mode
	// also refuses non-zero padding bits, which Section 3.5 lets a decoder
	// reject.
	if strings.ContainsAny(str, "\r\n") {
		return binaryLexicalError(path, yangType, str)
	}
	octets, err := base64.StdEncoding.Strict().DecodeString(str)
	if err != nil {
		return binaryLexicalError(path, yangType, str)
	}
	// RFC 7950 Section 9.8.1: "The length of a binary value is the number of
	// octets it contains."
	if len(yangType.Length) > 0 && !v.checkYangRange(uint64(len(octets)), yangType.Length) {
		return &ValidationError{
			Path:     path,
			Type:     ErrTypeLength,
			Message:  textbuf.StrIntStr("binary length ", int64(len(octets)), " octets is outside allowed range"),
			Expected: yangType.Length.String(),
			Got:      strconv.Itoa(len(octets)),
		}
	}
	return nil
}

// binaryLexicalError is the refusal for a value that is not base64.
func binaryLexicalError(path string, yangType *yang.YangType, str string) error {
	var tb textbuf.Buffer
	return &ValidationError{
		Path:     path,
		Type:     ErrTypeType,
		Message:  tb.Str("expected base64-encoded binary (RFC 4648 Section 4, with padding), got ").Quoted(str).String(),
		Expected: yangType.Name,
		Got:      str,
	}
}

// validateBits validates a bits value: every name a defined bit, none twice.
func (v *Validator) validateBits(path string, yangType *yang.YangType, value any) error {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Path:     path,
			Type:     ErrTypeType,
			Message:  "expected a space-separated list of bit names",
			Expected: yangType.Name,
			Got:      valueTypeName(value),
		}
	}
	// RFC 7950 Section 9.7.2: "The lexical representation of the bits type is
	// a space-separated list of the names of the bits that are set. A
	// zero-length string thus represents a value where no bits are set."
	// The separator is the space character alone, so a tab or a newline stays
	// inside a name and is refused as no defined bit.
	names := strings.FieldsFunc(str, func(r rune) bool { return r == ' ' })
	seen := make(map[string]struct{}, len(names))
	var tb textbuf.Buffer
	for _, name := range names {
		if !bitDefined(yangType, name) {
			return &ValidationError{
				Path:     path,
				Type:     ErrTypeEnum,
				Message:  tb.Str("bit ").Quoted(name).Str(" is not defined by the type: name only the defined bits ").Quoted(bitNames(yangType)).Str(", separated by spaces").String(),
				Expected: bitNames(yangType),
				Got:      str,
			}
		}
		// Ze's own rule, not Section 9.7.2's, which does not call a repeated
		// name malformed: a bit is set or not, so a second naming is refused
		// as the likely typo it is.
		if _, twice := seen[name]; twice {
			return &ValidationError{
				Path:     path,
				Type:     ErrTypeType,
				Message:  tb.Str("bit ").Quoted(name).Str(" is named twice: a bit is set or not, so name it once").String(),
				Expected: bitNames(yangType),
				Got:      str,
			}
		}
		seen[name] = struct{}{}
	}
	return nil
}

// bitDefined reports whether the type defines a bit called name. A bits type
// with no bit statement defines none, so every name is refused.
func bitDefined(yangType *yang.YangType, name string) bool {
	if yangType.Bit == nil {
		return false
	}
	return yangType.Bit.IsDefined(name)
}

// bitNames answers the defined bit names, space-separated as a value lists them.
func bitNames(yangType *yang.YangType) string {
	if yangType.Bit == nil {
		return ""
	}
	return strings.Join(yangType.Bit.Names(), " ")
}

// uncheckedKindError is the refusal for a value whose type Ze has no lexical
// check for. RFC 7950 Section 9.1: "Implementations MUST support all lexical
// representations specified in this document." Accepting the value unread
// would claim a check that never ran, so it is refused. The loader refuses a
// module that declares leafref, identityref or instance-identifier
// (ErrUncheckedType), so a value reaching here comes from a schema that did
// not pass through Loader.Resolve.
func uncheckedKindError(path string, yangType *yang.YangType, value any) error {
	got := valueTypeName(value)
	if str, ok := value.(string); ok {
		got = str
	}
	var tb textbuf.Buffer
	return &ValidationError{
		Path:     path,
		Type:     ErrTypeType,
		Message:  tb.Str("type ").Quoted(yangType.Name).Str(" (").Str(yangType.Kind.String()).Str(") has no value check in ze, so no value is accepted for it: declare the leaf in its module as a string with a pattern, or as an enumeration").String(),
		Expected: yangType.Name,
		Got:      got,
	}
}

// valueTypeName answers the Go type of value for a refusal, "nil" for none.
func valueTypeName(value any) string {
	t := reflect.TypeOf(value)
	if t == nil {
		return "nil"
	}
	return t.String()
}
