// Design: docs/architecture/api/process-protocol.md -- delayed local-origin receipts.
package reactor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
)

func TestOriginIntakeDelayedReceiptCannotAcknowledgeReplacement(t *testing.T) {
	f, store, producer, source, value := originReceiptFixture(t)
	first, err := store.issue(t.Context(), producer, source, value, f.peer, producer.owner)
	require.NoError(t, err)
	require.NoError(t, store.validateOriginIntakeReceipt(first, f.peer, producer.owner))
	second, err := store.issue(t.Context(), producer, source, value, f.peer, producer.owner)
	require.NoError(t, err)
	require.Error(t, store.validateOriginIntakeReceipt(first, f.peer, producer.owner))
	require.NoError(t, store.retireOriginReceipt(t.Context(), first))
	require.NoError(t, store.validateOriginIntakeReceipt(second, f.peer, producer.owner))
	require.Len(t, store.current, 1)
}

func TestOriginIntakeStoredBatchKeepsOriginalProducerToken(t *testing.T) {
	original, end := originOwnerFixture(t, "same-producer")
	replacement, _ := originOwnerFixture(t, "same-producer")
	batch := bgptypes.NLRIBatch{OriginOwner: original}
	ctx := plugin.WithOriginProducerContext(context.Background(), replacement, nil)
	producer, err := originBatchProducer(ctx, batch, plugin.ProcessSender("same-producer"))
	require.NoError(t, err)
	require.True(t, producer.owner.SameIncarnation(original))
	end()
	_, err = originBatchProducer(ctx, batch, plugin.ProcessSender("same-producer"))
	require.Error(t, err, "a stored ended token must not fall back to its same-name replacement")
}
