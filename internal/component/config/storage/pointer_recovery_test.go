package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromotionResumesAfterActiveCommit(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			s := backend.newStore(t, t.TempDir())
			old := "20260524-090000.000"
			next := "20260524-100000.000"
			require.NoError(t, s.WriteVersion("router.conf", []byte("old"), mustParseVersionStamp(t, old)))
			require.NoError(t, s.WriteVersion("router.conf", []byte("new"), mustParseVersionStamp(t, next)))
			require.NoError(t, s.WriteFile("router.conf", []byte("stale mirror"), 0))
			require.NoError(t, writePointer(s, "router.conf", pointerRollback, old))
			require.NoError(t, writePointer(s, "router.conf", pointerActive, next))
			require.NoError(t, writePointer(s, "router.conf", pointerCandidate, next))
			require.NoError(t, PromoteCandidate(s, "router.conf"))
			rollback, present, err := readPointer(s, "router.conf", pointerRollback)
			require.NoError(t, err)
			require.True(t, present)
			assert.Equal(t, old, rollback)
			_, present, err = readPointer(s, "router.conf", pointerCandidate)
			require.NoError(t, err)
			assert.False(t, present)
			data, err := s.ReadFile("router.conf")
			require.NoError(t, err)
			assert.Equal(t, "new", string(data))
			data, err = readVersion(s, "router.conf", old)
			require.NoError(t, err)
			assert.Equal(t, "old", string(data))
		})
	}
}

func TestRemoveVersionPreservesEveryReference(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			for _, pointer := range []pointerName{pointerActive, pointerCandidate, pointerRollback, pointerRecovery} {
				t.Run(string(pointer), func(t *testing.T) {
					s := backend.newStore(t, t.TempDir())
					stamp := "20260524-100000.000"
					require.NoError(t, s.WriteVersion("router.conf", []byte("referenced"), mustParseVersionStamp(t, stamp)))
					require.NoError(t, writePointer(s, "router.conf", pointer, stamp))
					require.NoError(t, removeVersion(s, "router.conf", stamp))
					got, err := readVersion(s, "router.conf", stamp)
					require.NoError(t, err)
					assert.Equal(t, "referenced", string(got))
					value, present, err := readPointer(s, "router.conf", pointer)
					require.NoError(t, err)
					assert.True(t, present)
					assert.Equal(t, stamp, value)
				})
			}
		})
	}
}

func TestCandidateTimestampCollisionPreservesVersion(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			s := backend.newStore(t, t.TempDir())
			stamp := mustParseVersionStamp(t, "20260524-100000.000")
			require.NoError(t, s.WriteVersion("router.conf", []byte("existing"), stamp))
			actual, err := WriteCandidateVersion(s, "router.conf", []byte("collision"), stamp)
			require.NoError(t, err)
			assert.NotEqual(t, FormatVersionStamp(stamp), actual)
			data, err := readVersion(s, "router.conf", FormatVersionStamp(stamp))
			require.NoError(t, err)
			assert.Equal(t, "existing", string(data))
			candidate, present, err := readPointer(s, "router.conf", pointerCandidate)
			require.NoError(t, err)
			assert.True(t, present)
			assert.Equal(t, actual, candidate)
			data, err = readVersion(s, "router.conf", actual)
			require.NoError(t, err)
			assert.Equal(t, "collision", string(data))
		})
	}
}
