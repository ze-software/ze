// Design: docs/architecture/web-interface.md -- explicit corrupt-change recovery.
package cli

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestCorruptChangeRecoveryScope removes only the inspected account's unreadable
// unsaved file. Saved drafts, committed data and another account survive intact.
func TestCorruptChangeRecoveryScope(t *testing.T) {
	store := storage.CreateMemory()
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	const path = "ze.conf"
	kept := map[string]string{
		path:                    "router-id 1.2.3.4\n",
		DraftPath(path):         "saved draft sentinel",
		ChangePath(path, "bob"): "other user sentinel",
	}
	for name, content := range kept {
		require.NoError(t, store.WriteFile(name, []byte(content), 0o600))
	}
	const broken = "set unknown-path secret-must-not-escape\n"
	require.NoError(t, store.WriteFile(ChangePath(path, "alice"), []byte(broken), 0o600))
	recovery, err := InspectCorruptChangeFile(store, path, testNavSchema(), "alice")
	require.NoError(t, err)
	require.Len(t, recovery.Fingerprint(), 64)
	before, err := store.ReadFile(ChangePath(path, "alice"))
	require.NoError(t, err)
	require.Equal(t, broken, string(before), "inspection is read-only")
	outcome, err := recovery.Discard()
	require.Equal(t, ChangeRecoveryRemoved, outcome)
	require.NoError(t, err)
	_, err = store.ReadFile(ChangePath(path, "alice"))
	require.ErrorIs(t, err, fs.ErrNotExist)
	for name, content := range kept {
		data, err := store.ReadFile(name)
		require.NoError(t, err)
		require.Equal(t, content, string(data), name)
	}
}

// TestCorruptChangeRecoveryStale refuses to delete different bytes, whether the
// replacement is another corrupt file or a repaired candidate.
func TestCorruptChangeRecoveryStale(t *testing.T) {
	for _, replacement := range []string{"set other-unknown value\n", "set router-id 10.0.0.1\n"} {
		t.Run(replacement, func(t *testing.T) {
			store := storage.CreateMemory()
			t.Cleanup(func() { require.NoError(t, store.Close()) })
			path := ChangePath("ze.conf", "alice")
			require.NoError(t, store.WriteFile(path, []byte("set unknown value\n"), 0o600))
			recovery, err := InspectCorruptChangeFile(store, "ze.conf", testNavSchema(), "alice")
			require.NoError(t, err)
			require.NoError(t, store.WriteFile(path, []byte(replacement), 0o600))
			outcome, err := recovery.Discard()
			require.Equal(t, ChangeRecoveryKept, outcome)
			require.Error(t, err)
			require.Contains(t, err.Error(), "changed")
			data, err := store.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, replacement, string(data))
		})
	}
}

// TestCorruptChangeRecoveryRefusesReadable includes an invalid value: candidate
// leaf validation stays deferred so ordinary editing can correct that value.
func TestCorruptChangeRecoveryRefusesReadable(t *testing.T) {
	for _, candidate := range []string{"", "set router-id 10.0.0.1\n", "set router-id invalid-ip\n"} {
		t.Run(candidate, func(t *testing.T) {
			store := storage.CreateMemory()
			t.Cleanup(func() { require.NoError(t, store.Close()) })
			path := ChangePath("ze.conf", "alice")
			require.NoError(t, store.WriteFile(path, []byte(candidate), 0o600))
			_, err := InspectCorruptChangeFile(store, "ze.conf", testNavSchema(), "alice")
			require.Error(t, err)
			data, err := store.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, candidate, string(data))
		})
	}
}

// TestCorruptChangeRecoveryInvalidUser refuses path aliases before reading any
// file; a recovery target always comes from the authenticated account.
func TestCorruptChangeRecoveryInvalidUser(t *testing.T) {
	_, err := InspectCorruptChangeFile(nil, "ze.conf", testNavSchema(), "../alice")
	require.Error(t, err)
}

// TestCorruptChangeRecoveryStorageFailures distinguishes refused removal from
// an applied removal whose final release reports an error. Callers must audit
// and refresh even in that second case.
func TestCorruptChangeRecoveryStorageFailures(t *testing.T) {
	for _, failure := range []string{"lock", "read", "remove", "release", "removed-error", "unknown-error"} {
		t.Run(failure, func(t *testing.T) {
			base := storage.CreateMemory()
			t.Cleanup(func() { require.NoError(t, base.Close()) })
			store := &recoveryFailureStore{Storage: base}
			path := ChangePath("ze.conf", "alice")
			require.NoError(t, base.WriteFile(path, []byte("set unknown value\n"), 0o600))
			recovery, err := InspectCorruptChangeFile(store, "ze.conf", testNavSchema(), "alice")
			require.NoError(t, err)
			store.failure = failure
			outcome, err := recovery.Discard()
			require.ErrorIs(t, err, errRecoveryInjected)
			switch failure {
			case "lock", "read", "remove":
				require.Equal(t, ChangeRecoveryKept, outcome)
				require.True(t, base.Exists(path))
			case "release", "removed-error":
				require.Equal(t, ChangeRecoveryRemoved, outcome)
				require.False(t, base.Exists(path))
			case "unknown-error":
				require.Equal(t, ChangeRecoveryUncertain, outcome)
				require.NotContains(t, err.Error(), "kept the file")
			}
		})
	}
}

// TestCorruptChangeRecoveryInspectionFailure refuses a recovery form when its
// read or final unlock fails; inspection never changes the unreadable bytes.
func TestCorruptChangeRecoveryInspectionFailure(t *testing.T) {
	for _, failure := range []string{"lock", "read", "release"} {
		t.Run(failure, func(t *testing.T) {
			base := storage.CreateMemory()
			t.Cleanup(func() { require.NoError(t, base.Close()) })
			path := ChangePath("ze.conf", "alice")
			const original = "set unknown value\n"
			require.NoError(t, base.WriteFile(path, []byte(original), 0o600))
			store := &recoveryFailureStore{Storage: base, failure: failure}
			_, err := InspectCorruptChangeFile(store, "ze.conf", testNavSchema(), "alice")
			require.ErrorIs(t, err, errRecoveryInjected)
			data, err := base.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, original, string(data))
		})
	}
}

// TestCorruptChangeRecoveryMissingOrUninitialized keeps missing files and
// unconstructed recovery values from becoming successful destructive actions.
func TestCorruptChangeRecoveryMissingOrUninitialized(t *testing.T) {
	store := storage.CreateMemory()
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	_, err := InspectCorruptChangeFile(store, "ze.conf", testNavSchema(), "alice")
	require.ErrorIs(t, err, fs.ErrNotExist)
	for _, recovery := range []*CorruptChangeFile{nil, {}} {
		outcome, err := recovery.Discard()
		require.Equal(t, ChangeRecoveryKept, outcome)
		require.Error(t, err)
	}
}

var errRecoveryInjected = errors.New("injected recovery storage failure")

type recoveryFailureStore struct {
	storage.Storage
	failure string
}

func (s *recoveryFailureStore) AcquireLock(path string) (storage.WriteGuard, error) {
	if s.failure == "lock" {
		return nil, errRecoveryInjected
	}
	guard, err := s.Storage.AcquireLock(path)
	if err != nil {
		return nil, err
	}
	return &recoveryFailureGuard{WriteGuard: guard, failure: s.failure}, nil
}

type recoveryFailureGuard struct {
	storage.WriteGuard
	failure string
	removed bool
}

func (g *recoveryFailureGuard) ReadFile(path string) ([]byte, error) {
	if g.removed {
		if g.failure == "unknown-error" {
			return nil, errRecoveryInjected
		}
	}
	if g.failure == "read" {
		return nil, errRecoveryInjected
	}
	return g.WriteGuard.ReadFile(path)
}

func (g *recoveryFailureGuard) Remove(path string) error {
	if g.failure == "remove" {
		return errRecoveryInjected
	}
	if g.failure == "removed-error" {
		if err := g.WriteGuard.Remove(path); err != nil {
			return err
		}
		g.removed = true
		return errRecoveryInjected
	}
	if g.failure == "unknown-error" {
		if err := g.WriteGuard.Remove(path); err != nil {
			return err
		}
		g.removed = true
		return errRecoveryInjected
	}
	return g.WriteGuard.Remove(path)
}

func (g *recoveryFailureGuard) Release() error {
	err := g.WriteGuard.Release()
	if g.failure == "release" {
		return errors.Join(err, errRecoveryInjected)
	}
	return err
}
