//go:build ze_l2tp

package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

func TestBuildL2TPSessionsTableData_NoService(t *testing.T) {
	table := buildL2TPSessionsTableData()
	assert.Equal(t, "L2TP Sessions", table.Title)
	assert.Nil(t, table.Rows)
	assert.Equal(t, "L2TP subsystem is not running.", table.EmptyMessage)
	require.Len(t, table.Columns, 6)
	assert.Equal(t, "tunnel-id", table.Columns[0].Key)
	assert.Equal(t, "session-id", table.Columns[1].Key)
	assert.Equal(t, "username", table.Columns[2].Key)
}

func TestBuildL2TPConfigFormData_WithValues(t *testing.T) {
	tree := config.NewTree()
	l2tp := tree.GetOrCreateContainer("l2tp")
	l2tp.Set("enabled", "true")
	l2tp.Set("max-tunnels", "100")
	l2tp.Set("max-sessions", "50")
	l2tp.Set("hello-interval", "60")
	l2tp.Set("cqm-enabled", "true")
	l2tp.Set("max-logins", "10000")

	form := buildL2TPConfigFormData(tree, nil)
	assert.Equal(t, "L2TP Configuration", form.Title)
	require.Len(t, form.Fields, 9)
	assert.Equal(t, "true", form.Fields[0].Value)
	assert.Equal(t, "toggle", form.Fields[0].Type)
	assert.Equal(t, "100", form.Fields[1].Value)
	assert.Equal(t, "number", form.Fields[1].Type)
	assert.Equal(t, "50", form.Fields[2].Value)
	assert.Equal(t, "password", form.Fields[3].Type)
	assert.Equal(t, "60", form.Fields[4].Value)
	assert.Equal(t, "/config/form/l2tp/", form.SaveURL)
}

func TestBuildL2TPConfigFormData_NilTree(t *testing.T) {
	form := buildL2TPConfigFormData(nil, nil)
	assert.Equal(t, "L2TP Configuration", form.Title)
	require.Len(t, form.Fields, 9)
	assert.Empty(t, form.Fields[0].Value)
	assert.Empty(t, form.Fields[1].Value)
}

func TestBuildL2TPConfigFormData_EmptyTree(t *testing.T) {
	tree := config.NewTree()
	form := buildL2TPConfigFormData(tree, nil)
	require.Len(t, form.Fields, 9)
	assert.Empty(t, form.Fields[0].Value)
}

// TestBuildL2TPConfigFormDataDescribesEachFieldFromTheSchema proves the form
// quotes the YANG account of a leaf rather than a second one written beside it.
//
// VALIDATES: every field of the L2TP configuration form carries the description
// its own YANG node declares, the eight leaves and the listener list alike.
// PREVENTS: the divergence the page shipped with. The form paraphrased
// hello-retries in one line while ze-l2tp-conf.yang explains the ZLB ACK, the
// 31s retransmit exhaustion and the default, so the web operator and the CLI
// operator read two different accounts of one leaf and no gate compared them
// (plan/journal/helper-bypassed-by-an-open-coded-copy.md, ai/rules/principles.md).
func TestBuildL2TPConfigFormDataDescribesEachFieldFromTheSchema(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err, "the YANG schema must load")

	form := buildL2TPConfigFormData(config.NewTree(), schema)
	require.Len(t, form.Fields, len(l2tpConfigLeaves)+1)

	for i, want := range l2tpConfigLeaves {
		parts := splitConfigPath(want.path)
		leaf := findLeafNode(schema, parts[:len(parts)-1], parts[len(parts)-1])
		require.NotNil(t, leaf, "the schema must declare %s", want.path)
		require.NotEmpty(t, leaf.Description, "%s must carry a YANG description", want.path)
		assert.Equal(t, leaf.Description, form.Fields[i].Description,
			"field %s must show the description %s declares", want.name, want.path)
	}

	// hello-retries is where the page and the model had already diverged, so it
	// is named rather than left to the loop above.
	retries := findLeafNode(schema, []string{"l2tp"}, "hello-retries")
	require.NotNil(t, retries, "the schema must declare l2tp/hello-retries")
	assert.Equal(t, retries.Description, form.Fields[5].Description)
	assert.Contains(t, form.Fields[5].Description, "ZLB ACK",
		"the form must carry the whole explanation, not a paraphrase of it")

	servers := form.Fields[len(form.Fields)-1]
	assert.Equal(t, schemaDescription(schema, l2tpServerListPath), servers.Description)
	assert.NotEmpty(t, servers.Description, "the listener list must describe itself")
}

// TestSchemaDescriptionIsEmptyWithoutASchema proves the form fails closed. A
// hint the schema did not write is the second account this derivation removes,
// so a page rendered with no schema shows none (ai/rules/principles.md).
//
// VALIDATES: schemaDescription answers "" for a nil schema and for a path the
// schema does not declare.
// PREVENTS: an invented description returning through the derivation that
// deleted the literals.
func TestSchemaDescriptionIsEmptyWithoutASchema(t *testing.T) {
	assert.Empty(t, schemaDescription(nil, "l2tp/hello-retries"), "no schema means no description")

	schema, err := config.YANGSchema()
	require.NoError(t, err, "the YANG schema must load")
	assert.Empty(t, schemaDescription(schema, "l2tp/not-a-leaf"), "an unknown path describes nothing")
}

func TestBuildL2TPHealthTableData_NoService(t *testing.T) {
	table := buildL2TPHealthTableData()
	assert.Equal(t, "L2TP Health", table.Title)
	assert.Nil(t, table.Rows)
	assert.Equal(t, "L2TP subsystem is not running.", table.EmptyMessage)
	require.Len(t, table.Columns, 5)
	assert.Equal(t, "session", table.Columns[0].Key)
	assert.Equal(t, "state", table.Columns[3].Key)
}
