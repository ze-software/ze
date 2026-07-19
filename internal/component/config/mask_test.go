package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validBcryptHash = "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234"

// maskTestSchema has, under system.authentication.user{name}, a ze:bcrypt
// "password", a ze:sensitive "api-secret", and a plain "shell" leaf.
func maskTestSchema() *Schema {
	schema := NewSchema()

	pw := Leaf(TypeString)
	pw.Bcrypt = true
	secret := Leaf(TypeString)
	secret.Sensitive = true
	plain := Leaf(TypeString)

	users := List(TypeString,
		Field("password", pw),
		Field("api-secret", secret),
		Field("shell", plain),
	)
	users.KeyName = "name"

	auth := Container(Field("user", users))
	sys := Container(Field("authentication", auth))
	schema.Define("system", sys)
	return schema
}

func maskTreeWithUser(t *testing.T, password string) *Tree {
	t.Helper()
	tree := NewTree()
	auth := tree.GetOrCreateContainer("system").GetOrCreateContainer("authentication")
	entry := NewTree()
	if password != "" {
		entry.Set("password", password)
	}
	entry.Set("api-secret", "topsecret")
	entry.Set("shell", "/bin/sh")
	auth.AddListEntry("user", "alice", entry)
	return tree
}

func aliceOf(tree *Tree) *Tree {
	return tree.GetContainer("system").GetContainer("authentication").GetList("user")["alice"]
}

// VALIDATES: AC-5 — MaskBcrypt replaces bcrypt leaf values with the placeholder,
// leaves sensitive/plain leaves untouched, and does not mutate the input tree.
func TestMaskBcryptLeavesForDisplay(t *testing.T) {
	schema := maskTestSchema()
	tree := maskTreeWithUser(t, validBcryptHash)

	masked := MaskBcrypt(tree, schema)

	// Masked clone: bcrypt leaf hidden.
	mAlice := aliceOf(masked)
	require.NotNil(t, mAlice)
	got, _ := mAlice.Get("password")
	assert.Equal(t, SecretDataPlaceholder, got, "bcrypt leaf must be masked")
	// Non-bcrypt leaves untouched by MaskBcrypt.
	secret, _ := mAlice.Get("api-secret")
	assert.Equal(t, "topsecret", secret, "sensitive leaf must not be touched by the bcrypt mask")
	shell, _ := mAlice.Get("shell")
	assert.Equal(t, "/bin/sh", shell, "plain leaf must not be touched")

	// Original tree unmodified (clone semantics).
	oAlice := aliceOf(tree)
	orig, _ := oAlice.Get("password")
	assert.Equal(t, validBcryptHash, orig, "MaskBcrypt must not mutate the input tree")
}

// VALIDATES: an empty bcrypt leaf is not turned into the placeholder.
func TestMaskBcryptSkipsEmpty(t *testing.T) {
	schema := maskTestSchema()
	tree := maskTreeWithUser(t, "") // no password set

	masked := MaskBcrypt(tree, schema)
	_, present := aliceOf(masked).Get("password")
	assert.False(t, present, "empty/absent bcrypt leaf must stay absent, not become the placeholder")
}

// VALIDATES: nil-safety.
func TestMaskBcryptNilInputs(t *testing.T) {
	assert.Nil(t, MaskBcrypt(nil, nil))
	tree := NewTree()
	assert.Same(t, tree, MaskBcrypt(tree, nil), "nil schema returns the tree unchanged")
}

// VALIDATES: BcryptKeys collects only ze:bcrypt leaf names.
func TestBcryptKeys(t *testing.T) {
	keys := BcryptKeys(maskTestSchema())
	assert.True(t, keys["password"], "password is a bcrypt leaf")
	assert.False(t, keys["api-secret"], "api-secret is sensitive, not bcrypt")
	assert.False(t, keys["shell"], "shell is a plain leaf")
	assert.Nil(t, BcryptKeys(nil))
}

// VALIDATES: AC-8 — the placeholder guard rejects a bcrypt leaf whose value is
// exactly the display placeholder, naming the offending leaf.
func TestRejectMaskedBcryptLeaves(t *testing.T) {
	schema := maskTestSchema()

	// A tree carrying the placeholder on the bcrypt leaf is rejected.
	bad := maskTreeWithUser(t, SecretDataPlaceholder)
	err := RejectMaskedBcryptLeaves(bad, schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "alice")
	assert.Contains(t, err.Error(), "password")
	assert.Contains(t, err.Error(), "plaintext-")

	// A tree with the real hash passes.
	good := maskTreeWithUser(t, validBcryptHash)
	assert.NoError(t, RejectMaskedBcryptLeaves(good, schema))

	// Empty / nil are no-ops.
	assert.NoError(t, RejectMaskedBcryptLeaves(NewTree(), schema))
	assert.NoError(t, RejectMaskedBcryptLeaves(nil, nil))
}
