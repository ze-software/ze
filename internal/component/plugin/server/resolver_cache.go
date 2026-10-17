// Design: docs/architecture/resolve.md -- Clearing a resolver cache
// Related: startup.go -- onRegistration validates the declaration at Stage 1
//
// The engine side of the resolver-cache-clear callback: which plugins own a
// resolver cache, and the call that asks one of them to clear it.

package server

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var errResolverCacheNoProcessManager = errors.New("resolver-cache-clear: no process manager")

// validateResolverCacheDecls refuses a Stage 1 declaration naming a resolver
// cache ze does not have, or one cache twice.
func validateResolverCacheDecls(name string, caches []string) error {
	for i, cache := range caches {
		if !rpc.KnownResolverCache(cache) {
			return fmt.Errorf("plugin %q declares resolver cache %q, which ze does not have: declare one of %s",
				name, cache, strings.Join(rpc.ResolverCacheNames(), ", "))
		}
		if slices.Contains(caches[:i], cache) {
			return fmt.Errorf("plugin %q declares resolver cache %q twice: declare each cache once", name, cache)
		}
	}
	return nil
}

// ResolverCacheOwners returns, sorted, the names of the running plugins that
// declared cache at Stage 1. Safe for concurrent use.
func (s *Server) ResolverCacheOwners(cache string) []string {
	pm := s.procManager.Load()
	if pm == nil {
		return nil
	}
	var owners []string
	for _, proc := range pm.AllProcesses() {
		reg := proc.Registration()
		if reg == nil {
			continue
		}
		if !slices.Contains(reg.ResolverCaches, cache) {
			continue
		}
		owners = append(owners, proc.Name())
	}
	slices.Sort(owners)
	return owners
}

// CallResolverCacheClear asks the plugin pluginName to clear one of its
// resolver caches and returns how many entries it removed. It returns an
// error when the plugin is not running, not connected, or the call fails, so
// a plugin that did not answer is never read as one that held nothing.
func (s *Server) CallResolverCacheClear(ctx context.Context, pluginName string, input *rpc.ResolverCacheClearInput) (*rpc.ResolverCacheClearOutput, error) {
	pm := s.procManager.Load()
	if pm == nil {
		return nil, errResolverCacheNoProcessManager
	}
	proc := pm.GetProcess(pluginName)
	if proc == nil {
		return nil, fmt.Errorf("resolver-cache-clear: plugin %q is not running", pluginName)
	}
	conn := proc.Conn()
	if conn == nil {
		return nil, fmt.Errorf("resolver-cache-clear: plugin %q is not connected", pluginName)
	}
	return conn.SendResolverCacheClear(ctx, input)
}
