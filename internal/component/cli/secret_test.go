package cli

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// The value the fixture stores on every secret leaf, and the value an operator
// rotates it to. Both are distinctive, so a render that carries either one is
// caught wherever it appears.
const (
	storedCLISecret  = "cli-stored-secret-value"
	rotatedCLISecret = "cli-rotated-secret-value"
)

var secretCLIPath = []string{"environment", "api-server"}

// secretCLISchemaAndTree builds the fixture every display path below renders.
//
// A container holds a ze:sensitive leaf. A list under it holds one of its own,
// because a list entry is a separate walk in every serializer. A second
// top-level container is marked inactive and holds a third, so the inactive
// view carries a secret too. PruneActive keeps an inactive CONTAINER and an
// inactive list entry. It never keeps an inactive leaf under an active parent,
// so a leaf-level marking leaves that view empty and proves nothing.
//
// sensitive is a parameter so a caller can unmark the leaves and watch each
// path publish the value. That is the experiment this file exists for: the mask
// must follow the schema's marking and nothing else. A shipped YANG module
// cannot be unmarked from a test, so the fixture schema stands in for one.
//
// Every secret leaf holds the SAME value. One assertion therefore covers a view
// that renders the active half, and a view that renders the inactive half.
func secretCLISchemaAndTree(sensitive bool) (*config.Schema, *config.Tree) {
	newSecret := func() *config.LeafNode {
		leaf := config.Leaf(config.TypeString)
		leaf.Sensitive = sensitive
		return leaf
	}

	clients := config.List(config.TypeString,
		config.Field("token", newSecret()),
		config.Field("ip", config.Leaf(config.TypeIP)),
	)
	clients.KeyName = "name"

	schema := config.NewSchema()
	schema.Define("environment", config.Container(
		config.Field("api-server", config.Container(
			config.Field("token", newSecret()),
			config.Field("enabled", config.Leaf(config.TypeBool)),
			config.Field("client", clients),
		)),
	))
	schema.Define("standby", config.Container(
		config.Field("api-server", config.Container(
			config.Field("token", newSecret()),
		)),
	))

	entry := config.NewTree()
	entry.Set("token", storedCLISecret)
	entry.Set("ip", "192.0.2.1")

	apiServer := config.NewTree()
	apiServer.Set("token", storedCLISecret)
	apiServer.Set("enabled", "true")
	apiServer.AddListEntry("client", "fleet", entry)

	environment := config.NewTree()
	environment.SetContainer("api-server", apiServer)

	standbyServer := config.NewTree()
	standbyServer.Set("token", storedCLISecret)
	standby := config.NewTree()
	standby.SetContainer("api-server", standbyServer)
	standby.SetInactive(true)

	tree := config.NewTree()
	tree.SetContainer("environment", environment)
	tree.SetContainer("standby", standby)

	return schema, tree
}

// secretCLIEditor answers an editor whose committed config and working tree are
// both the fixture, with the real text on disk on both sides.
func secretCLIEditor(schema *config.Schema, tree *config.Tree) *Editor {
	content := config.Serialize(tree, schema)
	return &Editor{
		schema:          schema,
		tree:            tree,
		treeValid:       true,
		workingContent:  content,
		originalContent: content,
		showColumns:     newShowColumnDefaults(),
		diffGutter:      true,
	}
}

// staleFieldCLIEditor answers an editor whose tree did not parse and whose text
// carries a field the schema does not define.
//
// The display paths retry that text leniently, so this is the branch that masks
// rather than publishing the file. Only a config that parses NOWHERE falls back
// to its own text, and the comment on displayTreeOf says what that costs.
//
// The text is the SET format, because that is the format the lenient retry
// treats differently. parseConfigLenient (editor_draft.go) sets pre-migration
// on the set parser alone. A hierarchical config parses the same way twice.
func staleFieldCLIEditor(schema *config.Schema, tree *config.Tree) *Editor {
	editor := secretCLIEditor(schema, tree)
	editor.treeValid = false
	editor.tree = config.NewTree()
	editor.workingContent = config.SerializeSet(tree, schema) + "set not-in-the-schema field value\n"
	editor.originalContent = editor.workingContent

	return editor
}

// rotatedSecretCLITree answers the fixture with the container secret replaced,
// which is what an operator does when they rotate a credential.
func rotatedSecretCLITree(tree *config.Tree) *config.Tree {
	rotated := tree.Clone()
	rotated.GetContainer("environment").GetContainer("api-server").Set("token", rotatedCLISecret)
	return rotated
}

// rotatedSecretCLIEditor answers an editor whose working tree rotated the
// container secret and whose committed config still holds the old one.
func rotatedSecretCLIEditor(schema *config.Schema, tree *config.Tree) *Editor {
	rotated := rotatedSecretCLITree(tree)
	editor := secretCLIEditor(schema, rotated)
	editor.originalContent = config.Serialize(tree, schema)
	return editor
}

// secretCLIRenders is every path in this package that copies a stored leaf value
// into text the operator reads. Each one renders the fixture above.
//
// One producer is not a population. The bcrypt-only mask that stood here held
// this property over ze:bcrypt alone, and seven producers published every
// ze:sensitive leaf underneath it.
func secretCLIRenders(t *testing.T, schema *config.Schema, tree *config.Tree) map[string]string {
	t.Helper()

	editor := secretCLIEditor(schema, tree)
	stale := staleFieldCLIEditor(schema, tree)
	model := &Model{editor: editor}
	columns := config.ShowColumns{Author: true, Date: true}

	out := map[string]string{
		"display-content":         editor.DisplayContentAtPath(nil),
		"display-content-subpath": editor.DisplayContentAtPath(secretCLIPath),
		"display-original":        editor.DisplayOriginalContentAtPath(nil),
		"display-tree-json":       displayTreeJSON(t, editor, nil),
		"display-tree-json-sub":   displayTreeJSON(t, editor, secretCLIPath),
		"display-content-stale":   stale.DisplayContentAtPath(nil),
		"display-tree-json-stale": displayTreeJSON(t, stale, nil),
		"annotated-tree":          editor.annotatedViewOf(tree, nil, columns, false),
		"annotated-set":           editor.annotatedViewOf(tree, nil, columns, true),
		"blame":                   editor.blameView(),
		"search":                  searchResultText(model),
		"show-tree":               model.renderTreeAtPath(tree, nil, fmtTree),
		"show-set":                model.renderTreeAtPath(tree, nil, fmtConfig),
		"display-active":          filteredShowOutput(t, model, cmdActive),
		"display-inactive":        filteredShowOutput(t, model, cmdInactive),
		"tree-diff":               treeDiffOutput(t, schema, tree),
		"config-view":             configViewText(schema, tree),
		"commit-diff":             commitDiffText(t, schema, tree),
	}

	// "token" is the probe rather than a container name, because the sub-path
	// render starts BELOW api-server and the inactive view starts below standby.
	for name, rendered := range out {
		require.Contains(t, rendered, "token",
			"%s rendered no configuration, so this case would prove nothing", name)
	}

	return out
}

// displayTreeJSON answers the masked tree at path. That
// command encodes the tree itself, never the serialized text. The map is
// therefore its own producer, and it carries its own copy of every leaf value.
func displayTreeJSON(t *testing.T, editor *Editor, path []string) string {
	t.Helper()

	subtree := editor.DisplayTreeAtPath(path)
	require.NotNil(t, subtree, "the path did not resolve, so this case would prove nothing")

	encoded, err := json.Marshal(subtree.ToMap())
	require.NoError(t, err)

	return string(encoded)
}

// commitDiffText answers the diff of a rotated credential. The commit audit
// record, the REST and gRPC session diff, the hub, and the --dry-run form of
// `ze config set` all read that one string.
//
// Two leaves rotate, and each answers what the other cannot. The container leaf
// writes a line that names "token". A list entry serializes inline and names no
// field at all. The list entry writes a line no other line repeats. The
// container leaf and the inactive leaf hold the same text at the same
// indentation. computeDiff compares line SETS, so it drops a removal whose text
// still appears in the new file.
func commitDiffText(t *testing.T, schema *config.Schema, tree *config.Tree) string {
	t.Helper()

	rotated := rotatedSecretCLITree(tree)
	entry := rotated.GetContainer("environment").GetContainer("api-server").GetList("client")["fleet"]
	require.NotNil(t, entry, "the fixture list entry is gone, so this case would prove nothing")
	entry.Set("token", rotatedCLISecret)

	editor := secretCLIEditor(schema, rotated)
	editor.originalContent = config.Serialize(tree, schema)

	return editor.Diff()
}

// searchResultText joins every config search result, both the completion text
// and the description the dropdown shows beside it.
func searchResultText(m *Model) string {
	var b strings.Builder
	for _, completion := range m.searchConfig("") {
		b.WriteString(completion.Text)
		b.WriteByte('\n')
		b.WriteString(completion.Description)
		b.WriteByte('\n')
	}
	return b.String()
}

// filteredShowOutput answers what `show | display active` and
// `show | display inactive` print. Each prunes the tree before it serializes,
// so each is its own walk over its own clone.
func filteredShowOutput(t *testing.T, m *Model, filter string) string {
	t.Helper()

	result, err := m.cmdShowFiltered(filter, nil)
	require.NoError(t, err)

	return result.output
}

// treeDiffOutput answers the tree-aware diff of a rotation, given the two
// UNMASKED texts. The config view masks upstream, so this covers the mask this
// function keeps for a caller that does not.
func treeDiffOutput(t *testing.T, schema *config.Schema, tree *config.Tree) string {
	t.Helper()

	before := config.Serialize(tree, schema)
	after := config.Serialize(rotatedSecretCLITree(tree), schema)
	require.NotEqual(t, before, after, "the fixture did not rotate, so this case would prove nothing")

	annotated, _ := annotateContentWithTreeDiff(before, after, schema)

	return annotated
}

// configViewText answers both sides of the config view an operator sees after a
// rotation: the working text and the committed text it is compared against.
func configViewText(schema *config.Schema, tree *config.Tree) string {
	m := &Model{editor: rotatedSecretCLIEditor(schema, tree)}
	view := m.configViewAtPath(nil)

	return view.content + "\n" + view.originalContent
}

// VALIDATES: every SSH CLI display path masks a ze:sensitive value.
// PREVENTS: the config editor printing a stored credential on the terminal.
// A read-only operator reaches `show`, `show | display active`, `show | blame`
// and config search, and each one serialized the tree the parser decoded $9$
// into.
func TestNoCLIRenderPathEmitsAStoredSecret(t *testing.T) {
	schema, tree := secretCLISchemaAndTree(true)

	for name, rendered := range secretCLIRenders(t, schema, tree) {
		assert.NotContains(t, rendered, storedCLISecret, "%s published the stored secret", name)
		assert.NotContains(t, rendered, rotatedCLISecret, "%s published the replacement secret", name)
		assert.Contains(t, rendered, config.SecretDataPlaceholder, "%s rendered no placeholder", name)
	}
}

// VALIDATES: an unmarked leaf renders its value, so every path above owes its
// silence to the schema marking and not to a fixture that holds nothing.
// PREVENTS: a vacuous pass. A render that carries no configuration satisfies
// the test above and proves nothing. So does a mask keyed on something other
// than the schema.
func TestCLISecretMaskingFollowsTheSchemaMarking(t *testing.T) {
	schema, tree := secretCLISchemaAndTree(false)

	for name, rendered := range secretCLIRenders(t, schema, tree) {
		assert.Contains(t, rendered, storedCLISecret,
			"%s masked a leaf the schema does not mark, so its mask reads something else", name)
	}
}

// VALIDATES: DisplayTreeAtPath answers nothing when nothing can be masked.
// PREVENTS: a caller answering the live tree. config.MaskSecrets
// answers its INPUT unchanged when the schema is nil. A nil schema therefore
// handed the caller every stored value, and the map it encoded carried them.
// Every sibling fails closed. DisplayContentAtPath guards on the schema, and
// the web's EditorManager.ContentAtPath denies and logs on the same input.
//
// A configuration that parses nowhere answers nothing too. The empty tree
// newEditor substitutes would otherwise encode as `{}`, which reads as a
// configuration that parsed and held nothing.
func TestDisplayTreeAtPathFailsClosed(t *testing.T) {
	schema, tree := secretCLISchemaAndTree(true)

	t.Run("no schema", func(t *testing.T) {
		editor := secretCLIEditor(schema, tree)
		editor.schema = nil

		assert.Nil(t, editor.DisplayTreeAtPath(nil),
			"a nil schema answered the live tree, and the map its caller encodes carries "+storedCLISecret)
		assert.Nil(t, editor.DisplayTreeAtPath(secretCLIPath))
	})

	t.Run("nothing parses", func(t *testing.T) {
		editor := secretCLIEditor(schema, tree)
		editor.treeValid = false
		editor.tree = config.NewTree()
		editor.workingContent = "environment {\n    api-server {\n"

		assert.Nil(t, editor.DisplayTreeAtPath(nil))
		assert.Nil(t, editor.DisplayTreeAtPath(secretCLIPath))
	})

	t.Run("a stale field still answers a masked tree", func(t *testing.T) {
		editor := staleFieldCLIEditor(schema, tree)

		masked := editor.DisplayTreeAtPath(nil)
		require.NotNil(t, masked, "the lenient retry must answer a tree, or the JSON form loses a parsable config")
		assert.NotContains(t, editor.DisplayContentAtPath(nil), storedCLISecret,
			"the text form of the same branch published the secret")
	})
}

// VALIDATES: the config view says a secret changed, and says neither value.
// PREVENTS: the second-order effect of masking a diff. Both sides read the same
// placeholder, so the text comparison answers "no change" for a rotated
// credential. The operator must learn the leaf moved.
func TestTheConfigViewNamesARotatedSecret(t *testing.T) {
	schema, tree := secretCLISchemaAndTree(true)
	m := &Model{
		editor:   rotatedSecretCLIEditor(schema, tree),
		viewport: viewport.New(viewport.WithWidth(120), viewport.WithHeight(20)),
	}

	view := m.configViewAtPath(nil)
	require.Equal(t, view.originalContent, view.content,
		"the two masked texts must be equal here, or the text diff would see the rotation and this case would prove nothing")

	assert.Equal(t, []string{"environment.api-server.token"}, view.secretChanges,
		"the rotated leaf must be named")

	m.setViewportData(*view)
	assert.Contains(t, m.viewportContent,
		"~ environment.api-server.token "+config.SecretDataPlaceholder+" (secret changed)",
		"the view must say the secret changed")
	assert.NotContains(t, m.viewportContent, storedCLISecret, "the view published the old value")
	assert.NotContains(t, m.viewportContent, rotatedCLISecret, "the view published the new value")
}

// VALIDATES: `show | compare confirmed` names a rotated secret too.
// PREVENTS: the compare surface answering that nothing changed. Both sides are
// pruned against each other while unmasked, then each is masked, so the text
// the operator reads carries the same placeholder twice.
func TestCompareNamesARotatedSecret(t *testing.T) {
	schema, tree := secretCLISchemaAndTree(true)
	m := &Model{editor: rotatedSecretCLIEditor(schema, tree)}

	result, err := m.cmdShowDisplayWithSource(fmtTree, srcConfirmed, "")
	require.NoError(t, err)
	require.NotNil(t, result.configView)

	assert.Equal(t, []string{"environment.api-server.token"}, result.configView.secretChanges,
		"compare must name the rotated leaf")
	assert.NotContains(t, result.configView.content, storedCLISecret)
	assert.NotContains(t, result.configView.content, rotatedCLISecret)
	assert.NotContains(t, result.configView.originalContent, storedCLISecret)
	assert.NotContains(t, result.configView.originalContent, rotatedCLISecret)
}

// VALIDATES: secretChangeLines names each path once, sorted, with no value.
// PREVENTS: the line that carries the change carrying the secret with it.
func TestSecretChangeLinesPublishNoValue(t *testing.T) {
	assert.Empty(t, secretChangeLines(nil))

	paths := []string{"environment.api-server.token", "environment.api-server.client.fleet.token"}
	sort.Strings(paths)
	rendered := secretChangeLines(paths)

	for _, leafPath := range paths {
		assert.Contains(t, rendered, leafPath+" "+config.SecretDataPlaceholder+" (secret changed)")
	}
	assert.NotContains(t, rendered, storedCLISecret)
}
