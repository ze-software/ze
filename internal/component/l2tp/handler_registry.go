// Design: docs/research/l2tpv2-ze-integration.md -- handler registration pattern
// Related: handler.go -- AuthHandler, PoolHandler types

package l2tp

import (
	"sync"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
)

// Prefix handlers remain L2TP-scoped (they use TunnelID/SessionID tuples).
// Auth and pool delegate to subscriber.
var (
	handlerMu      sync.RWMutex
	prefixHandler  PrefixHandler
	prefixReleaser PrefixReleaser
)

// AuthPriority orders registered subscriber authentication handlers. Lower
// non-zero values take precedence.
type AuthPriority = subscriber.AuthPriority

const (
	// AuthPriorityUnspecified marks an unset priority and is rejected on registration.
	AuthPriorityUnspecified = subscriber.AuthPriorityUnspecified
	// AuthPriorityRemote is the precedence slot for a configured remote AAA
	// backend such as RADIUS.
	AuthPriorityRemote = subscriber.AuthPriorityRemote
	// AuthPriorityLocal is the precedence slot for the local static user store.
	AuthPriorityLocal = subscriber.AuthPriorityLocal
)

// RegisterAuthHandler delegates to subscriber.RegisterAuthHandler at priority,
// adapting the L2TP-typed handler to the subscriber type system.
// Safe for concurrent use.
func RegisterAuthHandler(priority AuthPriority, h AuthHandler) {
	if h == nil {
		subscriber.RegisterAuthHandler(priority, nil)
		return
	}
	subscriber.RegisterAuthHandler(priority, func(req ppp.EventAuthRequest, respond subscriber.AuthRespondFunc) subscriber.AuthResult {
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

// GetAuthHandler returns the highest-precedence registered auth handler
// adapted from the subscriber registry, or nil if none.
// Safe for concurrent use.
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

// UnregisterAuthHandler removes the auth handler registered at priority.
// Safe for concurrent use.
func UnregisterAuthHandler(priority AuthPriority) {
	subscriber.UnregisterAuthHandler(priority)
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
