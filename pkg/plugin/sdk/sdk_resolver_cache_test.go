package sdk

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// VALIDATES: OnResolverCacheClear makes Run declare the caches at Stage 1, and
// its callback answers with the count the handler returned.
// PREVENTS: a plugin whose handler is set but whose caches the engine never
// learns, so no clear ever reaches it.
//
// The method registers a handler for irr and peeringdb, reads the Stage 1
// request on the engine side, then calls the registered callback directly.
func TestOnResolverCacheClear_DeclaresAndAnswers(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)
	var asked rpc.ResolverCacheClearInput
	p.OnResolverCacheClear([]string{rpc.ResolverCacheIRR, rpc.ResolverCachePeeringDB},
		func(input rpc.ResolverCacheClearInput) (int, error) {
			asked = input
			return 7, nil
		})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, &Registration{}) }()

	req := readEngineRequest(t, ctx, engine.mux)
	var sent Registration
	require.NoError(t, json.Unmarshal(req.Params, &sent))
	assert.Equal(t, []string{rpc.ResolverCacheIRR, rpc.ResolverCachePeeringDB}, sent.ResolverCaches)
	cancel()
	<-errCh

	p.mu.Lock()
	callback := p.callbacks[rpc.MethodResolverCacheClear]
	p.mu.Unlock()
	require.NotNil(t, callback)
	params, err := json.Marshal(rpc.ResolverCacheClearInput{Cache: rpc.ResolverCacheIRR, Action: rpc.ResolverCacheDeleteEntry, Key: "AS-X"})
	require.NoError(t, err)
	result, err := callback(params)
	require.NoError(t, err)
	var out rpc.ResolverCacheClearOutput
	require.NoError(t, json.Unmarshal(result, &out))
	assert.Equal(t, 7, out.Removed)
	assert.Equal(t, "AS-X", asked.Key)
}
