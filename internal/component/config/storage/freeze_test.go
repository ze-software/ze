// Design: docs/architecture/fleet-config.md -- the freeze of a managed device's own configuration.

package storage

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/zefs"
)

// freezeOwnConfig is the device's own configuration in these tests, and
// freezeOtherConfig a second configuration of the same store, as a hub holds
// for a client.
const (
	freezeOwnConfig   = "router.conf"
	freezeOtherConfig = "client-edge.conf"
	freezeActive      = "router-id 192.0.2.1\n"
	freezeChange      = "router-id 192.0.2.2\n"
)

// freezeLiveStores are the encodings a live store has. A blob is an explicit
// artifact, which the freeze never covers (TestFreezeScopeStoppedTool).
func freezeLiveStores() []pointerTestStore {
	var live []pointerTestStore
	for _, backend := range pointerTestStores() {
		if backend.name == "blob" {
			continue
		}
		live = append(live, backend)
	}
	return live
}

// freezeSeed gives store an active, versioned configuration under name and
// writes flag under meta/instance/managed. An empty flag writes no key.
func freezeSeed(t *testing.T, store Storage, name, flag string) {
	t.Helper()
	require.NoError(t, store.WriteFile(name, []byte(freezeActive), 0o600))
	_, _, err := EnsureActiveVersion(store, name, []byte(freezeActive), time.Now().Add(-time.Hour))
	require.NoError(t, err)
	if flag == "" {
		return
	}
	require.NoError(t, store.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte(flag)))
}

// freezeState is what a refused write must leave as it found it: every
// version key of the store, the candidate, and the active bytes.
type freezeState struct {
	versionKeys []string
	candidate   bool
	active      string
}

func freezeStateOf(t *testing.T, store Storage, name string) freezeState {
	t.Helper()
	keys, err := store.ListKeys(zefs.KeyFileVersion.Prefix())
	require.NoError(t, err)
	_, _, candidate, err := ReadCandidateConfig(store, name)
	require.NoError(t, err)
	active, err := ReadActiveConfig(store, name)
	require.NoError(t, err)
	return freezeState{versionKeys: keys, candidate: candidate, active: string(active)}
}

// requireFrozen holds the refusal: the freeze's own error, an operator error
// whose fix is the command that lifts it, alone on the last line.
func requireFrozen(t *testing.T, err error) {
	t.Helper()
	require.ErrorIs(t, err, ErrConfigFrozen)
	var refusal *opererr.Error
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, "request fleet disable", refusal.Fix())
	lines := strings.Split(err.Error(), "\n")
	assert.Equal(t, "fix: request fleet disable", lines[len(lines)-1], "the fix is not alone on the last line of:\n%s", err)
	assert.Contains(t, err.Error(), "this device is managed by a hub")
}

// TestFreezeRefusesOperatorStaging proves that every store entry an operator
// route stages through refuses the device's own configuration while the
// device is managed, and writes nothing.
//
// It seeds a managed store bound to its stored configuration, as a daemon
// started from its store holds it, drives each entry, and compares the
// version keys, the candidate and the active bytes before and after.
//
// VALIDATES: AC-16. A commit, a restore, an import and a rollback are refused
// with the operator error naming `request fleet disable`, and no version and
// no candidate is left in the store.
// PREVENTS: a refusal after the version was written, which leaves a candidate
// that makes the next hub push fail.
func TestFreezeRefusesOperatorStaging(t *testing.T) {
	entries := []struct {
		name  string
		stage func(store Storage) error
	}{
		{"candidate staging", func(store Storage) error {
			_, err := WriteCandidateVersion(store, freezeOwnConfig, []byte(freezeChange), time.Now())
			return err
		}},
		{"candidate staging under a held guard", func(store Storage) (err error) {
			guard, err := store.AcquireLock(freezeOwnConfig)
			if err != nil {
				return err
			}
			defer func() { err = releaseGuard(guard, err) }()
			_, err = WriteCandidateVersionWithGuard(store, guard, freezeOwnConfig, []byte(freezeChange), time.Now())
			return err
		}},
		{"editor commit", func(store Storage) (err error) {
			guard, err := store.AcquireLock(freezeOwnConfig)
			if err != nil {
				return err
			}
			defer func() { err = releaseGuard(guard, err) }()
			_, err = CommitConfigWithGuard(store, guard, freezeOwnConfig, []byte(freezeChange))
			return err
		}},
		{"restore", func(store Storage) error {
			_, err := RestoreConfig(store, freezeOwnConfig, []byte(freezeChange))
			return err
		}},
		{"import", func(store Storage) error {
			return ImportConfig(store, freezeOwnConfig, []byte(freezeChange))
		}},
		{"rollback", func(store Storage) error {
			_, _, err := RollbackConfig(store, freezeOwnConfig, 1)
			return err
		}},
	}
	for _, backend := range freezeLiveStores() {
		for _, entry := range entries {
			t.Run(backend.name+"/"+entry.name, func(t *testing.T) {
				owner := backend.newStore(t, t.TempDir())
				freezeSeed(t, owner, freezeOwnConfig, "true\n")
				store := BindConfigSource(owner, freezeOwnConfig, ConfigSourceStored)
				before := freezeStateOf(t, store, freezeOwnConfig)

				requireFrozen(t, entry.stage(store))

				assert.Equal(t, before, freezeStateOf(t, store, freezeOwnConfig), "a refused write changed the store")
			})
		}
	}
}

// TestFreezeAdmitsMachineEntry proves that the machine entries stage the
// device's own configuration while the device is managed.
//
// VALIDATES: AC-17, at the store: StageMachineCandidate (the hub push, the
// reverts, the boot record, schema recovery), StageMachineCandidateOver (the
// revert a hub's operator decided) and StageSourceCandidate (the
// signal reload) do not ask the freeze.
// PREVENTS: a managed device that refuses its own hub push or its own boot
// recovery.
func TestFreezeAdmitsMachineEntry(t *testing.T) {
	for _, backend := range freezeLiveStores() {
		t.Run(backend.name+"/machine candidate", func(t *testing.T) {
			owner := backend.newStore(t, t.TempDir())
			freezeSeed(t, owner, freezeOwnConfig, "true\n")
			store := BindConfigSource(owner, freezeOwnConfig, ConfigSourceStored)

			stamp, err := StageMachineCandidate(store, freezeOwnConfig, []byte(freezeChange), time.Now())
			require.NoError(t, err)

			staged, stagedStamp, present, err := ReadCandidateConfig(store, freezeOwnConfig)
			require.NoError(t, err)
			require.True(t, present, "the machine entry staged no candidate")
			assert.Equal(t, stamp, stagedStamp)
			assert.Equal(t, freezeChange, string(staged))
		})
		t.Run(backend.name+"/machine candidate over", func(t *testing.T) {
			owner := backend.newStore(t, t.TempDir())
			freezeSeed(t, owner, freezeOwnConfig, "true\n")
			store := BindConfigSource(owner, freezeOwnConfig, ConfigSourceStored)

			_, err := StageMachineCandidateOver(store, freezeOwnConfig, []byte(freezeChange), time.Now(),
				func([]byte) error { return nil })
			require.NoError(t, err)

			staged, _, present, err := ReadCandidateConfig(store, freezeOwnConfig)
			require.NoError(t, err)
			require.True(t, present, "the revert's machine entry staged no candidate")
			assert.Equal(t, freezeChange, string(staged))
		})
		t.Run(backend.name+"/source candidate", func(t *testing.T) {
			owner := backend.newStore(t, t.TempDir())
			freezeSeed(t, owner, freezeOwnConfig, "true\n")
			store := BindConfigSource(owner, freezeOwnConfig, ConfigSourceStored)

			_, err := StageSourceCandidate(store, freezeOwnConfig, time.Now())
			require.NoError(t, err)

			staged, _, present, err := ReadCandidateConfig(store, freezeOwnConfig)
			require.NoError(t, err)
			require.True(t, present, "the signal reload staged no candidate")
			assert.Equal(t, freezeActive, string(staged))
		})
	}
}

// TestFreezeAdmitsOtherConfig proves that the freeze covers the device's own
// configuration alone: another configuration of the same managed store is
// staged, as a hub that is itself managed stages a client's.
//
// VALIDATES: AC-19, R-7.
// PREVENTS: a managed hub that cannot write the configurations of its clients.
func TestFreezeAdmitsOtherConfig(t *testing.T) {
	for _, backend := range freezeLiveStores() {
		t.Run(backend.name, func(t *testing.T) {
			owner := backend.newStore(t, t.TempDir())
			freezeSeed(t, owner, freezeOwnConfig, "true\n")
			freezeSeed(t, owner, freezeOtherConfig, "")
			store := BindConfigSource(owner, freezeOwnConfig, ConfigSourceStored)

			_, err := WriteCandidateVersion(store, freezeOtherConfig, []byte(freezeChange), time.Now())
			require.NoError(t, err)

			_, _, present, err := ReadCandidateConfig(store, freezeOtherConfig)
			require.NoError(t, err)
			assert.True(t, present, "the other configuration was not staged")
			_, err = WriteCandidateVersion(store, freezeOwnConfig, []byte(freezeChange), time.Now())
			requireFrozen(t, err)
		})
	}
}

// TestFreezeAdmitsFileSource proves that a handle bound to an explicit file,
// or to stdin, is not frozen whatever the flag says: that input is the
// authority, and such a daemon runs no hub client.
//
// VALIDATES: AC-16, permitted survivor "a file-sourced daemon is not frozen".
// PREVENTS: a daemon started from a file that cannot commit because the store
// beside the file carries a managed flag.
func TestFreezeAdmitsFileSource(t *testing.T) {
	for _, backend := range freezeLiveStores() {
		t.Run(backend.name+"/file", func(t *testing.T) {
			dir := t.TempDir()
			owner := backend.newStore(t, dir)
			path := backend.configPath(dir)
			freezeSeed(t, owner, path, "true\n")
			store := BindConfigSource(owner, path, ConfigSourceFile)

			_, err := WriteCandidateVersion(store, path, []byte(freezeChange), time.Now())
			require.NoError(t, err)
		})
		t.Run(backend.name+"/stdin", func(t *testing.T) {
			owner := backend.newStore(t, t.TempDir())
			freezeSeed(t, owner, freezeOwnConfig, "true\n")
			store := BindConfigSource(owner, "-", ConfigSourceStdin)

			_, err := WriteCandidateVersion(store, freezeOwnConfig, []byte(freezeChange), time.Now())
			require.NoError(t, err)
		})
	}
}

// TestFreezeScopeStoppedTool proves how a handle with no source binding, the
// one a tool opens with the daemon stopped, tells the device's own
// configuration: by the instance name, as the launcher does.
//
// VALIDATES: A-4, and AC-22 for a device that is not managed.
// PREVENTS: a stopped tool that changes a managed device's configuration
// unrefused, and one that refuses a configuration that is not the device's.
func TestFreezeScopeStoppedTool(t *testing.T) {
	cases := []struct {
		name         string
		instanceName string
		flag         string
		staged       string
		frozen       bool
	}{
		{"the configuration the instance name selects", "edge-1", "true\n", "edge-1.conf", true},
		{"another configuration of that store", "edge-1", "true\n", freezeOtherConfig, false},
		{"no instance name selects the default name", "", "true", DefaultConfigName, true},
		{"no instance name and another configuration", "", "true", freezeOwnConfig, false},
		{"an instance name that is no file stem selects the default name", "edge.example", "true", DefaultConfigName, true},
		{"a device whose flag is false", "edge-1", "false\n", "edge-1.conf", false},
		{"a device with no flag", "edge-1", "", "edge-1.conf", false},
	}
	for _, backend := range freezeLiveStores() {
		for _, tt := range cases {
			t.Run(backend.name+"/"+tt.name, func(t *testing.T) {
				store := backend.newStore(t, t.TempDir())
				freezeSeed(t, store, tt.staged, tt.flag)
				if tt.instanceName != "" {
					require.NoError(t, store.WriteKey(zefs.KeyInstanceName.Pattern, []byte(tt.instanceName+"\n")))
				}
				before := freezeStateOf(t, store, tt.staged)

				_, err := RestoreConfig(store, tt.staged, []byte(freezeChange))

				if !tt.frozen {
					require.NoError(t, err)
					return
				}
				requireFrozen(t, err)
				assert.Equal(t, before, freezeStateOf(t, store, tt.staged), "a refused write changed the store")
			})
		}
	}
	// A backup carries the flag and the name of the device it was taken from.
	// It is not that device: its configurations stay writable.
	t.Run("an explicit blob artifact", func(t *testing.T) {
		artifact := newBlobStorageAt(t, t.TempDir())
		freezeSeed(t, artifact, "edge-1.conf", "true\n")
		require.NoError(t, artifact.WriteKey(zefs.KeyInstanceName.Pattern, []byte("edge-1\n")))

		_, err := RestoreConfig(artifact, "edge-1.conf", []byte(freezeChange))
		require.NoError(t, err)
	})
}

// unreadableKeyGuard is a held guard whose store cannot read one key: every
// other operation reaches the real guard.
type unreadableKeyGuard struct {
	WriteGuard
	key     string
	failure error
}

func (g unreadableKeyGuard) ReadKey(key string) ([]byte, error) {
	if key == g.key {
		return nil, g.failure
	}
	return g.WriteGuard.ReadKey(key)
}

// TestFreezeRefusesOnUnreadableFlag proves that a key the freeze decides on,
// and that the store cannot read, refuses the write and names the key. Only an
// absent key is "not managed".
//
// VALIDATES: R-8.
// PREVENTS: a damaged store whose unreadable flag reads as "not managed" and
// lifts the freeze.
func TestFreezeRefusesOnUnreadableFlag(t *testing.T) {
	failure := errors.New("checksum mismatch")
	cases := []struct {
		name  string
		key   string
		bound bool
	}{
		{"the managed flag of a daemon's handle", zefs.KeyInstanceManaged.Pattern, true},
		{"the managed flag of a stopped tool's handle", zefs.KeyInstanceManaged.Pattern, false},
		{"the instance name of a stopped tool's handle", zefs.KeyInstanceName.Pattern, false},
	}
	for _, backend := range freezeLiveStores() {
		for _, tt := range cases {
			t.Run(backend.name+"/"+tt.name, func(t *testing.T) {
				store := backend.newStore(t, t.TempDir())
				freezeSeed(t, store, freezeOwnConfig, "true\n")
				// The instance name makes router.conf the device's own on a
				// stopped tool's handle too: the freeze compares the name before
				// it reads the flag, and a name that is not the device's own is
				// admitted whatever the flag says.
				require.NoError(t, store.WriteKey(zefs.KeyInstanceName.Pattern, []byte(strings.TrimSuffix(freezeOwnConfig, ".conf"))))
				if tt.bound {
					store = BindConfigSource(store, freezeOwnConfig, ConfigSourceStored)
				}
				before := freezeStateOf(t, store, freezeOwnConfig)

				held, err := store.AcquireLock(freezeOwnConfig)
				require.NoError(t, err)
				guard := unreadableKeyGuard{WriteGuard: held, key: tt.key, failure: failure}
				_, stageErr := WriteCandidateVersionWithGuard(store, guard, freezeOwnConfig, []byte(freezeChange), time.Now())
				require.NoError(t, held.Release())

				require.ErrorIs(t, stageErr, failure)
				require.NotErrorIs(t, stageErr, ErrConfigFrozen)
				var refusal *opererr.Error
				require.ErrorAs(t, stageErr, &refusal)
				assert.Contains(t, refusal.Problem(), tt.key)
				assert.Equal(t, "ze show data check", refusal.Fix())
				assert.Equal(t, before, freezeStateOf(t, store, freezeOwnConfig), "a refused write changed the store")
			})
		}
	}
}

// TestFreezeReadsTheNameFirst proves the order in which the freeze reads the
// two keys it decides on: the configuration name first, so a configuration
// that is not the device's own is admitted whatever the flag, and a read
// failure refuses only when the keys that were read leave the outcome open.
// Each case stages through a guard that cannot read the keys it names.
//
// VALIDATES: a non-own name is admitted with an unreadable flag, on a stopped
// tool's handle and on a daemon's; an unreadable instance name is admitted on
// a device whose flag is false or absent, and refused when the flag is true or
// unreadable.
// PREVENTS: a stopped managed store with a damaged flag on which every client
// configuration import is refused.
func TestFreezeReadsTheNameFirst(t *testing.T) {
	failure := errors.New("checksum mismatch")
	flagKey := zefs.KeyInstanceManaged.Pattern
	nameKey := zefs.KeyInstanceName.Pattern
	cases := []struct {
		name       string
		bound      bool
		flag       string
		staged     string
		unreadable []string
		refused    bool
	}{
		{"another configuration of a stopped tool's store, flag unreadable", false, "true\n", freezeOtherConfig, []string{flagKey}, false},
		{"another configuration of a daemon's store, flag unreadable", true, "true\n", freezeOtherConfig, []string{flagKey}, false},
		{"unreadable instance name, flag false", false, "false\n", freezeOwnConfig, []string{nameKey}, false},
		{"unreadable instance name, no flag", false, "", freezeOwnConfig, []string{nameKey}, false},
		{"unreadable instance name, flag true", false, "true\n", freezeOwnConfig, []string{nameKey}, true},
		{"unreadable instance name and flag", false, "true\n", freezeOwnConfig, []string{nameKey, flagKey}, true},
	}
	for _, backend := range freezeLiveStores() {
		for _, tt := range cases {
			t.Run(backend.name+"/"+tt.name, func(t *testing.T) {
				store := backend.newStore(t, t.TempDir())
				freezeSeed(t, store, tt.staged, tt.flag)
				require.NoError(t, store.WriteKey(nameKey, []byte(strings.TrimSuffix(freezeOwnConfig, ".conf")+"\n")))
				if tt.bound {
					store = BindConfigSource(store, freezeOwnConfig, ConfigSourceStored)
				}

				held, err := store.AcquireLock(tt.staged)
				require.NoError(t, err)
				guard := held
				for _, key := range tt.unreadable {
					guard = unreadableKeyGuard{WriteGuard: guard, key: key, failure: failure}
				}
				_, stageErr := WriteCandidateVersionWithGuard(store, guard, tt.staged, []byte(freezeChange), time.Now())
				require.NoError(t, held.Release())

				if !tt.refused {
					require.NoError(t, stageErr)
					return
				}
				require.ErrorIs(t, stageErr, failure)
				var refusal *opererr.Error
				require.ErrorAs(t, stageErr, &refusal)
				assert.Equal(t, "ze show data check", refusal.Fix())
			})
		}
	}
}

// TestOwnConfigFrozenFollowsTheBinding proves that the predicate `show fleet
// status` asks gives the freeze's answer for each handle over one managed
// store.
//
// VALIDATES: unbound and stored-bound handles are frozen, file- and
// stdin-bound handles are not.
// PREVENTS: a status that reports the freeze by another rule than the one the
// store applies.
func TestOwnConfigFrozenFollowsTheBinding(t *testing.T) {
	for _, backend := range freezeLiveStores() {
		dir := t.TempDir()
		store := backend.newStore(t, dir)
		freezeSeed(t, store, DefaultConfigName, "true\n")
		for name, want := range map[string]struct {
			handle Storage
			frozen bool
		}{
			"unbound": {store, true},
			"stored":  {BindConfigSource(store, DefaultConfigName, ConfigSourceStored), true},
			"file":    {BindConfigSource(store, backend.configPath(dir), ConfigSourceFile), false},
			"stdin":   {BindConfigSource(store, "-", ConfigSourceStdin), false},
		} {
			t.Run(backend.name+"/"+name, func(t *testing.T) {
				frozen, err := OwnConfigFrozen(want.handle)
				require.NoError(t, err)
				assert.Equal(t, want.frozen, frozen)
			})
		}
	}
}

// TestStageMachineCandidateOverRefusesChangedActive proves the revert's stage
// hands its check the configuration active under the lock and stages nothing
// when the check refuses it.
//
// VALIDATES: spec-fleet-7 review note N1, at the store: check receives the
// active bytes, its refusal is answered as is, and the store is left as it
// was found (no version written, no candidate).
// PREVENTS: a revert decided on a configuration a local commit replaced
// overwriting that commit.
//
// MUTATION: drop the check call of StageMachineCandidateOver and this fails:
// a candidate is staged.
func TestStageMachineCandidateOverRefusesChangedActive(t *testing.T) {
	for _, backend := range freezeLiveStores() {
		t.Run(backend.name, func(t *testing.T) {
			owner := backend.newStore(t, t.TempDir())
			freezeSeed(t, owner, freezeOwnConfig, "true\n")
			store := BindConfigSource(owner, freezeOwnConfig, ConfigSourceStored)
			before := freezeStateOf(t, store, freezeOwnConfig)
			changed := errors.New("the active configuration is not the discarded one")
			var checked []string

			_, err := StageMachineCandidateOver(store, freezeOwnConfig, []byte(freezeChange), time.Now(), func(active []byte) error {
				checked = append(checked, string(active))
				return changed
			})

			require.ErrorIs(t, err, changed)
			assert.Equal(t, []string{freezeActive}, checked, "check was not handed the active configuration once")
			assert.Equal(t, before, freezeStateOf(t, store, freezeOwnConfig), "a refused stage changed the store")
		})
	}
}
