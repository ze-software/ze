// Design: docs/architecture/config/yang-config-design.md -- Value refusals
// Related: schema.go -- the schema nodes whose restrictions a value is judged against
// Related: yang/refusal.go -- the refusal each restriction builds
// Related: parser.go -- the hierarchical parser frames the refusal for a file
// Related: setparser.go -- the set parsers frame it for a set line

package config

import (
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"unicode/utf8"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/suggest"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ValidateLeafValue validates a leaf value against its type and the YANG
// restrictions the schema node carries, in the order type, union, enum,
// range, length, pattern. path is the setting path as the operator types it,
// ending with the leaf's name: a parser that does not yet know the whole path
// passes the leaf's name alone and names the rest later (WithPath).
//
// An enum, union, range, length or pattern miss answers a
// *configyang.ValidationError holding the operator error built from the
// schema: the path, the value (masked on a secret leaf), the accepted values
// with their help, the bounds or the format, and a set-form fix a surface may
// replace (WithFix). A type miss keeps its own text.
func ValidateLeafValue(node *LeafNode, path []string, value string) error {
	return validateLeafRestrictions(node, leafSubject(node, path, lastWord(path), value))
}

// ValidateNodeValue judges value typed at path for the leaf or leaf-list node
// at path, with the checks the parsers apply: a leaf by its own restrictions
// (ValidateLeafValue, union arm included), each member of a leaf-list node by
// its member leaf (validateMembers). The set surfaces call it, so a value is
// refused at set exactly where a file holding it would be refused. A presence
// container, a flex node, a freeform node and an inline list carry no value
// restriction, so there is nothing here to judge: whether such a node may be
// set, and to what, is the editor's own verdict, as it was before this check
// existed (`set firewall ... then accept true` sets a presence container). A
// path the schema does not resolve refuses, because no check can answer for it.
func ValidateNodeValue(node Node, path []string, value string) error {
	switch n := node.(type) {
	case *LeafNode:
		return ValidateLeafValue(n, path, value)
	case *ValueOrArrayNode:
		return validateMembers(n.memberLeaf(), path, inlineValueItems(value))
	case *MultiLeafNode:
		return validateMembers(n.memberLeaf(), path, inlineValueItems(value))
	case *BracketLeafListNode:
		return validateMembers(n.memberLeaf(), path, inlineValueItems(value))
	case *ContainerNode, *FlexNode, *FreeformNode, *InlineListNode, *ListNode:
		// refuse-by-default: guards ValidateLeafValue -- these kinds hold no leaf restriction, so the leaf checks have no verdict to give and the editor's own set decides
		return nil
	}
	return fmt.Errorf("%s holds no value", lastWord(path))
}

// ValidateListKey validates a list key against the restrictions of the
// list's first key leaf. listPath is the setting path of the list, ending
// with its name; the refusal names it followed by the key, and its why names
// the key leaf. Its set-form fix names the list with the correction in the
// key's place, which a surface that holds the rest of the line replaces
// (WithFix). A list built with no key leaf (a schema written in Go) is judged
// by its key type alone.
func ValidateListKey(node *ListNode, listPath []string, key string) error {
	if node.KeyLeaf == nil {
		return ValidateValue(node.KeyType, key)
	}
	keyLeaf := node.FirstKeyLeafName()
	if keyLeaf == "" {
		keyLeaf = lastWord(listPath)
	}
	path := append(slices.Clip(listPath), key)
	err := validateLeafRestrictions(node.KeyLeaf, leafSubject(node.KeyLeaf, path, keyLeaf, key))
	refusal, ok := operatorRefusal(err)
	if !ok {
		return err
	}
	var fix textbuf.Buffer
	fix.Str(cmdSet)
	for _, word := range listPath {
		fix.Byte(' ').Str(QuoteIfNeeded(word))
	}
	return refusal.WithFix(fix.Byte(' ').Str(refusal.CorrectionText()).String())
}

// leafSubject names what a refusal of value refuses: where, which leaf, and
// whether the leaf holds a secret, which the refusal then masks.
func leafSubject(node *LeafNode, path []string, leaf, value string) configyang.RefusedValue {
	return configyang.RefusedValue{Path: path, Leaf: leaf, Value: value, Secret: LeafHoldsSecret(node)}
}

// lastWord answers the last element of path, the leaf a path ends at.
func lastWord(path []string) string {
	if len(path) == 0 {
		return ""
	}
	return path[len(path)-1]
}

// validateLeafRestrictions judges subject.Value against node.
func validateLeafRestrictions(node *LeafNode, subject configyang.RefusedValue) error {
	value := subject.Value
	if node.Type == TypeDecimal64 {
		return validateDecimal64Leaf(node, subject)
	}
	if err := ValidateValue(node.Type, value); err != nil {
		// A whole number too large for the type is outside every range the
		// leaf declares, and the range is what the operator can write.
		if len(node.Ranges) > 0 && allDigits(value) {
			return rangeRefusal(node, subject)
		}
		return err
	}
	if len(node.Union) > 0 {
		return validateUnionValue(node.Union, subject)
	}
	if len(node.Enums) > 0 && !slices.Contains(node.Enums, value) {
		return configyang.EnumRefusal(subject, enumValues(node.Enums, node.EnumHelp))
	}
	if len(node.Ranges) > 0 {
		if err := validateNumericRanges(node, subject); err != nil {
			return err
		}
	}
	if err := validateLengths(node, subject); err != nil {
		return err
	}
	return validateLeafPatterns(node, subject)
}

// enumValues pairs each accepted value, in declared order, with its ze:help.
func enumValues(enums []string, help map[string]string) []suggest.Value {
	values := make([]suggest.Value, 0, len(enums))
	for _, name := range enums {
		values = append(values, suggest.Value{Name: name, Help: help[name]})
	}
	return values
}

// validateDecimal64Leaf judges a decimal64 leaf: its form and fraction
// digits, then its range compared as a decimal.
//
// RFC 7950 Section 9.3.4: "The "fraction-digits" statement ... restricts the
// value space of the type by defining the total number of digits after the
// decimal point".
func validateDecimal64Leaf(node *LeafNode, subject configyang.RefusedValue) error {
	number, err := configyang.ParseDecimal64(subject.Value, node.FractionDigits)
	if err != nil {
		return err
	}
	if len(node.Ranges) == 0 {
		return nil
	}
	for _, r := range node.Ranges {
		inside, err := configyang.Decimal64InRange(number, r.Min, r.Max, node.FractionDigits)
		if err != nil {
			return err
		}
		if inside {
			return nil
		}
	}
	return rangeRefusal(node, subject)
}

// validateUnionValue judges a value against each member type of a union, in
// declared order, and accepts it at the first member that does. A union carries
// no restriction of its own, so the members are the whole check. Its refusal
// lists the values the enumeration members accept and names every other
// member type.
//
// RFC 7950 Section 9.12: "When interpreting an XML encoding, a value is
// validated consecutively against each member type, in the order they are
// specified in the "type" statement, until a match is found."
func validateUnionValue(members []UnionMember, subject configyang.RefusedValue) error {
	var values []suggest.Value
	var others []string
	for _, member := range members {
		if validateLeafRestrictions(member.Leaf, subject) == nil {
			return nil
		}
		if len(member.Leaf.Enums) > 0 {
			values = append(values, enumValues(member.Leaf.Enums, member.Leaf.EnumHelp)...)
			continue
		}
		others = append(others, member.TypeName)
	}
	return configyang.UnionRefusal(subject, values, others)
}

// validateLengths enforces YANG `length` on a string value. The length is
// counted in CHARACTERS, not bytes (RFC 7950 Section 9.4.4), so a multi-byte
// value is not rejected for its encoding.
//
// Length is checked BEFORE pattern: for an over-long value the length is the
// operator's actual mistake, and a pattern message about a 4096-character
// string is unreadable. A declared error-message is the refusal's why.
func validateLengths(node *LeafNode, subject configyang.RefusedValue) error {
	if len(node.Lengths) == 0 {
		return nil
	}
	got := big.NewInt(int64(utf8.RuneCountInString(subject.Value)))
	for _, r := range node.Lengths {
		low, ok := new(big.Int).SetString(r.Min, 10)
		if !ok {
			continue
		}
		high, ok := new(big.Int).SetString(r.Max, 10)
		if !ok {
			continue
		}
		if got.Cmp(low) >= 0 && got.Cmp(high) <= 0 {
			return nil
		}
	}
	return configyang.LengthRefusal(subject, formatNumericRanges(node.Lengths), node.Constraint.Length())
}

// validateLeafPatterns enforces a leaf's YANG patterns. The refusal shows the
// format in words, never the regular expression, and a pattern that declares
// an error-message speaks it as the refusal's why.
//
// RFC 7950 Section 7.5.4.1: "If the constraint evaluates to "false", the string is passed as <error-message> in the <rpc-error> in NETCONF."
func validateLeafPatterns(node *LeafNode, subject configyang.RefusedValue) error {
	for _, pattern := range node.Patterns {
		matched, err := configyang.MatchPattern(pattern, subject.Value)
		if err != nil {
			return fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
		if matched {
			continue
		}
		return configyang.PatternRefusal(subject, pattern, node.Format, node.Constraint.Pattern(pattern))
	}
	return nil
}

// validateNumericRanges judges a number against the leaf's ranges.
func validateNumericRanges(node *LeafNode, subject configyang.RefusedValue) error {
	parsed, err := parseNumericRangeValue(node.Type, subject.Value)
	if err != nil {
		return err
	}
	for _, r := range node.Ranges {
		low, ok := new(big.Int).SetString(r.Min, 10)
		if !ok {
			continue
		}
		high, ok := new(big.Int).SetString(r.Max, 10)
		if !ok {
			continue
		}
		if parsed.Cmp(low) >= 0 && parsed.Cmp(high) <= 0 {
			return nil
		}
	}
	return rangeRefusal(node, subject)
}

// rangeRefusal is the refusal of a value outside the leaf's ranges: the
// declared bounds, or the error-message the range declares (RFC 7950 Section
// 7.5.4.1), as its why.
func rangeRefusal(node *LeafNode, subject configyang.RefusedValue) error {
	return configyang.RangeRefusal(subject, formatNumericRanges(node.Ranges), node.Constraint.Range())
}

// allDigits reports whether value is one or more ASCII decimal digits.
func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for i := range len(value) {
		if value[i] < '0' {
			return false
		}
		if value[i] > '9' {
			return false
		}
	}
	return true
}

func parseNumericRangeValue(typ ValueType, value string) (*big.Int, error) {
	// Only integer leaf types project to integer range values; a decimal64
	// leaf is compared by validateDecimal64Leaf.
	//exhaustive:ignore
	switch typ {
	case TypeUint16:
		v, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid uint16: %q", value)
		}
		return new(big.Int).SetUint64(v), nil
	case TypeUint32:
		v, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid uint: %q", value)
		}
		return new(big.Int).SetUint64(v), nil
	case TypeASN:
		// The YANG `range "1..4294967295"` on the asn typedef is compared
		// against the NUMBER, so a dotted value is read first. Without this
		// case a range check would refuse every asdot value it was given.
		v, err := asn.Parse(value)
		if err != nil {
			return nil, err
		}
		return new(big.Int).SetUint64(uint64(v)), nil
	case TypeInt:
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid int: %q", value)
		}
		return big.NewInt(v), nil
	default:
		return nil, fmt.Errorf("range validation not supported for type %s", typ)
	}
}

func formatNumericRanges(ranges []NumericRange) string {
	parts := make([]string, 0, len(ranges))
	for _, r := range ranges {
		if r.Min == r.Max {
			parts = append(parts, r.Min)
			continue
		}
		parts = append(parts, r.Min+".."+r.Max)
	}
	return textbuf.Join(parts, ", ")
}

// memberLeaf answers the restrictions each value of a multi-word leaf is
// judged against, as a leaf of its type: its enumeration, its format and its
// patterns.
func (n *MultiLeafNode) memberLeaf() *LeafNode {
	return &LeafNode{Type: n.Type, Enums: n.Enums, EnumHelp: n.EnumHelp, Format: n.Format, Patterns: n.Patterns, Constraint: n.Constraint}
}

// memberLeaf answers the restrictions each member of a bracket leaf-list is
// judged against, as a leaf of its type.
func (n *BracketLeafListNode) memberLeaf() *LeafNode {
	return &LeafNode{Type: n.Type, Enums: n.Enums, EnumHelp: n.EnumHelp, Format: n.Format, Patterns: n.Patterns, Constraint: n.Constraint}
}

// memberLeaf answers the restrictions each member of a leaf-list is judged
// against, as a leaf of its type: ValidValues is its enumeration.
func (n *ValueOrArrayNode) memberLeaf() *LeafNode {
	return &LeafNode{Type: n.Type, Enums: n.ValidValues, EnumHelp: n.EnumHelp, Format: n.Format, Patterns: n.Patterns, Constraint: n.Constraint}
}
