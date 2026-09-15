// Design: docs/architecture/web-interface.md -- the config editor renders both help texts

package web

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// VALIDATES: AC-3, AC-5 -- the config editor puts the ze:help summary in the
// tooltip and the description explanation in a block under the input, escaped
// like every other text, and renders no block for a leaf that declares no
// explanation. Both the config view (leafInput) and the workbench editor
// (fieldWrapper) read the pair off the schema node.
// PREVENTS: the explanation an author writes on a config leaf reaching no
// web surface, and a multi-line explanation landing inside a title attribute.
func TestConfigLeafFormRendersBothTexts(t *testing.T) {
	leaf := &config.LeafNode{
		Type:        config.TypeString,
		ShortHelp:   "The summary",
		Description: "The <explanation> paragraph.",
	}

	field := buildLeafField("hold-time", leaf, "", false)
	assert.Equal(t, "The summary", field.ShortHelp)
	assert.Equal(t, "The <explanation> paragraph.", field.Description)

	var view bytes.Buffer
	require.NoError(t, leafInput(field).Render(context.Background(), &view))
	assert.Contains(t, view.String(), `title="The summary"`)
	assert.Contains(t, view.String(),
		`<p class="config-help">The &lt;explanation&gt; paragraph.</p>`)
	assert.NotContains(t, view.String(), `title="The <explanation>`)

	meta := buildFieldMetaFromLeaf("hold-time", leaf, "", "bgp")
	assert.Equal(t, "The summary", meta.ShortHelp)
	assert.Equal(t, "The <explanation> paragraph.", meta.Description)
	var workbench bytes.Buffer
	require.NoError(t, fieldWrapper(meta, templ.Raw(`<input id="probe">`)).
		Render(context.Background(), &workbench))
	assert.Contains(t, workbench.String(), `<span class="ze-field-tooltip" id="`)
	assert.Contains(t, workbench.String(), `aria-hidden="true">The summary</span>`)
	assert.Contains(t, workbench.String(),
		`<p class="ze-field-help">The &lt;explanation&gt; paragraph.</p>`)

	// A leaf that declares no explanation renders no block and no placeholder.
	silent := &config.LeafNode{Type: config.TypeString, ShortHelp: "The summary"}
	var none bytes.Buffer
	require.NoError(t, leafInput(buildLeafField("hold-time", silent, "", false)).
		Render(context.Background(), &none))
	assert.NotContains(t, none.String(), "config-help")
	none.Reset()
	require.NoError(t, fieldWrapper(buildFieldMetaFromLeaf("hold-time", silent, "", "bgp"),
		templ.Raw(`<input id="probe">`)).Render(context.Background(), &none))
	assert.NotContains(t, none.String(), "ze-field-help")
}

// TestConfigContainerAndListRenderTheExplanation: a container's and a list's
// description reach the config view as a block under the heading, and a node
// that declares none renders no block (AC-3 for the two grouping kinds).
func TestConfigContainerAndListRenderTheExplanation(t *testing.T) {
	container := &ConfigViewData{
		NodeKind:    config.NodeContainer,
		Description: "The <container> paragraph.",
	}
	var view bytes.Buffer
	require.NoError(t, configContainer(container).Render(context.Background(), &view))
	assert.Contains(t, view.String(), `<p class="config-help">The &lt;container&gt; paragraph.</p>`)

	list := &ConfigViewData{
		NodeKind:    config.NodeList,
		Description: "The <list> paragraph.",
		Keys:        []string{"one"},
		BasePath:    "/show/bgp/peer/",
	}
	view.Reset()
	require.NoError(t, configList(list).Render(context.Background(), &view))
	assert.Contains(t, view.String(), `<p class="config-help">The &lt;list&gt; paragraph.</p>`)

	view.Reset()
	require.NoError(t, configContainer(&ConfigViewData{NodeKind: config.NodeContainer}).
		Render(context.Background(), &view))
	assert.NotContains(t, view.String(), "config-help")
	view.Reset()
	require.NoError(t, configList(&ConfigViewData{NodeKind: config.NodeList, Keys: []string{"one"}}).
		Render(context.Background(), &view))
	assert.NotContains(t, view.String(), "config-help")
}

// TestConfigViewDataCarriesTheNodeExplanation: buildConfigViewData copies the
// container's and the list's description, each on its own.
func TestConfigViewDataCarriesTheNodeExplanation(t *testing.T) {
	peer := config.List(config.TypeString, config.Field("hold-time", config.Leaf(config.TypeUint32)))
	peer.ShortHelp = "A peer"
	peer.Description = "The peer paragraph."
	bgp := config.Container(config.Field("peer", peer))
	bgp.ShortHelp = "BGP"
	bgp.Description = "The BGP paragraph."
	schema := config.NewSchema()
	schema.Define("bgp", bgp)
	tree := config.NewTree()

	data, err := buildConfigViewData(schema, tree, []string{"bgp"})
	require.NoError(t, err)
	assert.Equal(t, "The BGP paragraph.", data.Description)

	data, err = buildConfigViewData(schema, tree, []string{"bgp", "peer"})
	require.NoError(t, err)
	assert.Equal(t, "The peer paragraph.", data.Description)
}
