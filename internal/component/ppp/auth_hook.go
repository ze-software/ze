// Design: docs/research/l2tpv2-ze-integration.md -- PPP auth-phase boundary (stub for 6a; real auth in 6b)

package ppp

import (
	"context"
	"log/slog"
)

// AuthHook is invoked by the per-session goroutine after LCP reaches
// the Opened state. The implementation decides whether to admit the
// session by returning Accept=true on the AuthResponse.
//
// Phase 6a ships only the StubAuthHook implementation (always accepts,
// logs a WARN) so that the LCP -> session-up path can be exercised
// end-to-end. Spec spec-l2tp-6b-auth REPLACES the AuthHook interface
// with a channel-based dispatcher; the stub is DELETED, not extended,
// per .claude/rules/no-layering.md.
type AuthHook interface {
	Authenticate(ctx context.Context, req AuthRequest) AuthResponse
}

// AuthRequest carries the data the hook needs to decide. The 6a stub
// ignores all fields. Method is the LCP-negotiated authentication
// protocol name ("none", "pap", "chap-md5", "ms-chap-v2"); in 6a the
// only value is "none" because the LCP advertised no Auth-Protocol
// option.
type AuthRequest struct {
	TunnelID  uint16
	SessionID uint16
	Method    string
}

// AuthResponse is returned by the hook. Accept=false causes the
// per-session goroutine to emit EventSessionDown and exit.
//
// Message is included in any wire-level Failure / Nak the auth method
// emits. In 6a there are no auth wire messages (Method is always
// "none"), so Message is unused.
type AuthResponse struct {
	Accept  bool
	Message string
}

// StubAuthHook is the Phase 6a placeholder AuthHook. It always
// accepts. A WARN log fires on every call so operators see the gap.
//
// REPLACED in spec-l2tp-6b-auth.
type StubAuthHook struct {
	Logger *slog.Logger
}

// Authenticate always returns Accept=true. Logs a WARN.
func (s StubAuthHook) Authenticate(_ context.Context, req AuthRequest) AuthResponse {
	if s.Logger != nil {
		s.Logger.Warn("ppp: auth stub returning success -- replace in spec-l2tp-6b-auth",
			"tunnel-id", req.TunnelID, "session-id", req.SessionID, "method", req.Method)
	}
	return AuthResponse{Accept: true}
}
