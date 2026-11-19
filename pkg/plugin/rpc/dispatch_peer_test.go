// Design: docs/architecture/api/process-protocol.md -- exact physical command receipt.
package rpc

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// Both raw tuple halves and the deadline bind admission. Legacy/zero replies
// cannot acknowledge physical control, and exact receipts preserve uint64 width.
func TestPhysicalPeerCommandRequiresExactTupleReceipt(t *testing.T) {
	now := time.Now()
	good := DispatchCommandInput{Command: "request peer 192.0.2.90 pause", PeerOwner: 18446744073709551614, ConnectionID: 18446744073709551613, DeadlineMS: now.Add(time.Second).UnixMilli()}
	require.NoError(t, good.ValidatePeer(now))
	require.True(t, good.PeerBound())
	require.False(t, (DispatchCommandInput{Command: good.Command}).PeerBound())
	for _, bad := range []DispatchCommandInput{
		{Command: good.Command, ConnectionID: good.ConnectionID, DeadlineMS: good.DeadlineMS},
		{Command: good.Command, PeerOwner: good.PeerOwner, DeadlineMS: good.DeadlineMS},
		{Command: good.Command, PeerOwner: good.PeerOwner, ConnectionID: good.ConnectionID},
		{Command: good.Command, PeerOwner: good.PeerOwner, ConnectionID: good.ConnectionID, DeadlineMS: now.Add(31 * time.Second).UnixMilli()},
	} {
		require.True(t, bad.PeerBound())
		require.Error(t, bad.ValidatePeer(now))
	}
	output := &DispatchCommandOutput{Status: StatusDone, PeerOwner: good.PeerOwner, ConnectionID: good.ConnectionID, DeadlineMS: good.DeadlineMS}
	require.NoError(t, output.ValidatePeerAnswer(good))
	for _, bad := range []*DispatchCommandOutput{nil, {},
		{PeerOwner: good.PeerOwner - 1, ConnectionID: good.ConnectionID, DeadlineMS: good.DeadlineMS},
		{PeerOwner: good.PeerOwner, ConnectionID: good.ConnectionID - 1, DeadlineMS: good.DeadlineMS},
		{PeerOwner: good.PeerOwner, ConnectionID: good.ConnectionID, DeadlineMS: good.DeadlineMS - 1},
	} {
		require.Error(t, bad.ValidatePeerAnswer(good))
	}
}
