// Design: docs/architecture/web-interface.md -- Template rendering
// Related: render.go -- the Renderer that calls the components these serve

package web

import (
	"github.com/ze-software/ze/internal/core/stringsx"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The helpers below build TEXT and attribute values for the templ components
// beside them. They build no markup: a tag literal in Go is what the port
// removes (AC-7 of plan/spec-web-templ-migration.md). A class list, a URL and a
// JSON payload are values, and Go is where a value is computed.

// intText renders a count for a reader.
func intText(n int) string {
	return textbuf.StringInt(int64(n))
}

// commitBarClass is the commit bar's class list. The bar is in the DOM on every
// editable page and `visible` is what shows it, so the class carries the
// pending-change state.
func commitBarClass(changes int) string {
	if changes > 0 {
		return "commit-bar visible"
	}

	return "commit-bar"
}

// pendingChangeLabel is the commit counter's text. It is empty with no pending
// change, which leaves the counter empty rather than reading "0".
func pendingChangeLabel(changes int) string {
	if changes <= 0 {
		return ""
	}

	var tb textbuf.Buffer

	tb.Int(int64(changes)).Str(" pending change")

	if changes != 1 {
		tb.Byte('s')
	}

	return tb.String()
}

// loginLang is the login page's html lang attribute. An unset locale renders
// English, which is what Translate falls back to.
func loginLang(locale string) string {
	if locale == "" {
		return "en"
	}

	return locale
}

// navSectionClass is one workbench nav section's class list.
func navSectionClass(selected bool) string {
	if selected {
		return "workbench-nav-section workbench-nav-section--active"
	}

	return "workbench-nav-section"
}

// navSubItemClass is one workbench nav sub-page's class list.
func navSubItemClass(selected bool) string {
	if selected {
		return "workbench-nav-subitem workbench-nav-subitem--active"
	}

	return "workbench-nav-subitem"
}

// fieldInputClass is a leaf editor's class list. ze-field-set marks a leaf the
// operator configured, so an unset leaf showing its schema default reads
// differently from one set to the same value.
func fieldInputClass(value string) string {
	if value != "" {
		return "ze-field-input ze-field-set"
	}

	return "ze-field-input"
}

// tristateDefaultClass is the class list of an unset boolean's track. The
// default direction is in the class, so the track leans the way the daemon
// behaves.
func tristateDefaultClass(def string) string {
	switch def {
	case boolTrue:
		return "ze-tristate-track ze-tristate-default ze-tristate-default-yes"
	case boolFalse:
		return "ze-tristate-track ze-tristate-default ze-tristate-default-no"
	}

	return "ze-tristate-track ze-tristate-default"
}

// enumBlankLabel is the text of an enum's blank option. It names the schema
// default when there is one, so an unset leaf shows what the daemon will use.
func enumBlankLabel(def string) string {
	if def == "" {
		return "-- select --"
	}

	var tb textbuf.Buffer

	return tb.Byte('(').Str(def).Byte(')').String()
}

// fieldPlaceholder is the placeholder an unset editor shows. A configured leaf
// has none: its value is already in the field.
func fieldPlaceholder(f FieldMeta) string {
	if f.Value != "" {
		return ""
	}

	if f.Default != "" {
		return f.Default
	}

	return f.Description
}

// configSetURL is the editor endpoint one leaf posts to.
func configSetURL(path string) string {
	var tb textbuf.Buffer

	return tb.Str("/config/set/").Str(path).String()
}

// fieldHxVals is the hx-vals payload naming the leaf a POST edits. The value
// itself travels in the form field, so only the leaf name is needed.
func fieldHxVals(leaf string) string {
	var tb textbuf.Buffer

	return tb.Str(`{"leaf":`).Quoted(leaf).Byte('}').String()
}

// fieldHxValsWith is the hx-vals payload for a control that carries its own
// value, such as the boolean track, which sends the value it toggles to.
func fieldHxValsWith(leaf, value string) string {
	var tb textbuf.Buffer

	return tb.Str(`{"leaf":`).Quoted(leaf).Str(`,"value":`).Quoted(value).Byte('}').String()
}

// splitFieldOptions splits an enum's comma-separated option list. An empty list
// yields no options, so the editor shows the blank option alone.
func splitFieldOptions(opts string) []string {
	if opts == "" {
		return nil
	}

	parts, _ := stringsx.SplitCount(opts, ",")

	return parts
}
