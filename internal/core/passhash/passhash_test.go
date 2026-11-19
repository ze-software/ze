// Design: docs/guide/authentication.md -- a migrated account keeps its SHA-crypt hash
// Related: passhash.go -- Check and Verify, the producers these tests drive

package passhash

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// Published test vectors of the SHA-crypt specification ("Unix crypt using
// SHA-256 and SHA-512"), both made from vectorPassword.
const (
	vector512      = "$6$saltstring$svn8UoSVapNtMuq1ukKS4tPQd8iKwSMHWjl/O817G3uBnIFNjnQJuesI68u4OTLiBFdcbYEdFCoEOfaS35inz1"
	vector256      = "$5$rounds=10000$saltstringsaltst$3xv.VbSHBb41AL9AvLeujZkZRBAwqFMz2.opqey6IcA"
	vectorPassword = "Hello world!"
)

// TestCheckAcceptsEveryStoredFormat: bcrypt, $6$ and $5$ are each a stored
// hash Verify can answer for.
func TestCheckAcceptsEveryStoredFormat(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	require.NoError(t, err)
	assert.NoError(t, Check(string(hash)), "bcrypt")
	assert.NoError(t, Check(vector512), "SHA-512 crypt")
	assert.NoError(t, Check(vector256), "SHA-256 crypt")
}

// TestCheckRefusesWhatVerifyCannotAnswer: a plaintext, a cut-short bcrypt hash
// and a malformed SHA-crypt hash are each refused, so a leaf holding one is
// reported instead of silently refusing every login.
func TestCheckRefusesWhatVerifyCannotAnswer(t *testing.T) {
	require.ErrorIs(t, Check("cleartext"), ErrNotPasswordHash)
	require.ErrorIs(t, Check("$2a$10$tooShort"), ErrNotPasswordHash)
	require.ErrorIs(t, Check("$1$salt$abcdefghijklmnopqrstuv"), ErrNotPasswordHash, "md5-crypt is not accepted")
	malformed := vector512[:len(vector512)-1]
	err := Check(malformed)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotPasswordHash, "a malformed SHA-crypt hash says what is wrong with it")
	assert.NotContains(t, err.Error(), malformed[20:], "the error carries no part of the digest")
}

// TestVerifyEveryFormat: each format accepts its own password and refuses a
// password one character different.
func TestVerifyEveryFormat(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte(vectorPassword), bcrypt.MinCost)
	require.NoError(t, err)
	for _, stored := range []string{string(hash), vector512, vector256} {
		assert.True(t, Verify(stored, vectorPassword), "right password, %q", stored[:3])
		assert.False(t, Verify(stored, "Hello world?"), "wrong password, %q", stored[:3])
	}
}

// TestVerifyRefusesMalformed: a value Check refuses verifies no password.
func TestVerifyRefusesMalformed(t *testing.T) {
	assert.False(t, Verify(vector512[:len(vector512)-1], vectorPassword))
	assert.False(t, Verify("cleartext", "cleartext"), "a plaintext on the leaf is not a hash of itself")
	assert.False(t, Verify("", ""))
}
