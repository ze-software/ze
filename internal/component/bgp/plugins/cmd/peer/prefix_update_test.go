package peer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/component/resolve/peeringdb"
)

type traceIDKey struct{}

type recordingPrefixLookupClient struct {
	called bool
	ctx    context.Context
}

func (r *recordingPrefixLookupClient) LookupASN(ctx context.Context, _ uint32) (peeringdb.PrefixCounts, error) {
	r.called = true
	r.ctx = ctx
	return peeringdb.PrefixCounts{IPv4: 10, IPv6: 20}, nil
}

// VALIDATES: prefix-update lookup and rate-limit waits stop on caller cancellation.
// PREVENTS: context.TODO/background-rooted lookups continuing after the request is canceled.
func TestPrefixUpdateStopsOnContextCancel(t *testing.T) {
	t.Run("canceled before rate-limit wait", func(t *testing.T) {
		client := &recordingPrefixLookupClient{}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := lookupPrefixCounts(ctx, client, 65001, true)

		assert.ErrorIs(t, err, context.Canceled)
		assert.False(t, client.called, "lookup should not run after cancellation")
	})

	t.Run("lookup uses caller context", func(t *testing.T) {
		client := &recordingPrefixLookupClient{}
		ctx := context.WithValue(context.Background(), traceIDKey{}, "trace-id")

		counts, err := lookupPrefixCounts(ctx, client, 65001, false)

		assert.NoError(t, err)
		assert.True(t, client.called)
		assert.Same(t, ctx, client.ctx)
		assert.Equal(t, uint32(10), counts.IPv4)
		assert.Equal(t, uint32(20), counts.IPv6)
	})
}
