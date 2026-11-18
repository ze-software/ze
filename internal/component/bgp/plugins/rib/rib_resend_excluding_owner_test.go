// Design: docs/architecture/plugin/rib-storage-design.md -- captured owner-excluded replay.
package rib

import (
	"context"
	"net/netip"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ownerExcludedResendFixture retains real immutable SENT bytes for two aliases
// and an independent destination. Tests MUST close their SDK through t.Cleanup;
// this fixture MUST release every pool reference after the question finishes.
func ownerExcludedResendFixture(t *testing.T) *RIBManager {
	t.Helper()
	r := newTestRIBManager(t)
	for _, item := range []struct {
		owner, session uint64
		address        string
		prefix         string
	}{
		{701, 41, "192.0.2.70", "198.51.100.0/24"},
		{702, 42, "192.0.2.70", "198.51.101.0/24"},
		{703, 43, "192.0.2.71", "198.51.102.0/24"},
	} {
		handle, err := pool.RibOut.Intern([]byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 1})
		require.NoError(t, err)
		r.peerUp[item.owner] = true
		r.peerMeta[item.owner] = &peerMetadata{Address: netip.MustParseAddr(item.address), refreshOwner: item.owner, refreshSession: item.session}
		r.ribOut[item.owner] = map[family.Family]map[ribOutKey]ribOutEntry{family.IPv4Unicast: {
			{Prefix: netip.MustParsePrefix(item.prefix)}: {MsgID: item.owner + 1000, AttrHandle: handle, LocalOrigin: true},
		}}
		t.Cleanup(func() {
			for _, entries := range r.ribOut[item.owner] {
				for _, entry := range entries {
					entry.release()
				}
			}
		})
	}
	return r
}

// The registered command excludes only owner701. Owner702 shares its address
// but retains its distinct UP token; every marker and raw replay keeps that
// tuple. A recorder of unbound text commands cannot satisfy this question.
// MUTATION: exclude the address instead of owner, or erase a captured token.
func TestOwnerExcludedResendRegisteredKeepsSameAddressSibling(t *testing.T) {
	r := ownerExcludedResendFixture(t)
	var inputs []rpc.UpdateRouteInput
	r.updateRefuseHook = func(input rpc.UpdateRouteInput) error { inputs = append(inputs, input); return nil }
	r.updateHook = func(string, map[string]any) {}
	status, data, err := r.handleCommand(ownerExcludedResendCommand, "*", []string{"owner", "701", "family", "ipv4/unicast"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	require.Equal(t, map[string]any{"refreshed-destinations": 2, "refused-destinations": 0}, data)
	require.Len(t, inputs, 6)
	for i, owner := range []uint64{702, 703} {
		group := inputs[i*3 : i*3+3]
		for _, input := range group {
			require.Equal(t, owner, input.DestinationOwner)
			require.Equal(t, owner-660, input.DestinationSession)
			require.Positive(t, input.DeadlineMS)
			require.Equal(t, r.peerMeta[owner].Address.String(), input.PeerSelector)
		}
		require.Equal(t, "borr", group[0].PeerMarker)
		require.Equal(t, "ipv4/unicast", group[0].MarkerFamily)
		require.Contains(t, group[1].Command, "update hex ")
		require.Equal(t, true, group[1].Meta[metaKeyReplay])
		require.Equal(t, "eorr", group[2].PeerMarker)
	}
}

// A target replaced after its Begin cannot lend the successor to the remaining
// bytes or End. An independent target still completes its useful full bracket.
func TestOwnerExcludedResendDoesNotRebindLegacySuccessor(t *testing.T) {
	r := ownerExcludedResendFixture(t)
	var inputs []rpc.UpdateRouteInput
	r.updateHook = func(string, map[string]any) {}
	r.updateRefuseHook = func(input rpc.UpdateRouteInput) error {
		inputs = append(inputs, input)
		if input.DestinationOwner == 702 {
			r.peerMu.Lock()
			r.peerMeta[702].refreshSession = 99
			r.peerMu.Unlock()
		}
		return nil
	}
	status, data, err := r.handleCommand(ownerExcludedResendCommand, "*", []string{"owner", "701", "family", "ipv4/unicast"})
	require.Error(t, err)
	require.Equal(t, statusError, status)
	require.Equal(t, map[string]any{"refreshed-destinations": 1, "refused-destinations": 1}, data)
	require.Len(t, inputs, 4)
	require.Equal(t, "borr", inputs[0].PeerMarker)
	require.Equal(t, uint64(702), inputs[0].DestinationOwner)
	require.Equal(t, uint64(42), inputs[0].DestinationSession)
	for _, input := range inputs[1:] {
		require.Equal(t, uint64(703), input.DestinationOwner)
		require.Equal(t, uint64(43), input.DestinationSession)
	}
}

// Parser and snapshot refusals reach no writer. A missing original UP token
// cannot silently turn the new narrowing operation into a bare-address resend.
func TestOwnerExcludedResendRefusesIncompleteScopeBeforeOutput(t *testing.T) {
	for _, args := range [][]string{nil, {"701"}, {"owner", "0"}, {"owner", "-1"}, {"owner", "18446744073709551616"}, {"owner", "701", "family"}, {"owner", "701", "family", "bogus"}, {"owner", "701", "owner", "702"}} {
		t.Run(strings.Join(args, "/"), func(t *testing.T) {
			r := ownerExcludedResendFixture(t)
			calls := 0
			r.updateRefuseHook = func(rpc.UpdateRouteInput) error { calls++; return nil }
			_, _, err := r.handleCommand(ownerExcludedResendCommand, "*", args)
			require.Error(t, err)
			require.Zero(t, calls)
		})
	}
	r := ownerExcludedResendFixture(t)
	r.peerMeta[702].refreshSession = 0
	calls := 0
	r.updateRefuseHook = func(rpc.UpdateRouteInput) error { calls++; return nil }
	_, _, err := r.handleCommand(ownerExcludedResendCommand, "*", []string{"owner", "701"})
	require.Error(t, err)
	require.Zero(t, calls)
}

// A receive-only source without any SENT state is not an output recipient and
// cannot make a useful owner-excluded replay depend on a nonexistent send token.
func TestOwnerExcludedResendSkipsReceiveOnlySource(t *testing.T) {
	r := ownerExcludedResendFixture(t)
	r.peerUp[704] = true
	r.peerMeta[704] = &peerMetadata{Address: netip.MustParseAddr("192.0.2.72")}
	r.updateRefuseHook = func(input rpc.UpdateRouteInput) error {
		require.NotEqual(t, uint64(704), input.DestinationOwner)
		return nil
	}
	r.updateHook = func(string, map[string]any) {}
	status, data, err := r.handleCommand(ownerExcludedResendCommand, "*", []string{"owner", "701"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	require.Equal(t, map[string]any{"refreshed-destinations": 2, "refused-destinations": 0}, data)
}

// An ordinary destination runs its existing selected refresh worker, not SENT
// history. The command cannot report completion while End is held. Canceling
// the command returns a refusal; the worker still owns its real final receipt.
// MUTATION: return after queueing refresh instead of joining the captured worker.
func TestOwnerExcludedOrdinaryResendJoinsCapturedEnd(t *testing.T) {
	ctx := selectedDispatchContext(t)
	r := newTestRIBManager(t)
	entered, release := make(chan struct{}, 1), make(chan struct{})
	var once sync.Once
	d, err := newSelectedDispatch(ctx, func(_ context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	r.selectedDispatch = d
	t.Cleanup(func() { once.Do(func() { close(release) }); require.NoError(t, d.stop(ctx)) })
	var markersMu sync.Mutex
	var markers []rpc.SelectRefreshInput
	require.NoError(t, d.setRefreshSender(func(work context.Context, input rpc.SelectRefreshInput) (*rpc.SelectRefreshOutput, error) {
		markersMu.Lock()
		markers = append(markers, input)
		markersMu.Unlock()
		if input.Action == rpc.SelectedRefreshEnd {
			select {
			case entered <- struct{}{}:
			default:
			}
			select {
			case <-release:
			case <-work.Done():
				return nil, work.Err()
			}
		}
		return selectedRefreshReceipt(input), nil
	}))
	peer := netip.MustParseAddr("192.0.2.70")
	route := selectedDispatchRoute(88, 188)
	require.NoError(t, d.publish(&route))
	for _, entry := range []struct{ owner, session uint64 }{{701, 41}, {702, 42}} {
		r.peerUp[entry.owner] = true
		r.peerMeta[entry.owner] = &peerMetadata{Address: peer, refreshOwner: entry.owner, refreshSession: entry.session, refreshOrdinary: true}
		require.NoError(t, d.destinationOwnerUp(peer, entry.owner, entry.session))
	}
	require.NoError(t, d.barrier(ctx))
	request, err := parseOwnerExcludedResend([]string{"owner", "701", "family", "ipv4/unicast"})
	require.NoError(t, err)
	work, cancel := context.WithCancel(ctx)
	defer cancel()
	joined := make(chan struct{})
	result := make(chan error, 1)
	go func() { defer close(joined); _, err := request.resend(work, r); result <- err }()
	t.Cleanup(func() { cancel(); once.Do(func() { close(release) }); <-joined })
	selectedDispatchAwait(t, ctx, entered)
	cancel()
	require.ErrorIs(t, selectedDispatchAwait(t, ctx, result), context.Canceled)
	once.Do(func() { close(release) })
	require.NoError(t, d.barrier(ctx))
	markersMu.Lock()
	snapshot := append([]rpc.SelectRefreshInput(nil), markers...)
	markersMu.Unlock()
	require.Len(t, snapshot, 2)
	require.Equal(t, rpc.SelectedRefreshBegin, snapshot[0].Action)
	require.Equal(t, rpc.SelectedRefreshEnd, snapshot[1].Action)
	for _, marker := range snapshot {
		require.Equal(t, uint64(702), marker.PeerOwner)
		require.Equal(t, uint64(42), marker.DestinationSession)
	}
	status, _, err := r.handleCommand(ownerExcludedResendCommand, "*", []string{"owner", "701", "family", "ipv4/unicast"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	require.Empty(t, r.ribOut, "ordinary refresh must not need a SENT projection")
}
