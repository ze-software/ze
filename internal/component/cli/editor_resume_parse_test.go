// Design: docs/architecture/web-interface.md -- committed baseline parsing on recovery.
// Related: editor_resume.go -- ResumeSession opens the existing committed view.
// Related: editor_mask.go -- stale set fields retain a masked display tree.

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestResumeSessionKeepsCommittedSetInspection compares a freshly constructed
// editor with recovery over the same stored bytes. Both set formats retain
// their known leaves when an obsolete field needs repair; ordinary set text
// is the positive control, and no path may publish or invent pending intent.
func TestResumeSessionKeepsCommittedSetInspection(t *testing.T) {
	for _, format := range []string{"set", "metadata"} {
		for _, stale := range []bool{false, true} {
			name := format + "/current"
			if stale {
				name = format + "/stale"
			}
			t.Run(name, func(t *testing.T) {
				content := "set bgp router-id 1.2.3.4\nset bgp session asn local 65000\n"
				if format == "metadata" {
					content = "#history @web %2000-01-01T00:00:01Z " + content
				}
				if stale {
					content += "set bgp retired-display-field obsolete\n"
				}
				path := writeTestConfig(t, content)
				store := newTestTreeStore(t, path)
				editor, err := NewEditorWithStorage(store, path)
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, editor.Close()) })
				require.True(t, editor.treeValid, "construction must already retain the known committed view")
				before := editor.DisplayContentAtPath(nil)
				require.Contains(t, before, "1.2.3.4")
				require.Contains(t, before, "65000")
				editor.SetSession(NewEditSession("alice", "web"))
				require.NoError(t, editor.ResumeSession())
				assert.Equal(t, before, editor.DisplayContentAtPath(nil))
				assert.Empty(t, editor.PendingChanges(editor.SessionID()))
				assert.False(t, editor.Dirty())
				unchanged, err := store.ReadFile(path)
				require.NoError(t, err)
				assert.Equal(t, content, string(unchanged), "opening the view never rewrites the committed bytes")
				if stale {
					_, _, err := config.NewSetParser(editor.schema).ParseWithMeta(content)
					require.Error(t, err, "inspection does not make an obsolete field valid for publication")
				}
			})
		}
	}
}
