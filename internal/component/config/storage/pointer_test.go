// Design: docs/architecture/zefs-format.md -- transactional config pointers

package storage

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pointerTestStore struct {
	name       string
	newStore   func(t *testing.T, dir string) Storage
	configPath func(dir string) string
}

func pointerTestStores() []pointerTestStore {
	return []pointerTestStore{
		{
			name: "filesystem",
			newStore: func(t *testing.T, _ string) Storage {
				t.Helper()
				return NewFilesystem()
			},
			configPath: func(dir string) string { return filepath.Join(dir, "router.conf") },
		},
		{
			name: "blob",
			newStore: func(t *testing.T, dir string) Storage {
				t.Helper()
				store, err := NewBlob(filepath.Join(dir, "test.zefs"), dir)
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, store.Close()) })
				return store
			},
			configPath: func(_ string) string { return "/etc/ze/router.conf" },
		},
	}
}

func TestWriteCandidateVersion(t *testing.T) {
	for _, tt := range pointerTestStores() {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			store := tt.newStore(t, dir)
			configPath := tt.configPath(dir)
			stamp := time.Date(2026, 5, 24, 10, 0, 0, 123_000_000, time.Local)

			// VALIDATES: AC-1 writes config as timestamped version and records candidate pointer before promotion.
			// PREVENTS: transactional commit mutating active config before runtime verification.
			gotStamp, err := WriteCandidateVersion(store, configPath, []byte("candidate"), stamp)
			require.NoError(t, err)
			assert.Equal(t, "20260524-100000.123", gotStamp)

			pointer, ok, err := ReadPointer(store, configPath, PointerCandidate)
			require.NoError(t, err)
			require.True(t, ok, "candidate pointer should exist")
			assert.Equal(t, gotStamp, pointer)

			data, err := ReadVersion(store, configPath, gotStamp)
			require.NoError(t, err)
			assert.Equal(t, "candidate", string(data))
		})
	}
}

func TestWriteCandidateVersionRejectsExistingCandidate(t *testing.T) {
	for _, tt := range pointerTestStores() {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			store := tt.newStore(t, dir)
			configPath := tt.configPath(dir)

			_, err := WriteCandidateVersion(store, configPath, []byte("first"), mustParseVersionStamp(t, "20260524-100000.000"))
			require.NoError(t, err)

			_, err = WriteCandidateVersion(store, configPath, []byte("second"), mustParseVersionStamp(t, "20260524-100001.000"))
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrCandidateExists))

			data, _, ok, readErr := ReadCandidateConfig(store, configPath)
			require.NoError(t, readErr)
			require.True(t, ok)
			assert.Equal(t, "first", string(data))
		})
	}
}

func TestWriteCandidateVersionWithGuardRejectsExistingCandidate(t *testing.T) {
	for _, tt := range pointerTestStores() {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			store := tt.newStore(t, dir)
			configPath := tt.configPath(dir)

			guard, err := store.AcquireLock(configPath)
			require.NoError(t, err)
			defer guard.Release() //nolint:errcheck // test cleanup

			_, err = WriteCandidateVersionWithGuard(store, guard, configPath, []byte("first"), mustParseVersionStamp(t, "20260524-100000.000"))
			require.NoError(t, err)

			_, err = WriteCandidateVersionWithGuard(store, guard, configPath, []byte("second"), mustParseVersionStamp(t, "20260524-100001.000"))
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrCandidateExists))

			candidate, ok, readErr := readPointerLocked(store, guard, configPath, PointerCandidate)
			require.NoError(t, readErr)
			require.True(t, ok)
			data, readErr := readVersionLocked(store, guard, configPath, candidate)
			require.NoError(t, readErr)
			assert.Equal(t, "first", string(data))
		})
	}
}

func TestEnsureActiveVersion(t *testing.T) {
	for _, tt := range pointerTestStores() {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			store := tt.newStore(t, dir)
			configPath := tt.configPath(dir)
			stamp := mustParseVersionStamp(t, "20260524-090000.000")

			// VALIDATES: AC-12 boots from an active pointer even for legacy configs after startup.
			// PREVENTS: first failed SIGHUP leaving boot fallback on an edited bad config file.
			gotStamp, wrote, err := EnsureActiveVersion(store, configPath, []byte("active"), stamp)
			require.NoError(t, err)
			assert.True(t, wrote)
			assert.Equal(t, "20260524-090000.000", gotStamp)

			active, ok, err := ReadPointer(store, configPath, PointerActive)
			require.NoError(t, err)
			require.True(t, ok)
			assert.Equal(t, gotStamp, active)

			data, err := ReadActiveConfig(store, configPath)
			require.NoError(t, err)
			assert.Equal(t, "active", string(data))

			gotStamp, wrote, err = EnsureActiveVersion(store, configPath, []byte("ignored"), mustParseVersionStamp(t, "20260524-100000.000"))
			require.NoError(t, err)
			assert.False(t, wrote)
			assert.Equal(t, active, gotStamp)
		})
	}
}

func TestPromoteCandidateToActive(t *testing.T) {
	for _, tt := range pointerTestStores() {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			store := tt.newStore(t, dir)
			configPath := tt.configPath(dir)
			oldStamp := "20260524-090000.000"
			newStamp := "20260524-100000.000"

			require.NoError(t, store.WriteVersion(configPath, []byte("old"), mustParseVersionStamp(t, oldStamp)))
			require.NoError(t, WritePointer(store, configPath, PointerActive, oldStamp))
			require.NoError(t, store.WriteVersion(configPath, []byte("new"), mustParseVersionStamp(t, newStamp)))
			require.NoError(t, WritePointer(store, configPath, PointerCandidate, newStamp))

			// VALIDATES: AC-1 and AC-15 promote candidate to active and set rollback to previous active.
			// PREVENTS: losing the previous active version during successful transactional commit.
			require.NoError(t, PromoteCandidate(store, configPath))

			active, ok, err := ReadPointer(store, configPath, PointerActive)
			require.NoError(t, err)
			require.True(t, ok)
			assert.Equal(t, newStamp, active)

			rollback, ok, err := ReadPointer(store, configPath, PointerRollback)
			require.NoError(t, err)
			require.True(t, ok)
			assert.Equal(t, oldStamp, rollback)

			_, ok, err = ReadPointer(store, configPath, PointerCandidate)
			require.NoError(t, err)
			assert.False(t, ok, "candidate pointer should be cleared after promotion")

			data, err := ReadActiveConfig(store, configPath)
			require.NoError(t, err)
			assert.Equal(t, "new", string(data))
		})
	}
}

func TestPromoteCandidateMirrorFailureIsNonFatal(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "router.conf")
	base := NewFilesystem()
	store := &mirrorFailStore{Storage: base, failPath: configPath}
	oldStamp := "20260524-090000.000"
	newStamp := "20260524-100000.000"

	require.NoError(t, base.WriteFile(configPath, []byte("old-file"), 0o600))
	require.NoError(t, store.WriteVersion(configPath, []byte("old"), mustParseVersionStamp(t, oldStamp)))
	require.NoError(t, WritePointer(store, configPath, PointerActive, oldStamp))
	require.NoError(t, store.WriteVersion(configPath, []byte("new"), mustParseVersionStamp(t, newStamp)))
	require.NoError(t, WritePointer(store, configPath, PointerCandidate, newStamp))

	// VALIDATES: AC-1 promotion result is authoritative even if legacy mirror write fails.
	// PREVENTS: callers reporting commit failure after active pointer already advanced.
	require.NoError(t, PromoteCandidate(store, configPath))

	active, ok, err := ReadPointer(store, configPath, PointerActive)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, newStamp, active)
	_, ok, err = ReadPointer(store, configPath, PointerCandidate)
	require.NoError(t, err)
	assert.False(t, ok)
	data, err := ReadActiveConfig(store, configPath)
	require.NoError(t, err)
	assert.Equal(t, "new", string(data))
}

type mirrorFailStore struct {
	Storage
	failPath string
}

func (s *mirrorFailStore) AcquireLock(name string) (WriteGuard, error) {
	guard, err := s.Storage.AcquireLock(name)
	if err != nil {
		return nil, err
	}
	return &mirrorFailGuard{WriteGuard: guard, failPath: s.failPath}, nil
}

type mirrorFailGuard struct {
	WriteGuard
	failPath string
}

func (g *mirrorFailGuard) WriteFile(name string, data []byte, perm fs.FileMode) error {
	if name == g.failPath {
		return errors.New("mirror failed")
	}
	return g.WriteGuard.WriteFile(name, data, perm)
}

func TestPromoteFailureCleanup(t *testing.T) {
	for _, tt := range pointerTestStores() {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			store := tt.newStore(t, dir)
			configPath := tt.configPath(dir)
			oldStamp := "20260524-090000.000"
			candidateStamp := "20260524-100000.000"

			require.NoError(t, store.WriteVersion(configPath, []byte("old"), mustParseVersionStamp(t, oldStamp)))
			require.NoError(t, WritePointer(store, configPath, PointerActive, oldStamp))
			require.NoError(t, store.WriteVersion(configPath, []byte("candidate"), mustParseVersionStamp(t, candidateStamp)))
			require.NoError(t, WritePointer(store, configPath, PointerCandidate, candidateStamp))

			// VALIDATES: AC-2 clears failed candidate while leaving active unchanged.
			// PREVENTS: boot or a later reload applying a candidate that failed verification.
			require.NoError(t, ClearCandidate(store, configPath))

			active, ok, err := ReadPointer(store, configPath, PointerActive)
			require.NoError(t, err)
			require.True(t, ok)
			assert.Equal(t, oldStamp, active)

			_, ok, err = ReadPointer(store, configPath, PointerCandidate)
			require.NoError(t, err)
			assert.False(t, ok)

			_, err = ReadVersion(store, configPath, candidateStamp)
			require.Error(t, err, "candidate version should be removed on failed transaction cleanup")
		})
	}
}

func TestBootFromActivePointer(t *testing.T) {
	for _, tt := range pointerTestStores() {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			store := tt.newStore(t, dir)
			configPath := tt.configPath(dir)
			activeStamp := "20260524-100000.000"
			candidateStamp := "20260524-110000.000"

			require.NoError(t, store.WriteFile(configPath, []byte("legacy-active"), 0o600))
			require.NoError(t, store.WriteVersion(configPath, []byte("active-pointer"), mustParseVersionStamp(t, activeStamp)))
			require.NoError(t, WritePointer(store, configPath, PointerActive, activeStamp))
			require.NoError(t, store.WriteVersion(configPath, []byte("stale-candidate"), mustParseVersionStamp(t, candidateStamp)))
			require.NoError(t, WritePointer(store, configPath, PointerCandidate, candidateStamp))

			// VALIDATES: AC-12 boots from active pointer, not legacy active file or stale candidate.
			// PREVENTS: crash recovery accidentally activating an unverified candidate.
			data, err := ReadActiveConfig(store, configPath)
			require.NoError(t, err)
			assert.Equal(t, "active-pointer", string(data))
		})
	}
}

func TestBootIgnoresStaleCandidate(t *testing.T) {
	for _, tt := range pointerTestStores() {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			store := tt.newStore(t, dir)
			configPath := tt.configPath(dir)
			activeStamp := "20260524-100000.000"
			candidateStamp := "20260524-110000.000"

			require.NoError(t, store.WriteVersion(configPath, []byte("active"), mustParseVersionStamp(t, activeStamp)))
			require.NoError(t, WritePointer(store, configPath, PointerActive, activeStamp))
			require.NoError(t, store.WriteVersion(configPath, []byte("candidate"), mustParseVersionStamp(t, candidateStamp)))
			require.NoError(t, WritePointer(store, configPath, PointerCandidate, candidateStamp))

			// VALIDATES: AC-12 stale candidate cleanup leaves active pointer authoritative.
			// PREVENTS: unverified candidate pointer surviving restart cleanup.
			require.NoError(t, ClearCandidate(store, configPath))

			data, err := ReadActiveConfig(store, configPath)
			require.NoError(t, err)
			assert.Equal(t, "active", string(data))

			_, ok, err := ReadPointer(store, configPath, PointerCandidate)
			require.NoError(t, err)
			assert.False(t, ok)
		})
	}
}

func mustParseVersionStamp(t *testing.T, stamp string) time.Time {
	t.Helper()
	parsed, err := ParseVersionStamp(stamp)
	require.NoError(t, err)
	return parsed
}
