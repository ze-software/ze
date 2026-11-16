// Design: docs/guide/config-editor.md -- typed collection replacement

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestLoadReplacePlainLeafListKeepsOrder proves a replacement moves existing
// members as requested, rather than treating the input as an unordered set.
func TestLoadReplacePlainLeafListKeepsOrder(t *testing.T) {
	ed, _, _ := newLoadSessionEditor(t, validBGPConfig)
	first := parseLoadInput(t, ed, `bgp { filter { import { filters [ first second ]; final reject; } } }`)
	require.NoError(t, ed.LoadMerge(nil, first))
	second := parseLoadInput(t, ed, `bgp { filter { import { filters [ second first ]; final reject; } } }`)
	require.NoError(t, ed.LoadReplace([]string{"bgp", "filter", "import"}, second.GetContainer("bgp").GetContainer("filter").GetContainer("import")))
	assert.Equal(t, []string{"second", "first"}, ed.WalkPath([]string{"bgp", "filter", "import"}).GetSlice("filters"))
	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)
	assert.Equal(t, []string{"second", "first"}, ed.WalkPath([]string{"bgp", "filter", "import"}).GetSlice("filters"))
}

// TestLoadMultiWordValuesUsesWholeStatement proves the multi-word kind accepted
// by the parser is applied by a load and preserves a quoted word's boundary.
func TestLoadMultiWordValuesUsesWholeStatement(t *testing.T) {
	ed, _, _ := newLoadSessionEditor(t, validBGPConfig)
	ed.schema.Define("graphical-words", &config.MultiLeafNode{Type: config.TypeString})
	loaded, err := ed.ParseLoad(`set graphical-words first "two words" last`, nil)
	require.NoError(t, err)
	require.NoError(t, ed.LoadMerge(nil, loaded))
	assert.Equal(t, []string{"first", "two words", "last"}, ed.tree.GetSlice("graphical-words"))
}

// TestLoadFreeformEntriesUsesSharedEditor proves quoted entry names can be
// changed atomically and removed through load, including session replay.
func TestLoadFreeformEntriesUsesSharedEditor(t *testing.T) {
	ed, _, _ := newLoadSessionEditor(t, validBGPConfig)
	ed.schema.Define("graphical-freeform", config.Freeform())
	loaded, err := ed.ParseLoad("set graphical-freeform \"entry with spaces\" true\nset graphical-freeform receive update\n", nil)
	require.NoError(t, err)
	require.NoError(t, ed.LoadMerge(nil, loaded))
	entries := ed.tree.GetContainer("graphical-freeform")
	require.NotNil(t, entries)
	value, present := entries.Get("entry with spaces")
	assert.True(t, present)
	assert.Equal(t, "true", value)
	replacement, err := ed.ParseLoad("set graphical-freeform receive refresh\n", nil)
	require.NoError(t, err)
	require.NoError(t, ed.LoadReplace(nil, replacement))
	value, present = ed.tree.GetContainer("graphical-freeform").Get("receive")
	assert.True(t, present)
	assert.Equal(t, "refresh", value)
	_, present = ed.tree.GetContainer("graphical-freeform").Get("entry with spaces")
	assert.False(t, present)
	require.NoError(t, ed.SaveDraft())
	_, present = ed.tree.GetContainer("graphical-freeform").Get("entry with spaces")
	assert.False(t, present, "deleted freeform entry must stay deleted after replay")
}

// TestLoadOrderedSequenceKeepsRepeats proves an ordered sequence survives load,
// saved-draft replay and commit without collapsing duplicate ASNs.
func TestLoadOrderedSequenceKeepsRepeats(t *testing.T) {
	ed, _, _ := newLoadSessionEditor(t, validBGPConfig)
	ed.schema.Define("graphical-sequence", &config.ValueOrArrayNode{Type: config.TypeString, Ordered: true})
	loaded, err := ed.ParseLoad("set graphical-sequence [ 65001 65001 65002 ]\n", nil)
	require.NoError(t, err)
	require.NoError(t, ed.LoadMerge(nil, loaded))
	assert.Equal(t, []string{"65001", "65001", "65002"}, ed.tree.GetSlice("graphical-sequence"))
	require.NoError(t, ed.SaveDraft())
	assert.Equal(t, []string{"65001", "65001", "65002"}, ed.tree.GetSlice("graphical-sequence"))
}

// TestInlineListCandidateSurvivesReplay checks the creator used by the graphical
// keyed form, followed by a field edit and normal saved-candidate replay.
func TestInlineListCandidateSurvivesReplay(t *testing.T) {
	ed, _, _ := newLoadSessionEditor(t, validBGPConfig)
	ed.schema.Define("graphical-inline", config.InlineList(config.TypeString, config.Field("description", config.Leaf(config.TypeString))))
	path := []string{"graphical-inline", "lab"}
	require.NoError(t, ed.CreateEntry(path))
	require.NoError(t, ed.SetValue(path, "description", "inline description"))
	require.NoError(t, ed.SaveDraft())
	entry := ed.tree.GetList("graphical-inline")["lab"]
	require.NotNil(t, entry)
	value, present := entry.Get("description")
	assert.True(t, present)
	assert.Equal(t, "inline description", value)
}
