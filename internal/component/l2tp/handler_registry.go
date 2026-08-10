// Design: docs/research/l2tpv2-ze-integration.md -- handler registration pattern
// Related: handler.go -- AuthHandler, PoolHandler types

package l2tp

import (
	"sync"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
)

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

// Prefix handlers and pool stats remain L2TP-scoped (they use
// TunnelID/SessionID tuples). Auth and pool delegate to subscriber.
var (
	handlerMu         sync.RWMutex
	prefixHandler     PrefixHandler
	prefixReleaser    PrefixReleaser
	poolStatsProvider PoolStatsProvider
)

// RegisterAuthHandler delegates to subscriber.RegisterAuthHandler,
// adapting the L2TP-typed handler to the subscriber type system.
func RegisterAuthHandler(h AuthHandler) {
	if h == nil {
		return
	}
	subscriber.RegisterAuthHandler(func(req ppp.EventAuthRequest, respond subscriber.AuthRespondFunc) subscriber.AuthResult {
		l2tpRespond := AuthRespondFunc(respond)
		r := h(req, l2tpRespond)
		return subscriber.AuthResult{
			Accept:           r.Accept,
			Message:          r.Message,
			AuthResponseBlob: r.AuthResponseBlob,
			Handled:          r.Handled,
		}
	})
}

// GetAuthHandler returns the registered auth handler adapted from the
// subscriber registry, or nil if none.
func GetAuthHandler() AuthHandler {
	sh := subscriber.GetAuthHandler()
	if sh == nil {
		return nil
	}
	return func(req ppp.EventAuthRequest, respond AuthRespondFunc) AuthResult {
		subRespond := subscriber.AuthRespondFunc(respond)
		r := sh(req, subRespond)
		return AuthResult{
			Accept:           r.Accept,
			Message:          r.Message,
			AuthResponseBlob: r.AuthResponseBlob,
			Handled:          r.Handled,
		}
	}
}

// UnregisterAuthHandler removes the auth handler. Only for use in tests.
func UnregisterAuthHandler() {
	subscriber.UnregisterAuthHandler()
}

// RegisterPoolHandler delegates to subscriber.RegisterPoolHandler.
func RegisterPoolHandler(h PoolHandler) {
	if h == nil {
		return
	}
	subscriber.RegisterPoolHandler(subscriber.PoolHandler(h))
}

// GetPoolHandler returns the registered pool handler, or nil if none.
func GetPoolHandler() PoolHandler {
	sh := subscriber.GetPoolHandler()
	if sh == nil {
		return nil
	}
	return PoolHandler(sh)
}

// UnregisterPoolHandler removes the pool handler. Only for use in tests.
func UnregisterPoolHandler() {
	subscriber.UnregisterPoolHandler()
}

// RegisterPrefixHandler registers the production IPv6 prefix handler.
// Called from plugin init(). Ignores nil handlers.
func RegisterPrefixHandler(h PrefixHandler) {
	if h == nil {
		return
	}
	handlerMu.Lock()
	prefixHandler = h
	handlerMu.Unlock()
}

// GetPrefixHandler returns the registered prefix handler, or nil if none.
func GetPrefixHandler() PrefixHandler {
	handlerMu.RLock()
	h := prefixHandler
	handlerMu.RUnlock()
	return h
}

// RegisterPrefixReleaser registers the function that releases an IPv6
// prefix on session teardown. Called from plugin init().
func RegisterPrefixReleaser(r PrefixReleaser) {
	if r == nil {
		return
	}
	handlerMu.Lock()
	prefixReleaser = r
	handlerMu.Unlock()
}

// GetPrefixReleaser returns the registered prefix releaser, or nil if none.
func GetPrefixReleaser() PrefixReleaser {
	handlerMu.RLock()
	r := prefixReleaser
	handlerMu.RUnlock()
	return r
}

// unregisterPrefixHandler removes the prefix handler. Only for use in tests.
func unregisterPrefixHandler() {
	handlerMu.Lock()
	prefixHandler = nil
	handlerMu.Unlock()
}

// unregisterPrefixReleaser removes the prefix releaser. Only for use in tests.
func unregisterPrefixReleaser() {
	handlerMu.Lock()
	prefixReleaser = nil
	handlerMu.Unlock()
}

// registerPoolStatsProvider registers the function that returns pool
// statistics for "show l2tp pool". Called from the l2tp-pool plugin
// init(). Ignores nil providers.
func registerPoolStatsProvider(p PoolStatsProvider) {
	if p == nil {
		return
	}
	handlerMu.Lock()
	poolStatsProvider = p
	handlerMu.Unlock()
}

// getPoolStatsProvider returns the registered pool stats provider, or nil.
func getPoolStatsProvider() PoolStatsProvider {
	handlerMu.RLock()
	p := poolStatsProvider
	handlerMu.RUnlock()
	return p
}
