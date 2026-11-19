// Design: docs/architecture/fleet-config.md -- the freeze answers an SSH config upload on a managed device.
// Related: config_upload.go -- configMutationOwner.Upload.

package hub

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestUploadOnManagedDeviceAnswersTheFreeze proves that an SSH configuration
// upload to a device that follows a hub is refused with the freeze's own
// operator error, its fix last.
//
// Method: the real mutation owner over a store bound to its own stored
// configuration, as a daemon started from its store binds it, with
// meta/instance/managed true. A valid replacement is uploaded.
//
// VALIDATES: spec-fleet-6 AC-16 for the upload route. The answer ends with
// `fix: request fleet disable`, nothing reloads, and the source is unchanged.
// PREVENTS: the upload answering "not accepted ... show config running",
// which never says the device is frozen nor names the command that lifts it.
func TestUploadOnManagedDeviceAnswersTheFreeze(t *testing.T) {
	const path = "router.conf"
	store := storage.BindConfigSource(storage.CreateMemory(), path, storage.ConfigSourceStored)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, initializeConfigSource(store, path, []byte(uploadWindowBefore)))
	require.NoError(t, store.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte("true\n")))
	owner := &configMutationOwner{}
	t.Cleanup(owner.close)
	reloads := 0
	require.NoError(t, owner.bind(store, path, nil, func(context.Context) error {
		reloads++
		return nil
	}))

	err := owner.Upload(context.Background(), []byte(uploadWindowConfirmed))

	require.ErrorIs(t, err, storage.ErrConfigFrozen)
	lines := strings.Split(strings.TrimRight(err.Error(), "\n"), "\n")
	assert.Equal(t, "fix: request fleet disable", lines[len(lines)-1])
	assert.Zero(t, reloads, "a refused upload reloaded the daemon")
	source, readErr := storage.ReadConfigSource(store, path)
	require.NoError(t, readErr)
	assert.Equal(t, uploadWindowBefore, string(source), "a refused upload changed the source")
}
