package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestHumanizeFieldLabel verifies raw YANG field paths become friendly labels
// for the Add-entry overlay, upper-casing common networking acronyms (F16).
func TestHumanizeFieldLabel(t *testing.T) {
	cases := map[string]string{
		"connection/remote/ip": "Connection Remote IP",
		"connection/local/ip":  "Connection Local IP",
		"session/asn/local":    "Session ASN Local",
		"router-id":            "Router ID",
		"name":                 "Name",
		"ipv4/unicast":         "IPv4 Unicast",
	}
	for in, want := range cases {
		assert.Equal(t, want, humanizeFieldLabel(in), "humanizeFieldLabel(%q)", in)
	}
}

// TestRenderWorkbenchForm renders a form with multiple field types and verifies
// the form structure, field labels, Save and Discard buttons.
func TestRenderWorkbenchForm(t *testing.T) {
	renderer, err := NewRenderer()
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

	html := string(renderer.renderComponent("workbench_form", workbenchForm(data)))
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
	renderer, err := NewRenderer()
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

			html := string(renderer.renderComponent("workbench_form", workbenchForm(data)))
			require.NotEmpty(t, html, "form fragment must render")
			assert.Contains(t, html, tt.contains)
		})
	}
}

// TestRenderWorkbenchForm_DisabledField verifies that disabled fields render
// the disabled attribute.
func TestRenderWorkbenchForm_DisabledField(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	data := WorkbenchFormData{
		Title:   "Test",
		SaveURL: "/save",
		Fields: []WorkbenchFormField{
			{Name: "readonly", Label: "Read Only", Type: "text", Value: "locked", Disabled: true},
		},
	}

	html := string(renderer.renderComponent("workbench_form", workbenchForm(data)))
	require.NotEmpty(t, html, "form fragment must render")
	assert.Contains(t, html, `disabled`)
}

// TestWorkbenchFormNeverRendersAStoredSecret verifies a form the workbench
// serves carries the display placeholder and never the stored value.
// VALIDATES: the response body of a form holding a secret carries no secret.
// PREVENTS: the stored password reaching the browser in the page source,
// where view-source, the disk cache and any proxy reading the document can read
// it. type="password" masks the characters on screen and nothing else.
//
// test-relax: this drove workbenchForm with a hand-built password field. It
// proved the component masked what a test handed it, which is not the property.
// The page builds its own fields out of tree reads. A sensitive leaf typed as
// text went out in the clear underneath a green bar here. The producer is
// renderPageContent (workbench_pages.go), so the producer is what runs now.
func TestWorkbenchFormNeverRendersAStoredSecret(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	schema, tree := secretSchemaAndTree(true)
	req := httptest.NewRequest(http.MethodGet, "/show/api/", http.NoBody)
	content, handled := renderPageContent(renderer, req, []string{segAPI}, tree, schema, nil, nil, nil)
	require.True(t, handled, "the workbench must serve the API page")

	html := string(content)
	require.NotEmpty(t, html, "form fragment must render")

	assert.NotContains(t, html, storedSecret, "the stored secret must not reach the response body")
	assert.Contains(t, html, config.SecretDataPlaceholder, "the password input must carry the display placeholder")
	assert.Contains(t, html, `type="password"`, "the input must stay a password input")
	// A non-secret field is untouched, so the mask is not a blanket over the form.
	assert.Contains(t, html, `id="field-rest-cors-origin"`, "a text field still renders")
}

// TestWorkbenchFormRendersAnEmptySecretAsEmpty verifies an unset password field
// stays empty rather than showing a placeholder that stands for nothing.
// VALIDATES: the mask fires on a stored value, never on the absence of one.
// PREVENTS: an operator reading the placeholder as evidence that a password is
// set, and a save writing the placeholder into a leaf that had no value.
func TestWorkbenchFormRendersAnEmptySecretAsEmpty(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	schema, _ := secretSchemaAndTree(true)
	req := httptest.NewRequest(http.MethodGet, "/show/api/", http.NoBody)
	content, handled := renderPageContent(renderer, req, []string{segAPI}, config.NewTree(), schema, nil, nil, nil)
	require.True(t, handled, "the workbench must serve the API page")

	html := string(content)
	require.NotEmpty(t, html, "form fragment must render")
	assert.NotContains(t, html, config.SecretDataPlaceholder, "an unset secret renders no placeholder")
}
