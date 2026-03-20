package cli

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// helper: create a temp zefs store and a History backed by it.
func newTestHistory(t *testing.T) (*History, *zefs.BlobStore) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.zefs")
	store, err := zefs.Create(path)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() }) //nolint:errcheck // test cleanup
	h := NewHistory(store)
	return h, store
}

// VALIDATES: Round-trip save then load returns same entries.
// PREVENTS: Data loss on persist/reload cycle.
func TestHistoryLoadSave(t *testing.T) {
	h, store := newTestHistory(t)

	for _, cmd := range []string{"show", "set bgp local-as 65000", "commit"} {
		h.Append(cmd)
	}
	h.Save("edit")

	// Reload from store.
	h2 := NewHistory(store)
	loaded := h2.Load("edit")
	assert.Equal(t, []string{"show", "set bgp local-as 65000", "commit"}, loaded)
}

// VALIDATES: Save 150 entries with max=100, load returns last 100.
// PREVENTS: Unbounded history growth in the store.
func TestHistoryRolling(t *testing.T) {
	h, store := newTestHistory(t)

	for i := range 150 {
		h.Append("command-" + string(rune('A'+i%26)) + string(rune('0'+i/26)))
	}
	h.Save("edit")

	h2 := NewHistory(store)
	loaded := h2.Load("edit")
	assert.Len(t, loaded, 100)
	// After Save, h.Entries() is already trimmed to the newest 100.
	assert.Equal(t, h.Entries(), loaded, "should keep the newest 100 entries")
}

// VALIDATES: No meta/history/max key defaults to 100.
// PREVENTS: Crash or zero-max when config key is absent.
func TestHistoryDefaultMax(t *testing.T) {
	h, _ := newTestHistory(t)
	assert.Equal(t, 100, h.Max())
}

// VALIDATES: meta/history/max=50 trims to 50.
// PREVENTS: Custom max being ignored.
func TestHistoryCustomMax(t *testing.T) {
	_, store := newTestHistory(t)

	// Write custom max.
	require.NoError(t, store.WriteFile("meta/history/max", []byte("50"), 0))

	// Reload to pick up the new max.
	h2 := NewHistory(store)
	assert.Equal(t, 50, h2.Max())

	// Append 80 entries, save should trim to 50.
	for i := range 80 {
		h2.Append("cmd-" + string(rune('A'+i%26)) + string(rune('0'+i/26)))
	}
	h2.Save("edit")

	h3 := NewHistory(store)
	loaded := h3.Load("edit")
	assert.Len(t, loaded, 50)
}

// VALIDATES: Load from empty store returns nil slice.
// PREVENTS: Error or panic on first launch with empty store.
func TestHistoryEmpty(t *testing.T) {
	h, _ := newTestHistory(t)
	loaded := h.Load("edit")
	assert.Nil(t, loaded)
}

// VALIDATES: Edit and command histories are stored independently.
// PREVENTS: Cross-mode history contamination.
func TestHistoryPerMode(t *testing.T) {
	h, store := newTestHistory(t)

	h.Append("set bgp local-as 65000")
	h.Append("commit")
	h.Save("edit")

	// Reset entries for command mode.
	h.restore(historySnapshot{idx: -1})
	h.Append("peer list")
	h.Append("daemon status")
	h.Save("command")

	h2 := NewHistory(store)
	assert.Equal(t, []string{"set bgp local-as 65000", "commit"}, h2.Load("edit"))
	assert.Equal(t, []string{"peer list", "daemon status"}, h2.Load("command"))
}

// VALIDATES: Nil store (no zefs) returns empty on load, save is no-op.
// PREVENTS: Panic or error when running without blob storage.
func TestHistoryNilGraceful(t *testing.T) {
	h := NewHistory(nil)

	assert.Nil(t, h.Load("edit"))
	h.Append("show")
	h.Save("edit")
	// Entries exist in memory but nothing persisted.
	assert.Equal(t, []string{"show"}, h.Entries())
}

// VALIDATES: max=0 in store is clamped to 1 (boundary test).
// PREVENTS: Division by zero or empty-max edge case.
func TestHistoryBoundaryMaxZero(t *testing.T) {
	_, store := newTestHistory(t)
	require.NoError(t, store.WriteFile("meta/history/max", []byte("0"), 0))

	h2 := NewHistory(store)
	assert.Equal(t, 1, h2.Max(), "max=0 should clamp to 1")
}

// VALIDATES: max=10000 is accepted (last valid boundary).
// PREVENTS: Arbitrary upper limit rejection.
func TestHistoryBoundaryMaxLarge(t *testing.T) {
	_, store := newTestHistory(t)
	require.NoError(t, store.WriteFile("meta/history/max", []byte("10000"), 0))

	h2 := NewHistory(store)
	assert.Equal(t, 10000, h2.Max())
}

// VALIDATES: Non-numeric max value falls back to default.
// PREVENTS: Crash on corrupt meta key.
func TestHistoryBoundaryMaxInvalid(t *testing.T) {
	_, store := newTestHistory(t)
	require.NoError(t, store.WriteFile("meta/history/max", []byte("abc"), 0))

	h2 := NewHistory(store)
	assert.Equal(t, 100, h2.Max(), "invalid max should fall back to default 100")
}

// VALIDATES: Consecutive duplicate commands are not added.
// PREVENTS: History cluttered with repeated identical commands.
func TestHistoryAppendDedup(t *testing.T) {
	h := NewHistory(nil)
	h.Append("show")
	assert.True(t, h.Append("commit"), "different command should be added")
	assert.False(t, h.Append("commit"), "duplicate consecutive should be rejected")
	assert.Equal(t, []string{"show", "commit"}, h.Entries())
}

// VALIDATES: Up/Down browsing navigation works correctly.
// PREVENTS: History browsing off-by-one or state corruption.
func TestHistoryUpDown(t *testing.T) {
	h := NewHistory(nil)
	h.Append("first")
	h.Append("second")
	h.Append("third")

	// Up from current input "typing"
	val, ok := h.Up("typing")
	assert.True(t, ok)
	assert.Equal(t, "third", val)

	val, ok = h.Up("")
	assert.True(t, ok)
	assert.Equal(t, "second", val)

	// Down back
	val, ok = h.Down()
	assert.True(t, ok)
	assert.Equal(t, "third", val)

	// Down past end restores saved input
	val, ok = h.Down()
	assert.True(t, ok)
	assert.Equal(t, "typing", val)

	// Down again when not browsing
	_, ok = h.Down()
	assert.False(t, ok)
}

// VALIDATES: Up on empty history returns false.
// PREVENTS: Panic on Up with no entries.
func TestHistoryUpEmpty(t *testing.T) {
	h := NewHistory(nil)
	_, ok := h.Up("input")
	assert.False(t, ok)
}

// VALIDATES: Negative max in store is clamped to 1.
// PREVENTS: Negative max bypassing clamp.
func TestHistoryBoundaryMaxNegative(t *testing.T) {
	_, store := newTestHistory(t)
	require.NoError(t, store.WriteFile("meta/history/max", []byte("-5"), 0))

	h2 := NewHistory(store)
	assert.Equal(t, 1, h2.Max(), "negative max should clamp to 1")
}

// VALIDATES: max above ceiling is clamped to ceiling.
// PREVENTS: Unbounded max from crafted meta key.
func TestHistoryBoundaryMaxAboveCeiling(t *testing.T) {
	_, store := newTestHistory(t)
	require.NoError(t, store.WriteFile("meta/history/max", []byte("999999"), 0))

	h2 := NewHistory(store)
	assert.Equal(t, historyMaxCeiling, h2.Max(), "max above ceiling should clamp")
}

// VALIDATES: Up at top of history stays at oldest entry.
// PREVENTS: Off-by-one when repeatedly pressing Up at boundary.
func TestHistoryUpAtTop(t *testing.T) {
	h := NewHistory(nil)
	h.Append("a")
	h.Append("b")
	h.Append("c")

	h.Up("x")           // -> c
	h.Up("")            // -> b
	val, ok := h.Up("") // -> a
	assert.True(t, ok)
	assert.Equal(t, "a", val)

	// One more Up stays at a.
	val, ok = h.Up("")
	assert.True(t, ok)
	assert.Equal(t, "a", val, "should stay at oldest entry")
}

// VALIDATES: Single-entry history navigation.
// PREVENTS: Edge case with only one history entry.
func TestHistoryUpDownSingleEntry(t *testing.T) {
	h := NewHistory(nil)
	h.Append("only")

	val, ok := h.Up("current")
	assert.True(t, ok)
	assert.Equal(t, "only", val)

	// Up again stays at the only entry.
	val, ok = h.Up("")
	assert.True(t, ok)
	assert.Equal(t, "only", val)

	// Down restores saved input.
	val, ok = h.Down()
	assert.True(t, ok)
	assert.Equal(t, "current", val)
}

// VALIDATES: Empty string is rejected by Append.
// PREVENTS: Empty entries polluting history.
func TestHistoryAppendEmpty(t *testing.T) {
	h := NewHistory(nil)
	assert.False(t, h.Append(""), "empty command should be rejected")
	assert.Nil(t, h.Entries())
}

// VALIDATES: Newlines in commands are replaced with spaces.
// PREVENTS: Newline injection breaking line-delimited storage.
func TestHistoryAppendNewlineReplaced(t *testing.T) {
	h, store := newTestHistory(t)
	h.Append("foo\nbar")
	h.Save("edit")

	h2 := NewHistory(store)
	loaded := h2.Load("edit")
	assert.Equal(t, []string{"foo bar"}, loaded)
}

// VALIDATES: ResetBrowsing clears browsing state.
// PREVENTS: Stale browse position after typing.
func TestHistoryResetBrowsing(t *testing.T) {
	h := NewHistory(nil)
	h.Append("a")
	h.Append("b")

	// Start browsing.
	h.Up("x")
	h.Up("")

	// Reset (simulates user typing).
	h.ResetBrowsing()

	// Next Up should start fresh from most recent.
	val, ok := h.Up("y")
	assert.True(t, ok)
	assert.Equal(t, "b", val, "should start fresh from most recent after reset")
}

// VALIDATES: Snapshot isolation after append.
// PREVENTS: Shared backing array corruption between modes.
func TestHistorySnapshotIsolation(t *testing.T) {
	h := NewHistory(nil)
	h.Append("a")
	h.Append("b")

	snap := h.snapshot()
	h.Append("c")

	// Snapshot should still have only a, b.
	assert.Equal(t, []string{"a", "b"}, snap.entries)
	// Live history has a, b, c.
	assert.Equal(t, []string{"a", "b", "c"}, h.Entries())
}

// VALIDATES: Restore from zero-value snapshot sets idx to -1.
// PREVENTS: Spurious Down() result after first mode switch.
func TestHistoryRestoreZeroValue(t *testing.T) {
	h := NewHistory(nil)
	h.restore(historySnapshot{}) // zero-value: entries=nil, idx=0

	// Should not be browsing (idx corrected to -1).
	_, ok := h.Down()
	assert.False(t, ok, "should not be browsing after restore from zero-value snapshot")
}

// VALIDATES: Load trims to max on load.
// PREVENTS: Unbounded memory from crafted blob.
func TestHistoryLoadTrimsToMax(t *testing.T) {
	_, store := newTestHistory(t)

	// Write custom max of 5.
	require.NoError(t, store.WriteFile("meta/history/max", []byte("5"), 0))

	// Manually write 20 entries to the store.
	entries := "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\no\np\nq\nr\ns\nt"
	require.NoError(t, store.WriteFile("meta/history/edit", []byte(entries), 0))

	h := NewHistory(store)
	loaded := h.Load("edit")
	assert.Len(t, loaded, 5)
	assert.Equal(t, []string{"p", "q", "r", "s", "t"}, loaded, "should keep newest 5")
}

// VALIDATES: Load with only-newline content returns nil.
// PREVENTS: Phantom empty entries from whitespace-only store data.
func TestHistoryLoadOnlyNewlines(t *testing.T) {
	_, store := newTestHistory(t)
	require.NoError(t, store.WriteFile("meta/history/edit", []byte("\n\n\n"), 0))

	h := NewHistory(store)
	loaded := h.Load("edit")
	assert.Nil(t, loaded)
}
