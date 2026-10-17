// Design: docs/architecture/resolve.md -- Clearing a resolver cache
// Related: store.go -- UseClients, where an IRR plugin replaces the clients
//
// The resolver-cache-clear answer of a plugin whose clients live in a
// PrefixStore: bgp-filter-irr and firewall-irr.

package store

import (
	"fmt"
	"strconv"

	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/component/resolve/peeringdb"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ResolverCaches returns the resolver caches a PrefixStore holds clients for:
// the list a plugin passes to sdk.Plugin.OnResolverCacheClear beside
// ClearResolverCache.
func ResolverCaches() []string {
	return []string{rpc.ResolverCacheIRR, rpc.ResolverCachePeeringDB}
}

// ClearResolverCache clears one resolver cache of the clients the store uses
// now and returns how many entries it removed. A client UseClients replaced is
// not the store's any more and is not counted, so the count is exact. The
// prefix entries the store holds are the filter's durable copy, not a client
// cache, and stay. Safe for concurrent use.
func (s *PrefixStore) ClearResolverCache(input rpc.ResolverCacheClearInput) (int, error) {
	irrClient, pdb := s.clients()
	switch input.Cache {
	case rpc.ResolverCacheIRR:
		return clearIRRClient(irrClient, input)
	case rpc.ResolverCachePeeringDB:
		return clearPeeringDBClient(pdb, input)
	}
	return 0, fmt.Errorf("resolver cache %q is not one this plugin holds: it holds %q and %q",
		input.Cache, rpc.ResolverCacheIRR, rpc.ResolverCachePeeringDB)
}

// clearIRRClient applies input to the IRR client. irrClient is never nil:
// New and UseClients require it.
func clearIRRClient(irrClient *irr.IRR, input rpc.ResolverCacheClearInput) (int, error) {
	switch input.Action {
	case rpc.ResolverCacheClearAll:
		return irrClient.ClearCache(), nil
	case rpc.ResolverCacheDeleteEntry:
		return irrClient.ClearCachedASSet(input.Key), nil
	}
	return 0, unknownClearAction(input.Action)
}

// clearPeeringDBClient applies input to the PeeringDB client. A store built
// with no PeeringDB client (firewall-irr before its first configure) has no
// PeeringDB cache, so it holds no entry and zero is the true count.
func clearPeeringDBClient(pdb *peeringdb.PeeringDB, input rpc.ResolverCacheClearInput) (int, error) {
	switch input.Action {
	case rpc.ResolverCacheClearAll:
		if pdb == nil {
			return 0, nil
		}
		return pdb.ClearCache(), nil
	case rpc.ResolverCacheDeleteEntry:
		number, err := strconv.ParseUint(input.Key, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("clear peeringdb cache: %q is not a decimal AS number: %w", input.Key, err)
		}
		if pdb == nil {
			return 0, nil
		}
		return pdb.ClearCachedASN(uint32(number)), nil
	}
	return 0, unknownClearAction(input.Action)
}

// unknownClearAction is the refusal of an action the engine never sends.
func unknownClearAction(action string) error {
	return fmt.Errorf("resolver cache clear action %q is not %q or %q",
		action, rpc.ResolverCacheClearAll, rpc.ResolverCacheDeleteEntry)
}
