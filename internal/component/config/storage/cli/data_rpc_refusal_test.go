// Design: docs/architecture/fleet-config.md -- the freeze's refusals keep their own fix through the daemon's import and rollback.
// Related: data_rpc_config.go -- daemonConfigImportRefusal.
// Related: data_rpc_rollback.go -- configRollbackRefusal.

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

// frozenStagingRefusal is the refusal a real staging answers on a managed
// device: a store whose flag says managed, bound to its own stored
// configuration, refuses a candidate for that configuration. It is wrapped as
// a staging route wraps it.
func frozenStagingRefusal(t *testing.T) error {
	t.Helper()
	const own = "router.conf"
	owner, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = owner.Close() })
	require.NoError(t, owner.WriteFile(own, []byte("router-id 192.0.2.1\n"), 0o600))
	_, _, err = storage.EnsureActiveVersion(owner, own, []byte("router-id 192.0.2.1\n"), time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.NoError(t, owner.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte("true")))
	bound := storage.BindConfigSource(owner, own, storage.ConfigSourceStored)

	_, err = storage.WriteCandidateVersion(bound, own, []byte("router-id 192.0.2.2\n"), time.Now())
	require.ErrorIs(t, err, storage.ErrConfigFrozen)
	return fmt.Errorf("stage candidate: %w", err)
}

// reloadRefusal is an operator error from a layer other than the freeze, deep
// in the chain, as the import's reload answers one.
func reloadRefusal() error {
	return fmt.Errorf("the reload refused the config: %w", opererr.New(
		"the configuration was not applied",
		"a plugin refused it at verify",
		"ze config validate ze.conf"))
}

// lastLine answers the last line of an operator error's text.
func lastLine(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	return lines[len(lines)-1]
}

// firstLine answers the first line of an operator error's text.
func firstLine(text string) string {
	return strings.SplitN(text, "\n", 2)[0]
}

// TestDaemonImportKeepsTheStoresFix proves that `update data config` answers
// the freeze's refusal as the freeze wrote it.
//
// VALIDATES: the import's refusal ends with `fix: request fleet disable`.
// PREVENTS: the freeze's refusal shown as a cause under `ze config validate`,
// which cannot lift it.
func TestDaemonImportKeepsTheStoresFix(t *testing.T) {
	refusal := daemonConfigImportRefusal("ze.conf", frozenStagingRefusal(t))

	require.NotNil(t, refusal)
	assert.Equal(t, "fix: request fleet disable", lastLine(refusal.Error()))
}

// TestDaemonImportWrapsOtherOperatorErrors proves that only the freeze's
// refusal is passed through: an operator error from the reload keeps the
// import's own first line and fix.
//
// VALIDATES: the answer starts "update data config: ze.conf was not imported"
// and ends on the import's fix.
// PREVENTS: a reload refusal answered without saying the import failed.
func TestDaemonImportWrapsOtherOperatorErrors(t *testing.T) {
	refusal := daemonConfigImportRefusal("ze.conf", reloadRefusal())

	require.NotNil(t, refusal)
	assert.Equal(t, pathUpdateDataConfig+": ze.conf was not imported", firstLine(refusal.Error()))
	assert.Equal(t, "fix: ze config validate <file>", lastLine(refusal.Error()))
}

// TestDaemonRollbackKeepsTheStoresFix proves the same for the rollback RPC.
//
// VALIDATES: the rollback's error answer ends with `fix: request fleet disable`.
// PREVENTS: the freeze's refusal shown under the rollback's usage line.
func TestDaemonRollbackKeepsTheStoresFix(t *testing.T) {
	answer := configRollbackRefusal("the stored rollback could not finish", frozenStagingRefusal(t))

	require.NotNil(t, answer)
	assert.Equal(t, "fix: request fleet disable", lastLine(answer.Error))
}

// TestDaemonRollbackWrapsOtherOperatorErrors proves that the rollback RPC
// keeps its own first line for an operator error the freeze did not produce.
//
// VALIDATES: the answer starts with the rollback's own problem.
// PREVENTS: a non-freeze refusal answered without saying the rollback failed.
func TestDaemonRollbackWrapsOtherOperatorErrors(t *testing.T) {
	answer := configRollbackRefusal("the stored rollback could not finish", reloadRefusal())

	require.NotNil(t, answer)
	assert.True(t, strings.HasPrefix(firstLine(answer.Error), "request data config rollback: "), answer.Error)
}
