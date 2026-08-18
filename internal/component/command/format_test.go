// Design: format.go -- YAML rendering for CLI output
//
// These tests own RenderYAML's shape rules. They came from
// internal/component/cli/client/main_test.go, where they reached RenderYAML
// through the client's local printFormatted. The client stopped formatting when
// the daemon took the job over (spec-fixit-cli-format-default-everywhere), so
// the cases moved to the package that produces the behavior. No assertion was
// dropped on the way.

package command

import (
	"encoding/json"
	"strings"
	"testing"
)

// renderJSON unmarshals a command answer and renders it, which is what every
// YAML caller does: RenderYAML takes the decoded data, never the JSON text.
func renderJSON(t *testing.T, payload string) string {
	t.Helper()
	var data any
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		t.Fatalf("fixture is not JSON: %v", err)
	}
	return RenderYAML(data)
}

// TestRenderYAMLScalarFields verifies that a flat record renders its key and its
// value.
//
// VALIDATES: a one-field object reaches the reader with both halves.
// PREVENTS: a rendering that drops the key, or the value, or both.
func TestRenderYAMLScalarFields(t *testing.T) {
	out := renderJSON(t, `{"version":"1.0"}`)

	for _, want := range []string{"version", "1.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("RenderYAML output = %q, want to contain %q", out, want)
		}
	}
}

// TestRenderYAMLNestedData verifies nested data formatting.
//
// VALIDATES: Nested maps and arrays format with proper indentation.
// PREVENTS: Nested data being flattened or misformatted.
func TestRenderYAMLNestedData(t *testing.T) {
	data := map[string]any{
		"peers": []any{
			map[string]any{"Address": "10.0.0.1", "State": "established"},
			map[string]any{"Address": "10.0.0.2", "State": "idle"},
		},
		"config": map[string]any{
			"local": map[string]any{"as": 65000},
		},
		"empty-list": []any{},
	}

	out := RenderYAML(data)

	// Check peer formatting
	if !strings.Contains(out, "10.0.0.1") {
		t.Errorf("output missing peer address: %q", out)
	}

	// Check empty list handling
	if !strings.Contains(out, "[]") {
		t.Errorf("output should show '[]' for empty list: %q", out)
	}

	// Check nested map
	if !strings.Contains(out, "local") {
		t.Errorf("output missing nested config: %q", out)
	}
}

// TestRenderYAMLStringList verifies string list formatting.
//
// VALIDATES: String arrays format as bullet points.
// PREVENTS: String lists being printed as Go slice syntax.
func TestRenderYAMLStringList(t *testing.T) {
	data := map[string]any{
		"commands": []any{
			"daemon shutdown",
			"peer list",
			"system help",
		},
	}

	out := RenderYAML(data)

	if !strings.Contains(out, "daemon shutdown") {
		t.Errorf("output missing command in list: %q", out)
	}

	if !strings.Contains(out, "- ") {
		t.Errorf("output should format list items with '- ': %q", out)
	}
}
