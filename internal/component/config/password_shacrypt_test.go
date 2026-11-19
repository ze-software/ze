// Design: docs/architecture/config/syntax.md -- password hashing on commit
// Related: password_hash.go -- CheckBcryptLeaves and ApplyPasswordHashing, the producers these tests drive
// Related: loader.go -- LoadConfig, the boot and reload load path

package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// Published test vectors of the SHA-crypt specification: hashes another system
// wrote, never ones Ze produced.
const (
	sha512CryptHash = "$6$saltstring$svn8UoSVapNtMuq1ukKS4tPQd8iKwSMHWjl/O817G3uBnIFNjnQJuesI68u4OTLiBFdcbYEdFCoEOfaS35inz1"
	sha256CryptHash = "$5$rounds=10000$saltstringsaltst$3xv.VbSHBb41AL9AvLeujZkZRBAwqFMz2.opqey6IcA"
)

// shaCryptUserTree returns a tree holding one user whose password leaf is value.
func shaCryptUserTree(name, value string) *Tree {
	tree := NewTree()
	auth := tree.GetOrCreateContainer("system").GetOrCreateContainer("authentication")
	entry := NewTree()
	entry.Set("password", value)
	auth.AddListEntry("user", name, entry)
	return tree
}

// TestCheckBcryptLeavesAcceptsSHACrypt: a SHA-crypt hash on the canonical leaf
// is a stored hash, so the validator and the boot path do not warn about it.
//
// VALIDATES: `ze config validate` and the daemon's load warning accept a
// migrated account's $6$ and $5$ hash.
// PREVENTS: a warning telling the operator to re-hash a password the account's
// user already has, and cannot get back.
func TestCheckBcryptLeavesAcceptsSHACrypt(t *testing.T) {
	schema := bcryptHashSchema()
	assert.Empty(t, CheckBcryptLeaves(shaCryptUserTree("migrated", sha512CryptHash), schema), "$6$")
	assert.Empty(t, CheckBcryptLeaves(shaCryptUserTree("migrated", sha256CryptHash), schema), "$5$")
}

// TestCheckBcryptLeavesWarnsMalformedSHACrypt: a value that names SHA-crypt
// but cannot be one (the digest is cut short) still warns, and the warning
// names both accepted formats.
func TestCheckBcryptLeavesWarnsMalformedSHACrypt(t *testing.T) {
	schema := bcryptHashSchema()
	warnings := CheckBcryptLeaves(shaCryptUserTree("migrated", sha512CryptHash[:40]), schema)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "migrated")
	assert.Contains(t, warnings[0], "SHA-crypt")
	assert.NotContains(t, warnings[0], sha512CryptHash[:40], "the warning carries no part of the stored value")
}

// TestApplyPasswordHashingKeepsSHACrypt: the commit transform stores a
// SHA-crypt hash as given. It hashes only the plaintext sibling, so a hash on
// the canonical leaf is never hashed again.
func TestApplyPasswordHashingKeepsSHACrypt(t *testing.T) {
	schema := bcryptHashSchema()
	tree := shaCryptUserTree("migrated", sha512CryptHash)

	hashed, err := ApplyPasswordHashing(tree, schema)
	require.NoError(t, err)
	assert.Empty(t, hashed, "nothing was hashed")

	entry := tree.GetContainer("system").GetContainer("authentication").GetList("user")["migrated"]
	stored, ok := entry.Get("password")
	require.True(t, ok)
	assert.Equal(t, sha512CryptHash, stored, "the SHA-crypt hash is stored byte-identical")
}

// TestApplyPasswordHashingPlaintextStillBcrypt: a plaintext written next to a
// SHA-crypt hash replaces it with bcrypt. Ze writes bcrypt only, so setting a
// new password moves the account off SHA-crypt.
func TestApplyPasswordHashingPlaintextStillBcrypt(t *testing.T) {
	schema := bcryptHashSchema()
	tree := shaCryptUserTree("migrated", sha512CryptHash)
	entry := tree.GetContainer("system").GetContainer("authentication").GetList("user")["migrated"]
	entry.Set("plaintext-password", "a new passphrase")

	_, err := ApplyPasswordHashing(tree, schema)
	require.NoError(t, err)

	stored, ok := entry.Get("password")
	require.True(t, ok)
	require.True(t, IsBcryptHash(stored), "the new password is stored as bcrypt")
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored), []byte("a new passphrase")))
}

// TestLoadConfigKeepsSHACryptPassword: a config file whose user carries a
// SHA-crypt hash loads with the hash byte-identical and with no warning.
//
// VALIDATES: the boot and SIGHUP load path, over the real YANG schema, accepts
// `password "$6$..."` on system authentication user.
func TestLoadConfigKeepsSHACryptPassword(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "migrated.conf")
	input := `system {
	authentication {
		user migrated {
			password "` + sha512CryptHash + `";
		}
	}
}
`

	result, err := LoadConfig(input, configPath, nil)
	require.NoError(t, err)

	migrated := loadConfigUser(t, result.Tree, "migrated")
	stored, ok := migrated.Get("password")
	require.True(t, ok)
	assert.Equal(t, sha512CryptHash, stored, "the loaded hash is the one in the file")
	assert.Empty(t, warningsNaming(t, configPath), "a stored hash is not warned about")
}
