package redistributeegress

import (
	"context"
	"net/netip"
	"testing"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// taggedAddBatch builds a one-entry add batch carrying tag.
func taggedAddBatch(p redistevents.ProtocolID, prefix string, tag uint32) *redistevents.RouteChangeBatch {
	return &redistevents.RouteChangeBatch{
		Protocol: p,
		AFI:      afiIPv4,
		SAFI:     safiUnicst,
		Entries: []redistevents.RouteChangeEntry{{
			Action: redistevents.ActionAdd,
			Prefix: netip.MustParsePrefix(prefix),
			Tag:    tag,
		}},
	}
}

// VALIDATES: spec-static-route-tag-reaches-no-consumer AC-2 -- the orchestrator
// copies RouteChangeEntry.Tag into the RouteEntry each consumer receives.
// PREVENTS: the tag reaching the bus and being dropped one hop before a consumer,
// which looks exactly like a producer that never set it.
func TestHandleBatchCarriesEntryTag(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("tagsource")
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "tagsource", Protocol: "tagsource"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "tagsource", Destination: "bgp"},
	}))

	consumer := registerBGPConsumer(t)
	handleBatch(context.Background(), skipIDs(bgpID), taggedAddBatch(id, "10.0.0.0/8", 7788))

	inj := consumer.snapshotInjected()
	require.Len(t, inj, 1)
	assert.Equal(t, uint32(7788), inj[0].entry.Tag)
}

// VALIDATES: spec-static-route-tag-reaches-no-consumer AC-8 -- an import rule naming a
// tag filters ENTRY BY ENTRY, because the tag belongs to the route and not to the batch.
// PREVENTS: a whole batch being accepted or rejected on the tag of no particular route,
// which is what a batch-level decision does when the entries disagree.
func TestHandleBatchFiltersEntriesByTag(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("tagsource")
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "tagsource", Protocol: "tagsource"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "tagsource", Destination: "bgp", MatchTag: true, Tag: 42},
	}))

	batch := &redistevents.RouteChangeBatch{
		Protocol: id,
		AFI:      afiIPv4,
		SAFI:     safiUnicst,
		Entries: []redistevents.RouteChangeEntry{
			{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("10.1.0.0/16"), Tag: 42},
			{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("10.2.0.0/16"), Tag: 43},
			{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("10.3.0.0/16")},
		},
	}

	consumer := registerBGPConsumer(t)
	handleBatch(context.Background(), skipIDs(bgpID), batch)

	inj := consumer.snapshotInjected()
	require.Len(t, inj, 1, "only the route carrying tag 42 is imported")
	assert.Equal(t, "10.1.0.0/16", inj[0].entry.Prefix)
}

// VALIDATES: spec-static-route-tag-reaches-no-consumer AC-2 -- an entry with no tag
// reaches the consumer as zero, which every consumer reads as "no tag".
// PREVENTS: an uninitialised or leaked pooled value arriving as a route tag.
func TestHandleBatchUntaggedEntryCarriesZero(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("tagsource")
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "tagsource", Protocol: "tagsource"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "tagsource", Destination: "bgp"},
	}))

	consumer := registerBGPConsumer(t)
	handleBatch(context.Background(), skipIDs(bgpID), addBatch(id, afiIPv4, "10.0.0.0/8", ""))

	inj := consumer.snapshotInjected()
	require.Len(t, inj, 1)
	assert.Equal(t, uint32(0), inj[0].entry.Tag)
	assert.Equal(t, family.IPv4Unicast, inj[0].fam)
}
