// Design: docs/guide/authentication.md -- a migrated account keeps its SHA-crypt hash
// Related: auth.go -- CheckPassword and LocalAuthenticator.Authenticate, the producers these tests drive

package authz

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Published test vectors of the SHA-crypt specification ("Unix crypt using
// SHA-256 and SHA-512"), so these hashes are ones another system wrote, not
// ones Ze produced for itself.
const (
	sha512CryptHash     = "$6$saltstring$svn8UoSVapNtMuq1ukKS4tPQd8iKwSMHWjl/O817G3uBnIFNjnQJuesI68u4OTLiBFdcbYEdFCoEOfaS35inz1"
	sha256CryptHash     = "$5$rounds=10000$saltstringsaltst$3xv.VbSHBb41AL9AvLeujZkZRBAwqFMz2.opqey6IcA"
	shaCryptPassword    = "Hello world!"
	shaCryptWrongSecret = "Hello world?"
)

// TestCheckPasswordAcceptsSHACrypt: a stored $6$ or $5$ hash verifies the
// password it was made from, over a remote transport too.
//
// VALIDATES: an account migrated with its SHA-crypt hash logs in with the
// password its user already has, on every surface that calls CheckPassword.
// PREVENTS: the bcrypt-only comparison refusing every migrated account.
func TestCheckPasswordAcceptsSHACrypt(t *testing.T) {
	for _, stored := range []string{sha512CryptHash, sha256CryptHash} {
		assert.True(t, CheckPassword(stored, shaCryptPassword, false), "remote login with the right password, %q", stored[:3])
		assert.True(t, CheckPassword(stored, shaCryptPassword, true), "local login with the right password, %q", stored[:3])
	}
}

// TestCheckPasswordRefusesSHACryptWrongPassword: the same hashes refuse a
// password one character different, and an empty one.
func TestCheckPasswordRefusesSHACryptWrongPassword(t *testing.T) {
	for _, stored := range []string{sha512CryptHash, sha256CryptHash} {
		assert.False(t, CheckPassword(stored, shaCryptWrongSecret, false), "wrong password, %q", stored[:3])
		assert.False(t, CheckPassword(stored, "", false), "empty password, %q", stored[:3])
	}
}

// TestCheckPasswordRefusesMalformedSHACrypt: a value that names SHA-crypt but
// is malformed (here a digest cut short) authenticates nothing.
func TestCheckPasswordRefusesMalformedSHACrypt(t *testing.T) {
	malformed := sha512CryptHash[:len(sha512CryptHash)-1]
	assert.False(t, CheckPassword(malformed, shaCryptPassword, false))
}

// TestCheckPasswordSHACryptHashIsNoRemoteToken: the stored SHA-crypt hash,
// presented as the password, is a credential only over a local transport, the
// same rule the bcrypt hash follows.
func TestCheckPasswordSHACryptHashIsNoRemoteToken(t *testing.T) {
	assert.False(t, CheckPassword(sha512CryptHash, sha512CryptHash, false), "the hash is no remote credential")
	assert.True(t, CheckPassword(sha512CryptHash, sha512CryptHash, true), "the hash is the local CLI's token")
}

// TestLocalAuthenticatorSHACryptUser: the local backend logs a SHA-crypt
// account in with its password and refuses a wrong one.
//
// VALIDATES: the backend every login surface reaches through the AAA chain
// answers for a migrated account, with the account's profiles.
func TestLocalAuthenticatorSHACryptUser(t *testing.T) {
	local := &LocalAuthenticator{Users: []UserConfig{
		{Name: "migrated", Hash: sha512CryptHash, Profiles: []string{"admin"}},
	}}

	result, err := local.Authenticate(AuthRequest{Username: "migrated", Password: shaCryptPassword})
	require.NoError(t, err)
	assert.True(t, result.Authenticated)
	assert.Equal(t, []string{"admin"}, result.Profiles)

	result, err = local.Authenticate(AuthRequest{Username: "migrated", Password: shaCryptWrongSecret})
	require.ErrorIs(t, err, ErrAuthRejected)
	assert.False(t, result.Authenticated)
}
