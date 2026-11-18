// Design: docs/architecture/api/process-protocol.md -- reactor-owned origin lifetime.
package reactor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOriginRuntimeConstructsIssuerAndClosesOnStop(t *testing.T) {
	r := New(&Config{})
	require.NotNil(t, r.originReceipts, "the real Reactor constructor must own the origin issuer before API admission")
	r.originReceipts.mu.Lock()
	live := r.originReceipts.live()
	r.originReceipts.mu.Unlock()
	require.NoError(t, live)
	r.Stop()
	r.originReceipts.mu.Lock()
	closed := r.originReceipts.live()
	current, receipts := len(r.originReceipts.current), len(r.originReceipts.receipts)
	r.originReceipts.mu.Unlock()
	require.Error(t, closed, "Stop also closes an issuer constructed before Start")
	require.Zero(t, current)
	require.Zero(t, receipts)
	require.NoError(t, r.originReceipts.close(context.Background()))
}
