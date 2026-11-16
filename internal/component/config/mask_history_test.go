// Design: docs/architecture/ssh/fixit-bcrypt-hash-credential.md -- display placeholders are not secret values.
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/redact"
)

// TestRejectHistoryPlaceholderOnlyOnSecretLeaves keeps ordinary literal text
// legal while covering both sensitivity markings and both display producers.
func TestRejectHistoryPlaceholderOnlyOnSecretLeaves(t *testing.T) {
	schema := maskTestSchema()
	for _, marker := range []string{SecretDataPlaceholder, redact.Placeholder} {
		for _, leaf := range []string{"password", "api-secret"} {
			t.Run(leaf+marker, func(t *testing.T) {
				tree := maskTreeWithUser(t, validBcryptHash)
				aliceOf(tree).Set(leaf, marker)
				err := RejectMaskedSecretLeaves(tree, schema)
				require.Error(t, err)
				assert.Contains(t, err.Error(), leaf)
				assert.Contains(t, err.Error(), "real value")
			})
		}
		tree := maskTreeWithUser(t, validBcryptHash)
		aliceOf(tree).Set("shell", marker)
		assert.NoError(t, RejectMaskedSecretLeaves(tree, schema), "a nonsecret literal marker remains legal")
	}
}
