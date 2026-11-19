// Design: docs/architecture/fleet-config.md -- managed client runtime commit wiring

package hub

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/managed"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/zefs"
)

func TestWireManagedCommitStagesCandidateAndPromotes(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	configPath := filepath.Join(dir, "ze.conf")
	activeStamp := managedActiveStamp
	reloadCalled := false

	require.NoError(t, store.WriteFile(configPath, []byte("old"), 0o600))
	require.NoError(t, store.WriteVersion(configPath, []byte("old"), managedActiveTime(t)))
	setConfigPointer(t, store, configPath, zefs.KeyConfigActive, activeStamp)

	client := &managed.ClientConfig{}
	wireManagedCommit(client, store, configPath, func() error {
		reloadCalled = true
		data, _, ok, err := storage.ReadCandidateConfig(store, configPath)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, "new", string(data))
		return storage.PromoteCandidate(store, configPath)
	}, nil, nil)

	require.NotNil(t, client.OnCommit)
	require.NoError(t, client.OnCommit([]byte("new")))
	assert.True(t, reloadCalled)

	rollback, ok := configPointer(t, store, configPath, zefs.KeyConfigRollback)
	require.True(t, ok)
	assert.Equal(t, activeStamp, rollback)

	_, ok = configPointer(t, store, configPath, zefs.KeyConfigCandidate)
	assert.False(t, ok)

	activeData, err := storage.ReadActiveConfig(store, configPath)
	require.NoError(t, err)
	assert.Equal(t, "new", string(activeData))
}

func TestWireManagedCommitClearsCandidateOnReloadFailure(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	configPath := filepath.Join(dir, "ze.conf")
	activeStamp := managedActiveStamp

	require.NoError(t, store.WriteFile(configPath, []byte("old"), 0o600))
	require.NoError(t, store.WriteVersion(configPath, []byte("old"), managedActiveTime(t)))
	setConfigPointer(t, store, configPath, zefs.KeyConfigActive, activeStamp)

	client := &managed.ClientConfig{}
	wireManagedCommit(client, store, configPath, func() error {
		return errors.New("verify rejected")
	}, nil, nil)

	err := client.OnCommit([]byte("new"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verify rejected")

	active, ok := configPointer(t, store, configPath, zefs.KeyConfigActive)
	require.True(t, ok)
	assert.Equal(t, activeStamp, active)

	_, ok = configPointer(t, store, configPath, zefs.KeyConfigCandidate)
	assert.False(t, ok)
}

// VALIDATES: a managed push that lands while the daemon's confirmed-commit
// window is open is refused with the window's refusal naming its owner, so the
// managed server reads an error, and nothing is staged or reloaded: the
// window's revert would wipe the push.
// PREVENTS: a hub push applied inside a window and silently undone at its
// deadline.
func TestWireManagedCommitRefusedDuringConfirmWindow(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	configPath := filepath.Join(dir, "ze.conf")
	activeStamp := managedActiveStamp
	require.NoError(t, store.WriteFile(configPath, []byte("old"), 0o600))
	require.NoError(t, store.WriteVersion(configPath, []byte("old"), managedActiveTime(t)))
	setConfigPointer(t, store, configPath, zefs.KeyConfigActive, activeStamp)

	window := confirm.NewWindow(func([]byte) error { return nil }, confirmNopRecorder{})
	t.Cleanup(window.Stop)
	opened := confirm.Commit{
		Snapshot: func() ([]byte, error) { return []byte("old"), nil },
		Apply:    func() error { return nil },
	}
	require.NoError(t, window.Confirmed("alice", time.Minute, false, opened))

	reloaded := false
	client := &managed.ClientConfig{}
	wireManagedCommit(client, store, configPath, func() error {
		reloaded = true
		return storage.PromoteCandidate(store, configPath)
	}, nil, func() *confirm.Window { return window })

	err := client.OnCommit([]byte("new"))
	var other *confirm.OtherUserError
	require.ErrorAs(t, err, &other, "a push during alice's window is refused with the window's refusal")
	assert.Equal(t, "alice", other.Owner)
	assert.False(t, reloaded, "the refused push reloaded the daemon")
	_, ok := configPointer(t, store, configPath, zefs.KeyConfigCandidate)
	assert.False(t, ok, "the refused push staged a candidate")
	activeData, err := storage.ReadActiveConfig(store, configPath)
	require.NoError(t, err)
	assert.Equal(t, "old", string(activeData))
}

// managedActiveStamp is the stamp of the active version every push test seeds.
const managedActiveStamp = "20260524-090000.000"

// managedActiveTime is the time of the active version the push tests seed.
func managedActiveTime(t *testing.T) time.Time {
	t.Helper()
	return mustParseReloadStamp(t, managedActiveStamp)
}

// managedPushStore is a device store whose configuration the boot record has
// versioned, as a managed client holds it before a hub push. It answers the
// store and the configuration path the push is given.
func managedPushStore(t *testing.T) (storage.Storage, string) {
	t.Helper()
	dir := t.TempDir()
	store := newTestStore(t, dir)
	configPath := filepath.Join(dir, "ze.conf")
	activeStamp := managedActiveStamp
	require.NoError(t, store.WriteFile(configPath, []byte("old"), 0o600))
	require.NoError(t, store.WriteVersion(configPath, []byte("old"), managedActiveTime(t)))
	setConfigPointer(t, store, configPath, zefs.KeyConfigActive, activeStamp)
	return store, configPath
}

// TestBaselineWrittenAfterPush proves that a hub push records the device's
// baseline: it runs the wired OnCommit with a reload that promotes the staged
// candidate, as the daemon's reload does, on a store that held no baseline.
//
// VALIDATES: AC-1. meta/managed/base-version holds the hash of the
// configuration the store answers as active after the push, which is the one
// the managed client then compares (managed.ActiveVersion).
// PREVENTS: a device with no record of what its hub gave it, which is what
// lets the next fetch overwrite a local change.
//
// MUTATION: drop recordPushBaseline from applyManagedPush and this fails: the
// baseline is not recorded.
func TestBaselineWrittenAfterPush(t *testing.T) {
	store, configPath := managedPushStore(t)
	client := &managed.ClientConfig{}
	wireManagedCommit(client, store, configPath, func() error {
		return storage.PromoteCandidate(store, configPath)
	}, nil, nil)

	require.NoError(t, client.OnCommit([]byte("new")))

	baseline, err := managed.BaselineVersion(store)
	require.NoError(t, err)
	require.True(t, baseline.Recorded(), "a successful push recorded no baseline")
	active, err := managed.ActiveVersion(store)
	require.NoError(t, err)
	assert.Equal(t, active, baseline, "the baseline is not the hash of the active configuration")
	versioned, err := storage.ReadActiveConfig(store, configPath)
	require.NoError(t, err)
	assert.Equal(t, fleet.VersionHash(versioned), baseline.Hash(), "the baseline is not the hash of the store's active version")
}

// TestBaselineUnchangedWhenPushFails proves that a push that made nothing
// active leaves the baseline alone: it runs the wired OnCommit with a reload
// that fails, on a store whose baseline is the configuration it runs.
//
// VALIDATES: AC-1, permitted survivor. The baseline is still the hash of the
// configuration the device runs, not of the bytes the hub sent.
// PREVENTS: a refused push moving the baseline to a configuration the device
// never ran, so the device holds against its own hub from then on.
func TestBaselineUnchangedWhenPushFails(t *testing.T) {
	store, configPath := managedPushStore(t)
	before, err := managed.RecordActiveAsBaseline(store)
	require.NoError(t, err)
	client := &managed.ClientConfig{}
	wireManagedCommit(client, store, configPath, func() error {
		return errors.New("verify rejected")
	}, nil, nil)

	require.Error(t, client.OnCommit([]byte("new")))

	baseline, err := managed.BaselineVersion(store)
	require.NoError(t, err)
	assert.Equal(t, before, baseline, "a failed push moved the baseline")
	assert.NotEqual(t, fleet.VersionHash([]byte("new")), baseline.Hash())
}

// managedLocalStamp is the stamp of the local commit the revert tests land
// after the client read the hash the revert discards.
const managedLocalStamp = "20260524-100000.000"

// TestWireManagedRevertAppliesOverChecked proves a revert whose check accepts
// the configuration active under the lock applies as a push does: it runs the
// wired OnRevert with a reload that promotes the staged candidate.
//
// VALIDATES: spec-fleet-7 AC-13 at the device's stage: check reads the active
// configuration ("old"), the hub's configuration becomes active, and the
// baseline is recorded.
// PREVENTS: a revert the stage never applies, or a check handed other bytes
// than the active configuration.
//
// MUTATION: stage through StageMachineCandidate in OnRevert, ignoring check,
// and the check is never called.
func TestWireManagedRevertAppliesOverChecked(t *testing.T) {
	store, configPath := managedPushStore(t)
	client := &managed.ClientConfig{}
	wireManagedCommit(client, store, configPath, func() error {
		return storage.PromoteCandidate(store, configPath)
	}, nil, nil)
	var checked []string

	require.NotNil(t, client.OnRevert)
	require.NoError(t, client.OnRevert([]byte("new"), func(active []byte) error {
		checked = append(checked, string(active))
		return nil
	}))

	assert.Equal(t, []string{"old"}, checked, "the stage did not hand check the active configuration once")
	activeData, err := storage.ReadActiveConfig(store, configPath)
	require.NoError(t, err)
	assert.Equal(t, "new", string(activeData))
	baseline, err := managed.BaselineVersion(store)
	require.NoError(t, err)
	assert.Equal(t, fleet.VersionHash([]byte("new")), baseline.Hash())
}

// TestWireManagedRevertRefusedOverLocalCommit proves a revert decided on a
// configuration a local commit replaced since never overwrites that commit:
// the local commit lands after the client read the hash, and the check refuses
// the configuration the stage reads under the lock.
//
// VALIDATES: spec-fleet-7 AC-15 (a newer local edit always wins), review note
// N1: the stage answers the check's refusal as is, stages and reloads
// nothing, and leaves the local edit active and the baseline alone.
// PREVENTS: a revert that discards a change the hub's operator never saw.
//
// MUTATION: stage through StageMachineCandidate in OnRevert, ignoring check,
// and the local edit is overwritten.
func TestWireManagedRevertRefusedOverLocalCommit(t *testing.T) {
	store, configPath := managedPushStore(t)
	before, err := managed.RecordActiveAsBaseline(store)
	require.NoError(t, err)
	require.NoError(t, store.WriteVersion(configPath, []byte("local"), mustParseReloadStamp(t, managedLocalStamp)))
	setConfigPointer(t, store, configPath, zefs.KeyConfigActive, managedLocalStamp)
	reloaded := false
	client := &managed.ClientConfig{}
	wireManagedCommit(client, store, configPath, func() error {
		reloaded = true
		return storage.PromoteCandidate(store, configPath)
	}, nil, nil)
	changed := errors.New("the active configuration is not the discarded one")

	err = client.OnRevert([]byte("new"), func(active []byte) error {
		if string(active) != "old" {
			return changed
		}
		return nil
	})

	require.ErrorIs(t, err, changed)
	assert.False(t, reloaded, "the refused revert reloaded the daemon")
	_, ok := configPointer(t, store, configPath, zefs.KeyConfigCandidate)
	assert.False(t, ok, "the refused revert staged a candidate")
	activeData, err := storage.ReadActiveConfig(store, configPath)
	require.NoError(t, err)
	assert.Equal(t, "local", string(activeData), "the revert overwrote the local commit")
	baseline, err := managed.BaselineVersion(store)
	require.NoError(t, err)
	assert.Equal(t, before, baseline, "a refused revert moved the baseline")
}
