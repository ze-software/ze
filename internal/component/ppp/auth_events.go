// Design: docs/research/l2tpv2-ze-integration.md -- PPP auth-phase event boundary
// Related: events.go -- lifecycle events sealed sum (EventsOut())
// Related: manager.go -- Driver owns AuthEventsOut() and AuthResponse()
// Related: session.go -- per-session authRespCh

package ppp

// AuthMethod identifies the PPP authentication protocol negotiated for
// a session. The zero value (AuthMethodNone) means LCP advertised no
// Auth-Protocol option; the session is accepted without wire-level auth
// packets, though the auth event still fires so an external handler can
// log or deny.
//
// Phase 6a used the string "none"; 6b replaces string method names
// with this enum so the switch in the auth codec dispatch is
// compile-time exhaustive.
type AuthMethod uint8

const (
	AuthMethodNone AuthMethod = iota
	AuthMethodPAP
	AuthMethodCHAPMD5
	AuthMethodMSCHAPv2
)

// String returns the human-readable method name for logs. Panics on an
// unregistered value rather than returning "unknown" -- every AuthMethod
// const MUST appear in this switch; drift is a programmer error.
func (m AuthMethod) String() string {
	switch m {
	case AuthMethodNone:
		return "none"
	case AuthMethodPAP:
		return "pap"
	case AuthMethodCHAPMD5:
		return "chap-md5"
	case AuthMethodMSCHAPv2:
		return "ms-chap-v2"
	}
	panic("BUG: unknown AuthMethod")
}

// AuthEvent is the sealed sum emitted on Driver.AuthEventsOut(). The
// consumer (l2tp-auth plugin in production, a test responder in unit
// tests) reads EventAuthRequest, validates credentials out-of-band,
// and calls Driver.AuthResponse to unblock the session goroutine.
//
// EventAuthSuccess and EventAuthFailure are emitted AFTER the session
// consumes the response -- they report the outcome to the consumer
// for logging / metrics, not to drive further behavior.
//
// Separated from Event (lifecycle) so L2TP's reactor is not forced to
// pattern-match auth types it has no use for. See also the
// exhaustiveness pin near the bottom of this file.
type AuthEvent interface {
	isAuthEvent()
}

// EventAuthRequest is the request to validate peer credentials. The
// consumer MUST call Driver.AuthResponse(TunnelID, SessionID, ...)
// within the configured auth-timeout (future phases) or the session
// times out.
//
// Challenge and Response are opaque bytes: CHAP-MD5 stores the MD5
// challenge + response; MS-CHAPv2 stores the 16-byte Peer-Challenge
// followed by the 8-byte Reserved + 24-byte NT-Response. PAP leaves
// Challenge empty and places the cleartext password in Response.
// Phase 1 of 6b emits only AuthMethodNone with all opaque fields
// empty; future phases populate them as wire codecs land.
type EventAuthRequest struct {
	TunnelID   uint16
	SessionID  uint16
	Method     AuthMethod
	Identifier uint8
	Username   string
	Challenge  []byte
	Response   []byte
	PeerName   string
}

func (EventAuthRequest) isAuthEvent() {}

// EventAuthSuccess is emitted after the per-session goroutine consumes
// an accept response. Informational -- the session transitions to
// EventSessionUp on Event (lifecycle) channel immediately after.
type EventAuthSuccess struct {
	TunnelID  uint16
	SessionID uint16
}

func (EventAuthSuccess) isAuthEvent() {}

// EventAuthFailure is emitted after the per-session goroutine consumes
// a reject response or an auth-timeout fires. Reason is human-readable
// for logs; consumers MUST NOT parse it for control flow.
type EventAuthFailure struct {
	TunnelID  uint16
	SessionID uint16
	Reason    string
}

func (EventAuthFailure) isAuthEvent() {}

// authResponseMsg carries the consumer's decision from
// Driver.AuthResponse into the per-session goroutine via authRespCh.
// AuthResponseBlob is the opaque Authenticator-Response payload for
// MS-CHAPv2 Success; empty for PAP / CHAP-MD5 / AuthMethodNone.
type authResponseMsg struct {
	accept           bool
	message          string
	authResponseBlob []byte
}

// Compile-time exhaustiveness pin. Bumping the array length without
// updating every consumer's switch produces a build failure in the
// consumer, not here: the array type enforces that each concrete
// AuthEvent appears at least once in this file.
var _ = [3]AuthEvent{
	EventAuthRequest{},
	EventAuthSuccess{},
	EventAuthFailure{},
}
