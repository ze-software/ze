// Design: docs/architecture/l2tp/subscriber-session-model.md -- transport-generic handler registries

package subscriber

import (
	"sync"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// AuthRespondFunc delivers an auth decision asynchronously.
type AuthRespondFunc func(accept bool, message string, authResponseBlob []byte) error

// AuthResult carries the auth handler's decision.
type AuthResult struct {
	Accept           bool
	Message          string
	AuthResponseBlob []byte
	Handled          bool
}

// AuthPriority orders registered subscriber authentication handlers. Lower
// non-zero values take precedence, matching internal/component/aaa's priority
// convention. Zero (AuthPriorityUnspecified) is invalid so an unset priority
// cannot silently claim highest precedence.
type AuthPriority uint8

const (
	// AuthPriorityUnspecified marks an unset priority and is rejected on registration.
	AuthPriorityUnspecified AuthPriority = 0
	// AuthPriorityRemote is the precedence slot for a configured remote AAA
	// backend such as RADIUS.
	AuthPriorityRemote AuthPriority = 50
	// AuthPriorityLocal is the precedence slot for the local static user store.
	AuthPriorityLocal AuthPriority = 200
)

// AuthHandler validates PPP credentials and returns a decision.
type AuthHandler func(req ppp.EventAuthRequest, respond AuthRespondFunc) AuthResult

// PoolHandler allocates an IP address for a PPP session.
type PoolHandler func(req ppp.EventIPRequest) ppp.IPResponseArgs

// ShaperHandler is called on session-up to apply traffic shaping.
type ShaperHandler func(iface string, downloadRate, uploadRate uint64)

var (
	handlerMu         sync.RWMutex
	authHandlers      = make(map[AuthPriority]AuthHandler)
	activeAuthHandler AuthHandler
	poolHandler       PoolHandler
	shaperHndlr       ShaperHandler
)

// RegisterAuthHandler installs h at priority and recomputes the active auth
// handler (lowest non-zero priority wins). Nil handlers are ignored without
// altering existing slots. Passing AuthPriorityUnspecified panics with BUG.
// Safe for concurrent use.
func RegisterAuthHandler(priority AuthPriority, h AuthHandler) {
	if priority == AuthPriorityUnspecified {
		panic("BUG: subscriber: auth handler priority must be specified")
	}
	if h == nil {
		return
	}
	handlerMu.Lock()
	authHandlers[priority] = h
	activeAuthHandler = selectActiveAuthHandlerLocked()
	handlerMu.Unlock()
}

// GetAuthHandler returns the registered auth handler with the highest
// precedence (lowest non-zero AuthPriority), or nil when none is registered.
// Safe for concurrent use.
func GetAuthHandler() AuthHandler {
	handlerMu.RLock()
	h := activeAuthHandler
	handlerMu.RUnlock()
	return h
}

// UnregisterAuthHandler removes the auth handler at priority and recomputes
// the active auth handler from the remaining registered slots.
// Safe for concurrent use.
func UnregisterAuthHandler(priority AuthPriority) {
	if priority == AuthPriorityUnspecified {
		return
	}
	handlerMu.Lock()
	delete(authHandlers, priority)
	activeAuthHandler = selectActiveAuthHandlerLocked()
	handlerMu.Unlock()
}

func selectActiveAuthHandlerLocked() AuthHandler {
	var bestPriority AuthPriority
	var best AuthHandler
	for priority, handler := range authHandlers {
		if best == nil {
			bestPriority = priority
			best = handler
			continue
		}
		if priority < bestPriority {
			bestPriority = priority
			best = handler
		}
	}
	return best
}

func RegisterPoolHandler(h PoolHandler) {
	if h == nil {
		return
	}
	handlerMu.Lock()
	poolHandler = h
	handlerMu.Unlock()
}

func GetPoolHandler() PoolHandler {
	handlerMu.RLock()
	h := poolHandler
	handlerMu.RUnlock()
	return h
}

func UnregisterPoolHandler() {
	handlerMu.Lock()
	poolHandler = nil
	handlerMu.Unlock()
}

func RegisterShaperHandler(h ShaperHandler) {
	if h == nil {
		return
	}
	handlerMu.Lock()
	shaperHndlr = h
	handlerMu.Unlock()
}

func GetShaperHandler() ShaperHandler {
	handlerMu.RLock()
	h := shaperHndlr
	handlerMu.RUnlock()
	return h
}

func unregisterShaperHandler() {
	handlerMu.Lock()
	shaperHndlr = nil
	handlerMu.Unlock()
}
