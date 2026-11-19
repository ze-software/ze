// Design: docs/architecture/ssh/fixit-bcrypt-hash-credential.md -- credential-token redaction for logs
// Related: redact.go -- IsPasswordHash, Command and JSON, the producers these tests drive

package redact

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// shaCryptHash is a published test vector of the SHA-crypt specification.
const shaCryptHash = "$6$saltstring$svn8UoSVapNtMuq1ukKS4tPQd8iKwSMHWjl/O817G3uBnIFNjnQJuesI68u4OTLiBFdcbYEdFCoEOfaS35inz1"

// TestIsPasswordHash: both stored formats Ze accepts are password hashes, a
// near-valid SHA-crypt value is treated as one so redaction fails closed, and a
// value of neither shape is not.
func TestIsPasswordHash(t *testing.T) {
	assert.True(t, IsPasswordHash("$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234"), "bcrypt")
	assert.True(t, IsPasswordHash(shaCryptHash), "SHA-512 crypt")
	assert.True(t, IsPasswordHash("$5$saltstring$5B8vYYiY.CVt1RlTTf8KbXBH3hsxY/GNooZaBBGWEc5"), "SHA-256 crypt")
	assert.False(t, IsPasswordHash("plaintext"))
	assert.True(t, IsPasswordHash("$6$saltstring$tooshort"), "a near-valid SHA-crypt value is still hidden")
	assert.True(t, IsPasswordHash("$6$rounds=10$salt$abc"), "a rounds value crypt(3) never prints is still hidden")
	assert.False(t, IsPasswordHash("$6$"), "a bare prefix carries no hash")
	assert.False(t, IsPasswordHash("$1$salt$abcdefghijklmnopqrstuv"), "md5-crypt is not accepted")
}

// TestCommandRedactsSHACryptToken: a SHA-crypt hash standing alone in a command,
// with no credential keyword before it, is still redacted.
//
// VALIDATES: a migrated account's hash reaches no log through a command line.
func TestCommandRedactsSHACryptToken(t *testing.T) {
	out := Command("show something " + shaCryptHash)
	assert.NotContains(t, out, shaCryptHash)
	assert.Contains(t, out, Placeholder)
}

// TestJSONRedactsSHACryptAnywhere: a SHA-crypt hash under a key that does not
// name a secret is redacted from a captured payload.
func TestJSONRedactsSHACryptAnywhere(t *testing.T) {
	out, err := JSON([]byte(`{"note":"` + shaCryptHash + `"}`))
	assert.NoError(t, err)
	assert.NotContains(t, string(out), shaCryptHash)
	assert.Contains(t, string(out), Placeholder)
}
