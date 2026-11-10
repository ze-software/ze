// Design: docs/architecture/config/yang-config-design.md -- Constraint error messages
// Related: validator.go -- the tree validator that speaks these messages
// Related: command.go -- the command argument definitions that carry them
// Related: rpc.go -- valueText, the reader of an optional statement's argument

package yang

import (
	gyang "github.com/openconfig/goyang/pkg/yang"
)

// ConstraintMessages holds the error-message strings that the range, length
// and pattern restrictions of one type declare. The zero value declares none,
// and every refusal then keeps its generic text.
//
// RFC 7950 Section 7.5.4.1: "If the constraint evaluates to "false", the string is passed as <error-message> in the <rpc-error> in NETCONF."
// The substatement tables of "range" (Section 9.2.4.1), "length" (Section
// 9.4.4.1) and "pattern" (Section 9.4.5.1) list "error-message", so a module
// author puts the operator's message on the constraint that refuses first.
// goyang parses the statement but its resolved YangType drops it, so Ze reads
// it from the type statements.
type ConstraintMessages struct {
	rangeText  string            // the error-message of the effective range
	lengthText string            // the error-message of the effective length
	patterns   map[string]string // pattern text to the error-message of that pattern
}

// Range answers the error-message of the effective range, "" when it has none.
func (m ConstraintMessages) Range() string { return m.rangeText }

// Length answers the error-message of the effective length, "" when it has none.
func (m ConstraintMessages) Length() string { return m.lengthText }

// Pattern answers the error-message of the pattern whose text is pattern, ""
// when that pattern has none.
func (m ConstraintMessages) Pattern(pattern string) string { return m.patterns[pattern] }

// LeafConstraintMessages answers the error-message strings of yt, a type that
// leaf's type statement resolved to: the leaf's own type, or one of its union
// members at any depth. A leaf with no type statement, or a yt that no
// statement under it resolved to, answers the zero value: a refusal then keeps
// its generic text, which is what a type with no error-message reads as.
func LeafConstraintMessages(leaf *gyang.Entry, yt *gyang.YangType) ConstraintMessages {
	return typeConstraintMessages(typeStatementOf(entryTypeStatement(leaf), yt, 0))
}

// typeConstraintMessages reads the error-message strings of declared and of
// the typedef chain it derives from.
//
// A derived range or length replaces the one of its base (RFC 7950 Section
// 9.2.4: "the new restriction MUST be equally limiting or more limiting"), so
// the message is read from the statement nearest declared that has one.
// Patterns add up along the chain (RFC 7950 Section 9.4.5: "values must match
// all patterns in the base type, in addition to the new patterns"), so each
// pattern keeps the message its own statement declares.
//
// The walk is bounded by typeDepthMax: goyang refuses a typedef chain that
// loops before this reader runs.
func typeConstraintMessages(declared *gyang.Type) ConstraintMessages {
	var messages ConstraintMessages
	rangeFound := false
	lengthFound := false
	node := declared
	for depth := 0; node != nil && depth <= typeDepthMax; depth++ {
		if !rangeFound && node.Range != nil {
			rangeFound = true
			messages.rangeText = valueText(node.Range.ErrorMessage)
		}
		if !lengthFound && node.Length != nil {
			lengthFound = true
			messages.lengthText = valueText(node.Length.ErrorMessage)
		}
		for _, pattern := range node.Pattern {
			text := valueText(pattern.ErrorMessage)
			if text == "" {
				continue
			}
			if messages.patterns == nil {
				messages.patterns = make(map[string]string)
			}
			if _, nearer := messages.patterns[pattern.Name]; nearer {
				continue
			}
			messages.patterns[pattern.Name] = text
		}
		node = resolvedBase(node)
	}
	return messages
}

// typeStatementOf answers the type statement under declared that goyang
// resolved yt from: declared itself, or a union member at any depth, along
// each typedef chain. It answers nil when no statement under declared did.
//
// The recursion follows the type nesting a loaded module declares, never
// input from outside Ze, and depth stops it at typeDepthMax.
func typeStatementOf(declared *gyang.Type, yt *gyang.YangType, depth int) *gyang.Type {
	if declared == nil {
		return nil
	}
	if depth > typeDepthMax {
		return nil
	}
	if declared.YangType == yt {
		return declared
	}
	for node := declared; node != nil && depth <= typeDepthMax; node = resolvedBase(node) {
		for _, member := range node.Type {
			if found := typeStatementOf(member, yt, depth+1); found != nil {
				return found
			}
		}
		depth++
	}
	return nil
}

// WithConstraintMessage joins a refusal and the error-message its constraint
// declares, so the operator reads what was refused and then the module
// author's reason and fix. An empty message leaves the refusal unchanged.
func WithConstraintMessage(refusal, message string) string {
	if message == "" {
		return refusal
	}
	return refusal + ": " + message
}
