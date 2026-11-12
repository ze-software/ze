// Design: docs/architecture/config/yang-config-design.md -- Value refusals
// Related: refusal.go -- the pattern refusal that shows this text
// Related: enum.go -- typeStatementOf's sibling readers of a leaf's type statements

package yang

import (
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"
)

// ValueFormat is what a pattern refusal tells the operator about the form a
// value takes, in words, never as the regular expression.
//
// The zero value holds no text and no type name: a refusal built from it says
// the leaf does not accept the value, which is all a model with no words for
// the format lets Ze say.
type ValueFormat struct {
	text     string // the format in words, "" when the model gives none
	typeName string // the nearest typedef's name, "" for an inline pattern
}

// NewValueFormat builds a format from its words and the name of the typedef
// that declares the pattern. Either may be empty. It is for a caller that
// holds no goyang entry (a test schema, a command argument); a config leaf
// reads its format with LeafValueFormat.
func NewValueFormat(text, typeName string) ValueFormat {
	return ValueFormat{text: text, typeName: typeName}
}

// Text answers the format in words, "" when the model gives none.
func (f ValueFormat) Text() string { return f.text }

// TypeName answers the name of the typedef that declares the pattern, "" for
// a pattern written on the leaf itself.
func (f ValueFormat) TypeName() string { return f.typeName }

// Placeholder answers what an operator replaces in a fix line: "<rate>" for a
// value of typedef rate, "<value>" for an inline pattern.
func (f ValueFormat) Placeholder() string {
	if f.typeName == "" {
		return "<value>"
	}
	return "<" + f.typeName + ">"
}

// LeafValueFormat answers the format of yt, a type leaf's type statement
// resolved to (the leaf's own type, or one of its union members): the nearest
// typedef's ze:help, else the first sentence of that typedef's description,
// else the leaf's own ze:help. A model that gives none of them answers the
// typedef's name alone, and an inline pattern on a leaf with no ze:help
// answers the zero value. The regular expression is never the text: it is
// what the operator cannot read.
//
// Only the nearest typedef is read. A typedef that refines another one names
// the narrower format, so its words describe the value better than its base's.
func LeafValueFormat(leaf *gyang.Entry, yt *gyang.YangType) ValueFormat {
	declared := entryTypeStatement(leaf)
	statement := typeStatementOf(declared, yt, 0)
	if statement == nil {
		statement = declared
	}
	var format ValueFormat
	if typedef := nearestTypedef(statement); typedef != nil {
		format.typeName = typedef.Name
		format.text = GetHelpExtension(typedef.Extensions)
		if format.text == "" {
			format.text = firstSentence(valueText(typedef.Description))
		}
	}
	if format.text == "" && leaf != nil {
		format.text = GetHelpExtension(leaf.Exts)
	}
	format.text = strings.Join(strings.Fields(format.text), " ")
	return format
}

// nearestTypedef answers the typedef statement's type names, or nil when
// statement is a builtin type. goyang leaves the typedef's own type statement
// at YangType.Base (resolvedBase), and that statement's parent is the typedef.
func nearestTypedef(statement *gyang.Type) *gyang.Typedef {
	if statement == nil {
		return nil
	}
	base := resolvedBase(statement)
	if base == nil {
		return nil
	}
	typedef, isTypedef := base.Parent.(*gyang.Typedef)
	if !isTypedef {
		return nil
	}
	return typedef
}

// firstSentence answers text up to its first sentence end (a full stop
// followed by white space or the end of the text), without that full stop, or
// the whole text when it holds no sentence end.
func firstSentence(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	for i := range len(text) {
		if text[i] != '.' {
			continue
		}
		if i+1 == len(text) || text[i+1] == ' ' {
			return text[:i]
		}
	}
	return text
}
