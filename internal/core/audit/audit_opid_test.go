package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: AC-1 -- audit.Entry.OpID round-trips through JSON-lines persistence and omits empty op-id.
// PREVENTS: audit records losing op-id across daemon restarts or emitting empty op-id keys.
func TestAuditOpIDPersistenceRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	log, err := Open(path, MinMaxEntries)
	require.NoError(t, err)

	ts1 := time.Unix(500, 0).UTC()
	ts2 := time.Unix(501, 0).UTC()
	require.NoError(t, log.Record(Entry{
		Timestamp: ts1,
		OpID:      "op-42",
		Actor:     "alice",
		Surface:   CLI,
		Action:    ActionConfigCommit,
		Outcome:   OutcomeSuccess,
	}))
	require.NoError(t, log.Record(Entry{
		Timestamp: ts2,
		Actor:     "bob",
		Surface:   CLI,
		Action:    ActionDaemonReload,
		Outcome:   OutcomeSuccess,
	}))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], `"operation-id":"op-42"`)
	assert.NotContains(t, lines[1], `"operation-id"`)

	reopened, err := Open(path, MinMaxEntries)
	require.NoError(t, err)
	entries := reopened.Query(Filter{})
	require.Len(t, entries, 2)
	assert.Equal(t, "op-42", entries[0].OpID)
	assert.Empty(t, entries[1].OpID)
}

// VALIDATES: AC-1 -- Open loads legacy JSON-lines audit files written without an op-id field.
// PREVENTS: upgrades failing to open existing audit logs on disk.
func TestAuditOpenLegacyWithoutOpID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-audit.jsonl")
	legacyLine := `{"timestamp":"2026-11-27T12:00:00Z","actor":"admin","surface":"ssh","action":"config-commit","outcome":"success"}` + "\n"
	require.NoError(t, os.WriteFile(path, []byte(legacyLine), 0o600))

	log, err := Open(path, MinMaxEntries)
	require.NoError(t, err)
	entries := log.Query(Filter{})
	require.Len(t, entries, 1)
	assert.Equal(t, "admin", entries[0].Actor)
	assert.Empty(t, entries[0].OpID)
}

// VALIDATES: AC-1 -- Filter.OpID filters entries accurately and composes with other filter fields.
// PREVENTS: show audit | op <id> returning unrelated records or dropping matching ones.
func TestAuditQueryFilterOpID(t *testing.T) {
	log, err := NewMemory(MinMaxEntries)
	require.NoError(t, err)

	base := time.Unix(600, 0).UTC()
	require.NoError(t, log.Record(Entry{Timestamp: base, OpID: "op-1", Actor: "alice", Surface: CLI, Action: ActionConfigCommit, Outcome: OutcomeSuccess}))
	require.NoError(t, log.Record(Entry{Timestamp: base.Add(time.Second), OpID: "op-2", Actor: "bob", Surface: CLI, Action: ActionDaemonReload, Outcome: OutcomeSuccess}))
	require.NoError(t, log.Record(Entry{Timestamp: base.Add(2 * time.Second), OpID: "op-1", Actor: "system", Surface: System, Action: ActionDaemonReload, Outcome: OutcomeSuccess}))
	require.NoError(t, log.Record(Entry{Timestamp: base.Add(3 * time.Second), Actor: "carol", Surface: Web, Action: ActionAuthFail, Outcome: OutcomeDenied}))

	op1 := log.Query(Filter{OpID: "op-1"})
	require.Len(t, op1, 2)
	assert.Equal(t, ActionConfigCommit, op1[0].Action)
	assert.Equal(t, ActionDaemonReload, op1[1].Action)

	op1Reload := log.Query(Filter{OpID: "op-1", Action: ActionDaemonReload})
	require.Len(t, op1Reload, 1)
	assert.Equal(t, "system", op1Reload[0].Actor)

	none := log.Query(Filter{OpID: "op-999"})
	assert.Empty(t, none)

	all := log.Query(Filter{})
	assert.Len(t, all, 4)
}
