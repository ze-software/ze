// VALIDATES: Tag registry tracks on-demand announcements by key+value, supports withdraw by tag/id/all.
// PREVENTS: Orphaned announcements surviving unnoticed; stale routes after attack ends.

package announce

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

type withdrawRecorder struct {
	mu    sync.Mutex
	calls []withdrawCall
}

type withdrawCall struct {
	sel    *selector.Selector
	batch  types.NLRIBatch
	sender plugin.Sender
}

func (w *withdrawRecorder) record(sel *selector.Selector, batch types.NLRIBatch, sender plugin.Sender) error {
	w.mu.Lock()
	w.calls = append(w.calls, withdrawCall{sel: sel, batch: batch, sender: sender})
	w.mu.Unlock()
	return nil
}

func (w *withdrawRecorder) count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.calls)
}

func newTestRegistry() (*Registry, *withdrawRecorder) {
	rec := &withdrawRecorder{}
	r := NewRegistry(rec.record)
	r.nowFunc = func() time.Time { return time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC) }
	return r, rec
}

func testBatch(fam family.Family) types.NLRIBatch {
	return types.NLRIBatch{Family: fam}
}

func testSel(s string) *selector.Selector {
	return selector.ParseDefault(s)
}

func mustAnnounce(t *testing.T, r *Registry, key, value, sel string, fam family.Family, source string, dur time.Duration) uint64 {
	t.Helper()
	id, err := r.Announce(key, value, testSel(sel), testBatch(fam), source, plugin.OperatorSender(), dur)
	require.NoError(t, err)
	return id
}

func TestRegistryAnnounceCreatesEntry(t *testing.T) {
	r, _ := newTestRegistry()

	id, err := r.Announce("mitigation", "ddos-udp", testSel("upstream"), testBatch(family.IPv4Unicast), "cli", plugin.OperatorSender(), 0)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), id)
	assert.Equal(t, 1, r.Len())

	entries := r.List(listFilter{})
	require.Len(t, entries, 1)
	assert.Equal(t, "mitigation", entries[0].TagKey)
	assert.Equal(t, "ddos-udp", entries[0].TagValue)
	assert.Equal(t, "cli", entries[0].Source)
	assert.Nil(t, entries[0].ExpiresAt)
}

func TestRegistryAnnounceIDsIncrement(t *testing.T) {
	r, _ := newTestRegistry()

	id1 := mustAnnounce(t, r, "a", "1", "*", family.IPv4Unicast, "cli", 0)
	id2 := mustAnnounce(t, r, "a", "2", "*", family.IPv4Unicast, "cli", 0)
	id3 := mustAnnounce(t, r, "b", "1", "*", family.IPv4Unicast, "cli", 0)

	assert.Equal(t, uint64(1), id1)
	assert.Equal(t, uint64(2), id2)
	assert.Equal(t, uint64(3), id3)
	assert.Equal(t, 3, r.Len())
}

func TestRegistrywithdrawTag(t *testing.T) {
	r, rec := newTestRegistry()

	mustAnnounce(t, r, "mitigation", "ddos-udp", "upstream", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "mitigation", "ddos-tcp", "upstream", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "mitigation", "ddos-udp", "peer-a", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "other", "x", "*", family.IPv4Unicast, "cli", 0)

	n, err := r.withdrawTag("", "mitigation", "ddos-udp")
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, 2, rec.count())
	assert.Equal(t, 2, r.Len())
}

func TestRegistrywithdrawTagKey(t *testing.T) {
	r, rec := newTestRegistry()

	mustAnnounce(t, r, "mitigation", "a", "*", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "mitigation", "b", "*", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "other", "x", "*", family.IPv4Unicast, "cli", 0)

	n, err := r.withdrawTagKey("", "mitigation")
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, 2, rec.count())
	assert.Equal(t, 1, r.Len())
}

// TestRegistrywithdrawAllTags drives `withdraw tag *`, which withdraws every
// tagged announcement.
//
// It walks the same entries TestRegistrywithdrawAll does. That is the point
// rather than a duplicate. Only a tagged announcement enters the registry, so
// the two verbs name one set and one function answers both.
func TestRegistrywithdrawAllTags(t *testing.T) {
	r, rec := newTestRegistry()

	mustAnnounce(t, r, "mitigation", "a", "*", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "other", "b", "*", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "third", "c", "*", family.IPv4Unicast, "cli", 0)

	n, err := r.withdrawAll("")
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, 3, rec.count())
	assert.Equal(t, 0, r.Len())
}

func TestRegistrywithdrawEntryByID(t *testing.T) {
	r, rec := newTestRegistry()

	id1 := mustAnnounce(t, r, "a", "1", "*", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "a", "2", "*", family.IPv4Unicast, "cli", 0)

	// The error return came back when the send permission made a withdraw
	// refusable. It was dropped as an always-nil value to satisfy unparam, which
	// was true of the reactor of the day and is not true of this one.
	found, err := r.withdrawEntryByID("", id1)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 1, rec.count())
	assert.Equal(t, 1, r.Len())

	found, err = r.withdrawEntryByID("", 999)
	require.NoError(t, err, "an id that was never tracked is not-found, not a failed withdraw")
	assert.False(t, found)
}

func TestRegistrywithdrawAll(t *testing.T) {
	r, rec := newTestRegistry()

	mustAnnounce(t, r, "a", "1", "upstream", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "b", "2", "peer-a", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "c", "3", "upstream", family.IPv4Unicast, "cli", 0)

	n, err := r.withdrawAll("")
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, 3, rec.count())
	assert.Equal(t, 0, r.Len())
}

// TestRegistrywithdrawAllWithSelector proves the peer filter. The value is the
// selector each announcement was MADE with, so naming one fan-out leaves the
// announcements of every other fan-out in place.
func TestRegistrywithdrawAllWithSelector(t *testing.T) {
	r, rec := newTestRegistry()

	mustAnnounce(t, r, "a", "1", "upstream", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "b", "2", "peer-a", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "c", "3", "upstream", family.IPv4Unicast, "cli", 0)

	n, err := r.withdrawAll("upstream")
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, 2, rec.count())
	assert.Equal(t, 1, r.Len())
}

func TestRegistryList(t *testing.T) {
	r, _ := newTestRegistry()

	mustAnnounce(t, r, "mitigation", "a", "upstream", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "mitigation", "b", "peer-a", family.IPv6Unicast, "plugin", 0)
	mustAnnounce(t, r, "other", "c", "upstream", family.IPv4Unicast, "cli", 0)

	all := r.List(listFilter{})
	assert.Len(t, all, 3)

	byKey := r.List(listFilter{TagKey: "mitigation"})
	assert.Len(t, byKey, 2)

	byKeyValue := r.List(listFilter{TagKey: "mitigation", TagValue: "a"})
	assert.Len(t, byKeyValue, 1)

	byFamily := r.List(listFilter{Family: family.IPv6Unicast.String()})
	assert.Len(t, byFamily, 1)
}

func TestRegistryMultipleEntriesSameTag(t *testing.T) {
	r, _ := newTestRegistry()

	mustAnnounce(t, r, "mitigation", "ddos", "upstream", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "mitigation", "ddos", "upstream", family.IPv6Unicast, "cli", 0)

	entries := r.List(listFilter{TagKey: "mitigation", TagValue: "ddos"})
	assert.Len(t, entries, 2)
}

func TestRegistryDurationAutoWithdraw(t *testing.T) {
	r, rec := newTestRegistry()

	_, err := r.Announce("mitigation", "ddos", testSel("upstream"), testBatch(family.IPv4Unicast), "cli", plugin.OperatorSender(), 50*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 1, r.Len())

	entries := r.List(listFilter{})
	require.Len(t, entries, 1)
	require.NotNil(t, entries[0].ExpiresAt)

	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 0, r.Len())
	assert.Equal(t, 1, rec.count())
}

func TestRegistryDurationCancelledByExplicitWithdraw(t *testing.T) {
	r, rec := newTestRegistry()

	id := mustAnnounce(t, r, "mitigation", "ddos", "upstream", family.IPv4Unicast, "cli", 500*time.Millisecond)

	found, err := r.withdrawEntryByID("", id)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 0, r.Len())
	assert.Equal(t, 1, rec.count())

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, rec.count())
}

// TestRegistryPeerNarrowsEveryWithdrawForm proves the peer prefix reaches all
// three forms, not just the one that used to carry a selector leaf.
//
// VALIDATES: naming a fan-out withdraws only the announcements made to it, for
// tag, for tag key, and for id.
// PREVENTS: a prefix the grammar offers on three commands and one of them
// honors, which reads to an operator as a withdraw that went nowhere.
func TestRegistryPeerNarrowsEveryWithdrawForm(t *testing.T) {
	r, rec := newTestRegistry()

	mustAnnounce(t, r, "mitigation", "ddos", "upstream", family.IPv4Unicast, "cli", 0)
	mustAnnounce(t, r, "mitigation", "ddos", "peer-a", family.IPv4Unicast, "cli", 0)

	n, err := r.withdrawTag("upstream", "mitigation", "ddos")
	require.NoError(t, err)
	assert.Equal(t, 1, n, "only the announcement made to upstream is withdrawn")
	assert.Equal(t, 1, rec.count())
	assert.Equal(t, 1, r.Len())

	n, err = r.withdrawTagKey("upstream", "mitigation")
	require.NoError(t, err)
	assert.Equal(t, 0, n, "the announcement left is peer-a's, and upstream does not name it")
	assert.Equal(t, 1, r.Len())

	n, err = r.withdrawTagKey("peer-a", "mitigation")
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, 0, r.Len())
}

// TestRegistryPeerNarrowsWithdrawByID proves an id scoped to the wrong fan-out
// is reported as not found rather than withdrawn.
//
// VALIDATES: the id is looked up, then the peer filter refuses it, and the entry
// stays in the registry.
// PREVENTS: a scoped withdraw reaching an announcement the operator did not
// name, which is the failure a filter exists to stop.
func TestRegistryPeerNarrowsWithdrawByID(t *testing.T) {
	r, rec := newTestRegistry()

	id := mustAnnounce(t, r, "mitigation", "ddos", "upstream", family.IPv4Unicast, "cli", 0)

	found, err := r.withdrawEntryByID("peer-a", id)
	require.NoError(t, err)
	assert.False(t, found, "peer-a did not receive this announcement")
	assert.Equal(t, 0, rec.count(), "nothing reached the wire")
	assert.Equal(t, 1, r.Len(), "the entry is still tracked")

	found, err = r.withdrawEntryByID("upstream", id)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 1, rec.count())
	assert.Equal(t, 0, r.Len())
}
