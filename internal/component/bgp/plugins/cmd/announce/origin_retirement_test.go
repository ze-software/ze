// Design: docs/architecture/bgp/on-demand-origination.md -- exact accepted timer data.
// Related: announce.go -- actual handler capture and registry callback.
package announce

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The result is the boundary input, not an engine authorization simulation.
// Real issuer/SDK/wire validation lives in reactor's registry expiry questions.
type retirementResultReactor struct {
	captureReactor
	plugin.ReactorLifecycle
	result  *rpc.UpdateRouteOutput
	retired []*plugin.OriginRetirement
}

func (r *retirementResultReactor) SignalPluginStartupComplete() error { return nil }

func (r *retirementResultReactor) AnnounceNLRIBatchResult(ctx context.Context, sel *selector.Selector, batch bgptypes.NLRIBatch, sender plugin.Sender) (*rpc.UpdateRouteOutput, error) {
	if err := r.AnnounceNLRIBatch(ctx, sel, batch, sender); err != nil {
		return nil, err
	}
	return r.result, nil
}

func (r *retirementResultReactor) WithdrawNLRIBatchResult(context.Context, *selector.Selector, bgptypes.NLRIBatch, plugin.Sender) (*rpc.UpdateRouteOutput, error) {
	return &rpc.UpdateRouteOutput{Withdrawn: 1}, nil
}

func (r *retirementResultReactor) WithdrawNLRIBatch(ctx context.Context, _ *selector.Selector, batch bgptypes.NLRIBatch, _ plugin.Sender) error {
	retirement, trusted := plugin.OriginRetirementFromContext(ctx)
	if trusted {
		r.retired = append(r.retired, retirement)
	}
	return nil
}

func acceptedRetirementResult() *rpc.UpdateRouteOutput {
	return &rpc.UpdateRouteOutput{Announced: 1, Admissions: []rpc.RouteAdmission{{
		Family: family.IPv4Unicast.String(), KeyNLRI: []byte{24, 198, 18, 7},
		Destination: "192.0.2.80", PeerOwner: 77, Generation: 101,
		Outcome: rpc.RouteAdmissionRetained, Disposition: rpc.RouteAdmissionSelected,
	}}}
}

// The actual operator handler must capture its actual result, and the REAL
// getOrInitRegistry callback must carry it through timer retirement without
// inventing a Process owner. The timer callback is invoked by its own method;
// reactor's companion tests exercise its automatic scheduler plus real wire.
func TestOperatorRegistryCapturesAcceptedRetirement(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	reactor := &retirementResultReactor{result: acceptedRetirementResult()}
	host, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	require.NoError(t, err)
	require.NoError(t, host.StartWithContext(t.Context()))
	t.Cleanup(host.Stop)
	ctx := &pluginserver.CommandContext{Server: host, Sender: plugin.OperatorSender()}
	registryMu.Lock()
	previous := globalRegistry
	globalRegistry = nil
	registryMu.Unlock()
	t.Cleanup(func() { registryMu.Lock(); globalRegistry = previous; registryMu.Unlock() })
	reg := getOrInitRegistry(ctx)
	require.NotNil(t, reg)
	t.Cleanup(func() { _, err := reg.withdrawAll(""); require.NoError(t, err) })
	response, err := handleAnnounceUnicast(ctx, reactor, reg,
		[]string{"198.18.7.0/24", "next-hop", "192.0.2.254", "tag", "owner", "operator", "for", "300s"})
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status)
	require.Equal(t, 1, reactor.calls)
	entries := reg.List(listFilter{})
	require.Len(t, entries, 1)
	require.NotNil(t, entries[0].ExpiresAt)
	retirement := entries[0].Batch.OriginRetirement
	require.NotNil(t, retirement)
	require.True(t, retirement.MatchesOrigin(plugin.OperatorSender(), plugin.SelectedOwner{}, family.IPv4Unicast.String(), []byte{24, 198, 18, 7}, 77, 101))
	// Mutating the already-returned result cannot retarget the accepted entry.
	reactor.result.Admissions[0].Generation = 202
	reactor.result.Admissions[0].KeyNLRI[3] = 9
	reg.withdrawEntryByTimer(entries[0].ID)
	require.Zero(t, reg.Len())
	require.Len(t, reactor.retired, 1)
	require.Same(t, retirement, reactor.retired[0])
	require.True(t, retirement.MatchesOrigin(plugin.OperatorSender(), plugin.SelectedOwner{}, family.IPv4Unicast.String(), []byte{24, 198, 18, 7}, 77, 101))
	require.False(t, retirement.MatchesOrigin(plugin.OperatorSender(), plugin.SelectedOwner{}, family.IPv4Unicast.String(), []byte{24, 198, 18, 7}, 77, 202))
	require.False(t, retirement.MatchesOrigin(plugin.OperatorSender(), plugin.SelectedOwner{}, family.IPv4Unicast.String(), []byte{24, 198, 18, 9}, 77, 101))
}

// A named producer's accepted result must retain that exact incarnation, not
// merely its configured name. A direct result with no ordinary admission owns
// no ordinary Peer and keeps its existing direct route semantics.
func TestRegistryRetirementPreservesExactProcessAndDirectResult(t *testing.T) {
	lifetime, cancel := context.WithCancel(t.Context())
	defer cancel()
	sender := plugin.ProcessSender("retirement-source")
	owner, err := plugin.NewSelectedOwner(sender, lifetime.Done(), nil)
	require.NoError(t, err)
	owner, err = owner.WithConnection(func(context.Context, rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
		return nil, context.Canceled
	}, lifetime)
	require.NoError(t, err)
	reactor := &retirementResultReactor{result: acceptedRetirementResult()}
	var retired *plugin.OriginRetirement
	reg := NewRegistry(func(_ *selector.Selector, batch bgptypes.NLRIBatch, _ plugin.Sender) error {
		retired = batch.OriginRetirement
		return nil
	})
	_, err = announceAndTrack(plugin.WithOriginProducerContext(lifetime, owner, nil), reg, reactor,
		testSel("upstream"), bgptypes.NLRIBatch{Family: family.IPv4Unicast, NLRIs: []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("198.18.7.0/24"), 0, false)}}, announceOpts{tagKey: "owner", tagValue: "process", duration: time.Hour}, "process", sender)
	require.NoError(t, err)
	cancel()
	require.Error(t, owner.Current())
	count, err := reg.withdrawTag("upstream", "owner", "process")
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.NotNil(t, retired)
	require.True(t, retired.MatchesOrigin(sender, owner, family.IPv4Unicast.String(), []byte{24, 198, 18, 7}, 77, 101))
	nextLife, stop := context.WithCancel(t.Context())
	defer stop()
	successor, err := plugin.NewSelectedOwner(sender, nextLife.Done(), nil)
	require.NoError(t, err)
	successor, err = successor.WithConnection(func(context.Context, rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
		return nil, context.Canceled
	}, nextLife)
	require.NoError(t, err)
	require.False(t, retired.MatchesOrigin(sender, successor, family.IPv4Unicast.String(), []byte{24, 198, 18, 7}, 77, 101))
	direct, err := plugin.CaptureOriginRetirement(sender, plugin.SelectedOwner{}, &rpc.UpdateRouteOutput{Announced: 1})
	require.NoError(t, err)
	require.False(t, direct.HasPeer(sender, 77))
}
