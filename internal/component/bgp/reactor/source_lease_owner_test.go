// Design: docs/architecture/api/process-protocol.md -- exact original Peer lease.
package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSourceLeaseRefusesAnotherPeerOwner reaches both engine adapters and proves
// wrong identities cannot acquire or revoke a useful original source lease.
func TestSourceLeaseRefusesAnotherPeerOwner(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "json", true: "native"}[native], func(t *testing.T) {
			proc := sourceLeaseProcessFactory(t)("source-owner-binding")
			peer, facts := sourceReceiptFixture(t)
			peer.settings.ProcessBindings = []ProcessBinding{sourceLeaseGrant(proc.Name())}
			live, err := sourceReceipts.issue(facts)
			require.NoError(t, err)
			t.Cleanup(live.close)
			server := &pluginserver.Server{}
			wrong := peer.sourceOwner + 1
			for _, owner := range []uint64{0, wrong} {
				_, err = sourceLeaseRetryAdapter(server, t.Context(), proc, native, sourceLeaseRequest(rpc.SourceLeaseAcquire, live.ID(), owner))
				require.Error(t, err)
			}
			input := sourceLeaseRequest(rpc.SourceLeaseAcquire, live.ID(), peer.sourceOwner)
			output, err := sourceLeaseRetryAdapter(server, t.Context(), proc, native, input)
			require.NoError(t, err)
			require.NoError(t, output.Validate(input))
			authenticated, err := proc.SelectedOwner()
			require.NoError(t, err)
			lease, err := sourceReceipts.ownedLease(live.ID(), authenticated)
			require.NoError(t, err)
			for _, action := range []rpc.SourceLeaseAction{rpc.SourceLeaseRetain, rpc.SourceLeaseRelease} {
				_, err = sourceLeaseRetryAdapter(server, t.Context(), proc, native, sourceLeaseRequest(action, live.ID(), wrong))
				require.Error(t, err)
				require.NoError(t, lease.current(), "a wrong owner cannot revoke the useful original lease")
			}
			input = sourceLeaseRequest(rpc.SourceLeaseRelease, live.ID(), 0)
			output, err = sourceLeaseRetryAdapter(server, t.Context(), proc, native, input)
			require.NoError(t, err)
			require.NoError(t, output.Validate(input))
			require.Error(t, lease.current(), "unrecorded terminal disposal grants no retained authority")
			input = sourceLeaseRequest(rpc.SourceLeaseRelease, ^uint64(0), wrong)
			output, err = sourceLeaseRetryAdapter(server, t.Context(), proc, native, input)
			require.NoError(t, err)
			require.NoError(t, output.Validate(input), "an absent receipt only echoes caller correlation")
		})
	}
}
