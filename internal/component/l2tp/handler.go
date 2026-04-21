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
}

// AuthHandler validates PPP credentials and returns a decision.
// Called synchronously by the auth drain goroutine.
type AuthHandler func(req ppp.EventAuthRequest) AuthResult

// PoolHandler allocates an IP address for a PPP session and returns
// the response args. Called synchronously by the pool drain goroutine.
type PoolHandler func(req ppp.EventIPRequest) ppp.IPResponseArgs
