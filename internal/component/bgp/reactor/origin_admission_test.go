// Design: docs/architecture/api/process-protocol.md -- accepted redistribution input.
package reactor

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func originAcceptedFixture(t *testing.T, owner plugin.SelectedOwner, lookup rpc.OriginAdmissionLookupHandler) *originSource {
	t.Helper()
	bound, err := owner.WithOriginAdmissionLookup(lookup)
	require.NoError(t, err)
	source, err := newOriginSource(originSourceInput{kind: originSourceRedistributed, protocol: redistevents.RegisterProtocol("static"), instance: 7, metric: 9})
	require.NoError(t, err)
	source.accepted = &originAcceptedBinding{owner: bound, query: rpc.OriginAdmissionLookupInput{AdmissionID: 51, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"},
		snapshot: rpc.OriginAdmissionSnapshot{Protocol: "static", SourceInstance: 7, SourceMetric: 9, PeerScope: rpc.OriginAdmissionAllPeers}}
	return source
}

func originAcceptedAnswer(input rpc.OriginAdmissionLookupInput, state rpc.OriginAdmissionRecordState, snapshot rpc.OriginAdmissionSnapshot) *rpc.OriginAdmissionLookupOutput {
	return &rpc.OriginAdmissionLookupOutput{AdmissionID: input.AdmissionID, Family: input.Family, Prefix: input.Prefix, Destination: input.Destination, PeerOwner: input.PeerOwner, Operation: input.Operation, State: state, Snapshot: snapshot}
}

// The actual issued final-write lease reads the original accepted record.
// Retirement refuses that write while an unrelated API-origin lease stays useful.
// MUTATION: returning accepts without the independent original lookup permits it.
func TestOriginAcceptedFinalWriteRefusesRetiredRecord(t *testing.T) {
	t.Cleanup(redistevents.PreserveForTest())
	f, store, producer, local, _ := originReceiptFixture(t)
	buildInput := originBuildFixture()
	buildInput.hasPathID = false
	value, err := newOriginBuildValue(buildInput)
	require.NoError(t, err)
	state := rpc.OriginAdmissionCurrent
	snapshot := rpc.OriginAdmissionSnapshot{Protocol: "static", SourceInstance: 7, SourceMetric: 9, PeerScope: rpc.OriginAdmissionAllPeers}
	source := originAcceptedFixture(t, producer.owner, func(_ context.Context, input rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
		return originAcceptedAnswer(input, state, snapshot), nil
	})
	id, err := store.issue(t.Context(), producer, source, value, f.peer, producer.owner)
	require.NoError(t, err)
	write, err := store.admit(id, producer.owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.NoError(t, err)
	defer write.complete()
	require.NoError(t, write.current())
	apiID, err := store.issue(t.Context(), producer, local, value, f.peer, producer.owner)
	require.NoError(t, err)
	api, err := store.admit(apiID, producer.owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.NoError(t, err)
	defer api.complete()
	state = rpc.OriginAdmissionRetiring
	require.Error(t, write.current())
	require.NoError(t, api.current())
}

// Accepted-data versions are immutable: changing only a policy transform or
// original preference fact cannot authorize an old engine-owned build.
func TestOriginAcceptedSnapshotCannotBorrowReplacement(t *testing.T) {
	t.Cleanup(redistevents.PreserveForTest())
	f, _, producer, _, _ := originReceiptFixture(t)
	snapshot := rpc.OriginAdmissionSnapshot{Protocol: "static", SourceInstance: 7, SourceMetric: 9, PeerScope: rpc.OriginAdmissionAllPeers}
	source := originAcceptedFixture(t, producer.owner, func(_ context.Context, input rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
		return originAcceptedAnswer(input, rpc.OriginAdmissionCurrent, snapshot), nil
	})
	require.NoError(t, source.acceptedCurrent(t.Context(), f.peer))
	snapshot.SourceMetric = 10
	require.Error(t, source.acceptedCurrent(t.Context(), f.peer))
	snapshot.SourceMetric = 9
	snapshot.RuleGeneration = 1
	require.Error(t, source.acceptedCurrent(t.Context(), f.peer))
	snapshot.RuleGeneration = 0
	require.NoError(t, source.acceptedCurrent(t.Context(), f.peer))
}

// Stopping the accepting process removes only redistributed source data. API
// origins keep their exact original data under the existing keep/reoffer contract.
// MUTATION: park every producer source, including redistributed input.
func TestOriginAcceptedOwnerEndPreservesOnlyAPIOriginal(t *testing.T) {
	t.Cleanup(redistevents.PreserveForTest())
	f, store, producer, local, value := originReceiptFixture(t)
	source, err := newOriginSource(originSourceInput{kind: originSourceRedistributed, protocol: redistevents.RegisterProtocol("static"), instance: 7})
	require.NoError(t, err)
	redistributed, err := store.issue(t.Context(), producer, source, value, f.peer, producer.owner)
	require.NoError(t, err)
	api, err := store.issue(t.Context(), producer, local, value, f.peer, producer.owner)
	require.NoError(t, err)
	records, err := store.endOwnerWithPolicy(t.Context(), producer.owner, true)
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.NotContains(t, store.receipts, redistributed)
	var found bool
	for _, record := range store.current {
		if record.id == api {
			found = true
			require.True(t, record.retained)
		}
		require.NotEqual(t, redistributed, record.id)
	}
	require.True(t, found)
}

// The accepted adapter emits only its declared origin/transform choices. An
// extra unaccepted attribute is refused, without touching ordinary API builders.
func TestOriginAcceptedBuildMatchesOriginalWireChoices(t *testing.T) {
	input := originBuildFixture()
	input.hasPathID = false
	input.attributes = []byte{0x40, 1, 1, 2, 0x80, 4, 4, 0, 0, 0, 99}
	build, err := newOriginBuildValue(input)
	require.NoError(t, err)
	snapshot := rpc.OriginAdmissionSnapshot{HasMetric: true, Metric: 99}
	require.NoError(t, originAdmissionBuildMatches(build, snapshot))
	snapshot.Metric = 100
	require.Error(t, originAdmissionBuildMatches(build, snapshot))
	snapshot.Metric = 99
	input.attributes = append(input.attributes, 0x40, 5, 4, 0, 0, 0, 200)
	unaccepted, err := newOriginBuildValue(input)
	require.NoError(t, err)
	require.Error(t, originAdmissionBuildMatches(unaccepted, snapshot))
	input.attributes = []byte{0x40, 1, 1, 2}
	input.nextHop = bgptypes.NewNextHopSelf()
	api, err := newOriginBuildValue(input)
	require.NoError(t, err)
	require.NotNil(t, api)
}

// The engine derives redistributed kind and native source identity from a
// checked original record. Neither an operator nor a same-name successor can
// borrow that record, and changing the native key is refused by the callback.
// MUTATION: local-BGP classification discards original preference and identity.
func TestOriginAcceptedEngineDerivesOriginalSource(t *testing.T) {
	t.Cleanup(redistevents.PreserveForTest())
	f, _, original, _, _ := originReceiptFixture(t)
	protocol := redistevents.RegisterProtocol("static")
	snapshot := rpc.OriginAdmissionSnapshot{Protocol: "static", SourceInstance: 7, Table: 90, SourceMetric: 9, HasDistanceOverride: true, DistanceOverride: 0, PeerScope: rpc.OriginAdmissionAllPeers}
	owner, err := original.owner.WithOriginAdmissionLookup(func(_ context.Context, input rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
		if input.AdmissionID != 51 {
			return nil, errors.New("unknown original accepted ID")
		}
		if input.Prefix != "10.0.0.0/24" {
			return nil, errors.New("native key differs from accepted source")
		}
		return originAcceptedAnswer(input, rpc.OriginAdmissionCurrent, snapshot), nil
	})
	require.NoError(t, err)
	producer, err := newProcessOriginProducer(owner)
	require.NoError(t, err)
	input := originBuildFixture()
	input.hasPathID = false
	input.attributes = []byte{0x40, 1, 1, 2}
	build, err := newOriginBuildValue(input)
	require.NoError(t, err)
	native := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0, false)
	source, err := originSourceFromAdmission(t.Context(), producer, 51, native, build, f.peer, rpc.OriginIntakeAnnounce)
	require.NoError(t, err)
	require.Equal(t, originSourceRedistributed, source.identity.kind)
	require.Equal(t, protocol, source.identity.protocol)
	require.Equal(t, uint64(90)<<32|7, source.identity.instance)
	require.Equal(t, uint32(9), source.metric)
	require.True(t, source.hasDistance)
	require.Zero(t, source.distance)
	_, err = originSourceFromAdmission(t.Context(), producer, 52, native, build, f.peer, rpc.OriginIntakeAnnounce)
	require.Error(t, err)
	other, _ := originOwnerFixture(t, selectedWriterProcess)
	successor, err := newProcessOriginProducer(other)
	require.NoError(t, err)
	_, err = originSourceFromAdmission(t.Context(), successor, 51, native, build, f.peer, rpc.OriginIntakeAnnounce)
	require.Error(t, err)
	operator := operatorOriginProducer()
	_, err = originSourceFromAdmission(t.Context(), operator, 51, native, build, f.peer, rpc.OriginIntakeAnnounce)
	require.Error(t, err)
	wrong := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.1.0.0/24"), 0, false)
	_, err = originSourceFromAdmission(t.Context(), producer, 51, wrong, build, f.peer, rpc.OriginIntakeAnnounce)
	require.Error(t, err)
}
