// Design: docs/guide/config-editor.md -- shared compare and insertion boundaries.

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestCompareMissingSavedBaseline refuses a nonexistent comparison baseline
// while retaining the ordinary comparison when the same baseline exists.
func TestCompareMissingSavedBaseline(t *testing.T) {
	m := testShowModel(t)
	require.Empty(t, m.editor.savedDraftContent())
	result, err := m.compareView(fmtTree, srcSaved, "")
	require.NoError(t, err)
	assert.Contains(t, result.output, "no saved draft")
	assert.Nil(t, result.configView, "missing saved content cannot mean an empty baseline")

	require.NoError(t, m.editor.store.WriteFile(m.editor.originalPath+".draft", []byte(m.editor.OriginalContent()), 0o600))
	result, err = m.compareView(fmtTree, srcSaved, "")
	require.NoError(t, err)
	require.NotNil(t, result.configView)
	assert.Empty(t, result.configView.content)
}

// TestInsertCreatesItsValidatedParent proves that insertion creates the key
// whose schema accepted it, persists that dependency, and refuses a missing
// reference without creating a partial path.
func TestInsertCreatesItsValidatedParent(t *testing.T) {
	const module = `module ze-insert-parent-conf {
 yang-version 1.1;
 namespace "urn:ze:insert-parent";
 prefix probe;
 container insert-parent {
   list lane {
     key "kind";
     leaf kind { type enumeration { enum blue; enum red; } }
     leaf-list member { type string; ordered-by user; }
   }
 }
}`
	t.Cleanup(configyang.RegisterModuleForTest("ze-insert-parent-conf.yang", module))
	ed, path := newLeafListSessionEditor(t, validBGPConfig)
	parent := []string{"insert-parent", "lane", "blue"}
	before := config.Serialize(ed.Tree(), ed.schema)
	require.Error(t, ed.InsertLeafListValue(parent, "member", "one", config.InsertBefore, "absent"))
	assert.Equal(t, before, config.Serialize(ed.Tree(), ed.schema))
	assert.False(t, ed.store.Exists(ChangePath(path, ed.session.User)))

	require.NoError(t, ed.InsertLeafListValue(parent, "member", "one", config.InsertLast, ""))
	require.NotNil(t, walkPath(ed.Tree(), ed.schema, parent))
	assert.Equal(t, []string{"one"}, walkPath(ed.Tree(), ed.schema, parent).GetMultiValues("member"))
	require.NoError(t, ed.SaveDraft())
	require.NoError(t, ed.SaveDraft(), "the materialized creation and insertion must replay together")
	reopened, err := NewEditorWithStorage(ed.store, path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reopened.Close() })
	reopened.SetSession(NewEditSession("thomas", "local"))
	require.True(t, reopened.LoadDraft())
	require.NotNil(t, walkPath(reopened.Tree(), ed.schema, parent))
	assert.Equal(t, []string{"one"}, walkPath(reopened.Tree(), ed.schema, parent).GetMultiValues("member"))
	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)
	committed, err := NewEditorWithStorage(ed.store, path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = committed.Close() })
	require.NotNil(t, walkPath(committed.Tree(), ed.schema, parent))
	assert.Equal(t, []string{"one"}, walkPath(committed.Tree(), ed.schema, parent).GetMultiValues("member"))
}
