// Design: docs/architecture/fleet-config.md -- the freeze's refusals keep their own fix through `ze config rollback`.
// Related: cmd_rollback.go -- storedRollbackRefusal.

package cli

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestStoredRollbackKeepsTheStoresFix proves that `ze config rollback` on a
// stopped store prints the freeze's refusal as the freeze wrote it: a real
// staging on a managed store's own configuration ends on the freeze's fix.
//
// VALIDATES: the refusal ends with `fix: request fleet disable`.
// PREVENTS: the store's refusal shown as a cause under the history line.
func TestStoredRollbackKeepsTheStoresFix(t *testing.T) {
	const own = "router.conf"
	owner, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = owner.Close() })
	require.NoError(t, owner.WriteFile(own, []byte("router-id 192.0.2.1\n"), 0o600))
	_, _, err = storage.EnsureActiveVersion(owner, own, []byte("router-id 192.0.2.1\n"), time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.NoError(t, owner.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte("true")))
	_, err = storage.WriteCandidateVersion(storage.BindConfigSource(owner, own, storage.ConfigSourceStored),
		own, []byte("router-id 192.0.2.2\n"), time.Now())
	require.ErrorIs(t, err, storage.ErrConfigFrozen)

	refusal := storedRollbackRefusal("ze.conf", fmt.Errorf("stage rollback: %w", err))

	lines := strings.Split(refusal.Error(), "\n")
	assert.Equal(t, "fix: request fleet disable", lines[len(lines)-1])
}

// TestStoredRollbackWrapsOtherOperatorErrors proves that only the freeze's
// refusal is passed through: another operator error in the chain keeps the
// command's own first line.
//
// VALIDATES: the answer starts "ze config rollback: the stored rollback could not finish".
// PREVENTS: a non-freeze refusal answered without saying the rollback failed.
func TestStoredRollbackWrapsOtherOperatorErrors(t *testing.T) {
	other := fmt.Errorf("publish: %w", opererr.New("the configuration was not applied", "a plugin refused it", "ze config validate ze.conf"))

	refusal := storedRollbackRefusal("ze.conf", other)

	assert.True(t, strings.HasPrefix(refusal.Error(), "ze config rollback: the stored rollback could not finish"), refusal.Error())
}
