// Design: docs/architecture/l2tp/subscriber-session-model.md -- PPPoE auth/pool drain goroutines
// Related: server.go -- InterfaceServer session lifecycle

package pppoe

import (
	"log/slog"
	"sync"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/pkg/ze"
)

func startPPPoEAuthDrain(logger *slog.Logger, d *ppp.Driver, resolve func() subscriber.AuthHandler, bus ze.EventBus, pending *sync.Map) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for ev := range d.AuthEventsOut() {
			req, ok := ev.(ppp.EventAuthRequest)
			if !ok {
				continue
			}
			// Plugins can replace the provider after this drain has started.
			handler := resolve()
			respond := func(accept bool, msg string, blob []byte) error {
				return d.AuthResponse(req.TunnelID, req.SessionID, accept, msg, blob)
			}
			r := callPPPoEAuthHandler(handler, req, subscriber.AuthRespondFunc(respond))
			subscriber.RecordAuthResult(subscriber.AccessPPPoE, r.Accept)
			if r.Accept && req.Username != "" {
				pending.Store(
					pendingAuthKey{ifindex: int(req.TunnelID), sessionID: req.SessionID},
					pendingAuthInfo{username: req.Username, authMethod: req.Method.String()},
				)
			}
			if bus != nil {
				if _, err := subevents.SessionAuthResult.Emit(bus, &subevents.SessionAuthResultPayload{
					SessionID:  pppoeSessionID(int(req.TunnelID), req.SessionID),
					AccessType: subscriber.AccessPPPoE,
					Username:   req.Username,
					Accept:     r.Accept,
					Reason:     r.Message,
				}); err != nil {
					logger.Warn("pppoe: auth-result emit failed", "error", err)
				}
			}
			if r.Handled {
				continue
			}
			if err := d.AuthResponse(req.TunnelID, req.SessionID, r.Accept, r.Message, r.AuthResponseBlob); err != nil {
				logger.Warn("pppoe: auth drain response failed",
					"ifindex", req.TunnelID, "session", req.SessionID, "error", err)
			}
		}
	}()
	return done
}

func callPPPoEAuthHandler(handler subscriber.AuthHandler, req ppp.EventAuthRequest, respond subscriber.AuthRespondFunc) (result subscriber.AuthResult) {
	// Owner ruling 2026-10-18: "PPPOE expectation of those session should be
	// explicitly configured." An empty auth slot refuses every request, the
	// same as the L2TP drain, so no credential is admitted unchecked. A
	// configured no-auth session is admitted by the registered provider
	// (l2tp-auth-local, l2tp-auth-radius) after PPP's allow-no-auth guard.
	if handler == nil {
		return subscriber.AuthResult{Accept: false, Message: "no auth handler configured"}
	}
	// The handler task's boundary (panic_guard.go): a panicking handler refuses
	// this session's authentication, and the drain answers the next request.
	defer pppoePanics().auth.Contain(describeHandlerCall(req.TunnelID, req.SessionID), func(*taskguard.Panic) {
		result = subscriber.AuthResult{Accept: false, Message: "internal error"}
	})
	return handler(req, respond)
}

func startPPPoEPoolDrain(logger *slog.Logger, d *ppp.Driver, handler subscriber.PoolHandler) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for ev := range d.IPEventsOut() {
			req, ok := ev.(ppp.EventIPRequest)
			if !ok {
				continue
			}
			r := callPPPoEPoolHandler(handler, req)
			if err := d.IPResponse(req.TunnelID, req.SessionID, r); err != nil {
				logger.Warn("pppoe: pool drain response failed",
					"ifindex", req.TunnelID, "session", req.SessionID, "error", err)
			}
		}
	}()
	return done
}

func callPPPoEPoolHandler(handler subscriber.PoolHandler, req ppp.EventIPRequest) (result ppp.IPResponseArgs) {
	if handler == nil {
		return ppp.IPResponseArgs{Accept: false, Reason: "no pool handler"}
	}
	// The handler task's boundary (panic_guard.go): a panicking handler refuses
	// this session's address request, and the drain answers the next request.
	defer pppoePanics().pool.Contain(describeHandlerCall(req.TunnelID, req.SessionID), func(*taskguard.Panic) {
		result = ppp.IPResponseArgs{Accept: false, Reason: "internal error"}
	})
	return handler(req)
}
