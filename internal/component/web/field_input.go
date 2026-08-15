// Design: docs/architecture/web-interface.md -- Template rendering
// Related: view.go -- the values the editors below render
// Related: fragment.go -- buildFieldMeta, which decides a field's type

package web

import (
	"github.com/a-h/templ"
)

// The config editor picks a leaf's editor at render time, from a type string
// the YANG schema produced. That dispatch used to be a template lookup by name.
// The renderer executed "input_" + type. When no template carried that name, it
// executed input_text instead. A miss and a broken template were the same
// event, and both rendered nothing.
//
// The registry below is that dispatch made explicit. An editor is a templ
// component, so a wrong field name is a build failure. A type nobody registered
// reaches the text editor by a named rule, not through an error nobody read.

// fieldInputBuilder builds the editor for one leaf.
type fieldInputBuilder func(FieldMeta) templ.Component

// fieldInputs maps a FieldMeta.Type onto the editor that renders it. Adding an
// editor is a new .templ file and one line here. No other file changes.
//
// The literal is the registration. Go refuses a duplicate key in a map literal.
// Two editors claiming one type is therefore a build failure, not an init-order
// race over which one renders.
var fieldInputs = map[string]fieldInputBuilder{
	"bool":   inputBool,
	"enum":   inputEnum,
	"number": inputNumber,
	"text":   inputText,
}

// fieldInputFor resolves a leaf's editor. A type with no editor of its own gets
// the text editor. Seven of the nine types a leaf can carry reach it: string,
// uint16, uint32, int, ip, prefix and duration. None of them shows a range or a
// mask in the browser.
//
// The number editor is registered and no leaf reaches it, because
// valueTypeToFieldType (fragment.go) answers uint16, uint32 or int for a
// numeric leaf. The pre-port template lookup missed input_number for the same
// reason. Recorded in plan/journal/unwired-feature.md.
func fieldInputFor(f FieldMeta) templ.Component {
	if build, ok := fieldInputs[f.GetType()]; ok {
		return build(f)
	}

	return inputText(f)
}

// fieldComponent is one whole field: the label frame around the editor its type
// resolves to. RenderField and the detail fragment both render this.
func fieldComponent(f FieldMeta) templ.Component {
	return fieldWrapper(f, fieldInputFor(f))
}
