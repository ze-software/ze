// VALIDATES: preview provenance follows the actual go-list input population.
// PREVENTS: silently omitting an embed, new runtime file or external dependency.
package buildcompile

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestSelectedInputsIncludesEmbedsAndUntrackedFiles(t *testing.T) {
	root := t.TempDir()
	body, err := json.Marshal(map[string]any{"Dir": filepath.Join(root, "cmd", "ze"), "ImportPath": "ze/cmd/ze", "GoFiles": []string{"main.go", "new_untracked.go"}, "SFiles": []string{"machine.s"}, "EmbedFiles": []string{"schema/config.yang"}})
	if err != nil {
		t.Fatal(err)
	}
	selected := map[string]bool{}
	if err := selectedInputs(root, body, selected); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cmd/ze/main.go", "cmd/ze/new_untracked.go", "cmd/ze/machine.s", "cmd/ze/schema/config.yang"} {
		if !selected[name] {
			t.Errorf("omitted selected input %s", name)
		}
	}
	for _, record := range []map[string]any{
		{"Dir": filepath.Join(root, "..", "modcache"), "ImportPath": "external", "GoFiles": []string{"external.go"}},
		{"Dir": filepath.Join(root, "cmd"), "ImportPath": "incomplete", "Incomplete": true},
	} {
		body, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := selectedInputs(root, body, map[string]bool{}); err == nil {
			t.Fatalf("accepted %v", record)
		}
	}
	if err := selectedInputs(root, nil, map[string]bool{}); err == nil {
		t.Fatal("empty compiler population accepted")
	}
}
