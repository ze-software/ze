package audit

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: AC-9 -- config commit records include timestamp, actor, surface, action, detail, and outcome.
// PREVENTS: successful config commits disappearing from the unified audit trail.
func TestAuditRecordConfigCommit(t *testing.T) {
	log, err := NewMemory(100)
	require.NoError(t, err)
	ts := time.Unix(100, 0).UTC()

	err = log.Record(Entry{
		Timestamp:  ts,
		Actor:      "alice",
		RemoteAddr: "192.0.2.1:12345",
		Surface:    Web,
		Action:     ActionConfigCommit,
		Detail:     "bgp.router-id changed",
		Outcome:    OutcomeSuccess,
	})
	require.NoError(t, err)

	entries := log.Query(Filter{Action: ActionConfigCommit})
	require.Len(t, entries, 1)
	assert.Equal(t, ts, entries[0].Timestamp)
	assert.Equal(t, "alice", entries[0].Actor)
	assert.Equal(t, "192.0.2.1:12345", entries[0].RemoteAddr)
	assert.Equal(t, Web, entries[0].Surface)
	assert.Equal(t, ActionConfigCommit, entries[0].Action)
	assert.Equal(t, "bgp.router-id changed", entries[0].Detail)
	assert.Equal(t, OutcomeSuccess, entries[0].Outcome)
}

// VALIDATES: AC-10 -- config discard/rollback records are queryable by action.
// PREVENTS: discarded candidate changes losing actor attribution.
func TestAuditRecordConfigDiscard(t *testing.T) {
	log, err := NewMemory(100)
	require.NoError(t, err)

	require.NoError(t, log.Record(Entry{Timestamp: time.Unix(101, 0).UTC(), Actor: "bob", Surface: REST, Action: ActionConfigDiscard, Outcome: OutcomeSuccess}))

	entries := log.Query(Filter{Action: ActionConfigDiscard})
	require.Len(t, entries, 1)
	assert.Equal(t, "bob", entries[0].Actor)
	assert.Equal(t, REST, entries[0].Surface)
	assert.Equal(t, ActionConfigDiscard, entries[0].Action)
}

// VALIDATES: AC-11 -- daemon reload records are queryable by action and surface.
// PREVENTS: lifecycle operations bypassing the unified audit trail.
func TestAuditRecordDaemonReload(t *testing.T) {
	log, err := NewMemory(100)
	require.NoError(t, err)

	require.NoError(t, log.Record(Entry{Timestamp: time.Unix(102, 0).UTC(), Actor: "carol", Surface: CLI, Action: ActionDaemonReload, Outcome: OutcomeSuccess}))

	entries := log.Query(Filter{Action: ActionDaemonReload})
	require.Len(t, entries, 1)
	assert.Equal(t, "carol", entries[0].Actor)
	assert.Equal(t, CLI, entries[0].Surface)
}

// VALIDATES: AC-13 -- audit queries filter by inclusive time range and action.
// PREVENTS: show audit returning unrelated records outside the requested window.
func TestAuditQueryTimeRange(t *testing.T) {
	log, err := NewMemory(100)
	require.NoError(t, err)
	base := time.Unix(200, 0).UTC()

	require.NoError(t, log.Record(Entry{Timestamp: base.Add(-time.Second), Actor: "old", Surface: CLI, Action: ActionConfigCommit, Outcome: OutcomeSuccess}))
	require.NoError(t, log.Record(Entry{Timestamp: base, Actor: "inside", Surface: Web, Action: ActionConfigCommit, Outcome: OutcomeSuccess}))
	require.NoError(t, log.Record(Entry{Timestamp: base.Add(time.Second), Actor: "other-action", Surface: Web, Action: ActionAuthFail, Outcome: OutcomeDenied}))

	entries := log.Query(Filter{Since: base, Until: base.Add(time.Second), Action: ActionConfigCommit})
	require.Len(t, entries, 1)
	assert.Equal(t, "inside", entries[0].Actor)
}

// VALIDATES: AC-14 -- audit records persist across log reopen.
// PREVENTS: daemon restart losing the local audit trail.
func TestAuditPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	log, err := Open(path, 100)
	require.NoError(t, err)
	timestamp := time.Unix(300, 0).UTC()
	require.NoError(t, log.Record(Entry{Timestamp: timestamp, Actor: "dana", Surface: API, Action: ActionConfigCommit, Outcome: OutcomeSuccess}))

	reopened, err := Open(path, 100)
	require.NoError(t, err)
	entries := reopened.Query(Filter{})
	require.Len(t, entries, 1)
	assert.Equal(t, timestamp, entries[0].Timestamp)
	assert.Equal(t, "dana", entries[0].Actor)
}

// VALIDATES: AC-16 -- failed authentication attempts include attempted username, source IP, surface, action, and denied outcome.
// PREVENTS: brute-force or credential failures being invisible to operators.
func TestAuditAuthFailRecord(t *testing.T) {
	log, err := NewMemory(100)
	require.NoError(t, err)

	require.NoError(t, log.Record(Entry{Timestamp: time.Unix(400, 0).UTC(), Actor: "mallory", RemoteAddr: "198.51.100.10:55000", Surface: SSH, Action: ActionAuthFail, Outcome: OutcomeDenied}))

	entries := log.Query(Filter{Action: ActionAuthFail})
	require.Len(t, entries, 1)
	assert.Equal(t, "mallory", entries[0].Actor)
	assert.Equal(t, "198.51.100.10:55000", entries[0].RemoteAddr)
	assert.Equal(t, SSH, entries[0].Surface)
	assert.Equal(t, OutcomeDenied, entries[0].Outcome)
}

// VALIDATES: Boundary tests for audit log max entries, last valid and first invalid values.
// PREVENTS: unbounded audit logs or rejecting the documented maximum.
func TestAuditMaxEntriesBoundary(t *testing.T) {
	assert.NoError(t, validateMaxEntries(100))
	assert.NoError(t, validateMaxEntries(100000))
	assert.Error(t, validateMaxEntries(99))
	assert.Error(t, validateMaxEntries(100001))
}
