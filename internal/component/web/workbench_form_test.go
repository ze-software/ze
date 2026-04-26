package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRenderWorkbenchForm renders a form with multiple field types and verifies
// the form structure, field labels, Save and Discard buttons.
func TestRenderWorkbenchForm(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	data := WorkbenchFormData{
		Title:      "System Identity",
		SaveURL:    "/admin/system/identity/save",
		DiscardURL: "/show/system/identity/",
		Fields: []WorkbenchFormField{
			{Name: "hostname", Label: "Hostname", Type: "text", Value: "router-1", Required: true},
			{Name: "as-number", Label: "AS Number", Type: "number", Value: "65001"},
			{Name: "protocol", Label: "Protocol", Type: "dropdown", Value: "bgp", Options: []string{"bgp", "ospf", "static"}},
			{Name: "passive", Label: "Passive Mode", Type: "toggle", Value: "true"},
			{Name: "router-id", Label: "Router ID", Type: "ip", Value: "10.0.0.1"},
			{Name: "communities", Label: "Communities", Type: "list", Items: []string{"65001:100", "65001:200"}},
		},
	}

	html := string(r.RenderFragment("workbench_form", data))
	require.NotEmpty(t, html, "form fragment must render")

	// Form structure.
	assert.Contains(t, html, `wb-form`)
	assert.Contains(t, html, `System Identity`)
	assert.Contains(t, html, `wb-form-save`, "save button must be present")
	assert.Contains(t, html, `wb-form-discard`, "discard button must be present")
	assert.Contains(t, html, `/admin/system/identity/save`, "save URL must be present")
	assert.Contains(t, html, `/show/system/identity/`, "discard URL must be present")

	// All field labels.
	assert.Contains(t, html, `Hostname`)
	assert.Contains(t, html, `AS Number`)
	assert.Contains(t, html, `Protocol`)
	assert.Contains(t, html, `Passive Mode`)
	assert.Contains(t, html, `Router ID`)
	assert.Contains(t, html, `Communities`)

	// Required field marker.
	assert.Contains(t, html, `wb-form-label--required`, "required field must have class")

	// Field values.
	assert.Contains(t, html, `router-1`)
	assert.Contains(t, html, `65001`)
	assert.Contains(t, html, `10.0.0.1`)
	assert.Contains(t, html, `65001:100`)
	assert.Contains(t, html, `65001:200`)
}

// TestRenderWorkbenchForm_FieldTypes verifies that each field type renders
// the correct HTML element.
func TestRenderWorkbenchForm_FieldTypes(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	tests := []struct {
		name     string
		field    WorkbenchFormField
		contains string
	}{
		{
			name:     "text renders input text",
			field:    WorkbenchFormField{Name: "f1", Label: "Text", Type: "text", Value: "hello"},
			contains: `type="text"`,
		},
		{
			name:     "number renders input number",
			field:    WorkbenchFormField{Name: "f2", Label: "Number", Type: "number", Value: "42"},
			contains: `type="number"`,
		},
		{
			name:     "dropdown renders select",
			field:    WorkbenchFormField{Name: "f3", Label: "Choice", Type: "dropdown", Options: []string{"a", "b"}},
			contains: `<select`,
		},
		{
			name:     "toggle renders checkbox",
			field:    WorkbenchFormField{Name: "f4", Label: "Toggle", Type: "toggle", Value: "true"},
			contains: `type="checkbox"`,
		},
		{
			name:     "ip renders text input",
			field:    WorkbenchFormField{Name: "f5", Label: "IP", Type: "ip", Value: "10.0.0.1"},
			contains: `type="text"`,
		},
		{
			name:     "password renders password input",
			field:    WorkbenchFormField{Name: "f6", Label: "Secret", Type: "password", Value: ""},
			contains: `type="password"`,
		},
		{
			name:     "list renders list container",
			field:    WorkbenchFormField{Name: "f7", Label: "Items", Type: "list", Items: []string{"one"}},
			contains: `wb-form-list`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := WorkbenchFormData{
				Title:   "Test",
				SaveURL: "/save",
				Fields:  []WorkbenchFormField{tt.field},
			}

			html := string(r.RenderFragment("workbench_form", data))
			require.NotEmpty(t, html, "form fragment must render")
			assert.Contains(t, html, tt.contains)
		})
	}
}

// TestRenderWorkbenchForm_DisabledField verifies that disabled fields render
// the disabled attribute.
func TestRenderWorkbenchForm_DisabledField(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	data := WorkbenchFormData{
		Title:   "Test",
		SaveURL: "/save",
		Fields: []WorkbenchFormField{
			{Name: "readonly", Label: "Read Only", Type: "text", Value: "locked", Disabled: true},
		},
	}

	html := string(r.RenderFragment("workbench_form", data))
	require.NotEmpty(t, html, "form fragment must render")
	assert.Contains(t, html, `disabled`)
}
