// Design: docs/architecture/api/process-protocol.md -- accepted redistribution input.
package redistribute

import (
	"context"
	"github.com/stretchr/testify/require"
	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	"net/netip"
	"testing"
)

type acceptedPreferenceConsumer struct {
	stubConsumer
	removed []configredist.RouteRemoval
}

func (c *acceptedPreferenceConsumer) WithdrawRoute(_ context.Context, _ family.Family, removal configredist.RouteRemoval) {
	c.removed = append(c.removed, removal)
}

// The actual dispatch boundary preserves original metric7 before MED99 is set,
// including explicit zero override, original instance/table and evaluator zero.
// Its exact removal has the same source identity and leaves the caller intact.
// MUTATION: capture entry.Metric after applySets instead of produced.Metric.
func TestAcceptedDispatchKeepsOriginalPreferenceAndRemoval(t *testing.T) {
	t.Cleanup(redistevents.PreserveForTest())
	protocol := redistevents.RegisterProtocol("static")
	entry := redistevents.RouteChangeEntry{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("10.9.0.0/24"), Metric: 7, SourceInstance: 11, Table: 90, HasDistanceOverride: true, DistanceOverride: 0}
	view := redistRoute("static", protocol, family.IPv4Unicast, &entry)
	consumer := &acceptedPreferenceConsumer{}
	dispatchEntryToConsumer(t.Context(), consumer, family.IPv4Unicast, "static", "", 0, nil, &entry, &view, ribpolicy.Sets{HasMetric: true, Metric: 99}, 0)
	received := consumer.snapshotInjected()
	require.Len(t, received, 1)
	accepted := received[0].entry
	require.Equal(t, uint32(7), accepted.SourceMetric)
	require.Equal(t, uint32(99), accepted.Metric)
	require.True(t, accepted.HasMetric)
	require.Equal(t, uint32(11), accepted.SourceInstance)
	require.Equal(t, uint32(90), accepted.Table)
	require.Equal(t, protocol, accepted.Protocol)
	require.True(t, accepted.HasDistanceOverride)
	require.Zero(t, accepted.DistanceOverride)
	require.Zero(t, accepted.AdmissionGeneration)
	require.Equal(t, uint32(7), entry.Metric)
	entry.Action = redistevents.ActionRemove
	dispatchEntryToConsumer(t.Context(), consumer, family.IPv4Unicast, "static", "", 0, nil, &entry, nil, ribpolicy.Sets{}, 0)
	require.Equal(t, []configredist.RouteRemoval{{Prefix: "10.9.0.0/24", Source: "static", Protocol: protocol, SourceInstance: 11, Table: 90}}, consumer.removed)
}
