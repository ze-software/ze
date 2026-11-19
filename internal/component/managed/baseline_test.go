package managed

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestActiveHashOneHelper proves that every reader names the same
// configuration as "active" after a hub push: it stages and promotes a
// configuration the way a push does (storage.StageMachineCandidate, the entry
// the freeze of a managed device does not refuse, then
// storage.PromoteCandidate), over a store whose earlier configuration the
// boot record versioned, and reads the hash through each reader.
//
// VALIDATES: A-2. The managed client (BaselineAccess), the doctor check
// (checkManagedDiverged), `show fleet status` (readFleetStatus) and the
// baseline writer (RecordActiveAsBaseline) all answer the hash of the bytes
// the store's active-version read answers (storage.ReadActiveConfig).
// PREVENTS: a client that holds on one hash while the doctor and the status
// report another, and a baseline recorded from bytes the device does not run.
func TestActiveHashOneHelper(t *testing.T) {
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() }) //nolint:errcheck // test cleanup
	require.NoError(t, store.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte(managedFlagTrue)))

	// The earlier configuration, cached and then versioned by the boot record.
	earlier := []byte(holdEarlierConfig)
	require.NoError(t, store.WriteFile(fleetStatusConfigName, earlier, 0))
	_, _, err = storage.EnsureActiveVersion(store, fleetStatusConfigName, earlier, time.Now().Add(-time.Minute))
	require.NoError(t, err)
	_, err = RecordActiveAsBaseline(store)
	require.NoError(t, err)

	// The push: staged, then promoted by the reload.
	pushed := []byte(holdHubConfig)
	_, err = storage.StageMachineCandidate(store, fleetStatusConfigName, pushed, time.Now())
	require.NoError(t, err)
	require.NoError(t, storage.PromoteCandidate(store, fleetStatusConfigName))

	versioned, err := storage.ReadActiveConfig(store, fleetStatusConfigName)
	require.NoError(t, err)
	want := fleet.VersionHash(versioned)
	require.Equal(t, fleet.VersionHash(pushed), want, "the store's active version is not the pushed configuration")

	fromClient, _, err := NewBaselineAccess(store).read()
	require.NoError(t, err)
	assert.Equal(t, want, fromClient.Hash(), "the managed client")

	status, err := readFleetStatus(store, store)
	require.NoError(t, err)
	assert.Equal(t, want, status.ActiveHash, "show fleet status")

	// The baseline is still the earlier configuration's, so the doctor check
	// reports, and its text names the active hash it compared.
	diags := checkManagedDiverged(diagnostic.DoctorCheckContext{Store: store})
	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "runs (version "+want+")", "the doctor check compared another hash")

	recorded, err := RecordActiveAsBaseline(store)
	require.NoError(t, err)
	assert.Equal(t, want, recorded.Hash(), "the baseline writer")
}

// TestActiveHashReadsThePointedVersion proves that every reader of "the
// configuration this device runs" reads the version the active pointer
// names, not the plain mirror of the configuration: the mirror is rewritten
// raw after the version was pointed, so the two differ.
//
// VALIDATES: the client's baseline access, ActiveVersion, `show fleet status`
// and the doctor check all answer the pointed version's hash.
// PREVENTS: a helper that reads the mirror again (store.ReadFile), which
// answers another hash after a raw write of the mirror key.
func TestActiveHashReadsThePointedVersion(t *testing.T) {
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() }) //nolint:errcheck // test cleanup
	require.NoError(t, store.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte(managedFlagTrue)))

	pointed := []byte(holdHubConfig)
	require.NoError(t, store.WriteFile(fleetStatusConfigName, pointed, 0))
	_, _, err = storage.EnsureActiveVersion(store, fleetStatusConfigName, pointed, time.Now().Add(-time.Minute))
	require.NoError(t, err)
	require.NoError(t, store.WriteFile(fleetStatusConfigName, []byte(holdDeviceConfig), 0))
	mirror, err := store.ReadFile(fleetStatusConfigName)
	require.NoError(t, err)
	require.Equal(t, holdDeviceConfig, string(mirror), "the raw write did not change the mirror")
	active, err := storage.ReadActiveConfig(store, fleetStatusConfigName)
	require.NoError(t, err)
	require.Equal(t, string(pointed), string(active), "the raw write moved the active pointer")
	want := fleet.VersionHash(pointed)
	require.NoError(t, StoreBaseline(store, versionOf([]byte(holdEarlierConfig))))

	version, err := ActiveVersion(store)
	require.NoError(t, err)
	assert.Equal(t, want, version.Hash(), "ActiveVersion")

	fromClient, _, err := NewBaselineAccess(store).read()
	require.NoError(t, err)
	assert.Equal(t, want, fromClient.Hash(), "the managed client")

	status, err := readFleetStatus(store, store)
	require.NoError(t, err)
	assert.Equal(t, want, status.ActiveHash, "show fleet status")

	diags := checkManagedDiverged(diagnostic.DoctorCheckContext{Store: store})
	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "runs (version "+want+")", "the doctor check")
}
