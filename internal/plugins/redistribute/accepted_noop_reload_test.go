// Design: docs/architecture/api/process-protocol.md -- accepted redistribution input.
// Related: replay_order_test.go -- unchanged reload must send nothing.
package redistribute

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	bgpredist "github.com/ze-software/ze/internal/component/bgp/redistribute"
	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// This reduced engine boundary observes actual accepted callbacks and useful
// route requests; the separate actual-SDK root owns typed/framed transport proof.
// It is synchronous, owned by this test, and never used concurrently.
type noopReloadUpdater struct {
	nopUpdater
	lookup     rpc.OriginAdmissionLookupHandler
	deliveries []rpc.OriginAdmissionLookupOutput
	issues     []sdk.Issue
}

func (u *noopReloadUpdater) OnOriginAdmissionLookup(handler rpc.OriginAdmissionLookupHandler) {
	u.lookup = handler
}

func (u *noopReloadUpdater) RaiseIssue(_ context.Context, issue sdk.Issue) error {
	u.issues = append(u.issues, issue)
	return nil
}

func (u *noopReloadUpdater) UpdateRouteRequestResult(ctx context.Context, input rpc.UpdateRouteInput) (*rpc.UpdateRouteOutput, error) {
	words := strings.Fields(input.Command)
	operation := rpc.OriginIntakeAnnounce
	var prefix string
	// Bound: the actual formatted route request contains one native prefix.
	for index := 0; index+1 < len(words); index++ {
		if words[index] == "add" {
			prefix = words[index+1]
			break
		}
		if words[index] == "del" {
			prefix = words[index+1]
			operation = rpc.OriginIntakeWithdraw
			break
		}
	}
	if prefix == "" {
		return nil, errors.New("accepted replay test received no native prefix")
	}
	destination := input.PeerSelector
	if destination == "*" {
		destination = "192.0.2.1"
	}
	query := rpc.OriginAdmissionLookupInput{AdmissionID: input.OriginAdmissionID,
		Family: "ipv4/unicast", Prefix: prefix, Destination: destination,
		PeerOwner: 91, Operation: operation, DeadlineMS: time.Now().Add(time.Second).UnixMilli()}
	output, err := u.lookup(ctx, query)
	if err != nil {
		return nil, err
	}
	u.deliveries = append(u.deliveries, *output)
	if operation == rpc.OriginIntakeWithdraw {
		return &rpc.UpdateRouteOutput{Withdrawn: 1}, nil
	}
	return &rpc.UpdateRouteOutput{Announced: 1}, nil
}

// The real reload/held/replay path must leave existing peers quiet and still
// advertise unchanged accepted routes to a later peer. Original admission IDs
// and snapshots remain immutable; stale changed versions and removed rules
// cannot replace them, while an independently permitted route stays useful.
// MUTATION: compare targeted replay evaluator generation as admission identity.
func TestAcceptedNoopReloadKeepsLatePeerReplay(t *testing.T) {
	emit := reloadRig(t)
	updater := &noopReloadUpdater{}
	consumer := bgpredist.NewBGPConsumer(updater)
	require.NoError(t, configredist.RegisterConsumer(consumer))
	state := installRules(bgpRules(configredist.ImportRule{}))
	emit()
	require.Len(t, updater.deliveries, 2)
	originals := append([]rpc.OriginAdmissionLookupOutput(nil), updater.deliveries...)
	_, originalGeneration := consumerHeld.evaluator()
	reload(t, state, bgpRules(configredist.ImportRule{}))
	_, replayGeneration := consumerHeld.evaluator()
	require.Greater(t, replayGeneration, originalGeneration)
	require.Len(t, updater.deliveries, 2, "no unchanged route is resent to existing peers")
	_, fired := getReplayCoordinator().onPeerUp(getEventBus(), "192.0.2.7")
	require.True(t, fired)
	require.Len(t, updater.deliveries, 4, "the late peer receives both useful routes")
	require.Empty(t, updater.issues)
	for index, original := range originals {
		replayed := updater.deliveries[index+2]
		require.Equal(t, "192.0.2.7", replayed.Destination)
		require.Equal(t, original.AdmissionID, replayed.AdmissionID)
		require.Equal(t, original.Snapshot, replayed.Snapshot, "replay keeps original admission generation")
	}

	reload(t, state, bgpRules(configredist.ImportRule{Final: "accept", Chain: metricChain(70)}))
	require.Len(t, updater.deliveries, 6, "changed transforms replace both original records")
	changed := append([]rpc.OriginAdmissionLookupOutput(nil), updater.deliveries[4:6]...)
	for index, original := range originals {
		require.NotEqual(t, original.AdmissionID, changed[index].AdmissionID)
		require.Equal(t, uint32(70), changed[index].Snapshot.Metric)
		query := rpc.OriginAdmissionLookupInput{AdmissionID: original.AdmissionID,
			Family: original.Family, Prefix: original.Prefix, Destination: "192.0.2.7",
			PeerOwner: 91, Operation: rpc.OriginIntakeAnnounce, DeadlineMS: time.Now().Add(time.Second).UnixMilli()}
		_, err := updater.lookup(t.Context(), query)
		require.Error(t, err, "an old admission cannot borrow its changed replacement")
	}
	id, known := redistevents.ProtocolIDOf(reloadSource)
	require.True(t, known)
	// Both a stale changed build and an older generation of the current build
	// are refused before any route request; neither may overwrite the record.
	stale := configredist.RouteEntry{Prefix: prefixTag42, Protocol: id,
		Tag: 42, Peer: "192.0.2.8", AdmissionGeneration: originalGeneration}
	consumer.InjectRoute(t.Context(), family.IPv4Unicast, stale)
	stale.Metric, stale.HasMetric = 70, true
	consumer.InjectRoute(t.Context(), family.IPv4Unicast, stale)
	_, stale.AdmissionGeneration = consumerHeld.evaluator()
	stale.SourceMetric = 1
	consumer.InjectRoute(t.Context(), family.IPv4Unicast, stale)
	require.Len(t, updater.deliveries, 6)
	require.Len(t, updater.issues, 3)
	_, fired = getReplayCoordinator().onPeerUp(getEventBus(), "192.0.2.8")
	require.True(t, fired)
	require.Len(t, updater.deliveries, 8, "both current changed contributions still replay")

	// Narrowing removes only tag7. The independently permitted tag42 route
	// remains available on the next peer-up even though that reload advanced.
	reload(t, state, bgpRules(configredist.ImportRule{Final: "accept", Chain: tagChain(42)}))
	// The surviving transform deliberately changes back to no MED; inspect
	// the real current record rather than retaining stale transformed output.
	before := len(updater.deliveries)
	_, fired = getReplayCoordinator().onPeerUp(getEventBus(), "192.0.2.9")
	require.True(t, fired)
	require.Len(t, updater.deliveries, before+1)
	survivor := updater.deliveries[before]
	require.Equal(t, prefixTag42, survivor.Prefix)
	require.Equal(t, "192.0.2.9", survivor.Destination)
	require.False(t, survivor.Snapshot.HasMetric)
	require.Len(t, updater.issues, 3)
	for _, previous := range changed {
		if previous.Prefix != prefixTag7 {
			continue
		}
		query := rpc.OriginAdmissionLookupInput{AdmissionID: previous.AdmissionID,
			Family: previous.Family, Prefix: previous.Prefix, Destination: "192.0.2.9",
			PeerOwner: 91, Operation: rpc.OriginIntakeAnnounce, DeadlineMS: time.Now().Add(time.Second).UnixMilli()}
		_, err := updater.lookup(t.Context(), query)
		require.Error(t, err, "the removed rule cannot authorize its old source")
	}

	reload(t, state, nil)
	before = len(updater.deliveries)
	_, fired = getReplayCoordinator().onPeerUp(getEventBus(), "192.0.2.10")
	require.False(t, fired, "removing the import rule opens no BGP replay")
	require.Len(t, updater.deliveries, before)
	query := rpc.OriginAdmissionLookupInput{AdmissionID: survivor.AdmissionID,
		Family: survivor.Family, Prefix: survivor.Prefix, Destination: "192.0.2.10",
		PeerOwner: 91, Operation: rpc.OriginIntakeAnnounce, DeadlineMS: time.Now().Add(time.Second).UnixMilli()}
	_, err := updater.lookup(t.Context(), query)
	require.Error(t, err, "the removed import cannot authorize its retired record")
	require.Len(t, updater.issues, 3)
}
