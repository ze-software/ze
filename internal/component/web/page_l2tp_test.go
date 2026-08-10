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

	form := buildL2TPConfigFormData(tree)
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
	form := buildL2TPConfigFormData(nil)
	assert.Equal(t, "L2TP Configuration", form.Title)
	require.Len(t, form.Fields, 9)
	assert.Empty(t, form.Fields[0].Value)
	assert.Empty(t, form.Fields[1].Value)
}

func TestBuildL2TPConfigFormData_EmptyTree(t *testing.T) {
	tree := config.NewTree()
	form := buildL2TPConfigFormData(tree)
	require.Len(t, form.Fields, 9)
	assert.Empty(t, form.Fields[0].Value)
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
