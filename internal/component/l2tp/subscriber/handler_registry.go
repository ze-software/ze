// Design: plan/learned/760-subscriber-session-model.md -- transport-generic handler registries

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

// AuthHandler validates PPP credentials and returns a decision.
type AuthHandler func(req ppp.EventAuthRequest, respond AuthRespondFunc) AuthResult

// PoolHandler allocates an IP address for a PPP session.
type PoolHandler func(req ppp.EventIPRequest) ppp.IPResponseArgs

// ShaperHandler is called on session-up to apply traffic shaping.
type ShaperHandler func(iface string, downloadRate, uploadRate uint64)

var (
	handlerMu   sync.RWMutex
	authHandler AuthHandler
	poolHandler PoolHandler
	shaperHndlr ShaperHandler
)

func RegisterAuthHandler(h AuthHandler) {
	if h == nil {
		return
	}
	handlerMu.Lock()
	authHandler = h
	handlerMu.Unlock()
}

func GetAuthHandler() AuthHandler {
	handlerMu.RLock()
	h := authHandler
	handlerMu.RUnlock()
	return h
}

func UnregisterAuthHandler() {
	handlerMu.Lock()
	authHandler = nil
	handlerMu.Unlock()
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

func UnregisterShaperHandler() {
	handlerMu.Lock()
	shaperHndlr = nil
	handlerMu.Unlock()
}
