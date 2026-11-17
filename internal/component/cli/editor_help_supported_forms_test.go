// Design: docs/architecture/config/yang-config-design.md -- supported editor help forms.
// Related: editor_help.go -- the shared CLI and web help producer.

package cli

import (
	"strings"
	"testing"
)

// TestEditorHelpListsSupportedEditForms checks the published entries and help
// overlay while ordinary subsection editing remains available.
func TestEditorHelpListsSupportedEditForms(t *testing.T) {
	foundEdit := false
	for _, entry := range EditorHelpEntries() {
		if entry.Name == "edit <path>" {
			foundEdit = true
			if entry.Desc == "" {
				t.Error("ordinary edit help must explain the supported form")
			}
		}
		if strings.HasPrefix(entry.Name, "edit ") && strings.Contains(entry.Name, "*") {
			t.Errorf("help offers unsupported wildcard form %q", entry.Name)
		}
	}
	if !foundEdit {
		t.Error("help must retain the supported edit <path> form")
	}
	overlay := editorHelpCommands()
	if !strings.Contains(overlay, "edit <path>") {
		t.Error("the help overlay must retain ordinary subsection editing")
	}
	if strings.Contains(overlay, "edit <list> *") || strings.Contains(overlay, "Edit template for all entries") {
		t.Error("the help overlay advertises unsupported wildcard editing")
	}
}
