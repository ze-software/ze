// Design: docs/architecture/storage-backends.md -- content-addressed config history
package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// findingKinds maps each finding's key to its kind, for assertions.
func findingKinds(findings []HistoryFinding) map[string]string {
	kinds := make(map[string]string, len(findings))
	for _, finding := range findings {
		kinds[finding.Key] = finding.Kind
	}
	return kinds
}

// TestCheckReportsOrphanAndDangling verifies AC-10: an unreferenced object is
// a warning, and an entry with no object and a malformed entry are errors.
func TestCheckReportsOrphanAndDangling(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			dir := t.TempDir()
			s := backend.newStore(t, dir)
			name := backend.configPath(dir)
			kept := []byte("kept\n")
			lost := []byte("lost\n")
			require.NoError(t, s.WriteVersion(name, kept, historyStampA))
			require.NoError(t, s.WriteVersion(name, lost, historyStampB))
			_, lostObject := historyKeys(historyStampB, lost)
			require.NoError(t, s.RemoveKey(lostObject))
			orphan := objectKey(contentDigest([]byte("orphan\n")))
			require.NoError(t, s.WriteKey(orphan, []byte("orphan\n")))
			malformed, _ := historyKeys(historyStampC, nil)
			require.NoError(t, s.WriteKey(malformed, []byte("not a digest")))

			findings, err := CheckHistory(s)
			require.NoError(t, err)
			kinds := findingKinds(findings)
			danglingEntry, _ := historyKeys(historyStampB, lost)
			assert.Equal(t, HistoryDanglingEntry, kinds[danglingEntry])
			assert.Equal(t, HistoryOrphanObject, kinds[orphan])
			assert.Equal(t, HistoryMalformedEntry, kinds[malformed])
			assert.Len(t, findings, 3)
			for _, finding := range findings {
				want := HistorySeverityError
				if finding.Kind == HistoryOrphanObject {
					want = HistorySeverityWarning
				}
				assert.Equal(t, want, finding.Severity, finding.Key)
			}
		})
	}
}

// TestCheckReportsWrongHashObject verifies AC-10 and R-8: a CRC-valid object
// whose bytes do not hash to its name is an error naming both hashes.
func TestCheckReportsWrongHashObject(t *testing.T) {
	dir := t.TempDir()
	s := newTreeStorage(t, dir)
	data := []byte("promised\n")
	require.NoError(t, s.WriteVersion("router.conf", data, historyStampA))
	_, object := historyKeys(historyStampA, data)
	require.NoError(t, s.WriteKey(object, []byte("impostor\n")))

	findings, err := CheckHistory(s)
	require.NoError(t, err)
	kinds := findingKinds(findings)
	assert.Equal(t, HistoryWrongHashObject, kinds[object])
	entry, _ := historyKeys(historyStampA, data)
	assert.Equal(t, HistoryDanglingEntry, kinds[entry])
	for _, finding := range findings {
		if finding.Kind == HistoryWrongHashObject {
			assert.Contains(t, finding.Detail, contentDigest(data))
			assert.Contains(t, finding.Detail, contentDigest([]byte("impostor\n")))
		}
	}
}

// TestCheckReportsDanglingPointer verifies AC-10: a pointer naming a stamp
// with no entry is an error naming the pointer and the stamp.
func TestCheckReportsDanglingPointer(t *testing.T) {
	dir := t.TempDir()
	s := newTreeStorage(t, dir)
	stamp, _, err := EnsureActiveVersion(s, "router.conf", []byte("active\n"), historyStampA)
	require.NoError(t, err)
	entry, _ := historyKeys(historyStampA, nil)
	require.NoError(t, s.RemoveKey(entry))

	findings, err := CheckHistory(s)
	require.NoError(t, err)
	var pointer HistoryFinding
	for _, finding := range findings {
		if finding.Kind == HistoryDanglingPointer {
			pointer = finding
		}
	}
	assert.Equal(t, "meta/config/router.conf/active", pointer.Key)
	assert.Equal(t, HistorySeverityError, pointer.Severity)
	assert.Contains(t, pointer.Detail, stamp)
}

// TestRepairDropsDangling verifies AC-11: repair drops a dangling entry, a
// malformed entry and a wrong-hash object, keeps the orphan, and leaves a
// store whose check reports nothing but the orphan.
func TestRepairDropsDangling(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			dir := t.TempDir()
			s := backend.newStore(t, dir)
			name := backend.configPath(dir)
			good := []byte("good\n")
			wrong := []byte("wrong\n")
			require.NoError(t, s.WriteVersion(name, good, historyStampA))
			require.NoError(t, s.WriteVersion(name, wrong, historyStampB))
			wrongEntry, wrongObject := historyKeys(historyStampB, wrong)
			require.NoError(t, s.WriteKey(wrongObject, []byte("rotten\n")))
			malformed, _ := historyKeys(historyStampC, nil)
			require.NoError(t, s.WriteKey(malformed, []byte("sha256:xyz")))
			orphan := objectKey(contentDigest([]byte("orphan\n")))
			require.NoError(t, s.WriteKey(orphan, []byte("orphan\n")))

			findings, err := RepairHistory(s)
			require.NoError(t, err)
			kinds := findingKinds(findings)
			assert.Equal(t, HistoryWrongHashObject, kinds[wrongObject])
			assert.Equal(t, HistoryDanglingEntry, kinds[wrongEntry])
			assert.Equal(t, HistoryMalformedEntry, kinds[malformed])
			requireAbsent(t, s, wrongObject)
			requireAbsent(t, s, wrongEntry)
			requireAbsent(t, s, malformed)
			requirePresent(t, s, orphan)
			got, err := s.ReadVersion(name, FormatVersionStamp(historyStampA))
			require.NoError(t, err)
			assert.Equal(t, good, got)

			after, err := CheckHistory(s)
			require.NoError(t, err)
			require.Len(t, after, 1)
			assert.Equal(t, HistoryOrphanObject, after[0].Kind)
		})
	}
}

// TestRepairReportsDanglingPointer verifies AC-11: a pointer left naming a
// dropped entry is reported and never retargeted.
func TestRepairReportsDanglingPointer(t *testing.T) {
	dir := t.TempDir()
	s := newTreeStorage(t, dir)
	data := []byte("active\n")
	stamp, _, err := EnsureActiveVersion(s, "router.conf", data, historyStampA)
	require.NoError(t, err)
	_, object := historyKeys(historyStampA, data)
	require.NoError(t, s.RemoveKey(object))

	findings, err := RepairHistory(s)
	require.NoError(t, err)
	kinds := findingKinds(findings)
	assert.Equal(t, HistoryDanglingPointer, kinds["meta/config/router.conf/active"])
	pointed, ok, err := readPointer(s, "router.conf", pointerActive)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, stamp, pointed)
}

// TestCheckOnRepairedOutputStillReportsPointer verifies AC-11: checking the
// repaired store reports the pointer rather than passing.
func TestCheckOnRepairedOutputStillReportsPointer(t *testing.T) {
	dir := t.TempDir()
	s := newTreeStorage(t, dir)
	data := []byte("active\n")
	_, _, err := EnsureActiveVersion(s, "router.conf", data, historyStampA)
	require.NoError(t, err)
	_, object := historyKeys(historyStampA, data)
	require.NoError(t, s.RemoveKey(object))
	_, err = RepairHistory(s)
	require.NoError(t, err)

	findings, err := CheckHistory(s)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, HistoryDanglingPointer, findings[0].Kind)
	assert.Equal(t, HistorySeverityError, findings[0].Severity)
}
