// Design: docs/architecture/resolve.md -- Clearing a resolver cache
// Related: sdk.go -- Run declares the caches OnResolverCacheClear recorded
//
// The resolver-cache-clear callback, for a plugin that holds its own PeeringDB,
// IRR or Team Cymru client.

package sdk

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ResolverCacheClearHandler clears one resolver cache of the plugin's own
// clients and returns the number of entries it removed. input.Cache is one of
// the caches the plugin declared, and input.Action is rpc.ResolverCacheClearAll
// or rpc.ResolverCacheDeleteEntry. A handler MUST count only the clients the
// plugin currently uses: a client it replaced is not its own any more.
type ResolverCacheClearHandler func(input rpc.ResolverCacheClearInput) (int, error)

// OnResolverCacheClear declares that the plugin holds clients for caches
// (rpc.ResolverCachePeeringDB, rpc.ResolverCacheIRR, rpc.ResolverCacheCymru)
// and sets the handler that clears them. Run declares caches at Stage 1, and
// the engine then sends the plugin every clear <cache> cache an operator runs
// for one of them. MUST be called before Run.
func (p *Plugin) OnResolverCacheClear(caches []string, fn ResolverCacheClearHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.resolverCaches = slices.Clone(caches)
	p.callbacks[rpc.MethodResolverCacheClear] = func(params json.RawMessage) (json.RawMessage, error) {
		var input rpc.ResolverCacheClearInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("unmarshal resolver-cache-clear: %w", err)
		}
		removed, err := fn(input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(&rpc.ResolverCacheClearOutput{Removed: removed})
	}
}
