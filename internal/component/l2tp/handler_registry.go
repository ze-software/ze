// Design: docs/research/l2tpv2-ze-integration.md -- handler registration pattern
// Related: handler.go -- AuthHandler, PoolHandler types

package l2tp

import "sync"

// PoolStats carries the current state of the IP address pool for
// "show l2tp pool" CLI output.
type PoolStats struct {
	Name      string `json:"name"`
	RangeStr  string `json:"range"`
	Total     int    `json:"total"`
	Allocated int    `json:"allocated"`
	Available int    `json:"available"`
}

// PoolStatsProvider returns the current pool statistics. Registered by
// the l2tp-pool plugin at init time.
type PoolStatsProvider func() []PoolStats

var (
	handlerMu         sync.RWMutex
	authHandler       AuthHandler
	poolHandler       PoolHandler
	prefixHandler     PrefixHandler
	prefixReleaser    PrefixReleaser
	poolStatsProvider PoolStatsProvider
)

// RegisterAuthHandler registers the production auth handler. Called
// from plugin init(). Ignores nil handlers. If a handler is already
// registered, it is replaced (last writer wins; import order determines
// priority when multiple auth plugins are loaded).
func RegisterAuthHandler(h AuthHandler) {
	if h == nil {
		return
	}
	handlerMu.Lock()
	defer handlerMu.Unlock()
	authHandler = h
}

// GetAuthHandler returns the registered auth handler, or nil if none.
func GetAuthHandler() AuthHandler {
	handlerMu.RLock()
	defer handlerMu.RUnlock()
	return authHandler
}

// UnregisterAuthHandler removes the auth handler. Only for use in tests.
func UnregisterAuthHandler() {
	handlerMu.Lock()
	defer handlerMu.Unlock()
	authHandler = nil
}

// RegisterPoolHandler registers the production pool handler. Called
// from plugin init(). Ignores nil handlers.
func RegisterPoolHandler(h PoolHandler) {
	if h == nil {
		return
	}
	handlerMu.Lock()
	defer handlerMu.Unlock()
	poolHandler = h
}

// GetPoolHandler returns the registered pool handler, or nil if none.
func GetPoolHandler() PoolHandler {
	handlerMu.RLock()
	defer handlerMu.RUnlock()
	return poolHandler
}

// UnregisterPoolHandler removes the pool handler. Only for use in tests.
func UnregisterPoolHandler() {
	handlerMu.Lock()
	defer handlerMu.Unlock()
	poolHandler = nil
}

// RegisterPrefixHandler registers the production IPv6 prefix handler.
// Called from plugin init(). Ignores nil handlers.
func RegisterPrefixHandler(h PrefixHandler) {
	if h == nil {
		return
	}
	handlerMu.Lock()
	defer handlerMu.Unlock()
	prefixHandler = h
}

// GetPrefixHandler returns the registered prefix handler, or nil if none.
func GetPrefixHandler() PrefixHandler {
	handlerMu.RLock()
	defer handlerMu.RUnlock()
	return prefixHandler
}

// RegisterPrefixReleaser registers the function that releases an IPv6
// prefix on session teardown. Called from plugin init().
func RegisterPrefixReleaser(r PrefixReleaser) {
	if r == nil {
		return
	}
	handlerMu.Lock()
	defer handlerMu.Unlock()
	prefixReleaser = r
}

// GetPrefixReleaser returns the registered prefix releaser, or nil if none.
func GetPrefixReleaser() PrefixReleaser {
	handlerMu.RLock()
	defer handlerMu.RUnlock()
	return prefixReleaser
}

// UnregisterPrefixHandler removes the prefix handler. Only for use in tests.
func UnregisterPrefixHandler() {
	handlerMu.Lock()
	defer handlerMu.Unlock()
	prefixHandler = nil
}

// UnregisterPrefixReleaser removes the prefix releaser. Only for use in tests.
func UnregisterPrefixReleaser() {
	handlerMu.Lock()
	defer handlerMu.Unlock()
	prefixReleaser = nil
}

// RegisterPoolStatsProvider registers the function that returns pool
// statistics for "show l2tp pool". Called from the l2tp-pool plugin
// init(). Ignores nil providers.
func RegisterPoolStatsProvider(p PoolStatsProvider) {
	if p == nil {
		return
	}
	handlerMu.Lock()
	defer handlerMu.Unlock()
	poolStatsProvider = p
}

// GetPoolStatsProvider returns the registered pool stats provider, or nil.
func GetPoolStatsProvider() PoolStatsProvider {
	handlerMu.RLock()
	defer handlerMu.RUnlock()
	return poolStatsProvider
}
