// Design: docs/research/l2tpv2-ze-integration.md -- auth/pool handler types
// Related: subsystem.go -- drain goroutines call registered handlers

package l2tp

import "codeberg.org/thomas-mangin/ze/internal/component/ppp"

// AuthResult carries the auth handler's decision back to the drain
// goroutine.
type AuthResult struct {
	Accept           bool
	Message          string
	AuthResponseBlob []byte
	// Handled signals that the handler has already called
	// Driver.AuthResponse directly (e.g. from a RADIUS goroutine).
	// When true, the drain goroutine skips its own AuthResponse call.
	Handled bool
}

// AuthRespondFunc delivers an auth decision to the per-session goroutine.
// Handlers that do async I/O (RADIUS) call this from a goroutine and
// return Handled=true so the drain skips its own response.
type AuthRespondFunc func(accept bool, message string, authResponseBlob []byte) error

// AuthHandler validates PPP credentials and returns a decision.
// respond delivers the decision asynchronously; handlers that use it
// MUST return Handled=true. Called by the auth drain goroutine.
type AuthHandler func(req ppp.EventAuthRequest, respond AuthRespondFunc) AuthResult

// PoolHandler allocates an IP address for a PPP session and returns
// the response args. Called synchronously by the pool drain goroutine.
type PoolHandler func(req ppp.EventIPRequest) ppp.IPResponseArgs
