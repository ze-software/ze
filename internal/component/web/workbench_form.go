// Design: docs/architecture/web-components.md -- Workbench form component
// Related: workbench_table.go -- Table component (sibling)
// Related: workbench_detail.go -- Detail panel (sibling)
// Related: render.go -- Fragment rendering
//
// Spec: plan/spec-web-3-foundation.md (Form component, Phase 5).
//
// WorkbenchFormData drives the workbench_form.html template, rendering a
// singleton configuration form with typed fields, save, and discard actions.

package web

import "strings"

//nolint:gochecknoglobals // Shared template helper state; immutable after init.
var formFieldIDReplacer = strings.NewReplacer("/", "-", ".", "-", ":", "-")

// WorkbenchFormData holds the data for a singleton configuration form.
type WorkbenchFormData struct {
	Title      string
	Fields     []WorkbenchFormField
	SaveURL    string
	DiscardURL string
}

// WorkbenchFormField describes one input field in the form.
type WorkbenchFormField struct {
	Name        string
	Path        string
	Label       string
	Type        string // "text", "number", "dropdown", "toggle", "ip", "list", "password"
	Value       string
	Options     []string // for dropdown type
	Items       []string // for list type
	Description string
	Required    bool
	Disabled    bool
}

func formFieldName(f WorkbenchFormField) string {
	if f.Path != "" {
		return "field:" + f.Path
	}
	return "field:" + f.Name
}

func formFieldChecked(f WorkbenchFormField) bool {
	return f.Value == "true"
}

// formFieldID returns a stable DOM id suffix for a config path.
func formFieldID(path string) string {
	return formFieldIDReplacer.Replace(path)
}
