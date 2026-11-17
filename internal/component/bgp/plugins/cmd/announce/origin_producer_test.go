// Design: docs/architecture/api/process-protocol.md -- tagged original producer withdrawal.
package announce

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestTaggedAnnouncementKeepsOriginalProducerIncarnation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	sender := plugin.ProcessSender("tagged-producer")
	owner, err := plugin.NewSelectedOwner(sender, ctx.Done(), nil)
	require.NoError(t, err)
	owner, err = owner.WithConnection(func(context.Context, rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
		return nil, context.Canceled
	}, ctx)
	require.NoError(t, err)
	work := plugin.WithOriginProducerContext(ctx, owner, nil)
	reactor := &captureReactor{}
	calls := 0
	reg := NewRegistry(func(_ *selector.Selector, batch bgptypes.NLRIBatch, actual plugin.Sender) error {
		calls++
		require.Equal(t, sender, actual)
		require.True(t, batch.OriginOwner.SameIncarnation(owner))
		return nil
	})
	_, err = announceAndTrack(work, reg, reactor, testSel("upstream"), testBatch(family.IPv4Unicast), announceOpts{tagKey: "test", tagValue: "origin"}, "process", sender)
	require.NoError(t, err)
	require.True(t, reactor.batch.OriginOwner.SameIncarnation(owner))
	count, err := reg.withdrawTag("upstream", "test", "origin")
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, 1, calls)
}
