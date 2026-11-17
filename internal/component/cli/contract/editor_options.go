// Design: docs/guide/config-editor.md -- shared editor display preferences.

package contract

const (
	editorOptionEnabled  = "enable"
	editorOptionDisabled = "disable"
)

// EditorOption is one display preference accepted by the shared editor.
// Values are the literal argument choices; an empty choice means no argument.
type EditorOption struct {
	Name   string
	Help   string
	Column bool
	Values []string
}

// EditorOptions returns a detached catalog for CLI completion and graphical
// forms. It grants no access to process-wide environment settings.
func EditorOptions() []EditorOption {
	return []EditorOption{
		{Name: "author", Help: "Author column", Column: true, Values: []string{editorOptionEnabled, editorOptionDisabled}},
		{Name: "date", Help: "Date column", Column: true, Values: []string{editorOptionEnabled, editorOptionDisabled}},
		{Name: "source", Help: "Source column", Column: true, Values: []string{editorOptionEnabled, editorOptionDisabled}},
		{Name: "changes", Help: "Changes column", Column: true, Values: []string{editorOptionEnabled, editorOptionDisabled}},
		{Name: "errors", Help: "Diagnostic display", Values: []string{"hints", "hide"}},
		{Name: "all", Help: "Enable all display columns", Values: []string{""}},
		{Name: "none", Help: "Disable all display columns", Values: []string{""}},
	}
}
