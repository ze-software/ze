// Design: docs/architecture/api/process-protocol.md -- failed accepted replacements.
package redistribute

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type bgpReplacementCall struct {
	input    rpc.UpdateRouteInput
	snapshot rpc.OriginAdmissionSnapshot
}

// The real consumer publishes its lookup handler and issues the full request.
// Each call reads its exact original ID through that handler before answering.
// This is the consumer/result seam, not a socket or deciding-plugin fixture.
type bgpReplacementDispatcher struct {
	fakeDispatcher
	t      *testing.T
	lookup rpc.OriginAdmissionLookupHandler
	answer func(rpc.UpdateRouteInput) (*rpc.UpdateRouteOutput, error)
	calls  []bgpReplacementCall
}

func (d *bgpReplacementDispatcher) OnOriginAdmissionLookup(handler rpc.OriginAdmissionLookupHandler) {
	d.lookup = handler
}

func (d *bgpReplacementDispatcher) query(id uint64, operation rpc.OriginIntakeOperation, peer string) (*rpc.OriginAdmissionLookupOutput, error) {
	owner := uint64(91)
	if peer == "192.0.2.8" {
		owner = 92
	}
	return d.lookup(d.t.Context(), rpc.OriginAdmissionLookupInput{AdmissionID: id, Family: "ipv4/unicast", Prefix: "10.9.0.0/24",
		Destination: peer, PeerOwner: owner, Operation: operation, DeadlineMS: time.Now().Add(time.Second).UnixMilli()})
}

func (d *bgpReplacementDispatcher) UpdateRouteRequestResult(_ context.Context, input rpc.UpdateRouteInput) (*rpc.UpdateRouteOutput, error) {
	operation := rpc.OriginIntakeAnnounce
	if strings.Contains(input.Command, " del ") {
		operation = rpc.OriginIntakeWithdraw
	}
	peer := input.PeerSelector
	if peer == "*" {
		peer = "192.0.2.7"
	}
	original, err := d.query(input.OriginAdmissionID, operation, peer)
	require.NoError(d.t, err, "the full request must still name its original accepted scope")
	d.calls = append(d.calls, bgpReplacementCall{input: input, snapshot: original.Snapshot})
	if d.answer != nil {
		return d.answer(input)
	}
	if operation == rpc.OriginIntakeWithdraw {
		return &rpc.UpdateRouteOutput{Withdrawn: 1}, nil
	}
	return bgpReplacementRetained(peer), nil
}

func bgpReplacementRetained(peer string) *rpc.UpdateRouteOutput {
	owner := uint64(91)
	if peer == "192.0.2.8" {
		owner = 92
	}
	return &rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{{Family: "ipv4/unicast", KeyNLRI: []byte{24, 10, 9, 0}, Destination: peer,
		PeerOwner: owner, Generation: 3, Outcome: rpc.RouteAdmissionRetained, Disposition: rpc.RouteAdmissionSuppressedSelection}}}
}

// A failed all-peer replacement must not lose its two original concrete scopes.
// Hold one exact withdrawal in failure, complete the other scopes, then retry
// after the current group is empty. A same-prefix independent source stays useful.
// MUTATION: finish predecessors on failure, or enumerate only the current group.
func TestBGPConsumerFailedReplacementKeepsOriginalCleanup(t *testing.T) {
	t.Cleanup(redistevents.PreserveForTest())
	refused := rpc.RouteAdmission{Family: "ipv4/unicast", KeyNLRI: []byte{24, 10, 9, 0}, Destination: "192.0.2.8", PeerOwner: 92,
		Outcome: rpc.RouteAdmissionRefused, Reason: "replacement scope refused"}
	invalid := bgpReplacementRetained("192.0.2.7")
	invalid.Admissions[0].Generation = 0
	for _, test := range []struct {
		name   string
		output *rpc.UpdateRouteOutput
		cause  error
	}{
		{name: "transport", cause: errors.New("replacement transport lost")},
		{name: "missing"},
		{name: "failure", output: &rpc.UpdateRouteOutput{Failure: "replacement failed"}},
		{name: "partial-count", output: &rpc.UpdateRouteOutput{Announced: 1, Withdrawn: 1, Failure: "later scope failed"}},
		{name: "refused", output: &rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{refused}}},
		{name: "partial-admission", output: &rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{bgpReplacementRetained("192.0.2.7").Admissions[0], refused}}},
		{name: "invalid-admission", output: invalid},
		{name: "zero", output: &rpc.UpdateRouteOutput{}},
		{name: "zero-warning", output: &rpc.UpdateRouteOutput{Warnings: []string{"no established recipient"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			d := &bgpReplacementDispatcher{t: t}
			consumer := NewBGPConsumer(d)
			entry := bgpAcceptedFixtureEntry(t)
			first, second := entry, entry
			first.Peer, second.Peer = "192.0.2.7", "192.0.2.8"
			consumer.InjectRoute(t.Context(), family.IPv4Unicast, first)
			consumer.InjectRoute(t.Context(), family.IPv4Unicast, second)
			sibling := entry
			sibling.SourceInstance++
			consumer.InjectRoute(t.Context(), family.IPv4Unicast, sibling)
			require.Len(t, d.calls, 3)
			oldIDs := []uint64{d.calls[0].input.OriginAdmissionID, d.calls[1].input.OriginAdmissionID}
			siblingID := d.calls[2].input.OriginAdmissionID
			originalKey := consumer.accepted.ids[oldIDs[0]].key
			replacement := entry
			replacement.Metric, replacement.SourceMetric, replacement.AdmissionGeneration = 100, 9, 1
			d.answer = func(rpc.UpdateRouteInput) (*rpc.UpdateRouteOutput, error) { return test.output, test.cause }
			consumer.InjectRoute(t.Context(), family.IPv4Unicast, replacement)
			require.Len(t, d.calls, 4)
			newID := d.calls[3].input.OriginAdmissionID
			require.NotContains(t, oldIDs, newID)
			for index, id := range oldIDs {
				peer := []string{first.Peer, second.Peer}[index]
				_, err := d.query(id, rpc.OriginIntakeAnnounce, peer)
				require.Error(t, err, "an original predecessor stays withdraw-only")
				original, err := d.query(id, rpc.OriginIntakeWithdraw, peer)
				require.NoError(t, err, "failed replacement cannot erase an unconfirmed original admission")
				require.Equal(t, rpc.OriginAdmissionRetiring, original.State)
				require.Equal(t, uint32(7), original.Snapshot.SourceMetric)
				require.Equal(t, uint32(99), original.Snapshot.Metric)
				require.Zero(t, original.Snapshot.RuleGeneration)
				require.Equal(t, peer, original.Snapshot.Peer)
				require.Equal(t, []uint32{65000<<16 | 17}, original.Snapshot.Community)
			}
			current, err := d.query(newID, rpc.OriginIntakeAnnounce, "192.0.2.7")
			require.NoError(t, err)
			require.Equal(t, rpc.OriginAdmissionCurrent, current.State, "current describes accepted input, not successful output")
			require.Equal(t, uint32(100), current.Snapshot.Metric)
			require.Equal(t, uint64(1), current.Snapshot.RuleGeneration)
			d.answer = nil
			consumer.InjectRoute(t.Context(), family.IPv4Unicast, replacement)
			require.Equal(t, newID, d.calls[len(d.calls)-1].input.OriginAdmissionID, "identical retry keeps the original snapshot and ID")
			for index, id := range oldIDs {
				_, err := d.query(id, rpc.OriginIntakeWithdraw, []string{first.Peer, second.Peer}[index])
				require.NoError(t, err, "a later retry cannot invent confirmation for an earlier unknown predecessor")
			}
			d.calls = nil
			d.answer = func(input rpc.UpdateRouteInput) (*rpc.UpdateRouteOutput, error) {
				require.Equal(t, formatWithdraw(family.IPv4Unicast.String(), entry.Prefix), input.Command)
				if input.OriginAdmissionID == oldIDs[0] {
					return &rpc.UpdateRouteOutput{Withdrawn: 1, Failure: "original cleanup remains unconfirmed"}, nil
				}
				return &rpc.UpdateRouteOutput{Withdrawn: 1}, nil
			}
			removal := configredist.RouteRemoval{Prefix: entry.Prefix, Protocol: entry.Protocol, SourceInstance: entry.SourceInstance, Table: entry.Table}
			require.ErrorContains(t, consumer.WithdrawRoute(t.Context(), family.IPv4Unicast, removal), "original cleanup remains unconfirmed", "an unconfirmed original cleanup is the withdrawal's answer")
			require.Len(t, d.calls, 3)
			require.Equal(t, []uint64{oldIDs[0], oldIDs[1], newID}, []uint64{d.calls[0].input.OriginAdmissionID, d.calls[1].input.OriginAdmissionID, d.calls[2].input.OriginAdmissionID})
			require.Equal(t, []string{first.Peer, second.Peer, "*"}, []string{d.calls[0].input.PeerSelector, d.calls[1].input.PeerSelector, d.calls[2].input.PeerSelector})
			require.Equal(t, uint32(99), d.calls[0].snapshot.Metric)
			require.Equal(t, uint32(99), d.calls[1].snapshot.Metric)
			require.Equal(t, uint32(100), d.calls[2].snapshot.Metric)
			require.Len(t, consumer.accepted.ids, 2, "only failed original cleanup and independent sibling remain")
			require.Empty(t, consumer.accepted.groups[originalKey])
			_, err = d.query(siblingID, rpc.OriginIntakeAnnounce, "192.0.2.7")
			require.NoError(t, err)
			d.calls, d.answer = nil, nil
			require.NoError(t, consumer.WithdrawRoute(t.Context(), family.IPv4Unicast, removal), "the retry confirms the last original cleanup")
			require.Len(t, d.calls, 1, "retry reaches an original ID even after the current group is gone")
			require.Equal(t, oldIDs[0], d.calls[0].input.OriginAdmissionID)
			require.Equal(t, first.Peer, d.calls[0].input.PeerSelector)
			require.Equal(t, uint32(99), d.calls[0].snapshot.Metric)
			require.Len(t, consumer.accepted.ids, 1)
			for _, id := range append(oldIDs, newID) {
				_, err := d.query(id, rpc.OriginIntakeWithdraw, "192.0.2.7")
				require.Error(t, err, "confirmed cleanup leaves no completed ID history")
			}
			consumer.InjectRoute(t.Context(), family.IPv4Unicast, sibling)
			require.Equal(t, siblingID, d.calls[len(d.calls)-1].input.OriginAdmissionID, "the independent source still does useful admitted work")
		})
	}
}

// Both established count success and an explicit retained loser confirm the
// immediate replacement. A failed targeted replay must keep its original current
// all-peer admission, rather than inventing a new ID or rolling back its snapshot.
func TestBGPConsumerConfirmedReplacementKeepsOriginalReplay(t *testing.T) {
	t.Cleanup(redistevents.PreserveForTest())
	for _, retained := range []bool{false, true} {
		t.Run(map[bool]string{false: "announced", true: "retained"}[retained], func(t *testing.T) {
			d := &bgpReplacementDispatcher{t: t}
			consumer := NewBGPConsumer(d)
			entry := bgpAcceptedFixtureEntry(t)
			consumer.InjectRoute(t.Context(), family.IPv4Unicast, entry)
			oldID := d.calls[0].input.OriginAdmissionID
			replacement := entry
			replacement.Metric = 100
			replacement.AdmissionGeneration = 1
			d.answer = func(rpc.UpdateRouteInput) (*rpc.UpdateRouteOutput, error) {
				if retained {
					return bgpReplacementRetained("192.0.2.7"), nil
				}
				return &rpc.UpdateRouteOutput{Announced: 1}, nil
			}
			consumer.InjectRoute(t.Context(), family.IPv4Unicast, replacement)
			newID := d.calls[1].input.OriginAdmissionID
			require.NotEqual(t, oldID, newID)
			_, err := d.query(oldID, rpc.OriginIntakeWithdraw, "192.0.2.7")
			require.Error(t, err, "confirmed immediate replacement retires its own predecessor")
			targeted := replacement
			targeted.Peer = "192.0.2.7"
			targeted.AdmissionGeneration = 2
			d.answer = func(rpc.UpdateRouteInput) (*rpc.UpdateRouteOutput, error) {
				return nil, errors.New("targeted replay transport failed")
			}
			consumer.InjectRoute(t.Context(), family.IPv4Unicast, targeted)
			require.Equal(t, newID, d.calls[2].input.OriginAdmissionID)
			original, err := d.query(newID, rpc.OriginIntakeAnnounce, targeted.Peer)
			require.NoError(t, err)
			require.Equal(t, rpc.OriginAdmissionCurrent, original.State)
			require.Equal(t, uint64(1), original.Snapshot.RuleGeneration, "a failed newer evaluator cannot restamp the original")
			require.Equal(t, uint32(100), original.Snapshot.Metric)
			require.Equal(t, rpc.OriginAdmissionAllPeers, original.Snapshot.PeerScope)
			require.Empty(t, original.Snapshot.Peer)
			d.answer = nil
			consumer.InjectRoute(t.Context(), family.IPv4Unicast, targeted)
			require.Equal(t, newID, d.calls[3].input.OriginAdmissionID)
			require.Len(t, consumer.accepted.ids, 1)
		})
	}
}

// Pending predecessors exist in the original ID map while a newer offered
// scope occupies the current group. Exact cleanup must find both collections,
// including when the latest scope has already completed its withdrawal.
// MUTATION: enumerate groups[key] instead of all live IDs of the exact key.
func TestBGPAcceptedCleanupFindsPendingOriginalScopes(t *testing.T) {
	t.Cleanup(redistevents.PreserveForTest())
	records := newBGPAcceptedRecords()
	release, err := records.begin(t.Context())
	require.NoError(t, err)
	defer release()
	entry := bgpAcceptedFixtureEntry(t)
	firstEntry, secondEntry := entry, entry
	firstEntry.Peer, secondEntry.Peer = "192.0.2.7", "192.0.2.8"
	first, _, err := records.offer(family.IPv4Unicast, firstEntry)
	require.NoError(t, err)
	second, _, err := records.offer(family.IPv4Unicast, secondEntry)
	require.NoError(t, err)
	siblingEntry := entry
	siblingEntry.SourceInstance++
	sibling, _, err := records.offer(family.IPv4Unicast, siblingEntry)
	require.NoError(t, err)
	current, replaced, err := records.offer(family.IPv4Unicast, entry)
	require.NoError(t, err)
	require.ElementsMatch(t, []*bgpAcceptedRecord{first, second}, replaced)
	removal := configredist.RouteRemoval{Prefix: entry.Prefix, Protocol: entry.Protocol, SourceInstance: entry.SourceInstance, Table: entry.Table}
	pending, err := records.retiring(family.IPv4Unicast, removal)
	require.NoError(t, err)
	require.Equal(t, []*bgpAcceptedRecord{first, second, current}, pending, "each original ID remains an exact cleanup obligation")
	records.withdrawn(current)
	require.Empty(t, records.groups[current.key])
	pending, err = records.retiring(family.IPv4Unicast, removal)
	require.NoError(t, err)
	require.Equal(t, []*bgpAcceptedRecord{first, second}, pending, "pending originals outlive the latest group entry")
	for _, original := range pending {
		query := bgpAcceptedFixtureQuery(original, rpc.OriginIntakeAnnounce)
		query.Destination = original.snapshot.Peer
		_, err := records.lookup(t.Context(), query)
		require.Error(t, err)
		query.Operation = rpc.OriginIntakeWithdraw
		_, err = records.lookup(t.Context(), query)
		require.NoError(t, err)
		records.withdrawn(original)
	}
	_, err = records.lookup(t.Context(), bgpAcceptedFixtureQuery(sibling, rpc.OriginIntakeAnnounce))
	require.NoError(t, err, "the independent same-prefix source remains useful")
	require.Len(t, records.ids, 1)
}
