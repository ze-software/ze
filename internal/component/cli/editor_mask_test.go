package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

const maskTestHash = "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO"

// bcryptEditorSchema builds a schema with a ze:bcrypt leaf at
// system.authentication.user{name}.password.
func bcryptEditorSchema() *config.Schema {
	schema := config.NewSchema()
	pw := config.Leaf(config.TypeString)
	pw.Bcrypt = true
	users := config.List(config.TypeString, config.Field("password", pw))
	users.KeyName = "name"
	auth := config.Container(config.Field("user", users))
	sys := config.Container(config.Field("authentication", auth))
	schema.Define("system", sys)
	return schema
}

func bcryptEditor(t *testing.T) *Editor {
	t.Helper()
	schema := bcryptEditorSchema()
	tree := config.NewTree()
	auth := tree.GetOrCreateContainer("system").GetOrCreateContainer("authentication")
	entry := config.NewTree()
	entry.Set("password", maskTestHash)
	auth.AddListEntry("user", "alice", entry)
	return &Editor{tree: tree, schema: schema, treeValid: true}
}

// VALIDATES: AC-5 -- DisplayContentAtPath masks ze:bcrypt leaves (the web CLI
// `show` verb renders through this), while ContentAtPath (validation/persistence)
// keeps the real hash.
// PREVENTS: the web /cli show verb leaking the raw bcrypt hash to the browser.
func TestDisplayContentAtPathMasksBcrypt(t *testing.T) {
	e := bcryptEditor(t)

	masked := e.DisplayContentAtPath(nil)
	assert.Contains(t, masked, config.SecretDataPlaceholder, "display must mask the hash")
	assert.NotContains(t, masked, maskTestHash, "display must not contain the raw hash")

	// The unmasked twin (feeds validation/persistence) keeps the real hash.
	raw := e.ContentAtPath(nil)
	assert.Contains(t, raw, maskTestHash, "ContentAtPath must keep the real hash for commit/validation")
}
