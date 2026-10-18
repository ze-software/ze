// Design: docs/architecture/plugin/rib-storage-design.md -- RouteEntry.ReceivedLLGRStale
// Related: rib_structured.go -- insertLabeled, receivedLLGRStaleFromMeta
// Related: rib.go -- insertPoolNLRIs, the JSON event sibling

package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
)

// staleInsertAttrs is ORIGIN IGP and an empty AS_PATH, enough for a stored route.
var staleInsertAttrs = []byte{
	0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
	0x40, 0x02, 0x00, // AS_PATH empty
}

// TestInsertLabeledWritesReceivedStale verifies the unparsed labeled insert
// stores the received LLGR_STALE verdict, and a later fresh UPDATE clears it.
//
// VALIDATES: insertLabeled writes receivedStale on the stored route, as
// insertLabeledEntry and the unlabeled branches do.
// PREVENTS: a labeled route whose attributes took the unparsed branch competing
// as fresh after the ingress step marked it LLGR-stale.
func TestInsertLabeledWritesReceivedStale(t *testing.T) {
	r := newTestRIBManager(t)
	peerRIB := storage.NewPeerRIB("192.0.2.11")
	wire := labeledNLRI(0, false, netip.MustParsePrefix("10.0.0.0/8"), []uint32{100})
	cidr := []byte{8, 10}

	var affected []affectedPrefix
	r.insertLabeled(peerRIB, labeledFamily, staleInsertAttrs, wire, false, true, &affected)
	entry, ok := peerRIB.Lookup(labeledFamily, cidr)
	require.True(t, ok, "the labeled route is stored")
	assert.True(t, entry.ReceivedLLGRStale, "the stale verdict is stored")

	r.insertLabeled(peerRIB, labeledFamily, staleInsertAttrs, wire, false, false, &affected)
	entry, ok = peerRIB.Lookup(labeledFamily, cidr)
	require.True(t, ok)
	assert.False(t, entry.ReceivedLLGRStale, "the latest UPDATE was fresh")
}

// TestInsertPoolNLRIsWritesReceivedStale verifies the JSON event path stores the
// verdict on an unlabeled route.
//
// VALIDATES: insertPoolNLRIs writes receivedStale on every stored route.
// PREVENTS: an out-of-process RIB that reads route-meta treating a received
// LLGR_STALE route as fresh.
func TestInsertPoolNLRIsWritesReceivedStale(t *testing.T) {
	r := newTestRIBManager(t)
	peerRIB := storage.NewPeerRIB("192.0.2.12")
	nlri := []byte{8, 10}

	r.insertPoolNLRIs(peerRIB, family.IPv4Unicast, nlri, staleInsertAttrs, false, 1, true)
	entry, ok := peerRIB.Lookup(family.IPv4Unicast, nlri)
	require.True(t, ok)
	assert.True(t, entry.ReceivedLLGRStale)
}

// TestReceivedLLGRStaleFromMetaJSONNumber verifies the verdict is read from a
// JSON event's route-meta, where the level decodes as a float64.
//
// VALIDATES: both the structured uint8 and the JSON float64 forms.
// PREVENTS: the JSON path reading every received LLGR_STALE route as fresh
// because its type assertion to uint8 fails.
func TestReceivedLLGRStaleFromMetaJSONNumber(t *testing.T) {
	assert.True(t, receivedLLGRStaleFromMeta(map[string]any{"stale": uint8(2)}))
	assert.True(t, receivedLLGRStaleFromMeta(map[string]any{"stale": float64(2)}))
	assert.False(t, receivedLLGRStaleFromMeta(map[string]any{"stale": float64(1)}))
	assert.False(t, receivedLLGRStaleFromMeta(map[string]any{"stale": "2"}))
	assert.False(t, receivedLLGRStaleFromMeta(nil))
}
