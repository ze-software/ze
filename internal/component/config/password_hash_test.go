package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// bcryptHashSchema returns a schema with a list "user" keyed by "name" and
// two leaves: canonical ze:bcrypt "password" plus ephemeral "plaintext-password".
// Mirrors the shape produced by ze-ssh-conf.yang after Phase 3.
func bcryptHashSchema() *Schema {
	schema := NewSchema()

	passwordLeaf := Leaf(TypeString)
	passwordLeaf.Bcrypt = true

	plaintextLeaf := Leaf(TypeString)
	plaintextLeaf.Ephemeral = true

	users := List(TypeString,
		Field("password", passwordLeaf),
		Field("plaintext-password", plaintextLeaf),
	)
	users.KeyName = "name"

	auth := Container(Field("user", users))
	sys := Container(Field("authentication", auth))
	schema.Define("system", sys)
	return schema
}

// TestApplyPasswordHashingPlaintextToHash: the canonical use case.
//
// VALIDATES: plaintext-password populates canonical password as bcrypt,
// plaintext sibling removed after hashing.
//
// PREVENTS: plaintext persisting on disk; canonical leaf left empty.
func TestApplyPasswordHashingPlaintextToHash(t *testing.T) {
	schema := bcryptHashSchema()
	tree := NewTree()
	sys := tree.GetOrCreateContainer("system")
	auth := sys.GetOrCreateContainer("authentication")
	entry := NewTree()
	entry.Set("plaintext-password", "secret")
	auth.AddListEntry("user", "alice", entry)

	_, hashErr := ApplyPasswordHashing(tree, schema)
	require.NoError(t, hashErr)

	alice := tree.GetContainer("system").GetContainer("authentication").GetList("user")["alice"]
	require.NotNil(t, alice)

	hash, hashOK := alice.Get("password")
	require.True(t, hashOK, "canonical password must be populated")
	assert.NotEmpty(t, hash)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte("secret")),
		"stored hash must validate against the original plaintext")

	_, plainOK := alice.Get("plaintext-password")
	assert.False(t, plainOK, "plaintext-password must be removed after hashing")
}

// TestApplyPasswordHashingIdempotent: running twice produces the same tree.
//
// VALIDATES: a tree with only canonical (hash already present) is unchanged.
//
// PREVENTS: re-hashing a hash, corruption on multiple commits.
func TestApplyPasswordHashingIdempotent(t *testing.T) {
	schema := bcryptHashSchema()
	tree := NewTree()
	sys := tree.GetOrCreateContainer("system")
	auth := sys.GetOrCreateContainer("authentication")
	entry := NewTree()
	entry.Set("plaintext-password", "secret")
	auth.AddListEntry("user", "alice", entry)

	hashed, hashErr := ApplyPasswordHashing(tree, schema)
	require.NoError(t, hashErr)
	assert.Equal(t, []string{"system.authentication.user.alice.password"}, hashed,
		"the walk reports the canonical leaf it hashed, by dot-path")
	alice := tree.GetContainer("system").GetContainer("authentication").GetList("user")["alice"]
	firstHash, _ := alice.Get("password")

	// Second invocation: no plaintext sibling -> no-op.
	hashed, hashErr = ApplyPasswordHashing(tree, schema)
	require.NoError(t, hashErr)
	assert.Empty(t, hashed, "a second run hashes nothing, so LoadConfig warns about nothing")
	secondHash, _ := alice.Get("password")
	assert.Equal(t, firstHash, secondHash, "second run must not re-hash the canonical")
}

// TestApplyPasswordHashingNoPlaintext: tree without plaintext-password is unchanged.
//
// VALIDATES: hook is a no-op when no plaintext is present.
//
// PREVENTS: accidental mutation of unrelated trees.
func TestApplyPasswordHashingNoPlaintext(t *testing.T) {
	schema := bcryptHashSchema()
	tree := NewTree()
	sys := tree.GetOrCreateContainer("system")
	auth := sys.GetOrCreateContainer("authentication")
	entry := NewTree()
	entry.Set("password", "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234")
	auth.AddListEntry("user", "bob", entry)

	_, hashErr := ApplyPasswordHashing(tree, schema)
	require.NoError(t, hashErr)

	bob := tree.GetContainer("system").GetContainer("authentication").GetList("user")["bob"]
	val, ok := bob.Get("password")
	require.True(t, ok)
	assert.Equal(t, "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234", val)
}

// TestApplyPasswordHashingEmptyPlaintext: empty plaintext hashes nothing and is
// still dropped.
//
// VALIDATES: two properties of hashPlaintextSibling for an empty value. It does
// not hash the empty string, and it deletes the leaf anyway, because
// plaintext-<name> is ze:ephemeral and must not reach a running tree or a
// serialized file whatever its value.
//
// PREVENTS: a stored hash of "" that would match any attacker input submitting an
// empty password (defense-in-depth), and a write-only leaf surviving into
// `show config` and into the file the editor commits.
func TestApplyPasswordHashingEmptyPlaintext(t *testing.T) {
	schema := bcryptHashSchema()
	tree := NewTree()
	sys := tree.GetOrCreateContainer("system")
	auth := sys.GetOrCreateContainer("authentication")
	entry := NewTree()
	entry.Set("plaintext-password", "")
	auth.AddListEntry("user", "eve", entry)

	_, hashErr := ApplyPasswordHashing(tree, schema)
	require.NoError(t, hashErr)

	eve := tree.GetContainer("system").GetContainer("authentication").GetList("user")["eve"]
	_, ok := eve.Get("password")
	assert.False(t, ok, "canonical must not be populated from empty plaintext")

	_, plainOK := eve.Get("plaintext-password")
	assert.False(t, plainOK, "the ephemeral leaf is dropped even when it is empty")
}

// TestApplyPasswordHashingMultipleUsers: each list entry is processed independently.
//
// VALIDATES: list iteration covers all entries; each is hashed separately.
//
// PREVENTS: skipping entries, shared-hash bugs.
func TestApplyPasswordHashingMultipleUsers(t *testing.T) {
	schema := bcryptHashSchema()
	tree := NewTree()
	sys := tree.GetOrCreateContainer("system")
	auth := sys.GetOrCreateContainer("authentication")

	a := NewTree()
	a.Set("plaintext-password", "alicepw")
	auth.AddListEntry("user", "alice", a)

	b := NewTree()
	b.Set("plaintext-password", "bobpw")
	auth.AddListEntry("user", "bob", b)

	_, hashErr := ApplyPasswordHashing(tree, schema)
	require.NoError(t, hashErr)

	users := tree.GetContainer("system").GetContainer("authentication").GetList("user")

	aliceHash, _ := users["alice"].Get("password")
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(aliceHash), []byte("alicepw")))
	assert.Error(t, bcrypt.CompareHashAndPassword([]byte(aliceHash), []byte("bobpw")),
		"alice's hash must not validate bob's plaintext")

	bobHash, _ := users["bob"].Get("password")
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(bobHash), []byte("bobpw")))
}

// TestApplyPasswordHashingNilInputs: nil tree or schema is a no-op.
//
// VALIDATES: defensive nil handling.
//
// PREVENTS: panics in call sites that pass optional tree/schema.
func TestApplyPasswordHashingNilInputs(t *testing.T) {
	for _, c := range []struct {
		tree   *Tree
		schema *Schema
	}{{nil, nil}, {NewTree(), nil}, {nil, NewSchema()}} {
		hashed, err := ApplyPasswordHashing(c.tree, c.schema)
		assert.NoError(t, err)
		assert.Empty(t, hashed, "a no-op walk reports no hashed leaf")
	}
}

// TestIsBcryptHash exercises the format check at boundaries.
func TestIsBcryptHash(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"valid 2a", "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234", true},
		{"valid 2b cost 12", "$2b$12$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234", true},
		{"valid 2y cost 04", "$2y$04$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234", true},
		{"plaintext", "secret", false},
		{"empty", "", false},
		{"truncated", "$2a$10$tooshort", false},
		{"wrong prefix", "$1$abc$xyz", false},
		{"$9$ obfuscation", "$9$abcdefg", false},
		{"trailing junk", "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234X", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, IsBcryptHash(c.in))
		})
	}
}

// TestCheckBcryptLeavesPlaintextWarns: literal plaintext on a canonical leaf
// produces a warning. AC-5 wiring.
func TestCheckBcryptLeavesPlaintextWarns(t *testing.T) {
	schema := bcryptHashSchema()
	tree := NewTree()
	auth := tree.GetOrCreateContainer("system").GetOrCreateContainer("authentication")
	entry := NewTree()
	entry.Set("password", "secret") // plaintext on canonical -> WARN
	auth.AddListEntry("user", "alice", entry)

	warnings := CheckBcryptLeaves(tree, schema)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "alice")
	assert.Contains(t, warnings[0], "password")
	assert.Contains(t, warnings[0], "bcrypt")
}

// TestCheckBcryptLeavesValidHashNoWarn: a properly-hashed value produces no warning.
func TestCheckBcryptLeavesValidHashNoWarn(t *testing.T) {
	schema := bcryptHashSchema()
	tree := NewTree()
	auth := tree.GetOrCreateContainer("system").GetOrCreateContainer("authentication")
	entry := NewTree()
	entry.Set("password", "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234")
	auth.AddListEntry("user", "bob", entry)

	assert.Empty(t, CheckBcryptLeaves(tree, schema))
}

// TestCheckBcryptLeavesEmptyNoWarn: missing/empty value is a separate concern.
func TestCheckBcryptLeavesEmptyNoWarn(t *testing.T) {
	schema := bcryptHashSchema()
	tree := NewTree()
	auth := tree.GetOrCreateContainer("system").GetOrCreateContainer("authentication")
	auth.AddListEntry("user", "ghost", NewTree())

	assert.Empty(t, CheckBcryptLeaves(tree, schema))
}

// TestApplyPasswordHashingOversizePlaintextRejected: a plaintext longer than
// bcrypt's 72-byte limit causes ApplyPasswordHashing to return an error
// rather than silently storing a hash that only validates the first 72 bytes.
//
// VALIDATES: commit hook fails fast on oversize input (matches bcrypt
// vendored behavior; ze passwd rejects too).
//
// PREVENTS: silent truncation surprise where alice can't authenticate with
// the full passphrase she set.
func TestApplyPasswordHashingOversizePlaintextRejected(t *testing.T) {
	schema := bcryptHashSchema()
	tree := NewTree()
	auth := tree.GetOrCreateContainer("system").GetOrCreateContainer("authentication")
	entry := NewTree()
	entry.Set("plaintext-password", strings.Repeat("a", 73))
	auth.AddListEntry("user", "alice", entry)

	_, err := ApplyPasswordHashing(tree, schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too long")
	assert.Contains(t, err.Error(), "72")

	// Tree must remain untouched on failure: plaintext leaf still present,
	// canonical leaf still empty. No partial state.
	alice := tree.GetContainer("system").GetContainer("authentication").GetList("user")["alice"]
	if v, ok := alice.Get("plaintext-password"); !ok || v == "" {
		t.Errorf("plaintext-password must remain on failure, got ok=%v val=%q", ok, v)
	}
	if _, ok := alice.Get("password"); ok {
		t.Errorf("canonical password must remain unset on failure")
	}
}
