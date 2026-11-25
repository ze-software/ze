// Design: docs/guide/config-editor.md -- ordered entries through a validated commit.

package cli

import (
	"strings"
	"testing"
)

// TestListOrderModelCommitClearsDirty exercises the command validation that a
// direct Editor.CommitSession call does not run. A failed command reports its
// status and full diagnostic output instead of hiding behind a dirty flag.
func TestListOrderModelCommitClearsDirty(t *testing.T) {
	for _, session := range []bool{false, true} {
		name := "file"
		if session {
			name = "session"
		}
		t.Run(name, func(t *testing.T) {
			content := strings.Replace(editorListOrderConfig, "interface {", "interface {\n  backend kernel;", 1)
			ed, path := newListOrderEditor(t, session, content)
			model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = model.Close() })
			for _, command := range []string{
				"delete interface dummy zulu",
				"set interface dummy zulu description first",
			} {
				if _, err := model.dispatchCommand(command); err != nil {
					t.Fatal(err)
				}
			}
			result, err := model.dispatchCommand("commit now")
			if err != nil {
				t.Fatal(err)
			}
			if ed.Dirty() {
				t.Fatalf("commit left pending changes: status=%q, output=%q", result.statusMessage, result.output)
			}
			requireEditorListOrder(t, reopenListOrderEditor(t, ed, path, session).tree, []string{"alpha", "mike", "zulu"})
		})
	}
}
